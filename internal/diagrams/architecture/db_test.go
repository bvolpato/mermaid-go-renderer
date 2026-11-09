package architecture

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// dbCase is one case of testdata/db.json.gz, which
// tools/reference/gen-architecture-db.mjs wrote with mermaid 12.2.0 in
// Chromium: a diagram text and what the db holds when the draw function of
// the real renderer starts, or the error of mermaid. A case with Full also
// ran the real draw function: Sizes has the size that drawServices gave to
// each service, and DrawError the error of the draw.
type dbCase struct {
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	Site      json.RawMessage `json:"site"`
	Full      bool            `json:"full"`
	Type      string          `json:"type"`
	Error     string          `json:"error"`
	DrawError string          `json:"drawError"`
	// DrawErrorName is the name of the error of the draw: "Error" for an
	// error that mermaid or cytoscape throws, "TypeError" for an error of
	// the JavaScript engine.
	DrawErrorName string  `json:"drawErrorName"`
	DB            any     `json:"db"`
	Sizes         [][]any `json:"sizes"`
}

func setOptionalAny(m map[string]any, key string, v *string) {
	if v != nil {
		m[key] = *v
	}
}

func stringList(list []string) []any {
	out := []any{}
	for _, s := range list {
		out = append(out, s)
	}
	return out
}

// dumpDB is the db in the form of the golden file. See
// tools/reference/architecture-db-page.js.
func dumpDB(t testing.TB, db *ArchitectureDB) any {
	edgeIndex := map[*ArchitectureEdge]int{}
	edges := []any{}
	for i, e := range db.GetEdges() {
		edgeIndex[e] = i
		m := map[string]any{
			"lhsId": e.LhsID, "lhsDir": e.LhsDir, "lhsInto": e.LhsInto, "lhsGroup": e.LhsGroup,
			"rhsId": e.RhsID, "rhsDir": e.RhsDir, "rhsInto": e.RhsInto, "rhsGroup": e.RhsGroup,
		}
		setOptionalAny(m, "title", e.Title)
		edges = append(edges, m)
	}
	nodes := []any{}
	for _, n := range db.GetNodes() {
		m := map[string]any{"id": n.ID, "type": n.Type}
		setOptionalAny(m, "icon", n.Icon)
		setOptionalAny(m, "iconText", n.IconText)
		setOptionalAny(m, "title", n.Title)
		setOptionalAny(m, "in", n.In)
		list := []any{}
		for _, e := range n.Edges {
			i, ok := edgeIndex[e]
			if !ok {
				t.Errorf("node %s has an edge that is not an edge of the db", n.ID)
			}
			list = append(list, float64(i))
		}
		m["edges"] = list
		nodes = append(nodes, m)
	}
	ids := func(list []*ArchitectureNode) []any {
		out := []any{}
		for _, n := range list {
			out = append(out, n.ID)
		}
		return out
	}
	groups := []any{}
	for _, g := range db.GetGroups() {
		m := map[string]any{"id": g.ID}
		setOptionalAny(m, "icon", g.Icon)
		setOptionalAny(m, "title", g.Title)
		setOptionalAny(m, "in", g.In)
		groups = append(groups, m)
	}
	hints := []any{}
	for _, h := range db.GetLayoutHints() {
		hints = append(hints, map[string]any{"direction": h.Direction, "members": stringList(h.Members)})
	}
	registered := []any{}
	db.registeredIds.Each(func(id, kind string) {
		registered = append(registered, []any{id, kind})
	})
	var config any
	if err := json.Unmarshal([]byte(js.StringifyJSON(db.GetConfig())), &config); err != nil {
		t.Fatal(err)
	}
	out := map[string]any{
		"title":         db.GetDiagramTitle(),
		"accTitle":      db.GetAccTitle(),
		"accDescr":      db.GetAccDescription(),
		"nodes":         nodes,
		"services":      ids(db.GetServices()),
		"junctions":     ids(db.GetJunctions()),
		"groups":        groups,
		"edges":         edges,
		"layoutHints":   hints,
		"registeredIds": registered,
		"config":        config,
	}
	ds, err := db.GetDataStructures()
	if err != nil {
		out["dataStructuresError"] = err.Error()
		return out
	}
	adjList := []any{}
	ds.AdjList.Each(func(id string, m ArchitectureDirectionPairMap) {
		pairs := []any{}
		m.Each(func(pair, other string) {
			pairs = append(pairs, []any{pair, other})
		})
		adjList = append(adjList, []any{id, pairs})
	})
	spatialMaps := []any{}
	for _, sm := range ds.SpatialMaps {
		list := []any{}
		sm.Each(func(id string, pos [2]float64) {
			list = append(list, []any{id, []any{pos[0], pos[1]}})
		})
		spatialMaps = append(spatialMaps, list)
	}
	groupAlignments := []any{}
	ds.GroupAlignments.Each(func(key, alignment string) {
		groupAlignments = append(groupAlignments, []any{key, alignment})
	})
	out["dataStructures"] = map[string]any{
		"adjList":         adjList,
		"spatialMaps":     spatialMaps,
		"groupAlignments": groupAlignments,
	}
	return out
}

