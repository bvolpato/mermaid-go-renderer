// Emits golden cases for the port of mermaid-graphlib.js in
// internal/ru/layout/dagre (layer 2 of its tests).
//
//   node gen-dagre-graphlib.mjs      (from tools/reference or the repository root)
//
// Output: internal/ru/layout/dagre/testdata/graphlib.json.gz
//
// The functions are the ones of the built mermaid 12.2.0
// (dist/chunks/mermaid.core): adjustClustersAndEdges, extractor,
// findNonClusterChild, sortNodesByHierarchy, extractDescendants and clear.
// A module load hook appends an export statement to the chunk for the
// functions and the Maps that the chunk does not export. The build drops
// validate, because nothing calls it, so validate comes from the upstream
// source file. The source file has all the functions, and the generator
// stops when a result of the source differs from the result of the build.
//
// A case is a random compound multigraph, given as the list of setNode,
// setParent and setEdge calls that build it, and the results:
//
//   validate     validate(graph)
//   sorted       sortNodesByHierarchy(graph)
//   nonCluster   findNonClusterChild(id, graph, clusterId) for every node,
//                with clusterId undefined, id and another node
//   descendants  extractDescendants(id, graph) for every node, and the
//                parents Map after these calls
//   adjust       the graph, clusterDb, descendants and parents after
//                clear() and adjustClustersAndEdges(graph), or the error
//   extract      the graph after extractor(graph, 0) on a second copy of
//                the graph, with the clusterDb that adjust left, or the error
//
// The graphs are printed by the functions of probes/dagre-layout-spy.js.
import { registerHooks } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { Graph } from 'dagre-d3-es/src/graphlib/index.js';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, '../..');
const outFile = path.join(root, 'internal/ru/layout/dagre/testdata/graphlib.json.gz');
const CORE = path.join(here, 'node_modules/mermaid/dist/chunks/mermaid.core');
const SOURCE = path.join(root, '.upstream/mermaid-12.2.0/packages/mermaid/src/rendering-util/layout-algorithms/dagre/mermaid-graphlib.js');

const chunkFile = fs.readdirSync(CORE).find((f) => f.endsWith('.mjs') && fs.readFileSync(path.join(CORE, f), 'utf8').includes('\nvar adjustClustersAndEdges = '));
const chunkSource = fs.readFileSync(path.join(CORE, chunkFile), 'utf8');
// The build renames clear, because other modules of the chunk have a clear.
const clearName = chunkSource.match(/\nvar (clear\d*) = [^\n]*\n\s+descendants\.clear\(\);/)[1];

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL === pathToFileURL(SOURCE).href) {
      if (specifier === '../../../logger.js') {
        return { url: 'data:text/javascript,export const log = new Proxy({}, { get: () => () => {} });', shortCircuit: true };
      }
      // The snapshot of the source has no node_modules.
      return nextResolve(specifier, { ...context, parentURL: import.meta.url });
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/' + chunkFile)) {
      const source =
        result.source.toString() +
        `\nexport { descendants as __descendants, parents as __parents, ${clearName} as __clear, extractDescendants as __extractDescendants, extractor as __extractor };\n`;
      return { ...result, source };
    }
    return result;
  },
});

const built = await import(pathToFileURL(path.join(CORE, chunkFile)).href);
const source = await import(pathToFileURL(SOURCE).href);
// The module state of the source is private, and the results below do not
// need it: the graph after the call shows what the state was.
const impls = {
  built: {
    clusterDb: built.clusterDb,
    descendants: built.__descendants,
    parents: built.__parents,
    clear: built.__clear,
    extractDescendants: built.__extractDescendants,
    findNonClusterChild: built.findNonClusterChild,
    adjustClustersAndEdges: built.adjustClustersAndEdges,
    extractor: built.__extractor,
    sortNodesByHierarchy: built.sortNodesByHierarchy,
  },
  source: {
    clusterDb: source.clusterDb,
    clear: source.clear,
    extractDescendants: source.extractDescendants,
    findNonClusterChild: source.findNonClusterChild,
    adjustClustersAndEdges: source.adjustClustersAndEdges,
    extractor: source.extractor,
    sortNodesByHierarchy: source.sortNodesByHierarchy,
  },
};

