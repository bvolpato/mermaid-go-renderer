// Writes the golden files of internal/venn: results of @upsetjs/venn.js
// 2.0.0, the version that mermaid 12.2.0 bundles, for seeded random inputs.
//
//   node gen-venn.mjs              internal/venn/testdata/venn.json.gz
//   node gen-venn.mjs --chromium   internal/venn/testdata/venn-chromium.json.gz
//
// Without a flag the script runs build/venn.esm.js in node. The native Math
// functions of node are not a stable reference (the build for arm64 fuses
// operations), so the script installs the fdlibm functions of V8 written in
// JavaScript (elk-fdlibm.mjs) as Math.sin, cos, acos, atan2 and pow. The Go
// test gives the same functions to the port (internal/venn/fdlibm_ref_test.go)
// and the sort of node (js.V8TimSort).
//
// With --chromium the script runs the same cases, fewer of them, in the page
// of the browser oracle (esm-eval): the code is then the copy of venn.js in
// the vennDiagram chunk of mermaid, with the Math and the sort of Chromium.
// The Go test runs the port with its defaults for this file. The file also
// has cases of the chart of VennDiagram: the markup that it builds.
//
// Both runs use the seeded Math.random of the oracle, reset before each
// call. Every number is the hexadecimal text of the 64 bits of the double.
//
// A layout case keeps the circles after venn(), the scaled circles, the text
// centres and a SHA-256 digest of all other results (the normalized circles,
// the result of layout() with its arcs and paths, and the numbers of the
// chart). The first cases of each family keep all results in full.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const chromium = process.argv.includes('--chromium');
const RANDOM_SEED = 0x6d6d6467;

// ---- numbers ----
const view = new DataView(new ArrayBuffer(8));
const hex = (v) => {
  if (v !== v) {
    return '7ff8000000000000';
  }
  view.setFloat64(0, v);
  return view.getUint32(0).toString(16).padStart(8, '0') + view.getUint32(4).toString(16).padStart(8, '0');
};

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

// ---- case generation ----
const rnd = prng(0x76656e6e);
const int = (n) => Math.floor(rnd() * n);
const pick = (arr) => arr[int(arr.length)];
const uniform = (a, b) => a + (b - a) * rnd();
const logUniform = (a, b) => Math.exp(uniform(Math.log(a), Math.log(b)));
const chance = (p) => rnd() < p;
const shuffle = (arr) => {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = int(i + 1);
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
};

const idPools = [
  ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L'],
  ['Frontend', 'Backend', 'DevOps', 'Design', 'Data', 'Ops', 'QA', 'Infra', 'Mobile', 'Web', 'ML', 'Sec'],
  ['b', 'a', 'd', 'c', 'Z', 'Y', 'e', 'X', 'f', 'g', 'h', 'i'],
  ['2', '10', '1', 'A', '3', 'B', '0', '7', 'C', '21', 'D', '4'],
  ['a b', 'a,b', 'a;b', 'a_b', 'a|b', 'ünï', '😀', '"q"', '<b>', 'x&y', 'é', 'ß'],
];
const protoIds = ['constructor', 'toString', 'valueOf', 'hasOwnProperty'];

// area makes [sets, size, label, weight]. The db of mermaid sorts the ids
// of an area as strings.
const area = (sets, size, label = null, weight = null) => [
  sets.slice().sort(),
  hex(size),
  label,
  weight === null ? null : hex(weight),
];

const pairsOf = (ids) => {
  const out = [];
  for (let i = 0; i < ids.length; i++) {
    for (let j = i + 1; j < ids.length; j++) {
      out.push([ids[i], ids[j]]);
    }
  }
  return out;
};
const triplesOf = (ids) => {
  const out = [];
  for (let i = 0; i < ids.length; i++) {
    for (let j = i + 1; j < ids.length; j++) {
      for (let k = j + 1; k < ids.length; k++) {
        out.push([ids[i], ids[j], ids[k]]);
      }
    }
  }
  return out;
};

