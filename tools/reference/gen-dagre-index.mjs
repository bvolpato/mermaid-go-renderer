// Emits golden cases for the functions that the dagre layout module of
// mermaid 12.2.0 exports and that need no DOM: prepareLayoutForDagre,
// getEdgesToRender and applyDagreLayoutResult. The Go port is
// internal/ru/layout/dagre.
//
//   node gen-dagre-index.mjs      (from tools/reference or the repository root)
//
// Output: internal/ru/layout/dagre/testdata/index.json.gz
//
// The functions are the ones of the built mermaid (dist/chunks/mermaid.core).
// The real renders of gen-dagre-layout.mjs reach these functions too, but a
// laid-out graph of dagre never has, for example, a self-loop whose dummy
// nodes have no position. The cases here are built by hand and at random,
// so that every branch runs:
//
//   prepare  random layout data, with self-loops on nodes and on groups,
//            edges that end at groups, and spacings that are numbers,
//            strings or undefined. The result is the graph, with the nested
//            cluster graphs, and clusterDb.
//   edges    getEdgesToRender on a graph with positions: self-loop groups
//            of one to four segments, with and without positions of the
//            dummy nodes, for every rankdir.
//   apply    applyDagreLayoutResult on a compound graph with positions.
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
const outFile = path.join(root, 'internal/ru/layout/dagre/testdata/index.json.gz');
const CORE = path.join(here, 'node_modules/mermaid/dist/chunks/mermaid.core');

const files = fs.readdirSync(CORE).filter((f) => f.endsWith('.mjs'));
const graphlibChunk = files.find((f) => fs.readFileSync(path.join(CORE, f), 'utf8').includes('\nvar adjustClustersAndEdges = '));
const dagreChunk = files.find((f) => f.startsWith('dagre-'));
const clearName = fs.readFileSync(path.join(CORE, graphlibChunk), 'utf8').match(/\nvar (clear\d*) = [^\n]*\n\s+descendants\.clear\(\);/)[1];

registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/' + graphlibChunk)) {
      return { ...result, source: result.source.toString() + `\nexport { ${clearName} as __clear };\n` };
    }
    return result;
  },
});
const graphlibModule = await import(pathToFileURL(path.join(CORE, graphlibChunk)).href);
const { prepareLayoutForDagre, getEdgesToRender, applyDagreLayoutResult } = await import(pathToFileURL(path.join(CORE, dagreChunk)).href);
const clusterDb = graphlibModule.clusterDb;
const clear = graphlibModule.__clear;

// The serialization functions of the spy, so that the golden files have
// one format.
const spy = fs.readFileSync(path.join(here, 'probes/dagre-layout-spy.js'), 'utf8');
const section = spy.slice(spy.indexOf('// ---- serialization ----'), spy.indexOf('// ---- the spy ----'));
const { projectGraph, projectClusterDb, projectInput, projectEdge, pos, dyn } = new Function(
  'clusterDb',
  `${section}\nreturn { projectGraph, projectClusterDb, projectInput, projectEdge, pos, dyn };`
)(clusterDb);

// ---- seeded PRNG ----
let prngState = 0x696e6478 >>> 0; // "indx"
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
// A coordinate or size: often not a whole number, so that the arithmetic
// of the self-loop points rounds.
const coord = (lo, hi) => ri(lo, hi) + pick([0, 0, 0.5, 0.25, 0.1, 0.7, 1 / 3, 0.015625]);
const DIRS = ['TB', 'BT', 'LR', 'RL'];
const message = (e) => String((e && e.message) || e);

// ---- prepare ----
const POOLS = [
  ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'],
  ['10', '2', '1', '30', '4', '7', '11', '100', '5', '21'],
  ['b', '10', 'a', '2', 'sg1', 'x.y', 'node-1', 'A B', '3', 'z'],
];
const ARROWS = ['arrow_point', 'none', 'arrow_circle', 'arrow_cross', 'aggregation'];

