package image

import "testing"

func TestDetectImageExt(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0, 0, 0, 0, 0}, "png"},
		{"gif", []byte("GIF89a______"), "gif"},
		{"webp", []byte("RIFF____WEBP"), "webp"},
		{"jpg default", []byte{0xFF, 0xD8, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "jpg"},
		{"too short", []byte{1, 2, 3}, "jpg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detectImageExt(c.data); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
