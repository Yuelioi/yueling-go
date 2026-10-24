package cards

import (
	"context"
	"fmt"
	"image"
	"sort"
	"strings"
	"unicode/utf8"
)

// ListEntry carries display facts, not a preformatted chat transcript.
type ListEntry struct {
	Name, Detail, Badge, ArtName, SkillID, Portrait string
	Tier                                            int
}

// Collection paginates before rendering, so large inventories never lose rows.
func Collection(ctx context.Context, title, hint string, entries []ListEntry) ([][]byte, error) {
	if err := loadEquipmentArt(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		entries = []ListEntry{{Name: "暂时还没有", Detail: "先去历练或寻宝，再回来看看。"}}
	}
	// Keep long mechanics intact across rows instead of silently clipping them.
	var expanded []ListEntry
	for _, e := range entries {
		runes := []rune(e.Detail)
		for len(runes) > 105 {
			part := e
			part.Detail = string(runes[:105])
			expanded = append(expanded, part)
			runes = runes[105:]
			e.Name += " · 续"
		}
		e.Detail = string(runes)
		expanded = append(expanded, e)
	}
	entries = expanded
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Tier > entries[j].Tier })
	var pages [][]byte
	total := (len(entries) + 5) / 6
	for start := 0; start < len(entries); start += 6 {
		rows := entries[start:min(start+6, len(entries))]
		height := 300 + len(rows)*155
		data, err := render(ctx, func(p *painter) {
			p.background(height)
			p.atmosphere(height)
			p.heading(title, -1)
			p.text(60, 145, 18, fmt.Sprintf("%s  ·  %d/%d 页", hint, start/6+1, total), uiMuted, 1080)
			for i, e := range rows {
				y := 175 + i*155
				x := 70
				if art := equipmentArt.images[e.ArtName]; art != nil {
					p.containArt(art, image.Rect(66, y, 186, y+120))
					x = 210
				} else if art := martialImage(e.SkillID); art != nil {
					p.containArt(art, image.Rect(66, y, 186, y+120))
					x = 210
				}
				if e.Portrait != "" {
					p.sprite("ink-sprite-"+e.Portrait, image.Rect(66, y, 186, y+120))
					x = 210
				}
				tint := uiText
				if e.Tier > 0 {
					_, tint = qualityStyle(e.Tier)
				}
				p.title(x, y+30, 25, e.Name, tint, 780)
				p.text(960, y+28, 17, e.Badge, uiAccent, 180)
				detailY := y + 62
				if e.Name == "" {
					detailY = y + 35
				}
				p.wrapText(x, detailY, 20, e.Detail, uiMuted, 1120-x, 3)
				p.line(60, y+139, 1140, y+139, uiEdge)
			}
			p.commandRibbon(height)
		}, height)
		if err != nil {
			return nil, err
		}
		pages = append(pages, data)
	}
	return pages, nil
}

// Notice wraps long instructions into bounded rows; it is also used for rule errors.
func Notice(ctx context.Context, title, body string) ([][]byte, error) {
	var rows []ListEntry
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// A row fits three lines at the card's font size; split instead of ellipsizing.
		for utf8.RuneCountInString(line) > 105 {
			r := []rune(line)
			rows = append(rows, ListEntry{Detail: string(r[:105])})
			line = string(r[105:])
		}
		entry := ListEntry{Detail: line}
		if name, detail, ok := strings.Cut(line, "："); ok && utf8.RuneCountInString(name) <= 28 {
			entry.Name = name
			entry.Detail = detail
		}
		rows = append(rows, entry)
	}
	return Collection(ctx, title, "在群聊发送下方指令", rows)
}
