package main

import (
	"fmt"
	"strings"
)

const (
	DefaultSenderBlacklistID = "default_sender_blacklist"
	DefaultSenderWhitelistID = "default_sender_whitelist"

	filterTypeSender  = "sender"
	filterTypeContent = "content"
	filterTypeSource  = "source"
	filterTypeAll     = "all"
)

// FilterRule 是独立的邮件过滤规则，可被多个转发规则复用。
type FilterRule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"` // "sender", "content", "source", "all"
	Mode     string   `json:"mode"` // "whitelist", "blacklist"
	Patterns []string `json:"patterns"`
	Enabled  bool     `json:"enabled"`
}

type filterContext struct {
	SourceType    string
	SourceName    string
	DisplaySender string
}

type filterResult struct {
	Allowed bool
	Reason  string
}

func normalizeFilterRule(rule FilterRule) FilterRule {
	rule.Type = strings.ToLower(strings.TrimSpace(rule.Type))
	rule.Mode = strings.ToLower(strings.TrimSpace(rule.Mode))
	switch rule.Type {
	case filterTypeContent, filterTypeSource, filterTypeAll:
	default:
		rule.Type = filterTypeSender
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

func ensureDefaultSenderFilterRulesNoLock() {
	ensureDefaultFilterRuleNoLock(FilterRule{
		ID:       DefaultSenderBlacklistID,
		Name:     "默认发送人黑名单",
		Type:     filterTypeSender,
		Mode:     "blacklist",
		Patterns: []string{},
		Enabled:  true,
	})
	ensureDefaultFilterRuleNoLock(FilterRule{
		ID:       DefaultSenderWhitelistID,
		Name:     "默认发送人白名单",
		Type:     filterTypeSender,
		Mode:     "whitelist",
		Patterns: []string{},
		Enabled:  true,
	})
}

func ensureDefaultFilterRuleNoLock(rule FilterRule) {
	rule = normalizeFilterRule(rule)
	for i, existing := range config.FilterRules {
		if existing.ID == rule.ID {
			existing.Name = rule.Name
			existing.Type = rule.Type
			existing.Mode = rule.Mode
			config.FilterRules[i] = normalizeFilterRule(existing)
			return
		}
	}
	config.FilterRules = append(config.FilterRules, rule)
}

func addSenderToDefaultFilterRuleNoLock(mode string, sender string) (FilterRule, bool) {
	sender = strings.TrimSpace(sender)
	if sender == "" {
		return FilterRule{}, false
	}

	ruleID := DefaultSenderBlacklistID
	if mode == "whitelist" {
		ruleID = DefaultSenderWhitelistID
	}
	ensureDefaultSenderFilterRulesNoLock()

	for i, rule := range config.FilterRules {
		if rule.ID != ruleID {
			continue
		}
		rule = normalizeFilterRule(rule)
		for _, pattern := range rule.Patterns {
			if strings.EqualFold(pattern, sender) {
				config.FilterRules[i] = rule
				return rule, false
			}
		}
		rule.Patterns = append(rule.Patterns, sender)
		config.FilterRules[i] = normalizeFilterRule(rule)
		return config.FilterRules[i], true
	}
	return FilterRule{}, false
}

func applyFilterRules(ids []string, rules map[string]FilterRule, msg *Message, ctx filterContext) filterResult {
	for _, id := range ids {
		rule, ok := rules[id]
		if !ok || !rule.Enabled {
			continue
		}
		rule = normalizeFilterRule(rule)
		if len(rule.Patterns) == 0 {
			continue
		}
		if !filterRuleAllows(rule, msg, ctx) {
			return filterResult{
				Allowed: false,
				Reason:  fmt.Sprintf("%s(%s)", displayFilterRuleName(rule), displayFilterMode(rule.Mode)),
			}
		}
	}
	return filterResult{Allowed: true}
}

func filterRuleAllows(rule FilterRule, msg *Message, ctx filterContext) bool {
	text := filterMatchText(rule, msg, ctx)
	matched := containsAnyFold(text, rule.Patterns)
	switch rule.Mode {
	case "blacklist":
		return !matched
	default:
		return matched
	}
}

func filterMatchText(rule FilterRule, msg *Message, ctx filterContext) string {
	switch rule.Type {
	case filterTypeContent:
		return msg.Subject + "\n" + msg.Body
	case filterTypeSource:
		return strings.Join([]string{
			ctx.SourceType,
			ctx.SourceName,
			msg.SourceEmail,
			msg.AccountID,
		}, "\n")
	case filterTypeAll:
		return strings.Join([]string{
			ctx.DisplaySender,
			msg.From,
			ctx.SourceType,
			ctx.SourceName,
			msg.SourceEmail,
			msg.AccountID,
			msg.Subject,
			msg.Body,
		}, "\n")
	default:
		return strings.Join([]string{
			ctx.DisplaySender,
			msg.From,
		}, "\n")
	}
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
