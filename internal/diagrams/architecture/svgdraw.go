// Ported from mermaid 12.2.0, diagrams/architecture/svgDraw.ts (MIT).

package architecture

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// labelOptions are the options of the createText calls of this file:
// { useHtmlLabels: false, width, classes: 'architecture-service-label' }.
func labelOptions(width float64) ru.TextOptions {
	o := ru.DefaultTextOptions()
	o.UseHtmlLabels = false
	o.Width = width
	o.Classes = "architecture-service-label"
	return o
}

// getIconSVG is await getIconSVG(name, { height: size, width: size,
// fallbackPrefix: architectureIcons.prefix }). The icon is drawn after the
// first of the two turns of the microtask queue that the call takes.
func getIconSVG(ctx *render.Context, name string, size any) (string, error) {
	ctx.Await()
	svg, err := ru.GetIconSVG(ctx, name, map[string]any{
		"height":         size,
		"width":          size,
		"fallbackPrefix": architectureIconsPrefix,
	}, nil)
	ctx.Await()
	return svg, err
}

// drawEdges is drawEdges(edgesEl, cy, db, diagramId).
//
// The ends of an edge are dynamic values. Upstream adds the padding of the
// config to them with the + operator, and for a padding that is a string
// that operator joins texts: the end is then a text, and the path gets that
// text. For a padding that is a number, each end is a number.
func drawEdges(ctx *render.Context, edgesEl d3Element, cy *cytoscape.ArchitectureResult, db *ArchitectureDB, diagramID string) error {
	padding := db.GetConfigField("padding")
	iconSize := db.configNumber("iconSize")
	halfIconSize := iconSize / 2
	arrowSize := iconSize / 6
	halfArrowSize := arrowSize / 2

	parsedEdges := db.GetEdges()
	err := ctx.All(len(cy.Edges), func(i int) error {
		edge := cy.Edges[i]
		// edgeData(edge): the data that addEdges gave to cytoscape.
		parsedEdge := parsedEdges[edge.Index]
		source := parsedEdge.LhsID
		sourceDir := parsedEdge.LhsDir
		sourceArrow := parsedEdge.LhsInto
		sourceGroup := parsedEdge.LhsGroup
		target := parsedEdge.RhsID
		targetDir := parsedEdge.RhsDir
		targetArrow := parsedEdge.RhsInto
		targetGroup := parsedEdge.RhsGroup
		label := parsedEdge.Title
		var startX, startY any = edge.SourceEndpoint.X, edge.SourceEndpoint.Y
		midX, midY := edge.Midpoint.X, edge.Midpoint.Y
		var endX, endY any = edge.TargetEndpoint.X, edge.TargetEndpoint.Y

		// Adjust the edge distance if it has the {group} modifier
		groupEdgeShift := js.Add(padding, 4.0)
		// +18 comes from the service label height that extends the padding on the bottom side of each group
		if sourceGroup {
			if isArchitectureDirectionX(sourceDir) {
				if sourceDir == "L" {
					startX = js.Add(startX, -js.ToNumber(groupEdgeShift))
				} else {
					startX = js.Add(startX, groupEdgeShift)
				}
			} else {
				if sourceDir == "T" {
					startY = js.Add(startY, -js.ToNumber(groupEdgeShift))
				} else {
					startY = js.Add(startY, js.Add(groupEdgeShift, 18.0))
				}
			}
		}

		if targetGroup {
			if isArchitectureDirectionX(targetDir) {
				if targetDir == "L" {
					endX = js.Add(endX, -js.ToNumber(groupEdgeShift))
				} else {
					endX = js.Add(endX, groupEdgeShift)
				}
			} else {
				if targetDir == "T" {
					endY = js.Add(endY, -js.ToNumber(groupEdgeShift))
				} else {
					endY = js.Add(endY, js.Add(groupEdgeShift, 18.0))
				}
			}
		}

		// Adjust the edge distance if it doesn't have the {group} modifier and the endpoint is a junction node
		if n := db.GetNode(source); !sourceGroup && n != nil && n.Type == "junction" {
			if isArchitectureDirectionX(sourceDir) {
				if sourceDir == "L" {
					startX = js.Add(startX, halfIconSize)
				} else {
					startX = js.Add(startX, -halfIconSize)
				}
			} else {
				if sourceDir == "T" {
					startY = js.Add(startY, halfIconSize)
				} else {
					startY = js.Add(startY, -halfIconSize)
				}
			}
		}
		if n := db.GetNode(target); !targetGroup && n != nil && n.Type == "junction" {
			if isArchitectureDirectionX(targetDir) {
				if targetDir == "L" {
					endX = js.Add(endX, halfIconSize)
				} else {
					endX = js.Add(endX, -halfIconSize)
				}
			} else {
				if targetDir == "T" {
					endY = js.Add(endY, halfIconSize)
				} else {
					endY = js.Add(endY, -halfIconSize)
				}
			}
		}

		if edge.HasRScratch {
			g := edgesEl.Insert("g")

			g.Insert("path").
				Attr("d", "M "+js.ToString(startX)+","+js.ToString(startY)+" L "+js.Str(midX)+","+js.Str(midY)+" L"+js.ToString(endX)+","+js.ToString(endY)+" ").
				Attr("class", "edge").
				Attr("id", diagramID+"-"+utils.GetEdgeId(source, target, utils.EdgeIdOptions{Prefix: "L"}, ""))

			if sourceArrow {
				var xShift, yShift float64
				if isArchitectureDirectionX(sourceDir) {
					xShift = architectureDirectionArrowShift(sourceDir, js.ToNumber(startX), arrowSize)
				} else {
					xShift = js.ToNumber(startX) - halfArrowSize
				}
				if isArchitectureDirectionY(sourceDir) {
					yShift = architectureDirectionArrowShift(sourceDir, js.ToNumber(startY), arrowSize)
				} else {
					yShift = js.ToNumber(startY) - halfArrowSize
				}

				g.Insert("polygon").
					Attr("points", architectureDirectionArrow(sourceDir, arrowSize)).
					Attr("transform", "translate("+js.Str(xShift)+","+js.Str(yShift)+")").
					Attr("class", "arrow")
			}
			if targetArrow {
				var xShift, yShift float64
				if isArchitectureDirectionX(targetDir) {
					xShift = architectureDirectionArrowShift(targetDir, js.ToNumber(endX), arrowSize)
				} else {
					xShift = js.ToNumber(endX) - halfArrowSize
				}
				if isArchitectureDirectionY(targetDir) {
					yShift = architectureDirectionArrowShift(targetDir, js.ToNumber(endY), arrowSize)
				} else {
					yShift = js.ToNumber(endY) - halfArrowSize
				}

				g.Insert("polygon").
					Attr("points", architectureDirectionArrow(targetDir, arrowSize)).
					Attr("transform", "translate("+js.Str(xShift)+","+js.Str(yShift)+")").
					Attr("class", "arrow")
			}

			if truthy(label) {
				var axis string
				if !isArchitectureDirectionXY(sourceDir, targetDir) {
					if isArchitectureDirectionX(sourceDir) {
						axis = "X"
					} else {
						axis = "Y"
					}
				} else {
					axis = "XY"
				}

				width := 0.0
				if axis == "X" {
					width = math.Abs(js.ToNumber(startX) - js.ToNumber(endX))
				} else if axis == "Y" {
					// Reduce width by a factor of 1.5 to avoid overlapping service labels
					width = math.Abs(js.ToNumber(startY)-js.ToNumber(endY)) / 1.5
				} else {
					width = math.Abs(js.ToNumber(startX)-js.ToNumber(endX)) / 2
				}

				textElem := g.Append("g")
				if _, err := ru.CreateText(ctx, textElem, *label, labelOptions(width)); err != nil {
					return err
				}

				textElem.
					Attr("dy", "1em").
					Attr("alignment-baseline", "middle").
					Attr("dominant-baseline", "middle").
					Attr("text-anchor", "middle")

				if axis == "X" {
					textElem.Attr("transform", "translate("+js.Str(midX)+", "+js.Str(midY)+")")
				} else if axis == "Y" {
					textElem.Attr("transform", "translate("+js.Str(midX)+", "+js.Str(midY)+") rotate(-90)")
				} else if axis == "XY" {
					pair, ok := getArchitectureDirectionPair(sourceDir, targetDir)
					if ok && isArchitecturePairXY(pair) {
						bboxOrig := textElem.BoundingClientRect()
						x, y := getArchitectureDirectionXYFactors(pair)

						textElem.
							Attr("dominant-baseline", "auto").
							Attr("transform", "rotate("+js.Str(-1*x*y*45)+")")

						// Calculate the new width/height with the rotation applied, and transform to the proper position
						bboxNew := textElem.BoundingClientRect()
						textElem.Attr(
							"transform",
							"\n"+
								"                translate("+js.Str(midX)+", "+js.Str(midY-float64(bboxOrig.Height/2))+")\n"+
								"                translate("+js.Str((x*bboxNew.Width)/2)+", "+js.Str((y*bboxNew.Height)/2)+")\n"+
								"                rotate("+js.Str(-1*x*y*45)+", 0, "+js.Str(bboxOrig.Height/2)+")\n"+
								"              ",
						)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// The await of the caller.
	ctx.Await()
	return nil
}

// drawGroups is drawGroups(groupsEl, cy, db, diagramId).
func drawGroups(ctx *render.Context, groupsEl d3Element, cy *cytoscape.ArchitectureResult, db *ArchitectureDB, diagramID string) error {
	padding := db.configNumber("padding")
	groupIconSize := padding * 0.75

	fontSize := db.configNumber("fontSize")

	iconSize := db.configNumber("iconSize")
	halfIconSize := iconSize / 2

	groups := db.GetGroups()
	err := ctx.All(len(cy.Nodes), func(i int) error {
		node := cy.Nodes[i]
		if node.Type == "group" {
			// nodeData(node): the data that addGroups gave to cytoscape.
			data := groups[node.Index]
			h, w, x1, y1 := node.BoundingBox.H, node.BoundingBox.W, node.BoundingBox.X1, node.BoundingBox.Y1

			groupsNode := groupsEl.Append("rect")
			groupsNode.
				Attr("id", diagramID+"-group-"+data.ID).
				Attr("x", x1+halfIconSize).
				Attr("y", y1+halfIconSize).
				Attr("width", w).
				Attr("height", h).
				Attr("class", "node-bkg")

			groupLabelContainer := groupsEl.Append("g")
			shiftedX1 := x1
			shiftedY1 := y1
			if truthy(data.Icon) {
				bkgElem := groupLabelContainer.Append("g")
				svg, err := getIconSVG(ctx, *data.Icon, groupIconSize)
				if err != nil {
					return err
				}
				bkgElem.HTML("<g>" + svg + "</g>")
				bkgElem.Attr(
					"transform",
					"translate("+
						js.Str(shiftedX1+halfIconSize+1)+
						", "+
						js.Str(shiftedY1+halfIconSize+1)+
						")",
				)
				shiftedX1 += groupIconSize
				// TODO: test with more values
				// - 1 - 2 comes from the Y axis transform of the icon and label
				shiftedY1 += float64(fontSize/2) - 1 - 2
			}
			if truthy(data.Title) {
				textElem := groupLabelContainer.Append("g")
				if _, err := ru.CreateText(ctx, textElem, *data.Title, labelOptions(w)); err != nil {
					return err
				}
				textElem.
					Attr("dy", "1em").
					Attr("alignment-baseline", "middle").
					Attr("dominant-baseline", "start").
					Attr("text-anchor", "start")

				textElem.Attr(
					"transform",
					"translate("+
						js.Str(shiftedX1+halfIconSize+4)+
						", "+
						js.Str(shiftedY1+halfIconSize+2)+
						")",
				)
			}
			db.SetElementForID(data.ID, groupsNode)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// The await of the caller.
	ctx.Await()
	return nil
}

// attrValue is a config value as the value of selection.attr: null removes
// the attribute, as undefined does.
func attrValue(v any) any {
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// nonDigits is text.replace(/\D/g, ”).
func nonDigits(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, text)
}

// drawServices is drawServices(db, elem, services, diagramId).
func drawServices(ctx *render.Context, db *ArchitectureDB, elem d3Element, services []*ArchitectureNode, diagramID string) error {
	for _, service := range services {
		serviceElem := elem.Append("g")
		iconSizeValue := db.GetConfigField("iconSize")
		iconSize := js.ToNumber(iconSizeValue)

		if truthy(service.Title) {
			textElem := serviceElem.Append("g")
			if _, err := ru.CreateText(ctx, textElem, *service.Title, labelOptions(iconSize*1.5)); err != nil {
				return err
			}

			textElem.
				Attr("dy", "1em").
				Attr("alignment-baseline", "middle").
				Attr("dominant-baseline", "middle").
				Attr("text-anchor", "middle")

			textElem.Attr("transform", "translate("+js.Str(iconSize/2)+", "+js.ToString(iconSizeValue)+")")
		}

		bkgElem := serviceElem.Append("g")
		if truthy(service.Icon) {
			// TODO: should a warning be given to end-users saying which icon names are available?
			svg, err := getIconSVG(ctx, *service.Icon, iconSizeValue)
			if err != nil {
				return err
			}
			bkgElem.HTML("<g>" + svg + "</g>")
		} else if truthy(service.IconText) {
			svg, err := getIconSVG(ctx, "blank", iconSizeValue)
			if err != nil {
				return err
			}
			bkgElem.HTML("<g>" + svg + "</g>")
			textElemContainer := bkgElem.Append("g")
			fo := textElemContainer.
				Append("foreignObject").
				Attr("width", attrValue(iconSizeValue)).
				Attr("height", attrValue(iconSizeValue))
			divElem := fo.
				Append("div").
				Attr("class", "node-icon-text").
				Attr("style", "height: "+js.ToString(iconSizeValue)+"px;").
				Append("div").
				HTML(ctx.SanitizeText(*service.IconText))
			// parseInt(...) ?? 16: the result of parseInt is never nullish.
			fontSize := js.ParseInt(nonDigits(divElem.ComputedStyle("font-size")), 0)
			divElem.Attr("style", "-webkit-line-clamp: "+js.Str(math.Floor((iconSize-2)/fontSize))+";")
		} else {
			bkgElem.
				Append("path").
				Attr("class", "node-bkg").
				Attr("id", diagramID+"-node-"+service.ID).
				Attr(
					"d",
					"M0,"+js.ToString(iconSizeValue)+" V5 Q0,0 5,0 H"+js.Str(iconSize-5)+" Q"+js.ToString(iconSizeValue)+",0 "+js.ToString(iconSizeValue)+",5 V"+js.ToString(iconSizeValue)+" Z",
				)
		}

		serviceElem.
			Attr("id", diagramID+"-service-"+service.ID).
			Attr("class", "architecture-service")

		bbox := serviceElem.BBox()
		service.Width = bbox.Width
		service.Height = bbox.Height
		service.HasSize = true
		db.SetElementForID(service.ID, serviceElem)
	}
	// The await of the caller.
	ctx.Await()
	return nil
}

// drawJunctions is drawJunctions(db, elem, junctions, diagramId).
func drawJunctions(db *ArchitectureDB, elem d3Element, junctions []*ArchitectureNode, diagramID string) {
	for _, junction := range junctions {
		junctionElem := elem.Append("g")
		iconSize := db.GetConfigField("iconSize")

		bkgElem := junctionElem.Append("g")
		bkgElem.
			Append("rect").
			Attr("id", diagramID+"-node-"+junction.ID).
			Attr("fill-opacity", "0").
			Attr("width", attrValue(iconSize)).
			Attr("height", attrValue(iconSize))

		junctionElem.Attr("class", "architecture-junction")

		// Upstream reads the bounding box of the element here and keeps the
		// size on the selection, where nothing reads it.
		db.SetElementForID(junction.ID, junctionElem)
	}
}
