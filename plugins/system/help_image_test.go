package system

import (
	"bytes"
	"image"
	"reflect"
	"testing"

	"golang.org/x/image/font/basicfont"
)

func TestEncodeHelpImageUsesPNG(t *testing.T) {
	data, err := hEncode(image.NewRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	wantSignature := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if !bytes.HasPrefix(data, wantSignature) {
		t.Fatalf("image signature = % x, want PNG", data[:min(len(data), len(wantSignature))])
	}
}

func TestParseHelpUsageRowsSeparatesCommandsAndDescriptions(t *testing.T) {
	rows := parseHelpUsageRows(
		"  单身 + 图片 / @某人          保留上半部分，替换下半部分图片\n" +
			"  解禁 @用户\n\n" +
			"  图片优先级：附图 > 引用消息图 > @用户头像 > 发送者头像",
	)
	want := []helpUsageRow{
		{Command: "单身 + 图片 / @某人", Description: "保留上半部分，替换下半部分图片"},
		{Command: "解禁 @用户"},
		{Spacer: true},
		{Note: "图片优先级：附图 > 引用消息图 > @用户头像 > 发送者头像"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("parseHelpUsageRows() = %#v, want %#v", rows, want)
	}
}

func TestLayoutHelpUsageRowsWrapsColumnsIndependently(t *testing.T) {
	previousBody, previousSmall := hfBody, hfSmall
	hfBody, hfSmall = basicfont.Face7x13, basicfont.Face7x13
	t.Cleanup(func() { hfBody, hfSmall = previousBody, previousSmall })

	rows := layoutHelpUsageRows([]helpUsageRow{{
		Command:     "short",
		Description: "a description that must wrap",
	}}, 70, 84, 160)
	if len(rows) != 1 {
		t.Fatalf("layout rows = %d, want 1", len(rows))
	}
	if len(rows[0].CommandLines) != 1 || len(rows[0].DescriptionLines) < 2 {
		t.Fatalf("unexpected wrapped columns: command=%q description=%q", rows[0].CommandLines, rows[0].DescriptionLines)
	}
	wantHeight := len(rows[0].DescriptionLines)*(hLH(szBody)+2) + 14
	if rows[0].Height != wantHeight {
		t.Fatalf("row height = %d, want %d", rows[0].Height, wantHeight)
	}
}

func TestParseHelpUsageRowsRecognizesSectionNotes(t *testing.T) {
	rows := parseHelpUsageRows("  支持平台：\n    B站  视频 / 番剧\n\n  所有命令均可附图，或回复图片后发送")
	want := []helpUsageRow{
		{Note: "支持平台："},
		{Command: "B站", Description: "视频 / 番剧"},
		{Spacer: true},
		{Note: "所有命令均可附图，或回复图片后发送"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("parseHelpUsageRows() = %#v, want %#v", rows, want)
	}
}
