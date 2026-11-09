// Writes what Chromium measures for text in the web fonts of the page of
// mermaid-cli (internal/webfonts: Recursive Variable, Open Sans, Font
// Awesome 7), for the tests of internal/browser:
//
//   internal/browser/testdata/webfont-<set>-cases.json.gz
//   internal/browser/testdata/linux-webfont-<set>-results.json.gz
//   internal/browser/testdata/darwin-webfont-<set>-results.json.gz
//
// The page has the style sheets and the loaded font faces of mermaid-cli
// (oracle.mjs --webfonts). The Linux results come from Chromium in the
// Playwright image with only the Liberation fonts (oracle-linux.mjs --fonts
// liberation), and the test replays them with
// font.Embedded().WithWebFonts(). The macOS results come from Chromium on
// this machine (oracle.mjs), and the test replays them with
// font.System().WithWebFonts().
//
// Sets:
//   text   one text style for each case, with many strings: the box and the
//          length of SVG text, and the box of an HTML label
//   label  the markup that mermaid writes for labels (tspans with bold and
//          italic words, HTML labels with line-height 1.5 and a max-width)
//
// Usage (in tools/reference): node gen-browser-webfont-probes.mjs [--linux] [--darwin] [text|label ...]
// Without --linux and --darwin it measures both.
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

// The strings of the text set.
const strings = [
  // ASCII.
  'Hello World', 'The quick brown fox jumps over the lazy dog', 'AVATAR To. fi fl ffi', 'WAVE Type Yo.', 'gjpqy Qg_|',
  '0123456789', '(a) [b] {c} <d>', '@#$%&*+=~^', 'x', ' ', 'a b', 'Start', 'Is it?', 'Decision', 'Christmas',
  'Go shopping', 'Let me think', 'iPhone', 'A-->B', 'snake_case camelCase kebab-case', "it's \"x\"",
  // Digits and punctuation.
  '1/2 3/4 10%', '12:34:56', '$1,234.56', 'a.b,c;d:e!f?g', 'A.B.C.',
  // Pairs that fonts kern.
  'To Te Ty Tr', 'VA AV Wa Ya', 'LT LV LY', 'r. f, y. F. P,',
  // Latin-1.
  'ÀÉÎÕÜ àéîõü', 'Ñandú çà ß', 'Åland Ångström', '¡¿ © ® ±', 'naïve café',
  // Latin Extended.
  'ĲĳŁł Œœ', 'Čeština Žluťoučký', 'Şţęą ğı', 'Őő Űű',
  // Vietnamese.
  'Việt Nam', 'Tiếng Việt', 'ơ ư ạ ế ờ',
  // Cyrillic, with the extended block.
  'Привет мир', 'Щука', 'Ѐѝ Ґґ', 'Ҋҋ Ӂӂ',
  // Greek, with the extended block.
  'Ωμέγα', 'αβγδ', 'ἀἁἂ',
  // Symbols.
  '∑ ∫ √ ∞ ≠ ≤', '← → ↑ ↓', '✓ ✗ ★', '€ £ ¥ ₫', '™ • … – —', '“quoted” ‘it’s’',
  // Characters outside every range of the web fonts.
  '中文', '日本語テキスト', 'Hello 世界', '\uf007 \uf1b9',
  // Mixed scripts.
  'Hello Привет Ωμέγα', 'Việt ĲĳŁł café',
];

const fullFamilies = [
  '"Recursive Variable", arial, sans-serif',
  '"Recursive Variable"',
  '"Open Sans", sans-serif',
  '"Open Sans"',
];
const shortFamilies = [
  '"Font Awesome 7 Free"',
  '"Font Awesome 7 Brands"',
  'arial, "Recursive Variable"',
  '"recursive variable", "open sans"',
  '"trebuchet ms", verdana, arial, sans-serif',
];
const sizes = [8, 9, 10, 11, 12, 13, 13.5, 14, 15, 16, 17.33, 18, 20, 22.4, 24, 28, 32, 40];
const weightSizes = [12, 14, 16, 20];
const shortSizes = [12, 14, 16, 24];
const mainStyles = [[400, 'normal'], [700, 'normal'], [400, 'italic'], [700, 'italic']];

function textCase(cases, family, size, weight, style) {
  let inner = `<g style='font-family:${family};font-size:${size}px;font-weight:${weight};font-style:${style}'>`;
  strings.forEach((s, i) => {
    inner += `<text class="t" x="10" y="${(i + 1) * 60}">${esc(s)}</text>`;
    inner += `<foreignObject width="3000" height="100" y="${(i + 1) * 60}"><div class="d" xmlns="http://www.w3.org/1999/xhtml" style="display:inline-block;white-space:pre">${esc(s)}</div></foreignObject>`;
  });
  inner += '</g>';
  cases.push({
    name: `webfont-text-${cases.length}`,
    html: svg(inner),
    queries: [
      { sel: '.t', op: 'textlen', all: true },
      { sel: '.t', op: 'bbox', all: true },
      { sel: '.d', op: 'rect', all: true },
    ],
  });
}

function textSet() {
  const cases = [];
  for (const family of fullFamilies) {
    for (const size of sizes) {
      for (const [weight, style] of mainStyles) {
        textCase(cases, family, size, weight, style);
      }
    }
    for (const size of weightSizes) {
      for (const weight of [100, 200, 300, 500, 600, 800, 900]) {
        for (const style of ['normal', 'italic']) {
          textCase(cases, family, size, weight, style);
        }
      }
    }
  }
  for (const family of shortFamilies) {
    for (const size of shortSizes) {
      for (const [weight, style] of mainStyles) {
        textCase(cases, family, size, weight, style);
      }
    }
  }
  return cases;
}

