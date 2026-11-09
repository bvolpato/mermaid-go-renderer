package wardley

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// dbCase is one case of testdata/db.json.gz, which
// tools/reference/gen-treeview-cynefin-wardley-db.mjs wrote with mermaid
// 12.2.0 in Chromium: a text and what the db holds after mermaid.render
// parsed it, or the error of the render.
type dbCase struct {
	Name  string  `json:"name"`
	Text  string  `json:"text"`
	DB    any     `json:"db"`
	Error *string `json:"error"`
}

// undefined is the form of undefined in the golden file.
func undefined() any { return map[string]any{"$u": 1.0} }

// encode is the encode function of the generator for a dynamic value:
// undefined is {"$u":1}, and NaN, the infinities and -0 are
// {"$n":"<text>"}.
func encode(v any) any {
	switch x := v.(type) {
	case nil:
		return undefined()
	case float64:
		return number(x)
	case *js.Obj[any]:
		m := map[string]any{}
		x.Each(func(key string, value any) { m[key] = encode(value) })
		return m
	case []any:
		list := make([]any, len(x))
		for i, e := range x {
			list[i] = encode(e)
		}
		return list
	}
	if v == js.Null {
		return nil
	}
	return v
}

// parseLikeRender runs the steps of render.Render up to the parser: the
// config of the front matter and of the directives, the type detection, the
// title of the front matter, and the parser on the cleaned text.
func parseLikeRender(text string) (*DB, error) {
	resolved, err := config.Resolve(text, nil)
	if err != nil {
		return nil, err
	}
	code := resolved.Source.Cleaned
	diagramType, err := config.DetectType(code, resolved.Config)
	if err != nil {
		return nil, err
	}
	if diagramType != "wardley" {
		return nil, fmt.Errorf("detected type %s", diagramType)
	}
	ctx := render.NewContext(render.ContextOptions{Config: resolved.Config, ConfigState: resolved.State})
	db := newDB(ctx)
	if resolved.Title != "" {
		db.SetDiagramTitle(resolved.Title)
	}
	if err := parseDiagram(ctx, db, render.EncodeEntities(code)+"\n"); err != nil {
		return nil, err
	}
	return db, nil
}

// optional is a number that can be undefined.
func optional(v float64, has bool) any {
	if !has {
		return undefined()
	}
	return number(v)
}

// optionalText is a text that is undefined when it is empty.
func optionalText(text string) any {
	if text == "" {
		return undefined()
	}
	return text
}

func plainPoints(kind string, list []AcceleratorData) []any {
	out := []any{}
	for _, a := range list {
		out = append(out, map[string]any{kind: a.Name, "x": number(a.X), "y": number(a.Y)})
	}
	return out
}

