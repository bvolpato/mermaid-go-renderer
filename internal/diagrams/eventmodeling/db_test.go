package eventmodeling

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
	"github.com/bvolpato/mermaid-go-renderer/internal/purify"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// The golden file is what the eventmodeling db of real mermaid holds after
// it parsed a text inside a real render: getDiagramProps() and getState().
// tools/reference/gen-eventmodeling-db.mjs writes it with the browser
// oracle. The test renders the same texts up to the draw function and
// compares everything, order included, with exact numbers. For a text that
// mermaid rejects it compares the error.

type goldenFile struct {
	Props map[string]any `json:"props"`
	Cases []goldenCase   `json:"cases"`
}

type goldenCase struct {
	Name string `json:"name"`
	// Text is absent for a corpus fixture: Name is then its path.
	Text *string `json:"text"`
	// Site is the site config: the argument of mermaid.initialize.
	Site      map[string]any `json:"site"`
	Error     *string        `json:"error"`
	ErrorName string         `json:"errorName"`
	Type      string         `json:"type"`

	StateKeys              []string `json:"stateKeys"`
	MaxR                   any      `json:"maxR"`
	PreviousSwimlaneNumber any      `json:"previousSwimlaneNumber"`
	PreviousFrame          any      `json:"previousFrame"`
	SwimlaneKeys           []string `json:"swimlaneKeys"`
	Swimlanes              []any    `json:"swimlanes"`
	SortedSwimlanes        []any    `json:"sortedSwimlanes"`
	Boxes                  []any    `json:"boxes"`
	Relations              []any    `json:"relations"`
	AccTitle               string   `json:"accTitle"`
	AccDescription         string   `json:"accDescription"`
	DiagramTitle           string   `json:"diagramTitle"`
}

func loadGolden(tb testing.TB) *goldenFile {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "db.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		tb.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

func caseText(tb testing.TB, c *goldenCase) string {
	tb.Helper()
	if c.Text != nil {
		return *c.Text
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(c.Name)))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

// errorMessage is the name and the message of the JavaScript exception.
func errorMessage(err error) (name, message string) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message
	}
	var le *langium.Error
	if errors.As(err, &le) {
		return "Error", le.Error()
	}
	var ee *Error
	if errors.As(err, &ee) {
		if ee.Name == "" {
			return "Error", ee.Message
		}
		return ee.Name, ee.Message
	}
	var te *purify.TypeError
	if errors.As(err, &te) {
		return "TypeError", te.Message
	}
	return "Error", err.Error()
}

// ---- the draw function that keeps the db ----

// errCaptured is the error of the capturing draw function: it stops the
// render, as the draw function of the page script does.
var errCaptured = errors.New("the db is captured")

// capture is what the capturing functions keep.
type capture struct {
	// code is the DiagramCode object, as renderDiagram computes it.
	code  config.DiagramCode
	db    *DB
	state *Context
}

var (
	captureOnce sync.Once
	captureMu   sync.Mutex
	// capturing is not nil while renderToDraw runs.
	capturing *capture
)

// sourceText is the choice of Diagram.fromText between the cleaned text and
// the text with its comments, for a db with preserveCommentsWhenParsing.
// The pipeline of internal/render does not make this choice yet: it parses
// the cleaned text. The capture of the test makes it, so that the test
// compares the db for the text that upstream parses.
func sourceText(code config.DiagramCode, cfg *js.Obj[any], d *DB) string {
	source := code.Cleaned
	if d.PreserveCommentsWhenParsing() && code.WithComments != "" {
		// config.maxTextSize ?? defaultConfig.maxTextSize
		var maxTextSize any = 50000.0
		if v := cfg.Get("maxTextSize"); !js.IsNullish(v) {
			maxTextSize = v
		}
		if !js.Compare(float64(js.UTF16Len(code.WithComments)), ">", maxTextSize) {
			source = code.WithComments
		}
	}
	return source
}

// installCapture replaces the functions of the eventmodeling definition
// with functions that, while a capture is active, parse the text that
// upstream parses and keep the db and its state instead of drawing.
func installCapture() {
	def, ok := render.Lookup("eventmodeling")
	if !ok {
		panic("the eventmodeling diagram is not registered")
	}
	wrapped := *def
	realParse, realDraw := def.Parse, def.Draw
	wrapped.Parse = func(ctx *render.Context, db render.DB, text string) error {
		if capturing == nil {
			return realParse(ctx, db, text)
		}
		// The step of Diagram.fromText that the pipeline does not have.
		textToParse := render.EncodeEntities(sourceText(capturing.code, ctx.Config, db.(*DB))) + "\n"
		return realParse(ctx, db, textToParse)
	}
	wrapped.Draw = func(ctx *render.Context, db render.DB, text string) error {
		if capturing == nil {
			return realDraw(ctx, db, text)
		}
		d := db.(*DB)
		state, err := d.GetState()
		if err != nil {
			return err
		}
		capturing.db, capturing.state = d, state
		return errCaptured
	}
	render.Register(&wrapped)
}

