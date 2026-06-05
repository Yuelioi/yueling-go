package bot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Segment is a single OneBot v11 message segment.
type Segment struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Message is an ordered list of segments.
type Message []Segment

// Text returns the concatenated text of all text segments.
func (m Message) Text() string {
	var sb strings.Builder
	for _, s := range m {
		if s.Type == "text" {
			var d struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				sb.WriteString(d.Text)
			}
		}
	}
	return sb.String()
}

// Summary renders the message for logging: text inline, other segments as
// bracketed placeholders so images/at/replies aren't silently dropped.
func (m Message) Summary() string {
	var sb strings.Builder
	for _, s := range m {
		switch s.Type {
		case "text":
			var d struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				sb.WriteString(d.Text)
			}
		case "image":
			sb.WriteString("[图片]")
		case "face", "mface":
			sb.WriteString("[表情]")
		case "at":
			var d struct {
				QQ string `json:"qq"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				if d.QQ == "all" {
					sb.WriteString("[@全体]")
				} else {
					sb.WriteString("[@" + d.QQ + "]")
				}
			}
		case "reply":
			sb.WriteString("[回复]")
		case "record":
			sb.WriteString("[语音]")
		case "video":
			sb.WriteString("[视频]")
		case "file":
			sb.WriteString("[文件]")
		case "json":
			sb.WriteString("[卡片]")
		case "forward":
			sb.WriteString("[合并转发]")
		case "markdown":
			sb.WriteString("[markdown]")
		default:
			sb.WriteString("[" + s.Type + "]")
		}
	}
	return strings.TrimSpace(sb.String())
}

// HasType reports whether the message contains a segment of the given type.
func (m Message) HasType(t string) bool {
	for _, s := range m {
		if s.Type == t {
			return true
		}
	}
	return false
}

// AtTargets returns the QQ numbers of all @-segments.
func (m Message) AtTargets() []string {
	var out []string
	for _, s := range m {
		if s.Type == "at" {
			var d struct {
				QQ string `json:"qq"`
			}
			if json.Unmarshal(s.Data, &d) == nil && d.QQ != "all" {
				out = append(out, d.QQ)
			}
		}
	}
	return out
}

// ReplyID returns the message-id being replied to, if any.
func (m Message) ReplyID() (string, bool) {
	for _, s := range m {
		if s.Type == "reply" {
			var d struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				return d.ID, true
			}
		}
	}
	return "", false
}

// ImageURLs returns the file/url of every image segment.
func (m Message) ImageURLs() []string {
	var out []string
	for _, s := range m {
		if s.Type == "image" {
			var d struct {
				File string `json:"file"`
				URL  string `json:"url"`
			}
			if json.Unmarshal(s.Data, &d) == nil {
				if d.URL != "" {
					out = append(out, d.URL)
				} else {
					out = append(out, d.File)
				}
			}
		}
	}
	return out
}

// ---- Builder ----

// Msg starts a fluent message builder.
func Msg() *MsgBuilder { return &MsgBuilder{} }

type MsgBuilder struct {
	segs []Segment
}

func seg(typ string, data any) Segment {
	raw, _ := json.Marshal(data)
	return Segment{Type: typ, Data: raw}
}

func (b *MsgBuilder) Text(text string) *MsgBuilder {
	b.segs = append(b.segs, seg("text", map[string]string{"text": text}))
	return b
}

func (b *MsgBuilder) At(qq int64) *MsgBuilder {
	b.segs = append(b.segs, seg("at", map[string]string{"qq": fmt.Sprintf("%d", qq)}))
	return b
}

func (b *MsgBuilder) AtAll() *MsgBuilder {
	b.segs = append(b.segs, seg("at", map[string]string{"qq": "all"}))
	return b
}

func (b *MsgBuilder) Image(file string) *MsgBuilder {
	b.segs = append(b.segs, seg("image", map[string]string{"file": file}))
	return b
}

// ImageBytes embeds raw image data as base64.
func (b *MsgBuilder) ImageBytes(data []byte) *MsgBuilder {
	return b.Image("base64://" + base64.StdEncoding.EncodeToString(data))
}

// LocalImage reads the file at path and embeds it as base64.
// This works regardless of where NapCat is running.
func (b *MsgBuilder) LocalImage(path string) *MsgBuilder {
	data, err := os.ReadFile(path)
	if err != nil {
		return b
	}
	return b.ImageBytes(data)
}

func (b *MsgBuilder) Reply(msgID int32) *MsgBuilder {
	b.segs = append(b.segs, seg("reply", map[string]string{"id": fmt.Sprintf("%d", msgID)}))
	return b
}

func (b *MsgBuilder) Face(id string) *MsgBuilder {
	b.segs = append(b.segs, seg("face", map[string]string{"id": id}))
	return b
}

func (b *MsgBuilder) Build() Message { return b.segs }
