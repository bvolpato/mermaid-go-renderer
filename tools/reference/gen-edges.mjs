// Generates the golden files of internal/ru/edges: cases for the edge
// functions of mermaid's unified renderer, each with the result of the
// real functions in Chromium (probes/edges.js).
//
//   node gen-edges.mjs all [outDir]      spy on the corpus, build the cases, write every golden
//   node gen-edges.mjs spy <out.json> [--layout dagre] [--limit N] <corpus dir>...
//   node gen-edges.mjs golden <cases.json> <golden.json[.gz]>
//
// `spy` renders corpus fixtures with real mermaid and records the inputs of
// insertEdge (probes/edges-spy.js). `golden` runs a cases file
// ({ cases: [...] }, see probes/edges.js) through the oracle. The golden
// file is { configs, styles, cases }: every case has its result, and the
// effective config and the style sheet are stored once, by index.
//
// The default outDir is internal/ru/edges/testdata. The cases are
// deterministic: random values come from a seeded generator.
//
// Environment variables of `all`: MMDG_ONLY=synthetic or MMDG_ONLY=corpus
// writes a part of the files. MMDG_SPY_DIR names a directory that keeps the
// spy results between runs. MMDG_DROPPED names a file that receives, one
// JSON line each, the spied edges whose replay does not reproduce the
// rendered path (they are not put into the golden files).
//
//   node gen-edges.mjs cases <dir>       write the synthetic case files, for inspection
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '../..');
const corpus = path.join(repo, 'testdata/corpus/mermaid-e2e');

function oracle(script, argsFile, outFile) {
  execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes', script), '--args', argsFile, '--out', outFile], {
    stdio: 'inherit',
    cwd: here,
  });
  return JSON.parse(fs.readFileSync(outFile, 'utf8'));
}

function tmpFile(name) {
  return path.join(os.tmpdir(), `mmdg-edges-${process.pid}-${name}`);
}

// ---- spy ----

function listFixtures(dirs, limit) {
  const fixtures = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) {
        walk(p);
      } else if (e.name.endsWith('.mmd')) {
        fixtures.push({ name: path.relative(corpus, p), text: fs.readFileSync(p, 'utf8') });
      }
    }
  };
  for (const d of dirs) {
    walk(path.join(corpus, d));
  }
  if (limit > 0 && fixtures.length > limit) {
    const step = fixtures.length / limit;
    return Array.from({ length: limit }, (_, i) => fixtures[Math.floor(i * step)]);
  }
  return fixtures;
}

function spy(dirs, { layout, limit = 0 } = {}) {
  const argsFile = tmpFile('spy-args.json');
  const outFile = tmpFile('spy-out.json');
  fs.writeFileSync(argsFile, JSON.stringify({ fixtures: listFixtures(dirs, limit), layout }));
  const res = oracle('edges-spy.js', argsFile, outFile);
  fs.rmSync(argsFile);
  fs.rmSync(outFile);
  return res;
}

// ---- golden ----

function runGolden(cases, outFile) {
  const argsFile = tmpFile('cases.json');
  const resFile = tmpFile('results.json');
  fs.writeFileSync(argsFile, JSON.stringify({ cases }));
  const results = oracle('edges.js', argsFile, resFile);
  fs.rmSync(argsFile);
  fs.rmSync(resFile);
  const configs = [];
  const styles = [];
  const index = (table, lookup, value) => {
    const key = JSON.stringify(value);
    if (!lookup.has(key)) {
      lookup.set(key, table.length);
      table.push(value);
    }
    return lookup.get(key);
  };
  const configIndex = new Map();
  const styleIndex = new Map();
  const golden = cases.map((c, i) => {
    const r = { ...c, directive: undefined, ...results[i] };
    if (r.config !== undefined) {
      r.config = index(configs, configIndex, r.config);
    }
    if (r.css !== undefined) {
      r.css = index(styles, styleIndex, r.css);
    }
    return r;
  });
  const json = JSON.stringify({ configs, styles, cases: golden });
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, outFile.endsWith('.gz') ? zlib.gzipSync(json, { level: 9 }) : json);
  const errors = golden.filter((g) => g.error).length;
  const opErrors = golden.reduce((n, g) => n + (g.results || []).filter((o) => o.error).length, 0);
  console.error(`${golden.length} cases, ${errors} case errors, ${opErrors} op errors -> ${outFile} (${fs.statSync(outFile).size} bytes)`);
  return golden;
}

// ---- case builders ----

// mulberry32, as in the oracle page.
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

const CURVES = ['linear', 'basis', 'cardinal', 'bumpX', 'bumpY', 'catmullRom', 'monotoneX', 'monotoneY', 'natural', 'step', 'stepAfter', 'stepBefore', 'rounded'];
const ARROWS = [
  'arrow_point',
  'arrow_circle',
  'arrow_cross',
  'arrow_open',
  'arrow_barb',
  'arrow_barb_neo',
  'arrow_hierarchy',
  'aggregation',
  'extension',
  'composition',
  'dependency',
  'lollipop',
  'only_one',
  'zero_or_one',
  'one_or_more',
  'zero_or_more',
  'requirement_arrow',
  'requirement_contains',
  'none',
  'unknown_arrow',
];
const MARKERS = {
  'flowchart-v2': ['point', 'circle', 'cross'],
  classDiagram: ['aggregation', 'extension', 'composition', 'dependency', 'lollipop'],
  stateDiagram: ['barb'],
  stateDiagramNeo: ['barbNeo'],
  er: ['only_one', 'zero_or_one', 'one_or_more', 'zero_or_more'],
  erNeo: ['only_one_neo', 'zero_or_one_neo', 'one_or_more_neo', 'zero_or_more_neo'],
  requirement: ['requirement_contains', 'requirement_arrow'],
  requirementNeo: ['requirement_contains_neo', 'requirement_arrow_neo'],
};
const ALL_MARKERS = [
  'extension',
  'composition',
  'aggregation',
  'dependency',
  'lollipop',
  'point',
  'circle',
  'cross',
  'barb',
  'hierarchy',
  'only_one',
  'zero_or_one',
  'one_or_more',
  'zero_or_more',
  'requirement_arrow',
  'requirement_contains',
];
const ALL_MARKERS_NEO = [
  'extension',
  'composition',
  'aggregation',
  'dependency',
  'lollipop',
  'point',
  'circle',
  'cross',
  'barbNeo',
  'hierarchy',
  'only_one_neo',
  'zero_or_one_neo',
  'one_or_more_neo',
  'zero_or_more_neo',
  'requirement_arrow_neo',
  'requirement_contains_neo',
];
const TYPES = ['flowchart-v2', 'classDiagram', 'stateDiagram', 'er', 'requirement'];

// Point list makers. Each returns [[x, y], ...].
function makePoints(rnd, n, kind) {
  const q = (v) => Math.round(v * 16) / 16; // values like those of dagre
  const pts = [];
  let x = q(20 + rnd() * 200);
  let y = q(20 + rnd() * 100);
  switch (kind) {
    case 'vertical':
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        y = q(y + 15 + rnd() * 50);
      }
      break;
    case 'horizontal':
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        x = q(x + 15 + rnd() * 60);
      }
      break;
    case 'orthogonal': {
      let horizontal = rnd() < 0.5;
      const sx = rnd() < 0.5 ? -1 : 1;
      const sy = rnd() < 0.5 ? -1 : 1;
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        const step = rnd() < 0.25 ? 3 + rnd() * 8 : 12 + rnd() * 70;
        if (horizontal) {
          x = q(x + sx * step);
        } else {
          y = q(y + sy * step);
        }
        horizontal = !horizontal;
      }
      break;
    }
    case 'collinear': {
      const dx = (rnd() - 0.5) * 60;
      const dy = 10 + rnd() * 40;
      for (let i = 0; i < n; i++) {
        pts.push([x + dx * i, y + dy * i]);
      }
      break;
    }
    case 'duplicate':
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        if (rnd() < 0.5) {
          x = q(x + (rnd() - 0.3) * 80);
          y = q(y + 10 + rnd() * 60);
        }
      }
      break;
    case 'short':
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        x = x + (rnd() - 0.5) * 9;
        y = y + (rnd() - 0.2) * 9;
      }
      break;
    case 'backward':
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        x = x + (rnd() - 0.5) * 120;
        y = y - rnd() * 70;
      }
      break;
    default:
      // free: any direction, unrounded doubles
      for (let i = 0; i < n; i++) {
        pts.push([x, y]);
        x = x + (rnd() - 0.5) * 160;
        y = y + (rnd() - 0.25) * 110;
      }
  }
  return pts;
}
const POINT_KINDS = ['free', 'vertical', 'horizontal', 'orthogonal', 'collinear', 'duplicate', 'short', 'backward'];
const toObj = (pts) => pts.map(([x, y]) => ({ x, y }));

