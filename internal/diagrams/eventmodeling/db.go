// Ported from mermaid 12.2.0, diagrams/eventmodeling/db.ts and types.ts (MIT).

package eventmodeling

import (
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// DiagramProps is the DiagramProps interface of types.ts.
type DiagramProps struct {
	SwimlaneMinHeight           float64
	SwimlanePadding             float64
	SwimlaneGap                 float64
	BoxPadding                  float64
	BoxOverlap                  float64
	BoxDefaultY                 float64
	BoxMinWidth                 float64
	BoxMaxWidth                 float64
	BoxMinHeight                float64
	BoxMaxHeight                float64
	ContentStartX               float64
	TextMaxWidth                float64
	BoxTextFontWeight           string
	BoxTextPadding              float64
	SwimlaneTextFontWeight      string
	LabelUiAutomation           string
	LabelUiAutomationPrefix     string
	LabelCommandReadModel       string
	LabelCommandReadModelPrefix string
	LabelEvents                 string
	LabelEventsPrefix           string
}

// diagramProps is the constant diagramProps of db.ts.
var diagramProps = DiagramProps{
	SwimlaneMinHeight:           70,
	SwimlanePadding:             15,
	SwimlaneGap:                 10,
	BoxPadding:                  10,
	BoxOverlap:                  90,
	BoxDefaultY:                 0,
	BoxMinWidth:                 80,
	BoxMaxWidth:                 450,
	BoxMinHeight:                80,
	BoxMaxHeight:                750,
	ContentStartX:               250,
	TextMaxWidth:                450 - 2*10,
	BoxTextFontWeight:           "bold",
	BoxTextPadding:              10,
	SwimlaneTextFontWeight:      "bold",
	LabelUiAutomation:           "UI/Automation",
	LabelUiAutomationPrefix:     "UI/A: ",
	LabelCommandReadModel:       "Command/Read Model",
	LabelCommandReadModelPrefix: "C/RM: ",
	LabelEvents:                 "Events",
	LabelEventsPrefix:           "Stream: ",
}

// Dimension is the Dimension interface.
type Dimension struct {
	Width  float64
	Height float64
}

// Coordinate is the Coordinate interface.
type Coordinate struct {
	X float64
	Y float64
}

// VisualProps is the VisualProps interface. A color is a value of the theme
// variables, or a string of db.ts.
type VisualProps struct {
	Fill   any
	Stroke any
}

// TextProps is the TextProps interface.
type TextProps struct {
	Content string
	Width   float64
	Height  float64
}

// Box is the Box interface.
type Box struct {
	R float64
	X float64
	// Y has no meaning for the time being. The renderer calculates the
	// position from Swimlane.Y.
	Y           float64
	Dimension   Dimension
	LeftSibling bool
	Swimlane    *Swimlane
	Visual      VisualProps
	Text        string
	Frame       *EmFrame
	// Index is the line index.
	Index int
}

// SwimlaneProps is the SwimlaneProps interface. Namespace is undefined
// when HasNamespace is false.
type SwimlaneProps struct {
	Index        int
	Label        string
	Namespace    string
	HasNamespace bool
}

// Swimlane is the Swimlane type.
type Swimlane struct {
	R         float64
	Y         float64
	Height    float64
	MaxHeight float64
	SwimlaneProps
}

// Relation is the Relation interface.
type Relation struct {
	Visual    VisualProps
	Source    Coordinate
	Target    Coordinate
	SourceBox *Box
	TargetBox *Box
}

// Context is the Context interface: the state of the layout. The swimlanes
// are an object with the index of the swimlane as the key, so they
// enumerate in the order of the index.
type Context struct {
	Boxes         []*Box
	Swimlanes     *js.Obj[*Swimlane]
	Relations     []*Relation
	PreviousFrame *EmFrame
	// PreviousSwimlaneNumber is undefined when HasPreviousSwimlaneNumber is
	// false.
	PreviousSwimlaneNumber    int
	HasPreviousSwimlaneNumber bool
	MaxR                      float64
	SortedSwimlanesArray      []*Swimlane
}

// The kinds of the commands and the events.
const (
	positionFrameKind      = "position frame"
	framePositionedKind    = "frame positioned"
	positionRelationKind   = "position relation"
	relationPositionedKind = "relation positioned"
)

// command is the Command type: PositionFrame or PositionRelation.
type command struct {
	kind      string
	index     int
	frame     *EmFrame
	textProps TextProps
	// sourceFrame is nil when the property is undefined.
	sourceFrame *EmFrame
}

// event is the Event type: FramePositioned or RelationPositioned.
type event struct {
	kind      string
	index     int
	frame     *EmFrame
	visual    VisualProps
	dimension Dimension
	textProps TextProps
	sourceBox *Box
	targetBox *Box
}

// Error is an error that the db or the renderer of upstream throws.
type Error struct {
	// Name is the name of the JavaScript error. Empty for Error.
	Name    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// DB is the db of db.ts: its store and its functions.
type DB struct {
	render.CommonDB
	ctx *render.Context
	// ast is store.ast. It is nil while the store has no AST.
	ast *EventModel
}

// NewDB returns an empty db: the state after clear().
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx}
	d.Sanitize = ctx.SanitizeText
	return d
}

