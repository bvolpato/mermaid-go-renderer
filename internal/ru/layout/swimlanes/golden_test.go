package swimlanes

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file testdata/layout.jsonl.gz is written by
// tools/reference/gen-swimlanes.mjs: it has what the real swimlane layout
// of mermaid does in Chromium, for the fixtures of the corpus, for extra
// diagram texts and for random layout data. Each line has the layout data
// before and after the prepare phase, the input of the layout core, the
// result of every Sugiyama phase, and the state of the layout after every
// later step. The tests run the Go port on the same input and compare
// every number bit for bit, step by step.

type goldenNode struct {
	ID          string  `json:"id"`
	ParentID    *string `json:"parentId"`
	IsGroup     bool    `json:"isGroup"`
	Shape       *string `json:"shape"`
	Label       *string `json:"label"`
	LabelStyle  *string `json:"labelStyle"`
	Look        *string `json:"look"`
	ColorIndex  any     `json:"colorIndex"`
	Padding     any     `json:"padding"`
	Dir         *string `json:"dir"`
	Direction   *string `json:"direction"`
	IsEdgeLabel bool    `json:"isEdgeLabel"`
	IsDummy     bool    `json:"isDummy"`
	EdgeStart   *string `json:"edgeStart"`
	EdgeEnd     *string `json:"edgeEnd"`
	Width       any     `json:"width"`
	Height      any     `json:"height"`
	X           any     `json:"x"`
	Y           any     `json:"y"`
}

type goldenEdge struct {
	ID           string  `json:"id"`
	Start        *string `json:"start"`
	End          *string `json:"end"`
	Label        *string `json:"label"`
	Text         *string `json:"text"`
	Type         *string `json:"type"`
	LabelStyle   any     `json:"labelStyle"`
	LabelNodeID  *string `json:"labelNodeId"`
	IsLayoutOnly bool    `json:"isLayoutOnly"`
	Curve        *string `json:"curve"`
	Points       []any   `json:"points"`
}

type goldenData struct {
	Direction *string `json:"direction"`
	Config    struct {
		Look      any `json:"look"`
		Flowchart struct {
			NodeSpacing any `json:"nodeSpacing"`
			RankSpacing any `json:"rankSpacing"`
		} `json:"flowchart"`
		Swimlane map[string]any `json:"swimlane"`
	} `json:"config"`
	Nodes []goldenNode `json:"nodes"`
	Edges []goldenEdge `json:"edges"`
}

type goldenGraph struct {
	Nodes []string    `json:"nodes"`
	Edges [][3]string `json:"edges"`
}

type goldenSugiyama struct {
	View       goldenGraph `json:"view"`
	Normalized goldenGraph `json:"normalized"`
	LaneOrder  []string    `json:"laneOrder"`
	Acyclic    [][3]string `json:"acyclic"`
	Reversed   [][3]string `json:"reversed"`
	Initial    struct {
		Layers [][]string     `json:"layers"`
		RankOf map[string]int `json:"rankOf"`
	} `json:"initial"`
	Proper struct {
		Layers [][]string     `json:"layers"`
		RankOf map[string]int `json:"rankOf"`
		Dummy  []string       `json:"dummy"`
	} `json:"proper"`
	WithDummies goldenGraph `json:"withDummies"`
	Ordered     [][]string  `json:"ordered"`
	Coords      struct {
		X map[string]any `json:"x"`
		Y map[string]any `json:"y"`
	} `json:"coords"`
}

// goldenLayering is the result of a direct call of a layering function, or
// the message of the error that it throws.
type goldenLayering struct {
	Layers [][]string     `json:"layers"`
	RankOf map[string]int `json:"rankOf"`
	Error  *string        `json:"error"`
}

// goldenDirect has the results of the direct calls of the layering
// functions with options that the pipeline never passes. See directCalls
// in tools/reference/probes/swimlanes.js.
type goldenDirect struct {
	LongestPath []goldenLayering `json:"longestPath"`
	Gravity     goldenLayering   `json:"gravity"`
	Compact     goldenLayering   `json:"compact"`
	Loose       map[string]int   `json:"loose"`
	Optimize    json.RawMessage  `json:"optimize"`
	Adjust      json.RawMessage  `json:"adjust"`
}

