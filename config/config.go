package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
)

type Config struct {
	Bot      BotConfig      `toml:"bot" json:"bot"`
	Database DatabaseConfig `toml:"database" json:"database"`
	NapCat   NapCatConfig   `toml:"napcat" json:"napcat"`
	AI       AIConfig       `toml:"ai" json:"ai"`
	Tools    ToolsConfig    `toml:"tools" json:"tools"`
	HTTPAPI  HTTPAPIConfig  `toml:"http_api" json:"http_api"`
	WebUI    WebUIConfig    `toml:"webui" json:"webui"`
	Feed     FeedConfig     `toml:"feed" json:"feed"`
	Image    ImageConfig    `toml:"image" json:"image"`
	Pack     PackConfig     `toml:"pack" json:"pack"`
	Bilibili BilibiliConfig `toml:"bilibili" json:"bilibili"`
	Plugins  PluginsConfig  `toml:"plugins" json:"plugins"`
}

type PluginsConfig struct {
	Disabled []int `toml:"disabled" json:"disabled"`
}

// DatabaseConfig configures the single PostgreSQL database used by the bot.
// YUELING_DATABASE_DSN can override DSN in container deployments.
type DatabaseConfig struct {
	DSN             string `toml:"dsn" json:"dsn"`
	MaxOpen         int    `toml:"max_open" json:"max_open"`
	MaxIdle         int    `toml:"max_idle" json:"max_idle"`
	ConnMaxLifetime int    `toml:"conn_max_lifetime_minutes" json:"conn_max_lifetime_minutes"`
}

// ImageConfig controls JPEG conversion of images saved by the 添加* commands.
// Convert off = store originals as-is.
type ImageConfig struct {
	Convert        bool         `toml:"convert" json:"convert"`                 // 入库时转 JPEG 总开关
	ConvertMinKB   int          `toml:"convert_min_kb" json:"convert_min_kb"`   // 仅 >= 此大小(KB)才转，0=全部转
	ConvertQuality int          `toml:"convert_quality" json:"convert_quality"` // JPEG 质量 1-100
	Entry          []ImageEntry `toml:"entry" json:"entry"`                     // [[image.entry]] 图片类目配置表；空则用插件内置默认表
}

// Kind 图片类目行为：随机一张 / 4合1网格 / 外链。
type Kind string

const (
	KindSingle   Kind = "single"
	KindGrid     Kind = "grid"
	KindExternal Kind = "external"
)

// ImageEntry 一条图片类目配置。kind 管调用方式，arg 管添加规则。
type ImageEntry struct {
	Folder string   `toml:"folder" json:"folder"` // 素材子目录；external 可空
	Call   []string `toml:"call" json:"call"`     // 调用命令（FullMatch）
	Add    string   `toml:"add" json:"add"`       // 添加命令（OnCommand）；external/无添加可空
	Kind   Kind     `toml:"kind" json:"kind"`     // 调用方式，缺省视为 single
	Arg    *bool    `toml:"arg" json:"arg"`       // 添加规则：true=关键词+图(存 名字_hash) / false=直接加图(hash)；省略时 grid→true 其余→false
	URL    string   `toml:"url" json:"url"`       // 仅 external：请求地址
	Pick   string   `toml:"pick" json:"pick"`     // 仅 external：JSON 取图路径；空=响应本身就是图
	Base   string   `toml:"base" json:"base"`     // 仅 external：pick 取到相对路径(/...)时拼在前面的站点前缀
}

// PackConfig controls the pack command's batch limits.
type PackConfig struct {
	MaxImages int `toml:"max_images" json:"max_images"` // 单次最多打包图片数
	MaxMB     int `toml:"max_mb" json:"max_mb"`         // 单次累计下载上限(MB)
}

type BotConfig struct {
	Name                    string  `toml:"name" json:"name"`
	OwnerID                 int64   `toml:"owner_id" json:"owner_id"` // legacy: merged into SuperUsers during validation
	SuperUsers              []int64 `toml:"superusers" json:"superusers"`
	Blacklist               []int64 `toml:"blacklist" json:"blacklist"`
	CmdPrefix               string  `toml:"cmd_prefix" json:"cmd_prefix"`
	CommandArgSpaceRequired bool    `toml:"command_arg_space_required" json:"command_arg_space_required"`
	DataDir                 string  `toml:"data_dir" json:"data_dir"`
	Timezone                string  `toml:"timezone" json:"timezone"`
}

