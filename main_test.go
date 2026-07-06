package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestCleanHTML_PreservesReadableStructure(t *testing.T) {
	html := `<html><body><h1>标题</h1><p>第一段</p><p>第二段</p><ul><li>事项A</li><li>事项B</li></ul><a href="https://example.com/path?utm_source=test">点击查看详情</a></body></html>`

	got := cleanHTML(html)

	if !strings.Contains(got, "标题") {
		t.Fatalf("expected title in output, got: %q", got)
	}
	if !strings.Contains(got, "第一段\n\n第二段") {
		t.Fatalf("expected paragraph break preserved, got: %q", got)
	}
	if !strings.Contains(got, "- 事项A") || !strings.Contains(got, "- 事项B") {
		t.Fatalf("expected list items to be bullet points, got: %q", got)
	}
	if !strings.Contains(got, "[点击查看详情](https://example.com/path)") {
		t.Fatalf("expected tracking params removed and markdown link kept, got: %q", got)
	}
}

func TestFormatPlainTextBody_ConvertsURLToMarkdownAndTruncatesLongLinks(t *testing.T) {
	body := "请查看 https://example.com/docs?id=1 和这个超长链接 https://example.com/" + strings.Repeat("a", 650)

	got := formatPlainTextBody(body)

	if !strings.Contains(got, "[https://example.com/docs?id=1](https://example.com/docs?id=1)") {
		t.Fatalf("expected normal URL converted to markdown link, got: %q", got)
	}
	if !strings.Contains(got, "长链接由于超长已被过滤") {
		t.Fatalf("expected overlong URL filtered message, got: %q", got)
	}
}

func TestCleanHTML_EscapesMarkdownLinkTextAndBlocksUnsafeScheme(t *testing.T) {
	html := `<html><body><a href="javascript:alert(1)">x](y)</a><a href="https://example.com/a_(b)?utm_source=xx">括号](链接)</a></body></html>`

	got := cleanHTML(html)

	if strings.Contains(got, "javascript:") {
		t.Fatalf("expected javascript scheme to be removed, got: %q", got)
	}
	if !strings.Contains(got, `x\\]\\(y\\)`) {
		t.Fatalf("expected unsafe-scheme anchor text kept as escaped plain text, got: %q", got)
	}
	if !strings.Contains(got, "括号") || !strings.Contains(got, "链接") {
		t.Fatalf("expected anchor text preserved, got: %q", got)
	}
	if !strings.Contains(got, "https://example.com/a_%28b%29") {
		t.Fatalf("expected escaped markdown link URL destination, got: %q", got)
	}
}

func TestFormatPlainTextBody_FirstLineUsesTwoSpaceIndent(t *testing.T) {
	body := "        第一行有很多前导空格\n\n第二行正文"

	got := formatPlainTextBody(body)

	lines := strings.Split(got, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected non-empty output")
	}
	if !strings.HasPrefix(lines[0], "  ") {
		t.Fatalf("expected first line to have two-space indent, got: %q", lines[0])
	}
	if strings.HasPrefix(lines[0], "   ") {
		t.Fatalf("expected first line to be exactly two spaces, got: %q", lines[0])
	}
}

func TestApplyFilterRules_BlacklistBlocksSender(t *testing.T) {
	msg := &Message{From: "newsletter@example.com", Subject: "周报", Body: "正文"}
	ctx := filterContext{DisplaySender: msg.From}
	rules := map[string]FilterRule{
		"filter_1": {
			ID:       "filter_1",
			Name:     "屏蔽发送人",
			Type:     "sender",
			Mode:     "blacklist",
			Patterns: []string{"example.com"},
			Enabled:  true,
		},
	}

	got := applyFilterRules([]string{"filter_1"}, rules, msg, ctx)

	if got.Allowed {
		t.Fatalf("expected blacklist sender rule to block message")
	}
}

