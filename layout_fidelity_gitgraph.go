package mermaid

import (
	"math"
	"sort"
	"strings"
)

type gitGraphBranchLabelLayout struct {
	BGX        float64
	BGY        float64
	BGWidth    float64
	BGHeight   float64
	TextX      float64
	TextY      float64
	TextWidth  float64
	TextHeight float64
}

type gitGraphBranchLayout struct {
	Name  string
	Index int
	Pos   float64
	Label gitGraphBranchLabelLayout
}

type gitGraphTransform struct {
	TranslateX float64
	TranslateY float64
	RotateDeg  float64
	RotateCX   float64
	RotateCY   float64
}

type gitGraphCommitLabelLayout struct {
	Text      string
	TextX     float64
	TextY     float64
	BGX       float64
	BGY       float64
	BGWidth   float64
	BGHeight  float64
	Transform *gitGraphTransform
}

type gitGraphTagLayout struct {
	Text      string
	TextX     float64
	TextY     float64
	Points    []Point
	HoleX     float64
	HoleY     float64
	Transform *gitGraphTransform
}

type gitGraphCommitLayout struct {
	ID            string
	Seq           int
	BranchIndex   int
	X             float64
	Y             float64
	AxisPos       float64
	CommitType    GitGraphCommitType
	CustomType    GitGraphCommitType
	HasCustomType bool
	Tags          []gitGraphTagLayout
	Label         *gitGraphCommitLabelLayout
}

type gitGraphArrowLayout struct {
	Path       string
	ColorIndex int
}

type gitGraphLayoutData struct {
	Branches  []gitGraphBranchLayout
	Commits   []gitGraphCommitLayout
	Arrows    []gitGraphArrowLayout
	Width     float64
	Height    float64
	OffsetX   float64
	OffsetY   float64
	MaxPos    float64
	Direction Direction
}

type gitBranchPosInfo struct {
	Pos         float64
	Index       int
	LabelWidth  float64
	LabelHeight float64
}

