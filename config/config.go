package config

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Bot      BotConfig      `mapstructure:"bot" json:"bot"`
	Database DatabaseConfig `mapstructure:"database" json:"database"`
	NapCat   NapCatConfig   `mapstructure:"napcat" json:"napcat"`
	AI       AIConfig       `mapstructure:"ai" json:"ai"`
	Tools    ToolsConfig    `mapstructure:"tools" json:"tools"`
	HTTPAPI  HTTPAPIConfig  `mapstructure:"http_api" json:"http_api"`
	WebUI    WebUIConfig    `mapstructure:"webui" json:"webui"`
	Feed     FeedConfig     `mapstructure:"feed" json:"feed"`
	Image    ImageConfig    `mapstructure:"image" json:"image"`
	Pack     PackConfig     `mapstructure:"pack" json:"pack"`
	Bilibili BilibiliConfig `mapstructure:"bilibili" json:"bilibili"`
	Plugins  PluginsConfig  `mapstructure:"plugins" json:"plugins"`
}

type PluginsConfig struct {
	Disabled []int `mapstructure:"disabled" json:"disabled"`
}

// DatabaseConfig configures the single PostgreSQL database used by the bot.
// YUELING_DATABASE_DSN can override DSN in container deployments.
type DatabaseConfig struct {
	DSN             string `mapstructure:"dsn" json:"dsn"`
	MaxOpen         int    `mapstructure:"max_open" json:"max_open"`
	MaxIdle         int    `mapstructure:"max_idle" json:"max_idle"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_minutes" json:"conn_max_lifetime_minutes"`
}

// ImageConfig controls JPEG conversion of images saved by the 添加* commands.
// Convert off = store originals as-is.
type ImageConfig struct {
	Convert        bool         `mapstructure:"convert" json:"convert"`                 // 入库时转 JPEG 总开关
	ConvertMinKB   int          `mapstructure:"convert_min_kb" json:"convert_min_kb"`   // 仅 >= 此大小(KB)才转，0=全部转
	ConvertQuality int          `mapstructure:"convert_quality" json:"convert_quality"` // JPEG 质量 1-100
	Entry          []ImageEntry `mapstructure:"entry" json:"entry"`                     // [[image.entry]] 图片类目配置表；空则用插件内置默认表
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
	Folder string   `mapstructure:"folder" json:"folder"` // 素材子目录；external 可空
	Call   []string `mapstructure:"call" json:"call"`     // 调用命令（FullMatch）
	Add    string   `mapstructure:"add" json:"add"`       // 添加命令（OnCommand）；external/无添加可空
	Kind   Kind     `mapstructure:"kind" json:"kind"`     // 调用方式，缺省视为 single
	Arg    *bool    `mapstructure:"arg" json:"arg"`       // 添加规则：true=关键词+图(存 名字_hash) / false=直接加图(hash)；省略时 grid→true 其余→false
	URL    string   `mapstructure:"url" json:"url"`       // 仅 external：请求地址
	Pick   string   `mapstructure:"pick" json:"pick"`     // 仅 external：JSON 取图路径；空=响应本身就是图
	Base   string   `mapstructure:"base" json:"base"`     // 仅 external：pick 取到相对路径(/...)时拼在前面的站点前缀
}

// PackConfig controls the pack command's batch limits.
type PackConfig struct {
	MaxImages int `mapstructure:"max_images" json:"max_images"` // 单次最多打包图片数
	MaxMB     int `mapstructure:"max_mb" json:"max_mb"`         // 单次累计下载上限(MB)
}

type BotConfig struct {
	Name                    string  `mapstructure:"name" json:"name"`
	OwnerID                 int64   `mapstructure:"owner_id" json:"owner_id"` // legacy: merged into SuperUsers during validation
	SuperUsers              []int64 `mapstructure:"superusers" json:"superusers"`
	CmdPrefix               string  `mapstructure:"cmd_prefix" json:"cmd_prefix"`
	CommandArgSpaceRequired bool    `mapstructure:"command_arg_space_required" json:"command_arg_space_required"`
	DataDir                 string  `mapstructure:"data_dir" json:"data_dir"`
	Timezone                string  `mapstructure:"timezone" json:"timezone"`
}

// NapCatConfig holds the connection parameters for NapCat WebSocket.
// Set URL for forward WS (bot connects to NapCat) or Serve for reverse WS (NapCat connects to bot).
type NapCatConfig struct {
	URL   string `mapstructure:"url" json:"url"`     // forward WS: ws://host:port/onebot/v11/ws
	Serve string `mapstructure:"serve" json:"serve"` // reverse WS: :9078 (listen addr)
	Token string `mapstructure:"token" json:"token"`
}

