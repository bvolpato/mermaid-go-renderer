// Runs cluster cases through upstream mermaid in Chromium and writes the
// golden file for internal/ru/clusters.
//
//   node gen-clusters.mjs [golden.json.gz]
//
// The default output is internal/ru/clusters/testdata/clusters.json.gz.
//
// The cases come from two sources:
//
//   1. Real cluster nodes: probes/clusters-capture.js renders corpus
//      fixtures with subgraphs, composite states, namespaces, kanban
//      sections, system boundaries and lanes, with the layout of the
//      fixture and with the other one of dagre and elk, and records the
//      nodes that reach insertCluster.
//   2. A matrix over every cluster shape: looks, themes, HTML and SVG
//      labels, label texts, styles, title margins, color slots, paddings and
//      missing properties.
//   3. Clusters inserted together with Promise.all (`all`): every ordered
//      pair of kinds of clusters, and mixes with the handDrawn look.
//
// probes/clusters.js then calls the real insertCluster for each case. The
// golden file is { baseConfig, configs, css, cases, all, geometry, paintsTitle }:
// each case has the indexes of its effective config and style sheet in the
// two lists. A config is a patch of baseConfig, see below.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const corpus = path.join(root, 'testdata/corpus/mermaid-e2e');
const outFile = path.resolve(process.argv[2] || path.join(root, 'internal/ru/clusters/testdata/clusters.json.gz'));

// A seeded generator, so that the cases are the same on every run.
let seed = 0x636c7573;
const rnd = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const between = (lo, hi) => lo + rnd() * (hi - lo);
// A number with a few binary digits after the point, as layouts produce.
const coarse = (lo, hi) => Math.round(between(lo, hi) * 64) / 64;

const oracle = (probe, args) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-clusters-'));
  const argsFile = path.join(dir, 'args.json');
  const resultFile = path.join(dir, 'out.json');
  fs.writeFileSync(argsFile, JSON.stringify(args));
  execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes', probe), '--args', argsFile, '--out', resultFile], { stdio: 'inherit' });
  const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
  fs.rmSync(dir, { recursive: true });
  return result;
};

