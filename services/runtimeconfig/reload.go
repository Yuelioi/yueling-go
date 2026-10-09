package runtimeconfig

import (
	"reflect"
	"strings"

	"github.com/Yuelioi/yueling-go/config"
)

var restartPaths = []string{"napcat", "http_api", "webui", "tools.meme_server", "bot.timezone", "image.entry", "bilibili.cookie"}

func requiresReload(path string) bool {
	for _, root := range restartPaths {
		if path == root || strings.HasPrefix(path, root+".") {
			return true
		}
	}
	return false
}

func NeedsReload(a, b config.Config) bool {
	return changedRestartSetting(reflect.ValueOf(a), reflect.ValueOf(b), "")
}

func changedRestartSetting(a, b reflect.Value, prefix string) bool {
	for i := 0; i < a.NumField(); i++ {
		path := a.Type().Field(i).Tag.Get("json")
		if prefix != "" {
			path = prefix + "." + path
		}
		if requiresReload(path) {
			if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
				return true
			}
		} else if a.Field(i).Kind() == reflect.Struct && changedRestartSetting(a.Field(i), b.Field(i), path) {
			return true
		}
	}
	return false
}

// hotSnapshot retains startup-owned fields until the next process initializes.
func hotSnapshot(active, saved config.Config) config.Config {
	next := saved.Clone()
	old := active.Clone()
	var retain func(reflect.Value, reflect.Value, string)
	retain = func(dst, src reflect.Value, prefix string) {
		for i := 0; i < dst.NumField(); i++ {
			path := dst.Type().Field(i).Tag.Get("json")
			if prefix != "" {
				path = prefix + "." + path
			}
			if requiresReload(path) {
				dst.Field(i).Set(src.Field(i))
			} else if dst.Field(i).Kind() == reflect.Struct {
				retain(dst.Field(i), src.Field(i), path)
			}
		}
	}
	retain(reflect.ValueOf(&next).Elem(), reflect.ValueOf(old), "")
	return next
}