const prepareCase = (name) => {
  const n = ri(1, 9);
  const ids = shuffle(pick(POOLS)).slice(0, n);
  const parent = new Map();
  const depth = new Map();
  for (let i = 0; i < ids.length; i++) {
    depth.set(ids[i], 0);
    if (i > 0 && chance(0.55)) {
      const p = ids[ri(0, i - 1)];
      if (depth.get(p) < 3) {
        parent.set(ids[i], p);
        depth.set(ids[i], depth.get(p) + 1);
      }
    }
  }
  const groups = new Set(parent.values());
  const nodes = shuffle(ids).map((id) => {
    const node = { id, label: `label of ${id}`, domId: `my-svg-${id}`, shape: groups.has(id) ? 'rect' : pick(['rect', 'circle', 'diamond']) };
    if (parent.has(id)) {
      node.parentId = parent.get(id);
    }
    if (groups.has(id) || chance(0.08)) {
      // A group without children is possible: an empty subgraph.
      node.isGroup = true;
      node.padding = 8;
      if (chance(0.3)) {
        node.dir = pick(DIRS);
      }
    }
    if (chance(0.1)) {
      node.width = coord(10, 200);
      node.height = coord(10, 80);
    }
    return node;
  });
  const leaves = ids.filter((id) => !groups.has(id));
  const edges = [];
  const m = ri(0, Math.round(1.6 * n));
  for (let k = 0; k < m; k++) {
    const end = () => (chance(0.25) ? pick(ids) : pick(leaves.length > 0 ? leaves : ids));
    const start = end();
    const edge = { id: `L_${k}`, start, end: chance(0.2) ? start : end(), labelpos: 'c' };
    if (chance(0.4)) {
      edge.label = pick(['yes', 'a longer label', 'x']);
    }
    if (chance(0.8)) {
      edge.minlen = ri(1, 3);
    }
    if (chance(0.85)) {
      edge.arrowTypeStart = pick(ARROWS);
      edge.arrowTypeEnd = pick(ARROWS);
    }
    for (const key of ['startLabelLeft', 'startLabelRight', 'endLabelLeft', 'endLabelRight']) {
      if (chance(0.12)) {
        edge[key] = pick(['1', '0..*', 'n']);
      }
    }
    if (chance(0.05)) {
      edge.weight = ri(1, 4);
    }
    if (chance(0.06)) {
      // An edge without an id has no name in the graph.
      delete edge.id;
    }
    edges.push(edge);
  }
  const data = { type: pick(['flowchart-v2', 'classDiagram', 'requirement']), diagramId: 'my-svg', layoutAlgorithm: 'dagre', nodes, edges };
  if (chance(0.9)) {
    data.direction = pick(DIRS);
  }
  // The spacings: a number, a string, zero or nothing, on the layout data
  // and in the config.
  const spacing = () => pick([50, 50, 50, 30, 80.5, '100', '40', 0, undefined]);
  for (const key of ['nodeSpacing', 'rankSpacing']) {
    const v = spacing();
    if (v !== undefined) {
      data[key] = v;
    }
  }
  if (chance(0.9)) {
    data.config = { flowchart: {} };
    for (const key of ['nodeSpacing', 'rankSpacing']) {
      if (chance(0.25)) {
        data.config[key] = pick([15, '25', 0]);
      }
      if (chance(0.9)) {
        data.config.flowchart[key] = pick([50, 60, '70']);
      }
    }
  }
  const input = projectInput(structuredClone(data), { flowchart: {} });
  const out = { kind: 'prepare', name, input };
  clear();
  try {
    const { graph } = prepareLayoutForDagre(data);
    out.graph = projectGraph(graph, true);
  } catch (e) {
    out.error = message(e);
  }
  out.clusterDb = projectClusterDb();
  return out;
};

// ---- edges ----
// The labels of a case are JSON: an edge label has points as [x, y] pairs,
// selfLoop as [id, order] and originalEdge as another edge label.
const edgeLabel = (json) => {
  const label = { ...json };
  if (json.points) {
    label.points = json.points.map(([x, y]) => ({ x, y }));
  }
  if (json.selfLoop) {
    label.selfLoop = { id: json.selfLoop[0], order: json.selfLoop[1] };
  }
  if (json.originalEdge) {
    label.originalEdge = edgeLabel(json.originalEdge);
  }
  return label;
};

const buildGraph = (c) => {
  const g = new Graph({ multigraph: true, compound: true }).setDefaultEdgeLabel(() => ({}));
  const label = { nodesep: 50, ranksep: 50, marginx: 8, marginy: 8 };
  if (c.rankdir !== null) {
    label.rankdir = c.rankdir;
  }
  g.setGraph(label);
  for (const [id, parent, value] of c.nodes) {
    if (value === null) {
      g.setNode(id);
    } else {
      const node = structuredClone(value);
      if (node.intersect) {
        node.intersect = () => ({ x: 0, y: 0 });
      }
      g.setNode(id, node);
    }
    if (parent !== null) {
      g.setParent(id, parent);
    }
  }
  for (const [v, w, name, value] of c.edges) {
    g.setEdge(v, w, edgeLabel(value), name);
  }
  return g;
};

