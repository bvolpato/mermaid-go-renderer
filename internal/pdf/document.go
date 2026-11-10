// This file implements original PDF serialization under ISO 32000-1.
// It does not port an upstream implementation.

// Package pdf writes vector PDF documents in CSS drawing units.
package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// Matrix maps (x, y) to (a*x+c*y+e, b*x+d*y+f).
type Matrix [6]float64

// Color is an unpremultiplied sRGB color.
type Color struct{ R, G, B, A uint8 }

// Style controls one fill or stroke operation.
type Style struct {
	Color        Color
	Stroke       bool
	Width, Miter float64
	Cap, Join    int
	Dash         []float64
	DashOffset   float64
}

// Document contains PDF pages. Its methods are not safe for concurrent use.
type Document struct {
	pages    []*page
	current  *drawingContext
	groups   []openGroup
	forms    []*form
	states   []alphaState
	fonts    []pdfFont
	shadings []pdfShading
	fontIDs  map[string]int
	badFonts map[string]bool
	type3IDs map[*font.Face][]int
	badType3 map[*font.Face]bool
	err      error
}

type pdfFont interface {
	encodeGlyph(gid uint32, text string) (uint16, bool)
	writePDF(w *objectWriter) int
}

type pdfShading interface {
	writePDF(w *objectWriter) int
}

type page struct {
	width, height float64
	content       *drawingContext
}

type clipPath struct {
	path    string
	evenOdd bool
}

type drawingState struct {
	matrix Matrix
	clips  []clipPath
	mask   int
}

type resources struct {
	fonts, states, forms, shadings, masks map[int]bool
}

type drawingContext struct {
	bytes.Buffer
	resources
	state drawingState
	saves []drawingState
}

type form struct {
	width, height float64
	content       *drawingContext
}

type alphaState struct {
	opacity  float64
	multiply bool
	mask     int
}

type openGroup struct {
	parent *drawingContext
	formID int
	alphaState
}

func newDrawingContext(state drawingState) *drawingContext {
	return &drawingContext{
		state: state,
		resources: resources{
			fonts: make(map[int]bool), states: make(map[int]bool), forms: make(map[int]bool), shadings: make(map[int]bool), masks: make(map[int]bool),
		},
	}
}

// New starts a page in CSS units. Bytes reports invalid dimensions.
func New(width, height float64) *Document {
	d := &Document{fontIDs: make(map[string]int), badFonts: make(map[string]bool)}
	d.NewPage(width, height)
	return d
}

// NewPage finishes the current page and starts a page with fresh drawing state.
func (d *Document) NewPage(width, height float64) {
	if d.err != nil {
		return
	}
	if len(d.groups) != 0 {
		d.err = fmt.Errorf("pdf: cannot start a page with an open group")
		return
	}
	if !finite(width) || !finite(height) || width <= 0 || height <= 0 || width*.75 == 0 || height*.75 == 0 {
		d.err = fmt.Errorf("pdf: page dimensions must be finite and positive")
		return
	}
	d.current = newDrawingContext(drawingState{matrix: Matrix{1, 0, 0, 1, 0, 0}})
	d.pages = append(d.pages, &page{width, height, d.current})
}

func (d *Document) ready() bool {
	if d.err != nil {
		return false
	}
	if d.current == nil {
		d.err = fmt.Errorf("pdf: document has no page")
		return false
	}
	return true
}

// Save saves the current transform and clipping path.
func (d *Document) Save() {
	if d.ready() {
		d.current.saves = append(d.current.saves, d.current.state)
	}
}

// Restore restores the most recently saved state.
func (d *Document) Restore() {
	if !d.ready() {
		return
	}
	c := d.current
	if len(c.saves) == 0 {
		d.err = fmt.Errorf("pdf: Restore without Save")
		return
	}
	c.state = c.saves[len(c.saves)-1]
	c.saves = c.saves[:len(c.saves)-1]
}

