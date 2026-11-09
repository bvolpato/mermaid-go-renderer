// Ported from Property.java, MapPropertyHolder.java and IPropertyHolder.java
// in plugins/org.eclipse.elk.graph/src/org/eclipse/elk/graph/properties of
// Eclipse ELK 0.9.1 (EPL-2.0).

// Package prop has the property system of ELK: typed properties with default
// values, and holders that map properties to values.
//
// A Java property value can be null. In Go a value of a type that has no nil
// (bool, int, float64, string, an enum) is never null: Get returns the zero
// value when the holder has no value and the property has no default. Where
// the Java code tests a value against null, use Has or Lookup.
package prop

import (
	"fmt"
	"reflect"
	"sync"
)

// Any is a property of any value type.
type Any interface {
	// ID is the identifier, for example "org.eclipse.elk.padding".
	ID() string
	// Type is the Go type of the values.
	Type() reflect.Type
	// DefaultAny returns the default value as Property.getDefault does: a
	// clone for a cloneable type. ok is false when there is no default.
	DefaultAny() (v any, ok bool)
	key() int32
}

// Property is the class Property<T>. Two properties with the same id are
// equal: they name the same entry of a holder, and each has its own default.
type Property[T any] struct {
	id     string
	k      int32
	def    T
	hasDef bool
	clone  func(T) T
	// fresh tells that Get returns a clone of the default without storing
	// it.
	fresh   bool
	nilable bool
}

var (
	keysMu sync.Mutex
	keys   = map[string]int32{}
	ids    []string
	// declared has the properties that the packages declare, by id. Several
	// properties can have one id.
	declared = map[string][]Any{}
)

// Declared returns the properties that have the given id. The test harness
// uses it to give a stored value the type of its property.
func Declared(id string) []Any {
	keysMu.Lock()
	defer keysMu.Unlock()
	return declared[id]
}

func declare(p Any) {
	keysMu.Lock()
	defer keysMu.Unlock()
	declared[p.ID()] = append(declared[p.ID()], p)
}

// keyOf interns an id. Properties are package variables, so the table is
// complete when the init functions end.
func keyOf(id string) int32 {
	keysMu.Lock()
	defer keysMu.Unlock()
	k, ok := keys[id]
	if !ok {
		k = int32(len(ids))
		keys[id] = k
		ids = append(ids, id)
	}
	return k
}

func isNilable(t reflect.Type) bool {
	if t == nil {
		return true
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.Func, reflect.Chan:
		return true
	}
	return false
}

// New is new Property<T>(id): a property without a default value.
func New[T any](id string) *Property[T] {
	p := &Property[T]{id: id, k: keyOf(id), nilable: isNilable(reflect.TypeFor[T]())}
	declare(p)
	return p
}

// NewDefault is new Property<T>(id, default) for a default value that is not
// cloneable: each Get without a value returns def itself.
func NewDefault[T any](id string, def T) *Property[T] {
	p := New[T](id)
	p.def, p.hasDef = def, true
	return p
}

// Cloneable is a value type that ELK clones when it returns a default value:
// the types that implement java.lang.Cloneable in GWT and that ElkReflect
// registers (KVector, KVectorChain, ElkPadding, ElkMargin and the collection
// classes). EnumSet is not among them: see NewFresh.
type Cloneable[T any] interface {
	Clone() T
}

// NewCloneable is new Property<T>(id, default) for a cloneable default. Get
// on a holder without a value stores a clone of the default in the holder and
// returns it, as MapPropertyHolder.getProperty does. So a caller can change
// the value in place.
func NewCloneable[T Cloneable[T]](id string, def T) *Property[T] {
	p := NewDefault(id, def)
	p.clone = func(v T) T { return v.Clone() }
	return p
}

// NewFresh is new Property<T>(id, default) for a default value that the
// caller could change but that ELK does not clone: an EnumSet. In elkjs, Get
// on a holder without a value returns the default object itself and does not
// store it, because the EnumSet of GWT is not Cloneable. Here Get returns a
// copy of the default each time and does not store it either, so that a
// caller cannot change the default of the next layout.
func NewFresh[T Cloneable[T]](id string, def T) *Property[T] {
	p := NewDefault(id, def)
	p.clone = func(v T) T { return v.Clone() }
	p.fresh = true
	return p
}

// NewCloneFunc is NewCloneable for a type without a Clone method, for example
// a slice.
func NewCloneFunc[T any](id string, def T, clone func(T) T) *Property[T] {
	p := NewDefault(id, def)
	p.clone = clone
	return p
}

// With is new Property<T>(other, default): the id of other with another
// default value that is not cloneable.
func With[T any](other Any, def T) *Property[T] {
	return NewDefault[T](other.ID(), def)
}

// ID is getId().
func (p *Property[T]) ID() string { return p.id }

// Type returns the Go type of the values.
func (p *Property[T]) Type() reflect.Type { return reflect.TypeFor[T]() }

func (p *Property[T]) key() int32 { return p.k }

// Default is getDefault(): the default value, or a clone of it for a
// cloneable type. ok is false when the property has no default.
func (p *Property[T]) Default() (v T, ok bool) {
	if !p.hasDef {
		return v, false
	}
	if p.clone != nil {
		return p.clone(p.def), true
	}
	return p.def, true
}