// ---- 1. Real cluster nodes from corpus fixtures ----

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
// pick returns up to n fixtures of a family whose text matches re, spread
// evenly over the sorted list.
const pick = (family, re, n) => {
  const dir = path.join(corpus, family);
  if (!fs.existsSync(dir)) {
    return [];
  }
  const hits = walk(dir).filter((f) => re.test(fs.readFileSync(f, 'utf8')));
  const out = [];
  for (let i = 0; i < Math.min(n, hits.length); i++) {
    out.push(hits[Math.floor((i * hits.length) / Math.min(n, hits.length))]);
  }
  return out;
};
const selected = [
  ...pick('flowchart', /^\s*subgraph\b/m, 8),
  ...pick('flowchart', /look:\s*handDrawn[\s\S]*^\s*subgraph\b/m, 2),
  ...pick('state-diagram-v2', /^\s*state\s+[^\n]*\{/m, 4),
  ...pick('state-diagram-v2', /^\s*--\s*$/m, 2),
  ...pick('state-diagram-v2', /^\s*note\s+(left|right)\s+of/m, 2),
  ...pick('class-diagram', /^\s*namespace\b/m, 4),
  ...pick('kanban', /./, 2),
  ...pick('usecase', /^\s*systemBoundary\b/m, 6),
  ...pick('swimlanes', /subgraph/, 5),
  ...pick('agentflow', /./, 3),
];
const fixtures = [];
for (const file of [...new Set(selected)]) {
  const text = fs.readFileSync(file, 'utf8');
  const name = path.relative(corpus, file);
  fixtures.push({ name, text });
  // The same diagram with each of the two layouts. One of them is the
  // layout of the fixture again; its nodes are dropped as duplicates.
  fixtures.push({ name: name + ' [dagre]', text, init: { layout: 'dagre' } });
  fixtures.push({ name: name + ' [elk]', text, init: { layout: 'elk' } });
}
const captured = oracle('clusters-capture.js', { fixtures });
const captureErrors = captured.filter((c) => c.error);
for (const c of captureErrors) {
  console.error(`capture: ${c.fixture}: ${c.error}`);
}

const cases = [];
const samples = {};
const seen = new Set();
const perGroup = {};
let bigHandDrawn = 0;
for (const c of captured) {
  if (c.error) {
    continue;
  }
  const fixture = fixtures.find((f) => f.name === c.fixture);
  samples[c.type] ||= fixture.text;
  // The layout of the capture is part of the directive only when it was
  // forced: a duplicate of the node of the plain render is then dropped.
  const key = JSON.stringify([c.type, c.directive, c.layout, c.node]);
  if (seen.has(key)) {
    continue;
  }
  seen.add(key);
  // At most 8 nodes for each diagram type, shape and layout.
  const group = [c.type, c.shape, c.layout].join('/');
  perGroup[group] = (perGroup[group] || 0) + 1;
  if (perGroup[group] > 8) {
    continue;
  }
  // A hachure fill of the handDrawn look has a path for every 1.5 px of the
  // box: keep only a few large ones, for the size of the golden file.
  if (c.node.look === 'handDrawn' && c.node.width * c.node.height > 6000 && ++bigHandDrawn > 4) {
    continue;
  }
  cases.push({ name: `corpus/${c.fixture}/${c.node.id}`, type: c.type, directive: { ...c.directive, layout: c.layout }, node: c.node });
}
const capturedCount = cases.length;

// ---- 2. The matrix ----

const base = {
  rect: {
    type: 'flowchart-v2',
    node: { id: 'S', label: 'Subgraph one', labelStyle: '', labelType: 'text', padding: 8, cssCompiledStyles: [], cssClasses: '', shape: 'rect', isGroup: true, colorIndex: 0, domId: 'my-svg-S', x: 112, y: 118, width: 200, height: 212 },
  },
  squareRect: {
    type: 'flowchart-v2',
    node: { id: 'Q', label: 'Square', labelStyle: '', labelType: 'text', padding: 8, cssCompiledStyles: [], cssClasses: 'default', shape: 'squareRect', isGroup: true, colorIndex: 1, domId: 'my-svg-Q', x: 300.5, y: 150.25, width: 260, height: 180 },
  },
  default: {
    type: 'classDiagram',
    node: { id: 'Auth', label: 'Authentication Service', isGroup: true, padding: 16, cssStyles: [], domId: 'my-svg-Auth', x: 108.83203125, y: 213, width: 193.6640625, height: 402 },
  },
  roundedWithTitle: {
    type: 'stateDiagram',
    node: { labelStyle: '', shape: 'roundedWithTitle', label: 'Long state name 2', cssClasses: ' statediagram-state statediagram-cluster ', cssCompiledStyles: [], cssStyles: [], id: 'NotShooting', dir: 'TB', domId: 'my-svg-state-NotShooting-0', type: 'group', isGroup: true, colorIndex: 0, padding: 8, rx: 10, ry: 10, labelType: 'markdown', centerLabel: true, description: [], wrappingWidth: 120, x: 104, y: 97, width: 184, height: 170 },
  },
  noteGroup: {
    type: 'stateDiagram',
    node: { id: 'State1----parent', labelStyle: '', shape: 'noteGroup', label: 'Important information! You can write notes.', cssClasses: ' statediagram-state statediagram-cluster ', cssStyles: [], isGroup: true, padding: 16, domId: 'my-svg-state-State1----parent-1', x: 152.5, y: 60, width: 289, height: 104 },
  },
  divider: {
    type: 'stateDiagram',
    node: { labelStyle: '', shape: 'divider', label: 'divider-id-1', cssClasses: ' statediagram-state statediagram-cluster statediagram-cluster-alt', cssCompiledStyles: [], cssStyles: [], id: 'divider-id-1', dir: 'TB', domId: 'my-svg-state-divider-id-1-3', type: 'group', isGroup: true, colorIndex: 0, padding: 8, rx: 10, ry: 10, parentId: 'Active', x: 150, y: 120.5, width: 220, height: 141 },
  },
  kanbanSection: {
    type: 'kanban',
    node: { id: 'id2', label: 'In progress', labelType: 'markdown', isGroup: true, shape: 'kanbanSection', level: 2, domId: 'my-svg-id2', x: 200, width: 200, y: 0, height: 600, rx: 5, ry: 5, cssClasses: 'undefined section-1' },
  },
  flowGroup: {
    type: 'flowchart-v2',
    node: { id: 'F', label: 'Research flow', labelStyle: '', labelType: 'text', padding: 12, cssCompiledStyles: [], cssClasses: 'flow', shape: 'flowGroup', isGroup: true, colorIndex: 2, domId: 'my-svg-F', x: 210, y: 140, width: 300, height: 200 },
  },
  usecaseSystemBoundary: {
    type: 'usecase',
    node: { id: 'Account', label: 'Account', labelType: 'text', shape: 'usecaseSystemBoundary', isGroup: true, padding: 20, colorIndex: 0, cssClasses: 'default system-boundary system-boundary-rect', cssStyles: [], cssCompiledStyles: [], boundaryType: 'rect', wrappingWidth: 120, domId: 'my-svg-usecase-Account', x: 438, y: 69, width: 200, height: 114 },
  },
  swimlane: {
    type: 'swimlane',
    node: { id: 'OwnedLane', label: 'OwnedLane', labelStyle: '', labelType: 'text', padding: 8, cssCompiledStyles: [], cssClasses: '', shape: 'swimlane', isGroup: true, colorIndex: 0, domId: 'my-svg-OwnedLane', direction: 'TB', x: 126, y: 22.5, width: 252, height: 117, swimlaneContentTop: 0 },
  },
};
// A type that has no captured fixture falls back to the flowchart styles.
for (const b of Object.values(base)) {
  if (!samples[b.type] && !['flowchart-v2', 'classDiagram', 'stateDiagram', 'kanban', 'block'].includes(b.type)) {
    console.error(`no fixture loads the diagram type ${b.type}: using flowchart-v2`);
    b.type = 'flowchart-v2';
  }
}

// The variants of the handDrawn look for which upstream never returns.
const neverReturns = {
  usecaseSystemBoundary: ['no-size', 'no-position', 'no-padding'],
  swimlane: ['no-size', 'no-position'],
};
const looks = ['classic', 'neo', 'handDrawn'];
const themes = ['default', 'redux-color'];
const add = (name, type, directive, node) => cases.push({ name, type, directive, node });
const without = (node, ...keys) => {
  const out = { ...node };
  for (const k of keys) {
    delete out[k];
  }
  return out;
};

// A hachure fill of the handDrawn look has a path for every 1.5 px of the
// box. The handDrawn cases use small boxes, for the size of the golden file.
const handBox = (node, w, h) => ({ ...node, width: w, height: h, ...(node.label === undefined ? {} : { label: 'Sub' }) });
const clean = (node) => {
  const out = { ...node };
  for (const k of Object.keys(out)) {
    if (out[k] === undefined) {
      delete out[k];
    }
  }
  return out;
};
// squareRect and the default shape are the function of rect: they get the
// main block and a few variants only.
const fewVariants = { squareRect: ['no-classes', 'css-styles', 'no-label'], default: ['no-classes', 'css-styles', 'no-label', 'long-label'] };
// The label variants do not depend on the look: no handDrawn case.
const noHandDrawn = ['long-label', 'br-label', 'markdown-wide', 'entity-label', 'unicode-label', 'space-label', 'color-index-14', 'padding-30', 'rx-ry', 'negative'];

for (const [shape, b] of Object.entries(base)) {
  // Every look and theme, with HTML and with SVG labels.
  for (const look of looks) {
    for (const theme of themes) {
      for (const html of [true, false]) {
        const directive = { look, theme, htmlLabels: html, flowchart: { htmlLabels: html } };
        const node = look === 'handDrawn' ? handBox(b.node, 90, 60) : b.node;
        add(`${shape}/${look}/${theme}/${html ? 'html' : 'svg'}`, b.type, directive, { ...node, look });
      }
    }
  }
  // The defaults of the diagram type, with no look in the node.
  add(`${shape}/defaults`, b.type, {}, b.node);

  // Variants, each with the classic look and HTML labels, the neo look and
  // SVG labels, and the handDrawn look.
  const variants = {
    'long-label': { label: 'A cluster title that is much longer than the box of the cluster is wide, to make it grow' },
    'br-label': { label: 'First line<br/>Second line<br>Third' },
    'markdown-label': { label: 'This is **bold** and *italic*', labelType: 'markdown' },
    'markdown-wide': { label: 'A **markdown** title that wraps at the width of the node because it is long', labelType: 'markdown', width: 120 },
    'empty-label': { label: '' },
    'space-label': { label: '   ' },
    'no-label': { label: undefined },
    'entity-label': { label: 'Tom & Jerry <b>bold</b> #9829; fa:fa-car' },
    'unicode-label': { label: 'Größe 日本語 😀' },
    narrow: { width: 20 },
    tiny: { width: 0, height: 0 },
    'css-styles': { cssStyles: ['fill:#f9f', 'stroke:#333', 'stroke-width:4px'] },
    'compiled-styles': { cssCompiledStyles: ['fill:#111827', 'stroke:#F59E0B', 'color:#F9FAFB', 'stroke-dasharray: 5 5'] },
    'label-style': { labelStyle: 'color:#ff0000;font-size:20px;font-weight:bold' },
    'mixed-styles': { cssStyles: ['fill:#bbf', 'color:#fff', 'font-style:italic', 'stroke-width:2px'], labelStyle: 'fill:#00f;stroke:#0f0' },
    'color-index-3': { colorIndex: 3 },
    'color-index-14': { colorIndex: 14 },
    'no-color-index': { colorIndex: undefined },
    'padding-0': { padding: 0 },
    'padding-30': { padding: 30 },
    'no-padding': { padding: undefined },
    'no-rx': { rx: undefined, ry: undefined },
    'rx-only': { rx: 7, ry: undefined },
    'rx-ry': { rx: 3, ry: 12 },
    fractional: { x: 133.33333333333334, y: 77.7, width: 50.1, height: 29.99 },
    negative: { x: -210.5, y: -80.25 },
    'no-classes': { cssClasses: undefined },
    'no-dom-id': { domId: undefined },
    'no-position': { x: undefined, y: undefined },
    'no-size': { width: undefined, height: undefined },
  };
  if (shape === 'roundedWithTitle' || shape === 'divider') {
    variants['alt'] = { cssClasses: ' statediagram-state statediagram-cluster statediagram-cluster-alt' };
    variants['not-alt'] = { cssClasses: ' statediagram-state statediagram-cluster ' };
  }
  if (shape === 'usecaseSystemBoundary') {
    variants['package'] = { boundaryType: 'package', cssClasses: 'default system-boundary system-boundary-package' };
    variants['package-long'] = { boundaryType: 'package', label: 'A package boundary with a long title' };
    variants['package-short'] = { boundaryType: 'package', label: 'P', height: 30 };
    variants['no-boundary-type'] = { boundaryType: undefined };
    variants['styled'] = { cssStyles: ['stroke-width:4px'], cssCompiledStyles: ['fill:#eef8ff', 'stroke:#3572a5'] };
  }
  if (shape === 'swimlane') {
    variants['lr'] = { direction: 'LR', x: 180, y: 60, width: 70, height: 40 };
    variants['lr-long'] = { direction: 'LR', label: 'A lane with a long title', x: 180, y: 60, width: 60, height: 120 };
    variants['content-top'] = { swimlaneContentTop: -20 };
    variants['content-top-below'] = { swimlaneContentTop: 10 };
    variants['no-content-top'] = { swimlaneContentTop: undefined };
    variants['no-direction'] = { direction: undefined };
  }
  const combos = [
    ['classic', { look: 'classic', theme: 'default', htmlLabels: true, flowchart: { htmlLabels: true } }],
    ['neo-svg', { look: 'neo', theme: 'redux-color', htmlLabels: false, flowchart: { htmlLabels: false } }],
    ['handDrawn', { look: 'handDrawn', theme: 'default' }],
  ];
  for (const [vname, change] of Object.entries(variants)) {
    if (fewVariants[shape] && !fewVariants[shape].includes(vname)) {
      continue;
    }
    for (const [cname, directive] of combos) {
      // Upstream never returns for these: rough loops on a hachure fill of a
      // rectangle with NaN coordinates until the page runs out of memory.
      if (cname === 'handDrawn' && neverReturns[shape]?.includes(vname)) {
        continue;
      }
      if (cname === 'handDrawn' && noHandDrawn.includes(vname)) {
        continue;
      }
      const from = cname === 'handDrawn' ? handBox(b.node, 44, 30) : b.node;
      add(`${shape}/${vname}/${cname}`, b.type, directive, clean({ ...from, look: directive.look, ...change }));
    }
  }
  if (fewVariants[shape]) {
    continue;
  }

  // Title margins and other config.
  const configs = {
    'title-margin': { flowchart: { subGraphTitleMargin: { top: 10, bottom: 5 } } },
    'title-margin-top': { look: 'classic', flowchart: { subGraphTitleMargin: { top: 7.5 } }, htmlLabels: false },
    'hand-drawn-seed': { look: 'handDrawn', handDrawnSeed: 42 },
    font: { fontFamily: 'courier, monospace', fontSize: 20 },
    'theme-dark': { theme: 'dark', look: 'classic' },
    'theme-redux-dark-color': { theme: 'redux-dark-color', look: 'neo' },
    'theme-neutral-hand': { theme: 'neutral', look: 'handDrawn' },
    'theme-variables': { theme: 'base', themeVariables: { clusterBkg: '#ffeedd', clusterBorder: '#aa3300', altBackground: '#ddeeff', nodeBorder: '#123456', compositeBackground: '#fafafa', compositeTitleBackground: '#e0e0e0' }, look: 'handDrawn' },
    'no-auto-wrap': { markdownAutoWrap: false },
    'flowchart-html-only': { flowchart: { htmlLabels: false } },
    'root-html-wins': { htmlLabels: true, flowchart: { htmlLabels: false } },
    'wrapping-width': { flowchart: { wrappingWidth: 60 } },
  };
  for (const [cname, directive] of Object.entries(configs)) {
    const from = directive.look === 'handDrawn' ? handBox(b.node, 44, 30) : b.node;
    const node = { ...from, look: directive.look || 'neo' };
    add(`${shape}/config-${cname}`, b.type, directive, node);
    if (cname === 'no-auto-wrap') {
      add(`${shape}/config-${cname}-markdown`, b.type, directive, { ...node, labelType: 'markdown', label: 'A **markdown** title with several words' });
    }
  }

  // Random boxes.
  for (let i = 0; i < 6; i++) {
    const look = looks[i % looks.length];
    const hand = look === 'handDrawn';
    const node = { ...b.node, look, x: coarse(-300, 600), y: coarse(-200, 500), width: coarse(10, hand ? 70 : 500), height: coarse(10, hand ? 50 : 400), padding: Math.floor(between(0, 25)) };
    if (hand) {
      node.label = 'R' + i;
    }
    if (shape === 'swimlane') {
      node.direction = i % 2 === 0 ? 'TB' : 'LR';
      node.swimlaneContentTop = coarse(node.y - node.height / 2 - 10, node.y);
    }
    if (shape === 'usecaseSystemBoundary') {
      node.boundaryType = i % 2 === 0 ? 'rect' : 'package';
    }
    add(`${shape}/random-${i}`, b.type, { look, htmlLabels: i % 2 === 0, flowchart: { htmlLabels: i % 2 === 0 } }, node);
  }
}

// Shapes drawn in the style scope of other diagram types.
for (const type of ['flowchart-v2', 'stateDiagram', 'classDiagram', 'kanban', 'block', 'usecase']) {
  if (type === 'usecase' && !samples[type]) {
    continue;
  }
  for (const shape of ['rect', 'roundedWithTitle', 'kanbanSection']) {
    for (const look of looks) {
      const from = look === 'handDrawn' ? handBox(base[shape].node, 44, 30) : base[shape].node;
      add(`types/${type}/${shape}/${look}`, type, { look }, { ...from, look });
    }
  }
}

// Errors of upstream.
add('errors/unknown-shape', 'flowchart-v2', {}, { ...base.rect.node, shape: 'noSuchClusterShape' });
add('errors/rounded-hand-drawn-no-classes', 'stateDiagram', { look: 'handDrawn' }, without({ ...base.roundedWithTitle.node, look: 'handDrawn' }, 'cssClasses'));

// ---- Clusters inserted together (Promise.all) ----

// The kinds of clusters differ in how many turns of the microtask queue
// they take before they draw and before they finish.
const kinds = {
  'rect-text': () => base.rect.node,
  'rect-markdown': () => ({ ...base.rect.node, label: 'A **bold** title', labelType: 'markdown' }),
  'rect-empty': () => ({ ...base.rect.node, label: '' }),
  'rect-icon': () => ({ ...base.rect.node, label: 'Car fa:fa-car icon' }),
  'square-rect': () => base.squareRect.node,
  'no-shape': () => base.default.node,
  'flow-group': () => base.flowGroup.node,
  'flow-group-markdown': () => ({ ...base.flowGroup.node, label: 'A *flow*', labelType: 'markdown' }),
  'flow-group-no-label': () => ({ ...base.flowGroup.node, label: '' }),
  divider: () => base.divider.node,
  'note-group': () => base.noteGroup.node,
  'rounded-with-title': () => base.roundedWithTitle.node,
  'rounded-alt': () => ({ ...base.roundedWithTitle.node, cssClasses: ' statediagram-state statediagram-cluster statediagram-cluster-alt' }),
  'kanban-section': () => ({ ...base.kanbanSection.node, padding: 8 }),
  usecase: () => base.usecaseSystemBoundary.node,
  'usecase-package': () => ({ ...base.usecaseSystemBoundary.node, boundaryType: 'package' }),
  swimlane: () => base.swimlane.node,
  'swimlane-lr': () => ({ ...base.swimlane.node, direction: 'LR' }),
};
const kindNames = Object.keys(kinds);
// together builds the nodes of one case: each node gets its own id and
// place, and with the handDrawn look a small box.
const together = (names, look) =>
  names.map((name, i) => {
    const node = { ...kinds[name](), look, id: `c${i}`, domId: `my-svg-c${i}`, x: 100 + 150 * i, y: 80 };
    if (look === 'handDrawn') {
      node.width = 40;
      node.height = 28;
      if (node.label && !/fa:|\*/.test(node.label)) {
        node.label = 'T' + i;
      }
    }
    return clean(node);
  });
const all = [];
// Every ordered pair of kinds: the order in which the two calls finish.
for (const html of [true, false]) {
  for (const a of kindNames) {
    for (const b of kindNames) {
      all.push({ name: `all/pair/${html ? 'html' : 'svg'}/${a}+${b}`, type: 'flowchart-v2', directive: { look: 'classic', htmlLabels: html, flowchart: { htmlLabels: html } }, nodes: together([a, b], 'classic') });
    }
  }
}
// Mixes with the handDrawn look: the order of the random draws.
for (const html of [true, false]) {
  const directive = { look: 'handDrawn', htmlLabels: html, flowchart: { htmlLabels: html } };
  const tag = html ? 'html' : 'svg';
  all.push({ name: `all/hand/${tag}/every-kind`, type: 'flowchart-v2', directive, nodes: together(kindNames, 'handDrawn') });
  all.push({ name: `all/hand/${tag}/every-kind-reversed`, type: 'flowchart-v2', directive, nodes: together([...kindNames].reverse(), 'handDrawn') });
  for (let i = 0; i < 8; i++) {
    const names = [];
    const count = 3 + Math.floor(rnd() * 4);
    for (let j = 0; j < count; j++) {
      names.push(kindNames[Math.floor(rnd() * kindNames.length)]);
    }
    all.push({ name: `all/hand/${tag}/mix-${i}`, type: 'flowchart-v2', directive, nodes: together(names, 'handDrawn') });
  }
}
all.push({ name: 'all/empty', type: 'flowchart-v2', directive: {}, nodes: [] });
all.push({ name: 'all/state-hand', type: 'stateDiagram', directive: { look: 'handDrawn' }, nodes: together(['rounded-with-title', 'divider', 'note-group', 'rounded-alt', 'divider'], 'handDrawn') });

// ---- Pure functions ----

const geometry = [];
for (let i = 0; i < 300; i++) {
  const node = { x: coarse(-500, 500), y: coarse(-500, 500), width: coarse(0, 600), height: coarse(0, 400), padding: Math.floor(between(0, 40)) };
  const r = rnd();
  if (r < 0.4) {
    node.boundaryType = 'package';
  } else if (r < 0.7) {
    node.boundaryType = 'rect';
  } else if (r < 0.75) {
    node.boundaryType = '';
  }
  if (i % 37 === 5) {
    delete node.padding;
  }
  if (i % 41 === 7) {
    delete node.width;
  }
  if (i % 43 === 9) {
    delete node.height;
  }
  geometry.push({ node, labelBBox: { width: coarse(0, 700), height: coarse(0, 80) } });
}
const paintsTitle = [null, 'rect', 'squareRect', 'roundedWithTitle', 'noteGroup', 'divider', 'kanbanSection', 'flowGroup', 'usecaseSystemBoundary', 'swimlane', '', 'other'];

// ---- Run upstream ----

if (process.env.MMDG_DUMP_CASES) {
  // For debugging: write the arguments of the probe and stop.
  fs.writeFileSync(process.env.MMDG_DUMP_CASES, JSON.stringify({ cases, all, samples, geometry, paintsTitle }));
  process.exit(0);
}
const result = oracle('clusters.js', { cases, all, samples, geometry, paintsTitle });
for (const [type, error] of Object.entries(result.loadErrors || {})) {
  console.error(`sample for ${type} did not load: ${error}`);
}
// The effective configs are large and differ in a few values. The golden
// file has the first one in full (baseConfig) and the others as patches:
// the properties that differ, recursively. A property that the base has and
// the config does not is the string DELETED.
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
const goldenCases = cases.map((c, i) => {
  const r = result.cases[i];
  const out = { name: c.name, type: c.type, node: c.node, ...r };
  if (r.config !== undefined) {
    out.config = index(configs, configIndex, r.config);
  }
  if (r.css !== undefined) {
    out.css = index(css, cssIndex, r.css);
  }
  return out;
});
const goldenAll = all.map((c, i) => {
  const r = result.all[i];
  const out = { name: c.name, type: c.type, nodes: c.nodes, ...r };
  if (r.config !== undefined) {
    out.config = index(configs, configIndex, r.config);
  }
  if (r.css !== undefined) {
    out.css = index(css, cssIndex, r.css);
  }
  return out;
});
const baseConfig = configs[0];
const golden = {
  baseConfig,
  configs: configs.map((c) => diff(baseConfig, c) ?? {}),
  css,
  cases: goldenCases,
  all: goldenAll,
  geometry: geometry.map((g, i) => ({ ...g, result: result.geometry[i] })),
  paintsTitle: paintsTitle.map((shape, i) => ({ shape, result: result.paintsTitle[i] })),
};
const json = JSON.stringify(golden);
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, outFile.endsWith('.gz') ? zlib.gzipSync(json, { level: 9 }) : json);
const errors = [...golden.cases, ...golden.all].filter((c) => c.error);
console.error(
  `${golden.cases.length} cases (${capturedCount} from the corpus, ${captureErrors.length} fixtures failed to render), ` +
    `${errors.length} upstream errors, ${configs.length} configs, ${css.length} style sheets, ` +
    `${golden.all.length} Promise.all cases, ${geometry.length} geometry cases -> ${outFile} (${fs.statSync(outFile).size} bytes)`
);
for (const c of errors) {
  console.error(`  upstream error: ${c.name}: ${c.error}`);
}
