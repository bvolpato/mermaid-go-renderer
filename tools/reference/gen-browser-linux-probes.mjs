// Writes what Chromium on Linux measures for the DOM probe cases of
// internal/browser: internal/browser/testdata/linux-<set>-results.json.gz.
//
// The reference environment is Chromium on Linux with only the Liberation
// fonts (oracle-linux.mjs --fonts liberation). The test
// TestLinuxProbesAgainstChromium replays the cases with font.Embedded().
//
// The sets are the case files that the macOS probes already have
// (<set>-cases.json, written by the gen-browser-<set>-probe.mjs scripts),
// and two sets that this script builds: linuxtext (SVG text of the words of
// the corpus in the font families of mermaid) and linuxdom (the generic
// cases of .parity/probe/gen-dom-cases.mjs, when that file exists).
//
// With --image the script measures in the Playwright image as it is, with
// all of its fonts, and writes linux-image-<set>-results.json.gz. The test
// TestLinuxImageProbesAgainstChromium replays those with the font files of
// the image (.parity/linux/image/fonts), which are not in the repository.
//
// Usage (in tools/reference): node gen-browser-linux-probes.mjs [--image] [set ...]
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const dataDir = path.join(repoRoot, 'internal/browser/testdata');
const workDir = path.join(repoRoot, '.parity/linux/work');
fs.mkdirSync(workDir, { recursive: true });

// The case files of the macOS probes that the Linux test replays.
const shared = ['arc', 'baseline', 'boundschange', 'brspace', 'cssvar', 'emptytext', 'ex', 'fontsize', 'htmlbox', 'kernsplit', 'pathbounds', 'scriptrun', 'union'];

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

const svg = (inner) => `<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`;
const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// linuxtext: the box and the length of SVG text, and the box of HTML
// labels, for the words of the corpus.
function linuxText() {
  const rnd = prng(0x6c6e7874);
  const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
  const words = new Set();
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(p);
      } else if (p.endsWith('.mmd')) {
        for (const w of fs.readFileSync(p, 'utf8').split(/[\s[\](){}"|:;,<>]+/)) {
          if (w.length > 0 && w.length < 24 && /^[\x21-\x7e]+$/.test(w)) {
            words.add(w);
          }
        }
      }
    }
  };
  walk(path.join(repoRoot, 'testdata/corpus/mermaid-e2e'));
  const ascii = [...words].sort();
  const stress = ['gjpqy', 'Qg_|', '(a) [b] {c}', 'ÀÉÎÕÜ', 'Åland Ångström', 'Ñandú çà', 'fjord', 'AVATAR To Te', 'Tj Ly', '@#$%&*', 'jumping (query);', 'x', '_', '|', 'ÿ', 'Ç', 'ĲĳŁł', 'Ωμέγα', 'Привет', 'Щука', '1/2 3/4', '"quoted"', "it's", 'a.b,c', 'É', 'g', 'j', 'Q', 'y', 'p', 'ţ', 'ş', 'ę'];
  const families = [
    "'trebuchet ms', verdana, arial, sans-serif", "'trebuchet ms', verdana, arial, sans-serif", 'arial', "'times new roman'",
    "'courier new'", 'monospace', 'sans-serif', 'serif', "'Recursive Variable', arial, sans-serif", 'verdana', 'system-ui',
  ];
  const sizes = [16, 16, 14, 14, 12, 10, 11, 13, 15, 17, 18, 20, 24, 32, 12.8, 14.4, 17.5, 13.333, 9.6, 40, 22.4, 28];
  const cases = [];
  const add = (kind, html, queries) => cases.push({ name: `linuxtext-${kind}-${cases.length}`, html, queries });
  const textQueries = [{ sel: '#t', op: 'bbox' }, { sel: '#t', op: 'textlen' }, { sel: '#t', op: 'rect' }];
  // Every stress string in the main families at the common sizes.
  for (const s of stress) {
    for (const fam of families.slice(1, 6)) {
      for (const size of [12, 14, 16, 17]) {
        add('stress', svg(`<text id="t" x="10" y="50" style="font-family:${fam};font-size:${size}px">${esc(s)}</text>`), textQueries);
      }
    }
  }
  // Random labels.
  for (let i = 0; i < 1200; i++) {
    const n = 1 + Math.floor(rnd() * 4);
    const text = Array.from({ length: n }, () => pick(ascii)).join(' ');
    const fam = pick(families);
    const size = pick(sizes);
    const weight = rnd() < 0.25 ? 'bold' : 'normal';
    const style = rnd() < 0.12 ? 'italic' : 'normal';
    const anchor = pick(['', '', 'text-anchor="middle"', 'text-anchor="end"']);
    const baseline = pick(['', '', '', 'dominant-baseline="middle"', 'dominant-baseline="central"', 'dominant-baseline="hanging"', 'dy="1em"']);
    add('label', svg(`<text id="t" x="${pick([0, 10, 100.5])}" y="${pick([0, 20, -7.25])}" ${anchor} ${baseline} style="font-family:${fam};font-size:${size}px;font-weight:${weight};font-style:${style}">${esc(text)}</text>`), textQueries);
  }
  // Lines of tspans, as mermaid writes a label without HTML.
  for (let i = 0; i < 150; i++) {
    const fam = pick(families.slice(0, 3));
    const size = pick([14, 16, 16, 18]);
    const lines = 1 + Math.floor(rnd() * 3);
    let t = `<text id="t" y="-10.1" ${pick(['', 'text-anchor="middle"'])} style="font-family:${fam};font-size:${size}px">`;
    for (let l = 0; l < lines; l++) {
      t += `<tspan class="text-outer-tspan" x="0" y="${l * 1.1 - 0.1}em" dy="1.1em" id="l${l}">`;
      const count = 1 + Math.floor(rnd() * 4);
      for (let k = 0; k < count; k++) {
        t += `<tspan font-style="${pick(['normal', 'normal', 'italic'])}" class="text-inner-tspan" font-weight="${pick(['normal', 'normal', 'bold'])}">${k ? ' ' : ''}${esc(pick(ascii))}</tspan>`;
      }
      t += '</tspan>';
    }
    t += '</text>';
    add('tspans', svg(`<g id="g">${t}</g>`), [{ sel: '#t', op: 'bbox' }, { sel: '#l0', op: 'textlen' }, { sel: '#l0', op: 'rect' }, { sel: '#g', op: 'bbox' }]);
  }
  // HTML labels: the markup of mermaid inside a foreignObject.
  const divStyles = [
    (w) => `display: table-cell; white-space: nowrap; line-height: 1.5; max-width: ${w}px; text-align: center;`,
    (w) => `display: table; white-space: break-spaces; line-height: 1.5; max-width: ${w}px; text-align: center; width: ${w}px;`,
    () => 'display: inline-block; white-space: nowrap;',
    (w) => `display: inline-block; white-space: normal; max-width: ${w}px; line-height: 1.5`,
  ];
  for (let i = 0; i < 300; i++) {
    const n = 1 + Math.floor(rnd() * 9);
    let label = Array.from({ length: n }, () => esc(pick(ascii))).join(' ');
    if (rnd() < 0.2) {
      label = label.replace(' ', ' <b>bold</b> ');
    }
    if (rnd() < 0.15) {
      label = label.replace(' ', '<br/>');
    }
    const fam = pick(families.slice(0, 4));
    const size = pick([16, 16, 14, 12, 13, 18]);
    const w = pick([200, 200, 120, 150]);
    const html = svg(`<g style="font-family:${fam};font-size:${size}px"><foreignObject id="fo" width="${10 * w}px" height="${10 * w}px"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="${pick(divStyles)(w)}"><span id="s" class="nodeLabel"><p>${label}</p></span></div></foreignObject></g>`);
    add('html', html, [{ sel: '#d', op: 'rect' }, { sel: '#s', op: 'rect' }]);
  }
  return cases;
}

