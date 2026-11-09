package railroad

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// The golden file comes from real mermaid in Chromium
// (tools/reference/gen-railroad-db.mjs): what the db holds after a real
// render of the corpus fixtures, of texts of the parser golden and of
// hand-written texts, or the error of mermaid.

type dbCase struct {
	Name  string          `json:"name"`
	Text  *string         `json:"text"`
	Site  json.RawMessage `json:"site"`
	Type  string          `json:"type"`
	Error *string         `json:"error"`
	DB    any             `json:"db"`
}

// siteConfig is the argument of mermaid.initialize in the golden script:
// { startOnLoad: false, ...site }.
func siteConfig(t testing.TB, raw json.RawMessage) *js.Obj[any] {
	t.Helper()
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	if len(raw) == 0 {
		return site
	}
	parsed, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if o := js.AsObj(parsed); o != nil {
		o.Each(func(key string, v any) { site.Set(key, v) })
	}
	return site
}

// parseLikeMermaid runs the steps of mermaid.render up to the end of the
// parse: the config of the front matter and of the directives, the type
// detection, a new db with the title of the front matter, and the parser of
// the diagram type on the cleaned text.
func parseLikeMermaid(text string, site *js.Obj[any]) (*DB, string, error) {
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, "", err
	}
	diagramType, err := config.DetectType(resolved.Source.Cleaned, resolved.Config)
	if err != nil {
		return nil, "", err
	}
	def, ok := render.Lookup(diagramType)
	if !ok || !strings.HasPrefix(diagramType, "railroad") {
		return nil, diagramType, errors.New("detected type " + diagramType)
	}
	ctx := &render.Context{ID: render.DefaultID, Type: diagramType, Config: resolved.Config, ConfigState: resolved.State, Title: resolved.Title, Sanitizer: sanitize.New()}
	db := def.New(ctx)
	if resolved.Title != "" {
		db.(render.TitleSetter).SetDiagramTitle(resolved.Title)
	}
	if err := def.Parse(ctx, db, render.EncodeEntities(resolved.Source.Cleaned)+"\n"); err != nil {
		return nil, diagramType, err
	}
	return db.(*DB), diagramType, nil
}

// undefinedValue is undefined in the form of the golden file.
var undefinedValue = map[string]any{"$u": 1.0}

// encodeNumber is a number in the form of the golden file: NaN and the
// infinities are {"$n": "<text>"}.
func encodeNumber(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return map[string]any{"$n": js.Str(v)}
	}
	return v
}

func encodeNodes(nodes []*Node) []any {
	out := make([]any, len(nodes))
	for i, node := range nodes {
		out[i] = encodeNode(node)
	}
	return out
}

// encodeNode is a node in the form of the golden file, with the properties
// that the node of upstream has.
func encodeNode(node *Node) any {
	m := map[string]any{"type": node.Type}
	switch node.Type {
	case typeTerminal:
		m["value"] = node.Value
	case typeNonTerminal:
		m["name"] = node.Name
	case typeSequence:
		m["elements"] = encodeNodes(node.Elements)
	case typeChoice:
		m["alternatives"] = encodeNodes(node.Alternatives)
	case typeOptional:
		m["element"] = encodeNode(node.Element)
	case typeRepetition:
		m["element"] = encodeNode(node.Element)
		m["min"] = encodeNumber(node.Min)
		m["max"] = encodeNumber(node.Max)
		// sanitizeAstNode gives every repetition the property separator.
		m["separator"] = undefinedValue
		if node.Separator != nil {
			m["separator"] = encodeNode(node.Separator)
		}
	case typeSpecial:
		m["text"] = node.Text
	}
	return m
}

// plain is the db in the form of the golden file.
func (d *DB) plain() any {
	rules := d.GetRules()
	list := make([]any, len(rules))
	for i, rule := range rules {
		m := map[string]any{"name": rule.Name, "definition": encodeNode(rule.Definition), "comment": undefinedValue}
		if rule.HasComment {
			m["comment"] = rule.Comment
		}
		list[i] = m
	}
	ruleMap := []any{}
	d.ruleMap.Each(func(name string, rule *Rule) {
		index := -1
		for i, r := range rules {
			if r == d.GetRule(name) {
				index = i
			}
		}
		ruleMap = append(ruleMap, []any{name, float64(index)})
	})
	return map[string]any{
		"title":          d.GetTitle(),
		"diagramTitle":   d.GetDiagramTitle(),
		"accTitle":       d.GetAccTitle(),
		"accDescription": d.GetAccDescription(),
		"rules":          list,
		"ruleMap":        ruleMap,
	}
}

// TestDBAgainstMermaid compares the db with the db of mermaid in Chromium,
// the order of the rules included, and requires an error where mermaid
// fails.
func TestDBAgainstMermaid(t *testing.T) {
	var golden struct {
		Cases []dbCase `json:"cases"`
	}
	loadGzipJSON(t, "testdata/db.json.gz", &golden)
	if len(golden.Cases) < 2000 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	byType := map[string]int{}
	rejected, fixtures := 0, 0
	for _, c := range golden.Cases {
		var text string
		if c.Text != nil {
			text = *c.Text
		} else {
			// A corpus fixture: the name is its path.
			data, err := os.ReadFile("../../../" + c.Name)
			if err != nil {
				t.Fatal(err)
			}
			text = string(data)
			fixtures++
		}
		db, diagramType, err := parseLikeMermaid(text, siteConfig(t, c.Site))
		if c.Error != nil {
			rejected++
			if err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%q", c.Name, *c.Error, text)
			}
			continue
		}
		if !strings.HasPrefix(c.Type, "railroad") {
			t.Errorf("%s: mermaid renders the type %q, which is not a railroad diagram\n%q", c.Name, c.Type, text)
			continue
		}
		if err != nil {
			t.Errorf("%s: error %v, mermaid accepts the text\n%q", c.Name, err, text)
			continue
		}
		if diagramType != c.Type {
			t.Errorf("%s: type %q, want %q", c.Name, diagramType, c.Type)
			continue
		}
		byType[c.Type]++
		if got := db.plain(); !reflect.DeepEqual(got, c.DB) {
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(c.DB)
			t.Errorf("%s: %q\n got %s\nwant %s", c.Name, text, gotJSON, wantJSON)
		}
	}
	if fixtures != 9 {
		t.Errorf("%d corpus fixtures, want 9", fixtures)
	}
	t.Logf("%d cases: %d that mermaid and the port reject, valid by type: %v", len(golden.Cases), rejected, byType)
}
