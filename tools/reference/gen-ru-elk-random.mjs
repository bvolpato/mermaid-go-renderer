// Emits random golden cases for internal/ru/layout/elk.
//
//   node gen-ru-elk-random.mjs [--count N]
//
// The script loads the ELK layout chunk of the built mermaid 12.2.0 in plain
// node and calls its real functions on random layout data: prepareLayoutForElk
// and runElkLayoutCore, with the genuine elkjs 0.9.3. It writes
// internal/ru/layout/elk/testdata/random.jsonl.gz, with the same fields per
// line as probes/ru-elk.js records for a corpus fixture. The random data has
// what the corpus has little of: other ELK algorithms, non-default config.elk
// values, container algorithms, edges that end on groups, nodes without an
// intersect function, terminal labels, and so on.
//
// The measure phase needs a DOM, so the script does what the measure phase
// does to the data: it gives each node a size and an intersect function, each
// group a labelBBox and each labelled edge a size.
//
// elkjs reads Math.random, Date.now and six Math functions. The script
// installs the same ones as the Go engine (internal/elk), as gen-elk.mjs does.
import { registerHooks } from 'node:module';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';
import * as fdlibm from './elk-fdlibm.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const outDir = path.join(root, 'internal/ru/layout/elk/testdata');
const CHUNKS = path.join(here, 'node_modules/mermaid/dist/chunks/mermaid.core');
const elkFile = fs.readdirSync(CHUNKS).find((f) => /^elk-.*\.mjs$/.test(f));

let count = 400;
const argv = process.argv.slice(2);
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--count') {
    count = Number(argv[++i]);
  }
}

// Export every top-level binding of the ELK chunk.
registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/' + elkFile)) {
      const src = result.source.toString();
      const names = new Set();
      for (const m of src.matchAll(/^(?:async function\*?|function\*?|var|let|const|class) ([A-Za-z_$][\w$]*)/gm)) {
        names.add(m[1]);
      }
      names.delete('render');
      return { ...result, source: src + `\nexport { ${[...names].map((n) => `${n} as __x_${n}`).join(', ')} };\n` };
    }
    return result;
  },
});
const mod = await import(pathToFileURL(path.join(CHUNKS, elkFile)).href);
const up = (name) => {
  const fn = mod['__x_' + name];
  if (fn === undefined) {
    throw new Error(`the ELK chunk has no ${name}`);
  }
  return fn;
};
const ELK = (await import('elkjs/lib/elk.bundled.js')).default;

// ---- the environment of elkjs, as in gen-elk.mjs ----
const FIXED_NOW = 1700000000000;
const RANDOM_SEED = 0x9e3779b9;
let randomState = RANDOM_SEED;
let randomCalls = 0;
let nowCalls = 0;
let nowBase = 0;
Math.random = () => {
  randomCalls++;
  randomState = (randomState + 0x6d2b79f5) >>> 0;
  let t = randomState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
Date.now = () => {
  nowCalls++;
  return FIXED_NOW - nowBase;
};
for (const [name, fn] of Object.entries(fdlibm.functions)) {
  Math[name] = fn;
}
const beginLayout = () => {
  randomState = RANDOM_SEED;
  randomCalls = 0;
  nowBase = nowCalls;
};

// ---- seeded PRNG for the cases (mulberry32) ----
function prng(seed) {
  let a = seed >>> 0;
  const next = () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  return {
    next,
    int: (lo, hi) => lo + Math.floor(next() * (hi - lo + 1)),
    chance: (p) => next() < p,
    pick: (list) => list[Math.floor(next() * list.length)],
    // A size like a measured one: a multiple of 1/64.
    size: (lo, hi) => Math.round((lo + next() * (hi - lo)) * 64) / 64,
    float: (lo, hi) => lo + next() * (hi - lo),
  };
}

// A number that is not finite is kept as { $num: "NaN" }.
const enc = (v) => JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x));
const clean = (v) => {
  const s = enc(v);
  return s === undefined ? undefined : JSON.parse(s);
};

