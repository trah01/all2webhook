package main

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// ===================== 邮件处理 =====================

var markdownLinkRegex = regexp.MustCompile(`\[([^\]\n]{0,240})\]\((https?://[^)\s]+|长链接由于超长已被过滤)\)`)

func cleanHTML(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return normalizeFormattedText(htmlStr)
	}
	doc.Find("script, style, head, title, meta, noscript").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	doc.Find("br").Each(func(i int, s *goquery.Selection) {
		s.ReplaceWithHtml("\n")
	})

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		href = sanitizeURL(href)
		if text == "" {
			text = "链接"
		} else {
			text = truncateRunes(text, maxForwardLinkTextRunes)
		}
		text = escapeMarkdownText(text)
		if href != "" {
			href = escapeMarkdownLinkURL(href)
			if len(href) > maxForwardURLLength {
				s.SetText(text)
			} else {
				s.SetText(fmt.Sprintf("[%s](%s)", text, href))
			}
		} else {
			s.SetText(text)
		}
	})

	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		s.PrependHtml("- ")
		s.AppendHtml("\n")
	})

	doc.Find("p,div,section,article,header,footer,blockquote,pre,h1,h2,h3,h4,h5,h6,tr,table").Each(func(i int, s *goquery.Selection) {
		s.AppendHtml("\n\n")
	})

	text := normalizeFormattedText(doc.Text())
	text = urlRegex.ReplaceAllStringFunc(text, func(u string) string {
		if len(u) > maxForwardURLLength {
			return ""
		}
		return u
	})
	return text
}

func sanitizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme != "" {
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" {
			return ""
		}
	}
	q := u.Query()
	for key := range q {
		k := strings.ToLower(key)
		if strings.HasPrefix(k, "utm_") || k == "fbclid" || k == "gclid" || k == "mc_cid" || k == "mc_eid" || k == "spm" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	if u.Path != "" {
		u.Path = u.EscapedPath()
	}
	return u.String()
}

func escapeMarkdownText(text string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`[`, `\\[`,
		`]`, `\\]`,
		`(`, `\\(`,
		`)`, `\\)`,
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
	)
	return replacer.Replace(text)
}

func escapeMarkdownLinkURL(raw string) string {
	return strings.NewReplacer("(", "%28", ")", "%29", " ", "%20").Replace(raw)
}

func normalizeFormattedText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	blankCount := 0
	firstContentLine := true
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			blankCount++
			if blankCount <= 1 {
				result = append(result, "")
			}
			continue
		}
		blankCount = 0
		if firstContentLine {
			line = "  " + line
			firstContentLine = false
		}
		result = append(result, line)
	}
	joined := strings.Join(result, "\n")
	return strings.Trim(joined, "\n")
}

func formatPlainTextBody(body string) string {
	body = urlRegex.ReplaceAllStringFunc(body, func(u string) string {
		if len(u) > maxForwardURLLength {
			return ""
		}
		disp := truncateRunes(u, maxForwardLinkTextRunes)
		u = escapeMarkdownLinkURL(u)
		return fmt.Sprintf("[%s](%s)", disp, u)
	})
	return normalizeFormattedText(body)
}

func formatForwardBody(body string, includeLinks bool) string {
	body = normalizeFormattedText(body)
	if includeLinks {
		return normalizeFormattedText(formatURLsAsMarkdown(body))
	}
	return normalizeFormattedText(removeLinksFromText(body))
}

func formatURLsAsMarkdown(text string) string {
	var b strings.Builder
	last := 0
	matches := urlRegex.FindAllStringIndex(text, -1)
	for _, loc := range matches {
		start, end := loc[0], loc[1]
		if isMarkdownURLDestination(text, start, end) {
			continue
		}
		u := text[start:end]
		b.WriteString(text[last:start])
		if len(u) <= maxForwardURLLength {
			disp := truncateRunes(u, maxForwardLinkTextRunes)
			b.WriteString(fmt.Sprintf("[%s](%s)", disp, escapeMarkdownLinkURL(u)))
		}
		last = end
	}
	if last == 0 {
		return text
	}
	b.WriteString(text[last:])
	return b.String()
}

func removeLinksFromText(text string) string {
	text = markdownLinkRegex.ReplaceAllStringFunc(text, func(match string) string {
		parts := markdownLinkRegex.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}
		return unescapeMarkdownText(parts[1])
	})
	text = urlRegex.ReplaceAllString(text, "")

	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isDanglingLinkLabel(line) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func isMarkdownURLDestination(text string, start int, end int) bool {
	if start <= 0 || text[start-1] != '(' {
		return false
	}
	return (end < len(text) && text[end] == ')') || (end > start && text[end-1] == ')')
}

