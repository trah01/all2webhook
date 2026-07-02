package main

import (
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
	rules := map[string]FilterRule{
		"filter_1": {
			ID:       "filter_1",
			Name:     "屏蔽发信人",
			Type:     "sender",
			Mode:     "blacklist",
			Patterns: []string{"example.com"},
			Enabled:  true,
		},
	}

	got := applyFilterRules([]string{"filter_1"}, rules, msg)

	if got.Allowed {
		t.Fatalf("expected blacklist sender rule to block message")
	}
}

func TestApplyFilterRules_WhitelistRequiresContentMatch(t *testing.T) {
	msg := &Message{From: "notice@example.com", Subject: "普通通知", Body: "没有关键词"}
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

	got := applyFilterRules([]string{"filter_1"}, rules, msg)

	if got.Allowed {
		t.Fatalf("expected whitelist content rule to block unmatched message")
	}

	msg.Subject = "本月账单"
	got = applyFilterRules([]string{"filter_1"}, rules, msg)
	if !got.Allowed {
		t.Fatalf("expected whitelist content rule to allow matched message: %s", got.Reason)
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