func siteConfig(site map[string]any) *js.Obj[any] {
	// The page of the oracle calls mermaid.initialize({ startOnLoad: false, ...site }).
	cfg := js.NewObj[any]()
	cfg.Set("startOnLoad", false)
	if site != nil {
		data, _ := json.Marshal(site)
		v, err := js.ParseJSON(data)
		if err != nil {
			panic(err)
		}
		js.AsObj(v).Each(func(k string, v any) { cfg.Set(k, v) })
	}
	return cfg
}

// renderToDraw renders text up to the draw function.
func renderToDraw(text string, site map[string]any) (got *capture, err error) {
	captureOnce.Do(installCapture)
	captureMu.Lock()
	defer captureMu.Unlock()
	cp := &capture{}
	if resolved, err := config.Resolve(text, siteConfig(site)); err == nil {
		cp.code = resolved.Source
		maxTextSize := 50000.0
		if v, ok := resolved.Config.Get("maxTextSize").(float64); ok {
			maxTextSize = v
		}
		if float64(js.UTF16Len(cp.code.Cleaned)) > maxTextSize {
			// The text is replaced by the text of the error diagram.
			cp.code = config.DiagramCode{}
		}
	}
	capturing = cp
	defer func() { capturing = nil }()
	_, err = render.Render(text, render.Options{Config: siteConfig(site)})
	if errors.Is(err, errCaptured) {
		return cp, nil
	}
	return nil, err
}

// ---- conversion of the Go side to JSON data ----

// num is a number as the oracle writes it: a number that is not finite is
// { $num: "NaN" }.
func num(x float64) any {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return map[string]any{"$num": js.Str(x)}
	}
	return x
}

// color is a color of the visual properties: a value of the theme
// variables.
func color(v any) any {
	switch x := v.(type) {
	case string, bool:
		return x
	case float64:
		return num(x)
	}
	panic(fmt.Sprintf("color: unexpected %T", v))
}

func visualJSON(v VisualProps) map[string]any {
	return map[string]any{"fill": color(v.Fill), "stroke": color(v.Stroke)}
}

func index[T comparable](list []T, v T) float64 {
	for i, e := range list {
		if e == v {
			return float64(i)
		}
	}
	return -1
}

