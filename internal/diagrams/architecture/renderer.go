// Ported from mermaid 12.2.0, diagrams/architecture/architectureRenderer.ts
// (MIT).

package architecture

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// configNumber is a config field in an arithmetic expression: the number
// that JavaScript converts the value to.
func (db *ArchitectureDB) configNumber(field string) float64 {
	return js.ToNumber(db.GetConfigField(field))
}

// addServices, addJunctions, addGroups and addEdges of upstream give the
// elements to cytoscape. They run in JavaScript, in internal/cytoscape, from
// the lists that layoutArchitecture passes on.

// positionNodes is positionNodes(db, cy).
func positionNodes(db *ArchitectureDB, cy *cytoscape.ArchitectureResult) {
	for _, node := range cy.Nodes {
		if node.Type == "group" {
			continue
		}
		x := node.X
		y := node.Y

		nodeElem := db.GetElementByID(node.ID)
		nodeElem.Attr("transform", "translate("+js.Str(orZero(x))+","+js.Str(orZero(y))+")")
	}
}

// orZero is v || 0.
func orZero(v float64) float64 {
	if v == 0 || math.IsNaN(v) {
		return 0
	}
	return v
}

// getAlignments is getAlignments(db, spatialMaps, groupAlignments,
// layoutHints): the alignmentConstraint option of fcose.
func getAlignments(
	db *ArchitectureDB,
	spatialMaps []ArchitectureSpatialMap,
	groupAlignments ArchitectureGroupAlignments,
	layoutHints []*ArchitectureLayoutHint,
) cytoscape.AlignmentConstraint {
	// flattenAlignments flattens the alignment map so nodes in different
	// groups will be in the same alignment array IFF their groups don't
	// connect in a conflicting alignment
	//
	// i.e., two groups which connect horizontally should not have nodes with
	// vertical alignments to one another
	//
	// See: #5952
	//
	// alignmentMap is the alignment map with the outer key being the row/col
	// # and the inner key being the group name mapped to the nodes on that
	// axis in the group. The result is the flattened alignment map with an
	// arbitrary key mapping to nodes in the same row/col.
	flattenAlignments := func(
		alignmentMap *js.Map[float64, *js.Map[string, *[]string]],
		alignmentDir ArchitectureAlignment,
	) *js.Map[string, []string] {
		// flattened is the mapping of x/y coordinate to an array of the nodes in that row/column
		flattened := js.NewMap[string, []string]()
		for _, numericDir := range alignmentMap.Keys() {
			alignments := alignmentMap.Get(numericDir)
			dir := js.Str(numericDir)
			cnt := 0
			// [group name, array of nodes within the group on axis dir]
			arrKeys := alignments.Keys()
			arrValues := alignments.Values()
			if len(arrKeys) == 1 {
				// If only one group exists in the row/column, we don't need to do anything else
				flattened.Set(dir, *arrValues[0])
				continue
			}
			for i := 0; i < len(arrKeys)-1; i++ {
				for j := i + 1; j < len(arrKeys); j++ {
					aGroupID, aNodeIDs := arrKeys[i], *arrValues[i]
					bGroupID, bNodeIDs := arrKeys[j], *arrValues[j]
					// Get how the two groups are intended to align (undefined if they aren't)
					alignment, hasAlignment := groupAlignments.Lookup(architectureGroupAlignmentKey(aGroupID, bGroupID))

					if hasAlignment && alignment == alignmentDir {
						// If the intended alignment between the two groups is the same as the alignment we are parsing
						// add the node ids of both groups to the axis array in flattened
						flattened.Set(dir, concat(flattened.Get(dir), aNodeIDs, bNodeIDs))
					} else if aGroupID == "default" || bGroupID == "default" {
						// If either of the groups are in the default space (not in a group), use the same behavior as above
						flattened.Set(dir, concat(flattened.Get(dir), aNodeIDs, bNodeIDs))
					} else {
						// Otherwise, the nodes in the two groups are not intended to align
						keyA := dir + "-" + strconv.Itoa(cnt)
						cnt++
						flattened.Set(keyA, aNodeIDs)
						keyB := dir + "-" + strconv.Itoa(cnt)
						cnt++
						flattened.Set(keyB, bNodeIDs)
					}
				}
			}
		}
		return flattened
	}

	type axes struct{ horiz, vert [][]string }
	alignments := make([]axes, len(spatialMaps))
	for i, spatialMap := range spatialMaps {
		horizontalAlignments := js.NewMap[float64, *js.Map[string, *[]string]]()
		verticalAlignments := js.NewMap[float64, *js.Map[string, *[]string]]()

		// Group service ids in a map with their x and y coordinate as the key
		spatialMap.Each(func(id string, pos [2]float64) {
			x, y := pos[0], pos[1]
			nodeGroup := "default"
			if n := db.GetNode(id); n != nil && n.In != nil {
				nodeGroup = *n.In
			}
			horizontalAlignment, ok := horizontalAlignments.Lookup(y)
			if !ok {
				horizontalAlignment = js.NewMap[string, *[]string]()
				horizontalAlignments.Set(y, horizontalAlignment)
			}

			verticalAlignment, ok := verticalAlignments.Lookup(x)
			if !ok {
				verticalAlignment = js.NewMap[string, *[]string]()
				verticalAlignments.Set(x, verticalAlignment)
			}

			for _, alignment := range []*js.Map[string, *[]string]{horizontalAlignment, verticalAlignment} {
				nodeList, ok := alignment.Lookup(nodeGroup)
				if !ok {
					nodeList = &[]string{}
					alignment.Set(nodeGroup, nodeList)
				}
				*nodeList = append(*nodeList, id)
			}
		})

		// Merge the values of each map into a list if the inner list has at least 2 elements
		alignments[i] = axes{
			horiz: longerThanOne(flattenAlignments(horizontalAlignments, "horizontal").Values()),
			vert:  longerThanOne(flattenAlignments(verticalAlignments, "vertical").Values()),
		}
	}

	// Merge the alignment lists for each spatial map into one 2d array per axis
	horizontalRaw, verticalRaw := [][]string{}, [][]string{}
	for _, a := range alignments {
		horizontalRaw = append(horizontalRaw, a.horiz...)
		verticalRaw = append(verticalRaw, a.vert...)
	}

	// Drop any heuristic alignment group that contains a member of a declared layout hint.
	// The user-declared hint takes precedence — keeping both produces conflicting constraints
	// that fcose cannot satisfy and crashes on (RangeError in FDLayout.calcGrid).
	declaredMembers := js.NewSet[string]()
	for _, hint := range layoutHints {
		for _, m := range hint.Members {
			declaredMembers.Add(m)
		}
	}
	dropOverlapping := func(groups [][]string) [][]string {
		out := [][]string{}
		for _, group := range groups {
			some := false
			for _, id := range group {
				if declaredMembers.Has(id) {
					some = true
					break
				}
			}
			if !some {
				out = append(out, group)
			}
		}
		return out
	}

	horizontal := dropOverlapping(horizontalRaw)
	vertical := dropOverlapping(verticalRaw)

	// Append user-declared layout hints. `align row` shares a Y coord (horizontal axis),
	// `align column` shares an X coord (vertical axis).
	for _, hint := range layoutHints {
		if len(hint.Members) < 2 {
			continue
		}
		if hint.Direction == "row" {
			horizontal = append(horizontal, append([]string{}, hint.Members...))
		} else {
			vertical = append(vertical, append([]string{}, hint.Members...))
		}
	}

	return cytoscape.AlignmentConstraint{
		Horizontal: horizontal,
		Vertical:   vertical,
	}
}