type goldenRecord struct {
	Name     string            `json:"name"`
	Kind     string            `json:"kind"`
	Raw      *goldenData       `json:"raw"`
	Prepared *goldenData       `json:"prepared"`
	Input    *goldenData       `json:"input"`
	Sugiyama *goldenSugiyama   `json:"sugiyama"`
	Direct   *goldenDirect     `json:"direct"`
	Steps    []json.RawMessage `json:"steps"`
	Issues   [][4]string       `json:"issues"`
	Error    *string           `json:"error"`
}

func loadGolden(t testing.TB) []*goldenRecord {
	t.Helper()
	f, err := os.Open("testdata/layout.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var records []*goldenRecord
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		record := &goldenRecord{}
		if err := json.Unmarshal(line, record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("the golden file has no records")
	}
	return records
}

// goldenNum decodes a number of the golden file: null is undefined (NaN),
// and { $num: "..." } is a number that JSON cannot hold.
func goldenNum(v any) float64 {
	switch x := v.(type) {
	case nil:
		return ru.Undef
	case float64:
		return x
	case map[string]any:
		switch x["$num"] {
		case "-0":
			return math.Copysign(0, -1)
		case "Infinity":
			return math.Inf(1)
		case "-Infinity":
			return math.Inf(-1)
		default:
			return math.NaN()
		}
	}
	panic(fmt.Sprintf("not a number: %v", v))
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// sameNum compares two numbers bit for bit. Two NaN values (undefined) are
// equal.
func sameNum(a, b float64) bool {
	if a != a && b != b {
		return true
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

func numText(v float64) string {
	if v != v {
		return "undefined"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// buildLayoutData makes the layout data of a snapshot.
func buildLayoutData(d *goldenData) *ru.LayoutData {
	data := &ru.LayoutData{Type: "swimlane", LayoutAlgorithm: "swimlane", Direction: str(d.Direction)}
	config := js.NewObj[any]()
	if d.Config.Look != nil {
		config.Set("look", d.Config.Look)
	}
	flowchart := js.NewObj[any]()
	if d.Config.Flowchart.NodeSpacing != nil {
		flowchart.Set("nodeSpacing", goldenNum(d.Config.Flowchart.NodeSpacing))
	}
	if d.Config.Flowchart.RankSpacing != nil {
		flowchart.Set("rankSpacing", goldenNum(d.Config.Flowchart.RankSpacing))
	}
	config.Set("flowchart", flowchart)
	swimlane := js.NewObj[any]()
	for _, key := range []string{"ignoreCrossLaneEdges", "optimizeRanksByCrossings", "automaticLaneOrdering"} {
		if v, ok := d.Config.Swimlane[key]; ok && v != nil {
			swimlane.Set(key, v)
		}
	}
	config.Set("swimlane", swimlane)
	data.Config = config

	for _, gn := range d.Nodes {
		n := ru.NewNode(gn.ID)
		n.ParentID = str(gn.ParentID)
		n.IsGroup = gn.IsGroup
		n.Shape = str(gn.Shape)
		if gn.Label != nil {
			n.Label, n.HasLabel = *gn.Label, true
		}
		n.LabelStyle = str(gn.LabelStyle)
		n.Look = str(gn.Look)
		n.ColorIndex = goldenNum(gn.ColorIndex)
		n.Padding = goldenNum(gn.Padding)
		n.Dir = str(gn.Dir)
		if gn.Direction != nil {
			n.Set("direction", *gn.Direction)
		}
		n.IsEdgeLabel = gn.IsEdgeLabel
		n.IsDummy = gn.IsDummy
		n.EdgeStart = str(gn.EdgeStart)
		n.EdgeEnd = str(gn.EdgeEnd)
		n.Width = goldenNum(gn.Width)
		n.Height = goldenNum(gn.Height)
		n.X = goldenNum(gn.X)
		n.Y = goldenNum(gn.Y)
		data.Nodes = append(data.Nodes, n)
	}
	for _, ge := range d.Edges {
		e := ru.NewEdge(ge.ID)
		e.Start = str(ge.Start)
		e.End = str(ge.End)
		if ge.Label != nil {
			e.Label, e.HasLabel = *ge.Label, true
		}
		e.Text = str(ge.Text)
		e.Type = str(ge.Type)
		switch ls := ge.LabelStyle.(type) {
		case []any:
			for _, s := range ls {
				e.LabelStyle = append(e.LabelStyle, s.(string))
			}
		case string:
			e.LabelStyleText, e.LabelStyleIsText = ls, true
		}
		e.LabelNodeID = str(ge.LabelNodeID)
		e.IsLayoutOnly = ge.IsLayoutOnly
		e.Curve = str(ge.Curve)
		for i := 0; i+1 < len(ge.Points); i += 2 {
			e.Points = append(e.Points, Point{X: goldenNum(ge.Points[i]), Y: goldenNum(ge.Points[i+1])})
		}
		data.Edges = append(data.Edges, e)
	}
	return data
}

// nodeStateOf is the state of a node that the steps change, in the layout
// of the golden file: [x, y, width, height, parentId, layer, order,
// swimlaneContentTop, groupTitleRect].
type nodeStateT struct {
	nums      [8]float64 // x, y, width, height, layer, order, contentTop, unused
	parentID  string
	titleRect *[4]float64
}

func nodeStateOf(n *ru.Node) nodeStateT {
	s := nodeStateT{parentID: n.ParentID}
	s.nums = [8]float64{n.X, n.Y, n.Width, n.Height, n.Layer, n.Order, swimlaneContentTop(n), 0}
	if r := n.GroupTitleRect; r != nil {
		s.titleRect = &[4]float64{r.Left, r.Right, r.Top, r.Bottom}
	}
	return s
}

func decodeNodeState(v []any) nodeStateT {
	s := nodeStateT{}
	s.nums = [8]float64{goldenNum(v[0]), goldenNum(v[1]), goldenNum(v[2]), goldenNum(v[3]), goldenNum(v[5]), goldenNum(v[6]), goldenNum(v[7]), 0}
	if p, ok := v[4].(string); ok {
		s.parentID = p
	}
	if r, ok := v[8].([]any); ok {
		s.titleRect = &[4]float64{goldenNum(r[0]), goldenNum(r[1]), goldenNum(r[2]), goldenNum(r[3])}
	}
	return s
}

var nodeStateNames = [...]string{"x", "y", "width", "height", "layer", "order", "swimlaneContentTop"}

func diffNodeState(got, want nodeStateT) string {
	for i, name := range nodeStateNames {
		if !sameNum(got.nums[i], want.nums[i]) {
			return fmt.Sprintf("%s = %s, want %s", name, numText(got.nums[i]), numText(want.nums[i]))
		}
	}
	if got.parentID != want.parentID {
		return fmt.Sprintf("parentId = %q, want %q", got.parentID, want.parentID)
	}
	if (got.titleRect == nil) != (want.titleRect == nil) {
		return fmt.Sprintf("groupTitleRect defined = %v, want %v", got.titleRect != nil, want.titleRect != nil)
	}
	if got.titleRect != nil {
		for i, name := range [...]string{"left", "right", "top", "bottom"} {
			if !sameNum(got.titleRect[i], want.titleRect[i]) {
				return fmt.Sprintf("groupTitleRect.%s = %s, want %s", name, numText(got.titleRect[i]), numText(want.titleRect[i]))
			}
		}
	}
	return ""
}

type edgeStateT struct {
	id     string
	curve  string
	points []Point
	// hasPoints is false when points is undefined.
	hasPoints bool
}

func edgeStateOf(e *ru.Edge) edgeStateT {
	return edgeStateT{id: e.ID, curve: e.Curve, points: e.Points, hasPoints: e.Points != nil}
}

func decodeEdgeState(v []any) edgeStateT {
	s := edgeStateT{id: v[0].(string)}
	if c, ok := v[1].(string); ok {
		s.curve = c
	}
	if pts, ok := v[2].([]any); ok {
		s.hasPoints = true
		s.points = []Point{}
		for i := 0; i+1 < len(pts); i += 2 {
			s.points = append(s.points, Point{X: goldenNum(pts[i]), Y: goldenNum(pts[i+1])})
		}
	}
	return s
}

func pointsText(points []Point) string {
	var sb strings.Builder
	for i, p := range points {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString("(" + numText(p.X) + "," + numText(p.Y) + ")")
	}
	return sb.String()
}

func diffEdgeState(got, want edgeStateT) string {
	if got.id != want.id {
		return fmt.Sprintf("id = %q, want %q", got.id, want.id)
	}
	if got.curve != want.curve {
		return fmt.Sprintf("curve = %q, want %q", got.curve, want.curve)
	}
	// An empty list of points and no points are the same for every reader.
	same := len(got.points) == len(want.points)
	for i := 0; same && i < len(got.points); i++ {
		same = sameNum(got.points[i].X, want.points[i].X) && sameNum(got.points[i].Y, want.points[i].Y)
	}
	if !same {
		return fmt.Sprintf("points =\n      %s\n    want\n      %s", pointsText(got.points), pointsText(want.points))
	}
	return ""
}

// expectedState is the state of the golden file after a step.
type expectedState struct {
	nodes []nodeStateT
	edges []edgeStateT
}

// apply applies the difference of a step: [name, [[node index, state]...],
// edges], where edges is the whole list or { d: [[edge index, state]...] }.
func (s *expectedState) apply(step json.RawMessage) (name string, err error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(step, &parts); err != nil {
		return "", err
	}
	if err := json.Unmarshal(parts[0], &name); err != nil {
		return "", err
	}
	var nodes [][]any
	if err := json.Unmarshal(parts[1], &nodes); err != nil {
		return name, err
	}
	for _, entry := range nodes {
		index := int(entry[0].(float64))
		for len(s.nodes) <= index {
			s.nodes = append(s.nodes, nodeStateT{})
		}
		s.nodes[index] = decodeNodeState(entry[1].([]any))
	}
	var edges any
	if err := json.Unmarshal(parts[2], &edges); err != nil {
		return name, err
	}
	switch e := edges.(type) {
	case []any:
		s.edges = s.edges[:0]
		for _, entry := range e {
			s.edges = append(s.edges, decodeEdgeState(entry.([]any)))
		}
	case map[string]any:
		for _, entry := range e["d"].([]any) {
			pair := entry.([]any)
			s.edges[int(pair[0].(float64))] = decodeEdgeState(pair[1].([]any))
		}
	}
	return name, nil
}

// compare returns the first difference between the layout data and the
// expected state, or "".
func (s *expectedState) compare(data *ru.LayoutData) string {
	if len(data.Nodes) != len(s.nodes) {
		return fmt.Sprintf("%d nodes, want %d", len(data.Nodes), len(s.nodes))
	}
	for i, n := range data.Nodes {
		if d := diffNodeState(nodeStateOf(n), s.nodes[i]); d != "" {
			return fmt.Sprintf("node %q: %s", n.ID, d)
		}
	}
	if len(data.Edges) != len(s.edges) {
		return fmt.Sprintf("%d edges, want %d", len(data.Edges), len(s.edges))
	}
	for i, e := range data.Edges {
		if d := diffEdgeState(edgeStateOf(e), s.edges[i]); d != "" {
			return fmt.Sprintf("edge %q: %s", e.ID, d)
		}
	}
	return ""
}

func graphOf(g *Graph) goldenGraph {
	out := goldenGraph{Nodes: append([]string{}, g.Nodes...), Edges: edgeTriples(g.Edges)}
	return out
}

func edgeTriples(edges []*EdgeRef) [][3]string {
	out := make([][3]string, len(edges))
	for i, e := range edges {
		out[i] = [3]string{e.ID, e.Src, e.Dst}
	}
	return out
}

func diffGraph(what string, got, want goldenGraph) string {
	if !reflect.DeepEqual(got.Nodes, append([]string{}, want.Nodes...)) {
		return fmt.Sprintf("%s nodes = %v, want %v", what, got.Nodes, want.Nodes)
	}
	return diffTriples(what+" edges", got.Edges, want.Edges)
}

func diffTriples(what string, got, want [][3]string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("%s: %d edges, want %d\n got %v\nwant %v", what, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Sprintf("%s[%d] = %v, want %v", what, i, got[i], want[i])
		}
	}
	return ""
}

func diffLayers(what string, got, want [][]string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("%s: %d layers, want %d\n got %v\nwant %v", what, len(got), len(want), got, want)
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			return fmt.Sprintf("%s[%d] = %v, want %v", what, i, got[i], want[i])
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				return fmt.Sprintf("%s[%d] = %v, want %v", what, i, got[i], want[i])
			}
		}
	}
	return ""
}

func diffRanks(what string, got, want map[string]int) string {
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g, ok := got[k]
		if !ok || g != want[k] {
			return fmt.Sprintf("%s[%q] = %d (defined %v), want %d", what, k, g, ok, want[k])
		}
	}
	if len(got) != len(want) {
		return fmt.Sprintf("%s has %d entries, want %d", what, len(got), len(want))
	}
	return ""
}

func diffCoords(what string, got map[string]float64, want map[string]any) string {
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g, ok := got[k]
		w := goldenNum(want[k])
		if !ok || !sameNum(g, w) {
			return fmt.Sprintf("%s[%q] = %s (defined %v), want %s", what, k, numText(g), ok, numText(w))
		}
	}
	if len(got) != len(want) {
		return fmt.Sprintf("%s has %d entries, want %d", what, len(got), len(want))
	}
	return ""
}

