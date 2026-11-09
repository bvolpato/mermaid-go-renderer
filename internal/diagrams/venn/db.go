// Ported from mermaid 12.2.0, diagrams/venn/vennDB.ts and vennTypes.ts (MIT).

package venn

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// SubsetData is VennData: a set, or a union of sets. HasLabel is false when
// the label is undefined.
type SubsetData struct {
	Sets     []string
	Size     float64
	Label    string
	HasLabel bool
}

// TextData is VennTextData: a text node in an area. HasLabel is false when
// the label is undefined.
type TextData struct {
	Sets     []string
	ID       string
	Label    string
	HasLabel bool
}

// StyleData is VennStyleData: the styles of a style statement. Styles has
// the properties in the order of a JavaScript object.
type StyleData struct {
	Targets []string
	Styles  *js.Obj[string]
}

// DB is the db of the venn diagram: the module state of vennDB.ts.
type DB struct {
	render.CommonDB

	ctx *render.Context

	subsets      []*SubsetData
	textNodes    []*TextData
	styleEntries []*StyleData
	knownSets    map[string]bool
	// currentSets is undefined upstream until the first set or union.
	currentSets    []string
	hasCurrentSets bool
	indentMode     bool

	// consumeIndentText is the property yy.consumeIndentText that the lexer
	// sets on the db.
	consumeIndentText bool
}

// NewDB returns an empty db. It stands in for clear() of the module.
func NewDB(ctx *render.Context) *DB {
	db := &DB{ctx: ctx, knownSets: map[string]bool{}}
	db.Sanitize = ctx.SanitizeText
	return db
}

// AddSubsetData is addSubsetData(identifierList, label, size). label and
// size are nil when they are undefined.
func (db *DB) AddSubsetData(identifierList []string, label *string, size *float64) {
	sets := normalizeIdentifierList(identifierList)
	js.SortStrings(sets)
	resolvedSize := 10 / js.Pow(float64(len(identifierList)), 2)
	if size != nil {
		resolvedSize = *size
	}
	db.currentSets, db.hasCurrentSets = sets, true
	if len(sets) == 1 {
		db.knownSets[sets[0]] = true
	}

	subset := &SubsetData{Sets: sets, Size: resolvedSize}
	if label != nil && *label != "" {
		subset.Label, subset.HasLabel = normalizeText(*label), true
	}
	db.subsets = append(db.subsets, subset)
}

// GetSubsetData is getSubsetData().
func (db *DB) GetSubsetData() []*SubsetData { return db.subsets }

// normalizeText is normalizeText(text).
func normalizeText(text string) string {
	trimmed := js.TrimSpace(text)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) {
		return trimmed[1 : len(trimmed)-1]
	}
	return trimmed
}

// normalizeStyleValue is normalizeStyleValue(value) for a string.
func normalizeStyleValue(value string) string {
	if value != "" {
		return normalizeText(value)
	}
	return value
}

// AddTextData is addTextData(identifierList, id, label). label is nil when
// it is undefined.
func (db *DB) AddTextData(identifierList []string, id string, label *string) {
	normalizedID := normalizeText(id)
	sets := normalizeIdentifierList(identifierList)
	js.SortStrings(sets)
	node := &TextData{Sets: sets, ID: normalizedID}
	if label != nil && *label != "" {
		node.Label, node.HasLabel = normalizeText(*label), true
	}
	db.textNodes = append(db.textNodes, node)
}

// AddStyleData is addStyleData(identifierList, data).
func (db *DB) AddStyleData(identifierList []string, data [][2]string) {
	targets := normalizeIdentifierList(identifierList)
	js.SortStrings(targets)
	styles := js.NewObj[string]()
	for _, entry := range data {
		key, value := entry[0], entry[1]
		if key == "__proto__" {
			// An assignment of a string to this key of a plain object
			// does nothing in JavaScript.
			continue
		}
		styles.Set(key, normalizeStyleValue(value))
	}
	db.styleEntries = append(db.styleEntries, &StyleData{Targets: targets, Styles: styles})
}

// GetStyleData is getStyleData().
func (db *DB) GetStyleData() []*StyleData { return db.styleEntries }

// normalizeIdentifierList is normalizeIdentifierList(identifierList).
func normalizeIdentifierList(identifierList []string) []string {
	out := make([]string, len(identifierList))
	for i, identifier := range identifierList {
		out[i] = normalizeText(identifier)
	}
	return out
}

// ValidateUnionIdentifiers is validateUnionIdentifiers(identifierList).
func (db *DB) ValidateUnionIdentifiers(identifierList []string) error {
	normalized := normalizeIdentifierList(identifierList)
	unknown := []string{}
	for _, identifier := range normalized {
		if !db.knownSets[identifier] {
			unknown = append(unknown, identifier)
		}
	}
	if len(unknown) > 0 {
		return errors.New("unknown set identifier: " + strings.Join(unknown, ", "))
	}
	return nil
}

// GetTextData is getTextData().
func (db *DB) GetTextData() []*TextData { return db.textNodes }

// GetCurrentSets is getCurrentSets(). The second result is false when the
// value is undefined.
func (db *DB) GetCurrentSets() ([]string, bool) { return db.currentSets, db.hasCurrentSets }

// GetIndentMode is getIndentMode().
func (db *DB) GetIndentMode() bool { return db.indentMode }

// SetIndentMode is setIndentMode(enabled).
func (db *DB) SetIndentMode(enabled bool) { db.indentMode = enabled }

// ConsumeIndentText reads the property yy.consumeIndentText of the lexer.
func (db *DB) ConsumeIndentText() bool { return db.consumeIndentText }

// SetConsumeIndentText writes the property yy.consumeIndentText.
func (db *DB) SetConsumeIndentText(v bool) { db.consumeIndentText = v }

// GetConfig is getConfig(): the venn config merged on the defaults.
func (db *DB) GetConfig() *js.Obj[any] {
	var current any
	if db.ctx != nil {
		current = db.ctx.Cfg("venn")
	}
	return utils.CleanAndMerge(config.DefaultSection("venn"), current)
}
