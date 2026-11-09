// Phase goldens for the crossing minimization of internal/elk/native: the
// packages p3order, greedyswitch and preserveorder.
//
// The phase goldens of gen-elk-phases.mjs come from graphs that mermaid makes
// and from the generic graphs of gen-elk.mjs. They leave parts of the
// crossing minimization without a test: the greedy switch never switches a
// node in them, and they have no one-sided greedy switch, no greedy switch
// in a hierarchy, and no crossing counter with a model order influence. This
// script makes graphs and option sets for those parts.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-crossmin.mjs           write the goldens
//   node gen-elk-crossmin.mjs --stats   print what the goldens would have
//   node gen-elk-crossmin.mjs --dump N  print the input graph of the case N
//
// Each graph runs through `node gen-elk-phases.mjs --dump`, the unminified
// elkjs 0.9.3 with a hook around each layout processor, in a child process.
// A layout that does not end in time with its dump is left out: the dump of
// a large hierarchy is slow, and elkjs has option combinations that loop
// forever.
//
// The golden is internal/elk/native/p3order/testdata/crossmin.jsonl.gz: one
// JSON object for each graph, in the format of a case of the phase goldens
// (internal/elk/native/nativetest/state.go), with the steps of the crossing
// minimization processors only.
import { execFile } from 'node:child_process';
import { gunzipSync, gzipSync } from 'node:zlib';
import { mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { availableParallelism, tmpdir } from 'node:os';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const phasesDir = resolve(root, 'internal/elk/native/nativetest/testdata/phases');
const outFile = resolve(root, 'internal/elk/native/p3order/testdata/crossmin.jsonl.gz');

// The processors of the golden steps.
const WANTED = (proc) => proc.startsWith('LayerSweepCrossingMinimizer') || proc === 'SortByInputModelProcessor';
// The time that one layout with its dump gets.
const TIMEOUT_MS = 30000;
// The size limit of the golden, and of the states of one graph.
const MAX_BYTES = 1900000;
const MAX_CASE_STATE_BYTES = 400000;

// ---------------------------------------------------------------------------
// A seeded random number generator (mulberry32).

function prng(seed) {
  let a = seed >>> 0;
  const next = () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  const R = {
    next,
    int: (lo, hi) => lo + Math.floor(next() * (hi - lo + 1)),
    chance: (p) => next() < p,
    pick: (list) => list[Math.floor(next() * list.length)],
  };
  return R;
}

// ---------------------------------------------------------------------------
// Option sets. An option set goes to the root node and to each node with
// children, as mermaid does it.

const P = 'elk.layered.';
const GREEDY = P + 'crossingMinimization.greedySwitch.type';
const GREEDY_THRESHOLD = P + 'crossingMinimization.greedySwitch.activationThreshold';
const GREEDY_HIER = P + 'crossingMinimization.greedySwitchHierarchical.type';
const STRATEGY = P + 'crossingMinimization.strategy';
const FORCE = P + 'crossingMinimization.forceNodeModelOrder';
const SWEEPINESS = P + 'crossingMinimization.hierarchicalSweepiness';
const MODEL_ORDER = P + 'considerModelOrder.strategy';
const NODE_INFLUENCE = P + 'considerModelOrder.crossingCounterNodeInfluence';
const PORT_INFLUENCE = P + 'considerModelOrder.crossingCounterPortInfluence';
const PORT_MODEL_ORDER = P + 'considerModelOrder.portModelOrder';
const LONG_EDGE = P + 'considerModelOrder.longEdgeStrategy';
const THOROUGHNESS = P + 'thoroughness';
const SWITCH_SIDES = P + 'allowNonFlowPortsToSwitchSides';
const SEMI = P + 'crossingMinimization.semiInteractive';
const SEED = 'elk.randomSeed';
const HIERARCHY = 'elk.hierarchyHandling';

// The option sets for a graph without hierarchy handling.
const PLAIN_SETS = [
  {},
  { [GREEDY]: 'ONE_SIDED' },
  { [GREEDY]: 'ONE_SIDED', [STRATEGY]: 'NONE' },
  { [GREEDY]: 'TWO_SIDED', [STRATEGY]: 'NONE' },
  { [GREEDY]: 'TWO_SIDED', [THOROUGHNESS]: 1 },
  { [GREEDY]: 'ONE_SIDED', [THOROUGHNESS]: 1, [SEED]: 7 },
  { [GREEDY]: 'OFF', [SEED]: 3 },
  { [MODEL_ORDER]: 'NODES_AND_EDGES' },
  { [MODEL_ORDER]: 'PREFER_EDGES', [GREEDY]: 'ONE_SIDED' },
  { [MODEL_ORDER]: 'PREFER_NODES', [PORT_MODEL_ORDER]: true },
  { [MODEL_ORDER]: 'NODES_AND_EDGES', [NODE_INFLUENCE]: 0.001, [PORT_INFLUENCE]: 0.001 },
  { [MODEL_ORDER]: 'PREFER_EDGES', [NODE_INFLUENCE]: 0.5, [PORT_INFLUENCE]: 0.25, [LONG_EDGE]: 'DUMMY_NODE_UNDER' },
  { [MODEL_ORDER]: 'PREFER_NODES', [NODE_INFLUENCE]: 2, [PORT_MODEL_ORDER]: true, [LONG_EDGE]: 'EQUAL' },
  { [MODEL_ORDER]: 'NONE', [NODE_INFLUENCE]: 0.1 },
  { [MODEL_ORDER]: 'NODES_AND_EDGES', [FORCE]: true },
  { [MODEL_ORDER]: 'NODES_AND_EDGES', [FORCE]: true, [GREEDY]: 'ONE_SIDED' },
  { [MODEL_ORDER]: 'PREFER_EDGES', [FORCE]: true, [NODE_INFLUENCE]: 0.01, [THOROUGHNESS]: 3 },
  { [FORCE]: true },
  { [SWITCH_SIDES]: true },
  { [SWITCH_SIDES]: true, [GREEDY]: 'ONE_SIDED', [SEED]: 11 },
  { [THOROUGHNESS]: 30, [SEED]: 5 },
];

// The option sets for a graph whose nodes have positions: the semi-interactive
// crossing minimization makes in-layer constraints between the nodes.
const POSITION_SETS = [
  { [SEMI]: true },
  { [SEMI]: true, [THOROUGHNESS]: 2, [SEED]: 4 },
  { [SEMI]: true, [MODEL_ORDER]: 'NODES_AND_EDGES' },
  { [SEMI]: true, [MODEL_ORDER]: 'NODES_AND_EDGES', [FORCE]: true },
  { [SEMI]: true, [SWITCH_SIDES]: true },
  { [SEMI]: true, [MODEL_ORDER]: 'PREFER_EDGES', [NODE_INFLUENCE]: 0.1 },
];

// The option sets for a graph with hierarchy handling.
const HIER_SETS = [
  {},
  { [GREEDY_HIER]: 'TWO_SIDED' },
  { [GREEDY_HIER]: 'ONE_SIDED' },
  { [GREEDY_HIER]: 'TWO_SIDED', [STRATEGY]: 'NONE' },
  { [GREEDY_HIER]: 'ONE_SIDED', [STRATEGY]: 'NONE' },
  { [SWEEPINESS]: -2 },
  { [SWEEPINESS]: 2 },
  { [SWEEPINESS]: 2, [GREEDY_HIER]: 'TWO_SIDED' },
  { [SWEEPINESS]: 2, [GREEDY_HIER]: 'ONE_SIDED', [SEED]: 9 },
  { [SWEEPINESS]: 2, [GREEDY_HIER]: 'TWO_SIDED', [STRATEGY]: 'NONE' },
  { [SWEEPINESS]: -2, [GREEDY_HIER]: 'TWO_SIDED' },
  { [SWEEPINESS]: 0.5, [THOROUGHNESS]: 2 },
  { [MODEL_ORDER]: 'NODES_AND_EDGES', [FORCE]: true },
  { [MODEL_ORDER]: 'NODES_AND_EDGES', [FORCE]: true, [SWEEPINESS]: 2 },
  { [MODEL_ORDER]: 'NODES_AND_EDGES', [NODE_INFLUENCE]: 0.001, [PORT_INFLUENCE]: 0.001 },
  { [MODEL_ORDER]: 'PREFER_EDGES', [NODE_INFLUENCE]: 0.5, [SWEEPINESS]: 2 },
  { [MODEL_ORDER]: 'PREFER_NODES', [PORT_MODEL_ORDER]: true, [GREEDY_HIER]: 'TWO_SIDED' },
  { [SWITCH_SIDES]: true, [SWEEPINESS]: 2 },
];

// ---------------------------------------------------------------------------
// Graphs.

const SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];
const CONSTRAINTS = ['UNDEFINED', 'FREE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS'];

