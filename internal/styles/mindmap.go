// Ported from mermaid 12.2.0, mermaid/src/diagrams/mindmap/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func mindmapGenSections(o *options) string {
	theme, look := o.Get("theme"), o.Get("look")

	var sections text

	limit := loopCount(o.Get("THEME_COLOR_LIMIT"))
	for i := 0; i < limit; i++ {
		key := "lineColor" + itoa(i)
		o.Set(key, or(o.Get(key), o.Get("cScaleInv"+itoa(i))))
		if isDark(o.Get(key)) {
			o.Set(key, lighten(o.Get(key), 20))
		} else {
			o.Set(key, darken(o.Get(key), 20))
		}
	}

	for i := 0; i < limit; i++ {
		sw := "" + itoa(17-3*i)
		if is(look, "neo") {
			sw = "" + itoa(max(10-(i-1)*2, 2))
		}
		// HTML labels must follow the neo fill swap too, so they use the same color as SVG text.
		var neoLabel any
		if is(theme, "redux") || is(theme, "redux-dark") {
			neoLabel = o.Get("nodeBorder")
		} else if is(theme, "neutral") {
			neoLabel = o.Get("cScaleLabel" + itoa(1))
		} else {
			neoLabel = o.Get("cScaleLabel" + itoa(i))
		}
		sections.add(`
    .section-` + itoa(i-1) + ` rect, .section-` + itoa(i-1) + ` path, .section-` + itoa(i-1) + ` circle, .section-` + itoa(i-1) + ` polygon, .section-` + itoa(i-1) + ` path  {
      fill: ` + o.s("cScale"+itoa(i)) + `;
    }
    .section-` + itoa(i-1) + ` text {
     fill: ` + o.s("cScaleLabel"+itoa(i)) + `;
    }
     .section-` + itoa(i-1) + ` span {
     color: ` + o.s("cScaleLabel"+itoa(i)) + `;
    }
    .node-icon-` + itoa(i-1) + ` {
      font-size: 40px;
      color: ` + o.s("cScaleLabel"+itoa(i)) + `;
    }
    .section-edge-` + itoa(i-1) + `{
      stroke: ` + o.s("cScale"+itoa(i)) + `;
    }
    .edge-depth-` + itoa(i-1) + `{
      stroke-width: ` + sw + `;
    }
    .section-` + itoa(i-1) + ` line {
      stroke: ` + o.s("cScaleInv"+itoa(i)) + ` ;
      stroke-width: 3;
    }

    .disabled, .disabled circle, .disabled text {
      fill: lightgray;
    }
    .disabled text {
      fill: #efefef;
    }
    [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` rect, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` path, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` circle, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` polygon {
      fill: ` + s(cond(is(theme, "redux") || is(theme, "redux-dark") || is(theme, "neutral"), o.Get("mainBkg"), o.Get("cScale"+itoa(i)))) + `;
      stroke: ` + s(cond(is(theme, "redux") || is(theme, "redux-dark"), o.Get("nodeBorder"), o.Get("cScale"+itoa(i)))) + `;
      stroke-width: ` + s(nn(o.Get("strokeWidth"), float64(2))) + `px;
    }
    [data-look="neo"].section-edge-` + itoa(i-1) + `{
      stroke: ` + s(cond(includes(theme, "redux") || is(theme, "neo-dark"), o.Get("nodeBorder"), o.Get("cScale"+itoa(i)))) + `;
    }
    [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` text, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` span {
     fill: ` + s(neoLabel) + `;
     color: ` + s(neoLabel) + `;
    }
    `)
	}
	return sections.String()
}

func mindmapGenGradient(themeColorLimit any, svgId string, mainBkg any) string {
	var sections text
	for i, n := 0, loopCount(themeColorLimit); i < n; i++ {
		sections.add(`
    [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` rect, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` path, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` circle, [data-look="neo"].mindmap-node.section-` + itoa(i-1) + ` polygon {
      stroke: url(` + svgId + `-gradient);
      fill: ` + s(mainBkg) + `;
    }
    .section-` + itoa(i-1) + ` line {
      stroke-width: 0;
    }`)
	}
	return sections.String()
}

// mindmapStyles is getStyles of the mindmap diagram: the CSS of the diagram type.
func mindmapStyles(o *options) string {
	theme := o.Get("theme")
	// svgId is passed inside options by the caller in styles.go.
	svgId := o.Get("svgId")
	var neoRootLabel any
	if includes(theme, "redux") {
		neoRootLabel = o.Get("nodeBorder")
	} else if is(theme, "neutral") {
		neoRootLabel = o.Get("cScaleLabel" + itoa(1))
	} else {
		neoRootLabel = o.Get("cScaleLabel" + itoa(0))
	}
	scopedDropShadow := "none"
	if dropShadow := o.Get("dropShadow"); js.Truthy(dropShadow) {
		scopedDropShadow = replace(dropShadow, "url(#drop-shadow)", "url("+s(svgId)+"-drop-shadow)")
	}
	// The template evaluates its expressions in order, and genSections writes
	// to options.
	sections := mindmapGenSections(o)
	isRedux := includes(theme, "redux")
	gradient := ""
	if js.Truthy(o.Get("useGradient")) && js.Truthy(svgId) && js.Truthy(o.Get("mainBkg")) {
		gradient = mindmapGenGradient(o.Get("THEME_COLOR_LIMIT"), s(svgId), o.Get("mainBkg"))
	}
	return `
  .edge {
    stroke-width: 3;
  }
  ` + sections + `
  .section-root rect, .section-root path, .section-root circle, .section-root polygon  {
    fill: ` + o.s("git0") + `;
  }
  .section-root text {
    fill: ` + o.s("gitBranchLabel0") + `;
  }
  .section-root span {
    color: ` + s(cond(isRedux, o.Get("nodeBorder"), o.Get("gitBranchLabel0"))) + `;
  }
  .icon-container {
    height:100%;
    display: flex;
    justify-content: center;
    align-items: center;
  }
  .edge {
    fill: none;
  }
  .mindmap-node-label {
    dy: 1em;
    alignment-baseline: middle;
    text-anchor: middle;
    dominant-baseline: middle;
    text-align: center;
  }
  [data-look="neo"].mindmap-node  {
    filter: ` + scopedDropShadow + `;
  }
  [data-look="neo"].mindmap-node.section-root rect, [data-look="neo"].mindmap-node.section-root path, [data-look="neo"].mindmap-node.section-root circle, [data-look="neo"].mindmap-node.section-root polygon  {
    fill: ` + s(cond(isRedux, o.Get("mainBkg"), o.Get("git0"))) + `;
  }
  [data-look="neo"].mindmap-node.section-root .text-inner-tspan, [data-look="neo"].mindmap-node.section-root span {
    fill: ` + s(neoRootLabel) + `;
    color: ` + s(neoRootLabel) + `;
  }
  ` + gradient + `
`
}