type AIConfig struct {
	Profiles            []AIProfile     `mapstructure:"profiles" json:"profiles"`
	ConversationProfile string          `mapstructure:"conversation_profile" json:"conversation_profile"`
	VisionProfile       string          `mapstructure:"vision_profile" json:"vision_profile"`
	DeepSeekKey         string          `mapstructure:"deepseek_key" json:"deepseek_key"`
	BaseURL             string          `mapstructure:"base_url" json:"base_url"`
	Model               string          `mapstructure:"model" json:"model"`
	ReasoningEffort     string          `mapstructure:"reasoning_effort" json:"reasoning_effort"`
	MaxTokens           int             `mapstructure:"max_tokens" json:"max_tokens"`
	ReplyMaxChars       int             `mapstructure:"reply_max_chars" json:"reply_max_chars"`
	VL                  VLConfig        `mapstructure:"vl" json:"vl"`
	RateLimit           RateLimitConfig `mapstructure:"ratelimit" json:"ratelimit"`
	Context             ContextConfig   `mapstructure:"context" json:"context"`
	Affinity            AffinityConfig  `mapstructure:"affinity" json:"affinity"`
}

const (
	DefaultAIMaxTokens     = 4096
	DefaultAIReplyMaxChars = 200
)

type AffinityConfig struct {
	Enabled    bool `mapstructure:"enabled" json:"enabled"`
	Initial    int  `mapstructure:"initial" json:"initial"`
	BlockBelow int  `mapstructure:"block_below" json:"block_below"`
	Min        int  `mapstructure:"min" json:"min"`
	Max        int  `mapstructure:"max" json:"max"`
}

// ContextConfig sets the default message-count context tools fetch when the
// model doesn't specify one. Hard caps stay in the tools (chat_history ≤ 30,
// summary ≤ 100); these only choose the default.
type ContextConfig struct {
	ChatHistory int `mapstructure:"chat_history" json:"chat_history"` // get_chat_history 默认条数
	Summary     int `mapstructure:"summary" json:"summary"`           // summarize_chat 默认条数
}

// RateLimitConfig caps user-triggered AI calls per minute. 0 = unlimited.
type RateLimitConfig struct {
	UserPerMin  int `mapstructure:"user_per_min" json:"user_per_min"`
	GroupPerMin int `mapstructure:"group_per_min" json:"group_per_min"`
}

type VLConfig struct {
	Key     string `mapstructure:"key" json:"key"`
	BaseURL string `mapstructure:"base_url" json:"base_url"`
	Model   string `mapstructure:"model" json:"model"`
}

type ToolsConfig struct {
	QWeatherKey  string `mapstructure:"qweather_key" json:"qweather_key"`
	QWeatherHost string `mapstructure:"qweather_host" json:"qweather_host"`
	TavilyKey    string `mapstructure:"tavily_key" json:"tavily_key"`
	Proxy        string `mapstructure:"proxy" json:"proxy"`             // e.g. http://127.0.0.1:7890
	MemeServer   string `mapstructure:"meme_server" json:"meme_server"` // e.g. http://127.0.0.1:2233
}

// HTTPAPIConfig configures the external send-message HTTP API.
// Addr empty = disabled. When Addr is set, Key is required.
type HTTPAPIConfig struct {
	Addr string `mapstructure:"addr" json:"addr"`
	Key  string `mapstructure:"key" json:"key"`
}

type WebUIConfig struct {
	Enabled  bool   `mapstructure:"enabled" json:"enabled"`
	Addr     string `mapstructure:"addr" json:"addr"`
	Password string `mapstructure:"password" json:"password"`
}

// FeedConfig controls platform shortcut subscriptions backed by RSSHub.
type FeedConfig struct {
	RSSHubBase string `mapstructure:"rsshub_base" json:"rsshub_base"`
}

var C Config

var configMu sync.RWMutex

func Current() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	c := C
	c.AI = c.AI.Resolved()
	return c
}

func Publish(c Config) {
	configMu.Lock()
	defer configMu.Unlock()
	c.AI = c.AI.Resolved()
	C = c
}

func PluginEnabled(id int) bool { return !slices.Contains(Current().Plugins.Disabled, id) }

func Load(path string) error {
	return load(path, false)
}

