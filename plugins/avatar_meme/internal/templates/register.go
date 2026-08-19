package templates

import (
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/model"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/handshake_wash"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/mocking_grid"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/screen_reaction"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/single_plan"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/spiderman_glasses"
	"github.com/Yuelioi/yueling-go/plugins/avatar_meme/internal/templates/yugioh_card"
)

// All explicitly lists locally owned templates.
func All() []model.Template {
	return []model.Template{
		single_plan.New(),
		handshake_wash.New(),
		mocking_grid.New(),
		screen_reaction.New(),
		spiderman_glasses.New(),
		yugioh_card.New(),
	}
}
