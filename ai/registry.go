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
	Type        string // "string" | "integer" | "boolean" | "number" | "array"
	Description string
	Required    bool
	Enum        []string // optional closed set for string parameters
	ItemsType   string   // item JSON type when Type is "array"
	MinItems    int
	MaxItems    int
}

// ToolMeta is the full descriptor for an AI-callable tool.
type ToolMeta struct {
	Name            string
	Description     string
	Requires        []string // Prerequisite tools, still subject to current permissions and plugin switches.
	Tags            []string
	Triggers        []string // R1: exact substrings → score 1.0
	Patterns        []string // R2: regex patterns  → score 0.8
	Slots           []string // R3: semantic keywords
	PluginID        int      // catalog plugin switch controlling this tool; 0 means always available
	Permission      PermLevel
	Risk            RiskLevel
	ConfirmRequired bool
	ReadOnly        bool // Explicitly excludes business side effects; legacy tools default to unknown.
	Params          []Param
	ActionKey       ToolActionKey
	Handler         ToolHandler
	ResultHandler   func(*ToolContext) ToolResult
}

type ToolHandler func(ctx *ToolContext) (string, error)

// ToolActionKey identifies equivalent business actions within one turn. It
// receives validated arguments and must not mutate them or perform I/O. An empty
// key keeps the default parsed-JSON identity for actions it does not normalize.
type ToolActionKey func(params map[string]any) string

// Schema converts ToolMeta to an OpenAI function tool.
func (t *ToolMeta) Schema() openai.Tool { return t.schema() }

func (t *ToolMeta) schema() openai.Tool {
	props := map[string]any{}
	var required []string
	for _, p := range t.Params {
		property := map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if len(p.Enum) > 0 {
			property["enum"] = p.Enum
		}
		if p.Type == "array" && p.ItemsType != "" {
			property["items"] = map[string]any{"type": p.ItemsType}
		}
		if p.MinItems > 0 {
			property["minItems"] = p.MinItems
		}
		if p.MaxItems > 0 {
			property["maxItems"] = p.MaxItems
		}
		props[p.Name] = property
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schemaMap := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
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

type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolMeta
	order []string
}

func NewRegistry() *Registry { return &Registry{tools: map[string]*ToolMeta{}} }

// Register preserves insertion order when replacing an existing definition.
func (r *Registry) Register(meta ToolMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = map[string]*ToolMeta{}
	}
	if _, exists := r.tools[meta.Name]; !exists {
		r.order = append(r.order, meta.Name)
	}
	r.tools[meta.Name] = &meta
}

// AllTools returns all registered tools in insertion order.
func (r *Registry) AllTools() []*ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ToolMeta, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// GetTool looks up a tool by name.
func (r *Registry) GetTool(name string) (*ToolMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}
