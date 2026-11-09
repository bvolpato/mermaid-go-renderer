// Ported from mermaid 12.2.0, diagrams/git/gitGraphRenderer.ts (MIT).

package git

import (
	"errors"
	"math"
	"regexp"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

type branchPosition struct {
	pos   float64
	index int
}

type commitPosition struct {
	x, y float64
}

type commitPositionOffset struct {
	x, y          float64
	posWithOffset float64
}

const (
	layoutOffset = 10.0
	commitStep   = 40.0
	px           = 4.0
	py           = 2.0

	themeColorLimit = 8.0

	// reduxBranchLabelPaddingY is the vertical padding inside the LR branch
	// label rect for redux themes (labelPaddingY of drawBranches).
	reduxBranchLabelPaddingY = 12.0

	defaultPos = 30.0
)

// reduxGeometryThemes are the themes that use redux-style geometry: smaller
// commit bullets and sharper label edges. The neo themes are not in the set.
func reduxGeometryThemes(theme any) bool {
	switch themeName(theme) {
	case "redux", "redux-dark", "redux-color", "redux-dark-color":
		return true
	}
	return false
}

// colorThemes are the themes that use per-branch color cycling with a
// first color that is not the default.
func colorThemes(theme any) bool {
	switch themeName(theme) {
	case "redux-color", "redux-dark-color":
		return true
	}
	return false
}

// darkThemes are the themes rendered on a dark background.
func darkThemes(theme any) bool {
	switch themeName(theme) {
	case "dark", "redux-dark", "redux-dark-color", "neo-dark":
		return true
	}
	return false
}

// themeName is `theme ?? ""` as a key of a Set of strings. A value that is
// not a string is in no set.
func themeName(theme any) string {
	s, _ := theme.(string)
	return s
}

// calcColorIndex maps a raw branch index to a CSS color-class index. When
// avoidDefaultColor is true, the branches after the first cycle through
// 1 to limit-1, so that color 0 is not used again.
func calcColorIndex(rawIndex, limit float64, avoidDefaultColor bool) float64 {
	if avoidDefaultColor && rawIndex > 0 {
		return math.Mod(rawIndex-1, limit-1) + 1
	}
	return math.Mod(rawIndex, limit)
}

// renderer holds the module-level state of gitGraphRenderer.ts.
type renderer struct {
	ctx *render.Context

	branchPos      map[string]branchPosition
	commitPos      map[string]commitPosition
	allCommitsDict *js.Map[string, *Commit]
	lanes          []float64
	maxPos         float64
	dir            string
}

// reRows is the separator of drawText: /\\n|\n|<br\s*\/?>/gi.
var reRows = regexp.MustCompile(`\\[nN]|\n|<[bB][rR][\t\n\x0B\f\r \x{A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]*/?>`)

// drawText is drawText(txt) for a string.
func (r *renderer) drawText(txt string) *dom.Node {
	doc := r.ctx.Doc
	svgLabel := doc.CreateElementNS(dom.NSSVG, "text")
	rows := reRows.Split(txt, -1)

	for _, row := range rows {
		tspan := doc.CreateElementNS(dom.NSSVG, "tspan")
		tspan.SetAttributeRaw(dom.NSXML, "xml", "space", "preserve")
		tspan.Attr("dy", "1em")
		tspan.Attr("x", "0")
		tspan.Attr("class", "row")
		tspan.Text(js.TrimSpace(row))
		svgLabel.AppendChild(tspan)
	}

	return svgLabel
}

// findClosestParent is findClosestParent(parents). The second result is
// false when the result is undefined.
func (r *renderer) findClosestParent(parents []string) (string, bool) {
	var closestParent string
	found := false
	var comparisonFunc func(a, b float64) bool
	var targetPosition float64
	if r.dir == "BT" {
		comparisonFunc = func(a, b float64) bool { return a <= b }
		targetPosition = math.Inf(1)
	} else {
		comparisonFunc = func(a, b float64) bool { return a >= b }
		targetPosition = 0
	}

	for _, parent := range parents {
		p, ok := r.commitPos[parent]
		if !ok {
			continue
		}
		parentPosition := p.x
		if r.dir == "TB" || r.dir == "BT" {
			parentPosition = p.y
		}

		if comparisonFunc(parentPosition, targetPosition) {
			closestParent = parent
			found = true
			targetPosition = parentPosition
		}
	}

	return closestParent, found
}

// findClosestParentBT is findClosestParentBT(parents).
func (r *renderer) findClosestParentBT(parents []string) (string, error) {
	closestParent := ""
	maxPosition := math.Inf(1)

	for _, parent := range parents {
		p, ok := r.commitPos[parent]
		if !ok {
			return "", errors.New("Cannot read properties of undefined (reading 'y')")
		}
		parentPosition := p.y
		if parentPosition <= maxPosition {
			closestParent = parent
			maxPosition = parentPosition
		}
	}
	return closestParent, nil
}

// setParallelBTPos is setParallelBTPos(sortedKeys, commits, defaultPos).
func (r *renderer) setParallelBTPos(sortedKeys []string, commits *js.Map[string, *Commit], defaultPos float64) error {
	curPos := defaultPos
	maxPosition := defaultPos
	roots := []*Commit{}

	for _, key := range sortedKeys {
		commit := commits.Get(key)
		if commit == nil {
			return errors.New("Commit not found for key " + key)
		}

		if len(commit.Parents) > 0 {
			var err error
			curPos, err = r.calculateCommitPosition(commit)
			if err != nil {
				return err
			}
			maxPosition = js.Max(curPos, maxPosition)
		} else {
			roots = append(roots, commit)
		}
		if _, err := r.setCommitPosition(commit, curPos); err != nil {
			return err
		}
	}

	curPos = maxPosition
	for _, commit := range roots {
		if err := r.setRootPosition(commit, curPos, defaultPos); err != nil {
			return err
		}
	}
	for _, key := range sortedKeys {
		commit := commits.Get(key)

		if commit != nil && len(commit.Parents) > 0 {
			closestParent, err := r.findClosestParentBT(commit.Parents)
			if err != nil {
				return err
			}
			cp, ok := r.commitPos[closestParent]
			if !ok {
				return errors.New("Cannot read properties of undefined (reading 'y')")
			}
			curPos = cp.y - commitStep
			if curPos <= maxPosition {
				maxPosition = curPos
			}
			branch, ok := r.branchPos[commit.Branch]
			if !ok {
				return errors.New("Cannot read properties of undefined (reading 'pos')")
			}
			x := branch.pos
			y := curPos - layoutOffset
			r.commitPos[commit.ID] = commitPosition{x: x, y: y}
		}
	}
	return nil
}

// findClosestParentPos is findClosestParentPos(commit).
func (r *renderer) findClosestParentPos(commit *Commit) (float64, error) {
	closestParent, ok := r.findClosestParent(commit.Parents)
	if !ok || closestParent == "" {
		return 0, errors.New("Closest parent not found for commit " + commit.ID)
	}

	closestParentPos, ok := r.commitPos[closestParent]
	if !ok {
		return 0, errors.New("Closest parent position not found for commit " + commit.ID)
	}
	return closestParentPos.y, nil
}

// calculateCommitPosition is calculateCommitPosition(commit).
func (r *renderer) calculateCommitPosition(commit *Commit) (float64, error) {
	closestParentPos, err := r.findClosestParentPos(commit)
	if err != nil {
		return 0, err
	}
	return closestParentPos + commitStep, nil
}

// setCommitPosition is setCommitPosition(commit, curPos).
func (r *renderer) setCommitPosition(commit *Commit, curPos float64) (commitPosition, error) {
	branch, ok := r.branchPos[commit.Branch]

	if !ok {
		return commitPosition{}, errors.New("Branch not found for commit " + commit.ID)
	}

	x := branch.pos
	y := curPos + layoutOffset
	r.commitPos[commit.ID] = commitPosition{x: x, y: y}
	return commitPosition{x: x, y: y}, nil
}

// setRootPosition is setRootPosition(commit, curPos, defaultPos).
func (r *renderer) setRootPosition(commit *Commit, curPos, defaultPos float64) error {
	branch, ok := r.branchPos[commit.Branch]
	if !ok {
		return errors.New("Branch not found for commit " + commit.ID)
	}

	y := curPos + defaultPos
	x := branch.pos
	r.commitPos[commit.ID] = commitPosition{x: x, y: y}
	return nil
}

// reduxNum is `useReduxGeometry ? a : b`.
func reduxNum(useReduxGeometry bool, a, b float64) float64 {
	if useReduxGeometry {
		return a
	}
	return b
}

// drawCommitBullet is drawCommitBullet(gBullets, commit, commitPosition,
// typeClass, branchIndex, commitSymbolType).
func (r *renderer) drawCommitBullet(gBullets *dom.Node, commit *Commit, commitPosition commitPositionOffset, typeClass string, branchIndex float64, commitSymbolType int) {
	theme := r.ctx.Cfg("theme")
	useReduxGeometry := reduxGeometryThemes(theme)
	useColorTheme := colorThemes(theme)
	isDark := darkThemes(theme)
	colorIndex := js.Str(calcColorIndex(branchIndex, themeColorLimit, useColorTheme))
	fill := "#fff"
	if isDark {
		fill = "#000000"
	}
	switch commitSymbolType {
	case commitTypeHighlight:
		gBullets.
			Append("rect").
			Attr("x", commitPosition.x-10+reduxNum(useReduxGeometry, 3, 0)).
			Attr("y", commitPosition.y-10+reduxNum(useReduxGeometry, 3, 0)).
			Attr("width", reduxNum(useReduxGeometry, 14, 20)).
			Attr("height", reduxNum(useReduxGeometry, 14, 20)).
			Attr("class", "commit "+commit.ID+" commit-highlight"+colorIndex+" "+typeClass+"-outer")
		gBullets.
			Append("rect").
			Attr("x", commitPosition.x-6+reduxNum(useReduxGeometry, 2, 0)).
			Attr("y", commitPosition.y-6+reduxNum(useReduxGeometry, 2, 0)).
			Attr("width", reduxNum(useReduxGeometry, 8, 12)).
			Attr("height", reduxNum(useReduxGeometry, 8, 12)).
			Attr("class", "commit "+commit.ID+" commit"+colorIndex+" "+typeClass+"-inner")
	case commitTypeCherryPick:
		gBullets.
			Append("circle").
			Attr("cx", commitPosition.x).
			Attr("cy", commitPosition.y).
			Attr("r", reduxNum(useReduxGeometry, 7, 10)).
			Attr("class", "commit "+commit.ID+" "+typeClass)
		gBullets.
			Append("circle").
			Attr("cx", commitPosition.x-3).
			Attr("cy", commitPosition.y+2).
			Attr("r", reduxNum(useReduxGeometry, 2.5, 2.75)).
			Attr("fill", fill).
			Attr("class", "commit "+commit.ID+" "+typeClass)
		gBullets.
			Append("circle").
			Attr("cx", commitPosition.x+3).
			Attr("cy", commitPosition.y+2).
			Attr("r", reduxNum(useReduxGeometry, 2.5, 2.75)).
			Attr("fill", fill).
			Attr("class", "commit "+commit.ID+" "+typeClass)
		gBullets.
			Append("line").
			Attr("x1", commitPosition.x+3).
			Attr("y1", commitPosition.y+1).
			Attr("x2", commitPosition.x).
			Attr("y2", commitPosition.y-5).
			Attr("stroke", fill).
			Attr("class", "commit "+commit.ID+" "+typeClass)
		gBullets.
			Append("line").
			Attr("x1", commitPosition.x-3).
			Attr("y1", commitPosition.y+1).
			Attr("x2", commitPosition.x).
			Attr("y2", commitPosition.y-5).
			Attr("stroke", fill).
			Attr("class", "commit "+commit.ID+" "+typeClass)
	default:
		circle := gBullets.Append("circle")
		circle.Attr("cx", commitPosition.x)
		circle.Attr("cy", commitPosition.y)
		circle.Attr("r", reduxNum(useReduxGeometry, 7, 10))
		circle.Attr("class", "commit "+commit.ID+" commit"+colorIndex)
		if commitSymbolType == commitTypeMerge {
			circle2 := gBullets.Append("circle")
			circle2.Attr("cx", commitPosition.x)
			circle2.Attr("cy", commitPosition.y)
			circle2.Attr("r", reduxNum(useReduxGeometry, 5, 6))
			circle2.Attr("class", "commit "+typeClass+" "+commit.ID+" commit"+colorIndex)
		}
		if commitSymbolType == commitTypeReverse {
			cross := gBullets.Append("path")
			constValue := reduxNum(useReduxGeometry, 4, 5)
			cross.
				Attr("d", "M "+js.Str(commitPosition.x-constValue)+","+js.Str(commitPosition.y-constValue)+
					"L"+js.Str(commitPosition.x+constValue)+","+js.Str(commitPosition.y+constValue)+
					"M"+js.Str(commitPosition.x-constValue)+","+js.Str(commitPosition.y+constValue)+
					"L"+js.Str(commitPosition.x+constValue)+","+js.Str(commitPosition.y-constValue)).
				Attr("class", "commit "+typeClass+" "+commit.ID+" commit"+colorIndex)
		}
	}
}

// drawCommitLabel is drawCommitLabel(gLabels, commit, commitPosition, pos,
// gitGraphConfig).
func (r *renderer) drawCommitLabel(gLabels *dom.Node, commit *Commit, commitPosition commitPositionOffset, pos float64, gitGraphConfig *js.Obj[any]) {
	if commit.Type != commitTypeCherryPick &&
		((commit.CustomID && commit.Type == commitTypeMerge) || commit.Type != commitTypeMerge) &&
		js.Truthy(gitGraphConfig.Get("showCommitLabel")) {
		wrapper := gLabels.Append("g")
		labelBkg := wrapper.Insert("rect").Attr("class", "commit-label-bkg")
		text := wrapper.
			Append("text").
			Attr("x", pos).
			Attr("y", commitPosition.y+25).
			Attr("class", "commit-label").
			Text(commit.ID)
		bbox := text.BBox()

		labelBkg.
			Attr("x", commitPosition.posWithOffset-float64(bbox.Width/2)-py).
			Attr("y", commitPosition.y+13.5).
			Attr("width", bbox.Width+2*py).
			Attr("height", bbox.Height+2*py)

		if r.dir == "TB" || r.dir == "BT" {
			labelBkg.
				Attr("x", commitPosition.x-(bbox.Width+4*px+5)).
				Attr("y", commitPosition.y-12)
			text.
				Attr("x", commitPosition.x-(bbox.Width+4*px)).
				Attr("y", commitPosition.y+bbox.Height-12)
		} else {
			text.Attr("x", commitPosition.posWithOffset-float64(bbox.Width/2))
		}

		if js.Truthy(gitGraphConfig.Get("rotateCommitLabel")) {
			if r.dir == "TB" || r.dir == "BT" {
				text.Attr("transform", "rotate("+js.Str(-45)+", "+js.Str(commitPosition.x)+", "+js.Str(commitPosition.y)+")")
				labelBkg.Attr("transform", "rotate("+js.Str(-45)+", "+js.Str(commitPosition.x)+", "+js.Str(commitPosition.y)+")")
			} else {
				rX := -7.5 - float64(((bbox.Width+10)/25)*9.5)
				rY := 10 + float64((bbox.Width/25)*8.5)
				wrapper.Attr("transform",
					"translate("+
						js.Str(rX)+
						", "+
						js.Str(rY)+
						") rotate("+
						js.Str(-45)+
						", "+
						js.Str(pos)+
						", "+
						js.Str(commitPosition.y)+
						")")
			}
		}
	}
}

type tagElement struct {
	tag, hole, rect *dom.Node
	yOffset         float64
}

// drawCommitTags is drawCommitTags(gLabels, commit, commitPosition, pos).
func (r *renderer) drawCommitTags(gLabels *dom.Node, commit *Commit, commitPosition commitPositionOffset, pos float64) {
	if len(commit.Tags) > 0 {
		yOffset := 0.0
		maxTagBboxWidth := 0.0
		maxTagBboxHeight := 0.0
		tagElements := []tagElement{}

		// commit.tags.reverse() reverses the array of the commit in place.
		for i, j := 0, len(commit.Tags)-1; i < j; i, j = i+1, j-1 {
			commit.Tags[i], commit.Tags[j] = commit.Tags[j], commit.Tags[i]
		}
		for _, tagValue := range commit.Tags {
			rect := gLabels.Insert("polygon")
			hole := gLabels.Append("circle")
			tag := gLabels.
				Append("text").
				Attr("y", commitPosition.y-16-yOffset).
				Attr("class", "tag-label").
				Text(tagValue)
			tagBbox := tag.BBox()

			maxTagBboxWidth = js.Max(maxTagBboxWidth, tagBbox.Width)
			maxTagBboxHeight = js.Max(maxTagBboxHeight, tagBbox.Height)

			tag.Attr("x", commitPosition.posWithOffset-float64(tagBbox.Width/2))

			tagElements = append(tagElements, tagElement{
				tag:     tag,
				hole:    hole,
				rect:    rect,
				yOffset: yOffset,
			})

			yOffset += 20
		}

		for _, el := range tagElements {
			tag, hole, rect, yOffset := el.tag, el.hole, el.rect, el.yOffset
			h2 := float64(maxTagBboxHeight / 2)
			ly := commitPosition.y - 19.2 - yOffset
			rect.Attr("class", "tag-label-bkg").Attr(
				"points",
				"\n"+
					"      "+js.Str(pos-float64(maxTagBboxWidth/2)-px/2)+","+js.Str(ly+py)+"  \n"+
					"      "+js.Str(pos-float64(maxTagBboxWidth/2)-px/2)+","+js.Str(ly-py)+"\n"+
					"      "+js.Str(commitPosition.posWithOffset-float64(maxTagBboxWidth/2)-px)+","+js.Str(ly-h2-py)+"\n"+
					"      "+js.Str(commitPosition.posWithOffset+float64(maxTagBboxWidth/2)+px)+","+js.Str(ly-h2-py)+"\n"+
					"      "+js.Str(commitPosition.posWithOffset+float64(maxTagBboxWidth/2)+px)+","+js.Str(ly+h2+py)+"\n"+
					"      "+js.Str(commitPosition.posWithOffset-float64(maxTagBboxWidth/2)-px)+","+js.Str(ly+h2+py),
			)

			hole.
				Attr("cy", ly).
				Attr("cx", pos-float64(maxTagBboxWidth/2)+px/2).
				Attr("r", 1.5).
				Attr("class", "tag-hole")

			if r.dir == "TB" || r.dir == "BT" {
				yOrigin := pos + yOffset

				rect.
					Attr("class", "tag-label-bkg").
					Attr(
						"points",
						"\n"+
							"        "+js.Str(commitPosition.x)+","+js.Str(yOrigin+2)+"\n"+
							"        "+js.Str(commitPosition.x)+","+js.Str(yOrigin-2)+"\n"+
							"        "+js.Str(commitPosition.x+layoutOffset)+","+js.Str(yOrigin-h2-2)+"\n"+
							"        "+js.Str(commitPosition.x+layoutOffset+maxTagBboxWidth+4)+","+js.Str(yOrigin-h2-2)+"\n"+
							"        "+js.Str(commitPosition.x+layoutOffset+maxTagBboxWidth+4)+","+js.Str(yOrigin+h2+2)+"\n"+
							"        "+js.Str(commitPosition.x+layoutOffset)+","+js.Str(yOrigin+h2+2),
					).
					Attr("transform", "translate(12,12) rotate(45, "+js.Str(commitPosition.x)+","+js.Str(pos)+")")
				hole.
					Attr("cx", commitPosition.x+px/2).
					Attr("cy", yOrigin).
					Attr("transform", "translate(12,12) rotate(45, "+js.Str(commitPosition.x)+","+js.Str(pos)+")")
				tag.
					Attr("x", commitPosition.x+5).
					Attr("y", yOrigin+3).
					Attr("transform", "translate(14,14) rotate(45, "+js.Str(commitPosition.x)+","+js.Str(pos)+")")
			}
		}
	}
}

// commitSymbolTypeOf is `commit.customType ?? commit.type`.
func commitSymbolTypeOf(commit *Commit) int {
	if commit.CustomType != nil {
		return *commit.CustomType
	}
	return commit.Type
}

// getCommitClassType is getCommitClassType(commit).
func getCommitClassType(commit *Commit) string {
	commitSymbolType := commitSymbolTypeOf(commit)
	switch commitSymbolType {
	case commitTypeNormal:
		return "commit-normal"
	case commitTypeReverse:
		return "commit-reverse"
	case commitTypeHighlight:
		return "commit-highlight"
	case commitTypeMerge:
		return "commit-merge"
	case commitTypeCherryPick:
		return "commit-cherry-pick"
	default:
		return "commit-normal"
	}
}

// calculatePosition is calculatePosition(commit, dir, pos, commitPos).
func (r *renderer) calculatePosition(commit *Commit, dir string) float64 {
	defaultCommitPosition := commitPosition{x: 0, y: 0} // Default position if commit is not found

	if len(commit.Parents) > 0 {
		closestParent, ok := r.findClosestParent(commit.Parents)
		if ok && closestParent != "" {
			parentPosition, ok := r.commitPos[closestParent]
			if !ok {
				parentPosition = defaultCommitPosition
			}

			if dir == "TB" {
				return parentPosition.y + commitStep
			} else if dir == "BT" {
				currentPosition, ok := r.commitPos[commit.ID]
				if !ok {
					currentPosition = defaultCommitPosition
				}
				return currentPosition.y - commitStep
			} else {
				return parentPosition.x + commitStep
			}
		}
	} else {
		if dir == "TB" {
			return defaultPos
		} else if dir == "BT" {
			currentPosition, ok := r.commitPos[commit.ID]
			if !ok {
				currentPosition = defaultCommitPosition
			}
			return currentPosition.y - commitStep
		} else {
			return 0
		}
	}
	return 0
}

// getCommitPosition is getCommitPosition(commit, pos, isParallelCommits).
func (r *renderer) getCommitPosition(commit *Commit, pos float64, isParallelCommits bool) (commitPositionOffset, error) {
	posWithOffset := pos + layoutOffset
	if r.dir == "BT" && isParallelCommits {
		posWithOffset = pos
	}
	branch, ok := r.branchPos[commit.Branch]
	if !ok {
		return commitPositionOffset{}, errors.New("Position were undefined for commit " + commit.ID)
	}
	branchY := branch.pos
	x := posWithOffset
	if r.dir == "TB" || r.dir == "BT" {
		x = branch.pos
	}
	useReduxGeometry := reduxGeometryThemes(r.ctx.Cfg("theme"))
	var y float64
	if r.dir == "TB" || r.dir == "BT" {
		y = posWithOffset
	} else {
		y = branchY + reduxNum(useReduxGeometry, reduxBranchLabelPaddingY/2+1, -2)
	}
	return commitPositionOffset{x: x, y: y, posWithOffset: posWithOffset}, nil
}

// drawCommits is drawCommits(svg, commits, modifyGraph, gitGraphConfig).
func (r *renderer) drawCommits(svg *dom.Node, commits *js.Map[string, *Commit], modifyGraph bool, gitGraphConfig *js.Obj[any]) error {
	gBullets := svg.Append("g").Attr("class", "commit-bullets")
	gLabels := svg.Append("g").Attr("class", "commit-labels")
	pos := 0.0
	if r.dir == "TB" || r.dir == "BT" {
		pos = defaultPos
	}
	keys := commits.Keys()
	// gitGraphConfig.parallelCommits ?? false, then read for truthiness.
	isParallelCommits := js.Truthy(gitGraphConfig.Get("parallelCommits"))

	sortKeys := func(a, b string) float64 {
		commitA, commitB := commits.Get(a), commits.Get(b)
		if commitA != nil && commitB != nil {
			return float64(commitA.Seq - commitB.Seq)
		}
		return 0
	}

	js.SortFunc(keys, sortKeys)
	sortedKeys := keys
	if r.dir == "BT" {
		if isParallelCommits {
			if err := r.setParallelBTPos(sortedKeys, commits, pos); err != nil {
				return err
			}
		}
		for i, j := 0, len(sortedKeys)-1; i < j; i, j = i+1, j-1 {
			sortedKeys[i], sortedKeys[j] = sortedKeys[j], sortedKeys[i]
		}
	}

	for _, key := range sortedKeys {
		commit := commits.Get(key)
		if commit == nil {
			return errors.New("Commit not found for key " + key)
		}
		if isParallelCommits {
			pos = r.calculatePosition(commit, r.dir)
		}

		position, err := r.getCommitPosition(commit, pos, isParallelCommits)
		if err != nil {
			return err
		}
		// Don't draw the commits now but calculate the positioning which is used by the branch lines etc.
		if modifyGraph {
			typeClass := getCommitClassType(commit)
			commitSymbolType := commitSymbolTypeOf(commit)
			branchIndex := 0.0
			if branch, ok := r.branchPos[commit.Branch]; ok {
				branchIndex = float64(branch.index)
			}
			r.drawCommitBullet(gBullets, commit, position, typeClass, branchIndex, commitSymbolType)
			r.drawCommitLabel(gLabels, commit, position, pos, gitGraphConfig)
			r.drawCommitTags(gLabels, commit, position, pos)
		}
		if r.dir == "TB" || r.dir == "BT" {
			r.commitPos[commit.ID] = commitPosition{x: position.x, y: position.posWithOffset}
		} else {
			r.commitPos[commit.ID] = commitPosition{x: position.posWithOffset, y: position.y}
		}
		if r.dir == "BT" && isParallelCommits {
			pos = pos + commitStep
		} else {
			pos = pos + commitStep + layoutOffset
		}
		if pos > r.maxPos {
			r.maxPos = pos
		}
	}
	return nil
}

// shouldRerouteArrow is shouldRerouteArrow(commitA, commitB, p1, p2,
// allCommits).
func (r *renderer) shouldRerouteArrow(commitA, commitB *Commit, p1, p2 commitPosition, allCommits *js.Map[string, *Commit]) bool {
	var commitBIsFurthest bool
	if r.dir == "TB" || r.dir == "BT" {
		commitBIsFurthest = p1.x < p2.x
	} else {
		commitBIsFurthest = p1.y < p2.y
	}
	branchToGetCurve := commitA.Branch
	if commitBIsFurthest {
		branchToGetCurve = commitB.Branch
	}
	isOnBranchToGetCurve := func(x *Commit) bool { return x.Branch == branchToGetCurve }
	isBetweenCommits := func(x *Commit) bool { return x.Seq > commitA.Seq && x.Seq < commitB.Seq }
	for _, commitX := range allCommits.Values() {
		if isBetweenCommits(commitX) && isOnBranchToGetCurve(commitX) {
			return true
		}
	}
	return false
}

// findLane is findLane(y1, y2, depth = 0).
func (r *renderer) findLane(y1, y2 float64, depth int) float64 {
	candidate := y1 + float64(math.Abs(y1-y2)/2)
	if depth > 5 {
		return candidate
	}

	ok := true
	for _, lane := range r.lanes {
		if !(math.Abs(lane-candidate) >= 10) {
			ok = false
			break
		}
	}
	if ok {
		r.lanes = append(r.lanes, candidate)
		return candidate
	}
	diff := math.Abs(y1 - y2)
	return r.findLane(y1, y2-diff/5, depth+1)
}

// drawArrow is drawArrow(svg, commitA, commitB, allCommits).
func (r *renderer) drawArrow(svg *dom.Node, commitA, commitB *Commit, allCommits *js.Map[string, *Commit]) error {
	useColorTheme := colorThemes(r.ctx.Cfg("theme"))
	if commitA == nil {
		return errors.New("Cannot read properties of undefined (reading 'id')")
	}
	p1, ok1 := r.commitPos[commitA.ID] // arrowStart
	p2, ok2 := r.commitPos[commitB.ID] // arrowEnd
	if !ok1 || !ok2 {
		return errors.New("Commit positions not found for commits " + commitA.ID + " and " + commitB.ID)
	}
	arrowNeedsRerouting := r.shouldRerouteArrow(commitA, commitB, p1, p2, allCommits)

	// Lower-right quadrant logic; top-left is 0,0

	var arc, arc2 string
	var radius, offset float64

	// branchIndex is branchPos.get(name)?.index. NaN stands for undefined.
	branchIndex := func(name string) float64 {
		if branch, ok := r.branchPos[name]; ok {
			return float64(branch.index)
		}
		return math.NaN()
	}
	// isMergeOfOther is the test that upstream repeats: commitB is a merge
	// commit and commitA is not its first parent.
	isMergeOfOther := func() bool {
		firstParent := ""
		hasFirstParent := len(commitB.Parents) > 0
		if hasFirstParent {
			firstParent = commitB.Parents[0]
		}
		return commitB.Type == commitTypeMerge && (!hasFirstParent || commitA.ID != firstParent)
	}
	s := js.Str

	colorClassNum := branchIndex(commitB.Branch)
	if isMergeOfOther() {
		colorClassNum = branchIndex(commitA.Branch)
	}

	lineDef := ""
	hasLineDef := false
	setLineDef := func(def string) {
		lineDef = def
		hasLineDef = true
	}
	if arrowNeedsRerouting {
		arc = "A 10 10, 0, 0, 0,"
		arc2 = "A 10 10, 0, 0, 1,"
		radius = 10
		offset = 10

		var lineY, lineX float64
		if p1.y < p2.y {
			lineY = r.findLane(p1.y, p2.y, 0)
		} else {
			lineY = r.findLane(p2.y, p1.y, 0)
		}

		if p1.x < p2.x {
			lineX = r.findLane(p1.x, p2.x, 0)
		} else {
			lineX = r.findLane(p2.x, p1.x, 0)
		}

		if r.dir == "TB" {
			if p1.x < p2.x {
				// Source commit is on branch position left of destination commit
				// so render arrow rightward with colour of destination branch

				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(lineX-radius) + " " + s(p1.y) + " " + arc2 + " " + s(lineX) + " " +
					s(p1.y+offset) +
					" L " + s(lineX) + " " + s(p2.y-radius) + " " + arc + " " + s(lineX+offset) + " " + s(p2.y) + " L " + s(p2.x) + " " + s(p2.y))
			} else {
				// Source commit is on branch position right of destination commit
				// so render arrow leftward with colour of source branch

				colorClassNum = branchIndex(commitA.Branch)

				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(lineX+radius) + " " + s(p1.y) + " " + arc + " " + s(lineX) + " " + s(p1.y+offset) + " L " + s(lineX) + " " + s(p2.y-radius) + " " + arc2 + " " + s(lineX-offset) + " " + s(p2.y) + " L " + s(p2.x) + " " + s(p2.y))
			}
		} else if r.dir == "BT" {
			if p1.x < p2.x {
				// Source commit is on branch position left of destination commit
				// so render arrow rightward with colour of destination branch

				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(lineX-radius) + " " + s(p1.y) + " " + arc + " " + s(lineX) + " " + s(p1.y-offset) + " L " + s(lineX) + " " + s(p2.y+radius) + " " + arc2 + " " + s(lineX+offset) + " " + s(p2.y) + " L " + s(p2.x) + " " + s(p2.y))
			} else {
				// Source commit is on branch position right of destination commit
				// so render arrow leftward with colour of source branch

				colorClassNum = branchIndex(commitA.Branch)

				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(lineX+radius) + " " + s(p1.y) + " " + arc2 + " " + s(lineX) + " " + s(p1.y-offset) + " L " + s(lineX) + " " + s(p2.y+radius) + " " + arc + " " + s(lineX-offset) + " " + s(p2.y) + " L " + s(p2.x) + " " + s(p2.y))
			}
		} else {
			if p1.y < p2.y {
				// Source commit is on branch positioned above destination commit
				// so render arrow downward with colour of destination branch

				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(lineY-radius) + " " + arc + " " +
					s(p1.x+offset) +
					" " + s(lineY) + " L " + s(p2.x-radius) + " " + s(lineY) + " " + arc2 + " " + s(p2.x) + " " + s(lineY+offset) + " L " + s(p2.x) + " " + s(p2.y))
			} else {
				// Source commit is on branch positioned below destination commit
				// so render arrow upward with colour of source branch

				colorClassNum = branchIndex(commitA.Branch)

				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(lineY+radius) + " " + arc2 + " " +
					s(p1.x+offset) +
					" " + s(lineY) + " L " + s(p2.x-radius) + " " + s(lineY) + " " + arc + " " + s(p2.x) + " " + s(lineY-offset) + " L " + s(p2.x) + " " + s(p2.y))
			}
		}
	} else {
		arc = "A 20 20, 0, 0, 0,"
		arc2 = "A 20 20, 0, 0, 1,"
		radius = 20
		offset = 20

		if r.dir == "TB" {
			if p1.x < p2.x {
				if isMergeOfOther() {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(p2.y-radius) + " " + arc + " " + s(p1.x+offset) + " " +
						s(p2.y) +
						" L " + s(p2.x) + " " + s(p2.y))
				} else {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x-radius) + " " + s(p1.y) + " " + arc2 + " " + s(p2.x) + " " +
						s(p1.y+offset) +
						" L " + s(p2.x) + " " + s(p2.y))
				}
			}

			if p1.x > p2.x {
				arc = "A 20 20, 0, 0, 0,"
				arc2 = "A 20 20, 0, 0, 1,"
				radius = 20
				offset = 20
				if isMergeOfOther() {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(p2.y-radius) + " " + arc2 + " " + s(p1.x-offset) + " " +
						s(p2.y) +
						" L " + s(p2.x) + " " + s(p2.y))
				} else {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x+radius) + " " + s(p1.y) + " " + arc + " " + s(p2.x) + " " +
						s(p1.y+offset) +
						" L " + s(p2.x) + " " + s(p2.y))
				}
			}
			if p1.x == p2.x {
				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x) + " " + s(p2.y))
			}
		} else if r.dir == "BT" {
			if p1.x < p2.x {
				if isMergeOfOther() {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(p2.y+radius) + " " + arc2 + " " + s(p1.x+offset) + " " +
						s(p2.y) +
						" L " + s(p2.x) + " " + s(p2.y))
				} else {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x-radius) + " " + s(p1.y) + " " + arc + " " + s(p2.x) + " " +
						s(p1.y-offset) +
						" L " + s(p2.x) + " " + s(p2.y))
				}
			}
			if p1.x > p2.x {
				arc = "A 20 20, 0, 0, 0,"
				arc2 = "A 20 20, 0, 0, 1,"
				radius = 20
				offset = 20

				if isMergeOfOther() {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(p2.y+radius) + " " + arc + " " + s(p1.x-offset) + " " +
						s(p2.y) +
						" L " + s(p2.x) + " " + s(p2.y))
				} else {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x+radius) + " " + s(p1.y) + " " + arc2 + " " + s(p2.x) + " " +
						s(p1.y-offset) +
						" L " + s(p2.x) + " " + s(p2.y))
				}
			}

			if p1.x == p2.x {
				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x) + " " + s(p2.y))
			}
		} else {
			if p1.y < p2.y {
				if isMergeOfOther() {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x-radius) + " " + s(p1.y) + " " + arc2 + " " + s(p2.x) + " " +
						s(p1.y+offset) +
						" L " + s(p2.x) + " " + s(p2.y))
				} else {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(p2.y-radius) + " " + arc + " " + s(p1.x+offset) + " " +
						s(p2.y) +
						" L " + s(p2.x) + " " + s(p2.y))
				}
			}
			if p1.y > p2.y {
				if isMergeOfOther() {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x-radius) + " " + s(p1.y) + " " + arc + " " + s(p2.x) + " " +
						s(p1.y-offset) +
						" L " + s(p2.x) + " " + s(p2.y))
				} else {
					setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p1.x) + " " + s(p2.y+radius) + " " + arc2 + " " + s(p1.x+offset) + " " +
						s(p2.y) +
						" L " + s(p2.x) + " " + s(p2.y))
				}
			}

			if p1.y == p2.y {
				setLineDef("M " + s(p1.x) + " " + s(p1.y) + " L " + s(p2.x) + " " + s(p2.y))
			}
		}
	}
	if !hasLineDef {
		return errors.New("Line definition not found")
	}
	svg.
		Append("path").
		Attr("d", lineDef).
		Attr("class", "arrow arrow"+s(calcColorIndex(colorClassNum, themeColorLimit, useColorTheme)))
	return nil
}

