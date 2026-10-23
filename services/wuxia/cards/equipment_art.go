package cards

import (
	"bytes"
	"embed"
	"encoding/json"
	"golang.org/x/image/webp"
	"image"
	"image/color"
	"sync"
)

//go:embed assets/equipment/*
var equipmentFiles embed.FS
var equipmentArt struct {
	sync.Once
	images map[string]image.Image
	err    error
}

func loadEquipmentArt() error {
	equipmentArt.Do(func() {
		equipmentArt.images = map[string]image.Image{}
		data, err := equipmentFiles.ReadFile("assets/equipment/manifest.json")
		if err != nil {
			equipmentArt.err = err
			return
		}
		var files map[string]string
		if err = json.Unmarshal(data, &files); err != nil {
			equipmentArt.err = err
			return
		}
		for name, path := range files {
			data, err = equipmentFiles.ReadFile("assets/equipment/" + path)
			if err != nil {
				equipmentArt.err = err
				return
			}
			img, e := webp.Decode(bytes.NewReader(data))
			if e != nil {
				equipmentArt.err = e
				return
			}
			equipmentArt.images[name] = img
		}
	})
	return equipmentArt.err
}
func qualityStyle(tier int) (string, color.RGBA) {
	names := []string{"凡品", "精制", "上品", "名器", "传世"}
	colors := []color.RGBA{{168, 162, 158, 255}, {111, 174, 138, 255}, {111, 167, 201, 255}, {179, 147, 208, 255}, {228, 183, 95, 255}}
	i := max(0, min(4, tier-1))
	return names[i], colors[i]
}
