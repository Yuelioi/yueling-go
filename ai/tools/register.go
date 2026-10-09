package tools

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/scheduler"
	"github.com/Yuelioi/yueling-go/services/feed"
	"github.com/Yuelioi/yueling-go/services/knowledge"
)

type Dependencies struct {
	Chats     *db.ChatStore
	Memory    *ai.MemoryManager
	Feeds     *feed.Manager
	Schedules *scheduler.Manager
	Knowledge *knowledge.Manager
	Todos     *db.TodoStore
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
	(&toolset{chats: deps.Chats, memory: deps.Memory, registry: registry, feeds: deps.Feeds, schedules: deps.Schedules, knowledge: deps.Knowledge, todos: deps.Todos}).registerAll()
}

func (t *toolset) registerAll() {
	t.registerAnalysisTools()
	t.registerChatInsightsTools()
	t.registerContextTools()
	t.registerDigestTools()
	t.registry.Register(FeedTool(t.feeds))
	t.registerFunTools()
	t.registerGameDealsTools()
	t.registerGroupAdminTools()
	t.registerGroupKnowledgeTools()
	t.registerProductivityTools()
	t.registerProfileTools()
	t.registerQQTools()
	t.registerReminderTools()
	t.registerSearchTools()
	t.registerSocialTools()
	t.registerUtilityTools()
	t.registerWeatherTools()
}
