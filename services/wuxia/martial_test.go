package wuxia

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestMartialEventsConserveHealthAndRespectLimits(t *testing.T) {
	c := DefaultCatalog()
	if os.Getenv("WUXIA_CANDIDATE") != "" {
		c = sixCatalog(t)
	}
	for _, prof := range c.Professions {
		for _, skill := range c.Skills {
			if skill.Kind != "martial" || skill.WeaponKind != prof.WeaponKind {
				continue
			}
			for _, mind := range c.Skills {
				if mind.Kind != "mind" {
					continue
				}
				p := upgradePlayer(c, prof, 5)
				p.LearnedSkills = map[string]bool{skill.ID: true, mind.ID: true}
				p.ActiveSkill = skill.ID
				p.ActiveMind = mind.ID
				view, err := c.View(p, DefaultSettings())
				if err != nil {
					t.Fatal(err)
				}
				for seed := range 40 {
					left, right := view.Fighter(), view.Fighter()
					right.Name = "对手"
					b := Duel(left, right, uint64(seed))
					hp := [2]int{left.Stats.HP, right.Stats.HP}
					uses, turns, last := [2]int{}, [2]int{}, [2]int{}
					healed := [2]int{}
					for _, a := range b.Actions {
						i, j := a.Actor, 1-a.Actor
						turns[i]++
						if hp[i] <= 0 || hp[j] <= 0 {
							t.Fatal("dead actor acted")
						}
						sum := 0
						for _, hit := range a.Hits {
							if hit < 0 {
								t.Fatal("negative hit")
							}
							sum += hit
						}
						if sum != a.Damage {
							t.Fatal("multi-hit total differs")
						}
						hp[i] += a.Healing - a.PoisonDamage
						hp[j] -= a.Damage
						if a.Healing > a.LifeSteal {
							healed[i]++
						}
						if hp != a.Remaining || hp[i] > left.Stats.HP || hp[j] < 0 {
							t.Fatal("health conservation violated")
						}
						if a.SkillID != "" {
							if uses[i] > 0 && turns[i]-last[i] <= skill.Cooldown {
								t.Fatal("cooldown ignored")
							}
							if skill.Every > 0 && turns[i]%skill.Every != 0 {
								t.Fatal("turn trigger ignored")
							}
							uses[i]++
							last[i] = turns[i]
							maxHits := skill.Hits
							if a.Combo {
								maxHits++
							}
							if len(a.Hits) > maxHits {
								t.Fatal("unbounded hits")
							}
						}
					}
					for i := range 2 {
						if (skill.Limit > 0 && uses[i] > skill.Limit) || healed[i] > 1 {
							t.Fatal("per-fight limit exceeded")
						}
					}
					if hp != b.Remaining || (b.Winner >= 0 && hp[1-b.Winner] != 0) {
						t.Fatal("fabricated winner")
					}
					again := Duel(left, right, uint64(seed))
					if !reflect.DeepEqual(b, again) {
						t.Fatal("seed replay differs")
					}
				}
			}
		}
	}
}

func TestMartialLearningSlotsAndDrawOdds(t *testing.T) {
	c := DefaultCatalog()
	pool, _ := c.Pool("jianghu")
	for _, prof := range c.Professions {
		totals := map[string]int{}
		for _, e := range c.EligibleEntries(pool, prof.ID, 1) {
			item, _ := c.Item(e.Reward.ItemID)
			totals[item.Kind] += e.Weight
		}
		if !reflect.DeepEqual(totals, map[string]int{"weapon": 350000, "armor": 250000, "charm": 200000, "manual": 200000}) {
			t.Fatalf("category drift %v", totals)
		}
	}
	p := upgradePlayer(c, c.Professions[0], 1)
	rewards := []Reward{{ItemID: "manual-sword-rain", Quantity: 2}, {ItemID: "manual-mind-breath", Quantity: 1}, {ItemID: "manual-sword-heaven", Quantity: 1}}
	if err := c.Grant(&p, rewards); err != nil {
		t.Fatal(err)
	}
	if err := c.LearnRewards(&p, rewards); err != nil {
		t.Fatal(err)
	}
	if p.ActiveSkill != "sword-rain" || p.ActiveMind != "mind-breath" || p.Inventory["manual-sword-rain"] != 0 || !p.LearnedSkills["sword-heaven"] {
		t.Fatal("learning overwrote build or duplicate")
	}
	v, err := c.View(p, DefaultSettings())
	if err != nil || v.Synergy == "" || v.Mind == nil {
		t.Fatal("missing loadout")
	}
	p.ProfessionID = "blade"
	p.WeaponKind = "刀"
	p.Weapons["刀"] = "刀-1"
	v, err = c.View(p, DefaultSettings())
	if err != nil || v.Skill != nil || v.Mind == nil || v.Synergy != "" {
		t.Fatal("profession eligibility leaked")
	}
	for _, mutate := range []func(*Catalog){func(c *Catalog) { c.Skills[0].Hits = 100 }, func(c *Catalog) { c.Skills[0].Cooldown = -1 }, func(c *Catalog) { c.Pools[0].CategoryWeights["weapon"] = -1 }} {
		bad := DefaultCatalog()
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, err := ParseCatalog(raw); err == nil {
			t.Fatal("invalid effect accepted")
		}
	}
}