func isDanglingLinkLabel(line string) bool {
	line = strings.TrimSpace(line)
	line = strings.TrimRight(line, "：:")
	switch strings.ToLower(line) {
	case "链接", "url", "link":
		return true
	default:
		return false
	}
}

func truncateRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func unescapeMarkdownText(text string) string {
	replacer := strings.NewReplacer(
		`\\[`, `[`,
		`\\]`, `]`,
		`\\(`, `(`,
		`\\)`, `)`,
		"\\`", "`",
		`\\*`, `*`,
		`\\_`, `_`,
		`\\\\`, `\`,
	)
	return replacer.Replace(text)
}

func normalizeFolderKey(folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "inbox"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ".", "_")
	return strings.ToLower(replacer.Replace(folder))
}

func buildMessageID(accountID string, folder string, uid uint32) string {
	return fmt.Sprintf("%s-%s-%d", accountID, normalizeFolderKey(folder), uid)
}

func displaySubject(subject string) string {
	if strings.TrimSpace(subject) == "" {
		return "(无主题)"
	}
	return subject
}

func messageDateForFilter(msg *imap.Message) time.Time {
	if msg == nil {
		return time.Time{}
	}
	if msg.Envelope != nil && !msg.Envelope.Date.IsZero() {
		return msg.Envelope.Date
	}
	return msg.InternalDate
}

func messageDateForStore(msg *imap.Message) time.Time {
	if msg == nil || msg.Envelope == nil || msg.Envelope.Date.IsZero() {
		return time.Time{}
	}
	return msg.Envelope.Date
}

func displayMessageDate(date time.Time) string {
	if date.IsZero() {
		return "无发送时间"
	}
	utc8 := time.FixedZone("UTC+8", 8*60*60)
	return date.In(utc8).Format("2006-01-02 15:04:05")
}

func fetchFirstMessage(c *client.Client, seqSet *imap.SeqSet, items []imap.FetchItem) (*imap.Message, error) {
	messages := make(chan *imap.Message, 1)
	fetchDone := make(chan error, 1)

	go func() {
		fetchDone <- c.UidFetch(seqSet, items, messages)
	}()

	var msg *imap.Message
	for m := range messages {
		if msg == nil {
			msg = m
		}
	}
	if err := <-fetchDone; err != nil {
		return nil, err
	}
	return msg, nil
}

func checkMailForAccount(account *EmailAccount) {
	if !account.Enabled {
		return
	}

	// 防并发：如果当前账号的上一轮检查卡住未完成，则放弃本次调度
	if _, loaded := accountChecking.LoadOrStore(account.ID, true); loaded {
		return
	}
	defer accountChecking.Delete(account.ID)

	// 记录最后检查时间而不是频繁刷屏日志
	accountLastCheck.Store(account.ID, time.Now().Format("2006-01-02 15:04:05"))

	folders := account.Folders
	if len(folders) == 0 {
		folders = []string{"INBOX"}
	}

	// 每个文件夹独立建立 IMAP 连接，避免 Select 切换导致 UID 操作错乱
	for _, folder := range folders {
		checkFolderForAccount(account, folder)
	}
}

// connectIMAP 建立 IMAP 连接并登录，返回客户端和错误
func connectIMAP(account *EmailAccount) (*client.Client, error) {
	imapServer := account.ImapServer
	if !strings.Contains(imapServer, ":") {
		imapServer = imapServer + ":993"
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	c, err := client.DialWithDialerTLS(dialer, imapServer, nil)
	if err != nil {
		return nil, fmt.Errorf("IMAP连接失败: %w", err)
	}

	if err := c.Login(account.EmailUser, account.EmailPass); err != nil {
		c.Logout()
		return nil, fmt.Errorf("IMAP登录失败: %w", err)
	}
	return c, nil
}

// checkFolderForAccount 检查单个文件夹的新邮件，使用独立的 IMAP 连接
func checkFolderForAccount(account *EmailAccount, folder string) {
	c, err := connectIMAP(account)
	if err != nil {
		addLog(fmt.Sprintf("[%s/%s] %v", account.Name, folder, err), "error")
		return
	}
	defer c.Logout()

	_, err = c.Select(folder, false)
	if err != nil {
		addLog(fmt.Sprintf("选择文件夹失败 [%s/%s]: %v", account.Name, folder, err), "error")
		return
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	uids, searchErr := c.UidSearch(criteria)
	if searchErr != nil {
		addLog(fmt.Sprintf("搜索未读邮件失败 [%s/%s]: %v", account.Name, folder, searchErr), "error")
		return
	}
	addLog(fmt.Sprintf("检查邮箱文件夹 [%s/%s]: 未读邮件 %d 封", account.Name, folder, len(uids)), "info")

	limit := 10
	if len(uids) > limit {
		addLog(fmt.Sprintf("邮箱文件夹 [%s/%s] 未读邮件过多，本轮仅处理最新 %d 封", account.Name, folder, limit), "warning")
		uids = uids[len(uids)-limit:]
	}

	for _, uid := range uids {
		msgID := buildMessageID(account.ID, folder, uid)

		// 检查是否已处理
		var count int
		if err := dbQueryRow(`SELECT COUNT(*) FROM messages WHERE id = ?`, msgID).Scan(&count); err != nil {
			addLog(fmt.Sprintf("检查消息处理状态失败 [%s/%s uid=%d]: %v", account.Name, folder, uid, err), "error")
			continue
		}
		if count > 0 {
			continue
		}

		seqSet := new(imap.SeqSet)
		seqSet.AddNum(uid)

		msg, fetchErr := fetchFirstMessage(c, seqSet, []imap.FetchItem{imap.FetchEnvelope, imap.FetchInternalDate})
		if fetchErr != nil {
			addLog(fmt.Sprintf("读取邮件失败 [%s/%s uid=%d]: %v", account.Name, folder, uid, fetchErr), "error")
			continue
		}

		if msg == nil || msg.Envelope == nil {
			addLog(fmt.Sprintf("读取邮件为空 [%s/%s uid=%d]", account.Name, folder, uid), "warning")
			continue
		}
		filterDate := messageDateForFilter(msg)
		if filterDate.IsZero() || filterDate.Before(serviceStartedAt) {
			addLog(fmt.Sprintf("跳过历史邮件 [%s/%s uid=%d]: %s", account.Name, folder, uid, displaySubject(msg.Envelope.Subject)), "info")
			continue
		}

		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].Address()
		}

		var section imap.BodySectionName
		bodyMsg, fetchErr := fetchFirstMessage(c, seqSet, []imap.FetchItem{section.FetchItem()})
		if fetchErr != nil {
			addLog(fmt.Sprintf("读取邮件正文失败 [%s/%s uid=%d]: %v", account.Name, folder, uid, fetchErr), "error")
			continue
		}

		if bodyMsg == nil {
			addLog(fmt.Sprintf("读取邮件正文为空 [%s/%s uid=%d]: %s", account.Name, folder, uid, displaySubject(msg.Envelope.Subject)), "warning")
			continue
		}

		r := bodyMsg.GetBody(&section)
		if r == nil {
			addLog(fmt.Sprintf("邮件正文为空 [%s/%s uid=%d]: %s", account.Name, folder, uid, displaySubject(msg.Envelope.Subject)), "warning")
			continue
		}
		mr, err := mail.CreateReader(r)
		if err != nil {
			addLog(fmt.Sprintf("解析邮件失败 [%s/%s uid=%d]: %v", account.Name, folder, uid, err), "error")
			continue
		}

		var body, bodyHTML string
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				contentType, _, _ := h.ContentType()
				b, _ := io.ReadAll(p.Body)
				if contentType == "text/html" {
					bodyHTML = string(b)
					body = cleanHTML(string(b))
				} else if contentType == "text/plain" && body == "" {
					body = formatPlainTextBody(string(b))
				}
			}
		}

		// 存储消息
		newMsg := &Message{
			ID:          msgID,
			SourceEmail: account.EmailUser,
			AccountID:   account.ID,
			Subject:     msg.Envelope.Subject,
			From:        from,
			To:          account.EmailUser,
			Date:        messageDateForStore(msg),
			Body:        body,
			BodyHTML:    bodyHTML,
			Status:      "pending",
			CreatedAt:   time.Now(),
		}

		if err := saveMessage(newMsg); err != nil {
			addLog(fmt.Sprintf("保存消息失败: %v", err), "error")
			continue
		}

		addLog(fmt.Sprintf("收到新邮件 [%s/%s]: %s", account.Name, folder, displaySubject(msg.Envelope.Subject)), "success")

		// 标记已读
		c.UidStore(seqSet, imap.AddFlags, []interface{}{imap.SeenFlag}, nil)
	}
}
