package image

import (
	"fmt"

	"github.com/Yuelioi/yueling-go/config"
)

// defaultEntries 现网不配 [[image.entry]] 时使用，逐条照搬重构前行为。
var defaultEntries = []config.ImageEntry{
	{Folder: "龙图", Call: []string{"龙图", "龙图攻击"}, Add: "添加龙图"},
	{Folder: "福瑞", Call: []string{"福瑞", "来点福瑞"}, Add: "添加福瑞"},
	{Folder: "老公", Call: []string{"我老公呢", "老公"}, Add: "添加老公"},
	{Folder: "老婆", Call: []string{"我老婆呢", "老婆"}, Add: "添加老婆"},
	{Folder: "沙雕图", Call: []string{"沙雕图"}, Add: "添加沙雕图"},
	{Folder: "杂鱼", Call: []string{"杂鱼"}, Add: "添加杂鱼"},
	{Folder: "美少女", Call: []string{"美少女"}, Add: "添加美少女"},
	{Folder: "ba", Call: []string{"ba", "来点ba"}, Add: "添加ba"},
	{Folder: "吃的", Call: []string{"随机吃的", "吃啥", "吃什么", "来点吃的"}, Add: "添加吃的", Kind: config.KindGrid},
	{Folder: "喝的", Call: []string{"随机喝的", "喝啥", "喝什么", "来点喝的"}, Add: "添加喝的", Kind: config.KindGrid},
	{Folder: "玩的", Call: []string{"随机玩的", "玩啥", "玩什么", "来点玩的"}, Add: "添加玩的", Kind: config.KindGrid},
	{Folder: "水果", Call: []string{"随机水果", "来点水果"}, Add: "添加水果", Kind: config.KindGrid},
	{Folder: "猫猫", Call: []string{"随机猫猫", "来点猫猫"}, Kind: config.KindExternal, URL: "http://edgecats.net/"},
}

func kindOf(e config.ImageEntry) config.Kind {
	if e.Kind == "" {
		return config.KindSingle
	}
	return e.Kind
}

// argRequired 解析「添加是否需要关键词」：显式 arg 优先，省略时 grid→true、其余→false。
func argRequired(e config.ImageEntry) bool {
	if e.Arg != nil {
		return *e.Arg
	}
	return kindOf(e) == config.KindGrid
}

// validateEntries 启动时校验配置表，非法即返回错误（fail-fast）。
func validateEntries(entries []config.ImageEntry) error {
	seen := map[string]bool{}
	mark := func(cmd string) error {
		if cmd == "" {
			return nil
		}
		if seen[cmd] {
			return fmt.Errorf("命令重复: %q", cmd)
		}
		seen[cmd] = true
		return nil
	}
	for i, e := range entries {
		switch kindOf(e) {
		case config.KindSingle, config.KindGrid:
			if e.Folder == "" {
				return fmt.Errorf("entry[%d] %s 缺少 folder", i, kindOf(e))
			}
			if len(e.Call) == 0 {
				return fmt.Errorf("entry[%d] %s 缺少 call", i, kindOf(e))
			}
			if kindOf(e) == config.KindGrid {
				if e.Add == "" {
					return fmt.Errorf("entry[%d] grid 缺少 add", i)
				}
				if e.Arg != nil && !*e.Arg {
					return fmt.Errorf("entry[%d] grid 不能 arg=false（4合1 需名字标签）", i)
				}
			}
		case config.KindExternal:
			if e.URL == "" {
				return fmt.Errorf("entry[%d] external 缺少 url", i)
			}
		default:
			return fmt.Errorf("entry[%d] 非法 kind: %q", i, e.Kind)
		}
		for _, c := range e.Call {
			if err := mark(c); err != nil {
				return err
			}
		}
		if err := mark(e.Add); err != nil {
			return err
		}
	}
	return nil
}

func nameByHash(hash, _ string, _ int64) string { return hash }

// nameByArg 命名为 "<名字>_<hash>"：名字用于 4合1 网格显示，hash 保证去重 +
// 同名不同图不互相覆盖。grid 的添加已强制要求名字非空，arg 为空仅作兜底。
func nameByArg(hash, arg string, _ int64) string {
	if arg == "" {
		return hash
	}
	return arg + "_" + hash
}
