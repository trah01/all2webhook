package main

import (
	"log"
	"strings"
	"time"
)

// ===================== 日志管理 =====================

const backendPollingLogWindow = 10 * time.Minute

var recentPollingLogs = make(map[string]time.Time)

func addLog(msg string, logType string) {
	logsMutex.Lock()
	defer logsMutex.Unlock()

	now := time.Now()
	if isFrontendHiddenPollingLog(msg, logType) {
		if !shouldSuppressBackendPollingLog(msg, logType, now) {
			writeConsoleLog(msg, logType)
		}
		return
	}

	entry := LogEntry{
		Time:    now.Format("15:04:05"),
		Message: msg,
		Type:    logType,
	}
	logs = append([]LogEntry{entry}, logs...)
	if len(logs) > 200 {
		logs = logs[:200]
	}

	// 同时输出到控制台
	writeConsoleLog(msg, logType)
}

func writeConsoleLog(msg string, logType string) {
	prefix := map[string]string{
		"info":    "[INFO]",
		"success": "[OK]",
		"error":   "[ERROR]",
		"warning": "[WARN]",
	}[logType]
	log.Printf("%s %s", prefix, msg)
}

func shouldSuppressBackendPollingLog(msg string, logType string, now time.Time) bool {
	key := logType + "\x00" + msg
	if last, ok := recentPollingLogs[key]; ok && now.Sub(last) < backendPollingLogWindow {
		return true
	}
	recentPollingLogs[key] = now
	return false
}

func isFrontendHiddenPollingLog(msg string, logType string) bool {
	if logType != "info" && logType != "warning" {
		return false
	}
	return strings.HasPrefix(msg, "检查邮箱文件夹 ") ||
		strings.HasPrefix(msg, "邮箱文件夹 ") ||
		strings.HasPrefix(msg, "跳过历史邮件 ")
}
