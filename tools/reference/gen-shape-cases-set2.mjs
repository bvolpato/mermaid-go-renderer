// Builds the shape cases of internal/ru/shapes/set2 and records them with
// gen-shapes.mjs (upstream mermaid in Chromium).
//
//   node gen-shape-cases-set2.mjs [--list] [--cases-dir DIR] [handler ...]
//
// With no handler name it records every shape of the package. Each shape
// gets one golden file: internal/ru/shapes/set2/testdata/<handler>.json.gz.
// --list prints the number of cases of each shape and records nothing.
// --cases-dir also keeps the case files (the input of gen-shapes.mjs).
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(here, '../../internal/ru/shapes/set2/testdata');

// Handler name in shapes.ts -> the shape ids that reach it, and what the
// shape reads. `label: false` marks the shapes that draw no label.
// `size: false` marks the shapes that ignore node.width and node.height,
// and `size: 'label'` the ones where only the label helper reads them.
// `heavy: true` marks the shapes with many points: one case of them takes
// 15 to 40 KB in the compressed golden file, so they get fewer cases.
const SHAPES = {
  curlyBraceLeft: { ids: ['brace', 'comment', 'brace-l'], size: 'label', heavy: true },
  curlyBraceRight: { ids: ['brace-r'], size: 'label', heavy: true },
  curlyBraces: { ids: ['braces'], size: 'label', heavy: true },
  lightningBolt: { ids: ['bolt', 'com-link', 'lightning-bolt'], label: false },
  waveEdgedRectangle: { ids: ['doc', 'document'], heavy: true },
  halfRoundedRectangle: { ids: ['delay', 'half-rounded-rectangle'], heavy: true },
  tiltedCylinder: { ids: ['h-cyl', 'das', 'horizontal-cylinder'] },
  linedCylinder: { ids: ['lin-cyl', 'disk', 'lined-cylinder'] },
  curvedTrapezoid: { ids: ['curv-trap', 'curved-trapezoid', 'display'], heavy: true },
  dividedRectangle: { ids: ['div-rect', 'div-proc', 'divided-rectangle', 'divided-process'], size: 'label' },
  triangle: { ids: ['tri', 'extract', 'triangle'], flowchartHtmlLabels: true },
  windowPane: { ids: ['win-pane', 'internal-storage', 'window-pane'] },
  filledCircle: { ids: ['f-circ', 'junction', 'filled-circle'], label: false, size: false },
  trapezoidalPentagon: { ids: ['notch-pent', 'loop-limit', 'notched-pentagon'] },
  flippedTriangle: { ids: ['flip-tri', 'manual-file', 'flipped-triangle'] },
  slopedRect: { ids: ['sl-rect', 'manual-input', 'sloped-rectangle'] },
  multiWaveEdgedRectangle: { ids: ['docs', 'documents', 'st-doc', 'stacked-document'], heavy: true },
  multiRect: { ids: ['st-rect', 'procs', 'processes', 'stacked-rectangle'] },
  bowTieRect: { ids: ['bow-rect', 'stored-data', 'bow-tie-rectangle'], heavy: true },
  crossedCircle: { ids: ['cross-circ', 'summary', 'crossed-circle'], label: false },
  taggedWaveEdgedRectangle: { ids: ['tag-doc', 'tagged-document'], heavy: true },
  taggedRect: { ids: ['tag-rect', 'tagged-rectangle', 'tag-proc', 'tagged-process'] },
  waveRectangle: { ids: ['flag', 'paper-tape'], heavy: true },
  rect_left_inv_arrow: { ids: ['odd', 'rect_left_inv_arrow'] },
  linedWaveEdgedRect: { ids: ['lin-doc', 'lined-document'], heavy: true },
};

// Look and theme of a case. The key is part of the case name.
const CTX = {
  cd: { look: 'classic', theme: 'default' },
  cr: { look: 'classic', theme: 'redux-color' },
  nd: { look: 'neo', theme: 'default' },
  nr: { look: 'neo', theme: 'redux-color' },
  hd: { look: 'handDrawn', theme: 'default' },
  hr: { look: 'handDrawn', theme: 'redux-color' },
};

const LABELS = {
  empty: { label: '' },
  long: { label: 'This is a long label that has to wrap over several lines of text' },
  br: { label: 'First line<br/>Second longer line' },
  md: { label: '**Bold** and *italic* words', labelType: 'markdown' },
  mdlines: { label: 'Line one\nLine **two** is longer', labelType: 'markdown' },
  chars: { label: 'Fish & chips < 5 > 3' },
};

