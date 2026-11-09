// Ported from packages/mermaid/src/config.ts and
// diagram-api/diagramConfigKeys.ts (mermaid 12.2.0).

// Package config ports mermaid's configuration: the default config, the site
// config of mermaid.initialize, the directives and front matter of a diagram,
// and the per-diagram appearance defaults. It also ports the text
// preprocessing that produces those directives and the diagram type
// detection.
//
// Config objects are dynamic JavaScript values (see internal/js/value.go).
// Upstream merges, enumerates and deletes their properties with JavaScript
// semantics, and the result has to match in content and in key order.
//
// Upstream keeps the config in module-level variables. This port keeps them
// in a State, so that concurrent renders do not share anything.
package config

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
)

// Settings a diagram type may default, and a user may set, for one diagram
// type alone.
var appearanceKeys = []string{"theme", "look", "layout"}

// looks is kept in step with the schema's `look` enum.
var looks = map[string]bool{"classic": true, "handDrawn": true, "neo": true}

// Diagram types whose id differs from the MermaidConfig key their
// configuration lives under. Several share one section on purpose: they are
// renderers or parser versions of one diagram, so a setting made under
// `flowchart`, `class` or `state` must reach any of them.
var diagramConfigKeyAliases = map[string]string{
	"flowchart-v2":  "flowchart",
	"flowchart-elk": "flowchart",
	"classDiagram":  "class",
	"stateDiagram":  "state",
	"xychart":       "xyChart",
	"railroadAbnf":  "railroad",
	"railroadEbnf":  "railroad",
	"railroadPeg":   "railroad",
}

// GetDiagramConfigKey returns the MermaidConfig key holding the configuration
// for diagramType. The key is not guaranteed to exist -- `info` and `error`
// have no section -- so treat a missing one as "nothing configured for this
// type".
func GetDiagramConfigKey(diagramType string) string {
	if key, ok := diagramConfigKeyAliases[diagramType]; ok {
		return key
	}
	return diagramType
}

// propertyKey is ToPropertyKey: the string that a value becomes when it is
// used as a property name.
func propertyKey(v any) string { return js.ToString(v) }

// hasTheme is Object.hasOwn(theme, value).
func hasTheme(value any) bool { return theme.Has(propertyKey(value)) }

// isUsableAppearance reports whether an appearance value is one this build
// answers to. initialize() normalises an unknown top-level `theme`, so the
// diagram-scoped keys need the same guard or they would carry a name no
// stylesheet matches. `layout` is exempt: its registry is extensible, and
// getRegisteredLayoutAlgorithm falls back at render time.
func isUsableAppearance(key string, value any) bool {
	if key == "theme" {
		// `'null'` is the documented sentinel for disabling the pre-defined themes.
		return js.StrictEquals(value, "null") || hasTheme(value)
	}
	if key != "look" {
		return true
	}
	s, ok := value.(string)
	return ok && looks[s]
}

// readAppearance reads one appearance setting from one config layer,
// diagram-scoped value first. A value this build cannot render is passed over
// rather than winning, so a bogus `flowchart.theme` falls back to the `theme`
// set beside it before the next layer is tried.
func readAppearance(layer any, diagramConfigKey, key string) any {
	if !js.Truthy(layer) {
		return nil
	}
	section, _ := js.GetProperty(layer, diagramConfigKey)
	fromSection, _ := js.GetProperty(section, key)
	fromLayer, _ := js.GetProperty(layer, key)
	for _, value := range []any{fromSection, fromLayer} {
		if value != nil && isUsableAppearance(key, value) {
			return value
		}
	}
	return nil
}

// Evaluate converts a string or a boolean into a boolean (evaluate).
func Evaluate(val any) bool {
	if b, ok := val.(bool); ok && !b {
		return false
	}
	switch strings.ToLower(js.TrimSpace(js.ToString(val))) {
	case "false", "null", "0":
		return false
	}
	return true
}

// State holds the variables that config.ts keeps at module level. The zero
// value is not usable; call NewState.
type State struct {
	defaultConfig *js.Obj[any]
	siteConfig    *js.Obj[any]
	// configFromInitialize is undefined until Initialize runs.
	configFromInitialize any
	// siteConfigDelta is what the user handed to SetSiteConfig, unmerged with
	// the defaults so the appearance resolution can tell a user's value from
	// the schema's.
	siteConfigDelta *js.Obj[any]
	directives      []*js.Obj[any]
	currentConfig   *js.Obj[any]
	// diagramConfigKey is the config section of the diagram in scope. It is
	// unset when hasDiagramConfigKey is false, which leaves the global
	// defaults in charge.
	diagramConfigKey    string
	hasDiagramConfigKey bool

	// tpl is set for a state that Resolve made from a template: the configs
	// of the state are then the frozen configs of the template, and the
	// state makes its own copy of what it hands out. See template.go.
	tpl *template
	// owned has the copies that the state made of frozen objects.
	owned []ownedObj
}

