package tools

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/scheduler"
	"github.com/Yuelioi/yueling-go/services/feed"
	"github.com/Yuelioi/yueling-go/services/knowledge"
)

type Dependencies struct {
	Knowledge *knowledge.Manager
}

type toolset struct {
	chats     *db.ChatStore
	memory    *ai.MemoryManager
	todos     *db.TodoStore
	knowledge *knowledge.Manager
	registry  *ai.Registry
	schedules *scheduler.Manager
	feeds     *feed.Manager
}

func RegisterAll(registry *ai.Registry, deps Dependencies) {
	(&toolset{registry: registry, knowledge: deps.Knowledge}).registerAll()
}

func (t *toolset) registerAll() {
	t.registerImageGeneration()
	t.registerImageInspection()
	t.registerContextTools()
	t.registerGroupKnowledgeTools()
	t.registerSearchTools()
}
