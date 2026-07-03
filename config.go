package main

import (
	"encoding/json"
	"os"
	"strings"
)

// ===================== 配置管理 =====================

func loadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	// 默认配置
	config = Config{
		DefaultInterval: 60,
		MaxRetries:      3,
	}

	data, err := os.ReadFile(ConfigFile)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		ensureDefaultSenderFilterRulesNoLock()
		saveConfigNoLock()
		return
	}
	ensureDefaultSenderFilterRulesNoLock()
	normalizeEmailAccountsNoLock()
	normalizeWebhooksNoLock()
	normalizeForwardRulesNoLock()
	saveConfigNoLock()
}

func normalizeEmailAccountsNoLock() {
	for i := range config.Accounts {
		config.Accounts[i] = normalizeEmailAccount(config.Accounts[i])
	}
}

func normalizeEmailAccount(account EmailAccount) EmailAccount {
	account.Type = strings.ToLower(strings.TrimSpace(account.Type))
	if account.Type != "smtp" {
		account.Type = "imap"
	}
	account.ImapServer = strings.TrimSpace(account.ImapServer)
	account.SmtpServer = strings.TrimSpace(account.SmtpServer)
	account.EmailUser = strings.TrimSpace(account.EmailUser)
	if account.Type == "imap" && account.Folders == nil {
		account.Folders = []string{"INBOX"}
	}
	return account
}

func normalizeWebhooksNoLock() {
	for i := range config.Webhooks {
		config.Webhooks[i] = normalizeWebhookTarget(config.Webhooks[i])
	}
}

func normalizeWebhookTarget(target WebhookTarget) WebhookTarget {
	target.Type = strings.ToLower(strings.TrimSpace(target.Type))
	target.PayloadType = strings.ToLower(strings.TrimSpace(target.PayloadType))
	target.Name = strings.TrimSpace(target.Name)
	target.URL = strings.TrimSpace(target.URL)
	target.Secret = strings.TrimSpace(target.Secret)
	target.Token = strings.TrimSpace(target.Token)
	target.ChatID = strings.TrimSpace(target.ChatID)
	target.Username = strings.TrimSpace(target.Username)
	target.IconURL = strings.TrimSpace(target.IconURL)
	target.LinkURL = strings.TrimSpace(target.LinkURL)
	target.MentionMobiles = strings.TrimSpace(target.MentionMobiles)
	target.MentionUserIDs = strings.TrimSpace(target.MentionUserIDs)
	target.Headers = strings.TrimSpace(target.Headers)
	target.TLSCACert = strings.TrimSpace(target.TLSCACert)
	target.TLSClientCert = strings.TrimSpace(target.TLSClientCert)
	target.TLSClientKey = strings.TrimSpace(target.TLSClientKey)
	if target.Type == "" {
		target.Type = "custom"
	}
	return target
}

func normalizeForwardRulesNoLock() {
	for i := range config.Rules {
		config.Rules[i] = normalizeForwardRule(config.Rules[i])
	}
}

func normalizeForwardRule(rule ForwardRule) ForwardRule {
	sources := make([]string, 0, len(rule.SourceAccounts)+1)
	seenSources := make(map[string]bool)
	if rule.SourceAccount != "" {
		sources = append(sources, rule.SourceAccount)
		seenSources[rule.SourceAccount] = true
	}
	for _, source := range rule.SourceAccounts {
		source = strings.TrimSpace(source)
		if source == "" || seenSources[source] {
			continue
		}
		sources = append(sources, source)
		seenSources[source] = true
	}
	if len(sources) == 0 {
		sources = append(sources, "all")
	}
	if seenSources["all"] {
		sources = []string{"all"}
	}
	rule.SourceAccount = sources[0]
	rule.SourceAccounts = sources

	targets := make([]string, 0, len(rule.TargetWebhooks)+1)
	seen := make(map[string]bool)
	if rule.TargetWebhook != "" {
		targets = append(targets, rule.TargetWebhook)
		seen[rule.TargetWebhook] = true
	}
	for _, id := range rule.TargetWebhooks {
		if id == "" || seen[id] {
			continue
		}
		targets = append(targets, id)
		seen[id] = true
	}
	rule.TargetWebhooks = targets
	if len(rule.TargetWebhooks) > 0 {
		rule.TargetWebhook = rule.TargetWebhooks[0]
	}
	return rule
}

func saveConfigNoLock() {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(ConfigFile, data, 0644)
}

func saveConfig() {
	configLock.Lock()
	defer configLock.Unlock()
	saveConfigNoLock()
}
