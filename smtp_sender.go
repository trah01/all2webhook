package main

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
)

func sendToEmailNotification(recipient string, accounts map[string]EmailAccount, subject, from, date, body string) error {
	return sendToEmailNotificationWithAccount(recipient, "", accounts, subject, from, date, body)
}

func sendToEmailNotificationWithAccount(recipient string, smtpAccountID string, accounts map[string]EmailAccount, subject, from, date, body string) error {
	// 支持多个收件人（逗号分隔）
	parts := strings.Split(recipient, ",")
	var recipients []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && strings.Contains(p, "@") {
			recipients = append(recipients, p)
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("邮件通知目标需要填写收件邮箱地址")
	}

	var account EmailAccount
	var ok bool

	// 优先使用指定的 SMTP 账号
	if smtpAccountID != "" {
		if acc, exists := accounts[smtpAccountID]; exists {
			acc = normalizeEmailAccount(acc)
			if acc.Type == "smtp" && acc.Enabled {
				account = acc
				ok = true
			}
		}
	}

	// 否则自动选择
	if !ok {
		account, ok = selectSMTPAccount(accounts, recipients[0])
	}
	if !ok {
		return fmt.Errorf("未配置可用的 SMTP 发信账号")
	}

	messageSubject := "[All2Webhook] " + subject
	messageBody := fmt.Sprintf("主题：%s\n发送人：%s\n时间：%s\n\n%s", subject, from, date, body)
	return sendSMTPMail(account, recipients, messageSubject, messageBody)
}

func testSMTPAccount(account EmailAccount) error {
	account = normalizeEmailAccount(account)
	if account.Type != "smtp" {
		return fmt.Errorf("账号类型不是 SMTP")
	}
	return sendSMTPMail(account, []string{account.EmailUser}, "All2Webhook SMTP 测试", "这是一条 SMTP 发信测试消息。")
}

func selectSMTPAccount(accounts map[string]EmailAccount, recipient string) (EmailAccount, bool) {
	var fallback EmailAccount
	hasFallback := false
	for _, account := range accounts {
		account = normalizeEmailAccount(account)
		if account.Type != "smtp" || !account.Enabled {
			continue
		}
		if strings.EqualFold(account.EmailUser, recipient) {
			return account, true
		}
		if !hasFallback {
			fallback = account
			hasFallback = true
		}
	}
	return fallback, hasFallback
}

func sendSMTPMail(account EmailAccount, recipients []string, subject string, body string) error {
	account = normalizeEmailAccount(account)
	server := account.SmtpServer
	if server == "" {
		return fmt.Errorf("SMTP 服务器不能为空")
	}
	if !strings.Contains(server, ":") {
		server += ":587"
	}
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		return fmt.Errorf("SMTP 服务器格式无效: %w", err)
	}
	if account.EmailUser == "" || account.EmailPass == "" {
		return fmt.Errorf("SMTP 账号或密码为空")
	}

	headers := textproto.MIMEHeader{}
	headers.Set("From", account.EmailUser)
	headers.Set("To", strings.Join(recipients, ", "))
	headers.Set("Subject", mimeHeaderEncode(subject))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", `text/plain; charset="UTF-8"`)
	headers.Set("Content-Transfer-Encoding", "8bit")

	var message strings.Builder
	for key, values := range headers {
		for _, value := range values {
			message.WriteString(key)
			message.WriteString(": ")
			message.WriteString(value)
			message.WriteString("\r\n")
		}
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	auth := smtp.PlainAuth("", account.EmailUser, account.EmailPass, host)
	if port == "465" {
		conn, err := tls.Dial("tcp", server, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return err
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer client.Close()
		return sendSMTPWithClient(client, auth, account.EmailUser, recipients, []byte(message.String()))
	}

	client, err := smtp.Dial(server)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	return sendSMTPWithClient(client, auth, account.EmailUser, recipients, []byte(message.String()))
}

func sendSMTPWithClient(client *smtp.Client, auth smtp.Auth, from string, recipients []string, message []byte) error {
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func mimeHeaderEncode(value string) string {
	return mime.QEncoding.Encode("UTF-8", strings.TrimSpace(value))
}
