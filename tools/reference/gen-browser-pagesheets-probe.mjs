// Writes the probe cases for the page of mermaid-cli and what Chromium
// measures for them in a page with the style sheets and the web fonts of
// mermaid-cli (oracle.mjs eval --bare --webfonts):
//
//   internal/browser/testdata/pagesheets-cases.json.gz
//   internal/browser/testdata/pagesheets-results.json.gz        Chromium on macOS
//   internal/browser/testdata/linux-pagesheets-results.json.gz  Chromium on
//       Linux with only the Liberation fonts (oracle-linux.mjs --fonts liberation)
//
// The cases are the labels that mermaid makes for the icons of Font
// Awesome: an i element with the classes of the icon, in the markup of a
// flowchart label. The probe records the computed styles of the icon and of
// its ::before pseudo-element, and the boxes of the icon and of the label.
// TestPageSheetsAgainstChromium and TestLinuxPageSheetsAgainstChromium of
// internal/browser replay them.
//
// A query has `fonts: true` when its value depends on a face of the web
// fonts: the place of an icon box on its line (the ascent of the icon
// font), the width of an icon box whose width is auto (the advance of the
// glyph), and text in Recursive Variable. The Go tests compare such a value
// only when the font library has the web fonts.
//
// The cases without an icon prove that the page sheets change nothing for
// an element with no Font Awesome class, but six custom properties that
// every element inherits from :root.
//
// Usage (in tools/reference): node gen-browser-pagesheets-probe.mjs [--mac] [--linux]
// Without an option the script records both.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');
const probe = path.join(here, 'probes/pagesheets.js');

// The rules of the style sheet of a flowchart that reach the HTML of a
// label, from a reference SVG of mermaid 12.2.0.
const style = (family, size) =>
  `#my-svg{font-family:${family};font-size:${size}px;fill:#28253D;}` +
  `#my-svg svg{font-family:${family};font-size:${size}px;}` +
  '#my-svg p{margin:0;}' +
  `#my-svg .label{font-family:${family};color:#28253D;}` +
  '#my-svg .cluster-label span{color:#000000;}' +
  '#my-svg .cluster-label span p{background-color:transparent;}' +
  '#my-svg .label text,#my-svg span{fill:#28253D;color:#28253D;}' +
  '#my-svg .rough-node .label,#my-svg .node .label,#my-svg .image-shape .label,#my-svg .icon-shape .label{text-align:center;}' +
  '#my-svg .edgeLabel{background-color:hsl(-120, 0%, 80%);text-align:center;}' +
  '#my-svg .edgeLabel p{background-color:hsl(-120, 0%, 80%);}' +
  '#my-svg .labelBkg{background-color:rgba(204, 204, 204, 0.5);}' +
  '#my-svg .label-icon{display:inline-block;height:1em;overflow:visible;vertical-align:-0.125em;}' +
  '#my-svg :root{--mermaid-font-family:"trebuchet ms",verdana,arial,sans-serif;}';

// The styles that createText of mermaid gives to the div of a label: the
// first measurement, a label that wraps, and a node label of the neo look.
const divStyles = {
  cell: 'display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;',
  wrap: 'display: table; white-space: break-spaces; line-height: 1.5; max-width: 200px; text-align: center; width: 200px;',
  node: 'display: table; white-space: nowrap; line-height: 1.5; max-width: 120px; text-align: center; width: 120px;',
};

const ARIAL = 'arial,sans-serif';
const TREBUCHET = '"trebuchet ms",verdana,arial,sans-serif';
const RECURSIVE = '"Recursive Variable",arial,sans-serif';

const svg = (family, size, inner, extraCss = '') =>
  `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" class="flowchart" style="max-width: 500px;" viewBox="0 0 500 300">` +
  `<style>${style(family, size)}${extraCss}</style><g class="root">${inner}</g></svg>`;

