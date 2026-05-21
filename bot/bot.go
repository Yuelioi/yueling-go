package bot

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Bot is the central hub: holds all registrations and manages the NapCat connection.
// It connects TO NapCat as a WebSocket client (forward WS mode).
type Bot struct {
	regs         []*reg
	connectHooks []func(*BotAPI)
}

// OnConnect registers a callback invoked (in a new goroutine) on each successful connection.
func (b *Bot) OnConnect(fn func(*BotAPI)) {
	b.connectHooks = append(b.connectHooks, fn)
}

// New returns a ready-to-use Bot.
func New() *Bot { return &Bot{} }

func (b *Bot) addReg(r *reg) {
	b.regs = append(b.regs, r)
	sortRegs(b.regs)
}

// ---- Registration helpers ----

// OnCommand registers a handler triggered by a slash/prefix command.
func (b *Bot) OnCommand(cmd string, aliases ...string) *Builder {
	return &Builder{bot: b, r: &reg{
		eventType: "group_message",
		matcher:   Command(cmd, aliases...),
		priority:  10,
	}}
}

// OnGroupMessage registers a handler for group messages.
// Pass a Matcher to filter; omit for catch-all.
func (b *Bot) OnGroupMessage(m ...Matcher) *Builder {
	var matcher Matcher = Any
	if len(m) > 0 {
		matcher = m[0]
	}
	return &Builder{bot: b, r: &reg{
		eventType: "group_message",
		matcher:   matcher,
		priority:  10,
	}}
}

// OnKeyword is shorthand for OnGroupMessage(Keyword(kws...)).
func (b *Bot) OnKeyword(kws ...string) *Builder {
	return b.OnGroupMessage(Keyword(kws...))
}

// OnRegex is shorthand for OnGroupMessage(Regex(pattern)).
func (b *Bot) OnRegex(pattern string) *Builder {
	return b.OnGroupMessage(Regex(pattern))
}

// OnFullMatch registers a handler triggered only when the message exactly matches one of the keywords.
func (b *Bot) OnFullMatch(keywords ...string) *Builder {
	return &Builder{bot: b, r: &reg{
		eventType: "group_message",
		matcher:   FullMatch(keywords...),
		priority:  10,
	}}
}

// OnNotice registers a handler for a specific notice type (e.g. "poke").
// Pass "" to handle all notice events.
func (b *Bot) OnNotice(noticeType string) *Builder {
	key := "notice"
	if noticeType != "" {
		key = "notice:" + noticeType
	}
	return &Builder{bot: b, r: &reg{eventType: key, priority: 10}}
}

// OnRequest registers a handler for friend/group-join requests.
func (b *Bot) OnRequest(requestType string) *Builder {
	key := "request"
	if requestType != "" {
		key = "request:" + requestType
	}
	return &Builder{bot: b, r: &reg{eventType: key, priority: 10}}
}

// ---- WebSocket client ----

// Start connects to NapCat as a client and blocks until the process exits.
// It reconnects automatically on disconnect.
func (b *Bot) Start(url, token string) {
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}

	backoff := time.Second
	for {
		log.Printf("[bot] connecting to %s", url)
		if err := b.connect(url, header); err != nil {
			log.Printf("[bot] disconnected: %v — retry in %s", err, backoff)
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (b *Bot) connect(url string, header http.Header) error {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		return err
	}
	log.Printf("[bot] connected to NapCat")
	return b.handleConn(conn)
}

// ---- WebSocket server ----

// Serve starts a WebSocket server so NapCat can connect to the bot (reverse WS mode).
// It blocks until the process exits; each incoming connection is handled concurrently.
func (b *Bot) Serve(addr, token string) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/onebot/v11/ws", func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[bot] ws upgrade error: %v", err)
			return
		}
		log.Printf("[bot] NapCat connected from %s", r.RemoteAddr)
		if err := b.handleConn(conn); err != nil {
			log.Printf("[bot] connection closed: %v", err)
		}
	})
	log.Printf("[bot] serving reverse WS on %s/onebot/v11/ws", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[bot] server error: %v", err)
	}
}

// handleConn runs send/recv loops for an established WebSocket connection.
func (b *Bot) handleConn(conn *websocket.Conn) error {
	sendCh := make(chan []byte, 256)
	api := &BotAPI{sendCh: sendCh}
	go b.sendLoop(conn, sendCh)
	return b.recvLoop(conn, api, sendCh)
}

