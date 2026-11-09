// Ported from the config functions of packages/mermaid/src/mermaidAPI.ts:
// initialize and processAndSetConfigs (mermaid 12.2.0).

package config

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
)

// Initialize is mermaidAPI.initialize(userOptions), which mermaid.initialize
// calls. userOptions is the site config; nil stands for no argument. It is
// not modified.
func (s *State) Initialize(userOptions *js.Obj[any]) error {
	s.detach()
	if userOptions == nil {
		userOptions = js.NewObj[any]()
	}
	options := assignToNew(userOptions)
	// Handle legacy location of font-family configuration
	themeVariables := options.Get("themeVariables")
	fontFamily, _ := js.GetProperty(themeVariables, "fontFamily")
	if js.Truthy(options.Get("fontFamily")) && !js.Truthy(fontFamily) {
		if !js.Truthy(themeVariables) {
			themeVariables = js.NewObj[any]()
			options.Set("themeVariables", themeVariables)
		}
		if err := assignProperty(themeVariables, "fontFamily", options.Get("fontFamily")); err != nil {
			return err
		}
	}

	// Set default options
	s.SaveConfigFromInitialize(options)

	// Stylesheets gate their palette rules on the theme *name*, so an
	// unrecognised name left in place loads a palette that is then never
	// rendered. Read the fallback from `defaultConfig` so the schema's
	// `theme.default` stays the one place it is written down.
	fallbackTheme := propertyKey(s.defaultConfig.Get("theme"))
	if js.Truthy(options.Get("theme")) && hasTheme(options.Get("theme")) {
		// Todo merge with user options
		vars, err := theme.GetThemeVariables(propertyKey(options.Get("theme")), options.Get("themeVariables"))
		if err != nil {
			return wrapThemeError(err)
		}
		options.Set("themeVariables", vars)
	} else {
		// Two values deliberately keep their name. `'null'` is the documented
		// sentinel for disabling the pre-defined themes, so normalising it
		// would re-enable one; and an absent theme needs no name written in,
		// because `defaultConfig` already supplies the same fallback.
		// Anything else is a name no theme answers to, and is corrected here.
		if t := options.Get("theme"); !js.IsNullish(t) && !js.StrictEquals(t, "null") {
			options.Set("theme", fallbackTheme)
		}
		vars, err := theme.GetThemeVariables(fallbackTheme, options.Get("themeVariables"))
		if err != nil {
			return wrapThemeError(err)
		}
		options.Set("themeVariables", vars)
	}

	_, err := s.SetSiteConfig(options)
	return err
}

// ProcessAndSetConfigs is processAndSetConfigs: it preprocesses the text,
// resets the config, enters the config scope of the detected diagram type and
// adds the config of the text as a directive.
//
// The second result is the detected diagram type, or "" when no detector
// matches. Upstream leaves the global defaults in charge then, and the parse
// step raises the real error.
func (s *State) ProcessAndSetConfigs(text string) (*Preprocessed, string, error) {
	processed, err := PreprocessDiagram(text)
	if err != nil {
		return nil, "", err
	}
	if err := s.Reset(); err != nil {
		return nil, "", err
	}
	// The config needs the diagram type to apply that type's appearance
	// defaults. Upstream passes a copy of the config to detectType. The only
	// detector that uses it writes to that copy, which nothing reads, so the
	// copy is left out here.
	diagramType, err := DetectType(processed.Code.Cleaned, nil)
	if err != nil {
		if err := s.LeaveDiagramConfigScope(); err != nil {
			return nil, "", err
		}
		diagramType = ""
	} else if err := s.SetDiagramConfigScope(diagramType); err != nil {
		return nil, "", err
	}
	if err := s.AddDirective(processed.Config); err != nil {
		return nil, "", err
	}
	return processed, diagramType, nil
}