// label is the markup of an edge label or of a node label of a flowchart.
const label = (kind, divStyle, inner) => {
  const fo =
    `<foreignObject id="fo" width="300" height="100"><div xmlns="http://www.w3.org/1999/xhtml" id="d"` +
    `${kind === 'edge' ? ' class="labelBkg"' : ''} style="${divStyle}">` +
    `<span id="s" class="${kind === 'edge' ? 'edgeLabel' : 'nodeLabel'}"><p id="p">${inner}</p></span></div></foreignObject>`;
  if (kind === 'edge') {
    return `<g class="edgeLabels"><g class="edgeLabel" transform="translate(120.5, 40)"><g class="label" transform="translate(-20.25, -10.5)">${fo}</g></g></g>`;
  }
  return `<g class="nodes"><g class="node default" transform="translate(88, 34.5)"><g class="label" transform="translate(-60, -10.5)">${fo}</g></g></g>`;
};

const cases = [];
const add = (name, html, queries) => cases.push({ name: `${name}-${cases.length}`, html, queries });

const ICON_PROPS = ['display', 'width', 'height', 'font-size', 'font-family', 'font-weight', 'font-style', 'line-height', 'text-align', 'vertical-align', 'content', '--fa', '--fa-family', '--_fa-family', '--fa-style', '--fa-width', '--fa-family-classic', '--fa-font-solid'];
const BEFORE_PROPS = ['display', 'content', 'font-size', 'font-family', 'font-weight', 'font-style', 'line-height', 'width', '--fa'];

// iconQueries are the queries of a label with icons. recursive is true when
// the text is in a web font. autoWidth is true when an icon box has the
// width of its glyph.
const iconQueries = (opts = {}) => {
  const textFonts = !!opts.recursive || !!opts.autoWidth || !!opts.lineFonts;
  const sel = opts.sel || '#p i';
  const q = [
    // The boxes of the label: what mermaid reads.
    { sel: '#d', op: 'rect', fonts: textFonts },
    { sel: '#p', op: 'rect', fonts: textFonts },
    { sel: '#s', op: 'rect', fonts: textFonts },
    // The boxes of the icons. Their place on the line needs the ascent of
    // the icon font.
    { sel, op: 'rect', all: true, fonts: true },
  ];
  for (const prop of ICON_PROPS) {
    const fonts = (prop === 'width' && !!opts.autoWidth) || (prop === 'height' && !!opts.autoHeight);
    q.push({ sel, op: 'style', arg: prop, fonts });
  }
  for (const prop of BEFORE_PROPS) {
    q.push({ sel, op: 'pstyle', arg: ['::before', prop] });
  }
  q.push({ sel, op: 'pstyle', arg: ['::after', 'content'] });
  return q;
};

const icon = (classes) => `<i class="${classes}"></i>`;

// 1. The labels of the corpus: the texts of testdata/corpus with fa: icons.
const corpusLabels = [
  `${icon('fa fa-car')} Car`,
  `${icon('fa fa-car')}`,
  `${icon('fa fa-cogs')} Shared Business Logic Server:Service 1`,
  `${icon('fa fa-server')} DBServer`,
  `${icon('fa fa-circle')} [DBServer\\SharedDbInstance].[SupportDb]`,
  `${icon('fa fa-database')} [DBServer\\SharedDbInstance]`,
  `${icon('fa fa-creative-commons')} My System | Test Environment`,
  `${icon('fab fa-twitter')} for peace`,
  `${icon('fa fa-twitter')} for peace`,
  `${icon('fab fa-github')}`,
  `${icon('fab fa-facebook')} Facebook`,
  `${icon('fa fa-ban')} forbidden`,
  `${icon('fa fa-spinner')}`,
  `A ${icon('fa fa-camera-retro')} perhaps?`,
  `${icon('fab fa-truck-bold')} a custom icon`,
  `${icon('fa fa-bell')} ${icon('fa fa-coffee')}`,
  `Laptop ${icon('fa fa-code')}`,
  `${icon('fa fa-car')}${icon('fa fa-bell')}`,
];
for (const family of [ARIAL, TREBUCHET, 'courier', RECURSIVE]) {
  for (const size of family === ARIAL ? [14, 16] : [14]) {
    for (const kind of ['edge', 'node']) {
      for (const text of corpusLabels) {
        const divStyle = kind === 'edge' ? divStyles.cell : divStyles.node;
        add(`corpus-${kind}`, svg(family, size, label(kind, divStyle, text)), iconQueries({ recursive: family === RECURSIVE }));
      }
    }
  }
}