// labelSet has the markup of mermaid labels with the words of the corpus.
function labelSet() {
  const rnd = prng(0x77656266);
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
  const families = [
    '"Recursive Variable", arial, sans-serif', '"Recursive Variable", arial, sans-serif', '"Recursive Variable", arial, sans-serif',
    '"Open Sans", sans-serif', '"Open Sans", sans-serif', '"trebuchet ms", verdana, arial, sans-serif',
  ];
  const cases = [];
  const add = (kind, html, queries) => cases.push({ name: `webfont-label-${kind}-${cases.length}`, html, queries });
  const textQueries = [{ sel: '#t', op: 'bbox' }, { sel: '#t', op: 'textlen' }, { sel: '#t', op: 'rect' }];
  // Random SVG labels.
  for (let i = 0; i < 900; i++) {
    const n = 1 + Math.floor(rnd() * 4);
    const text = Array.from({ length: n }, () => pick(ascii)).join(' ');
    const fam = pick(families);
    const size = pick([14, 14, 14, 16, 16, 12, 10, 11, 13, 15, 18, 20, 24, 12.8, 17.5]);
    const weight = rnd() < 0.25 ? 'bold' : 'normal';
    const style = rnd() < 0.12 ? 'italic' : 'normal';
    const anchor = pick(['', '', 'text-anchor="middle"', 'text-anchor="end"']);
    const baseline = pick(['', '', '', 'dominant-baseline="middle"', 'dominant-baseline="central"', 'dominant-baseline="hanging"', 'dy="1em"']);
    add('svg', svg(`<text id="t" x="${pick([0, 10, 100.5])}" y="${pick([0, 20, -7.25])}" ${anchor} ${baseline} style='font-family:${fam};font-size:${size}px;font-weight:${weight};font-style:${style}'>${esc(text)}</text>`), textQueries);
  }
  // Lines of tspans, as mermaid writes a label without HTML.
  for (let i = 0; i < 200; i++) {
    const fam = pick(families.slice(0, 5));
    const size = pick([14, 14, 16, 18]);
    const lines = 1 + Math.floor(rnd() * 3);
    let t = `<text id="t" y="-10.1" ${pick(['', 'text-anchor="middle"'])} style='font-family:${fam};font-size:${size}px'>`;
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
  for (let i = 0; i < 500; i++) {
    const n = 1 + Math.floor(rnd() * 9);
    let label = Array.from({ length: n }, () => esc(pick(ascii))).join(' ');
    if (rnd() < 0.2) {
      label = label.replace(' ', ' <b>bold</b> ');
    }
    if (rnd() < 0.15) {
      label = label.replace(' ', ' <em>italic</em> ');
    }
    if (rnd() < 0.15) {
      label = label.replace(' ', '<br/>');
    }
    const fam = pick(families);
    const size = pick([14, 14, 14, 16, 16, 12, 13, 18]);
    const w = pick([200, 200, 120, 150]);
    const html = svg(`<g style='font-family:${fam};font-size:${size}px'><foreignObject id="fo" width="${10 * w}px" height="${10 * w}px"><div id="d" xmlns="http://www.w3.org/1999/xhtml" style="${pick(divStyles)(w)}"><span id="s" class="nodeLabel"><p>${label}</p></span></div></foreignObject></g>`);
    add('html', html, [{ sel: '#d', op: 'rect' }, { sel: '#s', op: 'rect' }]);
  }
  return cases;
}

function gz(file, value) {
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify(value), { level: 9 }));
}

const builders = { text: textSet, label: labelSet };
let argv = process.argv.slice(2);
const platforms = [];
for (const flag of ['linux', 'darwin']) {
  if (argv.includes(`--${flag}`)) {
    platforms.push(flag);
  }
}
argv = argv.filter((a) => !a.startsWith('--'));
if (platforms.length === 0) {
  platforms.push('linux', 'darwin');
}
const wanted = argv.length > 0 ? argv : Object.keys(builders);

for (const name of wanted) {
  const cases = builders[name]();
  gz(path.join(dataDir, `webfont-${name}-cases.json.gz`), { cases });
  // A browser run stays short: a few hundred cases for each run.
  const step = 200;
  for (const platform of platforms) {
    const results = [];
    for (let start = 0; start < cases.length; start += step) {
      const sets = [{ name, cases: cases.slice(start, start + step) }];
      const argsFile = path.join(workDir, `browser-webfont-${platform}-args.json`);
      const outFile = path.join(workDir, `browser-webfont-${platform}-out.json`);
      fs.writeFileSync(argsFile, JSON.stringify({ sets }));
      const command = platform === 'linux'
        ? ['oracle-linux.mjs', '--fonts', 'liberation', 'eval', 'linux-dom-probe.js', '--webfonts', '--args', argsFile, '--out', outFile]
        : ['oracle.mjs', 'eval', 'linux-dom-probe.js', '--webfonts', '--args', argsFile, '--out', outFile];
      execFileSync('node', command, { cwd: here, stdio: 'inherit' });
      const result = JSON.parse(fs.readFileSync(outFile, 'utf8'));
      if (!result[name] || result[name].length !== sets[0].cases.length) {
        throw new Error(`${name}: ${result[name] ? result[name].length : 0} results for ${sets[0].cases.length} cases`);
      }
      results.push(...result[name]);
    }
    gz(path.join(dataDir, `${platform}-webfont-${name}-results.json.gz`), results);
    console.error(`${platform}-webfont-${name}-results.json.gz: ${cases.length} cases`);
  }
}