function baseEdge(id, pts, extra = {}) {
  return {
    id,
    start: 'A',
    end: 'B',
    type: 'arrow_point',
    label: '',
    labelType: 'text',
    labelpos: 'c',
    thickness: 'normal',
    minlen: 1,
    classes: 'edge-thickness-normal edge-pattern-solid flowchart-link',
    arrowTypeStart: 'none',
    arrowTypeEnd: 'arrow_point',
    arrowheadStyle: 'fill: #333',
    cssCompiledStyles: [],
    labelStyle: [],
    style: [],
    pattern: 'solid',
    look: 'classic',
    points: toObj(pts),
    ...extra,
  };
}

// rectNode is a node spec whose intersect is intersect.rect around a point.
const rectNode = (p, w, h) => ({ rect: { x: p[0], y: p[1], width: w, height: h } });

function pureCases() {
  const rnd = prng(0x65646731);
  const cases = [];
  const add = (fn, args) => cases.push({ name: `pure-${fn}-${cases.length}`, pure: { fn, args } });
  // computeLabelTransform
  for (let i = 0; i < 60; i++) {
    const bbox = { x: (rnd() - 0.5) * 200, y: (rnd() - 0.5) * 60, width: rnd() * 300, height: rnd() * 80 };
    add('computeLabelTransform', [bbox, i % 2 === 0]);
  }
  add('computeLabelTransform', [{ width: 10, height: 4 }, false]);
  add('computeLabelTransform', [{ x: 0, y: 0, width: 0, height: 0 }, true]);
  add('computeLabelTransform', [{ x: -2, y: -2, width: 37.34375, height: 23 }, false]);
  // getLabelStyles
  add('getLabelStyles', ['#undefined']);
  add('getLabelStyles', ['']);
  add('getLabelStyles', ['color:red']);
  add('getLabelStyles', [[]]);
  add('getLabelStyles', [['color:red']]);
  add('getLabelStyles', [['color:red', 'font-size:12px', '']]);
  // hasEdgeLabel
  for (const e of [{}, { label: '' }, { label: 'a' }, { startLabelLeft: 'x' }, { startLabelRight: 'x' }, { endLabelLeft: 'x' }, { endLabelRight: 'x' }, { label: '', endLabelRight: '' }]) {
    add('hasEdgeLabel', [e]);
  }
  // intersection: random and degenerate cases
  for (let i = 0; i < 300; i++) {
    const node = { x: Math.round(rnd() * 400), y: Math.round(rnd() * 300), width: 10 + Math.round(rnd() * 200), height: 10 + Math.round(rnd() * 120) };
    const inside = { x: node.x + (rnd() - 0.5) * node.width * 0.98, y: node.y + (rnd() - 0.5) * node.height * 0.98 };
    const ang = rnd() * Math.PI * 2;
    const dist = Math.hypot(node.width, node.height) * (0.6 + rnd());
    let outside = { x: node.x + Math.cos(ang) * dist, y: node.y + Math.sin(ang) * dist };
    if (i % 10 === 0) {
      outside = { x: inside.x, y: node.y + node.height };
    } else if (i % 10 === 1) {
      outside = { x: node.x - node.width, y: inside.y };
    } else if (i % 10 === 2) {
      outside = { x: node.x + node.width / 2, y: node.y + node.height };
    } else if (i % 10 === 3) {
      outside = { x: node.x + node.width, y: node.y + node.height / 2 };
    } else if (i % 10 === 4) {
      outside = { ...inside };
    }
    add('intersection', [node, outside, inside]);
  }
  // generateRoundedPath
  for (let i = 0; i < 400; i++) {
    const n = i < 8 ? i % 4 : 2 + Math.floor(rnd() * 11);
    const kind = POINT_KINDS[i % POINT_KINDS.length];
    add('generateRoundedPath', [makePoints(rnd, n, kind), i % 7 === 0 ? 1 + rnd() * 20 : 5]);
  }
  add('generateRoundedPath', [[[0, 0], [10, 0], [10, 0], [10, 10]], 5]);
  add('generateRoundedPath', [[[0, 0], [10, 0], [20, 0], [30, 0]], 5]);
  add('generateRoundedPath', [[[0, 0], [10, 0], [0, 0]], 5]);
  add('generateRoundedPath', [[[0, 0], [100, 0], [100, 100], [0, 100], [0, 1]], 5]);
  // applyMarkerOffsetsToPoints
  for (let i = 0; i < 400; i++) {
    const n = i < 6 ? i % 3 : 2 + Math.floor(rnd() * 6);
    const kind = POINT_KINDS[i % POINT_KINDS.length];
    const edge = { arrowTypeStart: ARROWS[Math.floor(rnd() * ARROWS.length)], arrowTypeEnd: ARROWS[Math.floor(rnd() * ARROWS.length)] };
    if (i % 23 === 0) {
      delete edge.arrowTypeStart;
    }
    add('applyMarkerOffsetsToPoints', [makePoints(rnd, n, kind), edge]);
  }
  // resolveEdgeLabelPosition
  for (let i = 0; i < 150; i++) {
    const orig = makePoints(rnd, 2 + Math.floor(rnd() * 6), POINT_KINDS[i % POINT_KINDS.length]);
    const upd = orig.slice(Math.floor(rnd() * 2), orig.length - Math.floor(rnd() * 2)).map(([x, y]) => [x + (rnd() < 0.3 ? rnd() * 10 : 0), y]);
    const edge = { x: rnd() * 300, y: rnd() * 300 };
    if (i % 17 === 0) {
      delete edge.x;
    }
    const mode = i % 5;
    const paths = mode === 0 ? '#undefined' : mode === 1 ? { originalPath: orig } : mode === 2 ? { updatedPath: upd } : { updatedPath: upd, originalPath: orig };
    add('resolveEdgeLabelPosition', [edge, paths]);
  }
  add('resolveEdgeLabelPosition', [{ x: 1, y: 2 }, { updatedPath: [[5, 5]], originalPath: [[1, 1]] }]);
  add('resolveEdgeLabelPosition', [{ x: 1, y: 2 }, { updatedPath: [], originalPath: [[1, 1]] }]);
  add('resolveEdgeLabelPosition', [{ x: 1, y: 2 }, { updatedPath: [[5, 5], [5, 5]], originalPath: [[1, 1], [9, 9]] }]);
  // isStraightPath, curveSupportsLineHops
  for (const d of ['', 'M0,0L10,10', 'M 0 0 L 1e-7 3', 'M0,0C1,1,2,2,3,3', 'M0,0 A6,6 0 0 1 3,3', 'm1,2 l3,4', 'M0,0Q1,1 2,2', 'M-1.5,+2 L3,4\n', 'M0,0L10,10Z', 'M0,0H10']) {
    add('isStraightPath', [d]);
  }
  for (const c of ['#undefined', '', ...CURVES, 'nope']) {
    add('curveSupportsLineHops', [c]);
  }
  // findEdgeIntersections and processEdgesWithJumps
  const geoms = (count, i) => {
    const out = [];
    for (let k = 0; k < count; k++) {
      const kind = ['orthogonal', 'free', 'horizontal', 'vertical', 'collinear', 'orthogonal'][(i + k) % 6];
      const g = { id: `e${k}`, points: makePoints(rnd, 2 + Math.floor(rnd() * 6), kind) };
      const c = (i + k) % 5;
      if (c === 0) {
        g.curve = 'rounded';
      } else if (c === 1) {
        g.curve = 'linear';
      } else if (c === 2) {
        g.curve = 'basis';
      }
      if (rnd() < 0.5) {
        g.arrowTypeEnd = ARROWS[Math.floor(rnd() * ARROWS.length)];
      }
      if (rnd() < 0.3) {
        g.arrowTypeStart = ARROWS[Math.floor(rnd() * ARROWS.length)];
      }
      out.push(g);
    }
    return out;
  };
  // A grid of long horizontal and vertical edges crosses often.
  const grid = (i) => {
    const out = [];
    const nh = 1 + Math.floor(rnd() * 3);
    const nv = 1 + Math.floor(rnd() * 4);
    for (let k = 0; k < nh; k++) {
      const y = 30 + k * (6 + rnd() * 40);
      const pts = [[0, y], [150 + rnd() * 100, y]];
      if (rnd() < 0.5) {
        pts.push([pts[1][0], y + 40 + rnd() * 40]);
      }
      out.push({ id: `h${k}`, points: rnd() < 0.3 ? pts.reverse() : pts, curve: ['rounded', 'linear', undefined][(i + k) % 3], arrowTypeEnd: 'arrow_point' });
    }
    for (let k = 0; k < nv; k++) {
      const x = 20 + k * (5 + rnd() * 30);
      const pts = [[x, 0], [x, 120 + rnd() * 80]];
      if (rnd() < 0.5) {
        pts.unshift([x - 30 - rnd() * 30, 0]);
      }
      out.push({ id: `v${k}`, points: rnd() < 0.3 ? pts.reverse() : pts, curve: ['rounded', 'linear', undefined][(i + k + 1) % 3], arrowTypeStart: rnd() < 0.3 ? 'arrow_circle' : undefined });
    }
    return JSON.parse(JSON.stringify(out));
  };
  for (let i = 0; i < 250; i++) {
    const edges = i % 2 === 0 ? grid(i) : geoms(2 + Math.floor(rnd() * 4), i);
    add('findEdgeIntersections', [edges]);
    add('processEdgesWithJumps', [edges, { enabled: i % 11 !== 0, jumpRadius: i % 7 === 0 ? 2 + rnd() * 10 : 6, jumpStyle: i % 3 === 0 ? 'gap' : 'arc' }]);
  }
  add('processEdgesWithJumps', [[{ id: 'a', points: [] }, { id: 'b', points: [[1, 2]] }], { enabled: true, jumpRadius: 6, jumpStyle: 'arc' }]);
  add('processEdgesWithJumps', [[{ id: 'a', points: [] }, { id: 'b', points: [[1, 2]] }], { enabled: false, jumpRadius: 6, jumpStyle: 'arc' }]);
  add('processEdgesWithJumps', [
    [
      { id: 'a', points: [[0, 50], [200, 50]], curve: 'rounded' },
      { id: 'b', points: [[40, 0], [40, 100]] },
      { id: 'c', points: [[46, 0], [46, 100]] },
      { id: 'd', points: [[100, 0], [100, 100]] },
      { id: 'a', points: [[0, 80], [200, 80]] },
    ],
    { enabled: true, jumpRadius: 6, jumpStyle: 'arc' },
  ]);
  return cases;
}

