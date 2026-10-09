package ai

import (
	"regexp"
	"sort"
	"strings"
)

const routeTopN = 8

// RouteResult pairs a tool with its relevance score and how it was matched.
type RouteResult struct {
	Tool  *ToolMeta
	Score float64
	Via   string // "R1" | "R2" | "R3"
}

// Route scores all tools against the input text and returns up to routeTopN results
// ordered by descending score.
//
//	R1  Trigger substring match  → 1.0
//	R2  Regex pattern match      → 0.8
//	R3  Slot keyword coverage    → 0.6–0.8
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

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > routeTopN {
		results = results[:routeTopN]
	}
	return results
}

// routeTools carries a bounded candidate set across clarification turns. Names
// are resolved against the current allowlist, never against stale descriptors.
// An explicit new route replaces the previous topic; session reset/TTL clears it.
func (s *Session) routeTools(text string, allowed []*ToolMeta) []RouteResult {
	byName := make(map[string]*ToolMeta, len(allowed))
	for _, tool := range allowed {
		byName[tool.Name] = tool
	}
	results := Route(text, allowed)
	if len(results) == 0 {
		for _, name := range s.routeCandidates {
			if tool := byName[name]; tool != nil {
				results = append(results, RouteResult{Tool: tool, Via: "context"})
			}
		}
	}
	s.routeCandidates = nil
	seen := map[string]bool{}
	for _, result := range results {
		seen[result.Tool.Name] = true
		s.routeCandidates = append(s.routeCandidates, result.Tool.Name)
	}
	// Resolve dependencies transitively with deduplication and a bounded schema set.
	// Prior candidates exclude dependencies, so a changed topic cannot retain them.
	for i := 0; i < len(results) && len(results) < 2*routeTopN; i++ {
		for _, name := range results[i].Tool.Requires {
			if tool := byName[name]; tool != nil && !seen[name] {
				results = append(results, RouteResult{Tool: tool, Via: "required"})
				seen[name] = true
				if len(results) == 2*routeTopN {
					break
				}
			}
		}
	}
	return results
}