func layoutGitGraphFidelity(graph *Graph, theme Theme, config LayoutConfig) Layout {
	layout := Layout{Kind: graph.Kind}
	if len(graph.GitCommits) == 0 {
		return layoutGeneric(graph, theme)
	}
	gg := config.GitGraph
	isVertical := graph.Direction == DirectionTopDown || graph.Direction == DirectionBottomTop
	isBottomTop := graph.Direction == DirectionBottomTop

	branches := append([]GitBranch(nil), graph.GitBranchDefs...)
	if len(branches) == 0 {
		branches = append(branches, GitBranch{
			Name:           gg.MainBranchName,
			Order:          &gg.MainBranchOrder,
			InsertionIndex: 0,
		})
	}

	type branchEntry struct {
		Branch GitBranch
		Order  float64
	}
	entries := make([]branchEntry, 0, len(branches))
	for _, branch := range branches {
		order := defaultGitGraphBranchOrder(branch.InsertionIndex)
		if branch.Order != nil {
			order = *branch.Order
		}
		entries = append(entries, branchEntry{Branch: branch, Order: order})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Order < entries[j].Order
	})

	branchPos := map[string]gitBranchPosInfo{}
	branchLayouts := []gitGraphBranchLayout{}
	pos := 0.0
	for index, entry := range entries {
		branch := entry.Branch
		measureFontSize := theme.FontSize
		if gg.BranchLabelFontSize > 0 {
			measureFontSize = gg.BranchLabelFontSize
		}
		labelWidth, labelHeight := measureGitGraphText(
			branch.Name,
			measureFontSize,
			gg.BranchLabelLineHeight,
			gg.TextWidthScale,
			config.FastTextMetrics,
		)
		spacingRotateExtra := 0.0
		labelRotateExtra := 0.0
		if gg.RotateCommitLabel {
			spacingRotateExtra = gg.BranchSpacingRotateExtra
			labelRotateExtra = gg.BranchLabelRotateExtra
		}
		bgWidth := labelWidth + gg.BranchLabelBGPadX
		bgHeight := labelHeight + gg.BranchLabelBGPadY
		bgFinalX := 0.0
		bgFinalY := 0.0
		textX := 0.0
		textY := 0.0
		if isVertical {
			bgX := pos - labelWidth/2.0 - gg.BranchLabelTBBGOffsetX
			textX = pos - labelWidth/2.0 - gg.BranchLabelTBTextOffsetX
			baseY := gg.BranchLabelTBOffsetY
			if isBottomTop {
				baseY = 0.0
			}
			bgFinalX = bgX
			bgFinalY = baseY
			textY = baseY
		} else {
			bgX := -labelWidth - gg.BranchLabelBGOffsetX - labelRotateExtra
			bgY := -labelHeight/2.0 + gg.BranchLabelBGOffsetY
			bgFinalX = bgX + gg.BranchLabelTranslateX
			bgFinalY = bgY + (pos - labelHeight/2.0)
			textX = -labelWidth - gg.BranchLabelTextOffsetX - labelRotateExtra
			textY = pos - labelHeight/2.0 + gg.BranchLabelTextOffsetY
		}
		label := gitGraphBranchLabelLayout{
			BGX:        bgFinalX,
			BGY:        bgFinalY,
			BGWidth:    bgWidth,
			BGHeight:   bgHeight,
			TextX:      textX,
			TextY:      textY,
			TextWidth:  labelWidth,
			TextHeight: labelHeight,
		}
		branchLayouts = append(branchLayouts, gitGraphBranchLayout{
			Name:  branch.Name,
			Index: index,
			Pos:   pos,
			Label: label,
		})
		branchPos[branch.Name] = gitBranchPosInfo{
			Pos:         pos,
			Index:       index,
			LabelWidth:  labelWidth,
			LabelHeight: labelHeight,
		}
		widthExtra := 0.0
		if isVertical {
			widthExtra = labelWidth / 2.0
		}
		pos += gg.BranchSpacing + spacingRotateExtra + widthExtra
	}

	commits := append([]GitCommit(nil), graph.GitCommits...)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Seq < commits[j].Seq
	})
	commitLayouts := []gitGraphCommitLayout{}
	commitPos := map[string]Point{}
	pos = 0.0
	if isVertical {
		pos = gg.DefaultPos
	}
	maxPos := pos
	isParallel := gg.ParallelCommits
	commitOrder := make([]*GitCommit, 0, len(commits))
	for i := range commits {
		commitOrder = append(commitOrder, &commits[i])
	}
	if isBottomTop && isParallel {
		gitGraphSetParallelBottomTopPositions(
			commitOrder,
			gg.DefaultPos,
			gg.CommitStep,
			gg.LayoutOffset,
			branchPos,
			commitPos,
		)
	}
	if isBottomTop {
		for i, j := 0, len(commitOrder)-1; i < j; i, j = i+1, j-1 {
			commitOrder[i], commitOrder[j] = commitOrder[j], commitOrder[i]
		}
	}

	for _, commit := range commitOrder {
		if isParallel {
			pos = gitGraphCalculatePosition(commit, graph.Direction, gg.DefaultPos, gg.CommitStep, commitPos)
		}
		x, y, posWithOffset := gitGraphCommitPosition(commit, pos, isParallel, graph.Direction, gg.LayoutOffset, branchPos)
		axisPos := pos
		branchInfo, ok := branchPos[commit.Branch]
		if !ok {
			branchInfo = gitBranchPosInfo{}
		}
		branchIndex := branchInfo.Index

		showLabel := gg.ShowCommitLabel &&
			commit.CommitType != GitGraphCommitTypeCherryPick &&
			(commit.CommitType != GitGraphCommitTypeMerge || commit.CustomID)
		var label *gitGraphCommitLabelLayout
		if showLabel {
			labelWidth, labelHeight := measureGitGraphText(
				commit.ID,
				gg.CommitLabelFontSize,
				gg.CommitLabelLineHeight,
				gg.TextWidthScale,
				config.FastTextMetrics,
			)
			textX := 0.0
			textY := 0.0
			bgX := 0.0
			bgY := 0.0
			var transform *gitGraphTransform
			if isVertical {
				textX = x - (labelWidth + gg.CommitLabelTBTextExtra)
				textY = y + labelHeight + gg.CommitLabelTBTextOffsetY
				bgX = x - (labelWidth + gg.CommitLabelTBBGExtra)
				bgY = y + gg.CommitLabelTBBGOffsetY
				if gg.RotateCommitLabel {
					transform = &gitGraphTransform{
						RotateDeg: gg.CommitLabelRotateAngle,
						RotateCX:  x,
						RotateCY:  y,
					}
				}
			} else {
				textX = posWithOffset - labelWidth/2.0
				textY = y + gg.CommitLabelOffsetY
				bgX = posWithOffset - labelWidth/2.0 - gg.CommitLabelPadding
				bgY = y + gg.CommitLabelBGOffsetY
				if gg.RotateCommitLabel {
					rotateX := gg.CommitLabelRotateTranslateXBase -
						(labelWidth+gg.CommitLabelRotateTranslateXWidthOffset)*gg.CommitLabelRotateTranslateXScale
					rotateY := gg.CommitLabelRotateTranslateYBase +
						labelWidth*gg.CommitLabelRotateTranslateYScale
					transform = &gitGraphTransform{
						TranslateX: rotateX,
						TranslateY: rotateY,
						RotateDeg:  gg.CommitLabelRotateAngle,
						RotateCX:   axisPos,
						RotateCY:   y,
					}
				}
			}
			label = &gitGraphCommitLabelLayout{
				Text:      commit.ID,
				TextX:     textX,
				TextY:     textY,
				BGX:       bgX,
				BGY:       bgY,
				BGWidth:   labelWidth + 2.0*gg.CommitLabelPadding,
				BGHeight:  labelHeight + 2.0*gg.CommitLabelPadding,
				Transform: transform,
			}
		}

		tagLayouts := []gitGraphTagLayout{}
		if len(commit.Tags) > 0 {
			maxWidth := 0.0
			maxHeight := 0.0
			type tagDef struct {
				Text    string
				Width   float64
				OffsetY float64
			}
			tagDefs := []tagDef{}
			yOffset := 0.0
			for idx := len(commit.Tags) - 1; idx >= 0; idx-- {
				tagValue := commit.Tags[idx]
				width, height := measureGitGraphText(
					tagValue,
					gg.TagLabelFontSize,
					gg.TagLabelLineHeight,
					gg.TextWidthScale,
					config.FastTextMetrics,
				)
				maxWidth = max(maxWidth, width)
				maxHeight = max(maxHeight, height)
				tagDefs = append(tagDefs, tagDef{
					Text:    tagValue,
					Width:   width,
					OffsetY: yOffset,
				})
				yOffset += gg.TagSpacingY
			}
			halfH := maxHeight / 2.0
			for _, tag := range tagDefs {
				if isVertical {
					yOrigin := axisPos + tag.OffsetY
					px := gg.TagPaddingX
					py := gg.TagPaddingY
					textTranslateDelta := gg.TagTextRotateTranslate - gg.TagRotateTranslate
					textX := x + gg.TagTextOffsetXTB + textTranslateDelta
					textY := yOrigin + gg.TagTextOffsetYTB + textTranslateDelta
					points := []Point{
						{X: x, Y: yOrigin + py},
						{X: x, Y: yOrigin - py},
						{X: x + gg.LayoutOffset, Y: yOrigin - halfH - py},
						{X: x + gg.LayoutOffset + maxWidth + px, Y: yOrigin - halfH - py},
						{X: x + gg.LayoutOffset + maxWidth + px, Y: yOrigin + halfH + py},
						{X: x + gg.LayoutOffset, Y: yOrigin + halfH + py},
					}
					tagLayouts = append(tagLayouts, gitGraphTagLayout{
						Text:   tag.Text,
						TextX:  textX,
						TextY:  textY,
						Points: points,
						HoleX:  x + px/2.0,
						HoleY:  yOrigin,
						Transform: &gitGraphTransform{
							TranslateX: gg.TagRotateTranslate,
							TranslateY: gg.TagRotateTranslate,
							RotateDeg:  gg.TagRotateAngle,
							RotateCX:   x,
							RotateCY:   axisPos,
						},
					})
				} else {
					textX := posWithOffset - tag.Width/2.0
					textY := y - gg.TagTextOffsetY - tag.OffsetY
					ly := y - gg.TagPolygonOffsetY - tag.OffsetY
					px := gg.TagPaddingX
					py := gg.TagPaddingY
					points := []Point{
						{X: axisPos - maxWidth/2.0 - px/2.0, Y: ly + py},
						{X: axisPos - maxWidth/2.0 - px/2.0, Y: ly - py},
						{X: posWithOffset - maxWidth/2.0 - px, Y: ly - halfH - py},
						{X: posWithOffset + maxWidth/2.0 + px, Y: ly - halfH - py},
						{X: posWithOffset + maxWidth/2.0 + px, Y: ly + halfH + py},
						{X: posWithOffset - maxWidth/2.0 - px, Y: ly + halfH + py},
					}
					tagLayouts = append(tagLayouts, gitGraphTagLayout{
						Text:      tag.Text,
						TextX:     textX,
						TextY:     textY,
						Points:    points,
						HoleX:     axisPos - maxWidth/2.0 + px/2.0,
						HoleY:     ly,
						Transform: nil,
					})
				}
			}
		}

		commitLayouts = append(commitLayouts, gitGraphCommitLayout{
			ID:            commit.ID,
			Seq:           commit.Seq,
			BranchIndex:   branchIndex,
			X:             x,
			Y:             y,
			AxisPos:       axisPos,
			CommitType:    commit.CommitType,
			CustomType:    commit.CustomType,
			HasCustomType: commit.HasCustomType,
			Tags:          tagLayouts,
			Label:         label,
		})

		if isVertical {
			commitPos[commit.ID] = Point{X: x, Y: posWithOffset}
		} else {
			commitPos[commit.ID] = Point{X: posWithOffset, Y: y}
		}
		if isBottomTop && isParallel {
			pos += gg.CommitStep
		} else {
			pos += gg.CommitStep + gg.LayoutOffset
		}
		maxPos = max(maxPos, pos)
	}

	if isBottomTop {
		for i := range branchLayouts {
			branchLayouts[i].Label.BGY = maxPos + gg.BranchLabelBTOffsetY
			branchLayouts[i].Label.TextY = maxPos + gg.BranchLabelBTOffsetY
		}
	}

	commitByID := map[string]GitCommit{}
	for _, commit := range commits {
		commitByID[commit.ID] = commit
	}
	arrows := []gitGraphArrowLayout{}
	lanes := []float64{}
	for _, commit := range graph.GitCommits {
		if len(commit.Parents) == 0 {
			continue
		}
		for _, parent := range commit.Parents {
			p1, ok1 := commitPos[parent]
			p2, ok2 := commitPos[commit.ID]
			if !ok1 || !ok2 {
				continue
			}
			commitA, okA := commitByID[parent]
			commitB, okB := commitByID[commit.ID]
			if !okA || !okB {
				continue
			}
			path := gitGraphArrowPath(
				graph.Direction,
				commitA,
				commitB,
				p1,
				p2,
				commits,
				gg,
				&lanes,
			)
			colorIndex := 0
			if info, ok := branchPos[commitB.Branch]; ok {
				colorIndex = info.Index
			}
			firstParent := ""
			if len(commitB.Parents) > 0 {
				firstParent = commitB.Parents[0]
			}
			if commitB.CommitType == GitGraphCommitTypeMerge && commitA.ID != firstParent {
				if info, ok := branchPos[commitA.Branch]; ok {
					colorIndex = info.Index
				}
			}
			arrows = append(arrows, gitGraphArrowLayout{Path: path, ColorIndex: colorIndex})
		}
	}

	minX := math.Inf(1)
	minY := math.Inf(1)
	maxX := math.Inf(-1)
	maxY := math.Inf(-1)

	for _, branch := range branchLayouts {
		x1, y1, x2, y2 := 0.0, 0.0, 0.0, 0.0
		if isVertical {
			start := gg.DefaultPos
			end := maxPos
			if isBottomTop {
				start = maxPos
				end = gg.DefaultPos
			}
			x1, y1, x2, y2 = branch.Pos, start, branch.Pos, end
		} else {
			x1, y1, x2, y2 = 0.0, branch.Pos, maxPos, branch.Pos
		}
		updateBoundsLineGitgraph(&minX, &minY, &maxX, &maxY, x1, y1, x2, y2)
		updateBoundsRectGitgraph(
			&minX, &minY, &maxX, &maxY,
			branch.Label.BGX,
			branch.Label.BGY,
			branch.Label.BGWidth,
			branch.Label.BGHeight,
			nil,
		)
	}

	for _, commit := range commitLayouts {
		radius := gg.CommitRadius
		if commit.CommitType == GitGraphCommitTypeMerge {
			radius = gg.MergeRadiusOuter
		}
		updateBoundsRectGitgraph(
			&minX, &minY, &maxX, &maxY,
			commit.X-radius,
			commit.Y-radius,
			radius*2.0,
			radius*2.0,
			nil,
		)
		if commit.Label != nil {
			updateBoundsRectGitgraph(
				&minX, &minY, &maxX, &maxY,
				commit.Label.BGX,
				commit.Label.BGY,
				commit.Label.BGWidth,
				commit.Label.BGHeight,
				commit.Label.Transform,
			)
		}
		for _, tag := range commit.Tags {
			updateBoundsPointsGitgraph(&minX, &minY, &maxX, &maxY, tag.Points, tag.Transform)
		}
	}

	if !isFinite(minX) {
		minX = 0.0
		minY = 0.0
		maxX = 1.0
		maxY = 1.0
	}
	minX -= gg.DiagramPadding
	minY -= gg.DiagramPadding
	maxX += gg.DiagramPadding
	maxY += gg.DiagramPadding
	if len(commitLayouts) > 0 {
		leftPad := 12.0
		rightPad := max(0.0, float64(len(commitLayouts))*8.0-6.0)
		topTrim := 6.0
		bottomPad := float64(len(commitLayouts)) * 4.5
		minX -= leftPad
		maxX += rightPad
		minY += topTrim
		maxY += bottomPad
		maxX += 8.0
		maxY += 5.0
	}

	data := gitGraphLayoutData{
		Branches:  branchLayouts,
		Commits:   commitLayouts,
		Arrows:    arrows,
		Width:     max(maxX-minX, 1.0),
		Height:    max(maxY-minY, 1.0),
		OffsetX:   -minX,
		OffsetY:   -minY,
		MaxPos:    maxPos,
		Direction: graph.Direction,
	}
	layout.Width = data.Width
	layout.Height = data.Height
	layout.ViewBoxX = -data.OffsetX
	layout.ViewBoxY = -data.OffsetY
	layout.ViewBoxWidth = data.Width
	layout.ViewBoxHeight = data.Height

	gitColors := theme.GitColors
	if len(gitColors) == 0 {
		gitColors = []string{"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7"}
	}
	gitInvColors := theme.GitInvColors
	if len(gitInvColors) == 0 {
		gitInvColors = append([]string(nil), gitColors...)
	}
	branchLabelColors := theme.GitBranchLabelColors
	if len(branchLabelColors) == 0 {
		branchLabelColors = []string{"#ffffff", "#000000", "#000000", "#ffffff", "#000000", "#000000", "#000000", "#000000"}
	}
	baseTranslate := ""

	if gg.ShowBranches {
		for _, branch := range data.Branches {
			x1, y1, x2, y2 := 0.0, 0.0, 0.0, 0.0
			switch data.Direction {
			case DirectionTopDown:
				x1, y1, x2, y2 = branch.Pos, gg.DefaultPos, branch.Pos, data.MaxPos
			case DirectionBottomTop:
				x1, y1, x2, y2 = branch.Pos, data.MaxPos, branch.Pos, gg.DefaultPos
			default:
				x1, y1, x2, y2 = 0.0, branch.Pos, data.MaxPos, branch.Pos
			}
			layout.Lines = append(layout.Lines, LayoutLine{
				Class:       "branch branch" + intString(branch.Index),
				X1:          x1,
				Y1:          y1,
				X2:          x2,
				Y2:          y2,
				Stroke:      theme.LineColor,
				StrokeWidth: gg.BranchStrokeWidth,
				DashArray:   gg.BranchDasharray,
				Transform:   baseTranslate,
			})
			colorIdx := branch.Index % len(gitColors)
			labelColor := gitColors[colorIdx]
			textColor := branchLabelColors[colorIdx%len(branchLabelColors)]
			layout.Rects = append(layout.Rects, LayoutRect{
				Class:     "branchLabelBkg label" + intString(colorIdx),
				X:         branch.Label.BGX,
				Y:         branch.Label.BGY,
				W:         branch.Label.BGWidth,
				H:         branch.Label.BGHeight,
				RX:        gg.BranchLabelCornerRadius,
				RY:        gg.BranchLabelCornerRadius,
				Fill:      labelColor,
				Stroke:    "none",
				Transform: baseTranslate,
			})
			branchFontSize := theme.FontSize
			if gg.BranchLabelFontSize > 0 {
				branchFontSize = gg.BranchLabelFontSize
			}
			appendGitGraphMultilineText(
				&layout,
				branch.Label.TextX,
				branch.Label.TextY,
				branch.Name,
				"branch-label"+intString(colorIdx),
				branchFontSize,
				gg.BranchLabelLineHeight,
				textColor,
				baseTranslate,
			)
		}
	}

	for _, arrow := range data.Arrows {
		colorIdx := arrow.ColorIndex % len(gitColors)
		layout.Paths = append(layout.Paths, LayoutPath{
			Class:       "arrow arrow" + intString(colorIdx),
			D:           arrow.Path,
			Fill:        "none",
			Stroke:      gitColors[colorIdx],
			StrokeWidth: gg.ArrowStrokeWidth,
			LineCap:     "round",
			Transform:   baseTranslate,
		})
	}

	for _, commit := range data.Commits {
		colorIdx := commit.BranchIndex % len(gitColors)
		color := gitColors[colorIdx]
		highlightColor := gitInvColors[colorIdx%len(gitInvColors)]
		symbolType := commit.CommitType
		if commit.HasCustomType {
			symbolType = commit.CustomType
		}
		switch symbolType {
		case GitGraphCommitTypeHighlight:
			layout.Rects = append(layout.Rects,
				LayoutRect{
					Class:     "commit " + commit.ID + " commit-highlight" + intString(colorIdx) + " commit-highlight-outer",
					X:         commit.X - gg.HighlightOuterSize/2.0,
					Y:         commit.Y - gg.HighlightOuterSize/2.0,
					W:         gg.HighlightOuterSize,
					H:         gg.HighlightOuterSize,
					Fill:      highlightColor,
					Stroke:    highlightColor,
					Transform: baseTranslate,
				},
				LayoutRect{
					Class:     "commit " + commit.ID + " commit" + intString(colorIdx) + " commit-highlight-inner",
					X:         commit.X - gg.HighlightInnerSize/2.0,
					Y:         commit.Y - gg.HighlightInnerSize/2.0,
					W:         gg.HighlightInnerSize,
					H:         gg.HighlightInnerSize,
					Fill:      theme.PrimaryColor,
					Stroke:    theme.PrimaryColor,
					Transform: baseTranslate,
				},
			)
		case GitGraphCommitTypeCherryPick:
			layout.Circles = append(layout.Circles,
				LayoutCircle{
					Class:     "commit " + commit.ID + " commit" + intString(colorIdx),
					CX:        commit.X,
					CY:        commit.Y,
					R:         gg.CommitRadius,
					Fill:      color,
					Stroke:    color,
					Transform: baseTranslate,
				},
				LayoutCircle{
					Class:     "commit " + commit.ID + " commit-cherry-pick-dot",
					CX:        commit.X - gg.CherryPickDotOffsetX,
					CY:        commit.Y + gg.CherryPickDotOffsetY,
					R:         gg.CherryPickDotRadius,
					Fill:      gg.CherryPickAccentColor,
					Stroke:    "none",
					Transform: baseTranslate,
				},
				LayoutCircle{
					Class:     "commit " + commit.ID + " commit-cherry-pick-dot",
					CX:        commit.X + gg.CherryPickDotOffsetX,
					CY:        commit.Y + gg.CherryPickDotOffsetY,
					R:         gg.CherryPickDotRadius,
					Fill:      gg.CherryPickAccentColor,
					Stroke:    "none",
					Transform: baseTranslate,
				},
			)
			layout.Lines = append(layout.Lines,
				LayoutLine{
					Class:       "commit " + commit.ID + " commit-cherry-pick-stem",
					X1:          commit.X + gg.CherryPickDotOffsetX,
					Y1:          commit.Y + gg.CherryPickStemStartOffsetY,
					X2:          commit.X,
					Y2:          commit.Y + gg.CherryPickStemEndOffsetY,
					Stroke:      gg.CherryPickAccentColor,
					StrokeWidth: gg.CherryPickStemStrokeWidth,
					Transform:   baseTranslate,
				},
				LayoutLine{
					Class:       "commit " + commit.ID + " commit-cherry-pick-stem",
					X1:          commit.X - gg.CherryPickDotOffsetX,
					Y1:          commit.Y + gg.CherryPickStemStartOffsetY,
					X2:          commit.X,
					Y2:          commit.Y + gg.CherryPickStemEndOffsetY,
					Stroke:      gg.CherryPickAccentColor,
					StrokeWidth: gg.CherryPickStemStrokeWidth,
					Transform:   baseTranslate,
				},
			)
		default:
			radius := gg.CommitRadius
			if commit.CommitType == GitGraphCommitTypeMerge {
				radius = gg.MergeRadiusOuter
			}
			layout.Circles = append(layout.Circles, LayoutCircle{
				Class:     "commit " + commit.ID + " commit" + intString(colorIdx),
				CX:        commit.X,
				CY:        commit.Y,
				R:         radius,
				Fill:      color,
				Stroke:    color,
				Transform: baseTranslate,
			})
			if symbolType == GitGraphCommitTypeMerge {
				layout.Circles = append(layout.Circles, LayoutCircle{
					Class:     "commit commit-merge " + commit.ID + " commit" + intString(colorIdx),
					CX:        commit.X,
					CY:        commit.Y,
					R:         gg.MergeRadiusInner,
					Fill:      theme.PrimaryColor,
					Stroke:    theme.PrimaryColor,
					Transform: baseTranslate,
				})
			}
			if symbolType == GitGraphCommitTypeReverse {
				size := gg.ReverseCrossSize
				layout.Paths = append(layout.Paths, LayoutPath{
					Class: "commit-reverse",
					D: "M " + formatFloat(commit.X-size) + "," + formatFloat(commit.Y-size) +
						" L " + formatFloat(commit.X+size) + "," + formatFloat(commit.Y+size) +
						" M " + formatFloat(commit.X-size) + "," + formatFloat(commit.Y+size) +
						" L " + formatFloat(commit.X+size) + "," + formatFloat(commit.Y-size),
					Fill:        "none",
					Stroke:      theme.PrimaryColor,
					StrokeWidth: gg.ReverseStrokeWidth,
					Transform:   baseTranslate,
				})
			}
		}
	}

	for _, commit := range data.Commits {
		if commit.Label != nil {
			extraTransform := gitGraphTransformString(commit.Label.Transform)
			transform := combineTransform(baseTranslate, extraTransform)
			layout.Rects = append(layout.Rects, LayoutRect{
				Class:     "commit-label-bkg",
				X:         commit.Label.BGX,
				Y:         commit.Label.BGY,
				W:         commit.Label.BGWidth,
				H:         commit.Label.BGHeight,
				Fill:      theme.GitCommitLabelBackground,
				Opacity:   gg.CommitLabelBGOpacity,
				Stroke:    "none",
				Transform: transform,
			})
			layout.Texts = append(layout.Texts, LayoutText{
				Class:     "commit-label",
				X:         commit.Label.TextX,
				Y:         commit.Label.TextY,
				Value:     commit.Label.Text,
				Anchor:    "start",
				Size:      gg.CommitLabelFontSize,
				Color:     theme.GitCommitLabelColor,
				Transform: transform,
			})
		}
		for _, tag := range commit.Tags {
			points := make([]Point, 0, len(tag.Points))
			points = append(points, tag.Points...)
			extraTransform := gitGraphTransformString(tag.Transform)
			transform := combineTransform(baseTranslate, extraTransform)
			layout.Polygons = append(layout.Polygons, LayoutPolygon{
				Class:     "tag-label-bkg",
				Points:    points,
				Fill:      theme.GitTagLabelBackground,
				Stroke:    theme.GitTagLabelBorder,
				Transform: transform,
			})
			layout.Circles = append(layout.Circles, LayoutCircle{
				Class:     "tag-hole",
				CX:        tag.HoleX,
				CY:        tag.HoleY,
				R:         gg.TagHoleRadius,
				Fill:      theme.TextColor,
				Stroke:    "none",
				Transform: transform,
			})
			layout.Texts = append(layout.Texts, LayoutText{
				Class:     "tag-label",
				X:         tag.TextX,
				Y:         tag.TextY,
				Value:     tag.Text,
				Anchor:    "start",
				Size:      gg.TagLabelFontSize,
				Color:     theme.GitTagLabelColor,
				Transform: transform,
			})
		}
	}

	return layout
}