// ensurePairwiseSubsets of vennRenderer.ts, on areas with numbers.
function ensurePairwiseSubsets(subsets) {
  const existingKeys = new Set(subsets.map((s) => [...s.sets].sort().join('|')));
  const individualSetSizes = new Map(subsets.filter((s) => s.sets.length === 1).map((s) => [s.sets[0], s.size]));
  const synthetic = [];
  for (const subset of subsets) {
    if (subset.sets.length < 3) {
      continue;
    }
    const members = [...subset.sets].sort();
    for (let i = 0; i < members.length - 1; i++) {
      for (let j = i + 1; j < members.length; j++) {
        const pair = [members[i], members[j]];
        const key = pair.join('|');
        if (!existingKeys.has(key)) {
          existingKeys.add(key);
          const sizeA = individualSetSizes.get(pair[0]);
          const sizeB = individualSetSizes.get(pair[1]);
          const pairSize = sizeA !== undefined && sizeB !== undefined ? Math.min(sizeA, sizeB) / 4 : 2.5;
          synthetic.push({ sets: pair, size: pairSize, label: '' });
        }
      }
    }
  }
  return synthetic.length > 0 ? [...subsets, ...synthetic] : subsets;
}

const defaultSize = (n) => 10 / Math.pow(n, 2);
const labels = [null, null, null, 'Label', 'Two words', 'A much longer label of many words', ''];

function ids(n, pool) {
  return shuffle(pool.slice()).slice(0, n);
}

// The data that the renderer of mermaid gives for a diagram text.
function mermaidAreas(n, pool) {
  const names = ids(n, pool);
  const subsets = [];
  for (const id of names) {
    const size = chance(0.25) ? pick([20, 12.5, 5, 30, 1, 100, 0.5, 15]) : defaultSize(1);
    subsets.push({ sets: [id], size, label: pick(labels) ?? undefined });
  }
  const unions = [];
  for (const p of pairsOf(names)) {
    if (chance(0.55)) {
      unions.push({ sets: p, size: chance(0.2) ? pick([1, 3, 5, 0.25, 8, 0]) : defaultSize(2), label: pick(labels) ?? undefined });
    }
  }
  if (n >= 3) {
    for (const t of triplesOf(names)) {
      if (chance(n === 3 ? 0.6 : 0.2)) {
        unions.push({ sets: t, size: chance(0.2) ? pick([1, 0.5, 2]) : defaultSize(3), label: pick(labels) ?? undefined });
      }
    }
  }
  if (n >= 4 && chance(0.3)) {
    unions.push({ sets: names.slice(0, 4), size: defaultSize(4), label: undefined });
  }
  if (chance(0.3)) {
    shuffle(unions);
  }
  const all = subsets.concat(unions).map((s) => ({ ...s, sets: s.sets.slice().sort() }));
  return ensurePairwiseSubsets(all).map((s) => area(s.sets, s.size, s.label === undefined ? null : s.label));
}

// Sets with random sizes, and overlaps from disjoint to contained.
function randomAreas(n, pool, pairChance, tripleChance) {
  const names = ids(n, pool);
  const size = new Map();
  const out = [];
  for (const id of names) {
    size.set(id, logUniform(0.5, 200));
    out.push(area([id], size.get(id), pick(labels)));
  }
  for (const p of pairsOf(names)) {
    if (chance(pairChance)) {
      const m = Math.min(size.get(p[0]), size.get(p[1]));
      const s = pick([() => m * rnd(), () => m * rnd(), () => m * rnd() * 0.3, () => 0, () => m, () => m * 1.2, () => m * uniform(0.95, 1.05), () => 1e-12])();
      out.push(area(p, s, pick(labels)));
    }
  }
  if (n >= 3) {
    for (const t of triplesOf(names)) {
      if (chance(tripleChance)) {
        const m = Math.min(...t.map((id) => size.get(id)));
        out.push(area(t, m * rnd() * 0.3, pick(labels)));
      }
    }
  }
  return chance(0.2) ? shuffle(out) : out;
}