function gz(file, value) {
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify(value), { level: 9 }));
}

// loadCases returns the cases of a set, and writes the case file of a set
// that this script builds.
function loadCases(name) {
  if (name === 'linuxtext') {
    const cases = linuxText();
    gz(path.join(dataDir, 'linux-linuxtext-cases.json.gz'), { cases });
    return cases;
  }
  if (name === 'linuxdom') {
    const source = path.join(repoRoot, '.parity/probe/dom-cases.json');
    if (!fs.existsSync(source)) {
      return null;
    }
    const { cases } = JSON.parse(fs.readFileSync(source, 'utf8'));
    gz(path.join(dataDir, 'linux-linuxdom-cases.json.gz'), { cases });
    return cases;
  }
  return JSON.parse(fs.readFileSync(path.join(dataDir, `${name}-cases.json`), 'utf8')).cases;
}

const argv = process.argv.slice(2);
const image = argv[0] === '--image';
if (image) {
  argv.shift();
}
const prefix = image ? 'linux-image-' : 'linux-';
const wanted = argv.length > 0 ? argv : [...shared, 'linuxtext', 'linuxdom'];
// One browser measures a few sets, so that a run stays short.
const chunks = [];
let chunk = [];
let weight = 0;
for (const name of wanted) {
  const cases = loadCases(name);
  if (!cases) {
    console.error(`${name}: no cases, skipped`);
    continue;
  }
  chunk.push({ name, cases });
  weight += cases.length;
  if (weight >= 2500) {
    chunks.push(chunk);
    chunk = [];
    weight = 0;
  }
}
if (chunk.length > 0) {
  chunks.push(chunk);
}
for (const sets of chunks) {
  const argsFile = path.join(workDir, 'browser-linux-args.json');
  const outFile = path.join(workDir, 'browser-linux-out.json');
  fs.writeFileSync(argsFile, JSON.stringify({ sets }));
  execFileSync('node', ['oracle-linux.mjs', '--fonts', image ? 'image' : 'liberation', 'eval', 'linux-dom-probe.js', '--args', argsFile, '--out', outFile], { cwd: here, stdio: 'inherit' });
  const result = JSON.parse(fs.readFileSync(outFile, 'utf8'));
  for (const { name, cases } of sets) {
    if (!result[name] || result[name].length !== cases.length) {
      throw new Error(`${name}: ${result[name] ? result[name].length : 0} results for ${cases.length} cases`);
    }
    gz(path.join(dataDir, `${prefix}${name}-results.json.gz`), result[name]);
    console.error(`${prefix}${name}-results.json.gz: ${cases.length} cases`);
  }
}