// PreserveCommentsWhenParsing is the property preserveCommentsWhenParsing
// of the db: Diagram.fromText gives the parser the text with its comments.
func (d *DB) PreserveCommentsWhenParsing() bool { return true }

// SetOptions is setOptions(rawOptString). It only logs upstream.
func (d *DB) SetOptions(string) {}

// GetOptions is getOptions(): an empty object.
func (d *DB) GetOptions() *js.Obj[any] { return js.NewObj[any]() }

// Clear is clear().
func (d *DB) Clear() {
	d.reset()
	d.ClearCommon()
}

func (d *DB) reset() {
	d.ast = nil
}

// GetDirection is getDirection().
func (d *DB) GetDirection() string { return "LR" }

// GetConfig is getConfig() of the db: the eventmodeling config over the
// default config.
func (d *DB) GetConfig() *js.Obj[any] {
	merged := js.NewObj[any]()
	spread := func(v any) {
		if o := js.AsObj(v); o != nil {
			o.Each(func(k string, e any) { merged.Set(k, e) })
		}
	}
	spread(d.ctx.ConfigState.DefaultSection("eventmodeling"))
	spread(d.ctx.Cfg("eventmodeling"))
	return utils.CleanAndMerge(merged, nil)
}

// GetState is getState(): it lays out the frames of the AST.
func (d *DB) GetState() (*Context, error) {
	state := initial()
	ast := d.ast
	diagramProps := d.GetDiagramProps()

	if ast == nil {
		return nil, &Error{Message: "No data for EventModel"}
	}

	for index, frame := range ast.Frames {
		textProps, err := d.calculateTextProps(frame, ast.DataEntities, diagramProps)
		if err != nil {
			return nil, err
		}

		if state, err = d.dispatch(state, command{
			kind:      positionFrameKind,
			index:     index,
			frame:     frame,
			textProps: textProps,
		}); err != nil {
			return nil, err
		}

		if hasSourceFrame(frame) {
			var sourceFrames []*EmFrame
			for _, currentFrame := range ast.Frames {
				for _, sf := range frame.SourceFrames {
					if sf == currentFrame.Name {
						sourceFrames = append(sourceFrames, currentFrame)
						break
					}
				}
			}

			for _, sourceFrame := range sourceFrames {
				if state, err = d.dispatch(state, command{
					kind:        positionRelationKind,
					index:       index,
					frame:       frame,
					sourceFrame: sourceFrame,
				}); err != nil {
					return nil, err
				}
			}
		} else {
			if state, err = d.dispatch(state, command{
				kind:  positionRelationKind,
				index: index,
				frame: frame,
			}); err != nil {
				return nil, err
			}
		}
	}

	next := *state
	next.SortedSwimlanesArray = sortedSwimlanesArray(state.Swimlanes)
	return &next, nil
}

