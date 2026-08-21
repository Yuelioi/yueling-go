package knowledge

import (
	"bytes"
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

var ErrEntryLimit = errors.New("当前知识库空间的条目已达上限")

func AddText(groupID, createdBy int64, title, content string) (*db.GroupKnowledge, error) {
	return AddTextWithShortcuts(groupID, createdBy, title, content, nil)
}

func AddTextWithShortcuts(groupID, createdBy int64, title, content string, rawShortcuts []string) (*db.GroupKnowledge, error) {
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
	if err := checkLimit(groupID); err != nil {
		return nil, err
	}
	row, err := db.CreateGroupKnowledge(groupID, createdBy, title, content, "")
	if err != nil {
		return nil, err
	}
	return attachShortcuts(row, shortcuts)
}

func AddURL(groupID, createdBy int64, title, rawURL string) (*db.GroupKnowledge, error) {
	return AddURLWithShortcuts(groupID, createdBy, title, rawURL, nil)
}

func AddURLWithShortcuts(groupID, createdBy int64, title, rawURL string, rawShortcuts []string) (*db.GroupKnowledge, error) {
	shortcuts, err := NormalizeShortcuts(rawShortcuts)
	if err != nil {
		return nil, err
	}
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || len(rawURL) > 1024 {
		return nil, fmt.Errorf("请输入有效的公网 HTTP/HTTPS 地址")
	}
	if err := checkLimit(groupID); err != nil {
		return nil, err
	}
	documentTitle, content, err := FetchPublicPage(rawURL)
	if err != nil {
		return nil, fmt.Errorf("导入网页失败: %s", cleanText(err.Error(), 160))
	}
	if strings.TrimSpace(title) == "" {
		title = documentTitle
	}
	if strings.TrimSpace(title) == "" {
		title = parsed.Hostname()
	}
	row, err := db.CreateGroupKnowledge(groupID, createdBy, cleanText(title, MaxTitleRunes), cleanContent(content, MaxContentRunes), rawURL)
	if err != nil {
		return nil, err
	}
	return attachShortcuts(row, shortcuts)
}

func Remove(id uint, groupID int64) error {
	return db.DeleteGroupKnowledge(id, groupID)
}

func List(groupID int64) ([]db.GroupKnowledge, error) {
	return db.ListGroupKnowledge(groupID)
}

// ListAvailable returns knowledge visible to a group: its private entries plus
// entries maintained in the all-groups shared scope.
func ListAvailable(groupID int64) ([]db.GroupKnowledge, error) {
	return db.ListAvailableGroupKnowledge(groupID)
}

func SetShortcuts(id uint, groupID int64, rawShortcuts []string) ([]db.GroupKnowledgeShortcut, error) {
	shortcuts, err := NormalizeShortcuts(rawShortcuts)
	if err != nil {
		return nil, err
	}
	return db.SetGroupKnowledgeShortcuts(id, groupID, shortcuts)
}

func FindShortcut(groupID int64, text string) (*db.GroupKnowledge, error) {
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > MaxShortcutRunes {
		return nil, nil
	}
	row, err := db.FindGroupKnowledgeShortcut(groupID, text)
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

func Search(groupID int64, question string, limit int) ([]db.GroupKnowledge, error) {
	question = cleanText(question, MaxQuestionRunes)
	if question == "" {
		return nil, fmt.Errorf("问题不能为空")
	}
	if limit <= 0 || limit > 8 {
		limit = 5
	}
	return db.SearchGroupKnowledge(groupID, question, limit)
}

func attachShortcuts(row *db.GroupKnowledge, shortcuts []string) (*db.GroupKnowledge, error) {
	if len(shortcuts) == 0 {
		row.Shortcuts = []db.GroupKnowledgeShortcut{}
		return row, nil
	}
	created, err := db.SetGroupKnowledgeShortcuts(row.ID, row.GroupID, shortcuts)
	if err != nil {
		_ = db.DeleteGroupKnowledge(row.ID, row.GroupID)
		if errors.Is(err, db.ErrKnowledgeShortcutConflict) {
			return nil, err
		}
		return nil, fmt.Errorf("保存快捷触发词: %w", err)
	}
	row.Shortcuts = created
	return row, nil
}

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

func FetchPublicPage(rawURL string) (string, string, error) {
	body, err := httpclient.GetPublicBytesLimit(rawURL, maxImportBytes, "Accept", "text/html, text/plain;q=0.9")
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

func checkLimit(groupID int64) error {
	count, err := db.CountGroupKnowledge(groupID)
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
