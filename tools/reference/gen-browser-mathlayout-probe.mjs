#!/usr/bin/env node
// Capture native MathML sizes in the label wrappers that Mermaid uses.
// Write the HTML cases and the Linux and macOS browser results.
//
// Run from any directory with:
//   node tools/reference/gen-browser-mathlayout-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/browser/testdata');
const probe = path.join(here, 'probes/mathml.js');
const mathmlNS = 'http://www.w3.org/1998/Math/MathML';
const xhtmlNS = 'http://www.w3.org/1999/xhtml';
const svgNS = 'http://www.w3.org/2000/svg';

const expressions = [
  ['token-mi-x', '<mi>x</mi>'],
  ['token-mi-abc', '<mi>abc</mi>'],
  ['token-mn-number', '<mn>123</mn>'],
  ['token-mo-operator', '<mo>+</mo>'],
  ['row-a-plus-b', '<mrow><mi>a</mi><mo>+</mo><mi>b</mi></mrow>'],
  ['superscript-x-2', '<msup><mi>x</mi><mn>2</mn></msup>'],
  ['subscript-x-i', '<msub><mi>x</mi><mi>i</mi></msub>'],
  ['subsuperscript-x-i-2', '<msubsup><mi>x</mi><mi>i</mi><mn>2</mn></msubsup>'],
  ['fraction-a-b', '<mfrac><mi>a</mi><mi>b</mi></mfrac>'],
  ['sqrt-x-plus-1', '<msqrt><mrow><mi>x</mi><mo>+</mo><mn>1</mn></mrow></msqrt>'],
  ['cube-root-x', '<mroot><mi>x</mi><mn>3</mn></mroot>'],
  [
    'sum-limits',
    '<mrow><munderover><mo>∑</mo><mrow><mi>i</mi><mo>=</mo><mn>1</mn></mrow><mi>n</mi></munderover><mi>x</mi></mrow>',
  ],
  [
    'matrix-2x2',
    '<mrow><mo>(</mo><mtable rowspacing="0.5em" columnspacing="1em"><mtr><mtd><mi>a</mi></mtd><mtd><mi>b</mi></mtd></mtr><mtr><mtd><mi>c</mi></mtd><mtd><mi>d</mi></mtd></mtr></mtable><mo>)</mo></mrow>',
  ],
  [
    'semantics-annotation',
    '<semantics><mrow><mi>x</mi><mo>+</mo><mn>1</mn></mrow><annotation encoding="application/x-tex">x+1</annotation></semantics>',
  ],
  [
    'mspace',
    '<mrow><mi>a</mi><mspace width="1em" height="0.5em" depth="0.25em"/><mi>b</mi></mrow>',
  ],
];

function math(body) {
  return `<math class="probe-math" xmlns="${mathmlNS}" display="block">${body}</math>`;
}

function katex(body) {
  return `<span class="katex">${math(body)}</span>`;
}

function labelInner(body) {
  return `<span id="probe-node-label" class="nodeLabel"><div id="probe-flex" style="display:flex;align-items:center;justify-content:center;white-space:nowrap">${body}</div></span>`;
}

function caseHTML(fontSize, body) {
  const label = labelInner(body);
  const svg = `<svg xmlns="${svgNS}"><foreignObject id="probe-fo" width="2000px" height="2000px"><div id="probe-outer" xmlns="${xhtmlNS}" style="display:table-cell;white-space:nowrap;line-height:1.5;max-width:200px;font-size:${fontSize}px;font-family:Arial">${label}</div></foreignObject></svg>`;
  const temp = `<div id="katex-temp" style="visibility:hidden;position:absolute;top:0;font-size:${fontSize}px;font-family:Arial">${body}</div>`;
  return svg + temp;
}

const selectors = '#probe-outer, math';
const cases = [];
for (const fontSize of [16, 14]) {
  for (const [name, body] of expressions) {
    cases.push({
      name: `mathlayout-${fontSize}-${name}`,
      html: caseHTML(fontSize, katex(body)),
      sel: selectors,

    });
  }

  const mixed = `<span class="probe-text">before </span>${katex('<mi>x</mi>')}<span class="probe-text"> between </span>${katex('<mfrac><mi>a</mi><mi>b</mi></mfrac>')}<span class="probe-text"> after</span>`;
  cases.push({
    name: `mathlayout-${fontSize}-adjacent-math-mixed-text`,
    html: caseHTML(fontSize, mixed),
    sel: selectors,

  });
}

const parityDir = path.join(repoRoot, '.parity');
fs.mkdirSync(parityDir, {recursive: true});
const tempDir = fs.mkdtempSync(path.join(parityDir, '.mathlayout-probe-'));
const casesFile = path.join(tempDir, 'mathlayout-cases.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases }, null, 1) + '\n');

const runs = [
  ['linux', 'oracle-linux.mjs', ['--fonts', 'liberation']],
  ['macos', 'oracle.mjs', []],
];

try {
  const results = new Map();
  for (const [environment, oracle, oracleArgs] of runs) {
    const resultFile = path.join(tempDir, `mathlayout-${environment}-results.json`);
    execFileSync(
      process.execPath,
      [
        path.join(here, oracle),
        ...oracleArgs,
        'eval',
        probe,
        '--bare',
        '--args',
        casesFile,
        '--out',
        resultFile,
      ],
      { cwd: here, stdio: 'inherit', timeout: 120_000 }
    );
    const data = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
    if (!Array.isArray(data) || data.length !== cases.length) {
      throw new Error(`${environment} oracle returned ${data?.length ?? 'no'} cases, expected ${cases.length}`);
    }
    for (let i = 0; i < cases.length; i++) {
      if (data[i].name !== cases[i].name || !Array.isArray(data[i].rows) || data[i].rows.length === 0) {
        throw new Error(`${environment} oracle returned an incomplete result for ${cases[i].name}`);
      }
    }
    results.set(environment, data.map(({name, rows}) => ({
      name, boxes: rows.map(([tag, , , width, height]) => ({ tag, width, height })),
    })));
    console.error(`${environment}: captured ${data.length} cases`);
  }

  fs.mkdirSync(outDir, { recursive: true });
  fs.writeFileSync(path.join(outDir, 'mathlayout-cases.json'), JSON.stringify({ cases }, null, 1) + '\n');
  for (const [environment] of runs) {
    fs.writeFileSync(path.join(outDir, `mathlayout-${environment}-results.json`), JSON.stringify(results.get(environment), null, 1) + '\n');
  }
  console.error(`wrote ${cases.length} cases to ${path.relative(repoRoot, outDir)}`);
} finally {
  fs.rmSync(tempDir, { recursive: true, force: true });
}