// Transform concatenates a matrix with the current matrix, like PDF cm.
func (d *Document) Transform(m Matrix) {
	if !d.ready() {
		return
	}
	a := d.current.state.matrix
	for _, v := range m {
		if !finite(v) {
			d.err = fmt.Errorf("pdf: matrix values must be finite")
			return
		}
	}
	r := Matrix{
		float64(a[0]*m[0]) + float64(a[2]*m[1]),
		float64(a[1]*m[0]) + float64(a[3]*m[1]),
		float64(a[0]*m[2]) + float64(a[2]*m[3]),
		float64(a[1]*m[2]) + float64(a[3]*m[3]),
		float64(a[0]*m[4]) + float64(a[2]*m[5]) + a[4],
		float64(a[1]*m[4]) + float64(a[3]*m[5]) + a[5],
	}
	for _, v := range r {
		if !finite(v) {
			d.err = fmt.Errorf("pdf: matrix multiplication overflow")
			return
		}
	}
	d.current.state.matrix = r
}

// Clip intersects the clip with a PDF path in current local coordinates.
func (d *Document) Clip(path string, evenOdd bool) {
	if !d.ready() {
		return
	}
	path, err := canonicalPath(path, &d.current.state.matrix)
	if err != nil {
		d.err = err
		return
	}
	clips := append([]clipPath(nil), d.current.state.clips...)
	d.current.state.clips = append(clips, clipPath{path, evenOdd})
}

// ClipShape defines one child of a clip union, with its own fill rule.
type ClipShape struct {
	Path    string
	EvenOdd bool
}

// ClipUnion intersects the current clip with the union of its child shapes.
func (d *Document) ClipUnion(shapes []ClipShape) {
	if !d.ready() {
		return
	}
	parent := d.current
	paths := make([]clipPath, 0, len(shapes))
	for _, shape := range shapes {
		path, err := canonicalPath(shape.Path, &parent.state.matrix)
		if err != nil {
			d.err = err
			return
		}
		paths = append(paths, clipPath{path, shape.EvenOdd})
	}
	p := d.pages[len(d.pages)-1]
	union := newDrawingContext(drawingState{matrix: Matrix{1, 0, 0, 1, 0, 0}, clips: parent.state.clips})
	unionID := len(d.forms)
	d.forms = append(d.forms, &form{p.width, p.height, union})
	d.current = union
	for _, path := range paths {
		d.Draw(path.path, path.evenOdd, Style{Color: Color{255, 255, 255, 255}})
	}
	d.current = parent
	if parent.state.mask != 0 {
		// Apply the prior mask once to the completed union, including overlaps.
		wrapper := newDrawingContext(drawingState{matrix: Matrix{1, 0, 0, 1, 0, 0}})
		state := d.alpha(alphaState{opacity: 1, mask: parent.state.mask})
		wrapper.states[state], wrapper.forms[unionID] = true, true
		fmt.Fprintf(wrapper, "q\n/GS%d gs\n/X%d Do\nQ\n", state+1, unionID+1)
		unionID = len(d.forms)
		d.forms = append(d.forms, &form{p.width, p.height, wrapper})
	}
	parent.state.mask = unionID + 1
}

// Draw fills or strokes a PDF path in current local coordinates.
func (d *Document) Draw(path string, evenOdd bool, style Style) {
	if !d.ready() {
		return
	}
	path, err := canonicalPath(path, nil)
	if err != nil {
		d.err = err
		return
	}
	if path == "" || !d.paintStart(style) {
		return
	}
	d.current.WriteString(path)
	if style.Stroke {
		d.current.WriteString("S\n")
	} else if evenOdd {
		d.current.WriteString("f*\n")
	} else {
		d.current.WriteString("f\n")
	}
	d.current.WriteString("Q\n")
}

