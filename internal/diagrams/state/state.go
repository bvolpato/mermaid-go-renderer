// Package state is the state diagram: the diagram type stateDiagram, which
// the keywords stateDiagram and stateDiagram-v2 both select.
//
// Ported from mermaid 12.2.0, diagrams/state/stateDiagram-v2.ts,
// stateDetector-V2.ts and stateRenderer-v3-unified.ts (MIT). The other files
// of the package port stateDb.ts, dataFetcher.ts, stateCommon.ts and the
// semantic actions of parser/stateDiagram.jison.
//
// Mermaid 12.2.0 registers one detector for state diagrams
// (stateDetector-V2.ts in diagram-orchestration.ts). Its id is stateDiagram
// and it loads stateDiagram-v2.ts: the db with version 2 and the unified
// renderer. The files stateDiagram.ts, stateRenderer.js and shapes.js (the
// old renderer with its own dagre layout) are not reachable in this
// version: no detector loads them, only test files import stateDiagram.ts,
// and the ESM build has no chunk for them. So they have no port, and there
// is no diagram type with the id state.
//
// The diagram is drawn by the unified renderer (internal/ru). The shapes,
// the edges, the clusters and the layouts register themselves there: a
// program must link those packages too.
package state

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// typeID is the id of stateDetector-V2.ts.
const typeID = "stateDiagram"

func init() {
	render.Register(NewDefinition(typeID))
}

// NewDefinition is the diagram object of stateDiagram-v2.ts for a diagram
// type id.
func NewDefinition(id string) *render.Definition {
	return &render.Definition{
		ID: id,
		// get db() { return new StateDB(2); }
		New:  func(ctx *render.Context) render.DB { return NewDB(ctx, 2) },
		Init: initDiagram,
		Parse: func(_ *render.Context, db render.DB, text string) error {
			return parse(db.(*DB), text)
		},
		Draw: func(ctx *render.Context, db render.DB, _ string) error {
			return draw(ctx, db.(*DB))
		},
		Classes: func(_ *render.Context, db render.DB, _ string) *js.Map[string, *render.ClassDef] {
			return getClasses(db.(*DB))
		},
	}
}

// initDiagram is the init hook of the diagram definition:
//
//	init: (cnf) => {
//	  if (!cnf.state) { cnf.state = {}; }
//	  cnf.state.arrowMarkerAbsolute = cnf.arrowMarkerAbsolute;
//	}
//
// cnf is a copy of the config (getConfig()), and the hook does not call
// setConfig. So the config of the render does not change. The assignment
// throws for a state section that is a primitive value, because the module
// is in strict mode. The config of mermaid keeps the section an object for
// a front matter or a site config such as `state: x`, so the exception does
// not occur in the cases of the golden file.
func initDiagram(ctx *render.Context, db render.DB) {
	cnf := ctx.Config
	if !js.Truthy(cnf.Get("state")) {
		cnf.Set("state", js.NewObj[any]())
	}
	switch state := cnf.Get("state").(type) {
	case *js.Obj[any]:
		state.Set("arrowMarkerAbsolute", cnf.Get("arrowMarkerAbsolute"))
	case []any:
		// An array takes the property, and nothing reads it.
	default:
		if d, ok := db.(*DB); ok {
			d.initErr = typeError("Cannot create property 'arrowMarkerAbsolute' on " + js.TypeOf(state) + " '" + js.ToString(state) + "'")
		}
	}
}

// getClasses is getClasses(text, diagramObj) of the renderer.
func getClasses(d *DB) *js.Map[string, *render.ClassDef] {
	return d.GetClasses()
}

// orNumber is `v || d` for a config value that is a number.
func orNumber(v any, d float64) float64 {
	if !js.Truthy(v) {
		return d
	}
	return js.ToNumber(v)
}

// layoutName is the argument of getRegisteredLayoutAlgorithm for the
// `layout` of the config. Its default parameter is the empty string.
func layoutName(layout any) string {
	if layout == nil {
		return ""
	}
	return js.ToString(layout)
}

