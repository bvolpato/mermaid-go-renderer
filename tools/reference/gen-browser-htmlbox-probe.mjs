// Writes internal/browser/testdata/htmlbox-cases.json and
// htmlbox-results.json: DOM probe cases for the boxes of HTML inside a
// foreignObject, and what Chromium measures for them.
//
// The cases have the structures that the labels of mermaid have: a div with
// the style of createText, a span, and inside it text, p, br, strong, em,
// i, small, code, sub and sup, empty elements, and text that wraps. Each case
// asks for the rectangle of the foreignObject and of every element in it.
//
// The rules of Blink that the cases show:
//
//   - The rectangle of an inline element is the union of the rectangles of
//     its text and of the boxes in it (LayoutInline::QuadsForSelf with a
//     culled inline box). The rectangle of a text has the height of the
//     ascent plus the descent of its font, not the height of the line.
//   - An inline element with no content has an empty box on its line.
//   - A br has the box of a text with no width.
//   - A block in an inline element (the p in the span of a label) is a line
//     of its own, and the inline element has its rectangle.
//
// The script runs .parity/probe/dom.js with the eval command of the browser
// oracle, one time.
//
// Usage (in tools/reference): node gen-browser-htmlbox-probe.mjs
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { runProbe } from './probe-run.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const cases = [];
const queries = [{ sel: '#fo, #fo *', op: 'rect', all: true }];
const add = (kind, fo) => cases.push({ name: `htmlbox-${kind}-${cases.length}`, html: svg(fo), queries });

const fonts = [
  'font-family:arial;font-size:16px',
  'font-family:arial;font-size:14px',
  'font-family:arial;font-size:32.5px',
  "font-family:'trebuchet ms',verdana,arial,sans-serif;font-size:16px",
  'font-family:times;font-size:16px',
  'font-family:courier;font-size:14px',
  'font-family:verdana;font-size:13px',
  'font-family:georgia;font-size:15px',
  'font-family:helvetica;font-size:11px',
  '',
];
const divStyles = [
  'display: table-cell; white-space: nowrap; line-height: 1.5; max-width: 200px; text-align: center;',
  'display: table; white-space: break-spaces; line-height: 1.5; max-width: 200px; text-align: center; width: 200px;',
  'display: table-cell; white-space: nowrap; max-width: 200px; text-align: center;',
  'display: inline-block; white-space: nowrap;',
  '',
  'line-height: 20px; width: 120px;',
];
const inners = [
  'Plain text',
  '<span>Span text</span>',
  '<span class="nodeLabel"><p>Paragraph</p></span>',
  '<span>a <b>bold</b> and <i>italic</i> c</span>',
  '<span><strong>strong</strong> <em>em</em></span>',
  '<span>before<p>Paragraph</p>after</span>',
  '<span><p>one<br>two</p></span>',
  '<span>one<br>two</span>',
  '<span>one<br/><br/>three</span>',
  '<span>one<br></span>',
  '<span><br>two</span>',
  '<span><br></span>',
  '<span></span>',
  '<span> </span>',
  '<span><p></p></span>',
  '<span><p> </p></span>',
  '<span><i class="fa fa-car"></i> Car</span>',
  '<span><p><i class="fa fa-car"></i> Car</p></span>',
  '<span>x<i></i></span>',
  '<span><i></i></span>',
  '<span><i></i><b></b></span>',
  '<span>a<span style="font-size:24px">B</span>c</span>',
  '<span style="font-size:10px">small <span style="font-size:20px">BIG</span></span>',
  '<span><small>small</small> normal</span>',
  '<span><code>code()</code> and text</span>',
  '<span>H<sub>2</sub>O x<sup>2</sup></span>',
  '<span><p>para one</p><p>para two</p></span>',
  '<p>para one</p><p>para two</p>',
  '<span>This is a much longer label that needs to wrap over several lines to fit in the box</span>',
  '<span><p>This is a much <strong>longer label</strong> that needs to <em>wrap over</em> several lines</p></span>',
  '<span>Supercalifragilisticexpialidocious_and_more_unbreakable</span>',
  '<span><a href="#">link</a> text</span>',
  '<span style="padding: 2px 4px; border: 1px solid red">decorated</span>',
  '<span style="background: #eee">a <span style="padding: 0 2px">b</span> c</span>',
  '<span><span><span>deep</span></span> tail</span>',
  '<span>  spaces   inside  </span>',
  '<div>inner div</div>',
  '<span><div>div in span</div></span>',
  '<span>你好，世界 mixed</span>',
];

for (let i = 0; i < inners.length; i++) {
  for (let k = 0; k < divStyles.length; k++) {
    // Each structure with each style, and a font that changes with both.
    for (const font of [fonts[(i + k) % fonts.length], fonts[(i * 3 + k + 5) % fonts.length]]) {
      add(
        `s${k}`,
        `<g transform="translate(20.5, 30.25)"><foreignObject id="fo" width="2000px" height="2000px"><div xmlns="http://www.w3.org/1999/xhtml" style="${divStyles[k]}${font}">${inners[i]}</div></foreignObject></g>`
      );
    }
  }
}

runProbe('htmlbox', cases);
