// Runs the layout scaffolding cases through upstream mermaid in Chromium
// and writes the golden file for internal/ru/graph.
//
//   node gen-graph.mjs [golden.json.gz]
//
// The default output is internal/ru/graph/testdata/graph.json.gz.
//
// The cases come from two sources:
//
//   1. Real layout data: probes/graph-capture.js builds the layout data of
//      small corpus fixtures (flowcharts, state, class and ER diagrams).
//   2. Made-up layout data with rect nodes, groups and edges, in every look,
//      with HTML and SVG labels.
//
// probes/graph.js then calls the real createLayoutElementGroups,
// measureGroupLabel, insertMeasuredNode, createGraphWithElements,
// terminalLabelTranslate, getSubGraphTitleMargins and
// createCommonLayoutRenderer for the cases. The golden file is
// { baseConfig, configs, css, cases }: each case has the indexes of its
// effective config and style sheet in the two lists, and a config is a
// patch of baseConfig (see gen-clusters.mjs).
//
// The script also prints how often the Promise.all of
// createGraphWithElements changes the result, measured over every corpus
// fixture with the handDrawn look.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const corpus = path.join(root, 'testdata/corpus/mermaid-e2e');
const outFile = path.resolve(process.argv[2] || path.join(root, 'internal/ru/graph/testdata/graph.json.gz'));

// A seeded generator, so that the cases are the same on every run.
let seed = 0x67726166;
const rnd = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const between = (lo, hi) => lo + rnd() * (hi - lo);
const coarse = (lo, hi) => Math.round(between(lo, hi) * 64) / 64;
const choose = (list) => list[Math.floor(rnd() * list.length)];

const oracle = (probe, args) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-graph-'));
  const argsFile = path.join(dir, 'args.json');
  const resultFile = path.join(dir, 'out.json');
  fs.writeFileSync(argsFile, JSON.stringify(args));
  execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes', probe), '--args', argsFile, '--out', resultFile], { stdio: 'inherit' });
  const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
  fs.rmSync(dir, { recursive: true });
  return result;
};

// ---- 1. Real layout data from corpus fixtures ----

const walk = (dir) => {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...walk(p));
    } else if (entry.name.endsWith('.mmd')) {
      out.push(p);
    }
  }
  return out;
};
const families = ['flowchart', 'state-diagram-v2', 'class-diagram', 'er-diagram', 'requirement'];
const all = [];
for (const family of families) {
  const dir = path.join(corpus, family);
  if (fs.existsSync(dir)) {
    for (const file of walk(dir)) {
      all.push({ family, name: path.relative(corpus, file), text: fs.readFileSync(file, 'utf8') });
    }
  }
}
// pick returns up to n of the list, spread evenly.
const pick = (list, n) => {
  const out = [];
  for (let i = 0; i < Math.min(n, list.length); i++) {
    out.push(list[Math.floor((i * list.length) / Math.min(n, list.length))]);
  }
  return out;
};
const isHand = (f) => /handDrawn/.test(f.text);
const small = (f) => f.text.length < 500;
const selected = [
  ...pick(all.filter((f) => f.family === 'flowchart' && small(f) && !isHand(f) && /subgraph/.test(f.text)), 7),
  ...pick(all.filter((f) => f.family === 'flowchart' && small(f) && !isHand(f) && !/subgraph/.test(f.text)), 7),
  ...pick(all.filter((f) => f.family === 'state-diagram-v2' && small(f) && !isHand(f)), 6),
  ...pick(all.filter((f) => f.family === 'class-diagram' && small(f) && !isHand(f)), 3),
  ...pick(all.filter((f) => f.family === 'er-diagram' && small(f) && !isHand(f)), 3),
  ...pick(all.filter((f) => f.family === 'requirement' && small(f) && !isHand(f)), 2),
  ...pick(all.filter((f) => isHand(f) && f.text.length < 300), 5),
];
const handDrawn = all.filter(isHand);
const captured = oracle('graph-capture.js', { fixtures: [...new Set([...selected, ...handDrawn])].map(({ name, text }) => ({ name, text })) });
const byFixture = new Map(captured.map((c) => [c.fixture, c]));
const samples = {};
for (const c of captured) {
  if (!c.error) {
    samples[c.type] ||= all.find((f) => f.name === c.fixture).text;
  }
}