// diffSugiyama returns the first difference in the Sugiyama phases, in the
// order of the phases.
func diffSugiyama(view *Graph, result *LayoutResult, want *goldenSugiyama) string {
	if d := diffGraph("toGraphView", graphOf(view), want.View); d != "" {
		return d
	}
	if d := diffGraph("normalizeGraph", graphOf(result.Normalized), want.Normalized); d != "" {
		return d
	}
	if !reflect.DeepEqual(append([]string{}, result.LaneOrder...), append([]string{}, want.LaneOrder...)) {
		return fmt.Sprintf("optimizeTopLaneOrder = %v, want %v", result.LaneOrder, want.LaneOrder)
	}
	if d := diffTriples("removeCycles acyclic", edgeTriples(result.Acyclic.Edges), want.Acyclic); d != "" {
		return d
	}
	if d := diffTriples("removeCycles reversed", edgeTriples(result.Reversed), want.Reversed); d != "" {
		return d
	}
	if d := diffLayers("layering layers", result.InitialLayers, want.Initial.Layers); d != "" {
		return d
	}
	// makeProperLayering adds the ranks of the dummies to the same record.
	dummies := map[string]bool{}
	for _, id := range result.Layering.Dummy.Values() {
		dummies[id] = true
	}
	initialRanks := map[string]int{}
	for k, v := range result.Layering.RankOf {
		if !dummies[k] {
			initialRanks[k] = v
		}
	}
	if d := diffRanks("layering rankOf", initialRanks, want.Initial.RankOf); d != "" {
		return d
	}
	if d := diffLayers("makeProperLayering layers", result.Layering.Layers, want.Proper.Layers); d != "" {
		return d
	}
	if d := diffRanks("makeProperLayering rankOf", result.Layering.RankOf, want.Proper.RankOf); d != "" {
		return d
	}
	if !reflect.DeepEqual(append([]string{}, result.Layering.Dummy.Values()...), append([]string{}, want.Proper.Dummy...)) {
		return fmt.Sprintf("makeProperLayering dummy = %v, want %v", result.Layering.Dummy.Values(), want.Proper.Dummy)
	}
	if d := diffGraph("graphWithDummies", graphOf(result.GraphWithDummies), want.WithDummies); d != "" {
		return d
	}
	if d := diffLayers("orderLayers", result.Ordered.Layers, want.Ordered); d != "" {
		return d
	}
	if d := diffCoords("assignCoordinates x", result.Coordinates.X, want.Coords.X); d != "" {
		return d
	}
	if d := diffCoords("assignCoordinates y", result.Coordinates.Y, want.Coords.Y); d != "" {
		return d
	}
	return ""
}