// SetAst is setAst(ast).
func (d *DB) SetAst(ast *EventModel) error {
	d.ast = nil
	frameIds := js.NewSet[string]()
	for _, frame := range ast.Frames {
		if frameIds.Has(frame.Name) {
			// The line of the CST node of the frame counts from 0.
			location := " on line " + strconv.Itoa(frame.Line+1)
			return &Error{Message: `Duplicate event modeling frame ID "` + frame.Name + `"` + location}
		}
		frameIds.Add(frame.Name)
	}
	d.ast = ast
	return nil
}

// GetDiagramProps is getDiagramProps().
func (d *DB) GetDiagramProps() DiagramProps {
	return diagramProps
}

// initial is the constant `initial` of db.ts. The functions of the db do
// not change a state: each makes a new one.
func initial() *Context {
	return &Context{
		Boxes:                []*Box{},
		Swimlanes:            js.NewObj[*Swimlane](),
		Relations:            []*Relation{},
		MaxR:                 0,
		SortedSwimlanesArray: []*Swimlane{},
	}
}

// extractNamespace is extractNamespace(entityIdentifier). The second result
// is false when the result is undefined.
func extractNamespace(entityIdentifier string) (string, bool) {
	spl := strings.Split(entityIdentifier, ".")
	if len(spl) == 2 {
		return spl[0], true
	}
	return "", false
}

// extractName is extractName(entityIdentifier).
func extractName(entityIdentifier string) string {
	spl := strings.Split(entityIdentifier, ".")
	if len(spl) == 2 {
		return spl[1]
	}
	return entityIdentifier
}

// findSwimlaneByNamespace is findSwimlaneByNamespace(swimlanes, namespace).
func findSwimlaneByNamespace(swimlanes *js.Obj[*Swimlane], namespace string, hasNamespace bool) *Swimlane {
	if !hasNamespace || len(namespace) == 0 {
		return nil
	}
	for _, swimlane := range swimlanes.Values() {
		if swimlane.HasNamespace && swimlane.Namespace == namespace {
			return swimlane
		}
	}
	return nil
}

// findNextAvailableIndex is findNextAvailableIndex(swimlanes, boundaryMin,
// boundaryMax).
func findNextAvailableIndex(swimlanes *js.Obj[*Swimlane], boundaryMin, boundaryMax float64) int {
	values := []float64{boundaryMin}
	for _, key := range swimlanes.Keys() {
		index := js.ParseInt(key, 0)
		if index > boundaryMin && index < boundaryMax {
			values = append(values, js.ParseInt(key, 0))
		}
	}
	return int(js.Max(values...) + 1)
}

// calculateSwimlaneProps is calculateSwimlaneProps(frame, swimlanes).
func calculateSwimlaneProps(frame *EmFrame, swimlanes *js.Obj[*Swimlane]) SwimlaneProps {
	namespace, hasNamespace := extractNamespace(frame.EntityIdentifier)
	sw := findSwimlaneByNamespace(swimlanes, namespace, hasNamespace)

	// label is `sw.namespace || fallback`.
	label := func(fallback string) string {
		if sw.HasNamespace && sw.Namespace != "" {
			return sw.Namespace
		}
		return fallback
	}
	// An empty namespace is falsy.
	withNamespace := hasNamespace && namespace != ""

	switch frame.ModelEntityType {
	case "ui", "pcr", "processor":
		if sw != nil {
			return SwimlaneProps{
				Index: sw.Index,
				Label: label(diagramProps.LabelUiAutomation),
			}
		} else if withNamespace {
			return SwimlaneProps{
				Index: findNextAvailableIndex(swimlanes, 0, 100),
				Label: diagramProps.LabelUiAutomationPrefix + namespace,
			}
		}
		return SwimlaneProps{Index: 0, Label: diagramProps.LabelUiAutomation}
	case "rmo", "readmodel", "cmd", "command":
		if sw != nil {
			return SwimlaneProps{
				Index: sw.Index,
				Label: label(diagramProps.LabelCommandReadModel),
			}
		} else if withNamespace {
			return SwimlaneProps{
				Index: findNextAvailableIndex(swimlanes, 100, 200),
				Label: diagramProps.LabelCommandReadModelPrefix + namespace,
			}
		}
		return SwimlaneProps{Index: 100, Label: diagramProps.LabelCommandReadModel}
	default: // 'evt', 'event' and the default
		if sw != nil {
			return SwimlaneProps{
				Index: sw.Index,
				Label: label(diagramProps.LabelEvents),
			}
		} else if withNamespace {
			return SwimlaneProps{
				Index: findNextAvailableIndex(swimlanes, 200, 300),
				Label: diagramProps.LabelEventsPrefix + namespace,
			}
		}
		return SwimlaneProps{Index: 200, Label: diagramProps.LabelEvents}
	}
}

