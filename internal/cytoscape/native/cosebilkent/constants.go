// Ported from layout-base 1.0.2 (MIT license): src/LayoutConstants.js,
// src/fd/FDLayoutConstants.js and src/util/Integer.js, and from cose-base
// 1.0.3 (MIT license): src/CoSEConstants.js.
//
// The JavaScript keeps these values in objects that all layouts of a page
// share, and cytoscape-cose-bilkent 4.1.0 (src/Layout/index.js,
// getUserOptions) writes its options into them before each run. The values
// here are the values after getUserOptions with the options of mermaid
// 12.2.0, { name: 'cose-bilkent', quality: 'proof', styleEnabled: false,
// animate: false }, over the defaults of the extension.
//
// A value that takes part in arithmetic that is not exact is used through a
// variable at the place of use: Go computes an expression of constants
// exactly, and JavaScript computes it in float64.

package cosebilkent

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
	defaultIncremental           = false // !options.randomize
	defaultAnimationOnLayout     = true
	defaultAnimationDuringLayout = false
	defaultAnimationPeriod       = 50.0
	defaultUniformLeafNodeSizes  = false
	defaultGraphMargin           = 15.0
	emptyCompoundNodeSize        = 40.0
	worldBoundary                = 1000000.0
	worldCenterX                 = 1200.0
	worldCenterY                 = 900.0
	// layoutConstantsAnimateDuring is LayoutConstants.ANIMATE === 'during'.
	layoutConstantsAnimateDuring = false
)

// FDLayoutConstants.
const (
	maxIterations                             = 2500.0 // options.numIter
	defaultEdgeLength                         = 50.0   // options.idealEdgeLength
	defaultSpringStrength                     = 0.45   // options.edgeElasticity
	defaultRepulsionStrength                  = 4500.0 // options.nodeRepulsion
	defaultGravityStrength                    = 0.25   // options.gravity
	defaultCompoundGravityStrength            = 1.0    // options.gravityCompound
	defaultGravityRangeFactor                 = 3.8    // options.gravityRange
	defaultCompoundGravityRangeFactor         = 1.5    // options.gravityRangeCompound
	defaultUseSmartIdealEdgeLengthCalculation = true
	defaultUseSmartRepulsionRangeCalculation  = true
	defaultCoolingFactorIncremental           = 0.5 // options.initialEnergyOnIncremental
	coolingAdaptationFactor                   = 0.33
	adaptationLowerNodeLimit                  = 1000.0
	adaptationUpperNodeLimit                  = 5000.0
	maxNodeDisplacementIncremental            = 100.0
	maxNodeDisplacement                       = 300.0 // MAX_NODE_DISPLACEMENT_INCREMENTAL * 3
	minRepulsionDist                          = 5.0   // DEFAULT_EDGE_LENGTH / 10.0 when the module loads
	convergenceCheckPeriod                    = 100.0
	gridCalculationCheckPeriod                = 10.0
)

// CoSEConstants.
const (
	// defaultRadialSeparation is FDLayoutConstants.DEFAULT_EDGE_LENGTH when
	// the module loads.
	defaultRadialSeparation    = 50.0
	defaultComponentSeperation = 60.0
	tile                       = true // options.tile
)
