// Ported from Eclipse ELK 0.9.1, in part: the ids and the default values of
// the internal properties are those of
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/options/InternalProperties.java
// (EPL-2.0). The two properties with the prefix org.eclipse.elk are layout
// options of ELK.
//
// The properties that the methods of this package read. The package iprops
// declares the full set of internal properties. It imports this package, so
// these few are declared here a second time. Two properties with the same id
// name the same entry, as in Java.

package lgraph

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

var (
	origin              = prop.New[any]("origin")
	reversed            = prop.NewDefault("reversed", false)
	inputCollect        = prop.NewDefault("inputCollect", false)
	outputCollect       = prop.NewDefault("outputCollect", false)
	representedLabels   = prop.New[[]*LLabel]("representedLabels")
	edgeLabelsInline    = prop.NewDefault("org.eclipse.elk.edgeLabels.inline", false)
	edgeLabelsPlacement = prop.NewDefault("org.eclipse.elk.edgeLabels.placement", core.EdgeLabelPlacementCenter)
)

var (
	extPortSize         = prop.NewCloneable("externalPortSize", emath.NewKVector(0, 0))
	extPortSide         = prop.NewDefault("externalPortSide", core.PortSideUndefined)
	edgeConstraint      = prop.NewDefault("edgeConstraint", lenum.EdgeConstraintNone)
	inLayerConstraint   = prop.NewDefault("inLayerConstraint", lenum.InLayerConstraintNone)
	portRatioOrPosition = prop.NewDefault("portRatioOrPosition", 0.0)
	graphProperties     = prop.NewFresh("graphProperties", prop.NoneOf[lenum.GraphProperties]())
	modelOrder          = prop.New[int]("modelOrder")
)
