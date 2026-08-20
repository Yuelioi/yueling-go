package avatar_meme

import (
	"context"
	"reflect"
	"testing"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/services/imaging"
)

func TestTemplateSpecDoesNotChooseImageSources(t *testing.T) {
	if _, exists := reflect.TypeOf(model.TemplateSpec{}).FieldByName("AllowAvatarFallback"); exists {
		t.Fatal("TemplateSpec exposes image-source policy; templates should only declare image counts")
	}
}

type fakeTemplate struct{ spec model.TemplateSpec }

func (f fakeTemplate) Spec() model.TemplateSpec { return f.spec }
func (f fakeTemplate) Render(context.Context, model.RenderRequest) (*imaging.Animation, error) {
	return nil, nil
}

func TestRegistryValidation(t *testing.T) {
	valid := fakeTemplate{model.TemplateSpec{Key: "one", Keywords: []string{"模板一"}, MinImages: 1, MaxImages: 1}}
	if _, err := newRegistry([]model.Template{valid}); err != nil {
		t.Fatalf("valid template rejected: %v", err)
	}
	duplicate := fakeTemplate{model.TemplateSpec{Key: "two", Keywords: []string{"模板一"}}}
	if _, err := newRegistry([]model.Template{valid, duplicate}); err == nil {
		t.Fatal("duplicate keyword accepted")
	}
	badRange := fakeTemplate{model.TemplateSpec{Key: "bad", Keywords: []string{"坏模板"}, MinImages: 2, MaxImages: 1}}
	if _, err := newRegistry([]model.Template{badRange}); err == nil {
		t.Fatal("invalid image range accepted")
	}
}

func TestResolveTexts(t *testing.T) {
	spec := model.TemplateSpec{MinTexts: 1, MaxTexts: 2, DefaultTexts: []string{"默认"}}
	got, err := resolveTexts([]string{"第一段", "|", "第二段"}, spec)
	if err != nil || len(got) != 2 || got[0] != "第一段" || got[1] != "第二段" {
		t.Fatalf("resolveTexts=%q err=%v", got, err)
	}
	got, err = resolveTexts(nil, spec)
	if err != nil || len(got) != 1 || got[0] != "默认" {
		t.Fatalf("default texts=%q err=%v", got, err)
	}
	if _, err := resolveTexts([]string{"只有一段"}, model.TemplateSpec{
		MinTexts: 2, MaxTexts: 2, DefaultTexts: []string{"默认一", "默认二"},
	}); err == nil {
		t.Fatal("partial text input was silently replaced by defaults")
	}
}