// paintStart sets all paint properties inside an independent graphics state.
func (d *Document) paintStart(style Style) bool {
	if style.Stroke {
		if !finite(style.Width) || style.Width < 0 || !finite(style.Miter) || !finite(style.DashOffset) ||
			style.Cap < 0 || style.Cap > 2 || style.Join < 0 || style.Join > 2 {
			d.err = fmt.Errorf("pdf: invalid stroke style")
			return false
		}
		positive := false
		for _, value := range style.Dash {
			if !finite(value) || value < 0 {
				d.err = fmt.Errorf("pdf: invalid dash pattern")
				return false
			}
			positive = positive || value > 0
		}
		if len(style.Dash) > 0 && !positive {
			d.err = fmt.Errorf("pdf: dash pattern must have a positive length")
			return false
		}
	}
	c := d.current
	c.WriteString("q\n")
	c.writeClips()
	state := d.alpha(alphaState{opacity: float64(style.Color.A) / 255, mask: c.state.mask})
	c.states[state] = true
	// Soft masks use page coordinates, so establish them before the local matrix.
	fmt.Fprintf(c, "/GS%d gs\n", state+1)
	c.WriteString(matrixString(c.state.matrix))
	fmt.Fprintf(c, "%s %s %s ", number(float64(style.Color.R)/255), number(float64(style.Color.G)/255), number(float64(style.Color.B)/255))
	if style.Stroke {
		c.WriteString("RG\n")
		miter := style.Miter
		if miter < 1 {
			miter = 10
		}
		fmt.Fprintf(c, "%s w\n%s M\n%d J\n%d j\n[", number(style.Width), number(miter), style.Cap, style.Join)
		for _, dash := range style.Dash {
			c.WriteString(number(dash))
			c.WriteByte(' ')
		}
		fmt.Fprintf(c, "] %s d\n", number(style.DashOffset))
	} else {
		c.WriteString("rg\n")
	}
	return true
}

func (c *drawingContext) writeClips() {
	for _, clip := range c.state.clips {
		c.WriteString(clip.path)
		if clip.evenOdd {
			c.WriteString("W* n\n")
		} else {
			c.WriteString("W n\n")
		}
	}
}

func (d *Document) alpha(state alphaState) int {
	for i, existing := range d.states {
		if existing == state {
			return i
		}
	}
	d.states = append(d.states, state)
	return len(d.states) - 1
}

// BeginGroup starts an isolated transparency group and saves the current state.
func (d *Document) BeginGroup(opacity float64, multiply bool) {
	if !d.ready() {
		return
	}
	if !finite(opacity) {
		d.err = fmt.Errorf("pdf: group opacity must be finite")
		return
	}
	p := d.pages[len(d.pages)-1]
	formID := len(d.forms)
	state := d.current.state
	// Apply the entry mask when the completed group is composited.
	state.mask = 0
	child := newDrawingContext(state)
	d.forms = append(d.forms, &form{p.width, p.height, child})
	d.groups = append(d.groups, openGroup{d.current, formID, alphaState{opacity: min(1, max(0, opacity)), multiply: multiply}})
	d.current = child
}

// EndGroup composites the group and restores its entry state.
func (d *Document) EndGroup() {
	if !d.ready() {
		return
	}
	if len(d.groups) == 0 {
		d.err = fmt.Errorf("pdf: EndGroup without BeginGroup")
		return
	}
	g := d.groups[len(d.groups)-1]
	d.closeGroup(g.opacity, g.multiply)
}

// EndGroupWithOpacity overrides the opacity and blend mode at group close.
func (d *Document) EndGroupWithOpacity(opacity float64, multiply bool) {
	if !d.ready() {
		return
	}
	if len(d.groups) == 0 {
		d.err = fmt.Errorf("pdf: EndGroupWithOpacity without BeginGroup")
		return
	}
	if !finite(opacity) {
		d.err = fmt.Errorf("pdf: group opacity must be finite")
		return
	}
	d.closeGroup(min(1, max(0, opacity)), multiply)
}