// 2. Font sizes, also sizes that are not whole pixels.
for (const size of [9, 10.5, 11, 12, 13, 13.333, 15, 17, 17.5, 18, 20, 22.4, 24, 32, 40]) {
  add('size', svg(ARIAL, size, label('edge', divStyles.cell, `${icon('fa fa-car')} Car`)), iconQueries());
  add('size', svg(ARIAL, size, label('edge', divStyles.cell, icon('fab fa-github'))), iconQueries());
}

// 3. Labels that wrap.
const wrapLabels = [
  `${icon('fa fa-cogs')} Shared Business Logic Server:Service 1`,
  `${icon('fa fa-cogs')} Dedicated Test Business Logic Server:Service 2`,
  `This is a long label ${icon('fa fa-car')} with an icon in the middle of the text`,
  `A label that ends with an icon after many words ${icon('fa fa-bell')}`,
  `${icon('fa fa-car')} ${icon('fa fa-bell')} ${icon('fa fa-coffee')} ${icon('fa fa-server')} ${icon('fa fa-cogs')} ${icon('fa fa-ban')} ${icon('fa fa-code')} ${icon('fa fa-circle')} ${icon('fa fa-database')} ${icon('fa fa-spinner')} ${icon('fa fa-car')} ${icon('fa fa-bell')}`,
  `${icon('fa fa-car')}${icon('fa fa-bell')}${icon('fa fa-coffee')}${icon('fa fa-server')}${icon('fa fa-cogs')}${icon('fa fa-ban')}${icon('fa fa-code')}${icon('fa fa-circle')}${icon('fa fa-database')}${icon('fa fa-spinner')}${icon('fa fa-car')}${icon('fa fa-bell')}${icon('fa fa-car')}`,
  `Averyveryverylongwordwithoutanybreakopportunityinit ${icon('fa fa-car')} x`,
];
for (const family of [ARIAL, RECURSIVE]) {
  for (const text of wrapLabels) {
    add('wrap', svg(family, 14, label('node', divStyles.wrap, text)), iconQueries({ recursive: family === RECURSIVE }));
  }
}

// 4. The style classes and the classes that are not in Font Awesome Free.
for (const prefix of ['fa', 'fas', 'far', 'fab', 'fal', 'fak', 'fa-solid', 'fa-regular', 'fa-brands', 'fa-classic', 'fa-classic fa-solid', 'fa-classic fa-regular', 'fa-classic fa-brands', 'fa-nosuchstyle']) {
  for (const name of ['fa-bell', 'fa-github', 'fa-nosuchicon']) {
    add('style', svg(ARIAL, 14, label('edge', divStyles.cell, `${icon(`${prefix} ${name}`)} Text`)), iconQueries());
  }
}