// concat is [...a, ...b, ...c].
func concat(a, b, c []string) []string {
	out := make([]string, 0, len(a)+len(b)+len(c))
	out = append(out, a...)
	out = append(out, b...)
	return append(out, c...)
}

// longerThanOne is lists.filter((arr) => arr.length > 1).
func longerThanOne(lists [][]string) [][]string {
	out := [][]string{}
	for _, arr := range lists {
		if len(arr) > 1 {
			out = append(out, arr)
		}
	}
	return out
}

// getRelativeConstraints is getRelativeConstraints(spatialMaps, db,
// layoutHints): the relativePlacementConstraint option of fcose.
func getRelativeConstraints(
	spatialMaps []ArchitectureSpatialMap,
	db *ArchitectureDB,
	layoutHints []*ArchitectureLayoutHint,
) []cytoscape.RelativePlacement {
	relativeConstraints := []cytoscape.RelativePlacement{}

	// Emit a chain of relative placements for each declared layout hint. This is what actually
	// separates members along the axis — the alignment constraint alone only says "share a y/x"
	// but does not order the nodes.
	iconSize := db.configNumber("iconSize")
	idealMult := db.configNumber("idealEdgeLengthMultiplier")
	hintGap := idealMult * iconSize
	// Track every (a,b) pair that the declared hints already constrain. The heuristic BFS
	// below will skip any pair already covered to avoid double constraints that fcose cannot
	// reconcile.
	declaredPairs := js.NewSet[string]()
	for _, hint := range layoutHints {
		for i := 0; i < len(hint.Members)-1; i++ {
			a := hint.Members[i]
			b := hint.Members[i+1]
			declaredPairs.Add(a + "|" + b)
			declaredPairs.Add(b + "|" + a)
			if hint.Direction == "row" {
				relativeConstraints = append(relativeConstraints, cytoscape.RelativePlacement{Left: a, Right: b, Gap: hintGap})
			} else {
				relativeConstraints = append(relativeConstraints, cytoscape.RelativePlacement{Top: a, Bottom: b, Gap: hintGap})
			}
		}
	}
	posToStr := func(pos [2]float64) string { return js.Str(pos[0]) + "," + js.Str(pos[1]) }
	strToPos := func(pos string) [2]float64 {
		parts := strings.Split(pos, ",")
		var out [2]float64
		for i := range out {
			if i < len(parts) {
				out[i] = js.ParseInt(parts[i], 0)
			} else {
				out[i] = math.NaN()
			}
		}
		return out
	}

	// The directions in the order of Object.entries(directions).
	directions := []struct {
		dir   ArchitectureDirection
		shift [2]float64
	}{
		{"L", [2]float64{-1, 0}},
		{"R", [2]float64{1, 0}},
		{"T", [2]float64{0, 1}},
		{"B", [2]float64{0, -1}},
	}

	for _, spatialMap := range spatialMaps {
		invSpatialMap := js.NewMap[string, string]()
		spatialMap.Each(func(key string, value [2]float64) {
			invSpatialMap.Set(posToStr(value), key)
		})

		// perform BFS
		queue := []string{posToStr([2]float64{0, 0})}
		visited := map[string]bool{}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			if curr != "" {
				visited[curr] = true
				currID := invSpatialMap.Get(curr)
				if currID != "" {
					currPos := strToPos(curr)
					for _, d := range directions {
						dir, shift := d.dir, d.shift
						newPos := posToStr([2]float64{currPos[0] + shift[0], currPos[1] + shift[1]})
						newID := invSpatialMap.Get(newPos)
						// If there is an adjacent service to the current one and it has not yet been visited
						if newID != "" && !visited[newPos] {
							queue = append(queue, newPos)
							// Skip the heuristic constraint if the user has already declared a relation
							// between this pair via `align row|column`. Without this, fcose receives both
							// a heuristic and a declared placement for the same pair, which can produce
							// inconsistent solutions and crash the layout (RangeError in calcGrid).
							if declaredPairs.Has(currID + "|" + newID) {
								continue
							}
							// { [name of dir]: newId, [name of the opposite of dir]: currId, gap }
							c := cytoscape.RelativePlacement{Gap: idealMult * iconSize}
							setSide(&c, architectureDirectionName(dir), newID)
							setSide(&c, architectureDirectionName(getOppositeArchitectureDirection(dir)), currID)
							relativeConstraints = append(relativeConstraints, c)
						}
					}
				}
			}
		}
	}
	return relativeConstraints
}

