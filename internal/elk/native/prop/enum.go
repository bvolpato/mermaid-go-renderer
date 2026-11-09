// Ported from java.util.EnumSet of the GWT runtime in elk-worker.js of elkjs
// 0.9.3 (Apache-2.0), and the enum conventions of the port.
//
// GWT has the Apache License 2.0. elk-worker.js has no notice for this
// code. The Java source of GWT for it is
// user/super/com/google/gwt/emul/java/util/EnumSet.java. Its copyright
// line, read from the main branch of https://github.com/gwtproject/gwt on
// 2026-11-09:
//
//	Copyright 2008 Google Inc.
//
// The part of this file that follows EnumSet is a Go translation of that
// code. It is a changed version of it. ../gwt/LICENSE-GWT.txt has the
// license, and ../gwt/NOTICE has the license notice of the GWT files.

package prop

import (
	"math/bits"
	"reflect"
	"strings"
)

// EnumName returns the name of the constant with the given ordinal.
func EnumName(names []string, ordinal int) string {
	if ordinal < 0 || ordinal >= len(names) {
		return "?"
	}
	return names[ordinal]
}

// EnumInfo describes a Java enum that has a Go type.
type EnumInfo struct {
	// Class is the Java class name.
	Class string
	// Type is the Go type, an integer type whose values are the ordinals.
	Type reflect.Type
	// Names has the names of the constants by ordinal.
	Names []string
}

var (
	enumsByType  = map[reflect.Type]*EnumInfo{}
	enumsByClass = map[string]*EnumInfo{}
)

// RegisterEnum records the names of an enum type. Call it from an init
// function.
func RegisterEnum[E ~int](class string, names []string) {
	info := &EnumInfo{Class: class, Type: reflect.TypeFor[E](), Names: names}
	enumsByType[info.Type] = info
	enumsByClass[class] = info
}

// EnumOf returns the description of the enum type t, or nil.
func EnumOf(t reflect.Type) *EnumInfo { return enumsByType[t] }

// EnumByClass returns the description of the Java enum class, or nil.
func EnumByClass(class string) *EnumInfo { return enumsByClass[class] }

// Ordinal returns the ordinal of the constant with the given name, or -1.
func (e *EnumInfo) Ordinal(name string) int {
	for i, n := range e.Names {
		if n == name {
			return i
		}
	}
	return -1
}

// ParseEnum is Enum.valueOf for a registered enum type.
func ParseEnum[E ~int](name string) (E, bool) {
	info := enumsByType[reflect.TypeFor[E]()]
	if info == nil {
		return 0, false
	}
	i := info.Ordinal(name)
	return E(i), i >= 0
}

// EnumSet is java.util.EnumSet<E>: a set of constants that iterates in the
// order of the ordinals. ELK changes sets in place, so ported code passes
// *EnumSet. An enum has at most 64 constants.
type EnumSet[E ~int] struct {
	bits uint64
}

// NoneOf is EnumSet.noneOf.
func NoneOf[E ~int]() *EnumSet[E] { return &EnumSet[E]{} }

// Of is EnumSet.of(values...).
func Of[E ~int](values ...E) *EnumSet[E] {
	s := &EnumSet[E]{}
	for _, v := range values {
		s.bits |= 1 << uint(v)
	}
	return s
}

// AllOf is EnumSet.allOf for an enum with n constants.
func AllOf[E ~int](n int) *EnumSet[E] {
	return &EnumSet[E]{bits: 1<<uint(n) - 1}
}

// Clone is clone().
func (s *EnumSet[E]) Clone() *EnumSet[E] { return &EnumSet[E]{bits: s.bits} }

// Contains is contains(e).
func (s *EnumSet[E]) Contains(e E) bool { return s.bits&(1<<uint(e)) != 0 }

// ContainsAll is containsAll(other).
func (s *EnumSet[E]) ContainsAll(o *EnumSet[E]) bool { return s.bits&o.bits == o.bits }

// Add is add(e). It returns true when the set changed.
func (s *EnumSet[E]) Add(e E) bool {
	old := s.bits
	s.bits |= 1 << uint(e)
	return s.bits != old
}

// AddAll is addAll(other).
func (s *EnumSet[E]) AddAll(o *EnumSet[E]) { s.bits |= o.bits }

// Remove is remove(e). It returns true when the set changed.
func (s *EnumSet[E]) Remove(e E) bool {
	old := s.bits
	s.bits &^= 1 << uint(e)
	return s.bits != old
}

// RemoveAll is removeAll(other).
func (s *EnumSet[E]) RemoveAll(o *EnumSet[E]) { s.bits &^= o.bits }

// RetainAll is retainAll(other).
func (s *EnumSet[E]) RetainAll(o *EnumSet[E]) { s.bits &= o.bits }

// Clear is clear().
func (s *EnumSet[E]) Clear() { s.bits = 0 }

// IsEmpty is isEmpty().
func (s *EnumSet[E]) IsEmpty() bool { return s.bits == 0 }

// Len is size().
func (s *EnumSet[E]) Len() int { return bits.OnesCount64(s.bits) }

// Equals is equals(other).
func (s *EnumSet[E]) Equals(o *EnumSet[E]) bool { return o != nil && s.bits == o.bits }

// Values returns the constants of the set in the order of the ordinals.
func (s *EnumSet[E]) Values() []E {
	out := make([]E, 0, s.Len())
	for b := s.bits; b != 0; b &= b - 1 {
		out = append(out, E(bits.TrailingZeros64(b)))
	}
	return out
}

// Bits returns the set as a bit mask of ordinals.
func (s *EnumSet[E]) Bits() uint64 { return s.bits }

// String is toString(): "[A, B]".
func (s *EnumSet[E]) String() string {
	info := enumsByType[reflect.TypeFor[E]()]
	var names []string
	for _, v := range s.Values() {
		if info != nil {
			names = append(names, EnumName(info.Names, int(v)))
		} else {
			names = append(names, "?")
		}
	}
	return "[" + strings.Join(names, ", ") + "]"
}

// EnumSetValue is the untyped view of an EnumSet, for code that handles sets
// of any enum type: the option parser and the test harness.
type EnumSetValue interface {
	Bits() uint64
	// EnumType is the Go type of the constants.
	EnumType() reflect.Type
	// SetBits replaces the content.
	SetBits(bits uint64)
}

// EnumType is the Go type of the constants.
func (s *EnumSet[E]) EnumType() reflect.Type { return reflect.TypeFor[E]() }

// SetBits replaces the content with a bit mask of ordinals.
func (s *EnumSet[E]) SetBits(bits uint64) { s.bits = bits }
