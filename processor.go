package main

import (
	"fmt"
	"strings"
	"time"
)

type plannedForwardTarget struct {
	ID           string
	IncludeLinks bool
}

func processPendingMessages() {
	// 防止消息发送队列重叠并发引发重复发送通知
	if !processingMutex.TryLock() {
		return
	}
	defer processingMutex.Unlock()

	// 获取所有启用的规则
	configLock.RLock()
	rules := make([]ForwardRule, 0)
	filterRules := make(map[string]FilterRule)
	webhooks := make(map[string]WebhookTarget)
	accounts := make(map[string]EmailAccount)

	for _, w := range config.Webhooks {
		webhooks[w.ID] = w
	}
	for _, a := range config.Accounts {
		accounts[a.ID] = a
	}
	for _, f := range config.FilterRules {
		filterRules[f.ID] = f
	}
	for _, r := range config.Rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	maxRetries := config.MaxRetries
	configLock.RUnlock()

	// 获取待发送消息
	messages, err := getMessages("pending", 100, 0)
	if err != nil {
		addLog(fmt.Sprintf("获取待发送消息失败: %v", err), "error")
		return
	}

	for _, msg := range messages {
		if msg.RetryCount >= maxRetries {
			msg.Status = "failed"
			msg.ErrorMessage = "超过最大重试次数"
			saveMessage(&msg)
			continue
		}

		// 汇总所有匹配规则的目标。同一目标被多条规则命中时只发送一次。
		filterCtx := buildFilterContext(&msg, accounts)
		targets, sourceMatched, filterBlocked := planForwardTargets(rules, filterRules, &msg, filterCtx)
		ruleMatched := len(targets) > 0
		for _, reason := range filterBlocked {
			addLog(fmt.Sprintf("消息被过滤规则拦截 [%s]: %s", displaySubject(msg.Subject), reason), "info")
		}

		if ruleMatched {
			dateStr := displayMessageDate(msg.Date)
			subjectForSend := displaySubject(msg.Subject)
			senderForSend := filterCtx.DisplaySender
			verificationCode := extractVerificationCode(&msg)
			sentTargets := make([]string, 0, len(targets))
			failedTargets := make([]string, 0)
			historyTargets := make([]string, 0, len(targets))

			for _, planned := range targets {
				webhook, ok := webhooks[planned.ID]
				if !ok {
					failedTargets = append(failedTargets, fmt.Sprintf("%s: 目标渠道不存在", planned.ID))
					historyTargets = append(historyTargets, planned.ID+"（失败）")
					continue
				}
				if !webhook.Enabled {
					failedTargets = append(failedTargets, fmt.Sprintf("%s: 目标渠道已禁用", webhook.Name))
					historyTargets = append(historyTargets, webhook.Name+"（失败）")
					continue
				}

				displayBody := formatForwardBody(msg.Body, planned.IncludeLinks)
				if verificationCode != "" {
					displayBody = fmt.Sprintf("**[智能提取验证码] %s**\n\n%s", verificationCode, displayBody)
				}

				if err := sendToWebhookTarget(webhook, accounts, subjectForSend, senderForSend, dateStr, displayBody); err != nil {
					failedTargets = append(failedTargets, fmt.Sprintf("%s: %v", webhook.Name, err))
					historyTargets = append(historyTargets, webhook.Name+"（失败）")
					addLog(fmt.Sprintf("发送失败 [%s -> %s]: %v", subjectForSend, webhook.Name, err), "error")
					continue
				}
				sentTargets = append(sentTargets, webhook.Name)
				historyTargets = append(historyTargets, webhook.Name)
				addLog(fmt.Sprintf("转发成功 [%s -> %s]", subjectForSend, webhook.Name), "success")
			}

			msg.TargetType = "multi"
			msg.TargetName = strings.Join(historyTargets, ", ")
			if len(sentTargets) == 0 {
				msg.RetryCount++
				msg.ErrorMessage = strings.Join(failedTargets, "; ")
				saveMessage(&msg)
			} else {
				msg.Status = "sent"
				msg.ErrorMessage = ""
				if len(failedTargets) > 0 {
					msg.ErrorMessage = "部分目标失败: " + strings.Join(failedTargets, "; ")
				}
				now := time.Now()
				msg.SentAt = &now
				saveMessage(&msg)
			}
		}

		// 没有匹配到任何转发规则的消息，标记状态防止永远卡在 pending 堵塞队列。
		if !ruleMatched {
			if sourceMatched && len(filterBlocked) > 0 {
				msg.Status = "filtered"
				msg.ErrorMessage = "已被过滤规则拦截"
				saveMessage(&msg)
				addLog(fmt.Sprintf("消息已过滤 [%s] account=%s", displaySubject(msg.Subject), msg.AccountID), "info")
			} else {
				msg.Status = "no_rule"
				msg.ErrorMessage = fmt.Sprintf("没有匹配的转发规则 (account_id=%s)", msg.AccountID)
				saveMessage(&msg)
				addLog(fmt.Sprintf("消息无匹配规则 [%s] account=%s", displaySubject(msg.Subject), msg.AccountID), "warning")
			}
		}
	}
}