const LOOKS = ['classic', 'neo', 'handDrawn'];

function directiveFor(look, extra = {}) {
  return { look, theme: look === 'neo' ? 'redux-color' : 'default', ...extra };
}

function markersFor(type, look) {
  if (type === 'stateDiagram') {
    return look === 'neo' ? MARKERS.stateDiagramNeo : MARKERS.stateDiagram;
  }
  if (type === 'er') {
    return look === 'neo' ? MARKERS.erNeo : MARKERS.er;
  }
  if (type === 'requirement') {
    return look === 'neo' ? MARKERS.requirementNeo : MARKERS.requirement;
  }
  return MARKERS[type];
}

// MARKER_OF_ARROW is the marker function that draws an arrow type.
const MARKER_OF_ARROW = {
  arrow_point: 'point',
  arrow_circle: 'circle',
  arrow_cross: 'cross',
  arrow_barb: 'barb',
  arrow_barb_neo: 'barbNeo',
  arrow_hierarchy: 'hierarchy',
  aggregation: 'aggregation',
  extension: 'extension',
  composition: 'composition',
  dependency: 'dependency',
  lollipop: 'lollipop',
  only_one: 'only_one',
  zero_or_one: 'zero_or_one',
  one_or_more: 'one_or_more',
  zero_or_more: 'zero_or_more',
  requirement_arrow: 'requirement_arrow',
  requirement_contains: 'requirement_contains',
};
const NEO_MARKER = { barb: 'barbNeo', only_one: 'only_one_neo', zero_or_one: 'zero_or_one_neo', one_or_more: 'one_or_more_neo', zero_or_more: 'zero_or_more_neo', requirement_arrow: 'requirement_arrow_neo', requirement_contains: 'requirement_contains_neo' };

// markersOfEdges are the marker functions that the arrow types of the
// edges need, in the neo variant for the neo look.
function markersOfEdges(edges, look) {
  const out = [];
  for (const e of edges) {
    for (const arrow of [e.arrowTypeStart, e.arrowTypeEnd]) {
      let m = MARKER_OF_ARROW[arrow];
      if (m && look === 'neo' && NEO_MARKER[m]) {
        m = NEO_MARKER[m];
      }
      if (m && !out.includes(m)) {
        out.push(m);
      }
    }
  }
  return out;
}

// edgeScenario is one insertEdge call in a page with the markers that the
// edge needs.
function edgeScenario(name, type, look, edge, op = {}, extra = {}) {
  return {
    name,
    type,
    directive: directiveFor(look, extra.directive),
    markers: extra.markers || markersOfEdges([edge], look),
    edges: [edge],
    ops: [{ op: 'edge', edge: 0, ...op }],
  };
}

