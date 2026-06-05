package bot

// Condition gates handler execution based on the event context.
type Condition interface {
	Check(api *BotAPI, msg *MsgCtx) bool
}

// ---- Combinators ----

type andCond struct{ conds []Condition }
type orCond struct{ conds []Condition }
type notCond struct{ cond Condition }

// And requires all conditions to pass.
func And(conds ...Condition) Condition { return &andCond{conds} }

// Or requires at least one condition to pass.
func Or(conds ...Condition) Condition { return &orCond{conds} }

// Not inverts a condition.
func Not(c Condition) Condition { return &notCond{c} }

func (a *andCond) Check(api *BotAPI, msg *MsgCtx) bool {
	for _, c := range a.conds {
		if !c.Check(api, msg) {
			return false
		}
	}
	return true
}

func (o *orCond) Check(api *BotAPI, msg *MsgCtx) bool {
	for _, c := range o.conds {
		if c.Check(api, msg) {
			return true
		}
	}
	return false
}

func (n *notCond) Check(api *BotAPI, msg *MsgCtx) bool {
	return !n.cond.Check(api, msg)
}

// CondFn wraps an inline function as a Condition.
func CondFn(fn func(*BotAPI, *MsgCtx) bool) Condition {
	return condFn(fn)
}

type condFn func(*BotAPI, *MsgCtx) bool

func (f condFn) Check(api *BotAPI, msg *MsgCtx) bool { return f(api, msg) }
