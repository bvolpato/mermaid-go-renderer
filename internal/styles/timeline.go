// Ported from mermaid 12.2.0, mermaid/src/diagrams/timeline/styles.js (MIT).

package styles

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// timelineRawSvgID is options.svgId?.replace(/^#/, ”) ?? ”. options.svgId
// is the CSS selector, for example "#mermaid-0".
func timelineRawSvgID(o *options) string {
	svgId, ok := o.Get("svgId").(string)
	if !ok {
		if js.IsNullish(o.Get("svgId")) {
			return ""
		}
		throwf("TypeError: options.svgId.replace is not a function")
	}
	return strings.TrimPrefix(svgId, "#")
}

func timelineGenReduxSections(o *options) string {
	// The theme is read from the config at render time.
	theme := o.config.Get("theme")
	isDarkTheme := includes(theme, "dark")
	// The shared gate checks that this is a colour theme and that a palette is
	// present.
	isColorTheme := isColorTheme(theme, o.Get("borderColorArray"))
	rawSvgId := timelineRawSvgID(o)
	scopedDropShadow := s(nn(o.Get("dropShadow"), "none"))
	if rawSvgId != "" {
		scopedDropShadow = `url(#` + rawSvgId + `-drop-shadow)`
	}

	var sections text

	for i := 0; i < colorSlotCount(o.Get("THEME_COLOR_LIMIT"), nil); i++ {
		sw := itoa(17 - 3*i)
		// The bound is the section count, not the palette length, so the palette
		// wraps across the sections.
		var slot any
		if isColorTheme {
			slot = paletteAt(o.Get("borderColorArray"), i)
		}
		color := nn(slot, o.Get("mainBkg"))
		stroke := nn(slot, o.Get("nodeBorder"))

		sections.add(`
    .section-` + itoa(i-1) + ` rect,
    .section-` + itoa(i-1) + ` path,
    .section-` + itoa(i-1) + ` circle {
      fill: ` + s(cond(isDarkTheme && isColorTheme, o.Get("mainBkg"), color)) + `;
      stroke: ` + s(stroke) + `;
      stroke-width: ` + o.s("strokeWidth") + `;
      filter: ` + scopedDropShadow + `;
    }

    .section-` + itoa(i-1) + ` text {
      fill: ` + o.s("nodeBorder") + `;
      font-weight: ` + o.s("fontWeight") + `
    }

    .node-icon-` + itoa(i-1) + ` {
      font-size: 40px;
      color: ` + o.s("cScaleLabel"+itoa(i)) + `;
    }

    .section-edge-` + itoa(i-1) + ` {
      stroke: ` + o.s("cScale"+itoa(i)) + `;
    }

    .edge-depth-` + itoa(i-1) + ` {
      stroke-width: ` + sw + `;
    }

    .section-` + itoa(i-1) + ` line {
      stroke: ` + o.s("cScaleInv"+itoa(i)) + `;
      stroke-width: 3;
    }

    .lineWrapper line {
      stroke: ` + o.s("nodeBorder") + `;
      stroke-width:` + o.s("strokeWidth") + `
    }

    .disabled,
    .disabled circle,
    .disabled text {
      fill: ` + s(nn(o.Get("tertiaryColor"), "lightgray")) + `;
    }

    .disabled text {
      fill: ` + s(nn(o.Get("clusterBorder"), "#efefef")) + `;
    }
    `)
	}

	return sections.String()
}

func timelineGenSections(o *options) string {
	var sections text

	// The loops are bounded in the same way as in timelineGenReduxSections.
	colorLimit := colorSlotCount(o.Get("THEME_COLOR_LIMIT"), nil)

	for i := 0; i < colorLimit; i++ {
		key := "lineColor" + itoa(i)
		o.Set(key, nn(o.Get(key), o.Get("cScaleInv"+itoa(i))))
		if isDark(o.Get(key)) {
			o.Set(key, lighten(o.Get(key), 20))
		} else {
			o.Set(key, darken(o.Get(key), 20))
		}
	}

	for i := 0; i < colorLimit; i++ {
		sw := "" + itoa(17-3*i)
		sections.add(`
    .section-` + itoa(i-1) + ` rect, .section-` + itoa(i-1) + ` path, .section-` + itoa(i-1) + ` circle, .section-` + itoa(i-1) + ` path  {
      fill: ` + o.s("cScale"+itoa(i)) + `;
    }
    .section-` + itoa(i-1) + ` text {
     fill: ` + o.s("cScaleLabel"+itoa(i)) + `;
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

    .lineWrapper line{
      stroke: ` + o.s("cScaleLabel"+itoa(i)) + ` ;
    }

    .disabled, .disabled circle, .disabled text {
      fill: ` + s(nn(o.Get("tertiaryColor"), "lightgray")) + `;
    }
    .disabled text {
      fill: ` + s(nn(o.Get("clusterBorder"), "#efefef")) + `;
    }
    `)
	}
	return sections.String()
}

// timelineStyles is getStyles of the timeline diagram: the CSS of the diagram type.
func timelineStyles(o *options) string {
	// The theme is read from the config at render time.
	theme := o.config.Get("theme")
	isReduxTheme := includes(theme, "redux")
	isNeutralTheme := is(theme, "neutral")
	// options.svgId is the CSS selector. The leading '#' is removed for url(#...).
	rawSvgId := timelineRawSvgID(o)
	var gradientSections text
	// The neutral theme keeps its grayscale colours, so it has no gradient.
	if js.Truthy(o.Get("useGradient")) && rawSvgId != "" && js.Truthy(o.Get("THEME_COLOR_LIMIT")) && !isNeutralTheme {
		gradientColorLimit := colorSlotCount(o.Get("THEME_COLOR_LIMIT"), nil)
		for i := 0; i < gradientColorLimit; i++ {
			gradientSections.add(`
      .section-` + itoa(i-1) + `[data-look="neo"] rect,
      .section-` + itoa(i-1) + `[data-look="neo"] path,
      .section-` + itoa(i-1) + `[data-look="neo"] circle {
        fill: ` + o.s("mainBkg") + `;
        stroke: url(#` + rawSvgId + `-gradient);
        stroke-width: 2;
      }
      .section-` + itoa(i-1) + `[data-look="neo"] line {
        stroke: url(#` + rawSvgId + `-gradient);
        stroke-width: 2;
      }`)
		}
	}

	sections := ""
	if isReduxTheme {
		sections = timelineGenReduxSections(o)
	} else {
		sections = timelineGenSections(o)
	}
	return `
  .edge {
    stroke-width: 3;
  }
  ` + sections + `
  ` + gradientSections.String() + `
  .section-root rect, .section-root path, .section-root circle  {
    fill: ` + o.s("git0") + `;
  }
  .section-root text {
    fill: ` + o.s("gitBranchLabel0") + `;
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
  .eventWrapper  {
   filter: brightness(120%);
  }
`
}