const cases = [];
const add = (c) => cases.push(c);
for (const f of [...new Set(selected)]) {
  const c = byFixture.get(f.name);
  if (!c || c.error || c.data.nodes.length > 12 || c.data.edges.length > 14) {
    continue;
  }
  // A hachure fill of the handDrawn look is large: only small diagrams.
  if (isHand(f) && c.data.nodes.length > 5) {
    continue;
  }
  // A layout with groups is measured as the ELK layout does, the others as
  // the default.
  const options = c.data.nodes.some((n) => n.isGroup) ? { unwrapGroupLabels: true } : undefined;
  add({ kind: 'createGraph', name: `corpus/${c.fixture}`, type: c.type, directive: c.directive, data: c.data, options });
}
// Layouts for which the Promise.all of createGraphWithElements changes the
// result (see the statistics at the end): composite states with the
// handDrawn look. Their start and end states draw without an await, before
// the states that wait for a label.
for (const f of all.filter((f) => /composite-containers-handdrawn-look-redux-color/.test(f.name))) {
  const c = byFixture.get(f.name);
  if (c && !c.error) {
    add({ kind: 'createGraph', name: `corpus/${c.fixture}`, type: c.type, directive: c.directive, data: c.data, options: { unwrapGroupLabels: true } });
  }
}
const corpusCount = cases.length;

// ---- 2. Made-up layout data ----

const texts = ['A', 'Start', 'A longer label for a node', 'Two<br/>lines', 'This is **bold** and *italic*', 'Größe 日本語', ''];
// A hachure fill of the handDrawn look has a path for every 1.5 px of the
// box. The nodes of the handDrawn look are small, for the size of the
// golden file: a small padding and no minimum width.
const makeNode = (id, look, extra = {}) => ({
  id,
  label: id,
  labelType: 'text',
  labelStyle: '',
  padding: look === 'handDrawn' ? 3 : 15,
  cssStyles: [],
  cssCompiledStyles: [],
  cssClasses: 'default ',
  domId: `my-svg-flowchart-${id}-0`,
  look,
  isGroup: false,
  shape: 'squareRect',
  ...(look === 'handDrawn' ? {} : { minWidth: 120 }),
  ...extra,
});
const makeGroup = (id, look, extra = {}) => ({
  id,
  label: `Group ${id}`,
  labelStyle: '',
  labelType: 'text',
  padding: 8,
  cssCompiledStyles: [],
  cssClasses: '',
  shape: 'rect',
  isGroup: true,
  look,
  colorIndex: 0,
  domId: `my-svg-${id}`,
  ...extra,
});
const makeEdge = (start, end, i, look, extra = {}) => ({
  id: `L_${start}_${end}_${i}`,
  isUserDefinedId: false,
  start,
  end,
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
  pattern: 'normal',
  look,
  curve: 'basis',
  ...extra,
});
const looks = ['classic', 'neo', 'handDrawn'];
const directiveOf = (look, html) => ({ look, theme: look === 'neo' ? 'redux-color' : 'default', htmlLabels: html, flowchart: { htmlLabels: html } });

// createLayoutElementGroups.
add({ kind: 'groups', name: 'groups/no-options', options: null });
add({ kind: 'groups', name: 'groups/empty-options', options: {} });
add({ kind: 'groups', name: 'groups/edge-paths-class', options: { edgePathsClass: 'edgePaths' } });

