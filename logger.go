package main

import (
	"log"
	"time"
)

// ===================== 日志管理 =====================

func addLog(msg string, logType string) {
	logsMutex.Lock()
	defer logsMutex.Unlock()

	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
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
