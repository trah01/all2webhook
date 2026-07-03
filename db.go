package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// ===================== 数据库操作 =====================

func initDB() {
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal("Failed to create user data directory:", err)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required; only PostgreSQL is supported")
	}

	var err error
	db, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	if err := initPostgresSchema(); err != nil {
		log.Fatal("Failed to create tables:", err)
	}
}

func initPostgresSchema() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			source_email TEXT,
			account_id TEXT,
			subject TEXT,
			from_addr TEXT,
			to_addr TEXT,
			date TIMESTAMPTZ,
			body TEXT,
			body_html TEXT,
			status TEXT DEFAULT 'pending',
			target_type TEXT,
			target_name TEXT,
			retry_count INTEGER DEFAULT 0,
			error_message TEXT,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			sent_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_status ON messages(status);
		CREATE INDEX IF NOT EXISTS idx_created ON messages(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_account ON messages(account_id);
		CREATE TABLE IF NOT EXISTS inbound_projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			secret TEXT NOT NULL UNIQUE,
			enabled BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
			rotated_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_inbound_projects_secret ON inbound_projects(secret);
	`)
	return err
}

func dbExec(query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(rebindQuery(query), args...)
}

func dbQuery(query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(rebindQuery(query), args...)
}

func dbQueryRow(query string, args ...interface{}) *sql.Row {
	return db.QueryRow(rebindQuery(query), args...)
}

func rebindQuery(query string) string {
	var b strings.Builder
	argIndex := 1
	for _, r := range query {
		if r == '?' {
			b.WriteString(fmt.Sprintf("$%d", argIndex))
			argIndex++
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func saveMessage(msg *Message) error {
	_, err := dbExec(`
		INSERT INTO messages
		(id, source_email, account_id, subject, from_addr, to_addr, date, body, body_html,
		 status, target_type, target_name, retry_count, error_message, created_at, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			source_email = EXCLUDED.source_email,
			account_id = EXCLUDED.account_id,
			subject = EXCLUDED.subject,
			from_addr = EXCLUDED.from_addr,
			to_addr = EXCLUDED.to_addr,
			date = EXCLUDED.date,
			body = EXCLUDED.body,
			body_html = EXCLUDED.body_html,
			status = EXCLUDED.status,
			target_type = EXCLUDED.target_type,
			target_name = EXCLUDED.target_name,
			retry_count = EXCLUDED.retry_count,
			error_message = EXCLUDED.error_message,
			created_at = EXCLUDED.created_at,
			sent_at = EXCLUDED.sent_at
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

	rows, err := dbQuery(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var nsSubject, nsFrom, nsTo, nsBody, nsTargetType, nsTargetName, nsErrorMsg, nsSourceEmail sql.NullString
		var niRetryCount sql.NullInt64
		var nsDate, nsCreatedAt, sentAtValue interface{}

		err := rows.Scan(&msg.ID, &nsSourceEmail, &msg.AccountID, &nsSubject, &nsFrom,
			&nsTo, &nsDate, &nsBody, &msg.Status, &nsTargetType, &nsTargetName,
			&niRetryCount, &nsErrorMsg, &nsCreatedAt, &sentAtValue)
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

		msg.Date = scanTimeValue(nsDate)
		msg.CreatedAt = scanTimeValue(nsCreatedAt)
		if t := scanTimeValue(sentAtValue); !t.IsZero() {
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

func scanTimeValue(value interface{}) time.Time {
	switch v := value.(type) {
	case nil:
		return time.Time{}
	case time.Time:
		return v
	case []byte:
		return tryParseTime(string(v))
	case string:
		return tryParseTime(v)
	default:
		return tryParseTime(fmt.Sprint(v))
	}
}

func getMessageStats() (map[string]int, error) {
	stats := make(map[string]int)
	rows, err := dbQuery(`SELECT status, COUNT(*) FROM messages GROUP BY status`)
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
	_, err := dbExec(`DELETE FROM messages WHERE created_at < NOW() - (? || ' days')::interval`, days)
	return err
}

func deleteMessages(status string) (int64, error) {
	var (
		result sql.Result
		err    error
	)
	if strings.TrimSpace(status) != "" {
		result, err = dbExec(`DELETE FROM messages WHERE status = ?`, status)
	} else {
		result, err = dbExec(`DELETE FROM messages`)
	}
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return deleted, nil
}

func listInboundProjects(publicBaseURL string) ([]InboundProject, error) {
	rows, err := dbQuery(`SELECT id, name, secret, enabled, created_at, rotated_at FROM inbound_projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []InboundProject{}
	for rows.Next() {
		var project InboundProject
		var createdAt, rotatedAt interface{}
		if err := rows.Scan(&project.ID, &project.Name, &project.Secret, &project.Enabled, &createdAt, &rotatedAt); err != nil {
			return nil, err
		}
		project.CreatedAt = scanTimeValue(createdAt)
		if t := scanTimeValue(rotatedAt); !t.IsZero() {
			project.RotatedAt = &t
		}
		project.URL = buildInboundURL(publicBaseURL, project.Secret)
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func getInboundProjectBySecret(secret string) (*InboundProject, error) {
	var project InboundProject
	var createdAt, rotatedAt interface{}
	err := dbQueryRow(
		`SELECT id, name, secret, enabled, created_at, rotated_at FROM inbound_projects WHERE secret = ?`,
		secret,
	).Scan(&project.ID, &project.Name, &project.Secret, &project.Enabled, &createdAt, &rotatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	project.CreatedAt = scanTimeValue(createdAt)
	if t := scanTimeValue(rotatedAt); !t.IsZero() {
		project.RotatedAt = &t
	}
	return &project, nil
}

func createInboundProject(name string, enabled bool) (*InboundProject, error) {
	project := &InboundProject{
		ID:        fmt.Sprintf("project_%d", time.Now().UnixNano()),
		Name:      strings.TrimSpace(name),
		Secret:    generateSecret(),
		Enabled:   enabled,
		CreatedAt: time.Now(),
	}
	if project.Name == "" {
		project.Name = "未命名项目"
	}
	_, err := dbExec(
		`INSERT INTO inbound_projects (id, name, secret, enabled, created_at) VALUES (?, ?, ?, ?, ?)`,
		project.ID, project.Name, project.Secret, project.Enabled, project.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func updateInboundProject(id string, name string, enabled bool) error {
	_, err := dbExec(`UPDATE inbound_projects SET name = ?, enabled = ? WHERE id = ?`, strings.TrimSpace(name), enabled, id)
	return err
}

func rotateInboundProjectSecret(id string) (string, error) {
	secret := generateSecret()
	_, err := dbExec(`UPDATE inbound_projects SET secret = ?, rotated_at = ? WHERE id = ?`, secret, time.Now(), id)
	return secret, err
}

func deleteInboundProject(id string) error {
	_, err := dbExec(`DELETE FROM inbound_projects WHERE id = ?`, id)
	return err
}

func generateSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func buildInboundURL(publicBaseURL string, secret string) string {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if base == "" {
		return "/hook/" + secret
	}
	return base + "/hook/" + secret
}
