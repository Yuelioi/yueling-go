package bot

import (
	"fmt"
	"sort"
)

// HandlerResult controls whether dispatch continues after a handler runs.
type HandlerResult int

const (
	Continue HandlerResult = iota
	Stop
)

// reg ties a matcher + conditions + typed handler together.
type reg struct {
	eventType  string  // "group_message" | "notice:<type>" | "request:<type>"
	matcher    Matcher // nil for notice/request regs
	conditions []Condition
	handler    any // one of the typed handler funcs below
	priority   int
	block      bool // stop dispatch after this handler regardless of return value
}

// Supported handler signatures — validated at registration time (not compile time).
//
//   func(*CommandContext) error
//   func(*GroupContext) error
//   func(*NoticeContext) error
//   func(*RequestContext) error

func validateHandler(fn any) error {
	switch fn.(type) {
	case func(*CommandContext) error,
		func(*GroupContext) error,
		func(*NoticeContext) error,
		func(*RequestContext) error:
		return nil
	}
	return fmt.Errorf("unsupported handler type %T", fn)
}

func sortRegs(regs []*reg) {
	sort.Slice(regs, func(i, j int) bool {
		return regs[i].priority > regs[j].priority
	})
}

// ---- Builder ----

// Builder is a fluent API for wiring a matcher to a handler.
type Builder struct {
	bot *Bot
	r   *reg
}

func (b *Builder) Where(conds ...Condition) *Builder {
	b.r.conditions = append(b.r.conditions, conds...)
	return b
}

// When adds message-context conditions (e.g. rule.NoReply, rule.NoAt).
func (b *Builder) When(conds ...Condition) *Builder {
	return b.Where(conds...)
}

func (b *Builder) Priority(p int) *Builder {
	b.r.priority = p
	return b
}

// Block stops dispatch after this handler fires, regardless of what it returns.
func (b *Builder) Block() *Builder {
	b.r.block = true
	return b
}

// Handle registers the handler. Panics on unsupported signature.
func (b *Builder) Handle(fn any) {
	if err := validateHandler(fn); err != nil {
		panic(err)
	}
	b.r.handler = fn
	b.bot.addReg(b.r)
}