// ---- intersect functions, as the shapes of mermaid make them ----
function intersectRect(node, point) {
  const x = node.x;
  const y = node.y;
  const dx = point.x - x;
  const dy = point.y - y;
  let w = node.width / 2;
  let h = node.height / 2;
  let sx, sy;
  if (Math.abs(dy) * w > Math.abs(dx) * h) {
    if (dy < 0) {
      h = -h;
    }
    sx = dy === 0 ? 0 : (h * dx) / dy;
    sy = h;
  } else {
    if (dx < 0) {
      w = -w;
    }
    sx = w;
    sy = dx === 0 ? 0 : (w * dy) / dx;
  }
  return { x: x + sx, y: y + sy };
}
function intersectEllipse(node, rx, ry, point) {
  const cx = node.x;
  const cy = node.y;
  const px = cx - point.x;
  const py = cy - point.y;
  const det = Math.sqrt(rx * rx * py * py + ry * ry * px * px);
  let dx = Math.abs((rx * ry * px) / det);
  if (point.x < cx) {
    dx = -dx;
  }
  let dy = Math.abs((rx * ry * py) / det);
  if (point.y < cy) {
    dy = -dy;
  }
  return { x: cx + dx, y: cy + dy };
}
function intersectDiamond(node, point) {
  const dx = point.x - node.x;
  const dy = point.y - node.y;
  const s = 1 / (Math.abs(dx) / (node.width / 2) + Math.abs(dy) / (node.height / 2));
  return { x: node.x + dx * s, y: node.y + dy * s };
}
// kind -> the intersect function of a node, or undefined.
const INTERSECT = {
  rect: (node) => (point) => intersectRect(node, point),
  circle: (node) => (point) => intersectEllipse(node, node.width / 2, node.width / 2, point),
  ellipse: (node) => (point) => intersectEllipse(node, node.width / 2, node.height / 2, point),
  diamond: (node) => (point) => intersectDiamond(node, point),
  // The result is on the far side of the node: tryNodeIntersect rejects it.
  wrongSide: (node) => (point) => ({ x: 2 * node.x - point.x, y: 2 * node.y - point.y }),
  // The result is the point itself: too close, tryNodeIntersect rejects it.
  self: () => (point) => ({ x: point.x, y: point.y }),
  none: () => undefined,
};

const ELK_CONFIG_CHOICES = {
  mergeEdges: [true, false],
  nodePlacementStrategy: ['SIMPLE', 'NETWORK_SIMPLEX', 'LINEAR_SEGMENTS', 'BRANDES_KOEPF'],
  nodePlacementAlignment: ['NONE', 'LEFTUP', 'LEFTDOWN', 'RIGHTUP', 'RIGHTDOWN', 'BALANCED'],
  preset: ['default', 'legacy', 'modelOrder', 'depthFirst', 'unknown', '__proto__'],
  // STRETCH_WIDTH is left out: elkjs does not terminate for many graphs.
  layeringStrategy: ['NETWORK_SIMPLEX', 'LONGEST_PATH', 'LONGEST_PATH_SOURCE', 'COFFMAN_GRAHAM', 'MIN_WIDTH'],
  layeringLayerBound: [2, 3, 4],
  cycleBreakingStrategy: ['GREEDY', 'DEPTH_FIRST', 'MODEL_ORDER', 'GREEDY_MODEL_ORDER'],
  forceNodeModelOrder: [true, false],
  considerModelOrder: ['NONE', 'NODES_AND_EDGES', 'PREFER_EDGES', 'PREFER_NODES'],
  keepEntryNodeOnTop: [true, false],
  orientFeedbackEdges: [true, false],
  straightenEdges: [true, false],
  lineHops: [true, false, 'gap'],
};

function randomElkConfig(R, level) {
  // The schema defaults of mermaid.
  const elk = {
    mergeEdges: false,
    forceNodeModelOrder: false,
    considerModelOrder: 'NODES_AND_EDGES',
    preset: 'default',
    keepEntryNodeOnTop: false,
    orientFeedbackEdges: true,
    straightenEdges: true,
    lineHops: true,
  };
  if (level === 0) {
    return elk;
  }
  for (const [key, values] of Object.entries(ELK_CONFIG_CHOICES)) {
    if (R.chance(level === 1 ? 0.15 : 0.5)) {
      elk[key] = R.pick(values);
    }
  }
  if (R.chance(0.05)) {
    delete elk.considerModelOrder;
  }
  if (R.chance(0.05)) {
    delete elk.preset;
  }
  return elk;
}

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
    case 'pairs':
      // Opposite edges between the same two nodes.
      for (let i = 0; i + 1 < n; i += 2) {
        add(i, i + 1);
        add(i + 1, i);
      }
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
const TOPOLOGIES = ['chain', 'tree', 'star', 'dag', 'dag', 'cyclic', 'cyclic', 'pairs', 'random', 'random', 'none'];

