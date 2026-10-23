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

func Historical(version string) (Catalog, error) {
	data := legacyDrawContent
	if version == "wuxia-5" {
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
