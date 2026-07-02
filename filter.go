package main

import (
	"fmt"
	"strings"
)

// FilterRule 是独立的邮件过滤规则，可被多个转发规则复用。
type FilterRule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"` // "sender", "content"
	Mode     string   `json:"mode"` // "whitelist", "blacklist"
	Patterns []string `json:"patterns"`
	Enabled  bool     `json:"enabled"`
}

type filterResult struct {
	Allowed bool
	Reason  string
}

func normalizeFilterRule(rule FilterRule) FilterRule {
	rule.Type = strings.ToLower(strings.TrimSpace(rule.Type))
	rule.Mode = strings.ToLower(strings.TrimSpace(rule.Mode))
	if rule.Type != "content" {
		rule.Type = "sender"
	}
	if rule.Mode != "blacklist" {
		rule.Mode = "whitelist"
	}

	patterns := make([]string, 0, len(rule.Patterns))
	seen := make(map[string]bool)
	for _, pattern := range rule.Patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		key := strings.ToLower(pattern)
		if seen[key] {
			continue
		}
		seen[key] = true
		patterns = append(patterns, pattern)
	}
	rule.Patterns = patterns
	return rule
}

func applyFilterRules(ids []string, rules map[string]FilterRule, msg *Message) filterResult {
	for _, id := range ids {
		rule, ok := rules[id]
		if !ok || !rule.Enabled {
			continue
		}
		rule = normalizeFilterRule(rule)
		if len(rule.Patterns) == 0 {
			continue
		}
		if !filterRuleAllows(rule, msg) {
			return filterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("%s(%s)", displayFilterRuleName(rule), displayFilterMode(rule.Mode)),
			}
		}
	}
	return filterResult{Allowed: true}
}

func filterRuleAllows(rule FilterRule, msg *Message) bool {
	text := filterMatchText(rule, msg)
	matched := containsAnyFold(text, rule.Patterns)
	switch rule.Mode {
	case "blacklist":
		return !matched
	default:
		return matched
	}
}

func filterMatchText(rule FilterRule, msg *Message) string {
	if rule.Type == "content" {
		return msg.Subject + "\n" + msg.Body
	}
	return msg.From
}

func containsAnyFold(text string, patterns []string) bool {
	text = strings.ToLower(text)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" && strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func removeString(items []string, value string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

func displayFilterRuleName(rule FilterRule) string {
	if strings.TrimSpace(rule.Name) == "" {
		return "未命名过滤规则"
	}
	return rule.Name
}

func displayFilterMode(mode string) string {
	if mode == "blacklist" {
		return "黑名单"
	}
	return "白名单"
}
