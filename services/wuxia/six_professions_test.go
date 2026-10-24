package wuxia

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func sixCatalog(t *testing.T) Catalog {
	t.Helper()
	raw, err := os.ReadFile("internal/content/six_professions.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := ParseCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSixProfessionContentAndGrowth(t *testing.T) {
	c := sixCatalog(t)
	if len(c.Professions) != 6 {
		t.Fatal("six professions required")
	}
	pool, _ := c.Pool("jianghu")
	for _, prof := range c.Professions {
		previous := Stats{}
		for level := 1; level <= 10; level++ {
			p := upgradePlayer(c, prof, level)
			v, err := c.View(p, DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			if v.Stats.HP <= previous.HP || v.Stats.Attack <= previous.Attack || v.Stats.Defense < previous.Defense || v.Stats.Speed < previous.Speed {
				t.Fatal("non-monotonic growth", prof.ID, level)
			}
			previous = v.Stats
		}
		for tier := 1; tier <= 5; tier++ {
			weapons, skills := 0, 0
			for _, w := range c.Weapons {
				if w.Kind == prof.WeaponKind && w.Tier == tier {
					weapons++
					item, _ := c.Item(w.ID)
					if !c.Allowed(item, prof.ID, 1) {
						t.Fatal("character level gates weapon", w.ID)
					}
					for _, other := range c.Professions {
						if other.ID != prof.ID && c.Allowed(item, other.ID, 10) {
							t.Fatal("foreign weapon allowed")
						}
					}
				}
			}
			for _, s := range c.Skills {
				if s.Kind == "martial" && s.WeaponKind == prof.WeaponKind && s.Tier == tier {
					skills++
				}
			}
			if weapons == 0 || skills == 0 {
				t.Fatal("missing tier", prof.ID, tier)
			}
		}
		drops := map[string]bool{}
		for _, entry := range c.EligibleEntries(pool, prof.ID, 1) {
			drops[entry.Reward.ItemID] = true
		}
		for _, w := range c.Weapons {
			if w.Kind == prof.WeaponKind && !drops[w.ID] {
				t.Fatal("missing weapon in pool", w.ID)
			}
		}
		for _, s := range c.Skills {
			if s.Kind == "martial" && s.WeaponKind == prof.WeaponKind {
				found := false
				for _, item := range c.Items {
					if item.SkillID == s.ID && drops[item.ID] {
						found = true
					}
				}
				if !found {
					t.Fatal("missing martial manual", s.ID)
				}
			}
		}
	}
}

// Opt-in expensive audit: run with WUXIA_SIX_REPORT and optionally WUXIA_HOLDOUT.
// Each cell swaps sides and uses 3000 real battles, never a computed win chance.
func TestSixProfessionMatrix(t *testing.T) {
	path := os.Getenv("WUXIA_SIX_REPORT")
	if path == "" {
		t.Skip("set WUXIA_SIX_REPORT for full candidate matrix")
	}
	c := sixCatalog(t)
	gear := os.Getenv("WUXIA_UNIVERSAL")
	var art []struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
		Tier int    `json:"tier"`
	}
	if gear != "" {
		raw, err := os.ReadFile("../../webui/src/data/universal-equipment-gallery.json")
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &art); err != nil {
			t.Fatal(err)
		}
	}
	offset := uint64(0)
	if os.Getenv("WUXIA_HOLDOUT") != "" {
		offset = 1000000
	}
	rows := []map[string]any{}
	violations := []string{}
	for _, level := range []int{1, 5, 10} {
		for tier := 1; tier <= 5; tier++ {
			for _, equipped := range []bool{false, true} {
				for _, mind := range []bool{false, true} {
					fighters := []Fighter{}
					for _, prof := range c.Professions {
						p := upgradePlayer(c, prof, level)
						for _, w := range c.Weapons {
							if w.Kind == prof.WeaponKind && w.Tier == tier {
								p.Weapons[prof.WeaponKind] = w.ID
								p.Inventory[w.ID] = 1
								break
							}
						}
						p.LearnedSkills = map[string]bool{}
						if equipped {
							for _, s := range c.Skills {
								if s.Kind == "martial" && s.WeaponKind == prof.WeaponKind && s.Tier == tier {
									p.ActiveSkill = s.ID
									p.LearnedSkills[s.ID] = true
									break
								}
							}
						}
						if mind {
							for _, s := range c.Skills {
								if s.Kind == "mind" && s.Tier == tier {
									p.ActiveMind = s.ID
									p.LearnedSkills[s.ID] = true
									break
								}
							}
						}
						if gear != "" {
							branch, err := strconv.Atoi(gear)
							if err != nil || branch < 0 || branch > 5 {
								t.Fatal("invalid universal set")
							}
							p.Equipment = map[string]string{}
							for _, slot := range []string{"armor", "charm"} {
								label := "护具"
								if slot == "charm" {
									label = "饰品"
								}
								n := 0
								for _, a := range art {
									if a.Tier == tier && a.Kind == label {
										if n == branch {
											for _, item := range c.Items {
												if item.Name == a.Name {
													p.Equipment[slot] = item.ID
													break
												}
											}
											break
										}
										n++
									}
								}
							}
						}
						v, err := c.View(p, DefaultSettings())
						if err != nil {
							t.Fatal(err)
						}
						fighters = append(fighters, v.Fighter())
					}
					matrix := [6][6]float64{}
					for i := 0; i < 6; i++ {
						for j := i + 1; j < 6; j++ {
							wins, draws, actions := 0, 0, 0
							for seed := 0; seed < 3000; seed++ {
								a, b := fighters[i], fighters[j]
								swap := seed%2 == 1
								if swap {
									a, b = b, a
								}
								battle := Duel(a, b, offset+uint64(seed))
								actions += len(battle.Actions)
								if battle.Winner < 0 {
									draws++
								} else if (battle.Winner == 0) != swap {
									wins++
								}
							}
							rate := float64(wins) / 3000
							matrix[i][j] = rate
							matrix[j][i] = 1 - rate - float64(draws)/3000
							rows = append(rows, map[string]any{"level": level, "tier": tier, "martial": equipped, "mind": mind, "left": fighters[i].Profession, "right": fighters[j].Profession, "win_rate": rate, "draws": draws, "mean_actions": float64(actions) / 3000, "samples": 3000})
							if rate < .3 || rate > .7 {
								violations = append(violations, fighters[i].Profession+" / "+fighters[j].Profession)
							}
						}
					}
					for i := 0; i < 6; i++ {
						dominant := true
						for j := 0; j < 6; j++ {
							if i != j && matrix[i][j] < .6 {
								dominant = false
							}
						}
						if dominant {
							violations = append(violations, "dominant: "+fighters[i].Profession)
						}
					}
				}
			}
		}
	}
	raw, err := json.MarshalIndent(map[string]any{"content_version": c.Version, "combat_version": DuelVersion, "seed_start": offset, "samples_per_pair": 3000, "universal_set": gear, "scope": "1/5/10级 × 五档同品兵器 × 有无同档武技 × 有无同档心法；universal_set为空时无护具饰品，0～5为图鉴各档六套；每档首门武技；左右互换", "violations": violations, "matches": rows}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("%d balance violations; inspect %s", len(violations), path)
	}
}