func TestMartialMindPersistenceAndReplay(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	req := Request{BotID: 1, GroupID: 890, UserID: 10, MessageID: 1, Name: "武者", Action: "daily"}
	if _, err := s.Operate(ctx, req); err != nil {
		t.Fatal(err)
	}
	inventory := `{"剑-1":1,"manual-sword-rain":2,"manual-mind-breath":1}`
	if err := s.database.Model(&Character{}).Where("group_id=? AND user_id=?", 890, 10).Update("inventory", inventory).Error; err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"manual-sword-rain", "manual-mind-breath"} {
		req.Action = "learn"
		req.Argument = name
		req.MessageID = int32(i + 2)
		first, err := s.Operate(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		second, err := s.Operate(ctx, req)
		if err != nil || !second.Replayed || first.Actor.Character.ActiveMind != second.Actor.Character.ActiveMind {
			t.Fatal("learning replay changed")
		}
	}
	v, _, err := s.Profile(ctx, 890, 10)
	if err != nil || v.Skill == nil || v.Mind == nil || v.Character.Inventory["manual-sword-rain"] != 1 {
		t.Fatal("slots did not persist")
	}
	req.Action = "profession"
	req.Argument = "blade"
	req.MessageID = 4
	r, err := s.Operate(ctx, req)
	if err != nil || r.Actor.Character.ActiveSkill != "" || r.Actor.Mind == nil {
		t.Fatal("switch must clear incompatible martial only")
	}
}

func TestOpeningDuelActionHalvesWholeComboOnly(t *testing.T) {
	c := DefaultCatalog()
	sk, _ := c.Skill("sword-unity")
	sk.Chance = 1
	sk.ReadyTurn = 0
	sk.GuaranteeTurn = 0
	sk.Limit = 0
	f := Fighter{Name: "甲", Stats: Stats{HP: 1000000, Attack: 41, Defense: 0, Speed: 20}, Martial: &sk}
	for seed := range 100 {
		duel, ordinary := Duel(f, f, uint64(seed)), Fight(f, f, uint64(seed))
		if len(duel.Actions) != len(ordinary.Actions) {
			t.Fatal("opening changed action schedule")
		}
		raw := 0
		for i, hit := range ordinary.Actions[0].Hits {
			before := raw / 2
			raw += hit
			if duel.Actions[0].Hits[i] != raw/2-before {
				t.Fatal("combo half rounding inconsistent")
			}
		}
		if duel.Actions[0].Damage != ordinary.Actions[0].Damage/2 || !duel.Actions[0].Opening {
			t.Fatal("first action not halved")
		}
		for i := 1; i < len(duel.Actions); i++ {
			if duel.Actions[i].Opening || duel.Actions[i].Damage != ordinary.Actions[i].Damage {
				t.Fatal("later action was reduced")
			}
		}
	}
}

func TestEveryProfessionHasAllMartialAndMindTiers(t *testing.T) {
	c := DefaultCatalog()
	pool, _ := c.Pool("jianghu")
	for _, prof := range c.Professions {
		tiers := map[string]map[int]int{"martial": {}, "mind": {}}
		for _, entry := range c.EligibleEntries(pool, prof.ID, 1) {
			item, _ := c.Item(entry.Reward.ItemID)
			if item.Kind != "manual" {
				continue
			}
			skill, ok := c.Skill(item.SkillID)
			if !ok || entry.Weight <= 0 {
				t.Fatalf("unobtainable manual: %s", item.ID)
			}
			if skill.Kind == "martial" && skill.WeaponKind != prof.WeaponKind {
				t.Fatal("wrong profession manual")
			}
			tiers[skill.Kind][skill.Tier] += entry.Weight
		}
		for _, kind := range []string{"martial", "mind"} {
			for tier := 1; tier <= 5; tier++ {
				if tiers[kind][tier] == 0 {
					t.Fatalf("%s missing %s tier %d", prof.ID, kind, tier)
				}
				if tier > 1 && tiers[kind][tier] >= tiers[kind][tier-1] {
					t.Fatalf("rarity not decreasing: %s %s tier %d", prof.ID, kind, tier)
				}
			}
		}
	}
}

