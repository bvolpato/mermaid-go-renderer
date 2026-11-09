// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/options/InternalProperties.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package iprops has the internal properties of the layered algorithm
// (InternalProperties): the values that the processors leave on the graph for
// each other.
//
// A property whose Java type is a class of a package above lgraph has the
// type any here. Its comment names the type of the values.
package iprops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lenum"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

func cloneNodes(v []*lgraph.LNode) []*lgraph.LNode { return append([]*lgraph.LNode{}, v...) }

// The fields of InternalProperties.
var (
	// Origin is ORIGIN: the object that an element was made for. It is an
	// element of the ELK graph (a value that implements lgraph.OriginRef) or
	// an element of the LGraph (*lgraph.LNode, *lgraph.LPort, *lgraph.LEdge).
	Origin = prop.New[any]("origin")
	// CoordinateSystemOrigin is COORDINATE_SYSTEM_ORIGIN.
	CoordinateSystemOrigin = prop.New[*lgraph.LGraph]("coordinateOrigin")
	// Processors is PROCESSORS: the processors of a graph, []lgraph.Processor.
	Processors = prop.New[[]lgraph.Processor]("processors")
	// CompoundNode is COMPOUND_NODE.
	CompoundNode = prop.NewDefault("compoundNode", false)
	// InsideConnections is INSIDE_CONNECTIONS.
	InsideConnections = prop.NewDefault("insideConnections", false)
	// OriginalBendpoints is ORIGINAL_BENDPOINTS.
	OriginalBendpoints = prop.New[*emath.KVectorChain]("originalBendpoints")
	// OriginalDummyNodePosition is ORIGINAL_DUMMY_NODE_POSITION. It has no
	// default: test it with prop.Has.
	OriginalDummyNodePosition = prop.New[float64]("originalDummyNodePosition")
	// OriginalLabelEdge is ORIGINAL_LABEL_EDGE.
	OriginalLabelEdge = prop.New[*lgraph.LEdge]("originalLabelEdge")
	// RepresentedLabels is REPRESENTED_LABELS.
	RepresentedLabels = prop.New[[]*lgraph.LLabel]("representedLabels")
	// EndLabels is END_LABELS. The values are a map from *lgraph.LPort to the
	// label cell of the node spacing package.
	EndLabels = prop.New[any]("endLabels")
	// EndLabelEdge is END_LABEL_EDGE.
	EndLabelEdge = prop.New[*lgraph.LEdge]("endLabel.origin")
	// LabelSide is LABEL_SIDE.
	LabelSide = prop.NewDefault("labelSide", core.LabelSideUnknown)
	// MaxEdgeThickness is MAX_EDGE_THICKNESS.
	MaxEdgeThickness = prop.NewDefault("maxEdgeThickness", 0.0)
	// Reversed is REVERSED.
	Reversed = prop.NewDefault("reversed", false)
	// Random is RANDOM.
	Random = prop.New[*gwt.Random]("random")
	// LongEdgeSource is LONG_EDGE_SOURCE.
	LongEdgeSource = prop.New[*lgraph.LPort]("longEdgeSource")
	// LongEdgeTarget is LONG_EDGE_TARGET.
	LongEdgeTarget = prop.New[*lgraph.LPort]("longEdgeTarget")
	// LongEdgeHasLabelDummies is LONG_EDGE_HAS_LABEL_DUMMIES.
	LongEdgeHasLabelDummies = prop.NewDefault("longEdgeHasLabelDummies", false)
	// LongEdgeBeforeLabelDummy is LONG_EDGE_BEFORE_LABEL_DUMMY.
	LongEdgeBeforeLabelDummy = prop.NewDefault("longEdgeBeforeLabelDummy", false)
	// EdgeConstraint is EDGE_CONSTRAINT.
	EdgeConstraint = prop.NewDefault("edgeConstraint", lenum.EdgeConstraintNone)
	// InLayerLayoutUnit is IN_LAYER_LAYOUT_UNIT.
	InLayerLayoutUnit = prop.New[*lgraph.LNode]("inLayerLayoutUnit")
	// InLayerConstraint is IN_LAYER_CONSTRAINT.
	InLayerConstraint = prop.NewDefault("inLayerConstraint", lenum.InLayerConstraintNone)
	// InLayerSuccessorConstraints is IN_LAYER_SUCCESSOR_CONSTRAINTS. The
	// default is an empty list that Get stores in the holder. To add a node,
	// set the property to the longer slice.
	InLayerSuccessorConstraints = prop.NewCloneFunc("inLayerSuccessorConstraint", []*lgraph.LNode{}, cloneNodes)
	// InLayerSuccessorConstraintsBetweenNonDummies is
	// IN_LAYER_SUCCESSOR_CONSTRAINTS_BETWEEN_NON_DUMMIES.
	InLayerSuccessorConstraintsBetweenNonDummies = prop.NewDefault("inLayerSuccessorConstraintBetweenNonDummies", false)
	// PortDummy is PORT_DUMMY.
	PortDummy = prop.New[*lgraph.LNode]("portDummy")
	// CrossingHint is CROSSING_HINT.
	CrossingHint = prop.NewDefault("crossingHint", 0)
	// GraphProperties is GRAPH_PROPERTIES. The default is an empty set that
	// Get does not store: change the set of a graph only after the importer
	// set the property.
	GraphProperties = prop.NewFresh("graphProperties", prop.NoneOf[lenum.GraphProperties]())
	// ExtPortSide is EXT_PORT_SIDE.
	ExtPortSide = prop.NewDefault("externalPortSide", core.PortSideUndefined)
	// ExtPortSize is EXT_PORT_SIZE.
	ExtPortSize = prop.NewCloneable("externalPortSize", emath.NewKVector(0, 0))
	// ExtPortReplacedDummies is EXT_PORT_REPLACED_DUMMIES.
	ExtPortReplacedDummies = prop.New[[]*lgraph.LNode]("externalPortReplacedDummies")
	// ExtPortReplacedDummy is EXT_PORT_REPLACED_DUMMY.
	ExtPortReplacedDummy = prop.New[*lgraph.LNode]("externalPortReplacedDummy")
	// ExtPortConnections is EXT_PORT_CONNECTIONS.
	ExtPortConnections = prop.NewFresh("externalPortConnections", prop.NoneOf[core.PortSide]())
	// PortRatioOrPosition is PORT_RATIO_OR_POSITION.
	PortRatioOrPosition = prop.NewDefault("portRatioOrPosition", 0.0)
	// BarycenterAssociates is BARYCENTER_ASSOCIATES.
	BarycenterAssociates = prop.New[[]*lgraph.LNode]("barycenterAssociates")
	// TopComments is TOP_COMMENTS.
	TopComments = prop.New[[]*lgraph.LNode]("TopSideComments")
	// BottomComments is BOTTOM_COMMENTS.
	BottomComments = prop.New[[]*lgraph.LNode]("BottomSideComments")
	// CommentConnPort is COMMENT_CONN_PORT.
	CommentConnPort = prop.New[*lgraph.LPort]("CommentConnectionPort")
	// InputCollect is INPUT_COLLECT.
	InputCollect = prop.NewDefault("inputCollect", false)
	// OutputCollect is OUTPUT_COLLECT.
	OutputCollect = prop.NewDefault("outputCollect", false)
	// Cyclic is CYCLIC.
	Cyclic = prop.NewDefault("cyclic", false)
	// CrossHierarchyMap is CROSS_HIERARCHY_MAP. The value is the multimap of
	// the compound graph preprocessor.
	CrossHierarchyMap = prop.New[any]("crossHierarchyMap")
	// TargetOffset is TARGET_OFFSET.
	TargetOffset = prop.New[*emath.KVector]("targetOffset")
	// SplineLabelSize is SPLINE_LABEL_SIZE.
	SplineLabelSize = prop.NewCloneable("splineLabelSize", emath.NewKVector(0, 0))
	// Spacings is SPACINGS.
	Spacings = prop.New[*lgraph.Spacings]("spacings")
	// PartitionDummy is PARTITION_DUMMY.
	PartitionDummy = prop.NewDefault("partitionConstraint", false)
	// BreakingPointInfo is BREAKING_POINT_INFO. The value is the BPInfo of
	// the breaking point inserter.
	BreakingPointInfo = prop.New[any]("breakingPoint.info")
	// SplineSurvivingEdge is SPLINE_SURVIVING_EDGE.
	SplineSurvivingEdge = prop.New[*lgraph.LEdge]("splines.survivingEdge")
	// SplineRouteStart is SPLINE_ROUTE_START. The value is a list of spline
	// segments.
	SplineRouteStart = prop.New[any]("splines.route.start")
	// SplineEdgeChain is SPLINE_EDGE_CHAIN.
	SplineEdgeChain = prop.New[[]*lgraph.LEdge]("splines.edgeChain")
	// OriginalPortConstraints is ORIGINAL_PORT_CONSTRAINTS. It has no
	// default: test it with prop.Has.
	OriginalPortConstraints = prop.New[core.PortConstraints]("originalPortConstraints")
	// SelfLoopHolder is SELF_LOOP_HOLDER. The value is the self loop holder
	// of the self loop package.
	SelfLoopHolder = prop.New[any]("selfLoopHolder")
	// SplineNsPortYCoord is SPLINE_NS_PORT_Y_COORD. It has no default.
	SplineNsPortYCoord = prop.New[float64]("splines.nsPortY")
	// ModelOrder is MODEL_ORDER. It has no default: test it with prop.Has.
	ModelOrder = prop.New[int]("modelOrder")
	// LongEdgeTargetNode is LONG_EDGE_TARGET_NODE.
	LongEdgeTargetNode = prop.New[*lgraph.LNode]("longEdgeTargetNode")
	// FirstTryWithInitialOrder is FIRST_TRY_WITH_INITIAL_ORDER.
	FirstTryWithInitialOrder = prop.NewDefault("firstTryWithInitialOrder", false)
	// SecondTryWithInitialOrder is SECOND_TRY_WITH_INITIAL_ORDER. It has the
	// id of FIRST_TRY_WITH_INITIAL_ORDER in the Java source too.
	SecondTryWithInitialOrder = prop.NewDefault("firstTryWithInitialOrder", false)
	// HiddenNodes is HIDDEN_NODES.
	HiddenNodes = prop.New[[]*lgraph.LNode]("layerConstraints.hiddenNodes")
	// OriginalOppositePort is ORIGINAL_OPPOSITE_PORT.
	OriginalOppositePort = prop.New[*lgraph.LPort]("layerConstraints.opposidePort")
	// TargetNodeModelOrder is TARGET_NODE_MODEL_ORDER.
	TargetNodeModelOrder = prop.New[*gwt.Map[*lgraph.LNode, int]]("targetNode.modelOrder")
)