function syntheticEdgeCases() {
  const rnd = prng(0x65646732);
  const cases = [];
  const withNodes = (pts) => ({ startNode: rectNode(pts[0], 60 + Math.round(rnd() * 60), 30 + Math.round(rnd() * 30)), endNode: rectNode(pts[pts.length - 1], 60 + Math.round(rnd() * 60), 30 + Math.round(rnd() * 30)) });
  // Every marker function, for every diagram type, in themes that set
  // different theme variables. One edge of each arrow type uses them.
  for (const type of TYPES) {
    const themes = [['classic', 'default'], ['neo', 'redux-color'], ['neo', 'redux-dark'], ['handDrawn', 'forest'], ['classic', 'neutral']];
    for (const [look, theme] of type === 'flowchart-v2' ? themes : themes.slice(0, 3)) {
      const edges = ARROWS.map((arrow, k) => baseEdge(`all_${arrow}`, makePoints(rnd, 3, 'vertical'), { look, curve: 'linear', arrowTypeStart: arrow, arrowTypeEnd: arrow, style: k % 4 === 0 ? ['stroke:#abc'] : [] }));
      cases.push({
        name: `markers-all-${type}-${look}-${theme}`,
        type,
        directive: { look, theme },
        markers: [...ALL_MARKERS, ...ALL_MARKERS_NEO.filter((m) => !ALL_MARKERS.includes(m))],
        edges,
        ops: edges.map((_, k) => ({ op: 'edge', edge: k, skipIntersect: true })),
      });
    }
  }
  cases.push({ name: 'markers-unknown', type: 'flowchart-v2', directive: {}, markers: ['point', 'nope'], edges: [], ops: [] });
  // Every curve, with every kind of point list and 2 to 12 points.
  for (const curve of [...CURVES, 'nope', undefined]) {
    let k = 0;
    for (const kind of POINT_KINDS) {
      for (const n of [2, 3, 4, 5, 7, 9, 12]) {
        if ((k++ + CURVES.indexOf(curve)) % 4 !== 0 && n > 3) {
          continue;
        }
        const pts = makePoints(rnd, n, kind);
        const look = LOOKS[k % 3];
        const edge = baseEdge(`L_${curve}_${kind}_${n}`, pts, { look, curve, arrowTypeStart: k % 4 === 0 ? 'arrow_circle' : 'none', arrowTypeEnd: ['arrow_point', 'arrow_cross', 'arrow_circle', 'none'][k % 4] });
        if (curve === undefined) {
          delete edge.curve;
        }
        const skip = k % 2 === 0;
        cases.push(
          edgeScenario(`curve-${curve}-${kind}-${n}-${look}`, 'flowchart-v2', look, edge, skip ? { skipIntersect: true } : n >= 3 ? withNodes(pts) : { skipIntersect: true }, {
            directive: curve === undefined ? { flowchart: { curve: CURVES[k % CURVES.length] } } : undefined,
          })
        );
      }
    }
  }
  // Every arrow type at the start and at the end, in every look, for every
  // diagram type, with and without a stroke color.
  let a = 0;
  for (const type of TYPES) {
    for (const look of LOOKS) {
      for (const arrow of ARROWS) {
        for (const side of ['start', 'end', 'both']) {
          a++;
          if (type !== 'flowchart-v2' && a % (type === 'classDiagram' ? 2 : 4) !== 0) {
            continue;
          }
          const kind = POINT_KINDS[a % POINT_KINDS.length];
          const pts = makePoints(rnd, 2 + (a % 5), kind);
          const edge = baseEdge(`e_${arrow}_${side}`, pts, {
            look,
            curve: a % 2 === 0 ? 'rounded' : CURVES[a % CURVES.length],
            arrowTypeStart: side === 'end' ? 'none' : arrow,
            arrowTypeEnd: side === 'start' ? 'none' : arrow,
            classes: type === 'classDiagram' ? 'relation' : 'transition',
            style: a % 5 === 0 ? ['stroke:#ff0000', 'stroke-width:2px'] : a % 5 === 1 ? ['fill:none'] : [],
            pattern: ['solid', 'dotted', 'dashed'][a % 3],
          });
          cases.push(edgeScenario(`arrow-${type}-${look}-${arrow}-${side}`, type, look, edge, { skipIntersect: true }));
        }
      }
    }
  }
  // Thickness, pattern, classes, style, compiled styles, animation.
  const thicknesses = ['normal', 'thick', 'invisible', 'dotted', undefined];
  const patterns = ['solid', 'dotted', 'dashed', 'normal', undefined];
  const styles = [
    [],
    ['stroke:red'],
    ['stroke: blue', 'stroke-width:4px'],
    ['fill:none', 'stroke:#00f;stroke-width:1px'],
    'fill:none',
    '',
    undefined,
    ['stroke:rgb(10, 20, 30)'],
    ['color:red', 'stroke: '],
    ['stroke-width:3px', 'stroke:hsl(120, 50%, 50%)'],
  ];
  const compiled = [undefined, [], ['stroke:#333'], ['color:blue', 'stroke:green', 'stroke-width:2px'], ['animation: dash 1s linear infinite'], ['font-size:20px', 'fill:none']];
  let s = 0;
  for (const look of LOOKS) {
    for (const thickness of thicknesses) {
      for (const pattern of patterns) {
        s++;
        const pts = makePoints(rnd, 2 + (s % 6), POINT_KINDS[s % POINT_KINDS.length]);
        const edge = baseEdge(`t${s}`, pts, {
          look,
          thickness,
          pattern,
          curve: CURVES[s % CURVES.length],
          style: styles[s % styles.length],
          cssCompiledStyles: compiled[s % compiled.length],
          classes: [undefined, '', 'a b', 'edge-thickness-normal edge-pattern-solid flowchart-link'][s % 4],
          arrowTypeEnd: ['arrow_point', 'arrow_cross', 'arrow_circle'][s % 3],
          arrowTypeStart: ['none', 'arrow_point', 'arrow_circle', 'arrow_cross'][s % 4],
        });
        for (const k of ['thickness', 'pattern', 'style', 'cssCompiledStyles', 'classes']) {
          if (edge[k] === undefined) {
            delete edge[k];
          }
        }
        if (s % 6 === 0) {
          edge.animate = true;
        } else if (s % 6 === 1) {
          edge.animation = 'slow';
        } else if (s % 6 === 2) {
          edge.animation = 'fast';
          edge.animate = false;
        } else if (s % 6 === 3) {
          edge.animate = false;
        }
        if (s % 9 === 0) {
          edge.showPoints = true;
        }
        if (s % 13 === 0) {
          delete edge.look;
        }
        cases.push(edgeScenario(`style-${look}-${thickness}-${pattern}-${s}`, TYPES[s % TYPES.length], look, edge, { skipIntersect: true }));
      }
    }
  }
  // Style and compiled style combinations, with stroke colors and markers.
  for (const [i, style] of styles.entries()) {
    for (const [j, comp] of compiled.entries()) {
      for (const look of [LOOKS[(i + j) % 3], LOOKS[(i + j + 1) % 3]]) {
        s++;
        const pts = makePoints(rnd, 3 + (s % 3), 'free');
        const edge = baseEdge(`s${s}`, pts, { look, style, cssCompiledStyles: comp, curve: 'basis', arrowTypeStart: 'arrow_point', arrowTypeEnd: ['arrow_point', 'arrow_circle', 'arrow_cross', 'arrow_barb'][s % 4] });
        if (style === undefined) {
          delete edge.style;
        }
        if (comp === undefined) {
          delete edge.cssCompiledStyles;
        }
        cases.push(edgeScenario(`styles-${look}-${i}-${j}`, 'flowchart-v2', look, edge, withNodes(pts)));
      }
    }
  }
  // Several edges with the same stroke color share the colored marker.
  for (const look of LOOKS) {
    const edges = [];
    const ops = [];
    for (let i = 0; i < 6; i++) {
      const pts = makePoints(rnd, 3, 'vertical');
      edges.push(
        baseEdge(`m${i}`, pts, {
          look,
          curve: 'basis',
          style: [['stroke:red'], ['stroke:red'], ['stroke:#00ff00'], [], ['stroke: red'], ['stroke:red', 'stroke:blue']][i],
          arrowTypeStart: i % 2 ? 'arrow_point' : 'none',
          arrowTypeEnd: ['arrow_point', 'arrow_point', 'arrow_circle', 'arrow_point', 'arrow_cross', 'arrow_point'][i],
        })
      );
      ops.push({ op: 'edge', edge: i, skipIntersect: true });
    }
    cases.push({ name: `colored-markers-${look}`, type: 'flowchart-v2', directive: directiveFor(look), markers: MARKERS['flowchart-v2'], edges, ops });
    // Without the marker definitions, no colored copy is made.
    cases.push({ name: `colored-markers-nodefs-${look}`, type: 'flowchart-v2', directive: directiveFor(look), edges, ops });
  }
  // Clusters: the path is cut at the border of the cluster node.
  for (let i = 0; i < 90; i++) {
    const look = LOOKS[i % 3];
    const kind = ['vertical', 'free', 'horizontal', 'backward', 'orthogonal'][i % 5];
    const n = 3 + (i % 6);
    const pts = makePoints(rnd, n, kind);
    const first = pts[0];
    const last = pts[n - 1];
    const mode = i % 4; // 0: to, 1: from, 2: both, 3: from without intersect
    const edge = baseEdge(`c${i}`, pts, { look, curve: i % 3 === 0 ? 'rounded' : CURVES[i % CURVES.length] });
    const clusterDb = {};
    const size = () => ({ width: 30 + Math.round(rnd() * 120), height: 30 + Math.round(rnd() * 90) });
    if (mode === 0 || mode === 2) {
      edge.toCluster = 'to';
      clusterDb.to = { x: last[0] + (rnd() - 0.5) * 10, y: last[1] + (rnd() - 0.5) * 10, ...size() };
    }
    if (mode === 1 || mode === 2 || mode === 3) {
      edge.fromCluster = 'from';
      clusterDb.from = { x: first[0] + (rnd() - 0.5) * 10, y: first[1] + (rnd() - 0.5) * 10, ...size() };
    }
    if (i % 17 === 0) {
      // A cluster that holds every point.
      clusterDb[edge.toCluster ? 'to' : 'from'] = { x: first[0], y: first[1], width: 5000, height: 5000 };
    }
    const op = mode === 3 ? { skipIntersect: i % 2 === 0, clusterDb } : { ...withNodes(pts), clusterDb };
    cases.push(edgeScenario(`cluster-${i}-${look}-${kind}`, 'flowchart-v2', look, edge, op));
  }
  // Marker offsets: short last and first segments.
  for (let i = 0; i < 96; i++) {
    const look = LOOKS[i % 3];
    const arrow = ['arrow_point', 'aggregation', 'extension', 'composition', 'dependency', 'lollipop', 'arrow_barb_neo', 'arrow_barb'][i % 8];
    const n = 2 + (i % 4);
    const pts = makePoints(rnd, n, ['short', 'vertical', 'horizontal', 'free'][i % 4]);
    // Bring the neighbour of the end point within the marker height.
    if (i % 3 === 0 && n > 2) {
      const dx = (rnd() - 0.5) * 12;
      const dy = (rnd() - 0.5) * 12;
      pts[n - 2] = [pts[n - 1][0] + dx, pts[n - 1][1] + dy];
      pts[1] = n > 3 ? [pts[0][0] - dx, pts[0][1] + dy] : pts[1];
    }
    const edge = baseEdge(`o${i}`, pts, { look, curve: CURVES[i % CURVES.length], arrowTypeStart: i % 2 ? arrow : 'none', arrowTypeEnd: arrow });
    cases.push(edgeScenario(`offset-${i}-${look}-${arrow}`, i % 2 ? 'classDiagram' : 'flowchart-v2', look, edge, { skipIntersect: true }));
  }
  // The swimlane clipping.
  for (let i = 0; i < 60; i++) {
    const n = 2 + (i % 5);
    const pts = makePoints(rnd, n, ['orthogonal', 'vertical', 'horizontal', 'free'][i % 4]);
    const edge = baseEdge(i % 6 === 0 ? `sw${i}-to-label` : `sw${i}`, pts, { look: LOOKS[i % 3], curve: ['rounded', 'linear', 'step'][i % 3] });
    if (i % 6 === 0 || i % 7 === 0) {
      edge.isLabelEdge = true;
    }
    const nodes = {
      startNode: rectNode(pts[0], i % 5 === 0 ? 0.4 : 40, i % 5 === 0 ? 0.4 : 30),
      endNode: i % 9 === 0 ? null : rectNode(pts[n - 1], i % 4 === 0 ? 0.6 : 50, i % 4 === 0 ? 0.6 : 20),
    };
    cases.push(edgeScenario(`swimlane-${i}`, 'flowchart-v2', edge.look, edge, { ...nodes, layoutAlgorithm: i % 2 === 0 ? 'swimlane' : undefined, skipIntersect: i % 11 === 0 }, { directive: i % 2 === 1 ? { layout: 'swimlane' } : undefined }));
  }
  // Points that are not numbers, missing nodes, errors.
  const p3 = [[10, 10], [10, 50], [60, 50]];
  cases.push(edgeScenario('nan-y', 'flowchart-v2', 'classic', { ...baseEdge('n1', p3, { curve: 'basis' }), points: [{ x: 10, y: 10 }, { x: 20, y: '#NaN' }, { x: 10, y: 50 }, { x: 60, y: 50 }] }, { skipIntersect: true }));
  cases.push(edgeScenario('no-diagram-id', 'flowchart-v2', 'classic', baseEdge('n2', p3), { skipIntersect: true, noDiagramId: true }));
  cases.push(edgeScenario('missing-cluster', 'flowchart-v2', 'classic', { ...baseEdge('n3', p3), toCluster: 'nope' }, { skipIntersect: true }));
  cases.push(edgeScenario('one-node-without-intersect', 'flowchart-v2', 'classic', baseEdge('n4', p3), { startNode: rectNode(p3[0], 20, 20), endNode: null }));
  cases.push(edgeScenario('two-points-with-intersect', 'flowchart-v2', 'classic', baseEdge('n5', p3.slice(0, 2)), { startNode: rectNode(p3[0], 20, 20), endNode: rectNode(p3[1], 20, 20) }));
  cases.push(edgeScenario('empty-points', 'flowchart-v2', 'classic', baseEdge('n6', []), { skipIntersect: true }));
  cases.push(edgeScenario('empty-points-handdrawn', 'flowchart-v2', 'handDrawn', baseEdge('n7', [], { look: 'handDrawn' }), { skipIntersect: true }));
  cases.push(edgeScenario('one-point', 'flowchart-v2', 'classic', baseEdge('n8', [[5, 5]]), { skipIntersect: true }));
  cases.push(edgeScenario('one-point-rounded', 'flowchart-v2', 'neo', baseEdge('n9', [[5, 5]], { look: 'neo', curve: 'rounded' }), { skipIntersect: true }));
  cases.push(edgeScenario('other-diagram-type', 'flowchart-v2', 'neo', baseEdge('n10', p3, { look: 'neo' }), { skipIntersect: true, diagramType: 'classDiagram' }));
  // arrowMarkerAbsolute, and other config.
  for (const look of LOOKS) {
    cases.push(edgeScenario(`absolute-flowchart-${look}`, 'flowchart-v2', look, baseEdge('ab1', p3, { look, arrowTypeStart: 'arrow_circle', style: ['stroke:red'] }), { skipIntersect: true }, { directive: { flowchart: { arrowMarkerAbsolute: true } } }));
    cases.push(edgeScenario(`absolute-state-${look}`, 'stateDiagram', look, baseEdge('ab2', p3, { look, arrowTypeEnd: 'arrow_barb' }), { skipIntersect: true }, { directive: { state: { arrowMarkerAbsolute: true } } }));
    cases.push(edgeScenario(`seed-${look}`, 'flowchart-v2', look, baseEdge('sd1', makePoints(rnd, 5, 'free'), { look, curve: 'basis' }), { skipIntersect: true }, { directive: { handDrawnSeed: 7 } }));
    cases.push({
      name: `curve-op-${look}`,
      type: 'flowchart-v2',
      directive: directiveFor(look, { flowchart: { curve: 'stepAfter' } }),
      edges: [baseEdge('cv1', p3, { curve: 'linear' }), baseEdge('cv2', p3)],
      ops: [{ op: 'curve', edge: 0 }, { op: 'curve', edge: 1 }],
    });
  }
  // Hand-drawn edges draw from Math.random: several edges in one page.
  for (let i = 0; i < 12; i++) {
    const edges = [];
    const ops = [];
    for (let k = 0; k < 4; k++) {
      const pts = makePoints(rnd, 2 + ((i + k) % 6), POINT_KINDS[(i + k) % POINT_KINDS.length]);
      edges.push(
        baseEdge(`h${k}`, pts, {
          look: 'handDrawn',
          curve: CURVES[(i * 4 + k) % CURVES.length],
          style: k % 2 ? ['stroke:#123456', 'stroke-width:2px'] : [],
          arrowTypeStart: k % 3 === 0 ? 'arrow_cross' : 'none',
          classes: k % 2 ? 'x y' : undefined,
          animation: k === 3 ? 'fast' : undefined,
        })
      );
      for (const key of ['classes', 'animation']) {
        if (edges[k][key] === undefined) {
          delete edges[k][key];
        }
      }
      ops.push({ op: 'edge', edge: k, skipIntersect: true });
    }
    cases.push({ name: `handdrawn-multi-${i}`, type: 'flowchart-v2', directive: directiveFor('handDrawn', i % 3 === 0 ? { handDrawnSeed: i + 1 } : {}), markers: MARKERS['flowchart-v2'], edges, ops });
  }
  return cases;
}

