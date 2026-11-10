// This file implements original PDF shading serialization under ISO 32000-1.
// It does not port an upstream implementation.

package pdf

import (
	"fmt"
	"strings"
)

// GradientStop defines an unpremultiplied color at an offset in [0, 1].
type GradientStop struct {
	Offset float64
	Color  Color
}

type shadingResource struct {
	coords [6]float64
	radial bool
	stops  []GradientStop
	mask   *gradientMask
}

type gradientMask struct {
	state         drawingState
	width, height float64
}

// Gradient paints an axial or radial gradient through the current transform and clip.
// Axial coordinates are x0, y0, x1, y1. Radial coordinates are x0, y0, r0, x1, y1, r1.
// Stops must have strictly increasing offsets. Endpoint colors extend outward.
// Varying stop alpha uses a luminosity soft mask. Opacity scales the result once.
// Unsupported or invalid inputs return false without changing the document.
func (d *Document) Gradient(coords [6]float64, radial bool, stops []GradientStop, opacity float64) bool {
	if d == nil || d.err != nil || d.current == nil || len(stops) == 0 || !finite(opacity) || opacity < 0 || opacity > 1 {
		return false
	}
	coordCount := 4
	if radial {
		coordCount = 6
	}
	for _, value := range coords[:coordCount] {
		if !finite(value) {
			return false
		}
	}
	if radial {
		if coords[2] < 0 || coords[5] < 0 || (coords[2] == 0 && coords[5] == 0) ||
			(coords[0] == coords[3] && coords[1] == coords[4] && coords[2] == coords[5]) {
			return false
		}
	} else if coords[0] == coords[2] && coords[1] == coords[3] {
		return false
	}
	alpha := stops[0].Color.A
	varyingAlpha := false
	for i, stop := range stops {
		if !finite(stop.Offset) || stop.Offset < 0 || stop.Offset > 1 ||
			(i > 0 && stop.Offset <= stops[i-1].Offset) {
			return false
		}
		varyingAlpha = varyingAlpha || stop.Color.A != alpha
	}

	// Copy the stops before adding constant intervals at missing endpoints.
	padded := make([]GradientStop, 0, len(stops)+2)
	if stops[0].Offset > 0 {
		padded = append(padded, GradientStop{Offset: 0, Color: stops[0].Color})
	}
	padded = append(padded, stops...)
	if stops[len(stops)-1].Offset < 1 {
		padded = append(padded, GradientStop{Offset: 1, Color: stops[len(stops)-1].Color})
	}
	id := len(d.shadings)
	c := d.current
	shading := &shadingResource{coords: coords, radial: radial, stops: padded}
	if varyingAlpha {
		page := d.pages[len(d.pages)-1]
		state := c.state
		state.clips = append([]clipPath(nil), state.clips...)
		shading.mask = &gradientMask{state: state, width: page.width, height: page.height}
	} else {
		opacity *= float64(alpha) / 255
	}
	d.shadings = append(d.shadings, shading)
	c.WriteString("q\n")
	c.writeClips()
	paint := alphaState{opacity: opacity}
	if !varyingAlpha {
		paint.mask = c.state.mask
	}
	state := d.alpha(paint)
	c.states[state] = true
	fmt.Fprintf(c, "/GS%d gs\n", state+1)
	if varyingAlpha {
		// Establish the mask in root CSS coordinates before the local transform.
		c.masks[id] = true
		fmt.Fprintf(c, "/SM%d gs\n", id+1)
	}
	c.WriteString(matrixString(c.state.matrix))
	c.shadings[id] = true
	fmt.Fprintf(c, "/Sh%d sh\nQ\n", id+1)
	return true
}

func (s *shadingResource) writePDF(w *objectWriter) int {
	return s.writeShadingPDF(w, false)
}

func (s *shadingResource) writeMaskPDF(w *objectWriter, formRefs []int) int {
	if s.mask == nil {
		return 0
	}
	alpha := s.writeShadingPDF(w, true)
	content := drawingContext{state: s.mask.state}
	content.writeClips()
	clipResources := ""
	if mask := s.mask.state.mask; mask > 0 {
		clip := w.addObject(fmt.Sprintf("<< /Type /ExtGState /SMask << /S /Alpha /G %d 0 R >> >>", formRefs[mask-1]))
		clipResources = fmt.Sprintf(" /ExtGState << /Clip %d 0 R >>", clip)
		content.WriteString("/Clip gs\n")
	}
	content.WriteString(matrixString(s.mask.state.matrix))
	content.WriteString("/Alpha sh\n")
	form := w.addStream(fmt.Sprintf("/Type /XObject /Subtype /Form /FormType 1 /BBox [0 0 %s %s] /Group << /S /Transparency /CS /DeviceGray /I true >> /Resources << /Shading << /Alpha %d 0 R >>%s >>",
		number(s.mask.width), number(s.mask.height), alpha, clipResources), content.Bytes())
	return w.addObject(fmt.Sprintf("<< /Type /ExtGState /SMask << /S /Luminosity /G %d 0 R /BC [0] >> >>", form))
}

func (s *shadingResource) writeShadingPDF(w *objectWriter, gray bool) int {
	var functions, bounds, encode strings.Builder
	functionRef := 0
	for i := 0; i < len(s.stops)-1; i++ {
		c0, c1 := s.stops[i].Color, s.stops[i+1].Color
		functionRef = w.addObject(fmt.Sprintf("<< /FunctionType 2 /Domain [0 1] /C0 [%s] /C1 [%s] /N 1 >>",
			gradientComponents(c0, gray), gradientComponents(c1, gray)))
		if i > 0 {
			functions.WriteByte(' ')
			bounds.WriteString(number(s.stops[i].Offset))
			bounds.WriteByte(' ')
		}
		fmt.Fprintf(&functions, "%d 0 R", functionRef)
		encode.WriteString("0 1 ")
	}
	if len(s.stops) > 2 {
		functionRef = w.addObject(fmt.Sprintf("<< /FunctionType 3 /Domain [0 1] /Functions [%s] /Bounds [%s] /Encode [%s] >>",
			functions.String(), bounds.String(), encode.String()))
	}
	typ, coordCount := 2, 4
	if s.radial {
		typ, coordCount = 3, 6
	}
	var coords strings.Builder
	for i, value := range s.coords[:coordCount] {
		if i > 0 {
			coords.WriteByte(' ')
		}
		coords.WriteString(number(value))
	}
	colorSpace := "DeviceRGB"
	if gray {
		colorSpace = "DeviceGray"
	}
	return w.addObject(fmt.Sprintf("<< /ShadingType %d /ColorSpace /%s /Coords [%s] /Domain [0 1] /Function %d 0 R /Extend [true true] >>",
		typ, colorSpace, coords.String(), functionRef))
}

func gradientComponents(color Color, gray bool) string {
	if gray {
		return number(float64(color.A) / 255)
	}
	return fmt.Sprintf("%s %s %s", number(float64(color.R)/255), number(float64(color.G)/255), number(float64(color.B)/255))
}
