package main

import (
	"encoding/json"
	"os"
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
	normalizeForwardRulesNoLock()
	saveConfigNoLock()
}

func normalizeForwardRulesNoLock() {
	for i := range config.Rules {
		config.Rules[i] = normalizeForwardRule(config.Rules[i])
	}
}

func normalizeForwardRule(rule ForwardRule) ForwardRule {
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