// setSide sets the property of a relative placement by its name.
func setSide(c *cytoscape.RelativePlacement, name, id string) {
	switch name {
	case "left":
		c.Left = id
	case "right":
		c.Right = id
	case "top":
		c.Top = id
	case "bottom":
		c.Bottom = id
	}
}

// seedOf is the seed that withSeededRandom gets, as the number that gives
// the same generator. A seed that is the number 0 opts out. Any other value
// that converts to 0, such as a string, does not: `seed === 0` is false for
// it and `seed >>> 0` is 0. 4294967296 stands for such a seed, because it
// also gives the state 0.
func seedOf(seed any) float64 {
	if n, ok := seed.(float64); ok {
		return n
	}
	n := js.ToNumber(seed)
	if n == 0 {
		return 4294967296
	}
	return n
}

// layoutInput builds the arguments of layoutArchitecture for cytoscape.
func layoutInput(
	services []*ArchitectureNode,
	junctions []*ArchitectureNode,
	groups []*ArchitectureGroup,
	edges []*ArchitectureEdge,
	db *ArchitectureDB,
	ds *ArchitectureDataStructures,
) *cytoscape.ArchitectureInput {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	in := &cytoscape.ArchitectureInput{
		Config: cytoscape.ArchitectureConfig{
			IconSize:                  db.configNumber("iconSize"),
			FontSize:                  db.configNumber("fontSize"),
			Padding:                   db.configNumber("padding"),
			IdealEdgeLengthMultiplier: db.configNumber("idealEdgeLengthMultiplier"),
			EdgeElasticity:            db.configNumber("edgeElasticity"),
			NodeSeparation:            db.configNumber("nodeSeparation"),
			NumIter:                   db.configNumber("numIter"),
			Randomize:                 js.Truthy(db.GetConfigField("randomize")),
			Seed:                      seedOf(db.GetConfigField("seed")),
		},
	}
	// addGroups
	for _, group := range groups {
		in.Groups = append(in.Groups, cytoscape.ArchitectureNode{
			ID:    group.ID,
			Icon:  deref(group.Icon),
			Title: deref(group.Title),
			In:    deref(group.In),
		})
	}
	// addServices
	for _, service := range services {
		in.Services = append(in.Services, cytoscape.ArchitectureNode{
			ID:    service.ID,
			Icon:  deref(service.Icon),
			Title: deref(service.Title),
			In:    deref(service.In),
		})
	}
	// addJunctions
	for _, junction := range junctions {
		in.Junctions = append(in.Junctions, cytoscape.ArchitectureNode{
			ID: junction.ID,
			In: deref(junction.In),
		})
	}
	// addEdges
	for _, parsedEdge := range edges {
		in.Edges = append(in.Edges, cytoscape.ArchitectureEdge{
			LhsID:  parsedEdge.LhsID,
			RhsID:  parsedEdge.RhsID,
			LhsDir: parsedEdge.LhsDir,
			RhsDir: parsedEdge.RhsDir,
			Title:  deref(parsedEdge.Title),
		})
	}
	layoutHints := db.GetLayoutHints()
	// Use the spatial map to create alignment arrays for fcose, then merge in any
	// user-declared `align row|column` hints.
	in.Alignment = getAlignments(db, ds.SpatialMaps, ds.GroupAlignments, layoutHints)

	// Create the relative constraints for fcose by using an inverse of the spatial map and performing BFS on it
	in.RelativePlacement = getRelativeConstraints(ds.SpatialMaps, db, layoutHints)
	return in
}

