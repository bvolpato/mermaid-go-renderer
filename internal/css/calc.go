// Ported from Chromium 156.0.8078.4 (Blink), in part: the value of a CSS math
// function. The evaluation follows two steps of
// third_party/blink/renderer/core/css/css_math_expression_node.cc
// (BSD-3-Clause, with the copyright of Google in the file):
//
//   - CSSMathExpressionOperation::CreateArithmeticOperationSimplified and
//     MaybeSimplifySumOrProductNode: a product of numbers and one dimension
//     is a dimension whose number is the product, and a sum of dimensions
//     with one unit is a dimension whose number is the sum.
//   - CSSMathExpressionOperation::ComputeLengthPx: other sums add the
//     lengths in px, in double arithmetic.
//
// The grammar is the one of CSS Values and Units, level 4, section 10. The
// file has the functions calc(), min(), max() and clamp() with the
// operators +, -, * and /. It does not have the other math functions. See
// NOTICE in this directory.

package css

import (
	"math"
	"strconv"
	"strings"
)

// CalcValue is the value of a math function.
type CalcValue struct {
	// Number is the value when the function is a number, such as calc(3 / 2).
	Number float64
	// Px is the sum of the lengths, and Percent the sum of the percentages.
	Px, Percent float64
	// IsNumber is true for a number. HasLength and HasPercent say which
	// parts a value that is not a number has.
	IsNumber, HasLength, HasPercent bool
}

// IsMathFunction reports whether a value is one math function that this
// package evaluates: calc(), min(), max() or clamp().
func IsMathFunction(value string) bool {
	s := strings.TrimSpace(value)
	if len(s) < 5 || s[len(s)-1] != ')' {
		return false
	}
	open := strings.IndexByte(s, '(')
	if open < 0 {
		return false
	}
	switch strings.ToLower(s[:open]) {
	case "calc", "min", "max", "clamp":
	default:
		return false
	}
	// The parenthesis that closes the function is the last character.
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i == len(s)-1
			}
		}
	}
	return false
}

// EvalCalc evaluates a math function. unitPx gives the length in px of a
// dimension: of 1.5 and "em", for example. The unit is in lower case. The
// second result of unitPx is false for a unit that is not a length. The
// second result of EvalCalc is false for a value that is not a valid math
// function, or that this package does not evaluate.
func EvalCalc(value string, unitPx func(number float64, unit string) (float64, bool)) (CalcValue, bool) {
	if !IsMathFunction(value) {
		return CalcValue{}, false
	}
	p := &calcParser{s: strings.TrimSpace(value), unitPx: unitPx}
	v, ok := p.value()
	p.skipSpace()
	if !ok || p.i != len(p.s) {
		return CalcValue{}, false
	}
	return p.result(v)
}

// calcTerm is a node of a math expression after the simplification of
// Blink: a number, a dimension with one unit (the unit "%" is a
// percentage), or a sum of lengths in px and of percentages.
type calcTerm struct {
	kind    calcKind
	number  float64 // the number, or the number of the dimension
	unit    string
	px, pct float64 // calcSum
	hasPx   bool
	hasPct  bool
}

type calcKind uint8

const (
	calcNumber calcKind = iota
	calcDimension
	calcSum
)

type calcParser struct {
	s      string
	i      int
	unitPx func(float64, string) (float64, bool)
	depth  int
}

// maxCalcDepth bounds the nesting of a math expression.
const maxCalcDepth = 64

func (p *calcParser) skipSpace() bool {
	start := p.i
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n' || p.s[p.i] == '\r' || p.s[p.i] == '\f') {
		p.i++
	}
	return p.i > start
}

// toSum converts a term to the sum form. The second result is false for a
// number, which is not a length, and for a unit that is not a length.
func (p *calcParser) toSum(t calcTerm) (calcTerm, bool) {
	switch t.kind {
	case calcSum:
		return t, true
	case calcDimension:
		if t.unit == "%" {
			return calcTerm{kind: calcSum, pct: t.number, hasPct: true}, true
		}
		px, ok := p.unitPx(t.number, t.unit)
		if !ok {
			return calcTerm{}, false
		}
		return calcTerm{kind: calcSum, px: px, hasPx: true}, true
	}
	return calcTerm{}, false
}

