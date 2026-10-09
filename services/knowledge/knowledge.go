package knowledge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"golang.org/x/net/html"
	"gorm.io/gorm"
)

const (
	MaxEntriesPerGroup = 100
	MaxTitleRunes      = 80
	MaxContentRunes    = 12000
	MaxQuestionRunes   = 300
	MaxShortcutRunes   = 128
	MaxShortcuts       = 10
	maxImportBytes     = 2 * 1024 * 1024
	maxContextRunes    = 9000
	maxExcerptRunes    = 1800
	maxShortcutReply   = 2000
)

type Manager struct{ store *db.KnowledgeStore }

func New(store *db.KnowledgeStore) *Manager { return &Manager{store: store} }

var ErrEntryLimit = db.ErrKnowledgeLimit

func (m *Manager) AddText(groupID, createdBy int64, title, content string) (*db.GroupKnowledge, error) {
	return m.AddTextWithShortcuts(groupID, createdBy, title, content, nil)
}

func (m *Manager) AddTextWithShortcuts(groupID, createdBy int64, title, content string, rawShortcuts []string) (*db.GroupKnowledge, error) {
	shortcuts, err := NormalizeShortcuts(rawShortcuts)
	if err != nil {
		return nil, err
	}
	title = cleanText(title, MaxTitleRunes)
	content = cleanContent(content, MaxContentRunes)
	if content == "" {
		return nil, fmt.Errorf("知识内容不能为空")
	}
	if title == "" {
		title = cleanText(content, 24)
	}
	return m.store.CreateEntry(groupID, createdBy, title, content, "", shortcuts, MaxEntriesPerGroup)
}

func (m *Manager) AddURL(ctx context.Context, groupID, createdBy int64, title, rawURL string) (*db.GroupKnowledge, error) {
	return m.AddURLWithShortcuts(ctx, groupID, createdBy, title, rawURL, nil)
}

func (m *Manager) AddURLWithShortcuts(ctx context.Context, groupID, createdBy int64, title, rawURL string, rawShortcuts []string) (*db.GroupKnowledge, error) {
	shortcuts, err := NormalizeShortcuts(rawShortcuts)
	if err != nil {
		return nil, err
	}
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || len(rawURL) > 1024 {
		return nil, fmt.Errorf("请输入有效的公网 HTTP/HTTPS 地址")
	}
	if err := m.checkLimit(groupID); err != nil {
		return nil, err
	}
	documentTitle, content, err := FetchPublicPage(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("导入网页失败: %s", cleanText(err.Error(), 160))
	}
	if strings.TrimSpace(title) == "" {
		title = documentTitle
	}
	if strings.TrimSpace(title) == "" {
		title = parsed.Hostname()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.store.CreateEntry(groupID, createdBy, cleanText(title, MaxTitleRunes), cleanContent(content, MaxContentRunes), rawURL, shortcuts, MaxEntriesPerGroup)
}

func (m *Manager) Remove(id uint, groupID int64) error {
	return m.store.DeleteGroupKnowledge(id, groupID)
}

func (m *Manager) List(groupID int64) ([]db.GroupKnowledge, error) {
	return m.store.ListGroupKnowledge(groupID)
}

// ListAvailable returns knowledge visible to a group: its private entries plus
// entries maintained in the all-groups shared scope.
func (m *Manager) ListAvailable(groupID int64) ([]db.GroupKnowledge, error) {
	return m.store.ListAvailableGroupKnowledge(groupID)
}

func (m *Manager) SetShortcuts(id uint, groupID int64, rawShortcuts []string) ([]db.GroupKnowledgeShortcut, error) {
	shortcuts, err := NormalizeShortcuts(rawShortcuts)
	if err != nil {
		return nil, err
	}
	return m.store.SetGroupKnowledgeShortcuts(id, groupID, shortcuts)
}

func (m *Manager) FindShortcut(groupID int64, text string) (*db.GroupKnowledge, error) {
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > MaxShortcutRunes {
		return nil, nil
	}
	row, err := m.store.FindGroupKnowledgeShortcut(groupID, text)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return row, err
}

func ShortcutResponse(row db.GroupKnowledge, trigger string) string {
	if row.SourceURL != "" {
		return truncateRunes(strings.TrimSpace(row.Title)+"\n"+row.SourceURL, maxShortcutReply)
	}
	content := strings.ReplaceAll(row.Content, "{}", strings.TrimSpace(trigger))
	return truncateRunes(strings.TrimSpace(content), maxShortcutReply)
}

func NormalizeShortcuts(values []string) ([]string, error) {
	if len(values) > MaxShortcuts {
		return nil, fmt.Errorf("每条知识最多设置 %d 个快捷触发词", MaxShortcuts)
	}
	seen := make(map[string]bool, len(values))
	shortcuts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
		if value == "" || seen[value] {
			continue
		}
		if utf8.RuneCountInString(value) > MaxShortcutRunes {
			return nil, fmt.Errorf("快捷触发词最多 %d 个字符", MaxShortcutRunes)
		}
		seen[value] = true
		shortcuts = append(shortcuts, value)
	}
	return shortcuts, nil
}