const STYLES = {
  fill: { cssStyles: ['fill:#f9f', 'stroke:#333', 'stroke-width:4px', 'color:#fff'] },
  color: { cssStyles: ['color:#ff0000', 'font-size:20px'] },
  class: {
    cssCompiledStyles: ['fill:#bbf', 'stroke:#f66', 'stroke-width:2px', 'color:#fff', 'stroke-dasharray: 5 5'],
    cssClasses: 'default foo',
  },
  none: { cssStyles: undefined, cssCompiledStyles: undefined },
};

function casesFor(handler) {
  const spec = SHAPES[handler];
  const cases = [];
  let n = 0;
  // add records one case. `node` are the properties that differ from a
  // plain flowchart node, and `directive` is added to look, theme and
  // htmlLabels.
  const add = (name, ctxKey, html, node = {}, directive = {}, dir) => {
    const ctx = CTX[ctxKey];
    const full = {
      id: 'A',
      label: 'Shape text',
      labelType: 'text',
      labelStyle: '',
      padding: 15,
      cssStyles: [],
      cssCompiledStyles: [],
      cssClasses: 'default ',
      domId: 'flowchart-A-0',
      look: ctx.look,
      isGroup: false,
      shape: spec.ids[n % spec.ids.length],
      minWidth: 120,
      ...node,
    };
    for (const key of Object.keys(full)) {
      if (full[key] === undefined) {
        delete full[key];
      }
    }
    const c = {
      name: `${handler}/${name}-${ctxKey}-${html ? 'html' : 'svg'}`,
      type: 'flowchart-v2',
      directive: { look: ctx.look, theme: ctx.theme, ...(html ? {} : { htmlLabels: false }), ...directive },
      node: full,
    };
    if (dir) {
      c.dir = dir;
    }
    cases.push(c);
    n++;
  };

  // Every look, both themes and both label modes. The cells take turns
  // with the label variants, so that each variant runs in both modes.
  const htmlLabels = [{}, LABELS.long, LABELS.md, {}, LABELS.br, LABELS.chars];
  const svgLabels = [{}, LABELS.br, {}, LABELS.mdlines, LABELS.long, LABELS.empty];
  const labelName = (node) => Object.keys(LABELS).find((k) => LABELS[k] === node) ?? 'plain';
  Object.keys(CTX).forEach((ctxKey, i) => {
    if (spec.label === false) {
      add('base-plain', ctxKey, true);
      add('base-long', ctxKey, false, LABELS.long);
      return;
    }
    add(`base-${labelName(htmlLabels[i])}`, ctxKey, true, htmlLabels[i]);
    add(`base-${labelName(svgLabels[i])}`, ctxKey, false, svgLabels[i]);
  });
  if (spec.label !== false) {
    add('base-empty', 'cd', true, LABELS.empty);
    add('base-mdlines', 'nr', true, LABELS.mdlines);
  }

  // Styles.
  add('style-fill', 'cd', true, STYLES.fill);
  add('style-fill', 'hd', false, STYLES.fill);
  add('style-class', 'nr', false, STYLES.class);
  add('style-color', 'nr', true, STYLES.color);
  add('style-none', 'cd', false, { ...STYLES.none, labelStyle: 'color:#00f;font-weight:bold' });

  // Padding.
  add('pad-0', 'cd', true, { padding: 0 });
  add('pad-30', 'nd', false, { padding: 30 });
  if (spec.label !== false) {
    add('pad-undefined', 'hd', true, { padding: undefined });
    // No label and no padding: the shape has no size in the classic look.
    add('pad-0-empty', 'cd', false, { padding: 0, label: '' });
  }

  // Explicit sizes.
  if (spec.size !== false) {
    add('size-200-100', 'cd', true, { width: 200, height: 100 });
    add('size-200-100', 'nr', false, { width: 200, height: 100 });
    if (spec.size !== 'label') {
      add('size-300-x', 'cd', false, { width: 300, height: undefined });
      add('size-300-x', 'nd', true, { width: 300, height: undefined });
      add('size-x-150', 'cd', false, { width: undefined, height: 150 });
      add('size-x-150', 'nd', true, { width: undefined, height: 150 });
      add('size-5-5', 'cd', true, { width: 5, height: 5 });
      add('size-5-5', 'nd', true, { width: 5, height: 5 });
      add('size-60-40', 'hd', true, { width: 60, height: 40 });
    }
    add('size-400-300', 'cd', false, { width: 400, height: 300, label: LABELS.long.label });
  } else {
    add('size-200-100', 'cd', true, { width: 200, height: 100 });
  }

  // Other node properties and config values.
  add('link', 'cd', true, {
    link: 'https://example.com/',
    linkTarget: '_blank',
    tooltip: 'A tooltip',
    haveCallback: true,
    cssClasses: 'default clickable',
  });
  add(
    'link-sandbox-noid',
    'nd',
    true,
    { link: 'https://example.com/', tooltip: 'tip', domId: undefined, cssClasses: undefined, id: 'node-1' },
    { securityLevel: 'sandbox' }
  );
  add('seed', 'hd', false, {}, { handDrawnSeed: 42 });
  add('font-20-dark', 'cd', true, {}, { theme: 'dark', themeVariables: { fontSize: '20px' } }, 'LR');
  add('theme-forest', 'nd', false, {}, { theme: 'forest' });
  if (spec.label !== false) {
    add('icon', 'cd', false, { icon: 'fa:fa-user' });
    add('wrap-80', 'nd', true, { minWidth: undefined, wrappingWidth: 80, label: LABELS.long.label });
    add('node-html-center', 'cd', false, { useHtmlLabels: true, centerLabel: true, minWidth: 200 });
  }

  if (spec.label === false) {
    // These shapes take their colors from the theme.
    add('theme-neutral', 'cd', true, {}, { theme: 'neutral' });
    add('theme-base', 'nd', true, {}, { theme: 'base', themeVariables: { primaryColor: '#ffcc00' } });
    add('theme-redux', 'nd', true, {}, { theme: 'redux' });
    add('theme-redux-dark-color', 'cd', true, {}, { theme: 'redux-dark-color' });
    add('theme-dark', 'hd', true, {}, { theme: 'dark' });
  }

  // triangle reads flowchart.htmlLabels, and the label helper reads the
  // root htmlLabels.
  if (spec.flowchartHtmlLabels) {
    add('flowchart-html-false', 'cd', true, {}, { flowchart: { htmlLabels: false } });
    add('flowchart-html-false', 'nr', false, {}, { flowchart: { htmlLabels: false } });
    add('flowchart-html-true', 'cd', false, {}, { flowchart: { htmlLabels: true } });
  }
  if (!spec.heavy) {
    return cases;
  }
  // A heavy shape keeps 30 cases. The cases that go first are the ones
  // that differ from another case only in what the label helper does.
  const drop = [
    'base-empty-cd-html',
    'base-mdlines-nr-html',
    'style-color-nr-html',
    'pad-30-nd-svg',
    'size-300-x-nd-html',
    'size-x-150-cd-svg',
    'size-5-5-nd-html',
    'icon-cd-svg',
    'node-html-center-cd-svg',
    'theme-forest-nd-svg',
    'wrap-80-nd-html',
  ];
  let kept = cases;
  for (const name of drop) {
    if (kept.length <= 30) {
      break;
    }
    kept = kept.filter((c) => c.name !== `${handler}/${name}`);
  }
  return kept;
}