func (d *Document) closeGroup(opacity float64, multiply bool) {
	g := d.groups[len(d.groups)-1]
	d.groups = d.groups[:len(d.groups)-1]
	d.current = g.parent
	c := d.current
	state := d.alpha(alphaState{opacity: opacity, multiply: multiply, mask: c.state.mask})
	c.states[state], c.forms[g.formID] = true, true
	// Child operations already contain their absolute CSS transforms and clips.
	// The Form uses page coordinates so singular drawing matrices remain valid.
	fmt.Fprintf(c, "q\n/GS%d gs\n/X%d Do\nQ\n", state+1, g.formID+1)
}

// Glyph emits one positioned glyph. Unsupported fonts return false for outline fallback.
// The text is optional Unicode metadata for text extraction.
func (d *Document) Glyph(face *font.Face, gid uint32, text string, size, x, y float64, style Style) bool {
	if !d.ready() || face == nil || face.Path == "" {
		return false
	}
	if !finite(size) || !finite(x) || !finite(y) || size < 0 {
		d.err = fmt.Errorf("pdf: glyph position and size must be finite, with a nonnegative size")
		return false
	}
	id, cid, ok := d.encodeGlyph(face, gid, text, style.Stroke)
	if !ok {
		return false
	}
	if !d.paintStart(style) {
		return false
	}
	c := d.current
	c.fonts[id] = true
	mode := 0
	if style.Stroke {
		mode = 1
	}
	code := fmt.Sprintf("%04X", cid)
	if encoder, ok := d.fonts[id].(interface{ glyphCode(uint16) string }); ok {
		code = encoder.glyphCode(cid)
	}
	fmt.Fprintf(c, "BT\n/F%d %s Tf\n%d Tr\n1 0 0 -1 %s %s Tm\n<%s> Tj\nET\nQ\n", id+1, number(size), mode, number(x), number(y), code)
	return true
}

func (d *Document) encodeGlyph(face *font.Face, gid uint32, text string, stroke bool) (int, uint16, bool) {
	if d.fontIDs == nil {
		d.fontIDs, d.badFonts = make(map[string]int), make(map[string]bool)
	}
	if face.Index == 0 {
		if id, ok := d.fontIDs[face.Path]; ok {
			cid, ok := d.fonts[id].encodeGlyph(gid, text)
			return id, cid, ok
		}
		if !d.badFonts[face.Path] {
			resource, err := newFontResource(face)
			if err == nil {
				cid, ok := resource.encodeGlyph(gid, text)
				if !ok {
					return 0, 0, false
				}
				id := len(d.fonts)
				d.fonts = append(d.fonts, resource)
				d.fontIDs[face.Path] = id
				return id, cid, true
			}
			d.badFonts[face.Path] = true
		}
	}
	// Type3 glyph procedures fill their outlines and do not honor text stroke mode.
	if stroke {
		return 0, 0, false
	}
	if d.type3IDs == nil {
		d.type3IDs, d.badType3 = make(map[*font.Face][]int), make(map[*font.Face]bool)
	}
	if d.badType3[face] {
		return 0, 0, false
	}
	for _, id := range d.type3IDs[face] {
		if cid, ok := d.fonts[id].encodeGlyph(gid, text); ok {
			return id, cid, true
		}
	}
	resource, err := newType3Resource(face)
	if err != nil {
		d.badType3[face] = true
		return 0, 0, false
	}
	cid, ok := resource.encodeGlyph(gid, text)
	if !ok {
		return 0, 0, false
	}
	id := len(d.fonts)
	d.fonts = append(d.fonts, resource)
	d.type3IDs[face] = append(d.type3IDs[face], id)
	return id, cid, true
}