const LEAF_SHAPES = [
  ['squareRect', 'rect'],
  ['squareRect', 'rect'],
  ['roundedRect', 'rect'],
  ['circle', 'circle'],
  ['ellipse', 'ellipse'],
  ['diamond', 'diamond'],
  ['stadium', 'none'],
  ['hexagon', 'wrongSide'],
  ['odd', 'self'],
  ['rect33', 'rect'],
];
const GROUP_SHAPES = ['rect', 'rect', 'rect', 'roundedWithTitle', 'noteGroup', 'divider', undefined];
const CONTAINER_ALGORITHMS = ['elk.box', 'elk.rectpacking', 'elk.layered', 'elk.mrtree', 'elk.nope', 7];
const ARROWS = ['none', 'arrow_point', 'arrow_cross', 'arrow_circle', 'aggregation', 'extension', 'composition', 'dependency', 'lollipop', 'arrow_barb', 'arrow_barb_neo', '', undefined];
const EDGE_TYPES = ['arrow_point', 'arrow_point', 'arrow_open', 'arrow_cross', 'arrow_circle', 'double_arrow_point', 'double_arrow_cross', 'double_arrow_circle', 'normal', undefined];
const LABELS = ['Yes', 'No', 'a<br/>b', 'x<BR >y<br\t/>z', 'long label text', 'q<br>r'];
const TERMINAL_KEYS = ['startLeft', 'startRight', 'endLeft', 'endRight'];
const TERMINAL_TEXT = { startLeft: 'startLabelLeft', startRight: 'startLabelRight', endLeft: 'endLabelLeft', endRight: 'endLabelRight' };