// The serialization functions of the spy, so that both golden files have
// one format.
const spy = fs.readFileSync(path.join(here, 'probes/dagre-layout-spy.js'), 'utf8');
const section = spy.slice(spy.indexOf('// ---- serialization ----'), spy.indexOf('// ---- the spy ----'));
const projector = (clusterDb) => new Function('clusterDb', `${section}\nreturn { projectGraph, projectClusterDb };`)(clusterDb);

// ---- seeded PRNG ----
let prngState = 0x64616772 >>> 0; // "dagr"
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
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(R() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
};

const POOLS = [
  ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N'],
  // Integer-like ids come first in the key order of a JavaScript object.
  ['10', '2', '1', '30', '4', '7', '11', '100', '5', '0', '21', '3', '9', '8'],
  ['b', '10', 'a', '2', 'sg1', 'x.y', '007', 'node-1', 'é', 'A B', '3', 'z', '99', 'toString'],
];
const DIRS = ['TB', 'LR', 'BT', 'RL'];

// randomCase builds the calls of a random compound graph.
//
// pParent is the chance that a node gets a parent. pClusterEdge is the
// chance that an end of an edge may be a node with children.
const randomCase = (name, { n, pParent, maxDepth, edgeFactor, pClusterEdge, pSelfLoop, pNoLabel, pUnnamed, pNoEdgeLabel = 0 }) => {
  const ids = shuffle(pick(POOLS)).slice(0, n);
  const parent = new Map();
  const depth = new Map();
  for (let i = 0; i < ids.length; i++) {
    depth.set(ids[i], 0);
    if (i > 0 && chance(pParent)) {
      const p = ids[ri(0, i - 1)];
      if (depth.get(p) < maxDepth) {
        parent.set(ids[i], p);
        depth.set(ids[i], depth.get(p) + 1);
      }
    }
  }
  const clusters = new Set(parent.values());
  const leaves = ids.filter((id) => !clusters.has(id));
  const ops = [];
  // The nodes are set in a random order, so a parent can be set after its
  // child: setParent then creates the parent first, without a label.
  for (const id of shuffle(ids)) {
    if (!(clusters.has(id) && chance(pNoLabel))) {
      const label = { id };
      if (clusters.has(id)) {
        label.isGroup = true;
        if (chance(0.3)) {
          label.dir = pick(DIRS);
        }
      }
      ops.push(['n', id, label]);
    }
    if (parent.has(id)) {
      ops.push(['p', id, parent.get(id)]);
    }
  }
  const m = Math.round(R() * edgeFactor * n);
  for (let k = 0; k < m; k++) {
    const end = () => (chance(pClusterEdge) ? pick(ids) : pick(leaves.length > 0 ? leaves : ids));
    const v = end();
    const w = chance(pSelfLoop) ? v : end();
    const edgeName = chance(pUnnamed) ? null : `e${k}`;
    // An edge without a label makes adjustClustersAndEdges throw when it
    // moves the edge.
    ops.push(['e', v, w, edgeName, chance(pNoEdgeLabel) ? null : { id: `e${k}` }]);
  }
  return { name, rankdir: pick(DIRS), ops };
};

const build = (c) => {
  const g = new Graph({ multigraph: true, compound: true })
    .setGraph({ rankdir: c.rankdir, nodesep: 50, ranksep: 50, marginx: 8, marginy: 8 })
    .setDefaultEdgeLabel(() => ({}));
  for (const op of c.ops) {
    if (op[0] === 'n') {
      g.setNode(op[1], structuredClone(op[2]));
    } else if (op[0] === 'p') {
      g.setParent(op[1], op[2]);
    } else if (op[3] === null) {
      g.setEdge(op[1], op[2], op[4] === null ? undefined : structuredClone(op[4]));
    } else {
      g.setEdge(op[1], op[2], op[4] === null ? undefined : structuredClone(op[4]), op[3]);
    }
  }
  return g;
};

const orNull = (v) => (v === undefined ? null : v);
const message = (e) => String((e && e.message) || e);