// Bytes serializes the page as a PDF document.
func (d *Document) Bytes() ([]byte, error) {
	if !d.ready() {
		return nil, d.err
	}
	if len(d.groups) > 0 {
		return nil, fmt.Errorf("pdf: document has an open group")
	}
	w := &objectWriter{}
	catalog := w.addObject("")
	pageTree := w.addObject("")
	fontRefs := make([]int, len(d.fonts))
	for i, font := range d.fonts {
		fontRefs[i] = font.writePDF(w)
	}
	formRefs := make([]int, len(d.forms))
	for i := range formRefs {
		formRefs[i] = w.addObject("")
	}
	stateRefs := make([]int, len(d.states))
	for i, state := range d.states {
		blend := "Normal"
		if state.multiply {
			blend = "Multiply"
		}
		mask := "/None"
		if state.mask > 0 {
			mask = fmt.Sprintf("<< /S /Alpha /G %d 0 R >>", formRefs[state.mask-1])
		}
		stateRefs[i] = w.addObject(fmt.Sprintf("<< /Type /ExtGState /ca %s /CA %s /BM /%s /SMask %s >>", number(state.opacity), number(state.opacity), blend, mask))
	}
	shadingRefs := make([]int, len(d.shadings))
	maskRefs := make([]int, len(d.shadings))
	for i, shading := range d.shadings {
		shadingRefs[i] = shading.writePDF(w)
		if masked, ok := shading.(interface {
			writeMaskPDF(*objectWriter, []int) int
		}); ok {
			maskRefs[i] = masked.writeMaskPDF(w, formRefs)
		}
	}
	for i, form := range d.forms {
		dict := fmt.Sprintf("/Type /XObject /Subtype /Form /FormType 1 /BBox [0 0 %s %s] /Group << /S /Transparency /I true /K false /CS /DeviceRGB >> /Resources %s", number(form.width), number(form.height), form.content.resources.pdf(fontRefs, stateRefs, formRefs, shadingRefs, maskRefs))
		ref := w.addStream(dict, form.content.Bytes())
		// The reserved object keeps forward references stable for nested groups.
		w.objects[formRefs[i]-1] = w.objects[ref-1]
		w.objects = w.objects[:ref-1]
	}
	var kids strings.Builder
	for _, page := range d.pages {
		var content bytes.Buffer
		fmt.Fprintf(&content, "q\n0.75 0 0 -0.75 0 %s cm\n", number(page.height*.75))
		content.Write(page.content.Bytes())
		content.WriteString("Q\n")
		stream := w.addStream("", content.Bytes())
		ref := w.addObject(fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources %s /Contents %d 0 R /Group << /S /Transparency /CS /DeviceRGB >> >>", pageTree, number(page.width*.75), number(page.height*.75), page.content.resources.pdf(fontRefs, stateRefs, formRefs, shadingRefs, maskRefs), stream))
		fmt.Fprintf(&kids, "%d 0 R ", ref)
	}
	w.objects[pageTree-1] = []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), len(d.pages)))
	w.objects[catalog-1] = []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pageTree))
	return w.bytes(catalog)
}

func (r resources) pdf(fontRefs, stateRefs, formRefs, shadingRefs, maskRefs []int) string {
	var out strings.Builder
	out.WriteString("<<")
	for _, kind := range []struct {
		key, prefix string
		used        map[int]bool
		refs        []int
	}{{"Font", "F", r.fonts, fontRefs}, {"ExtGState", "GS", r.states, stateRefs}, {"XObject", "X", r.forms, formRefs}, {"Shading", "Sh", r.shadings, shadingRefs}} {
		if len(kind.used) == 0 && (kind.key != "ExtGState" || len(r.masks) == 0) {
			continue
		}
		fmt.Fprintf(&out, " /%s <<", kind.key)
		ids := make([]int, 0, len(kind.used))
		for id := range kind.used {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			fmt.Fprintf(&out, " /%s%d %d 0 R", kind.prefix, id+1, kind.refs[id])
		}
		if kind.key == "ExtGState" {
			ids = ids[:0]
			for id := range r.masks {
				ids = append(ids, id)
			}
			sort.Ints(ids)
			for _, id := range ids {
				fmt.Fprintf(&out, " /SM%d %d 0 R", id+1, maskRefs[id])
			}
		}
		out.WriteString(" >>")
	}
	out.WriteString(" >>")
	return out.String()
}
