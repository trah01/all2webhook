package main

import (
	"database/sql"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// ===================== 全局变量 =====================

var (
	config           Config
	configLock       sync.RWMutex
	db               *sql.DB
	logs             []LogEntry
	logsMutex        sync.Mutex
	urlRegex         = regexp.MustCompile(`https?://[a-zA-Z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)
	codeRegex        = regexp.MustCompile(`(?i)(?:验证码|校验码|动态码|验证|确认码|verification code|security code|auth code|\bcode\b)[\s:：\-\[【]*([a-zA-Z0-9]{4,8})\b`)
	accountLastCheck sync.Map                                  // 记录每个账号最后的检查时间
	accountChecking  sync.Map                                  // 防止同一个账号的 IMAP 检查并发
	folderFirstSeen  sync.Map                                  // 记录每个账号-文件夹组合首次扫描的时间基线
	processingMutex  sync.Mutex                                // 防止并发处理待发送消息
	httpClient       = &http.Client{Timeout: 15 * time.Second} // 全局 Webhook 请求带超时的客户端
)

const (
	ConfigFile = "data/config.json"
	DBFile     = "data/messages.db"
)