// NapCatConfig holds the connection parameters for NapCat WebSocket.
// Set URL for forward WS (bot connects to NapCat) or Serve for reverse WS (NapCat connects to bot).
type NapCatConfig struct {
	URL   string `toml:"url" json:"url"`     // forward WS: ws://host:port/onebot/v11/ws
	Serve string `toml:"serve" json:"serve"` // reverse WS: :9078 (listen addr)
	Token string `toml:"token" json:"token"`
}

type AIConfig struct {
	Profiles            []AIProfile     `toml:"profiles" json:"profiles"`
	ConversationProfile string          `toml:"conversation_profile" json:"conversation_profile"`
	VisionProfile       string          `toml:"vision_profile" json:"vision_profile"`
	DeepSeekKey         string          `toml:"deepseek_key" json:"deepseek_key"`
	BaseURL             string          `toml:"base_url" json:"base_url"`
	Model               string          `toml:"model" json:"model"`
	ReasoningEffort     string          `toml:"reasoning_effort" json:"reasoning_effort"`
	MaxTokens           int             `toml:"max_tokens" json:"max_tokens"`
	ReplyMaxChars       int             `toml:"reply_max_chars" json:"reply_max_chars"`
	VL                  VLConfig        `toml:"vl" json:"vl"`
	RateLimit           RateLimitConfig `toml:"ratelimit" json:"ratelimit"`
	Context             ContextConfig   `toml:"context" json:"context"`
	Affinity            AffinityConfig  `toml:"affinity" json:"affinity"`
}

const (
	DefaultAIMaxTokens     = 4096
	DefaultAIReplyMaxChars = 200
)

type AffinityConfig struct {
	Enabled    bool `toml:"enabled" json:"enabled"`
	Initial    int  `toml:"initial" json:"initial"`
	BlockBelow int  `toml:"block_below" json:"block_below"`
	Min        int  `toml:"min" json:"min"`
	Max        int  `toml:"max" json:"max"`
}

// ContextConfig sets the default message-count context tools fetch when the
// model doesn't specify one. Hard caps stay in the tools (chat_history ≤ 30,
// summary ≤ 100); these only choose the default.
type ContextConfig struct {
	ChatHistory int `toml:"chat_history" json:"chat_history"` // get_chat_history 默认条数
	Summary     int `toml:"summary" json:"summary"`           // summarize_chat 默认条数
}

// RateLimitConfig caps user-triggered AI calls per minute. 0 = unlimited.
type RateLimitConfig struct {
	UserPerMin  int `toml:"user_per_min" json:"user_per_min"`
	GroupPerMin int `toml:"group_per_min" json:"group_per_min"`
}

type VLConfig struct {
	Key     string `toml:"key" json:"key"`
	BaseURL string `toml:"base_url" json:"base_url"`
	Model   string `toml:"model" json:"model"`
}

type ToolsConfig struct {
	QWeatherKey  string `toml:"qweather_key" json:"qweather_key"`
	QWeatherHost string `toml:"qweather_host" json:"qweather_host"`
	TavilyKey    string `toml:"tavily_key" json:"tavily_key"`
	Proxy        string `toml:"proxy" json:"proxy"`             // e.g. http://127.0.0.1:7890
	MemeServer   string `toml:"meme_server" json:"meme_server"` // e.g. http://127.0.0.1:2233
}

// HTTPAPIConfig configures the external send-message HTTP API.
// Addr empty = disabled. When Addr is set, Key is required.
type HTTPAPIConfig struct {
	Addr string `toml:"addr" json:"addr"`
	Key  string `toml:"key" json:"key"`
}

type WebUIConfig struct {
	Enabled  bool   `toml:"enabled" json:"enabled"`
	Addr     string `toml:"addr" json:"addr"`
	Password string `toml:"password" json:"password"`
}

// FeedConfig controls platform shortcut subscriptions backed by RSSHub.
type FeedConfig struct {
	RSSHubBase string `toml:"rsshub_base" json:"rsshub_base"`
}

var C Config

var configMu sync.RWMutex

