package group

import "strings"

type joinDecision int

const (
	decisionNone joinDecision = iota
	decisionApprove
	decisionReject
)

func decideJoin(comment string, allow, deny []string) joinDecision {
	if comment == "" {
		return decisionNone
	}
	for _, kw := range deny {
		if kw != "" && strings.Contains(comment, kw) {
			return decisionReject
		}
	}
	for _, kw := range allow {
		if kw == "*" || (kw != "" && strings.Contains(comment, kw)) {
			return decisionApprove
		}
	}
	return decisionNone
}

func parseKeywordArg(raw string) (add bool, keywords []string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil, false
	}
	switch raw[0] {
	case '+':
		add = true
	case '-':
		add = false
	default:
		return false, nil, false
	}
	rest := strings.ReplaceAll(raw[1:], "，", ",")
	for _, p := range strings.Split(rest, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			keywords = append(keywords, p)
		}
	}
	if len(keywords) == 0 {
		return false, nil, false
	}
	return add, keywords, true
}
