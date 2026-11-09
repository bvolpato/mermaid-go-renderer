// Package nativetest has what the tests of internal/cytoscape/native and of
// internal/cytoscape share: the golden files of the native layouts, the
// digest of a trace record, the random generator of the oracle, and random
// inputs.
package nativetest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
)

// Num is a number of a golden file: a JSON number, or one of the strings
// "NaN", "Infinity", "-Infinity", "-0" and "undefined". undefined is NaN, as
// in the results of the Go API.
type Num float64

func (n Num) MarshalJSON() ([]byte, error) {
	f := float64(n)
	switch {
	case f != f:
		return []byte(`"NaN"`), nil
	case math.IsInf(f, 1):
		return []byte(`"Infinity"`), nil
	case math.IsInf(f, -1):
		return []byte(`"-Infinity"`), nil
	case f == 0 && math.Signbit(f):
		return []byte(`"-0"`), nil
	}
	return strconv.AppendFloat(nil, f, 'g', -1, 64), nil
}

func (n *Num) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		switch s {
		case "NaN", "undefined":
			*n = Num(math.NaN())
		case "Infinity":
			*n = Num(math.Inf(1))
		case "-Infinity":
			*n = Num(math.Inf(-1))
		case "-0":
			*n = Num(math.Copysign(0, -1))
		default:
			return fmt.Errorf("unknown number %q", s)
		}
		return nil
	}
	f, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	*n = Num(f)
	return nil
}

// TraceRecord is one record of a trace: the stage, the number of values,
// the digest of the values, and the number of Math.random calls before it.
type TraceRecord struct {
	Stage  string
	Count  int
	Digest string
	Random int
}

func (r TraceRecord) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{r.Stage, r.Count, r.Digest, r.Random})
}

func (r *TraceRecord) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 4 {
		return fmt.Errorf("a trace record has %d members", len(raw))
	}
	if err := json.Unmarshal(raw[0], &r.Stage); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &r.Count); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &r.Digest); err != nil {
		return err
	}
	return json.Unmarshal(raw[3], &r.Random)
}

func (r TraceRecord) String() string {
	return fmt.Sprintf("%s (%d values, digest %s, %d random calls)", r.Stage, r.Count, r.Digest, r.Random)
}

// Digest is the hash of a list of numbers by their bits, as
// tools/reference/gen-cytoscape-native.mjs computes it. Every NaN is the
// same value.
func Digest(values []float64) string {
	h1 := uint32(0x811c9dc5)
	h2 := uint32(0x9747b28c)
	for _, v := range values {
		bits := math.Float64bits(v)
		lo, hi := uint32(bits), uint32(bits>>32)
		if v != v {
			lo, hi = 0, 0x7ff80000
		}
		h1 = (h1 ^ lo) * 16777619
		h1 = (h1 ^ hi) * 16777619
		h2 = (h2 ^ hi) * 0x85ebca6b
		h2 = (h2 ^ lo) * 0xc2b2ae35
		h2 ^= h2 >> 13
	}
	return fmt.Sprintf("%08x%08x", h1, h2)
}

// Tracer collects the records of a layout.
type Tracer struct {
	Records []TraceRecord
	// Values has the values of each record when Keep is set.
	Keep   bool
	Values [][]float64
	// RandomCalls returns the number of Math.random calls so far.
	RandomCalls func() int
}

// Trace is the trace function of a layout.
func (t *Tracer) Trace(stage string, values []float64) {
	r := TraceRecord{Stage: stage, Count: len(values), Digest: Digest(values)}
	if t.RandomCalls != nil {
		r.Random = t.RandomCalls()
	}
	t.Records = append(t.Records, r)
	if t.Keep {
		t.Values = append(t.Values, append([]float64(nil), values...))
	}
}

// FirstDifference returns the index of the first record that differs between
// two traces, and a text for it. The index is -1 when the traces are equal.
func FirstDifference(got, want []TraceRecord) (int, string) {
	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			return i, fmt.Sprintf("record %d: got %v, want %v", i, got[i], want[i])
		}
	}
	if len(got) != len(want) {
		i := min(len(got), len(want))
		return i, fmt.Sprintf("got %d records, want %d", len(got), len(want))
	}
	return -1, ""
}

// Mulberry32 is the Math.random of the reference oracle
// (tools/reference/oracle.mjs) for a seed.
type Mulberry32 struct {
	state uint32
	// Calls is the number of draws.
	Calls int
}

// NewMulberry32 returns the generator after skip draws, with Calls 0.
func NewMulberry32(seed uint32, skip int) *Mulberry32 {
	m := &Mulberry32{state: seed}
	for i := 0; i < skip; i++ {
		m.Random()
	}
	m.Calls = 0
	return m
}

// Random is one draw.
func (m *Mulberry32) Random() float64 {
	m.Calls++
	m.state += 0x6d2b79f5
	t := m.state
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

// PageSeed is the seed of Math.random of the reference oracle.
const PageSeed = 0x6d6d6467

// ReadGzipJSON decodes a gzip-compressed JSON file.
func ReadGzipJSON(file string, v any) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	return json.NewDecoder(zr).Decode(v)
}

// GoldenError is the error that the JavaScript threw for a case.
type GoldenError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// GoldenNode is a node of a cose-bilkent case: [id, width, height, x, y] in
// the file.
type GoldenNode struct {
	ID         string
	W, H, X, Y Num
}

func (n GoldenNode) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{n.ID, n.W, n.H, n.X, n.Y})
}

func (n *GoldenNode) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 5 {
		return fmt.Errorf("a node has %d members", len(raw))
	}
	if err := json.Unmarshal(raw[0], &n.ID); err != nil {
		return err
	}
	for i, dst := range []*Num{&n.W, &n.H, &n.X, &n.Y} {
		if err := json.Unmarshal(raw[i+1], dst); err != nil {
			return err
		}
	}
	return nil
}

