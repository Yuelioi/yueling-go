package cards

import (
	"context"
	"github.com/Yuelioi/yueling-go/services/wuxia"
	"os"
	"path/filepath"
	"testing"
)

// Every current preview is rendered from the candidate catalog, never DefaultCatalog.
func TestSixCandidateScreens(t *testing.T) {
	raw, err := os.ReadFile("../internal/content/six_professions.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := wuxia.ParseCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	save := func(id string, data []byte, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(id, err)
		}
		if dir := os.Getenv("YUELING_WUXIA_PREVIEW_DIR"); dir != "" {
			if err = os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, id+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	views := []wuxia.View{}
	for _, prof := range c.Professions {
		build := wuxia.PreviewBuild{Profession: prof.ID, Level: 5}
		for _, w := range c.Weapons {
			if w.Kind == prof.WeaponKind && w.Tier == 5 {
				build.Weapon = w.ID
				break
			}
		}
		for _, s := range c.Skills {
			if s.Tier == 5 {
				if s.Kind == "mind" {
					build.Mind = s.ID
				}
				if s.WeaponKind == prof.WeaponKind && s.Kind == "martial" && build.Skill == "" {
					build.Skill = s.ID
				}
			}
		}
		for _, i := range c.Items {
			if i.Image != "" && i.Tier == 5 {
				if i.Kind == "armor" && build.Armor == "" {
					build.Armor = i.ID
				}
				if i.Kind == "charm" && build.Charm == "" {
					build.Charm = i.ID
				}
			}
		}
		// Render actual game stats; the legacy equipment simulator has its own reinforcement scale.
		player := wuxia.Character{Name: prof.Name, ProfessionID: prof.ID, Experience: wuxia.XPForLevel(build.Level), WeaponKind: prof.WeaponKind, Weapons: map[string]string{prof.WeaponKind: build.Weapon}, Equipment: map[string]string{"armor": build.Armor, "charm": build.Charm}, LearnedSkills: map[string]bool{build.Skill: true, build.Mind: true}, ActiveSkill: build.Skill, ActiveMind: build.Mind}
		v, err := c.View(player, wuxia.DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		v.Character.Name = prof.Name
		v.Character.Silver = 1280
		views = append(views, v)
		data, err := Profile(context.Background(), v, wuxia.Daily{Runs: 1}, 3)
		save("candidate-profile-"+prof.ID, data, err)
		empty, err := c.View(wuxia.Character{Name: prof.Name, ProfessionID: prof.ID, WeaponKind: prof.WeaponKind, Weapons: map[string]string{prof.WeaponKind: c.StarterFor(prof.ID).ID}}, wuxia.DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		data, err = Profile(context.Background(), empty, wuxia.Daily{}, 3)
		save("candidate-empty-"+prof.ID, data, err)
		p := v.Character
		p.Experience = 0
		r := wuxia.Result{Actor: v, DailyLimit: 3, Daily: wuxia.Daily{Runs: 3}}
		for seed := uint64(11); seed < 14; seed++ {
			e, err := c.Explore(&p, wuxia.DefaultSettings(), seed)
			if err != nil {
				t.Fatal(err)
			}
			r.Encounters = append(r.Encounters, e)
			r.SilverDelta += e.Silver
			r.ExperienceDelta += e.Experience
		}
		r.Actor, err = c.View(p, wuxia.DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		data, err = Report(context.Background(), r)
		save("candidate-adventure-"+prof.ID, data, err)
		r = wuxia.Result{Actor: v, DrawPoolName: "江湖宝匣", SilverDelta: -300}
		for tier := 1; tier <= 5; tier++ {
			for _, w := range c.Weapons {
				if w.Kind == prof.WeaponKind && w.Tier == tier {
					item, _ := c.Item(w.ID)
					r.Rewards = append(r.Rewards, wuxia.Reward{ItemID: w.ID, Quantity: 1})
					r.RewardItems = append(r.RewardItems, item)
					break
				}
			}
		}
		data, err = Report(context.Background(), r)
		save("candidate-draw-"+prof.ID, data, err)
	}
	for i, v := range views {
		b := wuxia.Duel(v.Fighter(), views[(i+1)%len(views)].Fighter(), uint64(77+i))
		data, err := Report(context.Background(), wuxia.Result{Actor: v, Battle: &b})
		save("candidate-duel-"+v.Profession.ID, data, err)
	}
	for i, v := range views {
		v.Character.Portrait = v.Profession.Portrait + "-female"
		data, err := Profile(context.Background(), v, wuxia.Daily{}, 3)
		save("candidate-female-profile-"+v.Profession.ID, data, err)
		other := views[(i+1)%len(views)]
		other.Character.Portrait = other.Profession.Portrait + "-female"
		// Search real deterministic bouts for a loss; do not invent a winner in a real report.
		var b wuxia.Battle
		for seed := uint64(1); seed < 10000; seed++ {
			b = wuxia.Duel(v.Fighter(), other.Fighter(), seed)
			if b.Winner == 1 {
				break
			}
		}
		if b.Winner != 1 {
			t.Fatal("missing female defeat fixture")
		}
		data, err = Report(context.Background(), wuxia.Result{Actor: v, Battle: &b})
		save("candidate-female-defeat-"+v.Profession.ID, data, err)
	}
	wager, _ := wuxia.ResolveCasino(100, [3]int{6, 6, 6})
	data, err := Report(context.Background(), wuxia.Result{Actor: views[0], Casino: &wager})
	save("candidate-casino", data, err)
}