func (p *calcParser) result(t calcTerm) (CalcValue, bool) {
	if t.kind == calcNumber {
		return CalcValue{Number: t.number, IsNumber: true}, true
	}
	s, ok := p.toSum(t)
	if !ok {
		return CalcValue{}, false
	}
	return CalcValue{Px: s.px, Percent: s.pct, HasLength: s.hasPx, HasPercent: s.hasPct}, true
}

// sum parses a calc-sum: products with + and - between them. The two
// operators need white space on both sides.
func (p *calcParser) sum() (calcTerm, bool) {
	left, ok := p.product()
	if !ok {
		return calcTerm{}, false
	}
	for {
		save := p.i
		hadSpace := p.skipSpace()
		if p.i >= len(p.s) || (p.s[p.i] != '+' && p.s[p.i] != '-') {
			p.i = save
			return left, true
		}
		op := p.s[p.i]
		p.i++
		if !hadSpace || !p.skipSpace() {
			return calcTerm{}, false
		}
		right, ok := p.product()
		if !ok {
			return calcTerm{}, false
		}
		if left, ok = p.add(left, right, op == '-'); !ok {
			return calcTerm{}, false
		}
	}
}

// add is the sum or the difference of two terms. Two numbers give a number.
// Two dimensions with one unit give a dimension. Other terms give the sum
// of their lengths in px and of their percentages.
func (p *calcParser) add(a, b calcTerm, subtract bool) (calcTerm, bool) {
	sign := 1.0
	if subtract {
		sign = -1
	}
	switch {
	case a.kind == calcNumber && b.kind == calcNumber:
		return calcTerm{kind: calcNumber, number: a.number + float64(sign*b.number)}, true
	case a.kind == calcNumber || b.kind == calcNumber:
		// A number and a length do not add.
		return calcTerm{}, false
	case a.kind == calcDimension && b.kind == calcDimension && a.unit == b.unit:
		return calcTerm{kind: calcDimension, number: a.number + float64(sign*b.number), unit: a.unit}, true
	}
	sa, ok := p.toSum(a)
	if !ok {
		return calcTerm{}, false
	}
	sb, ok := p.toSum(b)
	if !ok {
		return calcTerm{}, false
	}
	return calcTerm{
		kind:   calcSum,
		px:     sa.px + float64(sign*sb.px),
		pct:    sa.pct + float64(sign*sb.pct),
		hasPx:  sa.hasPx || sb.hasPx,
		hasPct: sa.hasPct || sb.hasPct,
	}, true
}

// product parses a calc-product: values with * and / between them.
func (p *calcParser) product() (calcTerm, bool) {
	left, ok := p.value()
	if !ok {
		return calcTerm{}, false
	}
	for {
		save := p.i
		p.skipSpace()
		if p.i >= len(p.s) || (p.s[p.i] != '*' && p.s[p.i] != '/') {
			p.i = save
			return left, true
		}
		op := p.s[p.i]
		p.i++
		p.skipSpace()
		right, ok := p.value()
		if !ok {
			return calcTerm{}, false
		}
		if op == '/' {
			// The divisor is a number.
			if right.kind != calcNumber {
				return calcTerm{}, false
			}
			left = scaleCalc(left, right.number, true)
			continue
		}
		switch {
		case right.kind == calcNumber:
			left = scaleCalc(left, right.number, false)
		case left.kind == calcNumber:
			left = scaleCalc(right, left.number, false)
		default:
			// One side of a product is a number.
			return calcTerm{}, false
		}
	}
}

// scaleCalc multiplies a term by a number, or divides it.
func scaleCalc(t calcTerm, f float64, divide bool) calcTerm {
	scale := func(v float64) float64 {
		if divide {
			return v / f
		}
		return float64(v * f)
	}
	switch t.kind {
	case calcSum:
		t.px, t.pct = scale(t.px), scale(t.pct)
	default:
		t.number = scale(t.number)
	}
	return t
}

