// Extra phase goldens for internal/elk/native/labels: the states of the
// LGraph before and after the edge label processors of ELK layered, for
// graphs that use the label options that the shared phase goldens do not
// reach (each center label placement strategy, each label side selection,
// label options on single labels, thick edges, edges that share a port).
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-labels.mjs
//
// The script makes seeded random ELK graphs and runs
// `node gen-elk-phases.mjs --dump` for each of them, so the states come from
// the same instrumented elkjs 0.9.3 as the shared goldens. It keeps the steps
// of the label processors and the states that they name, in the format of
// internal/elk/native/nativetest (one Case for each graph), and writes
// internal/elk/native/labels/testdata/label-steps.json.gz.
import { execFile } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const outFile = resolve(root, 'internal/elk/native/labels/testdata/label-steps.json.gz');

const PROCESSORS = new Set([
  'LabelDummyInserter',
  'LabelDummySwitcher',
  'LabelSideSelector',
  'LabelDummyRemover',
  'EndLabelPreprocessor',
  'EndLabelPostprocessor',
  'EndLabelSorter',
]);

const STRATEGIES = ['MEDIAN_LAYER', 'TAIL_LAYER', 'HEAD_LAYER', 'SPACE_EFFICIENT_LAYER', 'WIDEST_LAYER', 'CENTER_LAYER'];
const SIDES = ['ALWAYS_UP', 'ALWAYS_DOWN', 'DIRECTION_UP', 'DIRECTION_DOWN', 'SMART_UP', 'SMART_DOWN'];
const DIRECTIONS = ['RIGHT', 'DOWN', 'LEFT', 'UP'];
const PORT_SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];

// The number of graphs, and the largest state that a golden keeps.
const GRAPHS = 168;
const MAX_STATE_BYTES = 40000;

// mulberry32 is a small seeded generator.
function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// graph makes the ELK graph with the given index. The first graphs walk
// through each strategy and each side selection in each direction; the
// later ones choose everything at random.
function graph(index) {
  const rnd = mulberry32(0x1abe15 + index * 7919);
  const int = (n) => Math.floor(rnd() * n);
  const pick = (list) => list[int(list.length)];
  const chance = (p) => rnd() < p;
  // Sizes with fractions, as text measurement gives them.
  const size = (lo, hi) => Math.round((lo + rnd() * (hi - lo)) * 64) / 64;

  const options = { 'elk.algorithm': 'layered' };
  const strategy = STRATEGIES[index % STRATEGIES.length];
  const side = SIDES[Math.floor(index / STRATEGIES.length) % SIDES.length];
  const direction = DIRECTIONS[Math.floor(index / 3) % DIRECTIONS.length];
  if (index < 72 || chance(0.8)) options['elk.layered.edgeLabels.centerLabelPlacementStrategy'] = strategy;
  if (index < 72 || chance(0.8)) options['elk.layered.edgeLabels.sideSelection'] = side;
  if (index % 5 !== 4) options['elk.direction'] = direction;
  if (chance(0.3)) options['elk.spacing.edgeLabel'] = pick([0, 1, 3.5, 6]);
  if (chance(0.3)) options['elk.spacing.labelLabel'] = pick([0, 2, 4.25]);
  // No spline routing: the spline router leaves objects on the edges that
  // the dummy nodes share, and the state format cannot follow such an object
  // when the element that has its fields leaves the graph.
  if (chance(0.15)) options['elk.edgeRouting'] = 'POLYLINE';
  if (chance(0.25)) options['elk.layered.layering.strategy'] = pick(['LONGEST_PATH', 'NETWORK_SIMPLEX', 'COFFMAN_GRAHAM']);
  if (chance(0.3)) options['elk.layered.mergeEdges'] = true;
  if (chance(0.2)) options['elk.layered.cycleBreaking.strategy'] = pick(['GREEDY', 'DEPTH_FIRST']);
  if (chance(0.15)) options['elk.layered.spacing.edgeNodeBetweenLayers'] = pick([4, 25]);
  if (chance(0.15)) options['elk.layered.spacing.nodeNodeBetweenLayers'] = pick([10, 60]);

  const nodeCount = 3 + int(6);
  const children = [];
  const withPorts = chance(0.3);
  for (let i = 0; i < nodeCount; i++) {
    const node = { id: `n${i}`, width: size(20, 120), height: size(20, 90) };
    if (withPorts && chance(0.6)) {
      node.ports = [];
      const portCount = 1 + int(4);
      for (let p = 0; p < portCount; p++) {
        node.ports.push({ id: `n${i}_p${p}`, width: pick([0, 6, 8]), height: pick([0, 6, 8]), layoutOptions: { 'elk.port.side': pick(PORT_SIDES) } });
      }
      node.layoutOptions = { 'elk.portConstraints': pick(['FIXED_SIDE', 'FIXED_ORDER']) };
    }
    children.push(node);
  }

  const label = (text) => {
    const l = { text, width: size(6, 90), height: size(10, 26) };
    const lo = {};
    const kind = rnd();
    if (kind < 0.2) lo['elk.edgeLabels.placement'] = 'HEAD';
    else if (kind < 0.4) lo['elk.edgeLabels.placement'] = 'TAIL';
    else if (kind < 0.7) lo['elk.edgeLabels.placement'] = 'CENTER';
    if (chance(0.4)) lo['elk.edgeLabels.inline'] = chance(0.8);
    if (chance(0.15)) lo['elk.layered.edgeLabels.centerLabelPlacementStrategy'] = pick(STRATEGIES);
    if (Object.keys(lo).length > 0) l.layoutOptions = lo;
    return l;
  };
  const end = (i) => {
    const node = children[i];
    return node.ports && chance(0.7) ? pick(node.ports).id : node.id;
  };

  const edges = [];
  const addEdge = (from, to) => {
    const e = { id: `e${edges.length}`, sources: [end(from)], targets: [end(to)] };
    const labels = [];
    const r = rnd();
    const count = r < 0.2 ? 0 : r < 0.65 ? 1 : r < 0.9 ? 2 : 3;
    for (let k = 0; k < count; k++) labels.push(label(`${e.id}.${k}`));
    if (labels.length > 0) e.labels = labels;
    if (chance(0.2)) e.layoutOptions = { 'elk.edge.thickness': pick([0, 2, 3, 3.5, -1]) };
    edges.push(e);
  };
  // A chain, so that other edges span several layers.
  const chain = 2 + int(nodeCount - 1);
  for (let i = 0; i + 1 < chain; i++) addEdge(i, i + 1);
  const extra = 1 + int(nodeCount + 2);
  for (let k = 0; k < extra; k++) {
    const r = rnd();
    let from = int(nodeCount);
    let to = int(nodeCount);
    if (r < 0.45) {
      // A forward edge that skips nodes of the chain.
      from = int(Math.max(1, chain - 2));
      to = Math.min(nodeCount - 1, from + 2 + int(nodeCount));
    } else if (r < 0.6) {
      // A backward edge, which the cycle breaker reverses.
      from = Math.min(nodeCount - 1, 1 + int(nodeCount));
      to = int(from);
    } else if (r < 0.68) {
      to = from;
    }
    addEdge(from, to);
  }
  return { id: 'root', layoutOptions: options, children, edges };
}