// plainData is the result of getWardleyData() in the form of the golden
// file.
func plainData(data *BuildResult) any {
	nodes := []any{}
	for _, n := range data.Nodes {
		node := map[string]any{
			"id":             n.ID,
			"label":          n.Label,
			"x":              number(n.X),
			"y":              number(n.Y),
			"className":      optionalText(n.ClassName),
			"labelOffsetX":   optional(n.LabelOffsetX, n.HasLabelOffsetX),
			"labelOffsetY":   optional(n.LabelOffsetY, n.HasLabelOffsetY),
			"inertia":        undefined(),
			"sourceStrategy": optionalText(n.SourceStrategy),
		}
		if n.HasInertia {
			node["inertia"] = n.Inertia
		}
		if n.InPipeline {
			node["inPipeline"] = true
		}
		if n.IsPipelineParent {
			node["isPipelineParent"] = true
		}
		nodes = append(nodes, node)
	}
	links := []any{}
	for _, l := range data.Links {
		link := map[string]any{"source": l.Source, "target": l.Target, "dashed": l.Dashed, "label": undefined(), "flow": optionalText(l.Flow)}
		if l.HasLabel {
			link["label"] = l.Label
		}
		links = append(links, link)
	}
	trends := []any{}
	for _, t := range data.Trends {
		trends = append(trends, map[string]any{"nodeId": t.NodeID, "targetX": number(t.TargetX), "targetY": number(t.TargetY)})
	}
	pipelines := []any{}
	for _, p := range data.Pipelines {
		componentIDs := []any{}
		for _, id := range p.ComponentIDs {
			componentIDs = append(componentIDs, id)
		}
		pipelines = append(pipelines, map[string]any{"nodeId": p.NodeID, "componentIds": componentIDs})
	}
	annotations := []any{}
	for _, a := range data.Annotations {
		coordinates := []any{}
		for _, c := range a.Coordinates {
			coordinates = append(coordinates, map[string]any{"x": number(c.X), "y": number(c.Y)})
		}
		annotations = append(annotations, map[string]any{"number": number(a.Number), "coordinates": coordinates, "text": a.Text})
	}
	notes := []any{}
	for _, n := range data.Notes {
		notes = append(notes, map[string]any{"text": n.Text, "x": number(n.X), "y": number(n.Y)})
	}
	axes := map[string]any{}
	if data.Axes.XLabel != nil {
		axes["xLabel"] = *data.Axes.XLabel
	}
	if data.Axes.YLabel != nil {
		axes["yLabel"] = *data.Axes.YLabel
	}
	if data.Axes.Stages != nil {
		stages := []any{}
		for _, s := range data.Axes.Stages {
			stages = append(stages, s)
		}
		axes["stages"] = stages
	}
	if data.Axes.StageBoundaries != nil {
		boundaries := []any{}
		for _, b := range data.Axes.StageBoundaries {
			boundaries = append(boundaries, number(b))
		}
		axes["stageBoundaries"] = boundaries
	}
	out := map[string]any{
		"nodes":          nodes,
		"links":          links,
		"trends":         trends,
		"pipelines":      pipelines,
		"annotations":    annotations,
		"notes":          notes,
		"accelerators":   plainPoints("name", data.Accelerators),
		"deaccelerators": plainPoints("name", data.Deaccelerators),
		"annotationsBox": undefined(),
		"axes":           axes,
		"size":           undefined(),
	}
	if data.AnnotationsBox != nil {
		out["annotationsBox"] = map[string]any{"x": number(data.AnnotationsBox.X), "y": number(data.AnnotationsBox.Y)}
	}
	if data.Size != nil {
		out["size"] = map[string]any{"width": number(data.Size.Width), "height": number(data.Size.Height)}
	}
	return out
}

// plain is the db in the form of the golden file.
func (db *DB) plain() any {
	return map[string]any{
		"title":          db.GetDiagramTitle(),
		"accTitle":       db.GetAccTitle(),
		"accDescription": db.GetAccDescription(),
		"data":           plainData(db.GetWardleyData()),
		"config":         encode(db.GetConfig()),
	}
}

// TestDB compares the db with the db of real mermaid in Chromium after a
// render, for the corpus fixtures and for extra inputs, the order of the
// nodes, links and other lists included. A text that mermaid rejects must
// give an error.
func TestDB(t *testing.T) {
	var cases []dbCase
	readGolden(t, "db.json.gz", &cases)
	if len(cases) < 160 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	valid := 0
	for _, c := range cases {
		if c.Error != nil {
			if _, err := render.Render(c.Text, render.Options{}); err == nil {
				t.Errorf("%s: no error, mermaid fails with %q\n%s", c.Name, *c.Error, c.Text)
			}
			continue
		}
		valid++
		db, err := parseLikeRender(c.Text)
		if err != nil {
			t.Errorf("%s: error %v, mermaid accepts the text\n%s", c.Name, err, c.Text)
			continue
		}
		if got := db.plain(); !reflect.DeepEqual(got, c.DB) {
			t.Errorf("%s:\n got %v\nwant %v\n%s", c.Name, got, c.DB, c.Text)
		}
	}
	t.Logf("%d cases, %d that mermaid accepts", len(cases), valid)
}
