// Ported from mermaid 12.2.0, diagrams/cynefin/cynefinBoundaries.ts (MIT).

package cynefin

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// seededRandom is seededRandom(seed): a deterministic pseudo-random number
// generator (mulberry32). It returns a value in [0, 1).
func seededRandom(seed float64) float64 {
	t := uint32(js.ToInt32(seed + 0x6d2b79f5))
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

// hashString is hashString(str): a simple string hash for seeding the PRNG.
func hashString(str string) float64 {
	var hash int32
	for _, ch := range js.UTF16Units(str) {
		hash = (hash << 5) - hash + int32(ch)
	}
	return float64(hash)
}

// resolveSeed is resolveSeed(configuredSeed, id): a configured seed that is
// a finite number and not zero wins. Otherwise the seed is the hash of the
// id of the SVG element.
func resolveSeed(configuredSeed any, id string) float64 {
	if seed, ok := configuredSeed.(float64); ok && !math.IsNaN(seed) && !math.IsInf(seed, 0) && seed != 0 {
		return seed
	}
	return hashString(id)
}

type point struct{ x, y float64 }

// amplitudeOr is amplitudeOverride ?? fallback, as a number.
func amplitudeOr(amplitudeOverride any, fallback float64) float64 {
	if js.IsNullish(amplitudeOverride) {
		return fallback
	}
	return js.ToNumber(amplitudeOverride)
}

// generateFoldPath is generateFoldPath(width, height, seed,
// amplitudeOverride): a vertical wavy line (the "fold") through the center
// of the diagram. It uses cubic bezier segments with alternating control
// point offsets.
func generateFoldPath(width, height, seed float64, amplitudeOverride any) string {
	cx := float64(width / 2)
	amplitude := amplitudeOr(amplitudeOverride, width*0.015)
	const segments = 7
	segHeight := height / segments
	points := []point{}

	for i := 0; i <= segments; i++ {
		jitter := float64(float64(seededRandom(seed+float64(i*17))*amplitude)*2) - amplitude
		points = append(points, point{x: cx + jitter, y: float64(i) * segHeight})
	}

	var d strings.Builder
	d.WriteString("M" + js.Str(points[0].x) + "," + js.Str(points[0].y))
	for i := 0; i < len(points)-1; i++ {
		p0 := points[i]
		p1 := points[i+1]
		midY := (p0.y + p1.y) / 2
		dir := -1.0
		if i%2 == 0 {
			dir = 1
		}
		offset := float64(amplitude * 1.5 * dir * seededRandom(seed+float64(i*31)+7))
		cp1x := p0.x + offset
		cp1y := midY
		cp2x := p1.x - offset
		cp2y := midY
		d.WriteString(" C" + js.Str(cp1x) + "," + js.Str(cp1y) + " " + js.Str(cp2x) + "," + js.Str(cp2y) + " " + js.Str(p1.x) + "," + js.Str(p1.y))
	}

	return d.String()
}

// generateHorizontalBoundary is generateHorizontalBoundary(width, height,
// seed, amplitudeOverride): a horizontal wavy line through the center of
// the diagram.
func generateHorizontalBoundary(width, height, seed float64, amplitudeOverride any) string {
	cy := float64(height / 2)
	amplitude := amplitudeOr(amplitudeOverride, height*0.015)
	const segments = 7
	segWidth := width / segments
	points := []point{}

	for i := 0; i <= segments; i++ {
		jitter := float64(float64(seededRandom(seed+float64(i*23))*amplitude)*2) - amplitude
		points = append(points, point{x: float64(i) * segWidth, y: cy + jitter})
	}

	var d strings.Builder
	d.WriteString("M" + js.Str(points[0].x) + "," + js.Str(points[0].y))
	for i := 0; i < len(points)-1; i++ {
		p0 := points[i]
		p1 := points[i+1]
		midX := (p0.x + p1.x) / 2
		dir := -1.0
		if i%2 == 0 {
			dir = 1
		}
		offset := float64(amplitude * 1.5 * dir * seededRandom(seed+float64(i*37)+11))
		cp1x := midX
		cp1y := p0.y + offset
		cp2x := midX
		cp2y := p1.y - offset
		d.WriteString(" C" + js.Str(cp1x) + "," + js.Str(cp1y) + " " + js.Str(cp2x) + "," + js.Str(cp2y) + " " + js.Str(p1.x) + "," + js.Str(p1.y))
	}

	return d.String()
}

// generateCliffPath is generateCliffPath(width, height): the "cliff" path
// between Clear (bottom-right) and Chaotic (bottom-left). bottomY is the
// text of the height as the template literal of upstream prints it.
func generateCliffPath(width, height float64, bottomYText string) string {
	cx := float64(width / 2)
	topY := float64(height * 0.5)
	bottomY := height
	amplitude := float64(width * 0.03)

	// A steep S-curve from the center downward
	return strings.Join([]string{
		"M" + js.Str(cx) + "," + js.Str(topY),
		"C" + js.Str(cx+amplitude) + "," + js.Str(topY+float64((bottomY-topY)*0.2)),
		js.Str(cx-float64(amplitude*1.5)) + "," + js.Str(topY+float64((bottomY-topY)*0.55)),
		js.Str(cx+float64(amplitude*0.5)) + "," + js.Str(topY+float64((bottomY-topY)*0.75)),
		"C" + js.Str(cx-amplitude) + "," + js.Str(topY+float64((bottomY-topY)*0.85)),
		js.Str(cx+float64(amplitude*0.3)) + "," + js.Str(topY+float64((bottomY-topY)*0.95)),
		js.Str(cx) + "," + bottomYText,
	}, " ")
}

// generateConfusionPath is generateConfusionPath(cx, cy, rx, ry): an
// ellipse path for the confusion/disorder region at the center.
func generateConfusionPath(cx, cy, rx, ry float64) string {
	// Draw an ellipse using two arc commands
	return strings.Join([]string{
		"M" + js.Str(cx-rx) + "," + js.Str(cy),
		"A" + js.Str(rx) + "," + js.Str(ry) + " 0 1,1 " + js.Str(cx+rx) + "," + js.Str(cy),
		"A" + js.Str(rx) + "," + js.Str(ry) + " 0 1,1 " + js.Str(cx-rx) + "," + js.Str(cy),
		"Z",
	}, " ")
}
