package wuxia

import "fmt"

type Suggestion struct {
	Title   string
	Command string
	Detail  string
}

func (v View) NextStep(day Daily, dailyLimit int) Suggestion {
	if !day.Signed || day.Runs < dailyLimit {
		return Suggestion{"一键日常", "江湖", "签到并完成剩余历练，自动收集更好的装备"}
	}
	if v.Character.Mastery < v.Level && v.Character.Silver >= v.PracticeCost {
		return Suggestion{"研习武学", "江湖练武", fmt.Sprintf("可花%d银两提升武学，每次发送只练一次", v.PracticeCost)}
	}
	if v.MaxLevel > 0 && v.Level >= v.MaxLevel && v.Character.Mastery >= v.Level {
		return Suggestion{"群侠切磋", "切磋 @群友", "已达本期等级与修为上限，试试新装备和武学搭配"}
	}
	if v.Character.Mastery >= v.Level {
		return Suggestion{"群侠切磋", "切磋 @群友", "武学已达当前等级上限，明日历练升级后可继续练武"}
	}
	return Suggestion{"群侠切磋", "切磋 @群友", fmt.Sprintf("练武还需%d银两；今日历练已完成，可免费与群友切磋", v.PracticeCost-v.Character.Silver)}
}