func combineTransform(base string, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if base == "" {
		return extra
	}
	if extra == "" {
		return base
	}
	return base + " " + extra
}

func gitGraphTransformString(transform *gitGraphTransform) string {
	if transform == nil {
		return ""
	}
	parts := []string{}
	if transform.TranslateX != 0 || transform.TranslateY != 0 {
		parts = append(parts,
			"translate("+formatFloat(transform.TranslateX)+", "+formatFloat(transform.TranslateY)+")",
		)
	}
	if transform.RotateDeg != 0 {
		parts = append(parts,
			"rotate("+formatFloat(transform.RotateDeg)+", "+
				formatFloat(transform.RotateCX)+", "+formatFloat(transform.RotateCY)+")",
		)
	}
	return strings.Join(parts, " ")
}

func appendGitGraphMultilineText(
	layout *Layout,
	x float64,
	y float64,
	text string,
	class string,
	fontSize float64,
	lineHeight float64,
	color string,
	transform string,
) {
	lines := splitLinesPreserve(text)
	startY := y + fontSize
	for idx, line := range lines {
		layout.Texts = append(layout.Texts, LayoutText{
			Class:     class,
			X:         x,
			Y:         startY + float64(idx)*fontSize*lineHeight,
			Value:     line,
			Anchor:    "start",
			Size:      fontSize,
			Color:     color,
			Transform: transform,
		})
	}
}