// calculateEntityVisualProps is calculateEntityVisualProps(frame).
func (d *DB) calculateEntityVisualProps(frame *EmFrame) (VisualProps, error) {
	themeVariables := d.ctx.Cfg("themeVariables")
	// color is `themeVariables.<key> ?? fallback`.
	color := func(key, fallback string) (any, error) {
		v, ok := js.GetProperty(themeVariables, key)
		if !ok {
			return nil, &Error{Name: "TypeError", Message: "Cannot read properties of " + js.ToString(themeVariables) + " (reading '" + key + "')"}
		}
		if js.IsNullish(v) {
			return fallback, nil
		}
		return v, nil
	}
	props := func(fillKey, fill, strokeKey, stroke string) (VisualProps, error) {
		f, err := color(fillKey, fill)
		if err != nil {
			return VisualProps{}, err
		}
		s, err := color(strokeKey, stroke)
		if err != nil {
			return VisualProps{}, err
		}
		return VisualProps{Fill: f, Stroke: s}, nil
	}
	switch frame.ModelEntityType {
	case "ui":
		return props("emUiFill", "white", "emUiStroke", "#dbdada")
	case "pcr", "processor":
		return props("emProcessorFill", "#edb3f6", "emProcessorStroke", "#b88cbf")
	case "rmo", "readmodel":
		return props("emReadModelFill", "#d3f1a2", "emReadModelStroke", "#a3b732")
	case "cmd", "command":
		return props("emCommandFill", "#bcd6fe", "emCommandStroke", "#679ac3")
	case "evt", "event":
		return props("emEventFill", "#ffb778", "emEventStroke", "#c19a0f")
	default:
		return VisualProps{
			Fill:   "red",
			Stroke: "black",
		}, nil
	}
}

// substring is String.prototype.substring(start, end) on UTF-16 code units.
func substring(units []rune, start, end int) []rune {
	clamp := func(i int) int {
		if i < 0 {
			return 0
		}
		if i > len(units) {
			return len(units)
		}
		return i
	}
	start, end = clamp(start), clamp(end)
	if start > end {
		start, end = end, start
	}
	return units[start:end]
}