// measureGroupLabel.
for (const html of [true, false]) {
  for (const [li, label] of [...texts, undefined].entries()) {
    for (const width of [undefined, 60, 300]) {
      for (const measureWidth of [undefined, 'Infinity', 80]) {
        const node = makeGroup('G', 'neo', { label, width, labelType: li === 4 ? 'markdown' : 'text' });
        if (label === undefined) {
          delete node.label;
        }
        if (width === undefined) {
          delete node.width;
        }
        const c = { kind: 'measureGroup', name: `measureGroup/${html ? 'html' : 'svg'}/label-${li}/width-${width}/measure-${measureWidth}`, type: 'flowchart-v2', directive: directiveOf('neo', html), node };
        if (measureWidth !== undefined) {
          c.measureWidth = measureWidth;
        }
        add(c);
      }
    }
  }
}
add({ kind: 'measureGroup', name: 'measureGroup/label-style', type: 'flowchart-v2', directive: {}, node: makeGroup('G', 'neo', { labelStyle: 'font-size:24px;font-weight:bold' }) });
add({ kind: 'measureGroup', name: 'measureGroup/wrapping-width', type: 'flowchart-v2', directive: { flowchart: { wrappingWidth: 50 } }, node: makeGroup('G', 'neo', { label: 'A title that wraps at the wrapping width' }) });
add({ kind: 'measureGroup', name: 'measureGroup/node-wrapping-width', type: 'flowchart-v2', directive: {}, node: makeGroup('G', 'neo', { label: 'A title that wraps at the wrapping width', wrappingWidth: 70 }) });
add({ kind: 'measureGroup', name: 'measureGroup/state', type: 'stateDiagram', directive: {}, node: { labelStyle: '', shape: 'roundedWithTitle', label: 'Composite state', cssClasses: ' statediagram-state statediagram-cluster ', cssCompiledStyles: [], cssStyles: [], id: 'Comp', dir: 'TB', domId: 'my-svg-state-Comp-1', type: 'group', isGroup: true, colorIndex: 0, padding: 8, rx: 10, ry: 10, look: 'neo', labelType: 'markdown', centerLabel: true, wrappingWidth: 120 } });
add({ kind: 'measureGroup', name: 'measureGroup/class', type: 'classDiagram', directive: {}, node: { id: 'Auth', label: 'Authentication Service', isGroup: true, padding: 16, shape: 'rect', cssStyles: [], look: 'neo', domId: 'my-svg-Auth' } });

// insertMeasuredNode.
for (const look of looks) {
  for (const html of [true, false]) {
    for (const [li, label] of texts.entries()) {
      if (look === 'handDrawn' && label.length > 12) {
        continue;
      }
      const extra = { label, labelType: li === 4 ? 'markdown' : 'text' };
      add({ kind: 'measuredNode', name: `measuredNode/${look}/${html ? 'html' : 'svg'}/label-${li}`, type: 'flowchart-v2', directive: directiveOf(look, html), node: makeNode('N', look, extra) });
    }
    add({ kind: 'measuredNode', name: `measuredNode/${look}/${html ? 'html' : 'svg'}/rounded`, type: 'flowchart-v2', directive: directiveOf(look, html), node: makeNode('N', look, { shape: 'roundedRect' }), dir: 'LR' });
    add({ kind: 'measuredNode', name: `measuredNode/${look}/${html ? 'html' : 'svg'}/rect-rx`, type: 'stateDiagram', directive: directiveOf(look, html), node: makeNode('N', look, { shape: 'rect', rx: 10, ry: 10, padding: 8, cssClasses: ' statediagram-state' }), dir: 'TB' });
    add({ kind: 'measuredNode', name: `measuredNode/${look}/${html ? 'html' : 'svg'}/link`, type: 'flowchart-v2', directive: directiveOf(look, html), node: makeNode('N', look, { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'A tip', haveCallback: true }) });
    add({ kind: 'measuredNode', name: `measuredNode/${look}/${html ? 'html' : 'svg'}/styles`, type: 'flowchart-v2', directive: directiveOf(look, html), node: makeNode('N', look, { cssStyles: ['fill:#f9f', 'stroke:#333', 'stroke-width:4px'], labelStyle: 'color:#f00' }) });
  }
}
add({ kind: 'measuredNode', name: 'measuredNode/unknown-shape', type: 'flowchart-v2', directive: {}, node: makeNode('N', 'neo', { shape: 'noSuchShape' }) });
add({ kind: 'measuredNode', name: 'measuredNode/no-shape', type: 'flowchart-v2', directive: {}, node: makeNode('N', 'neo', { shape: undefined }) });

