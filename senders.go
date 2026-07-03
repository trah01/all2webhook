package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// ===================== 消息转发 =====================

func sendToFeishu(webhookURL, subject, from, date, body string) error {
	runes := []rune(body)
	chunkSize := 1800
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	totalBatches := (len(chunks) + 9) / 10
	for i := 0; i < len(chunks); i += 10 {
		end := i + 10
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		idx := i/10 + 1
		isFirst := (i == 0)

		elements := []interface{}{}
		if isFirst {
			elements = append(elements, map[string]interface{}{
				"tag":  "div",
				"text": map[string]interface{}{"tag": "lark_md", "content": fmt.Sprintf("**主题：** %s\n**发送人：** %s\n**时间：** %s", subject, from, date)},
			})
			elements = append(elements, map[string]interface{}{"tag": "hr"})
		}

		for _, txt := range batch {
			if strings.TrimSpace(txt) != "" {
				elements = append(elements, map[string]interface{}{
					"tag":  "div",
					"text": map[string]interface{}{"tag": "lark_md", "content": txt},
				})
			}
		}

		elements = append(elements, map[string]interface{}{
			"tag":      "note",
			"elements": []map[string]interface{}{{"tag": "plain_text", "content": fmt.Sprintf("[All2Webhook] Part %d / %d", idx, totalBatches)}},
		})

		card := map[string]interface{}{
			"config": map[string]interface{}{"wide_screen_mode": true},
			"header": map[string]interface{}{
				"template": "turquoise",
				"title":    map[string]interface{}{"tag": "plain_text", "content": subject},
			},
			"elements": elements,
		}
		if !isFirst {
			card["header"].(map[string]interface{})["template"] = "grey"
			card["header"].(map[string]interface{})["title"].(map[string]interface{})["content"] = "[邮件正文续接]"
		}

		payload := map[string]interface{}{"msg_type": "interactive", "card": card}
		jsonBody, _ := json.Marshal(payload)
		resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
		}
		resp.Body.Close()
		time.Sleep(600 * time.Millisecond)
	}
	return nil
}

func sendToSlack(webhookURL, subject, from, date, body string) error {
	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": subject,
				},
			},
			{
				"type": "section",
				"fields": []map[string]interface{}{
					{"type": "mrkdwn", "text": fmt.Sprintf("*发送人:*\n%s", from)},
					{"type": "mrkdwn", "text": fmt.Sprintf("*时间:*\n%s", date)},
				},
			},
			{
				"type": "divider",
			},
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": body,
				},
			},
		},
	}
	jsonBody, _ := json.Marshal(payload)
	resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func sendToDiscord(webhookURL, subject, from, date, body string) error {
	// Discord 限制 2000 字符
	if len(body) > 1900 {
		body = body[:1900] + "..."
	}
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       subject,
				"description": body,
				"color":       3447003,
				"fields": []map[string]interface{}{
					{"name": "发送人", "value": from, "inline": true},
					{"name": "时间", "value": date, "inline": true},
				},
				"footer": map[string]interface{}{
					"text": "All2Webhook",
				},
			},
		},
	}
	jsonBody, _ := json.Marshal(payload)
	resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func sendToCustomWebhook(webhookURL, subject, from, date, body string) error {
	payload := map[string]interface{}{
		"subject": subject,
		"from":    from,
		"date":    date,
		"body":    body,
	}
	jsonBody, _ := json.Marshal(payload)
	resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func sendToDingTalk(webhookURL, subject, from, date, body string) error {
	if len(body) > 15000 {
		body = body[:15000] + "..."
	}
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": subject,
			"text":  fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body),
		},
	}
	jsonBody, _ := json.Marshal(payload)
	resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func sendToWeCom(webhookURL, subject, from, date, body string) error {
	fullText := fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body)
	runes := []rune(fullText)
	if len(runes) > 1300 {
		fullText = string(runes[:1300]) + "..."
	}
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fullText,
		},
	}
	jsonBody, _ := json.Marshal(payload)
	resp, err := httpClient.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

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