func TestApplyFilterRules_WhitelistRequiresContentMatch(t *testing.T) {
	msg := &Message{From: "notice@example.com", Subject: "普通通知", Body: "没有关键词"}
	ctx := filterContext{DisplaySender: msg.From}
	rules := map[string]FilterRule{
		"filter_1": {
			ID:       "filter_1",
			Name:     "只允许账单",
			Type:     "content",
			Mode:     "whitelist",
			Patterns: []string{"账单"},
			Enabled:  true,
		},
	}

	got := applyFilterRules([]string{"filter_1"}, rules, msg, ctx)

	if got.Allowed {
		t.Fatalf("expected whitelist content rule to block unmatched message")
	}

	msg.Subject = "本月账单"
	got = applyFilterRules([]string{"filter_1"}, rules, msg, ctx)
	if !got.Allowed {
		t.Fatalf("expected whitelist content rule to allow matched message: %s", got.Reason)
	}
}

func TestApplyFilterRules_WebhookSourceMatchesProject(t *testing.T) {
	msg := &Message{
		AccountID:   "proj_1",
		SourceEmail: "GitHub 项目",
		From:        "github",
		Subject:     "GitHub push",
		Body:        "main 分支已更新",
	}
	ctx := filterContext{
		SourceType:    "webhook",
		SourceName:    msg.SourceEmail,
		DisplaySender: "webhook转发",
	}
	rules := map[string]FilterRule{
		"filter_1": {
			ID:       "filter_1",
			Name:     "只允许 GitHub 项目",
			Type:     "source",
			Mode:     "whitelist",
			Patterns: []string{"github"},
			Enabled:  true,
		},
	}

	got := applyFilterRules([]string{"filter_1"}, rules, msg, ctx)
	if !got.Allowed {
		t.Fatalf("expected webhook source rule to allow matched project: %s", got.Reason)
	}
}

func TestAddSenderToDefaultFilterRuleNoLock_CreatesDefaultsAndDeduplicates(t *testing.T) {
	oldConfig := config
	t.Cleanup(func() {
		config = oldConfig
	})
	config = Config{}

	rule, added := addSenderToDefaultFilterRuleNoLock("blacklist", "spam@example.com")
	if !added {
		t.Fatalf("expected sender to be added")
	}
	if rule.ID != DefaultSenderBlacklistID {
		t.Fatalf("expected blacklist rule, got %q", rule.ID)
	}
	if len(rule.Patterns) != 1 || rule.Patterns[0] != "spam@example.com" {
		t.Fatalf("expected sender in blacklist patterns, got %#v", rule.Patterns)
	}

	rule, added = addSenderToDefaultFilterRuleNoLock("blacklist", "SPAM@example.com")
	if added {
		t.Fatalf("expected duplicate sender to be ignored")
	}
	if len(rule.Patterns) != 1 {
		t.Fatalf("expected duplicate sender to keep one pattern, got %#v", rule.Patterns)
	}

	if len(config.FilterRules) != 2 {
		t.Fatalf("expected both default sender rules to exist, got %d", len(config.FilterRules))
	}
}

func TestNormalizeForwardRule_MigratesSingleTargetAndDeduplicates(t *testing.T) {
	rule := normalizeForwardRule(ForwardRule{
		TargetWebhook:  "wh_1",
		TargetWebhooks: []string{"wh_1", "wh_2", "wh_2"},
	})

	if rule.TargetWebhook != "wh_1" {
		t.Fatalf("expected first target to stay compatible, got %q", rule.TargetWebhook)
	}
	if strings.Join(rule.TargetWebhooks, ",") != "wh_1,wh_2" {
		t.Fatalf("expected deduplicated target list, got %#v", rule.TargetWebhooks)
	}
}