// randomLayoutData builds data4Layout as a diagram db hands it to render,
// with the node property order of flowDb.getData.
function randomLayoutData(R, o) {
  const nodes = [];
  const kinds = new Map();
  const groups = [];
  const groupCount = o.groups ?? R.pick([0, 0, 1, 1, 2, 3, 4]);
  for (let g = 0; g < groupCount; g++) {
    const id = `G${g}`;
    // A group nests in an earlier group.
    const parent = g > 0 && R.chance(0.4) ? `G${R.int(0, g - 1)}` : undefined;
    const node = {
      id,
      label: R.chance(0.85) ? R.pick(['Group', 'A longer group title', 'g', '']) : undefined,
      labelStyle: '',
      labelType: 'text',
      parentId: parent,
      padding: R.pick([8, 8, 8, 20, undefined]),
      cssCompiledStyles: [],
      cssClasses: '',
      shape: R.pick(GROUP_SHAPES),
      dir: R.chance(o.dirs ?? 0.25) ? R.pick(['TB', 'LR', 'RL', 'BT', 'TD']) : undefined,
      isGroup: true,
      look: 'neo',
      colorIndex: g,
      metadata: R.chance(o.algorithms ?? 0.15) ? { algorithm: R.pick(CONTAINER_ALGORITHMS) } : R.chance(0.1) ? {} : undefined,
    };
    groups.push(node);
    nodes.push(node);
  }
  const n = o.n ?? R.int(1, 10);
  const leaves = [];
  for (let i = 0; i < n; i++) {
    const [shape, kind] = R.pick(LEAF_SHAPES);
    const tiny = R.chance(0.12);
    const node = {
      id: `n${i}`,
      label: `Node ${i}`,
      labelStyle: '',
      labelType: 'text',
      parentId: groups.length > 0 && R.chance(0.6) ? R.pick(groups).id : undefined,
      padding: 15,
      cssStyles: [],
      cssCompiledStyles: [],
      cssClasses: 'default ',
      domId: `my-svg-flowchart-n${i}-${i}`,
      look: 'neo',
      isGroup: false,
      shape: tiny ? 'stateStart' : shape,
      minWidth: 120,
    };
    if (R.chance(0.1)) {
      node.spreadPorts = true;
    }
    kinds.set(node, tiny ? 'circle' : kind);
    leaves.push(node);
    nodes.push(node);
  }
  if (R.chance(0.05)) {
    // An orphan: its parent is not a group of the diagram.
    nodes.push({ id: 'orphan', label: 'orphan', parentId: 'nowhere', isGroup: false, shape: 'squareRect' });
  }

  const edges = [];
  const endpoints = leaves.map((l) => l.id);
  const pairs = topology(R, n, o.topology ?? R.pick(TOPOLOGIES));
  // Some edges end on a group that has children.
  const usedGroups = groups.filter((g) => nodes.some((x) => x.parentId === g.id));
  if (usedGroups.length > 0 && n > 0) {
    const extra = R.int(0, 2);
    for (let k = 0; k < extra; k++) {
      const g = R.pick(usedGroups).id;
      const l = R.pick(endpoints);
      pairs.push(R.chance(0.5) ? [g, l] : [l, g]);
    }
    if (usedGroups.length > 1 && R.chance(0.3)) {
      pairs.push([usedGroups[0].id, usedGroups[1].id]);
    }
  }
  const id = (v) => (typeof v === 'number' ? endpoints[v] : v);
  pairs.forEach(([a, b], index) => {
    const start = id(a);
    const end = id(b);
    const type = R.pick(EDGE_TYPES);
    const edge = { id: `L_${start}_${end}_${index}`, start, end };
    if (type !== undefined) {
      edge.type = type;
    }
    if (R.chance(o.labels ?? 0.3)) {
      edge.label = R.pick(LABELS);
    } else if (R.chance(0.6)) {
      edge.label = '';
    }
    // A text that is the empty string is left out: the structs of the Go
    // port cannot tell it from an undefined text, and no diagram db sets it.
    if (R.chance(0.1)) {
      edge.text = 'text';
    }
    edge.labelType = 'text';
    if (R.chance(0.8)) {
      edge.labelpos = 'c';
    }
    const stroke = R.pick(['normal', 'normal', 'thick', 'dotted', 'invisible', undefined]);
    if (R.chance(0.7)) {
      edge.thickness = stroke ?? 'normal';
    }
    if (R.chance(0.3)) {
      edge.stroke = stroke;
    }
    if (R.chance(0.5)) {
      edge.minlen = R.int(1, 3);
    } else if (R.chance(0.3)) {
      edge.length = R.int(1, 3);
    }
    const classes = R.pick(['edge-thickness-normal edge-pattern-solid flowchart-link', 'relation', '', undefined]);
    if (classes !== undefined) {
      edge.classes = classes;
    }
    for (const key of ['arrowTypeStart', 'arrowTypeEnd']) {
      const arrow = R.pick(ARROWS);
      if (arrow !== undefined) {
        edge[key] = arrow;
      }
    }
    if (R.chance(0.3)) {
      edge.arrowhead = R.pick(['normal', 'none']);
    }
    if (R.chance(0.5)) {
      edge.arrowheadStyle = R.pick(['fill: #333', '']);
    }
    edge.cssCompiledStyles = [];
    const labelStyle = R.pick([[], ['color:red'], '', undefined]);
    if (labelStyle !== undefined) {
      edge.labelStyle = labelStyle;
    }
    const style = R.pick([[], ['stroke:blue', 'fill:none'], 'fill:none', '', undefined]);
    if (style !== undefined) {
      edge.style = style;
    }
    if (R.chance(0.6)) {
      edge.pattern = R.pick(['normal', 'solid', 'dashed', 'dotted']);
    }
    edge.look = 'neo';
    const curve = R.pick(['basis', 'linear', 'rounded', undefined]);
    if (curve !== undefined) {
      edge.curve = curve;
    }
    if (R.chance(o.terminals ?? 0.15)) {
      for (const key of TERMINAL_KEYS) {
        if (R.chance(0.5)) {
          edge[TERMINAL_TEXT[key]] = R.pick(['1', '0..*', 'many']);
        }
      }
    }
    edges.push(edge);
  });
  if (n > 0 && R.chance(o.missing ?? 0.02)) {
    // An edge to a node that does not exist: elkjs rejects the graph.
    edges.push({ id: 'L_missing', start: endpoints[0], end: 'missing', type: 'arrow_point', label: '', classes: 'x' });
  }

  const data = {
    nodes,
    edges,
    other: {},
    config: { elk: randomElkConfig(R, o.configLevel ?? R.pick([0, 0, 1, 1, 2])) },
    type: 'flowchart-v2',
    layoutAlgorithm: o.layout ?? 'elk',
    direction: o.direction === undefined ? R.pick(['TB', 'TB', 'TD', 'LR', 'RL', 'BT', undefined]) : o.direction,
    diagramId: 'my-svg',
  };
  if (data.direction === undefined) {
    delete data.direction;
  }
  return { data, kinds, groups };
}