// makeNodes returns leaf nodes, some with ports. portIds collects the ids of
// the ports of each node.
function makeNodes(R, prefix, n, o) {
  const nodes = [];
  for (let i = 0; i < n; i++) {
    const node = { id: `${prefix}n${i}`, width: 20 + R.int(0, 6) * 10, height: 20 + R.int(0, 4) * 10 };
    if (R.chance(o.positions ?? 0)) node.layoutOptions = { 'elk.position': `(${R.int(0, 30) * 10}, ${R.int(0, 30) * 10})` };
    if (R.chance(o.ports)) {
      const count = R.int(1, 5);
      const constraint = R.pick(o.constraints ?? CONSTRAINTS);
      node.layoutOptions = { ...(node.layoutOptions ?? {}), 'elk.portConstraints': constraint };
      node.ports = [];
      for (let p = 0; p < count; p++) {
        const side = R.chance(o.northSouth) ? R.pick(['NORTH', 'SOUTH']) : R.pick(['EAST', 'WEST', 'EAST', 'WEST', ...SIDES]);
        const port = { id: `${node.id}p${p}`, width: 6, height: 6 };
        if (constraint !== 'FREE' && constraint !== 'UNDEFINED') port.layoutOptions = { 'elk.port.side': side };
        if (constraint === 'FIXED_POS') {
          // A position on the border of the side.
          const along = R.int(0, 4) * 4;
          if (side === 'NORTH') Object.assign(port, { x: along, y: -6 });
          else if (side === 'SOUTH') Object.assign(port, { x: along, y: node.height });
          else if (side === 'WEST') Object.assign(port, { x: -6, y: along });
          else Object.assign(port, { x: node.width, y: along });
        }
        if (constraint === 'FIXED_ORDER' && R.chance(0.5)) (port.layoutOptions ??= {})['elk.port.index'] = R.int(0, 6);
        if (R.chance(o.anchors ?? 0)) (port.layoutOptions ??= {})['elk.port.anchor'] = `(${R.int(0, 6)}, ${R.int(0, 6)})`;
        node.ports.push(port);
      }
    }
    nodes.push(node);
  }
  return nodes;
}

