// Writes internal/browser/testdata/brspace-cases.json and
// brspace-results.json: DOM probe cases for a collapsible space at the end
// of a line that a br element ends, and what Chromium measures for them.
//
// CSS Text 3, 4.1.2: a sequence of collapsible spaces at the end of a line
// is removed. A br element ends the line, so the space in front of it does
// not count for the width of the line, whether the space is in the same
// text, in an inline element or in a text of its own. With a white-space
// value that preserves spaces (break-spaces, pre-wrap, pre) the space stays.
// The label of a state transition such as "long line using <br>should work"
// depends on it: mermaid sizes the label with the rectangle of the div.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-brspace-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const fonts = ['font-family:courier;font-size:14px', 'font-family:arial;font-size:16px', "font-family:'trebuchet ms',verdana,arial,sans-serif;font-size:16px"];
const divStyles = [
  // The styles of createText for a label that fits and for one that wraps.
  'display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;',
  'display: table; white-space: break-spaces; line-height: 1.5; max-width: 200px; text-align: center; width: 200px;',
  'display: inline-block; white-space: nowrap;',
  'display: inline-block;',
  'display: inline-block; width: 70px;',
  'display: inline-block; white-space: pre-line;',
  'display: inline-block; white-space: pre-wrap;',
  'display: inline-block; white-space: pre;',
];
const inners = [
  '<span class="edgeLabel"><p>long line using <br>should work</p></span>',
  '<span class="edgeLabel"><p>long line using<br>should work</p></span>',
  '<span><p>long line using<br> should work</p></span>',
  '<span><p>long line using <br> should work</p></span>',
  '<span><p>aaaa   <br>   b</p></span>',
  '<span>aaaa <br>b</span>',
  'aaaa <br>b',
  '<span><p>aaaa <b>bold </b><br>c</p></span>',
  '<span><p>aaaa <b>bold</b> <br>c</p></span>',
  '<span><p>aaaa <i>it </i> <br>c</p></span>',
  '<span><p>aaaa <br><br>b</p></span>',
  '<span><p>aaaa <br> <br>b</p></span>',
  '<span><p> <br>bbbb</p></span>',
  '<span><p>aaaa <br></p></span>',
  '<span><p>aaaa&nbsp;<br>b</p></span>',
  '<span><p>aaaa\n<br>b</p></span>',
  '<span><p>aaaa\t<br>b</p></span>',
  '<span><p>aaaa <br>bbbbbbbbbb</p></span>',
  '<span><p>a <br>b <br>cccc </p></span>',
  '<span><p>aaaa <span> </span><br>b</p></span>',
  '<span><p>aaaa<span> </span><br>b</p></span>',
  '<span><p>aaaa <!-- c --><br>b</p></span>',
  '<span><p>aaaa <br>b</p><p>cccccc <br>d</p></span>',
];
const cases = [];
const queries = [{ sel: '#d', op: 'rect' }];
inners.forEach((inner, i) => {
  divStyles.forEach((style, k) => {
    // A tab that is preserved goes to the next tab stop. That is another
    // rule, so the tab is here only where it is a collapsible space.
    if (inner.includes('\t') && /pre-wrap|pre;/.test(style)) {
      return;
    }
    const font = fonts[(i + k) % fonts.length];
    cases.push({
      name: `brspace-${i}-s${k}`,
      html: svg(
        `<g transform="translate(20.5, 30.25)"><foreignObject id="fo" width="2000px" height="2000px"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="${style}${font}">${inner}</div></foreignObject></g>`
      ),
      queries,
    });
  });
});

fs.mkdirSync(outDir, { recursive: true });
const casesFile = path.join(outDir, 'brspace-cases.json');
const resultsFile = path.join(outDir, 'brspace-results.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(repoRoot, '.parity/probe/dom.js'), '--args', casesFile, '--out', resultsFile],
  { stdio: 'inherit' }
);
console.error(`${cases.length} cases`);