// NewState returns the config state of a freshly loaded mermaid module:
// the site config and the current config are copies of the default config.
func NewState() (*State, error) {
	s, err := newBareState()
	if err != nil {
		return nil, err
	}
	s.siteConfig = assignToNew(s.defaultConfig)
	s.currentConfig = assignToNew(s.defaultConfig)
	// mermaidAPI.ts ends with configApi.reset(configApi.getConfig()).
	if err := s.ResetTo(s.GetConfig()); err != nil {
		return nil, err
	}
	return s, nil
}

// newBareState returns a state that has the default config and no site
// config and no current config yet. Initialize sets both before anything
// reads them, so Resolve starts from this state and skips the copies that
// NewState makes.
func newBareState() (*State, error) {
	shared, err := defaults()
	if err != nil {
		return nil, err
	}
	// Each state owns a copy of the default config. Upstream copies only the
	// upper levels of a config and shares the deeper objects, so a private
	// copy keeps one state from reaching another through the defaults.
	return &State{
		defaultConfig:   js.Clone(shared).(*js.Obj[any]),
		siteConfigDelta: js.NewObj[any](),
	}, nil
}

// DefaultConfig returns the default config of this state (defaultConfig).
// The caller must not modify it.
func (s *State) DefaultConfig() *js.Obj[any] {
	if s.tpl != nil {
		// The copy of this state of the frozen defaults. It is made on the
		// first call, and the state finds it again.
		return s.own(s.defaultConfig).(*js.Obj[any])
	}
	return s.defaultConfig
}

// resolveAppearance resolves `theme`, `look` and `layout` onto the top level
// of cfg, where renderers read them. Highest first: frontmatter/directive,
// initialize(), diagram default, global default.
func (s *State) resolveAppearance(cfg, sumOfDirectives *js.Obj[any]) {
	if !s.hasDiagramConfigKey || s.diagramConfigKey == "" {
		return
	}
	layers := []any{sumOfDirectives}
	// setConfig passes only its own object as the directive list, so consult
	// the real directives too, or a diagram default wins back over
	// frontmatter. Later ones win.
	for i := len(s.directives) - 1; i >= 0; i-- {
		layers = append(layers, s.directives[i])
	}
	layers = append(layers, s.siteConfigDelta, s.defaultConfig)

	section := cfg.Get(s.diagramConfigKey)
	for _, key := range appearanceKeys {
		for _, layer := range layers {
			value := readAppearance(layer, s.diagramConfigKey, key)
			if value == nil {
				continue
			}
			if s.tpl != nil {
				// A value of a frozen layer: the config gets the copy of
				// this state, or a new array.
				switch x := value.(type) {
				case *js.Obj[any]:
					value = s.own(x)
				case []any:
					value = append([]any(nil), x...)
				}
			}
			cfg.Set(key, value)
			// Keep the section in step, so getConfig().flowchart.look cannot
			// contradict .look.
			if current, _ := js.GetProperty(section, key); current != nil {
				setProperty(section, key, value)
			}
			break
		}
	}
}

func (s *State) updateCurrentConfig(siteCfg *js.Obj[any], directives []*js.Obj[any]) (*js.Obj[any], error) {
	// start with config being the siteConfig
	cfg := s.copyConfig(siteCfg)

	// Join directives
	sumOfDirectives := js.NewObj[any]()
	for i, d := range directives {
		if err := s.Sanitize(d); err != nil {
			return nil, err
		}
		// Apply the data from the directive where the overrides the themeVariables
		if i == 0 && d != nil {
			// The sum is still empty: assignWithDepth({}, d).
			sumOfDirectives = assignToNew(d)
			continue
		}
		assignWithDepth(sumOfDirectives, d)
	}

	assignWithDepth(cfg, sumOfDirectives)

	s.resolveAppearance(cfg, sumOfDirectives)

	// `cfg.themeVariables` were built from `siteCfg.theme`, and stylesheets
	// gate their rules on the theme *name*, so a resolved theme of another
	// name needs them rebuilt.
	themeWasOverridden := js.Truthy(sumOfDirectives.Get("theme")) || !js.StrictEquals(cfg.Get("theme"), siteCfg.Get("theme"))
	if themeWasOverridden && js.Truthy(cfg.Get("theme")) && hasTheme(cfg.Get("theme")) {
		// Only `configFromInitialize` holds the variables as the user wrote
		// them; the site config's are already derived, and feeding those back
		// would override the new theme.
		tmpConfigFromInitialize := s.copyConfig(s.configFromInitialize)
		var base any = tmpConfigFromInitialize.Get("themeVariables")
		if !js.Truthy(base) {
			base = js.NewObj[any]()
		}
		themeVariables := assignWithDepth(base, sumOfDirectives.Get("themeVariables"))
		vars, err := theme.GetThemeVariables(propertyKey(cfg.Get("theme")), themeVariables)
		if err != nil {
			return nil, wrapThemeError(err)
		}
		cfg.Set("themeVariables", vars)
	}

	s.currentConfig = cfg
	return s.currentConfig, nil
}