func LoadBootstrap(path string) error {
	return load(path, true)
}

func bindFields(t reflect.Type, prefix string) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key := f.Tag.Get("mapstructure")
		if prefix != "" {
			key = prefix + "." + key
		}
		if f.Type.Kind() == reflect.Struct {
			bindFields(f.Type, key)
		} else {
			_ = viper.BindEnv(key)
		}
	}
}

func load(path string, bootstrap bool) error {
	viper.Reset()
	viper.SetConfigFile(path)
	viper.SetConfigType("toml")
	viper.SetEnvPrefix("YUELING")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	bindFields(reflect.TypeOf(Config{}), "")
	viper.SetDefault("bot.command_arg_space_required", true)
	viper.SetDefault("bot.data_dir", "data")
	viper.SetDefault("bot.timezone", "Asia/Shanghai")
	viper.SetDefault("database.dsn", "")
	viper.SetDefault("database.max_open", 20)
	viper.SetDefault("database.max_idle", 5)
	viper.SetDefault("database.conn_max_lifetime_minutes", 30)
	viper.SetDefault("ai.model", "deepseek-chat")
	viper.SetDefault("ai.base_url", "https://api.deepseek.com/v1")
	viper.SetDefault("ai.max_tokens", DefaultAIMaxTokens)
	viper.SetDefault("ai.reasoning_effort", "")
	viper.SetDefault("ai.reply_max_chars", DefaultAIReplyMaxChars)
	viper.SetDefault("ai.vl.base_url", "https://api.siliconflow.cn/v1")
	viper.SetDefault("ai.vl.model", "Qwen/Qwen2.5-VL-72B-Instruct")
	viper.SetDefault("ai.context.chat_history", 15)
	viper.SetDefault("ai.context.summary", 50)
	viper.SetDefault("ai.affinity.enabled", true)
	viper.SetDefault("ai.affinity.initial", 50)
	viper.SetDefault("ai.affinity.block_below", 10)
	viper.SetDefault("ai.affinity.min", 0)
	viper.SetDefault("ai.affinity.max", 100)
	viper.SetDefault("image.convert_min_kb", 1024)
	viper.SetDefault("image.convert_quality", 85)
	viper.SetDefault("pack.max_images", 100)
	viper.SetDefault("pack.max_mb", 100)
	viper.SetDefault("webui.addr", ":9080")
	viper.SetDefault("feed.rsshub_base", "https://rsshub.app")
	setBilibiliDefaults()
	if bootstrap {
		viper.SetDefault("bot.name", "月灵")
		viper.SetDefault("webui.enabled", true)
	}
	if err := viper.ReadInConfig(); err != nil {
		if !bootstrap || !os.IsNotExist(err) {
			return err
		}
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}
	if bootstrap {
		if cfg.NapCat.URL == "" && cfg.NapCat.Serve == "" {
			cfg.NapCat.Serve = ":9077"
		}
		if cfg.Bot.OwnerID != 0 && !slices.Contains(cfg.Bot.SuperUsers, cfg.Bot.OwnerID) {
			cfg.Bot.SuperUsers = append(cfg.Bot.SuperUsers, cfg.Bot.OwnerID)
		}
		if err := cfg.ValidateDatabase(); err != nil {
			return err
		}
	} else {
		if err := cfg.validate(); err != nil {
			return err
		}
	}
	Publish(cfg)
	return nil
}

func (c *Config) validate() error {
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
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	v.SetEnvPrefix("YUELING")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetDefault("ai.deepseek_key", "")
	v.SetDefault("ai.model", "deepseek-chat")
	v.SetDefault("ai.base_url", "https://api.deepseek.com/v1")
	v.SetDefault("ai.max_tokens", DefaultAIMaxTokens)
	v.SetDefault("ai.reply_max_chars", DefaultAIReplyMaxChars)
	v.SetDefault("ai.reasoning_effort", "")
	if err := v.ReadInConfig(); err != nil {
		return AIConfig{}, fmt.Errorf("cannot read AI configuration")
	}
	var cfg struct {
		AI AIConfig `mapstructure:"ai"`
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return AIConfig{}, fmt.Errorf("cannot parse AI configuration")
	}
	if strings.TrimSpace(cfg.AI.DeepSeekKey) == "" || strings.TrimSpace(cfg.AI.Model) == "" {
		return AIConfig{}, fmt.Errorf("AI key and model are required")
	}
	return cfg.AI, nil
}
