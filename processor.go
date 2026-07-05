package main

import (
	"fmt"
	"strings"
	"time"
)

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

		// 匹配规则，找到第一条匹配的规则进行发送
		ruleMatched := false
		filterBlocked := false
		for _, rule := range rules {
			rule = normalizeForwardRule(rule)
			// 检查源账号匹配
			if !ruleMatchesSource(rule, msg.AccountID) {
				continue
			}

			filterCtx := buildFilterContext(&msg, accounts)
			filterResult := applyFilterRules(rule.FilterRuleIDs, filterRules, &msg, filterCtx)
			if !filterResult.Allowed {
				filterBlocked = true
				addLog(fmt.Sprintf("消息被过滤规则拦截 [%s]: %s", displaySubject(msg.Subject), filterResult.Reason), "info")
				continue
			}

			targetIDs := rule.TargetWebhooks
			if len(targetIDs) == 0 {
				continue
			}

			ruleMatched = true

			// 发送
			var sendErr error
			dateStr := displayMessageDate(msg.Date)
			subjectForSend := displaySubject(msg.Subject)
			senderForSend := filterCtx.DisplaySender

			// 智能提取验证码并高亮前置
			displayBody := msg.Body
			var verificationCode string
			if matches := codeRegex.FindStringSubmatch(msg.Subject); len(matches) > 1 {
				verificationCode = matches[1]
			} else if matches := codeRegex.FindStringSubmatch(msg.Body); len(matches) > 1 {
				verificationCode = matches[1]
			}

			if verificationCode != "" {
				displayBody = fmt.Sprintf("**[智能提取验证码] %s**\n\n%s", verificationCode, msg.Body)
			}

			sentTargets := make([]string, 0, len(targetIDs))
			failedTargets := make([]string, 0)
			for _, targetID := range targetIDs {
				webhook, ok := webhooks[targetID]
				if !ok || !webhook.Enabled {
					continue
				}

				sendErr = sendToWebhookTarget(webhook, accounts, subjectForSend, senderForSend, dateStr, displayBody)

				if sendErr != nil {
					failedTargets = append(failedTargets, fmt.Sprintf("%s: %v", webhook.Name, sendErr))
					addLog(fmt.Sprintf("发送失败 [%s -> %s]: %v", subjectForSend, webhook.Name, sendErr), "error")
					continue
				}
				sentTargets = append(sentTargets, webhook.Name)
				addLog(fmt.Sprintf("转发成功 [%s -> %s]", subjectForSend, webhook.Name), "success")
			}

			if len(sentTargets) == 0 {
				msg.RetryCount++
				msg.ErrorMessage = strings.Join(failedTargets, "; ")
				if msg.ErrorMessage == "" {
					msg.ErrorMessage = "没有可用的目标 Webhook"
				}
				saveMessage(&msg)
			} else {
				msg.Status = "sent"
				msg.TargetType = "multi"
				msg.TargetName = strings.Join(sentTargets, ", ")
				if len(failedTargets) > 0 {
					msg.ErrorMessage = "部分目标失败: " + strings.Join(failedTargets, "; ")
				}
				now := time.Now()
				msg.SentAt = &now
				saveMessage(&msg)
			}
			break // 匹配到第一条规则并尝试发送后，不再继续匹配后续规则
		}

		// 没有匹配到任何转发规则的消息，标记为 no_rule 防止永远卡在 pending 堵塞队列
		if !ruleMatched {
			if filterBlocked {
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
