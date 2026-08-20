package model

import (
	"context"

	"github.com/Yuelioi/yueling-go/services/imaging"
)

// TemplateSpec describes one locally owned template and its command contract.
type TemplateSpec struct {
	Key          string
	Description  string
	Keywords     []string
	MinImages    int
	MaxImages    int
	MinTexts     int
	MaxTexts     int
	DefaultTexts []string
}

type RenderRequest struct {
	Images  []*imaging.Animation
	Names   []string
	Texts   []string
	Options map[string]string
}

// Template is the complete seam between Bot command handling and a local renderer.
type Template interface {
	Spec() TemplateSpec
	Render(context.Context, RenderRequest) (*imaging.Animation, error)
}
