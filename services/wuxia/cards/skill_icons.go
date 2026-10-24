package cards

import (
	"image"
	"image/color"

	"github.com/Yuelioi/yueling-go/services/wuxia"
)

func (p *painter) skillIcon(r image.Rectangle, s *wuxia.Skill) {
	if s != nil {
		if art := martialImage(s.ID); art != nil {
			p.paintMartial(r, art)
			return
		}
	}
	p.battleSkillIcon(r, s)
}

// Small and large views retain the same painted silhouette and material colors.
func (p *painter) battleSkillIcon(r image.Rectangle, s *wuxia.Skill) {
	var tint color.Color = uiMuted
	if s != nil {
		_, tint = qualityStyle(s.Tier)
	}
	p.chamfer(r, color.RGBA{19, 35, 48, 255})
	p.outline(r.Inset(1), tint)
	if s != nil {
		if art := martialImage(s.ID); art != nil {
			p.paintMartial(r.Inset(max(2, r.Dx()/24)), art)
			return
		}
	}
	p.line(r.Min.X+r.Dx()/3, r.Min.Y+r.Dy()/2, r.Max.X-r.Dx()/3, r.Min.Y+r.Dy()/2, tint)
}

func (p *painter) actionArt(r image.Rectangle, f wuxia.Fighter, skillID string, compact bool) {
	if skillID != "" && f.Martial != nil && f.Martial.ID == skillID {
		if compact {
			p.battleSkillIcon(r, f.Martial)
		} else {
			p.skillIcon(r, f.Martial)
		}
		return
	}
	p.combatIcon(r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2, r.Dx()*2/3, "命中", uiMuted)
}
