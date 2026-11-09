// Ported from layout-base 2.0.1 (MIT license): src/LayoutConstants.js,
// src/fd/FDLayoutConstants.js and src/util/Integer.js, and from cose-base
// 2.2.0 (MIT license): src/CoSEConstants.js.
//
// The JavaScript keeps these values in objects that all layouts of a page
// share, and cytoscape-fcose 2.2.0 (src/fcose/cose.js) writes its options
// into them before each run. The constants here are the values after that
// with the options of mermaid 12.2.0 (architectureRenderer.ts): quality
// proof, no animation, nodeDimensionsIncludeLabels false, and constraints,
// which switch the tiling off. The values that depend on the diagram are in
// the type params.
//
// A value that takes part in arithmetic that is not exact is used through a
// variable at the place of use: Go computes an expression of constants
// exactly, and JavaScript computes it in float64.

package fcose

// Integer.
const (
	integerMaxValue = 2147483647
	integerMinValue = -2147483648
)

// LayoutConstants.
const (
	// quality is LayoutConstants.QUALITY: 0 draft, 1 default, 2 proof.
	quality = 2

	defaultCreateBendsAsNeeded   = false
	defaultIncremental           = true
	defaultAnimationOnLayout     = true
	defaultAnimationDuringLayout = false
	defaultAnimationPeriod       = 50.0
	defaultUniformLeafNodeSizes  = false // options.uniformNodeDimensions
	defaultGraphMargin           = 15.0
	nodeDimensionsIncludeLabels  = false
	simpleNodeSize               = 40.0
	emptyCompoundNodeSize        = 40.0
	// layoutConstantsAnimateDuring is LayoutConstants.ANIMATE === 'during'.
	layoutConstantsAnimateDuring = false
)

// FDLayoutConstants.
const (
	defaultSpringStrength                     = 0.45
	defaultRepulsionStrength                  = 4500.0
	defaultGravityStrength                    = 0.25 // options.gravity
	defaultCompoundGravityStrength            = 1.0  // options.gravityCompound
	defaultGravityRangeFactor                 = 3.8  // options.gravityRange
	defaultCompoundGravityRangeFactor         = 1.5  // options.gravityRangeCompound
	defaultUseSmartIdealEdgeLengthCalculation = true
	defaultUseSmartRepulsionRangeCalculation  = true
	defaultCoolingFactorIncremental           = 0.3 // options.initialEnergyOnIncremental
	coolingAdaptationFactor                   = 0.33
	adaptationLowerNodeLimit                  = 1000.0
	adaptationUpperNodeLimit                  = 5000.0
	maxNodeDisplacementIncremental            = 100.0
	maxNodeDisplacement                       = 300.0 // MAX_NODE_DISPLACEMENT_INCREMENTAL * 3
	convergenceCheckPeriod                    = 100.0
	perLevelIdealEdgeLengthFactor             = 0.1 // options.nestingFactor
	gridCalculationCheckPeriod                = 10.0
)

// CoSEConstants.
const (
	tile                       = false // options.tile: fcose clears it when a constraint exists
	enforceConstraints         = true  // options.step is "all"
	applyLayout                = true
	relaxMovementOnConstraints = true
	treeReductionOnIncremental = false // a constraint exists
)

// params are the constants that fcose derives from the options and from the
// graph of one run.
type params struct {
	// maxIterations is FDLayoutConstants.MAX_ITERATIONS: options.numIter.
	maxIterations float64
	// defaultEdgeLength is FDLayoutConstants.DEFAULT_EDGE_LENGTH: the mean
	// of the ideal lengths of the edges, or 50 for a graph without edges.
	defaultEdgeLength float64
	// minRepulsionDist is FDLayoutConstants.MIN_REPULSION_DIST:
	// DEFAULT_EDGE_LENGTH / 10.0.
	minRepulsionDist float64
	// transformOnConstraintHandling is
	// CoSEConstants.TRANSFORM_ON_CONSTRAINT_HANDLING: options.randomize.
	transformOnConstraintHandling bool
}
