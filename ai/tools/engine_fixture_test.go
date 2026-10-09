package tools

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/scheduler"
)

var testEngine = ai.NewEngine(ai.EngineDependencies{})
var testTools = &toolset{registry: testEngine.Registry, schedules: scheduler.New(nil)}

func init() { testTools.registerAll() }
