// Writes internal/browser/testdata/norender-cases.json and
// norender-results.json: DOM probe cases for SVG elements that have no
// layout object, and what Chromium measures for them.
//
// An element with no layout object has no box: getBBox and
// getBoundingClientRect return 0, 0, 0, 0, getComputedTextLength returns 0,
// and its container does not count it. The cases show when an SVG element
// has no layout object:
//
//   - It has display: none, or an ancestor has it.
//   - It is a child of a switch element and is not the first child that is
//     valid (SVGSwitchElement::ChildShouldCreateLayoutObject). The journey
//     diagram puts a foreignObject and a text in a switch: only the
//     foreignObject has a box.
//   - Its conditional processing attributes do not pass (SVGTests::IsValid):
//     requiredExtensions with a value, or systemLanguage with no value of
//     two letters that starts the language of the browser (en-US).
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-norender-probe.mjs
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runProbe } from './probe-run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const fo = (attrs = '') => `<foreignObject id="f" x="150" y="50" width="120" height="50"${attrs}><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="font: 14px arial">Onboarding</div></foreignObject>`;
const text = (attrs = '') => `<text id="x" x="20" y="75" style="font: 14px arial"${attrs}><tspan id="s" x="20" dy="0">Onboarding</tspan></text>`;
const rect = (attrs = '') => `<rect id="r" x="5" y="6" width="30" height="40"${attrs}/>`;
const circle = (attrs = '') => `<circle id="c" cx="300" cy="300" r="10"${attrs}/>`;

// Each entry is the content of a group #t.
const contents = {
  // The switch of the journey diagram, and its children in the other order.
  'switch-fo-text': `<switch id="w">${fo()}${text()}</switch>`,
  'switch-text-fo': `<switch id="w">${text()}${fo()}</switch>`,
  'switch-rect-circle': `<switch id="w">${rect()}${circle()}</switch>`,
  'switch-three': `<switch id="w">${rect()}${circle()}${text()}</switch>`,
  'switch-g': `<switch id="w"><g id="g">${rect()}${circle()}</g>${text()}</switch>`,
  'switch-empty': `<switch id="w"></switch>${rect()}`,
  'switch-one': `<switch id="w">${text()}</switch>`,
  'switch-space': `<switch id="w">\n  ${rect()}\n  ${circle()}\n</switch>`,
  // The first child is not valid, or has no box.
  'switch-extension': `<switch id="w">${rect(' requiredExtensions="http://example.org/ext"')}${circle()}</switch>`,
  'switch-extension-xhtml': `<switch id="w">${fo(' requiredExtensions="http://www.w3.org/1999/xhtml"')}${text()}</switch>`,
  'switch-extension-empty': `<switch id="w">${rect(' requiredExtensions=""')}${circle()}</switch>`,
  'switch-features': `<switch id="w">${rect(' requiredFeatures="http://www.w3.org/TR/SVG11/feature#Extensibility"')}${circle()}</switch>`,
  'switch-lang-en': `<switch id="w">${rect(' systemLanguage="en"')}${circle()}</switch>`,
  'switch-lang-en-us': `<switch id="w">${rect(' systemLanguage="en-US"')}${circle()}</switch>`,
  'switch-lang-fr': `<switch id="w">${rect(' systemLanguage="fr"')}${circle()}</switch>`,
  'switch-lang-fr-en': `<switch id="w">${rect(' systemLanguage="fr, en"')}${circle()}</switch>`,
  'switch-lang-empty': `<switch id="w">${rect(' systemLanguage=""')}${circle()}</switch>`,
  'switch-display-none': `<switch id="w">${rect(' style="display: none"')}${circle()}</switch>`,
  'switch-empty-rect': `<switch id="w"><rect id="r"/>${circle()}</switch>`,
  'switch-title': `<switch id="w"><title>t</title>${rect()}${circle()}</switch>`,
  'switch-defs': `<switch id="w"><defs></defs>${rect()}${circle()}</switch>`,
  'switch-nested': `<switch id="w"><switch>${rect(' systemLanguage="fr"')}${circle()}</switch>${text()}</switch>`,
  'switch-in-switch-second': `<switch id="w">${rect()}<switch>${circle()}</switch></switch>`,
  'switch-moved': `<switch id="w" transform="translate(10, 20)">${rect()}${circle()}</switch>`,
  // The conditional attributes outside a switch.
  'extension': `${rect(' requiredExtensions="http://example.org/ext"')}${circle()}`,
  'lang-fr': `${rect(' systemLanguage="fr"')}${circle()}`,
  'lang-en': `${rect(' systemLanguage="en"')}${circle()}`,
  'lang-fr-text': `${text(' systemLanguage="fr"')}${circle()}`,
  'lang-fr-g': `<g id="g" systemLanguage="fr">${rect()}${text()}</g>${circle()}`,
  'lang-fr-fo': `${fo(' systemLanguage="fr"')}${circle()}`,
  // display: none.
  'none-text': `${text(' display="none"')}${circle()}`,
  'none-text-style': `<text id="x" x="20" y="75" style="display: none; font: 14px arial"><tspan id="s">Onboarding</tspan></text>${circle()}`,
  'none-tspan': `<text id="x" x="20" y="75" style="font: 14px arial">ab<tspan id="s" style="display: none">Onboarding</tspan>cd</text>${circle()}`,
  'none-g': `<g id="g" style="display: none">${rect()}${text()}${fo()}</g>${circle()}`,
  'none-g-moved': `<g id="g" style="display: none" transform="translate(40, 50)">${rect()}${text()}</g>${circle()}`,
  'none-rect': `${rect(' style="display: none"')}${circle()}`,
  'none-fo': `${fo(' style="display: none"')}${circle()}`,
  'none-in-fo': `<foreignObject id="f" x="150" y="50" width="120" height="50"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="display: none; font: 14px arial">Onboarding</div></foreignObject>${circle()}`,
  'hidden-text': `${text(' visibility="hidden"')}${circle()}`,
  'defs-rect': `<defs>${rect()}${text()}</defs>${circle()}`,
  'marker-rect': `<marker id="m">${rect()}</marker>${circle()}`,
  'symbol-rect': `<symbol id="sy">${rect()}</symbol>${circle()}`,
};
const selectors = { '#t': ['bbox'], '#w': ['bbox', 'rect'], '#g': ['bbox', 'rect'], '#r': ['bbox', 'rect'], '#c': ['bbox', 'rect'], '#x': ['bbox', 'rect', 'textlen'], '#s': ['bbox', 'textlen'], '#f': ['bbox', 'rect'], '#d': ['rect'], '#my-svg': ['bbox'] };

const cases = [];
for (const [name, inner] of Object.entries(contents)) {
  const queries = [];
  for (const [sel, ops] of Object.entries(selectors)) {
    if (sel === '#t' || sel === '#my-svg' || inner.includes(`id="${sel.slice(1)}"`)) {
      for (const op of ops) {
        queries.push({ sel, op });
      }
    }
  }
  cases.push({ name: `norender-${name}`, html: svg(`<g id="t">${inner}</g>`), queries });
  cases.push({ name: `norender-${name}-moved`, html: svg(`<g transform="translate(3.5, 4.25)"><g id="t">${inner}</g></g>`), queries });
}

runProbe('norender', cases);
