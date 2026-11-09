// Emits golden cases for internal/elk.
//
// Each case is an ELK JSON graph and the result of elk.layout(graph) from the
// genuine elkjs 0.9.3 in node (V8). The Go test runs the same engine in goja
// and must reproduce every output byte for byte.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk.mjs            write internal/elk/testdata/golden.jsonl.gz
//   node gen-elk.mjs --bench    time elkjs in V8 on the benchmark graphs
//   node gen-elk.mjs --math     write internal/elk/testdata/math.json.gz
//   node gen-elk.mjs --dump N   print the input of the case with the name N
//   node gen-elk.mjs --times    print the time of a warm layout of each case
//
// A golden output is the first layout of a fresh elkjs module in a fresh V8
// isolate (a worker thread), because elkjs keeps a global hash code counter
// and writes its values into the graph as "$H" members. The script also checks
// that the layout itself does not depend on the layouts that ran before it.
// A layout that does not end within HANG_MS is recorded as a hang: elkjs has
// option combinations that loop forever.
import { createRequire } from 'node:module';
import { gzipSync } from 'node:zlib';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Worker, isMainThread, parentPort, workerData } from 'node:worker_threads';
import * as fdlibm from './elk-fdlibm.mjs';

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, '../../internal/elk/testdata');
const ELK_PATH = require.resolve('elkjs/lib/elk.bundled.js');

// ---------------------------------------------------------------------------
// Deterministic environment. The Go bridge installs the same clock, the same
// Math.random and the same Math functions in its runtimes.
//
// Math: elkjs calls Math.sin, cos, acos, atan2, log and pow. Their native
// results depend on the build of V8 (see elk-fdlibm.mjs), so the script
// installs the fdlibm functions in JavaScript.
//
// Clock: java.util.Random without a seed (the option elk.randomSeed = 0) uses
// uniqueSeed++ + Date.now(), where uniqueSeed counts the Random objects of the
// module. Date.now returns FIXED_NOW minus the number of calls before the
// current layout, so the seed is FIXED_NOW + k for the k-th Random of a
// layout, in a fresh module and in a used one.

const FIXED_NOW = 1700000000000;
const RANDOM_SEED = 0x9e3779b9;
const native = { random: Math.random, now: Date.now };
for (const name of Object.keys(fdlibm.functions)) native[name] = Math[name];
let randomState = RANDOM_SEED;
let randomCalls = 0;
let nowCalls = 0;
let nowBase = 0;
function bridgeRandom() {
  randomCalls++;
  randomState = (randomState + 0x6d2b79f5) >>> 0;
  let t = randomState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
function installEnvironment() {
  Math.random = bridgeRandom;
  Date.now = () => {
    nowCalls++;
    return FIXED_NOW - nowBase;
  };
  for (const [name, fn] of Object.entries(fdlibm.functions)) Math[name] = fn;
}
function restoreEnvironment() {
  Math.random = native.random;
  Date.now = native.now;
  for (const name of Object.keys(fdlibm.functions)) Math[name] = native[name];
}
// beginLayout resets the state that the bridge resets before each layout.
function beginLayout() {
  randomState = RANDOM_SEED;
  randomCalls = 0;
  nowBase = nowCalls;
}

function freshELK() {
  delete require.cache[ELK_PATH];
  nowCalls = 0;
  return require(ELK_PATH);
}

// ---------------------------------------------------------------------------
// Seeded PRNG for graph generation (mulberry32).

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
    pick: (arr) => arr[Math.floor(next() * arr.length)],
    chance: (p) => next() < p,
    shuffle: (arr) => {
      for (let i = arr.length - 1; i > 0; i--) {
        const j = Math.floor(next() * (i + 1));
        [arr[i], arr[j]] = [arr[j], arr[i]];
      }
      return arr;
    },
  };
  return R;
}

// Sizes as mermaid measures them: integers, halves, 1/64 steps and doubles
// with a full mantissa.
function size(R, lo, hi) {
  const v = lo + R.next() * (hi - lo);
  switch (R.int(0, 9)) {
    case 0:
    case 1:
    case 2:
    case 3:
      return Math.round(v);
    case 4:
      return Math.round(v * 2) / 2;
    case 5:
    case 6:
    case 7:
      return Math.round(v * 64) / 64;
    default:
      return v;
  }
}

const WORDS = ['Start', 'Stop', 'Check', 'Yes', 'No', 'Load data', 'Is it?', 'Retry', 'Done', 'Fail', 'α β', 'Größe', '日本語', 'a "q" b', 'x\\y', 'A & B < C'];

// ---------------------------------------------------------------------------
// The part of mermaid's ELK adapter that builds the graph. Ported from
// rendering-util/layout-algorithms/elk/render.ts (mermaid 12.2.0).

const PORTS_SURROUNDING = '[top=12,left=12,bottom=12,right=12]';
const SUBGRAPH_PADDING = 24;
const DEFAULT_SUBGRAPH_SPACING_BASE_VALUE = 24;
const DEFAULT_SUBGRAPH_NODE_SPACING = 50;
const CONTAINER_PADDING = 15;
const RECTPACKING_CONTAINER_PADDING = 10;
const RECTPACKING_OPTIONS = {
  'spacing.baseValue': 15,
  'spacing.nodeNode': 15,
  'elk.aspectRatio': '1.6',
  'elk.expandNodes': 'true',
  'elk.rectpacking.trybox': 'true',
  'elk.rectpacking.packing.compaction.rowHeightReevaluation': 'true',
  'elk.rectpacking.packing.compaction.iterations': 10,
  'elk.rectpacking.whiteSpaceElimination.strategy': 'EQUAL_BETWEEN_STRUCTURES',
  'elk.rectpacking.widthApproximation.strategy': 'SCANLINE',
};
const CONTAINER_ALGORITHM_OVERRIDES = [
  'nodeSize.constraints',
  'nodeSize.minimum',
  'elk.algorithm',
  'elk.aspectRatio',
  'elk.contentAlignment',
  'elk.expandNodes',
  'elk.padding',
  ...Object.keys(RECTPACKING_OPTIONS),
];
const CONTAINER_ALGORITHMS = ['elk.layered', 'elk.box', 'elk.rectpacking', 'elk.stress', 'elk.force', 'elk.mrtree', 'elk.radial', 'elk.sporeOverlap'];
const ELK_PRESETS = {
  default: { layering: 'NETWORK_SIMPLEX', placement: 'BRANDES_KOEPF', containerPlacement: 'BRANDES_KOEPF', alignment: 'BALANCED', cycleBreaking: 'DEPTH_FIRST' },
  legacy: { layering: 'NETWORK_SIMPLEX', placement: 'BRANDES_KOEPF', containerPlacement: 'BRANDES_KOEPF', alignment: 'NONE', cycleBreaking: 'GREEDY' },
  modelOrder: { layering: 'NETWORK_SIMPLEX', placement: 'NETWORK_SIMPLEX', containerPlacement: 'BRANDES_KOEPF', alignment: 'NONE', cycleBreaking: 'GREEDY_MODEL_ORDER' },
  depthFirst: { layering: 'NETWORK_SIMPLEX', placement: 'NETWORK_SIMPLEX', containerPlacement: 'BRANDES_KOEPF', alignment: 'NONE', cycleBreaking: 'DEPTH_FIRST' },
};
const resolveElkPreset = (name) => (name !== undefined && Object.hasOwn(ELK_PRESETS, name) ? ELK_PRESETS[name] : ELK_PRESETS.default);

function dir2ElkDirection(dir) {
  switch (dir) {
    case 'LR':
      return 'RIGHT';
    case 'RL':
      return 'LEFT';
    case 'TB':
    case 'TD':
      return 'DOWN';
    case 'BT':
      return 'UP';
    default:
      return 'DOWN';
  }
}

const clusterPaintsTitle = (shape) => shape !== 'noteGroup';

function groupTitleSizeOptions(node) {
  if (!clusterPaintsTitle(node.shape)) {
    return {};
  }
  const width = (node.labelData?.width ?? node.labels?.[0]?.width ?? 0) + (node.padding ?? 0);
  return { 'nodeSize.constraints': '[MINIMUM_SIZE, NODE_LABELS]', 'nodeSize.minimum': `(${width}, 0)` };
}

function clearContainerAlgorithmOptions(layoutOptions) {
  for (const key of CONTAINER_ALGORITHM_OVERRIDES) {
    delete layoutOptions[key];
  }
  layoutOptions['spacing.baseValue'] = DEFAULT_SUBGRAPH_SPACING_BASE_VALUE;
  layoutOptions['spacing.nodeNode'] = DEFAULT_SUBGRAPH_NODE_SPACING;
}

const resolveContainerAlgorithm = (requested) => (typeof requested === 'string' && CONTAINER_ALGORITHMS.includes(requested) ? requested : undefined);

function buildSubgraphLayoutOptions(node, elkConfig, algorithm) {
  const labelW = node.labelData?.width ?? 0;
  const pad = node.padding ?? 0;
  const minWidth = labelW + 2 * pad;
  const labelH = node.labelData?.height ?? 0;
  const preset = resolveElkPreset(elkConfig?.preset);
  const layoutOptions = {
    ...groupTitleSizeOptions(node),
    'spacing.baseValue': DEFAULT_SUBGRAPH_SPACING_BASE_VALUE,
    'elk.layered.spacing.edgeNodeBetweenLayers': 30,
    'elk.spacing.edgeEdge': 20,
    'spacing.nodeNode': DEFAULT_SUBGRAPH_NODE_SPACING,
    'elk.padding': `[top=${SUBGRAPH_PADDING},left=${SUBGRAPH_PADDING},bottom=${SUBGRAPH_PADDING},right=${SUBGRAPH_PADDING}]`,
    'nodeLabels.placement': '[H_CENTER V_TOP, INSIDE]',
    'elk.layered.mergeEdges': elkConfig?.mergeEdges,
    'elk.layered.nodePlacement.bk.fixedAlignment': elkConfig?.nodePlacementAlignment ?? preset.alignment,
    'elk.layered.nodePlacement.strategy': elkConfig?.nodePlacementStrategy ?? preset.containerPlacement,
    'elk.layered.cycleBreaking.strategy': elkConfig?.cycleBreakingStrategy ?? preset.cycleBreaking,
    'elk.layered.nodePlacement.networkSimplex.nodeFlexibility': 'PORT_POSITION',
    'elk.spacing.portsSurrounding': PORTS_SURROUNDING,
  };
  const algo = resolveContainerAlgorithm(node.metadata?.algorithm);
  if (algo) {
    const padTop = labelH + CONTAINER_PADDING;
    layoutOptions['nodeSize.constraints'] = '[MINIMUM_SIZE, NODE_LABELS]';
    layoutOptions['nodeSize.minimum'] = `(${minWidth}, ${padTop + CONTAINER_PADDING})`;
    layoutOptions['elk.algorithm'] = algo;
    layoutOptions['elk.hierarchyHandling'] = 'SEPARATE_CHILDREN';
    layoutOptions['elk.aspectRatio'] = '2.0';
    layoutOptions['elk.contentAlignment'] = 'H_CENTER V_TOP';
    layoutOptions['elk.expandNodes'] = 'true';
    layoutOptions['elk.padding'] = `[top=${padTop},left=${CONTAINER_PADDING},bottom=${CONTAINER_PADDING},right=${CONTAINER_PADDING}]`;
    if (algo === 'elk.rectpacking') {
      const rectPadTop = labelH + RECTPACKING_CONTAINER_PADDING;
      Object.assign(layoutOptions, RECTPACKING_OPTIONS, {
        'elk.padding': `[top=${rectPadTop},left=${RECTPACKING_CONTAINER_PADDING},bottom=${RECTPACKING_CONTAINER_PADDING},right=${RECTPACKING_CONTAINER_PADDING}]`,
        'nodeSize.minimum': `(${minWidth}, ${rectPadTop + RECTPACKING_CONTAINER_PADDING})`,
      });
    }
  } else if (node.dir) {
    layoutOptions['elk.algorithm'] = algorithm;
    layoutOptions['elk.direction'] = dir2ElkDirection(node.dir);
    layoutOptions['elk.hierarchyHandling'] = 'SEPARATE_CHILDREN';
  }
  return layoutOptions;
}

