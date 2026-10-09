package ai

import (
	"time"

	"github.com/Yuelioi/yueling-go/db"
	model "github.com/Yuelioi/yueling-go/services/llm"
)

// Engine owns conversation state; each application registers its own tools.
type Engine struct {
	Conversations ConversationStore
	Groups        *db.GroupStore
	Affinity      *AffinityManager
	Chats         *db.ChatStore
	Memory        *MemoryManager
	modelBackend  model.Completer
	limiter       *aiLimiter
	Registry      *Registry
	Sessions      *SessionManager
	Confirms      *ConfirmManager
	Proactive     *ProactiveManager
}

type EngineDependencies struct {
	Conversations ConversationStore
	Groups        *db.GroupStore
	Memory        *MemoryManager
	Chats         *db.ChatStore
	Affinity      *AffinityManager
	ModelBackend  model.Completer
}

func NewEngine(deps EngineDependencies) *Engine {
	if deps.Memory == nil {
		deps.Memory = NewMemoryManager(nil)
	}
	if deps.Affinity == nil {
		deps.Affinity = NewAffinityManager(nil)
	}
	return &Engine{Conversations: deps.Conversations, Groups: deps.Groups, Memory: deps.Memory, Chats: deps.Chats, Affinity: deps.Affinity, modelBackend: deps.ModelBackend,
		limiter: newAILimiter(0, 0, time.Minute), Registry: NewRegistry(), Sessions: &SessionManager{},
		Confirms: &ConfirmManager{pending: map[string]*PendingAction{}}, Proactive: &ProactiveManager{groups: deps.Groups, affinity: deps.Affinity, heats: map[int64]*groupHeat{}}}
}