func TestSixProfessionPreviewMatchesCandidate(t *testing.T) {
	c := sixCatalog(t)
	raw, err := os.ReadFile("../../webui/public/wuxia-concepts/profession-data.json")
	if err != nil {
		t.Fatal(err)
	}
	var preview Catalog
	if err = json.Unmarshal(raw, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Version != c.Version || !reflect.DeepEqual(preview.Professions, c.Professions) || !reflect.DeepEqual(preview.Weapons, c.Weapons) || !reflect.DeepEqual(preview.Skills, c.Skills) {
		t.Fatal("design preview differs from executable candidate")
	}
	for _, s := range c.Skills {
		if s.Tier == 5 && (s.WeaponKind == "短刃" || s.WeaponKind == "蛊器" || s.WeaponKind == "古琴") {
			lines := strings.Split(s.Description, "\n")
			if len(lines) != 2 || len([]rune(lines[0])) != 7 || len([]rune(lines[1])) != 7 {
				t.Fatal("legendary requires two seven-character lines", s.ID)
			}
		}
	}
}

func TestSixSavedAuditMatchesCandidate(t *testing.T) {
	raw, err := os.ReadFile("internal/content/six_professions.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("six_professions_balance.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Hash     string `json:"candidate_sha256"`
		Training struct {
			Version    string            `json:"combat_version"`
			Violations []string          `json:"violations"`
			Matches    []json.RawMessage `json:"matches"`
		} `json:"training"`
		Holdout struct {
			Version    string            `json:"combat_version"`
			Violations []string          `json:"violations"`
			Seed       int               `json:"seed_start"`
			Matches    []json.RawMessage `json:"matches"`
		} `json:"holdout"`
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Hash != fmt.Sprintf("%x", sha256.Sum256(raw)) || report.Training.Version != DuelVersion || report.Holdout.Version != DuelVersion {
		t.Fatal("candidate or engine changed; rerun candidate audit")
	}
	if len(report.Training.Matches) != 900 || len(report.Holdout.Matches) != 900 || len(report.Training.Violations) > 0 || len(report.Holdout.Violations) > 0 || report.Holdout.Seed != 1000000 {
		t.Fatal("incomplete or failed candidate matrix")
	}
}

func TestUniversalEquipmentTraitsAndAccess(t *testing.T) {
	c := sixCatalog(t)
	coverage := map[string]bool{}
	counts := map[string]int{}
	pool, _ := c.Pool("jianghu")
	for _, item := range c.Items {
		if item.Image == "" || (item.Kind != "armor" && item.Kind != "charm") {
			continue
		}
		counts[fmt.Sprintf("%s-%d", item.Kind, item.Tier)]++
		if item.Traits.Combo > 0 {
			coverage["combo"] = true
		}
		if item.Traits.Poison > 0 {
			coverage["poison"] = true
		}
		if item.Traits.LifeSteal > 0 {
			coverage["life_steal"] = true
		}
		if item.Traits.CriticalDamage > 0 {
			coverage["critical_damage"] = true
		}
		for _, prof := range c.Professions {
			if !c.Allowed(item, prof.ID, 1) {
				t.Fatal("universal item gated", item.ID, prof.ID)
			}
			found := false
			for _, e := range c.EligibleEntries(pool, prof.ID, 1) {
				if e.Reward.ItemID == item.ID {
					found = true
				}
			}
			if !found {
				t.Fatal("universal item absent from loot", item.ID)
			}
			p := upgradePlayer(c, prof, 1)
			p.Equipment = map[string]string{item.Kind: item.ID}
			v, err := c.View(p, DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			baseline, err := c.View(upgradePlayer(c, prof, 1), DefaultSettings())
			if err != nil {
				t.Fatal(err)
			}
			want := baseline.Fighter().Traits.Add(item.Traits)
			if !reflect.DeepEqual(v.Fighter().Traits, want) {
				t.Fatal("equipment trait dropped", item.ID, prof.ID)
			}
		}
	}
	if len(coverage) != 4 {
		t.Fatal("new attributes not covered")
	}
	for _, kind := range []string{"armor", "charm"} {
		for tier := 1; tier <= 5; tier++ {
			if counts[fmt.Sprintf("%s-%d", kind, tier)] != 6 {
				t.Fatal("expected six designed choices per tier", kind, tier)
			}
		}
	}
}

// Parallel profession releases must not leave a class without a tier or with a much smaller pool.
func TestCandidateProfessionTierCoverageParity(t *testing.T) {
	c := sixCatalog(t)
	for tier := 1; tier <= 5; tier++ {
		for _, kind := range []string{"weapon", "martial"} {
			low, high := int(^uint(0)>>1), 0
			for _, p := range c.Professions {
				count := 0
				if kind == "weapon" {
					for _, w := range c.Weapons {
						if w.Kind == p.WeaponKind && w.Tier == tier {
							count++
						}
					}
				} else {
					for _, s := range c.Skills {
						if s.Kind == "martial" && s.WeaponKind == p.WeaponKind && s.Tier == tier {
							count++
						}
					}
				}
				if count < 1 {
					t.Fatalf("%s tier %d %s missing", p.Name, tier, kind)
				}
				low = min(low, count)
				high = max(high, count)
			}
			if high-low > 1 {
				t.Fatalf("tier %d %s coverage diverges: %d..%d", tier, kind, low, high)
			}
		}
	}
}

// Adding choice within a tier must not silently increase rare-drop odds.
func TestUniversalLootTierBudgets(t *testing.T) {
	c := sixCatalog(t)
	pool, _ := c.Pool("jianghu")
	totals := map[string]int{}
	for _, entry := range pool.Entries {
		item, ok := c.Item(entry.Reward.ItemID)
		if ok && (item.Kind == "armor" || item.Kind == "charm") {
			totals[fmt.Sprintf("%s-%d", item.Kind, item.Tier)] += entry.Weight
		}
	}
	for _, kind := range []string{"armor", "charm"} {
		for i, want := range []int{10000, 6000, 3000, 1000, 200} {
			key := fmt.Sprintf("%s-%d", kind, i+1)
			if totals[key] != want {
				t.Fatalf("%s loot budget: got %d want %d", key, totals[key], want)
			}
		}
	}
	if pool.CategoryWeights["armor"] != 25 || pool.CategoryWeights["charm"] != 20 {
		t.Fatal("universal category budgets changed", pool.CategoryWeights)
	}
}

func TestCandidateMartialUsesIndependentChance(t *testing.T) {
	for _, sk := range sixCatalog(t).Skills {
		if sk.Kind != "martial" {
			continue
		}
		if sk.Chance <= 0 || sk.Chance >= 1 || sk.Every != 0 || sk.ReadyTurn != 0 || sk.GuaranteeTurn != 0 || sk.Cooldown != 0 || sk.Limit != 0 {
			t.Fatalf("%s must use independent probability without scheduling or cast caps", sk.Name)
		}
		total, casts, first, repeats, overThree := 0, 0, 0, 0, 0
		for seed := range 300 {
			f := Fighter{Stats: Stats{HP: 1000000, Attack: 1, Speed: 20}, Martial: &sk}
			b := Duel(f, f, uint64(seed))
			seen, used, last := [2]int{}, [2]int{}, [2]bool{}
			for _, a := range b.Actions {
				total++
				seen[a.Actor]++
				cast := a.SkillID == sk.ID
				if cast {
					casts++
					used[a.Actor]++
					if seen[a.Actor] == 1 {
						first++
					}
					if last[a.Actor] {
						repeats++
					}
					if used[a.Actor] > 3 {
						overThree++
					}
				}
				last[a.Actor] = cast
			}
		}
		rate := float64(casts) / float64(total)
		if math.Abs(rate-sk.Chance) > .025 || first == 0 || repeats == 0 || overThree == 0 {
			t.Fatalf("%s probability %.4f want %.4f first=%d consecutive=%d beyond-three=%d", sk.Name, rate, sk.Chance, first, repeats, overThree)
		}
	}
}