func planForwardTargets(rules []ForwardRule, filterRules map[string]FilterRule, msg *Message, filterCtx filterContext) ([]plannedForwardTarget, bool, []string) {
	targets := make([]plannedForwardTarget, 0)
	targetIndexes := make(map[string]int)
	sourceMatched := false
	blockedReasons := make([]string, 0)

	for _, rule := range rules {
		rule = normalizeForwardRule(rule)
		if !ruleMatchesSource(rule, msg.AccountID) {
			continue
		}
		sourceMatched = true

		filterResult := applyFilterRules(rule.FilterRuleIDs, filterRules, msg, filterCtx)
		if !filterResult.Allowed {
			blockedReasons = append(blockedReasons, filterResult.Reason)
			continue
		}

		for _, targetID := range rule.TargetWebhooks {
			if index, exists := targetIndexes[targetID]; exists {
				// 任一匹配规则要求保留链接时，为该目标保留链接。
				targets[index].IncludeLinks = targets[index].IncludeLinks || rule.IncludeLinks
				continue
			}
			targetIndexes[targetID] = len(targets)
			targets = append(targets, plannedForwardTarget{ID: targetID, IncludeLinks: rule.IncludeLinks})
		}
	}

	return targets, sourceMatched, blockedReasons
}

func extractVerificationCode(msg *Message) string {
	if matches := codeRegex.FindStringSubmatch(msg.Subject); len(matches) > 1 {
		return matches[1]
	}
	if matches := codeRegex.FindStringSubmatch(msg.Body); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func ruleMatchesSource(rule ForwardRule, accountID string) bool {
	for _, sourceID := range rule.SourceAccounts {
		if sourceID == "all" || sourceID == accountID {
			return true
		}
	}
	return false
}

func buildFilterContext(msg *Message, accounts map[string]EmailAccount) filterContext {
	if account, ok := accounts[msg.AccountID]; ok {
		return filterContext{
			SourceType:    "mail",
			SourceName:    firstNonEmpty(account.Name, account.EmailUser, msg.SourceEmail, msg.AccountID),
			DisplaySender: firstNonEmpty(msg.From, account.EmailUser, msg.SourceEmail),
		}
	}
	return filterContext{
		SourceType:    "webhook",
		SourceName:    firstNonEmpty(msg.SourceEmail, msg.AccountID),
		DisplaySender: "webhook转发",
	}
}

// ===================== 后台任务 =====================

func startBackgroundTasks() {
	// 消息处理循环
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			processPendingMessages()
		}
	}()

	// 清理旧消息
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			deleteOldMessages(30) // 保留30天
		}
	}()

	// 为保护隐私，定期清理已处理完毕消息的邮件正文内容 (1分钟清理一次)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			dbExec(`UPDATE messages SET body = '', body_html = '' WHERE status IN ('sent', 'failed') AND body != ''`)
		}
	}()

	// 邮箱检查调度
	go func() {
		accountLastRun := make(map[string]time.Time)
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			configLock.RLock()
			accounts := config.Accounts
			configLock.RUnlock()

			now := time.Now()
			for i := range accounts {
				acc := accounts[i]
				acc = normalizeEmailAccount(acc)
				if !acc.Enabled || acc.Type != "imap" {
					continue
				}

				interval := acc.CheckInterval
				if interval < 15 {
					// 若账号未配置有效间隔，回退使用全局默认
					configLock.RLock()
					interval = config.DefaultInterval
					configLock.RUnlock()
					if interval < 15 {
						interval = 60
					}
				}

				lastRun, exists := accountLastRun[acc.ID]
				if !exists || now.Sub(lastRun) >= time.Duration(interval)*time.Second {
					accountLastRun[acc.ID] = now
					// 启动独立协程去收件，不阻塞其他账号的调度
					go checkMailForAccount(&acc)
				}
			}
		}
	}()
}
