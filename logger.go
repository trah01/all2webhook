package main

import (
	"log"
	"strings"
	"time"
)

// ===================== 日志管理 =====================

const noisyPollingLogWindow = 10 * time.Minute

var recentPollingLogs = make(map[string]time.Time)

func addLog(msg string, logType string) {
	logsMutex.Lock()
	defer logsMutex.Unlock()

	now := time.Now()
	if shouldSuppressPollingLog(msg, logType, now) {
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
	prefix := map[string]string{
		"info":    "[INFO]",
		"success": "[OK]",
		"error":   "[ERROR]",
		"warning": "[WARN]",
	}[logType]
	log.Printf("%s %s", prefix, msg)
}

func shouldSuppressPollingLog(msg string, logType string, now time.Time) bool {
	if logType != "info" && logType != "warning" {
		return false
	}
	if !isNoisyPollingLog(msg) {
		return false
	}

	key := logType + "\x00" + msg
	if last, ok := recentPollingLogs[key]; ok && now.Sub(last) < noisyPollingLogWindow {
		return true
	}
	recentPollingLogs[key] = now
	return false
}

func isNoisyPollingLog(msg string) bool {
	return strings.HasPrefix(msg, "检查邮箱文件夹 ") ||
		strings.HasPrefix(msg, "邮箱文件夹 ") ||
		strings.HasPrefix(msg, "跳过历史邮件 ")
}
