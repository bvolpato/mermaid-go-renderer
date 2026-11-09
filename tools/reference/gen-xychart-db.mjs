// Writes internal/diagrams/xychart/testdata/db.json.gz: what real mermaid
// has in the xychart db after it renders
//   - every fixture of testdata/corpus/mermaid-e2e/xychart,
//   - seeded mutations of the body of those fixtures,
//   - charts with a seeded random config (sizes, paddings, flags, rotation),
//   - hand-written inputs for the branches of xychartDb.ts, of the chart
//     builder and of the renderer.
// Each record has the drawable elements of the chart builder (all the
// coordinates, before any SVG is written), the chart config, the theme
// config, the chart data, the title, the accessibility values and the SVG
// without its style element. The chart builder measures text in the DOM, so
// the script runs xychart-db-page.js with the eval command of the browser
// oracle, one time.
//
// All the cases run in one page. Chromium keeps one platform font for each
// font size key of 1/100 px, so that a chart that is scaled to fit the page
// can get the font of an earlier case (see gen-browser-svgscale.mjs). The Go
// test reproduces the scaled charts of this file with the fonts of a new
// page, so they do not have that effect.
//
// Usage (in tools/reference): node gen-xychart-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const family = 'testdata/corpus/mermaid-e2e/xychart';
const outFile = path.join(repoRoot, 'internal/diagrams/xychart/testdata/db.json.gz');
const mutantsPerFixture = 3;
const randomConfigs = 160;

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

const fixtures = fs
  .readdirSync(path.join(repoRoot, family), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => family + '/' + f.split(path.sep).join('/'))
  .sort();

const snippets = [
  'title "Another title"',
  'title Plain title',
  'x-axis "Time" 0 --> 10',
  'x-axis 2 --> 3',
  'x-axis [a, b, c, d]',
  'x-axis "Cats" [alpha, "beta gamma", delta]',
  'y-axis "Value" 0 --> 100',
  'y-axis -5 --> 5',
  'y-axis "Only title"',
  'line [1, 2, 3]',
  'line "named line" [5, 3.5, -2, 8, 1]',
  'line [10 "ten", 20, 30 "thirty"]',
  'bar [4, 5, 6]',
  'bar "named bar" [9, 1, 7.25, 3]',
  'bar [100]',
  'accTitle: An accessible title',
  'accDescr: An accessible description',
  'accDescr {\n  A description\n  on two lines\n}',
  '%% a comment',
  ';',
];

// mutate changes the lines after the "xychart" line.
function mutate(rnd, text) {
  const lines = text.split('\n');
  let start = lines.findIndex((l) => /^\s*xychart/.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 6);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind === 3) {
      lines.splice(p, 0, '  ' + snippets[Math.floor(rnd() * snippets.length)]);
    } else if (kind === 4) {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    } else {
      lines.length = Math.max(start, p);
    }
  }
  return lines.join('\n');
}

// Bodies for the charts with a random config.
const bodies = [
  'xychart\n  title "Sales Revenue"\n  x-axis Months [jan, feb, mar, apr, may, jun, jul, aug, sep, oct, nov, dec]\n  y-axis "Revenue (in $)" 4000 --> 11000\n  bar [5000, 6000, 7500, 8200, 9500, 10500, 11000, 10200, 9200, 8500, 7000, 6000]\n  line [5000, 6000, 7500, 8200, 9500, 10500, 11000, 10200, 9200, 8500, 7000, 6000]\n',
  'xychart\n  title "Named plots"\n  x-axis ["90d", "60d", "30d", "7d", "1d", "Current"]\n  y-axis "Seconds" 0 --> 198.2\n  line "avg" [48.1, 41.5, 45.7, 72.8, 67.7, 59.9]\n  line "p50" [38.2, 36.8, 39.7, 54.5, 49.0, 38.4]\n  bar "p95" [112.2, 75.3, 103.0, 177.0, 180.2, 109.4]\n',
  'xychart\n  title "Linear axis"\n  x-axis "Elapsed Time" 0 --> 10\n  y-axis "Percentage (%)" 0 --> 100\n  line "CPU" [10, 20, 30, 40, 50, 60, 70, 80, 90, 100]\n  line "RAM" [20, 30, 40, 50, 60, 70, 80, 90, 95, 90]\n',
  'xychart\n  title "Point labels"\n  x-axis "Date" ["Apr 2022", "Feb 2023", "Jul 2023", "Sep 2023", "Apr 2024"]\n  y-axis "Parameters (B)" 0 --> 600\n  line [540 "PaLM", 65 "LLaMA-65B", 34 "Llama 2 34B", 7 "Mistral 7B", 3.8 "Phi-3-mini"]\n  bar "sizes" [540, 65, 34, 7, 3.8]\n',
  'xychart\n  x-axis [A, B, C]\n  y-axis -10 --> 10\n  bar [ -2.5, 0.75, 5.1 ]\n  bar "second" [ 3, -7, 9.99 ]\n',
  'xychart\n  title "Git Commits per Month"\n  x-axis "Date" [ 2023-04, 2023-05, 2023-06, 2023-07, 2023-08, 2023-09, 2023-10, 2023-11 ]\n  y-axis "Number of Commits"\n  bar    [ 344, 523, 81, 7, 3, 3, 5, 17 ]\n',
];

