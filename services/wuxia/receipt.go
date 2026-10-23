package wuxia

import (
	"encoding/json"
	"github.com/Yuelioi/yueling-go/services/wuxia/internal/content"
)

func decodeReceipt(raw string) (Result, error) {
	var r Result
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, err
	}
	if len(r.Actor.Collection) == 0 && r.Actor.Weapon.ID != "" {
		r.Actor.Collection = []Weapon{r.Actor.Weapon}
	}
	if len(r.Rewards) > 0 && len(r.RewardItems) == 0 {
		c, err := content.Historical(r.Version)
		if err != nil {
			return r, err
		}
		for _, reward := range r.Rewards {
			item, ok := c.Item(reward.ItemID)
			if !ok {
				return r, rulef("历史物品快照不存在")
			}
			r.RewardItems = append(r.RewardItems, item)
		}
		if pool, ok := c.Pool(r.DrawPool); ok {
			r.DrawPoolName = pool.Name
		}
	}
	return r, nil
}
