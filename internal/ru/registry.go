// Ported from mermaid 12.2.0, rendering-elements/shapes.ts (generateShapeMap,
// isValidShape), rendering-elements/clusters.js (the shapes map) and render.ts
// (the layout algorithm registry) (MIT).

package ru

import (
	"fmt"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

var (
	regMu         sync.RWMutex
	shapeHandlers = map[string]ShapeHandler{}
	// shapeIDs maps every valid shape id (short names, aliases, internal
	// aliases, undocumented shapes) to the name of its handler.
	shapeIDs      map[string]string
	clusterShapes = map[string]ClusterHandler{}
	layouts       = map[string]LayoutAlgorithm{}
)

func buildShapeIDs() {
	shapeIDs = map[string]string{}
	for _, name := range undocumentedShapes {
		shapeIDs[name] = name
	}
	for _, d := range shapeDefs {
		shapeIDs[d.ShortName] = d.Handler
		for _, a := range d.Aliases {
			shapeIDs[a] = d.Handler
		}
		for _, a := range d.InternalAliases {
			shapeIDs[a] = d.Handler
		}
	}
}

// RegisterShape adds the handler of a shape. The name is the identifier of
// the handler function in shapes.ts, for example "squareRect". Shape
// packages call it from init.
func RegisterShape(handlerName string, h ShapeHandler) {
	regMu.Lock()
	defer regMu.Unlock()
	shapeHandlers[handlerName] = h
}

// IsValidShape is isValidShape(shape): whether mermaid knows the shape id.
func IsValidShape(shape string) bool {
	regMu.RLock()
	defer regMu.RUnlock()
	if shapeIDs == nil {
		regMu.RUnlock()
		regMu.Lock()
		if shapeIDs == nil {
			buildShapeIDs()
		}
		regMu.Unlock()
		regMu.RLock()
	}
	_, ok := shapeIDs[shape]
	return ok
}

// ShapeDefs returns the documented shape definitions.
func ShapeDefs() []ShapeDef { return shapeDefs }

// NotPortedError reports a shape, cluster shape or layout that mermaid has
// and that this port does not have yet.
type NotPortedError struct {
	Kind string // "shape", "cluster shape" or "layout"
	Name string
}

func (e *NotPortedError) Error() string {
	return fmt.Sprintf("%s %q is not ported yet", e.Kind, e.Name)
}

// LookupShape returns the handler for a shape id. The error says whether
// the shape is unknown to mermaid or not ported yet.
func LookupShape(shape string) (ShapeHandler, error) {
	if !IsValidShape(shape) {
		return nil, fmt.Errorf("No such shape: %s. Please check your syntax.", shape)
	}
	regMu.RLock()
	defer regMu.RUnlock()
	h, ok := shapeHandlers[shapeIDs[shape]]
	if !ok {
		return nil, &NotPortedError{Kind: "shape", Name: shape}
	}
	return h, nil
}

// ClusterHandler draws one cluster (a group node) into elem. It returns
// the cluster group and the bounding box of its label.
type ClusterHandler func(ctx *render.Context, elem *dom.Node, node *Node) (*ClusterResult, error)

// ClusterResult is the return value of a cluster shape function.
type ClusterResult struct {
	Cluster   *dom.Node
	LabelBBox dom.Rect
	// HasLabelBBox is false for the shapes that return an empty object.
	HasLabelBBox bool
}

// RegisterClusterShape adds a cluster shape of clusters.js.
func RegisterClusterShape(name string, h ClusterHandler) {
	regMu.Lock()
	defer regMu.Unlock()
	clusterShapes[name] = h
}

// LookupClusterShape returns the handler of a cluster shape.
func LookupClusterShape(name string) (ClusterHandler, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	h, ok := clusterShapes[name]
	if !ok {
		return nil, &NotPortedError{Kind: "cluster shape", Name: name}
	}
	return h, nil
}

// LayoutAlgorithm lays out and draws layout data: the render function of a
// layout module.
type LayoutAlgorithm func(ctx *render.Context, data *LayoutData, svg *dom.Node, algorithm string) error

// layoutEntry is LayoutLoaderDefinition.
type layoutEntry struct {
	algorithm string
}

var layoutAlgorithmNames = map[string]layoutEntry{}

// RegisterLayout adds a layout. name is the value of the `layout` config,
// and algorithm is the option that the layout receives (for example
// "elk.layered" for "elk"), or "".
func RegisterLayout(name, algorithm string, fn LayoutAlgorithm) {
	regMu.Lock()
	defer regMu.Unlock()
	layouts[name] = fn
	layoutAlgorithmNames[name] = layoutEntry{algorithm: algorithm}
}

// knownLayouts are the layouts that mermaid registers in its full build.
// A layout in this list that has no Go implementation yet is an error, and
// not a silent fallback to another layout.
var knownLayouts = map[string]string{
	"dagre": "", "swimlane": "", "cose-bilkent": "", "elk": "elk.layered",
	"elk.stress": "elk.stress", "elk.force": "elk.force", "elk.mrtree": "elk.mrtree",
	"elk.sporeOverlap": "elk.sporeOverlap", "elk.box": "elk.box", "elk.rectpacking": "elk.rectpacking",
}

// lastResortLayout is always registered upstream, so the fallback chain
// can always end.
const lastResortLayout = "dagre"

// GetRegisteredLayoutAlgorithm is getRegisteredLayoutAlgorithm(algorithm,
// {fallback}): the name of the layout to use. An unknown name falls back,
// as upstream does.
func GetRegisteredLayoutAlgorithm(algorithm string, fallback string) string {
	if _, ok := knownLayouts[algorithm]; ok {
		return algorithm
	}
	if fallback == "" {
		fallback = lastResortLayout
	}
	if _, ok := knownLayouts[fallback]; ok {
		return fallback
	}
	return lastResortLayout
}

func lookupLayout(name string) (LayoutAlgorithm, string, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	fn, ok := layouts[name]
	if !ok {
		if _, known := knownLayouts[name]; known {
			return nil, "", &NotPortedError{Kind: "layout", Name: name}
		}
		return nil, "", fmt.Errorf("Unknown layout algorithm: %s", name)
	}
	return fn, layoutAlgorithmNames[name].algorithm, nil
}

// pageLayout is a layout that a page registers by itself.
type pageLayout struct {
	fn        LayoutAlgorithm
	algorithm string
}

// pageLayouts are the layouts that a kind of page adds to the registry of
// mermaid before it renders, with mermaid.registerLayoutLoaders. They are
// apart from layouts, so that a plain page does not have them.
var pageLayouts = map[render.Page]map[string]pageLayout{}

// RegisterPageLayout adds a layout that only one kind of page has: the page
// calls mermaid.registerLayoutLoaders before it renders. The page of
// mermaid-cli does that for the layout "tidy-tree". name and algorithm are
// as in RegisterLayout. Layout packages call it from init.
func RegisterPageLayout(page render.Page, name, algorithm string, fn LayoutAlgorithm) {
	regMu.Lock()
	defer regMu.Unlock()
	if pageLayouts[page] == nil {
		pageLayouts[page] = map[string]pageLayout{}
	}
	pageLayouts[page][name] = pageLayout{fn: fn, algorithm: algorithm}
}

// lookupPageLayout returns the layout that the page of the render
// registered under the `layout` of the config of the layout data. ok is
// false when the page registered no layout with that name. That is always
// so in a plain page.
//
// Upstream finds such a layout in getRegisteredLayoutAlgorithm. Its port,
// GetRegisteredLayoutAlgorithm, has no render context and so knows only the
// layouts of mermaid itself. Render has the context: it calls
// lookupPageLayout first.
//
// The name is data.config.layout, which is the value that every diagram
// renderer gives to getRegisteredLayoutAlgorithm. The config of the layout
// data is getConfig() for all of them but the mindmap, which sets the
// layout "cose-bilkent" in its copy when the user set no top-level layout,
// and then reads the layout from that copy. So a mindmap whose layout
// comes only from `mindmap.layout` does not get a layout of the page, as
// in mermaid-cli.
func lookupPageLayout(ctx *render.Context, data *LayoutData) (name string, l pageLayout, ok bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	registered := pageLayouts[ctx.Page]
	if len(registered) == 0 {
		return "", pageLayout{}, false
	}
	// The default parameter of getRegisteredLayoutAlgorithm is the empty
	// string, and Object.hasOwn converts another value to a string.
	if layout := js.Path(data.Config, "layout"); layout != nil {
		name = js.ToString(layout)
	}
	l, ok = registered[name]
	return name, l, ok
}