const argv = process.argv.slice(2);
let list = false;
let casesDir = '';
const handlers = [];
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--list') {
    list = true;
  } else if (argv[i] === '--cases-dir') {
    casesDir = argv[++i];
  } else if (SHAPES[argv[i]]) {
    handlers.push(argv[i]);
  } else {
    console.error(`unknown shape handler: ${argv[i]}`);
    process.exit(2);
  }
}
if (handlers.length === 0) {
  handlers.push(...Object.keys(SHAPES));
}
if (list) {
  for (const handler of handlers) {
    console.log(`${handler}: ${casesFor(handler).length} cases`);
  }
  process.exit(0);
}
const tmpDir = casesDir || fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-set2-'));
fs.mkdirSync(tmpDir, { recursive: true });
for (const handler of handlers) {
  const casesFile = path.join(tmpDir, `${handler}.cases.json`);
  fs.writeFileSync(casesFile, JSON.stringify({ cases: casesFor(handler) }));
  const outFile = path.join(outDir, `${handler}.json.gz`);
  execFileSync('node', [path.join(here, 'gen-shapes.mjs'), casesFile, outFile], { stdio: 'inherit' });
  // Write the file again in a form that compresses better. Cases with the
  // same config and style sheet go next to each other: a case repeats both,
  // and gzip finds a repetition only within 32 KB. The best level helps
  // with the long lists of numbers.
  const golden = JSON.parse(zlib.gunzipSync(fs.readFileSync(outFile)).toString('utf8'));
  const order = new Map();
  for (const c of golden) {
    const key = JSON.stringify(c.config) + c.css;
    if (!order.has(key)) {
      order.set(key, order.size);
    }
    c.order = order.get(key);
  }
  golden.sort((a, b) => a.order - b.order);
  for (const c of golden) {
    delete c.order;
  }
  fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden), { level: 9 }));
}
if (!casesDir) {
  fs.rmSync(tmpDir, { recursive: true });
}
