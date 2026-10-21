package tools

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/scheduler"
)

var testEngine = ai.NewEngine(ai.EngineDependencies{})
var testTools = &toolset{registry: testEngine.Registry, schedules: scheduler.New(nil)}

func init() {
	testTools.registerAll()
	testTools.registerImageTools()
	testTools.registerAnalysisTools()
	testTools.registerChatInsightsTools()
	testTools.registerDigestTools()
	testEngine.Registry.Register(FeedTool(nil))
	testTools.registerFunTools()
	testTools.registerGameDealsTools()
	testTools.registerGroupAdminTools()
	testTools.registerProductivityTools()
	testTools.registerProfileTools()
	testTools.registerQQTools()
	testTools.registerReminderTools()
	testTools.registerTravelSearchTools()
	testTools.registerSocialTools()
	testTools.registerUtilityTools()
	testTools.registerWeatherTools()
}
