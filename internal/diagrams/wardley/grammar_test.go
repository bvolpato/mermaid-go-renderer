package wardley

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// astGolden is testdata/ast.json.gz, which
// tools/reference/gen-treeview-cynefin-wardley-ast.mjs wrote with the real
// parser: texts with the AST of parse('wardley', text), or the error.
type astGolden struct {
	Fixtures int `json:"fixtures"`
	Cases    []struct {
		Text  string  `json:"text"`
		AST   any     `json:"ast"`
		Error *string `json:"error"`
	} `json:"cases"`
}

func readGolden(t testing.TB, name string, v any) {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		t.Fatal(err)
	}
}

// number is a JavaScript number in the form of the golden files: NaN, the
// infinities and -0 are {"$n":"<text>"}, because JSON has no text for them.
func number(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return map[string]any{"$n": js.Str(v)}
	}
	if v == 0 && math.Signbit(v) {
		return map[string]any{"$n": "-0"}
	}
	return v
}

func plainLabel(l *Label) any {
	return map[string]any{
		"$type":   "Label",
		"negX":    l.NegX,
		"offsetX": number(l.OffsetX),
		"negY":    l.NegY,
		"offsetY": number(l.OffsetY),
	}
}

func plainAccelerators(kind string, list []Accelerator) []any {
	out := []any{}
	for _, a := range list {
		out = append(out, map[string]any{"$type": kind, "name": a.Name, "x": number(a.X), "y": number(a.Y)})
	}
	return out
}

// plain is the AST in the form of the golden file.
func (ast *Wardley) plain() any {
	out := map[string]any{"$type": "Wardley"}
	if ast.AccDescr != nil {
		out["accDescr"] = *ast.AccDescr
	}
	if ast.AccTitle != nil {
		out["accTitle"] = *ast.AccTitle
	}
	if ast.Title != nil {
		out["title"] = *ast.Title
	}
	if ast.Size != nil {
		out["size"] = map[string]any{"$type": "Size", "width": number(ast.Size.Width), "height": number(ast.Size.Height)}
	}
	if ast.Evolution != nil {
		stages := []any{}
		for _, s := range ast.Evolution.Stages {
			stage := map[string]any{"$type": "EvolutionStage", "name": s.Name}
			if s.Boundary != nil {
				stage["boundary"] = number(*s.Boundary)
			}
			if s.SecondName != nil {
				stage["secondName"] = *s.SecondName
			}
			stages = append(stages, stage)
		}
		out["evolution"] = map[string]any{"$type": "Evolution", "stages": stages}
	}
	anchors := []any{}
	for _, a := range ast.Anchors {
		anchors = append(anchors, map[string]any{
			"$type": "Anchor", "name": a.Name, "visibility": number(a.Visibility), "evolution": number(a.Evolution),
		})
	}
	out["anchors"] = anchors
	components := []any{}
	for _, c := range ast.Components {
		component := map[string]any{
			"$type": "Component", "name": c.Name, "visibility": number(c.Visibility), "evolution": number(c.Evolution),
			"inertia": c.Inertia,
		}
		if c.Label != nil {
			component["label"] = plainLabel(c.Label)
		}
		if c.Decorator != nil {
			component["decorator"] = map[string]any{"$type": "Decorator", "strategy": c.Decorator.Strategy}
		}
		components = append(components, component)
	}
	out["components"] = components
	links := []any{}
	for _, l := range ast.Links {
		link := map[string]any{"$type": "Link", "from": l.From, "to": l.To}
		if l.FromPort != nil {
			link["fromPort"] = *l.FromPort
		}
		if l.Arrow != nil {
			link["arrow"] = *l.Arrow
		}
		if l.ToPort != nil {
			link["toPort"] = *l.ToPort
		}
		if l.LinkLabel != nil {
			link["linkLabel"] = *l.LinkLabel
		}
		links = append(links, link)
	}
	out["links"] = links
	evolves := []any{}
	for _, e := range ast.Evolves {
		evolves = append(evolves, map[string]any{"$type": "Evolve", "component": e.Component, "target": number(e.Target)})
	}
	out["evolves"] = evolves
	pipelines := []any{}
	for _, p := range ast.Pipelines {
		pipelineComponents := []any{}
		for _, c := range p.Components {
			component := map[string]any{"$type": "PipelineComponent", "name": c.Name, "evolution": number(c.Evolution)}
			if c.Label != nil {
				component["label"] = plainLabel(c.Label)
			}
			pipelineComponents = append(pipelineComponents, component)
		}
		pipelines = append(pipelines, map[string]any{"$type": "Pipeline", "parent": p.Parent, "components": pipelineComponents})
	}
	out["pipelines"] = pipelines
	notes := []any{}
	for _, n := range ast.Notes {
		notes = append(notes, map[string]any{
			"$type": "Note", "text": n.Text, "visibility": number(n.Visibility), "evolution": number(n.Evolution),
		})
	}
	out["notes"] = notes
	annotations := []any{}
	for _, a := range ast.Annotations {
		annotations = append(annotations, map[string]any{"$type": "Annotations", "x": number(a.X), "y": number(a.Y)})
	}
	out["annotations"] = annotations
	annotation := []any{}
	for _, a := range ast.Annotation {
		annotation = append(annotation, map[string]any{
			"$type": "Annotation", "number": number(a.Number), "x": number(a.X), "y": number(a.Y), "text": a.Text,
		})
	}
	out["annotation"] = annotation
	out["accelerators"] = plainAccelerators("Accelerator", ast.Accelerators)
	out["deaccelerators"] = plainAccelerators("Deaccelerator", ast.Deaccelerators)
	return out
}

// TestParse compares the parser with parse('wardley', text) of the parser
// package of mermaid, for the corpus fixtures, hand-written inputs and
// mutated inputs. A text that the real parser rejects must give an error.
func TestParse(t *testing.T) {
	var golden astGolden
	readGolden(t, "ast.json.gz", &golden)
	if len(golden.Cases) < 5000 || golden.Fixtures < 20 {
		t.Fatalf("the golden file has %d cases, %d from fixtures", len(golden.Cases), golden.Fixtures)
	}
	valid := 0
	for _, c := range golden.Cases {
		ast, err := parse(c.Text)
		if c.Error != nil {
			if err == nil {
				t.Errorf("text %q: no error, the real parser fails with %q", c.Text, *c.Error)
			}
			continue
		}
		valid++
		if err != nil {
			t.Errorf("text %q: error %v, the real parser accepts the text", c.Text, err)
			continue
		}
		if got := ast.plain(); !reflect.DeepEqual(got, c.AST) {
			t.Errorf("text %q:\n got %v\nwant %v", c.Text, got, c.AST)
		}
	}
	t.Logf("%d cases, %d that the real parser accepts", len(golden.Cases), valid)
}
