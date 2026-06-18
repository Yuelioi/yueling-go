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