func defaultGitGraphBranchOrder(index int) float64 {
	if index == 0 {
		return 0.0
	}
	denom := 1.0
	value := index
	for value > 0 {
		denom *= 10.0
		value /= 10
	}
	return float64(index) / denom
}

func measureGitGraphText(text string, fontSize, lineHeight, widthScale float64, fastMetrics bool) (float64, float64) {
	lines := splitLinesPreserve(text)
	maxWidth := 0.0
	for _, line := range lines {
		maxWidth = max(maxWidth, measureTextWidthWithFontSize(line, fontSize, fastMetrics))
	}
	return maxWidth * widthScale, float64(len(lines)) * fontSize * lineHeight
}

func gitGraphFindClosestParent(parents []string, commitPos map[string]Point, dir Direction) string {
	chosen := ""
	target := 0.0
	if dir == DirectionBottomTop {
		target = math.Inf(1)
	}
	for _, parent := range parents {
		pos, ok := commitPos[parent]
		if !ok {
			continue
		}
		axisPos := pos.X
		if dir == DirectionTopDown || dir == DirectionBottomTop {
			axisPos = pos.Y
		}
		accept := false
		if dir == DirectionBottomTop {
			accept = axisPos <= target
		} else {
			accept = axisPos >= target
		}
		if accept {
			target = axisPos
			chosen = parent
		}
	}
	return chosen
}