// Layout data for createGraphWithElements and for the layouts.
const makeData = (look, options = {}) => {
  const n = options.nodes ?? 4;
  const nodes = [];
  const edges = [];
  if (options.groups) {
    nodes.push(makeGroup('G1', look, options.groupDir ? { dir: options.groupDir } : {}));
    nodes.push(makeGroup('G2', look, { parentId: 'G1', label: options.groupLabel ?? (look === 'handDrawn' ? 'Inner' : 'An inner group with a long title to measure'), labelType: options.groupMarkdown ? 'markdown' : 'text' }));
    if (options.emptyGroup) {
      nodes.push(makeGroup('G3', look, { label: '' }));
    }
  }
  const shortTexts = texts.filter((t) => t.length <= 12);
  for (let i = 0; i < n; i++) {
    const id = `N${i}`;
    const extra = { label: options.labels ? choose(look === 'handDrawn' ? shortTexts : texts) : look === 'handDrawn' ? id : `Node ${i}`, domId: `my-svg-flowchart-${id}-${i}` };
    if (options.groups && i % 3 === 0) {
      extra.parentId = 'G2';
    } else if (options.groups && i % 3 === 1) {
      extra.parentId = 'G1';
    }
    if (i % 2 === 1) {
      extra.shape = 'roundedRect';
    }
    nodes.push(makeNode(id, look, extra));
  }
  for (let i = 0; i + 1 < n; i++) {
    const extra = {};
    if (options.edgeLabels && i % 2 === 0) {
      extra.label = `edge ${i}`;
    }
    if (options.terminalLabels && i === 1) {
      extra.startLabelLeft = '1';
      extra.endLabelRight = '*';
    }
    edges.push(makeEdge(`N${i}`, `N${i + 1}`, i, look, extra));
  }
  if (options.selfLoop) {
    edges.push(makeEdge('N0', 'N0', 9, look));
  }
  return { nodes, edges, type: 'flowchart-v2', direction: options.direction ?? 'TB', diagramId: 'my-svg', markers: options.markers ?? ['point', 'circle', 'cross'] };
};

for (const look of looks) {
  for (const html of [true, false]) {
    const tag = `${look}/${html ? 'html' : 'svg'}`;
    const directive = directiveOf(look, html);
    const n = look === 'handDrawn' ? 3 : 5;
    add({ kind: 'createGraph', name: `createGraph/${tag}/plain`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n }) });
    add({ kind: 'createGraph', name: `createGraph/${tag}/groups`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n, groups: true, emptyGroup: true }) });
    add({ kind: 'createGraph', name: `createGraph/${tag}/groups-unwrap`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n, groups: true }), options: { unwrapGroupLabels: true } });
    add({ kind: 'createGraph', name: `createGraph/${tag}/groups-markdown-unwrap`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n, groups: true, groupMarkdown: true }), options: { unwrapGroupLabels: true } });
    add({ kind: 'createGraph', name: `createGraph/${tag}/group-dir`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n, groups: true, groupDir: 'LR', direction: 'BT' }) });
    add({ kind: 'createGraph', name: `createGraph/${tag}/labels`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n, labels: true }) });
    add({ kind: 'createGraph', name: `createGraph/${tag}/edge-labels`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: n, edgeLabels: true, terminalLabels: true }) });
    add({ kind: 'createGraph', name: `createGraph/${tag}/self-loop`, type: 'flowchart-v2', directive, data: makeData(look, { nodes: 2, selfLoop: true }) });
  }
}
add({ kind: 'createGraph', name: 'createGraph/detached', type: 'flowchart-v2', directive: {}, data: makeData('neo', { nodes: 4, groups: true }), detached: true });
add({ kind: 'createGraph', name: 'createGraph/empty', type: 'flowchart-v2', directive: {}, data: { nodes: [], edges: [], direction: 'TB' } });
add({ kind: 'createGraph', name: 'createGraph/numeric-ids', type: 'flowchart-v2', directive: {}, data: { nodes: [makeNode('b', 'neo'), makeNode('10', 'neo'), makeNode('2', 'neo'), makeNode('a', 'neo')], edges: [makeEdge('b', '2', 0, 'neo'), makeEdge('10', 'a', 1, 'neo')], direction: 'LR' } });
add({ kind: 'createGraph', name: 'createGraph/duplicate-edge-ids', type: 'flowchart-v2', directive: {}, data: { nodes: [makeNode('a', 'neo'), makeNode('b', 'neo')], edges: [makeEdge('a', 'b', 0, 'neo'), makeEdge('a', 'b', 0, 'neo'), makeEdge('a', 'b', 1, 'neo', { id: undefined })], direction: 'LR' } });
add({ kind: 'createGraph', name: 'createGraph/unknown-shape', type: 'flowchart-v2', directive: {}, data: { nodes: [makeNode('a', 'neo'), makeNode('b', 'neo', { shape: 'noSuchShape' })], edges: [], direction: 'LR' } });