// layoutArchitecture is layoutArchitecture(services, junctions, groups,
// edges, db, ds). The statements that talk to cytoscape run in JavaScript:
// the style sheet, the elements, the fcose layout with its options, the
// layoutstop handler that bends the XY edges, and withSeededRandom
// (architectureSeed.ts). The result has what the draw function reads from
// the instance afterwards.
func layoutArchitecture(
	ctx *render.Context,
	services []*ArchitectureNode,
	junctions []*ArchitectureNode,
	groups []*ArchitectureGroup,
	edges []*ArchitectureEdge,
	db *ArchitectureDB,
	ds *ArchitectureDataStructures,
) (*cytoscape.ArchitectureResult, error) {
	in := layoutInput(services, junctions, groups, edges, db, ds)
	env := cytoscape.Env{
		Random:      ctx.Random,
		Now:         ctx.Now,
		MeasureText: canvasMeasureText(ctx),
	}
	observer, _ := ctx.Value(layoutObserverKey, func() any { return (*layoutObserver)(nil) }).(*layoutObserver)
	if observer != nil {
		observer.input = in
		env = observer.wrap(env)
	}
	cy, err := cytoscape.Architecture(ctx.Cancel, env, in)
	if observer != nil {
		observer.result = cy
	}
	if err != nil {
		ctx.CheckCanceled()
		var jsErr *cytoscape.Error
		if errors.As(err, &jsErr) {
			// The error that the layout throws, with its own message.
			if jsErr.Name == "" || jsErr.Name == "Error" {
				return nil, errors.New(jsErr.Message)
			}
			return nil, &layoutError{name: jsErr.Name, message: jsErr.Message}
		}
		return nil, err
	}
	// The await of the caller.
	ctx.Await()
	return cy, nil
}

