// This file has no upstream source. It keeps the result of the config
// resolution of mermaidAPI.ts (initialize and processAndSetConfigs) for the
// renders that have the same site config and the same config in the diagram
// text.

package config

import (
	"encoding/binary"
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// A template is the config state of mermaid at one point of the resolution,
// as frozen objects. The functions of State compute it one time, with the
// code that ports config.ts, and later renders with the same input start
// from it.
//
// The rule that keeps a render independent of the others: a frozen object
// never reaches code that could change it.
//
//   - State reads the frozen objects and never writes to them. A write
//     would panic (see js.Obj.Freeze).
//   - What a State hands out is a copy: getConfig copies three levels, as
//     upstream does. Upstream shares the values below those levels between
//     the copy and the state. Here such a value of a template is replaced by
//     a copy that belongs to the State, the same copy in every place that
//     hands it out (see State.own), so the sharing inside one render is the
//     sharing of upstream.
//   - An array cannot be frozen. A template is used only when its arrays are
//     in the places where every copy makes a new array (see deepPaths and
//     appendValueKey). Other configs are resolved without the cache.

// initTemplate is the state after mermaid.initialize(site).
type initTemplate struct {
	siteConfig           *js.Obj[any]
	siteConfigDelta      *js.Obj[any]
	configFromInitialize any
	// plain is false when a config of the state has an array in a place
	// where a copy would share it. See deepPaths.
	plain bool
}

// template is the state after processAndSetConfigs for one diagram text.
type template struct {
	init *initTemplate
	// directive is the config of the diagram text after sanitization.
	directive *js.Obj[any]
	// currentConfig is the config that getConfig copies.
	currentConfig *js.Obj[any]
	// currentDeep lists the objects that a copy of currentConfig shares
	// with it.
	currentDeep         []deepPath
	diagramConfigKey    string
	hasDiagramConfigKey bool
}

// The caches. A site config is a small set in a service, and the config of a
// diagram text is the empty config for most diagrams. A template has the
// values of a config (about 1000) and shares the keys with the defaults.
var (
	initTemplates = memo.New[*initTemplate](32)
	templates     = memo.New[*template](256)
)

// deepPath is the place of an object below the levels that assignToNew
// copies: config[0][1][2].
type deepPath [3]string

// deepPaths lists the objects that a copy of a config by assignToNew shares
// with the config: the object values of the objects of the third level.
//
// The second result is false when the copy would also share an array, or an
// object that is an element of an array. A State cannot keep such a value
// apart for each render, so the config is then not used as a template.
func deepPaths(root *js.Obj[any]) (paths []deepPath, ok bool) {
	ok = true
	plainArray := func(arr []any) {
		for _, e := range arr {
			if js.IsObject(e) {
				ok = false
			}
		}
	}
	root.Each(func(k1 string, v1 any) {
		switch x1 := v1.(type) {
		case []any:
			plainArray(x1)
		case *js.Obj[any]:
			if x1 == nil {
				return
			}
			x1.Each(func(k2 string, v2 any) {
				switch x2 := v2.(type) {
				case []any:
					plainArray(x2)
				case *js.Obj[any]:
					if x2 == nil {
						return
					}
					x2.Each(func(k3 string, v3 any) {
						switch x3 := v3.(type) {
						case []any:
							ok = false
						case *js.Obj[any]:
							if x3 != nil {
								paths = append(paths, deepPath{k1, k2, k3})
							}
						}
					})
				}
			})
		}
	})
	return paths, ok
}

// keyEncoder writes the cache key of a dynamic value.
type keyEncoder struct {
	buf []byte
	// seen has the objects of the value, to find an object that the value
	// has two times.
	seen []*js.Obj[any]
	ok   bool
}

// maxKeyDepth bounds the depth of a config that has a cache key.
const maxKeyDepth = 32

// value appends the exact encoding of a dynamic value: two values with the
// same encoding have the same types, the same numbers bit for bit, the same
// strings and the same keys in the same order.
//
// level is the level of the value in the config: the config is level 1, its
// values are level 2. e.ok becomes false for a value that must not be used
// with the cache:
//
//   - a value of a type that is not a dynamic value;
//   - an array that a copy of the config would share: an array below the
//     third level, or an array that holds an object or an array;
//   - an object that the value has in two places, because a copy of a
//     template would not keep that;
//   - a value that is too deep.
func (e *keyEncoder) value(v any, level int) {
	switch x := v.(type) {
	case nil:
		e.buf = append(e.buf, 'u')
	case bool:
		if x {
			e.buf = append(e.buf, 't')
		} else {
			e.buf = append(e.buf, 'f')
		}
	case float64:
		e.buf = append(e.buf, 'd')
		e.buf = binary.LittleEndian.AppendUint64(e.buf, math.Float64bits(x))
	case int:
		e.buf = append(e.buf, 'i')
		e.buf = binary.LittleEndian.AppendUint64(e.buf, uint64(x))
	case string:
		e.buf = append(e.buf, 's')
		e.str(x)
	case []any:
		if level < 2 || level > 3 {
			e.ok = false
			return
		}
		e.buf = append(e.buf, 'a')
		e.buf = binary.AppendUvarint(e.buf, uint64(len(x)))
		for _, el := range x {
			if js.IsObject(el) {
				e.ok = false
				return
			}
			e.value(el, level+1)
		}
	case *js.Obj[any]:
		if x == nil {
			e.buf = append(e.buf, 'z')
			return
		}
		if level > maxKeyDepth {
			e.ok = false
			return
		}
		for _, s := range e.seen {
			if s == x {
				e.ok = false
				return
			}
		}
		e.seen = append(e.seen, x)
		e.buf = append(e.buf, 'o')
		e.buf = binary.AppendUvarint(e.buf, uint64(x.Len()))
		x.Each(func(k string, el any) {
			e.str(k)
			e.value(el, level+1)
		})
	default:
		if isNull(v) {
			e.buf = append(e.buf, 'n')
			return
		}
		e.ok = false
	}
}

func (e *keyEncoder) str(s string) {
	e.buf = binary.AppendUvarint(e.buf, uint64(len(s)))
	e.buf = append(e.buf, s...)
}

// initTemplateFor returns the state after mermaid.initialize(site), from the
// cache or by State.Initialize. siteKey is the cache key of site.
func initTemplateFor(site *js.Obj[any], siteKey []byte) (*initTemplate, error) {
	if t, ok := initTemplates.GetBytes(siteKey); ok {
		return t, nil
	}
	shared, err := defaults()
	if err != nil {
		return nil, err
	}
	// Initialize copies the upper levels of the site config and keeps its
	// deeper objects. The template must not keep an object of the caller,
	// so it starts from a copy.
	if site != nil {
		site = js.Clone(site).(*js.Obj[any])
	}
	// The state reads the frozen defaults: it has no copy of them.
	s := &State{defaultConfig: shared, siteConfigDelta: js.NewObj[any]()}
	if err := s.Initialize(site); err != nil {
		return nil, err
	}
	t := &initTemplate{
		siteConfig:           s.siteConfig,
		siteConfigDelta:      s.siteConfigDelta,
		configFromInitialize: s.configFromInitialize,
	}
	js.FreezeValue(t.siteConfig)
	js.FreezeValue(t.siteConfigDelta)
	js.FreezeValue(t.configFromInitialize)
	t.plain = true
	for _, v := range []any{t.siteConfig, t.siteConfigDelta, t.configFromInitialize} {
		if o, isObj := v.(*js.Obj[any]); isObj && o != nil {
			if _, ok := deepPaths(o); !ok {
				t.plain = false
			}
		}
	}
	initTemplates.Put(string(siteKey), t)
	return t, nil
}

// build runs the config steps of processAndSetConfigs after the text is
// preprocessed, on the state after initialize: reset, the config scope of
// the diagram type, and the directive. The second result is false for a
// config that cannot be a template: see deepPaths.
//
// directive is the config of the diagram text. build sanitizes it and
// freezes it.
func (it *initTemplate) build(diagramType string, hasType bool, directive *js.Obj[any]) (*template, bool, error) {
	shared, err := defaults()
	if err != nil {
		return nil, false, err
	}
	s := &State{
		defaultConfig:        shared,
		siteConfig:           it.siteConfig,
		siteConfigDelta:      it.siteConfigDelta,
		configFromInitialize: it.configFromInitialize,
	}
	if err := s.Reset(); err != nil {
		return nil, false, err
	}
	if hasType {
		err = s.SetDiagramConfigScope(diagramType)
	} else {
		err = s.LeaveDiagramConfigScope()
	}
	if err != nil {
		return nil, false, err
	}
	if err := s.AddDirective(directive); err != nil {
		return nil, false, err
	}
	t := &template{
		init:                it,
		directive:           directive,
		currentConfig:       s.currentConfig,
		diagramConfigKey:    s.diagramConfigKey,
		hasDiagramConfigKey: s.hasDiagramConfigKey,
	}
	js.FreezeValue(t.directive)
	js.FreezeValue(t.currentConfig)
	_, plainDirective := deepPaths(t.directive)
	var plain bool
	t.currentDeep, plain = deepPaths(t.currentConfig)
	return t, plain && plainDirective && it.plain, nil
}

// newState returns the config state of one render that starts from the
// template.
func (t *template) newState() *State {
	shared, _ := defaults()
	s := &State{
		tpl:                  t,
		defaultConfig:        shared,
		siteConfig:           t.init.siteConfig,
		siteConfigDelta:      t.init.siteConfigDelta,
		configFromInitialize: t.init.configFromInitialize,
		currentConfig:        t.currentConfig,
		diagramConfigKey:     t.diagramConfigKey,
		hasDiagramConfigKey:  t.hasDiagramConfigKey,
	}
	// The directive is the one object of the state that upstream hands to
	// the caller as it is (the config that mermaid.parse returns). The
	// state has its own copy.
	s.directives = []*js.Obj[any]{s.ownRoot(t.directive)}
	return s
}

// ownedObj is a frozen object of the template and its copy in the state.
type ownedObj struct{ frozen, own *js.Obj[any] }

// own returns the value that the state uses in place of a value of its
// template: for a frozen object, a copy that belongs to the state. The copy
// is deep, and the state makes one copy of an object, so two places that
// share an object in the template share the copy. A value that is not
// frozen already belongs to the state.
func (s *State) own(v any) any {
	switch x := v.(type) {
	case *js.Obj[any]:
		if x == nil || !x.Frozen() {
			return v
		}
		// A state has few such objects: one for the usual config.
		for _, o := range s.owned {
			if o.frozen == x {
				return o.own
			}
		}
		c := x.Copy()
		s.owned = append(s.owned, ownedObj{x, c})
		s.ownCopy(c)
		return c
	case []any:
		// An array of a frozen object: a new array for the state.
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = s.own(e)
		}
		return out
	}
	return v
}

