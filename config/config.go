package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Bot    BotConfig    `mapstructure:"bot"`
	NapCat NapCatConfig `mapstructure:"napcat"`
	AI     AIConfig     `mapstructure:"ai"`
	Tools  ToolsConfig  `mapstructure:"tools"`
}

type BotConfig struct {
	Name         string   `mapstructure:"name"`
	OwnerID      int64    `mapstructure:"owner_id"`
	SuperUsers   []int64  `mapstructure:"superusers"`
	JoinKeywords []string `mapstructure:"join_keywords"`
	CmdPrefix    string   `mapstructure:"cmd_prefix"`
	DataDir      string   `mapstructure:"data_dir"`
	Timezone     string   `mapstructure:"timezone"`
}

// NapCatConfig holds the connection parameters for NapCat WebSocket.
// Set URL for forward WS (bot connects to NapCat) or Serve for reverse WS (NapCat connects to bot).
type NapCatConfig struct {
	URL   string `mapstructure:"url"`   // forward WS: ws://host:port/onebot/v11/ws
	Serve string `mapstructure:"serve"` // reverse WS: :9078 (listen addr)
	Token string `mapstructure:"token"`
}

type AIConfig struct {
	DeepSeekKey string   `mapstructure:"deepseek_key"`
	BaseURL     string   `mapstructure:"base_url"`
	Model       string   `mapstructure:"model"`
	VL          VLConfig `mapstructure:"vl"`
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
	return nil
}
