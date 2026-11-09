#!/usr/bin/env node
// Emits internal/rough/testdata/chromium.json: rough shapes from the
// reference SVGs that mermaid 12.2.0 made in headless Chromium (.parity/ref).
//
// Run it in tools/reference:
//
//   node gen-rough-chromium.mjs
//
// The other golden file of internal/rough comes from roughjs in Node.js.
// This file ties the port to the real target. For a shape in a reference
// SVG, the script finds the call that mermaid made:
//
//   - A stadium node of a look that is not handDrawn: rc.path(d, options)
//     with roughness 0. The path data is in the output, because the end
//     point of each segment is exact.
//   - A rectangle node of the handDrawn look in a flowchart:
//     rc.rectangle(x, y, w, h, options). The size comes from the size of
//     the label.
//
// With the seed option 0, roughjs draws from Math.random, which is one
// seeded stream for the page. The script looks for the position in the
// stream at which roughjs in Node.js makes the same path strings as
// Chromium. It records only the shapes that it reproduces byte for byte,
// with the position. The Go test then makes the same call at the same
// position.
//
// The script prints how many shapes it finds and how many it reproduces. A
// shape with a seed that is not 0 does not depend on Math.random.
import { createHash } from 'node:crypto';
import { readFileSync, readdirSync, statSync, writeFileSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import rough from './node_modules/roughjs/bundled/rough.esm.js';
import { install as installFdlibm } from './rough-fdlibm.mjs';

installFdlibm();

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REF = path.join(HERE, '../../.parity/ref');
const CORPUS = path.join(HERE, '../../testdata/corpus');
const OUT = path.join(HERE, '../../internal/rough/testdata/chromium.json');

// Math.random: the PRNG of oracle.mjs, with a position that the script sets.
const RANDOM_SEED = 0x6d6d6467;
let randState = RANDOM_SEED >>> 0;
let randCalls = 0;
Math.random = function () {
  randCalls++;
  randState = (randState + 0x6d2b79f5) >>> 0;
  let t = randState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const seek = (n) => {
  randState = (RANDOM_SEED + Math.imul(n, 0x6d2b79f5)) >>> 0;
  randCalls = 0;
};

const fakeDoc = {
  createElementNS(_ns, name) {
    return {
      name,
      attrs: [],
      children: [],
      setAttribute(k, v) {
        this.attrs.push([k, String(v)]);
      },
      appendChild(c) {
        this.children.push(c);
      },
    };
  },
};
const newSvg = () => rough.svg({ ownerDocument: fakeDoc });

function* walk(dir) {
  for (const e of readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.svg')) yield p;
  }
}

// The attributes that RoughSVG.draw sets. mermaid adds other attributes
// (style, class) to the paths afterwards.
const ROUGH_ATTRS = new Set(['d', 'stroke', 'stroke-width', 'fill', 'stroke-dasharray', 'stroke-dashoffset', 'fill-rule']);
const parsePaths = (html) => [...html.matchAll(/<path ([^>]*)>/g)].map((m) => [...m[1].matchAll(/([\w-]+)="([^"]*)"/g)].map((a) => [a[1], a[2]]).filter((a) => ROUGH_ATTRS.has(a[0])));
const attr = (attrs, name) => (attrs.find((a) => a[0] === name) || [])[1];
const sameAttrs = (a, b) => JSON.stringify(a) === JSON.stringify(b);
const strokeWidth = (s) => (s === '1.3' ? 1.3 : s);
const dash = (s) => (s === undefined ? undefined : s.split(' ').map(Number));

// run makes the call at the given stream position and returns the path
// elements.
function run(shape, position) {
  seek(position);
  const svg = newSvg();
  const g = shape.f === 'path' ? svg.path(shape.a[0], shape.o) : svg.rectangle(...shape.a, shape.o);
  return { paths: g.children.map((p) => p.attrs), draws: randCalls };
}

// findPosition returns the stream position at which the call reproduces the
// expected paths, or -1.
function findPosition(shape, expected, limit) {
  for (let k = 0; k < limit; k++) {
    if (sameAttrs(run(shape, k).paths, expected)) return k;
  }
  return -1;
}

const handDrawnSeed = (svgFile) => {
  const mmd = path.join(CORPUS, path.relative(REF, svgFile)).replace(/\.svg$/, '.mmd');
  if (!existsSync(mmd)) return 0;
  const m = readFileSync(mmd, 'utf8').match(/handDrawnSeed:\s*(\d+)/);
  return m ? Number(m[1]) : 0;
};

// The first draws of the stream, for the search of a stadium.
seek(0);
const stream = Float64Array.from({ length: 1000000 }, () => Math.random());

const shapes = [];
const stats = { stadium: { found: 0, reproduced: 0 }, rectangle: { found: 0, reproduced: 0 } };
const seen = new Set();
const LIMIT = { stadium: 8, rectangle: 8 };
// The number of candidates for the search, for each kind.
const SEARCH = { stadium: 40, rectangle: 12 };

for (const file of walk(REF)) {
  const html = readFileSync(file, 'utf8');
  const source = path.relative(REF, file);

  // Stadium nodes that are not handDrawn.
  for (const m of html.matchAll(/<g class="basic label-container outer-path"[^>]*>((?:<path [^>]*><\/path>)+)<\/g>/g)) {
    const paths = parsePaths(m[1]);
    if (paths.length !== 2 || attr(paths[0], 'stroke') !== 'none') continue;
    const fill = attr(paths[0], 'd');
    const segs = fill.split(' C');
    if (segs.length !== 104 || fill.indexOf('M', 1) >= 0) continue;
    const start = segs[0].slice(1).trim().split(' ');
    const ends = segs.slice(1).map((c) => c.split(', ')[2].trim().split(' '));
    // The last segment is the Z command.
    const d = [`M${start[0]},${start[1]}`, ...ends.slice(0, 102).map((p) => `L${p[0]},${p[1]}`), 'Z'].join(' ');
    const o = { roughness: 0, fill: attr(paths[0], 'fill'), fillStyle: 'solid', fillWeight: 1.5, hachureGap: 1.5, stroke: attr(paths[1], 'stroke'), seed: 0, strokeWidth: strokeWidth(attr(paths[1], 'stroke-width')), fillLineDash: [0, 0], strokeLineDash: dash(attr(paths[1], 'stroke-dasharray')) };
    const shape = { f: 'path', a: [d], o };
    const key = `stadium ${start.join(' ')} ${JSON.stringify(o)}`;
    if (seen.has(key) || stats.stadium.reproduced >= LIMIT.stadium || stats.stadium.found >= SEARCH.stadium) continue;
    seen.add(key);
    stats.stadium.found++;
    // The stroke shape draws first: 103 segments with 22 draws each. The
    // first draw of the fill shape is the diverge point of its first
    // segment: x = x1 + (x2 - x1) * (0.2 + random * 0.2).
    const x1 = Number(start[0]);
    const x2 = Number(ends[0][0]);
    const cx = Number(segs[1].split(', ')[0].trim().split(' ')[0]);
    const want = ((cx - x1) / (x2 - x1) - 0.2) / 0.2;
    let position = -1;
    for (let k = 103 * 22; k < stream.length; k++) {
      if (Math.abs(stream[k] - want) < 1e-9 && sameAttrs(run(shape, k - 103 * 22).paths, paths)) {
        position = k - 103 * 22;
        break;
      }
    }
    if (position < 0) continue;
    stats.stadium.reproduced++;
    shapes.push({ kind: 'stadium', source, ...shape, position, draws: run(shape, position).draws, paths });
  }

  // Rectangle nodes of the handDrawn look in a flowchart.
  if (!source.includes('flowchart')) continue;
  for (const m of html.matchAll(/<g class="rough-node default"[^>]*data-look="handDrawn"[^>]*><g class="basic label-container" style="">((?:<path [^>]*><\/path>)+)<\/g><g class="label"[^>]*><rect><\/rect><foreignObject width="([\d.]+)" height="([\d.]+)"/g)) {
    const paths = parsePaths(m[1]);
    if (paths.length !== 2) continue;
    // squareRect.ts and drawRect.ts with the padding 15 of a flowchart.
    const w = Number(m[2]) + 15 * 2 * 2;
    const h = Number(m[3]) + 15 * 2;
    const seed = handDrawnSeed(file);
    const o = { roughness: 0.7, fill: attr(paths[0], 'stroke'), fillStyle: 'hachure', fillWeight: 1.5, hachureGap: 1.5, stroke: attr(paths[1], 'stroke'), seed, strokeWidth: strokeWidth(attr(paths[1], 'stroke-width')), fillLineDash: dash(attr(paths[0], 'stroke-dasharray')), strokeLineDash: dash(attr(paths[1], 'stroke-dasharray')) };
    const shape = { f: 'rectangle', a: [-w / 2, -h / 2, w, h], o };
    const key = `rectangle ${w} ${h} ${JSON.stringify(o)}`;
    if (seen.has(key) || stats.rectangle.reproduced >= LIMIT.rectangle || (seed === 0 && stats.rectangle.found >= SEARCH.rectangle)) continue;
    seen.add(key);
    stats.rectangle.found++;
    const position = seed ? (sameAttrs(run(shape, 0).paths, paths) ? 0 : -1) : findPosition(shape, paths, 2000);
    if (position < 0) continue;
    stats.rectangle.reproduced++;
    shapes.push({ kind: 'rectangle', source, ...shape, position, draws: run(shape, position).draws, paths });
  }
}

// The golden file holds the length, a hash and the start of each path string.
const hashStr = (s) => createHash('sha256').update(s, 'utf8').digest('hex').slice(0, 20);
for (const s of shapes) {
  s.paths = s.paths.map((attrs) => attrs.map(([k, v]) => (k === 'd' ? [k, `${v.length}:${hashStr(v)}:${v.slice(0, 120)}`] : [k, v])));
}
writeFileSync(OUT, JSON.stringify({ meta: { reference: 'mermaid 12.2.0 in headless Chromium (.parity/ref)', randomSeed: RANDOM_SEED }, shapes }, null, 1) + '\n');
console.error(`${shapes.length} shapes written. Shapes found and reproduced: ${JSON.stringify(stats)}`);
for (const s of shapes) console.error(`${s.kind} position ${s.position} draws ${s.draws} ${s.source}`);