function randomConfig(rnd) {
  const pick = (list) => list[Math.floor(rnd() * list.length)];
  const int = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));
  const maybe = (p, value) => (rnd() < p ? value() : undefined);
  const axis = () => ({
    showLabel: maybe(0.3, () => rnd() < 0.5),
    labelFontSize: maybe(0.5, () => int(8, 24)),
    labelPadding: maybe(0.5, () => int(0, 12)),
    showTitle: maybe(0.3, () => rnd() < 0.5),
    titleFontSize: maybe(0.5, () => int(8, 28)),
    titlePadding: maybe(0.5, () => int(0, 12)),
    showTick: maybe(0.3, () => rnd() < 0.5),
    tickLength: maybe(0.5, () => int(0, 12)),
    tickWidth: maybe(0.5, () => int(0, 5)),
    showAxisLine: maybe(0.3, () => rnd() < 0.5),
    axisLineWidth: maybe(0.5, () => int(0, 6)),
    labelRotation: maybe(0.4, () => pick([-120, -90, -60, -45, -30, -10, 0, 15, 30, 45, 60, 90, 100])),
  });
  return {
    width: maybe(0.6, () => pick([120, 200, 300, 400, 500, 640, 700, 784, 785, 800, 900, 1000, 1200, 1568, 2000, 4000])),
    height: maybe(0.6, () => pick([20, 40, 60, 100, 200, 300, 450, 500, 600, 800])),
    titleFontSize: maybe(0.5, () => int(8, 30)),
    titlePadding: maybe(0.5, () => int(0, 20)),
    showTitle: maybe(0.3, () => rnd() < 0.5),
    showLegend: maybe(0.3, () => rnd() < 0.5),
    legendFontSize: maybe(0.5, () => int(8, 24)),
    legendPadding: maybe(0.5, () => int(0, 20)),
    showDataLabel: maybe(0.5, () => rnd() < 0.7),
    showDataLabelOutsideBar: maybe(0.4, () => rnd() < 0.5),
    chartOrientation: maybe(0.5, () => pick(['vertical', 'horizontal'])),
    plotReservedSpacePercent: maybe(0.4, () => pick([0, 10, 30, 50, 60, 75, 100])),
    xAxis: maybe(0.7, axis),
    yAxis: maybe(0.7, axis),
  };
}

const init = (config) => `%%{init: ${JSON.stringify(config)}}%%\n`;
const xy = (body, front = '') => front + 'xychart\n' + body + '\n';
const complete =
  '  title "Sales Revenue"\n  x-axis Months [jan, feb, mar, apr, may, jun]\n  y-axis "Revenue (in $)" 4000 --> 11000\n  bar "bars" [5000, 6000, 7500, 8200, 9500, 10500]\n  line "lines" [5000, 6000, 7500, 8200, 9500, 10500]';
