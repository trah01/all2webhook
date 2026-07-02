package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// ===================== 数据库操作 =====================

func initDB() {
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal("Failed to create user data directory:", err)
	}

	var err error
	db, err = sql.Open("sqlite", DBFile)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	// 开启 WAL 模式和设置 busy_timeout ，避免并发写入锁定和提升性能
	db.Exec("PRAGMA journal_mode=WAL;")
	db.Exec("PRAGMA busy_timeout=5000;")

	// 创建消息表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			source_email TEXT,
			account_id TEXT,
			subject TEXT,
			from_addr TEXT,
			to_addr TEXT,
			date DATETIME,
			body TEXT,
			body_html TEXT,
			status TEXT DEFAULT 'pending',
			target_type TEXT,
			target_name TEXT,
			retry_count INTEGER DEFAULT 0,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			sent_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_status ON messages(status);
		CREATE INDEX IF NOT EXISTS idx_created ON messages(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_account ON messages(account_id);
	`)
	if err != nil {
		log.Fatal("Failed to create tables:", err)
	}
}

func saveMessage(msg *Message) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO messages
		(id, source_email, account_id, subject, from_addr, to_addr, date, body, body_html,
		 status, target_type, target_name, retry_count, error_message, created_at, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.SourceEmail, msg.AccountID, msg.Subject, msg.From, msg.To, msg.Date,
		msg.Body, msg.BodyHTML, msg.Status, msg.TargetType, msg.TargetName,
		msg.RetryCount, msg.ErrorMessage, msg.CreatedAt, msg.SentAt)
	return err
}

func getMessages(status string, limit int, offset int) ([]Message, error) {
	query := `SELECT id, source_email, account_id, subject, from_addr, to_addr, date, body,
		status, target_type, target_name, retry_count, error_message, created_at, sent_at
		FROM messages`
	args := []interface{}{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var nsSubject, nsFrom, nsTo, nsBody, nsTargetType, nsTargetName, nsErrorMsg, nsSourceEmail sql.NullString
		var niRetryCount sql.NullInt64
		var dateStr, createdAtStr string
		var nsDate, nsCreatedAt, sentAtStr sql.NullString

		err := rows.Scan(&msg.ID, &nsSourceEmail, &msg.AccountID, &nsSubject, &nsFrom,
			&nsTo, &nsDate, &nsBody, &msg.Status, &nsTargetType, &nsTargetName,
			&niRetryCount, &nsErrorMsg, &nsCreatedAt, &sentAtStr)
		if err != nil {
			return nil, err
		}

		msg.SourceEmail = nsSourceEmail.String
		msg.Subject = nsSubject.String
		msg.From = nsFrom.String
		msg.To = nsTo.String
		msg.Body = nsBody.String
		msg.TargetType = nsTargetType.String
		msg.TargetName = nsTargetName.String
		msg.ErrorMessage = nsErrorMsg.String
		msg.RetryCount = int(niRetryCount.Int64)

		if nsDate.Valid {
			dateStr = nsDate.String
		}
		if nsCreatedAt.Valid {
			createdAtStr = nsCreatedAt.String
		}

		msg.Date = tryParseTime(dateStr)
		msg.CreatedAt = tryParseTime(createdAtStr)
		if sentAtStr.Valid && sentAtStr.String != "" {
			t := tryParseTime(sentAtStr.String)
			msg.SentAt = &t
		}

		messages = append(messages, msg)
	}
	return messages, nil
}

func tryParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// 尝试常见的 SQLite 时间格式
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func getMessageStats() (map[string]int, error) {
	stats := make(map[string]int)
	rows, err := db.Query(`SELECT status, COUNT(*) FROM messages GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}
	return stats, nil
}

func deleteOldMessages(days int) error {
	_, err := db.Exec(`DELETE FROM messages WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days))
	return err
}
