// Writes internal/browser/testdata/cssvar-cases.json and
// cssvar-results.json: DOM probe cases for the CSS function var() and what
// Chromium measures for them.
//
// The cases cover a reference to a custom property that is not defined (the
// declaration is then invalid at computed-value time and acts as `unset`),
// a custom property of the element or of an ancestor, the fallback value,
// chains and cycles, shorthands, the style attribute and presentation
// attributes. The gitGraph style sheet of mermaid has
// `font-family: var(--mermaid-font-family)` with the property defined on
// `#id :root`, which matches no element.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-cssvar-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

const ROOT = `#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}`;
const svg = (css, inner) =>
  `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg"><style>${ROOT}${css}</style>${inner}</svg>`;
const TEXT = '<g class="p"><text id="t" class="c" y="20">Branch main 123</text></g>';
const textQueries = [
  { sel: '#t', op: 'textlen' },
  { sel: '#t', op: 'bbox' },
  { sel: '#t', op: 'style', arg: 'font-size' },
  { sel: '#t', op: 'style', arg: 'font-weight' },
];

const cases = [];
const add = (kind, html, queries = textQueries) => cases.push({ name: `cssvar-${kind}-${cases.length}`, html, queries });
const text = (kind, css, inner = TEXT) => add(kind, svg(css, inner));

