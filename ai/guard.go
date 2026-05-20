package ai

import "strings"

// GuardResult is the outcome of a security check.
type GuardResult int

const (
	GuardAllow          GuardResult = iota
	GuardBlockInjection             // prompt injection attempt
	GuardBlockPerm                  // high-risk action by non-admin
)

// Prompt injection patterns (case-insensitive).
var injectionPatterns = []string{
	"ignore previous", "ignore all", "disregard",
	"你现在是", "你是一个", "system:", "system prompt",
	"act as", "pretend you are", "jailbreak",
	"forget your instructions",
}

// Keywords that require admin+ even if the matched tool allows lower permission.
var highRiskKeywords = []string{
	"禁言", "ban", "踢出", "kick", "撤回", "重启", "reboot",
	"全员禁言", "shutdown", "rm -", "drop table",
}

// Guard checks the user input for injection attempts and unauthorised high-risk intent.
func Guard(text, role string) GuardResult {
	lower := strings.ToLower(text)

	for _, p := range injectionPatterns {
		if strings.Contains(lower, p) {
			return GuardBlockInjection
		}
	}

	if role == "member" {
		for _, kw := range highRiskKeywords {
			if strings.Contains(lower, kw) {
				return GuardBlockPerm
			}
		}
	}

	return GuardAllow
}