func (m *Manager) Search(groupID int64, question string, limit int) ([]db.GroupKnowledge, error) {
	question = cleanText(question, MaxQuestionRunes)
	if question == "" {
		return nil, fmt.Errorf("问题不能为空")
	}
	if limit <= 0 || limit > 8 {
		limit = 5
	}
	return m.store.SearchGroupKnowledge(groupID, normalizeKnowledgeQuestion(question), limit)
}

// Normalize common group-chat aliases before PostgreSQL tokenization. This is
// lexical retrieval, not an embedding-based semantic search or a model guess.
var knowledgeAliases = strings.NewReplacer(
	"新群友", "新成员", "新人", "新成员", "进群", "入群", "群昵称", "群名片",
)

func normalizeKnowledgeQuestion(question string) string { return knowledgeAliases.Replace(question) }

func BuildContext(rows []db.GroupKnowledge) string {
	var builder strings.Builder
	for _, row := range rows {
		content := cleanContent(row.Content, maxExcerptRunes)
		scope := "group"
		if row.GroupID == db.SharedKnowledgeGroupID {
			scope = "shared"
		}
		block := fmt.Sprintf("<knowledge id=\"%d\" scope=\"%s\" title=\"%s\">\n%s\n", row.ID, scope, escapeAttribute(row.Title), escapeText(content))
		if row.SourceURL != "" {
			block += "来源: " + escapeText(row.SourceURL) + "\n"
		}
		block += "</knowledge>\n"
		if utf8.RuneCountInString(builder.String())+utf8.RuneCountInString(block) > maxContextRunes {
			break
		}
		builder.WriteString(block)
	}
	return builder.String()
}

func FetchPublicPage(ctx context.Context, rawURL string) (string, string, error) {
	body, err := httpclient.GetPublicBytesLimitContext(ctx, rawURL, maxImportBytes, "Accept", "text/html, text/plain;q=0.9")
	if err != nil {
		return "", "", err
	}
	mime := http.DetectContentType(body)
	if strings.HasPrefix(mime, "text/plain") {
		content := cleanContent(string(body), MaxContentRunes)
		if content == "" {
			return "", "", fmt.Errorf("网页没有可导入的文本")
		}
		return "", content, nil
	}
	if !strings.HasPrefix(mime, "text/html") {
		return "", "", fmt.Errorf("当前只支持 HTML 或纯文本网页")
	}
	return extractHTML(body)
}

func extractHTML(body []byte) (string, string, error) {
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	var title string
	var builder strings.Builder
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, hidden bool) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "script", "style", "noscript", "svg", "nav", "footer":
				hidden = true
			case "title":
				if node.FirstChild != nil {
					title = node.FirstChild.Data
				}
			}
		}
		if !hidden && node.Type == html.TextNode {
			if value := strings.TrimSpace(node.Data); value != "" {
				builder.WriteString(value)
				builder.WriteByte('\n')
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, hidden)
		}
	}
	walk(document, false)
	content := cleanContent(builder.String(), MaxContentRunes)
	if content == "" {
		return "", "", fmt.Errorf("网页没有可导入的文本")
	}
	return cleanText(title, MaxTitleRunes), content, nil
}

func (m *Manager) checkLimit(groupID int64) error {
	count, err := m.store.CountGroupKnowledge(groupID)
	if err != nil {
		return err
	}
	if count >= MaxEntriesPerGroup {
		return ErrEntryLimit
	}
	return nil
}

func cleanText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	return truncateRunes(value, maxRunes)
}

func cleanContent(value string, maxRunes int) string {
	lines := strings.Split(strings.ReplaceAll(value, "\x00", ""), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return truncateRunes(strings.Join(cleaned, "\n"), maxRunes)
}

func truncateRunes(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

func escapeAttribute(value string) string {
	return strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func escapeText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func (m *Manager) GroupDisabled(groupID int64, pluginID int) (bool, error) {
	return m.store.IsGroupPluginDisabled(groupID, pluginID)
}
