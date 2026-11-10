// Resolve combines two functions of packages/mermaid/src/mermaidAPI.ts
// (mermaid 12.2.0): initialize and processAndSetConfigs, in the order in which
// mermaid.initialize and mermaid.render run them.

package config

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// Resolved is what mermaid has computed for one diagram when
// mermaidAPI.render starts to parse it.
type Resolved struct {
	// Code is the cleaned diagram text that the parser receives
	// (processed.code.cleaned). It is equal to Source.Cleaned.
	Code string
	// Source is the DiagramCode object: the raw text, the cleaned text, the
	// text with comments, and the number of front matter lines. Diagrams that
	// report source positions need the last two.
	Source DiagramCode
	// Title is the title of the front matter, or "".
	Title string
	// DisplayMode is the displayMode of the front matter, or "". The config
	// also has it, as gantt.displayMode.
	DisplayMode string
	// DiagramType is the result of detectType, for example "flowchart-v2".
	// It is "" when no detector matches the text: upstream then renders with
	// the global defaults and fails in the parse step.
	DiagramType string
	// Config is the effective config: the value of getConfig() after
	// processAndSetConfigs, including themeVariables.
	Config *js.Obj[any]
	// Directive is the config of the front matter merged with the init
	// directive, after sanitization (the config that mermaid.parse returns).
	Directive *js.Obj[any]
	// State is the config state of this diagram. A diagram init function that
	// calls setConfig upstream calls State.SetConfig, and reads the result
	// with State.GetConfig.
	State *State
}

// Resolve runs mermaid's preprocessing and config resolution for one diagram:
// mermaid.initialize(site) on a freshly loaded mermaid, followed by the
// processAndSetConfigs step of mermaid.render(text).
//
// site is the site config, the argument of mermaid.initialize. It can be nil.
// The reference renderer passes { startOnLoad: false } merged with the
// user's config. site is not modified.
//
// The error is a *Error when upstream throws in these steps: for front
// matter that is not valid YAML, for a text that has an init directive and no
// known diagram type, and for a theme variable that is not a color.
func Resolve(text string, site *js.Obj[any]) (*Resolved, error) {
	return resolve(text, site, false)
}

func resolve(text string, site *js.Obj[any], includeZen bool) (*Resolved, error) {
	if !memo.Enabled() {
		return resolveStateWithDetector(text, site, includeZen, nil)
	}
	// The key of the site config. It is empty for the config that most
	// renders have: no site config.
	var enc keyEncoder
	enc.ok = true
	if site != nil {
		enc.value(site, 1)
		if !enc.ok {
			return resolveStateWithDetector(text, site, includeZen, nil)
		}
	}
	it, err := initTemplateFor(site, enc.buf)
	if err != nil {
		return nil, err
	}

	// processAndSetConfigs: the text, then the config steps.
	processed, err := preprocessDiagram(text, includeZen)
	if err != nil {
		return nil, err
	}
	diagramType, err := detect(processed.Code.Cleaned, nil, includeZen)
	hasType := err == nil
	if !hasType {
		diagramType = ""
	}

	// The key of the config steps: the site config, the config section of
	// the diagram type and the config of the text.
	enc.buf = append(enc.buf, 0)
	if hasType {
		enc.buf = append(enc.buf, 1)
		enc.str(GetDiagramConfigKey(diagramType))
	}
	enc.seen = enc.seen[:0]
	enc.value(processed.Config, 1)
	if !enc.ok {
		return resolveStateWithDetector(text, site, includeZen, processed)
	}
	t, ok := templates.GetBytes(enc.buf)
	if !ok {
		var plain bool
		// build freezes its directive. Keep the preprocessed config intact
		// so that a state fallback can reuse it without preprocessing again.
		directive := js.Clone(processed.Config).(*js.Obj[any])
		t, plain, err = it.build(diagramType, hasType, directive)
		if err != nil {
			return nil, err
		}
		if !plain {
			return resolveStateWithDetector(text, site, includeZen, processed)
		}
		templates.Put(string(enc.buf), t)
	}
	state := t.newState()
	return &Resolved{
		Code:        processed.Code.Cleaned,
		Source:      processed.Code,
		Title:       processed.Title,
		DisplayMode: processed.DisplayMode,
		DiagramType: diagramType,
		Config:      state.GetConfig(),
		Directive:   state.directives[0],
		State:       state,
	}, nil
}

// resolveState is Resolve by the functions of State alone, as upstream runs
// them: a new state for the render, initialize, processAndSetConfigs. Resolve
// uses it for a config that it does not keep as a template, and the tests
// compare it with Resolve.
func resolveState(text string, site *js.Obj[any]) (*Resolved, error) {
	return resolveStateWithDetector(text, site, false, nil)
}

func resolveStateWithDetector(text string, site *js.Obj[any], includeZen bool, processed *Preprocessed) (*Resolved, error) {
	state, err := newBareState()
	if err != nil {
		return nil, err
	}
	if err := state.Initialize(site); err != nil {
		return nil, err
	}
	processed, diagramType, err := state.processAndSetConfigs(text, includeZen, processed)
	if err != nil {
		return nil, err
	}
	return &Resolved{
		Code:        processed.Code.Cleaned,
		Source:      processed.Code,
		Title:       processed.Title,
		DisplayMode: processed.DisplayMode,
		DiagramType: diagramType,
		Config:      state.GetConfig(),
		Directive:   processed.Config,
		State:       state,
	}, nil
}