// thrown runs fn and returns the message of the error that it raises, as
// upstream throws it, or "".
func thrown(fn func()) (message string) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(jsError); ok {
				message = string(e)
				return
			}
			panic(r)
		}
	}()
	fn()
	return ""
}

func diffLayering(what string, call func() *Layering, want goldenLayering) string {
	var got *Layering
	message := thrown(func() { got = call() })
	if want.Error != nil {
		if message != *want.Error {
			return fmt.Sprintf("%s: error %q, upstream throws %q", what, message, *want.Error)
		}
		return ""
	}
	if message != "" {
		return fmt.Sprintf("%s: error %q, upstream has none", what, message)
	}
	if d := diffLayers(what+" layers", got.Layers, want.Layers); d != "" {
		return d
	}
	return diffRanks(what+" rankOf", got.RankOf, want.RankOf)
}

// diffRankResult compares ranks with a recorded result, which is a record
// of ranks or { error }.
func diffRankResult(what string, call func() map[string]int, want json.RawMessage) string {
	var got map[string]int
	message := thrown(func() { got = call() })
	var wantRanks map[string]int
	if err := json.Unmarshal(want, &wantRanks); err != nil {
		var wantError struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(want, &wantError); err != nil {
			return what + ": " + err.Error()
		}
		if message != wantError.Error {
			return fmt.Sprintf("%s: error %q, upstream throws %q", what, message, wantError.Error)
		}
		return ""
	}
	if message != "" {
		return fmt.Sprintf("%s: error %q, upstream has none", what, message)
	}
	return diffRanks(what, got, wantRanks)
}

