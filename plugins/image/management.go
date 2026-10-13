package image

import (
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/imageassets"
)

// AssetCategories resolves local and external sources for the admin library.
func AssetCategories(entries []config.ImageEntry) []imageassets.Category {
	if len(entries) == 0 {
		entries = defaultEntries
	}
	out := []imageassets.Category{{Name: "语录", Upload: true, NameRequired: true, Naming: "quotation"}, {Name: "表情", Aliases: []string{"表情包"}, Upload: true, NameRequired: true, Naming: "optional_label"}}
	for _, e := range entries {
		name := e.Folder
		if name == "" && len(e.Call) > 0 {
			name = e.Call[0]
		}
		if name == "" {
			continue
		}
		aliases := append([]string(nil), e.Call...)
		if e.Add != "" {
			aliases = append(aliases, e.Add)
		}
		naming := "hash"
		if argRequired(e) {
			naming = "label"
		}
		out = append(out, imageassets.Category{Name: name, Aliases: aliases, External: kindOf(e) == config.KindExternal, Upload: e.Add != "" && kindOf(e) != config.KindExternal, NameRequired: argRequired(e), Naming: naming})
	}
	return out
}
