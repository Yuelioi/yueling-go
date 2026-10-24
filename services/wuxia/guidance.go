package wuxia

type Suggestion struct {
	Title   string
	Command string
	Detail  string
}

func (v View) NextStep(day Daily, dailyLimit int) Suggestion {
	if day.Runs < dailyLimit {
		return Suggestion{"一键日常", "江湖", "完成剩余三次历练，自动收集更好的装备"}
	}
	return Suggestion{"群侠切磋", "切磋 @群友", "今日历练已完成，可切磋或寻宝换装"}
}