func gitGraphFindClosestParentBottomTop(parents []string, commitPos map[string]Point) string {
	chosen := ""
	maxPos := math.Inf(1)
	for _, parent := range parents {
		pos, ok := commitPos[parent]
		if !ok {
			continue
		}
		if pos.Y <= maxPos {
			maxPos = pos.Y
			chosen = parent
		}
	}
	return chosen
}

func gitGraphFindClosestParentPosition(commit *GitCommit, commitPos map[string]Point) (float64, bool) {
	parent := gitGraphFindClosestParent(commit.Parents, commitPos, DirectionBottomTop)
	if parent == "" {
		return 0, false
	}
	pos, ok := commitPos[parent]
	if !ok {
		return 0, false
	}
	return pos.Y, true
}

func gitGraphCalculateCommitPosition(commit *GitCommit, commitStep float64, commitPos map[string]Point) float64 {
	closestParentPos := 0.0
	if parentPos, ok := gitGraphFindClosestParentPosition(commit, commitPos); ok {
		closestParentPos = parentPos
	}
	return closestParentPos + commitStep
}

func gitGraphSetCommitPosition(
	commit *GitCommit,
	curPos float64,
	layoutOffset float64,
	branchPos map[string]gitBranchPosInfo,
	commitPos map[string]Point,
) {
	info := branchPos[commit.Branch]
	commitPos[commit.ID] = Point{X: info.Pos, Y: curPos + layoutOffset}
}