func Current() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	c := C.Clone()
	c.AI = c.AI.Resolved()
	return c
}

func Publish(c Config) {
	configMu.Lock()
	defer configMu.Unlock()
	c.AI = c.AI.Resolved()
	C = c.Clone()
}

func UserBlocked(id int64) bool { return id > 0 && slices.Contains(Current().Bot.Blacklist, id) }

func PluginEnabled(id int) bool { return !slices.Contains(Current().Plugins.Disabled, id) }

func Load(path string) error {
	return load(path, false)
}

func LoadBootstrap(path string) error {
	return load(path, true)
}

func load(path string, bootstrap bool) error {
	cfg, err := readConfig(path, bootstrap, osLookupEnv)
	if err != nil {
		return err
	}
	if bootstrap {
		if cfg.NapCat.URL == "" && cfg.NapCat.Serve == "" {
			cfg.NapCat.Serve = ":9077"
		}
		if cfg.Bot.OwnerID != 0 && !slices.Contains(cfg.Bot.SuperUsers, cfg.Bot.OwnerID) {
			cfg.Bot.SuperUsers = append(cfg.Bot.SuperUsers, cfg.Bot.OwnerID)
		}
		err = cfg.ValidateDatabase()
	} else {
		err = cfg.validate()
	}
	if err != nil {
		return err
	}
	Publish(cfg)
	return nil
}

func (c *Config) validate() error {
	for _, id := range c.Bot.Blacklist {
		if id <= 0 {
			return fmt.Errorf("黑名单 QQ 必须为正整数")
		}
	}
	if c.Bot.DataDir == "" {
		c.Bot.DataDir = "data"
	}
	if c.Bot.OwnerID != 0 && !slices.Contains(c.Bot.SuperUsers, c.Bot.OwnerID) {
		c.Bot.SuperUsers = append(c.Bot.SuperUsers, c.Bot.OwnerID)
	}
	if err := c.ValidateDatabase(); err != nil {
		return err
	}
	if c.NapCat.URL == "" && c.NapCat.Serve == "" {
		return fmt.Errorf("napcat.url or napcat.serve is required")
	}
	if c.HTTPAPI.Addr != "" && c.HTTPAPI.Key == "" {
		return fmt.Errorf("http_api.key is required when http_api.addr is set")
	}
	if c.WebUI.Enabled {
		if c.WebUI.Addr == "" {
			c.WebUI.Addr = ":9080"
		}
		if c.WebUI.Password == "" {
			return fmt.Errorf("webui.password is required when webui.enabled is true")
		}
	}
	feedBase, err := url.Parse(strings.TrimSpace(c.Feed.RSSHubBase))
	if err != nil || feedBase.Hostname() == "" || (feedBase.Scheme != "http" && feedBase.Scheme != "https") || feedBase.User != nil {
		return fmt.Errorf("feed.rsshub_base must be a valid http/https origin")
	}
	return c.Bilibili.Validate()
}

func (c Config) Validate() error { return c.validate() }

func (c Config) ValidateDatabase() error {
	databaseURL, err := url.Parse(strings.TrimSpace(c.Database.DSN))
	if err != nil || databaseURL.Hostname() == "" || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") {
		return fmt.Errorf("database.dsn must be a valid postgres/postgresql URL")
	}
	if c.Database.MaxOpen <= 0 {
		return fmt.Errorf("database.max_open must be greater than zero")
	}
	if c.Database.MaxIdle < 0 || c.Database.MaxIdle > c.Database.MaxOpen {
		return fmt.Errorf("database.max_idle must be between zero and database.max_open")
	}
	if c.Database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("database.conn_max_lifetime_minutes must be greater than zero")
	}
	return nil
}

// LoadAI reads only model configuration for diagnostics, without requiring a
// database or a QQ connection. It does not change the running global config.
func LoadAI(path string) (AIConfig, error) {
	cfg, err := readConfig(path, false, osLookupEnv)
	if err != nil {
		return AIConfig{}, err
	}
	ai := cfg.AI.Resolved()
	if strings.TrimSpace(ai.DeepSeekKey) == "" || strings.TrimSpace(ai.Model) == "" {
		return AIConfig{}, fmt.Errorf("AI key and model are required")
	}
	return ai, nil
}