const LABEL_TEXTS = [
  'yes',
  'A label',
  'This is a long edge label that has to wrap because it is wider than two hundred pixels in every font',
  'two<br/>lines',
  'with <b>bold</b> and <i>italic</i>',
  'a `code` **strong** _em_ label',
  'Ünïcödé naïve café',
  'x',
  'multi word label',
  'a < b && c > d',
  '&lt;&lt;satisfies&gt;&gt;',
  'line one\nline two',
];

function labelCases() {
  const rnd = prng(0x65646733);
  const cases = [];
  let i = 0;
  const p4 = () => makePoints(rnd, 4, 'vertical');
  // Labels: HTML and SVG, text and markdown, empty and long, styled.
  for (const type of TYPES) {
    for (const look of LOOKS) {
      for (const htmlLabels of [true, false]) {
        for (const [t, text] of [...LABEL_TEXTS, ''].entries()) {
          i++;
          if (type !== 'flowchart-v2' && i % 4 !== 0) {
            continue;
          }
          const pts = p4();
          const edge = baseEdge(`lbl${i}`, pts, {
            look,
            label: text,
            labelType: i % 3 === 0 ? 'markdown' : i % 3 === 1 ? 'text' : 'string',
            labelStyle: [[], ['color:red'], ['font-size:20px', 'color: blue'], ['background:#eee', 'stroke:red', 'fill:yellow'], ['font-weight:bold', 'font-style:italic']][i % 5],
            cssCompiledStyles: i % 7 === 0 ? ['color:green', 'stroke:red'] : [],
            x: Math.round(rnd() * 300),
            y: Math.round(rnd() * 300),
          });
          if (i % 11 === 0) {
            edge.cssStyles = ['font-family:courier', 'stroke-width:2px'];
          }
          cases.push({
            name: `label-${type}-${look}-${htmlLabels ? 'html' : 'svg'}-${t}`,
            type,
            directive: directiveFor(look, { htmlLabels, ...(i % 13 === 0 ? { flowchart: { subGraphTitleMargin: { top: 10, bottom: 5 } } } : {}) }),
            markers: markersFor(type, look),
            edges: [edge],
            ops: [{ op: 'label', edge: 0 }, { op: 'edge', edge: 0, skipIntersect: true }, { op: 'position', edge: 0, paths: { fromOp: 1 } }],
          });
        }
      }
    }
  }
  // Terminal labels.
  const terminalSets = [
    { startLabelLeft: '1' },
    { startLabelRight: '0..*' },
    { endLabelLeft: 'many' },
    { endLabelRight: '1..n' },
    { startLabelLeft: 'a', startLabelRight: 'b', endLabelLeft: 'c', endLabelRight: 'd' },
    { startLabelRight: '1', endLabelLeft: '*' },
    { startLabelLeft: 'a long terminal label', endLabelRight: 'x' },
    { startLabelRight: '', endLabelLeft: '' },
    { startLabelLeft: 'two<br/>lines', endLabelLeft: '<b>bold</b>' },
  ];
  for (const look of LOOKS) {
    for (const htmlLabels of [true, false]) {
      for (const [t, terms] of terminalSets.entries()) {
        for (const withLabel of [true, false]) {
          i++;
          const pts = makePoints(rnd, 3 + (i % 4), POINT_KINDS[i % 4]);
          const edge = baseEdge(`term${i}`, pts, {
            look,
            classes: 'relation',
            label: withLabel ? LABEL_TEXTS[i % LABEL_TEXTS.length] : '',
            labelType: 'markdown',
            labelStyle: i % 3 === 0 ? ['color:red'] : '',
            style: '',
            arrowTypeStart: ['none', 'extension', 'aggregation', ''][i % 4],
            arrowTypeEnd: ['none', 'composition', 'dependency', 'lollipop'][i % 4],
            ...terms,
            x: Math.round(rnd() * 300),
            y: Math.round(rnd() * 300),
          });
          const pathsModes = [{ fromOp: 1 }, { originalPath: pts }, { updatedPath: pts.slice(1), originalPath: pts }, {}];
          cases.push({
            name: `terminal-${look}-${htmlLabels ? 'html' : 'svg'}-${t}-${withLabel ? 'label' : 'nolabel'}`,
            type: 'classDiagram',
            directive: directiveFor(look, { htmlLabels }),
            markers: MARKERS.classDiagram,
            edges: [edge],
            ops: [{ op: 'label', edge: 0 }, { op: 'edge', edge: 0, skipIntersect: true }, { op: 'position', edge: 0, paths: pathsModes[i % pathsModes.length] }],
          });
        }
      }
    }
  }
  // Several edges: one call after another, and all together (Promise.all).
  for (const look of LOOKS) {
    for (const htmlLabels of [true, false]) {
      for (const batch of [true, false]) {
        for (let v = 0; v < 4; v++) {
          i++;
          const edges = [];
          const count = 3 + (v % 3);
          for (let k = 0; k < count; k++) {
            const pts = makePoints(rnd, 3, 'vertical');
            const terms = terminalSets[(v * 3 + k * 2 + (k === 1 ? 7 : 0)) % terminalSets.length];
            edges.push(
              baseEdge(`b${k}`, pts, {
                look,
                classes: 'relation',
                label: k % 2 === (v % 2) ? LABEL_TEXTS[(i + k) % LABEL_TEXTS.length] : '',
                labelType: k % 2 ? 'markdown' : 'text',
                labelStyle: k === 0 ? ['color:red'] : [],
                ...(k % 3 === v % 3 ? {} : terms),
                x: 50 * k,
                y: 20 * k,
              })
            );
          }
          if (v === 3) {
            // A label with an icon awaits more often than the others
            // (replaceIconSubstring), so it changes the turns.
            edges[1].label = 'fa:fa-car by car';
            edges[1].startLabelLeft = 'fa:fa-user 1';
            edges[2].endLabelRight = 'x fa:fa-bell fa:fa-ban';
          }
          const idx = edges.map((_, k) => k);
          const ops = batch ? [{ op: 'labels', edges: idx }] : idx.map((k) => ({ op: 'label', edge: k }));
          for (const k of idx) {
            ops.push({ op: 'edge', edge: k, skipIntersect: true });
            ops.push({ op: 'position', edge: k, paths: { fromOp: ops.length - 1 } });
          }
          cases.push({ name: `multi-${look}-${htmlLabels ? 'html' : 'svg'}-${batch ? 'together' : 'sequence'}-${v}`, type: 'classDiagram', directive: directiveFor(look, { htmlLabels }), markers: MARKERS.classDiagram, edges, ops });
        }
      }
    }
  }
  // Label positions without a label element, without paths, twice, cleared.
  const e1 = () => baseEdge('p1', p4(), { label: 'pos', x: 10, y: 20, startLabelLeft: 's', endLabelRight: 'e' });
  cases.push({ name: 'position-without-insert', type: 'flowchart-v2', directive: {}, edges: [e1()], ops: [{ op: 'position', edge: 0, paths: {} }] });
  cases.push({ name: 'position-null-paths', type: 'flowchart-v2', directive: {}, edges: [e1()], ops: [{ op: 'label', edge: 0 }, { op: 'position', edge: 0, paths: null }] });
  cases.push({ name: 'position-empty-paths', type: 'flowchart-v2', directive: {}, edges: [e1()], ops: [{ op: 'label', edge: 0 }, { op: 'position', edge: 0, paths: {} }] });
  cases.push({ name: 'position-no-xy', type: 'flowchart-v2', directive: {}, edges: [{ ...e1(), x: undefined, y: undefined }].map((e) => JSON.parse(JSON.stringify(e))), ops: [{ op: 'label', edge: 0 }, { op: 'position', edge: 0, paths: {} }] });
  cases.push({ name: 'position-no-xy-updated', type: 'flowchart-v2', directive: {}, edges: [{ ...e1(), x: undefined, y: undefined }].map((e) => JSON.parse(JSON.stringify(e))), ops: [{ op: 'label', edge: 0 }, { op: 'position', edge: 0, paths: { updatedPath: [[0, 0], [0, 100]], originalPath: [[0, 0], [0, 50]] } }] });
  cases.push({ name: 'position-short-path', type: 'flowchart-v2', directive: {}, edges: [e1()], ops: [{ op: 'label', edge: 0 }, { op: 'position', edge: 0, paths: { originalPath: [[0, 0], [0, 10]] } }] });
  cases.push({ name: 'label-twice', type: 'flowchart-v2', directive: {}, edges: [e1()], ops: [{ op: 'label', edge: 0 }, { op: 'label', edge: 0 }, { op: 'position', edge: 0, paths: { originalPath: [[0, 0], [0, 100]] } }] });
  cases.push({ name: 'label-clear-position', type: 'flowchart-v2', directive: {}, edges: [e1()], ops: [{ op: 'label', edge: 0 }, { op: 'clear' }, { op: 'position', edge: 0, paths: { originalPath: [[0, 0], [0, 100]] } }] });
  cases.push({ name: 'label-string-style', type: 'flowchart-v2', directive: {}, edges: [{ ...e1(), labelStyle: 'color: red; font-size:18px;;stroke:blue' }], ops: [{ op: 'label', edge: 0 }] });
  return cases;
}