func gitGraphSetRootPosition(
	commit *GitCommit,
	curPos float64,
	defaultPos float64,
	branchPos map[string]gitBranchPosInfo,
	commitPos map[string]Point,
) {
	info := branchPos[commit.Branch]
	commitPos[commit.ID] = Point{X: info.Pos, Y: curPos + defaultPos}
}

func gitGraphSetParallelBottomTopPositions(
	commits []*GitCommit,
	defaultPos float64,
	commitStep float64,
	layoutOffset float64,
	branchPos map[string]gitBranchPosInfo,
	commitPos map[string]Point,
) {
	curPos := defaultPos
	maxPosition := defaultPos
	roots := []*GitCommit{}
	for _, commit := range commits {
		if len(commit.Parents) > 0 {
			curPos = gitGraphCalculateCommitPosition(commit, commitStep, commitPos)
			maxPosition = max(maxPosition, curPos)
		} else {
			roots = append(roots, commit)
		}
		gitGraphSetCommitPosition(commit, curPos, layoutOffset, branchPos, commitPos)
	}
	curPos = maxPosition
	for _, commit := range roots {
		gitGraphSetRootPosition(commit, curPos, defaultPos, branchPos, commitPos)
	}
	for _, commit := range commits {
		if len(commit.Parents) == 0 {
			continue
		}
		closestParent := gitGraphFindClosestParentBottomTop(commit.Parents, commitPos)
		if closestParent == "" {
			continue
		}
		parentPos, ok := commitPos[closestParent]
		if !ok {
			continue
		}
		curPos = parentPos.Y - commitStep
		if curPos <= maxPosition {
			maxPosition = curPos
		}
		info := branchPos[commit.Branch]
		commitPos[commit.ID] = Point{
			X: info.Pos,
			Y: curPos - layoutOffset,
		}
	}
}

