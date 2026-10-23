package gameplay

import (
	"fmt"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/content"
	"math/rand/v2"
)

func Draw(entries []content.DrawEntry, count int, seed uint64) ([]content.Reward, error) {
	total := 0
	for _, entry := range entries {
		total += entry.Weight
	}
	if total <= 0 {
		return nil, fmt.Errorf("当前职业和等级没有可抽取奖励")
	}
	rng := rand.New(rand.NewPCG(seed, 0x79c3c4a7e154e01b))
	rewards := make([]content.Reward, 0, count)
	for range count {
		roll := rng.IntN(total)
		for _, entry := range entries {
			roll -= entry.Weight
			if roll < 0 {
				rewards = append(rewards, entry.Reward)
				break
			}
		}
	}
	return rewards, nil
}
