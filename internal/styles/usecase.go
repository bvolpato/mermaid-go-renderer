// Ported from mermaid 12.2.0, mermaid/src/diagrams/usecase/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// usecaseRoleColors gives one colour for each kind of element. These are the
// values that the role scheme paints with. Each fallback ends at the value
// that the style sheet used before the tokens existed.
func usecaseRoleColors(o *options) *js.Obj[any] {
	role := js.NewObj[any]()
	role.Set("actorBkg", nn(nn(o.Get("usecaseActorBkg"), o.Get("actorBkg")), o.Get("mainBkg")))
	role.Set("actorBorder", nn(nn(o.Get("usecaseActorBorder"), o.Get("actorBorder")), o.Get("primaryColor")))
	role.Set("bkg", nn(o.Get("usecaseBkg"), o.Get("mainBkg")))
	role.Set("border", nn(nn(o.Get("usecaseBorder"), o.Get("nodeBorder")), o.Get("primaryColor")))
	role.Set("boundaryBkg", nn(o.Get("usecaseBoundaryBkg"), o.Get("clusterBkg")))
	role.Set("boundaryBorder", nn(o.Get("usecaseBoundaryBorder"), o.Get("clusterBorder")))
	role.Set("includeLine", nn(o.Get("usecaseIncludeLine"), o.Get("lineColor")))
	role.Set("extendLine", nn(o.Get("usecaseExtendLine"), o.Get("lineColor")))
	return role
}

// usecaseGenColor emits the cycling colour of each item for the colour
// themes. System boundaries take a slot in both colour schemes. Actors and
// use cases take one only in the rotate scheme.
func usecaseGenColor(o *options) string {
	theme, bkgColorArray, borderColorArray := o.Get("theme"), o.Get("bkgColorArray"), o.Get("borderColorArray")
	// Both halves of the gate: a colour theme and a palette that is not empty.
	if !isColorTheme(theme, borderColorArray) {
		return ""
	}
	// The colour scheme is in the config of the diagram, not in the theme variables.
	rotate := is(js.Path(o.config, "usecase", "colorScheme"), "rotate")
	// look is validated before it reaches the selector. See safeLook.
	look := safeLook(o.Get("look"))
	isHandDrawn := look == "handDrawn"
	hasBkgColors := hasPalette(bkgColorArray)
	var sections text

	// One rule for each slot that can be stamped.
	palette, _ := borderColorArray.([]any)
	for i := 0; i < paletteSlotCount(borderColorArray); i++ {
		borderColor := s(elem(palette, i))
		fill := ""
		if hasBkgColors {
			fill = `fill: ` + s(paletteAt(bkgColorArray, i)) + `;`
		}
		slot := `[data-look="` + look + `"][data-color-id="color-` + itoa(i) + `"]`

		// System boundaries, in both schemes.
		sections.add(`

    & ` + slot + `.system-boundary rect.boundary-body,
    & ` + slot + `.system-boundary rect.boundary-tab,
    & ` + slot + `.system-boundary .boundary-body path,
    & ` + slot + `.system-boundary .boundary-tab path {
      stroke: ` + borderColor + `;
      ` + fill + `
    }
    `)

		if !rotate {
			continue
		}

		glyphRule := ""
		if !isHandDrawn {
			glyphRule = `
    /* The group rule above reaches the glyph by inheritance, which the neo look breaks: it
       ships a ` + "`" + `[data-look="neo"].node path { stroke }` + "`" + ` rule that hits the glyph's own paths,
       and a value set directly on the child always beats one inherited from the parent,
       whatever the parent rule's specificity. So name the children too.

       Emitted for every look *except* handDrawn, where roughjs draws the glyph as an
       outline path plus a hachure fill path stroked in the fill colour, indistinguishable
       in CSS -- stroking both turns a hollow actor into a solid disc. Deliberately no
       ` + "`" + `fill` + "`" + ` either way, so the hollow variant's own ` + "`" + `fill="none"` + "`" + ` keeps winning. */
    & ` + slot + `.usecase-actor .usecase-actor-glyph path,
    & ` + slot + `.usecase-actor .usecase-actor-glyph circle {
      stroke: ` + borderColor + `;
    }
`
		}
		sections.add(`

    /* Use case bodies -- ` + "`" + `.usecase-element` + "`" + ` covers the ellipse form, the ` + "`" + `[Rect]` + "`" + ` form and
       the business variant.

       Element selectors only, never a bare ` + "`" + `path` + "`" + `. Under the handDrawn look roughjs draws
       the body as a *pair* of paths, an outline stroked in the border colour and a hachure
       fill stroked in the background colour, with no class to tell them apart. Stroking
       both repaints the fill lines as border colour and the shape collapses into a solid
       block -- which is what ` + "`" + `.usecase-element path` + "`" + ` did. So handDrawn bodies keep the
       theme's uniform colours, exactly as handDrawn flowchart nodes do. */
    & ` + slot + `.usecase-element ellipse,
    & ` + slot + `.usecase-element rect {
      stroke: ` + borderColor + `;
      ` + fill + `
    }

    /* The business marker is a single classed path, so it can be reached safely by name --
       without it the marker keeps the uniform border beside a palette-coloured body. No
       ` + "`" + `fill` + "`" + `: the marker is drawn with ` + "`" + `fill="none"` + "`" + ` and has to stay that way. */
    & ` + slot + `.usecase-element .usecase-business-marker {
      stroke: ` + borderColor + `;
    }

    /* Actor glyphs, mirroring the uniform rule further down. The fill goes on the glyph
       group, never on its children, so the hollow variant's own ` + "`" + `fill="none"` + "`" + ` keeps
       winning and a hollow actor stays hollow. Same reason as above for not descending
       into the handDrawn paths. */
    & ` + slot + `.usecase-actor .usecase-actor-shape,
    & ` + slot + `.usecase-actor .usecase-actor-hollow,
    & ` + slot + `.usecase-actor .usecase-actor-awesome,
    & ` + slot + `.usecase-actor .usecase-actor-icon {
      stroke: ` + borderColor + `;
      ` + fill + `
    }
` + glyphRule + `
    `)
	}
	return sections.String()
}

