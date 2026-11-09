package treeview

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// unitsGolden is testdata/units.json.gz, which
// tools/reference/gen-treeview-units.mjs wrote with the real functions of
// the treeView diagram of mermaid 12.2.0.
type unitsGolden struct {
	BoxDrawing []struct {
		Input   string   `json:"input"`
		Text    string   `json:"text"`
		LineMap [][2]int `json:"lineMap"`
		Error   *string  `json:"error"`
	} `json:"boxDrawing"`
	Remap []struct {
		Message string   `json:"message"`
		LineMap [][2]int `json:"lineMap"`
		Result  string   `json:"result"`
	} `json:"remap"`
	Icons []struct {
		Node   json.RawMessage `json:"node"`
		Config json.RawMessage `json:"config"`
		Result any             `json:"result"`
		Error  *string         `json:"error"`
	} `json:"icons"`
}

// TestPreprocessBoxDrawing compares the pre-processor of box-drawing input
// with preprocessBoxDrawing of mermaid: the text, the line map and the
// error, for hand-written and random inputs.
func TestPreprocessBoxDrawing(t *testing.T) {
	var golden unitsGolden
	readGolden(t, "units.json.gz", &golden)
	if len(golden.BoxDrawing) < 1500 {
		t.Fatalf("the golden file has %d cases", len(golden.BoxDrawing))
	}
	changed, failed := 0, 0
	for _, c := range golden.BoxDrawing {
		text, lineMap, err := preprocessBoxDrawing(c.Input)
		if c.Error != nil {
			failed++
			if err == nil {
				t.Errorf("input %q: no error, mermaid fails with %q", c.Input, *c.Error)
			} else if err.Error() != *c.Error {
				t.Errorf("input %q:\n got error %q\nwant error %q", c.Input, err.Error(), *c.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("input %q: error %v, mermaid accepts the input", c.Input, err)
			continue
		}
		if text != c.Text {
			t.Errorf("input %q:\n got %q\nwant %q", c.Input, text, c.Text)
			continue
		}
		if text != c.Input {
			changed++
		}
		entries := [][2]int{}
		lineMap.Each(func(k, v int) { entries = append(entries, [2]int{k, v}) })
		if !reflect.DeepEqual(entries, append([][2]int{}, c.LineMap...)) {
			t.Errorf("input %q:\n got line map %v\nwant line map %v", c.Input, entries, c.LineMap)
		}
	}
	t.Logf("%d cases: %d that change the text, %d errors", len(golden.BoxDrawing), changed, failed)
}

// TestRemapErrorLines compares remapErrorLines with mermaid.
func TestRemapErrorLines(t *testing.T) {
	var golden unitsGolden
	readGolden(t, "units.json.gz", &golden)
	if len(golden.Remap) < 30 {
		t.Fatalf("the golden file has %d cases", len(golden.Remap))
	}
	for _, c := range golden.Remap {
		lineMap := js.NewMap[int, int]()
		for _, e := range c.LineMap {
			lineMap.Set(e[0], e[1])
		}
		if got := remapErrorLines(c.Message, lineMap); got != c.Result {
			t.Errorf("message %q, line map %v:\n got %q\nwant %q", c.Message, c.LineMap, got, c.Result)
		}
	}
	t.Logf("%d cases", len(golden.Remap))
}

// decode reads a value of the golden: {"$u":1} is undefined.
func decode(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var walk func(v any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case *js.Obj[any]:
			if x.Len() == 1 && x.Has("$u") {
				return nil
			}
			out := js.NewObj[any]()
			x.Each(func(key string, e any) { out.Set(key, walk(e)) })
			return out
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = walk(e)
			}
			return out
		}
		return v
	}
	return walk(v)
}

// TestGetNodeIcon compares getNodeIcon with mermaid for random nodes and
// configs: the icon reference, no icon, or the error for an icon that is not
// a string.
func TestGetNodeIcon(t *testing.T) {
	var golden unitsGolden
	readGolden(t, "units.json.gz", &golden)
	if len(golden.Icons) < 3000 {
		t.Fatalf("the golden file has %d cases", len(golden.Icons))
	}
	withIcon, failed := 0, 0
	for _, c := range golden.Icons {
		nodeObj := js.AsObj(decode(t, c.Node))
		config := js.AsObj(decode(t, c.Config))
		node := &Node{Name: js.ToString(nodeObj.Get("name")), NodeType: NodeType(js.ToString(nodeObj.Get("nodeType")))}
		if icon, ok := nodeObj.Get("icon").(string); ok {
			node.Icon = icon
		}
		icon, err := getNodeIcon(node, config)
		if c.Error != nil {
			failed++
			if err == nil {
				t.Errorf("node %s, config %s: no error, mermaid fails with %q", c.Node, c.Config, *c.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("node %s, config %s: error %v, mermaid gives %v", c.Node, c.Config, err, c.Result)
			continue
		}
		// The result is a string, an array of strings or undefined.
		var got any = map[string]any{"$u": 1.0}
		switch x := icon.(type) {
		case string:
			got = x
		case []any:
			got = x
		}
		if !reflect.DeepEqual(got, c.Result) {
			t.Errorf("node %s, config %s:\n got %v\nwant %v", c.Node, c.Config, got, c.Result)
			continue
		}
		if icon != nil {
			withIcon++
		}
	}
	t.Logf("%d cases: %d with an icon, %d errors", len(golden.Icons), withIcon, failed)
}