// terminalLabelTranslate.
const keys = ['startLeft', 'startRight', 'endLeft', 'endRight'];
for (let i = 0; i < 240; i++) {
  const edge = {};
  if (rnd() < 0.8) {
    edge.x = coarse(-200, 400);
    edge.y = coarse(-200, 400);
  }
  if (rnd() < 0.5) {
    edge.arrowTypeStart = choose(['arrow_point', 'none', 'aggregation', '']);
  }
  if (rnd() < 0.5) {
    edge.arrowTypeEnd = choose(['arrow_point', 'none', 'extension', '']);
  }
  if (rnd() < 0.2) {
    edge.terminalLabelCenters = {};
    for (const k of keys) {
      if (rnd() < 0.5) {
        edge.terminalLabelCenters[k] = { x: coarse(-100, 100), y: coarse(-100, 100) };
      }
    }
  }
  let p = null;
  const r = rnd();
  if (r < 0.85) {
    p = [];
    const count = 2 + Math.floor(rnd() * 5);
    let x = coarse(-100, 300);
    let y = coarse(-100, 300);
    for (let j = 0; j < count; j++) {
      p.push({ x, y });
      // Segments along an axis and diagonal ones, some of length zero.
      const kind = rnd();
      if (kind < 0.3) {
        x += coarse(-120, 120);
      } else if (kind < 0.6) {
        y += coarse(-120, 120);
      } else if (kind < 0.95) {
        x += between(-120, 120);
        y += between(-120, 120);
      }
    }
  }
  add({ kind: 'terminal', name: `terminal/${i}`, edge, key: keys[i % 4], path: p });
}
add({ kind: 'terminal', name: 'terminal/one-point', edge: { x: 1, y: 2 }, key: 'startLeft', path: [{ x: 5, y: 6 }] });
add({ kind: 'terminal', name: 'terminal/unknown-key', edge: { x: 1, y: 2 }, key: 'middle', path: [{ x: 5, y: 6 }, { x: 50, y: 60 }] });

// getSubGraphTitleMargins.
const marginConfigs = [
  {},
  { flowchart: {} },
  { flowchart: { subGraphTitleMargin: {} } },
  { flowchart: { subGraphTitleMargin: { top: 10 } } },
  { flowchart: { subGraphTitleMargin: { bottom: 7.5 } } },
  { flowchart: { subGraphTitleMargin: { top: 0.1, bottom: 0.2 } } },
  { flowchart: { subGraphTitleMargin: { top: 0, bottom: 0 } } },
  { flowchart: { subGraphTitleMargin: { top: -3, bottom: 12 } } },
  { flowchart: { subGraphTitleMargin: { top: null, bottom: 4 } } },
  { flowchart: null },
  { flowchart: { subGraphTitleMargin: null } },
  { flowchart: { subGraphTitleMargin: { top: 1e21, bottom: 1 } } },
];
marginConfigs.forEach((config, i) => add({ kind: 'margins', name: `margins/${i}`, marginConfig: config }));

