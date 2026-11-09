// Writes internal/browser/testdata/fontmono-cases.json and
// fontmono-results.json: DOM probe cases for the font size of an element
// whose font family is monospace, and what Chromium measures for them.
//
// Chromium has two default font sizes: 16px, and 13px for a font family
// that is only `monospace`. When an element changes between the two kinds
// of family and its font size is not an absolute length, Blink scales the
// size (FontBuilder::CheckForGenericFamilyChange):
//
//   - A keyword size (the initial `medium` too) is read again from the
//     table of the other kind: code in a page with no font size is 13px.
//   - Another relative size is multiplied by 13/16, or divided by it on
//     the way back: font-size: 1.5em with font-family: monospace is 19.5px.
//   - An absolute size, such as the 16px that mermaid sets on the svg
//     element, stays, and so does a size that depends on it.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-fontmono-probe.mjs
import { runProbe } from './probe-run.mjs';

const svg = (inner, attrs = '') => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg"${attrs}>${inner}</svg>`;
const cases = [];
const textQueries = [
  { sel: '#t', op: 'style', arg: 'font-size' },
  { sel: '#t', op: 'style', arg: 'font-family' },
  { sel: '#t', op: 'textlen' },
  { sel: '#t', op: 'bbox' },
];
const htmlQueries = [
  { sel: '#t', op: 'style', arg: 'font-size' },
  { sel: '#t', op: 'style', arg: 'font-family' },
  { sel: '#t', op: 'rect' },
];

const sizes = ['', 'font-size:1.5em', 'font-size:120%', 'font-size:larger', 'font-size:smaller', 'font-size:16px', 'font-size:12pt', 'font-size:medium', 'font-size:large', 'font-size:x-small', 'font-size:xx-large', 'font-size:xxx-large', 'font-size:2ex', 'font-size:1rem'];
const families = ['font-family:monospace', "font-family:monospace, 'courier new'", 'font-family:courier', 'font-family:Monospace', 'font-family:serif', ''];
// The font of the parent: none (the default of the page), a relative size,
// an absolute size, a keyword, and monospace.
const parents = ['', 'font-size:20px', 'font-size:1.25em', 'font-size:large', 'font-family:monospace', 'font-family:monospace;font-size:20px', 'font-family:monospace;font-size:150%', "font-family:'trebuchet ms',verdana,arial,sans-serif;font-size:16px"];

let n = 0;
for (const parent of parents) {
  for (const family of families) {
    for (const size of sizes) {
      // Not every combination: a third of them, in a fixed pattern.
      if (n++ % 3 !== 0) {
        continue;
      }
      const style = [family, size].filter(Boolean).join(';');
      cases.push({ name: `fontmono-text-${cases.length}`, html: svg(`<g style="${parent}"><text id="t" x="10" y="40" style="${style}">code() text</text></g>`), queries: textQueries });
      cases.push({
        name: `fontmono-html-${cases.length}`,
        html: svg(`<foreignObject width="2000" height="300"><div xmlns="http://www.w3.org/1999/xhtml" style="white-space: nowrap; ${parent}"><span id="t" style="${style}">code() text</span></div></foreignObject>`),
        queries: htmlQueries,
      });
    }
  }
}
// The elements that are monospace in the style sheet of the user agent.
for (const parent of parents) {
  for (const tag of ['code', 'kbd', 'samp', 'tt', 'pre']) {
    cases.push({
      name: `fontmono-${tag}-${cases.length}`,
      html: svg(`<foreignObject width="2000" height="300"><div xmlns="http://www.w3.org/1999/xhtml" style="white-space: nowrap; ${parent}">a <${tag} id="t" style="margin: 0">code() text</${tag}> b</div></foreignObject>`),
      queries: htmlQueries,
    });
    cases.push({
      name: `fontmono-${tag}-in-${cases.length}`,
      html: svg(`<foreignObject width="2000" height="300"><div xmlns="http://www.w3.org/1999/xhtml" style="white-space: nowrap; ${parent}"><${tag} style="margin: 0">a <span id="t" style="font-family: arial">text</span> <b style="font-size: 1.5em">b</b></${tag}></div></foreignObject>`),
      queries: htmlQueries,
    });
  }
}
// A size on the svg element, as mermaid has it, and a presentation attribute.
cases.push({ name: 'fontmono-root-size', html: svg('<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;}</style><foreignObject width="2000" height="300"><div xmlns="http://www.w3.org/1999/xhtml">a <code id="t">code() text</code></div></foreignObject>'), queries: htmlQueries });
cases.push({ name: 'fontmono-attr-size', html: svg('<g font-size="20"><text id="t" x="10" y="40" font-family="monospace">code() text</text></g>'), queries: textQueries });
cases.push({ name: 'fontmono-attr-family', html: svg('<text id="t" x="10" y="40" font-family="monospace">code() text</text>'), queries: textQueries });
cases.push({ name: 'fontmono-nested', html: svg('<g style="font-family:monospace"><g style="font-family:serif"><g style="font-family:monospace;font-size:2em"><text id="t" x="10" y="40">code() text</text></g></g></g>'), queries: textQueries });

runProbe('fontmono', cases);