// run computes the results of a case with one implementation.
const run = (c, impl) => {
  const { projectGraph, projectClusterDb } = projector(impl.clusterDb);
  const out = {};
  let g = build(c);
  const nodes = g.nodes();
  if (impl.validate) {
    out.validate = impl.validate(g);
  }
  out.sorted = impl.sortNodesByHierarchy(g);
  out.nonCluster = nodes.map((id, i) => {
    const other = nodes[(i * 7 + 3) % nodes.length];
    return [id, other, orNull(impl.findNonClusterChild(id, g)), orNull(impl.findNonClusterChild(id, g, id)), orNull(impl.findNonClusterChild(id, g, other))];
  });
  impl.clear();
  out.descendants = nodes.map((id) => [id, impl.extractDescendants(id, g)]);
  if (impl.parents) {
    out.parents = [...impl.parents.entries()];
  }

  impl.clear();
  g = build(c);
  out.adjust = {};
  try {
    impl.adjustClustersAndEdges(g);
    out.adjust.graph = projectGraph(g, true);
  } catch (e) {
    out.adjust.error = message(e);
  }
  out.adjust.clusterDb = projectClusterDb();
  if (impl.descendants) {
    out.adjust.descendants = [...impl.descendants.entries()];
    out.adjust.parents = [...impl.parents.entries()];
  }

  // extractor on a second copy, with the clusterDb of the first copy.
  g = build(c);
  out.extract = {};
  try {
    impl.extractor(g, 0);
    out.extract.graph = projectGraph(g, true);
  } catch (e) {
    out.extract.error = message(e);
  }
  return out;
};

const cases = [];
const add = (name, params) => cases.push(randomCase(`${name}-${cases.length}`, params));
for (let i = 0; i < 60; i++) {
  // Clusters whose edges stay inside: they are extracted.
  add('closed', { n: ri(2, 12), pParent: 0.6, maxDepth: 3, edgeFactor: 1.2, pClusterEdge: 0, pSelfLoop: 0, pNoLabel: 0, pUnnamed: 0 });
}
for (let i = 0; i < 120; i++) {
  // Edges between the leaves of all clusters.
  add('leaves', { n: ri(3, 14), pParent: 0.7, maxDepth: 4, edgeFactor: 1.5, pClusterEdge: 0, pSelfLoop: 0.03, pNoLabel: 0, pUnnamed: 0.05 });
}
for (let i = 0; i < 160; i++) {
  // Edges that end at clusters: the anchor logic.
  add('anchors', { n: ri(3, 14), pParent: 0.7, maxDepth: 4, edgeFactor: 1.2, pClusterEdge: 0.35, pSelfLoop: 0.03, pNoLabel: 0, pUnnamed: 0.03 });
}
for (let i = 0; i < 60; i++) {
  // Few edges, deep nesting, and clusters without a label.
  add('deep', { n: ri(4, 14), pParent: 0.9, maxDepth: 6, edgeFactor: 0.6, pClusterEdge: 0.2, pSelfLoop: 0.05, pNoLabel: 0.15, pUnnamed: 0 });
}
for (let i = 0; i < 40; i++) {
  // Dense graphs.
  add('dense', { n: ri(2, 8), pParent: 0.5, maxDepth: 3, edgeFactor: 3, pClusterEdge: 0.5, pSelfLoop: 0.1, pNoLabel: 0.05, pUnnamed: 0.1 });
}