// drawArrows is drawArrows(svg, commits).
func (r *renderer) drawArrows(svg *dom.Node, commits *js.Map[string, *Commit]) error {
	gArrows := svg.Append("g").Attr("class", "commit-arrows")
	for _, key := range commits.Keys() {
		commit := commits.Get(key)

		if len(commit.Parents) > 0 {
			for _, parent := range commit.Parents {
				if err := r.drawArrow(gArrows, commits.Get(parent), commit, commits); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// drawBranches is drawBranches(svg, branches, gitGraphConfig, id).
func (r *renderer) drawBranches(svg *dom.Node, branches []string, gitGraphConfig *js.Obj[any], id string) error {
	look := r.ctx.Cfg("look")
	theme := r.ctx.Cfg("theme")
	dropShadow := r.ctx.Cfg("themeVariables", "dropShadow")
	themeColorLimitVar := r.ctx.Cfg("themeVariables", "THEME_COLOR_LIMIT")
	useReduxGeometry := reduxGeometryThemes(theme)
	useColorTheme := colorThemes(theme)
	rotateCommitLabel := gitGraphConfig.Get("rotateCommitLabel") == true
	g := svg.Append("g")
	for index, name := range branches {
		limit := themeColorLimit
		if useReduxGeometry {
			limit = js.ToNumber(themeColorLimitVar)
		}
		adjustIndexForTheme := js.Str(calcColorIndex(float64(index), limit, useColorTheme))

		branch, ok := r.branchPos[name]
		if !ok {
			return errors.New("Position not found for branch " + name)
		}
		pos := branch.pos
		// LR spine Y: bkg rect center, dotted line, and commits all sit here.
		// TB/BT use pos directly (their line attrs are overridden below).
		var spineY float64
		if r.dir == "TB" || r.dir == "BT" {
			spineY = pos
		} else if useReduxGeometry {
			spineY = pos + reduxBranchLabelPaddingY/2 + 1
		} else {
			spineY = pos - 2
		}
		line := g.Append("line")
		line.Attr("x1", 0)
		line.Attr("y1", spineY)
		line.Attr("x2", r.maxPos)
		line.Attr("y2", spineY)
		line.Attr("class", "branch branch"+adjustIndexForTheme)

		if r.dir == "TB" {
			line.Attr("y1", defaultPos)
			line.Attr("x1", pos)
			line.Attr("y2", r.maxPos)
			line.Attr("x2", pos)
		} else if r.dir == "BT" {
			line.Attr("y1", r.maxPos)
			line.Attr("x1", pos)
			line.Attr("y2", defaultPos)
			line.Attr("x2", pos)
		}
		r.lanes = append(r.lanes, spineY)

		// Create the actual text element
		labelElement := r.drawText(name)
		// Create outer g, edgeLabel, this will be positioned after graph layout
		bkg := g.Insert("rect")
		branchLabel := g.Insert("g").Attr("class", "branchLabel")

		// Create inner g, label, this will be positioned now for centering the text
		label := branchLabel.Insert("g").Attr("class", "label branch-label"+adjustIndexForTheme)

		label.AppendChild(labelElement)
		bbox := labelElement.BBox()
		borderRadius := reduxNum(useReduxGeometry, 0, 4)
		labelPaddingX := reduxNum(useReduxGeometry, 16, 0)
		labelPaddingY := reduxNum(useReduxGeometry, reduxBranchLabelPaddingY, 0)
		if look == "neo" {
			bkg.Attr("data-look", "neo")
		}

		style := ""
		if look == "neo" {
			if useReduxGeometry {
				style = "filter:url(#" + id + "-drop-shadow)"
			} else {
				style = "filter:" + js.ToString(dropShadow)
			}
		}
		rotateOffset := 0.0
		if rotateCommitLabel {
			rotateOffset = 30
		}
		bkg.
			Attr("class", "branchLabelBkg label"+adjustIndexForTheme).
			Attr("style", style).
			Attr("rx", borderRadius).
			Attr("ry", borderRadius).
			Attr("x", -bbox.Width-4-rotateOffset).
			Attr("y", float64(-bbox.Height/2)+10).
			Attr("width", bbox.Width+18+labelPaddingX).
			Attr("height", bbox.Height+4+labelPaddingY)
		label.Attr(
			"transform",
			"translate("+
				js.Str(-bbox.Width-
					14-
					rotateOffset+
					float64(labelPaddingX/2))+
				", "+
				js.Str(spineY-float64(bbox.Height/2)-2)+
				")",
		)
		if r.dir == "TB" {
			bkg.Attr("x", pos-float64(bbox.Width/2)-10).Attr("y", 0)
			label.Attr("transform", "translate("+js.Str(pos-float64(bbox.Width/2)-5)+", "+js.Str(0)+")")
			if useReduxGeometry {
				bkg.Attr("transform", "translate("+js.Str(-float64(labelPaddingX/2)-3)+", "+js.Str(-labelPaddingY-10)+")")
				label.Attr(
					"transform",
					"translate("+js.Str(pos-float64(bbox.Width/2)-5)+", "+js.Str(float64(-labelPaddingY*2)+7)+")",
				)
			}
		} else if r.dir == "BT" {
			bkg.Attr("x", pos-float64(bbox.Width/2)-10).Attr("y", r.maxPos)
			label.Attr("transform", "translate("+js.Str(pos-float64(bbox.Width/2)-5)+", "+js.Str(r.maxPos)+")")
			if useReduxGeometry {
				bkg.Attr("transform", "translate("+js.Str(-float64(labelPaddingX/2)-3)+", "+js.Str(labelPaddingY+10)+")")
				label.Attr(
					"transform",
					"translate("+js.Str(pos-float64(bbox.Width/2)-5)+", "+js.Str(r.maxPos+float64(labelPaddingY*2)+4)+")",
				)
			}
		} else {
			bkg.Attr("transform", "translate(-19, "+js.Str(spineY-12-float64(labelPaddingY/2))+")")
		}
	}
	return nil
}

// setBranchPosition is setBranchPosition(name, pos, index, bbox,
// rotateCommitLabel).
func (r *renderer) setBranchPosition(name string, pos float64, index int, bbox dom.Rect, rotateCommitLabel bool) float64 {
	r.branchPos[name] = branchPosition{pos: pos, index: index}
	rotate := 0.0
	if rotateCommitLabel {
		rotate = 40
	}
	vertical := 0.0
	if r.dir == "TB" || r.dir == "BT" {
		vertical = bbox.Width / 2
	}
	pos += 50 + rotate + vertical
	return pos
}

// attrValue is a dynamic value as an argument of selection.attr: null and
// undefined remove the attribute.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// draw is draw(txt, id, ver, diagObj).
func draw(ctx *render.Context, db *DB) error {
	r := &renderer{
		ctx:       ctx,
		branchPos: map[string]branchPosition{},
		commitPos: map[string]commitPosition{},
		dir:       "LR",
	}

	gitGraphConfig := db.GetConfig()
	// gitGraphConfig.rotateCommitLabel ?? false, then read for truthiness.
	rotateCommitLabel := js.Truthy(gitGraphConfig.Get("rotateCommitLabel"))
	r.allCommitsDict = db.GetCommits()
	branches := db.GetBranchesAsObjArray()
	r.dir = db.GetDirection()
	diagram := ctx.GetDiagramElement()
	id := ctx.ID

	if db.badMainBranchName {
		// drawText takes a name that is not a string for an array of rows
		// and calls forEach on it.
		return errors.New("rows.forEach is not a function")
	}

	// Add linearGradient for neo look. render.ts does this for layout-based
	// diagrams, but gitGraph uses its own draw function.
	look := ctx.Cfg("look")
	theme := ctx.Cfg("theme")
	useGradient := ctx.Cfg("themeVariables", "useGradient")
	gradientStart := ctx.Cfg("themeVariables", "gradientStart")
	gradientStop := ctx.Cfg("themeVariables", "gradientStop")
	filterColor := ctx.Cfg("themeVariables", "filterColor")
	if js.Truthy(useGradient) {
		gradient := diagram.
			Append("defs").
			Append("linearGradient").
			Attr("id", id+"-gradient").
			Attr("gradientUnits", "objectBoundingBox").
			Attr("x1", "0%").
			Attr("y1", "0%").
			Attr("x2", "100%").
			Attr("y2", "0%")
		gradient.
			Append("stop").
			Attr("offset", "0%").
			Attr("stop-color", attrValue(gradientStart)).
			Attr("stop-opacity", 1)
		gradient.
			Append("stop").
			Attr("offset", "100%").
			Attr("stop-color", attrValue(gradientStop)).
			Attr("stop-opacity", 1)
	}

	// Add drop-shadow SVG filter for neo+redux look. Defined once on the
	// root <svg> with a diagram-unique ID.
	if look == "neo" && reduxGeometryThemes(theme) {
		diagram.
			Append("defs").
			Append("filter").
			Attr("id", id+"-drop-shadow").
			Attr("height", "130%").
			Attr("width", "130%").
			Append("feDropShadow").
			Attr("dx", "4").
			Attr("dy", "4").
			Attr("stdDeviation", 0).
			Attr("flood-opacity", "0.06").
			Attr("flood-color", attrValue(filterColor))
	}

	pos := 0.0

	for index, name := range branches {
		labelElement := r.drawText(name)
		g := diagram.Append("g")
		branchLabel := g.Insert("g").Attr("class", "branchLabel")
		label := branchLabel.Insert("g").Attr("class", "label branch-label")
		label.AppendChild(labelElement)
		bbox := labelElement.BBox()

		pos = r.setBranchPosition(name, pos, index, bbox, rotateCommitLabel)
		label.Remove()
		branchLabel.Remove()
		g.Remove()
	}

	if err := r.drawCommits(diagram, r.allCommitsDict, false, gitGraphConfig); err != nil {
		return err
	}
	if js.Truthy(gitGraphConfig.Get("showBranches")) {
		if err := r.drawBranches(diagram, branches, gitGraphConfig, id); err != nil {
			return err
		}
	}
	if err := r.drawArrows(diagram, r.allCommitsDict); err != nil {
		return err
	}
	if err := r.drawCommits(diagram, r.allCommitsDict, true, gitGraphConfig); err != nil {
		return err
	}

	titleTopMargin := 0.0
	if v := gitGraphConfig.Get("titleTopMargin"); !js.IsNullish(v) {
		titleTopMargin = js.ToNumber(v)
	}
	render.InsertTitle(diagram, "gitTitleText", titleTopMargin, db.GetDiagramTitle())

	// Setup the view box and size of the svg element
	render.SetupGraphViewbox(
		diagram,
		js.ToNumber(gitGraphConfig.Get("diagramPadding")),
		js.Truthy(gitGraphConfig.Get("useMaxWidth")),
	)
	return nil
}
