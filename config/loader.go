package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func defaults(bootstrap bool) Config {
	c := Config{
		Bot:      BotConfig{CommandArgSpaceRequired: true, DataDir: "data", Timezone: "Asia/Shanghai"},
		Database: DatabaseConfig{MaxOpen: 20, MaxIdle: 5, ConnMaxLifetime: 30},
		AI: AIConfig{
			MaxTokens: DefaultAIMaxTokens, ReplyMaxChars: DefaultAIReplyMaxChars,
			Context:  ContextConfig{ChatHistory: 15, Summary: 50},
			Affinity: AffinityConfig{Enabled: true, Initial: 50, BlockBelow: 10, Max: 100},
		},
		Image: ImageConfig{ConvertMinKB: 1024, ConvertQuality: 85},
		Pack:  PackConfig{MaxImages: 100, MaxMB: 100},
		WebUI: WebUIConfig{Addr: ":9080"},
		Feed:  FeedConfig{RSSHubBase: "https://rsshub.app"},
		Bilibili: BilibiliConfig{RecentCount: 5, PollSeconds: 60, ReplyMode: "off", ReplyCooldownSeconds: 60, ReplyPerMinute: 5,
			AIPrompt: "你是我的 B站私信助手。请用简洁、友好的中文回复对方。不了解的事项直接说明，不编造承诺、联系方式或链接。"},
	}
	if bootstrap {
		c.Bot.Name = "月灵"
		c.WebUI.Enabled = true
	}
	return c
}

func osLookupEnv(key string) (string, bool) { return os.LookupEnv(key) }

func readConfig(path string, bootstrap bool, lookup func(string) (string, bool)) (Config, error) {
	c := defaults(bootstrap)
	data, err := os.ReadFile(path)
	if err != nil && !(bootstrap && os.IsNotExist(err)) {
		return Config{}, fmt.Errorf("cannot read configuration: %w", err)
	}
	if err == nil {
		if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&c); err != nil {
			// Decoder errors can contain the offending value, including credentials.
			return Config{}, fmt.Errorf("invalid TOML configuration: check field names and types")
		}
	}
	if err := applyEnvironment(reflect.ValueOf(&c).Elem(), "YUELING", lookup); err != nil {
		return Config{}, err
	}
	return c, nil
}

func applyEnvironment(value reflect.Value, prefix string, lookup func(string) (string, bool)) error {
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		name := prefix + "_" + strings.ToUpper(value.Type().Field(i).Tag.Get("toml"))
		if field.Kind() == reflect.Struct {
			if err := applyEnvironment(field, name, lookup); err != nil {
				return err
			}
			continue
		}
		raw, found := lookup(name)
		if !found || raw == "" {
			continue
		}
		var err error
		switch field.Kind() {
		case reflect.String:
			field.SetString(raw)
		case reflect.Bool:
			var parsed bool
			parsed, err = strconv.ParseBool(raw)
			if err == nil {
				field.SetBool(parsed)
			}
		case reflect.Int, reflect.Int64:
			var parsed int64
			parsed, err = strconv.ParseInt(raw, 10, field.Type().Bits())
			if err == nil {
				field.SetInt(parsed)
			}
		case reflect.Slice, reflect.Map:
			err = json.Unmarshal([]byte(raw), field.Addr().Interface())
		default:
			err = fmt.Errorf("unsupported environment field")
		}
		if err != nil {
			return fmt.Errorf("invalid environment variable %s: expected %s", name, field.Type())
		}
	}
	return nil
}