// Layouts made with createCommonLayoutRenderer.
const defs = {
  default: {},
  algorithm: { algorithm: 'toy.layered' },
  'all-phases': { prepare: true, measure: 'unwrap', afterPaint: true, algorithm: 'toy' },
  'custom-measure': { prepare: true, measure: 'custom', afterPaint: true },
  'custom-paint': { paint: 'custom', afterPaint: true },
  reverse: { paintOptions: { getNodes: 'reverse' } },
  'skip-node': { paintOptions: { skipNode: ['N1', 'G2'] } },
  'not-cluster': { measure: 'custom', paintOptions: { notCluster: ['G2'] } },
  'skip-edge': { paintOptions: { skipEdge: ['L_N0_N1_0'] } },
  'skip-intersect': { paintOptions: { skipIntersect: true } },
  'skip-intersect-some': { paintOptions: { skipIntersect: ['L_N1_N2_1'] } },
  'no-edge-node': { paintOptions: { getEdgeNode: 'none' } },
  'empty-paint-options': { paintOptions: {} },
};
// The clusters of the handDrawn look are large in the golden file: only a
// few definitions get a case with that look.
const handDefs = ['default', 'all-phases', 'reverse'];
for (const [dname, def] of Object.entries(defs)) {
  for (const look of looks) {
    if (look === 'handDrawn' && !handDefs.includes(dname)) {
      continue;
    }
    const html = look !== 'neo';
    const directive = directiveOf(look, html);
    const n = look === 'handDrawn' ? 2 : 4;
    add({ kind: 'render', name: `render/${dname}/${look}/groups`, type: 'flowchart-v2', directive, def, data: makeData(look, { nodes: n, groups: true, edgeLabels: dname !== 'custom-paint' }) });
  }
}
for (const look of looks) {
  const directive = directiveOf(look, look === 'classic');
  add({ kind: 'render', name: `render/plain/${look}`, type: 'flowchart-v2', directive, def: {}, data: makeData(look, { nodes: 3 }) });
  add({ kind: 'render', name: `render/no-markers/${look}`, type: 'flowchart-v2', directive, def: {}, data: makeData(look, { nodes: 3, markers: [] }) });
  add({ kind: 'render', name: `render/terminal-labels/${look}`, type: 'flowchart-v2', directive, def: {}, data: makeData(look, { nodes: 3, edgeLabels: true, terminalLabels: true }) });
  if (look !== 'handDrawn') {
    add({ kind: 'render', name: `render/title-margin/${look}`, type: 'flowchart-v2', directive: { ...directive, flowchart: { ...directive.flowchart, subGraphTitleMargin: { top: 10, bottom: 6 } } }, def: {}, data: makeData(look, { nodes: 3, groups: true, edgeLabels: true }) });
    add({ kind: 'render', name: `render/empty-group/${look}`, type: 'flowchart-v2', directive, def: {}, data: makeData(look, { nodes: 2, groups: true, emptyGroup: true }) });
  }
}
add({ kind: 'render', name: 'render/layout-only-edge', type: 'flowchart-v2', directive: {}, def: {}, data: (() => { const d = makeData('neo', { nodes: 3 }); d.edges[0].isLayoutOnly = true; return d; })() });
add({ kind: 'render', name: 'render/unknown-edge-node', type: 'flowchart-v2', directive: {}, def: { paintOptions: { skipIntersect: true } }, data: (() => { const d = makeData('neo', { nodes: 3 }); d.edges.push({ ...makeEdge('N0', 'N1', 7, 'neo'), points: [{ x: 0, y: 0 }, { x: 10, y: 10 }], start: undefined }); return d; })() });
// An edge with labels that the layout gave no position.
for (const skipIntersect of [true, false]) {
  add({
    kind: 'render',
    name: `render/label-without-position/${skipIntersect ? 'skip-intersect' : 'intersect'}`,
    type: 'flowchart-v2',
    directive: {},
    def: { paintOptions: skipIntersect ? { skipIntersect: true } : {} },
    data: (() => {
      const d = makeData('neo', { nodes: 3 });
      d.edges.push({ ...makeEdge('N0', 'N1', 7, 'neo', { label: 'lost', startLabelLeft: 'a', endLabelRight: 'b' }), points: [{ x: 0, y: 0 }, { x: 40, y: 10 }, { x: 80, y: 0 }], start: undefined });
      return d;
    })(),
  });
}
add({ kind: 'render', name: 'render/empty', type: 'flowchart-v2', directive: {}, def: {}, data: { nodes: [], edges: [], type: 'flowchart-v2', direction: 'TB', diagramId: 'my-svg', markers: [] } });

