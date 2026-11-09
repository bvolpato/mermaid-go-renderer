// Ported from mermaid 12.2.0, mermaid/src/diagrams/git/styles.js (MIT).

package styles

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const gitNamedColorCount = 8

// The theme sets of git/styles.js. A set of JavaScript has only the strings
// that it lists, so a theme that is not a string is in no set.
func gitReduxGeometryTheme(theme any) bool {
	return is(theme, "redux") || is(theme, "redux-dark") || is(theme, "redux-color") || is(theme, "redux-dark-color")
}

func gitNeoTheme(theme any) bool {
	return is(theme, "neo") || is(theme, "neo-dark")
}

func gitDarkTheme(theme any) bool {
	return is(theme, "dark") || is(theme, "redux-dark") || is(theme, "redux-dark-color") || is(theme, "neo-dark")
}

func gitNeoColorGenTheme(theme any) bool {
	return gitReduxGeometryTheme(theme) || gitNeoTheme(theme)
}

func gitGenGraphGradient(o *options) string {
	svgId := o.Get("svgId")
	var sections text
	if js.Truthy(o.Get("useGradient")) && js.Truthy(svgId) {
		for i, n := 0, loopCount(o.Get("THEME_COLOR_LIMIT")); i < n; i++ {
			sections.add(`
      .label` + itoa(i) + `  { fill: ` + o.s("mainBkg") + `; stroke: url(` + s(svgId) + `-gradient); stroke-width: ` + o.s("strokeWidth") + `;}
             `)
		}
	}
	return sections.String()
}

func gitGenColor(o *options) string {
	config := o.config
	theme, themeVariables := config.Get("theme"), config.Get("themeVariables")
	borderColorArray := prop(themeVariables, "borderColorArray")
	useReduxGeometry := gitReduxGeometryTheme(theme)
	if gitNeoTheme(theme) {
		var sections text

		for i, n := 0, loopCount(o.Get("THEME_COLOR_LIMIT")); i < n; i++ {
			if i == 0 {
				sections.add(`
        .branch-label` + itoa(i) + ` { fill: ` + o.s("nodeBorder") + `;}
        .commit` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `;   }
        .commit-highlight` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; fill: ` + o.s("nodeBorder") + `; }
        .arrow` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; }
        .commit-bullets { fill: ` + o.s("nodeBorder") + `; }
        .commit-cherry-pick` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; }
        ` + gitGenGraphGradient(o))
			} else {
				// Wrap index to stay within the range of defined git color variables (git0..git7).
				ci := i % gitNamedColorCount
				sections.add(`
        .branch-label` + itoa(i) + ` { fill: ` + o.s("gitBranchLabel"+itoa(ci)) + `; }
        .commit` + itoa(i) + ` { stroke: ` + o.s("git"+itoa(ci)) + `; fill: ` + o.s("git"+itoa(ci)) + `; }
        .commit-highlight` + itoa(i) + ` { stroke: ` + o.s("gitInv"+itoa(ci)) + `; fill: ` + o.s("gitInv"+itoa(ci)) + `; }
        .arrow` + itoa(i) + ` { stroke: ` + o.s("git"+itoa(ci)) + `; }
        `)
			}
		}
		return sections.String()
	} else if !inColorThemes(theme) {
		var sections text
		fontWeight := ""
		if useReduxGeometry {
			fontWeight = `font-weight:` + o.s("noteFontWeight")
		}

		for i, n := 0, loopCount(o.Get("THEME_COLOR_LIMIT")); i < n; i++ {
			sections.add(`
        .branch-label` + itoa(i) + ` { fill: ` + o.s("nodeBorder") + `; ` + fontWeight + ` }
        .commit` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `;   }
        .commit-highlight` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; fill: ` + o.s("nodeBorder") + `; }
        .label` + itoa(i) + `  { fill: ` + o.s("mainBkg") + `; stroke: ` + o.s("nodeBorder") + `; stroke-width: ` + o.s("strokeWidth") + `; ` + fontWeight + `}
        .arrow` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; }
        .commit-bullets { fill: ` + o.s("nodeBorder") + `; }
        .commit-cherry-pick` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; }
        `)
		}
		return sections.String()
	} else {
		var sections text
		fontWeight := ""
		if useReduxGeometry {
			fontWeight = `font-weight:` + o.s("noteFontWeight")
		}

		for i, n := 0, loopCount(o.Get("THEME_COLOR_LIMIT")); i < n; i++ {
			if i == 0 {
				sections.add(`
        .branch-label` + itoa(i) + ` { fill: ` + o.s("nodeBorder") + `; ` + fontWeight + ` }
        .commit` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; }
        .commit-highlight` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; fill: ` + o.s("mainBkg") + `; }
        .label` + itoa(i) + `  { fill: ` + o.s("mainBkg") + `; stroke: ` + o.s("nodeBorder") + `; stroke-width: ` + o.s("strokeWidth") + `; ` + fontWeight + ` }
        .arrow` + itoa(i) + ` { stroke: ` + o.s("nodeBorder") + `; }
        .commit-bullets { fill: ` + o.s("nodeBorder") + `; }
        `)
			} else {
				color := s(paletteAt(borderColorArray, i))
				sections.add(`
        .branch-label` + itoa(i) + ` { fill: ` + o.s("nodeBorder") + `; ` + fontWeight + ` }
        .commit` + itoa(i) + ` { stroke: ` + color + `; fill: ` + color + `; }
        .commit-highlight` + itoa(i) + ` { stroke: ` + color + `; fill: ` + color + `; }
        .label` + itoa(i) + `  { fill: ` + s(cond(gitDarkTheme(theme), o.Get("mainBkg"), color)) + `; stroke: ` + color + `;  stroke-width: ` + o.s("strokeWidth") + `; }
        .arrow` + itoa(i) + ` { stroke: ` + color + `; }
        `)
			}
		}
		return sections.String()
	}
}

