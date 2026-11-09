// Emits internal/elk/native/networksimplex/testdata/networksimplex.json.gz:
// results of the network simplex algorithm of elkjs 0.9.3
// (org.eclipse.elk.alg.common.networksimplex in elk-worker.js) for graphs from
// a seeded generator, for the tests of the Go port.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-networksimplex.mjs
//
// The script builds each NGraph with the builders of the worker and runs the
// compiled NetworkSimplex on it. A case has:
//
//   n       the number of nodes
//   edges   [source, target, weight, delta] for each edge, in creation order
//   connect true when NGraph.makeConnected runs first
//   balance, limit (null for no limit), prev (null or the node counts of an
//           earlier layering)
//   layers  the layer of each node (the node that makeConnected adds is the
//           last one)
//   order   the node list of the graph after the run
//   out, in the outgoing and the incoming edges of each node after the run.
//           An edge is its index, or -1 for an edge of makeConnected.
//   err     the message, when the worker throws
//   again   null, or a second run on the same graph, as the network simplex
//           node placer does: weights has the new weight of each edge, and
//           balance and limit are the settings of the second run. layers,
//           order, out and in then have the state after the second run, and
//           first has the layers after the first run.
import { gzipSync } from 'node:zlib';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadElk } from './elk-phases-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const outFile = resolve(here, '../../internal/elk/native/networksimplex/testdata/networksimplex.json.gz');

const elk = loadElk();
const S = elk.hook.scope;
const NGraph = S('NGraph');
const NNodeBuilder = S('NNode$NNodeBuilder');
const NEdgeBuilder = S('NEdge$NEdgeBuilder');
const createNode = S('$create_2');
const createEdge = S('$create_1');
const setDelta = S('$delta');
const setWeight = S('$weight');
const setSource = S('$source');
const setTarget = S('$target');
const forGraph = S('forGraph');
const withBalancing = S('$withBalancing');
const withIterationLimit = S('$withIterationLimit');
const withPreviousLayering = S('$withPreviousLayering');
const execute = S('$execute_0');
const makeConnected = S('$makeConnected');
const monitor = { begin() {}, done_1() {} };

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

function run(c) {
  const graph = new NGraph();
  const nodes = [];
  const index = new Map();
  for (let i = 0; i < c.n; i++) {
    const node = createNode(new NNodeBuilder(), graph);
    index.set(node, i);
    nodes.push(node);
  }
  const edgeIndex = new Map();
  const result = {};
  try {
    c.edges.forEach(([s, t, w, d], i) => {
      const e = createEdge(setTarget(setSource(setWeight(setDelta(new NEdgeBuilder(), d), w), nodes[s]), nodes[t]));
      edgeIndex.set(e, i);
    });
    if (c.connect) {
      const root = makeConnected(graph);
      if (root) {
        index.set(root, nodes.length);
        nodes.push(root);
      }
    }
    let ns = forGraph(graph);
    if (c.limit !== null) ns = withIterationLimit(ns, c.limit);
    if (c.prev !== null) ns = withPreviousLayering(ns, c.prev.slice());
    ns = withBalancing(ns, c.balance);
    execute(ns, monitor);
    if (c.again) {
      result.first = nodes.map((n) => n.layer);
      for (const [e, i] of edgeIndex) e.weight = c.again.weights[i];
      ns = forGraph(graph);
      if (c.again.limit !== null) ns = withIterationLimit(ns, c.again.limit);
      ns = withBalancing(ns, c.again.balance);
      execute(ns, monitor);
    }
  } catch (e) {
    result.err = String(e && e.detailMessage !== undefined ? e.detailMessage : e);
    return result;
  }
  const edgeRef = (e) => (edgeIndex.has(e) ? edgeIndex.get(e) : -1);
  result.layers = nodes.map((n) => n.layer);
  result.order = graph.nodes.array.map((n) => index.get(n));
  result.out = nodes.map((n) => n.outgoingEdges.list.array.map(edgeRef));
  result.in = nodes.map((n) => n.incomingEdges.list.array.map(edgeRef));
  return result;
}

// ---------------------------------------------------------------------------
// The graphs.

