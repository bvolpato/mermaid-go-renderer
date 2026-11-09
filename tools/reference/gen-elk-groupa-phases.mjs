// Emits more phase goldens for the cycle breakers and the layerers of
// internal/elk/native (packages p1cycles and p2layers):
//
//   internal/elk/native/p1cycles/testdata/phases.json.gz
//   internal/elk/native/p2layers/testdata/phases.json.gz
//
// The phase goldens of gen-elk-phases.mjs come from graphs that mermaid
// produces. They do not reach each branch of these processors: a component
// with 40 nodes or more (the network simplex removes subtrees first), edge
// priorities, layer constraints, several components in one graph, the value
// ranges of the MinWidth layerer, and more than 128 nodes in the
// Coffman-Graham layerer. This script makes ELK graphs for those branches
// from a seeded generator, records the states with
// `node gen-elk-phases.mjs --dump`, and keeps the steps of the nine
// processors with the state before and after each one.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-groupa-phases.mjs
//
// A file is a gzip JSON list of cases in the format of the phase goldens
// (nativetest.Case), with the kept steps and their states only.
import { execFile } from 'node:child_process';
import { gzipSync } from 'node:zlib';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const here = dirname(fileURLToPath(import.meta.url));
const nativeDir = resolve(here, '../../internal/elk/native');
const run = promisify(execFile);

const P1 = ['DepthFirstCycleBreaker', 'GreedyCycleBreaker', 'GreedyModelOrderCycleBreaker', 'ModelOrderCycleBreaker'];
const P2 = ['NetworkSimplexLayerer', 'LongestPathLayerer', 'LongestPathSourceLayerer', 'CoffmanGrahamLayerer', 'MinWidthLayerer'];
const CYCLE = { DEPTH_FIRST: P1[0], GREEDY: P1[1], GREEDY_MODEL_ORDER: P1[2], MODEL_ORDER: P1[3] };
const LAYERING = { NETWORK_SIMPLEX: P2[0], LONGEST_PATH: P2[1], LONGEST_PATH_SOURCE: P2[2], COFFMAN_GRAHAM: P2[3], MIN_WIDTH: P2[4] };

// mulberry32
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

// makeGraph returns an ELK graph for a spec:
//   n         the number of nodes
//   shape     dag, cyclic, tree, pendant or parts
//   cycle     the cycle breaking strategy
//   layering  the layering strategy
//   options   more options of the root
//   priorities, constraints: edge priorities and layer constraints
function makeGraph(seed, spec) {
  const r = prng(seed);
  const int = (lo, hi) => lo + Math.floor(r() * (hi - lo + 1));
  const pick = (list) => list[int(0, list.length - 1)];
  const { n } = spec;
  const children = [];
  for (let i = 0; i < n; i++) {
    const node = { id: `n${i}`, width: int(20, 80), height: int(10, 70) };
    if (spec.constraints && r() < 0.15) {
      node.layoutOptions = { 'elk.layered.layering.layerConstraint': pick(['FIRST', 'LAST', 'FIRST_SEPARATE', 'LAST_SEPARATE']) };
    }
    children.push(node);
  }
  const pairs = [];
  const add = (a, b) => {
    if (a !== b) pairs.push([a, b]);
  };
  switch (spec.shape) {
    case 'dag': {
      const p = Math.min(0.6, pick([1.2, 2, 3]) / Math.max(1, n));
      for (let a = 0; a < n; a++) for (let b = a + 1; b < n; b++) if (r() < p) add(a, b);
      break;
    }
    case 'cyclic': {
      const m = Math.round(n * pick([1, 1.4, 2]));
      for (let i = 0; i < m; i++) add(int(0, n - 1), int(0, n - 1));
      break;
    }
    case 'tree': {
      for (let b = 1; b < n; b++) {
        const a = int(0, b - 1);
        if (r() < 0.3) add(b, a);
        else add(a, b);
      }
      break;
    }
    case 'pendant': {
      // A core with cycles, and trees that hang on it.
      const core = Math.max(2, Math.floor(n * pick([0.2, 0.4, 0.7])));
      const m = Math.round(core * pick([1.2, 1.8]));
      for (let i = 0; i < m; i++) add(int(0, core - 1), int(0, core - 1));
      for (let b = core; b < n; b++) {
        const a = int(0, b - 1);
        if (r() < 0.3) add(b, a);
        else add(a, b);
      }
      break;
    }
    case 'parts': {
      // Several components of different sizes.
      const parts = int(2, 5);
      const part = Array.from({ length: n }, () => (r() < 0.5 ? 0 : int(0, parts - 1)));
      const last = new Array(parts).fill(-1);
      for (let b = 0; b < n; b++) {
        const k = part[b];
        if (last[k] >= 0) {
          add(last[k], b);
          if (r() < 0.3) {
            let a = int(0, b - 1);
            while (a >= 0 && part[a] !== k) a--;
            if (a >= 0) add(r() < 0.25 ? b : a, r() < 0.25 ? a : b);
          }
        }
        if (r() < 0.8) last[k] = b;
      }
      break;
    }
  }
  const edges = pairs.map(([a, b], i) => {
    const edge = { id: `e${i}`, sources: [`n${a}`], targets: [`n${b}`] };
    if (spec.priorities && r() < 0.4) {
      edge.layoutOptions = {};
      if (r() < 0.7) edge.layoutOptions['elk.layered.priority.direction'] = String(int(-2, 6));
      if (r() < 0.7) edge.layoutOptions['elk.layered.priority.shortness'] = String(int(-1, 12));
    }
    return edge;
  });
  return {
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.layered.cycleBreaking.strategy': spec.cycle,
      'elk.layered.layering.strategy': spec.layering,
      ...spec.options,
    },
    children,
    edges,
  };
}