// directLongestPathOptions are LP_OPTIONS of the probe.
var directLongestPathOptions = []*LayeringOptions{
	nil,
	{IgnoreCrossLaneEdges: optTrue, CompactSingleInput: optTrue, OptimizeRanksByCrossings: optTrue},
	{IgnoreCrossLaneEdges: optTrue, CompactSingleInput: optFalse, OptimizeRanksByCrossings: optFalse},
	{IgnoreCrossLaneEdges: optFalse, CompactSingleInput: optTrue, OptimizeRanksByCrossings: optFalse},
}

// diffDirect compares the direct calls of the layering functions.
func diffDirect(gAcyclic *Graph, direction string, want *goldenDirect) string {
	if len(want.LongestPath) != len(directLongestPathOptions) {
		return fmt.Sprintf("direct: %d longest-path results", len(want.LongestPath))
	}
	for i, opts := range directLongestPathOptions {
		what := fmt.Sprintf("direct assignLayers_LongestPath[%d]", i)
		if d := diffLayering(what, func() *Layering { return assignLayers_LongestPath(gAcyclic, opts) }, want.LongestPath[i]); d != "" {
			return d
		}
	}
	if d := diffLayering("direct assignLayers_Gravity", func() *Layering {
		return assignLayers_Gravity(gAcyclic, &LayeringOptions{IgnoreCrossLaneEdges: optTrue, CompactSingleInput: optTrue, OptimizeRanksByCrossings: optTrue})
	}, want.Gravity); d != "" {
		return d
	}
	if d := diffLayering("direct assignLayers_LaneAwareCompact", func() *Layering {
		return assignLayers_LaneAwareCompact(gAcyclic, &LayeringOptions{IgnoreCrossLaneEdges: optFalse, Direction: direction})
	}, want.Compact); d != "" {
		return d
	}
	if want.Loose == nil {
		return ""
	}
	gn := normalizeGraph(gAcyclic)
	loose := func() map[string]int {
		ranks := make(map[string]int, len(want.Loose))
		for k, v := range want.Loose {
			ranks[k] = v
		}
		return ranks
	}
	if d := diffRankResult("direct optimizeRanksByCrossings", func() map[string]int {
		return optimizeRanksByCrossings(gn, loose())
	}, want.Optimize); d != "" {
		return d
	}
	return diffRankResult("direct adjustCrossLaneSources", func() map[string]int {
		ranks := loose()
		adjustCrossLaneSources(gn, ranks)
		return ranks
	}, want.Adjust)
}