// measure does to the data what createGraphWithElements does: sizes,
// intersect functions and label boxes.
function measure(R, { data, kinds, groups }) {
  for (const node of data.nodes) {
    if (node.isGroup) {
      node.labelBBox = node.label ? { width: R.size(10, R.chance(0.2) ? 400 : 120), height: R.pick([21, 24, 42]) } : { width: 0, height: 0 };
      continue;
    }
    if (node.shape === 'stateStart') {
      node.width = 14;
      node.height = 14;
    } else if (kinds.get(node) === 'circle') {
      node.width = node.height = R.size(30, 120);
    } else {
      node.width = R.chance(0.1) ? R.pick([10, 20, 23.5]) : R.size(40, 220);
      node.height = R.chance(0.1) ? R.pick([10, 20, 23.5]) : R.size(24, 130);
    }
    const make = INTERSECT[kinds.get(node) ?? 'rect'];
    const fn = make(node);
    if (fn) {
      node.intersect = fn;
    }
  }
  void groups;
  for (const edge of data.edges) {
    if (edge.label) {
      edge.width = R.size(12, 90);
      edge.height = R.pick([21, 42, 63]);
      // insertEdgeLabel replaces the label style array by a string.
      if (Array.isArray(edge.labelStyle)) {
        edge.labelStyle = edge.labelStyle.map((s) => ';' + s).join('');
      }
    }
    for (const key of TERMINAL_KEYS) {
      if (edge[TERMINAL_TEXT[key]]) {
        edge.terminalLabelSizes ??= {};
        edge.terminalLabelSizes[key] = { width: R.size(8, 40), height: 16.5 };
      }
    }
  }
}

const quiet = { debug() {}, info() {}, warn() {}, error() {} };
const point = (p) => (p ? { x: p.x, y: p.y } : null);

// The layout method of the class ELK, wrapped as in probes/ru-elk.js.
let proto = ELK.prototype;
while (proto && !Object.prototype.hasOwnProperty.call(proto, 'layout')) {
  proto = Object.getPrototypeOf(proto);
}
const origLayout = proto.layout;
let active = null;
proto.layout = function (graph, ...rest) {
  if (!active) {
    return origLayout.call(this, graph, ...rest);
  }
  const call = active.call;
  call.data = active.snapData();
  call.in = JSON.stringify(graph);
  let index = 0;
  const walk = (node) => {
    const i = index++;
    if (node && typeof node.intersect === 'function') {
      const orig = node.intersect;
      node.intersect = function (p) {
        call.intersectCount++;
        return orig.call(this, p);
      };
    }
    for (const c of (node && node.children) || []) {
      walk(c);
    }
  };
  for (const c of graph.children || []) {
    walk(c);
  }
  beginLayout();
  const started = performance.now();
  return origLayout.call(this, graph, ...rest).then(
    (result) => {
      call.ms = Math.round(performance.now() - started);
      call.random = randomCalls;
      call.same = result === graph;
      // The output repeats the input, so only its hash is kept. The Go
      // test runs the engine on the input. elkjs adds a member "$H" to
      // each node, which the Go engine removes.
      const out = JSON.stringify(result).replace(/,"\$H":-?\d+/g, '');
      call.outHash = crypto.createHash('sha256').update(out).digest('hex');
      return result;
    },
    (err) => {
      call.error = String((err && err.message) || err);
      throw err;
    }
  );
};

const prepareLayoutForElk = up('prepareLayoutForElk');
const runElkLayoutCore = up('runElkLayoutCore');