func gitGraphCalculatePosition(
	commit *GitCommit,
	dir Direction,
	defaultPos float64,
	commitStep float64,
	commitPos map[string]Point,
) float64 {
	if len(commit.Parents) > 0 {
		parent := gitGraphFindClosestParent(commit.Parents, commitPos, dir)
		if parent != "" {
			parentPos := commitPos[parent]
			if dir == DirectionTopDown {
				return parentPos.Y + commitStep
			}
			if dir == DirectionBottomTop {
				current := commitPos[commit.ID]
				return current.Y - commitStep
			}
			return parentPos.X + commitStep
		}
	} else {
		if dir == DirectionTopDown {
			return defaultPos
		}
		if dir == DirectionBottomTop {
			current := commitPos[commit.ID]
			return current.Y - commitStep
		}
		return 0.0
	}
	return 0.0
}

func gitGraphCommitPosition(
	commit *GitCommit,
	pos float64,
	isParallel bool,
	dir Direction,
	layoutOffset float64,
	branchPos map[string]gitBranchPosInfo,
) (x float64, y float64, posWithOffset float64) {
	posWithOffset = pos + layoutOffset
	if dir == DirectionBottomTop && isParallel {
		posWithOffset = pos
	}
	info := branchPos[commit.Branch]
	branchAxisPos := info.Pos
	if dir == DirectionTopDown || dir == DirectionBottomTop {
		return branchAxisPos, posWithOffset, posWithOffset
	}
	return posWithOffset, branchAxisPos, posWithOffset
}

func updateBoundsLineGitgraph(minX, minY, maxX, maxY *float64, x1, y1, x2, y2 float64) {
	*minX = min(*minX, min(x1, x2))
	*minY = min(*minY, min(y1, y2))
	*maxX = max(*maxX, max(x1, x2))
	*maxY = max(*maxY, max(y1, y2))
}

func updateBoundsRectGitgraph(minX, minY, maxX, maxY *float64, x, y, width, height float64, transform *gitGraphTransform) {
	corners := []Point{
		{X: x, Y: y},
		{X: x + width, Y: y},
		{X: x + width, Y: y + height},
		{X: x, Y: y + height},
	}
	updateBoundsPointsGitgraph(minX, minY, maxX, maxY, corners, transform)
}

func updateBoundsPointsGitgraph(minX, minY, maxX, maxY *float64, points []Point, transform *gitGraphTransform) {
	for _, point := range points {
		px, py := applyGitGraphTransform(point.X, point.Y, transform)
		*minX = min(*minX, px)
		*minY = min(*minY, py)
		*maxX = max(*maxX, px)
		*maxY = max(*maxY, py)
	}
}

func applyGitGraphTransform(x, y float64, transform *gitGraphTransform) (float64, float64) {
	if transform == nil {
		return x, y
	}
	px := x + transform.TranslateX
	py := y + transform.TranslateY
	if math.Abs(transform.RotateDeg) > math.SmallestNonzeroFloat64 {
		angle := transform.RotateDeg * math.Pi / 180.0
		cosA := math.Cos(angle)
		sinA := math.Sin(angle)
		dx := px - transform.RotateCX
		dy := py - transform.RotateCY
		px = transform.RotateCX + dx*cosA - dy*sinA
		py = transform.RotateCY + dx*sinA + dy*cosA
	}
	return px, py
}

