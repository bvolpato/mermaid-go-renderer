// Writes internal/browser/testdata/svgviewport-cases.json and
// svgviewport-results.json: DOM probe cases for SVG lengths that are a
// percentage or that are very large, and for the weight of b and strong,
// and what Chromium measures for them.
//
//   - A percentage in x, y, width, height and the like refers to the
//     nearest svg element: to its viewBox when it has one, else to its
//     size (SVGLengthContext). The venn diagram places its title at
//     x="50%". The outermost svg of the page is 784 x 150 while mermaid
//     renders.
//   - Blink keeps a length as a CSS length, which has a largest value:
//     33554428 as a float (kMaxValueForCssLength). The gantt diagram draws
//     the marker of today at x = 117550174 when the chart is in another
//     century, and the box of the line is at 33554428.
//   - b and strong are bolder than their parent, not bold: in a bold
//     parent they have the weight 900.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-svgviewport-probe.mjs
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runProbe } from './probe-run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

const cases = [];
const shapeQueries = [{ sel: '#t', op: 'bbox' }, { sel: '#t', op: 'rect' }, { sel: '#my-svg', op: 'bbox' }];

// Percentages. Each root is an svg element as mermaid has it while it
// renders, or with a size or a viewBox.
const roots = {
  render: 'width="100%"',
  sized: 'width="400" height="300"',
  viewBox: 'width="100%" viewBox="0 0 1000 500"',
  'sized-viewBox': 'width="400" height="300" viewBox="10 20 800 200"',
  'max-width': 'width="100%" style="max-width: 300px" viewBox="0 0 600 400"',
};
const shapes = {
  'text-x': '<text id="t" x="50%" y="16" text-anchor="middle" style="font: 16px arial">Fruits and Vegetables</text>',
  'text-xy': '<text id="t" x="25%" y="50%" style="font: 16px arial">abc</text>',
  'text-dx': '<text id="t" x="10" y="20" dx="10%" dy="10%" style="font: 16px arial">abc</text>',
  tspan: '<text id="t" y="20" style="font: 16px arial">ab<tspan x="50%" dy="20%">cd</tspan></text>',
  rect: '<rect id="t" x="10%" y="20%" width="50%" height="50%"/>',
  'rect-rx': '<rect id="t" x="1" y="2" width="33.3%" height="12.5%" rx="5%" ry="5%"/>',
  circle: '<circle id="t" cx="50%" cy="50%" r="10%"/>',
  ellipse: '<ellipse id="t" cx="25%" cy="75%" rx="10%" ry="10%"/>',
  line: '<line id="t" x1="0" y1="50%" x2="100%" y2="75%"/>',
  fo: '<foreignObject id="t" x="10%" y="10%" width="50%" height="50%"><div xmlns="http://www.w3.org/1999/xhtml">hi</div></foreignObject>',
  'in-g': '<g transform="translate(10, 20) scale(2)"><rect id="t" x="10%" y="20%" width="50%" height="50%"/></g>',
  nested: '<svg x="5" y="6" width="200" height="100"><rect id="t" x="10%" y="20%" width="50%" height="50%"/></svg>',
  'nested-viewBox': '<svg x="5" y="6" width="200" height="100" viewBox="0 0 400 50"><rect id="t" x="10%" y="20%" width="50%" height="50%"/></svg>',
  'nested-percent': '<svg x="10%" y="10%" width="50%" height="50%"><circle id="t" cx="50%" cy="50%" r="25%"/></svg>',
};
for (const [rootName, attrs] of Object.entries(roots)) {
  for (const [shapeName, shape] of Object.entries(shapes)) {
    cases.push({ name: `svgviewport-percent-${rootName}-${shapeName}`, html: `<svg id="my-svg" ${attrs} xmlns="http://www.w3.org/2000/svg">${shape}</svg>`, queries: shapeQueries });
  }
}