// The specs. Each one names what it is for.
function specs() {
  const out = [];
  const cycles = Object.keys(CYCLE);
  const layerings = Object.keys(LAYERING);
  let seed = 1000;
  const push = (spec) => out.push({ seed: seed++, ...spec });
  const shapes = ['cyclic', 'dag', 'pendant', 'tree', 'parts', 'cyclic'];
  // Each pair of strategies with small graphs: priorities, constraints,
  // several components in one graph.
  let k = 0;
  for (const cycle of cycles) {
    for (const layering of layerings) {
      for (let i = 0; i < 4; i++, k++) {
        const options = {};
        if (k % 3 === 0) options['elk.separateConnectedComponents'] = 'false';
        if (k % 5 === 1) options['elk.layered.thoroughness'] = '1';
        if (k % 4 === 2) options['elk.layered.considerModelOrder.strategy'] = 'NODES_AND_EDGES';
        if (k % 7 === 3) options['elk.direction'] = 'DOWN';
        push({
          n: [3, 5, 8, 12, 17, 24][k % 6],
          shape: shapes[(k + i) % shapes.length],
          cycle,
          layering,
          options,
          priorities: k % 2 === 0,
          constraints: k % 4 === 1,
        });
      }
    }
  }
  // The network simplex with 40 nodes or more in one component: it removes
  // the subtrees first, which changes the order of the nodes in the layers.
  const big = [40, 41, 47, 55, 64, 80, 96, 120];
  big.forEach((n, i) => {
    for (const shape of ['pendant', 'tree', 'dag']) {
      const options = {};
      if (i % 2 === 1) options['elk.separateConnectedComponents'] = 'false';
      if (i % 3 === 2) options['elk.layered.thoroughness'] = '1';
      push({ n, shape, cycle: cycles[(i + shape.length) % cycles.length], layering: 'NETWORK_SIMPLEX', options, priorities: i % 2 === 0, big: true });
    }
  });
  // Several components in one graph, the largest with 40 nodes or more.
  for (const n of [70, 100, 140]) {
    push({ n, shape: 'parts', cycle: 'GREEDY', layering: 'NETWORK_SIMPLEX', options: { 'elk.separateConnectedComponents': 'false' }, big: true });
    push({ n, shape: 'parts', cycle: 'DEPTH_FIRST', layering: 'NETWORK_SIMPLEX', options: {}, big: true });
  }
  // The Coffman-Graham layerer: a bound on the nodes of a layer, and more
  // than 128 nodes (the numbers of the topological order from 128 are other
  // Integer objects).
  for (const [n, bound, shape] of [
    [6, 1, 'dag'],
    [9, 2, 'cyclic'],
    [14, 3, 'pendant'],
    [20, 2, 'dag'],
    [30, 4, 'tree'],
    [45, 3, 'cyclic'],
    [140, 4, 'dag'],
    [150, 2147483647, 'dag'],
    [170, 6, 'pendant'],
    [200, 2147483647, 'tree'],
  ]) {
    push({ n, shape, cycle: 'GREEDY', layering: 'COFFMAN_GRAHAM', options: { 'elk.layered.layering.coffmanGraham.layerBound': String(bound) }, big: n >= 40 });
  }
  // The MinWidth layerer: negative values select the ranges of both
  // parameters.
  for (const [n, ubw, c, shape] of [
    [5, -1, -1, 'dag'],
    [9, -1, 2, 'cyclic'],
    [13, 3, -1, 'pendant'],
    [18, -1, -1, 'tree'],
    [26, 1, 1, 'dag'],
    [34, -1, -1, 'cyclic'],
    [40, 2, 3, 'parts'],
    [60, -1, -1, 'pendant'],
  ]) {
    push({
      n,
      shape,
      cycle: 'DEPTH_FIRST',
      layering: 'MIN_WIDTH',
      options: {
        'elk.layered.layering.minWidth.upperBoundOnWidth': String(ubw),
        'elk.layered.layering.minWidth.upperLayerEstimationScalingFactor': String(c),
      },
      big: n >= 40,
    });
  }
  // The longest path layerers and the cycle breakers with larger graphs.
  for (const [n, shape, cycle, layering] of [
    [45, 'cyclic', 'GREEDY', 'LONGEST_PATH'],
    [60, 'pendant', 'GREEDY_MODEL_ORDER', 'LONGEST_PATH_SOURCE'],
    [75, 'cyclic', 'MODEL_ORDER', 'LONGEST_PATH'],
    [90, 'dag', 'DEPTH_FIRST', 'LONGEST_PATH_SOURCE'],
    [50, 'cyclic', 'GREEDY', 'NETWORK_SIMPLEX'],
    [66, 'cyclic', 'GREEDY', 'NETWORK_SIMPLEX'],
  ]) {
    push({ n, shape, cycle, layering, options: {}, priorities: true, big: true });
  }
  return out;
}

