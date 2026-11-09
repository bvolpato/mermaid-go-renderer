// Emits golden cases for internal/graphlib and internal/graphlib/alg from the
// graphlib of dagre-d3-es (the version that mermaid bundles).
//
// Usage (from tools/reference): node gen-graphlib.mjs
//
// Output:
//   internal/graphlib/testdata/graphlib.json.gz      Graph and json.js
//   internal/graphlib/alg/testdata/alg.json.gz       alg/*.js
//
// A graph case is a list of operations and a snapshot of every query method
// after the last operation. The snapshot fixes the enumeration order of
// nodes, edges, children, predecessors and successors. Labels are JSON
// values. The case also has the JSON form (write), and the snapshot of the
// graph that read builds from it.
//
// Distances are JSON numbers, or the string "Infinity".
import { writeFileSync, mkdirSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { Graph } from 'dagre-d3-es/src/graphlib/index.js';
import * as json from 'dagre-d3-es/src/graphlib/json.js';
import * as alg from 'dagre-d3-es/src/graphlib/alg/index.js';

// ---------------------------------------------------------------------------
// Seeded PRNG

let prngState = 0x51ed270b >>> 0;
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

const ID_POOLS = {
  alpha: ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l'],
  int: ['1', '2', '3', '10', '20', '0', '5', '7', '11', '100', '15', '4'],
  mixed: ['b', '10', 'a', '2', '1', 'x.y', '007', '1.5', '-1', '4294967295', '4294967294', 'é', '漢', '😀', 'length', 'A B', '3', 'z', '0', '1e3', ' ', '99'],
};

const LABELS = [null, 1, 0, -2.5, 'text', '', true, false, { a: 1 }, { 2: 'two', b: [1, 2, { c: null }], 1: 'one', a: 'x' }, [1, 'two', null], { width: 10, height: 20.5, label: 'N' }];

// The default label functions of a case with defaults. A name that is
// undefined or empty prints nothing, so that the Go port (which gets an
// empty name for both) can build the same string.
const defaultNodeLabel = (v) => 'node:' + v;
const defaultEdgeLabel = (v, w, name) => 'edge:' + v + '>' + w + (name === undefined || name === '' ? '' : '#' + name);

function randomOps(opts, pool, count) {
  const ops = [];
  const id = () => pick(pool);
  const label = () => pick(LABELS);
  // The empty name is not in this list: see the fixed cases below.
  const name = () => pick(['n1', 'n2', 'x', '0', 'null']);
  for (let i = 0; i < count; i++) {
    const r = R();
    if (r < 0.2) {
      ops.push(chance(0.6) ? ['setNode', id(), label()] : ['setNode', id()]);
    } else if (r < 0.55) {
      const v = id();
      const w = chance(0.08) ? v : id();
      const named = opts.multigraph ? chance(0.4) : chance(0.03);
      const form = R();
      if (form < 0.2) ops.push(['setEdge', v, w]);
      else if (form < 0.55) ops.push(named ? ['setEdge', v, w, label(), name()] : ['setEdge', v, w, label()]);
      else if (form < 0.8) ops.push(['setEdgeObj', named ? { v, w, name: pick(['n1', 'n2', 'x']) } : { v, w }, label()]);
      else ops.push(['setEdgeObj', named ? { v, w, name: pick(['n1', 'n2', 'x']) } : { v, w }]);
    } else if (r < 0.65) {
      ops.push(['removeNode', id()]);
    } else if (r < 0.75) {
      const v = id();
      const w = id();
      const form = R();
      if (form < 0.5) ops.push(['removeEdge', v, w]);
      else if (form < 0.75) ops.push(['removeEdge', v, w, name()]);
      else ops.push(['removeEdgeObj', chance(0.5) ? { v, w } : { v, w, name: pick(['n1', 'n2', 'x']) }]);
    } else if (r < 0.95) {
      if (opts.compound || chance(0.1)) ops.push(chance(0.85) ? ['setParent', id(), id()] : ['setParent', id()]);
      else ops.push(['setNode', id(), label()]);
    } else if (r < 0.975) {
      const vs = [id(), id(), id()];
      ops.push(chance(0.5) ? ['setNodes', vs, label()] : ['setNodes', vs]);
    } else {
      const vs = [id(), id(), id(), id()];
      ops.push(chance(0.5) ? ['setPath', vs, label()] : ['setPath', vs]);
    }
  }
  return ops;
}

// apply runs the operations. It returns the error message of each
// operation that throws, by operation index.
function apply(g, ops) {
  const errors = {};
  ops.forEach((op, i) => {
    try {
      const [kind, ...a] = op;
      switch (kind) {
        case 'setNode':
          a.length > 1 ? g.setNode(a[0], a[1]) : g.setNode(a[0]);
          break;
        case 'setNodes':
          a.length > 1 ? g.setNodes(a[0], a[1]) : g.setNodes(a[0]);
          break;
        case 'setPath':
          a.length > 1 ? g.setPath(a[0], a[1]) : g.setPath(a[0]);
          break;
        case 'setEdge':
          if (a.length === 2) g.setEdge(a[0], a[1]);
          else if (a.length === 3) g.setEdge(a[0], a[1], a[2]);
          else g.setEdge(a[0], a[1], a[2], a[3]);
          break;
        case 'setEdgeObj':
          a.length > 1 ? g.setEdge(a[0], a[1]) : g.setEdge(a[0]);
          break;
        case 'removeNode':
          g.removeNode(a[0]);
          break;
        case 'removeEdge':
          a.length > 2 ? g.removeEdge(a[0], a[1], a[2]) : g.removeEdge(a[0], a[1]);
          break;
        case 'removeEdgeObj':
          g.removeEdge(a[0]);
          break;
        case 'setParent':
          a.length > 1 ? g.setParent(a[0], a[1]) : g.setParent(a[0]);
          break;
        default:
          throw new Error('bad op ' + kind);
      }
    } catch (e) {
      errors[i] = String(e.message);
    }
  });
  return errors;
}

const edgeArr = (e) => [e.v, e.w, e.name === undefined ? null : e.name];
const edges = (list) => (list === undefined ? null : list.map(edgeArr));
const orNull = (x) => (x === undefined ? null : x);
// A label that is undefined is recorded as {"undefined": true}.
const lab = (x) => (x === undefined ? { undefined: true } : { value: x });

function snapshot(g, pool) {
  const probe = [...new Set([...g.nodes(), ...pool.slice(0, 4), 'missing'])];
  const snap = {
    directed: g.isDirected(),
    multigraph: g.isMultigraph(),
    compound: g.isCompound(),
    graph: lab(g.graph()),
    nodeCount: g.nodeCount(),
    edgeCount: g.edgeCount(),
    nodes: g.nodes(),
    edges: edges(g.edges()),
    sources: g.sources(),
    sinks: g.sinks(),
    rootChildren: orNull(g.children()),
    nodeLabels: g.nodes().map((v) => lab(g.node(v))),
    edgeLabels: g.edges().map((e) => lab(g.edge(e))),
    perNode: probe.map((v) => ({
      v,
      has: g.hasNode(v),
      parent: orNull(g.parent(v)),
      children: orNull(g.children(v)),
      predecessors: orNull(g.predecessors(v)),
      successors: orNull(g.successors(v)),
      neighbors: orNull(g.neighbors(v)),
      inEdges: edges(g.inEdges(v)),
      outEdges: edges(g.outEdges(v)),
      nodeEdges: edges(g.nodeEdges(v)),
      isLeaf: g.hasNode(v) ? g.isLeaf(v) : null,
    })),
    pairs: [],
  };
  // Edge queries between pairs of nodes.
  const some = g.nodes().slice(0, 6);
  for (const v of some) {
    for (const w of some) {
      snap.pairs.push({
        v,
        w,
        has: g.hasEdge(v, w),
        hasN1: g.hasEdge(v, w, 'n1'),
        hasObj: g.hasEdge({ v, w, name: 'x' }),
        label: lab(g.edge(v, w)),
        labelN1: lab(g.edge(v, w, 'n1')),
        inEdges: edges(g.inEdges(v, w)),
        outEdges: edges(g.outEdges(v, w)),
        nodeEdges: edges(g.nodeEdges(v, w)),
      });
    }
  }
  return snap;
}

// keep is the filter of the filterNodes test: it depends on the id only.
const keep = (v) => {
  // filterNodes passes an array index (a number) when lodash treats the
  // node map as an array. See arrayLikeLength in graph.go.
  v = String(v);
  let h = 7;
  for (let i = 0; i < v.length; i++) h = (Math.imul(h, 31) + v.charCodeAt(i)) | 0;
  return (h & 3) !== 0;
};

// ---------------------------------------------------------------------------
// Graph cases

const graphCases = [];
let seq = 0;
function addGraphCase(opts, poolName, count, withDefaults) {
  const pool = ID_POOLS[poolName];
  const ops = randomOps(opts, pool, count);
  const graphLabel = pick([undefined, null, 'g', { rankdir: 'TB', 10: 1, a: [1] }, 5]);
  const g = new Graph(opts);
  if (graphLabel !== undefined) g.setGraph(graphLabel);
  if (withDefaults) {
    g.setDefaultNodeLabel(defaultNodeLabel);
    g.setDefaultEdgeLabel(defaultEdgeLabel);
  }
  const errors = apply(g, ops);
  const written = JSON.stringify(json.write(g));
  const g2 = json.read(JSON.parse(written));
  const filtered = g.filterNodes(keep);
  graphCases.push({
    name: `graph-${String(++seq).padStart(3, '0')}-${poolName}`,
    opts,
    graphLabel: lab(graphLabel),
    withDefaults,
    pool: poolName,
    ops,
    errors,
    snap: snapshot(g, pool),
    written,
    readSnap: snapshot(g2, pool),
    rewritten: JSON.stringify(json.write(g2)),
    filteredSnap: snapshot(filtered, pool),
  });
}

const OPTS = [
  {},
  { multigraph: true },
  { compound: true },
  { multigraph: true, compound: true },
  { directed: false },
  { directed: false, multigraph: true, compound: true },
  { directed: true, multigraph: false, compound: false },
];
for (const opts of OPTS) {
  for (const poolName of Object.keys(ID_POOLS)) {
    for (let i = 0; i < 6; i++) addGraphCase(opts, poolName, ri(3, 60), i % 2 === 1);
    addGraphCase(opts, poolName, ri(100, 250), true);
  }
}

// An edge with the empty name '' has an edge object without a name, so the
// methods that take an edge object do not find it, and removeNode leaves it
// in the graph. These cases have no parallel unnamed edge: with one,
// removeNode throws a TypeError and leaves the graph in a broken state,
// which the port does not follow.
function addFixedCase(name, opts, ops) {
  const pool = ID_POOLS.alpha;
  const g = new Graph(opts);
  const errors = apply(g, ops);
  const written = JSON.stringify(json.write(g));
  const g2 = json.read(JSON.parse(written));
  graphCases.push({ name, opts, graphLabel: lab(undefined), withDefaults: false, pool: 'alpha', ops, errors, snap: snapshot(g, pool), written, readSnap: snapshot(g2, pool), rewritten: JSON.stringify(json.write(g2)), filteredSnap: snapshot(g.filterNodes(keep), pool) });
}
addFixedCase('fixed-empty-name', { multigraph: true }, [['setEdge', 'a', 'b', 'L1', ''], ['setEdge', 'a', 'c', 'L2', 'n1'], ['setEdge', 'b', 'c', 'L3']]);
addFixedCase('fixed-empty-name-update', { multigraph: true }, [['setEdge', 'a', 'b', 'L1', ''], ['setEdge', 'a', 'b', 'L1b', ''], ['setEdgeObj', { v: 'a', w: 'b' }, 'unnamed'], ['removeEdge', 'a', 'b', ''], ['setEdge', 'c', 'a', 'L4', '']]);
addFixedCase('fixed-empty-name-remove-node', { multigraph: true, compound: true }, [['setEdge', 'a', 'b', 'L1', ''], ['setEdge', 'c', 'a', 'L2'], ['setParent', 'b', 'p'], ['removeNode', 'a'], ['setNode', 'd', 1]]);
addFixedCase('fixed-empty-name-remove-obj', { multigraph: true }, [['setEdge', 'a', 'b', 'L1', ''], ['removeEdgeObj', { v: 'a', w: 'b' }], ['setEdge', 'b', 'a', 'L2', 'x'], ['removeEdgeObj', { v: 'b', w: 'a', name: 'x' }]]);
// Node ids that are names of functions of Object.prototype. The graph counts
// the edges between two nodes in plain objects, and miscounts for these ids.
addFixedCase('fixed-proto-ids', { multigraph: true }, [['setEdge', 'a', 'constructor', 1, 'x'], ['setEdge', 'a', 'constructor', 2, 'y'], ['setEdge', 'a', 'constructor', 3, 'n1'], ['setEdge', 'toString', 'a', 4, 'x'], ['setEdge', 'toString', 'a', 5], ['setEdge', 'b', 'valueOf', 6], ['setEdge', 'valueOf', 'constructor', 7], ['removeEdge', 'a', 'constructor', 'x'], ['removeEdge', 'toString', 'a', 'x'], ['setEdge', 'hasOwnProperty', 'hasOwnProperty', 8], ['setEdge', 'c', 'toString', 9], ['setEdge', 'c', 'toString', 10, 'n2']]);
addFixedCase('fixed-proto-ids-remove', { multigraph: true, compound: true }, [['setEdge', 'a', 'constructor', 1, 'x'], ['setEdge', 'a', 'constructor', 2, 'y'], ['removeEdge', 'a', 'constructor', 'y'], ['removeEdge', 'a', 'constructor', 'x'], ['setEdge', 'a', 'toString', 3], ['setEdge', 'a', 'toString', 4, 'n1'], ['setEdge', 'a', 'toString', 5, 'n2'], ['removeEdge', 'a', 'toString'], ['setParent', 'toString', 'valueOf'], ['setParent', 'a', 'valueOf'], ['removeNode', 'b'], ['setEdge', 'constructor', 'a', 6], ['removeNode', 'constructor'], ['setEdge', 'd', 'isPrototypeOf', 1], ['setEdge', 'd', 'isPrototypeOf', 1, 'n1'], ['removeEdge', 'd', 'isPrototypeOf', 'n1']]);
addFixedCase('fixed-proto-ids-undirected', { multigraph: true, directed: false }, [['setEdge', 'toString', 'a', 1, 'x'], ['setEdge', 'a', 'toString', 2, 'y'], ['setEdge', 'constructor', 'toString', 3], ['setEdge', 'toString', 'constructor', 4, 'n1'], ['removeEdge', 'a', 'toString', 'x'], ['removeEdge', 'constructor', 'toString', 'n1']]);
addFixedCase('fixed-length-node', { compound: true }, [['setNode', 'length', 3], ['setNode', '1', 'one'], ['setNode', 'a', 'A'], ['setNode', '0', null], ['setEdge', 'a', '1', 5], ['setEdge', '0', '1', 6], ['setParent', '1', 'length']]);
addFixedCase('fixed-length-node-object-label', {}, [['setNode', 'length', { a: 1 }], ['setNode', '1', 'one'], ['setNode', 'a', 'A'], ['setEdge', 'a', '1', 5], ['setEdge', 'length', 'a', 7]]);

// read() of JSON that write() does not produce.
const readCases = [
  { options: { directed: true, multigraph: true, compound: true }, nodes: [{ v: 1 }, { v: 'b', value: 5, parent: 1 }, { v: 'c', parent: '' }], edges: [{ v: 1, w: 'b', name: null }, { v: 'b', w: 2, name: 7, value: 'x' }, { v: 'b', w: 'c', name: '' }] },
  { nodes: [{ v: 'a' }], edges: [{ v: 'a', w: 'b' }] },
  { options: {}, nodes: [], edges: [] },
  { options: { directed: false }, value: null, nodes: [{ v: 'b' }, { v: 'a', value: null }], edges: [{ v: 'b', w: 'a', value: null }] },
  { options: { directed: 0, multigraph: 1, compound: 'yes' }, value: { x: 1 }, nodes: [{ v: 'p' }, { v: 'q', parent: 'p' }], edges: [{ v: 'q', w: 'p', name: 'e' }] },
].map((input) => {
  const g = json.read(input);
  return { input: JSON.stringify(input), snap: snapshot(g, ['a', 'b', 'c', '1']), nodeJSON: JSON.stringify(json.write(g).nodes), edgeJSON: JSON.stringify(json.write(g).edges) };
});

// ---------------------------------------------------------------------------
// alg cases. Each case runs on the graph that read() builds from a JSON
// form, so that the Go test can build the same graph.

const dist = (x) => (x === Infinity ? 'Infinity' : x === -Infinity ? '-Infinity' : Number.isNaN(x) ? 'NaN' : x);
const pathResult = (res) => Object.keys(res).map((v) => [v, dist(res[v].distance), orNull(res[v].predecessor)]);
const attempt = (fn) => {
  try {
    return { result: fn() };
  } catch (e) {
    return { error: e instanceof Error ? String(e.message) : 'CycleException' };
  }
};

const algCases = [];
function addAlgCase(opts, poolName, nodeCount, edgeCount, o = {}) {
  const pool = ID_POOLS[poolName].slice(0, nodeCount);
  const g0 = new Graph(opts);
  for (const v of pool) g0.setNode(v);
  for (let i = 0; i < edgeCount; i++) {
    let v = pick(pool);
    let w = pick(pool);
    if (o.dag) {
      const a = pool.indexOf(v);
      const b = pool.indexOf(w);
      if (a === b) continue;
      if (a > b) [v, w] = [w, v];
    }
    if (v === w && !o.selfLoops) continue;
    const weight = o.negative && chance(0.1) ? -1 : pick([1, 1, 2, 3, 0.5, 7, 0]);
    if (opts.multigraph && chance(0.3)) g0.setEdge(v, w, weight, pick(['n1', 'n2']));
    else g0.setEdge(v, w, weight);
  }
  const written = JSON.stringify(json.write(g0));
  const g = json.read(JSON.parse(written));
  const weightFn = (e) => g.edge(e);
  const undirectedEdges = (v) => g.nodeEdges(v);
  const nodes = g.nodes();
  const source = nodes.length ? pick(nodes) : 'missing';
  const starts = nodes.length ? [pick(nodes), pick(nodes)] : [];
  algCases.push({
    name: `alg-${String(algCases.length + 1).padStart(3, '0')}-${poolName}`,
    written,
    source,
    starts,
    components: alg.components(g),
    preorder: attempt(() => alg.preorder(g, starts)),
    postorder: attempt(() => alg.postorder(g, starts)),
    preorderAll: attempt(() => alg.preorder(g, nodes)),
    postorderAll: attempt(() => alg.postorder(g, nodes)),
    preorderMissing: attempt(() => alg.preorder(g, [...starts, 'missing'])),
    tarjan: alg.tarjan(g),
    findCycles: alg.findCycles(g),
    topsort: attempt(() => alg.topsort(g)),
    isAcyclic: alg.isAcyclic(g),
    dijkstra: attempt(() => pathResult(alg.dijkstra(g, source, weightFn))),
    dijkstraDefault: attempt(() => pathResult(alg.dijkstra(g, source))),
    dijkstraUndirected: attempt(() => pathResult(alg.dijkstra(g, source, weightFn, undirectedEdges))),
    dijkstraAll: attempt(() => {
      const res = alg.dijkstraAll(g, weightFn);
      return Object.keys(res).map((v) => [v, pathResult(res[v])]);
    }),
    floydWarshall: attempt(() => {
      const res = alg.floydWarshall(g, weightFn);
      return Object.keys(res).map((v) => [v, pathResult(res[v])]);
    }),
    prim: attempt(() => {
      const t = alg.prim(g, weightFn);
      return { nodes: t.nodes(), edges: edges(t.edges()) };
    }),
  });
}

for (const poolName of Object.keys(ID_POOLS)) {
  for (let i = 0; i < 30; i++) {
    const n = ri(0, 12);
    addAlgCase(pick([{}, {}, { multigraph: true }, { directed: false }]), poolName, n, ri(0, n * 2), { dag: chance(0.4), selfLoops: chance(0.2), negative: chance(0.15) });
  }
  for (let i = 0; i < 10; i++) {
    // Connected undirected graphs, for prim.
    const n = ri(2, 12);
    const pool = ID_POOLS[poolName].slice(0, n);
    const g0 = new Graph({ directed: false });
    for (let k = 1; k < n; k++) g0.setEdge(pool[k], pool[ri(0, k - 1)], pick([1, 2, 3, 5, 0.5]));
    for (let k = 0; k < n; k++) g0.setEdge(pick(pool), pick(pool), pick([1, 2, 3, 5, 4]));
    const written = JSON.stringify(json.write(g0));
    const g = json.read(JSON.parse(written));
    const weightFn = (e) => g.edge(e);
    const nodes = g.nodes();
    algCases.push({
      name: `alg-${String(algCases.length + 1).padStart(3, '0')}-${poolName}-connected`,
      written,
      source: nodes[0],
      starts: [nodes[0]],
      components: alg.components(g),
      preorder: attempt(() => alg.preorder(g, [nodes[0]])),
      postorder: attempt(() => alg.postorder(g, [nodes[0]])),
      preorderAll: attempt(() => alg.preorder(g, nodes)),
      postorderAll: attempt(() => alg.postorder(g, nodes)),
      preorderMissing: attempt(() => alg.preorder(g, [nodes[0], 'missing'])),
      tarjan: alg.tarjan(g),
      findCycles: alg.findCycles(g),
      topsort: attempt(() => alg.topsort(g)),
      isAcyclic: alg.isAcyclic(g),
      dijkstra: attempt(() => pathResult(alg.dijkstra(g, nodes[0], weightFn))),
      dijkstraDefault: attempt(() => pathResult(alg.dijkstra(g, nodes[0]))),
      dijkstraUndirected: attempt(() => pathResult(alg.dijkstra(g, nodes[0], weightFn, (v) => g.nodeEdges(v)))),
      dijkstraAll: attempt(() => {
        const res = alg.dijkstraAll(g, weightFn);
        return Object.keys(res).map((v) => [v, pathResult(res[v])]);
      }),
      floydWarshall: attempt(() => {
        const res = alg.floydWarshall(g, weightFn);
        return Object.keys(res).map((v) => [v, pathResult(res[v])]);
      }),
      prim: attempt(() => {
        const t = alg.prim(g, weightFn);
        return { nodes: t.nodes(), edges: edges(t.edges()) };
      }),
    });
  }
}

// ---------------------------------------------------------------------------

function write(rel, data) {
  const url = new URL(rel, import.meta.url);
  mkdirSync(new URL('.', url), { recursive: true });
  const text = JSON.stringify(data);
  const gz = gzipSync(text, { level: 9 });
  writeFileSync(url, gz);
  console.log(`${rel}: ${text.length} bytes, ${gz.length} gzipped`);
}
const meta = { generator: 'tools/reference/gen-graphlib.mjs', dagre: 'dagre-d3-es@7.0.14' };
write('../../internal/graphlib/testdata/graphlib.json.gz', { ...meta, cases: graphCases, readCases });
write('../../internal/graphlib/alg/testdata/alg.json.gz', { ...meta, cases: algCases });
console.log(`${graphCases.length} graph cases, ${readCases.length} read cases, ${algCases.length} alg cases`);
const errCount = graphCases.reduce((n, c) => n + Object.keys(c.errors).length, 0);
console.log(`${errCount} operations throw`);
