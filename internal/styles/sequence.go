// Ported from mermaid 12.2.0, mermaid/src/diagrams/sequence/styles.js (MIT).

package styles

// sequenceStyles is getStyles of the sequence diagram: the CSS of the diagram type.
func sequenceStyles(o *options) string {
	dropShadow := s(nn(o.Get("dropShadow"), "none"))
	look := o.config.Get("look")

	return `.actor {
    stroke: ` + o.s("actorBorder") + `;
    fill: ` + o.s("actorBkg") + `;
    stroke-width: ` + s(nn(o.Get("strokeWidth"), float64(1))) + `;
  }

  rect.actor.outer-path[data-look="neo"] {
      filter: ` + dropShadow + `;
  }

  rect.note[data-look="neo"] {
      stroke:` + o.s("noteBorderColor") + `;
      fill:` + o.s("noteBkgColor") + `;
      filter: ` + dropShadow + `;
  }

  text.actor > tspan {
    fill: ` + o.s("actorTextColor") + `;
    stroke: none;
  }

  .actor-line {
    stroke: ` + o.s("actorLineColor") + `;
  }

  .innerArc {
    stroke-width: 1.5;
    stroke-dasharray: none;
  }

  .messageLine0 {
    stroke-width: 1.5;
    stroke-dasharray: none;
    stroke: ` + o.s("signalColor") + `;
  }

  .messageLine1 {
    stroke-width: 1.5;
    stroke-dasharray: 2, 2;
    stroke: ` + o.s("signalColor") + `;
  }

  [id$="-arrowhead"] path {
    fill: ` + o.s("signalColor") + `;
    stroke: ` + o.s("signalColor") + `;
  }

  .sequenceNumber {
    fill: ` + o.s("sequenceNumberColor") + `;
  }

  [id$="-sequencenumber"] {
    fill: ` + o.s("signalColor") + `;
  }

  [id$="-crosshead"] path {
    fill: ` + o.s("signalColor") + `;
    stroke: ` + o.s("signalColor") + `;
  }

  .messageText {
    fill: ` + o.s("signalTextColor") + `;
    stroke: none;
  }

  .labelBox {
    stroke: ` + o.s("labelBoxBorderColor") + `;
    fill: ` + o.s("labelBoxBkgColor") + `;
    filter: ` + s(cond(is(look, "neo"), dropShadow, "none")) + `;
  }

  .labelText, .labelText > tspan {
    fill: ` + o.s("labelTextColor") + `;
    stroke: none;
  }

  .loopText, .loopText > tspan {
    fill: ` + o.s("loopTextColor") + `;
    stroke: none;
  }

  .sectionTitle, .sectionTitle > tspan {
    fill: ` + o.s("loopTextColor") + `;
    stroke: none;
  }

  .loopLine {
    stroke-width: 2px;
    stroke-dasharray: 2, 2;
    stroke: ` + o.s("labelBoxBorderColor") + `;
    fill: ` + o.s("labelBoxBorderColor") + `;
  }

  .note {
    //stroke: #decc93;
    stroke: ` + o.s("noteBorderColor") + `;
    fill: ` + o.s("noteBkgColor") + `;
  }

  /*
   * No font-weight here, deliberately. Note text renders inside tspan elements, and a weight emitted at
   * tspan level outranks the inline style drawText puts on the parent text element from the
   * sequence.noteFontWeight config key -- so that documented key would never reach the glyphs.
   * The theme variable formerly emitted here is a different value from that config key, and
   * git/styles.js reads it as its own bold-label weight under redux and neo, so it cannot be
   * lowered there either.
   */
  .noteText, .noteText > tspan {
    fill: ` + o.s("noteTextColor") + `;
    stroke: none;
  }

  .activation0 {
    fill: ` + o.s("activationBkgColor") + `;
    stroke: ` + o.s("activationBorderColor") + `;
  }

  .activation1 {
    fill: ` + o.s("activationBkgColor") + `;
    stroke: ` + o.s("activationBorderColor") + `;
  }

  .activation2 {
    fill: ` + o.s("activationBkgColor") + `;
    stroke: ` + o.s("activationBorderColor") + `;
  }

  .actorPopupMenu {
    position: absolute;
  }

  .actorPopupMenuPanel {
    position: absolute;
    fill: ` + o.s("actorBkg") + `;
    box-shadow: 0px 8px 16px 0px rgba(0,0,0,0.2);
    filter: drop-shadow(3px 5px 2px rgb(0 0 0 / 0.4));
}
  .actor-man circle, line {
    fill: ` + o.s("actorBkg") + `;
    stroke-width: 2px;
  }

  g rect.rect {
    filter: ` + dropShadow + `;
    stroke: ` + o.s("nodeBorder") + `;
  }
`
}
