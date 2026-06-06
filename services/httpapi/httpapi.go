package httpapi

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/logx"
)

// Sender is the subset of *bot.BotAPI the HTTP API needs.
type Sender interface {
	SendGroupMsg(groupID int64, msg bot.Message) (int32, error)
	SendPrivateMsg(userID int64, msg bot.Message) (int32, error)
}

// Server exposes an authenticated HTTP endpoint for sending messages.
// It holds the latest per-connection BotAPI, refreshed on every (re)connect.
type Server struct {
	key     string
	current atomic.Pointer[bot.BotAPI]
	resolve func() Sender
}

func New(key string) *Server {
	s := &Server{key: key}
	s.resolve = func() Sender {
		api := s.current.Load()
		if api == nil { // avoid returning a typed-nil interface
			return nil
		}
		return api
	}
	return s
}

// BindBot wires the server to the bot so each (re)connection refreshes the live BotAPI.
// Must be called before bot.Start (registration-before-start rule).
func (s *Server) BindBot(b *bot.Bot) {
	b.OnConnect(func(a *bot.BotAPI) { s.current.Store(a) })
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/send", s.handleSend)
	return mux
}

func (s *Server) Start(addr string) {
	logx.Infof("[httpapi] serving on %s", addr)
	if err := http.ListenAndServe(addr, s.Handler()); err != nil {
		logx.Fatalf("[httpapi] server error: %v", err)
	}
}

type sendRequest struct {
	MessageType string      `json:"message_type"`
	GroupID     int64       `json:"group_id"`
	UserID      int64       `json:"user_id"`
	Message     bot.Message `json:"message"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			logx.Errorf("[httpapi] panic: %v", rec)
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
	}()

	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.key {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Message) == 0 {
		writeErr(w, http.StatusBadRequest, "message is empty")
		return
	}
	// Reject invalid UTF-8 up front: bot.Message segments carry json.RawMessage,
	// which Go's decoder does NOT validate. Forwarding bad bytes to NapCat makes it
	// drop the whole WebSocket (close 1007), taking the bot offline for every request.
	for _, seg := range req.Message {
		if !utf8.Valid(seg.Data) {
			writeErr(w, http.StatusBadRequest, "message contains invalid utf-8")
			return
		}
	}
	switch req.MessageType {
	case "group":
		if req.GroupID == 0 {
			writeErr(w, http.StatusBadRequest, "group_id required")
			return
		}
	case "private":
		if req.UserID == 0 {
			writeErr(w, http.StatusBadRequest, "user_id required")
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, "invalid message_type")
		return
	}

	sender := s.resolve()
	if sender == nil {
		writeErr(w, http.StatusServiceUnavailable, "bot not connected")
		return
	}

	var (
		msgID int32
		err   error
	)
	if req.MessageType == "group" {
		msgID, err = sender.SendGroupMsg(req.GroupID, req.Message)
	} else {
		msgID, err = sender.SendPrivateMsg(req.UserID, req.Message)
	}
	if err != nil {
		logx.Warnf("[httpapi] send failed: %v", err)
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message_id": msgID})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"ok": false, "error": msg})
}
