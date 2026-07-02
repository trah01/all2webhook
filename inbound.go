package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxInboundBodyBytes = 1024 * 1024

type inboundPayload struct {
	Subject string
	From    string
	Body    string
}

func setupPublicAPI(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.Any("/hook/:secret", handleInboundHook)
	r.Any("/webhook/:secret", handleInboundHook)
}

func handleInboundHook(c *gin.Context) {
	project, err := getInboundProjectBySecret(c.Param("secret"))
	if err != nil {
		addLog(fmt.Sprintf("接收入口查询失败: %v", err), "error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if project == nil || !project.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	bodyBytes, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxInboundBodyBytes))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
		return
	}

	payload := parseInboundPayload(c.Request, bodyBytes)
	now := time.Now()
	msg := Message{
		ID:          fmt.Sprintf("msg_%d", now.UnixNano()),
		SourceEmail: project.Name,
		AccountID:   project.ID,
		Subject:     payload.Subject,
		From:        payload.From,
		To:          "all2webhook",
		Date:        now,
		Body:        payload.Body,
		BodyHTML:    "",
		Status:      "pending",
		CreatedAt:   now,
	}
	if err := saveMessage(&msg); err != nil {
		addLog(fmt.Sprintf("保存外部通知失败 [%s]: %v", project.Name, err), "error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message"})
		return
	}

	addLog(fmt.Sprintf("已接收外部通知 [%s]: %s", project.Name, displaySubject(msg.Subject)), "success")
	c.JSON(http.StatusAccepted, gin.H{"success": true, "message_id": msg.ID})
}

func parseInboundPayload(req *http.Request, body []byte) inboundPayload {
	event := strings.TrimSpace(req.Header.Get("X-GitHub-Event"))
	contentType := strings.ToLower(req.Header.Get("Content-Type"))
	from := firstNonEmpty(req.Header.Get("X-GitHub-Delivery"), req.RemoteAddr, "external")

	if strings.Contains(contentType, "application/json") || json.Valid(body) {
		var data interface{}
		if err := json.Unmarshal(body, &data); err == nil {
			subject, _ := extractJSONSummary(data)
			if event != "" {
				subject = firstNonEmpty(formatGitHubSubject(event, data), subject)
				from = "github"
			}
			return inboundPayload{
				Subject: firstNonEmpty(subject, "外部通知"),
				From:    from,
				Body:    formatJSONBody(data, body),
			}
		}
	}

	bodyText := strings.TrimSpace(string(body))
	if bodyText == "" {
		bodyText = "(空请求体)"
	}
	return inboundPayload{
		Subject: firstNonEmpty(req.Header.Get("X-Title"), req.Header.Get("X-Subject"), "外部通知"),
		From:    from,
		Body:    buildRawRequestBody(req, bodyText),
	}
}

func extractJSONSummary(value interface{}) (string, string) {
	obj, ok := value.(map[string]interface{})
	if !ok {
		return "外部通知", compactJSONString(value)
	}

	titleKeys := []string{"title", "subject", "summary", "event", "name"}
	textKeys := []string{"message", "text", "content", "body", "description", "markdown"}
	title := firstJSONText(obj, titleKeys...)
	text := firstJSONText(obj, textKeys...)

	if gotifyTitle := firstJSONText(obj, "title"); gotifyTitle != "" {
		title = gotifyTitle
	}
	if barkTitle := firstJSONText(obj, "title"); barkTitle != "" {
		title = barkTitle
	}
	if feishuMsgType := firstJSONText(obj, "msg_type"); feishuMsgType != "" && title == "" {
		title = "飞书通知"
	}
	if title == "" && text != "" {
		title = firstLine(text)
	}
	if text == "" {
		text = compactJSONString(value)
	}
	return firstNonEmpty(title, "外部通知"), text
}

func formatGitHubSubject(event string, data interface{}) string {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return "GitHub " + event
	}
	repo := nestedJSONText(obj, "repository", "full_name")
	action := firstJSONText(obj, "action")
	switch event {
	case "push":
		ref := firstJSONText(obj, "ref")
		return strings.TrimSpace(fmt.Sprintf("GitHub push %s %s", repo, ref))
	case "pull_request":
		number := jsonNumberText(obj["number"])
		title := nestedJSONText(obj, "pull_request", "title")
		return strings.TrimSpace(fmt.Sprintf("GitHub PR #%s %s %s", number, action, title))
	case "issues":
		number := jsonNumberText(obj["number"])
		title := nestedJSONText(obj, "issue", "title")
		return strings.TrimSpace(fmt.Sprintf("GitHub Issue #%s %s %s", number, action, title))
	default:
		return strings.TrimSpace(fmt.Sprintf("GitHub %s %s %s", event, repo, action))
	}
}

func formatJSONBody(data interface{}, original []byte) string {
	_, text := extractJSONSummary(data)
	var out bytes.Buffer
	if strings.TrimSpace(text) != "" {
		out.WriteString(text)
		out.WriteString("\n\n")
	}
	out.WriteString("```json\n")
	formatted := compactJSONString(data)
	if strings.TrimSpace(formatted) == "" {
		formatted = string(original)
	}
	out.WriteString(formatted)
	out.WriteString("\n```")
	return out.String()
}

func buildRawRequestBody(req *http.Request, bodyText string) string {
	headers := []string{}
	for _, name := range []string{"User-Agent", "Content-Type", "X-GitHub-Event", "X-GitHub-Delivery"} {
		if value := req.Header.Get(name); value != "" {
			headers = append(headers, fmt.Sprintf("%s: %s", name, value))
		}
	}
	sort.Strings(headers)
	if len(headers) == 0 {
		return bodyText
	}
	return strings.Join(headers, "\n") + "\n\n" + bodyText
}

func firstJSONText(obj map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			if text := jsonValueText(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func nestedJSONText(obj map[string]interface{}, parent string, child string) string {
	nested, ok := obj[parent].(map[string]interface{})
	if !ok {
		return ""
	}
	return jsonValueText(nested[child])
}

func jsonValueText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return jsonNumberText(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func jsonNumberText(value interface{}) string {
	switch v := value.(type) {
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.0f", v), "0"), ".")
	case int:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return ""
	}
}

func compactJSONString(value interface{}) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