// CoseBilkentGolden is the result of a cose-bilkent case.
type CoseBilkentGolden struct {
	// Nodes are [x, y].
	Nodes [][2]Num `json:"nodes"`
	// Edges are [startX, startY, midX, midY, endX, endY].
	Edges [][6]Num `json:"edges"`
}

// CoseBilkentCase is a case of testdata/cosebilkent.json.gz.
type CoseBilkentCase struct {
	Name string `json:"name"`
	// Draws are the results of Math.random of the page that the JavaScript
	// took, in order.
	Draws []Num        `json:"draws"`
	Nodes []GoldenNode `json:"nodes"`
	// Edges are [id, start, end].
	Edges  [][3]string        `json:"edges"`
	Trace  []TraceRecord      `json:"trace"`
	Error  *GoldenError       `json:"error,omitempty"`
	Result *CoseBilkentGolden `json:"result,omitempty"`
}

// Input returns the arguments of native.CoseBilkent.
func (c *CoseBilkentCase) Input() ([]native.Node, []native.Edge) {
	nodes := make([]native.Node, len(c.Nodes))
	for i, n := range c.Nodes {
		nodes[i] = native.Node{ID: n.ID, Width: float64(n.W), Height: float64(n.H), X: float64(n.X), Y: float64(n.Y)}
	}
	edges := make([]native.Edge, len(c.Edges))
	for i, e := range c.Edges {
		edges[i] = native.Edge{ID: e[0], Start: e[1], End: e[2]}
	}
	return nodes, edges
}

// CoseBilkentFile is testdata/cosebilkent.json.gz.
type CoseBilkentFile struct {
	Cases []*CoseBilkentCase `json:"cases"`
}

// Measure is one result of MeasureText: [font, text, width] in the file.
type Measure struct {
	Font, Text string
	Width      Num
}

func (m Measure) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{m.Font, m.Text, m.Width})
}

func (m *Measure) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) != 3 {
		return fmt.Errorf("a measure has %d members", len(raw))
	}
	if err := json.Unmarshal(raw[0], &m.Font); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &m.Text); err != nil {
		return err
	}
	return json.Unmarshal(raw[2], &m.Width)
}

// DocsCoseBilkent is the input of a cose-bilkent layout of a real diagram.
type DocsCoseBilkent struct {
	Name  string       `json:"name"`
	Nodes []GoldenNode `json:"nodes"`
	Edges [][3]string  `json:"edges"`
}

// DocsArchitecture is the input of an fcose layout of a real diagram, with
// the widths that the layout measured.
type DocsArchitecture struct {
	Name    string                   `json:"name"`
	Input   native.ArchitectureInput `json:"input"`
	Measure []Measure                `json:"measure"`
}

// DocsInputs is testdata/docs-inputs.json.gz: the inputs that the diagrams
// of testdata/corpus/mermaid-docs and testdata/corpus/mermaid-large give to
// cytoscape.
type DocsInputs struct {
	CoseBilkent  []DocsCoseBilkent  `json:"coseBilkent"`
	Architecture []DocsArchitecture `json:"architecture"`
}

// Replay returns a Math.random that gives the draws in order, and then
// fails the test of the caller through used: *used counts the calls.
func Replay(draws []Num, used *int) func() float64 {
	return func() float64 {
		i := *used
		*used++
		if i < len(draws) {
			return float64(draws[i])
		}
		return 0.5
	}
}

// WriteGzipJSON writes v as gzip-compressed JSON.
func WriteGzipJSON(file string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		f.Close()
		return err
	}
	if _, err := zw.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// SameBits tells whether two numbers have the same bits. Every NaN is the
// same value.
func SameBits(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (a != a && b != b)
}

// ArchitectureGolden is the result of an architecture case.
type ArchitectureGolden struct {
	// Nodes has, for each node, the six values of the bounding box of a
	// group, or the position of another node.
	Nodes [][]Num `json:"nodes"`
	// Edges are [sourceEndpoint.x, sourceEndpoint.y, midpoint.x,
	// midpoint.y, targetEndpoint.x, targetEndpoint.y].
	Edges [][6]Num `json:"edges"`
}

// ArchitectureCase is a case of testdata/fcose.json.gz.
type ArchitectureCase struct {
	Name  string                   `json:"name"`
	Input native.ArchitectureInput `json:"input"`
	// Measure are the calls of MeasureText of the JavaScript, in order.
	Measure []Measure `json:"measure"`
	// Draws are the results of Math.random of the page that the JavaScript
	// took, in order.
	Draws  []Num               `json:"draws"`
	Trace  []TraceRecord       `json:"trace"`
	Error  *GoldenError        `json:"error,omitempty"`
	Result *ArchitectureGolden `json:"result,omitempty"`
}

// ArchitectureFile is testdata/fcose.json.gz.
type ArchitectureFile struct {
	Cases []*ArchitectureCase `json:"cases"`
}

// MeasureReplay returns a MeasureText that gives the recorded widths. calls
// receives each call.
func MeasureReplay(recorded []Measure, calls *[]Measure) func(font, text string) float64 {
	widths := make(map[[2]string]float64, len(recorded))
	for _, m := range recorded {
		widths[[2]string{m.Font, m.Text}] = float64(m.Width)
	}
	return func(font, text string) float64 {
		w, ok := widths[[2]string{font, text}]
		if !ok {
			w = math.NaN()
		}
		*calls = append(*calls, Measure{Font: font, Text: text, Width: Num(w)})
		return w
	}
}