func loadDBCases(t testing.TB) []dbCase {
	var file struct {
		Cases []dbCase `json:"cases"`
	}
	loadGzipJSON(t, "testdata/db.json.gz", &file)
	return file.Cases
}

// sameError compares the error of the port with the message of mermaid. A
// parser error must be an error of the parser: its text is not compared.
func sameError(err error, want string) bool {
	if strings.HasPrefix(want, "Parsing failed:") {
		var perr *langium.Error
		return errors.As(err, &perr)
	}
	return err.Error() == want
}

// engineTypeError tells whether both errors are a TypeError that the
// JavaScript engine throws while cytoscape runs. V8 and goja word such an
// error in different ways, and V8 quotes the source text. Only the fact
// that both fail is compared.
func engineTypeError(err error, want string) bool {
	var lerr *layoutError
	if !errors.As(err, &lerr) || lerr.name != "TypeError" {
		return false
	}
	return strings.HasPrefix(want, "Cannot destructure property") || strings.HasPrefix(want, "Cannot read properties of")
}

// knownDivergence returns the reason for which the drawing of a text is
// known to differ from mermaid, or "". The db and the layout of such a text
// are still compared.
//
// An icon is a nested svg element with the width and the height of the
// config: iconSize for a service, 0.75 times padding for a group. For a
// width or a height that is zero or negative, getBBox of Chromium takes the
// content of that element without the transform of the viewBox: 80 by 80 at
// the origin for an icon (a probe with .parity/probe/dom.js shows it). The
// measurement engine (nestedSVGTransform of internal/browser) applies the
// transform of the viewBox for every size.
func knownDivergence(text string) string {
	if strings.Contains(text, "iconSize: -") || strings.Contains(text, "padding: -") || strings.Contains(text, "iconSize: 0\n") {
		return "the config has an icon size that is zero or negative, and the measurement engine does not give the bounding box of Chromium for an svg element of that size"
	}
	return ""
}

