// Emits golden cases for internal/dagre from dagre-d3-es (the version that
// mermaid bundles).
//
// Usage (from tools/reference):
//   node gen-dagre.mjs                 write internal/dagre/testdata/layout.json.gz
//   node gen-dagre.mjs --debug <name>  print the layout graph after each
//                                      pipeline stage of one golden case
//   node gen-dagre.mjs --seed <n> --out <file>
//                                      write other random graphs to a file,
//                                      for a wider check. Run the Go tests on
//                                      it with DAGRE_GOLDEN=<file>.
//
// A case is a list of graph operations (so that the Go test builds the graph
// in the same order), the graph options, and the result of layout(): node
// x/y (and width/height for a node with children), edge points and label
// x/y, and the graph width/height. A case where layout() throws records the
// error message.
//
// Numbers are JSON numbers, except: -0 is "-0", NaN is "NaN", the infinities
// are "Infinity" and "-Infinity", and undefined is null. In the input, the
// string "undefined" is a property that exists with the value undefined.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { gzipSync, gunzipSync } from 'node:zlib';
import { layout } from 'dagre-d3-es/src/dagre/index.js';
import { Graph } from 'dagre-d3-es/src/graphlib/index.js';
import { uniqueId } from 'lodash-es';

const argOf = (name) => {
  const i = process.argv.indexOf(name);
  return i < 0 ? undefined : process.argv[i + 1];
};
const OUT = argOf('--out') ?? new URL('../../internal/dagre/testdata/layout.json.gz', import.meta.url);

// ---------------------------------------------------------------------------
// Seeded PRNG

