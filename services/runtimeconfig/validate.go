package runtimeconfig

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
)

func Validate(c config.Config) error {
	if err := validateProfiles(c.AI); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if _, err := time.LoadLocation(c.Bot.Timezone); err != nil {
		return fmt.Errorf("时区无效")
	}
	for _, id := range c.Bot.SuperUsers {
		if id <= 0 {
			return fmt.Errorf("超级管理员 QQ 必须为正整数")
		}
	}
	if c.AI.MaxTokens < 1 || c.AI.MaxTokens > 200000 || c.AI.ReplyMaxChars < 1 || c.AI.ReplyMaxChars > 10000 {
		return fmt.Errorf("AI 输出限制无效")
	}
	if c.AI.RateLimit.UserPerMin < 0 || c.AI.RateLimit.GroupPerMin < 0 {
		return fmt.Errorf("AI 频率限制不能为负数")
	}
	if c.AI.Context.ChatHistory < 1 || c.AI.Context.ChatHistory > 30 || c.AI.Context.Summary < 10 || c.AI.Context.Summary > 100 {
		return fmt.Errorf("上下文条数：聊天 1～30，总结 10～100")
	}
	if c.AI.Affinity.Max <= c.AI.Affinity.Min || c.AI.Affinity.Initial < c.AI.Affinity.Min || c.AI.Affinity.Initial > c.AI.Affinity.Max {
		return fmt.Errorf("好感度范围或初始值无效")
	}
	if c.Image.ConvertQuality < 1 || c.Image.ConvertQuality > 100 || c.Image.ConvertMinKB < 0 {
		return fmt.Errorf("图片转换质量或大小无效")
	}
	if c.Pack.MaxImages < 1 || c.Pack.MaxImages > 10000 || c.Pack.MaxMB < 1 || c.Pack.MaxMB > 4096 {
		return fmt.Errorf("打包限制无效")
	}
	for label, value := range map[string]string{"表情包服务地址": c.Tools.MemeServer} {
		if value == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("%s 必须是 HTTP/HTTPS 地址", label)
		}
	}
	if c.Bilibili.Cookie != "" {
		if strings.ContainsAny(c.Bilibili.Cookie, "\r\n") {
			return fmt.Errorf("B站 Cookie 必须为单行文本")
		}
		req := http.Request{Header: http.Header{"Cookie": {c.Bilibili.Cookie}}}
		cookies := map[string]string{}
		for _, cookie := range req.Cookies() {
			cookies[cookie.Name] = cookie.Value
		}
		uid, err := strconv.ParseInt(cookies["DedeUserID"], 10, 64)
		if err != nil || uid <= 0 || cookies["SESSDATA"] == "" {
			return fmt.Errorf("B站 Cookie 需要有效的 DedeUserID 和 SESSDATA；回复还需 bili_jct 与 buvid3")
		}
	}
	if c.Tools.Proxy != "" {
		u, err := url.Parse(c.Tools.Proxy)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
			return fmt.Errorf("代理地址无效")
		}
	}
	if c.NapCat.URL != "" {
		u, err := url.Parse(c.NapCat.URL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "ws" && u.Scheme != "wss") {
			return fmt.Errorf("QQ WebSocket 地址无效")
		}
	}
	for label, value := range map[string]string{"QQ监听": c.NapCat.Serve, "后台监听": c.WebUI.Addr, "消息接口监听": c.HTTPAPI.Addr} {
		if value != "" {
			_, port, err := net.SplitHostPort(value)
			n, portErr := strconv.Atoi(port)
			if err != nil || portErr != nil || n < 1 || n > 65535 {
				return fmt.Errorf("%s 地址应包含端口", label)
			}
		}
	}
	for _, id := range c.Plugins.Disabled {
		if !catalog.Known(id) {
			return fmt.Errorf("未知插件编号")
		}
	}
	return nil
}
