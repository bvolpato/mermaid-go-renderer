// Hand-written cases for gen-xml-serialize.mjs: the corners of the XML
// serialization of an svg element that came from the HTML parser.
//
// Each case is { name, svg, script? }. The svg text becomes the innerHTML of
// a div, and the first svg element is serialized. A script changes the DOM
// before that, for trees that the HTML parser cannot make.
const SVG = 'http://www.w3.org/2000/svg';
const XLINK = 'http://www.w3.org/1999/xlink';
const XHTML = 'http://www.w3.org/1999/xhtml';
const MATHML = 'http://www.w3.org/1998/Math/MathML';

const root = (inner, attrs = '') => `<svg xmlns="${SVG}"${attrs}>${inner}</svg>`;

const voidTags = [
  'area', 'base', 'basefont', 'bgsound', 'br', 'col', 'embed', 'frame', 'hr', 'img',
  'input', 'keygen', 'link', 'meta', 'param', 'source', 'track', 'wbr',
];
const htmlTags = [
  'a', 'b', 'i', 'p', 'div', 'span', 'em', 'strong', 'code', 'pre', 'ul', 'li', 'table',
  'textarea', 'button', 'label', 'small', 'sub', 'sup', 'u', 's', 'h1', 'center', 'font',
  'section', 'canvas', 'video', 'audio', 'picture', 'object', 'select', 'option', 'details',
  'summary', 'dialog', 'slot', 'template', 'custom-element', 'unknownelement',
];
const svgTags = [
  'g', 'path', 'rect', 'circle', 'ellipse', 'line', 'polygon', 'polyline', 'text', 'tspan',
  'textPath', 'defs', 'marker', 'style', 'title', 'desc', 'use', 'image', 'a', 'switch',
  'symbol', 'clipPath', 'mask', 'pattern', 'linearGradient', 'radialGradient', 'stop',
  'filter', 'feGaussianBlur', 'feDropShadow', 'foreignObject', 'script', 'animate',
  'unknownelement',
];

