package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ===================== 消息转发 =====================

func sendToWebhookTarget(target WebhookTarget, accounts map[string]EmailAccount, subject, from, date, body string) error {
	if target.Type != "email" && strings.TrimSpace(target.Template) != "" {
		return sendTemplateWebhook(target, subject, from, date, body)
	}

	switch target.Type {
	case "feishu":
		return sendToFeishuWithTarget(target, subject, from, date, body)
	case "dingtalk":
		return sendToDingTalkWithTarget(target, subject, from, date, body)
	case "wecom":
		return sendToWeComWithTarget(target, subject, from, date, body)
	case "slack":
		return sendToSlackWithTarget(target, subject, from, date, body)
	case "discord":
		return sendToDiscordWithTarget(target, subject, from, date, body)
	case "teams":
		return sendToTeams(target, subject, from, date, body)
	case "mattermost":
		return sendToMattermost(target, subject, from, date, body)
	case "gotify":
		return sendToGotify(target, subject, body)
	case "bark":
		return sendToBark(target, subject, body)
	case "serverchan":
		return sendToServerChan(target, subject, body)
	case "telegram":
		return sendToTelegram(target, subject, body)
	case "ntfy":
		return sendToNtfy(target, subject, body)
	case "pushplus":
		return sendToPushPlus(target, subject, body)
	case "chanify":
		return sendToChanify(target, subject, body)
	case "pushover":
		return sendToPushover(target, subject, body)
	case "custom":
		return sendToCustomWebhookTarget(target, subject, from, date, body)
	case "email":
		return sendToEmailNotificationWithAccount(target.URL, target.SmtpAccountID, accounts, subject, from, date, body)
	default:
		return fmt.Errorf("不支持的 Webhook 类型: %s", target.Type)
	}
}

func sendTemplateWebhook(target WebhookTarget, subject, from, date, body string) error {
	rendered := renderWebhookTemplate(target.Template, subject, from, date, body)
	if !json.Valid([]byte(rendered)) {
		return fmt.Errorf("自定义 payload 模板不是合法 JSON")
	}
	return postJSONWithTarget(target, target.URL, []byte(rendered))
}

func renderWebhookTemplate(template, subject, from, date, body string) string {
	replacer := strings.NewReplacer(
		"{{subject}}", jsonStringContent(subject),
		"{{title}}", jsonStringContent(subject),
		"{{from}}", jsonStringContent(from),
		"{{date}}", jsonStringContent(date),
		"{{body}}", jsonStringContent(body),
		"{{message}}", jsonStringContent(body),
	)
	return replacer.Replace(template)
}

func jsonStringContent(value string) string {
	quoted, err := json.Marshal(value)
	if err != nil {
		return value
	}
	text := string(quoted)
	if len(text) >= 2 {
		return text[1 : len(text)-1]
	}
	return text
}

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

func sendToFeishuWithTarget(target WebhookTarget, subject, from, date, body string) error {
	payloadType := strings.ToLower(strings.TrimSpace(target.PayloadType))
	if payloadType == "" || payloadType == "interactive" {
		return sendToFeishuTarget(target, subject, from, date, body)
	}
	if len(body) > 15000 {
		body = body[:15000] + "..."
	}

	var payload map[string]interface{}
	switch payloadType {
	case "text":
		payload = map[string]interface{}{
			"msg_type": "text",
			"content": map[string]interface{}{
				"text": fmt.Sprintf("%s\n发送人: %s\n时间: %s\n\n%s", subject, from, date, body),
			},
		}
	case "post":
		payload = map[string]interface{}{
			"msg_type": "post",
			"content": map[string]interface{}{
				"post": map[string]interface{}{
					"zh_cn": map[string]interface{}{
						"title": subject,
						"content": [][]map[string]interface{}{
							{{"tag": "text", "text": fmt.Sprintf("发送人: %s\n时间: %s\n\n%s", from, date, body)}},
						},
					},
				},
			},
		}
	default:
		return sendToFeishuTarget(target, subject, from, date, body)
	}
	applyFeishuSignature(payload, target.Secret)
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToFeishuTarget(target WebhookTarget, subject, from, date, body string) error {
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
		applyFeishuSignature(payload, target.Secret)
		jsonBody, _ := json.Marshal(payload)
		if err := postJSONWithTarget(target, target.URL, jsonBody); err != nil {
			return err
		}
		time.Sleep(600 * time.Millisecond)
	}
	return nil
}