// Inputs for the branches that ordinary data does not reach.
function edgeAreas(i) {
  const cases = [
    () => [area(['A'], 10)],
    () => [area(['A'], 0)],
    () => [area(['A'], 0), area(['B'], 10)],
    () => [area(['A'], 0), area(['B'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A'], 0), area(['B'], 0)],
    () => [area(['A'], 10), area(['A'], 10)],
    () => [area(['A'], 10), area(['A'], 20), area(['B'], 5), area(['A', 'B'], 2.5)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'A'], 2.5)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B'], 2.5), area(['A', 'B'], 5)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B'], -2.5)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B'], 10)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B'], 100)],
    () => [area(['A'], 10), area(['B'], 4), area(['A', 'B'], 4)],
    () => [area(['A'], 10), area(['B'], 4), area(['C'], 1), area(['A', 'B'], 4), area(['B', 'C'], 1), area(['A', 'C'], 1)],
    () => [area(['A'], Infinity), area(['B'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B'], Infinity)],
    () => [area(['A'], Infinity), area(['B'], Infinity), area(['C'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A'], -10), area(['B'], 10)],
    () => [area(['A'], -10), area(['B'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A'], NaN), area(['B'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A'], 1e300), area(['B'], 1e-300), area(['A', 'B'], 1e-301)],
    () => [area(['A'], 1e-300), area(['B'], 1e-300)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B'], 2.5, null, 3)],
    () => [area(['A'], 10), area(['B'], 10), area(['C'], 10), area(['A', 'B'], 2.5, null, 0), area(['B', 'C'], 2.5, null, 5), area(['A', 'B', 'C'], 1, null, 2)],
    () => [area(['A'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A', 'B'], 2.5)],
    () => [area(['A'], 10), area(['B'], 10), area(['A', 'B', 'C'], 1)],
    () => [],
    () => [area(['constructor'], 10, 'C'), area(['A'], 10), area(['B'], 10), area(['A', 'B'], 2.5)],
    () => [area(['A'], 10), area(['toString'], 10, 'T'), area(['B'], 10), area(['A', 'toString'], 2.5), area(['B', 'toString'], 2.5)],
    () => [area(['A'], 10), area(['B'], 10), area(['valueOf'], 5, 'V'), area(['hasOwnProperty'], 5, 'H'), area(['A', 'B'], 2.5)],
    () => [area(['constructor'], 10), area(['A'], 10)],
    () => [area(['__proto__'], 10), area(['A'], 10)],
    () => [area(['__proto__'], 10)],
    () => [area(['A'], 10), area(['A', 'A', 'B'], 1), area(['B'], 10)],
    () => [area(['2'], 10), area(['10'], 10), area(['1'], 10), area(['1', '10'], 2.5), area(['10', '2'], 2.5)],
    () => [area(['B'], 10), area(['A'], 10), area(['C'], 10), area(['A', 'B', 'C'], defaultSize(3))],
    () => [area(['A'], 10), area(['B'], 10), area(['C'], 10), area(['D'], 10), area(['A', 'B', 'C', 'D'], defaultSize(4))],
    () => [area(['A'], 10), area(['B'], 10), area(['C'], 10), area(['D'], 10), area(['E'], 10)],
    () => ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I'].map((id) => area([id], 10)),
    () => [area(['A'], 100), area(['B'], 1), area(['C'], 1), area(['D'], 1), area(['A', 'B'], 1), area(['A', 'C'], 1), area(['A', 'D'], 1)],
    () => [area(['A'], 10), area(['B'], 10), area(['C'], 10), area(['A', 'B'], 2.5), area(['A', 'C'], 2.5), area(['B', 'C'], 2.5), area(['A', 'B', 'C'], 1.1111111111111112)],
  ];
  return cases[i % cases.length]();
}
const edgeCount = 42;

const optionSets = [
  [800, 450, 8],
  [800, 450, 8],
  [800, 426, 8],
  [1600, 900, 8],
  [400, 300, 8],
  [600, 350, 15],
  [800, 450, 0],
  [300, 600, 40],
];
function options() {
  if (chance(0.15)) {
    return [uniform(100, 2000), uniform(100, 1500), pick([0, 8, 15, 40, 3.5])];
  }
  return pick(optionSets);
}

function layoutCase(family, i) {
  let areas;
  switch (family) {
    case 'mermaid':
      areas = mermaidAreas(1 + int(5), pick(idPools));
      break;
    case 'random':
      areas = randomAreas(1 + int(7), pick(idPools), 0.6, 0.15);
      break;
    case 'many':
      areas = chance(0.5) ? randomAreas(6 + int(7), idPools[int(4)], 0.3, 0.02) : mermaidAreas(6 + int(4), idPools[int(4)]);
      break;
    case 'disjoint':
      areas = randomAreas(2 + int(8), pick(idPools), 0.08, 0);
      break;
    case 'proto':
      areas = randomAreas(2 + int(4), protoIds.concat(['A', 'B', '1']), 0.6, 0.15).map((a) => [a[0], a[1], 'L', a[3]]);
      break;
    case 'edge':
      areas = edgeAreas(i);
      break;
  }
  const [width, height, padding] = options();
  return { kind: 'layout', family, areas, width: hex(width), height: hex(height), padding: hex(padding) };
}

const circle = (x, y, r) => [hex(x), hex(y), hex(r)];
function randomCircles(n) {
  const mode = int(8);
  const out = [];
  for (let i = 0; i < n; i++) {
    switch (mode) {
      case 0: // far apart
        out.push(circle(uniform(-100, 100), uniform(-100, 100), logUniform(0.5, 15)));
        break;
      case 1: // concentric
        out.push(circle(1.5, -2.5, logUniform(0.5, 15)));
        break;
      case 2: // identical
        out.push(circle(3, 4, 5));
        break;
      case 3: // on a line, touching
        out.push(circle(i * 10, 0, 5));
        break;
      case 4: // a flower around the origin
        out.push(circle(4 * Math.cos((2 * Math.PI * i) / n), 4 * Math.sin((2 * Math.PI * i) / n), uniform(4, 7)));
        break;
      case 5: // a radius of zero in the mix
        out.push(circle(uniform(-3, 3), uniform(-3, 3), i === 0 ? 0 : logUniform(1, 8)));
        break;
      default: // a cluster
        out.push(circle(uniform(-6, 6), uniform(-6, 6), logUniform(1, 12)));
    }
  }
  return out;
}

function scalarCase() {
  const fn = pick(['circleOverlap', 'circleOverlap', 'circleArea', 'distanceFromIntersectArea', 'distanceFromIntersectArea', 'circleCircleIntersection']);
  const r1 = pick([() => logUniform(0.01, 100), () => 1, () => 0, () => 5])();
  const r2 = pick([() => logUniform(0.01, 100), () => r1, () => 0, () => 2.5])();
  let values;
  switch (fn) {
    case 'circleOverlap':
      values = [r1, r2, pick([() => uniform(0, (r1 + r2) * 1.1), () => r1 + r2, () => Math.abs(r1 - r2), () => 0, () => Math.abs(r1 - r2) + 1e-9, () => r1 + r2 - 1e-9])()];
      break;
    case 'circleArea':
      values = [r1, pick([() => uniform(0, 2 * r1), () => 0, () => 2 * r1, () => r1, () => 2.1 * r1, () => -0.1])()];
      break;
    case 'distanceFromIntersectArea': {
      const m = Math.min(r1, r2) * Math.min(r1, r2) * Math.PI;
      values = [r1, r2, pick([() => m * rnd(), () => m * rnd(), () => 0, () => m, () => m * 1.1, () => m - 1e-11, () => -1, () => m * 1e-9])()];
      break;
    }
    default:
      values = [uniform(-5, 5), uniform(-5, 5), r1, uniform(-5, 5), uniform(-5, 5), r2];
  }
  return { kind: 'scalar', fn, values: values.map(hex) };
}

function textCentreCase() {
  const mode = int(6);
  const interior = [];
  const exterior = [];
  const ni = 1 + int(4);
  const ne = int(5);
  for (let i = 0; i < ni; i++) {
    if (mode === 0) {
      interior.push(circle(uniform(-50, 50), uniform(-50, 50), logUniform(1, 10))); // mostly disjoint
    } else if (mode === 1) {
      interior.push(circle(0, 0, logUniform(1, 10))); // concentric
    } else {
      interior.push(circle(uniform(-4, 4), uniform(-4, 4), logUniform(3, 12)));
    }
  }
  for (let i = 0; i < ne; i++) {
    if (mode === 2) {
      exterior.push(circle(uniform(-1, 1), uniform(-1, 1), logUniform(20, 40))); // covers the interior
    } else {
      exterior.push(circle(uniform(-10, 10), uniform(-10, 10), logUniform(1, 10)));
    }
  }
  return { kind: 'textCentre', interior, exterior };
}

function normalizeCase() {
  const n = 1 + int(10);
  const names = ids(n, pick(idPools));
  const mode = int(5);
  const solution = names.map((id, i) => {
    switch (mode) {
      case 0: // one cluster
        return [id, ...circle(uniform(-3, 3), uniform(-3, 3), logUniform(2, 8))];
      case 1: // all disjoint
        return [id, ...circle(i * 100, uniform(-5, 5), logUniform(1, 20))];
      case 2: // two circles, one inside the other
        return [id, ...circle(i * 0.1, 0, i === 0 ? 10 : uniform(1, 3))];
      case 3: // equal radii
        return [id, ...circle(uniform(-20, 20), uniform(-20, 20), 4)];
      default:
        return [id, ...circle(uniform(-30, 30), uniform(-30, 30), logUniform(1, 12))];
    }
  });
  const [width, height, padding] = options();
  return { kind: 'normalize', solution, width: hex(width), height: hex(height), padding: hex(padding) };
}

function initialCase(i) {
  const family = pick(['mermaid', 'random', 'many', 'many', 'disjoint', 'edge']);
  return { kind: 'initial', family, areas: layoutCase(family, i).areas };
}

// The data of the chart cases: what the renderer of mermaid gives.
function chartCase(i) {
  const n = 1 + int(4);
  const pool = pick(idPools);
  let areas = mermaidAreas(n, pool);
  const longLabels = ['A very long label that has many words in it', 'Two words', 'x', 'Hyphen-ated words-here and more', 'a  b   c', '<b>bold</b> & "quoted"', 'ünï cödé ☃ 😀 text'];
  areas = areas.map((a) => (chance(0.4) ? [a[0], a[1], pick(longLabels), a[3]] : a));
  if (i % 9 === 0) {
    areas = edgeAreas(i / 9);
  }
  const attach = i % 5 === 4;
  const [width, height] = pick([
    [800, 450],
    [800, 426],
    [400, 300],
    [1600, 900],
  ]);
  return { kind: 'chart', areas, width: hex(width), height: hex(height), attach };
}

const counts = chromium
  ? { mermaid: 150, random: 150, many: 60, disjoint: 50, proto: 20, edge: edgeCount, intersectionArea: 300, scalar: 300, textCentre: 200, normalize: 150, initial: 80, chart: 90 }
  : { mermaid: 500, random: 500, many: 200, disjoint: 150, proto: 60, edge: edgeCount, intersectionArea: 900, scalar: 900, textCentre: 600, normalize: 400, initial: 250, chart: 0 };
const fullPerFamily = chromium ? 12 : 25;

const cases = [];
for (const family of ['mermaid', 'random', 'many', 'disjoint', 'proto', 'edge']) {
  for (let i = 0; i < counts[family]; i++) {
    cases.push({ name: `layout/${family}-${i}`, full: i < fullPerFamily || family === 'edge', ...layoutCase(family, i) });
  }
}
for (let i = 0; i < counts.intersectionArea; i++) {
  cases.push({ name: `intersectionArea-${i}`, kind: 'intersectionArea', circles: randomCircles(1 + int(5)) });
}
for (let i = 0; i < counts.scalar; i++) {
  cases.push({ name: `scalar-${i}`, ...scalarCase() });
}
for (let i = 0; i < counts.textCentre; i++) {
  cases.push({ name: `textCentre-${i}`, ...textCentreCase() });
}
for (let i = 0; i < counts.normalize; i++) {
  cases.push({ name: `normalize-${i}`, ...normalizeCase() });
}
for (let i = 0; i < counts.initial; i++) {
  cases.push({ name: `initial-${i}`, ...initialCase(i) });
}
for (let i = 0; i < counts.chart; i++) {
  cases.push({ name: `chart-${i}`, ...chartCase(i) });
}

// ---- run ----
const probeFile = path.join(here, 'probes/venn.js');
let results;
let meta;
if (chromium) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'venn-golden-'));
  const argsFile = path.join(tmp, 'args.json');
  const resultFile = path.join(tmp, 'result.json');
  fs.writeFileSync(argsFile, JSON.stringify({ cases }));
  execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'esm-eval', probeFile, '--args', argsFile, '--out', resultFile], {
    stdio: 'inherit',
    maxBuffer: 1 << 30,
  });
  results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
  fs.rmSync(tmp, { recursive: true, force: true });
  meta = { engine: 'chromium', mermaid: '12.2.0', venn: '2.0.0' };
} else {
  // The module with every top-level function exported.
  const pkgDir = path.join(here, 'node_modules/@upsetjs/venn.js');
  const version = JSON.parse(fs.readFileSync(path.join(pkgDir, 'package.json'), 'utf8')).version;
  let src = fs.readFileSync(path.join(pkgDir, 'build/venn.esm.js'), 'utf8');
  const exported = new Set(src.match(/^export \{([^}]*)\};?\s*$/m)[1].split(',').map((s) => s.trim()));
  const extra = [...src.matchAll(/^function ([A-Za-z_$][\w$]*)/gm)].map((m) => m[1]).filter((n) => !exported.has(n));
  src += `\nexport { ${extra.join(', ')} };\n`;
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'venn-golden-'));
  const modFile = path.join(tmp, 'venn.all.mjs');
  fs.writeFileSync(modFile, src);
  globalThis.__vennLib = await import(pathToFileURL(modFile).href);
  fs.rmSync(tmp, { recursive: true, force: true });

  const fdlibm = (await import('./elk-fdlibm.mjs')).functions;
  Math.sin = fdlibm.sin;
  Math.cos = fdlibm.cos;
  Math.acos = fdlibm.acos;
  Math.atan2 = fdlibm.atan2;
  Math.pow = fdlibm.pow;
  let a = RANDOM_SEED >>> 0;
  globalThis.__mmdgRandomCalls = 0;
  globalThis.__mmdgResetRandom = () => {
    a = RANDOM_SEED >>> 0;
    globalThis.__mmdgRandomCalls = 0;
  };
  Math.random = function () {
    globalThis.__mmdgRandomCalls++;
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  const log = console.log;
  console.log = () => {}; // venn.js logs "not scaling solution"
  const body = fs.readFileSync(probeFile, 'utf8');
  const AsyncFunction = (async () => {}).constructor;
  results = await new AsyncFunction('args', body)({ cases });
  console.log = log;
  meta = { engine: 'node', node: process.version, v8: process.versions.v8, venn: version, math: 'fdlibm (elk-fdlibm.mjs)' };
}

// ---- digests ----
let errors = 0;
const out = cases.map((c, i) => {
  const r = results[i];
  if (r.error !== undefined || r.stepsError !== undefined || r.layoutError !== undefined) {
    errors++;
  }
  const rec = { ...c };
  delete rec.full;
  if (c.kind !== 'layout') {
    rec.out = r;
    return rec;
  }
  const details = [r.normalized ?? null, r.layout ?? null, r.chart ? [r.chart.circles, r.chart.paths, r.chart.centres] : null];
  const digest = createHash('sha256').update(JSON.stringify(details)).digest('hex').slice(0, 32);
  // A case with an exception keeps all results: the test then compares the
  // blocks that have no exception one by one.
  if (c.full || r.stepsError !== undefined || r.layoutError !== undefined || r.chartError !== undefined) {
    rec.out = { ...r, digest };
  } else {
    rec.out = { digest };
    for (const key of ['venn', 'vennRandom', 'scaled', 'centres', 'stepsError', 'layoutError', 'layoutRandom', 'chartError', 'error']) {
      if (r[key] !== undefined) {
        rec.out[key] = r[key];
      }
    }
  }
  return rec;
});

const file = path.join(repoRoot, 'internal/venn/testdata', chromium ? 'venn-chromium.json.gz' : 'venn.json.gz');
fs.mkdirSync(path.dirname(file), { recursive: true });
const data = JSON.stringify({ meta, cases: out }) + '\n';
fs.writeFileSync(file, zlib.gzipSync(data, { level: 9 }));
const kinds = {};
for (const c of cases) {
  kinds[c.kind] = (kinds[c.kind] ?? 0) + 1;
}
console.error(
  `${path.relative(repoRoot, file)}: ${cases.length} cases ${JSON.stringify(kinds)}, ${errors} with an error, ${(fs.statSync(file).size / 1024).toFixed(0)} kB (${(data.length / 1024).toFixed(0)} kB of JSON)`
);
