// Ported from mermaid 12.2.0, mermaid/src/diagrams/ishikawa/ishikawaStyles.ts
// (MIT).

package styles

// ishikawaStyles is getStyles of the ishikawa diagram: the CSS of the diagram type.
func ishikawaStyles(o *options) string {
	return `
.ishikawa .ishikawa-spine,
.ishikawa .ishikawa-branch,
.ishikawa .ishikawa-sub-branch {
  stroke: ` + o.s("lineColor") + `;
  stroke-width: 2;
  fill: none;
}

.ishikawa .ishikawa-sub-branch {
  stroke-width: 1;
}

.ishikawa .ishikawa-arrow {
  fill: ` + o.s("lineColor") + `;
}

.ishikawa .ishikawa-head {
  fill: ` + o.s("mainBkg") + `;
  stroke: ` + o.s("lineColor") + `;
  stroke-width: 2;
}

.ishikawa .ishikawa-label-box {
  fill: ` + o.s("mainBkg") + `;
  stroke: ` + o.s("lineColor") + `;
  stroke-width: 2;
}

.ishikawa text {
  font-family: ` + o.s("fontFamily") + `;
  font-size: ` + o.s("fontSize") + `;
  fill: ` + o.s("textColor") + `;
}

.ishikawa .ishikawa-head-label {
  font-weight: 600;
  text-anchor: middle;
  dominant-baseline: middle;
  font-size: 14px;
}

.ishikawa .ishikawa-label {
  text-anchor: end;
}

.ishikawa .ishikawa-label.cause {
  text-anchor: middle;
  dominant-baseline: middle;
}

.ishikawa .ishikawa-label.align {
  text-anchor: end;
  dominant-baseline: middle;
}

.ishikawa .ishikawa-label.up {
  dominant-baseline: baseline;
}

.ishikawa .ishikawa-label.down {
  dominant-baseline: hanging;
}
`
}
