// Ported from @upsetjs/venn.js 2.0.0, build/venn.esm.js (src/index.js): the
// build that mermaid 12.2.0 bundles. The build also holds the functions of
// the fmin package (nelderMead, conjugateGradient, bisect).
//
// venn.js: Copyright (c) 2013 Ben Frederickson, Copyright (c) 2021 Samuel
// Gratzl. MIT License.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package venn is a port of @upsetjs/venn.js 2.0.0, the library that the
// venn diagram of mermaid uses: it lays out circles so that the areas of
// their overlaps match given sizes.
//
// The port has the functions that mermaid reaches. Layout is venn.layout: the
// circles, the text centres and the paths of every area. Diagram is the chart
// function of venn.VennDiagram: it builds the SVG elements of the diagram.
// The loss function is the default one. The options "distinct", "scaleToFit",
// "symmetricalTextCentre" and "orientationOrder" have their defaults.
//
// The numbers are the numbers of the JavaScript code: every function keeps
// the iteration, the order of the floating-point operations and the order in
// which JavaScript enumerates the keys of an object. Random numbers come from
// Env.Random in the order and the number in which venn.js calls Math.random.
// The transcendental functions come from Env.Math, by default the wrappers of
// package js, which return the values of Chromium.
//
// venn.js throws for some inputs. The exported functions return an *Error
// for them.
package venn

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// pi is Math.PI.
const pi float64 = math.Pi

// Area is one area of the diagram: a set, or the intersection of sets. It is
// the object { sets, size, label, weight } that venn.js gets.
type Area struct {
	// Sets holds the ids of the sets of the area.
	Sets []string
	// Size is the size of the area.
	Size float64
	// Label is the label. HasLabel is false when the label is undefined.
	Label    string
	HasLabel bool
	// Weight is the weight of the area in the loss function. HasWeight is
	// false when the weight is undefined: the weight is then 1.
	Weight    float64
	HasWeight bool
}

// Circle is the circle of one set.
type Circle struct {
	X, Y, Radius float64
	// Size is the size of the set. Only the circles of the greedy layout
	// have it.
	Size float64
	// SetID is the id of the set (setid). It is set when a solution is
	// converted to a list of circles.
	SetID string

	// parent is the link of the union-find clustering in disjointCluster.
	parent *Circle
}

// Point is a point { x, y }.
type Point struct {
	X, Y float64
}

// TextCentre is the centre of the text of an area. Disjoint is true when
// the area is not on the screen.
type TextCentre struct {
	X, Y     float64
	Disjoint bool
}

// Error is a JavaScript exception of venn.js: a thrown string, or a
// TypeError for a property of undefined.
type Error struct {
	// Name is "Error" for a thrown string, "TypeError" for a TypeError and
	// "Hang" for an input that needs more steps than MaxWork.
	Name string
	// Message is the thrown string, or the message of the error.
	Message string
}

func (e *Error) Error() string { return "venn: " + e.Message }

// Math has the Math functions that venn.js calls and that are not exact by
// definition. A nil field selects the function of internal/js, which gives
// the result of Chromium.
type Math struct {
	Sin   func(x float64) float64
	Cos   func(x float64) float64
	Acos  func(x float64) float64
	Atan2 func(y, x float64) float64
	Pow   func(x, y float64) float64
}

// MaxWork is the default bound of the work of one layout, counted in area
// overlaps that the loss function computes. venn.js has no bound: the time
// of its greedy layout grows with the fifth power of the number of sets, and
// a diagram of a hundred sets takes minutes. A layout that passes the bound
// fails with an error.
const MaxWork = 400_000_000

// Env is the environment of the JavaScript code: Math.random, the Math
// functions and Array.prototype.sort. The zero value is the environment of
// a page in Chromium, with the random numbers of math/rand.
type Env struct {
	// Random is Math.random. The constrained MDS layout calls it for
	// diagrams of four or more sets.
	Random func() float64
	// Math has the transcendental functions.
	Math *Math
	// Sort is the implementation of Array.prototype.sort. The default is
	// the sort of Chromium. The implementations differ only when a
	// comparator returns NaN, which happens for sizes that are not finite.
	Sort js.V8Sort
	// MaxWork bounds the work of one call. 0 selects the constant MaxWork.
	MaxWork int
}

