package architecture

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// The golden tests render through render.Render, as a caller of the
// renderer does, and they need the db and the call of cytoscape of each
// render. TestMain registers a definition that gives them: it finds the
// state of the test in the context of the render (render.Options.Context).
// A render with another context goes through the definition of the package
// with no change.

// renderKey is the key of the *observed of a render in its context.
type renderKey struct{}

// errStop ends a render after the db is filled.
var errStop = errors.New("stop before the draw")

// measureCall is one call of Env.MeasureText.
type measureCall struct {
	font, text string
	width      float64
}

// observed is what a render of a test gives.
type observed struct {
	// stop ends the render with errStop in place of the draw function.
	stop bool

	svg      string
	err      error
	db       *ArchitectureDB
	observer *layoutObserver
	measure  []measureCall
	// random is the number of draws of the layout from the generator of
	// the page.
	random int
}

func TestMain(m *testing.M) {
	production, _ := render.Lookup("architecture")
	render.Register(&render.Definition{
		ID: "architecture",
		New: func(ctx *render.Context) render.DB {
			db := NewDB(ctx)
			if o, ok := ctx.Cancel.Value(renderKey{}).(*observed); ok {
				o.db = db
				ctx.Value(layoutObserverKey, func() any { return o.observer })
			}
			return db
		},
		Parse: production.Parse,
		Draw: func(ctx *render.Context, db render.DB, text string) error {
			if o, ok := ctx.Cancel.Value(renderKey{}).(*observed); ok && o.stop {
				return errStop
			}
			return production.Draw(ctx, db, text)
		},
	})
	os.Exit(m.Run())
}

// renderWith renders a text through render.Render and returns the db, the
// call of cytoscape, the SVG and the error. With stop the draw function
// does not run.
func renderWith(text string, site *js.Obj[any], stop bool) *observed {
	o := &observed{stop: stop}
	o.observer = &layoutObserver{
		wrap: func(env cytoscape.Env) cytoscape.Env {
			random, measure := env.Random, env.MeasureText
			env.Random = func() float64 {
				o.random++
				return random()
			}
			env.MeasureText = func(font, text string) float64 {
				w := measure(font, text)
				o.measure = append(o.measure, measureCall{font, text, w})
				return w
			}
			return env
		},
	}
	res, err := render.Render(text, render.Options{
		Config:  site,
		Context: context.WithValue(context.Background(), renderKey{}, o),
	})
	if err != nil {
		o.err = err
		return o
	}
	o.svg = res.SVG
	return o
}

// renders keeps the whole render of each text, so that the tests of the db
// and of the layout render a text one time.
var (
	rendersMu sync.Mutex
	renders   = map[string]*renderEntry{}
)

type renderEntry struct {
	once sync.Once
	o    *observed
}

// renderObserved is the whole render of a text. site is the site config as
// JSON, or nothing.
func renderObserved(t testing.TB, text string, site json.RawMessage) *observed {
	config := siteConfig(t, site)
	key := string(site) + "\x00" + text
	rendersMu.Lock()
	e := renders[key]
	if e == nil {
		e = &renderEntry{}
		renders[key] = e
	}
	rendersMu.Unlock()
	e.once.Do(func() { e.o = renderWith(text, config, false) })
	return e.o
}

func siteConfig(t testing.TB, raw json.RawMessage) *js.Obj[any] {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return js.AsObj(v)
}