func TestEquippedMartialDoesNotRenameNormalAttacks(t *testing.T) {
	c := DefaultCatalog()
	prof, _ := c.Profession("swordsman")
	p := upgradePlayer(c, prof, 5)
	p.LearnedSkills = map[string]bool{"sword-heaven": true}
	p.ActiveSkill = "sword-heaven"
	v, err := c.View(p, DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	f := v.Fighter()
	if f.Techniques != v.Weapon.Techniques {
		t.Fatal("martial skill overwrote normal attack names")
	}
	normal, active := false, false
	for seed := uint64(0); seed < 100; seed++ {
		for _, a := range Duel(f, f, seed).Actions {
			if a.SkillID == "" {
				normal = true
				if a.Technique != v.Weapon.Techniques[0] {
					t.Fatal("normal attack pretended to use martial art")
				}
			} else {
				active = true
				if a.Technique != v.Skill.Name {
					t.Fatal("triggered martial art lost its name")
				}
			}
		}
	}
	if !normal || !active {
		t.Fatal("fixture must cover both normal and martial attacks")
	}
}

func TestBurstCombatSample(t *testing.T) {
	c := DefaultCatalog()
	for _, prof := range c.Professions {
		var skill Skill
		for _, s := range c.Skills {
			if s.Kind == "martial" && s.Tier == 5 && s.WeaponKind == prof.WeaponKind {
				skill = s
				break
			}
		}
		p := upgradePlayer(c, prof, 10)
		p.LearnedSkills = map[string]bool{skill.ID: true}
		p.ActiveSkill = skill.ID
		v, err := c.View(p, DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		total, dodged, reversals, casts := 0, 0, 0, 0
		for seed := range 2000 {
			b := Duel(v.Fighter(), v.Fighter(), uint64(seed))
			total += len(b.Actions)
			for _, a := range b.Actions {
				if a.SkillID != "" {
					casts++
					if a.Damage == 0 {
						dodged++
						if b.Winner == 1-a.Actor {
							reversals++
						}
					}
				}
			}
		}
		if dodged == 0 || reversals == 0 {
			t.Fatal("expected dodged skills and subsequent wins")
		}
		t.Logf("%s: 2000场 平均出手%.2f 武技%d 完全闪避%d 闪避方获胜%d", prof.Name, float64(total)/2000, casts, dodged, reversals)
	}
}

func TestMartialCooldownStillAllowsNormalAttack(t *testing.T) {
	f := Fighter{Stats: Stats{HP: 100000, Attack: 10, Speed: 10}, Martial: &Skill{ID: "test", Kind: "martial", Chance: 1, Hits: 1, Damage: 5, Cooldown: 1}}
	b := Duel(f, f, 42)
	turns := [2]int{}
	for _, a := range b.Actions {
		turns[a.Actor]++
		if (a.SkillID != "") != (turns[a.Actor]%2 == 1) {
			t.Fatal("cooldown should alternate martial and normal attacks")
		}
		if len(a.Hits) != 1 {
			t.Fatal("cooldown skipped the attack")
		}
	}
}

// Cross-profession fixtures keep progression and equipment tiers comparable.
func TestBurstCrossProfessionAudit(t *testing.T) {
	c := DefaultCatalog()
	if path := os.Getenv("WUXIA_CANDIDATE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		c, err = ParseCatalog(raw)
		if err != nil {
			t.Fatal(err)
		}
	}
	offset := uint64(0)
	if os.Getenv("WUXIA_HOLDOUT") != "" {
		offset = 1000000
	}
	rows := []map[string]any{}
	for _, level := range []int{1, 5, 10} {
		for _, tier := range []int{1, 2, 3, 4, 5} {
			fighters := []Fighter{}
			for _, prof := range c.Professions {
				p := upgradePlayer(c, prof, level)
				weapon := c.Starter(prof.WeaponKind).ID
				if level >= 4 {
					weapon = prof.WeaponKind + "-2"
				}
				if level == 10 {
					weapon = prof.WeaponKind + "-4"
				}
				p.Weapons[prof.WeaponKind] = weapon
				p.Inventory[weapon] = 1
				if level == 10 {
					p.Equipment = map[string]string{"armor": "armor-mountain", "charm": "charm-moon"}
				}
				for _, s := range c.Skills {
					if s.Kind == "martial" && s.Tier == tier && s.WeaponKind == prof.WeaponKind {
						p.ActiveSkill = s.ID
						break
					}
				}
				if level == 10 {
					p.ActiveMind = "mind-nineyang"
				}
				p.LearnedSkills = map[string]bool{p.ActiveSkill: true, p.ActiveMind: true}
				v, err := c.View(p, DefaultSettings())
				if err != nil {
					t.Fatal(err)
				}
				fighters = append(fighters, v.Fighter())
			}
			for i := 0; i < len(fighters); i++ {
				for j := i + 1; j < len(fighters); j++ {
					wins, draws, actions, casts, noCast, openingKills := 0, 0, 0, 0, 0, 0
					for seed := 0; seed < 3000; seed++ {
						a, b := fighters[i], fighters[j]
						swapped := seed%2 == 1
						if swapped {
							a, b = b, a
						}
						battle := Duel(a, b, offset+uint64(seed))
						if battle.Winner < 0 {
							draws++
						} else if (battle.Winner == 0) != swapped {
							wins++
						}
						actions += len(battle.Actions)
						used := [2]bool{}
						for _, act := range battle.Actions {
							if act.SkillID != "" {
								casts++
								used[act.Actor] = true
							}
						}
						for _, u := range used {
							if !u {
								noCast++
							}
						}
						if len(battle.Actions) == 1 {
							openingKills++
						}
					}
					rows = append(rows, map[string]any{"level": level, "tier": tier, "left": fighters[i].Profession, "right": fighters[j].Profession, "win_rate": float64(wins) / 3000, "draws": draws, "mean_actions": float64(actions) / 3000, "casts_per_match": float64(casts) / 3000, "no_cast_rate": float64(noCast) / 6000, "opening_kills": openingKills, "samples": 3000})
					t.Logf("level%d tier%d %s vs %s: 左胜%.1f%% 平%d 平均出手%.1f 武技/场%.2f 未放技能角色%.1f%% 首手结束%d", level, tier, fighters[i].Profession, fighters[j].Profession, float64(wins)/30, draws, float64(actions)/3000, float64(casts)/3000, float64(noCast)/60, openingKills)
				}
			}
		}
	}
	if path := os.Getenv("WUXIA_MARTIAL_REPORT"); path != "" {
		raw, err := json.MarshalIndent(map[string]any{"content_version": c.Version, "combat_version": DuelVersion, "scope": "1/5/10级，同档养成，全部五档武技，每职业每档选首个技能；10级九阳心法，其他无心法；非全组合保证", "matches": rows}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegendaryWindowAndActionOpportunity(t *testing.T) {
	c := DefaultCatalog()
	for _, sk := range c.Skills {
		if sk.Kind != "martial" || sk.Tier != 5 {
			continue
		}
		for seed := range 200 {
			left := Fighter{Stats: Stats{HP: 1000000, Attack: 40, Speed: 100}, Martial: &sk}
			right := left
			right.Stats.Speed = 1
			b := Duel(left, right, uint64(seed))
			turns, uses := [2]int{}, [2]int{}
			previous, streak := -1, 0
			for _, a := range b.Actions {
				turns[a.Actor]++
				if a.Actor == previous {
					streak++
				} else {
					previous, streak = a.Actor, 1
				}
				if streak > 2 {
					t.Fatal("starved slower fighter")
				}
				if a.SkillID != "" {
					uses[a.Actor]++
					if turns[a.Actor] < sk.ReadyTurn {
						t.Fatal("martial cast before ready")
					}
				}
				if sk.GuaranteeTurn > 0 && turns[a.Actor] >= sk.GuaranteeTurn && uses[a.Actor] == 0 {
					t.Fatal("missed first-cast guarantee")
				}
			}
			if uses != [2]int{sk.Limit, sk.Limit} {
				t.Fatalf("%s: expected configured casts per living fighter: %v", sk.ID, uses)
			}
		}
	}
}

func TestMartialQualityAudit(t *testing.T) {
	c := DefaultCatalog()
	if os.Getenv("WUXIA_CANDIDATE") != "" {
		c = sixCatalog(t)
	}
	for _, prof := range c.Professions {
		p := upgradePlayer(c, prof, 10)
		p.Weapons[prof.WeaponKind] = prof.WeaponKind + "-4"
		p.Equipment = map[string]string{"armor": "armor-mountain", "charm": "charm-moon"}
		v, err := c.View(p, DefaultSettings())
		if err != nil {
			t.Fatal(err)
		}
		base := v.Fighter()
		for _, hi := range c.Skills {
			if hi.Kind != "martial" || hi.Tier != 5 || hi.WeaponKind != prof.WeaponKind {
				continue
			}
			for _, lo := range c.Skills {
				if lo.Kind != "martial" || lo.Tier >= 5 || lo.WeaponKind != prof.WeaponKind {
					continue
				}
				a, b := base, base
				a.Martial = &hi
				b.Martial = &lo
				wins := 0
				for seed := range 3000 {
					left, right := a, b
					if seed%2 == 1 {
						left, right = b, a
					}
					r := Duel(left, right, uint64(seed))
					if r.Winner >= 0 && (r.Winner == 0) == (seed%2 == 0) {
						wins++
					}
				}
				t.Logf("%s vs %s: %.1f%%", hi.Name, lo.Name, float64(wins)/30)
				if wins < 1500 {
					t.Errorf("legendary underperforms lower tier in equal-build fixture: %s vs %s", hi.ID, lo.ID)
				}
			}
		}
	}
}
