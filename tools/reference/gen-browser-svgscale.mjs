// Writes internal/browser/testdata/svgscale.json: what Chromium measures for
// SVG content whose user space is not the CSS pixel space of the page.
//   - A root svg with a viewBox and width="100%" plus max-width, as the
//     xychart renderer makes it before it measures text: the size of the
//     svg box, getScreenCTM of a group, and the box of text in the group.
//   - Text and shapes below a group with a scale, a rotation or a
//     translation that is not an integer.
//   - Text whose box crosses a power of two when the offset of the page is
//     added: getBoundingClientRect adds the offset to each corner in
//     float32.
// The script runs browser-svgscale-page.js with the eval command of the
// browser oracle: one time for the transforms, and then one time for each
// batch of cases (see below).
//
// Usage (in tools/reference): node gen-browser-svgscale.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/browser/testdata/svgscale.json');

function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = prng(0x73766773);
const pick = (list) => list[Math.floor(rnd() * list.length)];

const style =
  '<style>#my-svg{font-family:"trebuchet ms",verdana,arial,sans-serif;font-size:16px;fill:#333;}</style>';
const texts = [
  'Sales Revenue',
  'Wide chart scaled down to fit',
  '11000',
  'jan',
  'Revenue (in $)',
  'Elapsed Time',
  'CPU',
  '0.5',
  'Percentage (%)',
  'Wg',
  'i',
  'AVATAR To. ffi',
  '',
];
// probe is the element that computeDimensionOfText of mermaid makes.
const probe = (id, fontSize, text) =>
  `<g id="${id}" visibility="hidden" font-size="${fontSize}"><text><tspan class="text-outer-tspan" x="0" y="0.9em" dy="1em">` +
  `<tspan font-style="normal" class="text-inner-tspan" font-weight="normal">${text}</tspan></tspan></text></g>`;
const probeQueries = (id) => [
  { sel: `#${id}`, op: 'ctm' },
  { sel: `#${id} .text-outer-tspan`, op: 'rect' },
  { sel: `#${id} text`, op: 'rect' },
  { sel: `#${id} text`, op: 'bbox' },
  { sel: `#${id} .text-outer-tspan`, op: 'bbox' },
  { sel: `#${id} .text-inner-tspan`, op: 'textlen' },
];

const cases = [];

// 1. A root svg with a viewBox, sized as mermaid does with useMaxWidth.
const sizes = [
  [700, 500], [784, 450], [785, 500], [800, 600], [900, 20], [1000, 600], [1200, 60], [1568, 500],
  [2000, 300], [4000, 500], [120, 800], [200, 20], [640, 100], [1000, 1000], [3000, 40], [5000, 5000],
  [700.5, 333.25], [999.9, 500], [1234.5, 678.9], [801, 601], [1001, 333], [7777, 777],
];
for (const [w, h] of sizes) {
  for (const fontSize of [20, 14, 16, 10, 12, 27, 30, 8]) {
    const text = pick(texts);
    cases.push({
      name: `root-${w}x${h}-${fontSize}`,
      html:
        `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" style="max-width: ${w}px;" viewBox="0 0 ${w} ${h}">` +
        `${style}<g></g><g class="main"><rect width="${w}" height="${h}"></rect></g>` +
        `<g class="mermaid-tmp-group">${probe('p', fontSize, text)}</g></svg>`,
      queries: [{ sel: '#my-svg', op: 'rect' }, ...probeQueries('p')],
    });
  }
}
// All the texts in one size, and a text with an explicit font size in px.
for (const [w, h] of [[4000, 500], [1000, 600], [1200, 60], [700, 500]]) {
  texts.forEach((text, i) => {
    cases.push({
      name: `rootall-${w}x${h}-${i}`,
      html:
        `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" style="max-width: ${w}px;" viewBox="0 0 ${w} ${h}">` +
        `${style}<g class="mermaid-tmp-group">${probe('p', 20, text)}${probe('q', 14, text)}</g></svg>`,
      queries: [...probeQueries('p'), ...probeQueries('q')],
    });
  });
}
// Other ways to size the root svg, and a viewBox that does not start at 0.
const roots = [
  'width="400" height="300" viewBox="0 0 800 600"',
  'width="400" height="300" viewBox="0 0 800 300"',
  'width="400" height="300" viewBox="0 0 300 600"',
  'width="400" height="300" viewBox="-50 -20 800 600"',
  'width="400" height="300" viewBox="0 0 800 300" preserveAspectRatio="none"',
  'width="400" height="300" viewBox="0 0 800 300" preserveAspectRatio="xMinYMin meet"',
  'width="400" height="300" viewBox="0 0 800 300" preserveAspectRatio="xMaxYMax slice"',
  'width="100%" viewBox="0 0 1600 400"',
  'width="100%" viewBox="0 0 400 100"',
  'width="50%" viewBox="0 0 1600 400"',
  'width="100%" height="200" viewBox="0 0 1600 400"',
  'viewBox="0 0 1600 400"',
  'viewBox="0 0 300 100" style="max-width: 300px;"',
  'width="100%" style="max-width: 500px;" viewBox="0 0 250 100"',
  'width="100%" style="max-width: 500px;"',
  'width="100%"',
  'width="2000" height="100"',
  'width="100%" style="max-width: 1000px;" viewBox="10 20 1000 600"',
];
roots.forEach((attrs, i) => {
  cases.push({
    name: `rootattr-${i}`,
    html:
      `<svg id="my-svg" xmlns="http://www.w3.org/2000/svg" ${attrs}>${style}` +
      `<g id="s"><rect id="r" x="10.5" y="20.25" width="100.5" height="30.25"></rect></g>` +
      `<g class="mermaid-tmp-group">${probe('p', 20, 'Sales Revenue')}</g></svg>`,
    queries: [
      { sel: '#my-svg', op: 'rect' },
      { sel: '#s', op: 'ctm' },
      { sel: '#r', op: 'rect' },
      { sel: '#s', op: 'rect' },
      ...probeQueries('p'),
    ],
  });
});