func applyFeishuSignature(payload map[string]interface{}, secret string) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	payload["timestamp"] = timestamp
	payload["sign"] = sign
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

func sendToCustomWebhookTarget(target WebhookTarget, subject, from, date, body string) error {
	payload := map[string]interface{}{
		"subject": subject,
		"from":    from,
		"date":    date,
		"body":    body,
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
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

func sendToDingTalkWithTarget(target WebhookTarget, subject, from, date, body string) error {
	webhookURL := signedDingTalkURL(target.URL, target.Secret)
	payloadType := strings.ToLower(strings.TrimSpace(target.PayloadType))
	if payloadType == "" {
		payloadType = "markdown"
	}
	if len(body) > 15000 {
		body = body[:15000] + "..."
	}

	var payload map[string]interface{}
	switch payloadType {
	case "text":
		payload = map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]interface{}{"content": fmt.Sprintf("%s\n\n%s", subject, body)},
		}
	case "actioncard":
		if target.LinkURL == "" {
			payload = map[string]interface{}{
				"msgtype": "markdown",
				"markdown": map[string]interface{}{
					"title": subject,
					"text":  fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body),
				},
			}
			break
		}
		payload = map[string]interface{}{
			"msgtype": "actionCard",
			"actionCard": map[string]interface{}{
				"title":          subject,
				"text":           fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body),
				"btnOrientation": "0",
				"singleTitle":    "查看详情",
				"singleURL":      target.LinkURL,
			},
		}
	case "feedcard":
		if target.LinkURL == "" {
			payload = map[string]interface{}{
				"msgtype": "markdown",
				"markdown": map[string]interface{}{
					"title": subject,
					"text":  fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body),
				},
			}
			break
		}
		payload = map[string]interface{}{
			"msgtype": "feedCard",
			"feedCard": map[string]interface{}{
				"links": []map[string]interface{}{
					{
						"title":      subject,
						"messageURL": target.LinkURL,
						"picURL":     target.IconURL,
					},
				},
			},
		}
	default:
		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"title": subject,
				"text":  fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body),
			},
		}
	}

	if target.MentionAll || target.MentionMobiles != "" {
		payload["at"] = map[string]interface{}{
			"atMobiles": splitCSV(target.MentionMobiles),
			"isAtAll":   target.MentionAll,
		}
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, webhookURL, jsonBody)
}

func signedDingTalkURL(webhookURL, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return webhookURL
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return webhookURL
	}
	q := parsed.Query()
	q.Set("timestamp", timestamp)
	q.Set("sign", sign)
	parsed.RawQuery = q.Encode()
	return parsed.String()
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