// dump runs the phase oracle for one graph file.
function dump(file) {
  return new Promise((done, fail) => {
    execFile(process.execPath, ['gen-elk-phases.mjs', '--dump', file], { cwd: here, maxBuffer: 1 << 30 }, (err, stdout) => {
      if (err) fail(err);
      else done(JSON.parse(stdout));
    });
  });
}

// trim keeps the steps of the label processors and the states that they
// name. It returns null for a layout that failed or that has no such step.
function trim(name, input, r) {
  if (r.err !== undefined || r.out === undefined) return null;
  const runs = [];
  for (const run of r.runs) {
    const states = [];
    const indexOf = new Map();
    const keep = (i) => {
      if (!indexOf.has(i)) {
        indexOf.set(i, states.length);
        states.push(run.states[i]);
      }
      return indexOf.get(i);
    };
    const steps = [];
    for (const s of run.steps) {
      if (!PROCESSORS.has(s.proc) || s.before === undefined || s.after === undefined) continue;
      steps.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: keep(s.before), after: keep(s.after), draws: s.draws, drawsAfter: s.drawsAfter });
    }
    if (steps.length > 0) runs.push({ elk: run.elk, steps, states });
  }
  if (runs.length === 0) return null;
  for (const run of runs) for (const s of run.states) if (JSON.stringify(s).length > MAX_STATE_BYTES) return null;
  return { name, source: 'labels', in: input, runs };
}

async function main() {
  const dir = mkdtempSync(join(tmpdir(), 'elk-labels-'));
  const cases = new Array(GRAPHS).fill(null);
  let next = 0;
  const worker = async () => {
    while (next < GRAPHS) {
      const i = next++;
      const input = graph(i);
      const file = join(dir, `g${i}.json`);
      writeFileSync(file, JSON.stringify(input));
      cases[i] = trim(`labels-${i}`, input, await dump(file));
    }
  };
  await Promise.all(Array.from({ length: 6 }, worker));
  rmSync(dir, { recursive: true, force: true });

  const kept = cases.filter(Boolean);
  const count = new Map();
  for (const c of kept) for (const run of c.runs) for (const s of run.steps) count.set(s.proc, (count.get(s.proc) ?? 0) + 1);
  mkdirSync(dirname(outFile), { recursive: true });
  const gz = gzipSync(Buffer.from(JSON.stringify(kept)), { level: 9 });
  writeFileSync(outFile, gz);
  console.log(`${kept.length} cases of ${GRAPHS} graphs, ${(gz.length / 1e6).toFixed(2)} MB gzip`);
  for (const [proc, n] of [...count].sort()) console.log(String(n).padStart(6), proc);
}

await main();