// 5. The classes of Font Awesome that are not icons, with an icon (HTML that
// the author of a diagram writes) and alone (fa:fa-2x in a label).
const utilities = [
  'fa-1x', 'fa-2x', 'fa-3x', 'fa-4x', 'fa-5x', 'fa-6x', 'fa-7x', 'fa-8x', 'fa-9x', 'fa-10x',
  'fa-2xs', 'fa-xs', 'fa-sm', 'fa-lg', 'fa-xl', 'fa-2xl',
  'fa-fw', 'fa-width-fixed', 'fa-width-auto',
  'fa-canvas-square', 'fa-canvas-roomy', 'fa-border', 'fa-inverse',
  'fa-spin', 'fa-pulse', 'fa-beat', 'fa-fade', 'fa-spin-reverse',
  'fa-stack', 'fa-stack-1x', 'fa-stack-2x', 'fa-li', 'fa-ul', 'fa-pull-left', 'fa-pull-right',
  'fa-rotate-90', 'fa-rotate-180', 'fa-rotate-270', 'fa-flip-horizontal', 'fa-flip-vertical', 'fa-flip-both', 'fa-rotate-by',
];
// The line of an icon with another font size has the height that the ascent
// of the icon font gives. A float and an absolute box are not in the flow.
const sizing = new Set(['fa-2x', 'fa-3x', 'fa-4x', 'fa-5x', 'fa-6x', 'fa-7x', 'fa-8x', 'fa-9x', 'fa-10x', 'fa-xl', 'fa-2xl', 'fa-lg', 'fa-sm', 'fa-xs', 'fa-2xs', 'fa-canvas-square', 'fa-canvas-roomy', 'fa-border', 'fa-stack']);
for (const u of utilities) {
  const opts = { autoWidth: u === 'fa-width-auto', lineFonts: sizing.has(u) };
  add('utility', svg(ARIAL, 14, label('edge', divStyles.cell, `${icon(`fa fa-car ${u}`)} Car`)), iconQueries(opts));
  add('utility-alone', svg(ARIAL, 14, label('edge', divStyles.cell, `${icon(`fa ${u}`)} Car`)), iconQueries({ lineFonts: sizing.has(u) }));
}

// 6. Other content around the icon: the markdown of a label, and HTML that
// the author writes.
const mixed = [
  `<strong>${icon('fa fa-car')} bold</strong> and <em>${icon('fa fa-bell')} italic</em>`,
  `${icon('fa fa-car')}<br>second line`,
  `first line<br>${icon('fa fa-car')}`,
  `<span style="font-size: 20px">${icon('fa fa-car')} big</span> small`,
  `<span style="font-size: 10px">${icon('fa fa-car')} small</span> normal`,
  `<span class="fa-car">${icon('fa')} the class of the icon is on the parent</span>`,
  `<i class="fa fa-car">text in the icon</i> after`,
  `<i class="fa fa-car" style="--fa-display: inline">x</i> inline icon`,
  `<i class="fa fa-car" style="--fa-width: 3em"></i> wide`,
  `<i class="fa fa-car" style="--fa: 'ab'"></i> two characters`,
  `<i class="fa fa-car" style="--fa: ''"></i> empty string`,
  `<i class="fa fa-car" style="font-size: 30px"></i> inline size`,
  `<i class="fa fa-car" style="vertical-align: middle"></i> middle`,
  `<i class="fa fa-car" style="vertical-align: 5px"></i> raised`,
  `<i class="fa fa-car" style="vertical-align: -50%"></i> lowered`,
  `<i class="fa fa-car" style="margin: 3px 5px"></i> margins`,
  `<i class="fa fa-car" style="padding: 2px 4px; border: 1px solid red"></i> padding and border`,
  `<i class="fa fa-car" style="overflow: hidden"></i> clipped`,
  `<b class="fa fa-car"></b> a b element`,
  `<span class="fa fa-car"></span> a span`,
];
for (const text of mixed) {
  const sel = text.includes('<i ') || text.includes('<i>') ? '#p i' : '#p .fa';
  add('mixed', svg(ARIAL, 14, label('node', divStyles.cell, text)), iconQueries({ lineFonts: true, sel }));
}