// dump runs the phase recorder for one graph.
async function dump(dir, index, graph) {
  const file = join(dir, `g${index}.json`);
  writeFileSync(file, JSON.stringify(graph));
  try {
    const { stdout } = await run(process.execPath, ['gen-elk-phases.mjs', '--dump', file], { cwd: here, maxBuffer: 1 << 30, timeout: 120000 });
    return JSON.parse(stdout);
  } catch (e) {
    return { failed: String(e.message).split('\n')[0] };
  } finally {
    rmSync(file, { force: true });
  }
}

// keep returns the case with the steps of the given processors, or null.
function keep(name, graph, rec, procs) {
  const runs = [];
  for (const r of rec.runs) {
    const states = [];
    const renumber = new Map();
    const stateIndex = (i) => {
      if (!renumber.has(i)) {
        renumber.set(i, states.length);
        states.push(r.states[i]);
      }
      return renumber.get(i);
    };
    const steps = [];
    for (const s of r.steps) {
      if (!procs.includes(s.proc) || s.before === undefined || s.after === undefined) continue;
      steps.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: stateIndex(s.before), after: stateIndex(s.after), draws: s.draws, drawsAfter: s.drawsAfter });
    }
    if (steps.length > 0) runs.push({ elk: r.elk, components: r.components ?? undefined, imported: -1, final: -1, steps, states });
  }
  if (runs.length === 0) return null;
  return { name, source: 'groupa', in: graph, out: null, runs };
}

const all = specs();
const dir = mkdtempSync(join(tmpdir(), 'elk-groupa-'));
const p1 = [];
const p2 = [];
const count = new Map();
let failed = 0;
try {
  const results = new Array(all.length);
  let next = 0;
  const worker = async () => {
    while (next < all.length) {
      const i = next++;
      const graph = makeGraph(all[i].seed, all[i]);
      results[i] = { graph, rec: await dump(dir, i, graph) };
    }
  };
  await Promise.all(Array.from({ length: 4 }, worker));
  all.forEach((spec, i) => {
    const { graph, rec } = results[i];
    const name = `groupa-${spec.seed}-${spec.shape}-n${spec.n}-${spec.cycle}-${spec.layering}`;
    if (rec.failed !== undefined || rec.err !== undefined || !rec.runs || rec.runs.length === 0) {
      failed++;
      console.log(`${name}: no states (${rec.failed ?? rec.err ?? 'no run'})`);
      return;
    }
    // A large graph is kept for the processor that it was made for.
    const wantP1 = !spec.big || ['cyclic', 'pendant'].includes(spec.shape);
    const c1 = wantP1 ? keep(name, graph, rec, [CYCLE[spec.cycle]]) : null;
    const c2 = keep(name, graph, rec, [LAYERING[spec.layering]]);
    for (const [c, list] of [
      [c1, p1],
      [c2, p2],
    ]) {
      if (!c) continue;
      list.push(c);
      for (const r of c.runs) for (const s of r.steps) count.set(s.proc, (count.get(s.proc) ?? 0) + 1);
    }
  });
} finally {
  rmSync(dir, { recursive: true, force: true });
}

for (const [pkg, list] of [
  ['p1cycles', p1],
  ['p2layers', p2],
]) {
  const file = resolve(nativeDir, pkg, 'testdata/phases.json.gz');
  mkdirSync(dirname(file), { recursive: true });
  const gz = gzipSync(Buffer.from(JSON.stringify(list)), { level: 9 });
  writeFileSync(file, gz);
  console.log(`${pkg}: ${list.length} cases, ${(gz.length / 1e6).toFixed(2)} MB gzip`);
}
for (const proc of [...P1, ...P2]) console.log(String(count.get(proc) ?? 0).padStart(5), proc);
console.log(`${all.length} graphs, ${failed} without states`);
