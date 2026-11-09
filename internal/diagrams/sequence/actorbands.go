// Ported from mermaid 12.2.0, diagrams/sequence/actorBands.ts: the vertical
// band model for sequence participants under the `neo` look (MIT).

package sequence

import "github.com/bvolpato/mermaid-go-renderer/internal/utils"

const (
	// glyphBandHeight is the height of the glyph band. The round icons set
	// it: a 22 unit circle plus its underline.
	glyphBandHeight = 44.0
	// glyphLabelGap is the air between the bottom of the glyph and the top
	// of the label block.
	glyphLabelGap = 6.0
	// labelLifelineGap is the air between the bottom of the label block and
	// the lifeline datum.
	labelLifelineGap = 6.0
)

// actorLabelHeight is the measured height of the actor's label block.
// calculateActorMargins stashes the measurement it already takes. The
// fallback measures the description.
func (r *renderer) actorLabelHeight(a *actor) (float64, error) {
	if a.hasActorTextHeight {
		return a.actorTextHeight, nil
	}
	dims, err := utils.CalculateTextDimensions(r.ctx, a.description, r.conf.actorFont())
	return dims.Height, err
}

// actorStackHeight is the stack an icon-family actor needs above the datum:
// glyph, gap, label, gap.
func actorStackHeight(textHeight float64) float64 {
	return glyphBandHeight + glyphLabelGap + textHeight + labelLifelineGap
}

// bands is HeaderBands or FooterBands: the fields that svgDraw.js reads.
type bands struct {
	// lifelineStartY is the datum: where this row's lifelines start. Only a
	// header has it; it is NaN (undefined) for a footer.
	lifelineStartY float64
	// glyphBottomY is the bottom edge of the glyph band.
	glyphBottomY float64
	// labelCenterY is the vertical centre of the label block.
	labelCenterY float64
}

// headerBands is the header geometry for one actor. rowHeight is the row's
// shared height.
func headerBands(actorY, rowHeight, textHeight float64) *bands {
	lifelineStartY := actorY + rowHeight
	labelCenterY := lifelineStartY - labelLifelineGap - float64(textHeight/2)
	return &bands{
		lifelineStartY: lifelineStartY,
		glyphBottomY:   lifelineStartY - labelLifelineGap - textHeight - glyphLabelGap,
		labelCenterY:   labelCenterY,
	}
}

// footerBands is the footer geometry: the same stack, top-anchored at the
// datum, growing downward.
func footerBands(actorY, textHeight float64) *bands {
	glyphTopY := actorY + labelLifelineGap
	glyphBottomY := glyphTopY + glyphBandHeight
	return &bands{
		lifelineStartY: nan,
		glyphBottomY:   glyphBottomY,
		labelCenterY:   glyphBottomY + glyphLabelGap + float64(textHeight/2),
	}
}