function createRootElkGraph(config, algorithm, rootLayoutOptions) {
  const preset = resolveElkPreset(config.elk?.preset);
  const graph = {
    id: 'root',
    layoutOptions: {
      'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
      'elk.algorithm': algorithm,
      'elk.layered.nodePlacement.strategy': config.elk?.nodePlacementStrategy ?? preset.placement,
      'elk.layered.nodePlacement.bk.fixedAlignment': config.elk?.nodePlacementAlignment ?? preset.alignment,
      'elk.layered.mergeEdges': config.elk?.mergeEdges,
      'elk.direction': 'DOWN',
      'spacing.baseValue': 40,
      'elk.layered.crossingMinimization.forceNodeModelOrder': config.elk?.forceNodeModelOrder,
      'elk.layered.considerModelOrder.strategy': config.elk?.considerModelOrder,
      'elk.layered.unnecessaryBendpoints': true,
      'elk.layered.cycleBreaking.strategy': config.elk?.cycleBreakingStrategy ?? preset.cycleBreaking,
      'elk.layered.layering.strategy': config.elk?.layeringStrategy ?? preset.layering,
      'elk.layered.layering.coffmanGraham.layerBound': config.elk?.layeringLayerBound,
      'elk.layered.wrapping.multiEdge.improveCuts': true,
      'elk.layered.wrapping.multiEdge.improveWrappedEdges': true,
      'elk.layered.edgeRouting.selfLoopDistribution': 'EQUALLY',
      'elk.layered.mergeHierarchyEdges': true,
      'elk.spacing.portsSurrounding': PORTS_SURROUNDING,
    },
    children: [],
    edges: [],
  };
  if (algorithm === 'elk.rectpacking') {
    Object.assign(graph.layoutOptions, RECTPACKING_OPTIONS, {
      'elk.contentAlignment': 'H_CENTER V_TOP',
      'elk.padding': '[top=15,left=15,bottom=15,right=15]',
    });
  }
  if (rootLayoutOptions) {
    Object.assign(graph.layoutOptions, rootLayoutOptions);
  }
  return graph;
}

function findCommonAncestor(id1, id2, { parentById }) {
  const visited = new Set();
  let currentId = id1;
  if (id1 === id2) {
    return parentById[id1] || 'root';
  }
  while (currentId) {
    visited.add(currentId);
    if (currentId === id2) {
      return currentId;
    }
    currentId = parentById[currentId];
  }
  currentId = id2;
  while (currentId) {
    if (visited.has(currentId)) {
      return currentId;
    }
    currentId = parentById[currentId];
  }
  return 'root';
}

function terminalElkLabels(edge) {
  const texts = { startLeft: edge.startLabelLeft, startRight: edge.startLabelRight, endLeft: edge.endLabelLeft, endRight: edge.endLabelRight };
  return Object.entries(edge.terminalLabelSizes ?? {}).map(([key, sz]) => {
    const atStart = key.startsWith('start');
    const marker = atStart ? edge.arrowTypeStart : edge.arrowTypeEnd;
    const pad = marker && marker !== 'none' ? 6 : 0;
    return {
      terminal: key,
      text: texts[key],
      width: sz.width + 2 * pad,
      height: sz.height + 2 * pad,
      layoutOptions: { 'edgeLabels.placement': atStart ? 'TAIL' : 'HEAD' },
    };
  });
}

// buildMermaidGraph is buildElkGraphFromLayoutData. data is mermaid's
// LayoutData: { nodes, edges, direction, config }.
function buildMermaidGraph(data, algorithm, rootLayoutOptions) {
  const nodeDb = {};
  const elkGraph = createRootElkGraph(data.config, algorithm, rootLayoutOptions);
  elkGraph.layoutOptions['elk.direction'] = dir2ElkDirection(data.direction ?? 'DOWN');

  const parentLookupDb = { parentById: {}, childrenById: {} };
  for (const subgraph of data.nodes.filter((n) => n.isGroup)) {
    for (const node of data.nodes.filter((n) => n.parentId === subgraph.id)) {
      parentLookupDb.parentById[node.id] = subgraph.id;
      parentLookupDb.childrenById[subgraph.id] ??= [];
      parentLookupDb.childrenById[subgraph.id].push(node.id);
    }
  }

  const addVertices = (graph, parentId) => {
    for (const node of data.nodes.filter((n) => n?.parentId === parentId)) {
      const child = { ...node };
      if (node.isGroup) {
        child.children = [];
      } else {
        child.width = node.width ?? 0;
        child.height = node.height ?? 0;
        if (node.spreadPorts) {
          child.layoutOptions = { ...child.layoutOptions, 'elk.portAlignment.default': 'CENTER' };
        }
      }
      graph.children.push(child);
      nodeDb[node.id] = child;
      if (node.isGroup) {
        addVertices(child, node.id);
        child.labelData = node.labelBBox
          ? { width: node.labelBBox.width, height: Math.max(0, node.labelBBox.height - 2), wrappingWidth: 200 }
          : { width: 0, height: 0, wrappingWidth: 200 };
      }
    }
  };
  addVertices(elkGraph, undefined);

  for (const edge of data.edges) {
    const startNode = edge.start ? nodeDb[edge.start] : undefined;
    const endNode = edge.end ? nodeDb[edge.end] : undefined;
    const ids = startNode && endNode ? { sourceId: edge.start, targetId: edge.end } : {};
    elkGraph.edges.push({
      ...edge,
      sources: [edge.start],
      targets: [edge.end],
      ...ids,
      labels: [
        {
          width: edge.width ?? 0,
          height: edge.height ?? 0,
          orgWidth: edge.width ?? 0,
          orgHeight: edge.height ?? 0,
          text: edge.label ?? '',
          layoutOptions: { 'edgeLabels.inline': 'true', 'edgeLabels.placement': 'CENTER' },
        },
        ...((rootLayoutOptions?.['elk.algorithm'] ?? algorithm ?? 'elk.layered') === 'elk.layered' ? terminalElkLabels(edge) : []),
      ],
    });
  }

  for (const n of data.nodes) {
    const node = nodeDb[n.id];
    if (!node || parentLookupDb.childrenById[node.id] === undefined) {
      continue;
    }
    node.labels = [{ text: node.label, width: node?.labelData?.width ?? 50, height: node?.labelData?.height ?? 50 }];
    node.layoutOptions = buildSubgraphLayoutOptions(node, data.config.elk, algorithm);
    delete node.x;
    delete node.y;
    delete node.width;
    delete node.height;
  }

  const setIncludeChildrenPolicy = (nodeId, ancestorId) => {
    const node = nodeDb[nodeId];
    if (!node) {
      return;
    }
    node.layoutOptions ??= {};
    if (node.layoutOptions['elk.hierarchyHandling'] === 'SEPARATE_CHILDREN' && resolveContainerAlgorithm(node.metadata?.algorithm)) {
      clearContainerAlgorithmOptions(node.layoutOptions);
      Object.assign(node.layoutOptions, groupTitleSizeOptions(node));
    }
    node.layoutOptions['elk.hierarchyHandling'] = 'INCLUDE_CHILDREN';
    if (node.id !== ancestorId && node.parentId) {
      setIncludeChildrenPolicy(node.parentId, ancestorId);
    }
  };
  for (const edge of elkGraph.edges) {
    const source = edge.sources[0];
    const target = edge.targets[0];
    if (nodeDb[source] && nodeDb[target] && nodeDb[source].parentId !== nodeDb[target].parentId) {
      const ancestorId = findCommonAncestor(source, target, parentLookupDb);
      setIncludeChildrenPolicy(source, ancestorId);
      setIncludeChildrenPolicy(target, ancestorId);
    }
  }

  for (const edge of elkGraph.edges) {
    if (edge.reverseForLayout) {
      [edge.sources, edge.targets] = [edge.targets, edge.sources];
      edge.layoutReversed = true;
      delete edge.reverseForLayout;
    }
  }
  for (const n of data.nodes) {
    if (n.entryOnTop && nodeDb[n.id]) {
      nodeDb[n.id].layoutOptions = { ...nodeDb[n.id].layoutOptions, 'elk.layered.layering.layerConstraint': 'FIRST' };
      delete nodeDb[n.id].entryOnTop;
    }
  }
  return elkGraph;
}

// ---------------------------------------------------------------------------
// Random mermaid LayoutData.

const ELK_CONFIG_CHOICES = {
  mergeEdges: [true, false],
  nodePlacementStrategy: ['SIMPLE', 'NETWORK_SIMPLEX', 'LINEAR_SEGMENTS', 'BRANDES_KOEPF'],
  nodePlacementAlignment: ['NONE', 'LEFTUP', 'LEFTDOWN', 'RIGHTUP', 'RIGHTDOWN', 'BALANCED'],
  preset: ['default', 'legacy', 'modelOrder', 'depthFirst'],
  // STRETCH_WIDTH is left out: elkjs does not terminate for many graphs with
  // it. Three cases below use it.
  layeringStrategy: ['NETWORK_SIMPLEX', 'LONGEST_PATH', 'LONGEST_PATH_SOURCE', 'COFFMAN_GRAHAM', 'MIN_WIDTH'],
  layeringLayerBound: [2, 3, 4],
  cycleBreakingStrategy: ['GREEDY', 'DEPTH_FIRST', 'MODEL_ORDER', 'GREEDY_MODEL_ORDER'],
  forceNodeModelOrder: [true, false],
  considerModelOrder: ['NONE', 'NODES_AND_EDGES', 'PREFER_EDGES', 'PREFER_NODES'],
};

function randomElkConfig(R, level) {
  // The schema default of considerModelOrder is NODES_AND_EDGES, and mermaid
  // passes it on, so it is nearly always set.
  const elk = { mergeEdges: false, forceNodeModelOrder: false, considerModelOrder: 'NODES_AND_EDGES' };
  if (level === 0) {
    return elk;
  }
  for (const [key, values] of Object.entries(ELK_CONFIG_CHOICES)) {
    if (R.chance(level === 1 ? 0.2 : 0.5)) {
      elk[key] = R.pick(values);
    }
  }
  if (R.chance(0.05)) {
    delete elk.considerModelOrder;
  }
  return elk;
}

// topology returns pairs [i, j] of leaf indexes.
function topology(R, n, kind) {
  const edges = [];
  const add = (a, b) => edges.push([a, b]);
  switch (kind) {
    case 'chain':
      for (let i = 0; i + 1 < n; i++) add(i, i + 1);
      break;
    case 'tree':
      for (let i = 1; i < n; i++) add(R.int(Math.max(0, i - 4), i - 1), i);
      break;
    case 'star':
      for (let i = 1; i < n; i++) (R.chance(0.5) ? add(0, i) : add(i, 0));
      break;
    case 'bipartite': {
      const half = Math.max(1, Math.floor(n / 2));
      for (let i = 0; i < half; i++) for (let j = half; j < n; j++) if (R.chance(0.5)) add(i, j);
      break;
    }
    case 'dag': {
      const m = Math.round(n * (1 + R.next() * 0.8));
      for (let k = 0; k < m && n > 1; k++) {
        const a = R.int(0, n - 2);
        add(a, R.int(a + 1, Math.min(n - 1, a + 6)));
      }
      break;
    }
    case 'cyclic': {
      for (let i = 0; i + 1 < n; i++) if (R.chance(0.8)) add(i, i + 1);
      const m = Math.max(1, Math.round(n * 0.4));
      for (let k = 0; k < m && n > 1; k++) {
        const a = R.int(1, n - 1);
        add(a, R.int(0, a - 1));
      }
      break;
    }
    case 'components': {
      let i = 0;
      while (i < n) {
        const len = R.int(1, 5);
        for (let k = i; k + 1 < Math.min(n, i + len); k++) add(k, k + 1);
        if (len > 2 && R.chance(0.4) && i + len <= n) add(i + len - 1, i);
        i += len;
      }
      break;
    }
    case 'dense':
      for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) if (i !== j && R.chance(Math.min(0.5, 3 / n))) add(i, j);
      break;
    case 'none':
      break;
    default: {
      const m = Math.round(n * (0.6 + R.next()));
      for (let k = 0; k < m && n > 0; k++) add(R.int(0, n - 1), R.int(0, n - 1));
    }
  }
  return edges;
}