// engine is the state of one call of an exported function.
type engine struct {
	random  func() float64
	m       Math
	sortAlg js.V8Sort
	work    int
	maxWork int
}

func newEngine(env *Env) *engine {
	e := &engine{
		random:  rand.Float64,
		m:       Math{Sin: js.Sin, Cos: js.Cos, Acos: js.Acos, Atan2: js.Atan2, Pow: js.Pow},
		sortAlg: js.V8PowerSort,
		maxWork: MaxWork,
	}
	if env == nil {
		return e
	}
	if env.Random != nil {
		e.random = env.Random
	}
	if env.Math != nil {
		if env.Math.Sin != nil {
			e.m.Sin = env.Math.Sin
		}
		if env.Math.Cos != nil {
			e.m.Cos = env.Math.Cos
		}
		if env.Math.Acos != nil {
			e.m.Acos = env.Math.Acos
		}
		if env.Math.Atan2 != nil {
			e.m.Atan2 = env.Math.Atan2
		}
		if env.Math.Pow != nil {
			e.m.Pow = env.Math.Pow
		}
	}
	e.sortAlg = env.Sort
	if env.MaxWork > 0 {
		e.maxWork = env.MaxWork
	}
	return e
}

// throw raises a JavaScript exception. The exported functions recover it
// and return it as an *Error.
func throw(name, message string) {
	panic(&Error{Name: name, Message: message})
}

// throwUndefined raises the TypeError of a property read on undefined.
func throwUndefined(property string) {
	throw("TypeError", "Cannot read properties of undefined (reading '"+property+"')")
}

// step counts work, and raises an error when the call passes its bound.
func (e *engine) step(n int) {
	e.work += n
	if e.work > e.maxWork {
		throw("Hang", "the layout needs too many steps (venn.js takes minutes for this input)")
	}
}

// run calls body and turns a raised exception into a result. A runtime
// panic of the port stands for a TypeError of the JavaScript code: both come
// from a value that is undefined.
func run[T any](body func() T) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T
			result = zero
			if e, ok := r.(*Error); ok {
				err = e
			} else {
				err = &Error{Name: "TypeError", Message: fmt.Sprint(r)}
			}
		}
	}()
	return body(), nil
}

// sortBy is Array.prototype.sort(cmp).
func sortBy[T any](e *engine, s []T, cmp func(a, b T) float64) {
	js.SortV8(e.sortAlg, s, cmp)
}

// objectPrototypeKeys has the names of the properties that a plain
// JavaScript object inherits from Object.prototype.
var objectPrototypeKeys = map[string]bool{
	"constructor":          true,
	"__defineGetter__":     true,
	"__defineSetter__":     true,
	"hasOwnProperty":       true,
	"__lookupGetter__":     true,
	"__lookupSetter__":     true,
	"isPrototypeOf":        true,
	"propertyIsEnumerable": true,
	"toString":             true,
	"valueOf":              true,
	"__proto__":            true,
	"toLocaleString":       true,
}

// inObject is the operator `key in object` for a plain object that has the
// given own keys: the inherited properties of Object.prototype count.
func inObject[V any](own *js.Obj[V], key string) bool {
	return own.Has(key) || objectPrototypeKeys[key]
}

// setsKey is the property key that JavaScript makes from an array of set
// ids: String(sets), the ids joined with commas.
func setsKey(sets []string) string { return strings.Join(sets, ",") }

// checkSets raises the error for the set id "__proto__". In JavaScript an
// assignment to that key of a plain object changes the prototype and adds no
// property, and venn.js then reads a property of undefined.
func checkSets(areas []*Area) {
	for _, area := range areas {
		for _, set := range area.Sets {
			if set == "__proto__" {
				throwUndefined("radius")
			}
		}
	}
}
