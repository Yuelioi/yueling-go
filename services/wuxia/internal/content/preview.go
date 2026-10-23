package content

import (
	_ "embed"
	"sync"
)

//go:embed equipment_preview.json
var previewData []byte

var equipmentPreview = sync.OnceValue(func() Catalog {
	c, err := Parse(previewData)
	if err != nil {
		panic(err)
	}
	return c
})

func EquipmentPreview() Catalog {
	// Callers receive a separate definition graph; previews cannot change live rules.
	return cloneCatalog(equipmentPreview())
}