const TOPOLOGIES = ['chain', 'tree', 'star', 'bipartite', 'dag', 'dag', 'cyclic', 'cyclic', 'components', 'dense', 'random', 'random'];
// Large graphs stay sparse, so that one layout does not take seconds.
const SPARSE_TOPOLOGIES = ['chain', 'tree', 'tree', 'dag', 'cyclic', 'components'];

// randomLayoutData builds mermaid LayoutData with n leaf nodes.
function randomLayoutData(R, o) {
  const n = o.n;
  // The time of the layered algorithm grows fast with the number of
  // subgraphs and of edges that cross their borders. Large graphs have few
  // subgraphs, so that one layout does not take seconds.
  const large = n > 30;
  const rich = o.rich ?? (!large && R.chance(0.2));
  const nodes = [];
  const groups = [];
  const nGroups = o.groups ?? (n < 3 ? 0 : R.chance(0.45) ? 0 : R.int(1, large ? Math.min(5, Math.round(n / 15)) : Math.max(1, Math.round(n / 4))));
  const maxDepth = o.maxDepth ?? (large ? R.int(1, 2) : R.int(1, 6));
  for (let g = 0; g < nGroups; g++) {
    const id = `sg${g}`;
    let parentId;
    let depth = 1;
    if (groups.length > 0 && R.chance(o.nest ?? 0.45)) {
      const p = R.pick(groups);
      if (p.depth < maxDepth) {
        parentId = p.id;
        depth = p.depth + 1;
      }
    }
    const node = { id, label: R.pick(WORDS), parentId, isGroup: true };
    if (rich) {
      Object.assign(node, { shape: 'rect', padding: 8, look: 'classic', cssClasses: '', labelStyle: '' });
    } else if (R.chance(0.5)) {
      node.padding = R.pick([8, 10, 15]);
    }
    if (R.chance(0.85)) {
      node.labelBBox = { width: size(R, 20, 160), height: R.pick([21, 24, 26, 42]) };
    }
    if (o.dirs && R.chance(0.35)) {
      node.dir = R.pick(['TB', 'LR', 'BT', 'RL']);
    }
    if (o.containerAlgorithms && R.chance(0.6)) {
      node.metadata = { algorithm: R.pick(o.containerAlgorithms) };
    }
    if (R.chance(0.03)) {
      node.shape = 'noteGroup';
    }
    groups.push({ id, depth, node });
    nodes.push(node);
  }
  const leaves = [];
  for (let i = 0; i < n; i++) {
    const id = o.ids ? o.ids(i) : `n${i}`;
    const parentId = groups.length > 0 && R.chance(o.inGroup ?? 0.6) ? R.pick(groups).id : undefined;
    const node = { id };
    if (rich) {
      Object.assign(node, { label: R.pick(WORDS), parentId, isGroup: false, shape: R.pick(['rect', 'diamond', 'circle', 'rounded']), padding: 15, look: 'classic', cssClasses: 'default' });
    } else {
      node.parentId = parentId;
    }
    node.width = size(R, 30, 240);
    node.height = R.chance(0.6) ? R.pick([45, 54, 39, 78]) : size(R, 24, 130);
    if (o.spreadPorts && R.chance(0.2)) {
      node.spreadPorts = true;
    }
    if (o.entryOnTop && R.chance(0.08)) {
      node.entryOnTop = true;
    }
    leaves.push(node);
  }
  // Interleave leaves and groups, as a diagram source does.
  const all = R.chance(0.5) ? [...nodes, ...leaves] : R.shuffle([...nodes, ...leaves]);
  // An empty group with a size.
  for (const g of groups) {
    if (!all.some((x) => x.parentId === g.id) && R.chance(0.7)) {
      g.node.width = size(R, 60, 200);
      g.node.height = size(R, 40, 100);
    }
  }

  const edges = [];
  const kind = o.topology ?? R.pick(large ? SPARSE_TOPOLOGIES : TOPOLOGIES);
  const pairs = topology(R, n, kind);
  const endpoints = (i) => leaves[i].id;
  let k = 0;
  const addEdge = (start, end) => {
    const edge = { id: `L_${start}_${end}_${k++}` };
    if (rich) {
      Object.assign(edge, { start, end, type: 'arrow_point', arrowTypeStart: 'none', arrowTypeEnd: 'arrow_point', thickness: 'normal', curve: 'basis' });
    } else {
      edge.start = start;
      edge.end = end;
    }
    if (R.chance(o.labels ?? (large ? 0.15 : 0.35))) {
      edge.label = R.pick(WORDS);
      edge.width = size(R, 14, 130);
      edge.height = R.pick([21, 24, 42, 63]);
    }
    if (o.terminalLabels && R.chance(0.4)) {
      const sizes = {};
      for (const key of ['startLeft', 'startRight', 'endLeft', 'endRight']) {
        if (R.chance(0.4)) {
          sizes[key] = { width: size(R, 8, 40), height: R.pick([18, 21]) };
          edge[{ startLeft: 'startLabelLeft', startRight: 'startLabelRight', endLeft: 'endLabelLeft', endRight: 'endLabelRight' }[key]] = R.pick(['1', '0..*', '1..n', 'many']);
        }
      }
      edge.terminalLabelSizes = sizes;
      edge.arrowTypeEnd = R.pick(['none', 'aggregation', 'extension']);
    }
    if (o.reversed && R.chance(0.06)) {
      edge.reverseForLayout = true;
    }
    edges.push(edge);
  };
  for (const [a, b] of pairs) {
    if (a === b && !(o.selfLoops ?? true)) {
      continue;
    }
    addEdge(endpoints(a), endpoints(b));
  }
  if ((o.selfLoops ?? true) && n > 0 && R.chance(0.2)) {
    const c = R.int(1, 3);
    for (let i = 0; i < c; i++) {
      const a = R.int(0, n - 1);
      addEdge(endpoints(a), endpoints(a));
    }
  }
  if (n > 1 && R.chance(0.15)) {
    // Parallel edges.
    const [a, b] = [R.int(0, n - 1), R.int(0, n - 1)];
    addEdge(endpoints(a), endpoints(b));
    addEdge(endpoints(a), endpoints(b));
  }
  if (groups.length > 0 && (o.groupEdges ?? !large) && R.chance(0.35)) {
    const c = R.int(1, 3);
    for (let i = 0; i < c; i++) {
      const g = R.pick(groups).id;
      const other = R.chance(0.3) && groups.length > 1 ? R.pick(groups).id : n > 0 ? endpoints(R.int(0, n - 1)) : g;
      if (other !== g) {
        R.chance(0.5) ? addEdge(g, other) : addEdge(other, g);
      }
    }
  }
  return {
    nodes: all,
    edges,
    direction: o.direction ?? R.pick(['TB', 'TD', 'LR', 'RL', 'BT', undefined]),
    config: { elk: o.elk ?? randomElkConfig(R, o.configLevel ?? 1) },
  };
}

// ---------------------------------------------------------------------------
// ELK options that mermaid can pass through rootLayoutOptions, from
// elkOptionCatalogue.ts.

const CATALOGUE = {
  'elk.layered.layering.strategy': ['COFFMAN_GRAHAM', 'NETWORK_SIMPLEX', 'LONGEST_PATH', 'LONGEST_PATH_SOURCE', 'MIN_WIDTH', 'INTERACTIVE'],
  'elk.layered.layering.coffmanGraham.layerBound': [2, 4],
  'elk.layered.layering.nodePromotion.strategy': ['NONE', 'NIKOLOV', 'NIKOLOV_PIXEL', 'NIKOLOV_IMPROVED', 'NIKOLOV_IMPROVED_PIXEL', 'DUMMYNODE_PERCENTAGE', 'NODECOUNT_PERCENTAGE', 'NO_BOUNDARY'],
  'elk.layered.crossingMinimization.strategy': ['LAYER_SWEEP', 'INTERACTIVE', 'NONE'],
  'elk.layered.crossingMinimization.greedySwitch.type': ['TWO_SIDED', 'ONE_SIDED', 'OFF'],
  'elk.layered.considerModelOrder.strategy': ['NODES_AND_EDGES', 'NONE', 'PREFER_EDGES', 'PREFER_NODES'],
  'elk.layered.nodePlacement.strategy': ['NETWORK_SIMPLEX', 'BRANDES_KOEPF', 'LINEAR_SEGMENTS', 'SIMPLE'],
  'elk.layered.nodePlacement.favorStraightEdges': [true, false],
  'elk.layered.nodePlacement.bk.fixedAlignment': ['NONE', 'BALANCED', 'LEFTUP', 'RIGHTUP', 'LEFTDOWN', 'RIGHTDOWN'],
  'elk.layered.nodePlacement.bk.edgeStraightening': ['IMPROVE_STRAIGHTNESS', 'NONE'],
  'elk.layered.nodePlacement.networkSimplex.nodeFlexibility': ['NONE', 'NODE_SIZE', 'PORT_POSITION', 'NODE_SIZE_WHERE_SPACE_PERMITS'],
  'elk.layered.cycleBreaking.strategy': ['GREEDY_MODEL_ORDER', 'GREEDY', 'DEPTH_FIRST', 'MODEL_ORDER', 'INTERACTIVE'],
  'elk.hierarchyHandling': ['INCLUDE_CHILDREN', 'SEPARATE_CHILDREN'],
  'elk.layered.compaction.postCompaction.strategy': ['NONE', 'LEFT', 'RIGHT', 'LEFT_RIGHT_CONSTRAINT_LOCKING', 'LEFT_RIGHT_CONNECTION_LOCKING', 'EDGE_LENGTH'],
  'spacing.baseValue': [40, 20, 60, 33.5],
  'nodeLabels.placement': ['[H_CENTER V_TOP, INSIDE]'],
  'elk.direction': ['UP', 'DOWN', 'LEFT', 'RIGHT'],
  'elk.spacing.edgeNode': [20],
  'elk.layered.wrapping.cutting.strategy': ['ARD'],
  'elk.layered.feedbackEdges': [true, false],
  'elk.layered.wrapping.strategy': ['MULTI_EDGE', 'SINGLE_EDGE'],
  'elk.layered.crossingMinimization.semiInteractive': [true],
  'elk.layered.edgeRouting.splines.sloppy.layerSpacingFactor': [1],
  'elk.layered.edgeRouting.polyline.slopedEdgeZoneWidth': [4.0],
  'elk.layered.wrapping.validify.strategy': ['LOOK_BACK'],
  'elk.insideSelfLoops.activate': [true],
  'elk.separateConnectedComponents': [true, false],
  'elk.alignment': ['LEFT'],
  'elk.edgeRouting': ['ORTHOGONAL', 'POLYLINE', 'SPLINES', 'UNDEFINED'],
  'elk.layered.unnecessaryBendpoints': [true, false],
  'elk.layered.mergeEdges': [true, false],
  'elk.layered.mergeHierarchyEdges': [true, false],
  'elk.layered.priority.straightness': [0, 5],
  'elk.layered.priority.shortness': [0, 5],
  'elk.layered.priority.direction': [1],
  'elk.layered.edgeRouting.selfLoopDistribution': ['EQUALLY', 'NORTH', 'NORTH_SOUTH'],
  'elk.layered.edgeRouting.selfLoopOrdering': ['STACKED', 'SEQUENCED'],
  'elk.layered.edgeRouting.splines.mode': ['CONSERVATIVE', 'CONSERVATIVE_SOFT', 'SLOPPY'],
  'spacing.edgeEdge': [10],
  'elk.layered.spacing.edgeEdgeBetweenLayers': [20],
  'spacing.edgeNode': [20],
  'elk.layered.spacing.edgeNodeBetweenLayers': [80],
  'elk.layered.edgeLabels.sideSelection': ['SMART_DOWN', 'SMART_UP', 'ALWAYS_UP', 'ALWAYS_DOWN', 'DIRECTION_UP', 'DIRECTION_DOWN'],
  'elk.layered.edgeLabels.centerLabelPlacementStrategy': ['MEDIAN_LAYER', 'HEAD_LAYER', 'TAIL_LAYER', 'SPACE_EFFICIENT_LAYER', 'WIDEST_LAYER', 'CENTER_LAYER'],
  // Not in the catalogue, but valid for rootLayoutOptions.
  'spacing.nodeNode': [20, 50, 120, 37.25],
  'spacing.nodeNodeBetweenLayers': [25, 50, 100],
  'elk.layered.spacing.nodeNodeBetweenLayers': [25, 60],
  'spacing.edgeNodeBetweenLayers': [10],
  'spacing.edgeEdgeBetweenLayers': [20],
  'spacing.nodeSelfLoop': [20],
  'elk.padding': ['[top=24,left=24,bottom=24,right=24]', '[top=0,left=0,bottom=0,right=0]', '[top=10.5,left=40,bottom=3,right=7]'],
  'elk.randomSeed': [1, 2, 12345],
  'elk.layered.thoroughness': [1, 7, 30],
  'elk.aspectRatio': [0.5, 1.6, 3],
  'elk.layered.highDegreeNodes.treatment': [true],
  'elk.layered.compaction.connectedComponents': [true],
  'elk.spacing.componentComponent': [10, 60],
  'elk.layered.spacing.baseValue': [30],
  'elk.interactive': [true],
  'elk.layered.crossingMinimization.hierarchicalSweepiness': [-1, 0.1, 1],
  'elk.layered.directionCongruency': ['READING_DIRECTION', 'ROTATION'],
  'elk.layered.layering.minWidth.upperBoundOnWidth': [2, 4],
  'elk.layered.layering.nodePromotion.maxIterations': [0, 5],
  'elk.layered.edgeRouting.splines.conservative': [true],
  'elk.layered.generatePositionAndLayerIds': [true],
};