// ownCopy makes the values of a new copy of a frozen object values of the
// state.
func (s *State) ownCopy(c *js.Obj[any]) {
	js.ReplaceObjects(c, s.own)
}

// ownRoot is own for a frozen object that no other object of the template
// refers to: the state does not need to find the copy again.
func (s *State) ownRoot(x *js.Obj[any]) *js.Obj[any] {
	c := x.Copy()
	if c.Len() > 0 {
		s.ownCopy(c)
	}
	return c
}

// ownValues replaces the template values of an object that the state made
// by the values of the state, at every depth.
func (s *State) ownValues(o *js.Obj[any]) {
	js.ReplaceObjects(o, s.ownValue)
}

// ownValue is the value of the state for a value inside an object that the
// state made: an object of the template becomes the copy of the state, and
// an object or an array that the state made keeps its identity.
func (s *State) ownValue(v any) any {
	switch x := v.(type) {
	case *js.Obj[any]:
		if x == nil {
			return v
		}
		if x.Frozen() {
			return s.own(x)
		}
		s.ownValues(x)
	case []any:
		for i, e := range x {
			if js.IsObject(e) {
				x[i] = s.ownValue(e)
			}
		}
	}
	return v
}

// copyCurrent is assignToNew(currentConfig) for the config of the template:
// the copy that getConfig returns. The objects that the copy shares with the
// template become objects of the state.
func (s *State) copyCurrent() *js.Obj[any] {
	t := s.tpl
	cfg := assignToNew(t.currentConfig)
	s.ownPaths(cfg, t.currentDeep)
	return cfg
}