// Very large lengths. The cases have no foreignObject: its rectangle on the
// page is a sum of LayoutUnits, which stops at the largest LayoutUnit, and
// the engine does not have that limit.
const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const large = {
  today: '<g id="t" class="today"><line x1="117550174" x2="117550174" y1="25" y2="123" class="today"></line></g>',
  'line-x': '<line id="t" x1="117550174" x2="117550174" y1="25" y2="123"/>',
  'line-y': '<line id="t" x1="5" x2="6" y1="-117550174" y2="4000000000"/>',
  'line-max': '<line id="t" x1="33554428" x2="33554429" y1="33554430" y2="33554432"/>',
  'line-under': '<line id="t" x1="33554426" x2="33554427" y1="-33554428" y2="-33554430"/>',
  'line-min': '<line id="t" x1="-33554431" x2="-33554432" y1="-33554433" y2="-1e12"/>',
  rect: '<rect id="t" x="1000000000" y="5" width="1000000000" height="10"/>',
  'rect-wide': '<rect id="t" x="5" y="5" width="1000000000" height="1e10"/>',
  circle: '<circle id="t" cx="-1000000000" cy="5" r="500000000"/>',
  text: '<text id="t" x="1000000000" y="-1000000000" style="font: 16px arial">abc</text>',
  moved: '<g transform="translate(-100000000, 0)"><line id="t" x1="117550174" x2="117550174" y1="25" y2="123"/></g>',
  polyline: '<polyline id="t" points="117550174,25 117550175,123"/>',
  path: '<path id="t" d="M117550174,25L117550174,123"/>',
  em: '<line id="t" x1="10000000em" x2="5" y1="25" y2="123" style="font-size: 16px"/>',
};
for (const [name, inner] of Object.entries(large)) {
  cases.push({ name: `svgviewport-large-${name}`, html: svg(inner), queries: shapeQueries });
}

// transform-origin: the milestone of a gantt chart is a rect with the CSS
// transform rotate(45deg) scale(0.8,0.8) about its center.
const origins = {
  milestone: '<g id="t"><rect x="75" y="122" width="20" height="20" transform-origin="85px 132px" style="transform: rotate(45deg) scale(0.8,0.8)"></rect></g>',
  'milestone-task': '<g id="t"><rect x="75" y="122" width="101" height="20" transform-origin="125.5px 132px"></rect><rect x="219" y="170" width="20" height="20" transform-origin="229px 180px" style="transform: rotate(45deg) scale(0.8,0.8)"></rect></g>',
  attribute: '<g id="t"><rect x="10" y="20" width="30" height="40" transform="rotate(30)" transform-origin="25px 40px"/></g>',
  'attribute-scale': '<g id="t"><rect x="10" y="20" width="30" height="40" transform="scale(2, 0.5)" transform-origin="25 40"/></g>',
  'style-origin': '<g id="t"><rect x="10" y="20" width="30" height="40" style="transform-origin: 10px 20px; transform: scale(2)"/></g>',
  'no-transform': '<g id="t"><rect x="10" y="20" width="30" height="40" transform-origin="25px 40px"/></g>',
  translate: '<g id="t"><rect x="10" y="20" width="30" height="40" transform="translate(5.5, 6.25)" transform-origin="25px 40px"/></g>',
  'on-g': '<g id="t"><g transform="rotate(90)" transform-origin="50px 50px"><rect x="10" y="20" width="30" height="40"/></g></g>',
  decimals: '<g id="t"><rect x="10.1" y="20.2" width="30.3" height="40.4" transform="rotate(33.3) scale(1.1)" transform-origin="25.25px 40.4px"/></g>',
  matrix: '<g id="t"><rect x="10" y="20" width="30" height="40" transform="matrix(0.866,0.5,-0.5,0.866,10,20)" transform-origin="25px 40px"/></g>',
};
for (const [name, inner] of Object.entries(origins)) {
  cases.push({ name: `svgviewport-origin-${name}`, html: svg(inner), queries: [{ sel: '#t', op: 'bbox' }, { sel: '#t rect', op: 'rect' }, { sel: '#my-svg', op: 'bbox' }] });
}

