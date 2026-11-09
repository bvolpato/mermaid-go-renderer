// Ported from mermaid 12.2.0, mermaid/src/diagrams/kanban/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func kanbanGenSections(o *options) string {
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

	adjuster := func(color any, level float64) string {
		if js.Truthy(o.Get("darkMode")) {
			return darken(color, level)
		}
		return lighten(color, level)
	}

	for i := 0; i < limit; i++ {
		sw := "" + itoa(17-3*i)
		sections.add(`
    .section-` + itoa(i-1) + ` rect, .section-` + itoa(i-1) + ` path, .section-` + itoa(i-1) + ` circle, .section-` + itoa(i-1) + ` polygon, .section-` + itoa(i-1) + ` path  {
      fill: ` + adjuster(o.Get("cScale"+itoa(i)), 10) + `;
      stroke: ` + adjuster(o.Get("cScale"+itoa(i)), 10) + `;

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

    .disabled, .disabled circle, .disabled text {
      fill: lightgray;
    }
    .disabled text {
      fill: #efefef;
    }

  .node rect,
  .node circle,
  .node ellipse,
  .node polygon,
  .node path {
    fill: ` + o.s("background") + `;
    stroke: ` + o.s("nodeBorder") + `;
    stroke-width: 1px;
  }

  .kanban-ticket-link {
    fill: ` + o.s("background") + `;
    stroke: ` + o.s("nodeBorder") + `;
    text-decoration: underline;
  }
    `)
	}
	return sections.String()
}

// kanbanStyles is getStyles of the kanban diagram: the CSS of the diagram type.
func kanbanStyles(o *options) string {
	return `
  .edge {
    stroke-width: 3;
  }
  ` + kanbanGenSections(o) + `
  .section-root rect, .section-root path, .section-root circle, .section-root polygon  {
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
  .cluster-label, .label {
    color: ` + o.s("textColor") + `;
    fill: ` + o.s("textColor") + `;
    }
  .kanban-label {
    dy: 1em;
    alignment-baseline: middle;
    text-anchor: middle;
    dominant-baseline: middle;
    text-align: center;
  }
    ` + getIconStyles() + `
`
}