// wrapThemeError turns an exception of the theme code into an Error.
func wrapThemeError(err error) error {
	if te, ok := err.(*theme.Error); ok {
		name := "Error"
		if te.TypeError {
			name = "TypeError"
		}
		return &Error{Name: name, Message: te.Message, Err: err}
	}
	return err
}

// DefaultSection returns one value of the default config of this state:
// DefaultConfig().Get(key), without the copy of the other sections that a
// state from a template makes for DefaultConfig. The caller must not modify
// it.
func (s *State) DefaultSection(key string) any {
	if s.tpl == nil {
		return s.defaultConfig.Get(key)
	}
	value := s.defaultConfig.Get(key)
	if _, isArray := value.([]any); isArray {
		// The array of the copy of the defaults, which is the same array
		// in every call.
		return s.DefaultConfig().Get(key)
	}
	return s.own(value)
}

// SetDiagramConfigScope names the diagram type being parsed or rendered, so
// its own appearance defaults apply (setDiagramConfigScope). diagramType is
// the type that DetectType returned, for example "flowchart-v2".
func (s *State) SetDiagramConfigScope(diagramType string) error {
	s.detach()
	s.diagramConfigKey = GetDiagramConfigKey(diagramType)
	s.hasDiagramConfigKey = true
	_, err := s.updateCurrentConfig(s.siteConfig, s.directives)
	return err
}

// LeaveDiagramConfigScope is setDiagramConfigScope(undefined).
func (s *State) LeaveDiagramConfigScope() error {
	s.detach()
	s.diagramConfigKey = ""
	s.hasDiagramConfigKey = false
	_, err := s.updateCurrentConfig(s.siteConfig, s.directives)
	return err
}

// SetSiteConfig sets the siteConfig to the desired values (setSiteConfig).
//
// The siteConfig is a protected configuration for repeat use. Calls to Reset
// reset the currentConfig to siteConfig. conf is merged with the default
// config. The result is the new siteConfig.
func (s *State) SetSiteConfig(conf *js.Obj[any]) (*js.Obj[any], error) {
	s.detach()
	s.siteConfig = assignToNew(s.defaultConfig)
	assignWithDepth(s.siteConfig, conf)
	s.siteConfigDelta = assignToNew(conf)

	if js.Truthy(conf.Get("theme")) && hasTheme(conf.Get("theme")) {
		vars, err := theme.GetThemeVariables(propertyKey(conf.Get("theme")), conf.Get("themeVariables"))
		if err != nil {
			return nil, wrapThemeError(err)
		}
		s.siteConfig.Set("themeVariables", vars)
	}

	if _, err := s.updateCurrentConfig(s.siteConfig, s.directives); err != nil {
		return nil, err
	}
	return s.siteConfig, nil
}

// SaveConfigFromInitialize is saveConfigFromInitialize.
func (s *State) SaveConfigFromInitialize(conf *js.Obj[any]) {
	s.detach()
	s.configFromInitialize = assignToNew(conf)
}

// UpdateSiteConfig is updateSiteConfig.
func (s *State) UpdateSiteConfig(conf *js.Obj[any]) (*js.Obj[any], error) {
	s.detach()
	assignWithDepth(s.siteConfig, conf)
	assignWithDepth(s.siteConfigDelta, conf)
	if _, err := s.updateCurrentConfig(s.siteConfig, s.directives); err != nil {
		return nil, err
	}
	return s.siteConfig, nil
}

// GetSiteConfig returns a copy of the current siteConfig base configuration
// (getSiteConfig).
func (s *State) GetSiteConfig() *js.Obj[any] {
	s.detach()
	return assignToNew(s.siteConfig)
}

// SetConfig updates the currentConfig with conf after sanitization
// (setConfig). Upstream marks it deprecated: the next call to AddDirective or
// Reset overwrites the change. Diagram init functions still call it.
func (s *State) SetConfig(conf *js.Obj[any]) (*js.Obj[any], error) {
	if _, err := s.updateCurrentConfig(s.currentConfig, []*js.Obj[any]{conf}); err != nil {
		return nil, err
	}
	return s.GetConfig(), nil
}

// GetConfig returns a copy of the currentConfig (getConfig).
//
// The copy is as deep as upstream makes it: the top level and two levels
// below it are new objects, and deeper values are shared with the state.
func (s *State) GetConfig() *js.Obj[any] {
	return s.copyConfig(s.currentConfig)
}