// The CSS properties scale, translate and rotate. A highlighted slice of a
// pie chart has scale: 1.05. They apply before the transform property:
// translate, then rotate, then scale, then transform, all about the
// transform-origin.
const individual = {
  'pie-slice': '<g id="t"><circle cx="0" cy="0" r="186"></circle><path d="M0,-185A185,185,0,0,1,155.632,100.019L0,0Z" style="scale: 1.05"></path></g>',
  scale: '<g id="t"><rect x="10" y="20" width="30" height="40" style="scale: 1.05"/></g>',
  'scale-xy': '<g id="t"><rect x="10" y="20" width="30" height="40" style="scale: 2 0.5"/></g>',
  'scale-percent': '<g id="t"><rect x="10" y="20" width="30" height="40" style="scale: 150%"/></g>',
  translate: '<g id="t"><rect x="10" y="20" width="30" height="40" style="translate: 5.5px 6.25px"/></g>',
  'translate-x': '<g id="t"><rect x="10" y="20" width="30" height="40" style="translate: 7px"/></g>',
  rotate: '<g id="t"><rect x="10" y="20" width="30" height="40" style="rotate: 30deg"/></g>',
  all: '<g id="t"><rect x="10" y="20" width="30" height="40" style="translate: 5px 6px; rotate: 30deg; scale: 1.5"/></g>',
  'with-transform': '<g id="t"><rect x="10" y="20" width="30" height="40" transform="translate(3, 4)" style="scale: 2"/></g>',
  'with-css-transform': '<g id="t"><rect x="10" y="20" width="30" height="40" style="scale: 2; transform: rotate(45deg)"/></g>',
  'with-origin': '<g id="t"><rect x="10" y="20" width="30" height="40" style="scale: 2; transform-origin: 25px 40px"/></g>',
  'on-g': '<g id="t"><g style="scale: 0.5"><rect x="10" y="20" width="30" height="40"/></g></g>',
  none: '<g id="t"><rect x="10" y="20" width="30" height="40" style="scale: none; rotate: none; translate: none"/></g>',
};
for (const [name, inner] of Object.entries(individual)) {
  cases.push({ name: `svgviewport-individual-${name}`, html: svg(inner), queries: [{ sel: '#t', op: 'bbox' }, { sel: '#t > :last-child', op: 'rect' }, { sel: '#my-svg', op: 'bbox' }] });
}

// The weight of b and strong, and of the other elements that are bold.
const weightQueries = [{ sel: '#t', op: 'style', arg: 'font-weight' }, { sel: '#t', op: 'rect' }];
const fo = (inner) => svg(`<foreignObject width="2000" height="200"><div xmlns="http://www.w3.org/1999/xhtml" style="font: 16px arial; white-space: nowrap; display: inline-block">${inner}</div></foreignObject>`);
for (const weight of ['normal', 'bold', '100', '300', '400', '500', '600', '700', '800', '900', 'lighter', 'bolder']) {
  for (const tag of ['b', 'strong']) {
    cases.push({ name: `svgviewport-weight-${tag}-${weight}`, html: fo(`<span style="font-weight: ${weight}">some <${tag} id="t">bold text</${tag}> here</span>`), queries: weightQueries });
  }
}
cases.push({ name: 'svgviewport-weight-nested', html: fo('<b>a <strong>b <b id="t">bold text</b></strong></b>'), queries: weightQueries });
cases.push({ name: 'svgviewport-weight-label', html: fo('<div style="font-weight: bold; display: table-cell"><span style="; font-weight: bold;" class="nodeLabel"><p><strong id="t">my bolded name</strong></p></span></div>'), queries: weightQueries });
cases.push({ name: 'svgviewport-weight-h1', html: fo('<h1 id="t" style="margin: 0; font-size: 16px">bold text</h1>'), queries: weightQueries });
cases.push({ name: 'svgviewport-weight-h1-b', html: fo('<h1 style="margin: 0; font-size: 16px"><b id="t">bold text</b></h1>'), queries: weightQueries });
cases.push({ name: 'svgviewport-weight-th', html: fo('<span style="font-weight: 300"><th id="t">bold text</th></span>'), queries: weightQueries });

runProbe('svgviewport', cases);
