package wuxia

import (
	"fmt"
	"math"
	"slices"
)

const CasinoRules = "江湖赌场 · 三骰开宝\n江湖赌场 100：下注100银两；不填默认20，最低20。\n豹子111～666依次返5～10倍；顺子123/234/345/456返3倍；恰好一对回本；135/246返半数；其余落空，排列顺序不限。\n倍率含本金，半数向下取整。仅使用游戏内银两。"

type CasinoResult struct {
	Dice           [3]int `json:"dice"`
	MultiplierHalf int64  `json:"multiplier_half"`
	Outcome        string `json:"outcome"`
	Stake          int64  `json:"stake"`
	Payout         int64  `json:"payout"`
	Won            bool   `json:"won"`
}

func ResolveCasino(stake int64, dice [3]int) (CasinoResult, error) {
	r := CasinoResult{Dice: dice, Stake: stake}
	if stake < 20 || stake > math.MaxInt64/10 {
		return r, rulef("赌金最低20银两，且不能超过安全结算范围")
	}
	sorted := dice
	for _, v := range dice {
		if v < 1 || v > 6 {
			return r, rulef("骰子点数无效")
		}
	}
	slices.Sort(sorted[:])
	a, b, c := sorted[0], sorted[1], sorted[2]
	switch {
	case a == c:
		r.Outcome = "豹子"
		r.MultiplierHalf = int64(a+4) * 2
	case a == b || b == c:
		r.Outcome = "对子回本"
		r.MultiplierHalf = 2
	case b == a+1 && c == b+1:
		r.Outcome = "顺子"
		r.MultiplierHalf = 6
	case b == a+2 && c == b+2:
		r.Outcome = "三奇三偶"
		r.MultiplierHalf = 1
	default:
		r.Outcome = "落空"
	}
	r.Payout = stake * (r.MultiplierHalf / 2)
	if r.MultiplierHalf%2 != 0 {
		r.Payout += stake / 2
	}
	r.Won = r.Payout > stake
	return r, nil
}

func handleCasino(g *gameAction) error {
	cmd := g.command.(CasinoCommand)
	if g.player.Silver < cmd.Stake {
		return rulef("银两不足：下注需%d，当前%d", cmd.Stake, g.player.Silver)
	}
	dice := [3]int{g.rng.IntN(6) + 1, g.rng.IntN(6) + 1, g.rng.IntN(6) + 1}
	result, err := ResolveCasino(cmd.Stake, dice)
	if err != nil {
		return err
	}
	if result.Payout > math.MaxInt64-(g.player.Silver-result.Stake) {
		return rulef("余额已达安全结算上限")
	}
	g.player.Silver += result.Payout - result.Stake
	g.result.Casino = &result
	g.result.Lines = append(g.result.Lines, fmt.Sprintf("赌金%d银两 · %s · 净收益%+d银两", result.Stake, result.Outcome, result.Payout-result.Stake))
	return nil
}