function makeCase(seed) {
  const r = prng(seed);
  const int = (lo, hi) => lo + Math.floor(r() * (hi - lo + 1));
  const pick = (list) => list[int(0, list.length - 1)];
  const kind = pick(['dag', 'dag', 'dag', 'tree', 'layered', 'chain', 'pendant', 'pendant', 'forest', 'forest', 'dense']);
  // Sizes around 40 matter: from 40 nodes the algorithm removes subtrees.
  const n = pick([1, 2, 3, 4, 5, 6, 8, 10, 13, 17, 24, 31, 38, 39, 40, 41, 45, 52, 64, 80, 110]);
  // perm maps a topological rank to the node index, so that an edge can point
  // to a node with a lower index.
  const perm = [...Array(n).keys()];
  if (r() < 0.6) {
    for (let i = n - 1; i > 0; i--) {
      const j = int(0, i);
      [perm[i], perm[j]] = [perm[j], perm[i]];
    }
  }
  const weightMode = pick(['one', 'one', 'int', 'int', 'frac', 'zero']);
  const deltaMode = pick(['one', 'one', 'one', 'int', 'zero']);
  const weight = () => {
    switch (weightMode) {
      case 'one':
        return 1;
      case 'int':
        return int(0, 6);
      case 'frac':
        return pick([0, 0.1, 0.25, 0.5, 1, 1.5, 2, 3.7, 10]);
      default:
        return r() < 0.5 ? 0 : int(1, 3);
    }
  };
  const delta = () => {
    switch (deltaMode) {
      case 'one':
        return 1;
      case 'int':
        return int(0, 5);
      default:
        return r() < 0.3 ? 0 : int(1, 40);
    }
  };
  const edges = [];
  const add = (a, b) => {
    // a and b are ranks, a < b.
    edges.push([perm[a], perm[b], weight(), delta()]);
  };
  // addAny is add with a random direction, for an edge that is the only
  // connection of the node b to the nodes before it: no cycle can come from
  // it.
  const bothDirections = r() < 0.5;
  const addAny = (a, b) => {
    if (bothDirections && r() < 0.5) edges.push([perm[b], perm[a], weight(), delta()]);
    else add(a, b);
  };
  switch (kind) {
    case 'dag': {
      const p = pick([0.03, 0.08, 0.15, 0.3]);
      for (let a = 0; a < n; a++) for (let b = a + 1; b < n; b++) if (r() < p) add(a, b);
      break;
    }
    case 'dense': {
      const p = pick([0.5, 0.8]);
      const m = Math.min(n, 24);
      for (let a = 0; a < m; a++) for (let b = a + 1; b < m; b++) if (r() < p) add(a, b);
      break;
    }
    case 'tree': {
      // Each node but the first one has one edge to or from an earlier node.
      for (let b = 1; b < n; b++) {
        const a = int(0, b - 1);
        addAny(a, b);
      }
      break;
    }
    case 'chain': {
      for (let b = 1; b < n; b++) add(b - 1, b);
      if (n > 3 && r() < 0.5) add(0, n - 1);
      break;
    }
    case 'layered': {
      // Nodes in ranks, edges to the next ranks, and some long edges.
      const width = int(1, 6);
      for (let b = width; b < n; b++) {
        const k = int(1, 3);
        for (let i = 0; i < k; i++) {
          const a = Math.max(0, b - int(1, width * pick([1, 1, 2, 4])));
          add(a, b);
        }
      }
      break;
    }
    case 'pendant': {
      // A core graph with trees that hang on it.
      const core = Math.max(1, Math.floor(n * pick([0.2, 0.5, 0.8])));
      const p = pick([0.1, 0.3]);
      for (let a = 0; a < core; a++) for (let b = a + 1; b < core; b++) if (r() < p) add(a, b);
      for (let b = core; b < n; b++) {
        const a = int(0, b - 1);
        addAny(a, b);
      }
      break;
    }
    case 'forest': {
      // Several components.
      const parts = int(2, 5);
      for (let b = parts; b < n; b++) {
        const a = b - parts * int(1, Math.max(1, Math.floor(b / parts)));
        if (a >= 0) add(a, b);
        if (r() < 0.2 && b - parts >= 0) add(b - parts, b);
      }
      break;
    }
  }
  // Parallel edges.
  if (edges.length > 0 && r() < 0.25) {
    const k = int(1, 3);
    for (let i = 0; i < k; i++) {
      const e = pick(edges);
      edges.push([e[0], e[1], weight(), delta()]);
    }
  }
  // Without makeConnected the worker throws for a graph with two components.
  const connect = r() < 0.92;
  const balance = r() < 0.6;
  const limit = pick([null, null, null, 0, 1, 2, 3, 7, 28, 100]);
  let prev = null;
  if (r() < 0.3) {
    prev = [];
    const len = int(1, 12);
    for (let i = 0; i < len; i++) prev.push(int(0, 5));
  }
  let again = null;
  if (r() < 0.3) {
    again = { weights: edges.map((e) => (r() < 0.5 ? e[2] : pick([0, 1, 2, 4, 0.5]))), balance: r() < 0.5, limit: pick([null, null, 2, 50]) };
  }
  return { seed, kind, n, edges, connect, balance, limit, prev, again };
}