func show(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func TestDBAgainstMermaid(t *testing.T) {
	g := loadGolden(t)
	if len(g.Cases) < 200 {
		t.Fatalf("only %d cases", len(g.Cases))
	}
	// getDiagramProps() is the same in every case.
	props := map[string]any{}
	data, _ := json.Marshal(diagramProps)
	var fields map[string]any
	_ = json.Unmarshal(data, &fields)
	for k, v := range fields {
		props[strings.ToLower(k[:1])+k[1:]] = v
	}
	if !reflect.DeepEqual(props, g.Props) {
		t.Errorf("getDiagramProps():\n got  %s\n want %s", show(props), show(g.Props))
	}

	reference := fonttest.IsMacOSReference()
	passed, rejected, models := 0, 0, 0
	for i := range g.Cases {
		c := &g.Cases[i]
		text := caseText(t, c)
		fail := func(format string, args ...any) {
			t.Errorf("%s: %s\n--- text ---\n%s", c.Name, fmt.Sprintf(format, args...), text)
		}
		got, err := renderToDraw(text, c.Site)
		if c.Error != nil {
			rejected++
			if err == nil {
				fail("no error, mermaid fails with %s: %q", c.ErrorName, *c.Error)
				continue
			}
			name, message := errorMessage(err)
			// The texts of the errors of the parser are not part of the
			// contract: only "error or no error" is.
			var parseError *langium.Error
			if errors.As(err, &parseError) && strings.HasPrefix(*c.Error, "Parsing failed:") {
				passed++
				continue
			}
			if name != c.ErrorName || message != *c.Error {
				fail("error %s: %q\n     want %s: %q", name, message, c.ErrorName, *c.Error)
				continue
			}
			passed++
			continue
		}
		if c.Type != "eventmodeling" {
			// The test binary has no other diagram type.
			var unsupported *render.UnsupportedError
			if !errors.As(err, &unsupported) || unsupported.Type != c.Type {
				fail("got %v, mermaid renders the type %s", err, c.Type)
				continue
			}
			passed++
			continue
		}
		models++
		if err != nil {
			fail("error %v, mermaid has no error", err)
			continue
		}
		ok := true
		check := func(what string, got, want any) {
			if !reflect.DeepEqual(got, want) {
				ok = false
				fail("%s:\n got  %s\n want %s", what, show(got), show(want))
			}
		}
		// measured is check for a part of the state that has sizes and
		// positions. They come from the text sizes of Chromium on the
		// macOS reference machine, so a machine with other fonts does not
		// compare them.
		measured := func(what string, got, want any) {
			if reference {
				check(what, got, want)
			}
		}
		state, frames := got.state, got.db.ast.Frames

		stateKeys := []string{"boxes", "swimlanes", "relations", "maxR", "sortedSwimlanesArray"}
		if state.HasPreviousSwimlaneNumber {
			stateKeys = append(stateKeys, "previousSwimlaneNumber", "previousFrame")
		}
		check("the keys of the state", stateKeys, c.StateKeys)
		measured("maxR", num(state.MaxR), c.MaxR)
		var previousSwimlaneNumber, previousFrame any
		if state.HasPreviousSwimlaneNumber {
			previousSwimlaneNumber = float64(state.PreviousSwimlaneNumber)
		}
		if state.PreviousFrame != nil {
			previousFrame = index(frames, state.PreviousFrame)
		}
		check("previousSwimlaneNumber", previousSwimlaneNumber, c.PreviousSwimlaneNumber)
		check("previousFrame", previousFrame, c.PreviousFrame)

		keys := state.Swimlanes.Keys()
		if keys == nil {
			keys = []string{}
		}
		check("the keys of the swimlanes", keys, c.SwimlaneKeys)
		swimlanes := []any{}
		for _, s := range state.Swimlanes.Values() {
			swimlane := map[string]any{
				"index":     float64(s.Index),
				"label":     s.Label,
				"r":         num(s.R),
				"y":         num(s.Y),
				"height":    num(s.Height),
				"maxHeight": num(s.MaxHeight),
			}
			if s.HasNamespace {
				swimlane["namespace"] = s.Namespace
			}
			swimlanes = append(swimlanes, swimlane)
		}
		measured("swimlanes", swimlanes, c.Swimlanes)
		sorted := []any{}
		for _, s := range state.SortedSwimlanesArray {
			sorted = append(sorted, float64(s.Index))
			if state.Swimlanes.Get(strconv.Itoa(s.Index)) != s {
				ok = false
				fail("the sorted swimlane %d is not the swimlane of the state", s.Index)
			}
		}
		check("sortedSwimlanesArray", sorted, c.SortedSwimlanes)

		boxes := []any{}
		for _, b := range state.Boxes {
			boxes = append(boxes, map[string]any{
				"r":           num(b.R),
				"x":           num(b.X),
				"y":           num(b.Y),
				"dimension":   map[string]any{"width": num(b.Dimension.Width), "height": num(b.Dimension.Height)},
				"leftSibling": b.LeftSibling,
				"swimlane":    float64(b.Swimlane.Index),
				"visual":      visualJSON(b.Visual),
				"text":        b.Text,
				"frame":       index(frames, b.Frame),
				"index":       float64(b.Index),
			})
		}
		measured("boxes", boxes, c.Boxes)
		relations := []any{}
		for _, r := range state.Relations {
			relations = append(relations, map[string]any{
				"visual":    visualJSON(r.Visual),
				"source":    map[string]any{"x": num(r.Source.X), "y": num(r.Source.Y)},
				"target":    map[string]any{"x": num(r.Target.X), "y": num(r.Target.Y)},
				"sourceBox": index(state.Boxes, r.SourceBox),
				"targetBox": index(state.Boxes, r.TargetBox),
			})
		}
		measured("relations", relations, c.Relations)
		check("accTitle", got.db.GetAccTitle(), c.AccTitle)
		check("accDescription", got.db.GetAccDescription(), c.AccDescription)
		check("diagramTitle", got.db.GetDiagramTitle(), c.DiagramTitle)
		if ok {
			passed++
		}
	}
	t.Logf("%d cases: %d pass, %d event models, %d that mermaid rejects", len(g.Cases), passed, models, rejected)
	fonttest.RequireMacOSReference(t)
}
