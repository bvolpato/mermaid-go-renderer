package mermaid

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

func RenderSVG(layout Layout, theme Theme, _ LayoutConfig) string {
	width := max(1.0, layout.Width)
	height := max(1.0, layout.Height)
	viewBoxX := 0.0
	viewBoxY := 0.0
	viewBoxWidth := width
	viewBoxHeight := height
	if layout.ViewBoxWidth > 0 && layout.ViewBoxHeight > 0 {
		viewBoxX = layout.ViewBoxX
		viewBoxY = layout.ViewBoxY
		viewBoxWidth = layout.ViewBoxWidth
		viewBoxHeight = layout.ViewBoxHeight
	}
	widthAttr := formatFloat(width)
	if strings.TrimSpace(layout.SVGWidth) != "" {
		widthAttr = layout.SVGWidth
	}
	heightAttr := formatFloat(height)
	includeHeight := true
	if strings.TrimSpace(layout.SVGHeight) != "" {
		heightAttr = layout.SVGHeight
	} else if strings.TrimSpace(layout.SVGWidth) != "" {
		includeHeight = false
	}
	styleAttr := strings.TrimSpace(layout.SVGStyle)
	mermaidLike := useMermaidLikeDOM(layout.Kind)
	mermaidRoot := useMermaidLikeRoot(layout.Kind)
	groupPrimitives := mermaidLike || useMermaidGroupWrappers(layout.Kind)
	includeArrowMarkers := useArrowMarkers(layout.Kind)
	styleOnlyPrimitives := layout.Kind == DiagramArchitecture
	classDrivenPrimitives := layout.Kind == DiagramPacket
	fontFamily := theme.FontFamily
	if fontFamily == "" {
		fontFamily = "sans-serif"
	}
	background := theme.Background
	if background == "" {
		background = "#ffffff"
	}
	svgClass, ariaRoleDesc := diagramDOMClass(layout.Kind)
	if mermaidRoot {
		if layout.Kind == DiagramRadar {
			if styleAttr == "" {
				styleAttr = fmt.Sprintf("background-color: %s;", background)
			}
		} else {
			widthAttr = "100%"
			includeHeight = false
			if styleAttr == "" {
				styleAttr = fmt.Sprintf("max-width: %spx; background-color: %s;", formatFloat(viewBoxWidth), background)
			}
		}
	}

	var b strings.Builder
	b.Grow(4096)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString("\n")
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"`)
	if mermaidRoot {
		b.WriteString(` xmlns:xlink="http://www.w3.org/1999/xlink"`)
	}
	b.WriteString(fmt.Sprintf(` width="%s"`, html.EscapeString(widthAttr)))
	if includeHeight {
		b.WriteString(fmt.Sprintf(` height="%s"`, html.EscapeString(heightAttr)))
	}
	if mermaidRoot {
		b.WriteString(` id="my-svg"`)
	}
	if mermaidRoot && svgClass != "" {
		b.WriteString(` class="` + html.EscapeString(svgClass) + `"`)
	}
	b.WriteString(
		fmt.Sprintf(
			` viewBox="%s %s %s %s"`,
			formatFloat(viewBoxX),
			formatFloat(viewBoxY),
			formatFloat(viewBoxWidth),
			formatFloat(viewBoxHeight),
		),
	)
	if styleAttr != "" {
		b.WriteString(` style="` + html.EscapeString(styleAttr) + `"`)
	}
	if mermaidRoot {
		b.WriteString(` role="graphics-document document"`)
		if ariaRoleDesc != "" {
			b.WriteString(` aria-roledescription="` + html.EscapeString(ariaRoleDesc) + `"`)
		}
	}
	b.WriteString(">")
	b.WriteString("\n")
	if mermaidRoot {
		if layout.Kind == DiagramPacket {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}#my-svg p{margin:0;}#my-svg .packetByte{font-size:10px;}#my-svg .packetByte.start{fill:black;}#my-svg .packetByte.end{fill:black;}#my-svg .packetLabel{fill:black;font-size:12px;}#my-svg .packetTitle{fill:black;font-size:14px;}#my-svg .packetBlock{stroke:black;stroke-width:1;fill:#efefef;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramClass {
			b.WriteString(`<style>` + classStyleCSS() + `</style>`)
		} else if layout.Kind == DiagramState {
			b.WriteString(`<style>` + stateStyleCSS() + `</style>`)
		} else if layout.Kind == DiagramGantt {
			b.WriteString(`<style>` + ganttStyleCSS() + `</style>`)
		} else if layout.Kind == DiagramZenUML {
			b.WriteString(`<style>` + zenumlStyleCSS() + `</style>`)
		} else if layout.Kind == DiagramSankey {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .label{font-family:"trebuchet ms",verdana,arial,sans-serif;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramRadar {
			b.WriteString(`<style>` + radarStyleCSS() + `</style>`)
		} else if layout.Kind == DiagramArchitecture {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .edge{stroke-width:3;stroke:#333333;fill:none;}#my-svg .arrow{fill:#333333;}#my-svg .node-bkg{fill:none;stroke:hsl(240, 60%, 86.2745098039%);stroke-width:2px;stroke-dasharray:8;}#my-svg .node-icon-text{display:flex;align-items:center;}#my-svg .node-icon-text>div{color:#fff;margin:1px;height:fit-content;text-align:center;overflow:hidden;display:-webkit-box;-webkit-box-orient:vertical;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramMindmap {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .edge{stroke-width:3;}#my-svg .section--1 rect,#my-svg .section--1 path,#my-svg .section--1 circle,#my-svg .section--1 polygon,#my-svg .section--1 path{fill:hsl(240, 100%, 76.2745098039%);}#my-svg .section--1 text{fill:#ffffff;}#my-svg .node-icon--1{font-size:40px;color:#ffffff;}#my-svg .section-edge--1{stroke:hsl(240, 100%, 76.2745098039%);}#my-svg .edge-depth--1{stroke-width:17;}#my-svg .section--1 line{stroke:hsl(60, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-0 rect,#my-svg .section-0 path,#my-svg .section-0 circle,#my-svg .section-0 polygon,#my-svg .section-0 path{fill:hsl(60, 100%, 73.5294117647%);}#my-svg .section-0 text{fill:black;}#my-svg .node-icon-0{font-size:40px;color:black;}#my-svg .section-edge-0{stroke:hsl(60, 100%, 73.5294117647%);}#my-svg .edge-depth-0{stroke-width:14;}#my-svg .section-0 line{stroke:hsl(240, 100%, 83.5294117647%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-1 rect,#my-svg .section-1 path,#my-svg .section-1 circle,#my-svg .section-1 polygon,#my-svg .section-1 path{fill:hsl(80, 100%, 76.2745098039%);}#my-svg .section-1 text{fill:black;}#my-svg .node-icon-1{font-size:40px;color:black;}#my-svg .section-edge-1{stroke:hsl(80, 100%, 76.2745098039%);}#my-svg .edge-depth-1{stroke-width:11;}#my-svg .section-1 line{stroke:hsl(260, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-2 rect,#my-svg .section-2 path,#my-svg .section-2 circle,#my-svg .section-2 polygon,#my-svg .section-2 path{fill:hsl(270, 100%, 76.2745098039%);}#my-svg .section-2 text{fill:#ffffff;}#my-svg .node-icon-2{font-size:40px;color:#ffffff;}#my-svg .section-edge-2{stroke:hsl(270, 100%, 76.2745098039%);}#my-svg .edge-depth-2{stroke-width:8;}#my-svg .section-2 line{stroke:hsl(90, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-3 rect,#my-svg .section-3 path,#my-svg .section-3 circle,#my-svg .section-3 polygon,#my-svg .section-3 path{fill:hsl(300, 100%, 76.2745098039%);}#my-svg .section-3 text{fill:black;}#my-svg .node-icon-3{font-size:40px;color:black;}#my-svg .section-edge-3{stroke:hsl(300, 100%, 76.2745098039%);}#my-svg .edge-depth-3{stroke-width:5;}#my-svg .section-3 line{stroke:hsl(120, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-4 rect,#my-svg .section-4 path,#my-svg .section-4 circle,#my-svg .section-4 polygon,#my-svg .section-4 path{fill:hsl(330, 100%, 76.2745098039%);}#my-svg .section-4 text{fill:black;}#my-svg .node-icon-4{font-size:40px;color:black;}#my-svg .section-edge-4{stroke:hsl(330, 100%, 76.2745098039%);}#my-svg .edge-depth-4{stroke-width:2;}#my-svg .section-4 line{stroke:hsl(150, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-5 rect,#my-svg .section-5 path,#my-svg .section-5 circle,#my-svg .section-5 polygon,#my-svg .section-5 path{fill:hsl(0, 100%, 76.2745098039%);}#my-svg .section-5 text{fill:black;}#my-svg .node-icon-5{font-size:40px;color:black;}#my-svg .section-edge-5{stroke:hsl(0, 100%, 76.2745098039%);}#my-svg .edge-depth-5{stroke-width:-1;}#my-svg .section-5 line{stroke:hsl(180, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-6 rect,#my-svg .section-6 path,#my-svg .section-6 circle,#my-svg .section-6 polygon,#my-svg .section-6 path{fill:hsl(30, 100%, 76.2745098039%);}#my-svg .section-6 text{fill:black;}#my-svg .node-icon-6{font-size:40px;color:black;}#my-svg .section-edge-6{stroke:hsl(30, 100%, 76.2745098039%);}#my-svg .edge-depth-6{stroke-width:-4;}#my-svg .section-6 line{stroke:hsl(210, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-7 rect,#my-svg .section-7 path,#my-svg .section-7 circle,#my-svg .section-7 polygon,#my-svg .section-7 path{fill:hsl(90, 100%, 76.2745098039%);}#my-svg .section-7 text{fill:black;}#my-svg .node-icon-7{font-size:40px;color:black;}#my-svg .section-edge-7{stroke:hsl(90, 100%, 76.2745098039%);}#my-svg .edge-depth-7{stroke-width:-7;}#my-svg .section-7 line{stroke:hsl(270, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-8 rect,#my-svg .section-8 path,#my-svg .section-8 circle,#my-svg .section-8 polygon,#my-svg .section-8 path{fill:hsl(150, 100%, 76.2745098039%);}#my-svg .section-8 text{fill:black;}#my-svg .node-icon-8{font-size:40px;color:black;}#my-svg .section-edge-8{stroke:hsl(150, 100%, 76.2745098039%);}#my-svg .edge-depth-8{stroke-width:-10;}#my-svg .section-8 line{stroke:hsl(330, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-9 rect,#my-svg .section-9 path,#my-svg .section-9 circle,#my-svg .section-9 polygon,#my-svg .section-9 path{fill:hsl(180, 100%, 76.2745098039%);}#my-svg .section-9 text{fill:black;}#my-svg .node-icon-9{font-size:40px;color:black;}#my-svg .section-edge-9{stroke:hsl(180, 100%, 76.2745098039%);}#my-svg .edge-depth-9{stroke-width:-13;}#my-svg .section-9 line{stroke:hsl(0, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-10 rect,#my-svg .section-10 path,#my-svg .section-10 circle,#my-svg .section-10 polygon,#my-svg .section-10 path{fill:hsl(210, 100%, 76.2745098039%);}#my-svg .section-10 text{fill:black;}#my-svg .node-icon-10{font-size:40px;color:black;}#my-svg .section-edge-10{stroke:hsl(210, 100%, 76.2745098039%);}#my-svg .edge-depth-10{stroke-width:-16;}#my-svg .section-10 line{stroke:hsl(30, 100%, 86.2745098039%);stroke-width:3;}#my-svg .disabled,#my-svg .disabled circle,#my-svg .disabled text{fill:lightgray;}#my-svg .disabled text{fill:#efefef;}#my-svg .section-root rect,#my-svg .section-root path,#my-svg .section-root circle,#my-svg .section-root polygon{fill:hsl(240, 100%, 46.2745098039%);}#my-svg .section-root text{fill:#ffffff;}#my-svg .section-root span{color:#ffffff;}#my-svg .section-2 span{color:#ffffff;}#my-svg .icon-container{height:100%;display:flex;justify-content:center;align-items:center;}#my-svg .edge{fill:none;}#my-svg .mindmap-node-label{dy:1em;alignment-baseline:middle;text-anchor:middle;dominant-baseline:middle;text-align:center;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramSequence {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .actor{stroke:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);fill:#ECECFF;}#my-svg text.actor&gt;tspan{fill:black;stroke:none;}#my-svg .actor-line{stroke:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);}#my-svg .innerArc{stroke-width:1.5;stroke-dasharray:none;}#my-svg .messageLine0{stroke-width:1.5;stroke-dasharray:none;stroke:#333;}#my-svg .messageLine1{stroke-width:1.5;stroke-dasharray:2,2;stroke:#333;}#my-svg #arrowhead path{fill:#333;stroke:#333;}#my-svg .sequenceNumber{fill:white;}#my-svg #sequencenumber{fill:#333;}#my-svg #crosshead path{fill:#333;stroke:#333;}#my-svg .messageText{fill:#333;stroke:none;}#my-svg .labelBox{stroke:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);fill:#ECECFF;}#my-svg .labelText,#my-svg .labelText&gt;tspan{fill:black;stroke:none;}#my-svg .loopText,#my-svg .loopText&gt;tspan{fill:black;stroke:none;}#my-svg .loopLine{stroke-width:2px;stroke-dasharray:2,2;stroke:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);fill:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);}#my-svg .note{stroke:#aaaa33;fill:#fff5ad;}#my-svg .noteText,#my-svg .noteText&gt;tspan{fill:black;stroke:none;}#my-svg .activation0{fill:#f4f4f4;stroke:#666;}#my-svg .activation1{fill:#f4f4f4;stroke:#666;}#my-svg .activation2{fill:#f4f4f4;stroke:#666;}#my-svg .actorPopupMenu{position:absolute;}#my-svg .actorPopupMenuPanel{position:absolute;fill:#ECECFF;box-shadow:0px 8px 16px 0px rgba(0,0,0,0.2);filter:drop-shadow(3px 5px 2px rgb(0 0 0 / 0.4));}#my-svg .actor-man line{stroke:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);fill:#ECECFF;}#my-svg .actor-man circle,#my-svg line{stroke:hsl(259.6261682243, 59.7765363128%, 87.9019607843%);fill:#ECECFF;stroke-width:2px;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramBlock {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .label{font-family:"trebuchet ms",verdana,arial,sans-serif;color:#333;}#my-svg .cluster-label text{fill:#333;}#my-svg .cluster-label span,#my-svg p{color:#333;}#my-svg .label text,#my-svg span,#my-svg p{fill:#333;color:#333;}#my-svg .node rect,#my-svg .node circle,#my-svg .node ellipse,#my-svg .node polygon,#my-svg .node path{fill:#ECECFF;stroke:#9370DB;stroke-width:1px;}#my-svg .flowchart-label text{text-anchor:middle;}#my-svg .node .label{text-align:center;}#my-svg .node.clickable{cursor:pointer;}#my-svg .arrowheadPath{fill:#333333;}#my-svg .edgePath .path{stroke:#333333;stroke-width:2.0px;}#my-svg .flowchart-link{stroke:#333333;fill:none;}#my-svg .edgeLabel{background-color:rgba(232,232,232, 0.8);text-align:center;}#my-svg .edgeLabel rect{opacity:0.5;background-color:rgba(232,232,232, 0.8);fill:rgba(232,232,232, 0.8);}#my-svg .labelBkg{background-color:rgba(232, 232, 232, 0.5);}#my-svg .node .cluster{fill:rgba(255, 255, 222, 0.5);stroke:rgba(170, 170, 51, 0.2);box-shadow:rgba(50, 50, 93, 0.25) 0px 13px 27px -5px,rgba(0, 0, 0, 0.3) 0px 8px 16px -8px;stroke-width:1px;}#my-svg .cluster text{fill:#333;}#my-svg .cluster span,#my-svg p{color:#333;}#my-svg div.mermaidTooltip{position:absolute;text-align:center;max-width:200px;padding:2px;font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:12px;background:hsl(80, 100%, 96.2745098039%);border:1px solid #aaaa33;border-radius:2px;pointer-events:none;z-index:100;}#my-svg .flowchartTitleText{text-anchor:middle;font-size:18px;fill:#333;}#my-svg .label-icon{display:inline-block;height:1em;overflow:visible;vertical-align:-0.125em;}#my-svg .node .label-icon path{fill:currentColor;stroke:revert;stroke-width:revert;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramKanban {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}#my-svg p{margin:0;}#my-svg .kanban-column{fill:hsl(240, 100%, 86.2745098039%);stroke:hsl(240, 100%, 76.2745098039%);stroke-width:2;}#my-svg .kanban-card{fill:white;stroke:#9370DB;stroke-width:1px;}#my-svg .kanban-column-title{fill:black;font-size:16px;}#my-svg .kanban-card-text{fill:black;font-size:14px;}#my-svg .kanban-card-meta{fill:black;font-size:12px;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else if layout.Kind == DiagramTreemap {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .treemapNode.section{stroke:black;stroke-width:1;fill:#efefef;}#my-svg .treemapNode.leaf{stroke:black;stroke-width:1;fill:#efefef;}#my-svg .treemapLabel{fill:black;font-size:12px;}#my-svg .treemapValue{fill:black;font-size:10px;}#my-svg .treemapTitle{fill:black;font-size:14px;}#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}</style>`)
		} else {
			b.WriteString(`<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}#my-svg p{margin:0;}</style>`)
		}
		b.WriteString("\n")
	}
	if layout.Kind == DiagramZenUML {
		b.WriteString("<g/>\n")
		b.WriteString(renderZenUMLForeignObject(layout))
		b.WriteString("\n</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramSequence {
		b.WriteString(renderSequenceMermaid(layout, theme))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramBlock {
		b.WriteString(renderBlockMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramArchitecture {
		b.WriteString(renderArchitectureMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramMindmap {
		b.WriteString(renderMindmapMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramTreemap {
		b.WriteString(renderTreemapMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramGantt {
		b.WriteString(renderGanttMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramSankey && len(layout.SankeyNodes) > 0 {
		b.WriteString(renderSankeyMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if layout.Kind == DiagramRadar && len(layout.RadarAxes) > 0 {
		b.WriteString(renderRadarMermaid(layout))
		b.WriteString("</svg>\n")
		return b.String()
	}
	if mermaidRoot {
		if mermaidLike {
			b.WriteString("<g>\n")
		} else {
			b.WriteString("<g/>\n")
		}
	}
	if includeArrowMarkers {
		b.WriteString("<defs>\n")
		if layout.Kind == DiagramER {
			writeERMarkerDefs(&b)
		} else if layout.Kind == DiagramState {
			b.WriteString(`<marker id="my-svg_stateDiagram-barbEnd" refX="19" refY="7" markerWidth="20" markerHeight="14" markerUnits="userSpaceOnUse" orient="auto">`)
			b.WriteString(`<path d="M 19,7 L9,13 L14,7 L9,1 Z"/>`)
			b.WriteString(`</marker>`)
			b.WriteString("\n")
		} else if layout.Kind == DiagramClass {
			writeClassMarkerDefs(&b)
		} else if layout.Kind == DiagramTimeline || layout.Kind == DiagramJourney {
			b.WriteString(`<marker id="arrowhead" refX="5" refY="2" markerWidth="6" markerHeight="4" orient="auto">`)
			b.WriteString(`<path d="M 0,0 V 4 L6,2 Z"/>`)
			b.WriteString(`</marker>`)
			b.WriteString("\n")
		} else {
			b.WriteString(`<marker id="arrow-end" markerWidth="10" markerHeight="7" refX="8" refY="3.5" orient="auto" markerUnits="strokeWidth">`)
			b.WriteString(`<path d="M0,0 L10,3.5 L0,7 z" fill="`)
			b.WriteString(theme.LineColor)
			b.WriteString(`"/></marker>`)
			b.WriteString("\n")
			b.WriteString(`<marker id="arrow-start" markerWidth="10" markerHeight="7" refX="2" refY="3.5" orient="auto" markerUnits="strokeWidth">`)
			b.WriteString(`<path d="M10,0 L0,3.5 L10,7 z" fill="`)
			b.WriteString(theme.LineColor)
			b.WriteString(`"/></marker>`)
			b.WriteString("\n")
		}
		b.WriteString("</defs>\n")
	}

	if mermaidRoot {
		if mermaidLike {
			b.WriteString(`<g class="root">`)
		} else {
			b.WriteString(`<g>`)
		}
		b.WriteString("\n")
		if mermaidLike && layout.Kind != DiagramState {
			b.WriteString(`<g class="clusters"></g>`)
			b.WriteString("\n")
			b.WriteString(`<g class="edgePaths"></g>`)
			b.WriteString("\n")
			b.WriteString(`<g class="edgeLabels"></g>`)
			b.WriteString("\n")
			b.WriteString(`<g class="nodes">`)
			b.WriteString("\n")
		}
	} else {
		b.WriteString(
			fmt.Sprintf(
				`<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`,
				formatFloat(viewBoxX),
				formatFloat(viewBoxY),
				formatFloat(viewBoxWidth),
				formatFloat(viewBoxHeight),
				html.EscapeString(background),
			),
		)
		b.WriteString("\n")
	}

	if layout.Kind == DiagramState {
		b.WriteString(renderStateMermaid(layout, theme))
		if mermaidRoot {
			b.WriteString("</g>\n")
		}
		if mermaidRoot && mermaidLike {
			b.WriteString("</g>\n")
		}
		b.WriteString("</svg>\n")
		return b.String()
	}

	for _, rect := range layout.Rects {
		if groupPrimitives {
			b.WriteString(`<g class="node default" transform="translate(0,0)">`)
		}
		rectAsPath := mermaidLike || layout.Kind == DiagramTimeline
		rectID := strings.TrimSpace(rect.ID)
		rectClass := strings.TrimSpace(rect.Class)
		if rectAsPath {
			b.WriteString(`<path d="` + html.EscapeString(rectToPath(rect)) + `"`)
		} else {
			b.WriteString(`<rect`)
			b.WriteString(fmt.Sprintf(` x="%s" y="%s" width="%s" height="%s"`,
				formatFloat(rect.X), formatFloat(rect.Y), formatFloat(rect.W), formatFloat(rect.H)))
			if rect.RX > 0 {
				b.WriteString(fmt.Sprintf(` rx="%s"`, formatFloat(rect.RX)))
			}
			if rect.RY > 0 {
				b.WriteString(fmt.Sprintf(` ry="%s"`, formatFloat(rect.RY)))
			}
		}
		if rectID != "" {
			b.WriteString(` id="` + html.EscapeString(rectID) + `"`)
		}
		if rectClass != "" {
			b.WriteString(` class="` + html.EscapeString(rectClass) + `"`)
		}
		fill := defaultColor(rect.Fill, "none")
		stroke := defaultColor(rect.Stroke, "none")
		strokeWidth := defaultFloat(rect.StrokeWidth, 1)
		dash := strings.TrimSpace(rect.StrokeDasharray)
		if rect.Dashed && dash == "" {
			dash = "5,4"
		}
		if !styleOnlyPrimitives && !classDrivenPrimitives {
			b.WriteString(` fill="` + html.EscapeString(fill) + `"`)
			b.WriteString(` stroke="` + html.EscapeString(stroke) + `"`)
			b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
			if rect.FillOpacity > 0 && rect.FillOpacity < 1 {
				b.WriteString(fmt.Sprintf(` fill-opacity="%s"`, formatFloat(rect.FillOpacity)))
			}
			if rect.StrokeOpacity > 0 && rect.StrokeOpacity < 1 {
				b.WriteString(fmt.Sprintf(` stroke-opacity="%s"`, formatFloat(rect.StrokeOpacity)))
			}
			if rect.Opacity > 0 && rect.Opacity < 1 {
				b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(rect.Opacity)))
			}
		}
		if strings.TrimSpace(rect.Transform) != "" {
			b.WriteString(` transform="` + html.EscapeString(rect.Transform) + `"`)
		}
		if strings.TrimSpace(rect.TransformOrigin) != "" {
			b.WriteString(` transform-origin="` + html.EscapeString(rect.TransformOrigin) + `"`)
		}
		if styleOnlyPrimitives || mermaidLike {
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(
				fill,
				stroke,
				strokeWidth,
				dash,
				"",
				"",
				rect.FillOpacity,
				rect.StrokeOpacity,
				rect.Opacity,
			)) + `"`)
		}
		if !styleOnlyPrimitives && !classDrivenPrimitives && strings.TrimSpace(rect.StrokeDasharray) != "" {
			b.WriteString(` stroke-dasharray="` + html.EscapeString(rect.StrokeDasharray) + `"`)
		}
		if !styleOnlyPrimitives && !classDrivenPrimitives && rect.Dashed {
			b.WriteString(` stroke-dasharray="5,4"`)
		}
		b.WriteString("/>")
		if groupPrimitives {
			b.WriteString("</g>")
		}
		b.WriteString("\n")
	}

	for _, path := range layout.Paths {
		if groupPrimitives {
			b.WriteString(`<g class="edgePath" transform="translate(0,0)">`)
		}
		b.WriteString(`<path d="` + html.EscapeString(path.D) + `"`)
		pathID := strings.TrimSpace(path.ID)
		pathClass := strings.TrimSpace(path.Class)
		if pathID != "" {
			b.WriteString(` id="` + html.EscapeString(pathID) + `"`)
		}
		if pathClass != "" {
			b.WriteString(` class="` + html.EscapeString(pathClass) + `"`)
		}
		fill := defaultColor(path.Fill, "none")
		stroke := defaultColor(path.Stroke, "none")
		strokeWidth := defaultFloat(path.StrokeWidth, 1)
		dash := strings.TrimSpace(path.DashArray)
		journeyMouth := layout.Kind == DiagramJourney && strings.TrimSpace(pathClass) == "mouth"
		if !styleOnlyPrimitives && !journeyMouth {
			b.WriteString(` fill="` + html.EscapeString(fill) + `"`)
			b.WriteString(` stroke="` + html.EscapeString(stroke) + `"`)
			b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
			if path.FillOpacity > 0 && path.FillOpacity < 1 {
				b.WriteString(fmt.Sprintf(` fill-opacity="%s"`, formatFloat(path.FillOpacity)))
			}
			if path.StrokeOpacity > 0 && path.StrokeOpacity < 1 {
				b.WriteString(fmt.Sprintf(` stroke-opacity="%s"`, formatFloat(path.StrokeOpacity)))
			}
			if path.Opacity > 0 && path.Opacity < 1 {
				b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(path.Opacity)))
			}
		}
		if strings.TrimSpace(path.Transform) != "" {
			b.WriteString(` transform="` + html.EscapeString(path.Transform) + `"`)
		}
		if (styleOnlyPrimitives || mermaidLike) && !journeyMouth {
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(
				fill,
				stroke,
				strokeWidth,
				dash,
				path.LineCap,
				path.LineJoin,
				path.FillOpacity,
				path.StrokeOpacity,
				path.Opacity,
			)) + `"`)
		}
		if !styleOnlyPrimitives && strings.TrimSpace(path.DashArray) != "" && !journeyMouth {
			b.WriteString(` stroke-dasharray="` + html.EscapeString(path.DashArray) + `"`)
		}
		if !styleOnlyPrimitives && strings.TrimSpace(path.LineCap) != "" && !journeyMouth {
			b.WriteString(` stroke-linecap="` + html.EscapeString(path.LineCap) + `"`)
		}
		if !styleOnlyPrimitives && strings.TrimSpace(path.LineJoin) != "" && !journeyMouth {
			b.WriteString(` stroke-linejoin="` + html.EscapeString(path.LineJoin) + `"`)
		}
		b.WriteString("/>")
		if groupPrimitives {
			b.WriteString("</g>")
		}
		b.WriteString("\n")
	}

	for _, poly := range layout.Polygons {
		if groupPrimitives {
			b.WriteString(`<g class="node default" transform="translate(0,0)">`)
		}
		parts := make([]string, 0, len(poly.Points))
		for _, point := range poly.Points {
			parts = append(parts, formatFloat(point.X)+","+formatFloat(point.Y))
		}
		b.WriteString(`<polygon points="` + strings.Join(parts, " ") + `"`)
		fill := defaultColor(poly.Fill, "none")
		stroke := defaultColor(poly.Stroke, "none")
		strokeWidth := defaultFloat(poly.StrokeWidth, 1)
		b.WriteString(` fill="` + html.EscapeString(fill) + `"`)
		b.WriteString(` stroke="` + html.EscapeString(stroke) + `"`)
		b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
		if poly.FillOpacity > 0 && poly.FillOpacity < 1 {
			b.WriteString(fmt.Sprintf(` fill-opacity="%s"`, formatFloat(poly.FillOpacity)))
		}
		if poly.StrokeOpacity > 0 && poly.StrokeOpacity < 1 {
			b.WriteString(fmt.Sprintf(` stroke-opacity="%s"`, formatFloat(poly.StrokeOpacity)))
		}
		if poly.Opacity > 0 && poly.Opacity < 1 {
			b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(poly.Opacity)))
		}
		if strings.TrimSpace(poly.Transform) != "" {
			b.WriteString(` transform="` + html.EscapeString(poly.Transform) + `"`)
		}
		if mermaidLike {
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(
				fill,
				stroke,
				strokeWidth,
				"",
				"",
				"",
				poly.FillOpacity,
				poly.StrokeOpacity,
				poly.Opacity,
			)) + `"`)
		}
		b.WriteString("/>")
		if groupPrimitives {
			b.WriteString("</g>")
		}
		b.WriteString("\n")
	}

	for _, line := range layout.Lines {
		if groupPrimitives {
			if line.ArrowStart || line.ArrowEnd {
				b.WriteString(`<g class="edgePath" transform="translate(0,0)">`)
			} else {
				b.WriteString(`<g class="node default" transform="translate(0,0)">`)
			}
		}
		lineAsPath := mermaidLike
		if lineAsPath {
			b.WriteString(`<path d="M`)
			b.WriteString(formatFloat(line.X1))
			b.WriteString(",")
			b.WriteString(formatFloat(line.Y1))
			b.WriteString(" L")
			b.WriteString(formatFloat(line.X2))
			b.WriteString(",")
			b.WriteString(formatFloat(line.Y2))
			b.WriteString(`"`)
		} else {
			b.WriteString(`<line`)
			b.WriteString(fmt.Sprintf(` x1="%s" y1="%s" x2="%s" y2="%s"`,
				formatFloat(line.X1), formatFloat(line.Y1), formatFloat(line.X2), formatFloat(line.Y2)))
		}
		lineID := strings.TrimSpace(line.ID)
		lineClass := strings.TrimSpace(line.Class)
		if lineID != "" {
			b.WriteString(` id="` + html.EscapeString(lineID) + `"`)
		}
		if lineClass != "" {
			b.WriteString(` class="` + html.EscapeString(lineClass) + `"`)
		}
		if layout.Kind == DiagramClass && strings.TrimSpace(lineClass) == "relation" && strings.TrimSpace(lineID) != "" {
			b.WriteString(` data-edge="true"`)
			b.WriteString(` data-et="edge"`)
			b.WriteString(` data-id="` + html.EscapeString(lineID) + `"`)
			b.WriteString(` data-points="W10="`)
		}
		stroke := defaultColor(line.Stroke, "#333333")
		strokeWidth := defaultFloat(line.StrokeWidth, 1)
		dash := strings.TrimSpace(line.DashArray)
		if line.Dashed && dash == "" {
			dash = "5,4"
		}
		if !styleOnlyPrimitives {
			b.WriteString(` stroke="` + html.EscapeString(stroke) + `"`)
			b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
			if line.StrokeOpacity > 0 && line.StrokeOpacity < 1 {
				b.WriteString(fmt.Sprintf(` stroke-opacity="%s"`, formatFloat(line.StrokeOpacity)))
			}
			if line.Opacity > 0 && line.Opacity < 1 {
				b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(line.Opacity)))
			}
			if strings.TrimSpace(line.LineCap) != "" {
				b.WriteString(` stroke-linecap="` + html.EscapeString(line.LineCap) + `"`)
			}
			if strings.TrimSpace(line.LineJoin) != "" {
				b.WriteString(` stroke-linejoin="` + html.EscapeString(line.LineJoin) + `"`)
			}
			if strings.TrimSpace(line.DashArray) != "" {
				b.WriteString(` stroke-dasharray="` + html.EscapeString(line.DashArray) + `"`)
			}
		}
		if strings.TrimSpace(line.Transform) != "" {
			b.WriteString(` transform="` + html.EscapeString(line.Transform) + `"`)
		}
		if styleOnlyPrimitives || mermaidLike {
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(
				"none",
				stroke,
				strokeWidth,
				dash,
				line.LineCap,
				line.LineJoin,
				0,
				line.StrokeOpacity,
				line.Opacity,
			)) + `"`)
		}
		if !styleOnlyPrimitives && line.Dashed {
			b.WriteString(` stroke-dasharray="5,4"`)
		}
		if strings.TrimSpace(line.MarkerStart) != "" {
			b.WriteString(` marker-start="url(#` + html.EscapeString(line.MarkerStart) + `)"`)
		} else if includeArrowMarkers && line.ArrowStart && layout.Kind != DiagramTimeline {
			b.WriteString(` marker-start="url(#arrow-start)"`)
		}
		if strings.TrimSpace(line.MarkerEnd) != "" {
			b.WriteString(` marker-end="url(#` + html.EscapeString(line.MarkerEnd) + `)"`)
		} else if includeArrowMarkers && line.ArrowEnd {
			if layout.Kind == DiagramTimeline || layout.Kind == DiagramJourney {
				b.WriteString(` marker-end="url(#arrowhead)"`)
			} else {
				b.WriteString(` marker-end="url(#arrow-end)"`)
			}
		}
		b.WriteString("/>")
		if groupPrimitives {
			b.WriteString("</g>")
		}
		b.WriteString("\n")
	}

	for _, circle := range layout.Circles {
		if groupPrimitives {
			b.WriteString(`<g class="node default" transform="translate(0,0)">`)
		}
		b.WriteString(`<circle`)
		b.WriteString(fmt.Sprintf(` cx="%s" cy="%s" r="%s"`,
			formatFloat(circle.CX), formatFloat(circle.CY), formatFloat(circle.R)))
		if strings.TrimSpace(circle.ID) != "" {
			b.WriteString(` id="` + html.EscapeString(circle.ID) + `"`)
		}
		if strings.TrimSpace(circle.Class) != "" {
			b.WriteString(` class="` + html.EscapeString(circle.Class) + `"`)
		}
		fill := defaultColor(circle.Fill, "none")
		stroke := defaultColor(circle.Stroke, "none")
		strokeWidth := defaultFloat(circle.StrokeWidth, 1)
		journeyFace := layout.Kind == DiagramJourney && strings.TrimSpace(circle.Class) == "face"
		if !styleOnlyPrimitives && !journeyFace {
			b.WriteString(` fill="` + html.EscapeString(fill) + `"`)
			b.WriteString(` stroke="` + html.EscapeString(stroke) + `"`)
			b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
			if circle.FillOpacity > 0 && circle.FillOpacity < 1 {
				b.WriteString(fmt.Sprintf(` fill-opacity="%s"`, formatFloat(circle.FillOpacity)))
			}
			if circle.StrokeOpacity > 0 && circle.StrokeOpacity < 1 {
				b.WriteString(fmt.Sprintf(` stroke-opacity="%s"`, formatFloat(circle.StrokeOpacity)))
			}
			if circle.Opacity > 0 && circle.Opacity < 1 {
				b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(circle.Opacity)))
			}
		} else if journeyFace {
			b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
			b.WriteString(` overflow="visible"`)
		}
		if strings.TrimSpace(circle.Transform) != "" {
			b.WriteString(` transform="` + html.EscapeString(circle.Transform) + `"`)
		}
		if (styleOnlyPrimitives || mermaidLike) && !journeyFace {
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(
				fill,
				stroke,
				strokeWidth,
				"",
				"",
				"",
				circle.FillOpacity,
				circle.StrokeOpacity,
				circle.Opacity,
			)) + `"`)
		}
		b.WriteString("/>")
		if groupPrimitives {
			b.WriteString("</g>")
		}
		b.WriteString("\n")
	}

	for _, ellipse := range layout.Ellipses {
		if groupPrimitives {
			b.WriteString(`<g class="node default" transform="translate(0,0)">`)
		}
		b.WriteString(`<ellipse`)
		b.WriteString(fmt.Sprintf(` cx="%s" cy="%s" rx="%s" ry="%s"`,
			formatFloat(ellipse.CX), formatFloat(ellipse.CY), formatFloat(ellipse.RX), formatFloat(ellipse.RY)))
		if strings.TrimSpace(ellipse.ID) != "" {
			b.WriteString(` id="` + html.EscapeString(ellipse.ID) + `"`)
		}
		if strings.TrimSpace(ellipse.Class) != "" {
			b.WriteString(` class="` + html.EscapeString(ellipse.Class) + `"`)
		}
		fill := defaultColor(ellipse.Fill, "none")
		stroke := defaultColor(ellipse.Stroke, "none")
		strokeWidth := defaultFloat(ellipse.StrokeWidth, 1)
		if !styleOnlyPrimitives {
			b.WriteString(` fill="` + html.EscapeString(fill) + `"`)
			b.WriteString(` stroke="` + html.EscapeString(stroke) + `"`)
			b.WriteString(fmt.Sprintf(` stroke-width="%s"`, formatFloat(strokeWidth)))
			if ellipse.FillOpacity > 0 && ellipse.FillOpacity < 1 {
				b.WriteString(fmt.Sprintf(` fill-opacity="%s"`, formatFloat(ellipse.FillOpacity)))
			}
			if ellipse.StrokeOpacity > 0 && ellipse.StrokeOpacity < 1 {
				b.WriteString(fmt.Sprintf(` stroke-opacity="%s"`, formatFloat(ellipse.StrokeOpacity)))
			}
			if ellipse.Opacity > 0 && ellipse.Opacity < 1 {
				b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(ellipse.Opacity)))
			}
		}
		if strings.TrimSpace(ellipse.Transform) != "" {
			b.WriteString(` transform="` + html.EscapeString(ellipse.Transform) + `"`)
		}
		if styleOnlyPrimitives || mermaidLike {
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(
				fill,
				stroke,
				strokeWidth,
				"",
				"",
				"",
				ellipse.FillOpacity,
				ellipse.StrokeOpacity,
				ellipse.Opacity,
			)) + `"`)
		}
		b.WriteString("/>")
		if groupPrimitives {
			b.WriteString("</g>")
		}
		b.WriteString("\n")
	}

	for _, text := range layout.Texts {
		textID := strings.TrimSpace(text.ID)
		textClass := strings.TrimSpace(text.Class)
		anchor := text.Anchor
		if anchor == "" {
			anchor = "start"
		}
		size := text.Size
		if size <= 0 {
			size = 13
		}
		weight := text.Weight
		if weight == "" {
			weight = "400"
		}
		color := text.Color
		if color == "" {
			color = theme.PrimaryTextColor
		}
		family := text.FontFamily
		if family == "" {
			family = fontFamily
		}
		if mermaidLike {
			textW := max(1.0, measureTextWidth(text.Value, false)+8)
			textH := max(16.0, size*1.5)
			if layout.Kind == DiagramER && strings.TrimSpace(text.Value) == "" {
				textW = 0
				textH = 0
			}
			if layout.Kind == DiagramClass && strings.TrimSpace(textClass) == "class-edge-label" && strings.TrimSpace(text.Value) == "" {
				textW = 0
				textH = 0
			}
			x := text.X
			switch anchor {
			case "middle":
				x -= textW / 2
			case "end":
				x -= textW
			}
			y := text.Y - textH*0.8
			groupTransform := `translate(0,0)`
			labelWithTransform := layout.Kind == DiagramER || layout.Kind == DiagramClass
			if labelWithTransform {
				groupTransform = `translate(` + formatFloat(x) + `,` + formatFloat(y) + `)`
			}
			outerClass := "nodeLabel"
			if textClass != "" {
				outerClass = textClass
			}
			if layout.Kind == DiagramClass {
				if textClass == "class-edge-label" {
					outerClass = "edgeLabel"
				} else {
					outerClass = "label-group text"
				}
			}
			b.WriteString(`<g class="` + html.EscapeString(outerClass) + `" transform="` + groupTransform + `">`)
			if layout.Kind == DiagramER {
				b.WriteString(`<g class="label" style="text-align: center;">`)
				b.WriteString(`<path d="M0,0 H` + formatFloat(textW) + ` V` + formatFloat(textH) + ` H0 Z" fill="none" stroke="none" stroke-width="0"/>`)
			}
			if layout.Kind == DiagramClass && textClass == "class-edge-label" {
				b.WriteString(`<g class="label" data-id="` + html.EscapeString(textID) + `" transform="translate(0, 0)">`)
				b.WriteString(`<foreignObject width="` + formatFloat(textW) + `" height="` + formatFloat(textH) + `">`)
				b.WriteString(`<div class="labelBkg" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;" xmlns="http://www.w3.org/1999/xhtml"><span class="edgeLabel">`)
				if strings.TrimSpace(text.Value) != "" {
					b.WriteString(`<p>`)
					b.WriteString(html.EscapeString(text.Value))
					b.WriteString(`</p>`)
				}
				b.WriteString(`</span></div></foreignObject></g>`)
			} else if layout.Kind == DiagramClass {
				styleAttr := ""
				if weight != "" && weight != "400" {
					styleAttr = ` style="font-weight: ` + html.EscapeString(weight) + `"`
				}
				b.WriteString(`<g class="label"` + styleAttr + ` transform="translate(0,-12)">`)
				b.WriteString(`<foreignObject width="` + formatFloat(textW) + `" height="` + formatFloat(textH) + `">`)
				b.WriteString(`<div style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;" xmlns="http://www.w3.org/1999/xhtml"><span class="nodeLabel markdown-node-label" style=""><p>`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</p></span></div></foreignObject></g>`)
			} else {
				b.WriteString(`<foreignObject`)
				if labelWithTransform {
					b.WriteString(fmt.Sprintf(` width="%s" height="%s"`, formatFloat(textW), formatFloat(textH)))
				} else {
					b.WriteString(fmt.Sprintf(` x="%s" y="%s" width="%s" height="%s"`,
						formatFloat(x), formatFloat(y), formatFloat(textW), formatFloat(textH)))
				}
				if text.Opacity > 0 && text.Opacity < 1 {
					b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(text.Opacity)))
				}
				if strings.TrimSpace(text.Transform) != "" {
					b.WriteString(` transform="` + html.EscapeString(text.Transform) + `"`)
				}
				b.WriteString(`><div xmlns="http://www.w3.org/1999/xhtml" style="display: inline-block; white-space: nowrap;">`)
				b.WriteString(`<span class="nodeLabel" style="font-size: `)
				b.WriteString(formatFloat(size))
				b.WriteString(`px; font-family: `)
				b.WriteString(html.EscapeString(family))
				b.WriteString(`; font-weight: `)
				b.WriteString(html.EscapeString(weight))
				b.WriteString(`; color: `)
				b.WriteString(html.EscapeString(defaultColor(color, "#1b263b")))
				b.WriteString(`;">`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</span></div></foreignObject>`)
			}
			if layout.Kind == DiagramER {
				b.WriteString(`</g>`)
			}
			b.WriteString(`</g>`)
			b.WriteString("\n")
		} else {
			if groupPrimitives {
				b.WriteString(`<g class="nodeLabel" transform="translate(0,0)">`)
			}
			if layout.Kind == DiagramKanban {
				textW := max(1.0, measureTextWidth(text.Value, false)+8)
				textH := max(24.0, size*1.5)
				if strings.TrimSpace(text.Value) == "" {
					textW = 0
					textH = 0
				}
				if strings.Contains(textClass, "kanban-card-text") && textW > 175 {
					textW = 175
					textH = max(textH, 48)
				}
				x := text.X
				switch anchor {
				case "middle":
					x -= textW / 2
				case "end":
					x -= textW
				}
				y := text.Y - textH
				groupClass := "label"
				if strings.Contains(textClass, "column-title") {
					groupClass = "cluster-label"
				}
				b.WriteString(`<g class="` + html.EscapeString(groupClass) + `" style="text-align:left !important" transform="translate(` + formatFloat(x) + `, ` + formatFloat(y) + `)">`)
				b.WriteString(`<rect/>`)
				b.WriteString(`<foreignObject width="` + formatFloat(textW) + `" height="` + formatFloat(textH) + `">`)
				divStyle := `text-align: center; display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 175px;`
				if strings.Contains(textClass, "kanban-card-text") && textH >= 48 {
					divStyle = `text-align: center; display: table; white-space: break-spaces; line-height: 1.5; max-width: 175px; width: 175px;`
				}
				b.WriteString(`<div style="` + divStyle + `" xmlns="http://www.w3.org/1999/xhtml"><span style="text-align:left !important" class="nodeLabel"><p>`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</p></span></div></foreignObject></g>`)
				b.WriteString("\n")
				if groupPrimitives {
					b.WriteString("</g>\n")
				}
				continue
			}
			if layout.Kind == DiagramJourney && strings.TrimSpace(text.Class) == "journey-title" {
				b.WriteString(`<text x="` + formatFloat(text.X) + `" y="` + formatFloat(text.Y) + `" font-size="4ex" font-weight="bold" fill="" font-family="` + html.EscapeString(text.FontFamily) + `">`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</text>`)
				b.WriteString("\n")
				if groupPrimitives {
					b.WriteString("</g>\n")
				}
				continue
			}
			if layout.Kind == DiagramJourney && text.BoxW > 0 && text.BoxH > 0 {
				b.WriteString(`<g>`)
				b.WriteString(`<switch>`)
				b.WriteString(`<foreignObject x="` + formatFloat(text.BoxX) + `" y="` + formatFloat(text.BoxY) + `" width="` + formatFloat(text.BoxW) + `" height="` + formatFloat(text.BoxH) + `">`)
				b.WriteString(`<div class="` + html.EscapeString(text.Class) + `" style="display: table; height: 100%; width: 100%;" xmlns="http://www.w3.org/1999/xhtml">`)
				b.WriteString(`<div class="label" style="display: table-cell; text-align: center; vertical-align: middle;">`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</div></div></foreignObject>`)
				b.WriteString(`<text x="` + formatFloat(text.X) + `" y="` + formatFloat(text.Y) + `" dominant-baseline="central" alignment-baseline="central" class="` + html.EscapeString(text.Class) + `" style="text-anchor: middle; font-size: 14px; font-family: &quot;Open Sans&quot;, sans-serif;">`)
				b.WriteString(`<tspan x="` + formatFloat(text.X) + `" dy="0">`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</tspan></text>`)
				b.WriteString(`</switch></g>`)
				b.WriteString("\n")
				if groupPrimitives {
					b.WriteString("</g>\n")
				}
				continue
			}
			preferTransformPos := layout.Kind == DiagramTimeline || layout.Kind == DiagramArchitecture
			isTitleLike := anchor == "start" && size > theme.FontSize+1
			useTransformPos := preferTransformPos && !isTitleLike
			positionTransform := ""
			b.WriteString(`<text`)
			if textID != "" {
				b.WriteString(` id="` + html.EscapeString(textID) + `"`)
			}
			if textClass != "" {
				b.WriteString(` class="` + html.EscapeString(textClass) + `"`)
			}
			if useTransformPos {
				positionTransform = "translate(" + formatFloat(text.X) + "," + formatFloat(text.Y) + ")"
			} else {
				b.WriteString(fmt.Sprintf(` x="%s" y="%s"`, formatFloat(text.X), formatFloat(text.Y)))
			}
			b.WriteString(` text-anchor="` + html.EscapeString(anchor) + `"`)
			omitInline := layout.Kind == DiagramTimeline || layout.Kind == DiagramArchitecture || layout.Kind == DiagramPacket || layout.Kind == DiagramJourney
			keepInline := size > theme.FontSize+1 || weight != "400"
			if !omitInline || keepInline {
				b.WriteString(` fill="` + html.EscapeString(defaultColor(color, "#1b263b")) + `"`)
				b.WriteString(` font-family="` + html.EscapeString(family) + `"`)
				b.WriteString(fmt.Sprintf(` font-size="%s"`, formatFloat(size)))
				b.WriteString(` font-weight="` + html.EscapeString(weight) + `"`)
			}
			if text.Opacity > 0 && text.Opacity < 1 {
				b.WriteString(fmt.Sprintf(` opacity="%s"`, formatFloat(text.Opacity)))
			}
			finalTransform := strings.TrimSpace(text.Transform)
			if strings.TrimSpace(positionTransform) != "" {
				if finalTransform == "" {
					finalTransform = positionTransform
				} else {
					finalTransform = positionTransform + " " + finalTransform
				}
			}
			if strings.TrimSpace(finalTransform) != "" {
				b.WriteString(` transform="` + html.EscapeString(finalTransform) + `"`)
			}
			if strings.TrimSpace(text.DominantBaseline) != "" {
				b.WriteString(` dominant-baseline="` + html.EscapeString(text.DominantBaseline) + `"`)
			}
			if useTspanText(layout.Kind) {
				b.WriteString(` alignment-baseline="middle" dominant-baseline="middle" dy="0">`)
				tspanX := formatFloat(text.X)
				if useTransformPos {
					tspanX = "0"
				}
				b.WriteString(`<tspan x="` + tspanX + `" dy="0">`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString(`</tspan></text>`)
				b.WriteString("\n")
			} else {
				b.WriteString(`>`)
				b.WriteString(html.EscapeString(text.Value))
				b.WriteString("</text>\n")
			}
			if groupPrimitives {
				b.WriteString("</g>\n")
			}
		}
	}

	if mermaidLike {
		b.WriteString("</g>\n")
	}
	if mermaidRoot {
		b.WriteString("</g>\n")
	}
	if mermaidRoot && mermaidLike {
		b.WriteString("</g>\n")
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func renderStateMermaid(layout Layout, theme Theme) string {
	var b strings.Builder
	b.Grow(8192)

	nodeFill := defaultColor(theme.PrimaryColor, "#ECECFF")
	nodeStroke := defaultColor(theme.PrimaryBorderColor, "#9370DB")
	edgeStroke := defaultColor(theme.LineColor, "#333333")

	b.WriteString(`<g class="clusters"></g>`)
	b.WriteString("\n")
	b.WriteString(`<g class="edgePaths">`)
	b.WriteString("\n")
	for idx, edge := range layout.Edges {
		edgeID := "edge" + intString(idx)
		b.WriteString(`<path d="M`)
		b.WriteString(formatFloat(edge.X1))
		b.WriteString(",")
		b.WriteString(formatFloat(edge.Y1))
		b.WriteString(" L")
		b.WriteString(formatFloat(edge.X2))
		b.WriteString(",")
		b.WriteString(formatFloat(edge.Y2))
		b.WriteString(`"`)
		b.WriteString(` class="edge-thickness-normal edge-pattern-solid transition"`)
		b.WriteString(` id="` + edgeID + `"`)
		b.WriteString(` data-id="` + edgeID + `"`)
		b.WriteString(` data-et="edge"`)
		b.WriteString(` data-edge="true"`)
		b.WriteString(` data-points="W10="`)
		b.WriteString(` fill="none"`)
		b.WriteString(` stroke="` + html.EscapeString(edgeStroke) + `"`)
		b.WriteString(` stroke-width="1"`)
		b.WriteString(` style="fill:none;;;fill:none"`)
		if edge.ArrowEnd || edge.From != "" {
			b.WriteString(` marker-end="url(#my-svg_stateDiagram-barbEnd)"`)
		}
		b.WriteString("/>")
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g class="edgeLabels">`)
	b.WriteString("\n")
	for idx, edge := range layout.Edges {
		edgeID := "edge" + intString(idx)
		label := strings.TrimSpace(edge.Label)
		textW := 0.0
		textH := 0.0
		outerTransform := ""
		innerX := 0.0
		innerY := 0.0
		if label != "" {
			textW = max(1.0, measureTextWidth(label, false)+8)
			textH = 24.0
			labelX := (edge.X1 + edge.X2) / 2
			labelY := (edge.Y1+edge.Y2)/2 - 6
			outerTransform = ` transform="translate(` + formatFloat(labelX) + `,` + formatFloat(labelY) + `)"`
			innerX = -textW / 2
			innerY = -textH / 2
		}
		b.WriteString(`<g class="edgeLabel"`)
		b.WriteString(outerTransform)
		b.WriteString(`>`)
		b.WriteString(`<g class="label" data-id="` + edgeID + `" transform="translate(` + formatFloat(innerX) + `, ` + formatFloat(innerY) + `)">`)
		b.WriteString(`<foreignObject width="` + formatFloat(textW) + `" height="` + formatFloat(textH) + `">`)
		b.WriteString(`<div class="labelBkg" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;" xmlns="http://www.w3.org/1999/xhtml"><span class="edgeLabel">`)
		if label != "" {
			b.WriteString(`<p>`)
			b.WriteString(html.EscapeString(label))
			b.WriteString(`</p>`)
		}
		b.WriteString(`</span></div></foreignObject></g></g>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g class="nodes">`)
	b.WriteString("\n")
	for idx, node := range layout.Nodes {
		cx := node.X + node.W/2
		cy := node.Y + node.H/2
		nodeID := "state-" + html.EscapeString(node.ID)
		if nodeID == "state-" {
			nodeID = "state-node-" + intString(idx)
		}

		switch node.Shape {
		case ShapeCircle:
			b.WriteString(`<g class="node default" id="` + nodeID + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
			b.WriteString(`<circle class="state-start" r="7" width="14" height="14" fill="` + html.EscapeString(edgeStroke) + `" stroke="` + html.EscapeString(edgeStroke) + `" stroke-width="1.5"/>`)
			b.WriteString(`</g>`)
			b.WriteString("\n")
		case ShapeDoubleCircle:
			b.WriteString(`<g class="node default" id="` + nodeID + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
			b.WriteString(`<g>`)
			b.WriteString(`<circle class="state-end" r="7" width="14" height="14" fill="` + html.EscapeString(nodeStroke) + `" stroke="white" stroke-width="1.5"/>`)
			b.WriteString(`<circle class="end-state-inner" r="5" width="10" height="10" fill="white" stroke-width="1.5"/>`)
			b.WriteString(`</g></g>`)
			b.WriteString("\n")
		case ShapeDiamond:
			b.WriteString(`<g class="node  statediagram-state" id="` + nodeID + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
			b.WriteString(`<g class="basic label-container outer-path">`)
			b.WriteString(`<path d="M0,` + formatFloat(-node.H/2) + ` L` + formatFloat(node.W/2) + `,0 L0,` + formatFloat(node.H/2) + ` L` + formatFloat(-node.W/2) + `,0 Z"`)
			b.WriteString(` fill="` + html.EscapeString(nodeFill) + `" stroke="` + html.EscapeString(nodeStroke) + `" stroke-width="1.8" stroke-dasharray="0 0"`)
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(nodeFill, nodeStroke, 1.8, "0 0", "", "", 0, 0, 0)) + `"`)
			b.WriteString(`/>`)
			b.WriteString(`</g></g>`)
			b.WriteString("\n")
		default:
			b.WriteString(`<g class="node  statediagram-state" id="` + nodeID + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
			b.WriteString(`<g class="basic label-container outer-path">`)
			d := rectToPath(LayoutRect{X: -node.W / 2, Y: -node.H / 2, W: node.W, H: node.H, RX: 6, RY: 6})
			b.WriteString(`<path d="` + html.EscapeString(d) + `"`)
			b.WriteString(` fill="` + html.EscapeString(nodeFill) + `" stroke="` + html.EscapeString(nodeStroke) + `" stroke-width="1.8" stroke-dasharray="0 0"`)
			b.WriteString(` style="` + html.EscapeString(mermaidStyle(nodeFill, nodeStroke, 1.8, "0 0", "", "", 0, 0, 0)) + `"`)
			b.WriteString(`/>`)
			b.WriteString(`</g></g>`)
			b.WriteString("\n")
		}
	}

	for _, node := range layout.Nodes {
		label := strings.TrimSpace(node.Label)
		if label == "" {
			continue
		}
		textW := max(1.0, measureTextWidth(label, false)+8)
		textH := 24.0
		x := node.X + node.W/2 - textW/2
		y := node.Y + node.H/2 - textH/2
		b.WriteString(`<g class="label" style="" transform="translate(` + formatFloat(x) + `, ` + formatFloat(y) + `)">`)
		b.WriteString(`<rect/>`)
		b.WriteString(`<foreignObject width="` + formatFloat(textW) + `" height="` + formatFloat(textH) + `">`)
		b.WriteString(`<div style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;" xmlns="http://www.w3.org/1999/xhtml"><span class="nodeLabel"><p>`)
		b.WriteString(html.EscapeString(label))
		b.WriteString(`</p></span></div></foreignObject></g>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")
	return b.String()
}

func renderGanttMermaid(layout Layout) string {
	var b strings.Builder
	b.Grow(8192)
	b.WriteString("<g/>\n")

	tickTexts := make([]LayoutText, 0)
	taskTexts := make([]LayoutText, 0)
	sectionTexts := make([]LayoutText, 0)
	titleText := LayoutText{}
	hasTitle := false
	for _, text := range layout.Texts {
		switch {
		case strings.TrimSpace(text.Class) == "gantt-tick-label":
			tickTexts = append(tickTexts, text)
		case strings.HasPrefix(strings.TrimSpace(text.Class), "sectionTitle"):
			sectionTexts = append(sectionTexts, text)
		case strings.Contains(strings.TrimSpace(text.Class), "taskText"):
			taskTexts = append(taskTexts, text)
		case strings.TrimSpace(text.Class) == "titleText":
			titleText = text
			hasTitle = true
		}
	}

	domainPath := ""
	for _, path := range layout.Paths {
		if strings.TrimSpace(path.Class) == "domain" {
			domainPath = path.D
			break
		}
	}

	sectionRects := make([]LayoutRect, 0)
	taskRects := make([]LayoutRect, 0)
	for _, rect := range layout.Rects {
		rectClass := strings.TrimSpace(rect.Class)
		switch {
		case strings.HasPrefix(rectClass, "section "):
			sectionRects = append(sectionRects, rect)
		case strings.HasPrefix(rectClass, "task"):
			taskRects = append(taskRects, rect)
		}
	}

	todayLine := LayoutLine{}
	hasToday := false
	for _, line := range layout.Lines {
		if strings.TrimSpace(line.Class) == "today" {
			todayLine = line
			hasToday = true
			break
		}
	}

	b.WriteString(`<g class="grid" transform="translate(75, 194)" fill="none" font-size="10" font-family="sans-serif" text-anchor="middle">`)
	b.WriteString("\n")
	if strings.TrimSpace(domainPath) != "" {
		b.WriteString(`<path class="domain" stroke="currentColor" d="` + html.EscapeString(domainPath) + `"></path>`)
		b.WriteString("\n")
	}
	for _, tick := range tickTexts {
		tickX := math.Round(tick.X) + 0.5
		b.WriteString(`<g class="tick" opacity="1" transform="translate(` + formatFloat(tickX) + `,0)">`)
		b.WriteString(`<line stroke="currentColor" y2="-159"></line>`)
		b.WriteString(`<text fill="#000" y="3" dy="1em" stroke="none" font-size="10" style="text-anchor: middle;">`)
		b.WriteString(html.EscapeString(tick.Value))
		b.WriteString(`</text></g>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g>`)
	b.WriteString("\n")
	for _, rect := range sectionRects {
		b.WriteString(`<rect x="` + formatFloat(rect.X) + `" y="` + formatFloat(rect.Y) + `" width="` + formatFloat(rect.W) + `" height="` + formatFloat(rect.H) + `" class="` + html.EscapeString(rect.Class) + `"></rect>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g>`)
	b.WriteString("\n")
	for _, rect := range taskRects {
		b.WriteString(`<rect`)
		if strings.TrimSpace(rect.ID) != "" {
			b.WriteString(` id="` + html.EscapeString(rect.ID) + `"`)
		}
		b.WriteString(` rx="` + formatFloat(rect.RX) + `" ry="` + formatFloat(rect.RY) + `"`)
		b.WriteString(` x="` + formatFloat(rect.X) + `" y="` + formatFloat(rect.Y) + `" width="` + formatFloat(rect.W) + `" height="` + formatFloat(rect.H) + `"`)
		if strings.TrimSpace(rect.TransformOrigin) != "" {
			b.WriteString(` transform-origin="` + html.EscapeString(rect.TransformOrigin) + `"`)
		}
		if strings.TrimSpace(rect.Class) != "" {
			b.WriteString(` class="` + html.EscapeString(rect.Class) + `"`)
		}
		b.WriteString(`></rect>`)
		b.WriteString("\n")
	}

	for _, text := range taskTexts {
		b.WriteString(`<text`)
		if strings.TrimSpace(text.ID) != "" {
			b.WriteString(` id="` + html.EscapeString(text.ID) + `"`)
		}
		b.WriteString(` font-size="11" x="` + formatFloat(text.X) + `" y="` + formatFloat(text.Y) + `"`)
		if strings.TrimSpace(text.Class) != "" {
			b.WriteString(` class="` + html.EscapeString(text.Class) + `"`)
		}
		b.WriteString(`>`)
		b.WriteString(html.EscapeString(text.Value))
		b.WriteString(`</text>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g>`)
	b.WriteString("\n")
	for _, text := range sectionTexts {
		b.WriteString(`<text dy="0em" x="` + formatFloat(text.X) + `" y="` + formatFloat(text.Y) + `" font-size="11" class="` + html.EscapeString(text.Class) + `">`)
		b.WriteString(`<tspan alignment-baseline="central" x="` + formatFloat(text.X) + `">`)
		b.WriteString(html.EscapeString(text.Value))
		b.WriteString(`</tspan></text>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	if hasToday {
		b.WriteString(`<g class="today">`)
		b.WriteString(`<line x1="` + formatFloat(todayLine.X1) + `" x2="` + formatFloat(todayLine.X2) + `" y1="` + formatFloat(todayLine.Y1) + `" y2="` + formatFloat(todayLine.Y2) + `" class="today"></line>`)
		b.WriteString(`</g>`)
		b.WriteString("\n")
	}
	if hasTitle {
		b.WriteString(`<text x="` + formatFloat(titleText.X) + `" y="` + formatFloat(titleText.Y) + `" class="titleText">`)
		b.WriteString(html.EscapeString(titleText.Value))
		b.WriteString(`</text>`)
		b.WriteString("\n")
	}
	return b.String()
}

func renderSequenceMermaid(layout Layout, theme Theme) string {
	participants := append([]string(nil), layout.SequenceParticipants...)
	if len(participants) == 0 {
		seen := map[string]bool{}
		for _, msg := range layout.SequenceMessages {
			if strings.TrimSpace(msg.From) != "" && !seen[msg.From] {
				seen[msg.From] = true
				participants = append(participants, msg.From)
			}
			if strings.TrimSpace(msg.To) != "" && !seen[msg.To] {
				seen[msg.To] = true
				participants = append(participants, msg.To)
			}
		}
	}
	labels := layout.SequenceParticipantLabels
	if labels == nil {
		labels = map[string]string{}
	}
	events := layout.SequenceEvents
	if len(events) == 0 {
		events = defaultSequenceEvents(layout.SequenceMessages)
	}
	plan := buildSequencePlan(participants, labels, layout.SequenceMessages, events, theme)

	var b strings.Builder
	b.Grow(16384)

	for i := len(participants) - 1; i >= 0; i-- {
		participant := participants[i]
		label := participant
		if named, ok := labels[participant]; ok && strings.TrimSpace(named) != "" {
			label = named
		}
		x := plan.ParticipantLeft[participant]
		w := plan.ParticipantWidth[participant]
		center := plan.ParticipantCenter[participant]
		b.WriteString(`<g>`)
		b.WriteString(`<rect x="` + formatFloat(x) + `" y="` + formatFloat(plan.BottomY) + `" fill="#eaeaea" stroke="#666" width="` + formatFloat(w) + `" height="65" name="` + html.EscapeString(participant) + `" rx="3" ry="3" class="actor actor-bottom"/>`)
		b.WriteString(`<text x="` + formatFloat(center) + `" y="` + formatFloat(plan.BottomY+32.5) + `" dominant-baseline="central" alignment-baseline="central" class="actor actor-box" style="text-anchor: middle; font-size: 16px; font-weight: 400;"><tspan x="` + formatFloat(center) + `" dy="0">` + html.EscapeString(label) + `</tspan></text>`)
		b.WriteString(`</g>`)
	}

	for i := len(participants) - 1; i >= 0; i-- {
		participant := participants[i]
		label := participant
		if named, ok := labels[participant]; ok && strings.TrimSpace(named) != "" {
			label = named
		}
		x := plan.ParticipantLeft[participant]
		w := plan.ParticipantWidth[participant]
		center := plan.ParticipantCenter[participant]
		b.WriteString(`<g>`)
		b.WriteString(`<line id="actor` + intString(i) + `" x1="` + formatFloat(center) + `" y1="65" x2="` + formatFloat(center) + `" y2="` + formatFloat(plan.LifelineEndY) + `" class="actor-line 200" stroke-width="0.5px" stroke="#999" name="` + html.EscapeString(participant) + `"/>`)
		b.WriteString(`<g id="root-` + intString(i) + `">`)
		b.WriteString(`<rect x="` + formatFloat(x) + `" y="0" fill="#eaeaea" stroke="#666" width="` + formatFloat(w) + `" height="65" name="` + html.EscapeString(participant) + `" rx="3" ry="3" class="actor actor-top"/>`)
		b.WriteString(`<text x="` + formatFloat(center) + `" y="32.5" dominant-baseline="central" alignment-baseline="central" class="actor actor-box" style="text-anchor: middle; font-size: 16px; font-weight: 400;"><tspan x="` + formatFloat(center) + `" dy="0">` + html.EscapeString(label) + `</tspan></text>`)
		b.WriteString(`</g></g>`)
	}

	b.WriteString(`<g/>`)
	b.WriteString("\n")
	writeSequenceDefs(&b)
	b.WriteString(`<g/>`)
	b.WriteString("\n")

	for _, activation := range plan.ActivationLayouts {
		b.WriteString(`<g><rect x="` + formatFloat(activation.X) + `" y="` + formatFloat(activation.Y) + `" fill="#EDF2AE" stroke="#666" width="` + formatFloat(activation.W) + `" height="` + formatFloat(activation.H) + `" class="activation` + intString(activation.ClassIndex) + `"/></g>`)
	}

	for _, loop := range plan.LoopLayouts {
		b.WriteString(`<g>`)
		b.WriteString(`<line x1="` + formatFloat(loop.StartX) + `" y1="` + formatFloat(loop.StartY) + `" x2="` + formatFloat(loop.StopX) + `" y2="` + formatFloat(loop.StartY) + `" class="loopLine"/>`)
		b.WriteString(`<line x1="` + formatFloat(loop.StopX) + `" y1="` + formatFloat(loop.StartY) + `" x2="` + formatFloat(loop.StopX) + `" y2="` + formatFloat(loop.StopY) + `" class="loopLine"/>`)
		b.WriteString(`<line x1="` + formatFloat(loop.StartX) + `" y1="` + formatFloat(loop.StopY) + `" x2="` + formatFloat(loop.StopX) + `" y2="` + formatFloat(loop.StopY) + `" class="loopLine"/>`)
		b.WriteString(`<line x1="` + formatFloat(loop.StartX) + `" y1="` + formatFloat(loop.StartY) + `" x2="` + formatFloat(loop.StartX) + `" y2="` + formatFloat(loop.StopY) + `" class="loopLine"/>`)
		for _, section := range loop.Sections {
			b.WriteString(`<line x1="` + formatFloat(loop.StartX) + `" y1="` + formatFloat(section.Y) + `" x2="` + formatFloat(loop.StopX) + `" y2="` + formatFloat(section.Y) + `" class="loopLine" style="stroke-dasharray: 3, 3;"/>`)
		}
		labelPoints := formatFloat(loop.StartX) + "," + formatFloat(loop.StartY) + " " +
			formatFloat(loop.StartX+50) + "," + formatFloat(loop.StartY) + " " +
			formatFloat(loop.StartX+50) + "," + formatFloat(loop.StartY+13) + " " +
			formatFloat(loop.StartX+41.6) + "," + formatFloat(loop.StartY+20) + " " +
			formatFloat(loop.StartX) + "," + formatFloat(loop.StartY+20)
		b.WriteString(`<polygon points="` + labelPoints + `" class="labelBox"/>`)
		b.WriteString(`<text x="` + formatFloat(loop.StartX+25) + `" y="` + formatFloat(loop.StartY+13) + `" text-anchor="middle" dominant-baseline="middle" alignment-baseline="middle" class="labelText" style="font-size: 16px; font-weight: 400;">` + html.EscapeString(loop.Kind) + `</text>`)
		if strings.TrimSpace(loop.Title) != "" {
			midX := (loop.StartX + loop.StopX) / 2
			b.WriteString(`<text x="` + formatFloat(midX) + `" y="` + formatFloat(loop.StartY+18) + `" text-anchor="middle" class="loopText" style="font-size: 16px; font-weight: 400;"><tspan x="` + formatFloat(midX) + `">[` + html.EscapeString(loop.Title) + `]</tspan></text>`)
		}
		for _, section := range loop.Sections {
			if strings.TrimSpace(section.Label) == "" {
				continue
			}
			midX := (loop.StartX + loop.StopX) / 2
			b.WriteString(`<text x="` + formatFloat(midX) + `" y="` + formatFloat(section.Y+18) + `" text-anchor="middle" class="loopText" style="font-size: 16px; font-weight: 400;">[` + html.EscapeString(section.Label) + `]</text>`)
		}
		b.WriteString(`</g>`)
	}

	for _, msg := range plan.MessageLayouts {
		b.WriteString(`<text x="` + formatFloat((msg.StartX+msg.StopX)/2) + `" y="` + formatFloat(msg.TextY) + `" text-anchor="middle" dominant-baseline="middle" alignment-baseline="middle" class="messageText" dy="1em" style="font-size: 16px; font-weight: 400;">`)
		b.WriteString(html.EscapeString(msg.Message.Label))
		b.WriteString(`</text>`)
		lineClass := "messageLine0"
		lineStyle := "fill: none;"
		if msg.Dashed {
			lineClass = "messageLine1"
			lineStyle = "stroke-dasharray: 3, 3; fill: none;"
		}
		if msg.Self {
			path := "M " + formatFloat(msg.StartX) + "," + formatFloat(msg.LineY) +
				" C " + formatFloat(msg.StartX+60) + "," + formatFloat(msg.LineY-10) +
				" " + formatFloat(msg.StartX+60) + "," + formatFloat(msg.LineY+30) +
				" " + formatFloat(msg.StartX) + "," + formatFloat(msg.LineY+20)
			b.WriteString(`<path d="` + path + `" class="` + lineClass + `" stroke-width="2" stroke="none" marker-end="url(#arrowhead)" style="` + lineStyle + `"/>`)
			continue
		}
		b.WriteString(`<line x1="` + formatFloat(msg.StartX) + `" y1="` + formatFloat(msg.LineY) + `" x2="` + formatFloat(msg.StopX) + `" y2="` + formatFloat(msg.LineY) + `" class="` + lineClass + `" stroke-width="2" stroke="none" marker-end="url(#arrowhead)" style="` + lineStyle + `"/>`)
	}

	return b.String()
}

func renderBlockMermaid(layout Layout) string {
	var b strings.Builder
	b.Grow(8192)
	b.WriteString(`<g/>`)
	writeBlockMarkers(&b)
	b.WriteString(`<g class="block">`)

	for _, node := range layout.Nodes {
		cx := node.X + node.W/2
		cy := node.Y + node.H/2
		b.WriteString(`<g class="node default default flowchart-label" id="` + html.EscapeString(node.ID) + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
		switch node.Shape {
		case ShapeDiamond:
			halfW := node.W / 2
			halfH := node.H / 2
			points := formatFloat(halfW) + ",0 " +
				formatFloat(node.W) + "," + formatFloat(-halfH) + " " +
				formatFloat(halfW) + "," + formatFloat(-node.H) + " " +
				"0," + formatFloat(-halfH)
			b.WriteString(`<polygon points="` + points + `" class="label-container" transform="translate(-` + formatFloat(halfW) + `,` + formatFloat(halfH) + `)" style=""/>`)
		case ShapeCylinder:
			rx := node.W / 2
			ry := node.H * 0.11125
			side := node.H - 2*ry
			path := "M 0," + formatFloat(ry) +
				" a " + formatFloat(rx) + "," + formatFloat(ry) + " 0,0,0 " + formatFloat(node.W) + ",0" +
				" a " + formatFloat(rx) + "," + formatFloat(ry) + " 0,0,0 -" + formatFloat(node.W) + ",0" +
				" l 0," + formatFloat(side) +
				" a " + formatFloat(rx) + "," + formatFloat(ry) + " 0,0,0 " + formatFloat(node.W) + ",0" +
				" l 0,-" + formatFloat(side)
			b.WriteString(`<path style="" d="` + path + `" transform="translate(-` + formatFloat(node.W/2) + `,-` + formatFloat(node.H/2) + `)"/>`)
		default:
			b.WriteString(`<rect class="basic label-container" style="" rx="0" ry="0" x="` + formatFloat(-node.W/2) + `" y="` + formatFloat(-node.H/2) + `" width="` + formatFloat(node.W) + `" height="` + formatFloat(node.H) + `"/>`)
		}

		labelW := max(1.0, measureTextWidth(node.Label, false))
		labelH := 18.5
		b.WriteString(`<g class="label" style="" transform="translate(` + formatFloat(-labelW/2) + `, -9.25)">`)
		b.WriteString(`<rect/>`)
		b.WriteString(`<foreignObject width="` + formatFloat(labelW) + `" height="` + formatFloat(labelH) + `">`)
		b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" style="display: inline-block; white-space: nowrap;"><span class="nodeLabel">`)
		b.WriteString(html.EscapeString(node.Label))
		b.WriteString(`</span></div></foreignObject></g>`)
		b.WriteString(`</g>`)
	}

	for idx, edge := range layout.Edges {
		edgeClass := "edge-thickness-normal edge-pattern-solid flowchart-link LS-a1 LE-b1"
		if edge.Style == EdgeDotted {
			edgeClass = "edge-thickness-normal edge-pattern-dotted flowchart-link LS-a1 LE-b1"
		}
		d := "M" + formatFloat(edge.X1) + "," + formatFloat(edge.Y1) +
			"L" + formatFloat(edge.X2) + "," + formatFloat(edge.Y2)
		edgeID := "1-" + edge.From + "-" + edge.To
		if idx > 0 {
			edgeID += "-" + intString(idx+1)
		}
		b.WriteString(`<path d="` + d + `" id="` + html.EscapeString(edgeID) + `" class="` + edgeClass + `" marker-end="url(#my-svg_block-pointEnd)"/>`)
	}

	b.WriteString(`</g>`)
	return b.String()
}

func renderMindmapMermaid(layout Layout) string {
	var b strings.Builder
	b.Grow(16384)

	rootID := strings.TrimSpace(layout.MindmapRootID)
	if rootID == "" && len(layout.MindmapNodes) > 0 {
		rootID = layout.MindmapNodes[0].ID
	}
	nodes := append([]MindmapNode(nil), layout.MindmapNodes...)
	if len(nodes) == 0 {
		return "<g/>"
	}

	indexByID := map[string]int{}
	childrenByID := map[string][]string{}
	byID := map[string]MindmapNode{}
	for i, node := range nodes {
		indexByID[node.ID] = i
		byID[node.ID] = node
		if strings.TrimSpace(node.Parent) != "" {
			childrenByID[node.Parent] = append(childrenByID[node.Parent], node.ID)
		}
	}

	sectionByID := map[string]int{rootID: -1}
	var assignSection func(string, int)
	assignSection = func(id string, section int) {
		sectionByID[id] = section
		for _, childID := range childrenByID[id] {
			assignSection(childID, section)
		}
	}
	for idx, childID := range childrenByID[rootID] {
		assignSection(childID, idx)
	}
	for _, node := range nodes {
		if _, ok := sectionByID[node.ID]; !ok {
			sectionByID[node.ID] = 0
		}
	}

	nonRoot := make([]MindmapNode, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.Parent) != "" {
			nonRoot = append(nonRoot, node)
		}
	}

	nodeLayoutByID := map[string]NodeLayout{}
	for _, node := range layout.Nodes {
		nodeLayoutByID[node.ID] = node
	}

	b.WriteString(`<g>`)
	b.WriteString(`<marker id="my-svg_mindmap-pointEnd" class="marker mindmap" viewBox="0 0 10 10" refX="5" refY="5" markerUnits="userSpaceOnUse" markerWidth="8" markerHeight="8" orient="auto"><path d="M 0 0 L 10 5 L 0 10 z" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<marker id="my-svg_mindmap-pointStart" class="marker mindmap" viewBox="0 0 10 10" refX="4.5" refY="5" markerUnits="userSpaceOnUse" markerWidth="8" markerHeight="8" orient="auto"><path d="M 0 5 L 10 10 L 10 0 z" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<g class="subgraphs"/>`)
	b.WriteString(`<g class="edgePaths">`)
	for i, line := range layout.Lines {
		if i >= len(nonRoot) {
			break
		}
		child := nonRoot[i]
		parentID := strings.TrimSpace(child.Parent)
		parentIdx, okParent := indexByID[parentID]
		childIdx, okChild := indexByID[child.ID]
		if !okParent || !okChild {
			continue
		}
		section := sectionByID[child.ID]
		depthClass := max(1, child.Level*2-1)
		edgeID := "edge_" + intString(parentIdx) + "_" + intString(childIdx)
		pathD := "M" + formatFloat(line.X1) + "," + formatFloat(line.Y1) +
			"L" + formatFloat(line.X2) + "," + formatFloat(line.Y2)
		b.WriteString(`<path d="` + pathD + `" id="` + edgeID + `" class="edge-thickness-normal edge-pattern-solid edge section-edge-` + intString(section) + ` edge-depth-` + intString(depthClass) + `" style="undefined;;;undefined" data-edge="true" data-et="edge" data-id="` + edgeID + `" data-points="W10="/>`)
	}
	b.WriteString(`</g>`)
	b.WriteString(`<g class="edgeLabels">`)
	for _, child := range nonRoot {
		parentIdx, okParent := indexByID[strings.TrimSpace(child.Parent)]
		childIdx, okChild := indexByID[child.ID]
		if !okParent || !okChild {
			continue
		}
		edgeID := "edge_" + intString(parentIdx) + "_" + intString(childIdx)
		b.WriteString(`<g class="edgeLabel"><g class="label" data-id="` + edgeID + `" transform="translate(0, 0)"><foreignObject width="0" height="0"><div xmlns="http://www.w3.org/1999/xhtml" class="labelBkg" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;"><span class="edgeLabel"></span></div></foreignObject></g></g>`)
	}
	b.WriteString(`</g>`)
	b.WriteString(`<g class="nodes">`)
	for i, node := range nodes {
		nodeLayout, ok := nodeLayoutByID[node.ID]
		if !ok {
			continue
		}
		cx := nodeLayout.X + nodeLayout.W/2
		cy := nodeLayout.Y + nodeLayout.H/2
		labelW := max(1.0, measureTextWidth(node.Label, true))
		if strings.TrimSpace(node.ID) == rootID {
			r := min(nodeLayout.W, nodeLayout.H) / 2
			b.WriteString(`<g class="node mindmap-node section-root section--1" id="node_` + intString(i) + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
			b.WriteString(`<circle class="basic label-container" style="" r="` + formatFloat(r) + `" cx="0" cy="0"/>`)
			b.WriteString(`<g class="label" style="" transform="translate(-` + formatFloat(labelW/2) + `, -12)">`)
			b.WriteString(`<rect/><foreignObject width="` + formatFloat(labelW) + `" height="24">`)
			b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;"><span class="nodeLabel"><p>`)
			b.WriteString(html.EscapeString(node.Label))
			b.WriteString(`</p></span></div></foreignObject></g></g>`)
			continue
		}

		section := sectionByID[node.ID]
		innerW := max(1.0, nodeLayout.W-10)
		halfW := nodeLayout.W / 2
		pathD := "M-" + formatFloat(halfW) + " 12\n" +
			"    v-24\n" +
			"    q0,-5 5,-5\n" +
			"    h" + formatFloat(innerW) + "\n" +
			"    q5,0 5,5\n" +
			"    v24\n" +
			"    q0,5 -5,5\n" +
			"    h-" + formatFloat(innerW) + "\n" +
			"    q-5,0 -5,-5\n" +
			"    Z"
		b.WriteString(`<g class="node mindmap-node section-` + intString(section) + `" id="node_` + intString(i) + `" transform="translate(` + formatFloat(cx) + `, ` + formatFloat(cy) + `)">`)
		b.WriteString(`<path id="node-` + intString(i) + `" class="node-bkg node-0" style="" d="` + html.EscapeString(pathD) + `"/>`)
		b.WriteString(`<line class="node-line-" x1="-` + formatFloat(halfW) + `" y1="17" x2="` + formatFloat(halfW) + `" y2="17"/>`)
		b.WriteString(`<g class="label" style="" transform="translate(-` + formatFloat(labelW/2) + `, -12)">`)
		b.WriteString(`<rect/><foreignObject width="` + formatFloat(labelW) + `" height="24">`)
		b.WriteString(`<div xmlns="http://www.w3.org/1999/xhtml" style="display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;"><span class="nodeLabel"><p>`)
		b.WriteString(html.EscapeString(node.Label))
		b.WriteString(`</p></span></div></foreignObject></g></g>`)
	}
	b.WriteString(`</g>`)
	b.WriteString(`</g>`)
	return b.String()
}

func renderTreemapMermaid(layout Layout) string {
	var b strings.Builder
	b.Grow(16384)

	sectionRects := make([]LayoutRect, 0, len(layout.Rects))
	leafRects := make([]LayoutRect, 0, len(layout.Rects))
	for _, rect := range layout.Rects {
		className := strings.TrimSpace(rect.Class)
		switch {
		case strings.Contains(className, "treemapSection"):
			sectionRects = append(sectionRects, rect)
		case strings.Contains(className, "treemapLeaf"):
			leafRects = append(leafRects, rect)
		}
	}
	if len(sectionRects) == 0 && len(leafRects) == 0 {
		return "<g/>"
	}

	type sectionTextPair struct {
		label *LayoutText
		value *LayoutText
	}
	sectionTextByIdx := make(map[int]sectionTextPair, len(sectionRects))
	leafLabelByIdx := make(map[int]*LayoutText, len(leafRects))
	leafValueByIdx := make(map[int]*LayoutText, len(leafRects))

	sectionIndexForPoint := func(x, y float64) int {
		bestIdx := -1
		bestArea := math.MaxFloat64
		for idx, rect := range sectionRects {
			if x < rect.X || x > rect.X+rect.W || y < rect.Y || y > rect.Y+rect.H {
				continue
			}
			area := rect.W * rect.H
			if area < bestArea {
				bestArea = area
				bestIdx = idx
			}
		}
		if bestIdx == -1 && len(sectionRects) > 0 {
			bestIdx = 0
		}
		return bestIdx
	}
	leafIndexForPoint := func(x, y float64) int {
		bestIdx := -1
		bestArea := math.MaxFloat64
		for idx, rect := range leafRects {
			if x < rect.X || x > rect.X+rect.W || y < rect.Y || y > rect.Y+rect.H {
				continue
			}
			area := rect.W * rect.H
			if area < bestArea {
				bestArea = area
				bestIdx = idx
			}
		}
		return bestIdx
	}

	for i := range layout.Texts {
		text := &layout.Texts[i]
		switch strings.TrimSpace(text.Class) {
		case "treemapSectionLabel":
			idx := sectionIndexForPoint(text.X, text.Y)
			if idx >= 0 {
				pair := sectionTextByIdx[idx]
				pair.label = text
				sectionTextByIdx[idx] = pair
			}
		case "treemapSectionValue":
			idx := sectionIndexForPoint(text.X, text.Y)
			if idx >= 0 {
				pair := sectionTextByIdx[idx]
				pair.value = text
				sectionTextByIdx[idx] = pair
			}
		case "treemapLabel":
			idx := leafIndexForPoint(text.X, text.Y)
			if idx >= 0 {
				leafLabelByIdx[idx] = text
			}
		case "treemapValue":
			idx := leafIndexForPoint(text.X, text.Y)
			if idx >= 0 {
				leafValueByIdx[idx] = text
			}
		}
	}

	rootValue := ""
	for i := 0; i < len(sectionRects); i++ {
		pair := sectionTextByIdx[i]
		if pair.value != nil && strings.TrimSpace(pair.value.Value) != "" {
			rootValue = pair.value.Value
			break
		}
	}

	textColorForFill := func(fill string) string {
		v := strings.TrimSpace(strings.ToLower(fill))
		if v == "" || v == "transparent" {
			return "black"
		}
		if strings.HasPrefix(v, "hsl(") && strings.HasSuffix(v, ")") {
			inside := strings.TrimSuffix(strings.TrimPrefix(v, "hsl("), ")")
			parts := strings.Split(inside, ",")
			if len(parts) > 0 {
				if hue, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
					h := math.Mod(hue+360, 360)
					if h >= 200 && h <= 300 {
						return "#ffffff"
					}
				}
			}
		}
		return "black"
	}

	b.WriteString(`<g/>`)
	b.WriteString(`<g transform="translate(0, 0)" class="treemapContainer">`)
	for idx, rect := range sectionRects {
		className := strings.TrimSpace(rect.Class)
		fill := defaultColor(rect.Fill, "transparent")
		stroke := defaultColor(rect.Stroke, "transparent")
		fillOpacity := defaultFloat(rect.FillOpacity, 0.6)
		strokeOpacity := defaultFloat(rect.StrokeOpacity, 0.4)
		strokeWidth := defaultFloat(rect.StrokeWidth, 2)
		hidden := idx == 0
		headerStyle := ""
		sectionStyle := ";"
		if hidden {
			headerStyle = "display: none;"
			sectionStyle = "display: none;"
		}

		pair := sectionTextByIdx[idx]
		labelValue := ""
		labelSize := 12.0
		if pair.label != nil {
			labelValue = pair.label.Value
			if pair.label.Size > 0 {
				labelSize = pair.label.Size
			}
		}
		valueValue := ""
		valueSize := 10.0
		if pair.value != nil {
			valueValue = pair.value.Value
			if pair.value.Size > 0 {
				valueSize = pair.value.Size
			}
		}
		if hidden && strings.TrimSpace(valueValue) == "" {
			valueValue = rootValue
		}
		textColor := textColorForFill(fill)

		b.WriteString(`<g class="treemapSection" transform="translate(` + formatFloat(rect.X) + `,` + formatFloat(rect.Y) + `)">`)
		b.WriteString(`<rect width="` + formatFloat(rect.W) + `" height="25" class="treemapSectionHeader" fill="none" fill-opacity="0.6" stroke-width="0.6" style="` + headerStyle + `"/>`)
		b.WriteString(`<clipPath id="clip-section-my-svg-` + intString(idx) + `"><rect width="` + formatFloat(max(1, rect.W-12)) + `" height="25"/></clipPath>`)
		b.WriteString(`<rect width="` + formatFloat(rect.W) + `" height="` + formatFloat(rect.H) + `" class="` + html.EscapeString(className) + `" fill="` + html.EscapeString(fill) + `" fill-opacity="` + formatFloat(fillOpacity) + `" stroke="` + html.EscapeString(stroke) + `" stroke-width="` + formatFloat(strokeWidth) + `" stroke-opacity="` + formatFloat(strokeOpacity) + `" style="` + sectionStyle + `"/>`)
		labelStyle := "dominant-baseline: middle; font-size: " + formatFloat(labelSize) + "px; fill:" + textColor + "; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"
		valueStyle := "text-anchor: end; dominant-baseline: middle; font-size: " + formatFloat(valueSize) + "px; fill:" + textColor + "; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"
		if hidden {
			labelStyle = "display: none;"
			valueStyle = "display: none;"
		}
		b.WriteString(`<text class="treemapSectionLabel" x="6" y="12.5" dominant-baseline="middle" font-weight="bold" style="` + labelStyle + `">` + html.EscapeString(labelValue) + `</text>`)
		b.WriteString(`<text class="treemapSectionValue" x="` + formatFloat(rect.W-10) + `" y="12.5" text-anchor="end" dominant-baseline="middle" font-style="italic" style="` + valueStyle + `">` + html.EscapeString(valueValue) + `</text>`)
		b.WriteString(`</g>`)
	}

	for idx, rect := range leafRects {
		fill := defaultColor(rect.Fill, "#efefef")
		fillOpacity := defaultFloat(rect.FillOpacity, 0.3)
		stroke := defaultColor(rect.Stroke, fill)
		strokeWidth := defaultFloat(rect.StrokeWidth, 3)
		label := leafLabelByIdx[idx]
		value := leafValueByIdx[idx]
		labelSize := 24.0
		if label != nil && label.Size > 0 {
			labelSize = label.Size
		}
		valueSize := 16.0
		if value != nil && value.Size > 0 {
			valueSize = value.Size
		}
		labelX := rect.W / 2
		labelY := rect.H / 2
		valueX := rect.W / 2
		valueY := rect.H / 2
		labelText := ""
		valueText := ""
		if label != nil {
			labelX = label.X - rect.X
			labelY = label.Y - rect.Y
			labelText = label.Value
		}
		if value != nil {
			valueX = value.X - rect.X
			valueY = value.Y - rect.Y
			valueText = value.Value
		}

		b.WriteString(`<g class="treemapNode treemapLeafGroup leaf` + intString(idx) + `x" transform="translate(` + formatFloat(rect.X) + `,` + formatFloat(rect.Y) + `)">`)
		b.WriteString(`<rect width="` + formatFloat(rect.W) + `" height="` + formatFloat(rect.H) + `" class="treemapLeaf" fill="` + html.EscapeString(fill) + `" style="" fill-opacity="` + formatFloat(fillOpacity) + `" stroke="` + html.EscapeString(stroke) + `" stroke-width="` + formatFloat(strokeWidth) + `"/>`)
		b.WriteString(`<clipPath id="clip-my-svg-` + intString(idx) + `"><rect width="` + formatFloat(max(1, rect.W-4)) + `" height="` + formatFloat(max(1, rect.H-4)) + `"/></clipPath>`)
		b.WriteString(`<text class="treemapLabel" x="` + formatFloat(labelX) + `" y="` + formatFloat(labelY) + `" style="text-anchor: middle; dominant-baseline: middle; font-size: ` + formatFloat(labelSize) + `px;fill:black;" clip-path="url(#clip-my-svg-` + intString(idx) + `)">` + html.EscapeString(labelText) + `</text>`)
		b.WriteString(`<text class="treemapValue" x="` + formatFloat(valueX) + `" y="` + formatFloat(valueY) + `" style="text-anchor: middle; dominant-baseline: hanging; font-size: ` + formatFloat(valueSize) + `px; fill: black;" clip-path="url(#clip-my-svg-` + intString(idx) + `)">` + html.EscapeString(valueText) + `</text>`)
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g>`)
	return b.String()
}

func renderArchitectureMermaid(layout Layout) string {
	var b strings.Builder
	b.Grow(8192)
	b.WriteString("<g/>\n")
	b.WriteString(`<g class="architecture-edges">`)
	b.WriteString("\n")
	for _, path := range layout.Paths {
		if strings.TrimSpace(path.Class) != "edge" {
			continue
		}
		b.WriteString(`<g><path d="` + html.EscapeString(path.D) + `" class="edge"`)
		if strings.TrimSpace(path.ID) != "" {
			b.WriteString(` id="` + html.EscapeString(path.ID) + `"`)
		}
		b.WriteString("/></g>\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g class="architecture-services">`)
	b.WriteString("\n")
	for _, service := range layout.ArchitectureServices {
		b.WriteString(`<g id="service-` + html.EscapeString(service.ID) + `" class="architecture-service" transform="translate(` + formatFloat(service.X) + `,` + formatFloat(service.Y) + `)">`)
		writeArchitectureLabel(&b, "middle", "middle", "middle", 40, 80, service.Label)
		b.WriteString(`<g><g>`)
		b.WriteString(architectureIconSVG(service.Icon, service.W))
		b.WriteString(`</g></g>`)
		b.WriteString(`</g>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")

	b.WriteString(`<g class="architecture-groups">`)
	b.WriteString("\n")
	for _, group := range layout.ArchitectureGroups {
		b.WriteString(`<rect id="group-` + html.EscapeString(group.ID) + `" x="` + formatFloat(group.X) + `" y="` + formatFloat(group.Y) + `" width="` + formatFloat(group.W) + `" height="` + formatFloat(group.H) + `" class="node-bkg"/>`)
		b.WriteString(`<g>`)
		b.WriteString(`<g transform="translate(` + formatFloat(group.X+1) + `, ` + formatFloat(group.Y+1) + `)">`)
		b.WriteString(`<g>`)
		b.WriteString(architectureIconSVG(group.Icon, 30))
		b.WriteString(`</g></g>`)
		writeArchitectureLabel(&b, "start", "middle", "start", group.X+34, group.Y+7, group.Label)
		b.WriteString(`</g>`)
		b.WriteString("\n")
	}
	b.WriteString(`</g>`)
	b.WriteString("\n")
	return b.String()
}

func writeArchitectureLabel(
	b *strings.Builder,
	anchor string,
	alignmentBaseline string,
	dominantBaseline string,
	x float64,
	y float64,
	label string,
) {
	b.WriteString(`<g dy="1em" alignment-baseline="` + html.EscapeString(alignmentBaseline) + `" dominant-baseline="` + html.EscapeString(dominantBaseline) + `" text-anchor="` + html.EscapeString(anchor) + `" transform="translate(` + formatFloat(x) + `, ` + formatFloat(y) + `)">`)
	b.WriteString(`<g><rect class="background" style="stroke: none"/>`)
	b.WriteString(`<text y="-10.1" style="">`)
	b.WriteString(`<tspan class="text-outer-tspan" x="0" y="-0.1em" dy="1.1em">`)
	b.WriteString(`<tspan font-style="normal" class="text-inner-tspan" font-weight="normal">`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</tspan></tspan></text></g></g>`)
}

func architectureIconSVG(icon string, size float64) string {
	dim := max(1.0, size)
	view := formatFloat(dim)
	return `<svg xmlns="http://www.w3.org/2000/svg" width="` + view + `" height="` + view + `" viewBox="0 0 80 80"><g>` +
		architectureIconBody(icon) +
		`</g></svg>`
}

func architectureIconBody(icon string) string {
	switch lower(strings.TrimSpace(icon)) {
	case "database":
		return `<rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><path id="b" data-name="4" d="m20,57.86c0,3.94,8.95,7.14,20,7.14s20-3.2,20-7.14" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><path id="c" data-name="3" d="m20,45.95c0,3.94,8.95,7.14,20,7.14s20-3.2,20-7.14" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><path id="d" data-name="2" d="m20,34.05c0,3.94,8.95,7.14,20,7.14s20-3.2,20-7.14" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse id="e" data-name="1" cx="40" cy="22.14" rx="20" ry="7.14" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="20" y1="57.86" x2="20" y2="22.14" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="60" y1="57.86" x2="60" y2="22.14" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/>`
	case "disk":
		return `<rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><rect x="20" y="15" width="40" height="50" rx="1" ry="1" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse cx="24" cy="19.17" rx=".8" ry=".83" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse cx="56" cy="19.17" rx=".8" ry=".83" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse cx="24" cy="60.83" rx=".8" ry=".83" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse cx="56" cy="60.83" rx=".8" ry=".83" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse cx="40" cy="33.75" rx="14" ry="14.58" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><ellipse cx="40" cy="33.75" rx="4" ry="4.17" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><path d="m37.51,42.52l-4.83,13.22c-.26.71-1.1,1.02-1.76.64l-4.18-2.42c-.66-.38-.81-1.26-.33-1.84l9.01-10.8c.88-1.05,2.56-.08,2.09,1.2Z" style="fill: #fff; stroke-width: 0px;"/>`
	case "internet":
		return `<rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><circle cx="40" cy="40" r="22.5" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="40" y1="17.5" x2="40" y2="62.5" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="17.5" y1="40" x2="62.5" y2="40" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><path d="m39.99,17.51c-15.28,11.1-15.28,33.88,0,44.98" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><path d="m40.01,17.51c15.28,11.1,15.28,33.88,0,44.98" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="19.75" y1="30.1" x2="60.25" y2="30.1" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="19.75" y1="49.9" x2="60.25" y2="49.9" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/>`
	case "cloud":
		return `<rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><path d="m65,47.5c0,2.76-2.24,5-5,5H20c-2.76,0-5-2.24-5-5,0-1.87,1.03-3.51,2.56-4.36-.04-.21-.06-.42-.06-.64,0-2.6,2.48-4.74,5.65-4.97,1.65-4.51,6.34-7.76,11.85-7.76.86,0,1.69.08,2.5.23,2.09-1.57,4.69-2.5,7.5-2.5,6.1,0,11.19,4.38,12.28,10.17,2.14.56,3.72,2.51,3.72,4.83,0,.03,0,.07-.01.1,2.29.46,4.01,2.48,4.01,4.9Z" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/>`
	default:
		return `<rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><rect x="17.5" y="17.5" width="45" height="45" rx="2" ry="2" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="17.5" y1="32.5" x2="62.5" y2="32.5" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><line x1="17.5" y1="47.5" x2="62.5" y2="47.5" style="fill: none; stroke: #fff; stroke-miterlimit: 10; stroke-width: 2px;"/><g><path d="m56.25,25c0,.27-.45.5-1,.5h-10.5c-.55,0-1-.23-1-.5s.45-.5,1-.5h10.5c.55,0,1,.23,1,.5Z" style="fill: #fff; stroke-width: 0px;"/><path d="m56.25,25c0,.27-.45.5-1,.5h-10.5c-.55,0-1-.23-1-.5s.45-.5,1-.5h10.5c.55,0,1,.23,1,.5Z" style="fill: none; stroke: #fff; stroke-miterlimit: 10;"/></g><g><path d="m56.25,40c0,.27-.45.5-1,.5h-10.5c-.55,0-1-.23-1-.5s.45-.5,1-.5h10.5c.55,0,1,.23,1,.5Z" style="fill: #fff; stroke-width: 0px;"/><path d="m56.25,40c0,.27-.45.5-1,.5h-10.5c-.55,0-1-.23-1-.5s.45-.5,1-.5h10.5c.55,0,1,.23,1,.5Z" style="fill: none; stroke: #fff; stroke-miterlimit: 10;"/></g><g><path d="m56.25,55c0,.27-.45.5-1,.5h-10.5c-.55,0-1-.23-1-.5s.45-.5,1-.5h10.5c.55,0,1,.23,1,.5Z" style="fill: #fff; stroke-width: 0px;"/><path d="m56.25,55c0,.27-.45.5-1,.5h-10.5c-.55,0-1-.23-1-.5s.45-.5,1-.5h10.5c.55,0,1,.23,1,.5Z" style="fill: none; stroke: #fff; stroke-miterlimit: 10;"/></g><g><circle cx="32.5" cy="25" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/><circle cx="27.5" cy="25" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/><circle cx="22.5" cy="25" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/></g><g><circle cx="32.5" cy="40" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/><circle cx="27.5" cy="40" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/><circle cx="22.5" cy="40" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/></g><g><circle cx="32.5" cy="55" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/><circle cx="27.5" cy="55" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/><circle cx="22.5" cy="55" r=".75" style="fill: #fff; stroke: #fff; stroke-miterlimit: 10;"/></g>`
	}
}

func writeBlockMarkers(b *strings.Builder) {
	b.WriteString(`<marker id="my-svg_block-pointEnd" class="marker block" viewBox="0 0 10 10" refX="6" refY="5" markerUnits="userSpaceOnUse" markerWidth="12" markerHeight="12" orient="auto"><path d="M 0 0 L 10 5 L 0 10 z" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<marker id="my-svg_block-pointStart" class="marker block" viewBox="0 0 10 10" refX="4.5" refY="5" markerUnits="userSpaceOnUse" markerWidth="12" markerHeight="12" orient="auto"><path d="M 0 5 L 10 10 L 10 0 z" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<marker id="my-svg_block-circleEnd" class="marker block" viewBox="0 0 10 10" refX="11" refY="5" markerUnits="userSpaceOnUse" markerWidth="11" markerHeight="11" orient="auto"><circle cx="5" cy="5" r="5" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<marker id="my-svg_block-circleStart" class="marker block" viewBox="0 0 10 10" refX="-1" refY="5" markerUnits="userSpaceOnUse" markerWidth="11" markerHeight="11" orient="auto"><circle cx="5" cy="5" r="5" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<marker id="my-svg_block-crossEnd" class="marker cross block" viewBox="0 0 11 11" refX="12" refY="5.2" markerUnits="userSpaceOnUse" markerWidth="11" markerHeight="11" orient="auto"><path d="M 1,1 l 9,9 M 10,1 l -9,9" class="arrowMarkerPath" style="stroke-width: 2; stroke-dasharray: 1, 0;"/></marker>`)
	b.WriteString(`<marker id="my-svg_block-crossStart" class="marker cross block" viewBox="0 0 11 11" refX="-1" refY="5.2" markerUnits="userSpaceOnUse" markerWidth="11" markerHeight="11" orient="auto"><path d="M 1,1 l 9,9 M 10,1 l -9,9" class="arrowMarkerPath" style="stroke-width: 2; stroke-dasharray: 1, 0;"/></marker>`)
}

func writeSequenceDefs(b *strings.Builder) {
	b.WriteString(`<defs><symbol id="computer" width="24" height="24"><path transform="scale(.5)" d="M2 2v13h20v-13h-20zm18 11h-16v-9h16v9zm-10.228 6l.466-1h3.524l.467 1h-4.457zm14.228 3h-24l2-6h2.104l-1.33 4h18.45l-1.297-4h2.073l2 6zm-5-10h-14v-7h14v7z"/></symbol></defs>`)
	b.WriteString("\n")
	b.WriteString(`<defs><symbol id="database" fill-rule="evenodd" clip-rule="evenodd"><path transform="scale(.5)" d="M2 6c0-2.2 4.5-4 10-4s10 1.8 10 4v12c0 2.2-4.5 4-10 4s-10-1.8-10-4v-12zm10-2c-5.5 0-8 1.7-8 2s2.5 2 8 2 8-1.7 8-2-2.5-2-8-2zm-8 6v8c0 .3 2.5 2 8 2s8-1.7 8-2v-8c-1.7 1.2-4.8 2-8 2s-6.3-.8-8-2z"/></symbol></defs>`)
	b.WriteString("\n")
	b.WriteString(`<defs><symbol id="clock" width="24" height="24"><path transform="scale(.5)" d="M12 2c5.514 0 10 4.486 10 10s-4.486 10-10 10-10-4.486-10-10 4.486-10 10-10zm0-2c-6.627 0-12 5.373-12 12s5.373 12 12 12 12-5.373 12-12-5.373-12-12-12zm5.848 12.459c.202.038.202.333.001.372-1.907.361-6.045 1.111-6.547 1.111-.719 0-1.301-.582-1.301-1.301 0-.512.77-5.447 1.125-7.445.034-.192.312-.181.343.014l.985 6.238 5.394 1.011z"/></symbol></defs>`)
	b.WriteString("\n")
	b.WriteString(`<defs><marker id="arrowhead" refX="7.9" refY="5" markerUnits="userSpaceOnUse" markerWidth="12" markerHeight="12" orient="auto-start-reverse"><path d="M -1 0 L 10 5 L 0 10 z"/></marker></defs>`)
	b.WriteString("\n")
	b.WriteString(`<defs><marker id="crosshead" markerWidth="15" markerHeight="8" orient="auto" refX="4" refY="4.5"><path fill="none" stroke="#000000" stroke-width="1pt" d="M 1,2 L 6,7 M 6,2 L 1,7" style="stroke-dasharray: 0, 0;"/></marker></defs>`)
	b.WriteString("\n")
	b.WriteString(`<defs><marker id="filled-head" refX="15.5" refY="7" markerWidth="20" markerHeight="28" orient="auto"><path d="M 18,7 L9,13 L14,7 L9,1 Z"/></marker></defs>`)
	b.WriteString("\n")
	b.WriteString(`<defs><marker id="sequencenumber" refX="15" refY="15" markerWidth="60" markerHeight="40" orient="auto"><circle cx="15" cy="15" r="6"/></marker></defs>`)
	b.WriteString("\n")
}

func renderZenUMLForeignObject(layout Layout) string {
	title := strings.TrimSpace(layout.ZenUMLTitle)
	if title == "" {
		title = "ZenUML"
	}

	participants := append([]string(nil), layout.ZenUMLParticipants...)
	if len(participants) == 0 {
		seen := map[string]bool{}
		for _, msg := range layout.ZenUMLMessages {
			if msg.From != "" && !seen[msg.From] {
				participants = append(participants, msg.From)
				seen[msg.From] = true
			}
			if msg.To != "" && !seen[msg.To] {
				participants = append(participants, msg.To)
				seen[msg.To] = true
			}
		}
	}
	if len(participants) == 0 {
		participants = []string{"Participant"}
	}

	indexByParticipant := map[string]int{}
	for i, name := range participants {
		indexByParticipant[name] = i
	}

	altByStart := map[int]ZenUMLAltBlock{}
	altElseAt := map[int]bool{}
	altEndAt := map[int]bool{}
	for _, block := range layout.ZenUMLAltBlocks {
		if block.Start < 0 || block.End < block.Start {
			continue
		}
		altByStart[block.Start] = block
		if block.ElseStart >= block.Start && block.ElseStart <= block.End {
			altElseAt[block.ElseStart] = true
		}
		altEndAt[block.End] = true
	}

	seqWidth := max(545.0, 120.0+float64(max(1, len(participants)-1))*114.0+130.0)

	var b strings.Builder
	b.Grow(8192)
	b.WriteString(`<foreignObject x="0" y="0" width="100%" height="100%">`)
	b.WriteString(`<div id="container-my-svg" xmlns="http://www.w3.org/1999/xhtml" style="display: flex;">`)
	b.WriteString(`<div id="zenUMLApp-my-svg"><div class="zenuml">`)
	b.WriteString(`<div class="p-1 bg-skin-canvas inline-block default"><div class="frame text-skin-base bg-skin-frame border-skin-frame relative m-1 origin-top-left whitespace-nowrap border rounded">`)
	b.WriteString(`<div class="header text-skin-title bg-skin-title border-skin-frame border-b p-1 flex justify-between rounded-t"><div class="left hide-export"></div><div class="right flex-grow flex justify-between">`)
	b.WriteString(`<div class="title text-skin-title text-base font-semibold">` + html.EscapeString(title) + `</div>`)
	b.WriteString(`<div class="hide-export flex items-center"><span class="flex items-center justify-center fill-current h-6 w-6 m-auto"></span></div>`)
	b.WriteString(`</div></div>`)
	b.WriteString(`<div class="zenuml sequence-diagram relative box-border text-left overflow-visible px-2.5 default origin-top-left" style="transform: scale(1);">`)
	b.WriteString(`<div class="relative z-container" style="padding-left: 10px;">`)
	b.WriteString(`<div class="life-line-layer lifeline-layer z-30 absolute h-full flex flex-col top-0 pt-2" data-participant-names="participantNames" style="min-width: auto; width: calc(100% - 10px); pointer-events: none;">`)
	b.WriteString(`<div class="z-lifeline-container relative grow">`)
	for i, participant := range participants {
		left := 50 + i*121
		escapedName := html.EscapeString(participant)
		b.WriteString(`<div id="` + escapedName + `" class="lifeline absolute flex flex-col h-full transform -translate-x-1/2" style="padding-top: 20px; left: ` + intString(left) + `px;">`)
		b.WriteString(`<div class="participant bg-skin-participant shadow-participant border-skin-participant text-skin-participant rounded text-base leading-4 flex flex-col justify-center z-10 h-10 top-8" data-participant-id="` + escapedName + `" style="transform: translateY(0px);">`)
		b.WriteString(`<div class="flex items-center justify-center"><div class="h-5 group flex flex-col justify-center"><div class="flex items-center justify-center"><label title="Click to edit" class="name leading-4 right px-1 editable-label-base cursor-pointer">` + escapedName + `</label></div></div></div>`)
		b.WriteString(`</div><div class="line w0 mx-auto flex-grow w-px bg-[linear-gradient(to_bottom,transparent_50%,var(--color-border-base)_50%)] bg-[length:1px_10px]"></div></div>`)
	}
	b.WriteString(`</div></div>`)
	b.WriteString(`<div class="message-layer relative z-30 pt-14 pb-10" style="width: ` + formatFloat(seqWidth) + `px;">`)
	b.WriteString(`<div class="block" data-origin="` + html.EscapeString(participants[0]) + `" style="padding-left: 51px;">`)

	openAlt := false
	for i, msg := range layout.ZenUMLMessages {
		if block, ok := altByStart[i]; ok {
			openAlt = true
			condition := strings.TrimSpace(block.Condition)
			b.WriteString(`<div class="statement-container my-4" data-origin="` + html.EscapeString(participants[0]) + `">`)
			b.WriteString(`<div data-origin="` + html.EscapeString(participants[0]) + `" data-left-participant="` + html.EscapeString(participants[0]) + `" class="group fragment fragment-alt alt border-skin-fragment rounded text-left text-sm text-skin-message" style="transform: translateX(-61px); width: ` + formatFloat(seqWidth+10) + `px; min-width: 100px;">`)
			b.WriteString(`<div class="segment"><div class="header bg-skin-fragment-header text-skin-fragment-header leading-4 rounded-t relative"><div class="name font-semibold p-1 border-b"><label class="p-0 flex items-center gap-0.5"><span class="flex items-center justify-center w-5 h-4"></span><div class="collapsible-header flex w-full justify-between"><label class="mb-0">Alt</label></div></label></div></div></div>`)
			if condition != "" {
				b.WriteString(`<div class="segment"><div class="text-skin-fragment flex"><label>[</label><label title="Click to edit" class="bg-skin-frame opacity-65 condition editable-label-base cursor-pointer">` + html.EscapeString(condition) + `</label><label>]</label></div>`)
			} else {
				b.WriteString(`<div class="segment"><div class="text-skin-fragment flex"><label>[</label><label title="Click to edit" class="bg-skin-frame opacity-65 condition editable-label-base cursor-pointer"></label><label>]</label></div>`)
			}
			b.WriteString(`<div class="block" data-origin="` + html.EscapeString(participants[0]) + `" style="padding-left: 60px;">`)
		}

		if altElseAt[i] {
			b.WriteString(`</div></div><div class="segment mt-2 border-t border-solid"><div class="text-skin-fragment"><label class="p-1">[else]</label></div><div class="block" data-origin="` + html.EscapeString(participants[0]) + `" style="padding-left: 60px;">`)
		}

		fromEsc := html.EscapeString(msg.From)
		toEsc := html.EscapeString(msg.To)
		labelEsc := html.EscapeString(msg.Label)
		idxEsc := html.EscapeString(strings.TrimSpace(msg.Index))
		fromPos := indexByParticipant[msg.From]
		toPos := indexByParticipant[msg.To]
		dirClass := "left-to-right"
		flexClass := ""
		arrowPath := `<path d="M1 1L4.14331 4.29299C4.14704 4.2969 4.14699 4.30306 4.1432 4.30691L1 7.5" stroke="currentColor" stroke-linecap="round" fill="none"/>`
		if toPos < fromPos {
			dirClass = "right-to-left"
			flexClass = " flex-row-reverse right-to-left"
			arrowPath = `<path d="M4.14844 1L1.00441 4.54711C1.00101 4.55094 1.00106 4.55671 1.00451 4.56049L4.14844 8" stroke="currentColor" stroke-linecap="round" fill="none"/>`
		}
		borderStyle := "solid"
		if msg.IsReturn {
			borderStyle = "dashed"
		}
		b.WriteString(`<div class="statement-container my-4" data-origin="` + fromEsc + `">`)
		b.WriteString(`<div data-origin="null" data-to="` + toEsc + `" data-source="` + fromEsc + `" data-target="` + toEsc + `" class="interaction async ` + dirClass + ` text-left text-sm text-skin-message" data-signature="` + labelEsc + `">`)
		b.WriteString(`<div class="message leading-none border-skin-message-arrow border-b-2 flex items-end` + flexClass + `" style="border-bottom-style: ` + borderStyle + `;">`)
		b.WriteString(`<div class="name group text-center flex-grow relative"><div class="inline-block static min-h-[1em]"><div> ` + labelEsc + `</div></div></div>`)
		b.WriteString(`<div class="point text-skin-message-arrow open flex-shrink-0 transform translate-y-1/2 -my-px"><svg xmlns="http://www.w3.org/2000/svg" class="arrow stroke-2" height="10" width="10" viewBox="0 0 5 9">` + arrowPath + `</svg></div>`)
		if idxEsc != "" {
			b.WriteString(`<div class="absolute text-xs right-[100%] top-0 pr-1 group-hover:hidden text-gray-500 font-thin">` + idxEsc + `</div>`)
		}
		b.WriteString(`</div></div></div>`)

		if openAlt && altEndAt[i] {
			b.WriteString(`</div></div></div></div>`)
			openAlt = false
		}
	}
	if openAlt {
		b.WriteString(`</div></div></div></div>`)
	}

	b.WriteString(`</div></div></div></div></div></div></div></div></div>`)
	b.WriteString(`</foreignObject>`)
	return b.String()
}

func writeERMarkerDefs(b *strings.Builder) {
	b.WriteString(`<marker id="my-svg_er-onlyOneStart" class="marker onlyOne er" refX="0" refY="9" markerWidth="18" markerHeight="18" orient="auto">`)
	b.WriteString(`<path d="M9,0 L9,18 M15,0 L15,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-onlyOneEnd" class="marker onlyOne er" refX="18" refY="9" markerWidth="18" markerHeight="18" orient="auto">`)
	b.WriteString(`<path d="M3,0 L3,18 M9,0 L9,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-zeroOrOneStart" class="marker zeroOrOne er" refX="0" refY="9" markerWidth="30" markerHeight="18" orient="auto">`)
	b.WriteString(`<circle fill="white" cx="21" cy="9" r="6"/><path d="M9,0 L9,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-zeroOrOneEnd" class="marker zeroOrOne er" refX="30" refY="9" markerWidth="30" markerHeight="18" orient="auto">`)
	b.WriteString(`<circle fill="white" cx="9" cy="9" r="6"/><path d="M21,0 L21,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-oneOrMoreStart" class="marker oneOrMore er" refX="18" refY="18" markerWidth="45" markerHeight="36" orient="auto">`)
	b.WriteString(`<path d="M0,18 Q 18,0 36,18 Q 18,36 0,18 M42,9 L42,27"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-oneOrMoreEnd" class="marker oneOrMore er" refX="27" refY="18" markerWidth="45" markerHeight="36" orient="auto">`)
	b.WriteString(`<path d="M3,9 L3,27 M9,18 Q27,0 45,18 Q27,36 9,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-zeroOrMoreStart" class="marker zeroOrMore er" refX="18" refY="18" markerWidth="57" markerHeight="36" orient="auto">`)
	b.WriteString(`<circle fill="white" cx="48" cy="18" r="6"/><path d="M0,18 Q18,0 36,18 Q18,36 0,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_er-zeroOrMoreEnd" class="marker zeroOrMore er" refX="39" refY="18" markerWidth="57" markerHeight="36" orient="auto">`)
	b.WriteString(`<circle fill="white" cx="9" cy="18" r="6"/><path d="M21,18 Q39,0 57,18 Q39,36 21,18"/>`)
	b.WriteString(`</marker>`)
	b.WriteString("\n")
}

func writeClassMarkerDefs(b *strings.Builder) {
	b.WriteString(`<marker id="my-svg_class-aggregationStart" class="marker aggregation class" refX="18" refY="7" markerWidth="190" markerHeight="240" orient="auto"><path d="M 18,7 L9,13 L1,7 L9,1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-aggregationEnd" class="marker aggregation class" refX="1" refY="7" markerWidth="20" markerHeight="28" orient="auto"><path d="M 18,7 L9,13 L1,7 L9,1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-extensionStart" class="marker extension class" refX="18" refY="7" markerWidth="190" markerHeight="240" orient="auto"><path d="M 1,7 L18,13 V 1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-extensionEnd" class="marker extension class" refX="1" refY="7" markerWidth="20" markerHeight="28" orient="auto"><path d="M 1,1 V 13 L18,7 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-compositionStart" class="marker composition class" refX="18" refY="7" markerWidth="190" markerHeight="240" orient="auto"><path d="M 18,7 L9,13 L1,7 L9,1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-compositionEnd" class="marker composition class" refX="1" refY="7" markerWidth="20" markerHeight="28" orient="auto"><path d="M 18,7 L9,13 L1,7 L9,1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-dependencyStart" class="marker dependency class" refX="6" refY="7" markerWidth="190" markerHeight="240" orient="auto"><path d="M 5,7 L9,13 L1,7 L9,1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-dependencyEnd" class="marker dependency class" refX="13" refY="7" markerWidth="20" markerHeight="28" orient="auto"><path d="M 18,7 L9,13 L14,7 L9,1 Z"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-lollipopStart" class="marker lollipop class" refX="13" refY="7" markerWidth="190" markerHeight="240" orient="auto"><circle stroke="black" fill="transparent" cx="7" cy="7" r="6"/></marker>`)
	b.WriteString("\n")
	b.WriteString(`<marker id="my-svg_class-lollipopEnd" class="marker lollipop class" refX="1" refY="7" markerWidth="190" markerHeight="240" orient="auto"><circle stroke="black" fill="transparent" cx="7" cy="7" r="6"/></marker>`)
	b.WriteString("\n")
}

func defaultColor(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultFloat(value, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}

func mermaidStyle(
	fill string,
	stroke string,
	strokeWidth float64,
	dash string,
	lineCap string,
	lineJoin string,
	fillOpacity float64,
	strokeOpacity float64,
	opacity float64,
) string {
	parts := make([]string, 0, 10)
	parts = append(parts, "fill: "+defaultColor(fill, "none"))
	parts = append(parts, "stroke: "+defaultColor(stroke, "none"))
	parts = append(parts, "stroke-width: "+formatFloat(defaultFloat(strokeWidth, 1))+"px")
	if strings.TrimSpace(dash) != "" {
		parts = append(parts, "stroke-dasharray: "+dash)
	}
	if strings.TrimSpace(lineCap) != "" {
		parts = append(parts, "stroke-linecap: "+lineCap)
	}
	if strings.TrimSpace(lineJoin) != "" {
		parts = append(parts, "stroke-linejoin: "+lineJoin)
	}
	if fillOpacity > 0 && fillOpacity < 1 {
		parts = append(parts, "fill-opacity: "+formatFloat(fillOpacity))
	}
	if strokeOpacity > 0 && strokeOpacity < 1 {
		parts = append(parts, "stroke-opacity: "+formatFloat(strokeOpacity))
	}
	if opacity > 0 && opacity < 1 {
		parts = append(parts, "opacity: "+formatFloat(opacity))
	}
	return strings.Join(parts, "; ") + ";"
}

func renderSankeyMermaid(layout Layout) string {
	var b strings.Builder
	b.WriteString("<g/>\n")

	if len(layout.SankeyNodes) == 0 {
		return b.String()
	}

	b.WriteString(`<g class="nodes">`)
	for i, node := range layout.SankeyNodes {
		b.WriteString(`<g class="node" id="node-` + intString(i+1) + `" transform="translate(` + formatFloat(node.X0) + `,` + formatFloat(node.Y0) + `)" x="` + formatFloat(node.X0) + `" y="` + formatFloat(node.Y0) + `">`)
		b.WriteString(`<rect height="` + formatFloat(node.Y1-node.Y0) + `" width="` + formatFloat(node.X1-node.X0) + `" fill="` + html.EscapeString(node.Color) + `"/>`)
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g>`)

	b.WriteString(`<g class="node-labels" font-size="14">`)
	width := max(1.0, layout.ViewBoxWidth)
	for _, node := range layout.SankeyNodes {
		x := node.X1 + 6
		anchor := "start"
		if node.X0 >= width*0.5 {
			x = node.X0 - 6
			anchor = "end"
		}
		y := (node.Y0 + node.Y1) * 0.5
		b.WriteString(`<text x="` + formatFloat(x) + `" y="` + formatFloat(y) + `" dy="0em" text-anchor="` + anchor + `">`)
		b.WriteString(html.EscapeString(node.ID))
		b.WriteString("\n")
		b.WriteString(html.EscapeString(formatSankeyValue(node.Value)))
		b.WriteString(`</text>`)
	}
	b.WriteString(`</g>`)

	b.WriteString(`<g class="links" fill="none" stroke-opacity="0.5">`)
	for i, link := range layout.SankeyLinks {
		gradientID := "linearGradient-" + intString(len(layout.SankeyNodes)+i+1)
		b.WriteString(`<g class="link" style="mix-blend-mode: multiply;">`)
		b.WriteString(`<linearGradient id="` + gradientID + `" gradientUnits="userSpaceOnUse" x1="` + formatFloat(link.X0) + `" x2="` + formatFloat(link.X1) + `">`)
		b.WriteString(`<stop offset="0%" stop-color="` + html.EscapeString(link.SourceColor) + `"/>`)
		b.WriteString(`<stop offset="100%" stop-color="` + html.EscapeString(link.TargetColor) + `"/>`)
		b.WriteString(`</linearGradient>`)
		b.WriteString(`<path d="` + link.Path + `" stroke="url(#` + gradientID + `)" stroke-width="` + formatFloat(max(1, link.Width)) + `"/>`)
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g>`)

	return b.String()
}

func formatSankeyValue(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-9 {
		return strconv.FormatInt(int64(math.Round(value)), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func renderRadarMermaid(layout Layout) string {
	var b strings.Builder
	b.WriteString("<g/>\n")
	b.WriteString(`<g transform="translate(` + formatFloat(layout.Width*0.5) + `, ` + formatFloat(layout.Height*0.5) + `)">`)

	if layout.RadarGraticule == "polygon" && len(layout.RadarAxes) > 0 {
		for _, r := range layout.RadarGraticuleRadii {
			points := make([]string, 0, len(layout.RadarAxes))
			for i := range layout.RadarAxes {
				angle := 2*math.Pi*float64(i)/float64(len(layout.RadarAxes)) - math.Pi*0.5
				points = append(points, formatFloat(r*math.Cos(angle))+","+formatFloat(r*math.Sin(angle)))
			}
			b.WriteString(`<polygon points="` + strings.Join(points, " ") + `" class="radarGraticule"/>`)
		}
	} else {
		for _, r := range layout.RadarGraticuleRadii {
			b.WriteString(`<circle r="` + formatFloat(r) + `" class="radarGraticule"/>`)
		}
	}

	for _, axis := range layout.RadarAxes {
		b.WriteString(`<line x1="0" y1="0" x2="` + formatFloat(axis.LineX) + `" y2="` + formatFloat(axis.LineY) + `" class="radarAxisLine"/>`)
		b.WriteString(`<text x="` + formatFloat(axis.TextX) + `" y="` + formatFloat(axis.TextY) + `" class="radarAxisLabel">` + html.EscapeString(axis.Label) + `</text>`)
	}

	for _, curve := range layout.RadarCurves {
		if curve.Polygon {
			b.WriteString(`<polygon points="` + curve.Path + `" class="` + curve.Class + `"/>`)
		} else {
			b.WriteString(`<path d="` + curve.Path + `" class="` + curve.Class + `"/>`)
		}
	}

	if layout.RadarShowLegend {
		for i, label := range layout.RadarLegend {
			y := layout.RadarLegendY + float64(i)*layout.RadarLegendLineHeight
			b.WriteString(`<g transform="translate(` + formatFloat(layout.RadarLegendX) + `, ` + formatFloat(y) + `)">`)
			b.WriteString(`<rect width="12" height="12" class="radarLegendBox-` + intString(i) + `"/>`)
			b.WriteString(`<text x="16" y="0" class="radarLegendText">` + html.EscapeString(label) + `</text>`)
			b.WriteString(`</g>`)
		}
	}

	b.WriteString(`<text class="radarTitle" x="0" y="-350">` + html.EscapeString(layout.RadarTitle) + `</text>`)
	b.WriteString(`</g>`)
	return b.String()
}

func radarStyleCSS() string {
	var b strings.Builder
	b.WriteString(`#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}@keyframes edge-animation-frame{from{stroke-dashoffset:0;}}@keyframes dash{to{stroke-dashoffset:0;}}#my-svg .edge-animation-slow{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 50s linear infinite;stroke-linecap:round;}#my-svg .edge-animation-fast{stroke-dasharray:9,5!important;stroke-dashoffset:900;animation:dash 20s linear infinite;stroke-linecap:round;}#my-svg .error-icon{fill:#552222;}#my-svg .error-text{fill:#552222;stroke:#552222;}#my-svg .edge-thickness-normal{stroke-width:1px;}#my-svg .edge-thickness-thick{stroke-width:3.5px;}#my-svg .edge-pattern-solid{stroke-dasharray:0;}#my-svg .edge-thickness-invisible{stroke-width:0;fill:none;}#my-svg .edge-pattern-dashed{stroke-dasharray:3;}#my-svg .edge-pattern-dotted{stroke-dasharray:2;}#my-svg .marker{fill:#333333;stroke:#333333;}#my-svg .marker.cross{stroke:#333333;}#my-svg svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}#my-svg p{margin:0;}#my-svg .radarTitle{font-size:16px;color:#333;dominant-baseline:hanging;text-anchor:middle;}#my-svg .radarAxisLine{stroke:#333333;stroke-width:2;}#my-svg .radarAxisLabel{dominant-baseline:middle;text-anchor:middle;font-size:12px;color:#333333;}#my-svg .radarGraticule{fill:#DEDEDE;fill-opacity:0.3;stroke:#DEDEDE;stroke-width:1;}#my-svg .radarLegendText{text-anchor:start;font-size:12px;dominant-baseline:hanging;}`)
	palette := []string{
		"hsl(240, 100%, 76.2745098039%)",
		"hsl(60, 100%, 73.5294117647%)",
		"hsl(80, 100%, 76.2745098039%)",
		"hsl(270, 100%, 76.2745098039%)",
		"hsl(300, 100%, 76.2745098039%)",
		"hsl(330, 100%, 76.2745098039%)",
		"hsl(0, 100%, 76.2745098039%)",
		"hsl(30, 100%, 76.2745098039%)",
		"hsl(90, 100%, 76.2745098039%)",
		"hsl(150, 100%, 76.2745098039%)",
		"hsl(180, 100%, 76.2745098039%)",
		"hsl(210, 100%, 76.2745098039%)",
	}
	for i, color := range palette {
		b.WriteString(`#my-svg .radarCurve-` + intString(i) + `{color:` + color + `;fill:` + color + `;fill-opacity:0.5;stroke:` + color + `;stroke-width:2;}`)
		b.WriteString(`#my-svg .radarLegendBox-` + intString(i) + `{fill:` + color + `;fill-opacity:0.5;stroke:` + color + `;}`)
	}
	b.WriteString(`#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}`)
	return b.String()
}

func useMermaidLikeDOM(kind DiagramKind) bool {
	switch kind {
	case DiagramClass, DiagramER, DiagramState, DiagramFlowchart, DiagramMindmap, DiagramRequirement:
		return true
	default:
		return false
	}
}

func useMermaidLikeRoot(_ DiagramKind) bool {
	return true
}

func useMermaidGroupWrappers(kind DiagramKind) bool {
	switch kind {
	case DiagramTimeline:
		return true
	default:
		return false
	}
}

func useTspanText(kind DiagramKind) bool {
	switch kind {
	case DiagramTimeline, DiagramArchitecture, DiagramSequence:
		return true
	default:
		return false
	}
}

func useArrowMarkers(kind DiagramKind) bool {
	switch kind {
	case DiagramArchitecture, DiagramKanban, DiagramPacket, DiagramRadar, DiagramZenUML, DiagramMindmap,
		DiagramGantt, DiagramTreemap:
		return false
	default:
		return true
	}
}

func diagramDOMClass(kind DiagramKind) (svgClass string, ariaRole string) {
	switch kind {
	case DiagramArchitecture:
		return "", "architecture"
	case DiagramBlock:
		return "", "block"
	case DiagramC4:
		return "", "c4"
	case DiagramClass:
		return "classDiagram", "class"
	case DiagramER:
		return "erDiagram", "er"
	case DiagramGantt:
		return "", "gantt"
	case DiagramGitGraph:
		return "", "gitGraph"
	case DiagramState:
		return "statediagram", "stateDiagram"
	case DiagramFlowchart:
		return "flowchart", "flowchart-v2"
	case DiagramMindmap:
		return "mindmapDiagram", "mindmap"
	case DiagramJourney:
		return "", "journey"
	case DiagramKanban:
		return "", "kanban"
	case DiagramPacket:
		return "", "packet"
	case DiagramPie:
		return "", "pie"
	case DiagramQuadrant:
		return "", "quadrantChart"
	case DiagramRadar:
		return "", "radar"
	case DiagramRequirement:
		return "requirementDiagram", "requirement"
	case DiagramSankey:
		return "", "sankey"
	case DiagramSequence:
		return "", "sequence"
	case DiagramTimeline:
		return "", "timeline"
	case DiagramTreemap:
		return "flowchart", "treemap"
	case DiagramXYChart:
		return "", "xychart"
	case DiagramZenUML:
		return "", "zenuml"
	default:
		return "", ""
	}
}

func rectToPath(rect LayoutRect) string {
	x := rect.X
	y := rect.Y
	w := rect.W
	h := rect.H
	rx := rect.RX
	ry := rect.RY
	if rx <= 0 && ry <= 0 {
		return "M" + formatFloat(x) + "," + formatFloat(y) +
			" H" + formatFloat(x+w) +
			" V" + formatFloat(y+h) +
			" H" + formatFloat(x) +
			" Z"
	}
	if rx <= 0 {
		rx = ry
	}
	if ry <= 0 {
		ry = rx
	}
	rx = min(rx, w/2)
	ry = min(ry, h/2)
	return "M" + formatFloat(x+rx) + "," + formatFloat(y) +
		" H" + formatFloat(x+w-rx) +
		" A" + formatFloat(rx) + "," + formatFloat(ry) + " 0 0 1 " + formatFloat(x+w) + "," + formatFloat(y+ry) +
		" V" + formatFloat(y+h-ry) +
		" A" + formatFloat(rx) + "," + formatFloat(ry) + " 0 0 1 " + formatFloat(x+w-rx) + "," + formatFloat(y+h) +
		" H" + formatFloat(x+rx) +
		" A" + formatFloat(rx) + "," + formatFloat(ry) + " 0 0 1 " + formatFloat(x) + "," + formatFloat(y+h-ry) +
		" V" + formatFloat(y+ry) +
		" A" + formatFloat(rx) + "," + formatFloat(ry) + " 0 0 1 " + formatFloat(x+rx) + "," + formatFloat(y) +
		" Z"
}