// 2. Groups with a transform, in a root svg without a viewBox.
const transforms = [
  'scale(2)', 'scale(0.5)', 'scale(0.196)', 'scale(1.5, 0.75)', 'scale(3.3)', 'rotate(30)', 'rotate(270)',
  'rotate(-45)', 'translate(10.5, 3.25)', 'translate(100, 50)', 'translate(7, 9) scale(1.25)',
  'matrix(0.866,0.5,-0.5,0.866,10,20)', 'scale(0.653125)', 'translate(0.3, 0.7)', 'scale(10)', 'scale(-1, 1)',
  'skewX(20)', 'translate(24, 56) rotate(90)',
];
transforms.forEach((t, i) => {
  for (const fontSize of [16, 20, 11]) {
    const text = pick(texts.slice(0, 12));
    cases.push({
      name: `group-${i}-${fontSize}`,
      html:
        `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${style}` +
        `<g id="o" transform="${t}"><rect id="r" x="10.5" y="20.25" width="100.5" height="30.25"></rect>` +
        `${probe('p', fontSize, text)}<text id="t" x="12.5" y="40" font-size="${fontSize}">${text}</text>` +
        `<g id="i" transform="translate(5, 6)"><text id="u" x="3" y="30" text-anchor="middle" font-size="${fontSize}">${text}</text></g></g></svg>`,
      queries: [
        { sel: '#o', op: 'ctm' },
        { sel: '#r', op: 'rect' },
        ...probeQueries('p'),
        { sel: '#t', op: 'rect' },
        { sel: '#t', op: 'bbox' },
        { sel: '#t', op: 'textlen' },
        { sel: '#u', op: 'rect' },
        { sel: '#u', op: 'bbox' },
        { sel: '#i', op: 'rect' },
        { sel: '#o', op: 'rect' },
        { sel: '#o', op: 'bbox' },
      ],
    });
  }
});

// 3. No transform: the offset of the page moves each corner in float32.
for (const fontSize of [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 33, 36, 40, 48, 64]) {
  const text = pick(texts.slice(0, 12));
  cases.push({
    name: `offset-${fontSize}`,
    html:
      `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${style}` +
      `<g class="mermaid-tmp-group">${probe('p', fontSize, text)}</g>` +
      `<g id="o" transform="translate(${fontSize}, ${fontSize * 2})">${probe('q', fontSize, text)}` +
      `<rect id="r" x="0.3" y="26.299999" width="${fontSize * 1.7}" height="31.1"></rect></g></svg>`,
    queries: [...probeQueries('p'), ...probeQueries('q'), { sel: '#r', op: 'rect' }, { sel: '#o', op: 'rect' }],
  });
}

// Chromium keeps one platform font for each font size key of 1/100 px, and
// two sizes can have the same key: 9.39px and 9.40px, because the float32 of
// 9.40 is below 9.40. The font of the first request then serves both, so
// that a measurement depends on what the page measured before. A render of
// mermaid starts with a new page. So the cases run in batches, each in a new
// browser, and no batch has two font sizes with the same key.
function evalCases(list) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'svgscale-'));
  const argsFile = path.join(tmp, 'args.json');
  const resultFile = path.join(tmp, 'result.json');
  fs.writeFileSync(argsFile, JSON.stringify({ cases: list }));
  execFileSync(
    process.execPath,
    ['oracle.mjs', 'eval', 'browser-svgscale-page.js', '--args', argsFile, '--out', resultFile, '--bare'],
    { cwd: here, stdio: 'inherit' }
  );
  const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
  fs.rmSync(tmp, { recursive: true, force: true });
  if (results.length !== list.length) {
    throw new Error(`expected ${list.length} results, got ${results.length}`);
  }
  return results;
}

// fontKeys returns the font size keys of a case and the effective size for
// each key: the font sizes of the markup, scaled as Blink scales the font of
// SVG text (the mean scale of the transform to the screen, in float32).
function fontKeys(c, ctm) {
  const f = Math.fround;
  const [a, b, cc, d] = ctm;
  const factor = f(Math.sqrt((a * a + b * b + (cc * cc + d * d)) / 2));
  const keys = new Map();
  for (const m of c.html.matchAll(/font-size="([0-9.]+)"/g)) {
    const scaled = f(f(Number(m[1])) * factor);
    const effective = f(Math.floor(f(scaled * 100)) / 100);
    keys.set(Math.trunc(f(effective * 100)), effective);
  }
  return keys;
}

// The first run gives the transform of each case. It does not depend on the
// fonts.
const first = evalCases(cases);
const batches = [];
cases.forEach((c, i) => {
  const ctmIndex = c.queries.findIndex((q) => q.op === 'ctm');
  const keys = fontKeys(c, first[i].results[ctmIndex]);
  let batch = batches.find((bt) => [...keys].every(([k, size]) => !bt.keys.has(k) || bt.keys.get(k) === size));
  if (!batch) {
    batch = { keys: new Map(), indexes: [] };
    batches.push(batch);
  }
  for (const [k, size] of keys) {
    batch.keys.set(k, size);
  }
  batch.indexes.push(i);
});
const golden = new Array(cases.length);
for (const batch of batches) {
  const results = evalCases(batch.indexes.map((i) => cases[i]));
  batch.indexes.forEach((i, k) => {
    golden[i] = { ...cases[i], results: results[k].results };
  });
}
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, JSON.stringify(golden, null, 0) + '\n');
console.log(`${outFile}: ${golden.length} cases in ${batches.length} batches`);
