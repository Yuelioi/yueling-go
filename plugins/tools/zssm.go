package tools

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/Yuelioi/yueling-go/services/httpclient"
)

//go:embed zssm_prompt.txt
var zssmSystemPrompt string

const zssmMaxPageChars = 8000

func extractVisibleText(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript") {
			return
		}
		if n.Type == html.TextNode {
			if t := strings.TrimSpace(n.Data); t != "" {
				sb.WriteString(t)
				sb.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	out := sb.String()
	if len([]rune(out)) > zssmMaxPageChars {
		out = string([]rune(out)[:zssmMaxPageChars])
	}
	return out
}

func fetchPageText(url string) (string, error) {
	body, err := httpclient.Direct.GetBytes(url, "User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	if err != nil {
		return "", err
	}
	return extractVisibleText(body), nil
}

var reCodeFence = regexp.MustCompile("(?s)^```[a-zA-Z]*\\s*|\\s*```$")

type zssmOutput struct {
	Output  string   `json:"output"`
	Keyword []string `json:"keyword"`
	Block   bool     `json:"block"`
}

func formatZssmResponse(raw string) (string, error) {
	data := reCodeFence.ReplaceAllString(strings.TrimSpace(raw), "")
	var out zssmOutput
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return "", err
	}
	if out.Block {
		return "（抱歉, 我现在还不会这个）", nil
	}
	if out.Output == "" {
		return "（AI回复内容异常，请重试）", nil
	}
	if len(out.Keyword) > 0 {
		return fmt.Sprintf("关键词：%s\n\n%s", strings.Join(out.Keyword, " | "), out.Output), nil
	}
	return out.Output, nil
}
