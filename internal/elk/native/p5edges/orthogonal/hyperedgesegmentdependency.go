// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/HyperEdgeSegmentDependency.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package orthogonal

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"

// DependencyType is the enum HyperEdgeSegmentDependency.DependencyType.
type DependencyType int

// The constants of DependencyType, in the order of the Java enum.
const (
	// DependencyTypeRegular is REGULAR: a dependency that, if ignored, can
	// cause more crossings.
	DependencyTypeRegular DependencyType = iota
	// DependencyTypeCritical is CRITICAL: a dependency that, if ignored,
	// causes edge overlaps.
	DependencyTypeCritical
)

var namesDependencyType = []string{"REGULAR", "CRITICAL"}

// String returns the name of the Java constant.
func (t DependencyType) String() string { return prop.EnumName(namesDependencyType, int(t)) }

func init() {
	prop.RegisterEnum[DependencyType]("org.eclipse.elk.alg.layered.p5edges.orthogonal.HyperEdgeSegmentDependency$DependencyType", namesDependencyType)
}

// CriticalDependencyWeight is CRITICAL_DEPENDENCY_WEIGHT: the weight of a
// critical dependency, which is not zero.
const CriticalDependencyWeight = 1

// HyperEdgeSegmentDependency is the class HyperEdgeSegmentDependency: the
// source segment wants to be in a lower routing slot than the target
// segment.
type HyperEdgeSegmentDependency struct {
	typ    DependencyType
	source *HyperEdgeSegment
	target *HyperEdgeSegment
	weight int
}

// newHyperEdgeSegmentDependency is the private constructor: it makes the
// dependency and adds it to the dependency lists of the two segments.
func newHyperEdgeSegmentDependency(typ DependencyType, source, target *HyperEdgeSegment, weight int) *HyperEdgeSegmentDependency {
	d := &HyperEdgeSegmentDependency{typ: typ, weight: weight}
	d.SetSource(source)
	d.SetTarget(target)
	return d
}

// CreateAndAddRegular is HyperEdgeSegmentDependency.createAndAddRegular(
// source, target, weight).
func CreateAndAddRegular(source, target *HyperEdgeSegment, weight int) *HyperEdgeSegmentDependency {
	return newHyperEdgeSegmentDependency(DependencyTypeRegular, source, target, weight)
}

// CreateAndAddCritical is HyperEdgeSegmentDependency.createAndAddCritical(
// source, target).
func CreateAndAddCritical(source, target *HyperEdgeSegment) *HyperEdgeSegmentDependency {
	return newHyperEdgeSegmentDependency(DependencyTypeCritical, source, target, CriticalDependencyWeight)
}

// Remove is remove(): it removes the dependency from its two segments.
func (d *HyperEdgeSegmentDependency) Remove() {
	d.SetSource(nil)
	d.SetTarget(nil)
}

// Reverse is reverse().
func (d *HyperEdgeSegmentDependency) Reverse() {
	oldSource := d.source
	oldTarget := d.target

	d.SetSource(oldTarget)
	d.SetTarget(oldSource)
}

// Type is getType().
func (d *HyperEdgeSegmentDependency) Type() DependencyType { return d.typ }

// Source is getSource().
func (d *HyperEdgeSegmentDependency) Source() *HyperEdgeSegment { return d.source }

func removeDependency(list []*HyperEdgeSegmentDependency, d *HyperEdgeSegmentDependency) []*HyperEdgeSegmentDependency {
	for i, x := range list {
		if x == d {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// SetSource is setSource(newSource): it moves the dependency to the end of
// the outgoing dependencies of the new source.
func (d *HyperEdgeSegmentDependency) SetSource(newSource *HyperEdgeSegment) {
	if d.source != nil {
		d.source.outgoingSegmentDependencies = removeDependency(d.source.outgoingSegmentDependencies, d)
	}

	d.source = newSource

	if d.source != nil {
		d.source.outgoingSegmentDependencies = append(d.source.outgoingSegmentDependencies, d)
	}
}

// Target is getTarget().
func (d *HyperEdgeSegmentDependency) Target() *HyperEdgeSegment { return d.target }

// SetTarget is setTarget(newTarget): it moves the dependency to the end of
// the incoming dependencies of the new target.
func (d *HyperEdgeSegmentDependency) SetTarget(newTarget *HyperEdgeSegment) {
	if d.target != nil {
		d.target.incomingSegmentDependencies = removeDependency(d.target.incomingSegmentDependencies, d)
	}

	d.target = newTarget

	if d.target != nil {
		d.target.incomingSegmentDependencies = append(d.target.incomingSegmentDependencies, d)
	}
}

// Weight is getWeight().
func (d *HyperEdgeSegmentDependency) Weight() int { return d.weight }

// String is toString().
func (d *HyperEdgeSegmentDependency) String() string {
	return segmentString(d.source) + "->" + segmentString(d.target) + " (" + d.typ.String() + ")"
}

func segmentString(s *HyperEdgeSegment) string {
	if s == nil {
		return "null"
	}
	return s.String()
}
