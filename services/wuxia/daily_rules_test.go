package wuxia

import (
	"math/rand/v2"
	"testing"
)

func TestDailyAdventureBatchAndFirstDuelReward(t *testing.T) {
	c := sixCatalog(t)
	p := upgradePlayer(c, c.Professions[0], 1)
	target := upgradePlayer(c, c.Professions[1], 1)
	v, err := c.View(p, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	d := Daily{}
	result := Result{}
	g := gameAction{catalog: c, player: &p, target: target, before: v, daily: &d, result: &result, settings: DefaultSettings(), rng: rand.New(rand.NewPCG(42, 42))}
	if err = handleAdventure(&g); err != nil {
		t.Fatal(err)
	}
	if d.Runs != 3 || len(result.Encounters) != 3 || d.Signed {
		t.Fatal("expected three encounters without sign reward")
	}
	result = Result{}
	g.result = &result
	if err = handleAdventure(&g); err != nil || len(result.Encounters) != 0 {
		t.Fatal("repeated adventure rewarded", err)
	}
	silver, xp := p.Silver, p.Experience
	if err = handleDuel(&g); err != nil {
		t.Fatal(err)
	}
	if !d.DuelRewarded || p.Silver != silver+100 || p.Experience != xp+20 {
		t.Fatal("first duel reward missing")
	}
	silver, xp = p.Silver, p.Experience
	if err = handleDuel(&g); err != nil || p.Silver != silver || p.Experience != xp {
		t.Fatal("repeat duel rewarded", err)
	}
	g.daily = &Daily{}
	if err = handleDuel(&g); err != nil || p.Silver != silver+100 {
		t.Fatal("next day reward missing", err)
	}
	if _, err = ParseCommand("sign", "", 0); err == nil {
		t.Fatal("sign action still accepted")
	}
}

func TestAdventureFortuneRange(t *testing.T) {
	c := sixCatalog(t)
	e := c.Events[0]
	e.Kind = "gift"
	e.Silver = [2]int{100, 100}
	e.Experience = 100
	e.LootChance = 0
	c.Events = []Event{e}
	low, high := int64(201), int64(0)
	for seed := range 1500 {
		p := upgradePlayer(c, c.Professions[0], 1)
		r, err := c.Explore(&p, DefaultSettings(), uint64(seed))
		if err != nil {
			t.Fatal(err)
		}
		if r.Silver < 50 || r.Silver > 200 || r.Experience != r.Silver {
			t.Fatalf("outside shared 50..200%%: %+v", r)
		}
		low = min(low, r.Silver)
		high = max(high, r.Silver)
	}
	if low != 50 || high != 200 {
		t.Fatalf("range too narrow: %d..%d", low, high)
	}
}

func TestAdventureDuplicateLootReceipt(t *testing.T) {
	c := DefaultCatalog()
	w := c.StarterFor(c.Professions[0].ID)
	c.Weapons = []Weapon{w}
	e := c.Events[0]
	e.Kind, e.Weight, e.MinLevel = "gift", 1, 1
	e.Silver, e.Experience, e.LootChance = [2]int{100, 100}, 0, 1
	c.Events = []Event{e}
	p := upgradePlayer(c, c.Professions[0], 1)
	before := p.Silver
	r, err := c.Explore(&p, DefaultSettings(), 42)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := c.Item(w.ID)
	want := ItemPrice(item) * 40 / 100
	if r.ConvertedSilver != want || p.Silver-before != r.Silver+r.ConvertedSilver {
		t.Fatalf("unaccounted duplicate silver: delta=%d event=%d converted=%d want=%d", p.Silver-before, r.Silver, r.ConvertedSilver, want)
	}
	if p.Inventory[w.ID] != 1 {
		t.Fatal("duplicate should not stack")
	}
}
