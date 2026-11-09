// Emits the values of Math.cos and Math.sin that Chromium computed, as far
// as the reference renders in .parity/ref show them without rounding:
// - the label positions of the pie charts: arc.centroid gives
//   [cos(a) * r, sin(a) * r];
// - the axis lines of the radar charts: x2 = f * cos(a), y2 = f * sin(a).
//
// Node.js does not always return the same values as Chromium for Math.sin
// and Math.cos, so this file, not node, is the oracle for js.SinCR and
// js.CosCR. The script only reads the reference renders.
//
// It also writes the geometry of the pie charts for internal/d3/shape: the
// values of each chart, with the slice paths and the label positions that
// Chromium rendered.
//
// Usage: node gen-d3-chromium-trig.mjs [directory of the reference renders]
// The default directory is .parity/ref.
import { readFileSync, readdirSync, existsSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { hex } from './d3-golden-lib.mjs';
import { arc, pie } from 'd3-shape';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const refDir = resolve(process.argv[2] ?? join(root, '.parity/ref'));
const pies = [];
const walk = (dir) => readdirSync(dir).flatMap((f) => { const p = join(dir, f); return statSync(p).isDirectory() ? walk(p) : [p]; });
const cases = [];
const seen = new Set();
const add = (src, angle, f, x, y) => {
  const key = hex(angle) + hex(f);
  if (seen.has(key)) return;
  seen.add(key);
  cases.push({ src, a: hex(angle), f: hex(f), x: hex(x), y: hex(y), nodeAgrees: Math.cos(angle) * f === x && Math.sin(angle) * f === y });
};

for (const mmd of walk(join(root, 'testdata/corpus/mermaid-e2e/pie')).filter((f) => f.endsWith('.mmd'))) {
  const svgPath = mmd.replace(join(root, 'testdata/corpus'), refDir).replace(/\.mmd$/, '.svg');
  if (!existsSync(svgPath)) continue;
  const src = readFileSync(mmd, 'utf8');
  const svg = readFileSync(svgPath, 'utf8');
  const values = [...src.matchAll(/^\s*"[^"]*"\s*:\s*([0-9.]+)\s*$/gm)].map((m) => +m[1]);
  if (values.length === 0) continue;
  const tp = /textPosition["']?\s*:\s*([0-9.]+)/.exec(src);
  const textPosition = tp ? +tp[1] : 0.75;
  const dh = /donutHole["']?\s*:\s*(-?[0-9.]+)/.exec(src);
  const donutHole = dh ? +dh[1] : 0;
  const innerHole = donutHole > 0 && donutHole <= 0.9 ? donutHole : 0;
  const sum = values.reduce((a, b) => a + b, 0);
  const data = values.filter((v) => (v / sum) * 100 >= 1);
  const r = (185 * textPosition + 185 * textPosition) / 2;
  {
    // The geometry of the whole chart, as pieRenderer.ts computes it. The
    // script keeps a chart only when node gives the slice paths of the
    // render, which shows that it read the values and the config correctly.
    const slices = pie().value((d) => d).sort(null)(data).filter((a) => ((a.data / sum) * 100).toFixed(0) !== '0');
    const paths = [...svg.matchAll(/<path d="([^"]*)"[^>]*class="pieCircle[^"]*"/g)].map((m) => m[1]);
    const labels = [...svg.matchAll(/<text transform="translate\(([^)]*)\)" class="slice"/g)].map((m) => m[1]);
    const gen = arc().innerRadius(innerHole * 185).outerRadius(185);
    if (paths.length === slices.length && labels.length === slices.length && slices.every((a, i) => gen(a) === paths[i])) {
      pies.push({ file: mmd.replace(join(root, 'testdata/corpus/'), ''), values, textPosition, innerHole, paths, labels });
    }
  }
  const transforms = [...svg.matchAll(/translate\(([^)]*)\)/g)].map((m) => m[1].split(',').map(Number)).filter((t) => t.length === 2);
  for (const a of pie().value((d) => d).sort(null)(data)) {
    const angle = (a.startAngle + a.endAngle) / 2 - Math.PI / 2;
    const x = Math.cos(angle) * r, y = Math.sin(angle) * r;
    // The label of this slice is the transform that is nearest to node's value.
    const best = transforms.map((t) => [Math.hypot(t[0] - x, t[1] - y), t]).sort((p, q) => p[0] - q[0])[0];
    if (best && best[0] < 1e-6) add('pie', angle, r, best[1][0], best[1][1]);
  }
}

const radarDir = join(refDir, 'mermaid-e2e/radar');
for (const svgPath of existsSync(radarDir) ? walk(radarDir).filter((f) => f.endsWith('.svg')) : []) {
  const svg = readFileSync(svgPath, 'utf8');
  const lines = [...svg.matchAll(/<line x1="0" y1="0" x2="([^"]*)" y2="([^"]*)" class="radarAxisLine"/g)].map((m) => [+m[1], +m[2]]);
  const n = lines.length;
  if (n < 2) continue;
  // The first axis points up: sin(-pi/2) is exactly -1, so y2 = -f.
  const f = -lines[0][1];
  for (let i = 0; i < n; i++) add('radar', (2 * i * Math.PI) / n - Math.PI / 2, f, lines[i][0], lines[i][1]);
}

const out = join(root, 'internal/js/testdata/d3_chromium_trig.json');
writeFileSync(out, JSON.stringify({ cases }, null, 0) + '\n');
console.log(`${out}: ${cases.length} cases, node agrees with Chromium in ${cases.filter((c) => c.nodeAgrees).length}`);

const pieOut = join(root, 'internal/d3/shape/testdata/chromium_pie.json');
writeFileSync(pieOut, JSON.stringify({ pies }, null, 0) + '\n');
console.log(`${pieOut}: ${pies.length} pie charts, ${pies.reduce((n, p) => n + p.labels.length, 0)} slices`);