let prngState = (0x2f6e2b1 + Math.imul(Number(argOf('--seed') ?? 0), 0x9e3779b1)) >>> 0;
const R = () => {
  prngState = (prngState + 0x6d2b79f5) >>> 0;
  let t = prngState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const ri = (a, b) => a + Math.floor(R() * (b - a + 1));
const pick = (arr) => arr[Math.floor(R() * arr.length)];
const chance = (p) => R() < p;
const shuffle = (arr) => {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(R() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
};
const ABSENT = Symbol('absent');
const put = (obj, key, value) => {
  if (value !== ABSENT) obj[key] = value;
};

// ---------------------------------------------------------------------------
// Encoding

const enc = (x) => {
  if (x === undefined) return null;
  if (typeof x !== 'number') return `?${typeof x}`;
  if (Number.isNaN(x)) return 'NaN';
  if (x === Infinity) return 'Infinity';
  if (x === -Infinity) return '-Infinity';
  if (Object.is(x, -0)) return '-0';
  return x;
};
const dec = (x) => (x === 'undefined' ? undefined : x === 'NaN' ? NaN : x);
const decObj = (o) => {
  if (o === null || o === undefined) return undefined;
  const r = {};
  for (const k of Object.keys(o)) r[k] = dec(o[k]);
  return r;
};

// ---------------------------------------------------------------------------
// Building and running a case

function build(c) {
  const g = new Graph(c.opts);
  if (c.graph !== null) g.setGraph(decObj(c.graph));
  if (c.defEdge) g.setDefaultEdgeLabel(() => ({}));
  for (const op of c.ops) {
    switch (op[0]) {
      case 'n':
        if (op.length > 2) g.setNode(op[1], decObj(op[2]));
        else g.setNode(op[1]);
        break;
      case 'p':
        if (op.length > 2) g.setParent(op[1], op[2]);
        else g.setParent(op[1]);
        break;
      case 'e': {
        const [, v, w, name, label] = op;
        if (label === null) {
          if (name === null) g.setEdge(v, w);
          else g.setEdge({ v, w, name });
        } else if (name === null) g.setEdge(v, w, decObj(label));
        else g.setEdge(v, w, decObj(label), name);
        break;
      }
      case 'rn':
        g.removeNode(op[1]);
        break;
      case 're':
        if (op[3] === null) g.removeEdge(op[1], op[2]);
        else g.removeEdge(op[1], op[2], op[3]);
        break;
      default:
        throw new Error('bad op ' + op[0]);
    }
  }
  return g;
}

function result(g) {
  const nodes = g.nodes().map((v) => {
    const n = g.node(v);
    if (!n) return [v];
    if (g.children(v).length) return [v, enc(n.x), enc(n.y), enc(n.width), enc(n.height)];
    return [v, enc(n.x), enc(n.y)];
  });
  const edges = g.edges().map((e) => {
    const l = g.edge(e);
    const pts = [];
    for (const p of l.points || []) pts.push(enc(p.x), enc(p.y));
    return [e.v, e.w, e.name === undefined ? null : e.name, l.points ? pts : null, enc(l.x), enc(l.y)];
  });
  return { nodes, edges, width: enc(g.graph().width), height: enc(g.graph().height) };
}

// The lodash uniqueId counter is a module global. counter() returns the
// number of ids made so far. Reading it makes one id.
const counter = () => Number(uniqueId(''));
const bumpTo = (n) => {
  let c = counter();
  while (c < n) c = counter();
  return c;
};

function run(c) {
  const g = build(c);
  const idStart = counter();
  try {
    layout(g);
    return { idStart, out: result(g) };
  } catch (e) {
    return { idStart, err: String(e && e.message) };
  }
}

// ---------------------------------------------------------------------------
// Random graphs

const SPECIAL_IDS = ['a.b', 'x[0]', 'A B', 'é', '漢字', '😀', 'length', '007', '1.5', '-1', '4294967295', '4294967294', '1e3', ' ', 'a-b', 'A', 'B', 'node:1', '_x', 'x_y', 'id-with-dash', '0'];
const DUMMY_LIKE = ['_d1', '_d2', '_d3', '_d10', '_root1', '_root2', '_bt1', '_bb2', '_ep1', '_bl3', '_br4', '_se1', 'rev1', '_d'];

function makeIds(n, scheme) {
  const ids = [];
  const used = new Set();
  const add = (id) => {
    if (used.has(id)) return false;
    used.add(id);
    ids.push(id);
    return true;
  };
  let i = 0;
  while (ids.length < n) {
    i++;
    switch (scheme) {
      case 'alpha':
        add('n' + String.fromCharCode(97 + ((i - 1) % 26)) + (i > 26 ? Math.floor((i - 1) / 26) : ''));
        break;
      case 'int':
        add(String(i));
        break;
      case 'intshuffled':
        add(String(ri(0, n * 3)));
        break;
      case 'mermaid':
        add(pick(['A', 'B', 'C', 'node', 'Start', 'End', 'id', 'flowchart-X-']) + i);
        break;
      case 'dummy':
        if (!(chance(0.5) && add(pick(DUMMY_LIKE)))) add('_d' + ri(1, 400));
        break;
      default: {
        // mixed
        const r = R();
        if (r < 0.35) add(String(ri(0, 99)));
        else if (r < 0.5) add(pick(SPECIAL_IDS));
        else if (r < 0.6) add(pick(DUMMY_LIKE));
        else add('v' + i);
      }
    }
  }
  return ids;
}

const SIZE_STYLES = ['int', 'frac', 'small', 'mermaid', 'mixed'];
function nodeSize(style) {
  switch (style) {
    case 'int':
      return { width: ri(10, 200), height: ri(10, 120) };
    case 'frac':
      return { width: ri(1000, 20000) / 64, height: ri(800, 9000) / 64 };
    case 'small':
      return { width: ri(0, 12), height: ri(0, 12) };
    case 'mermaid':
      return { width: 30 + ri(100, 9000) / 128 + 2 * pick([8, 15]), height: pick([24, 54, 39, 57.5, 78]) + pick([0, 0, 21.5]) };
    default:
      return nodeSize(pick(['int', 'frac', 'small', 'mermaid']));
  }
}

function graphAttrs(o = {}) {
  const g = {};
  put(g, 'rankdir', o.rankdir ?? pick([ABSENT, 'TB', 'TB', 'BT', 'LR', 'RL', 'tb', 'lr', 'Bt', 'rL']));
  put(g, 'nodesep', pick([ABSENT, ABSENT, 50, 30, 12.5, 100, 7]));
  put(g, 'edgesep', pick([ABSENT, ABSENT, ABSENT, 20, 10, 5.5, 40]));
  put(g, 'ranksep', pick([ABSENT, ABSENT, 50, 30, 75, 12.5, 100, 9]));
  put(g, 'marginx', pick([ABSENT, ABSENT, 0, 8, 20, 12.5]));
  put(g, 'marginy', pick([ABSENT, ABSENT, 0, 8, 20, 3.25]));
  put(g, 'ranker', o.ranker ?? pick([ABSENT, ABSENT, ABSENT, 'network-simplex', 'tight-tree', 'longest-path']));
  put(g, 'acyclicer', o.acyclicer ?? pick([ABSENT, ABSENT, ABSENT, 'greedy']));
  put(g, 'align', o.align ?? pick([ABSENT, ABSENT, ABSENT, ABSENT, 'UL', 'UR', 'DL', 'DR', 'ul', 'Dr']));
  return g;
}

function edgeLabel(o = {}) {
  const l = {};
  if (chance(o.labelProb ?? 0.4)) {
    l.width = pick([ri(5, 120), ri(200, 9000) / 64]);
    l.height = pick([ri(5, 40), 21.5, 24]);
  } else if (chance(0.1)) {
    l.width = pick([0, ri(1, 50)]);
    if (chance(0.5)) l.height = 0;
  }
  put(l, 'labelpos', pick([ABSENT, ABSENT, 'l', 'c', 'c', 'r', 'L', 'C', 'R']));
  put(l, 'minlen', pick(o.minlenPool ?? [ABSENT, ABSENT, ABSENT, ABSENT, 1, 1, 2, 3]));
  put(l, 'weight', pick(o.weightPool ?? [ABSENT, ABSENT, ABSENT, ABSENT, ABSENT, 1, 2, 3, 10]));
  put(l, 'labeloffset', pick([ABSENT, ABSENT, ABSENT, ABSENT, ABSENT, ABSENT, 10, 0, 5, 20, 7.5]));
  return l;
}

// randomCase returns a case for a random graph. o selects the features.
function randomCase(name, o = {}) {
  const n = o.n ?? ri(1, 60);
  const compound = o.compound ?? chance(0.45);
  const multigraph = o.multigraph ?? chance(0.6);
  const idScheme = o.ids ?? pick(['alpha', 'int', 'intshuffled', 'mermaid', 'mixed', 'mixed', 'mixed', 'dummy']);
  const sizeStyle = o.sizes ?? pick(SIZE_STYLES);
  const shape = o.shape ?? pick(['dag', 'dag', 'random', 'tree', 'chain', 'dense', 'components', 'star', 'sparse']);

  const ids = o.idList ? o.idList.slice() : makeIds(n, idScheme);
  const ops = [];

  // Clusters: some ids are clusters. parentOf maps a node to its cluster.
  const parentOf = new Map();
  let clusters = [];
  if (compound && n >= 2) {
    const c = Math.min(ri(1, 6), Math.floor(ids.length / 2));
    clusters = o.clusterList ? o.clusterList.slice() : shuffle(ids.slice()).slice(0, c);
    const clusterSet = new Set(clusters);
    clusters.forEach((cl, i) => {
      if (i > 0 && chance(0.5)) parentOf.set(cl, clusters[ri(0, i - 1)]);
    });
    for (const v of ids) {
      if (!clusterSet.has(v) && chance(o.inClusterProb ?? 0.6)) parentOf.set(v, pick(clusters));
    }
  }
  const hasChildren = new Set(parentOf.values());
  const leaves = ids.filter((v) => !hasChildren.has(v));

  // Node operations. The order varies: all nodes then parents (as mermaid
  // does it per node), or parents first, so that the parent is created by
  // setParent before its own setNode.
  const nodeOrder = chance(0.3) ? shuffle(ids.slice()) : ids;
  const parentsFirst = chance(0.25);
  if (parentsFirst) {
    for (const v of nodeOrder) if (parentOf.has(v)) ops.push(['p', v, parentOf.get(v)]);
  }
  for (const v of nodeOrder) {
    ops.push(['n', v, nodeSize(sizeStyle)]);
    if (!parentsFirst && parentOf.has(v)) ops.push(['p', v, parentOf.get(v)]);
  }

  // Edges between leaves.
  const edges = [];
  const m = leaves.length;
  const addEdge = (a, b) => {
    if (a === b && !o.selfLoops) return;
    edges.push([a, b]);
  };
  if (m > 1) {
    switch (shape) {
      case 'chain':
        for (let i = 1; i < m; i++) addEdge(leaves[i - 1], leaves[i]);
        break;
      case 'tree':
        for (let i = 1; i < m; i++) addEdge(leaves[ri(0, i - 1)], leaves[i]);
        break;
      case 'star':
        for (let i = 1; i < m; i++) chance(0.5) ? addEdge(leaves[0], leaves[i]) : addEdge(leaves[i], leaves[0]);
        break;
      case 'dag': {
        const count = ri(Math.floor(m / 2), Math.floor(m * 1.6));
        for (let k = 0; k < count; k++) {
          const i = ri(0, m - 2);
          addEdge(leaves[i], leaves[ri(i + 1, Math.min(m - 1, i + 1 + ri(0, 8)))]);
        }
        break;
      }
      case 'dense': {
        const count = ri(m, m * 3);
        for (let k = 0; k < count; k++) {
          const i = ri(0, m - 2);
          addEdge(leaves[i], leaves[ri(i + 1, m - 1)]);
        }
        break;
      }
      case 'components': {
        const groups = ri(2, 5);
        for (let i = 1; i < m; i++) {
          if (i % groups !== 0 && i - groups >= 0) addEdge(leaves[i - groups], leaves[i]);
          else if (chance(0.3) && i - groups >= 0) addEdge(leaves[i - groups], leaves[i]);
        }
        break;
      }
      case 'sparse': {
        const count = ri(0, Math.floor(m / 2));
        for (let k = 0; k < count; k++) addEdge(pick(leaves), pick(leaves));
        break;
      }
      default: {
        // random: cycles are likely
        const count = ri(Math.floor(m / 2), Math.floor(m * 1.5));
        for (let k = 0; k < count; k++) addEdge(pick(leaves), pick(leaves));
      }
    }
  }
  if (o.cycles && m > 2) {
    const count = ri(1, 4);
    for (let k = 0; k < count; k++) {
      const i = ri(1, m - 1);
      addEdge(leaves[i], leaves[ri(0, i - 1)]);
    }
  }
  if (o.selfLoops && m > 0) {
    const count = ri(1, 3);
    for (let k = 0; k < count; k++) {
      const v = pick(leaves);
      edges.push([v, v]);
    }
  }

  const seen = new Map();
  for (const [a, b] of edges) {
    const key = a + '\u0001' + b;
    const k = seen.get(key) ?? 0;
    seen.set(key, k + 1);
    let name = null;
    if (multigraph) {
      if (k > 0 || chance(0.3)) name = pick(['e', 'L_' + a + '_' + b + '_', 'id', '']) + k + (chance(0.1) ? '-x' : '');
      if (o.parallel && chance(0.5)) {
        ops.push(['e', a, b, 'p' + k + '_' + ops.length, edgeLabel(o)]);
      }
    } else if (k > 0) {
      if (chance(0.5)) continue;
    }
    ops.push(['e', a, b, name, edgeLabel(o)]);
  }

  // Remove and add again, as mermaid does when it adjusts cluster edges.
  if (o.churn) {
    const edgeOps = ops.filter((op) => op[0] === 'e');
    for (let k = 0; k < Math.min(4, edgeOps.length); k++) {
      const op = pick(edgeOps);
      ops.push(['re', op[1], op[2], op[3]]);
      ops.push(['e', op[1], op[2], op[3], op[4]]);
    }
    if (leaves.length > 3 && chance(0.5)) {
      const v = pick(leaves);
      ops.push(['rn', v]);
      ops.push(['n', v, nodeSize(sizeStyle)]);
      if (parentOf.has(v)) ops.push(['p', v, parentOf.get(v)]);
    }
  }

  return {
    name,
    opts: { multigraph, compound },
    graph: graphAttrs(o),
    defEdge: false,
    ops,
  };
}

// mermaidCase mimics prepareLayoutForDagre in mermaid: a compound multigraph
// with the graph label {rankdir, nodesep, ranksep, marginx: 8, marginy: 8},
// a default edge label function, edges with labelpos 'c' and a name, and
// self-loops as two 10x10 helper nodes and three edges.
function mermaidCase(name, o = {}) {
  const n = o.n ?? ri(2, 40);
  const ids = makeIds(n, pick(['mermaid', 'alpha', 'mixed', 'int']));
  const ops = [];
  const clusters = [];
  const parentOf = new Map();
  if (o.clusters) {
    const c = Math.min(ri(1, 5), Math.floor(n / 3));
    for (let i = 0; i < c; i++) {
      const id = 'subGraph' + i;
      if (i > 0 && chance(0.4)) parentOf.set(id, clusters[ri(0, i - 1)]);
      clusters.push(id);
    }
    for (const v of ids) if (clusters.length && chance(0.7)) parentOf.set(v, pick(clusters));
  }
  for (const v of ids) {
    ops.push(['n', v, { ...nodeSize('mermaid'), padding: 8, shape: 0 }]);
    if (parentOf.has(v)) ops.push(['p', v, parentOf.get(v)]);
  }
  for (const cl of clusters) {
    // A cluster node has no size before layout.
    ops.push(['n', cl, { padding: 8 }]);
    if (parentOf.has(cl)) ops.push(['p', cl, parentOf.get(cl)]);
  }
  const m = ids.length;
  const count = ri(m - 1, Math.floor(m * 1.5));
  let ecount = 0;
  for (let k = 0; k < count; k++) {
    const a = pick(ids);
    const b = chance(0.06) ? a : pick(ids);
    const label = { labelpos: 'c', minlen: pick([1, 1, 1, 1, 2, 3]), thickness: 0 };
    if (chance(0.35)) {
      label.width = ri(1000, 12000) / 128;
      label.height = pick([21.5, 24, 43]);
    }
    if (a === b) {
      const s1 = a + '---' + a + '---1';
      const s2 = a + '---' + a + '---2';
      ops.push(['n', s1, { width: 10, height: 10, padding: 0 }]);
      if (parentOf.has(a)) ops.push(['p', s1, parentOf.get(a)]);
      else ops.push(['p', s1]);
      ops.push(['n', s2, { width: 10, height: 10, padding: 0 }]);
      if (parentOf.has(a)) ops.push(['p', s2, parentOf.get(a)]);
      else ops.push(['p', s2]);
      ops.push(['e', a, s1, a + '-cyclic-special-0', { labelpos: 'c', minlen: label.minlen }]);
      ops.push(['e', s1, s2, a + '-cyclic-special-1', label]);
      ops.push(['e', s2, a, a + '-cyclic-special-2', { labelpos: 'c', minlen: label.minlen }]);
    } else {
      ops.push(['e', a, b, 'L_' + a + '_' + b + '_' + ecount++, label]);
    }
  }
  const graph = { rankdir: o.rankdir ?? pick(['TB', 'TB', 'LR', 'BT', 'RL']), nodesep: pick([50, 50, 30, 80]), ranksep: pick([50, 50, 75, 40]), marginx: 8, marginy: 8 };
  if (o.undefinedSep) {
    graph.nodesep = 'undefined';
  }
  return { name, opts: { multigraph: true, compound: true }, graph, defEdge: true, ops };
}

// ---------------------------------------------------------------------------
// Case list

const cases = [];
const add = (c) => cases.push(c);

// Fixed small cases.
const N = (w, h) => ({ width: w, height: h });
add({ name: 'fixed-empty', opts: {}, graph: {}, defEdge: false, ops: [] });
add({ name: 'fixed-empty-compound', opts: { compound: true, multigraph: true }, graph: { marginx: 8, marginy: 8 }, defEdge: false, ops: [] });
add({ name: 'fixed-one-node', opts: {}, graph: {}, defEdge: false, ops: [['n', 'a', N(50, 100)]] });
add({ name: 'fixed-one-node-no-size', opts: {}, graph: {}, defEdge: false, ops: [['n', 'a', {}]] });
add({ name: 'fixed-two-nodes', opts: {}, graph: { ranksep: 300 }, defEdge: false, ops: [['n', 'a', N(50, 100)], ['n', 'b', N(75, 200)], ['e', 'a', 'b', null, {}]] });
add({ name: 'fixed-default-edge-label', opts: {}, graph: {}, defEdge: true, ops: [['n', 'a', N(50, 100)], ['n', 'b', N(75, 200)], ['e', 'a', 'b', null, null]] });
add({ name: 'fixed-implicit-nodes', opts: {}, graph: {}, defEdge: true, ops: [['e', 'a', 'b', null, null], ['e', 'b', 'c', null, null]] });
for (const rankdir of ['TB', 'BT', 'LR', 'RL']) {
  for (const labelpos of ['l', 'c', 'r']) {
    add({
      name: `fixed-label-${rankdir}-${labelpos}`,
      opts: { multigraph: true },
      graph: { rankdir, ranksep: 300 },
      defEdge: false,
      ops: [['n', 'a', N(50, 100)], ['n', 'b', N(75, 200)], ['e', 'a', 'b', null, { width: 60, height: 70, labelpos }]],
    });
  }
  add({
    name: `fixed-selfloop-${rankdir}`,
    opts: { multigraph: true },
    graph: { rankdir, edgesep: 75 },
    defEdge: false,
    ops: [['n', 'a', N(100, 100)], ['e', 'a', 'a', null, { width: 50, height: 50 }], ['e', 'a', 'a', 'x', {}], ['n', 'b', N(20, 30)], ['e', 'a', 'b', null, {}]],
  });
  add({
    name: `fixed-cluster-${rankdir}`,
    opts: { compound: true },
    graph: { rankdir },
    defEdge: false,
    ops: [['n', 'a', N(50, 50)], ['n', 'sg1', {}], ['p', 'a', 'sg1'], ['n', 'b', N(40, 60)], ['e', 'b', 'a', null, {}], ['n', 'c', N(10, 10)], ['p', 'c', 'sg1'], ['e', 'a', 'c', null, { width: 20, height: 10 }]],
  });
}
add({
  name: 'fixed-nested-clusters',
  opts: { compound: true, multigraph: true },
  graph: { marginx: 8, marginy: 8 },
  defEdge: false,
  ops: [
    ['n', 'a', N(50, 50)], ['n', 'b', N(50, 50)], ['n', 'c', N(50, 50)], ['n', 'd', N(50, 50)], ['n', 'e', N(30, 20)],
    ['n', 'g1', {}], ['n', 'g2', {}], ['n', 'g3', {}],
    ['p', 'a', 'g2'], ['p', 'b', 'g2'], ['p', 'g2', 'g1'], ['p', 'c', 'g1'], ['p', 'd', 'g3'],
    ['e', 'a', 'b', null, {}], ['e', 'b', 'c', 'x', { width: 30, height: 20 }], ['e', 'c', 'd', null, {}], ['e', 'e', 'a', null, {}], ['e', 'd', 'e', null, { minlen: 2 }],
  ],
});
add({
  name: 'fixed-parallel-edges',
  opts: { multigraph: true },
  graph: {},
  defEdge: false,
  ops: [['n', 'a', N(50, 50)], ['n', 'b', N(50, 50)], ['e', 'a', 'b', 'one', { width: 40, height: 20 }], ['e', 'a', 'b', 'two', {}], ['e', 'a', 'b', null, { width: 10, height: 10, labelpos: 'l' }], ['e', 'b', 'a', 'back', {}]],
});
add({
  name: 'fixed-int-ids',
  opts: {},
  graph: {},
  defEdge: true,
  ops: [['n', 'b', N(30, 30)], ['n', '10', N(30, 30)], ['n', 'a', N(30, 30)], ['n', '2', N(30, 30)], ['n', '1', N(30, 30)], ['e', 'b', '10', null, null], ['e', 'b', '2', null, null], ['e', 'a', '1', null, null], ['e', '10', '1', null, null], ['e', 'b', 'a', null, null]],
});
add({
  name: 'fixed-node-without-label',
  opts: {},
  graph: {},
  defEdge: true,
  ops: [['n', 'a'], ['n', 'b', N(30, 30)], ['e', 'a', 'b', null, null]],
});

// Exotic inputs: results are defined by the JavaScript semantics of
// undefined, NaN and array holes.
const two = (label, graph = {}) => ({
  opts: { multigraph: true },
  graph,
  defEdge: false,
  ops: [['n', 'a', N(50, 100)], ['n', 'b', N(75, 200)], ['n', 'c', N(20, 20)], ['e', 'a', 'b', null, label], ['e', 'a', 'c', null, {}], ['e', 'c', 'b', null, {}]],
});
add({ name: 'exotic-minlen-0', ...two({ minlen: 0 }) });
add({ name: 'exotic-minlen-half', ...two({ minlen: 0.5, width: 10, height: 10 }) });
add({ name: 'exotic-minlen-1.5', ...two({ minlen: 1.5, width: 10, height: 10 }) });
add({ name: 'exotic-minlen-negative', ...two({ minlen: -1 }) });
add({ name: 'exotic-minlen-fraction', ...two({ minlen: 0.3 }) });
add({ name: 'exotic-minlen-undefined', ...two({ minlen: 'undefined' }) });
add({ name: 'exotic-weight-0', ...two({ weight: 0 }) });
add({ name: 'exotic-weight-half', ...two({ weight: 0.5 }) });
add({ name: 'exotic-weight-nan', ...two({ weight: 'NaN' }) });
add({ name: 'exotic-width-nan', ...two({ width: 'NaN', height: 5 }) });
add({ name: 'exotic-labelpos-upper', ...two({ width: 30, height: 30, labelpos: 'L' }) });
add({ name: 'exotic-labelpos-other', ...two({ width: 30, height: 30, labelpos: 'x' }) });
add({ name: 'exotic-nodesep-undefined', ...two({}, { nodesep: 'undefined', ranksep: 'undefined' }) });
add({ name: 'exotic-edgesep-nan', ...two({ width: 30, height: 30 }, { edgesep: 'NaN' }) });
add({ name: 'exotic-margin-undefined', ...two({}, { marginx: 'undefined', marginy: 'NaN' }) });
add({ name: 'exotic-ranksep-0', ...two({}, { ranksep: 0 }) });
add({ name: 'exotic-zero-everything', opts: {}, graph: { ranksep: 0, nodesep: 0 }, defEdge: true, ops: [['n', 'a', N(0, 0)], ['n', 'b', N(0, 0)], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-align-unknown', ...two({}, { align: 'XX' }) });
add({ name: 'exotic-ranker-unknown', ...two({}, { ranker: 'foo', acyclicer: 'foo', rankdir: 'XY' }) });
add({ name: 'exotic-node-size-nan', opts: {}, graph: {}, defEdge: true, ops: [['n', 'a', { width: 'NaN', height: 10 }], ['n', 'b', N(10, 10)], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-node-size-undefined', opts: {}, graph: {}, defEdge: true, ops: [['n', 'a', { width: 'undefined', height: 10 }], ['n', 'b', N(10, 10)], ['n', 'c', N(10, 10)], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-edge-to-cluster', opts: { compound: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(10, 10)], ['n', 'g', {}], ['n', 'b', N(10, 10)], ['p', 'b', 'g'], ['e', 'a', 'g', null, null]] });
add({ name: 'exotic-edge-without-label', opts: {}, graph: {}, defEdge: false, ops: [['n', 'a', N(10, 10)], ['n', 'b', N(10, 10)], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-no-graph-label', opts: {}, graph: null, defEdge: true, ops: [['n', 'a', N(10, 10)], ['n', 'b', N(10, 10)], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-cycle-minlen-0', opts: {}, graph: {}, defEdge: false, ops: [['n', 'a', N(10, 10)], ['n', 'b', N(10, 10)], ['e', 'a', 'b', null, {}], ['e', 'b', 'a', null, { minlen: 0 }]] });
add({ name: 'exotic-greedy-weight-half', ...two({ weight: 0.5 }, { acyclicer: 'greedy' }) });
add({ name: 'exotic-undirected', opts: { directed: false }, graph: {}, defEdge: true, ops: [['n', 'b', N(10, 10)], ['n', 'a', N(10, 10)], ['n', 'c', N(30, 10)], ['e', 'b', 'a', null, null], ['e', 'c', 'a', null, null]] });
add({ name: 'exotic-cluster-only', opts: { compound: true }, graph: {}, defEdge: true, ops: [['n', 'g', {}], ['n', 'a', N(10, 10)], ['p', 'a', 'g']] });
add({ name: 'exotic-cluster-no-leaf', opts: { compound: true }, graph: {}, defEdge: true, ops: [['n', 'g', {}], ['n', 'h', {}], ['p', 'h', 'g']] });
add({ name: 'exotic-cluster-in-non-compound-input', opts: {}, graph: {}, defEdge: true, ops: [['n', 'a', N(10, 10)], ['n', 'b', N(10, 10)], ['e', 'a', 'b', null, null]] });

add({ name: 'exotic-empty-id', opts: { multigraph: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(30, 20)], ['n', '', N(40, 20)], ['n', 'b', N(30, 20)], ['e', 'a', '', null, null], ['e', '', 'b', null, { width: 10, height: 10 }], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-empty-id-one-rank', opts: {}, graph: {}, defEdge: true, ops: [['n', 'a', N(30, 20)], ['n', '', N(40, 20)], ['n', 'b', N(30, 20)]] });
add({ name: 'exotic-empty-id-cluster', opts: { compound: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(30, 20)], ['n', '', {}], ['n', 'b', N(30, 20)], ['p', 'a', ''], ['e', 'a', 'b', null, null]] });
add({ name: 'exotic-edge-name-empty', opts: { multigraph: true }, graph: {}, defEdge: false, ops: [['n', 'a', N(30, 20)], ['n', 'b', N(30, 20)], ['e', 'a', 'b', '', {}]] });
add({ name: 'exotic-id-undefined', opts: { multigraph: true }, graph: {}, defEdge: true, ops: [['n', 'undefined', N(30, 20)], ['n', 'null', N(30, 20)], ['n', 'NaN', N(30, 20)], ['e', 'undefined', 'null', null, null], ['e', 'null', 'NaN', 'x', null], ['e', 'undefined', 'NaN', null, { width: 5, height: 5 }]] });
add({ name: 'exotic-huge-sizes', opts: {}, graph: { nodesep: 1e6, ranksep: 1e-3 }, defEdge: true, ops: [['n', 'a', N(1e9, 1e-9)], ['n', 'b', N(0.1, 0.2)], ['n', 'c', N(1 / 3, 2 / 3)], ['e', 'a', 'b', null, null], ['e', 'a', 'c', null, { width: 1e-7, height: 3e5 }]] });

// Random graphs. Each block forces one feature and leaves the rest random.
let seq = 0;
const nm = (p) => `${p}-${String(++seq).padStart(3, '0')}`;
for (const rankdir of ['TB', 'BT', 'LR', 'RL']) {
  for (let i = 0; i < 12; i++) add(randomCase(nm('rankdir-' + rankdir), { rankdir, cycles: chance(0.3), selfLoops: chance(0.2) }));
}
for (const ranker of ['network-simplex', 'tight-tree', 'longest-path']) {
  for (let i = 0; i < 14; i++) add(randomCase(nm('ranker-' + ranker), { ranker, cycles: chance(0.3) }));
  for (let i = 0; i < 8; i++) add(randomCase(nm('ranker-compound-' + ranker), { ranker, compound: true, n: ri(4, 40) }));
}
for (let i = 0; i < 20; i++) add(randomCase(nm('greedy'), { acyclicer: 'greedy', cycles: true, shape: pick(['random', 'dag', 'dense']) }));
for (const align of ['UL', 'UR', 'DL', 'DR', ABSENT]) {
  for (let i = 0; i < 8; i++) add(randomCase(nm('align-' + String(align === ABSENT ? 'none' : align)), { align }));
}
for (let i = 0; i < 30; i++) add(randomCase(nm('multi'), { multigraph: true, parallel: true, selfLoops: chance(0.3), labelProb: 0.6 }));
for (let i = 0; i < 50; i++) add(randomCase(nm('compound'), { compound: true, n: ri(3, 60), cycles: chance(0.3), selfLoops: chance(0.2) }));
for (let i = 0; i < 20; i++) add(randomCase(nm('compound-deep'), { compound: true, n: ri(10, 60), inClusterProb: 0.9, labelProb: 0.5 }));
for (let i = 0; i < 20; i++) add(randomCase(nm('selfloop'), { selfLoops: true, multigraph: true }));
for (let i = 0; i < 20; i++) add(randomCase(nm('labels'), { labelProb: 0.9 }));
for (let i = 0; i < 20; i++) add(randomCase(nm('components'), { shape: pick(['components', 'sparse']) }));
for (let i = 0; i < 20; i++) add(randomCase(nm('cycles'), { cycles: true, shape: 'random' }));
for (let i = 0; i < 25; i++) add(randomCase(nm('ids-mixed'), { ids: 'mixed' }));
for (let i = 0; i < 10; i++) add(randomCase(nm('ids-int'), { ids: pick(['int', 'intshuffled']) }));
for (let i = 0; i < 10; i++) add(randomCase(nm('ids-dummy'), { ids: 'dummy', compound: chance(0.5), cycles: true }));
for (let i = 0; i < 15; i++) add(randomCase(nm('churn'), { churn: true, compound: chance(0.5) }));
for (let n = 1; n <= 60; n++) add(randomCase(nm('size-' + n), { n }));
for (let i = 0; i < 8; i++) add(randomCase(nm('large'), { n: ri(61, 150), compound: chance(0.5) }));

// An edge weight of 0 gives a NaN barycenter. Then the comparator of the
// order stage is not consistent, and the result depends on the exact
// algorithm of Array.prototype.sort.
for (let i = 0; i < 30; i++) add(randomCase(nm('weight0'), { n: ri(8, 70), weightPool: [0, 0, 0, 1, 2, ABSENT], compound: chance(0.4), shape: pick(['dense', 'dag', 'random']), cycles: chance(0.3) }));
// A minlen of 0 puts both nodes of an edge on one rank.
for (let i = 0; i < 15; i++) add(randomCase(nm('minlen0'), { n: ri(3, 40), minlenPool: [0, 0, 1, 2, ABSENT, ABSENT], compound: chance(0.3) }));
// In a compound graph with the longest-path or tight-tree ranker, the rank
// of an edge label can be a fraction. The label then gets no position.
for (let i = 0; i < 20; i++) add(randomCase(nm('fraclabel'), { n: ri(5, 40), compound: true, inClusterProb: 0.8, labelProb: 0.9, ranker: pick(['longest-path', 'tight-tree']) }));

// Ids that are property names of Object.prototype. dagre reads some maps
// from id to value without a hasOwnProperty test.
const PROTO = ['constructor', 'toString', 'valueOf', 'hasOwnProperty', 'isPrototypeOf', 'propertyIsEnumerable', 'toLocaleString', '__defineGetter__', '__defineSetter__', '__lookupGetter__', '__lookupSetter__'];
add({ name: 'proto-single', opts: {}, graph: {}, defEdge: true, ops: [['n', 'constructor', N(40, 30)]] });
add({ name: 'proto-one-rank', opts: { multigraph: true }, graph: { marginx: 8 }, defEdge: true, ops: [['n', 'a', N(40, 30)], ['n', 'toString', N(40, 30)], ['n', 'b', N(20, 10)], ['n', 'constructor', N(50, 30)], ['n', 'c', N(40, 30)], ['n', 'valueOf', N(40, 30)]] });
add({ name: 'proto-one-rank-self', opts: { multigraph: true }, graph: { rankdir: 'LR' }, defEdge: true, ops: [['n', 'a', N(40, 30)], ['n', 'hasOwnProperty', N(40, 30)], ['e', 'hasOwnProperty', 'hasOwnProperty', null, { width: 10, height: 10 }], ['n', 'constructor', N(50, 30)], ['e', 'a', 'a', null, {}]] });
add({ name: 'proto-one-rank-minlen0', opts: { multigraph: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(40, 30)], ['n', 'constructor', N(40, 30)], ['n', 'b', N(40, 30)], ['e', 'a', 'constructor', null, { minlen: 0 }], ['e', 'constructor', 'b', null, { minlen: 0 }]] });
for (const id of PROTO) {
  add({ name: 'proto-chain-' + id, opts: { multigraph: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(40, 30)], ['n', id, N(60, 40)], ['n', 'b', N(40, 30)], ['e', 'a', id, 'e1', null], ['e', id, 'b', 'e2', null], ['e', 'a', 'b', 'e3', null]] });
}
for (const id of ['constructor', 'toString', 'valueOf']) {
  // A subgraph with such an id, and a nested subgraph in it.
  add({ name: 'proto-subgraph-' + id, opts: { multigraph: true, compound: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(40, 30)], ['n', 'b', N(40, 30)], ['n', 'c', N(40, 30)], ['n', 'd', N(40, 30)], ['n', id, {}], ['n', 'in1', {}], ['n', 'in2', {}], ['p', 'in1', id], ['p', 'in2', id], ['p', 'a', 'in1'], ['p', 'b', 'in1'], ['p', 'c', 'in2'], ['p', 'd', id], ['e', 'a', 'b', null, null], ['e', 'a', 'c', null, null], ['e', 'c', 'd', null, null], ['n', 'x', N(10, 10)], ['e', 'x', 'a', null, null]] });
  add({ name: 'proto-subgraph-top-' + id, opts: { multigraph: true, compound: true }, graph: {}, defEdge: true, ops: [['n', 'a', N(40, 30)], ['n', 'b', N(40, 30)], ['n', 'c', N(40, 30)], ['n', id, {}], ['n', 'other', {}], ['p', 'a', id], ['p', 'b', id], ['p', 'c', 'other'], ['e', 'a', 'b', null, null], ['e', 'a', 'c', null, null], ['n', 'x', N(10, 10)], ['e', 'x', 'c', null, null], ['e', 'c', 'x', 'back', null]] });
}
for (let i = 0; i < 6; i++) add(randomCase(nm('proto-random'), { n: 12, idList: shuffle(['a', 'b', 'c', 'd', 'e', 'f', 'g', 'constructor', 'toString', 'valueOf', 'sg1', 'sg2']), clusterList: pick([['constructor', 'sg1'], ['toString', 'valueOf'], ['sg1', 'constructor', 'sg2']]), compound: true, shape: 'sparse' }));

// The graph settings of the mermaid renderers that call dagre directly.
// Class diagram: a multigraph with the default graph label.
for (let i = 0; i < 8; i++) {
  const c = randomCase(nm('mermaid-class'), { multigraph: true, compound: false, ids: 'mermaid', sizes: 'mermaid', labelProb: 0 });
  c.graph = {};
  c.defEdge = true;
  for (const op of c.ops) if (op[0] === 'e') { op[3] = op[3] ?? 'DEFAULT'; op[4] = {}; }
  add(c);
}
// State diagram: tight-tree ranker.
for (let i = 0; i < 8; i++) {
  const c = randomCase(nm('mermaid-state'), { multigraph: true, compound: true, ids: 'mermaid', sizes: 'mermaid', labelProb: 0.5 });
  c.graph = { rankdir: pick(['TB', 'LR']), ranker: 'tight-tree', ranksep: pick([1, 50]), nodesep: pick([1, 50]) };
  for (const op of c.ops) if (op[0] === 'e') op[4] = { width: op[4].width ?? 1, height: op[4].height ?? 14, labelpos: 'c' };
  add(c);
}
// ER diagram.
for (let i = 0; i < 8; i++) {
  const c = randomCase(nm('mermaid-er'), { multigraph: true, compound: false, ids: 'mermaid', sizes: 'mermaid', labelProb: 0, selfLoops: chance(0.3) });
  c.graph = { rankdir: pick(['TB', 'BT', 'LR', 'RL']), marginx: 20, marginy: 20, nodesep: 100, edgesep: 100, ranksep: 100 };
  c.defEdge = true;
  for (const op of c.ops) if (op[0] === 'e') op[4] = {};
  add(c);
}
for (let i = 0; i < 40; i++) add(mermaidCase(nm('mermaid'), {}));
for (let i = 0; i < 50; i++) add(mermaidCase(nm('mermaid-cluster'), { clusters: true }));
for (let i = 0; i < 4; i++) add(mermaidCase(nm('mermaid-undefined-sep'), { undefinedSep: true, n: ri(2, 8) }));

// Cases that depend on the uniqueId counter. Each function gets n, the
// first id that the layout will make, and returns a case.
const dynamicCases = [
  // dagre gives a reversed edge the name 'rev' + id. Here the graph has an
  // edge with that name between the same nodes, so setEdge replaces the
  // label of that edge. This is the only known way for the counter to
  // change a result.
  (n) => ({
    name: 'counter-rev-collision',
    opts: { multigraph: true },
    graph: {},
    defEdge: false,
    ops: [['n', 'a', N(30, 20)], ['n', 'b', N(30, 20)], ['e', 'a', 'b', null, {}], ['e', 'b', 'a', null, {}], ['e', 'a', 'b', 'rev' + n, { width: 10, height: 10 }]],
  }),
  // Node ids that are equal to the next dummy node ids. dagre skips an id
  // that is in use, so the result does not depend on the counter.
  (n) => {
    const idList = ['sgA', 'sgB', 'x', 'y'];
    for (let j = 0; j < 60; j++) {
      for (const prefix of ['_root', '_bt', '_bb', '_ep', '_d', '_bl', '_br', '_se']) {
        if (chance(0.12)) idList.push(prefix + (n + j));
      }
    }
    return randomCase('counter-dummy-collision', { idList, clusterList: ['sgA', 'sgB'], compound: true, multigraph: true, labelProb: 0.6, selfLoops: true, cycles: true, shape: 'dag', acyclicer: ABSENT });
  },
];

// ---------------------------------------------------------------------------
// Debug dump of one case

function fmtNum(x) {
  if (Object.is(x, -0)) return '-0';
  return String(x);
}
function fmtFields(o, skip = []) {
  const parts = [];
  for (const k of Object.keys(o).sort()) {
    if (skip.includes(k)) continue;
    const v = o[k];
    if (v === undefined || v === null || v === false || v === '') continue;
    if (typeof v === 'number') {
      if (Number.isNaN(v)) continue;
      parts.push(`${k}=${fmtNum(v)}`);
    } else if (typeof v === 'string') parts.push(`${k}=${v}`);
    else if (v === true) parts.push(`${k}=true`);
    else if (k === 'points') parts.push(`${k}=[${v.map((p) => fmtNum(p.x) + ',' + fmtNum(p.y)).join(' ')}]`);
    else if (k === 'selfEdges') parts.push(`${k}=${v.length}`);
    else if (Array.isArray(v)) parts.push(`${k}=[${Array.from(v, (x) => (x === undefined ? '' : x)).join(',')}]`);
    else if (k === 'e' || k === 'edgeObj') parts.push(`${k}=${v.v}>${v.w}>${v.name ?? ''}`);
  }
  return parts.join(' ');
}
function dumpGraph(g) {
  const lines = [];
  lines.push('G ' + fmtFields(g.graph() ?? {}, ['maxRank', 'marginx', 'marginy']));
  for (const v of g.nodes()) {
    lines.push(`N ${JSON.stringify(v)} parent=${JSON.stringify(g.parent(v) ?? null)} ` + fmtFields(g.node(v) ?? {}, ['edgeLabel', 'label']));
  }
  for (const e of g.edges()) {
    lines.push(`E ${JSON.stringify(e.v)} ${JSON.stringify(e.w)} ${JSON.stringify(e.name ?? null)} ` + fmtFields(g.edge(e) ?? {}));
  }
  return lines;
}

function debugCase(name) {
  const golden = JSON.parse(gunzipSync(readFileSync(OUT)).toString());
  const c = golden.cases.find((x) => x.name === name);
  if (!c) throw new Error('no case named ' + name);
  const g = build(c);
  if (c.idStart > 1) bumpTo(c.idStart - 1);
  const at = counter();
  if (at !== c.idStart) throw new Error(`counter is ${at}, want ${c.idStart}`);

  // The layout graph is the first graph that gets a label during layout().
  let layoutGraph = null;
  const origSetGraph = Graph.prototype.setGraph;
  Graph.prototype.setGraph = function (label) {
    if (!layoutGraph) layoutGraph = this;
    return origSetGraph.call(this, label);
  };
  const stages = [];
  const origLog = console.log;
  console.log = (msg) => {
    const m = /^ {4}(\S+) time:/.exec(String(msg));
    if (m && layoutGraph) stages.push({ stage: m[1], lines: dumpGraph(layoutGraph) });
  };
  let err = null;
  try {
    layout(g, { debugTiming: true });
  } catch (e) {
    err = String(e && e.stack);
  } finally {
    console.log = origLog;
    Graph.prototype.setGraph = origSetGraph;
  }
  process.stdout.write(JSON.stringify({ name, err, stages }) + '\n');
}

// ---------------------------------------------------------------------------
// Main

if (process.argv[2] === '--debug') {
  debugCase(process.argv[3]);
} else {
  // Run every case. Then run it again at other counter values and record
  // whether the result depends on the uniqueId counter.
  const counterSensitive = [];
  const outCases = [];
  let errors = 0;
  const queue = [...dynamicCases, ...cases];
  for (let c of queue) {
    if (typeof c === 'function') {
      // run() reads the counter once, so the first id of the layout is the
      // value after next.
      c = c(counter() + 2);
    } else if (chance(0.05)) {
      // Move the counter forward now and then, also across a digit boundary.
      bumpTo(counter() + pick([7, 93, 1000, 12345, 100000]));
    }
    const r = run(c);
    const base = JSON.stringify(r.out ?? r.err);
    for (const jump of [1, 97, 1009]) {
      bumpTo(counter() + jump);
      const r2 = run(c);
      if (JSON.stringify(r2.out ?? r2.err) !== base) {
        counterSensitive.push(c.name);
        break;
      }
    }
    if (r.err !== undefined) errors++;
    outCases.push({ ...c, ...r });
  }
  const names = new Set(outCases.map((c) => c.name));
  if (names.size !== outCases.length) throw new Error('duplicate case names');
  const json = JSON.stringify({ generator: 'tools/reference/gen-dagre.mjs', dagre: 'dagre-d3-es@7.0.14', v8: process.versions.v8, counterSensitive, cases: outCases });
  if (OUT instanceof URL) mkdirSync(new URL('.', OUT), { recursive: true });
  const gz = gzipSync(json, { level: 9 });
  writeFileSync(OUT, gz);
  console.log(`${outCases.length} cases (${errors} throw), ${json.length} bytes, ${gz.length} gzipped`);
  console.log(`counter sensitive cases: ${counterSensitive.length ? counterSensitive.join(', ') : 'none'}`);
  console.log('errors:', outCases.filter((c) => c.err !== undefined).map((c) => `${c.name}: ${c.err}`).join('\n  '));
}
