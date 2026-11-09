// Ported from LayoutMetaDataService.getOptionDataBySuffix and
// LayoutOptionData.parseValue in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/data of Eclipse ELK
// 0.9.1 (EPL-2.0), as compiled into elk-worker.js of elkjs 0.9.3.

package elkgraph

import (
	"errors"
	"reflect"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts" // declares the core options
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts" // declares the layered options
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// ErrUnsupported is the error of an input that the native importer does not
// handle. The caller lays the graph out with elkjs.
var ErrUnsupported = errors.New("elkgraph: unsupported input")

func unsupported(reason string) error {
	return &UnsupportedError{Reason: reason}
}

// UnsupportedError tells why the native path does not handle a graph.
type UnsupportedError struct {
	Reason string
}

func (e *UnsupportedError) Error() string { return "elkgraph: unsupported input: " + e.Reason }

// Is makes errors.Is(err, ErrUnsupported) true.
func (e *UnsupportedError) Is(target error) bool { return target == ErrUnsupported }

// Option is one layout option that elkjs registers.
type Option struct {
	Meta *core.OptionMeta
	// Prop is a Go property with the id of the option, or nil when no ported
	// code reads the option.
	Prop prop.Any
}

var (
	optionsOnce sync.Once
	// bySuffix maps a suffix of an option id to the option. A suffix that
	// ends more than one id maps to nil.
	bySuffix map[string]*Option
	// byLegacySuffix is the same for the legacy ids.
	byLegacySuffix map[string]*Option
)

// suffixes returns each suffix of id that starts at the start or after a
// dot.
func suffixes(id string) []string {
	out := []string{id}
	for i := 0; i < len(id); i++ {
		if id[i] == '.' && i+1 < len(id) {
			out = append(out, id[i+1:])
		}
	}
	return out
}

func buildOptions() {
	bySuffix = map[string]*Option{}
	byLegacySuffix = map[string]*Option{}
	add := func(m map[string]*Option, id string, o *Option) {
		for _, s := range suffixes(id) {
			if _, dup := m[s]; dup {
				m[s] = nil
				continue
			}
			m[s] = o
		}
	}
	for i := range core.Options {
		meta := &core.Options[i]
		o := &Option{Meta: meta}
		if decl := prop.Declared(meta.ID); len(decl) > 0 {
			o.Prop = decl[0]
		}
		add(bySuffix, meta.ID, o)
		for _, legacy := range meta.Legacy {
			add(byLegacySuffix, legacy, o)
		}
	}
}

// OptionBySuffix is LayoutMetaDataService.getOptionDataBySuffix: the option
// whose id is the suffix or ends with a dot and the suffix. It returns nil
// when no option or more than one option matches.
func OptionBySuffix(suffix string) *Option {
	optionsOnce.Do(buildOptions)
	if suffix == "" {
		return nil
	}
	if o, found := bySuffix[suffix]; found {
		return o
	}
	return byLegacySuffix[suffix]
}

// isASCII tells that s has only ASCII characters. The regular expressions of
// the option parser treat other white space like JavaScript does, which this
// port does not copy.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// enumForString is LayoutOptionData.enumForString: the constant with the
// name, or the constant with the ordinal that the text gives.
func enumForString(info *prop.EnumInfo, s string) (int, bool) {
	if o := info.Ordinal(s); o >= 0 {
		return o, true
	}
	if i, ok := gwt.ParseInt(s); ok && i >= 0 && i < len(info.Names) {
		return i, true
	}
	return 0, false
}

// ParseValue is LayoutOptionData.parseValue for an option that has a Go
// property. ok is false where Java returns null: the option is then not set.
// The error is ErrUnsupported for a value type that is not ported.
func (o *Option) ParseValue(s string) (v any, ok bool, err error) {
	kind := o.Meta.Kind
	if s == "null" {
		return nil, false, nil
	}
	if s == "" && kind != "ENUMSET" {
		return nil, false, nil
	}
	if !isASCII(s) && kind != "STRING" {
		return nil, false, unsupported("option value with characters outside ASCII")
	}
	t := o.Prop.Type()
	switch kind {
	case "BOOLEAN":
		switch strings.ToLower(s) {
		case "true":
			return true, true, nil
		case "false":
			return false, true, nil
		}
		return nil, false, nil
	case "INT":
		i, ok := gwt.ParseInt(s)
		if !ok {
			return nil, false, nil
		}
		return i, true, nil
	case "DOUBLE":
		f, ok := gwt.ParseDouble(s)
		if !ok {
			return nil, false, nil
		}
		return f, true, nil
	case "STRING":
		return s, true, nil
	case "ENUM":
		info := prop.EnumOf(t)
		if info == nil {
			return nil, false, unsupported("enum " + o.Meta.Class + " is not ported")
		}
		ord, ok := enumForString(info, s)
		if !ok {
			return nil, false, nil
		}
		return reflect.ValueOf(ord).Convert(t).Interface(), true, nil
	case "ENUMSET":
		if t.Kind() != reflect.Pointer {
			return nil, false, unsupported("enum set " + o.Meta.Class + " is not ported")
		}
		set, isSet := reflect.New(t.Elem()).Interface().(prop.EnumSetValue)
		if !isSet {
			return nil, false, unsupported("enum set " + o.Meta.Class + " is not ported")
		}
		info := prop.EnumOf(set.EnumType())
		if info == nil {
			return nil, false, unsupported("enum " + o.Meta.Class + " is not ported")
		}
		var bits uint64
		// String.split("[\\[\\]\\s,]+"), then trim.
		for _, component := range strings.FieldsFunc(s, func(r rune) bool {
			return r == '[' || r == ']' || r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
		}) {
			if gwt.Trim(component) == "" {
				continue
			}
			ord, ok := enumForString(info, component)
			if !ok {
				return nil, false, nil
			}
			bits |= 1 << uint(ord)
		}
		set.SetBits(bits)
		return set, true, nil
	case "OBJECT":
		switch o.Meta.Class {
		case "org.eclipse.elk.core.math.KVector":
			out := &emath.KVector{}
			if out.Parse(s) != nil {
				return nil, false, nil
			}
			return out, true, nil
		case "org.eclipse.elk.core.math.ElkPadding":
			out := &emath.ElkPadding{}
			if out.Parse(s) != nil {
				return nil, false, nil
			}
			return out, true, nil
		case "org.eclipse.elk.core.math.ElkMargin":
			out := &emath.ElkMargin{}
			if out.Parse(s) != nil {
				return nil, false, nil
			}
			return out, true, nil
		}
		return nil, false, unsupported("option value of the class " + o.Meta.Class)
	}
	return nil, false, unsupported("option of the kind " + kind)
}