// layoutObserver sees the call of cytoscape of one render: the input, the
// environment and the result. The golden test of the layout puts one in the
// render context under layoutObserverKey. A render has none.
type layoutObserver struct {
	// wrap returns the environment that the layout runs with.
	wrap   func(env cytoscape.Env) cytoscape.Env
	input  *cytoscape.ArchitectureInput
	result *cytoscape.ArchitectureResult
}

const layoutObserverKey = "architecture.layoutObserver"

// layoutError is an error of JavaScript that is not a plain Error, for
// example a TypeError that cytoscape throws.
type layoutError struct{ name, message string }

func (e *layoutError) Error() string { return e.name + ": " + e.message }

// canvasMeasureText returns context.measureText(text).width of a canvas 2D
// context for the labels that cytoscape measures. cytoscape sets the font
// "<font-style> <font-weight> <font-size>px <font-family>" and measures each
// line of a label.
//
// The width of a canvas is not the width of an SVG text element with the
// same font: no layout rounds it. A canvas replaces each white space
// character with a space and collapses nothing. The widths that Chromium
// records for the labels of the golden file (testdata/layout.json.gz) are,
// bit for bit, the float sum of the widths of the words of the text, where
// each word is shaped alone and a space is a word of its own. A float sum
// over the glyphs of the whole text differs from them in the last bit for
// two long labels of that file.
func canvasMeasureText(ctx *render.Context) func(font, text string) float64 {
	return func(fontText, text string) float64 {
		if text == "" {
			return 0
		}
		f, ok := parseCanvasFont(fontText)
		if !ok {
			// A canvas keeps its font when the new value is not valid. The
			// font of a new canvas is "10px sans-serif".
			f = canvasFont{size: 10, weight: 400, families: []string{"sans-serif"}}
		}
		// Text preparation: replace all ASCII whitespace with U+0020 SPACE.
		runes := []rune(text)
		for i, r := range runes {
			switch r {
			case '\t', '\n', '\f', '\r':
				runes[i] = ' '
			}
		}
		chain := ctx.Engine.Fonts.Resolve(f.families, f.weight, f.italic)
		var width float32
		for start := 0; start < len(runes); {
			end := nextWordEnd(runes, start)
			width += float32(font.TextWidth(chain.ShapeText(runes[start:end], f.size)))
			start = end
		}
		return float64(width)
	}
}

// nextWordEnd returns the end of the word of plain text that starts at
// start. A space is a word of its own, and another word ends in front of the
// next space. A character of a script that writes no spaces between words
// is a word of its own, with the marks that follow it.
//
// The rule for ideographs, symbols and emoji is an assumption: the golden
// file has one label with ideographs and an emoji, and its width is the
// width of Chromium. Another grouping can change the last bit of the sum.
// cytoscape rounds the width up to a whole number, so that bit matters only
// for a width near a whole number.
func nextWordEnd(text []rune, start int) int {
	if start+1 >= len(text) || text[start] == ' ' {
		return start + 1
	}
	if !isCJKIdeographOrSymbol(text[start]) {
		for end := start + 1; end < len(text); end++ {
			if text[end] == ' ' || isCJKIdeographOrSymbol(text[end]) {
				return end
			}
		}
		return len(text)
	}
	for end := start + 1; end < len(text); end++ {
		ch := text[end]
		// A modifier does not split a sequence.
		if unicode.In(ch, unicode.M, unicode.Lm, unicode.Sk) || ch == 0x200d || (ch >= 0xfe00 && ch <= 0xfe0f) {
			continue
		}
		return end
	}
	return len(text)
}