func sendToWeComWithTarget(target WebhookTarget, subject, from, date, body string) error {
	payloadType := strings.ToLower(strings.TrimSpace(target.PayloadType))
	if payloadType == "" {
		payloadType = "markdown"
	}
	fullText := fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body)
	runes := []rune(fullText)
	if len(runes) > 1300 {
		fullText = string(runes[:1300]) + "..."
	}

	var payload map[string]interface{}
	if payloadType == "text" {
		payload = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]interface{}{
				"content":               fmt.Sprintf("%s\n发送人: %s\n时间: %s\n\n%s", subject, from, date, body),
				"mentioned_mobile_list": splitCSV(target.MentionMobiles),
				"mentioned_list":        splitCSV(target.MentionUserIDs),
			},
		}
		if target.MentionAll {
			payload["text"].(map[string]interface{})["mentioned_list"] = []string{"@all"}
		}
	} else if payloadType == "news" && target.LinkURL != "" {
		payload = map[string]interface{}{
			"msgtype": "news",
			"news": map[string]interface{}{
				"articles": []map[string]interface{}{
					{
						"title":       subject,
						"description": fmt.Sprintf("发送人: %s\n时间: %s\n\n%s", from, date, body),
						"url":         target.LinkURL,
						"picurl":      target.IconURL,
					},
				},
			},
		}
	} else {
		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"content": fullText,
			},
		}
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToSlackWithTarget(target WebhookTarget, subject, from, date, body string) error {
	payloadType := strings.ToLower(strings.TrimSpace(target.PayloadType))
	if payloadType == "" {
		payloadType = "blocks"
	}
	payload := map[string]interface{}{}
	if target.Username != "" {
		payload["username"] = target.Username
	}
	if target.IconURL != "" {
		payload["icon_url"] = target.IconURL
	}
	if payloadType == "text" {
		payload["text"] = fmt.Sprintf("*%s*\n发送人: %s\n时间: %s\n\n%s", subject, from, date, body)
	} else if payloadType == "attachments" {
		payload["text"] = subject
		payload["attachments"] = []map[string]interface{}{
			{
				"fallback": subject,
				"title":    subject,
				"text":     body,
				"fields": []map[string]interface{}{
					{"title": "发送人", "value": from, "short": true},
					{"title": "时间", "value": date, "short": true},
				},
			},
		}
	} else {
		payload["text"] = subject
		payload["blocks"] = []map[string]interface{}{
			{"type": "header", "text": map[string]interface{}{"type": "plain_text", "text": subject}},
			{"type": "section", "fields": []map[string]interface{}{
				{"type": "mrkdwn", "text": fmt.Sprintf("*发送人:*\n%s", from)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*时间:*\n%s", date)},
			}},
			{"type": "divider"},
			{"type": "section", "text": map[string]interface{}{"type": "mrkdwn", "text": body}},
		}
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToDiscordWithTarget(target WebhookTarget, subject, from, date, body string) error {
	if len(body) > 1900 {
		body = body[:1900] + "..."
	}
	payloadType := strings.ToLower(strings.TrimSpace(target.PayloadType))
	payload := map[string]interface{}{"allowed_mentions": map[string]interface{}{"parse": []string{}}}
	if payloadType == "content" {
		payload["content"] = fmt.Sprintf("**%s**\n发送人: %s\n时间: %s\n\n%s", subject, from, date, body)
	} else {
		payload["content"] = ""
		payload["embeds"] = []map[string]interface{}{
			{
				"title":       subject,
				"description": body,
				"color":       3447003,
				"fields": []map[string]interface{}{
					{"name": "发送人", "value": from, "inline": true},
					{"name": "时间", "value": date, "inline": true},
				},
				"footer": map[string]interface{}{"text": "All2Webhook"},
			},
		}
	}
	if target.Username != "" {
		payload["username"] = target.Username
	}
	if target.IconURL != "" {
		payload["avatar_url"] = target.IconURL
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToTeams(target WebhookTarget, subject, from, date, body string) error {
	payloadType := strings.ToLower(strings.TrimSpace(target.PayloadType))
	if payloadType == "" {
		payloadType = "messagecard"
	}
	var payload map[string]interface{}
	if payloadType == "adaptivecard" {
		content := map[string]interface{}{
			"type":    "AdaptiveCard",
			"version": "1.4",
			"body": []map[string]interface{}{
				{"type": "TextBlock", "size": "Medium", "weight": "Bolder", "text": subject, "wrap": true},
				{"type": "FactSet", "facts": []map[string]string{{"title": "发送人", "value": from}, {"title": "时间", "value": date}}},
				{"type": "TextBlock", "text": body, "wrap": true},
			},
		}
		if target.LinkURL != "" {
			content["actions"] = []map[string]string{{"type": "Action.OpenUrl", "title": "查看详情", "url": target.LinkURL}}
		}
		payload = map[string]interface{}{
			"type": "message",
			"attachments": []map[string]interface{}{
				{
					"contentType": "application/vnd.microsoft.card.adaptive",
					"content":     content,
				},
			},
		}
	} else {
		payload = map[string]interface{}{
			"@type":    "MessageCard",
			"@context": "https://schema.org/extensions",
			"summary":  subject,
			"title":    subject,
			"text":     fmt.Sprintf("**发送人:** %s\n\n**时间:** %s\n\n%s", from, date, body),
			"sections": []map[string]interface{}{
				{"activityTitle": "All2Webhook", "facts": []map[string]string{{"name": "发送人", "value": from}, {"name": "时间", "value": date}}},
			},
		}
		if target.LinkURL != "" {
			payload["potentialAction"] = []map[string]interface{}{
				{"@type": "OpenUri", "name": "查看详情", "targets": []map[string]string{{"os": "default", "uri": target.LinkURL}}},
			}
		}
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToMattermost(target WebhookTarget, subject, from, date, body string) error {
	payload := map[string]interface{}{
		"text": fmt.Sprintf("### %s\n**发送人:** %s\n**时间:** %s\n\n%s", subject, from, date, body),
	}
	if target.Username != "" {
		payload["username"] = target.Username
	}
	if target.IconURL != "" {
		payload["icon_url"] = target.IconURL
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToGotify(target WebhookTarget, subject, body string) error {
	webhookURL := target.URL
	if target.Token != "" {
		parsed, err := url.Parse(webhookURL)
		if err == nil {
			q := parsed.Query()
			q.Set("token", target.Token)
			parsed.RawQuery = q.Encode()
			webhookURL = parsed.String()
		}
	}
	priority := target.Priority
	if priority == 0 {
		priority = 5
	}
	payload := map[string]interface{}{
		"title":    subject,
		"message":  body,
		"priority": priority,
		"extras": map[string]interface{}{
			"client::display": map[string]interface{}{"contentType": "text/markdown"},
		},
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, webhookURL, jsonBody)
}

func sendToBark(target WebhookTarget, subject, body string) error {
	payload := map[string]interface{}{
		"title": subject,
		"body":  body,
		"group": "All2Webhook",
	}
	if target.IconURL != "" {
		payload["icon"] = target.IconURL
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToServerChan(target WebhookTarget, subject, body string) error {
	payload := map[string]interface{}{
		"title": subject,
		"desp":  body,
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToTelegram(target WebhookTarget, subject, body string) error {
	webhookURL := strings.TrimSpace(target.URL)
	token := strings.TrimSpace(target.Token)
	if webhookURL == "" && token != "" {
		webhookURL = "https://api.telegram.org/bot" + token + "/sendMessage"
	} else if webhookURL != "" && !strings.HasPrefix(webhookURL, "http") {
		webhookURL = "https://api.telegram.org/bot" + webhookURL + "/sendMessage"
	}
	payload := map[string]interface{}{
		"chat_id":                  target.ChatID,
		"text":                     fmt.Sprintf("*%s*\n\n%s", subject, body),
		"parse_mode":               "Markdown",
		"disable_web_page_preview": true,
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, webhookURL, jsonBody)
}

func sendToNtfy(target WebhookTarget, subject, body string) error {
	priority := target.Priority
	if priority == 0 {
		priority = 3
	}
	payload := map[string]interface{}{
		"title":    subject,
		"message":  body,
		"priority": priority,
		"markdown": true,
	}
	if target.Token != "" && strings.TrimSpace(target.Headers) == "" {
		target.Headers = fmt.Sprintf(`{"Authorization":"Bearer %s"}`, target.Token)
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToPushPlus(target WebhookTarget, subject, body string) error {
	payload := map[string]interface{}{
		"token":    target.Token,
		"title":    subject,
		"content":  body,
		"template": "markdown",
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToChanify(target WebhookTarget, subject, body string) error {
	payload := map[string]interface{}{
		"title": subject,
		"text":  body,
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}

func sendToPushover(target WebhookTarget, subject, body string) error {
	payload := map[string]interface{}{
		"token":    target.Token,
		"user":     target.ChatID,
		"title":    subject,
		"message":  body,
		"priority": target.Priority,
	}
	jsonBody, _ := json.Marshal(payload)
	return postJSONWithTarget(target, target.URL, jsonBody)
}