// DefaultAny is Default with an untyped result.
func (p *Property[T]) DefaultAny() (any, bool) {
	v, ok := p.Default()
	if !ok || p.isNil(v) {
		return nil, false
	}
	return v, true
}

func (p *Property[T]) isNil(v T) bool {
	if !p.nilable {
		return false
	}
	rv := reflect.ValueOf(&v).Elem()
	return rv.IsNil()
}

// String is toString().
func (p *Property[T]) String() string { return p.id }

// Holder is IPropertyHolder. A type becomes a holder when it embeds
// MapHolder.
type Holder interface {
	propertyMap() *MapHolder
}

type entry struct {
	key int32
	val any
}

// MapHolder is MapPropertyHolder: the property values of one object. The zero
// value is an empty holder.
type MapHolder struct {
	entries []entry
}

func (h *MapHolder) propertyMap() *MapHolder { return h }

func (h *MapHolder) find(k int32) int {
	for i := range h.entries {
		if h.entries[i].key == k {
			return i
		}
	}
	return -1
}

func (h *MapHolder) put(k int32, v any) {
	if i := h.find(k); i >= 0 {
		h.entries[i].val = v
		return
	}
	h.entries = append(h.entries, entry{k, v})
}

func (h *MapHolder) remove(k int32) {
	if i := h.find(k); i >= 0 {
		h.entries = append(h.entries[:i], h.entries[i+1:]...)
	}
}

// Get is getProperty(property): the value of the holder, or the default
// value. A cloneable default is stored in the holder first.
func Get[T any](h Holder, p *Property[T]) T {
	m := h.propertyMap()
	if i := m.find(p.k); i >= 0 {
		return m.entries[i].val.(T)
	}
	if !p.hasDef {
		var zero T
		return zero
	}
	if p.clone != nil {
		if p.isNil(p.def) {
			return p.def
		}
		v := p.clone(p.def)
		if !p.fresh {
			m.entries = append(m.entries, entry{p.k, v})
		}
		return v
	}
	return p.def
}

// Lookup returns the value that the holder has for the property, without the
// default. ok is hasProperty(property).
func Lookup[T any](h Holder, p *Property[T]) (v T, ok bool) {
	m := h.propertyMap()
	if i := m.find(p.k); i >= 0 {
		return m.entries[i].val.(T), true
	}
	return v, false
}

// Has is hasProperty(property): the holder has a value that is not null.
func Has(h Holder, p Any) bool {
	return h.propertyMap().find(p.key()) >= 0
}

// IsNull tells that getProperty(property) is null: the holder has no value
// and the property has no default.
func IsNull(h Holder, p Any) bool {
	if Has(h, p) {
		return false
	}
	_, ok := p.DefaultAny()
	return !ok
}

// Set is setProperty(property, value). A nil value removes the entry.
func Set[T any](h Holder, p *Property[T], v T) {
	m := h.propertyMap()
	if p.isNil(v) {
		m.remove(p.k)
		return
	}
	m.put(p.k, v)
}

// Unset is setProperty(property, null).
func Unset(h Holder, p Any) {
	h.propertyMap().remove(p.key())
}

// SetAny is setProperty for an untyped value. The value must have the type
// of the property.
func SetAny(h Holder, p Any, v any) {
	if v == nil {
		Unset(h, p)
		return
	}
	if t := reflect.TypeOf(v); t != p.Type() && !(p.Type().Kind() == reflect.Interface && t.Implements(p.Type())) {
		panic(fmt.Sprintf("prop: value of type %v for the property %s of type %v", t, p.ID(), p.Type()))
	}
	h.propertyMap().put(p.key(), v)
}

// GetAny is getProperty with an untyped result. ok is false for null.
func GetAny(h Holder, p Any) (v any, ok bool) {
	m := h.propertyMap()
	if i := m.find(p.key()); i >= 0 {
		return m.entries[i].val, true
	}
	return p.DefaultAny()
}

// CopyProperties is copyProperties(other): each value of src replaces the
// value of dst. The values are shared, not cloned.
func CopyProperties(dst, src Holder) {
	if src == nil {
		return
	}
	d, s := dst.propertyMap(), src.propertyMap()
	for _, e := range s.entries {
		d.put(e.key, e.val)
	}
}

// Each calls fn for each value of the holder, with the id of its property.
// The order is the order of the first assignment, which is not the order of
// the Java map. ELK does not depend on that order.
func Each(h Holder, fn func(id string, v any)) {
	for _, e := range h.propertyMap().entries {
		fn(idOf(e.key), e.val)
	}
}

// Len is the number of values of the holder.
func Len(h Holder) int { return len(h.propertyMap().entries) }

func idOf(k int32) string {
	keysMu.Lock()
	defer keysMu.Unlock()
	return ids[k]
}

// SetByID stores a value under a property id, without a Property object. The
// test harness uses it for properties that no Go package declares.
func SetByID(h Holder, id string, v any) {
	h.propertyMap().put(keyOf(id), v)
}

// Clear removes each value of the holder.
func Clear(h Holder) {
	h.propertyMap().entries = nil
}
