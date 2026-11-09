#!/usr/bin/env node
// Selects the sample of corpus diagrams for the comparison of the PNG
// rasterizer with Chromium on Linux (cmd/pixelparity -fonts embedded).
//
// The input is the Linux reference SVG of each fixture
// (.parity/ref-linux/mermaid-e2e). The sample has files of every diagram
// family, and in each family it has files with every text feature that the
// family has: SVG text, HTML labels, text under a quarter turn, text under
// another angle, an opacity below 1, a clip path, bold, italic, small and
// large font sizes, decorations and filters.
//
// Usage, from the repository root:
//
//   node tools/reference/gen-raster-linux-sample.mjs
//
// It writes into .parity/raster/linux:
//
//   sample.rel      paths relative to the reference directory (for -list)
//   sample-s2.rel   the part of the sample that is also shot at scale 2
//   sample.tsv      the features of each file of the sample
//   shoot-N.list    absolute paths for oracle-linux.mjs shoot, at most 150
//   shoot-s2-N.list the same for the scale 2 part
//
// The screenshots are then made with:
//
//   node tools/reference/oracle-linux.mjs --fonts liberation shoot --quiet \
//       --out .parity/shots/linux-ref-s1 --base .parity/ref-linux/mermaid-e2e \
//       --list .parity/raster/linux/shoot-N.list
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '../..');
const REF = path.join(ROOT, '.parity/ref-linux/mermaid-e2e');
const OUT = path.join(ROOT, '.parity/raster/linux');
const PER_RUN = 150;
// The largest bitmap of a sample file at scale 1, in pixels.
const MAX_PIXELS = 6e6;

// quota is the number of files of a family with n fixtures. The families
// whose text has special cases get more.
const EXTRA = { gantt: 14, xychart: 12, timeline: 10, treemap: 10, git: 26, sequence: 34 };
function quota(family, n) {
  const base = Math.max(Math.min(n, 6), Math.min(40, Math.round(n * 0.16)));
  return Math.min(n, Math.max(base, EXTRA[family] || 0));
}

function walk(dir, out) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      walk(p, out);
    } else if (e.name.endsWith('.svg')) {
      out.push(path.relative(REF, p));
    }
  }
  return out;
}

function features(svg) {
  const f = new Set();
  if (svg.includes('<text')) f.add('text');
  if (svg.includes('<foreignObject')) f.add('html');
  if (svg.includes('<tspan')) f.add('tspan');
  for (const m of svg.matchAll(/rotate\((-?[\d.]+)/g)) {
    const a = ((Number(m[1]) % 360) + 360) % 360;
    if (a === 0) continue;
    f.add(a % 90 === 0 ? 'quarter' : 'angle');
  }
  if (/[^-a-z]opacity\s*[:=]\s*"?0?\.\d/.test(svg)) f.add('opacity');
  if (svg.includes('clip-path')) f.add('clip');
  if (/font-weight\s*[:=]\s*"?\s*(bold|[6-9]00)/.test(svg) || /<(b|strong)>/.test(svg)) f.add('bold');
  if (/font-style\s*[:=]\s*"?\s*italic/.test(svg) || /<(i|em)>/.test(svg)) f.add('italic');
  for (const m of svg.matchAll(/font-size\s*[:=]\s*"?\s*([\d.]+)(px)?/g)) {
    const s = Number(m[1]);
    if (s > 0 && s < 11) f.add('small');
    if (s > 24) f.add('large');
  }
  if (svg.includes('text-decoration')) f.add('decoration');
  if (/filter\s*[:=]\s*"?url/.test(svg)) f.add('filter');
  if (svg.includes('dominant-baseline')) f.add('baseline');
  if (svg.includes('mix-blend-mode')) f.add('blend');
  return f;
}

// pixels is the size of the bitmap at scale 1, from the viewBox.
function pixels(svg) {
  const m = /viewBox="([^"]+)"/.exec(svg);
  if (!m) return 0;
  const v = m[1].trim().split(/[\s,]+/).map(Number);
  return v.length === 4 ? v[2] * v[3] : 0;
}

const all = walk(REF, []).sort();
const byFamily = new Map();
for (const rel of all) {
  const family = rel.split('/')[0];
  if (!byFamily.has(family)) byFamily.set(family, []);
  byFamily.get(family).push(rel);
}

// The files of the first sample stay in the sample.
const keep = new Set();
const first = path.join(OUT, 'corpus.rel');
if (fs.existsSync(first)) {
  for (const line of fs.readFileSync(first, 'utf8').split('\n')) {
    if (line.trim()) keep.add(line.trim());
  }
}

const sample = [];
const rows = [];
for (const [family, files] of [...byFamily].sort()) {
  const info = new Map();
  for (const rel of files) {
    const svg = fs.readFileSync(path.join(REF, rel), 'utf8');
    info.set(rel, { f: features(svg), px: pixels(svg) });
  }
  const usable = files.filter((rel) => keep.has(rel) || info.get(rel).px <= MAX_PIXELS);
  const want = quota(family, usable.length);
  const picked = new Set(usable.filter((rel) => keep.has(rel)));
  // Two files for each feature of the family, spread over the family.
  const names = new Set();
  for (const rel of usable) for (const n of info.get(rel).f) names.add(n);
  for (const name of [...names].sort()) {
    const withIt = usable.filter((rel) => info.get(rel).f.has(name));
    let have = withIt.filter((rel) => picked.has(rel)).length;
    for (const i of [Math.floor(withIt.length / 3), Math.floor((2 * withIt.length) / 3)]) {
      if (have >= 2) break;
      if (!picked.has(withIt[i])) {
        picked.add(withIt[i]);
        have++;
      }
    }
  }
  // The rest: evenly spaced files.
  const rest = usable.filter((rel) => !picked.has(rel));
  const need = Math.max(0, want - picked.size);
  for (let i = 0; i < need && rest.length > 0; i++) {
    picked.add(rest[Math.floor(((i + 0.5) * rest.length) / need)]);
  }
  for (const rel of [...picked].sort()) {
    sample.push(rel);
    rows.push(`${rel}\t${Math.round(info.get(rel).px)}\t${[...info.get(rel).f].sort().join(',')}`);
  }
}

// Scale 2: every fifth file of each family, and at least two.
const s2 = [];
const count = new Map();
for (const rel of sample) {
  const family = rel.split('/')[0];
  const n = count.get(family) || 0;
  count.set(family, n + 1);
  if (n % 5 === 0 || n === 1) s2.push(rel);
}

fs.mkdirSync(OUT, { recursive: true });
for (const name of fs.readdirSync(OUT)) {
  if (/^shoot-.*\.list$/.test(name)) fs.rmSync(path.join(OUT, name));
}
const write = (name, lines) => fs.writeFileSync(path.join(OUT, name), lines.join('\n') + '\n');
write('sample.rel', sample);
write('sample-s2.rel', s2);
write('sample.tsv', rows);
const chunks = (prefix, list) => {
  for (let i = 0; i * PER_RUN < list.length; i++) {
    write(`${prefix}-${i + 1}.list`, list.slice(i * PER_RUN, (i + 1) * PER_RUN).map((rel) => path.join(REF, rel)));
  }
};
chunks('shoot', sample);
chunks('shoot-s2', s2);
process.stderr.write(`sample: ${sample.length} files of ${byFamily.size} families, ${s2.length} at scale 2\n`);