func gitNormalTheme(o *options) string {
	// Array.from({ length: n }) takes the integer part of n.
	length := js.ToNumber(o.Get("THEME_COLOR_LIMIT"))
	if math.IsNaN(length) || length < 1 {
		return ""
	}
	if length > maxLoop {
		throwf("styles: the loop limit %s is too large", o.s("THEME_COLOR_LIMIT"))
	}
	parts := make([]string, int(length))
	for i := range parts {
		// Wrap index to stay within the range of defined git color variables (git0..git7).
		ci := i % gitNamedColorCount
		parts[i] = `
        .branch-label` + itoa(i) + ` { fill: ` + o.s("gitBranchLabel"+itoa(ci)) + `; }
        .commit` + itoa(i) + ` { stroke: ` + o.s("git"+itoa(ci)) + `; fill: ` + o.s("git"+itoa(ci)) + `; }
        .commit-highlight` + itoa(i) + ` { stroke: ` + o.s("gitInv"+itoa(ci)) + `; fill: ` + o.s("gitInv"+itoa(ci)) + `; }
        .label` + itoa(i) + `  { fill: ` + o.s("git"+itoa(ci)) + `; }
        .arrow` + itoa(i) + ` { stroke: ` + o.s("git"+itoa(ci)) + `; }
        `
	}
	return strings.Join(parts, "\n")
}

// gitStyles is getStyles of the git graph diagram: the CSS of the diagram type.
func gitStyles(o *options) string {
	config := o.config
	theme := config.Get("theme")
	useNeoColorGen := gitNeoColorGenTheme(theme)

	colors := ""
	if useNeoColorGen {
		colors = gitGenColor(o)
	} else {
		colors = gitNormalTheme(o)
	}
	labelWeight, tagFilter := "", ""
	if useNeoColorGen {
		labelWeight = `font-weight:` + o.s("noteFontWeight") + `;`
		tagFilter = `filter:` + o.s("dropShadow")
	}
	return `
  .commit-id,
  .commit-msg,
  .branch-label {
    fill: lightgrey;
    color: lightgrey;
    font-family: 'trebuchet ms', verdana, arial, sans-serif;
    font-family: var(--mermaid-font-family);
  }
  
  ` + colors + `

  .branch {
    stroke-width: ` + o.s("strokeWidth") + `;
    stroke: ` + s(nn(o.Get("commitLineColor"), o.Get("lineColor"))) + `;
    stroke-dasharray:  ` + s(cond(useNeoColorGen, "4 2", "2")) + `;
  }
  .commit-label { font-size: ` + o.s("commitLabelFontSize") + `; fill: ` + s(cond(useNeoColorGen, o.Get("nodeBorder"), o.Get("commitLabelColor"))) + `; ` + labelWeight + `}
  .commit-label-bkg { font-size: ` + o.s("commitLabelFontSize") + `; fill: ` + s(cond(useNeoColorGen, "transparent", o.Get("commitLabelBackground"))) + `; opacity: ` + s(cond(useNeoColorGen, "", float64(0.5))) + `;  }
  .tag-label { font-size: ` + o.s("tagLabelFontSize") + `; fill: ` + o.s("tagLabelColor") + `;}
  .tag-label-bkg { fill: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("tagLabelBackground"))) + `; stroke: ` + s(cond(useNeoColorGen, o.Get("nodeBorder"), o.Get("tagLabelBorder"))) + `; ` + tagFilter + `  }
  .tag-hole { fill: ` + o.s("textColor") + `; }

  .commit-merge {
    stroke: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("primaryColor"))) + `;
    fill: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("primaryColor"))) + `;
  }
  .commit-reverse {
    stroke: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("primaryColor"))) + `;
    fill: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("primaryColor"))) + `;
    stroke-width: ` + s(cond(useNeoColorGen, o.Get("strokeWidth"), float64(3))) + `;
  }
  .commit-highlight-outer {
  }
  .commit-highlight-inner {
    stroke: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("primaryColor"))) + `;
    fill: ` + s(cond(useNeoColorGen, o.Get("mainBkg"), o.Get("primaryColor"))) + `;
  }

  .arrow {
    /* Intentional: neo themes keep the bold 8px arrow (like classic themes); only redux-geometry themes use the thinner options.strokeWidth. */
    stroke-width: ` + s(cond(gitReduxGeometryTheme(theme), o.Get("strokeWidth"), float64(8))) + `;
    stroke-linecap: round;
    fill: none
  }
  .gitTitleText {
    text-anchor: middle;
    font-size: 18px;
    fill: ` + o.s("textColor") + `;
  }
`
}