// secureKeys is ['secure', ...(siteConfig.secure ?? [])].
func (s *State) secureKeys() ([]string, error) {
	keys := []string{"secure"}
	switch secure := s.siteConfig.Get("secure").(type) {
	case []any:
		for _, k := range secure {
			keys = append(keys, propertyKey(k))
		}
	case string:
		// Spreading a string yields its characters.
		for _, r := range secure {
			keys = append(keys, string(r))
		}
	default:
		if !js.IsNullish(secure) {
			return nil, typeError("siteConfig.secure is not iterable")
		}
	}
	return keys, nil
}

// Sanitize ensures options does not attempt to override siteConfig secure
// keys (sanitize). It modifies options in place.
func (s *State) Sanitize(options any) error {
	if !js.Truthy(options) || isNilObj(options) {
		return nil
	}
	// Checking that options are not in the list of excluded options
	secure, err := s.secureKeys()
	if err != nil {
		return err
	}
	for _, key := range secure {
		if js.HasOwn(options, key) {
			deleteProperty(options, key)
		}
	}

	// Check that there no attempts of prototype pollution
	keys, _ := js.OwnKeys(options)
	for _, key := range keys {
		if strings.HasPrefix(key, "__") {
			deleteProperty(options, key)
		}
	}
	// Check that there no attempts of xss, there should be no tags at all in
	// the directive blocking data urls as base64 urls can contain svg's with
	// inline script tags
	keys, _ = js.OwnKeys(options)
	for _, key := range keys {
		value, _ := js.GetProperty(options, key)
		if str, ok := value.(string); ok &&
			(strings.Contains(str, "<") || strings.Contains(str, ">") || strings.Contains(str, "url(data:")) {
			if _, isString := options.(string); isString {
				return typeError("Cannot delete property '%s' of [object String]", key)
			}
			deleteProperty(options, key)
		}
		value, _ = js.GetProperty(options, key)
		if typeofObject(value) {
			if err := s.Sanitize(value); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddDirective pushes in a directive to the configuration (addDirective).
// It sanitizes directive in place.
func (s *State) AddDirective(directive *js.Obj[any]) error {
	s.detach()
	if err := SanitizeDirective(directive); err != nil {
		return err
	}

	// If the directive has a fontFamily, but no themeVariables, add the
	// fontFamily to the themeVariables
	themeVariables := directive.Get("themeVariables")
	fontFamily, _ := js.GetProperty(themeVariables, "fontFamily")
	if js.Truthy(directive.Get("fontFamily")) && !js.Truthy(fontFamily) {
		// { ...directive.themeVariables, fontFamily: directive.fontFamily }
		spread := js.NewObj[any]()
		objectAssign(spread, themeVariables)
		spread.Set("fontFamily", directive.Get("fontFamily"))
		directive.Set("themeVariables", spread)
	}

	s.directives = append(s.directives, directive)
	_, err := s.updateCurrentConfig(s.siteConfig, s.directives)
	return err
}

// Reset resets the current config and the applied directives to the
// siteConfig (reset with no argument).
func (s *State) Reset() error { return s.ResetTo(s.siteConfig) }

// ResetTo resets the current config and the applied directives to config
// (reset).
func (s *State) ResetTo(config *js.Obj[any]) error {
	s.detach()
	// Replace current config with siteConfig
	s.directives = nil
	// Leave diagram scope too, or a stale type keeps applying its appearance
	// defaults.
	s.diagramConfigKey = ""
	s.hasDiagramConfigKey = false
	_, err := s.updateCurrentConfig(config, s.directives)
	return err
}

// GetUserDefinedConfig is getUserDefinedConfig: the config of initialize
// merged with the directives, without the defaults.
func (s *State) GetUserDefinedConfig() *js.Obj[any] {
	userConfig := js.NewObj[any]()
	if js.Truthy(s.configFromInitialize) {
		assignWithDepth(userConfig, s.configFromInitialize)
	}
	for _, d := range s.directives {
		assignWithDepth(userConfig, d)
	}
	if s.tpl != nil {
		// The copy shares its deep values with the frozen config of
		// initialize.
		s.ownValues(userConfig)
	}
	return userConfig
}

// GetEffectiveHtmlLabels handles the deprecated flowchart.htmlLabels
// (getEffectiveHtmlLabels). config is the merged config with defaults. The
// precedence is: root htmlLabels, flowchart.htmlLabels, true.
func GetEffectiveHtmlLabels(config *js.Obj[any]) bool {
	value := config.Get("htmlLabels")
	if js.IsNullish(value) {
		value, _ = js.GetProperty(config.Get("flowchart"), "htmlLabels")
	}
	if js.IsNullish(value) {
		value = true
	}
	return Evaluate(value)
}
