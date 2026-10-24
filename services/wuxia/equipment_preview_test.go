package wuxia

import (
	"encoding/json"
	"os"
	"testing"
)

func TestEquipmentPreviewAllLegalBuilds(t *testing.T) {
	c := EquipmentPreviewCatalog()
	count := 0
	for _, prof := range c.Professions {
		for _, level := range []int{1, 4, 6, 9, 10} {
			weapons := []string{}
			armor := []string{""}
			charms := []string{""}
			skills := []string{""}
			for _, w := range c.Weapons {
				i, _ := c.Item(w.ID)
				if c.Allowed(i, prof.ID, level) {
					weapons = append(weapons, w.ID)
				}
			}
			for _, i := range c.Items {
				if c.Allowed(i, prof.ID, level) {
					if i.Kind == "armor" {
						armor = append(armor, i.ID)
					}
					if i.Kind == "charm" {
						charms = append(charms, i.ID)
					}
				}
			}
			for _, s := range c.Skills {
				if s.WeaponKind == prof.WeaponKind && s.MinLevel <= level {
					skills = append(skills, s.ID)
				}
			}
			for _, w := range weapons {
				for _, a := range armor {
					for _, ch := range charms {
						for _, sk := range skills {
							v, err := c.PreviewBuild(PreviewBuild{Profession: prof.ID, Level: level, Mastery: level, Weapon: w, Armor: a, Charm: ch, Skill: sk})
							if err != nil {
								t.Fatal(err)
							}
							tr := v.Fighter().Traits
							if tr.Critical > .18+1e-9 || tr.Pierce > .18+1e-9 || tr.Guard > .17+1e-9 || tr.Dodge > .15+1e-9 {
								t.Fatalf("overflow: %s %s %s %s %+v", prof.ID, w, a, ch, tr)
							}
							count++
						}
					}
				}
			}
		}
	}
	t.Logf("verified %d legal builds against trait caps", count)
	if _, err := c.PreviewBuild(PreviewBuild{Profession: "swordsman", Level: 1, Weapon: "刀-1"}); err == nil {
		t.Fatal("cross-profession preview accepted")
	}
	if DefaultCatalog().Version == c.Version {
		t.Fatal("preview replaced live catalog")
	}
}