const points = (count) => Array.from({ length: count }, () => [coord(0, 400), coord(0, 400)]);

// selfLoopGroup adds the segments of one self-loop to a case.
const selfLoopGroup = (c, nodeId, k, opts) => {
  const node = { id: nodeId, x: coord(50, 300), y: coord(50, 300), width: opts.width ?? coord(8, 260), height: opts.height ?? coord(8, 120) };
  if (opts.isGroup) {
    node.isGroup = true;
  }
  if (!opts.nodeMissing) {
    c.nodes.push([nodeId, null, node]);
  }
  const s1 = `${nodeId}---${nodeId}---1`;
  const s2 = `${nodeId}---${nodeId}---2`;
  // Where the dummy nodes are decides the side of the loop.
  const dx = opts.dx ?? pick([-80, -30, 0, 30, 80]) + pick([0, 0.5]);
  const dy = opts.dy ?? pick([-80, -30, 0, 30, 80]) + pick([0, 0.25]);
  for (const [i, id] of [s1, s2].entries()) {
    const dummy = { id, width: 10, height: 10 };
    if (opts.hints === 'nodes' || (opts.hints === 'one' && i === 0)) {
      dummy.x = node.x + dx + (i === 0 ? -5 : 5);
      dummy.y = node.y + dy;
    }
    if (opts.hints !== 'missing') {
      c.nodes.push([id, null, dummy]);
    }
  }
  const selfId = `L_${nodeId}_${nodeId}_${k}`;
  const original = { id: selfId, start: nodeId, end: nodeId, labelpos: 'c' };
  if (chance(0.6)) {
    original.label = 'loop';
  }
  if (chance(0.8)) {
    original.arrowTypeStart = pick(ARROWS);
  }
  if (chance(0.8)) {
    original.arrowTypeEnd = pick(ARROWS);
  }
  for (const key of ['startLabelLeft', 'startLabelRight', 'endLabelLeft', 'endLabelRight']) {
    if (chance(0.2)) {
      original[key] = pick(['1', '0..*']);
    }
  }
  const segment = (order, id, v, w) => {
    const label = { ...original, id, selfLoop: [selfId, order] };
    if (opts.original === 'all' || (opts.original === 'last' && order === 2) || (opts.original === 'middle' && order === 1)) {
      label.originalEdge = { ...original };
    }
    if (opts.points !== 'none') {
      label.points = points(ri(2, 4));
    }
    return [v, w, id, label];
  };
  const first = segment(0, `${nodeId}-cyclic-special-1`, nodeId, s1);
  first[2] = `${nodeId}-cyclic-special-0`;
  first[3].label = '';
  first[3].arrowTypeEnd = 'none';
  const middle = segment(1, `${nodeId}-cyclic-special-mid`, s1, s2);
  middle[2] = `${nodeId}-cyclic-special-1`;
  middle[3].arrowTypeStart = 'none';
  middle[3].arrowTypeEnd = 'none';
  for (const key of ['startLabelLeft', 'startLabelRight', 'endLabelLeft', 'endLabelRight']) {
    delete middle[3][key];
  }
  if (opts.labelSize !== 'none') {
    middle[3].width = opts.labelSize === 'wide' ? coord(120, 300) : coord(0, 60);
    middle[3].height = coord(0, 30);
    middle[3].x = coord(0, 400);
    middle[3].y = coord(0, 400);
  }
  const last = segment(2, `${nodeId}-cyclic-special-2`, s2, nodeId);
  last[3].label = '';
  last[3].arrowTypeStart = 'none';
  if (opts.isGroup) {
    pick([first, middle])[3].fromCluster = nodeId;
    pick([middle, last])[3].toCluster = nodeId;
  }
  let segments = [first, middle, last];
  if (opts.count === 2) {
    segments = [first, last];
  } else if (opts.count === 1) {
    segments = [middle];
  } else if (opts.count === 4) {
    const extra = segment(pick([0, 1, 2, 3]), `${nodeId}-cyclic-special-extra`, s2, s1);
    segments.push(extra);
  }
  // The order of the edges in the graph is not the order of the segments.
  c.edges.push(...(opts.shuffle ? shuffle(segments) : segments));
};

const edgesCase = (name, build) => {
  const c = { kind: 'edges', name, rankdir: pick([...DIRS, null]), yOffset: pick([0, 0, 2.5, 7.5, 10]), mergeSelfLoops: true, nodes: [], edges: [] };
  build(c);
  const graph = buildGraph(c);
  try {
    c.want = getEdgesToRender(graph, c.yOffset, { mergeSelfLoops: c.mergeSelfLoops }).map(({ edge, start, end }) => [projectEdge(edge), start, end]);
  } catch (e) {
    c.error = message(e);
  }
  return c;
};

