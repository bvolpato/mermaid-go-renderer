package agentflow

import (
	"fmt"
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The api cases of the golden file call the methods of a new AgentFlowDB of
// upstream directly, for the branches that the grammar does not reach. The
// test calls the same methods of this port with the same arguments, and
// compares the results of the calls, the error and the state of the db.

// isUndefined says whether an argument of an op is undefined: { $u: 1 },
// or an argument that the op leaves out.
func isUndefined(v any) bool {
	if v == nil {
		return true
	}
	o, ok := v.(*js.Obj[any])
	return ok && o != nil && o.Has("$u")
}

// opArgs are the arguments of one op.
type opArgs []any

func (a opArgs) at(i int) any {
	if i >= len(a) || isUndefined(a[i]) {
		return nil
	}
	return a[i]
}

func (a opArgs) str(i int) string {
	s, _ := a.at(i).(string)
	return s
}

func (a opArgs) num(i int) int {
	n, _ := a.at(i).(float64)
	return int(n)
}

func (a opArgs) text(i int) *FlowText {
	return textOf(a.at(i))
}

func (a opArgs) strings(i int) []string {
	l, ok := a.at(i).([]any)
	if !ok {
		return nil
	}
	return stringList(l)
}

func (a opArgs) loc(i int) *Location {
	o := js.AsObj(a.at(i))
	if o == nil {
		return nil
	}
	n := func(key string) int {
		f, _ := o.Get(key).(float64)
		return int(f)
	}
	return &Location{FirstLine: n("first_line"), FirstColumn: n("first_column"), LastLine: n("last_line"), LastColumn: n("last_column")}
}

func stringList(l []any) []string {
	out := make([]string, len(l))
	for i, e := range l {
		out[i], _ = e.(string)
	}
	return out
}

// textOf converts { text, type } to a FlowText. The result is nil for a
// value that is undefined.
func textOf(v any) *FlowText {
	o := js.AsObj(v)
	if o == nil {
		return nil
	}
	return &FlowText{Text: str(o, "text"), Type: str(o, "type")}
}

// documentOf converts the list argument of addSubGraph and addGlobal: an
// item is a string, a list of strings, a direction statement or a number.
func documentOf(v any) []any {
	l, _ := v.([]any)
	out := make([]any, len(l))
	for i, e := range l {
		switch x := e.(type) {
		case []any:
			out[i] = stringList(x)
		case *js.Obj[any]:
			if x != nil && js.StrictEquals(x.Get("stmt"), "dir") {
				out[i] = &dirStmt{value: str(x, "value")}
			} else {
				out[i] = e
			}
		default:
			out[i] = e
		}
	}
	return out
}

// linkOfJS converts the link argument of addSingleLink and addLink.
func linkOfJS(v any) *FlowLink {
	link := &FlowLink{Length: undef}
	o := js.AsObj(v)
	if o == nil {
		// A value that is not an object: every property is undefined, but
		// a string has a length.
		if s, ok := v.(string); ok {
			link.Length = float64(js.UTF16Len(s))
		}
		return link
	}
	link.Type = str(o, "type")
	link.Stroke = str(o, "stroke")
	if n, ok := o.Get("length").(float64); ok {
		link.Length = n
	}
	link.EdgeSemantic = str(o, "edgeSemantic")
	link.Text = textOf(o.Get("text"))
	// isLinkData: the id is a string.
	link.ID, link.HasID = o.Get("id").(string)
	return link
}

// subGraphsOf converts a list of { nodes } objects.
func subGraphsOf(v any) []*FlowSubGraph {
	l, _ := v.([]any)
	out := make([]*FlowSubGraph, len(l))
	for i, e := range l {
		out[i] = subGraphOf(e)
	}
	return out
}

func subGraphOf(v any) *FlowSubGraph {
	return &FlowSubGraph{Nodes: stringList(list(js.AsObj(v), "nodes"))}
}

// positionsOf converts the positions of updateLink: a number or a text.
func positionsOf(v any) []any {
	l, _ := v.([]any)
	return l
}

// undefinedResult is the record of an op that returns undefined.
func undefinedResult() any { return obj("$u", 1.0) }

func optString(s string, ok bool) any {
	if !ok {
		return undefinedResult()
	}
	return s
}

// applyOp calls one method of the db. The result is the value that the
// oracle records for the call.
func applyOp(d *DB, method string, a opArgs) (any, error) {
	void := undefinedResult()
	switch method {
	case "setGen":
		d.SetGen(a.str(0))
		return void, nil
	case "clear":
		ver := "gen-2"
		if !isUndefined(a.at(0)) {
			ver = a.str(0)
		}
		d.Clear(ver)
		return void, nil
	case "setDiagramId":
		d.SetDiagramId(a.str(0))
		return void, nil
	case "lookUpDomId":
		return d.LookUpDomId(a.str(0)), nil
	case "getTooltip":
		return optString(d.GetTooltip(a.str(0))), nil
	case "setTooltip":
		return void, d.SetTooltip(a.str(0), a.str(1))
	case "addVertex":
		args := VertexArgs{
			TextObj:     a.text(1),
			Type:        a.str(2),
			Style:       a.strings(3),
			Classes:     a.strings(4),
			Dir:         a.str(5),
			Props:       js.AsObj(a.at(6)),
			MetadataLoc: a.loc(8),
		}
		if m, ok := a.at(7).(string); ok {
			args.Metadata = &m
		}
		return void, d.AddVertex(a.str(0), args)
	case "addSingleLink":
		return void, d.AddSingleLink(a.str(0), a.str(1), linkOfJS(a.at(2)), a.str(3))
	case "addLink":
		return void, d.AddLink(a.strings(0), a.strings(1), linkOfJS(a.at(2)))
	case "updateLink":
		return void, d.UpdateLink(positionsOf(a.at(0)), a.strings(1))
	case "updateLinkInterpolate":
		return void, d.UpdateLinkInterpolate(positionsOf(a.at(0)), a.str(1))
	case "addClass":
		d.AddClass(a.str(0), a.strings(1))
		return void, nil
	case "setClass":
		d.SetClass(a.str(0), a.str(1))
		return void, nil
	case "setDirection":
		d.SetDirection(a.str(0))
		return void, nil
	case "getDirection":
		return optString(d.GetDirection()), nil
	case "firstGraph":
		return d.FirstGraph(), nil
	case "defaultStyle":
		return d.DefaultStyle(), nil
	case "defaultConfig":
		return jsonValue(d.DefaultConfig()), nil
	case "setAccTitle":
		d.SetAccTitle(a.str(0))
		return void, nil
	case "setAccDescription":
		d.SetAccDescription(a.str(0))
		return void, nil
	case "setDiagramTitle":
		d.SetDiagramTitle(a.str(0))
		return void, nil
	case "setLink":
		var target *string
		if t, ok := a.at(2).(string); ok {
			target = &t
		}
		d.SetLink(a.str(0), a.str(1), target)
		return void, nil
	case "setClickEvent":
		var args *string
		if s, ok := a.at(2).(string); ok {
			args = &s
		}
		d.SetClickEvent(a.str(0), a.str(1), args)
		return void, nil
	case "addSubGraph":
		id, err := d.AddSubGraph(a.text(0), documentOf(a.at(1)), a.text(2), a.str(3))
		return id, err
	case "addSubGraphSame":
		// The id and the title are the same object.
		text := a.text(0)
		id, err := d.AddSubGraph(text, documentOf(a.at(1)), text, a.str(2))
		return id, err
	case "addGlobal":
		d.AddGlobal(documentOf(a.at(0)))
		return void, nil
	case "exists":
		return d.Exists(subGraphsOf(a.at(0)), a.str(1)), nil
	case "makeUniq":
		return obj("nodes", stringsJS(d.MakeUniq(subGraphOf(a.at(0)), subGraphsOf(a.at(1))))), nil
	case "getDepthFirstPos":
		pos, ok := d.GetDepthFirstPos(a.num(0))
		if !ok {
			return void, nil
		}
		return float64(pos), nil
	case "indexNodes":
		d.IndexNodes()
		return void, nil
	case "addConnector":
		id, err := d.AddConnector(a.text(0), a.text(1))
		return id, err
	case "isToolDefinition":
		v, ok := d.GetVertices().Lookup(a.str(0))
		return ok && d.IsToolDefinition(v), nil
	case "addVertexMapping":
		_, hasShape := a.at(2).(string)
		d.AddVertexMapping(a.str(0), hasShape, a.loc(3))
		return void, nil
	case "extendVertexMapping":
		d.ExtendVertexMapping(a.str(0), a.loc(1))
		return void, nil
	case "addEdgeMapping":
		// toNodes.map((n) => (typeof n === 'string' ? n : (n?.id ?? ''))).filter(Boolean)
		var ids []string
		if l, ok := a.at(1).([]any); ok {
			for _, n := range l {
				if s, isString := n.(string); isString {
					ids = append(ids, s)
				} else if o := js.AsObj(n); o != nil {
					ids = append(ids, str(o, "id"))
				}
			}
		}
		d.AddEdgeMapping(ids, a.loc(3))
		return void, nil
	case "addSubgraphMapping":
		// The id is a string or an object with a text.
		id := a.text(0)
		if s, ok := a.at(0).(string); ok {
			id = &FlowText{Text: s}
		}
		d.AddSubgraphMapping(id, a.loc(2), a.loc(3))
		return void, nil
	case "addConnectorMapping":
		d.AddConnectorMapping(a.text(0), a.loc(2), a.loc(3))
		return void, nil
	case "setFrontmatterLineOffset":
		d.SetFrontmatterLineOffset(a.num(0))
		return void, nil
	case "emitWarning", "emitError":
		var ctx DiagnosticContext
		if o := js.AsObj(a.at(2)); o != nil {
			ctx.NodeID, ctx.HasNodeID = o.Get("nodeId").(string)
			ctx.EdgeID = str(o, "edgeId")
		}
		if method == "emitWarning" {
			d.EmitWarning(a.str(0), a.str(1), ctx)
		} else {
			d.EmitError(a.str(0), a.str(1), ctx)
		}
		return void, nil
	case "getDiagnostics":
		return diagnosticsJSON(d.GetDiagnostics()), nil
	case "getElementAtPosition":
		return tag(d.GetElementAtPosition(a.num(0), a.num(1))), nil
	case "getElementById":
		return mappingJSON(d.GetElementById(a.str(0))), nil
	case "getElementsOnLine":
		return tags(d.GetElementsOnLine(a.num(0))), nil
	case "getMappingStats":
		s := d.GetMappingStats()
		return obj(
			"vertices", float64(s.Vertices),
			"edges", float64(s.Edges),
			"subgraphs", float64(s.Subgraphs),
			"connectors", float64(s.Connectors),
			"attachments", float64(s.Attachments),
			"totalElements", float64(s.TotalElements),
		), nil
	case "getData":
		data := d.GetData()
		nodes := make([]any, len(data.Nodes))
		for i, n := range data.Nodes {
			nodes[i] = nodeJSON(n)
		}
		edges := make([]any, len(data.Edges))
		for i, e := range data.Edges {
			edges[i] = edgeJSON(e)
		}
		return obj("nodes", nodes, "edges", edges, "other", js.NewObj[any](), "connectors", verticesJSON(data.Connectors)), nil
	case "assignColorSlots":
		// The nodes, the kind of each node id, and the order of the containers.
		l, _ := a.at(0).([]any)
		nodes := make([]*ru.Node, len(l))
		for i, e := range l {
			o := js.AsObj(e)
			nodes[i] = ru.NewNode(str(o, "id"))
			nodes[i].Shape = str(o, "shape")
			nodes[i].IsGroup = js.Truthy(o.Get("isGroup"))
			nodes[i].CSSClasses, nodes[i].CSSClassesSet = o.Get("cssClasses").(string)
		}
		kindByID := js.AsObj(a.at(1))
		order := js.NewMap[string, int]()
		pairs, _ := a.at(2).([]any)
		for _, p := range pairs {
			pair := p.([]any)
			n, _ := pair[1].(float64)
			order.Set(pair[0].(string), int(n))
		}
		assignColorSlots(d.getConfig(), nodes, func(id string) string { return str(kindByID, id) }, order)
		out := make([]any, len(nodes))
		for i, n := range nodes {
			out[i] = obj(
				"id", n.ID,
				"isGroup", n.IsGroup,
				"shape", optStr(n.Shape),
				"cssClasses", optFlag(n.CSSClassesSet, n.CSSClasses),
				"colorIndex", optNum(n.ColorIndex),
			)
		}
		return out, nil
	case "transformData":
		// The nodes of the op have an id, a shape and isGroup.
		l, _ := a.at(0).([]any)
		nodes := make([]*ru.Node, len(l))
		for i, e := range l {
			o := js.AsObj(e)
			nodes[i] = ru.NewNode(str(o, "id"))
			nodes[i].Shape = str(o, "shape")
			nodes[i].IsGroup = js.Truthy(o.Get("isGroup"))
		}
		data := &ru.LayoutData{Nodes: nodes}
		if js.Truthy(a.at(1)) {
			TransformData(data, d)
		} else {
			TransformData(data, nil)
		}
		out := make([]any, len(nodes))
		for i, n := range nodes {
			out[i] = obj("id", n.ID, "shape", n.Shape, "isGroup", optFlag(n.IsGroup, true))
		}
		return out, nil
	}
	return nil, fmt.Errorf("the test has no op %q", method)
}

// runAPI runs one api case and returns the first difference, or "".
func runAPI(base map[string]any, c *goldenAPI) (problem string) {
	defer func() {
		if p := recover(); p != nil {
			problem = fmt.Sprintf("panic: %v", p)
		}
	}()
	// mermaid.initialize({ startOnLoad: false, ...site }) in a new page.
	state, err := config.NewState()
	if err != nil {
		return err.Error()
	}
	if err := state.Initialize(siteConfig(c.Site)); err != nil {
		return err.Error()
	}
	cfg := state.GetConfig()
	if s := diffConfig(cfg, wantConfig(base, js.AsObj(c.rec.Get("configDelta")))); s != "" {
		return s
	}
	d := NewDB(render.NewContext(render.ContextOptions{Config: cfg, ConfigState: state}))

	returns := []any{}
	var opErr error
	for i, op := range c.Ops {
		l, _ := op.([]any)
		method, _ := l[0].(string)
		result, err := applyOp(d, method, opArgs(l[1:]))
		if err != nil {
			opErr = fmt.Errorf("op %d (%s): %w", i, method, err)
			break
		}
		returns = append(returns, result)
	}

	wantError, hasError := c.rec.Get("error").(string)
	switch {
	case hasError && opErr == nil:
		return fmt.Sprintf("got no error, want %q", wantError)
	case !hasError && opErr != nil:
		return fmt.Sprintf("got the error %q, want none", opErr)
	case hasError:
		name, message, hash, mark := errorInfo(opErr)
		if message != wantError {
			return fmt.Sprintf("error message:\n got %q\nwant %q", message, wantError)
		}
		if want := str(c.rec, "errorName"); name != want {
			return fmt.Sprintf("error name: got %s, want %s", name, want)
		}
		if s := diff("error.hash", hash, c.rec.Get("hash"), false); s != "" {
			return s
		}
		if s := diff("error.mark", mark, c.rec.Get("mark"), false); s != "" {
			return s
		}
	}
	if s := diff("returns", returns, c.rec.Get("returns"), false); s != "" {
		return s
	}
	got := dbState(d)
	want := js.AsObj(c.rec.Get("state"))
	for _, key := range got.Keys() {
		if s := diff("state."+key, got.Get(key), want.Get(key), false); s != "" {
			return s
		}
	}
	for _, key := range want.Keys() {
		if !got.Has(key) {
			return fmt.Sprintf("state.%s: got undefined, want %s", key, show(want.Get(key)))
		}
	}
	return ""
}

// TestAPI compares the methods of the db with upstream for the api cases
// of the golden file.
func TestAPI(t *testing.T) {
	g := loadGolden(t)
	base := map[string]any{}
	flatten(g.BaseConfig, nil, base)
	ok, ops := 0, 0
	for _, c := range g.API {
		ops += len(c.Ops)
		if problem := runAPI(base, c); problem != "" {
			t.Errorf("%s: %s", c.Name, problem)
			continue
		}
		ok++
	}
	t.Logf("api cases: %d of %d agree (%d calls)", ok, len(g.API), ops)
	if len(g.API) == 0 {
		t.Error("no api cases")
	}
}

// TestDynamicValues checks the conversion of the config values that a
// layout node cannot hold in its field.
func TestDynamicValues(t *testing.T) {
	n := ru.NewNode("n")
	if got := dynString(n, "look", nil); got != "" || n.Extra != nil {
		t.Errorf("undefined: got %q, %v", got, n.Extra)
	}
	if got := dynString(n, "look", "neo"); got != "neo" || n.Extra != nil {
		t.Errorf("string: got %q, %v", got, n.Extra)
	}
	if got := dynString(n, "look", ""); got != "" || n.Extra["look"] != "" {
		t.Errorf("empty string: got %q, %v", got, n.Extra)
	}
	if got := dynNumber(n, "padding", "20"); got != 20 || n.Extra["padding"] != "20" {
		t.Errorf("number text: got %v, %v", got, n.Extra)
	}
	if got := dynNumber(n, "minWidth", nil); !math.IsNaN(got) || n.Get("minWidth") != nil {
		t.Errorf("undefined number: got %v, %v", got, n.Extra)
	}
}