// runGoldenCore runs the layout core on the input of a record and compares
// every step. It returns the name of the first step that differs with the
// difference, and the number of steps that were compared.
func runGoldenCore(record *goldenRecord) (failedStep, diff string, steps int) {
	data := buildLayoutData(record.Input)
	view := toGraphView(data)

	expected := &expectedState{}
	next := 0
	// check applies the next expected step, which must have the given name,
	// and compares the layout data with it.
	check := func(name string) bool {
		if failedStep != "" {
			return false
		}
		if next >= len(record.Steps) {
			failedStep, diff = name, "upstream has no such step"
			return false
		}
		wantName, err := expected.apply(record.Steps[next])
		next++
		if err != nil {
			failedStep, diff = name, err.Error()
			return false
		}
		if wantName != name {
			failedStep, diff = name, fmt.Sprintf("upstream runs the step %q here", wantName)
			return false
		}
		if d := expected.compare(data); d != "" {
			failedStep, diff = name, d
			return false
		}
		steps++
		return true
	}

	if record.Error != nil {
		// Upstream throws: the layout core must fail too. The record has no
		// steps.
		if _, err := runSwimlaneLayoutCore(data); err == nil {
			return "error", fmt.Sprintf("no error, upstream throws %q", *record.Error), 0
		} else if err.Error() != *record.Error {
			return "error", fmt.Sprintf("error %q, upstream throws %q", err, *record.Error), 0
		}
		return "", "", 1
	}

	check("input")
	_, err := runSwimlaneLayoutCoreTraced(data, func(step string, result *LayoutResult) {
		if failedStep != "" {
			return
		}
		if step == "sugiyama" {
			if record.Sugiyama == nil {
				failedStep, diff = step, "upstream has no result of the Sugiyama phases"
				return
			}
			if d := diffSugiyama(view, result, record.Sugiyama); d != "" {
				failedStep, diff = step, d
				return
			}
			steps++
			if record.Direct != nil {
				if d := diffDirect(result.Acyclic, getSwimlaneDirection(data), record.Direct); d != "" {
					failedStep, diff = "direct", d
					return
				}
				steps++
			}
			return
		}
		check(step)
	})
	if failedStep != "" {
		return failedStep, diff, steps
	}
	if err != nil {
		return "error", fmt.Sprintf("error %q, upstream has none", err), steps
	}
	if next != len(record.Steps) {
		return "end", fmt.Sprintf("%d steps, upstream has %d", next, len(record.Steps)), steps
	}

	// validateSwimlanesLayout runs on the final layout.
	issues := validateSwimlanesLayout(data)
	if len(issues) != len(record.Issues) {
		return "validate", fmt.Sprintf("%d issues, want %d", len(issues), len(record.Issues)), steps
	}
	for i, issue := range issues {
		got := [4]string{issue.Type, issue.EdgeId, issue.TargetId, issue.Detail}
		if got != record.Issues[i] {
			return "validate", fmt.Sprintf("issue %d = %v, want %v", i, got, record.Issues[i]), steps
		}
	}
	steps++
	return "", "", steps
}