function randomOverrides(R, count) {
  const keys = Object.keys(CATALOGUE);
  const out = {};
  for (let i = 0; i < count; i++) {
    const key = R.pick(keys);
    out[key] = R.pick(CATALOGUE[key]);
  }
  return out;
}

// ---------------------------------------------------------------------------
// Generic ELK graphs: ports, node and port labels, edges that live in child
// nodes, hyperedges, fixed positions.

const SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];

function genericGraph(R, o) {
  const n = o.n;
  let seq = 0;
  const root = { id: 'root', layoutOptions: { 'elk.algorithm': o.algorithm ?? 'layered', ...(o.options ?? {}) }, children: [], edges: [] };
  const containers = [root];
  const leaves = [];
  const allNodes = [];
  const ports = new Map();
  const nContainers = o.containers ?? (n > 3 && R.chance(0.5) ? R.int(1, Math.max(1, Math.round(n / 5))) : 0);
  for (let c = 0; c < nContainers; c++) {
    const parent = R.chance(0.4) ? R.pick(containers) : root;
    const node = { id: `c${c}`, children: [], edges: [] };
    const lo = {};
    if (R.chance(0.5)) lo['elk.padding'] = `[top=${R.int(5, 40)},left=${R.int(5, 30)},bottom=${R.int(5, 30)},right=${R.int(5, 30)}]`;
    if (R.chance(0.3)) lo['elk.direction'] = R.pick(['UP', 'DOWN', 'LEFT', 'RIGHT']);
    if (R.chance(0.25)) lo['elk.hierarchyHandling'] = R.pick(['INCLUDE_CHILDREN', 'SEPARATE_CHILDREN']);
    if (R.chance(0.2)) lo['elk.algorithm'] = R.pick(o.childAlgorithms ?? ['layered', 'box', 'rectpacking', 'mrtree', 'force', 'stress', 'radial']);
    if (R.chance(0.3)) {
      node.labels = [{ text: R.pick(WORDS), width: size(R, 20, 120), height: R.pick([14, 21]) }];
      lo['elk.nodeLabels.placement'] = R.pick(['[H_CENTER, V_TOP, INSIDE]', '[H_LEFT, V_TOP, INSIDE]', '[H_CENTER, V_TOP, OUTSIDE]', '[H_RIGHT, V_BOTTOM, INSIDE]']);
      if (R.chance(0.6)) lo['elk.nodeSize.constraints'] = R.pick(['[NODE_LABELS, MINIMUM_SIZE]', '[NODE_LABELS, PORTS, MINIMUM_SIZE]', '[PORTS]']);
    }
    if (Object.keys(lo).length > 0) node.layoutOptions = lo;
    parent.children.push(node);
    containers.push(node);
    allNodes.push(node);
  }
  for (let i = 0; i < n; i++) {
    const parent = nContainers > 0 && R.chance(0.6) ? R.pick(containers) : root;
    const node = { id: o.numericIds ? i + 1 : `v${i}`, width: size(R, 20, 200), height: size(R, 20, 120) };
    if (o.positions || R.chance(0.1)) {
      node.x = size(R, 0, 60 * Math.sqrt(n + 1));
      node.y = size(R, 0, 60 * Math.sqrt(n + 1));
    }
    const lo = {};
    if ((o.ports ?? true) && R.chance(o.portChance ?? 0.3)) {
      const count = R.int(1, 5);
      const constraint = R.pick(['FREE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS', 'FIXED_RATIO', undefined]);
      if (constraint) lo['elk.portConstraints'] = constraint;
      node.ports = [];
      for (let p = 0; p < count; p++) {
        const port = { id: `${node.id}_p${p}`, width: R.pick([0, 5, 8, 10]), height: R.pick([0, 5, 8, 10]) };
        const plo = {};
        if (constraint && constraint !== 'FREE') plo['elk.port.side'] = R.pick(SIDES);
        if (constraint === 'FIXED_ORDER' && R.chance(0.7)) plo['elk.port.index'] = p;
        if (constraint === 'FIXED_POS' || constraint === 'FIXED_RATIO') {
          const side = plo['elk.port.side'];
          port.x = side === 'WEST' ? -port.width : side === 'EAST' ? node.width : size(R, 0, node.width - port.width);
          port.y = side === 'NORTH' ? -port.height : side === 'SOUTH' ? node.height : size(R, 0, node.height - port.height);
        }
        if (R.chance(0.25)) {
          port.labels = [{ text: `p${p}`, width: size(R, 8, 30), height: 12 }];
        }
        if (Object.keys(plo).length > 0) port.layoutOptions = plo;
        node.ports.push(port);
        ports.set(port.id, node);
      }
      if (R.chance(0.3)) lo['elk.portLabels.placement'] = R.pick(['[INSIDE]', '[OUTSIDE]', '[OUTSIDE, NEXT_TO_PORT_IF_POSSIBLE]']);
      if (R.chance(0.3)) lo['elk.portAlignment.default'] = R.pick(['CENTER', 'BEGIN', 'END', 'DISTRIBUTED', 'JUSTIFIED']);
    }
    if (R.chance(0.2)) {
      node.labels = [{ text: R.pick(WORDS), width: size(R, 10, 90), height: R.pick([12, 16, 21]) }];
      if (R.chance(0.7)) lo['elk.nodeLabels.placement'] = R.pick(['[H_CENTER, V_CENTER, INSIDE]', '[H_LEFT, V_TOP, OUTSIDE]', '[H_CENTER, V_BOTTOM, OUTSIDE]', '[H_RIGHT, V_CENTER, INSIDE]']);
      if (R.chance(0.5)) lo['elk.nodeSize.constraints'] = R.pick(['[NODE_LABELS]', '[NODE_LABELS, PORTS, PORT_LABELS, MINIMUM_SIZE]', '[MINIMUM_SIZE]']);
      if (R.chance(0.3)) lo['elk.nodeSize.minimum'] = `(${R.int(20, 150)}, ${R.int(20, 90)})`;
    }
    if (o.nodeOptions && R.chance(0.15)) {
      const [k, v] = R.pick(o.nodeOptions);
      lo[k] = v;
    }
    if (Object.keys(lo).length > 0) node.layoutOptions = lo;
    parent.children.push(node);
    leaves.push(node);
    allNodes.push(node);
    seq++;
  }
  const kind = o.topology ?? R.pick(n > 40 ? SPARSE_TOPOLOGIES : TOPOLOGIES);
  let e = 0;
  const endpoint = (node) => {
    if (node.ports && R.chance(0.7)) {
      return R.pick(node.ports).id;
    }
    return node.id;
  };
  const containerOf = new Map();
  const walk = (parent) => {
    for (const c of parent.children ?? []) {
      containerOf.set(c.id, parent);
      walk(c);
    }
  };
  walk(root);
  for (const [a, b] of topology(R, n, kind)) {
    const src = leaves[a];
    const dst = leaves[b];
    const edge = { id: `e${e++}`, sources: [endpoint(src)], targets: [endpoint(dst)] };
    if (o.hyper && R.chance(0.1) && n > 2) {
      edge.targets.push(endpoint(leaves[R.int(0, n - 1)]));
    }
    if (R.chance(o.labels ?? 0.25)) {
      const count = R.chance(0.2) ? 2 : 1;
      edge.labels = [];
      for (let l = 0; l < count; l++) {
        const label = { text: R.pick(WORDS), width: size(R, 10, 90), height: R.pick([12, 16, 21]) };
        if (R.chance(0.6)) {
          label.layoutOptions = { 'elk.edgeLabels.placement': R.pick(['CENTER', 'HEAD', 'TAIL']) };
          if (R.chance(0.3)) label.layoutOptions['elk.edgeLabels.inline'] = true;
        }
        if (R.chance(0.3)) label.id = `${edge.id}_l${l}`;
        edge.labels.push(label);
      }
    }
    if (R.chance(0.05)) edge.layoutOptions = { 'elk.priority': R.int(1, 10) };
    // ELK accepts an edge in any node. Most tools put it in the root or in
    // the lowest common ancestor.
    const where = R.chance(0.7) ? root : (containerOf.get(src.id) ?? root);
    (where.edges ??= []).push(edge);
  }
  if (o.old && R.chance(0.5)) {
    // The primitive edge format.
    for (let k = 0; k < R.int(1, 3) && n > 1; k++) {
      root.edges.push({ id: `pe${k}`, source: leaves[R.int(0, n - 1)].id, target: leaves[R.int(0, n - 1)].id });
    }
  }
  for (const c of allNodes) {
    if (c.edges && c.edges.length === 0) delete c.edges;
  }
  return root;
}