// 7. Inline blocks with no icon: the atomic inline box itself, in fonts
// that are not web fonts.
const boxQueries = [
  { sel: '#d', op: 'rect' },
  { sel: '#p', op: 'rect' },
  { sel: '#s', op: 'rect' },
  { sel: '.b', op: 'rect', all: true },
];
const blocks = [
  'a <span class="b" style="display: inline-block">box</span> b',
  'a <span class="b" style="display: inline-block; width: 50px">box</span> b',
  'a <span class="b" style="display: inline-block; width: 50px; height: 40px">box</span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 10px"></span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 40px"></span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 40px; vertical-align: middle"></span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 40px; vertical-align: sub"></span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 40px; vertical-align: super"></span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 20px; vertical-align: -7px"></span> b',
  'a <span class="b" style="display: inline-block; width: 30px; height: 20px; vertical-align: 40%"></span> b',
  'a <span class="b" style="display: inline-block; font-size: 30px">big</span> b',
  'a <span class="b" style="display: inline-block; font-size: 8px">small</span> b',
  'a <span class="b" style="display: inline-block; line-height: 1">one</span> b',
  'a <span class="b" style="display: inline-block; line-height: 3">three</span> b',
  'a <span class="b" style="display: inline-block; white-space: normal; width: 40px">two lines here</span> b',
  'a <span class="b" style="display: inline-block; overflow: hidden">hidden</span> b',
  'a <span class="b" style="display: inline-block; overflow: hidden; height: 40px">hidden</span> b',
  'a <span class="b" style="display: inline-block; margin: 4px 6px; padding: 3px 5px; border: 2px solid red">edges</span> b',
  'a <span class="b" style="display: inline-block"><span class="b" style="display: inline-block; font-size: 24px">in</span> out</span> b',
  '<span class="b" style="display: inline-block; width: 20px; height: 20px"></span>',
  '<span class="b" style="display: inline-block; width: 20px; height: 20px"></span><span class="b" style="display: inline-block; width: 20px; height: 30px"></span>',
  'a <b><span class="b" style="display: inline-block">in bold</span></b> b',
  'a <span class="b" style="display: inline-flex; width: 30px; height: 12px"></span> b',
  'a <span class="b" style="display: inline-block; width: calc(10px + 2em); height: calc(3 * 0.5em)"></span> b',
  'a <span class="b" style="display: inline-block; width: 2em; height: 1em; font-size: calc(20 / 16 * 1em)"></span> b',
];
for (const family of [ARIAL, TREBUCHET]) {
  for (const text of blocks) {
    add('block', svg(family, 14, label('node', divStyles.cell, text)), boxQueries);
    add('block', svg(family, 14, label('node', divStyles.wrap, text)), boxQueries);
  }
}

// 8. No Font Awesome class: the page sheets change nothing. The elements
// have the fonts of the system, and each has the custom properties of the
// :root rules.
const ROOT_PROPS = ['--fa-family-classic', '--fa-family-brands', '--fa-font-solid', '--fa-font-regular', '--fa-font-brands', '--fa-style-family-classic', '--fa', '--fa-family'];
const plainQueries = [
  { sel: '#d', op: 'rect' },
  { sel: '#p', op: 'rect' },
  { sel: '#s', op: 'rect' },
  { sel: '#p *', op: 'rect', all: true },
  ...ROOT_PROPS.flatMap((prop) => [
    { sel: '#my-svg', op: 'style', arg: prop },
    { sel: '#fo', op: 'style', arg: prop },
    { sel: '#p', op: 'style', arg: prop },
  ]),
  ...['display', 'font-family', 'font-weight', 'font-style', 'font-size', 'line-height', 'text-align', 'content'].map((prop) => ({ sel: '#p', op: 'style', arg: prop })),
  { sel: '#p', op: 'pstyle', arg: ['::before', 'content'] },
  { sel: '#p', op: 'pstyle', arg: ['::after', 'content'] },
];
const plain = [
  'Car',
  'Some <b>bold</b> and <i>italic</i> text',
  'An i element with <i></i> no content',
  '<i class="other">an i with a class</i> and <code>code</code>',
  'line one<br>line two',
  '<em>emphasis</em> and <strong>strong</strong>',
];
for (const family of [ARIAL, TREBUCHET, 'courier']) {
  for (const text of plain) {
    add('plain', svg(family, 14, label('edge', divStyles.cell, text)), plainQueries);
    add('plain', svg(family, 14, label('node', divStyles.wrap, text)), plainQueries);
  }
}

