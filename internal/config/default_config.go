// Ported from packages/mermaid/src/defaultConfig.ts (mermaid 12.2.0).

package config

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/theme"
)

// defaultConfigJSON is the default mermaid configuration: the default values
// of schemas/config.schema.yaml plus the values that defaultConfig.ts adds.
// tools/reference/gen-config.mjs dumps it from the built mermaid.
//
// Three markers stand for values that JSON cannot hold:
//
//	{"$u":1}               undefined. The key exists, so that configKeys finds it.
//	{"$f":1}               a function, for example sequence.messageFont.
//	{"$theme":"default"}   theme.default.getThemeVariables(), computed in Go.
//
//go:embed default_config.json
var defaultConfigJSON []byte

var (
	defaultOnce sync.Once
	defaultErr  error
	// defaultTree is the shared default config. It is frozen, so that every
	// render can read it: a write to it panics. A user that changes the
	// defaults takes a copy first.
	defaultTree *js.Obj[any]
	// configKeySet is configKeys in defaultConfig.ts: every key of the default
	// config at any depth, except keys that hold an array.
	configKeySet map[string]struct{}
)

func loadDefaults() {
	parsed, err := js.ParseJSON(defaultConfigJSON)
	if err != nil {
		defaultErr = fmt.Errorf("config: default_config.json: %w", err)
		return
	}
	root, ok := parsed.(*js.Obj[any])
	if !ok {
		defaultErr = fmt.Errorf("config: default_config.json is not an object")
		return
	}
	themeVariables, err := theme.GetThemeVariables("default", nil)
	if err != nil {
		defaultErr = fmt.Errorf("config: default theme: %w", err)
		return
	}
	configKeySet = map[string]struct{}{}
	defaultTree = buildDefault(root, themeVariables).(*js.Obj[any])
	js.FreezeValue(defaultTree)
}

// marker returns the name of the marker that v is, or "".
func marker(v any) string {
	o, ok := v.(*js.Obj[any])
	if !ok || o.Len() != 1 {
		return ""
	}
	for _, name := range []string{"$u", "$f", "$theme"} {
		if o.Has(name) {
			return name
		}
	}
	return ""
}

// buildDefault turns the embedded data into the config tree and collects
// configKeys on the way. It follows keyify in defaultConfig.ts: a key that
// holds an array is not a config key, and a key that holds an object adds its
// own name and the keys below it.
func buildDefault(v any, themeVariables *js.Obj[any]) any {
	o, ok := v.(*js.Obj[any])
	if !ok {
		return v
	}
	out := js.NewObj[any]()
	for _, k := range o.Keys() {
		value := o.Get(k)
		switch marker(value) {
		case "$u":
			configKeySet[k] = struct{}{}
			out.Set(k, nil)
		case "$f":
			// A function-valued property is a config key, but the value model
			// has no functions. The renderers compute these font getters.
			configKeySet[k] = struct{}{}
		case "$theme":
			configKeySet[k] = struct{}{}
			keyify(themeVariables)
			out.Set(k, themeVariables)
		default:
			if js.IsArray(value) {
				out.Set(k, value)
				continue
			}
			configKeySet[k] = struct{}{}
			out.Set(k, buildDefault(value, themeVariables))
		}
	}
	return out
}

// keyify adds the keys of an object and of the objects below it to configKeys.
func keyify(o *js.Obj[any]) {
	for _, k := range o.Keys() {
		value := o.Get(k)
		if js.IsArray(value) {
			continue
		}
		configKeySet[k] = struct{}{}
		if child, ok := value.(*js.Obj[any]); ok && child != nil {
			keyify(child)
		}
	}
}

// defaults returns the shared default config. The caller must not modify it.
func defaults() (*js.Obj[any], error) {
	defaultOnce.Do(loadDefaults)
	return defaultTree, defaultErr
}

// Default returns a copy of mermaid's default configuration (defaultConfig).
// Properties that are undefined upstream exist with a nil value. Properties
// that are functions upstream (the font getters of sequence and c4) are
// absent.
func Default() *js.Obj[any] {
	d, err := defaults()
	if err != nil {
		// The embedded data is fixed at build time, so this cannot happen in
		// a build that passes its tests.
		panic(err)
	}
	return js.Clone(d).(*js.Obj[any])
}

// DefaultSection returns a copy of one value of mermaid's default
// configuration: defaultConfig[key]. For a diagram that reads its own
// section of the defaults, it is Default().Get(key) without the copy of the
// other sections.
func DefaultSection(key string) any {
	d, err := defaults()
	if err != nil {
		panic(err)
	}
	return js.Clone(d.Get(key))
}

// IsConfigKey reports whether key is in configKeys: sanitizeDirective deletes
// every other key from a directive.
func IsConfigKey(key string) bool {
	if _, err := defaults(); err != nil {
		return false
	}
	_, ok := configKeySet[key]
	return ok
}
