package tools

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/Yuelioi/yueling-go/ai"
)

func init() {
	registerChoose()
	registerRoll()
	registerEightBall()
}

func registerChoose() {
	ai.Register(ai.ToolMeta{
		Name:        "help_choose",
		Description: "帮用户从多个选项中做选择，解决选择困难症",
		Tags:        []string{"娱乐"},
		Triggers:    []string{"帮我选", "选一个", "随机选"},
		Slots:       []string{"选择", "还是", "哪个", "哪个好", "选哪"},
		Params: []ai.Param{
			{Name: "options", Type: "string", Description: "逗号分隔的选项，如\"A,B,C\"", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			raw := ctx.String("options")
			opts := strings.Split(raw, ",")
			var choices []string
			for _, o := range opts {
				if s := strings.TrimSpace(o); s != "" {
					choices = append(choices, s)
				}
			}
			if len(choices) == 0 {
				return "请提供至少一个选项", nil
			}
			pick := choices[rand.Intn(len(choices))]
			return fmt.Sprintf("经过深思熟虑，我选择：【%s】！", pick), nil
		},
	})
}

func registerRoll() {
	ai.Register(ai.ToolMeta{
		Name:        "roll_dice",
		Description: "掷骰子，在指定范围内生成随机数",
		Tags:        []string{"娱乐", "随机"},
		Triggers:    []string{"roll", "骰子", "掷骰", "随机数"},
		Slots:       []string{"随机", "抽", "roll"},
		Params: []ai.Param{
			{Name: "max", Type: "integer", Description: "最大值（含），默认100", Required: false},
			{Name: "min", Type: "integer", Description: "最小值（含），默认1", Required: false},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			min := ctx.Int("min")
			max := ctx.Int("max")
			if max == 0 {
				max = 100
			}
			if min == 0 {
				min = 1
			}
			if min >= max {
				return "最小值必须小于最大值", nil
			}
			result := min + rand.Int63n(max-min+1)
			return fmt.Sprintf("roll 结果：%d（范围 %d-%d）", result, min, max), nil
		},
	})
}

func registerEightBall() {
	answers := []string{
		"是的，毫无疑问。",
		"当然，必须的！",
		"大概率是。",
		"不太确定，再想想？",
		"说不清楚，问别人吧。",
		"我的答案是否定的。",
		"绝对不可能。",
		"时机还不成熟。",
		"命运之球说：有缘自然成。",
		"专注于当下，答案自然来。",
	}
	ai.Register(ai.ToolMeta{
		Name:        "magic_eight_ball",
		Description: "魔法8号球，给用户的是非问题一个神秘答案",
		Tags:        []string{"娱乐", "占卜"},
		Triggers:    []string{"8号球", "magic ball", "占卜", "问卦"},
		Slots:       []string{"会不会", "能不能", "是否", "吗"},
		Params: []ai.Param{
			{Name: "question", Type: "string", Description: "用户的问题", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			q := ctx.String("question")
			ans := answers[rand.Intn(len(answers))]
			return fmt.Sprintf("关于「%s」\n魔法8号球说：%s", q, ans), nil
		},
	})
}
