package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// TestConfigAPI replays scripts of config API calls on a State and compares
// every result with the real mermaid: setConfig, updateSiteConfig,
// setSiteConfig, addDirective, setDiagramConfigScope, reset, getConfig,
// getSiteConfig, getUserDefinedConfig and getEffectiveHtmlLabels.
func TestConfigAPI(t *testing.T) {
	var g struct {
		Pool    []json.RawMessage
		Scripts [][][]json.RawMessage
		Results [][]*struct {
			Config *int
			Value  *int
			Error  *goldenError
		}
		Evaluate [][]json.RawMessage
	}
	readGolden(t, "api.json.gz", &g)
	p := newPool(t, g.Pool)
	root := filepath.Join("..", "..", "testdata", "corpus", "mermaid-e2e")
	steps, failed, threw := 0, 0, 0
	for si, script := range g.Scripts {
		// A script starts on a state that initialize() has set up, as the
		// generator does with reset() and initialize().
		state, err := newBareState()
		if err != nil {
			t.Fatal(err)
		}
		for i, step := range script {
			steps++
			var op string
			if err := json.Unmarshal(step[0], &op); err != nil {
				t.Fatal(err)
			}
			arg := decode(t, step[1])
			obj, _ := arg.(*js.Obj[any])
			var gotConfig *js.Obj[any]
			var gotValue any
			hasValue := false
			var gotErr error
			switch op {
			case "initialize":
				gotErr = state.Initialize(obj)
			case "process":
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(arg.(string))))
				if err != nil {
					t.Fatal(err)
				}
				_, _, gotErr = state.ProcessAndSetConfigs(string(data))
			case "setConfig":
				gotConfig, gotErr = state.SetConfig(obj)
			case "updateSiteConfig":
				gotConfig, gotErr = state.UpdateSiteConfig(obj)
			case "setSiteConfig":
				gotConfig, gotErr = state.SetSiteConfig(obj)
			case "addDirective":
				gotErr = state.AddDirective(obj)
				gotValue, hasValue = obj, true
			case "setDiagramConfigScope":
				if s, ok := arg.(string); ok {
					gotErr = state.SetDiagramConfigScope(s)
				} else {
					gotErr = state.LeaveDiagramConfigScope()
				}
			case "reset":
				gotErr = state.Reset()
			case "getConfig":
				gotConfig = state.GetConfig()
			case "getSiteConfig":
				gotConfig = state.GetSiteConfig()
			case "getUserDefinedConfig":
				gotValue, hasValue = state.GetUserDefinedConfig(), true
			case "getEffectiveHtmlLabels":
				gotValue, hasValue = GetEffectiveHtmlLabels(state.GetConfig()), true
			default:
				t.Fatalf("unknown step %s", op)
			}
			want := g.Results[si][i]
			var problem string
			switch {
			case want != nil && want.Error != nil:
				threw++
				if gotErr == nil {
					problem = fmt.Sprintf("no error, want %s: %s", want.Error.Name, want.Error.Message)
				} else if e, ok := gotErr.(*Error); !ok || e.Name != want.Error.Name {
					problem = fmt.Sprintf("error %v, want %s: %s", gotErr, want.Error.Name, want.Error.Message)
				}
			case gotErr != nil:
				problem = fmt.Sprintf("unexpected error %v", gotErr)
			case want != nil && want.Config != nil:
				problem = diff("config", gotConfig, p.object(*want.Config))
			case want != nil && want.Value != nil:
				if !hasValue {
					problem = "no value"
				} else {
					problem = diff("value", gotValue, p.value(*want.Value))
				}
			}
			if problem != "" {
				failed++
				if failed <= 15 {
					t.Errorf("script %d step %d %s(%s):\n  %s", si, i, op, step[1], problem)
				}
				break // the state has diverged
			}
		}
	}
	for _, c := range g.Evaluate {
		var want bool
		if err := json.Unmarshal(c[1], &want); err != nil {
			t.Fatal(err)
		}
		if got := Evaluate(decode(t, c[0])); got != want {
			t.Errorf("Evaluate(%s) = %v, want %v", c[0], got, want)
		}
	}
	t.Logf("%d scripts, %d steps (%d throw upstream), %d scripts failed", len(g.Scripts), steps, threw, failed)
}
