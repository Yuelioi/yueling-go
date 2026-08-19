package templates

import (
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/single_plan"
)

// All explicitly lists locally owned templates. Template packages and their
// embedded assets will be added here as the user supplies source images.
func All() []model.Template {
	return []model.Template{
		single_plan.New(),
	}
}
