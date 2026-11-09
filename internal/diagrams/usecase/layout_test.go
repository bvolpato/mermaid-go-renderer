package usecase

import (
	"errors"
	"sync"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

// The tests of this package put a hook in front of each layout, as
// tools/reference/usecase-db-page.js does in the browser: the layout keeps
// the layout data that it receives, and a test can stop the render there.
// So the draw function of the renderer runs up to the layout. A render that
// is not stopped goes on with the real layout.

// capture is what a test keeps of one render.
type capture struct {
	// warm says that the page has made a render before, so the DOMPurify
	// hooks of mermaid are installed.
	warm bool
	// stop makes the layout fail with errCaptured, as the layout of the
	// oracle does.
	stop bool

	ctx *render.Context
	db  *DB
	// data is the layout data that the layout received.
	data *ru.LayoutData
}

var errCaptured = errors.New("layout data captured")

// captures has the capture of each render that wants one. The key is the
// time zone of the render: a pointer that the test makes for the render and
// that the pipeline hands to the context.
var captures sync.Map

// newCapture registers a capture and returns the time zone that selects it.
func newCapture(c *capture) *time.Location {
	loc := time.FixedZone("UTC", 0)
	captures.Store(loc, c)
	return loc
}

func captureOf(ctx *render.Context) *capture {
	if c, ok := captures.Load(ctx.Location); ok {
		return c.(*capture)
	}
	return nil
}

// hooked returns a layout that keeps the layout data and then runs the
// real layout. A nil layout stands for a layout that this test binary does
// not link: it can only be captured.
func hooked(layout ru.LayoutAlgorithm) ru.LayoutAlgorithm {
	return func(ctx *render.Context, data *ru.LayoutData, svg *dom.Node, algorithm string) error {
		if c := captureOf(ctx); c != nil {
			c.data = data
			if c.stop {
				return errCaptured
			}
		}
		if layout == nil {
			return errors.New("the layout is not linked into the test")
		}
		return layout(ctx, data, svg, algorithm)
	}
}

func init() {
	// The layout packages have registered their layouts: this package
	// imports them, so their init functions ran before this one.
	ru.RegisterLayout("dagre", "", hooked(dagre.Render))
	ru.RegisterLayout("elk", "elk.layered", hooked(elk.Render))
	for _, algorithm := range elk.ELK_ALGORITHMS {
		ru.RegisterLayout(algorithm, algorithm, hooked(elk.Render))
	}
	ru.RegisterLayout("swimlane", "", hooked(nil))
	ru.RegisterLayout("cose-bilkent", "", hooked(nil))

	// The definition of the diagram, with hooks for the captures.
	def, _ := render.Lookup("usecase")
	newDB, drawDiagram := def.New, def.Draw
	def.New = func(ctx *render.Context) render.DB {
		if c := captureOf(ctx); c != nil && c.warm {
			ctx.Sanitizer.RemoveScript("")
		}
		return newDB(ctx)
	}
	def.Draw = func(ctx *render.Context, db render.DB, text string) error {
		if c := captureOf(ctx); c != nil {
			c.ctx, c.db = ctx, db.(*DB)
		}
		return drawDiagram(ctx, db, text)
	}
}