// layoutData is the part of draw that builds the layout data: extract,
// getData() and the properties that the renderer adds.
func layoutData(ctx *render.Context, d *DB) (*ru.LayoutData, error) {
	conf := ctx.Cfg("state")
	layout := ctx.Cfg("layout")
	// Extracting the data from the parsed structure into a more usable form
	// Not related to the refactoring, but this is the first step in the rendering process
	root, err := d.getRootDocV2()
	if err != nil {
		return nil, err
	}
	if err := d.Extract(root); err != nil {
		return nil, err
	}

	// The getData method provided in all supported diagrams is used to extract the data from the parsed structure
	// into the Layout data format
	data4Layout, err := d.GetData()
	if err != nil {
		return nil, err
	}

	data4Layout.Type = ctx.Type
	// Resolve rather than assign: an unregistered layout would otherwise
	// reach render() and throw, where every other unified renderer falls
	// back to dagre.
	data4Layout.LayoutAlgorithm = ru.GetRegisteredLayoutAlgorithm(layoutName(layout), "")

	data4Layout.NodeSpacing = orNumber(js.Path(conf, "nodeSpacing"), 50)
	data4Layout.RankSpacing = orNumber(js.Path(conf, "rankSpacing"), 50)
	config := ctx.Config
	if js.StrictEquals(config.Get("look"), "neo") {
		data4Layout.Markers = []string{"barbNeo"}
	} else {
		data4Layout.Markers = []string{"barb"}
	}
	data4Layout.DiagramID = ctx.ID
	return data4Layout, nil
}

// trimQuotes is s.replace(/^"+|"+$/g, ”): it removes leading and trailing
// quotes.
func trimQuotes(s string) string {
	return strings.TrimRight(strings.TrimLeft(s, `"`), `"`)
}

// injectLinks is the part of draw that injects clickable links after the
// nodes are rendered. Upstream runs it in a try block and logs an
// exception: the links that follow are then not injected.
func injectLinks(svg *dom.Node, d *DB, data4Layout *ru.LayoutData) {
	stop := false
	d.GetLinks().Each(func(key any, linkInfo *Link) {
		if stop {
			return
		}
		// typeof key === 'string' ? key : typeof key?.id === 'string' ? key.id : ''
		stateID, isString := key.(string)
		if !isString {
			stateID, _ = strProp(key, "id")
		}
		var stateNode *ru.Node
		for _, node := range data4Layout.Nodes {
			if node.ID == stateID {
				stateNode = node
				break
			}
		}

		if stateID == "" {
			return
		}

		allNodes := svg.SelectAll("g.node, g.rough-node")
		var matchedElem *dom.Node

		for _, g := range allNodes {
			text := js.TrimSpace(g.TextContent())
			if (stateNode != nil && g.ID() == stateNode.DomID) || text == stateID {
				matchedElem = g
			}
		}

		if matchedElem == nil {
			return
		}

		parentNode := matchedElem.Parent
		if parentNode == nil {
			return
		}

		a := svg.Document().CreateElementNS(dom.NSSVG, "a")
		url, ok := linkInfo.URL.(string)
		if !ok {
			// linkInfo.url.replace throws.
			stop = true
			return
		}
		cleanedURL := trimQuotes(url) // remove leading/trailing quotes
		a.SetAttributeRaw(dom.NSXLink, "xlink", "href", cleanedURL)
		a.Attr("target", "_blank")
		if js.Truthy(linkInfo.Tooltip) {
			tip, ok := linkInfo.Tooltip.(string)
			if !ok {
				stop = true
				return
			}
			tooltip := trimQuotes(tip)
			a.Attr("title", tooltip)
			matchedElem.Attr("title", tooltip)
		}

		// parent.replaceChild(a, matchedElem); a.appendChild(matchedElem);
		parentNode.InsertBefore(a, matchedElem)
		a.AppendChild(matchedElem)
	})
}

// draw is draw(text, id, _version, diag) of stateRenderer-v3-unified.ts.
func draw(ctx *render.Context, d *DB) error {
	conf := ctx.Cfg("state")
	data4Layout, err := layoutData(ctx, d)
	if err != nil {
		return err
	}

	// Create the root SVG - the element is the div containing the SVG element
	svg := ctx.GetDiagramElement()

	if err := ru.Render(ctx, data4Layout, svg); err != nil {
		return err
	}
	padding := 8.0

	// Inject clickable links after nodes are rendered
	injectLinks(svg, d, data4Layout)

	// conf?.titleTopMargin ?? 25
	titleTopMargin := 25.0
	if v := js.Path(conf, "titleTopMargin"); !js.IsNullish(v) {
		titleTopMargin = js.ToNumber(v)
	}
	render.InsertTitle(svg, "statediagramTitleText", titleTopMargin, d.GetDiagramTitle())
	// conf?.useMaxWidth ?? true
	useMaxWidth := true
	if v := js.Path(conf, "useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}
	render.SetupViewPortForSVG(svg, padding, cssDiagram, useMaxWidth)
	return nil
}