function lineJumpCases() {
  const rnd = prng(0x65646734);
  const cases = [];
  for (let i = 0; i < 90; i++) {
    const look = LOOKS[i % 3];
    const edges = [];
    const nh = 1 + (i % 3);
    const nv = 1 + (i % 4);
    const curveOf = (k) => ['rounded', 'linear', undefined, 'basis', 'step'][(i + k) % 5];
    for (let k = 0; k < nh; k++) {
      const y = 40 + k * (8 + rnd() * 40);
      const pts = [[10, y], [200 + rnd() * 60, y]];
      if (rnd() < 0.5) {
        pts.push([pts[1][0], y + 50 + rnd() * 30]);
      }
      edges.push(baseEdge(`h${k}`, rnd() < 0.3 ? pts.reverse() : pts, { look, curve: curveOf(k), pattern: k % 2 ? 'dotted' : 'solid', arrowTypeStart: k % 2 ? 'arrow_circle' : 'none' }));
    }
    for (let k = 0; k < nv; k++) {
      const x = 30 + k * (6 + rnd() * 35);
      const pts = [[x, 5], [x, 150 + rnd() * 60]];
      if (rnd() < 0.5) {
        pts.unshift([x - 30 - rnd() * 30, 5]);
      }
      edges.push(baseEdge(`v${k}`, rnd() < 0.3 ? pts.reverse() : pts, { look, curve: curveOf(k + 2), arrowTypeEnd: ['arrow_point', 'arrow_cross', 'none'][k % 3] }));
    }
    for (const e of edges) {
      if (e.curve === undefined) {
        delete e.curve;
      }
    }
    const ops = edges.map((_, k) => ({ op: 'edge', edge: k, skipIntersect: true }));
    const geoms = edges.map((e) => {
      const g = { id: e.id, points: e.points.map((p) => [p.x, p.y]), arrowTypeStart: e.arrowTypeStart, arrowTypeEnd: e.arrowTypeEnd };
      if (e.curve !== undefined) {
        g.curve = e.curve;
      }
      return g;
    });
    if (i % 10 === 0) {
      geoms.push({ id: 'missing', points: [[0, 0], [500, 500]] });
    }
    ops.push({ op: 'lineJumps', edges: geoms, config: { enabled: i % 13 !== 0, jumpRadius: 6, jumpStyle: i % 4 === 0 ? 'gap' : 'arc' } });
    cases.push({ name: `linejumps-${i}-${look}`, type: 'flowchart-v2', directive: directiveFor(look, i % 5 === 0 ? { flowchart: { curve: 'linear' } } : {}), markers: MARKERS['flowchart-v2'], edges, ops });
  }
  return cases;
}