// TestDBAgainstMermaid compares the db with the db of mermaid in Chromium,
// order included, for the corpus fixtures, mutated fixtures and hand-written
// texts. It requires the error of mermaid where mermaid fails. For the
// cases that ran the real draw function it compares the sizes of the
// services bit for bit, and the error of the draw.
//
// The size of a service has the size of its label in Chromium on the macOS
// reference machine. A machine with other fonts does not compare the sizes,
// and the test is marked as skipped after all the other checks.
func TestDBAgainstMermaid(t *testing.T) {
	cases := loadDBCases(t)
	if len(cases) < 300 {
		t.Fatalf("only %d cases", len(cases))
	}
	reference := fonttest.IsMacOSReference()
	var dumps, rejected, sizes, drawErrors, known, failed atomic.Int64
	t.Run("cases", func(t *testing.T) {
		for _, c := range cases {
			if c.Full && testing.Short() && !strings.HasPrefix(c.Name, "testdata/") {
				continue
			}
			t.Run(c.Name, func(t *testing.T) {
				t.Parallel()
				defer func() {
					if t.Failed() {
						failed.Add(1)
					}
				}()
				var o *observed
				if c.Full && c.Error == "" {
					// The whole render. The test of the layout renders the
					// same text.
					o = renderObserved(t, c.Text, c.Site)
				} else {
					o = renderWith(c.Text, siteConfig(t, c.Site), true)
				}
				db, err := o.db, o.err
				if c.Error != "" {
					rejected.Add(1)
					if err == nil || errors.Is(err, errStop) {
						t.Fatalf("mermaid fails with %q, the port does not\ninput: %q", c.Error, c.Text)
					}
					if !sameError(err, c.Error) {
						t.Fatalf("error %q, want %q\ninput: %q", err, c.Error, c.Text)
					}
					return
				}
				if db == nil {
					t.Fatalf("no db: %v\ninput: %q", err, c.Text)
				}
				if c.Full {
					if c.DrawError != "" {
						drawErrors.Add(1)
						if err == nil {
							t.Fatalf("the draw of mermaid fails with %q, the port draws\ninput: %q", c.DrawError, c.Text)
						}
						if err.Error() != c.DrawError && !(c.DrawErrorName == "TypeError" && engineTypeError(err, c.DrawError)) {
							t.Fatalf("draw error %q, want %q\ninput: %q", err, c.DrawError, c.Text)
						}
					} else if err != nil {
						t.Fatalf("the port fails: %v\ninput: %q", err, c.Text)
					}
				} else if !errors.Is(err, errStop) {
					t.Fatalf("the port fails before the draw: %v\ninput: %q", err, c.Text)
				}
				dumps.Add(1)
				got := dumpDB(t, db)
				if !reflect.DeepEqual(got, c.DB) {
					gj, _ := json.Marshal(got)
					wj, _ := json.Marshal(c.DB)
					t.Fatalf("db differs\ninput: %q\n got: %s\nwant: %s", c.Text, gj, wj)
				}
				if reason := knownDivergence(c.Text); c.Full && reason != "" {
					known.Add(1)
					t.Logf("the sizes of the services are not compared: %s", reason)
				} else if c.Full {
					services := db.GetServices()
					if len(services) != len(c.Sizes) {
						t.Fatalf("%d services, want %d", len(services), len(c.Sizes))
					}
					for i, s := range services {
						want := c.Sizes[i]
						if s.ID != want[0] {
							t.Fatalf("service %d is %q, want %q", i, s.ID, want[0])
						}
						if want[1] == nil {
							// The draw of mermaid failed before it measured the service.
							continue
						}
						sizes.Add(1)
						if !s.HasSize {
							t.Fatalf("service %s has no size, want %v x %v", s.ID, want[1], want[2])
						}
						w, h := want[1].(float64), want[2].(float64)
						if reference && (math.Float64bits(s.Width) != math.Float64bits(w) || math.Float64bits(s.Height) != math.Float64bits(h)) {
							t.Errorf("service %s: size %v x %v, want %v x %v\ninput: %q", s.ID, s.Width, s.Height, w, h, c.Text)
						}
					}
				}
			})
		}
	})
	t.Logf("%d cases: %d db dumps, %d errors of mermaid, %d service sizes, %d draw errors, %d known divergences of the drawing; %d cases differ",
		len(cases), dumps.Load(), rejected.Load(), sizes.Load(), drawErrors.Load(), known.Load(), failed.Load())
	fonttest.RequireMacOSReference(t)
}