// 9. SVG text: a text element has no generated content. The classes of an
// icon change its font, so the boxes need the web fonts.
const textQueries = [
  { sel: '#t', op: 'bbox', fonts: true },
  { sel: '#t', op: 'style', arg: 'font-family' },
  { sel: '#t', op: 'style', arg: 'font-weight' },
  { sel: '#t', op: 'pstyle', arg: ['::before', 'content'] },
];
add('svgtext', svg(ARIAL, 14, '<text id="t" x="10" y="50">fa:fa-car Car</text>'), [{ sel: '#t', op: 'bbox' }, ...textQueries.slice(1)]);
add('svgtext', svg(ARIAL, 14, '<text id="t" x="10" y="50" class="fa fa-car">Car</text>'), textQueries);
add('svgtext', svg(ARIAL, 14, '<text id="t" x="10" y="50"><tspan class="fa fa-car"></tspan> Car</text>'), [{ sel: '#t', op: 'bbox' }, ...textQueries.slice(1)]);

// 10. Generated content with rules of the style element of the diagram, in
// a font that is not a web font: the property content, the cascade of a
// pseudo-element and the layout of its box.
const pseudoQueries = [
  { sel: '#d', op: 'rect' },
  { sel: '#p', op: 'rect' },
  { sel: '#s', op: 'rect' },
  { sel: '#c', op: 'rect' },
  { sel: '#c', op: 'pstyle', arg: ['::before', 'content'] },
  { sel: '#c', op: 'pstyle', arg: ['::after', 'content'] },
  { sel: '#c', op: 'pstyle', arg: ['::before', 'display'] },
  { sel: '#c', op: 'pstyle', arg: ['::before', 'font-size'] },
  { sel: '#c', op: 'pstyle', arg: ['::before', 'font-weight'] },
  { sel: '#p', op: 'pstyle', arg: ['::before', 'content'] },
];
const SPAN = 'a <span id="c" class="c" title="the title">x</span> b';
const pseudoRules = [
  '.c::before{content:"[";}.c::after{content:"]";}',
  '.c::before{content:"abc ";font-weight:bold;}',
  '.c::after{content:" (after)";font-size:20px;}',
  '.c::before{content:"x";display:block;}',
  '.c::before{content:"x";display:inline-block;width:30px;}',
  '.c::before{content:"x";display:none;}',
  '.c::before{content:"";}',
  '.c::before{content:none;}',
  '.c::before{content:normal;}',
  '.c:before{content:"one colon ";}.c:after{content:" one colon";}',
  'p::before{content:"p ";}',
  '.c::before{content:"a" "b";}',
  '.c::before{content:"a""b" "c";}',
  '.c::before{content:"x"/"alt";}',
  '.c::before{content:"x" / "alt" "more";}',
  '.c::before{content:"x" /;}',
  '.c::before{content:/ "x";}',
  '.c::before{content:"x" foo;}',
  '.c::before{content:"old";content:foo;}',
  '.c::before{content:"\\41 B";}',
  '.c::before{content:"\\000041B";}',
  '.c::before{content:"\\41B";}',
  '.c::before{content:"a\\"b";}',
  ".c::before{content:'a\\'b';}",
  '.c::before{content:"a\\\\b";}',
  '.c::before{content:"\\0";}',
  '.c::before{content:"\\d800 x";}',
  '.c::before{content:"\\110000 x";}',
  '.c::before{content:"\\e9 t\\e9";}',
  '.c::before{content:"é—";}',
  '.c::before{content:"tab\\9 x";}',
  '.c::before{content:"line\\a two";}',
  '.c::before{content:"line\\a two";white-space:pre;}',
  '.c::before{content:"q";}span.c::before{content:"more specific";}',
  'span.c::before{content:"more specific";}.c::before{content:"q";}',
  '.c::before{content:"important" !important;}span.c::before{content:"q";}',
  '#my-svg .c::before{content:"id";}.c::before{content:"q";}',
  '.c{--x:"from a variable";}.c::before{content:var(--x);}',
  '.c::before{--x:"on the pseudo-element";content:var(--x);}',
  '.c::before{content:var(--undefined);}',
  '.c::before{content:"old";content:var(--undefined);}',
  '.c::before{content:var(--undefined, "fallback");}',
  '.c{display:inline-block;}.c::before{content:"in a box ";}',
  '.c{font-size:20px;}.c::before{content:"big ";}',
  '.c::before{content:"wide";padding:0 5px;}',
  '.c::before{content:"x";margin-right:8px;}',
  '.c::before{content:"U";text-transform:lowercase;}',
  ':is(.a,.c)::before{content:"is ";}',
  '.c::before,.c::after{content:"*";}',
  '.nosuch::before{content:"no";}',
  '.c::first-line{content:"x";}.c::marker{content:"m";}',
  '.c::before{content:attr(title);}',
  '.c::before{content:"t:" attr(title) ".";}',
  '.c::before{content:attr(nosuch);}',
];
for (const rule of pseudoRules) {
  add('pseudo', svg(ARIAL, 14, label('node', divStyles.cell, SPAN), rule.replace(/(^|\})([^{}]+)\{/g, (m, a, sel) => `${a}${sel.split(',').map((x) => '#my-svg ' + x.trim().replace(/^#my-svg /, '')).join(',')}{`)), pseudoQueries);
}

