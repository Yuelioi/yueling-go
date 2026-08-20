package avatar_meme

import (
	"fmt"
	"strings"

	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
)

type registry struct {
	templates []model.Template
	keywords  map[string]model.Template
}

func newRegistry(templates []model.Template) (*registry, error) {
	r := &registry{templates: append([]model.Template(nil), templates...), keywords: map[string]model.Template{}}
	keys := map[string]struct{}{}
	for _, template := range templates {
		if template == nil {
			return nil, fmt.Errorf("头像表情模板不能为空")
		}
		spec := template.Spec()
		if strings.TrimSpace(spec.Key) == "" || len(spec.Keywords) == 0 {
			return nil, fmt.Errorf("模板必须提供 key 和至少一个关键词")
		}
		if _, exists := keys[spec.Key]; exists {
			return nil, fmt.Errorf("模板 key 重复: %s", spec.Key)
		}
		keys[spec.Key] = struct{}{}
		if spec.MinImages < 0 || spec.MaxImages < spec.MinImages || spec.MinTexts < 0 || spec.MaxTexts < spec.MinTexts || len(spec.DefaultTexts) > spec.MaxTexts {
			return nil, fmt.Errorf("模板 %s 的图片或文字数量约束无效", spec.Key)
		}
		for _, keyword := range spec.Keywords {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				return nil, fmt.Errorf("模板 %s 含空关键词", spec.Key)
			}
			if _, exists := r.keywords[keyword]; exists {
				return nil, fmt.Errorf("头像表情关键词重复: %s", keyword)
			}
			r.keywords[keyword] = template
		}
	}
	return r, nil
}