// ---- realistic cases from the spy ----

const hasEdgeLabel = (e) => Boolean(e.label || e.startLabelLeft || e.startLabelRight || e.endLabelLeft || e.endLabelRight);

// usable reports whether a spied edge can be replayed: its values must
// survive JSON (no NaN or missing coordinates).
function usable(rec) {
  if (!rec.edge || !Array.isArray(rec.edge.points) || !rec.dataPoints) {
    return false;
  }
  const s = JSON.stringify(rec.edge);
  return !s.includes('#num:') && rec.edge.points.every((p) => typeof p.x === 'number' && typeof p.y === 'number');
}

function corpusCases(spied, prefix, maxPerSignature, maxCases) {
  const bySig = new Map();
  const cases = [];
  for (const fx of spied) {
    if (fx.error || fx.skipped || !fx.edges) {
      continue;
    }
    const skipIntersect = fx.layout !== 'dagre';
    if (fx.layout !== 'dagre' && !fx.layout.startsWith('elk')) {
      continue;
    }
    for (const rec of fx.edges) {
      if (!usable(rec)) {
        continue;
      }
      const e = rec.edge;
      const sig = [fx.type, fx.layout, e.look, e.curve, e.arrowTypeStart, e.arrowTypeEnd, e.pattern, e.thickness, e.points.length, hasEdgeLabel(e), Boolean(e.toCluster), Boolean(e.fromCluster), JSON.stringify(e.style), e.classes, e.labelType, e.animate, e.animation].join('|');
      // Edges at cluster borders and self loops are few: take them all.
      const rare = Boolean(e.toCluster || e.fromCluster || (e.start !== undefined && e.start === e.end));
      const n = bySig.get(sig) || 0;
      if (n >= maxPerSignature && !rare) {
        continue;
      }
      bySig.set(sig, n + 1);
      const op = { op: 'edge', edge: 0, layoutAlgorithm: fx.layoutAlgorithm };
      if (skipIntersect) {
        op.skipIntersect = true;
      } else {
        const dp = rec.dataPoints;
        op.startNode = { fixed: [dp[0].x, dp[0].y] };
        op.endNode = { fixed: [dp[dp.length - 1].x, dp[dp.length - 1].y] };
      }
      if (rec.clusterDb && Object.keys(rec.clusterDb).length > 0) {
        op.clusterDb = rec.clusterDb;
      }
      if (e.look === 'handDrawn' && rec.randomAt !== undefined) {
        op.randomAt = rec.randomAt;
      }
      const ops = [];
      if (hasEdgeLabel(e)) {
        ops.push({ op: 'label', edge: 0 });
      }
      ops.push(op);
      if (hasEdgeLabel(e)) {
        ops.push({ op: 'position', edge: 0, paths: { fromOp: ops.length - 1 } });
      }
      cases.push({
        name: `${prefix}:${fx.name}:${rec.id}`,
        type: fx.type,
        id: fx.diagramId,
        directive: { ...fx.directive, layout: fx.layout === 'dagre' ? 'dagre' : fx.directive.layout },
        markers: fx.markers,
        edges: [e],
        ops,
        spied: rec.html,
        rare,
      });
    }
  }
  const strip = (c) => {
    delete c.rare;
    return c;
  };
  const rareCases = cases.filter((c) => c.rare);
  const common = cases.filter((c) => !c.rare);
  const room = Math.max(maxCases - rareCases.length, 0);
  if (common.length > room) {
    const step = common.length / room;
    return [...Array.from({ length: room }, (_, i) => common[Math.floor(i * step)]), ...rareCases].map(strip);
  }
  return cases.map(strip);
}

