package assets

import (
	"fmt"
	"maps"
	"math"
)

type Settlement struct {
	SilverCost int64
	Costs      map[string]int64
	Rewards    map[string]int64
}

func Settle(silver int64, inventory map[string]int64, change Settlement) (int64, map[string]int64, error) {
	if change.SilverCost < 0 || silver < change.SilverCost {
		return silver, nil, fmt.Errorf("银两不足")
	}
	next := maps.Clone(inventory)
	if next == nil {
		next = map[string]int64{}
	}
	for id, n := range change.Costs {
		if n < 0 || next[id] < n {
			return silver, nil, fmt.Errorf("物品不足: %s", id)
		}
		next[id] -= n
	}
	for id, n := range change.Rewards {
		if n < 1 || next[id] > math.MaxInt64-n {
			return silver, nil, fmt.Errorf("物品数量超出上限: %s", id)
		}
		next[id] += n
	}
	return silver - change.SilverCost, next, nil
}
