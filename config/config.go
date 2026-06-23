package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Bot     BotConfig     `mapstructure:"bot"`
	NapCat  NapCatConfig  `mapstructure:"napcat"`
	AI      AIConfig      `mapstructure:"ai"`
	Tools   ToolsConfig   `mapstructure:"tools"`
	HTTPAPI HTTPAPIConfig `mapstructure:"http_api"`
	Image   ImageConfig   `mapstructure:"image"`
	Pack    PackConfig    `mapstructure:"pack"`
}

// ImageConfig controls JPEG conversion of images saved by the 添加* commands.
// Convert off = store originals as-is.
type ImageConfig struct {
	Convert        bool         `mapstructure:"convert"`         // 入库时转 JPEG 总开关
	ConvertMinKB   int          `mapstructure:"convert_min_kb"`  // 仅 >= 此大小(KB)才转，0=全部转
	ConvertQuality int          `mapstructure:"convert_quality"` // JPEG 质量 1-100
	Entry          []ImageEntry `mapstructure:"entry"`           // [[image.entry]] 图片类目配置表；空则用插件内置默认表
}

// Kind 图片类目行为：随机一张 / 4合1网格 / 外链。
type Kind string

const (
	KindSingle   Kind = "single"
	KindGrid     Kind = "grid"
	KindExternal Kind = "external"
)

// ImageEntry 一条图片类目配置。kind 隐含带不带参与文件名策略。
type ImageEntry struct {
	Folder string   `mapstructure:"folder"` // 素材子目录；external 可空
	Call   []string `mapstructure:"call"`   // 调用命令（FullMatch）
	Add    string   `mapstructure:"add"`    // 添加命令（OnCommand）；external/无添加可空
	Kind   Kind     `mapstructure:"kind"`   // 缺省视为 single
	URL    string   `mapstructure:"url"`    // 仅 external：请求地址
	Pick   string   `mapstructure:"pick"`   // 仅 external：JSON 取图路径；空=响应本身就是图
}

// PackConfig controls the pack command's batch limits.
type PackConfig struct {
	MaxImages int `mapstructure:"max_images"` // 单次最多打包图片数
	MaxMB     int `mapstructure:"max_mb"`     // 单次累计下载上限(MB)
}

type BotConfig struct {
	Name       string  `mapstructure:"name"`
	OwnerID    int64   `mapstructure:"owner_id"`
	SuperUsers []int64 `mapstructure:"superusers"`
	CmdPrefix  string  `mapstructure:"cmd_prefix"`
	DataDir    string  `mapstructure:"data_dir"`
	Timezone   string  `mapstructure:"timezone"`
}

// NapCatConfig holds the connection parameters for NapCat WebSocket.
// Set URL for forward WS (bot connects to NapCat) or Serve for reverse WS (NapCat connects to bot).
type NapCatConfig struct {
	URL   string `mapstructure:"url"`   // forward WS: ws://host:port/onebot/v11/ws
	Serve string `mapstructure:"serve"` // reverse WS: :9078 (listen addr)
	Token string `mapstructure:"token"`
}

type AIConfig struct {
	DeepSeekKey string          `mapstructure:"deepseek_key"`
	BaseURL     string          `mapstructure:"base_url"`
	Model       string          `mapstructure:"model"`
	VL          VLConfig        `mapstructure:"vl"`
	RateLimit   RateLimitConfig `mapstructure:"ratelimit"`
	Context     ContextConfig   `mapstructure:"context"`
}

// ContextConfig sets the default message-count context tools fetch when the
// model doesn't specify one. Hard caps stay in the tools (chat_history ≤ 30,
// summary ≤ 100); these only choose the default.
type ContextConfig struct {
	ChatHistory int `mapstructure:"chat_history"` // get_chat_history 默认条数
	Summary     int `mapstructure:"summary"`      // summarize_chat 默认条数
}

// RateLimitConfig caps user-triggered AI calls per minute. 0 = unlimited.
type RateLimitConfig struct {
	UserPerMin  int `mapstructure:"user_per_min"`
	GroupPerMin int `mapstructure:"group_per_min"`
}

type VLConfig struct {
	Key     string `mapstructure:"key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type ToolsConfig struct {
	QWeatherKey  string `mapstructure:"qweather_key"`
	QWeatherHost string `mapstructure:"qweather_host"`
	TavilyKey    string `mapstructure:"tavily_key"`
	Proxy        string `mapstructure:"proxy"`       // e.g. http://127.0.0.1:7890
	MemeServer   string `mapstructure:"meme_server"` // e.g. http://127.0.0.1:2233
}

// HTTPAPIConfig configures the external send-message HTTP API.
// Addr empty = disabled. When Addr is set, Key is required.
type HTTPAPIConfig struct {
	Addr string `mapstructure:"addr"`
	Key  string `mapstructure:"key"`
}

var C Config

func Load(path string) error {
	viper.SetConfigFile(path)
	viper.SetConfigType("toml")
	viper.SetDefault("bot.data_dir", "data")
	viper.SetDefault("bot.timezone", "Asia/Shanghai")
	viper.SetDefault("ai.model", "deepseek-chat")
	viper.SetDefault("ai.base_url", "https://api.deepseek.com/v1")
	viper.SetDefault("ai.vl.base_url", "https://api.siliconflow.cn/v1")
	viper.SetDefault("ai.vl.model", "Qwen/Qwen2.5-VL-72B-Instruct")
	viper.SetDefault("ai.context.chat_history", 15)
	viper.SetDefault("ai.context.summary", 50)
	viper.SetDefault("image.convert_min_kb", 1024)
	viper.SetDefault("image.convert_quality", 85)
	viper.SetDefault("pack.max_images", 100)
	viper.SetDefault("pack.max_mb", 100)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	if err := viper.Unmarshal(&C); err != nil {
		return err
	}
	return C.validate()
}

func (c *Config) validate() error {
	if c.AI.DeepSeekKey == "" {
		return fmt.Errorf("ai.deepseek_key is required")
	}
	if c.Bot.DataDir == "" {
		c.Bot.DataDir = "data"
	}
	if c.NapCat.URL == "" && c.NapCat.Serve == "" {
		return fmt.Errorf("napcat.url or napcat.serve is required")
	}
	if c.HTTPAPI.Addr != "" && c.HTTPAPI.Key == "" {
		return fmt.Errorf("http_api.key is required when http_api.addr is set")
	}
	return nil
}
