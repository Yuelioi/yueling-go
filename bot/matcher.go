package bot

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CmdPrefix is the command prefix checked before bare command names.
// Empty string means no prefix required (commands match as-is).
// Set this from main() after loading config, e.g. bot.CmdPrefix = config.C.Bot.CmdPrefix
var CmdPrefix = ""

// MatchResult carries what a Matcher extracted from the message.
type MatchResult struct {
	Matched bool
	Cmd     string   // set by CommandMatcher
	Args    []string // set by CommandMatcher
	Groups  []string // set by RegexMatcher
}

// Matcher decides whether a MsgCtx should trigger a handler.
type Matcher interface {
	Match(ctx *MsgCtx) MatchResult
}

// ---- CommandMatcher ----

// CommandMatcher triggers on "/cmd", "cmd arg", or exact "cmd".
type CommandMatcher struct {
	cmd     string
	aliases []string
}

func Command(cmd string, aliases ...string) *CommandMatcher {
	return &CommandMatcher{cmd: cmd, aliases: aliases}
}

func (m *CommandMatcher) Match(ctx *MsgCtx) MatchResult {
	text := strings.TrimSpace(ctx.Text())
	for _, c := range append([]string{m.cmd}, m.aliases...) {
		var prefixes []string
		if CmdPrefix != "" {
			prefixes = []string{CmdPrefix + c, c}
		} else {
			prefixes = []string{c}
		}
		for _, prefix := range prefixes {
			if !strings.HasPrefix(text, prefix) {
				continue
			}
			rest := text[len(prefix):]
			// If the next rune is a Han character it's likely a different command
			// (e.g. "添加标签" should not match "添加"), so skip.
			if r, _ := utf8.DecodeRuneInString(rest); unicode.Is(unicode.Han, r) {
				continue
			}
			rest = strings.TrimSpace(rest)
			var args []string
			if rest != "" {
				args = strings.Fields(rest)
			}
			return MatchResult{Matched: true, Cmd: c, Args: args}
		}
	}
	return MatchResult{}
}

// ---- KeywordMatcher ----

type KeywordMatcher struct {
	keywords []string
}

func Keyword(kws ...string) *KeywordMatcher {
	return &KeywordMatcher{keywords: kws}
}

func (m *KeywordMatcher) Match(ctx *MsgCtx) MatchResult {
	text := ctx.Text()
	for _, kw := range m.keywords {
		if strings.Contains(text, kw) {
			return MatchResult{Matched: true}
		}
	}
	return MatchResult{}
}

// ---- RegexMatcher ----

type RegexMatcher struct {
	re *regexp.Regexp
}

func Regex(pattern string) *RegexMatcher {
	return &RegexMatcher{re: regexp.MustCompile(pattern)}
}

func (m *RegexMatcher) Match(ctx *MsgCtx) MatchResult {
	groups := m.re.FindStringSubmatch(ctx.Text())
	if groups == nil {
		return MatchResult{}
	}
	return MatchResult{Matched: true, Groups: groups}
}

// ---- FullMatchMatcher ----

// FullMatchMatcher triggers only when the trimmed message exactly equals one of the keywords.
type FullMatchMatcher struct {
	keywords []string
}

func FullMatch(keywords ...string) *FullMatchMatcher {
	return &FullMatchMatcher{keywords: keywords}
}

func (m *FullMatchMatcher) Match(ctx *MsgCtx) MatchResult {
	text := strings.TrimSpace(ctx.Text())
	for _, kw := range m.keywords {
		if text == kw {
			return MatchResult{Matched: true}
		}
	}
	return MatchResult{}
}

// ---- AnyMatcher ----

type anyMatcher struct{}

// Any matches every group message (use as fallback / catch-all).
var Any Matcher = anyMatcher{}

func (anyMatcher) Match(_ *MsgCtx) MatchResult { return MatchResult{Matched: true} }