// value parses a calc-value: a number, a dimension, a percentage, a sum in
// parentheses or a math function.
func (p *calcParser) value() (calcTerm, bool) {
	p.skipSpace()
	if p.i >= len(p.s) {
		return calcTerm{}, false
	}
	if p.depth++; p.depth > maxCalcDepth {
		return calcTerm{}, false
	}
	defer func() { p.depth-- }()
	c := p.s[p.i]
	if c == '(' {
		return p.parenthesized()
	}
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		start := p.i
		for p.i < len(p.s) && isIdent(p.s[p.i:p.i+1]) {
			p.i++
		}
		name := strings.ToLower(p.s[start:p.i])
		if p.i >= len(p.s) || p.s[p.i] != '(' {
			return calcTerm{}, false
		}
		switch name {
		case "calc":
			return p.parenthesized()
		case "min", "max", "clamp":
			return p.minMaxClamp(name)
		}
		return calcTerm{}, false
	}
	return p.numeric()
}

// parenthesized parses "(" calc-sum ")".
func (p *calcParser) parenthesized() (calcTerm, bool) {
	p.i++ // (
	v, ok := p.sum()
	p.skipSpace()
	if !ok || p.i >= len(p.s) || p.s[p.i] != ')' {
		return calcTerm{}, false
	}
	p.i++
	return v, true
}

// minMaxClamp parses the arguments of min(), max() or clamp() and returns
// the result. The arguments are all numbers or all lengths.
func (p *calcParser) minMaxClamp(name string) (calcTerm, bool) {
	p.i++ // (
	var args []calcTerm
	for {
		v, ok := p.sum()
		if !ok {
			return calcTerm{}, false
		}
		args = append(args, v)
		p.skipSpace()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			continue
		}
		break
	}
	if p.i >= len(p.s) || p.s[p.i] != ')' {
		return calcTerm{}, false
	}
	p.i++
	if name == "clamp" && len(args) != 3 {
		return calcTerm{}, false
	}
	numbers := args[0].kind == calcNumber
	vals := make([]float64, len(args))
	for i, a := range args {
		if (a.kind == calcNumber) != numbers {
			return calcTerm{}, false
		}
		if numbers {
			vals[i] = a.number
			continue
		}
		s, ok := p.toSum(a)
		if !ok || s.hasPct {
			// A percentage has no value before layout.
			return calcTerm{}, false
		}
		vals[i] = s.px
	}
	var out float64
	switch name {
	case "min":
		out = vals[0]
		for _, v := range vals[1:] {
			out = math.Min(out, v)
		}
	case "max":
		out = vals[0]
		for _, v := range vals[1:] {
			out = math.Max(out, v)
		}
	default:
		out = math.Max(vals[0], math.Min(vals[1], vals[2]))
	}
	if numbers {
		return calcTerm{kind: calcNumber, number: out}, true
	}
	return calcTerm{kind: calcSum, px: out, hasPx: true}, true
}

// numeric parses a number, a percentage or a dimension.
func (p *calcParser) numeric() (calcTerm, bool) {
	start := p.i
	if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		p.i++
	}
	digits := 0
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
		digits++
	}
	if p.i+1 < len(p.s) && p.s[p.i] == '.' && p.s[p.i+1] >= '0' && p.s[p.i+1] <= '9' {
		p.i++
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			digits++
		}
	}
	if digits == 0 {
		return calcTerm{}, false
	}
	// An exponent has a digit after the e, or a sign and a digit.
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		j := p.i + 1
		if j < len(p.s) && (p.s[j] == '+' || p.s[j] == '-') {
			j++
		}
		if j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
			for j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
				j++
			}
			p.i = j
		}
	}
	f, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return calcTerm{}, false
	}
	if p.i < len(p.s) && p.s[p.i] == '%' {
		p.i++
		return calcTerm{kind: calcDimension, number: f, unit: "%"}, true
	}
	unitStart := p.i
	for p.i < len(p.s) && ((p.s[p.i] >= 'a' && p.s[p.i] <= 'z') || (p.s[p.i] >= 'A' && p.s[p.i] <= 'Z')) {
		p.i++
	}
	if p.i == unitStart {
		return calcTerm{kind: calcNumber, number: f}, true
	}
	return calcTerm{kind: calcDimension, number: f, unit: strings.ToLower(p.s[unitStart:p.i])}, true
}
