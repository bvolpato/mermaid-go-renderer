// Ported from mermaid 12.2.0, diagrams/common/commonDb.ts (MIT).

package render

import "regexp"

// CommonDB holds the accessible title, the accessible description and the
// diagram title. A diagram db embeds it to get the functions that every
// diagram shares.
type CommonDB struct {
	accTitle       string
	diagramTitle   string
	accDescription string
	// Sanitize is sanitizeText(text, getConfig()). A db sets it from its
	// context: db.Sanitize = ctx.SanitizeText.
	Sanitize func(text string) string
}

var (
	reLeadingSpace     = regexp.MustCompile(`^\s+`)
	reNewlineWithSpace = regexp.MustCompile(`\n\s+`)
)

func (d *CommonDB) sanitize(text string) string {
	if d.Sanitize != nil {
		return d.Sanitize(text)
	}
	return text
}

// ClearCommon is clear().
func (d *CommonDB) ClearCommon() {
	d.accTitle, d.diagramTitle, d.accDescription = "", "", ""
}

// SetAccTitle is setAccTitle(txt).
func (d *CommonDB) SetAccTitle(txt string) {
	d.accTitle = reLeadingSpace.ReplaceAllString(d.sanitize(txt), "")
}

// GetAccTitle is getAccTitle().
func (d *CommonDB) GetAccTitle() string { return d.accTitle }

// SetAccDescription is setAccDescription(txt).
func (d *CommonDB) SetAccDescription(txt string) {
	d.accDescription = reNewlineWithSpace.ReplaceAllString(d.sanitize(txt), "\n")
}

// GetAccDescription is getAccDescription().
func (d *CommonDB) GetAccDescription() string { return d.accDescription }

// SetDiagramTitle is setDiagramTitle(txt).
func (d *CommonDB) SetDiagramTitle(txt string) {
	d.diagramTitle = d.sanitize(txt)
}

// GetDiagramTitle is getDiagramTitle().
func (d *CommonDB) GetDiagramTitle() string { return d.diagramTitle }