// isCJKIdeographOrSymbol tells whether a character belongs to a script that
// writes no spaces between words, or is a symbol or an emoji that Blink
// treats in the same way.
func isCJKIdeographOrSymbol(r rune) bool {
	switch {
	case r < 0x2e80:
		// Symbols and pictographs below the CJK blocks.
		return (r >= 0x2190 && r <= 0x21ff) || (r >= 0x2300 && r <= 0x23ff) || (r >= 0x2460 && r <= 0x27bf) || (r >= 0x2b00 && r <= 0x2bff)
	case r <= 0x33ff, // CJK radicals, symbols, kana, bopomofo, enclosed and compatibility forms
		r >= 0x3400 && r <= 0x4dbf,   // CJK extension A
		r >= 0x4e00 && r <= 0x9fff,   // CJK unified ideographs
		r >= 0xf900 && r <= 0xfaff,   // CJK compatibility ideographs
		r >= 0xfe30 && r <= 0xfe4f,   // CJK compatibility forms
		r >= 0xff00 && r <= 0xffef,   // halfwidth and fullwidth forms
		r >= 0x1f000 && r <= 0x1faff, // emoji and pictographs
		r >= 0x20000 && r <= 0x3ffff: // CJK extensions B and later
		return true
	}
	return false
}

// canvasFont is the value of the font property of a canvas context.
type canvasFont struct {
	italic   bool
	weight   int
	size     float64
	families []string
}

// parseCanvasFont reads a font value in the form that cytoscape writes:
// "<font-style> <font-weight> <font-size>px <font-family>". The second
// result is false for a value that is not a valid font.
func parseCanvasFont(value string) (canvasFont, bool) {
	var f canvasFont
	fields := strings.SplitN(strings.TrimSpace(value), " ", 4)
	if len(fields) != 4 {
		return f, false
	}
	switch fields[0] {
	case "normal":
	case "italic", "oblique":
		f.italic = true
	default:
		return f, false
	}
	switch fields[1] {
	case "normal":
		f.weight = 400
	case "bold", "bolder":
		f.weight = 700
	case "lighter":
		f.weight = 100
	default:
		w, err := strconv.Atoi(fields[1])
		if err != nil || w < 1 || w > 1000 {
			return f, false
		}
		f.weight = w
	}
	size, ok := strings.CutSuffix(fields[2], "px")
	if !ok {
		return f, false
	}
	f.size = js.Number(size)
	if math.IsNaN(f.size) || math.IsInf(f.size, 0) || f.size < 0 {
		return f, false
	}
	for _, family := range strings.Split(fields[3], ",") {
		family = strings.TrimSpace(family)
		if family == "" {
			return f, false
		}
		f.families = append(f.families, family)
	}
	return f, true
}

// draw is draw(text, id, _version, diagObj).
func draw(ctx *render.Context, db *ArchitectureDB, _ string) error {
	// TODO: Add title support for architecture diagrams

	// The module registers its icon pack when it loads. Here the packs
	// belong to the render.
	if err := ru.RegisterIconPacks(ctx, ru.IconLoader{
		Name:  architectureIconsPrefix,
		Icons: architectureIcons(),
	}); err != nil {
		return err
	}

	id := ctx.ID
	db.SetDiagramID(id)

	services := db.GetServices()
	junctions := db.GetJunctions()
	groups := db.GetGroups()
	edges := db.GetEdges()
	ds, err := db.GetDataStructures()
	if err != nil {
		return err
	}

	svg := ctx.SelectSvgElement()

	edgesElem := svg.Append("g")
	edgesElem.Attr("class", "architecture-edges")

	servicesElem := svg.Append("g")
	servicesElem.Attr("class", "architecture-services")

	groupElem := svg.Append("g")
	groupElem.Attr("class", "architecture-groups")

	if err := drawServices(ctx, db, servicesElem, services, id); err != nil {
		return err
	}
	drawJunctions(db, servicesElem, junctions, id)

	cy, err := layoutArchitecture(ctx, services, junctions, groups, edges, db, ds)
	if err != nil {
		return err
	}

	if err := drawEdges(ctx, edgesElem, cy, db, id); err != nil {
		return err
	}
	if err := drawGroups(ctx, groupElem, cy, db, id); err != nil {
		return err
	}
	positionNodes(db, cy)

	render.SetupGraphViewbox(svg, db.configNumber("padding"), js.Truthy(db.GetConfigField("useMaxWidth")))
	return nil
}
