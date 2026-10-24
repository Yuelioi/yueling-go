package cards

import (
	"bytes"
	"context"
	"image"
	"testing"
)

func TestFemaleArtworkAndResultState(t *testing.T) {
	if err := loadAssets(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"sword", "blade", "boxer", "shadow", "venom", "chord"} {
		id += "-female"
		for _, prefix := range []string{"ink-sprite-", "ink-defeat-"} {
			if err := checkAsset(prefix + id); err != nil {
				t.Fatal(err)
			}
			im := asset(prefix + id)
			b := im.Bounds()
			_, _, _, alpha := im.At(b.Min.X, b.Min.Y).RGBA()
			if alpha != 0 {
				t.Fatalf("%s has opaque corner", prefix+id)
			}
		}
		// Normal result portraits must be byte-identical to the combat portrait;
		// the same character's defeat result must use distinct artwork.
		paint := func(mode int) []byte {
			data, err := render(context.Background(), func(p *painter) {
				r := image.Rect(40, 40, 300, 280)
				if mode == 0 {
					p.fighterAvatar(id, r, uiDanger)
				} else {
					p.resultAvatar(id, r, uiDanger, mode == 2)
				}
			}, 320)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		normal := paint(0)
		if !bytes.Equal(normal, paint(1)) {
			t.Fatal("winner portrait changed")
		}
		if bytes.Equal(normal, paint(2)) {
			t.Fatal("defeated portrait unchanged")
		}
	}
}