export const cases = [
  // The root element and its namespace declaration.
  { name: 'root/with-xmlns', svg: root('<g></g>') },
  { name: 'root/no-xmlns', svg: '<svg viewBox="0 0 10 10" id="a"><g></g></svg>' },
  { name: 'root/xmlns-in-the-middle', svg: `<svg id="a" xmlns="${SVG}" width="10"><g></g></svg>` },
  { name: 'root/xmlns-last', svg: `<svg id="a" width="10" xmlns="${SVG}"></svg>` },
  { name: 'root/wrong-xmlns', svg: '<svg xmlns="http://example.com/x" id="a"><g xmlns="http://example.com/y"><rect></rect></g></svg>' },
  { name: 'root/empty-xmlns', svg: '<svg xmlns="" id="a"><g xmlns=""></g></svg>' },
  { name: 'root/empty', svg: '<svg></svg>' },
  { name: 'root/mermaid-header', svg: `<svg id="my-svg" width="100%" xmlns="${SVG}" xmlns:xlink="${XLINK}" class="flowchart" style="max-width: 123.456px;" viewBox="0 0 123.456 78" role="graphics-document document" aria-roledescription="flowchart-v2"><style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;}#my-svg .a>b{fill:#333;}</style><g></g></svg>` },
  { name: 'root/not-first', svg: `<p>before</p><div><svg id="inner"><g></g></svg></div><svg id="second"></svg>` },
  { name: 'root/text-around', svg: `text before ${root('<g></g>')} text after` },

  // Elements without children.
  { name: 'empty/svg-elements', svg: root(svgTags.map((t) => `<${t}></${t}>`).join('')) },
  { name: 'empty/svg-elements-with-attrs', svg: root(svgTags.map((t) => `<${t} class="c" data-x="1"></${t}>`).join('')) },
  { name: 'empty/svg-self-closing-source', svg: root('<g/><path d="M0,0"/><rect width="1" height="1"/><text/>') },
  { name: 'empty/html-elements', svg: root(`<foreignObject><div>${htmlTags.map((t) => `<${t}></${t}>`).join('')}</div></foreignObject>`) },
  { name: 'empty/html-void', svg: root(`<foreignObject><div>${voidTags.map((t) => `<${t}>`).join('')}</div></foreignObject>`) },
  { name: 'empty/html-void-with-attrs', svg: root('<foreignObject><div>a<br>b<br/>c<img src="x.png" alt="x"><hr class="h"><input type="text" value="v" disabled><wbr></div></foreignObject>') },
  { name: 'empty/whitespace-only', svg: root('<g> </g><g>\n</g><text> </text><foreignObject><div> </div></foreignObject>') },
  { name: 'empty/comment-only', svg: root('<g><!--c--></g><foreignObject><div><!--c--></div><br><!--c--></foreignObject>') },
  { name: 'empty/math-elements', svg: root(`<foreignObject><div><math xmlns="${MATHML}"><mi></mi><mrow></mrow><mspace width="1em"></mspace><annotation></annotation></math></div></foreignObject>`) },

  // Text.
  { name: 'text/entities', svg: root('<text>a&nbsp;b &amp; &lt;c&gt; &quot;q&quot; &apos;s&apos; &copy; &#x1F600; &hellip; &#38;amp; &#160;</text>') },
  { name: 'text/html-entities', svg: root('<foreignObject><div>a&nbsp;b &amp; &lt;c&gt; &quot;q&quot; &apos;s&apos; &copy; &#x1F600; &amp;nbsp; \u00a0</div></foreignObject>') },
  { name: 'text/non-bmp', svg: root('<text>\u{1F600} \u{1D4B3} \u{10FFFF} \u{2028} \u{FEFF} \u{200B}</text><foreignObject><div title="\u{1F600}">\u{1F469}\u200D\u{1F4BB}</div></foreignObject>') },
  { name: 'text/whitespace', svg: root('<text>  a\tb\nc\r\nd  </text><foreignObject><div>  a\tb\nc\r\nd  <pre>\n\nx</pre><textarea>\n\ny</textarea></div></foreignObject>') },
  { name: 'text/control', svg: root('<text>a\u0001b\u007fc\u0085d\ufffee</text>') },
  { name: 'text/cdata-end', svg: root('<text>a]]>b</text><style>a]]>b</style>') },
  { name: 'text/style-svg', svg: root('<style>#a > b{content:"<&>";font-family:\'x\'}/* c */@media (min-width:1px){a{}}</style>') },
  { name: 'text/style-escaped-source', svg: root('<style>#a &gt; b{content:"&lt;&amp;&gt;"}</style>') },
  { name: 'text/style-html', svg: root('<foreignObject><div><style>#a > b{content:"<&>"}</style><script>if (a < b && c > d) {}</script><xmp><b>&amp;</xmp><textarea><b>&amp;</textarea><title><b>&amp;</title><noscript><b>&amp;</noscript><iframe><b>&amp;</iframe><plaintext><b>&amp;</plaintext></div></foreignObject>') },
  { name: 'text/cdata-foreign', svg: root('<text><![CDATA[a < b & c > d]]></text><style><![CDATA[a > b]]></style>') },
  { name: 'text/cdata-html', svg: root('<foreignObject><div><![CDATA[a < b]]></div></foreignObject>') },
  { name: 'text/adjacent', svg: root('<text>a</text>'), script: `svg.firstChild.appendChild(document.createTextNode('b<'));svg.firstChild.appendChild(document.createTextNode(''));svg.firstChild.appendChild(document.createTextNode('>c'));` },

  // Comments.
  { name: 'comment/plain', svg: root('<!-- a comment --><g><!--x--></g><!---->') },
  { name: 'comment/markup-inside', svg: root('<!-- a -- b <g> &amp; --><!--->-->') },
  { name: 'comment/in-html', svg: root('<foreignObject><div><!-- c --><!bogus><!doctype html></div></foreignObject>') },
  { name: 'comment/before-root', svg: `<!-- before -->${root('<g></g>')}<!-- after -->` },

  // Attribute values.
  { name: 'attr/quotes', svg: root(`<text data-a='say "hi"' data-b="it's" data-c="&quot;&apos;" data-d='"'>t</text>`) },
  { name: 'attr/newlines', svg: root('<text data-a="line1\nline2" data-b="line1&#10;line2" data-c="tab\tx&#9;y" data-d="cr&#13;x" data-e="crlf\r\nx">t</text>') },
  { name: 'attr/markup', svg: root('<text data-a="a<b>c&amp;d" data-b="&lt;&gt;&amp;lt;" data-c="a&b" data-d="x&notit;y">t</text>') },
  { name: 'attr/nbsp', svg: root('<text data-a="a&nbsp;b" data-b="a\u00a0b" data-c="\u{1F600}\u2028">t</text>') },
  { name: 'attr/empty-and-bare', svg: root('<text data-a="" data-b data-c=x data-d = "y">t</text><foreignObject><input disabled checked="" value></foreignObject>') },
  { name: 'attr/duplicate', svg: root('<text class="a" class="b" CLASS="c">t</text>') },
  { name: 'attr/case', svg: `<svg viewbox="0 0 1 1" VIEWBOX="0 0 2 2" preserveaspectratio="none" Data-Foo="1"><marker markerwidth="1" refx="2" markerUnits="userSpaceOnUse"></marker><lineargradient gradientunits="x"></lineargradient><foreignobject requiredextensions="x"><DIV Class="A" onClick="x"></DIV></foreignobject></svg>` },
  { name: 'attr/odd-names', svg: root('<text a:b="1" :c="2" d:="3" _e="4" -f="5" 9g="6" h.i="7" j/k="8">t</text>') },
  { name: 'attr/style', svg: root('<g style="fill:red;stroke:#00F ; opacity:.5"><rect style=""></rect><rect style="not css"></rect></g>') },
  { name: 'attr/url', svg: root('<a href="http://x/?a=1&amp;b=2&c=3#f"><text>t</text></a><image href="data:image/png;base64,AAAA"></image>') },

  // xlink and xml attributes.
  { name: 'xlink/declared-on-root', svg: root('<a xlink:href="http://x/?a=1&amp;b=2"><text>t</text></a><use xlink:href="#a"></use>', ` xmlns:xlink="${XLINK}"`) },
  { name: 'xlink/not-declared', svg: root('<image xlink:href="a.png"></image><g><use xlink:href="#b"></use><use xlink:href="#c" xlink:title="t" xlink:show="new"></use></g>') },
  { name: 'xlink/declared-on-element', svg: root(`<a xmlns:xlink="${XLINK}" xlink:href="#a"><use xlink:href="#b"></use></a><use xlink:href="#c"></use>`) },
  { name: 'xlink/declared-after-use', svg: root(`<a xlink:href="#a" xmlns:xlink="${XLINK}"><use xlink:href="#b"></use></a>`) },
  { name: 'xlink/nested-not-declared', svg: root('<a xlink:href="#a"><use xlink:href="#b"></use><g><use xlink:href="#c"></use></g></a><use xlink:href="#d"></use>') },
  { name: 'xlink/wrong-declaration', svg: root('<a xlink:href="#a"><use xlink:href="#b"></use></a>', ' xmlns:xlink="http://example.com/wrong"') },
  { name: 'xlink/all-names', svg: root('<a xlink:actuate="a" xlink:arcrole="b" xlink:href="c" xlink:role="d" xlink:show="e" xlink:title="f" xlink:type="g" xlink:other="h"></a>', ` xmlns:xlink="${XLINK}"`) },
  { name: 'xlink/on-html', svg: root('<foreignObject><div xlink:href="#a" xml:space="preserve" xmlns:xlink="x"></div></foreignObject>', ` xmlns:xlink="${XLINK}"`) },
  { name: 'xlink/prefix-for-html-namespace', svg: root('<foreignObject><div><span>a</span><br></div><p xmlns="http://www.w3.org/1999/xhtml">b</p></foreignObject>', ` xmlns:xlink="${XHTML}"`) },
  { name: 'xlink/prefix-for-svg-namespace', svg: `<svg xmlns:xlink="${SVG}" id="a"><g xlink:href="#x"></g></svg>` },
  { name: 'xml/space-lang', svg: root('<text xml:space="preserve" xml:lang="en" xml:base="b" xml:id="i" xml:other="o">t</text>') },
  { name: 'xmlns/unknown-prefix', svg: `<svg xmlns:foo="http://foo/" xmlns="${SVG}"><g foo:bar="1" xmlns:bar="http://bar/"></g></svg>` },
  { name: 'xlink/set-without-prefix', svg: root('<use></use><use></use>'), script: `svg.children[0].setAttributeNS('${XLINK}', 'href', '#a');svg.children[1].setAttributeNS('${XLINK}', 'href', '#b');svg.children[1].setAttributeNS('${XLINK}', 'x:title', 't');svg.children[1].setAttributeNS('http://example.com/ns', 'k', 'v');` },
  { name: 'xlink/set-with-prefix', svg: root('<use></use><g><use></use></g>'), script: `svg.children[0].setAttributeNS('${XLINK}', 'xlink:href', '#a');svg.children[1].setAttributeNS('http://www.w3.org/2000/xmlns/', 'xmlns:xl', '${XLINK}');svg.children[1].firstChild.setAttributeNS('${XLINK}', 'xlink:href', '#b');` },
  { name: 'xmlns/set-by-script', svg: '<svg><g></g><text>t</text></svg>', script: `svg.setAttributeNS('http://www.w3.org/2000/xmlns/', 'xmlns', '${SVG}');svg.setAttributeNS('http://www.w3.org/2000/xmlns/', 'xmlns:xlink', '${XLINK}');svg.children[0].setAttribute('xmlns', 'http://example.com/plain');svg.children[1].setAttribute('xmlns:q', 'http://example.com/q');` },

  // Nested svg and foreignObject.
  { name: 'nested/svg', svg: root(`<svg x="1" y="2"><rect></rect></svg><svg xmlns="${SVG}"><g></g></svg><svg xmlns="http://example.com/x"></svg><g><svg></svg></g>`) },
  { name: 'nested/foreignobject-html-xmlns', svg: root(`<foreignObject width="10" height="10"><div xmlns="${XHTML}" style="display: table-cell;"><span class="nodeLabel"><p>a<br>b</p></span></div></foreignObject>`) },
  { name: 'nested/foreignobject-html-no-xmlns', svg: root('<foreignObject width="10" height="10"><div><span class="nodeLabel"><p>a<br>b</p></span></div><span>c</span></foreignObject>') },
  { name: 'nested/foreignobject-wrong-xmlns', svg: root(`<foreignObject><div xmlns="http://example.com/wrong"><span xmlns="${XHTML}">a</span></div><div xmlns="${SVG}"></div><div xmlns=""><b></b></div></foreignObject>`) },
  { name: 'nested/foreignobject-xmlns-prefix', svg: root(`<foreignObject><div xmlns:foo="http://foo/" foo:bar="1" xmlns:xlink="${XLINK}" xlink:href="#a"><span foo:baz="2"></span></div></foreignObject>`) },
  { name: 'nested/svg-in-html', svg: root(`<foreignObject><div><svg width="1" height="1"><circle r="1"></circle></svg><svg xmlns="${SVG}"><circle r="1"></circle></svg>text<span><svg><foreignObject><b>deep</b><svg></svg></foreignObject></svg></span></div></foreignObject>`) },
  { name: 'nested/foreignobject-text', svg: root('<foreignObject>bare text <b>bold</b> &amp; more</foreignObject>') },
  { name: 'nested/html-breaks-out', svg: root('<g><div>x</div><rect></rect></g>') },
  { name: 'nested/desc-title-html', svg: root('<title>t <b>bold</b></title><desc><p>para</p></desc>') },
  { name: 'nested/table', svg: root('<foreignObject><table><tr><td>a</td><td>b<br></td></tr></table><ul><li>x<li>y</ul><p>p1<p>p2</foreignObject>') },
  { name: 'nested/label-icons', svg: root(`<foreignObject><div xmlns="${XHTML}"><span class="nodeLabel"><i class="fa fa-car"></i> Car <img src="https://example.com/a.png" width="20" height="20" style="display: flex;"> <strong>bold</strong> <em>em</em></span></div></foreignObject>`) },

  // MathML.
  { name: 'math/with-xmlns', svg: root(`<foreignObject><div xmlns="${XHTML}"><math xmlns="${MATHML}" display="block"><semantics><mrow><mi>x</mi><mo>=</mo><mfrac><mn>1</mn><mn>2</mn></mfrac><mspace></mspace><mo>&lt;</mo><mo>&InvisibleTimes;</mo></mrow><annotation encoding="application/x-tex">x=\\frac{1}{2}&lt;</annotation></semantics></math></div></foreignObject>`) },
  { name: 'math/no-xmlns', svg: root('<foreignObject><div><math><mi>y</mi><mtext><b>html</b></mtext><annotation-xml encoding="text/html"><b>bold</b><br></annotation-xml><annotation-xml encoding="application/xhtml+xml"><i>it</i></annotation-xml><annotation-xml><mi>z</mi><svg></svg></annotation-xml></math></div></foreignObject>') },
  { name: 'math/in-svg', svg: root('<math><mi>x</mi></math><g><math></math></g>') },
  { name: 'math/definitionurl', svg: root(`<foreignObject><math definitionurl="u" xlink:href="#a" xmlns:xlink="${XLINK}"><mi mathvariant="bold">x</mi></math></foreignObject>`) },

  // Tag names.
  { name: 'tags/case', svg: '<SVG><G><ForeignObject><DIV>a</DIV></ForeignObject><foreignobject></foreignobject><LINEARGRADIENT></LINEARGRADIENT><clippath></clippath><textpath></textpath><altglyph></altglyph></G></SVG>' },
  { name: 'tags/odd-names', svg: root('<a:b></a:b><foo-bar></foo-bar><x.y></x.y><_z></_z>') },
  { name: 'tags/svg-prefix', svg: root('<svg:g></svg:g><xlink:a></xlink:a><foreignObject><html:div></html:div><svg:rect></svg:rect></foreignObject>') },
  // A Node of the Go DOM has no prefix. These elements are in the namespace
  // of their parent or have no namespace, so that the prefix has no effect.
  { name: 'tags/created-with-prefix', svg: root('<g></g>'), script: `const e = document.createElementNS('${SVG}', 'svg:rect'); e.setAttribute('x', '1'); svg.firstChild.appendChild(e); const n = document.createElementNS(null, 'nons'); n.appendChild(document.createElementNS(null, 'kid')); n.appendChild(document.createElementNS('${SVG}', 'g')); n.setAttribute('xmlns', ''); svg.appendChild(n); const m = document.createElementNS(null, 'nons2'); m.setAttributeNS('http://www.w3.org/2000/xmlns/', 'xmlns', 'http://example.com/z'); svg.appendChild(m);` },
  { name: 'tags/html-style-created', svg: root('<g></g>'), script: `const s = document.createElement('style'); s.textContent = '#a>b{c:"<&>"}'; svg.insertBefore(s, svg.firstChild); const d = document.createElement('div'); svg.appendChild(d); d.appendChild(document.createElement('br')); d.appendChild(document.createElementNS('${SVG}', 'br'));` },
];