// usecaseStyles is getStyles of the use case diagram: the CSS of the diagram type.
func usecaseStyles(o *options) string {
	role := usecaseRoleColors(o)
	// The descendant rules of the glyph are emitted for every look except
	// handDrawn. See usecaseGenColor for the same split.
	isHandDrawn := safeLook(o.Get("look")) == "handDrawn"
	genColor := usecaseGenColor(o)
	actorGlyphRule, elementRule := "", ""
	if !isHandDrawn {
		actorGlyphRule = `
  /* The rule above colours the glyph group and lets its children inherit, which the neo
     look breaks: it ships a ` + "`" + `[data-look="neo"].node path { stroke }` + "`" + ` rule that lands on
     the glyph's own paths, and a value set directly on a child always beats one inherited
     from its parent, whatever the parent rule's specificity. Since neo is the default look,
     without this every actor renders in the node border colour rather than the actor
     colour the rule above asks for.

     ` + "`" + `.node` + "`" + ` is in the selector to outrank that neo rule rather than tie with it: both
     would otherwise be one attribute plus one class plus one element, leaving the winner to
     depend on which stylesheet is concatenated last.

     Stroke only: the hollow variant's own ` + "`" + `fill="none"` + "`" + ` has to keep winning. */
  & .node.usecase-actor .usecase-actor-glyph path,
  & .node.usecase-actor .usecase-actor-glyph circle {
    stroke: ` + s(role.Get("actorBorder")) + `;
  }
`
		elementRule = `
  /* The same interception the actor glyph hits, one element down: neo ships
     ` + "`" + `[data-look="neo"].node rect { stroke: nodeBorder }` + "`" + `, which outranks the plain
     ` + "`" + `.usecase-element rect` + "`" + ` above, so a use case written in the ` + "`" + `[Rect]` + "`" + ` form kept the node
     border colour while its ellipse siblings took the role colour. An ` + "`" + `<ellipse>` + "`" + ` has no
     equivalent neo rule and is already correct; restating it here costs nothing and means
     the two forms cannot drift apart again.

     Qualified with ` + "`" + `[data-look]` + "`" + ` *and* ` + "`" + `.node` + "`" + ` to land strictly above that rule rather than
     tie with it -- on a tie the later stylesheet would win, which is how neo took this in
     the first place. Skipped under handDrawn, where roughjs draws paths and neither element
     exists. */
  & [data-look="` + safeLook(o.Get("look")) + `"].node.usecase-element ellipse,
  & [data-look="` + safeLook(o.Get("look")) + `"].node.usecase-element rect {
    fill: ` + s(role.Get("bkg")) + `;
    stroke: ` + s(role.Get("border")) + `;
  }

  /* The business marker is a ` + "`" + `<path>` + "`" + `, so it loses to ` + "`" + `[data-look="neo"].node path` + "`" + ` the same
     way. No ` + "`" + `fill` + "`" + `: the marker is drawn with ` + "`" + `fill="none"` + "`" + ` and has to stay that way. */
  & [data-look="` + safeLook(o.Get("look")) + `"].node.usecase-element .usecase-business-marker {
    stroke: ` + s(role.Get("border")) + `;
  }
`
	}
	return `
  ` + genColor + `
  & .usecase-actor {
    color: ` + s(nn(o.Get("actorTextColor"), o.Get("primaryTextColor"))) + `;
  }

  & .usecase-actor-shape,
  & .usecase-actor-hollow,
  & .usecase-actor-awesome,
  & .usecase-actor-icon {
    fill: ` + s(role.Get("actorBkg")) + `;
    stroke: ` + s(role.Get("actorBorder")) + `;
    stroke-width: 2px;
  }
` + actorGlyphRule + `
  & .usecase-actor .nodeLabel,
  & .actor-label {
    color: ` + s(nn(o.Get("actorTextColor"), o.Get("primaryTextColor"))) + `;
    fill: ` + s(nn(o.Get("actorTextColor"), o.Get("primaryTextColor"))) + `;
    font-family: var(--mermaid-usecase-actor-font-family, ` + o.s("fontFamily") + `);
    font-size: var(--mermaid-usecase-actor-font-size, 14px);
    font-weight: var(--mermaid-usecase-actor-font-weight, normal);
  }

  & .usecase-element ellipse,
  & .usecase-element rect,
  & .usecase-business ellipse,
  & .usecase-business rect {
    fill: ` + s(role.Get("bkg")) + `;
    stroke: ` + s(role.Get("border")) + `;
    stroke-width: 2px;
  }
` + elementRule + `
  & .usecase-element .nodeLabel,
  & .usecase-label {
    color: ` + o.s("primaryTextColor") + `;
    fill: ` + o.s("primaryTextColor") + `;
    font-family: var(--mermaid-usecase-font-family, ` + o.s("fontFamily") + `);
    font-size: var(--mermaid-usecase-font-size, 12px);
    font-weight: var(--mermaid-usecase-font-weight, normal);
  }

  & .usecase-stereotype,
  & .usecase-business-marker {
    color: ` + o.s("primaryTextColor") + `;
    fill: ` + o.s("primaryTextColor") + `;
    stroke: ` + s(role.Get("border")) + `;
  }

  & .system-boundary rect.boundary-body,
  & .system-boundary rect.boundary-tab,
  & .system-boundary-package-tab {
    fill: ` + s(role.Get("boundaryBkg")) + `;
    stroke: ` + s(role.Get("boundaryBorder")) + `;
    stroke-width: 1px;
  }

  & .system-boundary-title text {
    fill: ` + s(nn(o.Get("titleColor"), o.Get("primaryTextColor"))) + `;
  }

  /* Only the span, never the <p> inside it: the renderer puts a user-supplied
     'color' on the span, and that has to stay inheritable by its children. */
  & .system-boundary-title span {
    color: ` + s(nn(o.Get("titleColor"), o.Get("primaryTextColor"))) + `;
  }

  & .usecase-note {
    fill: ` + o.s("noteBkgColor") + `;
    stroke: ` + o.s("noteBorderColor") + `;
    color: ` + o.s("noteTextColor") + `;
  }

  & .usecase-note .nodeLabel {
    color: ` + o.s("noteTextColor") + `;
    fill: ` + o.s("noteTextColor") + `;
  }

  & .usecase-json-table,
  & .usecase-json-table rect,
  & .usecase-json-cell {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + s(nn(o.Get("nodeBorder"), o.Get("primaryColor"))) + `;
  }

  & .usecase-json-title,
  & .usecase-json-key,
  & .usecase-json-value {
    color: ` + o.s("primaryTextColor") + `;
    fill: ` + o.s("primaryTextColor") + `;
  }

  & .relationship {
    fill: none;
    stroke: ` + o.s("lineColor") + `;
  }

  & .relationship-include,
  & .relationship-extend,
  & .relationship-note {
    stroke-dasharray: 3;
  }

  /* Include and extend are both dashed, which is a weak distinction at small sizes. The
     tokens default to ` + "`" + `lineColor` + "`" + `, so a theme that does not set them is unchanged. */
  & .relationship-include {
    stroke: ` + s(role.Get("includeLine")) + `;
  }

  & .relationship-extend {
    stroke: ` + s(role.Get("extendLine")) + `;
  }

  & .relationship.edge-animation-fast,
  & .relationship.edge-animation-slow {
    stroke-linecap: round;
  }

  & .edgeLabel,
  & .edgeLabel p {
    background-color: ` + o.s("edgeLabelBackground") + `;
  }

  & .labelBkg {
    background-color: ` + o.s("edgeLabelBackground") + `;
    padding: 0 2px;
  }

  & .edgeLabel .label rect {
    fill: ` + o.s("edgeLabelBackground") + `;
  }

  & .relationship-label,
  & .edgeLabel {
    color: ` + o.s("primaryTextColor") + `;
    fill: ` + o.s("primaryTextColor") + `;
    font-family: ` + o.s("fontFamily") + `;
    font-size: 10px;
    font-weight: normal;
  }

  & .marker,
  & .marker.point,
  & .marker.circle,
  & .marker.cross {
    fill: ` + o.s("lineColor") + `;
    stroke: ` + o.s("lineColor") + `;
  }

  & .marker.extension {
    fill: ` + o.s("mainBkg") + `;
    stroke: ` + o.s("lineColor") + `;
  }
`
}