func TestParseInboundPayload_GitHubPush(t *testing.T) {
	req, err := http.NewRequest("POST", "/hook/test", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	body := []byte(`{"ref":"refs/heads/main","repository":{"full_name":"owner/repo"},"head_commit":{"message":"update"}}`)

	got := parseInboundPayload(req, body)

	if !strings.Contains(got.Subject, "GitHub push owner/repo refs/heads/main") {
		t.Fatalf("expected GitHub push subject, got %q", got.Subject)
	}
	if got.From != "github" {
		t.Fatalf("expected github sender, got %q", got.From)
	}
	if !strings.Contains(got.Body, "owner/repo") {
		t.Fatalf("expected original JSON body content, got %q", got.Body)
	}
}

func TestParseInboundPayload_GitHubFormPayload(t *testing.T) {
	req, err := http.NewRequest("POST", "/hook/test", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-GitHub-Event", "ping")
	body := []byte(`payload=%7B%22zen%22%3A%22Responsive+is+better+than+fast.%22%2C%22repository%22%3A%7B%22full_name%22%3A%22trah01%2Fall2webhook%22%7D%2C%22sender%22%3A%7B%22login%22%3A%22trah01%22%7D%7D`)

	got := parseInboundPayload(req, body)

	if got.Subject != "GitHub ping trah01/all2webhook" {
		t.Fatalf("expected GitHub ping subject, got %q", got.Subject)
	}
	if got.From != "github" {
		t.Fatalf("expected github sender, got %q", got.From)
	}
	if strings.Contains(got.Body, "payload=%7B") {
		t.Fatalf("expected decoded GitHub body, got %q", got.Body)
	}
	if !strings.Contains(got.Body, "仓库：trah01/all2webhook") {
		t.Fatalf("expected readable repository body, got %q", got.Body)
	}
}

func TestParseInboundPayload_UnknownBodyIsPreserved(t *testing.T) {
	req, err := http.NewRequest("POST", "/hook/test", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")

	got := parseInboundPayload(req, []byte("raw notice"))

	if got.Subject != "外部通知" {
		t.Fatalf("expected fallback subject, got %q", got.Subject)
	}
	if !strings.Contains(got.Body, "raw notice") {
		t.Fatalf("expected raw body preserved, got %q", got.Body)
	}
}

func TestParseInboundPayload_DiscordEmbeds(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{
		"username": "deploy-bot",
		"content": "部署通知",
		"embeds": [{
			"title": "生产发布完成",
			"description": "版本 v1.2.3 已上线",
			"fields": [
				{"name": "服务", "value": "api"},
				{"name": "耗时", "value": "42s"}
			],
			"url": "https://example.com/release"
		}]
	}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "部署通知" {
		t.Fatalf("expected content first line as subject, got %q", got.Subject)
	}
	if got.From != "deploy-bot" {
		t.Fatalf("expected discord username as sender, got %q", got.From)
	}
	for _, want := range []string{"生产发布完成", "版本 v1.2.3 已上线", "服务", "api", "https://example.com/release"} {
		if !strings.Contains(got.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, got.Body)
		}
	}
}

func TestParseInboundPayload_FeishuText(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{"msg_type":"text","content":{"text":"飞书文本通知\n第二行"}}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "飞书文本通知" {
		t.Fatalf("expected Feishu text first line as subject, got %q", got.Subject)
	}
	if got.From != "feishu" {
		t.Fatalf("expected feishu sender, got %q", got.From)
	}
	if !strings.Contains(got.Body, "第二行") {
		t.Fatalf("expected Feishu text body, got %q", got.Body)
	}
}

func TestParseInboundPayload_FeishuPost(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{
		"msg_type": "post",
		"content": {
			"post": {
				"zh_cn": {
					"title": "项目更新",
					"content": [
						[{"tag":"text","text":"构建完成"}],
						[{"tag":"a","text":"查看详情","href":"https://example.com"}]
					]
				}
			}
		}
	}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "项目更新" {
		t.Fatalf("expected Feishu post title, got %q", got.Subject)
	}
	if !strings.Contains(got.Body, "构建完成") || !strings.Contains(got.Body, "查看详情") {
		t.Fatalf("expected Feishu post content, got %q", got.Body)
	}
}

func TestParseInboundPayload_FeishuInteractive(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{
		"msg_type":"interactive",
		"card":{
			"header":{"title":{"tag":"plain_text","content":"卡片告警"}},
			"elements":[
				{"tag":"div","text":{"tag":"lark_md","content":"**服务：** api"}},
				{"tag":"note","elements":[{"tag":"plain_text","content":"请关注"}]}
			]
		}
	}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "卡片告警" {
		t.Fatalf("expected Feishu card title, got %q", got.Subject)
	}
	if !strings.Contains(got.Body, "服务") || !strings.Contains(got.Body, "请关注") {
		t.Fatalf("expected Feishu card content, got %q", got.Body)
	}
}

func TestParseInboundPayload_DingTalkMarkdown(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{"msgtype":"markdown","markdown":{"title":"钉钉告警","text":"### 钉钉告警\nCPU 过高"}}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "钉钉告警" {
		t.Fatalf("expected DingTalk markdown title, got %q", got.Subject)
	}
	if got.From != "dingtalk" {
		t.Fatalf("expected dingtalk sender, got %q", got.From)
	}
	if !strings.Contains(got.Body, "CPU 过高") {
		t.Fatalf("expected DingTalk markdown text, got %q", got.Body)
	}
}

func TestParseInboundPayload_WeComMarkdown(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{"msgtype":"markdown","markdown":{"content":"# 企业微信告警\n磁盘不足"}}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "企业微信告警" {
		t.Fatalf("expected WeCom markdown heading, got %q", got.Subject)
	}
	if got.From != "wecom" {
		t.Fatalf("expected wecom sender, got %q", got.From)
	}
	if !strings.Contains(got.Body, "磁盘不足") {
		t.Fatalf("expected WeCom markdown content, got %q", got.Body)
	}
}

func TestParseInboundPayload_FormEncoded(t *testing.T) {
	req, err := http.NewRequest("POST", "/hook/test", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got := parseInboundPayload(req, []byte("title=ServerChan&desp=%E5%8F%91%E5%B8%83%E5%AE%8C%E6%88%90"))

	if got.Subject != "ServerChan" {
		t.Fatalf("expected form title, got %q", got.Subject)
	}
	if got.Body != "发布完成" {
		t.Fatalf("expected form body decoded, got %q", got.Body)
	}
}

func TestParseInboundPayload_BarkGETQuery(t *testing.T) {
	req, err := http.NewRequest("GET", "/hook/test?title=%E5%91%8A%E8%AD%A6&body=CPU%E8%BF%87%E9%AB%98&group=ops&url=https%3A%2F%2Fexample.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	got := parseInboundPayload(req, nil)

	if got.Subject != "告警" {
		t.Fatalf("expected Bark query title as subject, got %q", got.Subject)
	}
	if got.From != "bark" {
		t.Fatalf("expected bark sender, got %q", got.From)
	}
	for _, want := range []string{"CPU过高", "group：ops", "url：https://example.com"} {
		if !strings.Contains(got.Body, want) {
			t.Fatalf("expected body to contain %q, got %q", want, got.Body)
		}
	}
}

func TestParseInboundPayload_BarkGETPath(t *testing.T) {
	req, err := http.NewRequest("GET", "/hook/test/%E9%83%A8%E7%BD%B2/%E5%8F%91%E5%B8%83%E5%AE%8C%E6%88%90?subtitle=prod", nil)
	if err != nil {
		t.Fatal(err)
	}

	got := parseInboundPayload(req, nil)

	if got.Subject != "部署" {
		t.Fatalf("expected Bark path title as subject, got %q", got.Subject)
	}
	if !strings.Contains(got.Body, "发布完成") || !strings.Contains(got.Body, "subtitle：prod") {
		t.Fatalf("expected Bark path body and query options, got %q", got.Body)
	}
}

func TestParseInboundPayload_GenericNestedFields(t *testing.T) {
	req := newJSONRequest(t)
	body := []byte(`{
		"event": {
			"title": "通用告警",
			"payload": {
				"message": "节点不可用",
				"service": "worker",
				"level": "critical"
			}
		}
	}`)

	got := parseInboundPayload(req, body)

	if got.Subject != "通用告警" {
		t.Fatalf("expected nested title, got %q", got.Subject)
	}
	if !strings.Contains(got.Body, "节点不可用") || !strings.Contains(got.Body, "worker") {
		t.Fatalf("expected nested fields in body, got %q", got.Body)
	}
}

func newJSONRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest("POST", "/hook/test", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}
