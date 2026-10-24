package content

import (
	"fmt"
	"math"
)

func validateSkill(s Skill) error {
	if s.Kind == "" {
		return nil
	} // Fixed historical catalogs retain their original trait-only definitions.
	fail := func() error { return fmt.Errorf("武技/心法规则无效: %s", s.ID) }
	if s.Tier < 1 || s.Tier > 5 {
		return fail()
	}
	if s.Kind == "mind" {
		if s.WeaponKind != "" || s.Bonus.HP < 0 || s.Bonus.Attack < 0 || s.Bonus.Defense < 0 || s.Bonus.Speed < 0 || s.Bonus.HP > 40 || s.Bonus.Attack > 30 || s.Bonus.Defense > 50 || s.Bonus.Speed > 25 {
			return fail()
		}
		switch s.MindEffect {
		case "recover", "opening", "protect", "focus":
		default:
			return fail()
		}
		return nil
	}
	if s.ReadyTurn < 0 || s.ReadyTurn > 10 || s.GuaranteeTurn < 0 || s.GuaranteeTurn > 10 || (s.GuaranteeTurn > 0 && (s.Chance == 0 || s.GuaranteeTurn < s.ReadyTurn)) {
		return fail()
	}
	if s.Kind != "martial" || math.IsNaN(s.Chance) || math.IsInf(s.Chance, 0) || s.Chance < 0 || s.Chance > 1 || s.Every < 0 || s.Every > 10 || (s.Chance == 0 && s.Every == 0) || (s.Chance > 0 && s.Every > 0) || s.Hits < 1 || s.Hits > 3 || math.IsNaN(s.Damage) || s.Damage <= 0 || s.Damage > 6 || math.IsNaN(s.Pierce) || s.Pierce < 0 || s.Pierce > .5 || s.Cooldown < 0 || s.Cooldown > 10 || s.Limit < 0 || s.Limit > 3 {
		return fail()
	}
	return nil
}

// RuleText is derived from executable fields, never used to drive combat.
func (s Skill) RuleText() string {
	if s.Kind == "mind" {
		effect := map[string]string{"recover": "第三次自身行动恢复5%最大气血，每场一次", "opening": "首次武技增伤10%", "protect": "首次有效受击减伤20%", "focus": "首次武技额外忽略15%防御"}[s.MindEffect]
		bonus := ""
		for _, v := range []struct {
			name string
			n    int
		}{{"气血", s.Bonus.HP}, {"攻击", s.Bonus.Attack}, {"防御", s.Bonus.Defense}, {"身法", s.Bonus.Speed}} {
			if v.n > 0 {
				bonus += fmt.Sprintf("%s+%d%% · ", v.name, v.n)
			}
		}
		return bonus + effect
	}
	trigger := fmt.Sprintf("每次自身行动%.0f%%发动", s.Chance*100)
	if s.Every > 0 {
		trigger = fmt.Sprintf("每第%d次自身行动发动", s.Every)
	}
	if s.ReadyTurn > 1 {
		trigger = fmt.Sprintf("第%d次自身出手起 · ", s.ReadyTurn) + trigger
	}
	if s.GuaranteeTurn > 0 {
		trigger += fmt.Sprintf(" · 最迟第%d次自身出手首发", s.GuaranteeTurn)
	}
	text := fmt.Sprintf("%s · %d段×%.0f%%伤害", trigger, s.Hits, s.Damage*100)
	if s.Limit != 1 && s.Cooldown > 0 {
		text += fmt.Sprintf(" · 再次施展前普攻%d次", s.Cooldown)
	}
	if s.Pierce > 0 {
		text += fmt.Sprintf(" · 忽略%.0f%%防御", s.Pierce*100)
	}
	if s.Limit > 0 {
		text += fmt.Sprintf(" · 每场最多%d次", s.Limit)
	}
	if s.Affinity != "" {
		text += " · " + s.Affinity + "兵器联动增伤10%"
	}
	return text
}
