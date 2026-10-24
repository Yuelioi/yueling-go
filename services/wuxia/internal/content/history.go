package content

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed archive/wuxia-3.json
var legacyDrawContent []byte

//go:embed archive/wuxia-4.json
var previousContent []byte

//go:embed archive/wuxia-5.json
var adventureContent []byte

//go:embed archive/wuxia-6.json
var martialPrevious []byte

//go:embed archive/wuxia-7.json
var martialInitial []byte

//go:embed archive/wuxia-8.json
var martialExpanded []byte

func Historical(version string) (Catalog, error) {
	data := legacyDrawContent
	if version == "wuxia-8" {
		data = martialExpanded
	} else if version == "wuxia-7" {
		data = martialInitial
	} else if version == "wuxia-6" {
		data = martialPrevious
	} else if version == "wuxia-5" {
		data = adventureContent
	} else if version == "wuxia-4" {
		data = previousContent
	} else if version != "wuxia-3" {
		return Catalog{}, fmt.Errorf("缺少历史内容版本: %s", version)
	}
	var c Catalog
	err := json.Unmarshal(data, &c)
	return c, err
}