func kindOf(name string) string {
	if i := strings.Index(name, "/"); i >= 0 {
		return name[:i]
	}
	return name
}

// TestGoldenLayoutCore compares the layout core with upstream, step by
// step, for every record of the golden file. With -short it runs one
// record in four.
func TestGoldenLayoutCore(t *testing.T) {
	records := loadGolden(t)
	if testing.Short() {
		var subset []*goldenRecord
		for i, record := range records {
			if i%4 == 0 {
				subset = append(subset, record)
			}
		}
		records = subset
	}
	type outcome struct {
		failedStep, diff string
		steps            int
	}
	outcomes := make([]outcome, len(records))
	var wg sync.WaitGroup
	slots := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, record := range records {
		if record.Input == nil {
			outcomes[i] = outcome{failedStep: "input", diff: "the record has no input"}
			continue
		}
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			failedStep, diff, steps := runGoldenCore(record)
			outcomes[i] = outcome{failedStep: failedStep, diff: diff, steps: steps}
		}()
	}
	wg.Wait()

	type count struct{ cases, exact, steps int }
	counts := map[string]*count{}
	firstFailure := map[string]int{}
	failures := 0
	for i, record := range records {
		kind := kindOf(record.Name)
		c := counts[kind]
		if c == nil {
			c = &count{}
			counts[kind] = c
		}
		c.cases++
		o := outcomes[i]
		c.steps += o.steps
		if o.failedStep == "" {
			c.exact++
			continue
		}
		firstFailure[o.failedStep]++
		failures++
		if failures <= 12 {
			t.Errorf("%s: step %s: %s", record.Name, o.failedStep, o.diff)
		}
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		c := counts[k]
		t.Logf("%-10s %4d cases, %4d exact, %6d steps compared", k, c.cases, c.exact, c.steps)
	}
	if failures > 0 {
		steps := make([]string, 0, len(firstFailure))
		for s := range firstFailure {
			steps = append(steps, s)
		}
		sort.Slice(steps, func(i, j int) bool { return firstFailure[steps[i]] > firstFailure[steps[j]] })
		for _, s := range steps {
			t.Logf("first difference in step %-45s %d cases", s, firstFailure[s])
		}
		t.Errorf("%d of %d cases differ from upstream", failures, len(records))
	}
}

