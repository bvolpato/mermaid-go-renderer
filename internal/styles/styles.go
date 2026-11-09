// Ported from mermaid 12.2.0, mermaid/src/styles.ts (MIT).

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// GetStyles is getStyles of styles.ts: the rules that all diagrams share, the
// rules of the diagram type and the user styles, as CSS text that is not yet
// compiled.
//
// opts is the options object that createUserStyles makes:
// { ...config.themeVariables, theme: config.theme, look: config.look }.
// config is the value that getConfig() gives during the call. Mermaid keeps it
// in module state, and the style functions of several diagrams read it.
//
// A theme variable that is missing prints as "undefined", as it does in
// JavaScript. The error is not nil when mermaid throws, for example for a
// colour that khroma cannot parse.
func GetStyles(typ string, userStyles string, opts *js.Obj[any], svgID string, config *js.Obj[any]) (out string, err error) {
	if opts == nil {
		opts = js.NewObj[any]()
	}
	if config == nil {
		config = js.NewObj[any]()
	}
	return getStyles(typ, userStyles, &options{base: opts, config: config}, svgID)
}

// getStyles is GetStyles for the options in the form of this package.
func getStyles(typ string, userStyles string, o *options, svgID string) (out string, err error) {
	defer catch(&err)

	diagramStyles := ""
	if provider, ok := themes[typ]; ok {
		// Pass svgId through options so diagram-specific style functions can use it
		// (e.g., for gradient URL references like url(svgId-gradient)).
		diagramStyles = provider(o.with("svgId", svgID))
	}

	neoStroke := s(o.Get("nodeBorder"))
	if js.Truthy(o.Get("useGradient")) {
		neoStroke = "url(" + svgID + "-gradient)"
	}
	neoFilter := "none"
	if dropShadow := o.Get("dropShadow"); js.Truthy(dropShadow) {
		neoFilter = replace(dropShadow, "url(#drop-shadow)", "url("+svgID+"-drop-shadow)")
	}
	return `& {
    font-family: ` + o.s("fontFamily") + `;
    font-size: ` + o.s("fontSize") + `;
    fill: ` + o.s("textColor") + `
  }
  @keyframes edge-animation-frame {
    from {
      stroke-dashoffset: 0;
    }
  }
  @keyframes dash {
    to {
      stroke-dashoffset: 0;
    }
  }
  & .edge-animation-slow {
    stroke-dasharray: 9,5 !important;
    stroke-dashoffset: 900;
    animation: dash 50s linear infinite;
    stroke-linecap: round;
  }
  & .edge-animation-fast {
    stroke-dasharray: 9,5 !important;
    stroke-dashoffset: 900;
    animation: dash 20s linear infinite;
    stroke-linecap: round;
  }
  /* Classes common for multiple diagrams */

  & .error-icon {
    fill: ` + o.s("errorBkgColor") + `;
  }
  & .error-text {
    fill: ` + o.s("errorTextColor") + `;
    stroke: ` + o.s("errorTextColor") + `;
  }

  & .edge-thickness-normal {
    stroke-width: ` + s(nn(o.Get("strokeWidth"), num(1))) + `px;
  }
  & .edge-thickness-thick {
    stroke-width: 3.5px
  }
  & .edge-pattern-solid {
    stroke-dasharray: 0;
  }
  & .edge-thickness-invisible {
    stroke-width: 0;
    fill: none;
  }
  & .edge-pattern-dashed{
    stroke-dasharray: 3;
  }
  .edge-pattern-dotted {
    stroke-dasharray: 2;
  }

  & .marker {
    fill: ` + o.s("lineColor") + `;
    stroke: ` + o.s("lineColor") + `;
  }
  & .marker.cross {
    stroke: ` + o.s("lineColor") + `;
  }

  & svg {
    font-family: ` + o.s("fontFamily") + `;
    font-size: ` + o.s("fontSize") + `;
  }
   & p {
    margin: 0
   }

  ` + diagramStyles + `
  .node .neo-node {
    stroke: ` + o.s("nodeBorder") + `;
  }

  [data-look="neo"].node rect, [data-look="neo"].cluster rect, [data-look="neo"].node polygon {
    stroke: ` + neoStroke + `;
    filter: ` + neoFilter + `;
  }
  [data-look="neo"].swimlane.cluster rect {
    filter: none;
  }


  [data-look="neo"].node path {
    stroke: ` + neoStroke + `;
    stroke-width: ` + s(nn(o.Get("strokeWidth"), num(1))) + `px;
  }

  [data-look="neo"].node .outer-path {
    filter: ` + neoFilter + `;
  }

  [data-look="neo"].node .neo-line path {
    stroke: ` + o.s("nodeBorder") + `;
    filter: none;
  }

  [data-look="neo"].node circle{
    stroke: ` + neoStroke + `;
    filter: ` + neoFilter + `;
  }

  [data-look="neo"].node circle .state-start{
    fill: #000000;
  }

  [data-look="neo"].icon-shape .icon {
    fill: ` + neoStroke + `;
    filter: ` + neoFilter + `;
  }

    [data-look="neo"].icon-shape .icon-neo path {
    stroke: ` + neoStroke + `;
    filter: ` + neoFilter + `;
  }

  ` + userStyles + `
`, nil
}