const plainEdges = (c, count) => {
  for (let i = 0; i < count; i++) {
    const a = `p${i}a`;
    const b = `p${i}b`;
    c.nodes.push([a, null, { id: a, x: coord(0, 300), y: coord(0, 300), width: 40, height: 30 }]);
    c.nodes.push([b, null, { id: b, x: coord(0, 300), y: coord(0, 300), width: 40, height: 30 }]);
    const label = { id: `L_${a}_${b}`, points: points(3) };
    if (chance(0.5)) {
      label.start = a;
      label.end = b;
    }
    c.edges.push([a, b, label.id, label]);
  }
};

// ---- apply ----
const applyCase = (name) => {
  const c = { kind: 'apply', name, rankdir: pick([...DIRS, null]), margin: pick([0, 0, 15, 7.5, null]), mergeSelfLoops: chance(0.7), nodes: [], edges: [], dataNodes: [] };
  const n = ri(1, 8);
  const ids = Array.from({ length: n }, (_, i) => pick(['', 'n']) + String(i * 3 + 1));
  const parent = new Map();
  for (let i = 1; i < n; i++) {
    if (chance(0.5)) {
      parent.set(ids[i], ids[ri(0, i - 1)]);
    }
  }
  const groups = new Set(parent.values());
  for (const id of shuffle(ids)) {
    let value = null;
    if (!chance(0.06)) {
      value = { id };
      // A node that dagre did not lay out has no position.
      if (chance(0.85)) {
        value.x = coord(0, 500);
        value.y = coord(0, 500);
      }
      if (chance(0.9)) {
        value.width = coord(10, 300);
        value.height = coord(10, 200);
      }
      if (groups.has(id)) {
        value.isGroup = true;
      } else if (chance(0.25)) {
        value.clusterNode = true;
        value.diff = pick([0, -8, 12.5]);
      }
      if (chance(0.3)) {
        value.labelBBox = { width: coord(0, 100), height: coord(0, 30) };
      }
      if (chance(0.5)) {
        value.intersect = true;
      }
    }
    c.nodes.push([id, parent.has(id) ? parent.get(id) : null, value]);
  }
  // The layout data has most nodes of the graph, and some that the graph
  // does not have.
  for (const id of ids) {
    if (chance(0.85)) {
      const node = { id };
      if (chance(0.3)) {
        node.x = 1;
        node.y = 2;
        node.width = 3;
        node.height = 4;
      }
      c.dataNodes.push(node);
    }
  }
  c.dataNodes.push({ id: 'not-in-graph', x: 5, y: 6 });
  plainEdges(c, ri(0, 3));
  if (chance(0.5)) {
    selfLoopGroup(c, 'loop', 0, { hints: 'nodes', original: 'all', count: 3, shuffle: true });
  }
  const graph = buildGraph(c);
  const data = { nodes: structuredClone(c.dataNodes), edges: [{ id: 'old' }] };
  const measured = { graph, mergeSelfLoops: c.mergeSelfLoops };
  if (c.margin !== null) {
    measured.subGraphTitleTotalMargin = c.margin;
  }
  const full = (node) => [...pos(node), node.labelBBox ? [dyn(node.labelBBox.width), dyn(node.labelBBox.height)] : null, typeof node.intersect === 'function'];
  try {
    applyDagreLayoutResult(data, measured);
    c.want = {
      dataNodes: data.nodes.map(full),
      graphNodes: graph.nodes().map((v) => [v, graph.node(v) ? full(graph.node(v)) : null]),
      edges: data.edges.map(projectEdge),
    };
  } catch (e) {
    c.error = message(e);
  }
  return c;
};

// ---- the cases ----
const cases = [];
for (let i = 0; i < 300; i++) {
  cases.push(prepareCase(`prepare-${i}`));
}

