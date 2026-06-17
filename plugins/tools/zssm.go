package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	openai "github.com/sashabaranov/go-openai"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"github.com/Yuelioi/yueling-go/services/logx"
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

const zssmMaxImages = 2

func RegisterZssm(b *bot.Bot) {
	b.OnCommand("zssm").Handle(func(ctx *bot.CommandContext) error {
		ctx.React(bot.EmojiProcessing)
		var userPrompt strings.Builder
		var rawInput strings.Builder

		var repliedMsg bot.Message
		if replyID, ok := ctx.Message().ReplyID(); ok {
			var mid int32
			fmt.Sscan(replyID, &mid)
			if mid != 0 {
				if replied, err := ctx.GetMsg(mid); err == nil {
					repliedMsg = replied
					t := replied.Text()
					userPrompt.WriteString("<type: text>\n" + t + "\n</type: text>")
					rawInput.WriteString(t)
				}
			}
		}

		argText := strings.TrimSpace(strings.Join(ctx.Args, " "))
		if argText != "" {
			userPrompt.WriteString("<type: interest>\n" + argText + "\n</type: interest>")
			rawInput.WriteString(" " + argText)
		}

		images := ctx.Message().ImageURLs()
		if repliedMsg != nil {
			images = append(images, repliedMsg.ImageURLs()...)
		}

		if userPrompt.Len() == 0 && len(images) == 0 {
			return ctx.Reply("请回复或输入内容")
		}

		if len(images) > 0 {
			if config.C.AI.VL.Key == "" {
				return ctx.Reply("未配置图片识别")
			}
			if len(images) > zssmMaxImages {
				return ctx.Reply("图片数量超过限制, 最多 2 张")
			}
			for i, u := range images {
				desc, err := describeImage(u)
				if err != nil {
					logx.Warnf("[zssm] 图片识别失败 url=%s: %v", u, err)
					return ctx.Reply("图片识别失败")
				}
				userPrompt.WriteString(fmt.Sprintf("\n<type: image, id: %d>\n%s\n</type: image, id: %d>", i, desc, i))
			}
		}

		if u := extractURL(rawInput.String()); u != "" {
			if page, err := fetchPageText(u); err == nil && page != "" {
				userPrompt.WriteString(fmt.Sprintf("\n<type: web_page, url: %s>\n%s\n</type: web_page>", u, page))
			}
		}

		randomNumber := fmt.Sprintf("%08d", rand.Intn(100000000))
		systemPrompt := zssmSystemPrompt + randomNumber
		finalUser := fmt.Sprintf("<random number: %s>\n%s\n</random number: %s>", randomNumber, userPrompt.String(), randomNumber)

		result, err := zssmGenerate(systemPrompt, finalUser)
		if err != nil {
			result, err = zssmGenerate(systemPrompt, finalUser)
		}
		if err != nil {
			logx.Errorf("[zssm] AI 生成失败: %v", err)
			return ctx.Reply("AI 回复解析失败, 请重试")
		}
		return ctx.Reply(result)
	})
}

func zssmGenerate(systemPrompt, userPrompt string) (string, error) {
	cfg := config.C.AI
	client := ai.NewClient(cfg.DeepSeekKey, cfg.BaseURL)
	resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("无响应")
	}
	return formatZssmResponse(resp.Choices[0].Message.Content)
}
