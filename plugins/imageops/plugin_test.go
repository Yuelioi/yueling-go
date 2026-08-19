package imageops

import (
	"testing"

	"github.com/Yuelioi/yueling-go/services/imaging"
)

func TestParseRotate(t *testing.T) {
	tests := []struct {
		args    []string
		degrees int
		ok      bool
	}{
		{nil, 90, true},
		{[]string{"180"}, 180, true},
		{[]string{"-90"}, -90, true},
		{[]string{"45"}, 0, false},
		{[]string{"x"}, 0, false},
	}
	for _, tt := range tests {
		got, err := parseRotate(tt.args)
		if (err == nil) != tt.ok {
			t.Fatalf("parseRotate(%q) err=%v", tt.args, err)
		}
		if tt.ok && (got.Kind != imaging.Rotate || got.Degrees != tt.degrees) {
			t.Fatalf("parseRotate(%q)=%+v", tt.args, got)
		}
	}
}

func TestParseResize(t *testing.T) {
	tests := []struct {
		arg  string
		want imaging.Operation
		ok   bool
	}{
		{"50%", imaging.Operation{Kind: imaging.Resize, Scale: 0.5}, true},
		{"512", imaging.Operation{Kind: imaging.Resize, Width: 512}, true},
		{"320x240", imaging.Operation{Kind: imaging.Resize, Width: 320, Height: 240}, true},
		{"320×240", imaging.Operation{Kind: imaging.Resize, Width: 320, Height: 240}, true},
		{"0", imaging.Operation{}, false},
		{"500%", imaging.Operation{}, false},
		{"NaN%", imaging.Operation{}, false},
	}
	for _, tt := range tests {
		got, err := parseResize([]string{tt.arg})
		if (err == nil) != tt.ok {
			t.Fatalf("parseResize(%q) err=%v", tt.arg, err)
		}
		if tt.ok && got != tt.want {
			t.Fatalf("parseResize(%q)=%+v want %+v", tt.arg, got, tt.want)
		}
	}
}