// TestGoldenPrepare compares the prepare phase (prepareLayoutForSwimlanes
// and createEdgeLabelNodes) with upstream.
func TestGoldenPrepare(t *testing.T) {
	records := loadGolden(t)
	failures := 0
	cases := 0
	for _, record := range records {
		if record.Raw == nil || record.Prepared == nil {
			continue
		}
		cases++
		data := buildLayoutData(record.Raw)
		if _, err := prepareSwimlaneLayout(data, nil); err != nil {
			t.Errorf("%s: %v", record.Name, err)
			continue
		}
		if d := diffPrepared(data, record.Prepared); d != "" {
			failures++
			if failures <= 10 {
				t.Errorf("%s: %s", record.Name, d)
			}
		}
	}
	t.Logf("%d cases, %d exact", cases, cases-failures)
	if cases == 0 {
		t.Fatal("no cases")
	}
}

func diffPrepared(data *ru.LayoutData, want *goldenData) string {
	if len(data.Nodes) != len(want.Nodes) {
		return fmt.Sprintf("%d nodes, want %d", len(data.Nodes), len(want.Nodes))
	}
	for i, n := range data.Nodes {
		w := want.Nodes[i]
		direction, _ := nodeDirection(n)
		got := []any{n.ID, n.ParentID, n.IsGroup, n.Shape, n.Label, n.LabelStyle, n.Look, n.Dir, direction, n.IsEdgeLabel, n.IsDummy, n.EdgeStart, n.EdgeEnd}
		exp := []any{w.ID, str(w.ParentID), w.IsGroup, str(w.Shape), str(w.Label), str(w.LabelStyle), str(w.Look), str(w.Dir), str(w.Direction), w.IsEdgeLabel, w.IsDummy, str(w.EdgeStart), str(w.EdgeEnd)}
		if !reflect.DeepEqual(got, exp) {
			return fmt.Sprintf("node %d = %v, want %v", i, got, exp)
		}
		if n.IsEdgeLabel && n.LabelStyleUndefined != (w.LabelStyle == nil) {
			return fmt.Sprintf("node %q: labelStyle undefined = %v, want %v", n.ID, n.LabelStyleUndefined, w.LabelStyle == nil)
		}
		if n.HasLabel != (w.Label != nil) {
			return fmt.Sprintf("node %q: label defined = %v, want %v", n.ID, n.HasLabel, w.Label != nil)
		}
		if !sameNum(n.ColorIndex, goldenNum(w.ColorIndex)) || !sameNum(n.Padding, goldenNum(w.Padding)) {
			return fmt.Sprintf("node %q: colorIndex %s padding %s, want %s %s", n.ID, numText(n.ColorIndex), numText(n.Padding), numText(goldenNum(w.ColorIndex)), numText(goldenNum(w.Padding)))
		}
	}
	if len(data.Edges) != len(want.Edges) {
		return fmt.Sprintf("%d edges, want %d", len(data.Edges), len(want.Edges))
	}
	for i, e := range data.Edges {
		w := want.Edges[i]
		got := []any{e.ID, e.Start, e.End, e.Label, e.Text, e.Type, e.LabelNodeID, e.IsLayoutOnly}
		exp := []any{w.ID, str(w.Start), str(w.End), str(w.Label), str(w.Text), str(w.Type), str(w.LabelNodeID), w.IsLayoutOnly}
		if !reflect.DeepEqual(got, exp) {
			return fmt.Sprintf("edge %d = %v, want %v", i, got, exp)
		}
	}
	return ""
}
