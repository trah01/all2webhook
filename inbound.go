package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type collectedTextField struct {
	Key   string
	Label string
	Value string
}

func setupPublicAPI(r *gin.Engine) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.Any("/hook/:secret", handleInboundHook)
	r.Any("/hook/:secret/*extra", handleInboundHook)
	r.Any("/webhook/:secret", handleInboundHook)
	r.Any("/webhook/:secret/*extra", handleInboundHook)
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

	if parsed := parseBarkGETPayload(req); parsed != nil {
		return *parsed
	}

	if strings.Contains(contentType, "application/json") || json.Valid(body) {
		var data interface{}
		if err := json.Unmarshal(body, &data); err == nil {
			if parsed := parseKnownJSONWebhook(data, event); parsed != nil {
				parsed.From = firstNonEmpty(parsed.From, from)
				return *parsed
			}
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

	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		values, err := url.ParseQuery(string(body))
		if err == nil {
			subject := firstNonEmpty(values.Get("title"), values.Get("subject"), values.Get("summary"), values.Get("text"))
			text := firstNonEmpty(values.Get("message"), values.Get("body"), values.Get("desp"), values.Get("content"), values.Get("text"))
			if text != "" {
				return inboundPayload{
					Subject: firstNonEmpty(subject, firstLine(text), "表单通知"),
					From:    firstNonEmpty(values.Get("source"), values.Get("from"), from),
					Body:    text,
				}
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

func parseBarkGETPayload(req *http.Request) *inboundPayload {
	if req.Method != http.MethodGet {
		return nil
	}

	query := req.URL.Query()
	subject := firstNonEmpty(query.Get("title"), query.Get("subject"))
	body := firstNonEmpty(query.Get("body"), query.Get("message"), query.Get("text"), query.Get("content"), query.Get("desp"))
	group := strings.TrimSpace(query.Get("group"))

	segments := extractBarkPathSegments(req.URL.EscapedPath())
	switch {
	case len(segments) >= 3:
		group = firstNonEmpty(group, segments[0])
		subject = firstNonEmpty(subject, segments[1])
		body = firstNonEmpty(body, strings.Join(segments[2:], "/"))
	case len(segments) == 2:
		subject = firstNonEmpty(subject, segments[0])
		body = firstNonEmpty(body, segments[1])
	case len(segments) == 1:
		body = firstNonEmpty(body, segments[0])
	}

	if subject == "" && body == "" {
		return nil
	}

	lines := []string{body}
	for _, key := range []string{"subtitle", "url", "sound", "icon", "level", "badge", "copy", "autoCopy"} {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			lines = append(lines, fmt.Sprintf("%s：%s", key, value))
		}
	}
	if group != "" {
		lines = append(lines, fmt.Sprintf("group：%s", group))
	}

	return &inboundPayload{
		Subject: firstNonEmpty(subject, firstLine(body), "Bark 通知"),
		From:    firstNonEmpty(query.Get("source"), query.Get("from"), "bark"),
		Body:    strings.Join(nonEmptyLines(lines), "\n\n"),
	}
}

func extractBarkPathSegments(escapedPath string) []string {
	path := strings.Trim(escapedPath, "/")
	if path == "" {
		return nil
	}

	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return nil
	}
	if parts[0] != "hook" && parts[0] != "webhook" {
		return nil
	}

	segments := make([]string, 0, len(parts)-2)
	for _, part := range parts[2:] {
		if part == "" {
			continue
		}
		text, err := url.PathUnescape(part)
		if err != nil {
			text = part
		}
		text = strings.TrimSpace(text)
		if text != "" {
			segments = append(segments, text)
		}
	}
	return segments
}

func parseKnownJSONWebhook(data interface{}, event string) *inboundPayload {
	obj, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	if event != "" {
		return &inboundPayload{
			Subject: firstNonEmpty(formatGitHubSubject(event, data), "GitHub 通知"),
			From:    "github",
			Body:    formatGitHubBody(event, obj),
		}
	}
	if payload := parseDiscordWebhook(obj); payload != nil {
		return payload
	}
	if payload := parseFeishuWebhook(obj); payload != nil {
		return payload
	}
	if payload := parseDingTalkWebhook(obj); payload != nil {
		return payload
	}
	if payload := parseWeComWebhook(obj); payload != nil {
		return payload
	}
	if payload := parseSimpleWebhook(obj); payload != nil {
		return payload
	}
	if payload := parseGenericNestedWebhook(obj); payload != nil {
		return payload
	}
	return nil
}

func parseDiscordWebhook(obj map[string]interface{}) *inboundPayload {
	content := firstJSONText(obj, "content")
	embeds := jsonArray(obj["embeds"])
	if content == "" && len(embeds) == 0 {
		return nil
	}

	lines := []string{}
	if content != "" {
		lines = append(lines, content)
	}
	subject := firstNonEmpty(firstLine(content), "Discord 通知")
	for _, embed := range embeds {
		title := firstJSONText(embed, "title")
		description := firstJSONText(embed, "description")
		if subject == "Discord 通知" && title != "" {
			subject = title
		}
		appendSection(&lines, title, description)
		if urlText := firstJSONText(embed, "url"); urlText != "" {
			lines = append(lines, fmt.Sprintf("链接：%s", urlText))
		}
		for _, field := range jsonArray(embed["fields"]) {
			name := firstJSONText(field, "name")
			value := firstJSONText(field, "value")
			appendSection(&lines, name, value)
		}
		if footer := nestedJSONText(embed, "footer", "text"); footer != "" {
			lines = append(lines, fmt.Sprintf("备注：%s", footer))
		}
	}
	return &inboundPayload{
		Subject: subject,
		From:    firstNonEmpty(firstJSONText(obj, "username"), "discord"),
		Body:    strings.Join(nonEmptyLines(lines), "\n\n"),
	}
}

func parseFeishuWebhook(obj map[string]interface{}) *inboundPayload {
	msgType := firstJSONText(obj, "msg_type")
	if msgType == "" {
		return nil
	}
	content := jsonObject(obj["content"])
	lines := []string{}
	subject := "飞书通知"

	switch msgType {
	case "text":
		text := firstJSONText(content, "text")
		subject = firstNonEmpty(firstLine(text), subject)
		lines = append(lines, text)
	case "post":
		post := jsonObject(content["post"])
		zhCN := jsonObject(post["zh_cn"])
		subject = firstNonEmpty(firstJSONText(zhCN, "title"), subject)
		lines = append(lines, extractFeishuPostContent(zhCN)...)
	case "interactive":
		card := jsonObject(obj["card"])
		header := jsonObject(card["header"])
		titleObj := jsonObject(header["title"])
		subject = firstNonEmpty(firstJSONText(titleObj, "content"), subject)
		lines = append(lines, extractFeishuCardElements(jsonArray(card["elements"]))...)
	default:
		text := firstJSONText(content, "text", "content")
		subject = firstNonEmpty(firstLine(text), subject)
		lines = append(lines, text)
	}
	if len(nonEmptyLines(lines)) == 0 {
		return nil
	}
	return &inboundPayload{Subject: subject, From: "feishu", Body: strings.Join(nonEmptyLines(lines), "\n\n")}
}

func parseDingTalkWebhook(obj map[string]interface{}) *inboundPayload {
	msgType := firstJSONText(obj, "msgtype")
	if msgType == "" {
		return nil
	}
	lines := []string{}
	subject := "钉钉通知"
	switch msgType {
	case "text":
		textObj := jsonObject(obj["text"])
		text := firstJSONText(textObj, "content")
		if text == "" {
			return nil
		}
		subject = firstNonEmpty(firstLine(text), subject)
		lines = append(lines, text)
	case "markdown":
		markdown := jsonObject(obj["markdown"])
		if firstJSONText(markdown, "title", "text") == "" {
			return nil
		}
		subject = firstNonEmpty(firstJSONText(markdown, "title"), subject)
		lines = append(lines, firstJSONText(markdown, "text"))
	case "link":
		link := jsonObject(obj["link"])
		subject = firstNonEmpty(firstJSONText(link, "title"), subject)
		appendSection(&lines, firstJSONText(link, "title"), firstJSONText(link, "text"))
		if messageURL := firstJSONText(link, "messageUrl"); messageURL != "" {
			lines = append(lines, fmt.Sprintf("链接：%s", messageURL))
		}
	case "actionCard":
		card := jsonObject(obj["actionCard"])
		subject = firstNonEmpty(firstJSONText(card, "title"), subject)
		lines = append(lines, firstJSONText(card, "text"))
	default:
		return nil
	}
	return &inboundPayload{Subject: subject, From: "dingtalk", Body: strings.Join(nonEmptyLines(lines), "\n\n")}
}

func parseWeComWebhook(obj map[string]interface{}) *inboundPayload {
	msgType := firstJSONText(obj, "msgtype")
	if msgType == "" {
		return nil
	}
	lines := []string{}
	subject := "企业微信通知"
	switch msgType {
	case "text":
		textObj := jsonObject(obj["text"])
		text := firstJSONText(textObj, "content")
		if text == "" {
			return nil
		}
		subject = firstNonEmpty(firstLine(text), subject)
		lines = append(lines, text)
	case "markdown":
		markdown := jsonObject(obj["markdown"])
		text := firstJSONText(markdown, "content")
		if text == "" {
			return nil
		}
		subject = firstNonEmpty(firstMarkdownHeading(text), firstLine(text), subject)
		lines = append(lines, text)
	case "news":
		news := jsonObject(obj["news"])
		for _, article := range jsonArray(news["articles"]) {
			title := firstJSONText(article, "title")
			description := firstJSONText(article, "description")
			if subject == "企业微信通知" && title != "" {
				subject = title
			}
			appendSection(&lines, title, description)
			if urlText := firstJSONText(article, "url"); urlText != "" {
				lines = append(lines, fmt.Sprintf("链接：%s", urlText))
			}
		}
	default:
		return nil
	}
	return &inboundPayload{Subject: subject, From: "wecom", Body: strings.Join(nonEmptyLines(lines), "\n\n")}
}

func parseSimpleWebhook(obj map[string]interface{}) *inboundPayload {
	title := firstJSONText(obj, "title", "subject", "summary")
	text := firstJSONText(obj, "message", "body", "desp", "description", "content", "text")
	if title == "" && text == "" {
		return nil
	}
	lines := []string{text}
	for _, key := range []string{"subtitle", "url", "group", "priority", "level", "event"} {
		if value := firstJSONText(obj, key); value != "" {
			lines = append(lines, fmt.Sprintf("%s：%s", key, value))
		}
	}
	return &inboundPayload{
		Subject: firstNonEmpty(title, firstLine(text), "外部通知"),
		From:    firstNonEmpty(firstJSONText(obj, "source", "from", "app"), "webhook"),
		Body:    strings.Join(nonEmptyLines(lines), "\n\n"),
	}
}

func parseGenericNestedWebhook(obj map[string]interface{}) *inboundPayload {
	fields := collectTextFields(obj, "")
	if len(fields) == 0 {
		return nil
	}

	subject := firstCollectedValue(fields, "title", "subject", "summary", "name", "event")
	body := firstCollectedValue(fields, "message", "body", "desp", "description", "content", "text", "markdown")
	lines := []string{}
	if body != "" {
		lines = append(lines, body)
	}

	for _, field := range fields {
		if field.Value == "" || field.Value == subject || field.Value == body {
			continue
		}
		if isNoisyJSONKey(field.Key) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s：%s", field.Label, field.Value))
		if len(lines) >= 10 {
			break
		}
	}

	if len(nonEmptyLines(lines)) == 0 {
		return nil
	}
	return &inboundPayload{
		Subject: firstNonEmpty(subject, firstLine(body), "外部通知"),
		From:    firstNonEmpty(firstCollectedValue(fields, "source", "from", "app", "service"), "webhook"),
		Body:    strings.Join(nonEmptyLines(lines), "\n\n"),
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

func formatGitHubBody(event string, obj map[string]interface{}) string {
	lines := []string{}
	repo := nestedJSONText(obj, "repository", "full_name")
	if repo != "" {
		lines = append(lines, fmt.Sprintf("仓库：%s", repo))
	}
	action := firstJSONText(obj, "action")
	if action != "" {
		lines = append(lines, fmt.Sprintf("动作：%s", action))
	}
	switch event {
	case "push":
		lines = append(lines, fmt.Sprintf("分支：%s", firstJSONText(obj, "ref")))
		if pusher := nestedJSONText(obj, "pusher", "name"); pusher != "" {
			lines = append(lines, fmt.Sprintf("推送者：%s", pusher))
		}
		for _, commit := range jsonArray(obj["commits"]) {
			message := firstJSONText(commit, "message")
			author := nestedJSONText(commit, "author", "name")
			urlText := firstJSONText(commit, "url")
			line := strings.TrimSpace(fmt.Sprintf("- %s", firstLine(message)))
			if author != "" {
				line += fmt.Sprintf(" (%s)", author)
			}
			if urlText != "" {
				line += fmt.Sprintf("\n  %s", urlText)
			}
			lines = append(lines, line)
		}
	case "pull_request":
		pr := jsonObject(obj["pull_request"])
		appendSection(&lines, nestedJSONText(pr, "user", "login"), firstJSONText(pr, "title"))
		if body := firstJSONText(pr, "body"); body != "" {
			lines = append(lines, body)
		}
		if urlText := firstJSONText(pr, "html_url"); urlText != "" {
			lines = append(lines, fmt.Sprintf("链接：%s", urlText))
		}
	case "issues":
		issue := jsonObject(obj["issue"])
		appendSection(&lines, nestedJSONText(issue, "user", "login"), firstJSONText(issue, "title"))
		if body := firstJSONText(issue, "body"); body != "" {
			lines = append(lines, body)
		}
		if urlText := firstJSONText(issue, "html_url"); urlText != "" {
			lines = append(lines, fmt.Sprintf("链接：%s", urlText))
		}
	default:
		if sender := nestedJSONText(obj, "sender", "login"); sender != "" {
			lines = append(lines, fmt.Sprintf("触发者：%s", sender))
		}
	}
	if len(nonEmptyLines(lines)) == 0 {
		return compactJSONString(obj)
	}
	return strings.Join(nonEmptyLines(lines), "\n\n")
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

func extractFeishuPostContent(post map[string]interface{}) []string {
	lines := []string{}
	for _, row := range jsonNestedArrays(post["content"]) {
		parts := []string{}
		for _, item := range row {
			text := firstJSONText(item, "text")
			if text == "" {
				text = firstJSONText(item, "href", "user_name")
			}
			if text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, strings.Join(parts, ""))
		}
	}
	return lines
}

func extractFeishuCardElements(elements []map[string]interface{}) []string {
	lines := []string{}
	for _, element := range elements {
		tag := firstJSONText(element, "tag")
		switch tag {
		case "div", "markdown":
			textObj := jsonObject(element["text"])
			lines = append(lines, firstJSONText(textObj, "content"))
		case "note":
			parts := []string{}
			for _, item := range jsonArray(element["elements"]) {
				if text := firstJSONText(item, "content", "text"); text != "" {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 {
				lines = append(lines, strings.Join(parts, " "))
			}
		case "hr":
			continue
		default:
			if text := firstJSONText(element, "content", "text"); text != "" {
				lines = append(lines, text)
			}
		}
	}
	return lines
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

func jsonObject(value interface{}) map[string]interface{} {
	obj, _ := value.(map[string]interface{})
	if obj == nil {
		return map[string]interface{}{}
	}
	return obj
}

func jsonArray(value interface{}) []map[string]interface{} {
	values, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		if obj, ok := value.(map[string]interface{}); ok {
			result = append(result, obj)
		}
	}
	return result
}

func jsonNestedArrays(value interface{}) [][]map[string]interface{} {
	rows, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([][]map[string]interface{}, 0, len(rows))
	for _, rowValue := range rows {
		items, ok := rowValue.([]interface{})
		if !ok {
			continue
		}
		row := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if obj, ok := item.(map[string]interface{}); ok {
				row = append(row, obj)
			}
		}
		result = append(result, row)
	}
	return result
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

func firstMarkdownHeading(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
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

func appendSection(lines *[]string, title string, body string) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	switch {
	case title != "" && body != "":
		*lines = append(*lines, fmt.Sprintf("**%s**\n%s", title, body))
	case title != "":
		*lines = append(*lines, title)
	case body != "":
		*lines = append(*lines, body)
	}
}

func nonEmptyLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func collectTextFields(value interface{}, path string) []collectedTextField {
	result := []collectedTextField{}
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			result = append(result, collectTextFields(item, childPath)...)
			if len(result) >= 40 {
				return result
			}
		}
	case []interface{}:
		for i, item := range v {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			result = append(result, collectTextFields(item, childPath)...)
			if len(result) >= 40 {
				return result
			}
		}
	default:
		text := jsonValueText(v)
		if text != "" {
			result = append(result, collectedTextField{
				Key:   lastJSONPathKey(path),
				Label: readableJSONPath(path),
				Value: text,
			})
		}
	}
	return result
}

func firstCollectedValue(fields []collectedTextField, keys ...string) string {
	for _, key := range keys {
		for _, field := range fields {
			if strings.EqualFold(field.Key, key) && strings.TrimSpace(field.Value) != "" {
				return strings.TrimSpace(field.Value)
			}
		}
	}
	return ""
}

func lastJSONPathKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		path = path[idx+1:]
	}
	if idx := strings.Index(path, "["); idx >= 0 {
		path = path[:idx]
	}
	return path
}

func readableJSONPath(path string) string {
	path = strings.ReplaceAll(path, ".", " / ")
	path = strings.ReplaceAll(path, "_", " ")
	return path
}

func isNoisyJSONKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "icon", "icon_url", "avatar_url", "thumbnail", "image", "image_url", "picurl", "picture":
		return true
	default:
		return false
	}
}