// endOf returns the id of a node or of one of its ports.
function endOf(R, node, usePorts) {
  if (node.ports?.length && R.chance(usePorts)) return R.pick(node.ports).id;
  return node.id;
}

// makeEdges returns edges between the given nodes. Most edges go forward in
// the list of the nodes, so that the graph has several layers.
function makeEdges(R, prefix, nodes, m, o) {
  const edges = [];
  for (let e = 0; e < m; e++) {
    let a = R.int(0, nodes.length - 1);
    let b = R.int(0, nodes.length - 1);
    if (a === b && !R.chance(o.selfLoops)) b = (b + 1) % nodes.length;
    if (a > b && !R.chance(o.backward)) [a, b] = [b, a];
    const edge = { id: `${prefix}e${e}`, sources: [endOf(R, nodes[a], o.usePorts)], targets: [endOf(R, nodes[b], o.usePorts)] };
    if (R.chance(o.labels)) edge.labels = [{ text: 'l', width: 14, height: 10 }];
    edges.push(edge);
  }
  return edges;
}

const FAMILY = {
  // A flat graph without ports: each edge end gets its own port.
  flat(R) {
    const n = R.int(4, 12);
    const o = { ports: 0, northSouth: 0, usePorts: 0, selfLoops: 0.02, backward: 0.1, labels: 0.05 };
    const children = makeNodes(R, '', n, o);
    return { id: 'root', layoutOptions: { 'elk.algorithm': 'layered' }, children, edges: makeEdges(R, '', children, R.int(n, 2 * n + 2), o) };
  },
  // A dense graph with few layers: many crossings.
  dense(R) {
    const n = R.int(5, 9);
    const o = { ports: 0, northSouth: 0, usePorts: 0, selfLoops: 0, backward: 0.3, labels: 0 };
    const children = makeNodes(R, '', n, o);
    const g = { id: 'root', layoutOptions: { 'elk.algorithm': 'layered', 'elk.direction': R.pick(['RIGHT', 'DOWN']) }, children, edges: makeEdges(R, '', children, R.int(2 * n, 3 * n), o) };
    if (R.chance(0.5)) g.layoutOptions[P + 'mergeEdges'] = true;
    return g;
  },
  // Few layers with many nodes: the sort of a layer is a merge sort for
  // seven nodes or more.
  wide(R) {
    const layers = R.int(2, 3);
    const o = { ports: R.pick([0, 0, 0.6]), northSouth: 0, usePorts: 0.8, selfLoops: 0, backward: 0, labels: 0, constraints: ['FREE', 'FIXED_SIDE', 'UNDEFINED'] };
    const children = [];
    const rows = [];
    for (let l = 0; l < layers; l++) {
      const row = makeNodes(R, `l${l}`, R.int(7, 12), o);
      rows.push(row);
      children.push(...row);
    }
    const edges = [];
    for (let l = 0; l + 1 < layers; l++) {
      const m = R.int(10, 22);
      for (let e = 0; e < m; e++) edges.push({ id: `l${l}e${e}`, sources: [endOf(R, R.pick(rows[l]), o.usePorts)], targets: [endOf(R, R.pick(rows[l + 1]), o.usePorts)] });
    }
    return { id: 'root', layoutOptions: { 'elk.algorithm': 'layered' }, children, edges };
  },
  // Nodes with positions, for the semi-interactive crossing minimization.
  positions(R) {
    const n = R.int(5, 11);
    const o = { ports: R.pick([0, 0.7]), northSouth: 0.3, usePorts: 0.8, selfLoops: 0, backward: 0.1, labels: 0, positions: 0.9, constraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FREE'] };
    const children = makeNodes(R, '', n, o);
    return { id: 'root', sets: POSITION_SETS, layoutOptions: { 'elk.algorithm': 'layered' }, children, edges: makeEdges(R, '', children, R.int(n, 2 * n + 2), o) };
  },
  // Nodes with ports on each side, and edges that share ports.
  ports(R) {
    const n = R.int(4, 10);
    const o = { ports: 0.8, northSouth: 0.25, usePorts: 0.85, selfLoops: 0.05, backward: 0.15, labels: 0.05 };
    const children = makeNodes(R, '', n, o);
    return { id: 'root', layoutOptions: { 'elk.algorithm': 'layered' }, children, edges: makeEdges(R, '', children, R.int(n, 2 * n + 3), o) };
  },
  // Ports whose order is not fixed: the port distributors sort them.
  freePorts(R) {
    const n = R.int(4, 9);
    const o = { ports: 1, northSouth: 0.1, usePorts: 1, selfLoops: 0, backward: 0.1, labels: 0, constraints: ['FREE', 'FIXED_SIDE', 'FIXED_SIDE', 'UNDEFINED'] };
    const children = makeNodes(R, '', n, o);
    return { id: 'root', layoutOptions: { 'elk.algorithm': 'layered', 'elk.direction': R.pick(['RIGHT', 'RIGHT', 'DOWN', 'LEFT']) }, children, edges: makeEdges(R, '', children, R.int(2 * n, 3 * n), o) };
  },
  // North and south ports with a fixed order.
  northSouth(R) {
    const n = R.int(4, 8);
    const o = { ports: 0.9, northSouth: 0.6, usePorts: 0.9, selfLoops: 0, backward: 0.1, labels: 0, anchors: 0.3, constraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS'] };
    const children = makeNodes(R, '', n, o);
    return { id: 'root', layoutOptions: { 'elk.algorithm': 'layered' }, children, edges: makeEdges(R, '', children, R.int(n, 2 * n), o) };
  },
  // A hierarchy: nodes with children, edges inside of them and across their
  // borders.
  hierarchy(R) {
    const o = { ports: R.pick([0, 0, 0.5]), northSouth: 0.1, usePorts: 0.7, selfLoops: 0, backward: 0.1, labels: 0.03 };
    const top = makeNodes(R, '', R.int(2, 5), o);
    const all = [...top];
    const g = { id: 'root', layoutOptions: { 'elk.algorithm': 'layered', [HIERARCHY]: 'INCLUDE_CHILDREN' }, children: top, edges: [] };
    const groups = R.int(1, 3);
    for (let k = 0; k < groups; k++) {
      const inner = makeNodes(R, `g${k}`, R.int(2, 5), o);
      const group = { id: `g${k}`, children: inner, edges: makeEdges(R, `g${k}`, inner, R.int(1, inner.length + 2), o), layoutOptions: { 'elk.padding': '[top=10,left=10,bottom=10,right=10]' } };
      if (R.chance(0.3)) {
        // A group inside of the group.
        const deep = makeNodes(R, `g${k}d`, R.int(2, 4), o);
        group.children.push({ id: `g${k}d`, children: deep, edges: makeEdges(R, `g${k}d`, deep, R.int(1, 4), o) });
        all.push(...deep);
      }
      if (R.chance(0.3)) {
        // Ports on the group.
        group.ports = [];
        const count = R.int(1, 3);
        group.layoutOptions['elk.portConstraints'] = R.pick(['FIXED_SIDE', 'FREE', 'FIXED_ORDER']);
        for (let p = 0; p < count; p++) group.ports.push({ id: `g${k}p${p}`, width: 6, height: 6, layoutOptions: { 'elk.port.side': R.pick(['EAST', 'WEST']) } });
      }
      g.children.push(group);
      all.push(...inner);
    }
    // Edges between any two leaf nodes: many cross a border.
    g.edges = makeEdges(R, 'x', all, R.int(all.length, 2 * all.length), o);
    return g;
  },
};

// applyOptions sets the options on the root and on each node with children.
function applyOptions(graph, options) {
  const g = structuredClone(graph);
  const walk = (node, depth) => {
    // A graph of the phase goldens can have a child that is not an object.
    if (node === null || typeof node !== 'object') return;
    if (depth === 0 || node.children?.length) {
      node.layoutOptions = { ...(node.layoutOptions ?? {}), ...options };
      // A group of a hierarchy must have the hierarchy handling of its root.
      if (depth > 0 && g.layoutOptions[HIERARCHY] === 'INCLUDE_CHILDREN') node.layoutOptions[HIERARCHY] = 'INCLUDE_CHILDREN';
    }
    if (Array.isArray(node.children)) for (const child of node.children) walk(child, depth + 1);
  };
  walk(g, 0);
  return g;
}

const isHierarchical = (graph) => graph.layoutOptions?.[HIERARCHY] === 'INCLUDE_CHILDREN';

function buildCases() {
  // Random graphs of each family, each with a few option sets.
  const R = prng(0x51ab1e);
  const counts = { flat: 14, dense: 20, wide: 16, ports: 24, freePorts: 24, northSouth: 20, positions: 18, hierarchy: 96 };
  const lists = [];
  for (const [family, count] of Object.entries(counts)) {
    const list = [];
    for (let i = 0; i < count; i++) {
      const graph = FAMILY[family](R);
      const sets = graph.sets ?? (isHierarchical(graph) ? HIER_SETS : PLAIN_SETS);
      delete graph.sets;
      // Each graph gets the next option sets of the table, and one random
      // one.
      const picks = new Set([i % sets.length, (i * 7 + 3) % sets.length, R.int(0, sets.length - 1)]);
      for (const k of picks) list.push({ name: `${family}-${i}-o${k}`, graph: applyOptions(graph, sets[k]) });
    }
    lists.push(list);
  }
  // The graphs of the phase goldens, with option sets that they do not have.
  const files = readdirSync(phasesDir).filter((f) => f.endsWith('.json.gz')).sort();
  const variants = [];
  let k = 0;
  for (const file of files) {
    const c = JSON.parse(gunzipSync(readFileSync(resolve(phasesDir, file))).toString());
    if (JSON.stringify(c.in).length > 4000) continue;
    const sets = isHierarchical(c.in) ? HIER_SETS : PLAIN_SETS;
    for (const s of [1 + (k % (sets.length - 1)), 1 + ((k * 5 + 2) % (sets.length - 1))]) {
      variants.push({ name: `${c.name}-o${s}`, graph: applyOptions(c.in, sets[s]) });
    }
    k++;
  }
  lists.push(variants);
  // Take the cases of the lists in turn, in proportion to the length of each
  // list, so that a full golden has cases of each kind.
  const cases = [];
  const longest = Math.max(...lists.map((l) => l.length));
  const taken = lists.map(() => 0);
  for (let step = 1; step <= longest; step++) {
    lists.forEach((list, li) => {
      const until = Math.round((step * list.length) / longest);
      while (taken[li] < until) cases.push(list[taken[li]++]);
    });
  }
  return cases;
}

// ---------------------------------------------------------------------------
// The layouts.

function dump(file) {
  return new Promise((done) => {
    execFile(process.execPath, ['gen-elk-phases.mjs', '--dump', file], { cwd: here, timeout: TIMEOUT_MS, maxBuffer: 1 << 30 }, (err, stdout) => {
      if (err) return done({ failed: err.killed ? 'timeout' : String(err.message).slice(0, 200) });
      try {
        done({ record: JSON.parse(stdout) });
      } catch (e) {
        done({ failed: 'no JSON output' });
      }
    });
  });
}

// golden returns the case of the golden file for a record of the dump: the
// wanted steps, and the states that they name.
function golden(name, graph, record) {
  const runs = [];
  for (const run of record.runs) {
    const states = [];
    const index = new Map();
    const state = (i) => {
      if (!index.has(i)) {
        index.set(i, states.length);
        states.push(run.states[i]);
      }
      return index.get(i);
    };
    const steps = [];
    for (const s of run.steps) {
      if (!WANTED(s.proc) || s.before === undefined || s.after === undefined) continue;
      steps.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: state(s.before), after: state(s.after), draws: s.draws, drawsAfter: s.drawsAfter });
    }
    if (steps.length > 0) runs.push({ elk: run.elk, steps, states });
  }
  return { name, source: 'crossmin', in: graph, runs };
}