async function runCase(name, R, o) {
  const made = randomLayoutData(R, o);
  const data4Layout = made.data;
  const algorithm = o.algorithm ?? 'elk.layered';
  const cfg = { curve: undefined, flowchart: { wrappingWidth: 200 } };
  const context = {
    element: null,
    helpers: {
      common: { lineBreakRegex: /<br\s*\/?>/gi },
      // A new object for each call, as getConfig returns a copy.
      getConfig: () => ({ flowchart: { ...cfg.flowchart } }),
      interpolateToCurve: () => () => {},
      log: quiet,
    },
    options: { algorithm },
  };
  const r = { layout: data4Layout.layoutAlgorithm, algorithm, elk: [], results: [] };
  r.entry = clean(data4Layout.edges);
  context.preparedLayout = prepareLayoutForElk(data4Layout, context);
  r.prepared = clean(data4Layout.edges);
  r.preparedCurveTypes = data4Layout.edges.map((e) => typeof e.curve);
  measure(R, made);
  // The intersect functions are simple and the Go test has the same ones,
  // so the calls are counted and not recorded.
  const call = { intersectCount: 0 };
  r.elk.push(call);
  active = {
    call,
    snapData: () => ({
      type: data4Layout.type,
      layoutAlgorithm: data4Layout.layoutAlgorithm,
      direction: data4Layout.direction,
      diagramId: data4Layout.diagramId,
      nodes: clean(data4Layout.nodes),
      edges: clean(data4Layout.edges),
      intersect: data4Layout.nodes.map((node) => (typeof node.intersect === 'function' ? 1 : 0)),
      // The kind of the intersect function of each node: a key of INTERSECT.
      kinds: data4Layout.nodes.map((node) => (typeof node.intersect === 'function' ? made.kinds.get(node) ?? 'rect' : null)),
      curveTypes: data4Layout.edges.map((e) => typeof e.curve),
      config: { elk: clean(data4Layout.config.elk) },
      getConfig: { flowchart: { wrappingWidth: cfg.flowchart.wrappingWidth } },
    }),
  };
  const rec = { name, type: 'random', elkCalls: 1, renders: [r] };
  try {
    await runElkLayoutCore(data4Layout, context);
    r.results.push({
      nodes: data4Layout.nodes.map((node) =>
        clean({ id: node.id, x: node.x, y: node.y, width: node.width, height: node.height, labelData: node.labelData, labels: node.labels })
      ),
      edges: data4Layout.edges.map((e) =>
        clean({
          id: e.id,
          points: Array.isArray(e.points) ? e.points.map(point) : e.points,
          x: e.x,
          y: e.y,
          curve: typeof e.curve === 'string' ? e.curve : { $type: typeof e.curve },
          terminalLabelCenters: e.terminalLabelCenters,
        })
      ),
    });
  } catch (e) {
    rec.error = String((e && e.message) || e);
  } finally {
    active = null;
  }
  return rec;
}

const ROOT_ALGORITHMS = [
  ['elk', 'elk.layered'],
  ['elk.stress', 'elk.stress'],
  ['elk.force', 'elk.force'],
  ['elk.mrtree', 'elk.mrtree'],
  ['elk.box', 'elk.box'],
  ['elk.rectpacking', 'elk.rectpacking'],
];

const records = [];
const stats = { cases: 0, errors: 0, random: 0, withOut: 0 };
const R0 = prng(0x52554c4b);
for (let i = 0; i < count; i++) {
  const R = prng(1000 + i);
  const o = {};
  let tag = 'layered';
  if (i % 7 === 3) {
    // Another root algorithm.
    const [layout, algorithm] = R0.pick(ROOT_ALGORITHMS.slice(1));
    o.layout = layout;
    o.algorithm = algorithm;
    tag = algorithm;
    if (algorithm === 'elk.force' || algorithm === 'elk.stress') {
      // These algorithms iterate. The Go engine (elkjs in goja) takes
      // about 25 s for elk.force with 10 nodes, so the graphs stay small.
      o.n = R.int(1, 6);
    }
  } else if (i % 7 === 5) {
    // Many groups, with directions and algorithms.
    o.groups = R.int(2, 5);
    o.dirs = 0.5;
    o.algorithms = 0.4;
    tag = 'groups';
  } else if (i % 7 === 6) {
    // Terminal labels and labelled opposite edges.
    o.terminals = 0.7;
    o.labels = 0.8;
    o.topology = R.pick(['pairs', 'dag', 'cyclic']);
    tag = 'labels';
  }
  const rec = await runCase(`random/${String(i).padStart(4, '0')}-${tag}`, R, o);
  stats.cases++;
  const call = rec.renders[0].elk[0];
  if (rec.error) {
    stats.errors++;
  }
  if (call.random) {
    stats.random++;
  }
  if (call.outHash) {
    stats.withOut++;
  }
  records.push(rec);
}
fs.mkdirSync(outDir, { recursive: true });
const gz = zlib.gzipSync(records.map((r) => JSON.stringify(r)).join('\n') + '\n', { level: 9 });
fs.writeFileSync(path.join(outDir, 'random.jsonl.gz'), gz);
console.error(`${JSON.stringify(stats)} ${gz.length} bytes -> internal/ru/layout/elk/testdata/random.jsonl.gz`);