// ---- Promise.all against one node after another ----

const stats = [];
for (const f of handDrawn) {
  const c = byFixture.get(f.name);
  if (c && !c.error) {
    stats.push({ kind: 'createGraph', name: `stats/${c.fixture}`, type: c.type, directive: c.directive, data: c.data, statsOnly: true });
  }
}

// ---- Run upstream ----

const strip = (v) => JSON.parse(JSON.stringify(v));
const result = oracle('graph.js', { cases: strip([...cases, ...stats]), samples });
for (const [type, error] of Object.entries(result.loadErrors || {})) {
  console.error(`sample for ${type} did not load: ${error}`);
}

const DELETED = '__mmdg_deleted__';
const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);
const diff = (from, to) => {
  if (isObject(from) && isObject(to)) {
    const out = {};
    for (const k of Object.keys(to)) {
      if (!(k in from)) {
        out[k] = to[k];
      } else {
        const d = diff(from[k], to[k]);
        if (d !== undefined) {
          out[k] = d;
        }
      }
    }
    for (const k of Object.keys(from)) {
      if (!(k in to)) {
        out[k] = DELETED;
      }
    }
    return Object.keys(out).length > 0 ? out : undefined;
  }
  return JSON.stringify(from) === JSON.stringify(to) ? undefined : to;
};
const configs = [];
const css = [];
const index = (list, map, value) => {
  const key = JSON.stringify(value);
  if (!map.has(key)) {
    map.set(key, list.length);
    list.push(value);
  }
  return map.get(key);
};
const configIndex = new Map();
const cssIndex = new Map();
const goldenCases = strip(cases).map((c, i) => {
  const r = result.cases[i];
  const out = { ...c, result: { ...r } };
  delete out.directive;
  delete out.result.name;
  if (r.config !== undefined) {
    out.config = index(configs, configIndex, r.config);
    delete out.result.config;
  }
  if (r.css !== undefined) {
    out.css = index(css, cssIndex, r.css);
    delete out.result.css;
  }
  return out;
});
const baseConfig = configs[0];
const golden = { baseConfig, configs: configs.map((c) => diff(baseConfig, c) ?? {}), css, cases: goldenCases };
const json = JSON.stringify(golden);
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, outFile.endsWith('.gz') ? zlib.gzipSync(json, { level: 9 }) : json);

const kinds = {};
for (const c of goldenCases) {
  kinds[c.kind] = (kinds[c.kind] || 0) + 1;
}
const errors = goldenCases.filter((c) => c.result.error);
console.error(`${goldenCases.length} cases (${corpusCount} from the corpus) ${JSON.stringify(kinds)}, ${errors.length} upstream errors -> ${outFile} (${fs.statSync(outFile).size} bytes)`);
for (const c of errors) {
  console.error(`  upstream error: ${c.name}: ${c.result.error}`);
}

// How often does the Promise.all of createGraphWithElements matter?
const statResults = result.cases.slice(cases.length);
const withStats = [...statResults, ...result.cases.slice(0, cases.length).filter((r, i) => cases[i].kind === 'createGraph')].filter((r) => r.sequential && !r.sequential.error);
const differs = withStats.filter((r) => !r.sequential.sameHtml);
console.error(`createGraphWithElements, one node after another against Promise.all: ${withStats.length} layouts compared, ${differs.length} differ`);
const handStats = statResults.filter((r) => r.sequential && !r.sequential.error);
const handDiffers = handStats.filter((r) => !r.sequential.sameHtml);
console.error(`  corpus fixtures with the handDrawn look: ${handStats.length} compared, ${handDiffers.length} differ`);
for (const r of differs.slice(0, 40)) {
  console.error(`  differs: ${r.name}`);
}