// A property that is not defined: the inherited value, not the earlier
// declaration and not the text of the value.
text('undefined', '.c{font-family:var(--undef);}');
text('undefined', '.c{font-family:courier;font-family:var(--undef);}');
text('undefined', '.p{font-family:courier;}.c{font-family:verdana;font-family:var(--undef);}');
text('undefined', '.c{font-family:var(--undef);font-family:courier;}');
text('undefined', '.c{font-family:var(--undef);}.c{font-family:courier;}');
text('undefined', '.c{font-family:courier;}.c{font-family:var(--undef);}');
text('undefined', '#t{font-family:courier;}.c{font-family:var(--undef);}');
text('undefined', '.p{font-size:24px;}.c{font-size:10px;font-size:var(--undef);}');
text('undefined', '.p{font-weight:bold;}.c{font-weight:normal;font-weight:var(--undef);}');
text('undefined', '.c{font-family:courier,var(--undef);}');
text('undefined', '.p{font-family:courier;font-size:20px;}.c{font:var(--undef);}');
// The style sheet of mermaid: the property is defined on an element that
// does not exist.
text('mermaid', '#my-svg .c{font-family:\'trebuchet ms\',verdana,arial,sans-serif;font-family:var(--mermaid-font-family);}#my-svg :root{--mermaid-font-family:courier;}');
text('mermaid', '.p{font-family:courier;}#my-svg .c{font-family:\'trebuchet ms\',verdana,arial,sans-serif;font-family:var(--mermaid-font-family);}#my-svg :root{--mermaid-font-family:verdana;}');
// A property that is defined.
text('defined', '#my-svg{--f:courier;}.c{font-family:var(--f);}');
text('defined', '.p{--f:courier;}.c{font-family:var(--f);}');
text('defined', '.c{--f:courier;font-family:var(--f);}');
text('defined', '.c{font-family:var(--f);}.c{--f:courier;}');
text('defined', '.c{font-family:var(--f);--f:courier;}');
text('defined', ':root{--f:courier;}.c{font-family:var(--f);}');
text('defined', '#my-svg{--f:verdana;}.p{--f:courier;}.c{font-family:var(--f);}');
text('defined', '#my-svg{--f:"trebuchet ms", courier;}.c{font-family:var(--f);}');
text('defined', '#my-svg{--f: courier ;}.c{font-family:var( --f );}');
text('defined', '#my-svg{--f:courier;}.c{font-family:VAR(--f);}');
text('defined', '#my-svg{--F:courier;}.c{font-family:verdana;font-family:var(--f);}');
text('defined', '#my-svg{--f:courier;}.c{font-family:var(--f) !important;}#t{font-family:verdana;}');
text('defined', '#my-svg{--f:verdana !important;}.p{--f:courier;}.c{font-family:var(--f);}');
text('defined', '#my-svg{--s:22px;--w:bold;}.c{font-size:var(--s);font-weight:var(--w);}');
text('defined', '#my-svg{--f:courier;}.c{font-family:nosuchfont,var(--f);}');
// The fallback value.
text('fallback', '.c{font-family:var(--undef, courier);}');
text('fallback', '.c{font-family:var(--undef,courier);}');
text('fallback', '#my-svg{--f:verdana;}.c{font-family:var(--f, courier);}');
text('fallback', '.c{font-family:var(--undef, "trebuchet ms", courier);}');
text('fallback', '.c{font-family:var(--a, var(--b, courier));}');
text('fallback', '#my-svg{--b:verdana;}.c{font-family:var(--a, var(--b, courier));}');
text('fallback', '.c{font-size:var(--undef, 12px);font-weight:var(--undef2, bold);}');
text('fallback', '.c{font-family:var(--mermaid-usecase-font-family, "trebuchet ms", verdana, arial, sans-serif);font-size:var(--mermaid-usecase-font-size, 12px);font-weight:var(--mermaid-usecase-font-weight, normal);}');
text('fallback', '.p{font-family:courier;}.c{font-family:verdana;font-family:var(--undef,);}');
// Chains and cycles.
text('chain', '#my-svg{--a:var(--b);--b:courier;}.c{font-family:var(--a);}');
text('chain', '#my-svg{--b:courier;}.p{--a:var(--b);}.c{font-family:var(--a);}');
text('chain', '#my-svg{--a:var(--b);}.p{--b:courier;}.c{font-family:verdana;font-family:var(--a);}');
text('chain', '#my-svg{--a:var(--b, courier);}.c{font-family:var(--a);}');
text('cycle', '#my-svg{--a:var(--b);--b:var(--a);}.p{font-family:courier;}.c{font-family:verdana;font-family:var(--a);}');
text('cycle', '#my-svg{--a:var(--a);}.p{font-family:courier;}.c{font-family:verdana;font-family:var(--a);}');
text('cycle', '#my-svg{--a:var(--b);--b:var(--a);}.c{font-family:var(--a, courier);}');
// CSS-wide keywords in a custom property.
text('keyword', '.p{--f:courier;font-family:verdana;}.c{--f:initial;font-family:courier;font-family:var(--f);}');
text('keyword', '.p{--f:courier;}.c{--f:inherit;font-family:var(--f);}');
text('keyword', '.p{--f:courier;}.c{--f:unset;font-family:var(--f);}');
text('keyword', '#my-svg{--e:;}.p{font-family:courier;}.c{font-family:verdana;font-family:var(--e);}');
// Shorthands.
text('shorthand', '#my-svg{--fs:bold 20px courier;}.c{font:var(--fs);}');
text('shorthand', '#my-svg{--fs:bold 20px courier;}.c{font-size:30px;font:var(--fs);}');
text('shorthand', '#my-svg{--fs:bold 20px courier;}.c{font:var(--fs);font-size:30px;}');
text('shorthand', '#my-svg{--fam:courier;}.c{font:italic 18px var(--fam);}');
// The display property does not inherit: the initial value. Only the
// bounding box is read here: the text length of an element that is not
// displayed is a different subject.
const bboxQueries = [{ sel: '#t', op: 'bbox' }];
add('display', svg('.c{display:none;display:var(--undef);}', TEXT), bboxQueries);
add('display', svg('#my-svg{--d:none;}.c{display:var(--d);}', TEXT), bboxQueries);
add('display', svg('.p{display:none;}.c{display:var(--undef);}', TEXT), bboxQueries);
add('display', svg('.p{--d:none;}.c{display:var(--undef, var(--d));}', TEXT), bboxQueries);
// The style attribute and presentation attributes.
text('inline', '#my-svg{--f:courier;}', '<g class="p"><text id="t" y="20" style="font-family: var(--f)">Branch main 123</text></g>');
text('inline', '', '<g class="p"><text id="t" y="20" style="--f: courier; font-family: var(--f)">Branch main 123</text></g>');
text('inline', '.p{font-family:courier;}', '<g class="p"><text id="t" y="20" style="font-family: verdana; font-family: var(--undef)">Branch main 123</text></g>');
text('inline', '.c{font-family:var(--f);}', '<g class="p" style="--f: courier"><text id="t" class="c" y="20">Branch main 123</text></g>');
text('attr', '#my-svg{--f:courier;--s:22px;}', '<g class="p"><text id="t" y="20" font-family="var(--f)" font-size="var(--s)">Branch main 123</text></g>');
text('attr', '.p{font-family:courier;}', '<g class="p"><text id="t" y="20" font-family="var(--undef)">Branch main 123</text></g>');
text('tspan', '#my-svg{--f:courier;}.r{font-family:var(--f);}.u{font-family:var(--undef);}', '<g class="p"><text id="t" y="20">Branch <tspan class="r">main</tspan> <tspan class="u">123</tspan></text></g>');
// HTML content.
const html = (css, inner) =>
  svg(css, `<foreignObject id="fo" width="500" height="500"><div xmlns="http://www.w3.org/1999/xhtml" class="p">${inner}</div></foreignObject>`);
const htmlQueries = [
  { sel: '#d', op: 'rect' },
  { sel: '#d', op: 'style', arg: 'font-size' },
];
add('html', html('#my-svg{--f:courier;--s:20px;}.c{display:inline-block;font-family:var(--f);font-size:var(--s);}', '<div id="d" class="c">Branch main 123</div>'), htmlQueries);
add('html', html('.p{font-family:courier;font-size:20px;}.c{display:inline-block;font-family:verdana;font-family:var(--undef);font-size:var(--undef);}', '<div id="d" class="c">Branch main 123</div>'), htmlQueries);
add('html', html('#my-svg{--pad:7px;--w:120px;}.c{display:inline-block;padding:var(--pad);width:var(--w);}', '<div id="d" class="c">Branch</div>'), htmlQueries);
add('html', html('.c{display:inline-block;padding:5px;padding:var(--undef);width:120px;width:var(--undef, 90px);}', '<div id="d" class="c">Branch</div>'), htmlQueries);
add('html', html('#my-svg{--pad:3px 11px;}.c{display:inline-block;padding:var(--pad);padding-left:1px;}', '<div id="d" class="c">Branch</div>'), htmlQueries);

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'cssvar-cases.json');
const resultsFile = path.join(outDir, 'cssvar-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
