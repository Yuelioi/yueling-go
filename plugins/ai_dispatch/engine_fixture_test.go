package ai_dispatch

import (
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/ai/tools"
)

var testEngine = ai.NewEngine(ai.EngineDependencies{})

func init() { tools.RegisterAll(testEngine.Registry, tools.Dependencies{}) }