// 11. Math functions in a length: the width of an inline block, and the
// font size and the line height of a text.
const calcQueries = [
  { sel: '#p', op: 'rect' },
  { sel: '.b', op: 'rect' },
  { sel: '.b', op: 'style', arg: 'width' },
  { sel: '.b', op: 'style', arg: 'font-size' },
  { sel: '.b', op: 'style', arg: 'line-height' },
];
const calcLengths = [
  'calc(10px + 5px)', 'calc(10px - 5px)', 'calc(2 * 10px)', 'calc(10px * 2)', 'calc(10px / 4)', 'calc(1em + 2px)',
  'calc(20 / 16 * 1em)', 'calc((6 / 20 - 0.375) * 1em + 10px)', 'calc(1 / 3 * 10px)', 'calc(0.1px * 3)', 'calc(3 * 0.1em)',
  'calc(1px + 2px * 3)', 'calc((1px + 2px) * 3)', 'calc(10px+5px)', 'calc(10px + 5)', 'calc(10px * 5px)', 'calc(10 / 2px)',
  'calc(10px / 0)', 'calc(1e1px + 1px)', 'calc(-5px + 20px)', 'calc( 10px )', 'calc(calc(5px) + calc(2 * 3px))',
  'min(10px, 2em)', 'max(10px, 2em)', 'clamp(5px, 2em, 20px)', 'clamp(5px, 1px, 20px)', 'calc(min(10px, 20px) + max(1px, 2px))',
  'calc(1in / 2)', 'calc(1pt * 3)', 'calc(1rem + 1px)', 'calc(1ex + 1px)', 'calc(1cm + 1mm)', 'calc(1pc - 1px)',
  'calc(50% - 10px)', 'calc(100%)', 'CALC(10PX + 5PX)', 'calc(10px + -5px)', 'calc(10px - -5px)', 'calc(5 * 2 * 1px)',
  'calc(2 * (3 + 4) * 1px)', 'calc(1px * 2 / 3)', 'calc(7px / 3 * 3)', 'calc(1.5em - 0.5em)', 'calc(1em * 1.1 + 1em * 0.1)',
  'calc(10px + )', 'calc()', 'calc(10px 5px)', 'calc(3)', 'calc(1px + (2px))', 'calc(1px + 2 * (3px + 1em) / 4)',
  'calc(-1 * 10px)', 'calc(0.333333px * 3)', 'calc(1em / 3)', 'calc(100px / 7)', 'calc(13.333px * 1.2)',
];
for (const v of calcLengths) {
  add('calc', svg(ARIAL, 14, label('node', divStyles.cell, `a <span class="b" style="display: inline-block; height: 10px; width: ${v}"></span> b`)), calcQueries);
}
const calcFontSizes = [
  'calc(20 / 16 * 1em)', 'calc(12 / 16 * 1em)', 'calc(10px + 0.5em)', 'calc(100% + 2px)', 'calc(50% * 3)', 'calc(1em / 3)',
  'calc(13.333px * 1.2)', 'calc(2ex + 1px)', 'calc(-5px)', 'calc(1rem - 2px)', 'max(1em, 20px)', 'calc(10px+1px)',
];
for (const v of calcFontSizes) {
  add('calc', svg(ARIAL, 14, label('node', divStyles.cell, `a <span class="b" style="display: inline-block; font-size: ${v}">text</span> b`)), calcQueries);
  add('calc', svg(ARIAL, 14, label('node', divStyles.cell, `a <span class="b" style="font-size: ${v}">text</span> b`)), calcQueries);
}
const calcLineHeights = ['calc(1 / 20 * 1em)', 'calc(1em + 4px)', 'calc(3 / 2)', 'calc(1.5 * 1)', 'calc(100% + 4px)', 'calc(2 * 10px)', 'calc(0.5em)', 'max(10px, 2em)'];
for (const v of calcLineHeights) {
  add('calc', svg(ARIAL, 14, label('node', divStyles.cell, `a <span class="b" style="display: inline-block; line-height: ${v}">text</span> b`)), calcQueries);
  add('calc', svg(ARIAL, 14, label('node', divStyles.cell, `a <span class="b" style="display: inline-block; font-size: 20px; line-height: ${v}">text</span> b`)), calcQueries);
}
for (const v of ['calc(1px + 2px)', 'calc(-0.25em)', 'calc((6 / 20 - 0.375) * 1em)', 'calc(50% - 2px)', 'calc(1em / 3)']) {
  add('calc', svg(ARIAL, 14, label('node', divStyles.cell, `a <span class="b" style="display: inline-block; width: 20px; height: 10px; vertical-align: ${v}"></span> b`)), calcQueries);
}

