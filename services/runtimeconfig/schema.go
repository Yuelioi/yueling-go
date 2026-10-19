package runtimeconfig

import (
	"reflect"
	"strings"

	"github.com/Yuelioi/yueling-go/config"
)

type Field struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Value      any    `json:"value"`
	Secret     bool   `json:"secret"`
	Configured bool   `json:"configured"`
	Reload     bool   `json:"reload"`
}
type Section struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}
type View struct {
	AppliedRevision int64          `json:"applied_revision"`
	RestartPending  bool           `json:"restart_pending"`
	Revision        int64          `json:"revision"`
	Sections        []Section      `json:"sections"`
	Bootstrap       map[string]any `json:"bootstrap"`
}

var sectionOrder = []string{"ai", "bot", "plugins", "tools", "image", "pack", "feed", "bilibili", "napcat", "http_api", "webui", "database"}
var titles = map[string]string{"ai": "AI 配置", "bot": "机器人", "plugins": "插件开关", "tools": "工具与服务", "image": "图片素材", "pack": "打包限制", "feed": "订阅服务", "bilibili": "B站账号", "napcat": "QQ 连接", "http_api": "消息接口", "webui": "管理后台", "database": "数据库连接池"}
var secrets = map[string]bool{"ai.deepseek_key": true, "ai.vl.key": true, "tools.qweather_key": true, "tools.tavily_key": true, "tools.proxy": true, "bilibili.cookie": true, "napcat.token": true, "http_api.key": true, "webui.password": true}
var labels = map[string]string{
	"ai.reasoning_effort": "推理强度",
	"ai.connections":      "连接账号", "ai.bindings": "功能模型分配", "ai.profiles": "模型配置", "ai.conversation_profile": "对话模型", "ai.vision_profile": "视觉模型", "ai.deepseek_key": "AI API Key", "ai.base_url": "API 地址", "ai.model": "模型名称", "ai.max_tokens": "最大输出 Token", "ai.reply_max_chars": "群聊回复字数上限",
	"ai.vl.key": "视觉模型 API Key", "ai.vl.base_url": "视觉模型 API 地址", "ai.vl.model": "视觉模型名称",
	"ai.ratelimit.user_per_min": "每人每分钟 AI 调用上限（0 不限）", "ai.ratelimit.group_per_min": "每群每分钟 AI 调用上限（0 不限）",
	"ai.context.chat_history": "默认聊天上下文条数", "ai.context.summary": "默认总结条数",
	"ai.affinity.enabled": "启用 AI 好感度", "ai.affinity.initial": "初始好感度", "ai.affinity.block_below": "静默阈值", "ai.affinity.min": "最低好感度", "ai.affinity.max": "最高好感度",
	"bot.blacklist": "全局黑名单 QQ",
	"bot.name":      "机器人名称", "bot.superusers": "超级管理员 QQ", "bot.cmd_prefix": "命令前缀（留空不需要前缀）", "bot.command_arg_space_required": "命令参数必须以空格分隔", "bot.timezone": "时区",
	"plugins.disabled": "全局关闭的插件", "tools.qweather_key": "和风天气 API Key", "tools.qweather_host": "和风天气 Host", "tools.tavily_key": "搜索 API Key", "tools.proxy": "HTTP 代理地址", "tools.meme_server": "表情包服务地址",
	"image.convert": "保存图片时转为 JPEG", "image.convert_min_kb": "转换图片最小大小（KB）", "image.convert_quality": "JPEG 质量（1～100）", "image.entry": "图片类目",
	"pack.max_images": "一次最多打包图片数", "pack.max_mb": "一次最多打包大小（MB）", "feed.rsshub_base": "RSSHub 地址",
	"bilibili.cookie": "B站登录 Cookie", "napcat.url": "正向 WebSocket 地址", "napcat.serve": "反向 WebSocket 监听地址", "napcat.token": "OneBot Token",
	"http_api.addr": "消息接口监听地址（留空关闭）", "http_api.key": "消息接口访问 Key", "webui.enabled": "启用管理后台", "webui.addr": "后台监听地址", "webui.password": "后台登录密码",
	"database.max_open": "最大数据库连接数", "database.max_idle": "最大空闲连接数", "database.conn_max_lifetime_minutes": "连接最长存活时间（分钟）",
}

func (m *Manager) View() View {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.current.Clone()
	return View{AppliedRevision: m.appliedRevision, RestartPending: m.restartPending, Revision: m.revision, Sections: fields(c), Bootstrap: map[string]any{"database_dsn_configured": c.Database.DSN != "", "data_dir": c.Bot.DataDir, "note": "数据库连接地址与数据根目录由环境变量或最小启动文件提供；业务配置保存在数据库。"}}
}

func fields(c config.Config) []Section {
	groups := map[string][]Field{}
	var walk func(reflect.Value, string)
	walk = func(v reflect.Value, prefix string) {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			key := t.Field(i).Tag.Get("json")
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if legacyAIPaths[path] || path == "ai.vl" || path == "tools.tavily_key" {
				continue
			}
			value := v.Field(i)
			if value.Kind() == reflect.Struct {
				walk(value, path)
				continue
			}
			if path == "database.dsn" || path == "bot.data_dir" || path == "bot.owner_id" || strings.HasPrefix(path, "bilibili.") && path != "bilibili.cookie" {
				continue
			}
			kind := "text"
			switch value.Kind() {
			case reflect.Bool:
				kind = "boolean"
			case reflect.Int, reflect.Int64:
				kind = "number"
			case reflect.Slice:
				kind = "numbers"
			}
			if path == "image.entry" {
				kind = "image_entries"
			}
			f := Field{Path: path, Label: labels[path], Kind: kind, Value: value.Interface(), Secret: secrets[path]}
			if f.Label == "" {
				f.Label = path
			}
			if path == "ai.profiles" {
				f.Kind = "ai_profiles"
				f.Value = profileViews(c.AI.Profiles)
			}
			if path == "ai.connections" {
				f.Kind = "ai_connections"
				f.Value = connectionViews(c.AI.Connections)
			}
			if path == "ai.bindings" {
				f.Kind = "ai_bindings"
				if c.AI.Bindings == nil {
					f.Value = map[string]string{}
				}
			}
			if f.Secret {
				f.Configured = value.String() != ""
				f.Value = ""
			}
			f.Reload = requiresReload(path)
			groups[strings.Split(path, ".")[0]] = append(groups[strings.Split(path, ".")[0]], f)
		}
	}
	walk(reflect.ValueOf(c), "")
	result := []Section{}
	for _, section := range sectionOrder {
		result = append(result, Section{section, titles[section], groups[section]})
	}
	return result
}