// A graph with a cycle could keep the worker in a loop, so each case is
// checked first.
function isAcyclic(c) {
  const indeg = new Array(c.n).fill(0);
  const out = Array.from({ length: c.n }, () => []);
  for (const [s, t] of c.edges) {
    out[s].push(t);
    indeg[t]++;
  }
  const queue = [];
  for (let i = 0; i < c.n; i++) if (indeg[i] === 0) queue.push(i);
  let seen = 0;
  while (queue.length) {
    const v = queue.pop();
    seen++;
    for (const t of out[v]) if (--indeg[t] === 0) queue.push(t);
  }
  return seen === c.n;
}

const cases = [];
const stats = { err: 0, subtrees: 0, limited: 0, again: 0 };
for (let seed = 1; cases.length < 700; seed++) {
  const c = makeCase(seed);
  if (!isAcyclic(c)) continue;
  Object.assign(c, run(c));
  if (c.err !== undefined) stats.err++;
  if (c.n >= 40) stats.subtrees++;
  if (c.limit !== null) stats.limited++;
  if (c.again && c.err === undefined) stats.again++;
  cases.push(c);
}
// Hand-made cases.
const hand = [
  // One node, no edge.
  { n: 1, edges: [], connect: false, balance: true, limit: null, prev: null },
  // No node.
  { n: 0, edges: [], connect: false, balance: true, limit: null, prev: null },
  // Two components without makeConnected: the worker throws.
  { n: 4, edges: [[0, 1, 1, 1], [2, 3, 1, 1]], connect: false, balance: false, limit: null, prev: null },
  // A self loop: the builder throws.
  { n: 2, edges: [[0, 1, 1, 1], [1, 1, 1, 1]], connect: false, balance: false, limit: null, prev: null },
  // An earlier layering that is longer than this one.
  { n: 3, edges: [[0, 1, 1, 1], [0, 2, 1, 1]], connect: false, balance: true, limit: null, prev: [9, 0, 3, 4, 5, 6] },
  // A node that the balancing can move.
  { n: 5, edges: [[0, 1, 1, 1], [1, 2, 1, 1], [2, 3, 1, 1], [0, 4, 1, 1], [4, 3, 1, 1]], connect: false, balance: true, limit: null, prev: null },
  { n: 5, edges: [[0, 1, 1, 1], [1, 2, 1, 1], [2, 3, 1, 1], [0, 4, 1, 1], [4, 3, 1, 1]], connect: false, balance: true, limit: null, prev: [0, 5, 0, 0] },
];
for (const c of hand) {
  c.again = null;
  c.seed = 0;
  c.kind = 'hand';
  Object.assign(c, run(c));
  if (c.err !== undefined) stats.err++;
  cases.push(c);
}

mkdirSync(dirname(outFile), { recursive: true });
const gz = gzipSync(Buffer.from(JSON.stringify(cases)), { level: 9 });
writeFileSync(outFile, gz);
console.log(`${cases.length} cases, ${stats.subtrees} with 40 nodes or more, ${stats.limited} with an iteration limit, ${stats.again} with a second run, ${stats.err} with an error, ${(gz.length / 1e3).toFixed(0)} kB gzip`);
