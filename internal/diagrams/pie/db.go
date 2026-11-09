// Ported from mermaid 12.2.0, diagrams/pie/pieDb.ts and pieTypes.ts (MIT).

package pie

import (
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// D3Section is one section: a label and a value.
type D3Section struct {
	Label string
	Value float64
}

// DB is the db of the pie chart.
type DB struct {
	render.CommonDB
	sections *js.Map[string, float64]
	showData bool
}

func newDB(ctx *render.Context) *DB {
	db := &DB{sections: js.NewMap[string, float64]()}
	db.Sanitize = ctx.SanitizeText
	return db
}

// GetConfig is getConfig(): a copy of the default config of the pie chart.
func (db *DB) GetConfig() *js.Obj[any] {
	return js.AsObj(config.DefaultSection("pie"))
}

// AddSection is addSection({ label, value }).
func (db *DB) AddSection(section D3Section) error {
	if section.Value < 0 {
		return fmt.Errorf(`"%s" has invalid value: %s. Negative values are not allowed in pie charts. All slice values must be >= 0.`,
			section.Label, js.Str(section.Value))
	}
	if !db.sections.Has(section.Label) {
		db.sections.Set(section.Label, section.Value)
	}
	return nil
}

// GetSections is getSections().
func (db *DB) GetSections() *js.Map[string, float64] { return db.sections }

// SetShowData is setShowData(toggle).
func (db *DB) SetShowData(toggle bool) { db.showData = toggle }

// GetShowData is getShowData().
func (db *DB) GetShowData() bool { return db.showData }