// indexOf is String.prototype.indexOf(search) on UTF-16 code units.
func indexOf(units []rune, search string) int {
	pat := js.UTF16Units(search)
	for i := 0; i+len(pat) <= len(units); i++ {
		match := true
		for k, u := range pat {
			if units[i+k] != u {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// lastIndexOf is String.prototype.lastIndexOf(search) on UTF-16 code units.
func lastIndexOf(units []rune, search string) int {
	pat := js.UTF16Units(search)
	for i := len(units) - len(pat); i >= 0; i-- {
		match := true
		for k, u := range pat {
			if units[i+k] != u {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// sanitizeText is sanitizeText(text, config).
func (d *DB) sanitizeText(text string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(text, d.ctx.Config)
}

// calculateTextProps is calculateTextProps(frame, dataEntities,
// diagramProps): the HTML of a box and its size.
func (d *DB) calculateTextProps(frame *EmFrame, dataEntities []*EmDataEntity, diagramProps DiagramProps) (TextProps, error) {
	name, err := d.sanitizeText(extractName(frame.EntityIdentifier))
	if err != nil {
		return TextProps{}, err
	}
	// toHtml is undefined while hasToHtml is false.
	var toHtml string
	hasToHtml := false

	wrapLabelConfig := utils.WrapLabelConfig{
		FontSize:   16.0,
		FontWeight: 700.0,
		FontFamily: `"trebuchet ms", verdana, arial, sans-serif`,
		JoinWith:   "<br/>",
	}

	wrappedName, err := utils.WrapLabel(d.ctx, name, diagramProps.TextMaxWidth, wrapLabelConfig)
	if err != nil {
		return TextProps{}, err
	}
	content := "<b>" + wrappedName + "</b>"

	// data is the common part of the two branches below: the text between
	// the braces, sanitized and wrapped, with spaces that do not break.
	data := func(units []rune) (string, error) {
		units = substring(units, 0, lastIndexOf(units, "}")-1)
		text, err := d.sanitizeText(js.FromUTF16Units(units))
		if err != nil {
			return "", err
		}
		if text, err = utils.WrapLabel(d.ctx, text, diagramProps.TextMaxWidth, wrapLabelConfig); err != nil {
			return "", err
		}
		return strings.ReplaceAll(text, " ", "&nbsp;"), nil
	}

	if frame.DataInlineValue != nil && *frame.DataInlineValue != "" {
		units := js.UTF16Units(*frame.DataInlineValue)
		units = substring(units, indexOf(units, "{")+1, len(units))
		if toHtml, err = data(units); err != nil {
			return TextProps{}, err
		}
		hasToHtml = true
	}

	if frame.DataReference != nil {
		var dataEntity *EmDataEntity
		for _, e := range dataEntities {
			if e.Name == *frame.DataReference {
				dataEntity = e
				break
			}
		}

		if dataEntity != nil {
			units := js.UTF16Units(dataEntity.DataBlockValue)
			units = substring(units, indexOf(units, "{\n")+2, len(units))
			if toHtml, err = data(units); err != nil {
				return TextProps{}, err
			}
			toHtml += "<br/>"
			hasToHtml = true
		}
	}

	hasRenderedData := hasToHtml

	if hasRenderedData {
		content += `<br/><br/><code style="text-align: left; display: block;max-width:` + js.Str(diagramProps.TextMaxWidth) + `px">` + toHtml + `</code>`
	}

	textDimensionConfig := utils.TextDimensionConfig{
		FontSize:   wrapLabelConfig.FontSize,
		FontWeight: wrapLabelConfig.FontWeight,
		FontFamily: wrapLabelConfig.FontFamily,
	}
	dimensions, err := utils.CalculateTextDimensions(d.ctx, content, textDimensionConfig)
	if err != nil {
		return TextProps{}, err
	}

	// this is a temporal workaround until a more complex dimension calculation is in place
	calculatedWidthFix := dimensions.Width
	if hasRenderedData {
		calculatedWidthFix = dimensions.Width / 3
	}

	return TextProps{
		Content: content,
		Width:   calculatedWidthFix,
		Height:  dimensions.Height,
	}, nil
}

// decidePositionFrame is decidePositionFrame(state, command).
func (d *DB) decidePositionFrame(_ *Context, command command) ([]event, error) {
	visual, err := d.calculateEntityVisualProps(command.frame)
	if err != nil {
		return nil, err
	}
	dimension := Dimension{
		Width:  command.textProps.Width + 2*diagramProps.BoxTextPadding,
		Height: command.textProps.Height + 2*diagramProps.BoxTextPadding,
	}

	return []event{{
		kind:      framePositionedKind,
		frame:     command.frame,
		index:     command.index,
		visual:    visual,
		dimension: dimension,
		textProps: command.textProps,
	}}, nil
}

// calculateX is calculateX(swimlane, previousSwimlane, lastBox).
func calculateX(swimlane *Swimlane, previousSwimlane *Swimlane, lastBox *Box) float64 {
	if previousSwimlane == nil {
		return diagramProps.ContentStartX
	}
	if previousSwimlane.Index == swimlane.Index && js.Truthy(swimlane.R) {
		return swimlane.R + diagramProps.BoxPadding
	}

	if lastBox == nil {
		return diagramProps.ContentStartX
	}

	return lastBox.R - diagramProps.BoxOverlap + diagramProps.BoxPadding
}

// calculateMaxRight is calculateMaxRight(swimlanes, swimlaneR).
func calculateMaxRight(swimlanes []*Swimlane, swimlaneR float64) float64 {
	rs := make([]float64, 0, len(swimlanes)+1)
	for _, s := range swimlanes {
		rs = append(rs, s.R)
	}
	rs = append(rs, swimlaneR)
	return js.Max(rs...)
}

// sortedSwimlanesArray is sortedSwimlanesArray(swimlanes).
func sortedSwimlanesArray(swimlanes *js.Obj[*Swimlane]) []*Swimlane {
	values := swimlanes.Values()
	js.SortFunc(values, func(a, b *Swimlane) float64 { return float64(a.Index - b.Index) })
	return values
}

// evolveFramePositioned is evolveFramePositioned(state, event).
func evolveFramePositioned(state *Context, event event) *Context {
	swimlaneProps := calculateSwimlaneProps(event.frame, state.Swimlanes)

	key := strconv.Itoa(swimlaneProps.Index)
	var swimlane *Swimlane
	if state.Swimlanes.Has(key) {
		swimlane = state.Swimlanes.Get(key)
	} else {
		swimlane = &Swimlane{
			SwimlaneProps: SwimlaneProps{
				Index: swimlaneProps.Index,
				Label: swimlaneProps.Label,
			},
			R:         0,
			Y:         float64(float64(swimlaneProps.Index)*diagramProps.SwimlaneMinHeight) + diagramProps.SwimlaneGap,
			Height:    diagramProps.SwimlaneMinHeight,
			MaxHeight: diagramProps.SwimlaneMinHeight,
		}
	}

	var lastBox *Box
	if len(state.Boxes) > 0 {
		lastBox = state.Boxes[len(state.Boxes)-1]
	}
	var previousSwimlane *Swimlane
	if state.HasPreviousSwimlaneNumber {
		// An index that the swimlanes do not have gives undefined.
		previousSwimlane = state.Swimlanes.Get(strconv.Itoa(state.PreviousSwimlaneNumber))
	}

	dimension := Dimension{
		Width: js.Max(
			diagramProps.BoxMinWidth,
			js.Min(diagramProps.BoxMaxWidth, event.dimension.Width),
		) +
			2*diagramProps.BoxPadding,
		Height: js.Max(
			diagramProps.BoxMinHeight,
			js.Min(diagramProps.BoxMaxHeight, event.dimension.Height),
		) +
			2*diagramProps.BoxPadding,
	}

	x := calculateX(swimlane, previousSwimlane, lastBox)
	r := x + dimension.Width + diagramProps.BoxPadding
	maxR := calculateMaxRight(state.Swimlanes.Values(), r)

	swimlane.R = x + dimension.Width
	swimlane.MaxHeight = js.Max(swimlane.MaxHeight, dimension.Height)
	swimlane.Height =
		js.Max(diagramProps.SwimlaneMinHeight, swimlane.MaxHeight) + 2*diagramProps.SwimlanePadding

	box := &Box{
		X:           x,
		Y:           diagramProps.SwimlanePadding + swimlane.Y,
		R:           r,
		Dimension:   dimension,
		LeftSibling: false,
		Swimlane:    swimlane,
		Visual:      event.visual,
		Text:        event.textProps.Content,
		Frame:       event.frame,
		Index:       event.index,
	}

	newState := *state
	newState.Boxes = append(append([]*Box{}, state.Boxes...), box)
	newState.Swimlanes = js.NewObj[*Swimlane]()
	state.Swimlanes.Each(func(k string, v *Swimlane) { newState.Swimlanes.Set(k, v) })
	newState.Swimlanes.Set(strconv.Itoa(swimlane.Index), swimlane)
	newState.PreviousSwimlaneNumber = swimlaneProps.Index
	newState.HasPreviousSwimlaneNumber = true
	newState.PreviousFrame = event.frame
	newState.MaxR = maxR

	// the following swimlane.y recalculation is suboptimal. Additionally
	// the value of Box.y is not taken into account in rendering time.
	// This is fine for the time being, but maybe needs improvement later on.
	swimlanes := sortedSwimlanesArray(newState.Swimlanes)
	if len(swimlanes) > 0 {
		swimlanes[0].Y = 0
	}
	for i := 1; i < len(swimlanes); i++ {
		sw := swimlanes[i]
		prevSw := swimlanes[i-1]

		sw.Y = prevSw.Y + prevSw.Height + diagramProps.SwimlaneGap
	}

	return &newState
}

// isFirstFrame is isFirstFrame(index, frame).
func isFirstFrame(index int, frame *EmFrame) bool {
	return index == 0 && len(frame.SourceFrames) == 0
}

// hasSourceFrame is hasSourceFrame(frame).
func hasSourceFrame(frame *EmFrame) bool {
	return len(frame.SourceFrames) > 0
}

// findBoxByFrame is findBoxByFrame(boxes, frame).
func findBoxByFrame(boxes []*Box, frame *EmFrame) *Box {
	if frame == nil {
		return nil
	}
	for _, box := range boxes {
		if box.Frame.Name == frame.Name {
			return box
		}
	}
	return nil
}

// findBoxByLineIndex is findBoxByLineIndex(boxes, targetSwimlane,
// lineIndex).
func findBoxByLineIndex(boxes []*Box, targetSwimlane int, lineIndex int) *Box {
	if lineIndex < 0 {
		return nil
	}

	for i := lineIndex; i >= 0; i-- {
		box := boxes[i]
		if box.Swimlane.Index != targetSwimlane {
			return box
		}
	}
	return nil
}

// decidePositionRelation is decidePositionRelation(state, command).
func decidePositionRelation(state *Context, command command) ([]event, error) {
	if isFirstFrame(command.index, command.frame) {
		return nil, nil
	}

	targetBox := findBoxByFrame(state.Boxes, command.frame)

	if targetBox == nil {
		return nil, &Error{Message: "Target box not found for frame " + command.frame.Name}
	}

	var sourceBox *Box
	if command.sourceFrame != nil {
		sourceBox = findBoxByFrame(state.Boxes, command.sourceFrame)
	} else {
		// isEmResetFrame(command.frame)
		if command.frame.Type == typeResetFrame {
			return nil, nil
		}
		sourceBox = findBoxByLineIndex(state.Boxes, targetBox.Swimlane.Index, command.index-1)
	}

	if sourceBox == nil {
		// Source box not found for frame ${command.frame.name}
		return nil, nil
	}
	return []event{{
		kind:      relationPositionedKind,
		frame:     command.frame,
		index:     command.index,
		sourceBox: sourceBox,
		targetBox: targetBox,
	}}, nil
}

// evolveRelationPositioned is evolveRelationPositioned(state, event).
func evolveRelationPositioned(state *Context, event event) *Context {
	relation := &Relation{
		Visual: VisualProps{
			Fill:   "none",
			Stroke: "#000",
		},
		Source: Coordinate{
			X: event.sourceBox.X,
			Y: event.sourceBox.Y,
		},
		Target: Coordinate{
			X: event.targetBox.X,
			Y: event.targetBox.Y,
		},
		SourceBox: event.sourceBox,
		TargetBox: event.targetBox,
	}

	newState := *state
	newState.Relations = append(append([]*Relation{}, state.Relations...), relation)
	return &newState
}

// decide is decide(state, command): the deciders by the kind of the
// command.
func (d *DB) decide(state *Context, command command) ([]event, error) {
	switch command.kind {
	case positionFrameKind:
		return d.decidePositionFrame(state, command)
	case positionRelationKind:
		return decidePositionRelation(state, command)
	}
	return nil, nil
}

// evolve is evolve(state, events): the evolvers by the kind of the event.
func evolve(state *Context, events []event) *Context {
	newState := state
	for _, event := range events {
		switch event.kind {
		case framePositionedKind:
			newState = evolveFramePositioned(newState, event)
		case relationPositionedKind:
			newState = evolveRelationPositioned(newState, event)
		}
	}
	return newState
}

// dispatch is dispatch(state, command).
func (d *DB) dispatch(state *Context, command command) (*Context, error) {
	events, err := d.decide(state, command)
	if err != nil {
		return nil, err
	}
	newState := evolve(state, events)
	return newState, nil
}