func gitGraphArrowPath(
	dir Direction,
	commitA GitCommit,
	commitB GitCommit,
	p1 Point,
	p2 Point,
	commits []GitCommit,
	cfg GitGraphConfig,
	lanes *[]float64,
) string {
	p1x, p1y := p1.X, p1.Y
	p2x, p2y := p2.X, p2.Y
	reroute := shouldRerouteGitGraphArrow(dir, commitA, commitB, p1, p2, commits)
	radius := cfg.ArrowRadius
	if reroute {
		radius = cfg.ArrowRerouteRadius
	}
	arc := "A " + formatFloat(radius) + " " + formatFloat(radius) + ", 0, 0, 0,"
	arc2 := "A " + formatFloat(radius) + " " + formatFloat(radius) + ", 0, 0, 1,"
	offset := radius

	if reroute {
		lineY := findGitGraphLane(min(p1y, p2y), max(p1y, p2y), lanes, cfg, 0)
		lineX := findGitGraphLane(min(p1x, p2x), max(p1x, p2x), lanes, cfg, 0)
		switch dir {
		case DirectionTopDown:
			if p1x < p2x {
				return "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(lineX-radius) + " " + formatFloat(p1y) +
					" " + arc2 + " " + formatFloat(lineX) + " " + formatFloat(p1y+offset) +
					" L " + formatFloat(lineX) + " " + formatFloat(p2y-radius) +
					" " + arc + " " + formatFloat(lineX+offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
			return "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
				" L " + formatFloat(lineX+radius) + " " + formatFloat(p1y) +
				" " + arc + " " + formatFloat(lineX) + " " + formatFloat(p1y+offset) +
				" L " + formatFloat(lineX) + " " + formatFloat(p2y-radius) +
				" " + arc2 + " " + formatFloat(lineX-offset) + " " + formatFloat(p2y) +
				" L " + formatFloat(p2x) + " " + formatFloat(p2y)
		case DirectionBottomTop:
			if p1x < p2x {
				return "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(lineX-radius) + " " + formatFloat(p1y) +
					" " + arc + " " + formatFloat(lineX) + " " + formatFloat(p1y-offset) +
					" L " + formatFloat(lineX) + " " + formatFloat(p2y+radius) +
					" " + arc2 + " " + formatFloat(lineX+offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
			return "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
				" L " + formatFloat(lineX+radius) + " " + formatFloat(p1y) +
				" " + arc2 + " " + formatFloat(lineX) + " " + formatFloat(p1y-offset) +
				" L " + formatFloat(lineX) + " " + formatFloat(p2y+radius) +
				" " + arc + " " + formatFloat(lineX-offset) + " " + formatFloat(p2y) +
				" L " + formatFloat(p2x) + " " + formatFloat(p2y)
		default:
			if p1y < p2y {
				return "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(lineY-radius) +
					" " + arc + " " + formatFloat(p1x+offset) + " " + formatFloat(lineY) +
					" L " + formatFloat(p2x-radius) + " " + formatFloat(lineY) +
					" " + arc2 + " " + formatFloat(p2x) + " " + formatFloat(lineY+offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
			return "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
				" L " + formatFloat(p1x) + " " + formatFloat(lineY+radius) +
				" " + arc2 + " " + formatFloat(p1x+offset) + " " + formatFloat(lineY) +
				" L " + formatFloat(p2x-radius) + " " + formatFloat(lineY) +
				" " + arc + " " + formatFloat(p2x) + " " + formatFloat(lineY-offset) +
				" L " + formatFloat(p2x) + " " + formatFloat(p2y)
		}
	}

	lineDef := ""
	isMergeFromSideBranch := commitB.CommitType == GitGraphCommitTypeMerge &&
		(len(commitB.Parents) > 0 && commitA.ID != commitB.Parents[0])
	switch dir {
	case DirectionTopDown:
		if p1x < p2x {
			if isMergeFromSideBranch {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(p2y-radius) +
					" " + arc + " " + formatFloat(p1x+offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			} else {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p2x-radius) + " " + formatFloat(p1y) +
					" " + arc2 + " " + formatFloat(p2x) + " " + formatFloat(p1y+offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
		} else if p1x > p2x {
			if isMergeFromSideBranch {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(p2y-radius) +
					" " + arc2 + " " + formatFloat(p1x-offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			} else {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p2x+radius) + " " + formatFloat(p1y) +
					" " + arc + " " + formatFloat(p2x) + " " + formatFloat(p1y+offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
		}
	case DirectionBottomTop:
		if p1x < p2x {
			if isMergeFromSideBranch {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(p2y+radius) +
					" " + arc2 + " " + formatFloat(p1x+offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			} else {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p2x-radius) + " " + formatFloat(p1y) +
					" " + arc + " " + formatFloat(p2x) + " " + formatFloat(p1y-offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
		} else if p1x > p2x {
			if isMergeFromSideBranch {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(p2y+radius) +
					" " + arc + " " + formatFloat(p1x-offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			} else {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p2x-radius) + " " + formatFloat(p1y) +
					" " + arc + " " + formatFloat(p2x) + " " + formatFloat(p1y-offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
		}
	default:
		if p1y < p2y {
			if isMergeFromSideBranch {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p2x-radius) + " " + formatFloat(p1y) +
					" " + arc2 + " " + formatFloat(p2x) + " " + formatFloat(p1y+offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			} else {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(p2y-radius) +
					" " + arc + " " + formatFloat(p1x+offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
		} else if p1y > p2y {
			if isMergeFromSideBranch {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p2x-radius) + " " + formatFloat(p1y) +
					" " + arc + " " + formatFloat(p2x) + " " + formatFloat(p1y-offset) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			} else {
				lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
					" L " + formatFloat(p1x) + " " + formatFloat(p2y+radius) +
					" " + arc2 + " " + formatFloat(p1x+offset) + " " + formatFloat(p2y) +
					" L " + formatFloat(p2x) + " " + formatFloat(p2y)
			}
		}
	}
	if lineDef == "" {
		lineDef = "M " + formatFloat(p1x) + " " + formatFloat(p1y) +
			" L " + formatFloat(p2x) + " " + formatFloat(p2y)
	}
	return lineDef
}

func shouldRerouteGitGraphArrow(dir Direction, commitA, commitB GitCommit, p1, p2 Point, commits []GitCommit) bool {
	commitBIsFurthest := false
	if dir == DirectionTopDown || dir == DirectionBottomTop {
		commitBIsFurthest = p1.X < p2.X
	} else {
		commitBIsFurthest = p1.Y < p2.Y
	}
	branchToGetCurve := commitA.Branch
	if commitBIsFurthest {
		branchToGetCurve = commitB.Branch
	}
	for _, commit := range commits {
		if commit.Seq > commitA.Seq && commit.Seq < commitB.Seq && commit.Branch == branchToGetCurve {
			return true
		}
	}
	return false
}

func findGitGraphLane(y1, y2 float64, lanes *[]float64, cfg GitGraphConfig, depth int) float64 {
	candidate := y1 + math.Abs(y2-y1)/2.0
	if depth > cfg.LaneMaxDepth {
		return candidate
	}
	ok := true
	for _, lane := range *lanes {
		if math.Abs(lane-candidate) < cfg.LaneSpacing {
			ok = false
			break
		}
	}
	if ok {
		*lanes = append(*lanes, candidate)
		return candidate
	}
	diff := math.Abs(y1 - y2)
	return findGitGraphLane(y1, y2-diff/5.0, lanes, cfg, depth+1)
}