// lineJumpCorpusCases are whole diagrams of the ELK layout whose rendered
// paths have line hops: every edge is drawn in the order of the layout
// data, and then the line hops are applied, as applyElkLineJumps does.
function lineJumpCorpusCases(spied, prefix, maxEdges, maxCases) {
  const cases = [];
  for (const fx of spied) {
    if (fx.error || fx.skipped || !fx.edges || !fx.layout.startsWith('elk') || fx.elkLineHops === false) {
      continue;
    }
    // A hop is an arc, or a second move for the gap style. A hand-drawn
    // path has moves of its own.
    const isHop = (rec) => {
      const d = / d="([^"]*)"/.exec(rec.html)?.[1] ?? '';
      return / A[\d.]+,[\d.]+ 0 0 [01] /.test(d) || (fx.elkLineHops === 'gap' && rec.edge?.look !== 'handDrawn' && / M/.test(d));
    };
    const hops = fx.edges.filter(isHop).length;
    if (hops === 0 || fx.edges.length > maxEdges || !fx.edges.every(usable)) {
      continue;
    }
    const byId = new Map(fx.edges.map((rec) => [rec.id, rec]));
    const order = fx.edgeOrder.filter((o) => !o.isLayoutOnly);
    if (order.length !== fx.edges.length || !order.every((o) => byId.has(o.id)) || new Set(order.map((o) => o.id)).size !== order.length) {
      continue;
    }
    const recs = order.map((o) => byId.get(o.id));
    const edges = recs.map((rec) => rec.edge);
    const ops = recs.map((rec, k) => {
      const op = { op: 'edge', edge: k, skipIntersect: true, layoutAlgorithm: fx.layoutAlgorithm };
      if (rec.edge.look === 'handDrawn' && rec.randomAt !== undefined) {
        op.randomAt = rec.randomAt;
      }
      return op;
    });
    ops.push({
      op: 'lineJumps',
      edges: edges
        .filter((e) => Array.isArray(e.points) && e.points.length >= 2)
        .map((e) => {
          const g = { id: e.id, points: e.points.map((p) => [p.x, p.y]) };
          for (const k of ['curve', 'arrowTypeStart', 'arrowTypeEnd']) {
            if (typeof e[k] === 'string') {
              g[k] = e[k];
            }
          }
          return g;
        }),
      config: { enabled: true, jumpRadius: 6, jumpStyle: fx.elkLineHops === 'gap' ? 'gap' : 'arc' },
    });
    cases.push({ name: `${prefix}:${fx.name}`, type: fx.type, id: fx.diagramId, directive: fx.directive, markers: fx.markers, edges, ops, spied: recs.map((rec) => rec.html), hops });
  }
  // Prefer the diagrams with the most hops for their size.
  cases.sort((a, b) => b.hops / b.edges.length - a.hops / a.edges.length || (a.name < b.name ? -1 : 1));
  return cases.slice(0, maxCases).map((c) => {
    delete c.hops;
    return c;
  });
}

// checkReplay compares the path that the replay made with the path that the
// real render made, and drops the cases that differ: their inputs are not
// the true inputs of insertEdge.
function checkReplay(golden) {
  const attr = (html, name) => {
    const m = new RegExp(` ${name}="([^"]*)"`).exec(html || '');
    return m ? m[1] : undefined;
  };
  let same = 0;
  const kept = [];
  for (const g of golden) {
    const spied = Array.isArray(g.spied) ? g.spied : [g.spied];
    delete g.spied;
    const pathsHtml = /<g class="edgePaths">(.*?)<\/g><g class="edgeLabels">/s.exec(g.html || '')?.[1] ?? '';
    const paths = pathsHtml.split('</path>').filter((p) => p.startsWith('<path'));
    // The final sanitizing of mermaid.render trims the class attribute.
    const sameAttr = (name, a, b) => (name === 'class' ? (a ?? '').trim() === (b ?? '').trim() : a === b);
    const ok =
      !g.randomGap &&
      paths.length === spied.length &&
      spied.every((sp, i) => ['d', 'class', 'style', 'marker-start', 'marker-end', 'data-points'].every((name) => sameAttr(name, attr(sp, name), attr(paths[i], name))));
    if (ok) {
      same++;
      kept.push(g);
    } else if (process.env.MMDG_DROPPED) {
      // For looking into the cases that do not reproduce.
      const diffs = [];
      spied.forEach((sp, i) => {
        for (const name of ['d', 'class', 'style', 'marker-start', 'marker-end', 'data-points']) {
          if (!sameAttr(name, attr(sp, name), attr(paths[i], name))) {
            diffs.push({ path: i, name, spied: attr(sp, name), replay: attr(paths[i], name) });
          }
        }
      });
      fs.appendFileSync(process.env.MMDG_DROPPED, JSON.stringify({ name: g.name, paths: paths.length, spied: spied.length, look: g.edges[0]?.look, diffs: diffs.slice(0, 4) }) + '\n');
    }
  }
  console.error(`replay of spied edges: ${same} of ${golden.length} reproduce the rendered path; the others are dropped`);
  return kept;
}

function writeGolden(outFile, obj) {
  const json = JSON.stringify(obj);
  fs.writeFileSync(outFile, outFile.endsWith('.gz') ? zlib.gzipSync(json, { level: 9 }) : json);
}

function readGolden(file) {
  const buf = fs.readFileSync(file);
  return JSON.parse(file.endsWith('.gz') ? zlib.gunzipSync(buf).toString('utf8') : buf.toString('utf8'));
}

// all writes every golden file. The environment variable MMDG_ONLY limits
// it to the synthetic cases ("synthetic") or to the corpus cases
// ("corpus"). MMDG_SPY_DIR names a directory that keeps the spy results:
// a result that is there is used again, and the corpus is not rendered.
function all(outDir) {
  fs.mkdirSync(outDir, { recursive: true });
  const only = process.env.MMDG_ONLY;
  if (only !== 'corpus') {
    runGolden(pureCases(), path.join(outDir, 'pure.json.gz'));
    runGolden(syntheticEdgeCases(), path.join(outDir, 'edges_synthetic.json.gz'));
    runGolden(labelCases(), path.join(outDir, 'labels.json.gz'));
    runGolden(lineJumpCases(), path.join(outDir, 'linejumps.json.gz'));
  }
  if (only === 'synthetic') {
    return;
  }
  const others = ['class-diagram', 'state-diagram-v2', 'er-diagram', 'requirement'];
  const passes = [
    ['corpus_flowchart_elk', ['flowchart'], undefined, 0, 2, 450],
    ['corpus_flowchart_dagre', ['flowchart'], 'dagre', 0, 2, 450],
    ['corpus_other_elk', others, undefined, 240, 2, 300],
    ['corpus_other_dagre', others, 'dagre', 240, 2, 300],
  ];
  const checked = (cases, name) => {
    const file = path.join(outDir, `${name}.json.gz`);
    runGolden(cases, file);
    const golden = readGolden(file);
    golden.cases = checkReplay(golden.cases);
    writeGolden(file, golden);
    console.error(`${golden.cases.length} cases -> ${file} (${fs.statSync(file).size} bytes)`);
  };
  for (const [name, dirs, layout, limit, perSig, maxCases] of passes) {
    const spyFile = process.env.MMDG_SPY_DIR && path.join(process.env.MMDG_SPY_DIR, `${name}.spy.json`);
    let spied;
    if (spyFile && fs.existsSync(spyFile)) {
      spied = JSON.parse(fs.readFileSync(spyFile, 'utf8'));
    } else {
      spied = spy(dirs, { layout, limit });
      if (spyFile) {
        fs.writeFileSync(spyFile, JSON.stringify(spied));
      }
    }
    checked(corpusCases(spied, name, perSig, maxCases), name);
    if (name === 'corpus_flowchart_elk') {
      checked(lineJumpCorpusCases(spied, 'corpus_linejumps', 30, 40), 'corpus_linejumps');
    }
  }
}

const [command, ...rest] = process.argv.slice(2);
if (command === 'all') {
  all(rest[0] || path.join(repo, 'internal/ru/edges/testdata'));
} else if (command === 'spy') {
  const out = rest.shift();
  const opts = {};
  const dirs = [];
  while (rest.length > 0) {
    const a = rest.shift();
    if (a === '--layout') {
      opts.layout = rest.shift();
    } else if (a === '--limit') {
      opts.limit = Number(rest.shift());
    } else {
      dirs.push(a);
    }
  }
  fs.writeFileSync(out, JSON.stringify(spy(dirs, opts)));
} else if (command === 'golden') {
  const { cases } = JSON.parse(fs.readFileSync(rest[0], 'utf8'));
  runGolden(cases, rest[1]);
} else if (command === 'cases') {
  // Writes the synthetic case files, for inspection.
  const outDir = rest[0];
  fs.mkdirSync(outDir, { recursive: true });
  for (const [name, cases] of [['pure', pureCases()], ['edges_synthetic', syntheticEdgeCases()], ['labels', labelCases()], ['linejumps', lineJumpCases()]]) {
    fs.writeFileSync(path.join(outDir, `${name}.cases.json`), JSON.stringify({ cases }));
    console.error(`${name}: ${cases.length} cases`);
  }
} else {
  console.error('usage: node gen-edges.mjs all [outDir] | spy <out.json> [--layout dagre] [--limit N] <corpus dir>... | golden <cases.json> <golden.json[.gz]> | cases <outDir>');
  process.exit(2);
}