// ownPaths makes the objects at the paths of a config objects of the state.
func (s *State) ownPaths(cfg *js.Obj[any], paths []deepPath) {
	for _, p := range paths {
		if third := js.AsObj(js.Path(cfg, p[0], p[1])); third != nil {
			third.Set(p[2], s.own(third.Get(p[2])))
		}
	}
}

// copyConfig is assignToNew(src) for a config that the state reads. For a
// frozen config of the template, the objects that the copy shares with it
// become objects of the state.
func (s *State) copyConfig(src any) *js.Obj[any] {
	if s.tpl == nil {
		return assignToNew(src)
	}
	if src == any(s.tpl.currentConfig) {
		return s.copyCurrent()
	}
	cfg := assignToNew(src)
	if o, ok := src.(*js.Obj[any]); ok && o != nil && o.Frozen() {
		paths, _ := deepPaths(o)
		s.ownPaths(cfg, paths)
	}
	return cfg
}

// detach turns the state into a state that owns all of its objects, as
// NewState makes it. The functions of State that upstream does not call
// during a render use it, so that they run on objects that they can change.
func (s *State) detach() {
	if s.tpl == nil {
		return
	}
	s.defaultConfig = s.own(s.defaultConfig).(*js.Obj[any])
	s.siteConfig = s.own(s.siteConfig).(*js.Obj[any])
	s.siteConfigDelta = s.own(s.siteConfigDelta).(*js.Obj[any])
	s.configFromInitialize = s.own(s.configFromInitialize)
	s.currentConfig = s.own(s.currentConfig).(*js.Obj[any])
	s.tpl = nil
}