func TestEquipmentPreviewBalance(t *testing.T) {
	c := EquipmentPreviewCatalog()
	type entry struct {
		Name  string       `json:"name"`
		Build PreviewBuild `json:"build"`
	}
	specs := []struct{ name, prof, w, a, ch string }{
		{"惊鸿快剑", "swordsman", "惊鸿剑", "夜行锦衣", "虎牙坠"},
		{"照水游身", "swordsman", "照水剑", "夜行锦衣", "明月环"},
		{"赤霄破阵", "blade", "赤霄刀", "破阵战袍", "砺锋石"},
		{"镇关守势", "blade", "镇关刀", "山纹软甲", "玄武玉"},
		{"镇岳稳守", "boxer", "镇岳拳套", "山纹软甲", "玄武玉"},
		{"裂碑短打", "boxer", "裂碑指虎", "赤练战袍", "虎牙坠"},
	}
	builds := []entry{}
	fighters := []Fighter{}
	for _, s := range specs {
		w, _ := c.Weapon(s.w)
		a, _ := c.Item(s.a)
		ch, _ := c.Item(s.ch)
		b := PreviewBuild{Profession: s.prof, Level: 10, Mastery: 10, WeaponLevel: 10, Weapon: w.ID, Armor: a.ID, Charm: ch.ID}
		for _, sk := range c.Skills {
			if sk.WeaponKind == w.Kind {
				b.Skill = sk.ID
			}
		}
		v, err := c.PreviewBuild(b)
		if err != nil {
			t.Fatal(err)
		}
		builds = append(builds, entry{s.name, b})
		fighters = append(fighters, v.Fighter())
	}
	type matchup struct {
		Left     int     `json:"left"`
		Right    int     `json:"right"`
		Expected float64 `json:"expected"`
		Observed float64 `json:"observed"`
		Samples  int     `json:"samples"`
	}
	matches := []matchup{}
	for i, a := range fighters {
		for j, b := range fighters {
			if j <= i {
				continue
			}
			p := DuelWinProbability(a, b)
			wins := 0
			for seed := range 600 {
				if Duel(a, b, uint64(seed)).Winner == 0 {
					wins++
				}
			}
			actual := float64(wins) / 600
			t.Logf("%s vs %s: expected %.3f, observed %.3f", builds[i].Name, builds[j].Name, p, actual)
			if actual < .15 || actual > .85 {
				t.Logf("balance review needed: %.3f", actual)
			}
			matches = append(matches, matchup{i, j, p, actual, 600})
		}
	}
	if path := os.Getenv("WUXIA_BALANCE_REPORT"); path != "" && !t.Failed() {
		raw, err := json.MarshalIndent(map[string]any{"version": c.Version, "rules_version": DuelVersion, "level": 10, "mastery": 10, "seed_start": 0, "builds": builds, "matches": matches, "scope": "六种推荐搭配，10级满修为；不代表全部搭配或PVE已平衡"}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEquipmentPreviewProgression(t *testing.T) {
	c := EquipmentPreviewCatalog()
	rows := []map[string]any{}
	pve := []map[string]any{}
	for _, level := range []int{1, 4, 6, 9, 10} {
		for _, mastery := range []int{0, level} {
			fighters := []Fighter{}
			for _, prof := range c.Professions {
				weapon := c.StarterFor(prof.ID)
				for _, w := range c.Weapons {
					item, _ := c.Item(w.ID)
					if w.Tier <= min(4, 1+level/3) && c.Allowed(item, prof.ID, level) && (w.MinLevel > weapon.MinLevel || w.MinLevel == weapon.MinLevel && w.Tier > weapon.Tier) {
						weapon = w
					}
				}
				build := PreviewBuild{Profession: prof.ID, Level: level, Mastery: mastery, WeaponLevel: max(1, level), Weapon: weapon.ID}
				for _, kind := range []string{"armor", "charm"} {
					var chosen Item
					for _, i := range c.Items {
						if i.Kind == kind && c.Allowed(i, prof.ID, level) && i.MinLevel > chosen.MinLevel {
							chosen = i
						}
					}
					if kind == "armor" {
						build.Armor = chosen.ID
					} else {
						build.Charm = chosen.ID
					}
				}
				view, err := c.PreviewBuild(build)
				if err != nil {
					t.Fatal(err)
				}
				fighters = append(fighters, view.Fighter())
				combats, wins := 0, 0
				for seed := range 200 {
					fresh, err := c.PreviewBuild(build)
					if err != nil {
						t.Fatal(err)
					}
					e, err := c.Explore(&fresh.Character, DefaultSettings(), uint64(seed))
					if err != nil {
						t.Fatal(err)
					}
					if e.Battle != nil {
						combats++
						if e.Battle.Winner == 0 {
							wins++
						}
					}
				}
				rate := float64(wins) / float64(combats)
				if combats == 0 || rate < .2 || rate > .85 {
					t.Errorf("PVE outlier %s level %d mastery %d: %.3f", prof.ID, level, mastery, rate)
				}
				pve = append(pve, map[string]any{"profession": prof.ID, "level": level, "mastery": mastery, "combats": combats, "wins": wins, "win_rate": rate})
			}
			for i, a := range fighters {
				for j, b := range fighters {
					if j <= i {
						continue
					}
					p := DuelWinProbability(a, b)
					wins := 0
					for seed := range 200 {
						if Duel(a, b, uint64(seed)).Winner == 0 {
							wins++
						}
					}
					if p < 0 || p > 1 {
						t.Errorf("progression outlier at %d/%d: %.3f", level, mastery, p)
					}
					rows = append(rows, map[string]any{"level": level, "mastery": mastery, "left": c.Professions[i].ID, "right": c.Professions[j].ID, "expected": p, "observed": float64(wins) / 200, "samples": 200})
				}
			}
		}
	}
	t.Logf("verified %d progression matchups and %d PVE configurations", len(rows), len(pve))
	if path := os.Getenv("WUXIA_BALANCE_REPORT"); path != "" && !t.Failed() {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var report map[string]any
		if err := json.Unmarshal(raw, &report); err != nil {
			t.Fatal(err)
		}
		report["progression"] = rows
		report["pve"] = pve
		raw, err = json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
