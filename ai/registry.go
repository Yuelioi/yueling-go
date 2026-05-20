package ai

import (
	"encoding/json"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

type PermLevel int

const (
	PermMember    PermLevel = 0
	PermTrusted   PermLevel = 1
	PermAdmin     PermLevel = 2
	PermOwner     PermLevel = 3
	PermSuperUser PermLevel = 4
)

type RiskLevel int

const (
	RiskLow    RiskLevel = 0
	RiskMedium RiskLevel = 1
	RiskHigh   RiskLevel = 2
)

// Param describes one parameter in a tool's JSON schema.
type Param struct {
	Name        string
	Type        string // "string" | "integer" | "boolean" | "number"
	Description string
	Required    bool
}

// ToolMeta is the full descriptor for an AI-callable tool.
type ToolMeta struct {
	Name            string
	Description     string
	Tags            []string
	Triggers        []string  // R1: exact substrings → score 1.0
	Patterns        []string  // R2: regex patterns  → score 0.8
	Slots           []string  // R3: semantic keywords
	Permission      PermLevel
	Risk            RiskLevel
	ConfirmRequired bool
	Params          []Param
	Handler         ToolHandler
}

type ToolHandler func(ctx *ToolContext) (string, error)

// Schema converts ToolMeta to an OpenAI function tool.
func (t *ToolMeta) Schema() openai.Tool { return t.schema() }

func (t *ToolMeta) schema() openai.Tool {
	props := map[string]any{}
	var required []string
	for _, p := range t.Params {
		props[p.Name] = map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schemaMap := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schemaMap["required"] = required
	}
	raw, _ := json.Marshal(schemaMap)
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  json.RawMessage(raw),
		},
	}
}

// ---- Global registry ----

type registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolMeta
	order []string
}

var global = &registry{tools: map[string]*ToolMeta{}}

// Register adds a tool to the global registry.
// Call from plugin init() functions.
func Register(meta ToolMeta) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.tools[meta.Name] = &meta
	global.order = append(global.order, meta.Name)
}

// AllTools returns all registered tools in insertion order.
func AllTools() []*ToolMeta {
	global.mu.RLock()
	defer global.mu.RUnlock()
	out := make([]*ToolMeta, 0, len(global.order))
	for _, name := range global.order {
		out = append(out, global.tools[name])
	}
	return out
}

// GetTool looks up a tool by name.
func GetTool(name string) (*ToolMeta, bool) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	t, ok := global.tools[name]
	return t, ok
}