// Values of --backgroundColor: svg.style.backgroundColor = value.
export const backgrounds = [
  'white', 'transparent', 'red', 'RED', 'Red', '#F0F0F0', '#f0f0f0', '#fff', '#FFF8', '#ff000080',
  '#12345678', 'rgb(1,2,3)', 'rgb(1 2 3)', 'rgba(1,2,3,.5)', 'rgba(1, 2, 3, 0.25)', 'rgb(1 2 3 / 50%)',
  'rgb(100%, 0%, 50%)', 'rgb(300, -5, 12.6)', 'hsl(120, 50%, 50%)', 'hsl(120 50% 50%)',
  'hsla(120, 50%, 50%, 0.3)', 'hsl(0.5turn 100% 25%)', 'hwb(120 10% 20%)', 'lab(50% 40 30)',
  'lch(50% 40 30)', 'oklab(0.5 0.1 0.1)', 'oklch(0.5 0.1 30)', 'color(display-p3 1 0 0)',
  'color(srgb 1 0.5 0 / 0.5)', 'color-mix(in srgb, red, blue)', 'light-dark(red, blue)',
  'currentcolor', 'currentColor', 'inherit', 'initial', 'unset', 'revert', 'var(--x)',
  'var(--x, red)', 'canvas', 'CanvasText', 'ButtonFace', 'rebeccapurple', 'grey', 'gray',
  'transparent ', ' white', 'white;', 'white !important', 'notacolor', '', ' ', '#12', '#ggg',
  'F0F0F0', 'fff', '123', '0', 'url(x.png)', 'red blue', 'rgb(1,2)', 'none', 'black/**/',
  '\\72 ed', 'white; fill: red', 'rgb(0 0 0 / 0)', 'rgba(255,255,255,1)', 'rgba(255,255,255,0.999)',
  'rgb(calc(1 + 2), 0, 0)', 'hsl(0deg 0% 0%)', '#000', 'black', 'WINDOW', 'accentcolor',
  // A hex color without the #: the inline style of an SVG element is parsed
  // in quirks mode.
  'abc', 'ABCDEF', '1', '12', '1234', '12345', '123456', '1234567', '000', '00', 'ff', 'ffff',
  'fffff', 'ffffff', '12ab', '1e3', '0001FF', 'bad', 'face', '#ab', '#abcd', '#abcde', '-1', '1.5',
];

// Root style attributes that the background step starts from.
export const backgroundRoots = [
  '<svg id="a" width="100%" xmlns="http://www.w3.org/2000/svg" style="max-width: 123.456px;" viewBox="0 0 1 1"><g></g></svg>',
  '<svg id="a" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><g></g></svg>',
  '<svg id="a" style="background-color: blue; max-width: 10px" height="20" class="c"><g></g></svg>',
  '<svg id="a" style="max-width:5e2px;BACKGROUND-COLOR:#FF0000;max-height: 7px" height="20" class="c"><g></g></svg>',
  '<svg id="a" style="" height="20"><g></g></svg>',
  '<svg id="a" style="not css; max-width: 0.10px !important" height="20"><g></g></svg>',
];