const GENERIC_ROOT_OPTIONS = [
  ['elk.direction', ['UP', 'DOWN', 'LEFT', 'RIGHT']],
  ['elk.edgeRouting', ['ORTHOGONAL', 'POLYLINE', 'SPLINES']],
  ['elk.hierarchyHandling', ['INCLUDE_CHILDREN', 'SEPARATE_CHILDREN']],
  ['elk.layered.nodePlacement.strategy', ['NETWORK_SIMPLEX', 'BRANDES_KOEPF', 'LINEAR_SEGMENTS', 'SIMPLE']],
  ['elk.layered.layering.strategy', ['NETWORK_SIMPLEX', 'LONGEST_PATH', 'COFFMAN_GRAHAM', 'MIN_WIDTH']],
  ['elk.layered.cycleBreaking.strategy', ['GREEDY', 'DEPTH_FIRST', 'MODEL_ORDER', 'GREEDY_MODEL_ORDER']],
  ['elk.layered.considerModelOrder.strategy', ['NODES_AND_EDGES', 'NONE', 'PREFER_EDGES', 'PREFER_NODES']],
  ['elk.layered.crossingMinimization.strategy', ['LAYER_SWEEP', 'NONE']],
  ['elk.spacing.nodeNode', [10, 20, 45.5, 80]],
  ['elk.layered.spacing.nodeNodeBetweenLayers', [10, 20, 70]],
  ['elk.spacing.edgeNode', [5, 10, 25]],
  ['elk.spacing.edgeEdge', [5, 10, 15]],
  ['elk.spacing.edgeLabel', [2, 6]],
  ['elk.spacing.labelLabel', [0, 4]],
  ['elk.spacing.portPort', [5, 10, 20]],
  ['elk.padding', ['[top=12,left=12,bottom=12,right=12]', '[top=30,left=5,bottom=0,right=50]']],
  ['elk.layered.edgeRouting.splines.mode', ['CONSERVATIVE', 'CONSERVATIVE_SOFT', 'SLOPPY']],
  ['elk.layered.mergeEdges', [true]],
  ['elk.layered.unnecessaryBendpoints', [true]],
  ['elk.layered.edgeLabels.sideSelection', ['SMART_DOWN', 'ALWAYS_UP', 'DIRECTION_DOWN']],
  ['elk.layered.edgeLabels.centerLabelPlacementStrategy', ['MEDIAN_LAYER', 'HEAD_LAYER', 'WIDEST_LAYER']],
  ['elk.layered.wrapping.strategy', ['MULTI_EDGE', 'SINGLE_EDGE']],
  ['elk.layered.wrapping.additionalEdgeSpacing', [5, 20]],
  ['elk.layered.wrapping.multiEdge.distancePenalty', [1.5, 2.3]],
  ['elk.layered.compaction.postCompaction.strategy', ['LEFT', 'EDGE_LENGTH']],
  ['elk.layered.nodePlacement.bk.fixedAlignment', ['BALANCED', 'LEFTUP', 'RIGHTDOWN']],
  ['elk.layered.thoroughness', [1, 15]],
  ['elk.randomSeed', [1, 99]],
  ['elk.separateConnectedComponents', [false]],
  ['elk.portConstraints', ['FREE', 'FIXED_SIDE']],
  ['elk.layered.feedbackEdges', [true]],
  ['elk.layered.highDegreeNodes.treatment', [true]],
  ['elk.layered.layering.nodePromotion.strategy', ['NIKOLOV', 'DUMMYNODE_PERCENTAGE']],
  ['elk.json.shapeCoords', ['ROOT', 'PARENT']],
  ['elk.json.edgeCoords', ['ROOT', 'PARENT', 'CONTAINER']],
];

const OTHER_ALGORITHM_OPTIONS = {
  stress: [
    ['elk.stress.desiredEdgeLength', [50, 100, 187.5]],
    ['elk.stress.epsilon', [0.001, 0.01]],
    ['elk.stress.iterationLimit', [50, 300]],
    ['elk.stress.dimension', ['XY', 'X', 'Y']],
    ['elk.interactive', [true]],
  ],
  force: [
    ['elk.force.model', ['EADES', 'FRUCHTERMAN_REINGOLD']],
    ['elk.force.iterations', [50, 300]],
    ['elk.force.repulsion', [2, 5.5]],
    ['elk.force.temperature', [0.001, 0.01]],
    ['elk.force.repulsivePower', [0, 1]],
    ['elk.spacing.nodeNode', [20, 80]],
    ['elk.randomSeed', [1, 5]],
  ],
  mrtree: [
    ['elk.direction', ['UP', 'DOWN', 'LEFT', 'RIGHT']],
    ['elk.mrtree.weighting', ['MODEL_ORDER', 'DESCENDANTS', 'FAN', 'CONSTRAINT']],
    ['elk.mrtree.searchOrder', ['DFS', 'BFS']],
    ['elk.mrtree.edgeRoutingMode', ['NONE', 'MIDDLE_TO_MIDDLE', 'AVOID_OVERLAP']],
    ['elk.mrtree.compaction', [true]],
    ['elk.spacing.nodeNode', [10, 40]],
  ],
  radial: [
    ['elk.radial.radius', [0, 100, 250.5]],
    ['elk.radial.compactor', ['NONE', 'RADIAL_COMPACTION', 'WEDGE_COMPACTION']],
    ['elk.radial.sorter', ['NONE', 'POLAR_COORDINATE', 'ID']],
    ['elk.radial.wedgeCriteria', ['LEAF_NUMBER', 'NODE_SIZE']],
    ['elk.radial.optimizationCriteria', ['NONE', 'EDGE_LENGTH', 'EDGE_LENGTH_BY_POSITION', 'CROSSING_MINIMIZATION_BY_POSITION']],
    ['elk.radial.rotate', [true]],
    ['elk.radial.rotation.targetAngle', [0, 0.7, 3.1]],
    ['elk.radial.rotation.computeAdditionalWedgeSpace', [true]],
    ['elk.radial.centerOnRoot', [true]],
    ['elk.spacing.nodeNode', [10, 40]],
  ],
  box: [
    ['elk.box.packingMode', ['SIMPLE', 'GROUP_DEC', 'GROUP_MIXED', 'GROUP_INC']],
    ['elk.aspectRatio', [0.8, 1.3, 2.5]],
    ['elk.expandNodes', [true]],
    ['elk.spacing.nodeNode', [5, 15, 30]],
    ['elk.priority', [1]],
  ],
  rectpacking: [
    ['elk.aspectRatio', [0.8, 1.6, 2.5]],
    ['elk.expandNodes', [true]],
    ['elk.rectpacking.trybox', [true]],
    ['elk.rectpacking.packing.compaction.rowHeightReevaluation', [true]],
    ['elk.rectpacking.packing.compaction.iterations', [1, 10]],
    ['elk.rectpacking.whiteSpaceElimination.strategy', ['NONE', 'EQUAL_BETWEEN_STRUCTURES', 'TO_ASPECT_RATIO']],
    ['elk.rectpacking.widthApproximation.strategy', ['TARGET_WIDTH', 'SCANLINE']],
    ['elk.rectpacking.widthApproximation.optimizationGoal', ['ASPECT_RATIO_DRIVEN', 'MAX_SCALE_DRIVEN', 'AREA_DRIVEN']],
    ['elk.rectpacking.packing.strategy', ['COMPACTION', 'SIMPLE', 'NONE']],
    ['elk.spacing.nodeNode', [5, 15]],
  ],
  sporeOverlap: [
    ['elk.spacing.nodeNode', [8, 20]],
    ['elk.overlapRemoval.maxIterations', [16, 64]],
    ['elk.overlapRemoval.runScanline', [true, false]],
    ['elk.structure.structureExtractionStrategy', ['DELAUNAY_TRIANGULATION']],
  ],
  sporeCompaction: [
    ['elk.spacing.nodeNode', [8, 20]],
    ['elk.compaction.orthogonal', [true]],
    ['elk.processingOrder.treeConstruction', ['MINIMUM_SPANNING_TREE', 'MAXIMUM_SPANNING_TREE']],
    ['elk.compaction.compactionStrategy', ['DEPTH_FIRST', 'IMPROVE_STRAIGHTNESS']],
    ['elk.processingOrder.spanningTreeCostFunction', ['CENTER_DISTANCE', 'CIRCLE_UNDERLAP', 'RECTANGLE_UNDERLAP', 'INVERTED_OVERLAP', 'MINIMUM_ROOT_DISTANCE']],
    ['elk.processingOrder.rootSelection', ['FIXED', 'CENTER_NODE']],
  ],
  disco: [
    ['elk.disco.componentCompaction.strategy', ['POLYOMINO']],
    ['elk.spacing.componentComponent', [10, 30]],
    ['elk.polyomino.traversalStrategy', ['SPIRAL', 'LINE_BY_LINE', 'MANHATTAN', 'JITTER', 'QUADRANTS_LINE_BY_LINE', 'QUADRANTS_MANHATTAN', 'QUADRANTS_JITTER', 'COMBINE_LINE_BY_LINE_MANHATTAN', 'COMBINE_JITTER_MANHATTAN']],
    ['elk.polyomino.fill', [true, false]],
    ['elk.polyomino.lowLevelSort', ['NONE', 'BY_SIZE', 'BY_SIZE_AND_SHAPE']],
    ['elk.polyomino.highLevelSort', ['NONE', 'NUM_OF_EXTERNAL_SIDES_THAN_NUM_OF_EXTENSIONS_LAST', 'CORNER_CASES_THAN_SINGLE_SIDE_LAST']],
  ],
};

function pickOptions(R, table, count) {
  const out = {};
  for (let i = 0; i < count; i++) {
    const [k, values] = R.pick(table);
    out[k] = R.pick(values);
  }
  return out;
}

// ---------------------------------------------------------------------------
// The cases.