// The oracle reads the cases from a file of the repository, which the
// container of the Linux oracle sees at the same path. The test data is the
// same text with gzip.
const work = path.join(repoRoot, '.parity/linux/work');
fs.mkdirSync(work, { recursive: true });
fs.mkdirSync(outDir, { recursive: true });
const gzip = (file, data) => fs.writeFileSync(path.join(outDir, file), zlib.gzipSync(data, { level: 9 }));
const casesFile = path.join(work, 'pagesheets-cases.json');
const casesText = JSON.stringify({ cases }) + '\n';
fs.writeFileSync(casesFile, casesText);
gzip('pagesheets-cases.json.gz', casesText);
console.error(`${cases.length} cases, ${cases.reduce((n, c) => n + c.queries.length, 0)} queries`);

const args = process.argv.slice(2);
const mac = args.length === 0 || args.includes('--mac');
const linux = args.length === 0 || args.includes('--linux');
if (mac) {
  const raw = path.join(work, 'pagesheets-results-mac.json');
  execFileSync(
    process.execPath,
    [path.join(here, 'oracle.mjs'), 'eval', probe, '--bare', '--webfonts', '--args', casesFile, '--out', raw],
    { stdio: 'inherit' }
  );
  gzip('pagesheets-results.json.gz', fs.readFileSync(raw));
}
if (linux) {
  const raw = path.join(work, 'pagesheets-results-linux.json');
  execFileSync(
    process.execPath,
    [path.join(here, 'oracle-linux.mjs'), '--fonts', 'liberation', 'eval', probe, '--bare', '--webfonts', '--args', casesFile, '--out', raw],
    { stdio: 'inherit' }
  );
  gzip('linux-pagesheets-results.json.gz', fs.readFileSync(raw));
}