async function main() {
  const stats = process.argv.includes('--stats');
  const cases = buildCases();
  if (process.argv.includes('--dump')) {
    // Print the input graph of one case.
    const name = process.argv[process.argv.indexOf('--dump') + 1];
    const c = cases.find((x) => x.name === name);
    if (!c) throw new Error(`no case with the name ${name}`);
    console.log(JSON.stringify(c.graph));
    return;
  }
  const tmp = mkdtempSync(resolve(tmpdir(), 'elk-crossmin-'));
  const results = new Array(cases.length);
  let next = 0;
  const worker = async () => {
    for (;;) {
      const i = next++;
      if (i >= cases.length) return;
      const file = resolve(tmp, `${i}.json`);
      writeFileSync(file, JSON.stringify(cases[i].graph));
      results[i] = await dump(file);
    }
  };
  await Promise.all(Array.from({ length: Math.max(1, Math.min(12, availableParallelism() - 2)) }, worker));
  rmSync(tmp, { recursive: true, force: true });

  const lines = [];
  const count = new Map();
  const add = (k, n = 1) => count.set(k, (count.get(k) ?? 0) + n);
  const seen = new Set();
  let bytes = 0;
  for (let i = 0; i < cases.length; i++) {
    const r = results[i];
    if (r.failed) {
      add(`left out: ${r.failed === 'timeout' ? 'the layout and its dump did not end in time' : 'the dump failed'}`);
      if (r.failed === 'timeout') console.log(`the layout and its dump did not end in time: ${cases[i].name}`);
      continue;
    }
    if (r.record.err !== undefined) {
      add('left out: elkjs reports an error');
      continue;
    }
    const c = golden(cases[i].name, cases[i].graph, r.record);
    if (c.runs.length === 0) {
      add('left out: no step of the processors');
      continue;
    }
    const text = JSON.stringify(c);
    const stateBytes = c.runs.reduce((a, run) => a + run.states.reduce((b, s) => b + JSON.stringify(s).length, 0), 0);
    if (stateBytes > MAX_CASE_STATE_BYTES) {
      add('left out: the states are too large');
      continue;
    }
    // The same steps on the same states one time only.
    const key = JSON.stringify(c.runs);
    if (seen.has(key)) {
      add('left out: equal to an earlier case');
      continue;
    }
    seen.add(key);
    const size = gzipSync(Buffer.from(text), { level: 9 }).length;
    if (bytes + size > MAX_BYTES) {
      add('left out: the golden is full');
      continue;
    }
    bytes += size;
    lines.push(text);
    add('cases');
    for (const run of c.runs) {
      for (const s of run.steps) {
        add(`steps of ${s.proc} (${s.kind})`);
        if (JSON.stringify(run.states[s.before]) !== JSON.stringify(run.states[s.after])) add(`steps of ${s.proc} that change the state`);
      }
    }
  }
  for (const [k, v] of [...count].sort((a, b) => (a[0] < b[0] ? -1 : 1))) console.log(String(v).padStart(6), k);
  const gz = gzipSync(Buffer.from(lines.join('\n') + '\n'), { level: 9 });
  console.log(`${lines.length} cases, ${(gz.length / 1e6).toFixed(2)} MB gzip`);
  if (stats) return;
  mkdirSync(dirname(outFile), { recursive: true });
  writeFileSync(outFile, gz);
}

await main();
