package main

import "time"

// ===================== 数据模型 =====================

// EmailAccount 邮箱账号配置
type EmailAccount struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"` // "imap" 用于收件，"smtp" 用于发送通知
	ImapServer    string   `json:"imap_server"`
	SmtpServer    string   `json:"smtp_server"`
	EmailUser     string   `json:"email_user"`
	EmailPass     string   `json:"email_pass"`
	Enabled       bool     `json:"enabled"`
	CheckInterval int      `json:"check_interval"` // 秒
	Folders       []string `json:"folders"`
}

// WebhookTarget Webhook 目标配置
type WebhookTarget struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"` // "feishu", "dingtalk", "wecom", "slack", "discord", "teams", "mattermost", "gotify", "bark", "serverchan", "telegram", "ntfy", "pushplus", "chanify", "pushover", "custom", "email"
	URL            string `json:"url"`
	Enabled        bool   `json:"enabled"`
	Template       string `json:"template"`         // 自定义 JSON payload 模板
	SmtpAccountID  string `json:"smtp_account_id"`  // 邮件类型：指定 SMTP 发信账号ID
	Secret         string `json:"secret"`           // 钉钉/飞书等机器人加签密钥
	PayloadType    string `json:"payload_type"`     // text, markdown, card, blocks, embeds 等
	Token          string `json:"token"`            // Gotify/Bark/Telegram 等可选 token
	ChatID         string `json:"chat_id"`          // Telegram chat_id
	Username       string `json:"username"`         // Discord/Slack 自定义发送名称
	IconURL        string `json:"icon_url"`         // Discord/Slack 头像
	LinkURL        string `json:"link_url"`         // 卡片/图文消息点击跳转 URL
	MentionAll     bool   `json:"mention_all"`      // 钉钉/企微 @所有人
	MentionMobiles string `json:"mention_mobiles"`  // 钉钉/企微手机号，逗号分隔
	MentionUserIDs string `json:"mention_user_ids"` // 企微 user_id，逗号分隔
	Priority       int    `json:"priority"`         // Gotify 优先级
	Headers        string `json:"headers"`          // 自定义 HTTP 请求头 JSON，例如 {"Authorization":"Bearer ..."}
	TLSCACert      string `json:"tls_ca_cert"`      // 自定义 CA 证书 PEM
	TLSClientCert  string `json:"tls_client_cert"`  // mTLS 客户端证书 PEM
	TLSClientKey   string `json:"tls_client_key"`   // mTLS 客户端私钥 PEM
	TLSSkipVerify  bool   `json:"tls_skip_verify"`  // 跳过 TLS 证书校验，仅用于内网测试
}

// ForwardRule 转发规则
type ForwardRule struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SourceAccount  string   `json:"source_account"`  // 邮箱账号ID、接收项目ID，"all" 表示所有
	SourceAccounts []string `json:"source_accounts"` // 来源ID列表，包含 "all" 表示所有来源
	TargetWebhook  string   `json:"target_webhook"`  // 兼容旧配置的单 Webhook 目标ID
	TargetWebhooks []string `json:"target_webhooks"` // Webhook 目标ID列表
	FilterRuleIDs  []string `json:"filter_rule_ids"` // 独立过滤规则ID
	Enabled        bool     `json:"enabled"`
}

// Message 存储的消息
type Message struct {
	ID           string     `json:"id"`
	SourceEmail  string     `json:"source_email"`
	AccountID    string     `json:"account_id"`
	Subject      string     `json:"subject"`
	From         string     `json:"from"`
	To           string     `json:"to"`
	Date         time.Time  `json:"date"`
	Body         string     `json:"body"`
	BodyHTML     string     `json:"body_html"`
	Status       string     `json:"status"` // "pending", "sent", "failed"
	TargetType   string     `json:"target_type"`
	TargetName   string     `json:"target_name"`
	RetryCount   int        `json:"retry_count"`
	ErrorMessage string     `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	SentAt       *time.Time `json:"sent_at"`
}

// LogEntry 日志条目
type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Type    string `json:"type"` // "info", "success", "error", "warning"
}

// InboundProject 公开接收入口配置。
type InboundProject struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Secret    string     `json:"secret,omitempty"`
	URL       string     `json:"url,omitempty"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
}

// Config 全局配置
type Config struct {
	Accounts        []EmailAccount  `json:"accounts"`
	Webhooks        []WebhookTarget `json:"webhooks"`
	Rules           []ForwardRule   `json:"rules"`
	FilterRules     []FilterRule    `json:"filter_rules"`
	DefaultInterval int             `json:"default_interval"`
	MaxRetries      int             `json:"max_retries"`
}
