package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/config"
)

const maxResponseBytes = 2 << 20

type Source struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Summary string  `json:"summary"`
	Snippet string  `json:"snippet"`
	Date    *string `json:"date"`
}

type Page struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Error struct{ Status int }

func (e *Error) Error() string {
	switch e.Status {
	case 400, 413:
		return "搜索请求参数无效，请缩短关键词或检查网页地址"
	case 401:
		return "搜索账号凭据无效，请检查所选搜索配置"
	case 402:
		return "搜索账号余额不足"
	case 403:
		return "网页读取被服务方拒绝"
	case 404:
		return "未能取得网页正文，或搜索服务地址不支持此接口"
	case 429:
		return "搜索服务繁忙，请稍后再试"
	default:
		return "搜索服务请求失败，请稍后再试"
	}
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

func ForAI(a config.AIConfig) (*Client, error) {
	p, conn, err := a.Select("search")
	if err != nil {
		return nil, errors.New("请先在 AI 模型中创建联网搜索配置，再在 AI 功能分配中指定；搜索不会继承聊天模型")
	}
	if conn.Kind != "openai" || !p.Supports("web_search") {
		return nil, errors.New("搜索配置必须使用支持 DeepSeek Search/Fetch 的 API 账号")
	}
	timeout := 15 * time.Second
	if p.TimeoutSeconds > 0 {
		timeout = time.Duration(p.TimeoutSeconds) * time.Second
	}
	return New(conn.BaseURL, conn.APIKey, timeout)
}

func New(baseURL, key string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("搜索 API 地址无效")
	}
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n") {
		return nil, errors.New("所选搜索账号尚未配置有效 API Key")
	}
	switch {
	case strings.HasSuffix(u.Path, "/v1/tools"):
	case strings.HasSuffix(u.Path, "/v1"):
		u.Path += "/tools"
	default:
		u.Path += "/v1/tools"
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{baseURL: u.String(), key: key, http: &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Search(ctx context.Context, query string, count int, question string) ([]Source, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 128 {
		return nil, errors.New("搜索关键词需为1至128个字符，请精简关键词")
	}
	if count == 0 {
		count = 5
	}
	if count < 1 || count > 10 {
		return nil, errors.New("一次搜索请返回1至10条结果")
	}
	body := map[string]any{"query": query, "count": count, "need_content": false}
	question = strings.TrimSpace(question)
	if question != "" {
		body["ori_question"] = string([]rune(question)[:min(128, utf8.RuneCountInString(question))])
	}
	var result struct {
		Results *[]Source `json:"results"`
	}
	if err := c.request(ctx, "search", body, &result); err != nil {
		return nil, err
	}
	if result.Results == nil {
		return nil, errors.New("搜索服务未返回有效结果列表")
	}
	rows := make([]Source, 0, count)
	for _, row := range *result.Results {
		if validPageURL(row.URL) != nil {
			continue
		}
		rows = append(rows, row)
		if len(rows) == count {
			break
		}
	}
	if len(*result.Results) > 0 && len(rows) == 0 {
		return nil, errors.New("搜索服务未返回可用的公开网页来源")
	}
	return rows, nil
}

func (c *Client) Fetch(ctx context.Context, rawURL string) (Page, error) {
	if err := validPageURL(rawURL); err != nil {
		return Page{}, err
	}
	var page Page
	if err := c.request(ctx, "fetch", map[string]string{"url": rawURL}, &page); err != nil {
		return Page{}, err
	}
	if strings.TrimSpace(page.Content) == "" {
		return Page{}, errors.New("网页没有可读取的正文，不能据此猜测内容")
	}
	page.URL = rawURL
	return page, nil
}

func validPageURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || utf8.RuneCountInString(raw) > 8192 || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("请提供不含账号密码的公开 HTTP/HTTPS 网页地址")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || !strings.Contains(host, ".") && net.ParseIP(host) == nil || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return errors.New("只支持公开网页，不能读取本地或内网地址")
	}
	if ip := net.ParseIP(host); ip != nil && (!ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return errors.New("只支持公开网页，不能读取本地或内网地址")
	}
	return nil
}

func (c *Client) request(ctx context.Context, operation string, body, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+operation, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		response, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errors.New("搜索服务连接失败或超时")
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			status := response.StatusCode
			if attempt == 0 && (status == 408 || status == 429 || status >= 500 && status <= 599) {
				delay := 250 * time.Millisecond
				if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
					if seconds > 2 {
						return &Error{Status: status}
					}
					delay = time.Duration(seconds) * time.Second
				}
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				}
				continue
			}
			return &Error{Status: status}
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		response.Body.Close()
		if err != nil {
			return errors.New("搜索结果读取失败")
		}
		if len(raw) > maxResponseBytes {
			return errors.New("搜索结果超过大小限制")
		}
		if err := json.Unmarshal(raw, result); err != nil {
			return errors.New("搜索服务返回格式无效")
		}
		return nil
	}
	return errors.New("搜索服务暂时不可用")
}
