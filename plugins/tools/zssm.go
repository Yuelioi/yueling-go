package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	openai "github.com/sashabaranov/go-openai"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
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

const zssmMaxImageBytes = 8 * 1024 * 1024

func imageToDataURL(url string) (string, error) {
	body, err := httpclient.Direct.GetBytes(url)
	if err != nil {
		return "", err
	}
	if len(body) > zssmMaxImageBytes {
		return "", fmt.Errorf("图片过大")
	}
	mime := http.DetectContentType(body)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func describeImage(url string) (string, error) {
	vl := config.C.AI.VL
	dataURL, err := imageToDataURL(url)
	if err != nil {
		return "", err
	}
	client := ai.NewClient(vl.Key, vl.BaseURL)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: vl.Model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: dataURL}},
					{Type: openai.ChatMessagePartTypeText, Text: "请你作为文本模型的眼睛，告诉她这张图片的内容"},
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("VL 无响应")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