const extras = [
  // The db.
  xy(''),
  xy('  title "No plot"\n  x-axis [a, b]'),
  xy('  line [1]'),
  xy('  bar [1]'),
  xy('  line [1, 2, 3, 4]\n  bar [5, 6]'),
  xy('  line [1, 2]\n  bar [5, 6, 7, 8]'),
  xy('  bar [5, 6, 7, 8]\n  line [1, 2]\n  x-axis [a, b, c]'),
  xy('  x-axis [a, b, c]\n  bar [1, 2]'),
  xy('  x-axis [a, b, c]\n  line [1, 2]'),
  xy('  x-axis [a, b, c]\n  bar [1, 2, 3, 4, 5]\n  line [9, 8, 7, 6]'),
  xy('  x-axis [a, b, a, c]\n  bar [1, 2, 3, 4]\n  line [4, 3, 2, 1]'),
  xy('  x-axis 5 --> 5\n  bar [1, 2, 3]'),
  xy('  x-axis 10 --> 0\n  line [1, 2, 3]\n  bar [3, 2, 1]'),
  xy('  y-axis 100 --> 0\n  line [10, 20, 30]\n  bar [30, 20, 10]'),
  xy('  y-axis 5 --> 5\n  line [10, 20, 30]'),
  xy('  line [0, 0, 0]'),
  xy('  bar [0, 0, 0]'),
  xy('  line [7]\n  line [1, 2, 3]'),
  xy('  y-axis "Title only"\n  x-axis "X title only"\n  line [3, 1, 2]'),
  xy('  x-axis "X" [a, b]\n  x-axis "Y" 1 --> 2\n  line [3, 1]'),
  xy('  line [3, 1]\n  x-axis "late" [a, b]\n  y-axis "late" 0 --> 9'),
  xy('  title "T"\n  title "Second title wins"\n  line [3, 1]'),
  xy('  title Plain words are joined\n  line [3, 1]'),
  xy('  title "  Padded title  "\n  x-axis "  padded  " [ "  a  ", b ]\n  line "  padded plot  " [3 "  padded label  ", 1]'),
  xy('  accTitle: The accessible title\n  accDescr: The accessible description\n  line [3, 1]'),
  xy('  accDescr {\n    A description\n    on two lines\n  }\n  line [3, 1]'),
  xy('  title "A <b>bold</b> & <script>alert(1)</script> title"\n  x-axis ["a<b", "c>d", "e&f"]\n  line "x<y" [3 "p<q", 1, 2]'),
  xy('  title "Entities #35; #quot; and ;"\n  line [3, 1]'),
  xy('  x-axis [a.b, c-d, e_f, g+h, i=j, k*l, m&n, 1x2]\n  bar [1, 2, 3, 4, 5, 6, 7, 8]'),
  xy('  x-axis ["`markdown`", `"md string"`]\n  bar [1, 2]'),
  xy('  line [+1.5, -.5, .25, 3.]\n'),
  xy('  line [1e3]'),
  xy('  line []'),
  xy('  line [1, 2,]'),
  xy('  line 1, 2'),
  xy('  x-axis [a, b\n  line [1, 2]'),
  xy('  x-axis 1 -> 2\n  line [1, 2]'),
  xy('  unknown [1, 2]'),
  'xychart-beta\n  line [1, 2]\n',
  'xychart horizontal\n  title "Horizontal keyword"\n  x-axis [a, b, c]\n  bar "b" [3, 1, 2]\n  line "l" [1, 2, 3]\n',
  'xychart vertical\n  bar [3, 1, 2]\n',
  'xychart-beta HORIZONTAL\n  bar [3, 1, 2]\n',
  'XYCHART\n  TITLE "Upper case"\n  X-AXIS [a, b]\n  BAR [1, 2]\n  LINE [2, 1]\n',
  '---\ntitle: Front matter title\n---\nxychart\n  line [1, 2]\n',
  '---\ntitle: Front matter title\n---\nxychart\n  title "Body title"\n  line [1, 2]\n',
  'xychart; title "Semi"; x-axis [a, b]; bar [1, 2]\n',
  // The legend, with data labels on the markers.
  xy(complete),
  xy(complete, init({ xyChart: { showDataLabel: true } })),
  xy(complete, init({ xyChart: { showDataLabel: true, chartOrientation: 'horizontal' } })),
  xy(complete, init({ xyChart: { showLegend: false } })),
  xy(complete, init({ xyChart: { legendFontSize: 30, legendPadding: 0 } })),
  xy(complete, init({ xyChart: { legendFontSize: 200 } })),
  xy('  x-axis [a, b]\n  bar "one" [1, 2]\n  bar "two" [2, 1]\n  bar "three" [3, 3]', init({ xyChart: { showDataLabel: true } })),
  xy('  x-axis [a, b]\n  bar "one" [1, 2]\n  bar "two" [2, 1]', init({ xyChart: { showDataLabel: true } })),
  xy('  x-axis [a, b, c]\n  bar [1, 2]', init({ xyChart: { showDataLabel: true } })),
  xy('  line [1, 2]\n  bar [1, 2, 3]', init({ xyChart: { showDataLabel: true } })),
  xy('  bar [1, 2, 3]\n  bar [4, 5]', init({ xyChart: { showDataLabel: true } })),
  xy('  bar [0, -1, 2]', init({ xyChart: { showDataLabel: true } })),
  xy('  bar [0, -1, 2]', init({ xyChart: { showDataLabel: true, chartOrientation: 'horizontal' } })),
  xy('  bar [123456789, 2, 30000]', init({ xyChart: { showDataLabel: true, showDataLabelOutsideBar: true } })),
  // Config values of other types.
  xy(complete, init({ xyChart: { width: '700', height: '500' } })),
  xy(complete, init({ xyChart: { titleFontSize: '18', titlePadding: '4' } })),
  xy(complete, init({ xyChart: { legendFontSize: '12' } })),
  xy(complete, init({ xyChart: { legendPadding: '6' } })),
  xy(complete, init({ xyChart: { xAxis: { labelFontSize: '12', titleFontSize: '18' }, yAxis: { labelFontSize: '18', titleFontSize: '12' } } })),
  xy(complete, init({ xyChart: { xAxis: { labelPadding: '8' }, yAxis: { labelPadding: '8' } } })),
  xy(complete, init({ xyChart: { xAxis: { titlePadding: '8' }, yAxis: { titlePadding: '8' } } })),
  xy(complete, init({ xyChart: { xAxis: { tickLength: '8', axisLineWidth: '4', tickWidth: '3' }, yAxis: { tickLength: '8', axisLineWidth: '4', tickWidth: '3' } } })),
  xy(complete, init({ xyChart: { chartOrientation: 'horizontal', xAxis: { titlePadding: '8', labelPadding: '8' }, yAxis: { titlePadding: '8', labelPadding: '8', tickLength: '8', axisLineWidth: '4' } } })),
  xy(complete, init({ xyChart: { xAxis: { labelRotation: '45' } } })),
  xy(complete, init({ xyChart: { xAxis: { labelRotation: 'abc' } } })),
  xy(complete, init({ xyChart: { showTitle: 'false', showLegend: 0, showDataLabel: 'yes' } })),
  xy(complete, init({ xyChart: { showTitle: false } })),
  xy(complete, init({ xyChart: { titleFontSize: null } })),
  xy(complete, init({ xyChart: { xAxis: null } })),
  xy(complete, init({ xyChart: { yAxis: 5 } })),
  xy(complete, init({ xyChart: { xAxis: { showLabel: null, labelFontSize: null } } })),
  xy(complete, init({ xyChart: { width: 'wide' } })),
  xy(complete, init({ xyChart: { height: null } })),
  xy(complete, init({ xyChart: { chartOrientation: 'diagonal' } })),
  xy(complete, init({ xyChart: { plotReservedSpacePercent: 100 } })),
  xy(complete, init({ xyChart: { plotReservedSpacePercent: 0 } })),
  xy(complete, init({ xyChart: { plotReservedSpacePercent: 250 } })),
  xy(complete, init({ xyChart: { width: 700.5, height: 333.25 } })),
  xy(complete, init({ xyChart: { useMaxWidth: false } })),
  // The theme.
  xy(complete, init({ theme: 'dark' })),
  xy(complete, init({ theme: 'forest' })),
  xy(complete, init({ theme: 'neutral' })),
  xy(complete, init({ theme: 'base', themeVariables: { primaryColor: '#ffcc00' } })),
  xy(complete, init({ themeVariables: { xyChart: { plotColorPalette: '#111111' } } })),
  xy(complete, init({ themeVariables: { xyChart: { plotColorPalette: ' red ,, blue , ' } } })),
  xy(complete, init({ themeVariables: { xyChart: { plotColorPalette: '' } } })),
  xy(complete, init({ themeVariables: { xyChart: { legendTextColor: '#123456', titleColor: 'red', backgroundColor: 'transparent' } } })),
  xy(complete, init({ fontFamily: 'courier', fontSize: 20 })),
  xy(complete, init({ themeVariables: { fontFamily: 'verdana' } })),
  // The rotation of the labels.
  xy(complete, init({ xyChart: { xAxis: { labelRotation: 90 } } })),
  xy(complete, init({ xyChart: { xAxis: { labelRotation: -90 } } })),
  xy(complete, init({ xyChart: { xAxis: { labelRotation: 33.3 } } })),
  xy(complete, init({ xyChart: { yAxis: { labelRotation: 45 } } })),
  xy(complete, init({ xyChart: { chartOrientation: 'horizontal', xAxis: { labelRotation: 45 }, yAxis: { labelRotation: 45 } } })),
  // Text that needs font fallback or shaping.
  xy('  title "Übergrößenträger – naïve café"\n  x-axis ["日本語", "한국어", "العربية", "emoji 😀"]\n  bar [1, 2, 3, 4]'),
  xy('  title "ffi fl office AVATAR To. Wa"\n  x-axis ["ffi", "AV", "To"]\n  line "Tj fi" [1, 2, 3]'),
];

const cases = [];
const rnd = prng(0x78796368);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, f), 'utf8');
  cases.push({ name: f, corpus: true, text });
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}
for (let i = 0; i < randomConfigs; i++) {
  const body = bodies[i % bodies.length];
  cases.push({ name: `config${i}`, text: init({ xyChart: randomConfig(rnd) }) + body });
}
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'xydb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text }) => ({ name, text })) }));
execFileSync(
  process.execPath,
  ['oracle.mjs', 'eval', 'xychart-db-page.js', '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
// The text of a corpus fixture is read from the corpus by the test.
const golden = results.map((r, i) => (cases[i].corpus ? { ...r } : { ...r, text: cases[i].text }));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
console.log(`${outFile}: ${golden.length} cases, ${errors} with an error`);