for (let i = 0; i < 60; i++) {
  // Edges without a label: upstream throws where it writes to the label.
  add('nolabel', { n: ri(3, 12), pParent: 0.7, maxDepth: 4, edgeFactor: 1.2, pClusterEdge: 0.4, pSelfLoop: 0.03, pNoLabel: 0.05, pUnnamed: 0.03, pNoEdgeLabel: 0.3 });
}
// The safe anchor of a cluster: a cluster with an edge of its own and an
// edge of a descendant to the outside, whose first descendants are inside
// a closed cluster. Optionally all of it is inside a closed cluster.
const safeAnchorCase = (name) => {
  const ops = [];
  const group = (id, parent) => {
    ops.push(['n', id, { id, isGroup: true }]);
    if (parent) {
      ops.push(['p', id, parent]);
    }
  };
  const leaf = (id, parent) => {
    ops.push(['n', id, { id }]);
    if (parent) {
      ops.push(['p', id, parent]);
    }
  };
  let k = 0;
  const edge = (v, w) => ops.push(['e', v, w, `e${k}`, { id: `e${k++}` }]);
  const world = chance(0.5) ? 'World' : null;
  if (world) {
    group(world, null);
  }
  group('Outer', world);
  const members = []; // the children of Outer, in the order of their definition
  const inner = ri(1, 3);
  const outerLeaves = ri(0, 2);
  const kinds = shuffle([...Array.from({ length: inner }, (_, i) => `Inner${i}`), ...Array.from({ length: outerLeaves }, (_, i) => `c${i}`)]);
  const innerLeaves = [];
  for (const id of kinds) {
    if (id.startsWith('Inner')) {
      const parent = chance(0.2) && members.some((m) => m.startsWith('Inner')) ? pick(members.filter((m) => m.startsWith('Inner'))) : 'Outer';
      group(id, parent);
      const count = ri(1, 3);
      const mine = [];
      for (let i = 0; i < count; i++) {
        leaf(`${id}_${i}`, id);
        mine.push(`${id}_${i}`);
      }
      for (let i = 1; i < mine.length; i++) {
        if (chance(0.7)) {
          edge(mine[i - 1], mine[i]);
        }
      }
      innerLeaves.push(...mine);
    } else {
      leaf(id, 'Outer');
    }
    members.push(id);
  }
  leaf('d', world);
  leaf('e', world);
  const leavesOfOuter = members.filter((m) => !m.startsWith('Inner'));
  // Edges of descendants to the outside.
  for (const id of leavesOfOuter) {
    if (chance(0.6)) {
      edge(id, 'd');
    }
  }
  for (const id of innerLeaves) {
    if (chance(0.15)) {
      chance(0.5) ? edge(id, 'd') : edge('d', id);
    }
  }
  // Edges of the clusters themselves.
  if (chance(0.85)) {
    edge('Outer', 'e');
  }
  if (chance(0.3)) {
    edge('e', 'Outer');
  }
  for (const id of members.filter((m) => m.startsWith('Inner'))) {
    if (chance(0.2)) {
      chance(0.5) ? edge(id, 'd') : edge(id, pick(members));
    }
  }
  return { name, rankdir: pick(DIRS), ops };
};
for (let i = 0; i < 120; i++) {
  cases.push(safeAnchorCase(`safe-anchor-${cases.length}`));
}

// A chain of nested clusters that is deeper than the limit of extractor.
for (const [name, depth, edges] of [
  ['chain-9', 9, true],
  ['chain-11', 11, true],
  ['chain-12', 12, true],
  ['chain-14', 14, true],
  ['chain-14-no-edges', 14, false],
]) {
  const ops = [];
  for (let i = 0; i < depth; i++) {
    ops.push(['n', `c${i}`, { id: `c${i}`, isGroup: true }]);
    if (i > 0) {
      ops.push(['p', `c${i}`, `c${i - 1}`]);
    }
    ops.push(['n', `leaf${i}a`, { id: `leaf${i}a` }], ['p', `leaf${i}a`, `c${i}`]);
    ops.push(['n', `leaf${i}b`, { id: `leaf${i}b` }], ['p', `leaf${i}b`, `c${i}`]);
    if (edges) {
      ops.push(['e', `leaf${i}a`, `leaf${i}b`, `e${i}`, { id: `e${i}` }]);
    }
  }
  cases.push({ name, rankdir: 'TB', ops });
}

let errors = 0;
let extracted = 0;
let external = 0;
const golden = cases.map((c) => {
  const want = run(c, { ...impls.built });
  const check = run(c, { ...impls.source, validate: source.validate });
  for (const key of ['sorted', 'nonCluster', 'descendants']) {
    if (JSON.stringify(want[key]) !== JSON.stringify(check[key])) {
      throw new Error(`${c.name}: ${key} of the build differs from the source`);
    }
  }
  for (const key of ['adjust', 'extract']) {
    if (JSON.stringify(want[key].graph) !== JSON.stringify(check[key].graph) || want[key].error !== check[key].error) {
      throw new Error(`${c.name}: ${key} of the build differs from the source`);
    }
  }
  if (JSON.stringify(want.adjust.clusterDb) !== JSON.stringify(check.adjust.clusterDb)) {
    throw new Error(`${c.name}: clusterDb of the build differs from the source`);
  }
  want.validate = check.validate;
  if (want.adjust.error) {
    errors++;
  }
  if (JSON.stringify(want.adjust.graph || '').includes('"clusterNode":true')) {
    extracted++;
  }
  if (want.adjust.clusterDb.some((e) => e.externalConnections)) {
    external++;
  }
  return { ...c, ...want };
});

const json = JSON.stringify({ generator: 'tools/reference/gen-dagre-graphlib.mjs', mermaid: '12.2.0', cases: golden });
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(json, { level: 9 }));
console.error(
  `${golden.length} cases: ${extracted} with an extracted cluster, ${external} with external connections, ${errors} where adjustClustersAndEdges throws -> ${path.relative(root, outFile)} (${fs.statSync(outFile).size} bytes)`
);