// One self-loop for every rankdir and every way to find its side.
let k = 0;
for (const rankdir of [...DIRS, null]) {
  for (const hints of ['nodes', 'one', 'points', 'none', 'missing']) {
    for (const original of ['all', 'last', 'middle', 'none']) {
      const c = edgesCase(`self-loop-${rankdir}-${hints}-${original}`, (c) => {
        selfLoopGroup(c, 'A', k++, { hints, original, count: 3, shuffle: chance(0.5), points: hints === 'none' || hints === 'missing' ? 'none' : 'some', labelSize: pick(['none', 'small', 'wide']) });
      });
      c.rankdir = rankdir;
      // The case was computed with a random rankdir: compute it again.
      const graph = buildGraph(c);
      c.want = getEdgesToRender(graph, c.yOffset, { mergeSelfLoops: true }).map(({ edge, start, end }) => [projectEdge(edge), start, end]);
      cases.push(c);
    }
  }
}
// The dummy nodes right, left, below and above the node, and on the
// diagonals, where the two distances are equal.
for (const [dx, dy] of [
  [40, 0],
  [-40, 0],
  [0, 40],
  [0, -40],
  [40, 40],
  [-40, 40],
  [40, -40],
  [-40, -40],
  [0, 0],
  [40, 39.5],
  [39.5, 40],
  [-0.25, 0],
  [0, 0.25],
]) {
  for (const rankdir of ['TB', 'LR', 'BT', 'RL']) {
    const c = edgesCase(`self-loop-side-${dx}-${dy}-${rankdir}`, (c) => {
      // The dummy nodes are placed at -5 and +5 of dx, so their center is dx.
      selfLoopGroup(c, 'A', k++, { hints: 'nodes', original: 'all', count: 3, dx, dy, labelSize: 'small' });
    });
    c.rankdir = rankdir;
    const graph = buildGraph(c);
    c.want = getEdgesToRender(graph, c.yOffset, { mergeSelfLoops: true }).map(({ edge, start, end }) => [projectEdge(edge), start, end]);
    cases.push(c);
  }
}
// The sizes of the node and of the label: the clamps of the loop.
for (const width of [4, 20, 45, 46, 60, 102.5, 125, 126, 300]) {
  for (const height of [4, 53, 54, 106, 107, 200]) {
    cases.push(
      edgesCase(`self-loop-size-${width}-${height}`, (c) => {
        selfLoopGroup(c, 'A', k++, { hints: 'nodes', original: 'all', count: 3, width, height, labelSize: pick(['none', 'small', 'wide']) });
      })
    );
  }
}
// Groups that are not three segments, a node that is missing, groups on
// clusters, several groups, and plain edges between them.
for (let i = 0; i < 160; i++) {
  cases.push(
    edgesCase(`self-loop-mix-${i}`, (c) => {
      c.mergeSelfLoops = !chance(0.15);
      plainEdges(c, ri(0, 2));
      const groups = ri(1, 3);
      for (let g = 0; g < groups; g++) {
        selfLoopGroup(c, `N${g}`, k++, {
          hints: pick(['nodes', 'nodes', 'one', 'points', 'none', 'missing']),
          original: pick(['all', 'all', 'last', 'middle', 'none']),
          count: pick([3, 3, 3, 1, 2, 4]),
          shuffle: chance(0.6),
          isGroup: chance(0.3),
          nodeMissing: chance(0.08),
          points: pick(['some', 'some', 'none']),
          labelSize: pick(['none', 'small', 'wide']),
        });
        if (chance(0.5)) {
          plainEdges(c, 1);
          // plainEdges names its nodes by index: keep the ids unique.
          const [a, b] = c.nodes.splice(-2);
          const edge = c.edges.pop();
          a[0] = a[2].id = `q${g}a`;
          b[0] = b[2].id = `q${g}b`;
          c.nodes.push(a, b);
          c.edges.push([a[0], b[0], `L_q${g}`, { ...edge[3], id: `L_q${g}`, ...(edge[3].start ? { start: a[0], end: b[0] } : {}) }]);
        }
      }
    })
  );
}
for (let i = 0; i < 200; i++) {
  cases.push(applyCase(`apply-${i}`));
}

const count = (kind) => cases.filter((c) => c.kind === kind).length;
const json = JSON.stringify({ generator: 'tools/reference/gen-dagre-index.mjs', mermaid: '12.2.0', cases });
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(json, { level: 9 }));
console.error(
  `${cases.length} cases: ${count('prepare')} prepare, ${count('edges')} edges, ${count('apply')} apply, ${cases.filter((c) => c.error).length} where upstream throws -> ${path.relative(root, outFile)} (${fs.statSync(outFile).size} bytes)`
);
const errors = new Map();
for (const c of cases.filter((c) => c.error)) {
  errors.set(`${c.kind}: ${c.error}`, (errors.get(`${c.kind}: ${c.error}`) || 0) + 1);
}
for (const [text, n] of errors) {
  console.error(`  ${n} x ${text}`);
}
