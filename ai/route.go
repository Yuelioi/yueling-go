package ai

import (
	"regexp"
	"sort"
	"strings"
)

const routeTopN = 12

// RouteResult pairs a tool with its relevance score and how it was matched.
type RouteResult struct {
	Tool  *ToolMeta
	Score float64
	Via   string // "R1" | "R2" | "R3"
}

// Route scores all tools against the input text and returns up to routeTopN results
// ordered by descending score.
//
//   R1  Trigger substring match  → 1.0
//   R2  Regex pattern match      → 0.8
//   R3  Slot keyword coverage    → 0.6–0.8
func Route(text string, tools []*ToolMeta) []RouteResult {
	lower := strings.ToLower(text)
	var results []RouteResult

	for _, t := range tools {
		best := RouteResult{Tool: t}

		// R1
		for _, trigger := range t.Triggers {
			if strings.Contains(lower, strings.ToLower(trigger)) {
				best.Score = 1.0
				best.Via = "R1"
				break
			}
		}

		// R2
		if best.Score < 1.0 {
			for _, pat := range t.Patterns {
				re, err := regexp.Compile(pat)
				if err != nil {
					continue
				}
				if re.MatchString(lower) {
					if best.Score < 0.8 {
						best.Score = 0.8
						best.Via = "R2"
					}
					break
				}
			}
		}

		// R3
		if best.Score < 0.8 && len(t.Slots) > 0 {
			matched := 0
			for _, slot := range t.Slots {
				if strings.Contains(lower, strings.ToLower(slot)) {
					matched++
				}
			}
			if matched > 0 {
				score := 0.6 + 0.2*float64(matched)/float64(len(t.Slots))
				if score > best.Score {
					best.Score = score
					best.Via = "R3"
				}
			}
		}

		if best.Score > 0 {
			results = append(results, best)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > routeTopN {
		results = results[:routeTopN]
	}
	return results
}