function buildCases() {
  const cases = [];
  const add = (name, graph, extra = {}) => cases.push({ name, input: JSON.stringify(graph), ...extra });
  let seed = 1000;

  // Benchmark graphs. The Go benchmarks load them by name.
  for (const n of [5, 20, 50, 150]) {
    const R = prng(42 + n);
    const data = randomLayoutData(R, { n, groups: Math.min(8, Math.max(1, Math.round(n / 10))), maxDepth: 2, topology: 'dag', rich: false, configLevel: 0, direction: 'TB', labels: 0.25, groupEdges: false });
    add(`bench-${n}`, buildMermaidGraph(data, 'elk.layered'));
  }

  // A. mermaid flowchart-like graphs with the default algorithm.
  const sizesA = [
    [1, 1, 4],
    [2, 2, 6],
    [3, 6, 56],
    [7, 15, 80],
    [16, 30, 36],
    [31, 60, 10],
    [61, 100, 3],
    [101, 150, 3],
  ];
  for (const [lo, hi, count] of sizesA) {
    for (let i = 0; i < count; i++) {
      const R = prng(seed++);
      const n = R.int(lo, hi);
      const data = randomLayoutData(R, {
        n,
        configLevel: R.pick([0, 0, 1, 1, 2]),
        spreadPorts: R.chance(0.3),
        entryOnTop: R.chance(0.15),
        terminalLabels: R.chance(0.2),
        reversed: R.chance(0.2),
        dirs: R.chance(0.3),
      });
      add(`mermaid-layered-n${n}-${i}`, buildMermaidGraph(data, 'elk.layered'));
    }
  }

  // B. mermaid graphs with rootLayoutOptions from the option catalogue.
  for (let i = 0; i < 80; i++) {
    const R = prng(seed++);
    const n = R.int(2, i % 10 === 0 ? 24 : 14);
    const data = randomLayoutData(R, { n, configLevel: R.pick([0, 1]), terminalLabels: R.chance(0.15), dirs: R.chance(0.2) });
    const overrides = randomOverrides(R, R.int(1, 3));
    add(`mermaid-catalogue-n${n}-${i}`, buildMermaidGraph(data, 'elk.layered', overrides));
  }
  // Every catalogue value once, on the same small graph.
  {
    let i = 0;
    for (const [key, values] of Object.entries(CATALOGUE)) {
      for (const value of values) {
        const R = prng(7 + (i % 5));
        const data = randomLayoutData(R, { n: 7 + (i % 5), groups: 2, topology: 'cyclic', configLevel: 0, rich: false, direction: 'TB' });
        add(`mermaid-option-${i++}-${key}=${value}`, buildMermaidGraph(data, 'elk.layered', { [key]: value }));
      }
    }
  }
  // Every direction with every edge routing.
  for (const direction of ['TB', 'BT', 'LR', 'RL']) {
    for (const routing of ['ORTHOGONAL', 'POLYLINE', 'SPLINES']) {
      const R = prng(seed++);
      const data = randomLayoutData(R, { n: 12, groups: 3, direction, configLevel: 0, topology: 'cyclic' });
      add(`mermaid-${direction}-${routing}`, buildMermaidGraph(data, 'elk.layered', { 'elk.edgeRouting': routing }));
    }
  }

  // The layering strategy STRETCH_WIDTH, which mermaid offers as
  // elk.layeringStrategy. elkjs does not terminate for some of these graphs.
  for (let i = 0; i < 4; i++) {
    const R = prng(seed++);
    const data = randomLayoutData(R, { n: R.int(3, 9), elk: { mergeEdges: false, forceNodeModelOrder: false, considerModelOrder: 'NODES_AND_EDGES', layeringStrategy: 'STRETCH_WIDTH' } });
    add(`mermaid-stretch-width-${i}`, buildMermaidGraph(data, 'elk.layered'));
  }

  // C. The other algorithms that mermaid exposes as layouts.
  const rootAlgorithms = ['elk.stress', 'elk.force', 'elk.mrtree', 'elk.sporeOverlap', 'elk.box', 'elk.rectpacking'];
  for (const algorithm of rootAlgorithms) {
    // force iterates 300 times over all pairs of nodes, stress starts with a
    // force layout, and the overlap removal of spore builds a triangulation.
    // These graphs stay small, and all but one for each algorithm limit the
    // iterations, as a diagram can do with rootLayoutOptions.
    const iterative = algorithm === 'elk.stress' || algorithm === 'elk.force';
    const spore = algorithm === 'elk.sporeOverlap';
    for (let i = 0; i < 11; i++) {
      const R = prng(seed++);
      let n;
      if (i === 0) n = 1;
      else if (iterative) n = i === 1 ? R.int(2, 3) : R.int(2, 16);
      else if (spore) n = R.int(2, i >= 8 ? 30 : 14);
      else n = i === 10 ? R.int(100, 150) : i >= 8 ? R.int(30, 60) : R.int(2, 20);
      const flat = R.chance(0.5);
      const data = randomLayoutData(R, {
        n,
        groups: flat ? 0 : undefined,
        configLevel: 0,
        groupEdges: false,
        selfLoops: R.chance(0.3),
        topology: algorithm === 'elk.mrtree' && R.chance(0.6) ? 'tree' : undefined,
      });
      const limit = iterative && i > 1 ? (algorithm === 'elk.stress' ? { 'elk.stress.iterationLimit': 15, 'elk.force.iterations': 15 } : { 'elk.force.iterations': 15 }) : undefined;
      add(`mermaid-${algorithm}-n${n}-${i}`, buildMermaidGraph(data, algorithm, limit));
    }
  }

  // D. Containers with their own algorithm or direction.
  for (let i = 0; i < 50; i++) {
    const R = prng(seed++);
    const n = R.int(3, i % 12 === 0 ? 30 : 16);
    // The iterative algorithms run in a few small graphs only.
    const withIterative = i % 20 === 1;
    const data = randomLayoutData(R, {
      n: withIterative ? Math.min(n, 7) : n,
      groups: R.int(1, Math.max(1, Math.round(n / 4))),
      containerAlgorithms: i % 3 === 0 ? ['elk.radial', 'elk.mrtree'] : withIterative ? CONTAINER_ALGORITHMS : CONTAINER_ALGORITHMS.filter((a) => a !== 'elk.stress' && a !== 'elk.force'),
      dirs: true,
      inGroup: 0.8,
      configLevel: R.pick([0, 1]),
      // Half of the graphs keep edges inside one container, so that the
      // container algorithm really runs.
      groupEdges: R.chance(0.5),
    });
    if (i % 2 === 0) {
      const parent = new Map(data.nodes.map((x) => [x.id, x.parentId]));
      data.edges = data.edges.filter((e) => parent.get(e.start) === parent.get(e.end));
    }
    add(`mermaid-container-n${data.nodes.filter((x) => !x.isGroup).length}-${i}`, buildMermaidGraph(data, 'elk.layered'));
  }

  // E. Generic ELK graphs for the layered algorithm.
  for (let i = 0; i < 80; i++) {
    const R = prng(seed++);
    const n = i % 25 === 0 ? R.int(80, 150) : i % 7 === 0 ? R.int(30, 50) : R.int(1, 18);
    const options = pickOptions(R, GENERIC_ROOT_OPTIONS.map(([k, v]) => [k, v]), R.int(0, 5));
    add(
      `generic-layered-n${n}-${i}`,
      genericGraph(R, {
        n,
        options,
        containers: n > 30 ? R.int(0, 3) : undefined,
        hyper: R.chance(0.2),
        old: R.chance(0.1),
        numericIds: i % 20 === 3,
        positions: R.chance(0.15),
        nodeOptions: [
          ['elk.layered.layering.layerConstraint', 'FIRST'],
          ['elk.layered.layering.layerConstraint', 'LAST'],
          ['elk.layered.layering.layerConstraint', 'FIRST_SEPARATE'],
          ['elk.alignment', 'CENTER'],
          ['elk.priority', 5],
          ['elk.layered.priority.straightness', 10],
        ],
      })
    );
  }

  // F. Generic graphs for the other algorithms, with positions.
  for (const [algorithm, table] of Object.entries(OTHER_ALGORITHM_OPTIONS)) {
    for (let i = 0; i < 9; i++) {
      const R = prng(seed++);
      const heavy = ['force', 'stress', 'sporeOverlap', 'sporeCompaction', 'disco'].includes(algorithm);
      const n = i === 0 ? 1 : i === 8 ? (heavy ? R.int(30, 45) : R.int(90, 150)) : R.int(2, heavy ? 14 : 30);
      const tree = algorithm === 'radial' || algorithm === 'mrtree';
      const options = pickOptions(R, table, R.int(0, 3));
      if (i > 1 && (algorithm === 'force' || algorithm === 'stress')) options['elk.force.iterations'] = 15;
      if (i > 1 && algorithm === 'stress') options['elk.stress.iterationLimit'] = 15;
      const graph = genericGraph(R, {
        n,
        algorithm,
        options,
        containers: R.chance(0.7) ? 0 : undefined,
        childAlgorithms: [algorithm, 'layered', 'box'],
        ports: !tree && R.chance(0.2),
        positions: R.chance(0.8),
        topology: tree ? 'tree' : undefined,
        labels: 0.1,
      });
      add(`generic-${algorithm}-n${n}-${i}`, graph);
    }
  }
  // Nodes with the same centre make sporeOverlap call Math.random.
  for (let i = 0; i < 4; i++) {
    const R = prng(seed++);
    const children = [];
    for (let k = 0; k < 6 + i; k++) children.push({ id: `s${k}`, width: 40, height: 30, x: (k % 2) * 10, y: 0 });
    add(`generic-sporeOverlap-same-centre-${i}`, { id: 'root', layoutOptions: { 'elk.algorithm': 'sporeOverlap', 'elk.spacing.nodeNode': R.pick([5, 12]) }, children, edges: [{ id: 'e0', sources: ['s0'], targets: ['s1'] }] });
  }

  // The width approximation GREEDY of rectpacking. elkjs does not terminate
  // for some graphs with it.
  for (let i = 0; i < 2; i++) {
    const R = prng(seed++);
    add(`generic-rectpacking-greedy-${i}`, genericGraph(R, { n: R.int(8, 30), algorithm: 'rectpacking', options: { 'elk.rectpacking.widthApproximation.strategy': 'GREEDY' }, containers: 0, ports: false }));
  }

  // G. Special inputs.
  const small = () => ({
    id: 'root',
    layoutOptions: { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN', 'elk.direction': 'DOWN' },
    children: [
      { id: 'A', width: 152, height: 45 },
      { id: 'B', width: 171, height: 171 },
      { id: 'sg', children: [{ id: 'E', width: 100, height: 45 }], layoutOptions: { 'elk.padding': '[top=40,left=20,bottom=20,right=20]' } },
    ],
    edges: [
      { id: 'e1', sources: ['A'], targets: ['B'] },
      { id: 'e2', sources: ['B'], targets: ['E'], labels: [{ text: 'Yes', width: 23, height: 21 }] },
    ],
  });
  const mutate = (fn) => {
    const g = small();
    fn(g);
    return g;
  };
  add('special-small', small());
  add('special-only-root', { id: 'r' });
  add('special-root-with-size', { id: 'r', width: 10, height: 20, x: 5, y: 6 });
  add('special-no-options', { id: 'root', children: [{ id: 'a', width: 10, height: 10 }, { id: 'b', width: 10, height: 10 }], edges: [{ id: 'e', sources: ['a'], targets: ['b'] }] });
  add('special-empty-children', { id: 'root', children: [], edges: [] });
  add('special-zero-size', { id: 'root', children: [{ id: 'a', width: 0, height: 0 }, { id: 'b' }], edges: [{ id: 'e', sources: ['a'], targets: ['b'] }] });
  add('special-hash-in-input', { id: 'root', $H: 900001, children: [{ id: 'a', $H: 900002, width: 10, height: 10 }, { $H: 900003, id: 'b', width: 10, height: 10 }], edges: [{ id: 'e', sources: ['a'], targets: ['b'] }] }, { keepHash: true });
  add('special-numeric-ids', { id: 0, children: [{ id: 1, width: 30, height: 30 }, { id: 2, width: 30, height: 30 }], edges: [{ id: 3, sources: [1], targets: [2] }] });
  add('special-index-like-keys', { id: 'root', 10: 'x', 2: 'y', children: [{ 7: 1, id: 'a', width: 10, height: 10, 3: 2 }], b: 1 });
  add('special-unicode', { id: 'root', children: [{ id: 'ä', width: 30, height: 30, text: '  line 😀' }, { id: '日本', width: 30, height: 30 }, { id: 'a"b\\c', width: 30, height: 30 }], edges: [{ id: 'é', sources: ['ä'], targets: ['日本'], labels: [{ text: '\u0000\u001f\u007f', width: 10, height: 10 }] }, { id: 'e2', sources: ['日本'], targets: ['a"b\\c'] }] });
  add('special-extra-values', { id: 'root', extra: { a: [1, 2.5, -0, 1e21, 1e-7, 123456789012345680000, null, true, 'x'], b: {} }, children: [{ id: 'a', width: 1e-7, height: 123456.789, junk: [[], {}, null] }] });
  add('special-large-coordinates', { id: 'root', layoutOptions: { 'elk.algorithm': 'layered', 'elk.spacing.nodeNode': 1e6 }, children: [{ id: 'a', width: 1e7, height: 3 }, { id: 'b', width: 0.001, height: 1e5 }, { id: 'c', width: 12345.678901234567, height: 1 / 3 }], edges: [{ id: 'e', sources: ['a'], targets: ['b'] }, { id: 'f', sources: ['a'], targets: ['c'] }] });
  add('special-fixed-layout', { id: 'root', layoutOptions: { 'elk.algorithm': 'fixed' }, children: [{ id: 'a', width: 10, height: 10, x: 3, y: 4 }] });
  add('special-random-layout', { id: 'root', layoutOptions: { 'elk.algorithm': 'random' }, children: [{ id: 'a', width: 10, height: 10 }, { id: 'b', width: 10, height: 10 }, { id: 'c', width: 10, height: 10 }] });
  add('special-string-numbers', mutate((g) => { g.children[0].width = '12'; }));
  add('special-option-types', mutate((g) => { Object.assign(g.layoutOptions, { 'elk.spacing.nodeNode': '33.5', 'elk.layered.mergeEdges': 'true', 'elk.layered.thoroughness': 3, 'elk.padding': '[top=5, left=6, bottom=7, right=8]' }); }));
  add('special-bad-enum', mutate((g) => { g.layoutOptions['elk.direction'] = 'SIDEWAYS'; }));
  add('special-unknown-option', mutate((g) => { g.layoutOptions['elk.nope.nope'] = 'x'; }));
  add('special-bad-padding', mutate((g) => { g.layoutOptions['elk.padding'] = 'garbage'; }));
  add('special-bad-double', mutate((g) => { g.layoutOptions['spacing.nodeNode'] = 'abc'; }));
  add('special-duplicate-id', mutate((g) => { g.children.push({ id: 'A', width: 1, height: 1 }); }));
  add('special-properties-key', mutate((g) => { g.properties = g.layoutOptions; delete g.layoutOptions; }));
  add('special-logging-key', mutate((g) => { g.logging = { x: 1 }; }));
  add('special-existing-layout', mutate((g) => { g.children[0].x = 500; g.children[0].y = 600; g.edges[0].sections = [{ id: 'old', startPoint: { x: 1, y: 2 }, endPoint: { x: 3, y: 4 }, bendPoints: [{ x: 9, y: 9 }] }]; g.edges[0].junctionPoints = [{ x: 1, y: 1 }]; }));
  add('special-layout-twice', mutate((g) => { g.children[0].x = 21.5; g.children[0].y = 12; g.width = 195; g.height = 431; g.edges[0].container = 'root'; }));
  for (const seed0 of [0, 1]) {
    const R = prng(5000 + seed0);
    const data = randomLayoutData(R, { n: 8, groups: 1, topology: 'dense', configLevel: 0 });
    add(`special-random-seed-zero-${seed0}`, buildMermaidGraph(data, 'elk.layered', { 'elk.randomSeed': 0 }));
    const g = genericGraph(prng(5100 + seed0), { n: 7, algorithm: 'force', options: { 'elk.randomSeed': 0, 'elk.force.iterations': 20 }, containers: 0, ports: false });
    add(`special-force-seed-zero-${seed0}`, g);
  }

  // H. Graphs that elkjs rejects.
  add('error-unknown-algorithm', mutate((g) => { g.layoutOptions['elk.algorithm'] = 'nope'; }));
  add('error-unknown-child-algorithm', mutate((g) => { g.children[2].layoutOptions['elk.algorithm'] = 'elk.missing'; g.children[2].layoutOptions['elk.hierarchyHandling'] = 'SEPARATE_CHILDREN'; g.layoutOptions['elk.hierarchyHandling'] = 'SEPARATE_CHILDREN'; }));
  add('error-missing-id', mutate((g) => { delete g.children[0].id; }));
  add('error-missing-root-id', {});
  add('error-array-graph', []);
  add('error-string-graph', 'x');
  add('error-number-graph', 5);
  add('error-true-graph', true);
  add('error-bad-id-type', mutate((g) => { g.children[0].id = { a: 1 }; }));
  add('error-fraction-id', mutate((g) => { g.children[0].id = 1.5; }));
  add('error-null-id', mutate((g) => { g.children[0].id = null; }));
  add('error-missing-target', mutate((g) => { g.edges[0].targets = ['ZZ']; }));
  add('error-missing-source', mutate((g) => { g.edges[0].sources = ['ZZ']; }));
  add('error-no-source', mutate((g) => { g.edges[0].sources = []; }));
  add('error-no-targets-key', mutate((g) => { delete g.edges[0].targets; }));
  add('error-edge-without-id', mutate((g) => { delete g.edges[0].id; }));
  add('error-null-width', mutate((g) => { g.children[0].width = null; }));
  add('error-null-child', mutate((g) => { g.children.push(null); }));
  add('error-children-number', mutate((g) => { g.children = 5; }));
  add('error-primitive-edge-no-source', mutate((g) => { g.edges.push({ id: 'pe', source: 'ZZ', target: 'A' }); }));
  add('error-port-of-other-node', { id: 'root', children: [{ id: 'a', width: 10, height: 10, ports: [{ id: 'p', width: 1, height: 1 }] }, { id: 'b', width: 10, height: 10 }], edges: [{ id: 'e', source: 'b', sourcePort: 'p', target: 'a' }] });
  add('error-section-reference', mutate((g) => { g.edges[0].sections = [{ id: 's', incomingSections: ['nope'], startPoint: { x: 0, y: 0 }, endPoint: { x: 1, y: 1 } }]; }));
  add('error-bend-point', mutate((g) => { g.edges[0].sections = [{ id: 's', startPoint: { x: 0, y: 0 }, endPoint: { x: 1, y: 1 }, bendPoints: [{ x: 1 }] }]; }));
  add('error-radial-not-a-tree', { id: 'root', layoutOptions: { 'elk.algorithm': 'radial' }, children: [{ id: 'a', width: 10, height: 10 }, { id: 'b', width: 10, height: 10 }, { id: 'c', width: 10, height: 10 }], edges: [{ id: 'e1', sources: ['a'], targets: ['b'] }, { id: 'e2', sources: ['b'], targets: ['c'] }, { id: 'e3', sources: ['c'], targets: ['a'] }] });
  add('error-layer-constraint-conflict', { id: 'root', layoutOptions: { 'elk.algorithm': 'layered' }, children: [{ id: 'a', width: 10, height: 10, layoutOptions: { 'elk.layered.layering.layerConstraint': 'FIRST_SEPARATE' } }, { id: 'b', width: 10, height: 10, layoutOptions: { 'elk.layered.layering.layerConstraint': 'FIRST_SEPARATE' } }], edges: [{ id: 'e1', sources: ['a'], targets: ['b'] }] });
  add('error-hierarchical-edge-separate', { id: 'root', layoutOptions: { 'elk.algorithm': 'stress' }, children: [{ id: 'g', children: [{ id: 'a', width: 10, height: 10 }] }, { id: 'b', width: 10, height: 10 }], edges: [{ id: 'e1', sources: ['a'], targets: ['b'] }] });

  return cases;
}

// ---------------------------------------------------------------------------
// Running elkjs.

const HANG_MS = Number(process.env.ELK_GEN_HANG_MS ?? 25000);
const PARALLEL = 6;

// layoutFresh is the first layout of a new elkjs module in a new isolate.
function layoutFresh(input) {
  return new Promise((done) => {
    const worker = new Worker(fileURLToPath(import.meta.url), { workerData: { input } });
    const timer = setTimeout(() => {
      worker.terminate();
      done({ hang: true, rand: 0 });
    }, HANG_MS);
    worker.once('message', (r) => {
      clearTimeout(timer);
      worker.terminate();
      done(r);
    });
    worker.once('error', (e) => {
      clearTimeout(timer);
      done({ err: `worker: ${e.message}`, java: false, rand: 0 });
    });
  });
}

async function workerMain() {
  installEnvironment();
  const ELK = freshELK();
  parentPort.postMessage(await layoutWith(new ELK(), workerData.input));
}

async function layoutWith(elk, input) {
  beginLayout();
  try {
    const graph = JSON.parse(input);
    const out = JSON.stringify(await elk.layout(graph));
    return { out, rand: randomCalls };
  } catch (e) {
    const message = String(e && e.message !== undefined ? e.message : e);
    return { err: message, java: /^(org\.eclipse|java\.|com\.google)[\w.$]*(: |$)/.test(message), rand: randomCalls };
  }
}

// countNodes counts the nodes of an ELK graph, without the root.
function countNodes(graph) {
  let n = 0;
  const walk = (node) => {
    for (const c of Array.isArray(node?.children) ? node.children : []) {
      if (c !== null && typeof c === 'object') {
        n++;
        walk(c);
      }
    }
  };
  walk(graph);
  return n;
}

// stripHash removes the "$H" members that elkjs adds.
const stripHash = (s) => (s === undefined ? s : s.replace(/,"\$H":\d+/g, ''));

async function main() {
  const cases = buildCases();
  const t0 = performance.now();
  const lines = [];
  const results = [];
  let errors = 0;
  let randoms = 0;
  let maxNodes = 0;
  const trace = process.env.ELK_GEN_TRACE === '1';
  // The fresh-isolate pass runs a few workers at a time.
  {
    let next = 0;
    const run = async () => {
      while (next < cases.length) {
        const i = next++;
        const t1 = performance.now();
        results[i] = await layoutFresh(cases[i].input);
        if (trace) process.stderr.write(`${cases[i].name} ${(performance.now() - t1).toFixed(0)} ms${results[i].hang ? ' HANG' : ''}\n`);
      }
    };
    await Promise.all(Array.from({ length: PARALLEL }, run));
  }
  let hangs = 0;
  for (let i = 0; i < cases.length; i++) {
    const c = cases[i];
    const r = results[i];
    if (r.hang) {
      hangs++;
    } else if (r.err !== undefined) {
      errors++;
    } else if (JSON.stringify(JSON.parse(r.out)) !== r.out) {
      throw new Error(`${c.name}: output does not survive a JSON round trip`);
    }
    if (r.rand > 0) randoms++;
    c.nodes = countNodes(JSON.parse(c.input));
    maxNodes = Math.max(maxNodes, c.nodes);
  }
  const tFresh = performance.now() - t0;

  // History check 1: one module, a new ELK per layout (what mermaid does on
  // one page), in generation order.
  const ELK = freshELK();
  let diffs = 0;
  let hashDiffs = 0;
  installEnvironment();
  const report = (what, c, a, b) => {
    diffs++;
    if (diffs <= 10) console.error(`HISTORY DIFFERENCE (${what}) in ${c.name}\n  fresh:  ${(a.out ?? a.err).slice(0, 300)}\n  shared: ${(b.out ?? b.err).slice(0, 300)}`);
  };
  const warmMs = new Array(cases.length).fill(0);
  for (let i = 0; i < cases.length; i++) {
    if (results[i].hang) continue;
    const t1 = performance.now();
    const r = await layoutWith(new ELK(), cases[i].input);
    warmMs[i] = performance.now() - t1;
    if (stripHash(r.out) !== stripHash(results[i].out) || r.err !== results[i].err) report('new ELK per layout', cases[i], results[i], r);
    else if (r.out !== results[i].out) hashDiffs++;
  }
  // History check 2: the same module, one ELK instance, three shuffled orders.
  const elk = new ELK();
  for (let round = 0; round < 3; round++) {
    const order = prng(77 + round).shuffle(cases.map((_, i) => i));
    for (const i of order) {
      if (results[i].hang) continue;
      const t1 = performance.now();
      const r = await layoutWith(elk, cases[i].input);
      warmMs[i] = Math.min(warmMs[i], performance.now() - t1);
      if (stripHash(r.out) !== stripHash(results[i].out) || r.err !== results[i].err) report(`shared ELK, round ${round}`, cases[i], results[i], r);
    }
  }
  // Undefined members: mermaid passes undefined option values, JSON drops them.
  let undefinedDiffs = 0;
  {
    const R = prng(31337);
    for (let i = 0; i < 40; i++) {
      const data = randomLayoutData(R, { n: R.int(2, 15), configLevel: 0 });
      delete data.config.elk.mergeEdges;
      const graph = buildMermaidGraph(data, 'elk.layered');
      const hasUndefined = Object.values(graph.layoutOptions).some((v) => v === undefined);
      if (!hasUndefined) throw new Error('expected undefined option values');
      const viaJson = await layoutWith(new ELK(), JSON.stringify(graph));
      beginLayout();
      let direct;
      try {
        direct = { out: JSON.stringify(await new ELK().layout(graph)) };
      } catch (e) {
        direct = { err: String(e.message) };
      }
      if (stripHash(direct.out) !== stripHash(viaJson.out) || direct.err !== viaJson.err) undefinedDiffs++;
    }
  }
  restoreEnvironment();

  // One line for each case. in and out are raw JSON, so that the Go test
  // reads the exact bytes. ms is the time of a warm layout in V8, which the
  // Go test uses to pick the cases of a short run.
  for (let i = 0; i < cases.length; i++) {
    const c = cases[i];
    const r = results[i];
    let line = `{"name":${JSON.stringify(c.name)},"nodes":${c.nodes},"ms":${Math.round(warmMs[i] * 100) / 100},"in":${c.input}`;
    if (r.hang) line += ',"hang":true';
    else if (r.err !== undefined) line += `,"err":${JSON.stringify(r.err)}` + (r.java ? '' : ',"native":true');
    else line += `,"out":${r.out}`;
    if (r.rand > 0) line += `,"random":${r.rand}`;
    if (c.keepHash) line += ',"keepHash":true';
    lines.push(line + '}');
  }
  if (process.env.ELK_GEN_TRACE === '1') {
    const slow = cases.map((c, i) => [warmMs[i], c.name]).sort((a, b) => b[0] - a[0]).slice(0, 25);
    for (const [ms, name] of slow) process.stderr.write(`warm ${ms.toFixed(1)} ms ${name}\n`);
    process.stderr.write(`warm total ${(warmMs.reduce((a, b) => a + b, 0) / 1000).toFixed(1)} s\n`);
  }

  mkdirSync(outDir, { recursive: true });
  const body = lines.join('\n') + '\n';
  const gz = gzipSync(Buffer.from(body), { level: 9 });
  writeFileSync(resolve(outDir, 'golden.jsonl.gz'), gz);
  console.log(`cases: ${cases.length} (errors: ${errors}, hangs: ${hangs}, with Math.random: ${randoms}), largest graph: ${maxNodes} nodes`);
  console.log(`golden.jsonl.gz: ${(body.length / 1e6).toFixed(2)} MB raw, ${(gz.length / 1e6).toFixed(2)} MB gzip`);
  console.log(`fresh-module pass: ${(tFresh / 1000).toFixed(1)} s`);
  console.log(`history check: ${diffs} layouts differ from the fresh-module layout (ignoring $H), ${hashDiffs} of ${cases.length} differ only in $H`);
  console.log(`undefined-member check: ${undefinedDiffs} of 40 graphs differ between undefined option values and omitted ones`);
  if (diffs > 0) process.exitCode = 1;
}

// ---------------------------------------------------------------------------
// --bench: V8 timings for the benchmark graphs.

async function bench() {
  const cases = buildCases().filter((c) => c.name.startsWith('bench-'));
  const median = (xs) => xs.slice().sort((a, b) => a - b)[Math.floor(xs.length / 2)];
  // Cold start: evaluate the bundle, register, lay out once.
  const cold = [];
  for (let i = 0; i < 7; i++) {
    const t = performance.now();
    const ELK = freshELK();
    const tLoaded = performance.now();
    await new ELK().layout(JSON.parse(cases[0].input));
    cold.push([tLoaded - t, performance.now() - tLoaded]);
  }
  console.log(`V8 ${process.versions.v8}, node ${process.version}`);
  console.log(`cold: load ${median(cold.map((c) => c[0])).toFixed(1)} ms, first layout (${cases[0].name}) ${median(cold.map((c) => c[1])).toFixed(1)} ms`);
  const ELK = freshELK();
  const elk = new ELK();
  for (const c of cases) {
    const times = [];
    const first = performance.now();
    await elk.layout(JSON.parse(c.input));
    const tFirst = performance.now() - first;
    const reps = c.name === 'bench-150' ? 15 : 40;
    for (let i = 0; i < reps; i++) {
      const g = JSON.parse(c.input);
      const t = performance.now();
      await elk.layout(g);
      times.push(performance.now() - t);
    }
    times.sort((a, b) => a - b);
    console.log(`${c.name}: first ${tFirst.toFixed(1)} ms, warm min ${times[0].toFixed(2)} ms, median ${median(times).toFixed(2)} ms (${c.input.length} bytes in)`);
  }
  const mem = process.memoryUsage();
  console.log(`heapUsed ${(mem.heapUsed / 1e6).toFixed(1)} MB, rss ${(mem.rss / 1e6).toFixed(1)} MB`);
}

// ---------------------------------------------------------------------------
// --math: results of the fdlibm functions in JavaScript (elk-fdlibm.mjs) for
// the Math functions that elkjs calls, and how often the native functions of
// this node build differ from them.

function mathGolden() {
  const R = prng(2024);
  const buf = new DataView(new ArrayBuffer(8));
  const hex = (x) => {
    buf.setFloat64(0, x);
    return buf.getBigUint64(0).toString(16).padStart(16, '0');
  };
  const fromBits = (hi, lo) => {
    buf.setUint32(0, hi);
    buf.setUint32(4, lo);
    return buf.getFloat64(0);
  };
  const ulps = (a, b) => {
    if (Number.isNaN(a) && Number.isNaN(b)) return 0;
    if (Number.isNaN(a) || Number.isNaN(b)) return Infinity;
    buf.setFloat64(0, a);
    const x = buf.getBigInt64(0);
    buf.setFloat64(0, b);
    const d = x - buf.getBigInt64(0);
    return Number(d < 0n ? -d : d);
  };
  const anyDouble = () => fromBits((R.next() * 4294967296) >>> 0, (R.next() * 4294967296) >>> 0);
  const special = [0, -0, 1, -1, 0.5, -0.5, 2, 3, 10, 0.1, 1e-300, 1e300, 5e-324, 2.2250738585072014e-308, 1.7976931348623157e308, Infinity, -Infinity, NaN, Math.PI, Math.PI / 2, Math.PI / 4, -Math.PI, (3 * Math.PI) / 4, 1e-8, 1e-5, 0.7853981633974483, 1.5707963267948966, 3.141592653589793, 6.283185307179586, 1e6, 1e15, 1e22, 0.6744, 0.4375, 0.6875, 1.1875, 2.4375, 0.2, 0.3, 0.9999999999999999, 1.0000000000000002, 0.7071067811865476, 1.4142135623730951];
  const nativeStats = {};
  const stat = (name, ref, nat) => {
    const st = (nativeStats[name] ??= { rows: 0, differ: 0, maxUlp: 0 });
    st.rows++;
    const d = ulps(ref, nat);
    if (d !== 0) {
      st.differ++;
      st.maxUlp = Math.max(st.maxUlp, d);
    }
  };
  const unary = (name, gen, n) => {
    const xs = [...special];
    for (let i = 0; i < n; i++) xs.push(gen(i));
    return xs.map((x) => {
      const ref = fdlibm.functions[name](x);
      stat(name, ref, native[name](x));
      return [hex(x), hex(ref)];
    });
  };
  const binary = (name, gen, n) => {
    const args = [];
    for (const a of special) for (const b of [0, -0, 1, -1, 0.5, 2, 3, Infinity, -Infinity, NaN, 1e-300, 7.5, -2.5]) args.push([a, b]);
    for (let i = 0; i < n; i++) args.push(gen(i));
    return args.map(([a, b]) => {
      const ref = fdlibm.functions[name](a, b);
      stat(name, ref, native[name](a, b));
      return [hex(a), hex(b), hex(ref)];
    });
  };
  const angle = (i) => (i % 4 === 0 ? anyDouble() : i % 4 === 1 ? (R.next() - 0.5) * 20 : i % 4 === 2 ? (R.next() - 0.5) * 1e-3 : (R.next() - 0.5) * 10 ** R.int(1, 18));
  const unit = (i) => (i % 5 === 0 ? anyDouble() : i % 5 === 1 ? (R.next() - 0.5) * 2 : i % 5 === 2 ? 1 - R.next() * 1e-9 : i % 5 === 3 ? (R.next() - 0.5) * 1e-6 : (R.next() < 0.5 ? -1 : 1) * (0.4 + R.next() * 0.6));
  const positive = (i) => (i % 4 === 0 ? Math.abs(anyDouble()) : i % 4 === 1 ? R.next() * 1000 : i % 4 === 2 ? 1 + (R.next() - 0.5) * 1e-3 : R.int(1, 100000));
  const coord = () => (R.next() - 0.5) * 10 ** R.int(-3, 6);
  const powArgs = (i) => {
    switch (i % 6) {
      case 0:
        return [anyDouble(), anyDouble()];
      case 1:
        return [R.next() * 1000, (R.next() - 0.5) * 10];
      case 2:
        return [R.int(0, 40), R.int(0, 40)];
      case 3:
        return [R.next() * 50, R.pick([1.5, 2.3, 2, 0.5, 3, -1, 1 / 3])];
      case 4:
        return [(R.next() - 0.5) * 100, R.int(-8, 8)];
      default:
        return [1 + (R.next() - 0.5) * 1e-4, (R.next() - 0.5) * 1e6];
    }
  };
  const N = 2000;
  const data = {
    v8: process.versions.v8,
    arch: process.arch,
    sin: unary('sin', angle, N),
    cos: unary('cos', angle, N),
    acos: unary('acos', unit, N),
    log: unary('log', positive, N),
    atan2: binary('atan2', (i) => (i % 3 === 0 ? [anyDouble(), anyDouble()] : [coord(), coord()]), N),
    pow: binary('pow', powArgs, N),
  };
  data.native = nativeStats;
  mkdirSync(outDir, { recursive: true });
  const gz = gzipSync(Buffer.from(JSON.stringify(data)), { level: 9 });
  writeFileSync(resolve(outDir, 'math.json.gz'), gz);
  console.log(`math.json.gz: ${(gz.length / 1e6).toFixed(2)} MB gzip`);
  for (const [name, st] of Object.entries(nativeStats)) {
    console.log(`Math.${name} of this node build (${process.arch}) differs from fdlibm in ${st.differ} of ${st.rows} rows, by at most ${st.maxUlp} ulp`);
  }
}

// --times: the time of a warm layout of each case in one module, to keep the
// run time of the Go test in bounds. It skips the cases that are known not
// to terminate.
async function times() {
  const cases = buildCases().filter((c) => !/stretch-width|rectpacking-greedy|STRETCH_WIDTH/.test(c.name));
  installEnvironment();
  const ELK = freshELK();
  const elk = new ELK();
  const rows = [];
  for (const c of cases) {
    let best = Infinity;
    for (let i = 0; i < 2; i++) {
      const t = performance.now();
      await layoutWith(elk, c.input);
      best = Math.min(best, performance.now() - t);
    }
    rows.push([best, c.name, countNodes(JSON.parse(c.input)), c.input.length]);
  }
  restoreEnvironment();
  rows.sort((a, b) => b[0] - a[0]);
  for (const [ms, name, nodes] of rows.slice(0, 40)) console.log(`${ms.toFixed(1).padStart(8)} ms  ${String(nodes).padStart(4)} nodes  ${name}`);
  const total = rows.reduce((a, r) => a + r[0], 0);
  const bytes = rows.reduce((a, r) => a + r[3], 0);
  console.log(`${cases.length} cases, total ${(total / 1000).toFixed(2)} s, ${rows.filter((r) => r[0] > 100).length} over 100 ms, ${rows.filter((r) => r[2] >= 100).length} with 100 nodes or more, input ${(bytes / 1e6).toFixed(1)} MB`);
}

if (!isMainThread) {
  await workerMain();
} else if (process.argv.includes('--times')) {
  await times();
} else if (process.argv.includes('--dump')) {
  // Print the input of one case, for debugging.
  const name = process.argv[process.argv.indexOf('--dump') + 1];
  console.log(buildCases().find((c) => c.name === name).input);
} else if (process.argv.includes('--bench')) {
  await bench();
} else if (process.argv.includes('--math')) {
  mathGolden();
} else {
  await main();
}