func (b *Bot) sendLoop(conn *websocket.Conn, ch <-chan []byte) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[bot] write error: %v", err)
				return
			}
		case <-tick.C:
			conn.WriteMessage(websocket.PingMessage, nil)
		}
	}
}

func (b *Bot) recvLoop(conn *websocket.Conn, api *BotAPI, sendCh chan []byte) error {
	defer close(sendCh)
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		go b.dispatch(api, raw)
	}
}

// ---- Dispatch ----

func (b *Bot) dispatch(api *BotAPI, raw []byte) {
	var base rawEvent
	if err := json.Unmarshal(raw, &base); err != nil {
		return
	}

	// API response — deliver to waiting caller.
	if base.Echo != "" {
		var resp struct {
			Data json.RawMessage `json:"data"`
			Echo string          `json:"echo"`
		}
		if json.Unmarshal(raw, &resp) == nil {
			api.deliver(resp.Echo, resp.Data)
		}
		return
	}

	switch base.PostType {
	case "message":
		if base.MessageType == "group" {
			var e GroupMessageEvent
			if json.Unmarshal(raw, &e) == nil {
				b.dispatchGroupMessage(api, &e)
			}
		}
	case "notice":
		var e NoticeEvent
		if json.Unmarshal(raw, &e) == nil {
			b.dispatchNotice(api, &e)
		}
	case "request":
		var e RequestEvent
		if json.Unmarshal(raw, &e) == nil {
			b.dispatchRequest(api, &e)
		}
	case "meta_event":
		var me struct {
			MetaEventType string `json:"meta_event_type"`
			SelfID        int64  `json:"self_id"`
		}
		if json.Unmarshal(raw, &me) == nil && me.MetaEventType == "lifecycle" && me.SelfID != 0 {
			api.SelfID = me.SelfID
			log.Printf("[bot] self_id set to %d", me.SelfID)
			for _, fn := range b.connectHooks {
				go fn(api)
			}
		}
	}
}

func (b *Bot) dispatchGroupMessage(api *BotAPI, e *GroupMessageEvent) {
	if e.UserID == e.SelfID {
		return
	}
	log.Printf("[msg] group=%d user=%d text=%q", e.GroupID, e.UserID, e.Message.Text())
	msgCtx := &MsgCtx{Event: e}

	for _, r := range b.regs {
		if r.eventType != "group_message" {
			continue
		}
		mr := r.matcher.Match(msgCtx)
		if !mr.Matched {
			continue
		}
		if !checkConditions(r.conditions, api, msgCtx) {
			continue
		}

		var result HandlerResult
		switch h := r.handler.(type) {
		case func(*CommandContext) error:
			ctx := &CommandContext{
				GroupContext: &GroupContext{BotAPI: api, MsgCtx: msgCtx},
				Cmd:          mr.Cmd,
				Args:         mr.Args,
			}
			if err := h(ctx); err != nil {
				log.Printf("[bot] handler error: %v", err)
				result = Stop
			}
		case func(*GroupContext) error:
			ctx := &GroupContext{BotAPI: api, MsgCtx: msgCtx}
			if err := h(ctx); err != nil {
				log.Printf("[bot] handler error: %v", err)
				result = Stop
			}
		}

		if result == Stop || r.block {
			return
		}
	}
}

func (b *Bot) dispatchNotice(api *BotAPI, e *NoticeEvent) {
	ctx := &NoticeContext{BotAPI: api, Event: e}
	specific := "notice:" + e.NoticeType

	for _, r := range b.regs {
		if r.eventType != "notice" && r.eventType != specific {
			continue
		}
		if h, ok := r.handler.(func(*NoticeContext) error); ok {
			if err := h(ctx); err != nil {
				log.Printf("[bot] notice handler error: %v", err)
			}
		}
		if r.block {
			return
		}
	}
}

func (b *Bot) dispatchRequest(api *BotAPI, e *RequestEvent) {
	ctx := &RequestContext{BotAPI: api, Event: e}
	specific := "request:" + e.RequestType

	for _, r := range b.regs {
		if r.eventType != "request" && r.eventType != specific {
			continue
		}
		if h, ok := r.handler.(func(*RequestContext) error); ok {
			if err := h(ctx); err != nil {
				log.Printf("[bot] request handler error: %v", err)
			}
		}
		if r.block {
			return
		}
	}
}

func checkConditions(conds []Condition, api *BotAPI, msg *MsgCtx) bool {
	for _, c := range conds {
		if !c.Check(api, msg) {
			return false
		}
	}
	return true
}
