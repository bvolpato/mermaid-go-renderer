// Builds the shape cases of internal/ru/shapes/set1 and records them with
// gen-shapes.mjs (upstream mermaid in Chromium).
//
//   node gen-shapes-set1.mjs            every shape
//   node gen-shapes-set1.mjs stadium    the named shapes (handler names)
//   node gen-shapes-set1.mjs --cases    only print the number of cases
//
// One golden file per handler: internal/ru/shapes/set1/testdata/<handler>.json.gz.
// The cases cover the looks, the themes, both label modes, label variants,
// styles, paddings, explicit sizes, links, and the node shapes of the
// diagram types that use a shape (flowchart, state diagram, mindmap).
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(here, '../../internal/ru/shapes/set1/testdata');

const LONG = 'This is a long label that must wrap over several lines to fit';

// Node objects as the diagram dbs build them (see flowDb.getData,
// stateDb.getData and mindmapDb.getData).
const flowNode = (shape, look, extra = {}) => ({
  id: 'A',
  label: 'Start',
  labelType: 'text',
  labelStyle: '',
  padding: 15,
  cssStyles: [],
  cssCompiledStyles: [],
  cssClasses: 'default ',
  domId: 'my-svg-flowchart-A-0',
  look,
  isGroup: false,
  shape,
  minWidth: 120,
  ...extra,
});
const stateNode = (shape, look, extra = {}) => ({
  labelStyle: '',
  shape,
  label: 'root_start',
  cssClasses: ' statediagram-state',
  cssCompiledStyles: [],
  cssStyles: [],
  id: 'root_start',
  domId: 'my-svg-state-root_start-0',
  isGroup: false,
  padding: 8,
  rx: 10,
  ry: 10,
  look,
  labelType: 'markdown',
  centerLabel: true,
  wrappingWidth: 120,
  minWidth: 120,
  ...extra,
});
const mindmapNode = (shape, look, extra = {}) => ({
  id: '1',
  domId: 'my-svg-node_1',
  label: 'Idea',
  labelType: 'markdown',
  isGroup: false,
  shape,
  width: 200,
  height: 0,
  padding: 10,
  cssClasses: 'mindmap-node section-0',
  cssStyles: [],
  centerLabel: true,
  look,
  level: 1,
  nodeId: 'A',
  type: 5,
  section: 0,
  ...extra,
});

const directive = (look, theme, html, extra = {}) => ({
  look,
  theme,
  ...(html ? {} : { htmlLabels: false }),
  ...extra,
});

class Builder {
  constructor(handler, ids) {
    this.handler = handler;
    this.ids = ids;
    this.n = 0;
    this.cases = [];
  }
  // The shape ids rotate, so that every alias reaches the handler.
  id() {
    return this.ids[this.n++ % this.ids.length];
  }
  flow(name, look, theme, html, nodeExtra = {}, directiveExtra = {}, dir) {
    this.cases.push({
      name: `${this.handler}/${name}`,
      type: 'flowchart-v2',
      directive: directive(look, theme, html, directiveExtra),
      node: flowNode(this.id(), look, nodeExtra),
      dir,
    });
  }
  state(name, look, theme, html, nodeExtra = {}, directiveExtra = {}, dir) {
    this.cases.push({
      name: `${this.handler}/${name}`,
      type: 'stateDiagram',
      directive: directive(look, theme, html, directiveExtra),
      node: stateNode(this.id(), look, nodeExtra),
      dir,
    });
  }
  mindmap(name, look, theme, html, nodeExtra = {}, directiveExtra = {}) {
    this.cases.push({
      name: `${this.handler}/${name}`,
      type: 'mindmap',
      directive: directive(look, theme, html, directiveExtra),
      node: mindmapNode(this.id(), look, nodeExtra),
    });
  }
}

const STYLES = ['fill:#f9f', 'stroke:#333', 'stroke-width:4px'];
const LINK = { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'tip', haveCallback: true };
const BARE = {
  domId: undefined,
  cssClasses: undefined,
  cssStyles: undefined,
  cssCompiledStyles: undefined,
  labelStyle: undefined,
  minWidth: undefined,
};

// The cases of a shape that draws a label. Most cases use the default
// theme: its config and style sheet are small, so gzip stores them once for
// a run of cases. The redux-color theme is the default of a flowchart.
function labelled(b) {
  b.flow('classic-html', 'classic', 'default', true);
  b.flow('classic-svg', 'classic', 'default', false);
  b.flow('neo-html', 'neo', 'default', true);
  b.flow('neo-svg', 'neo', 'default', false);
  // Label variants.
  b.flow('long-classic-html', 'classic', 'default', true, { label: LONG });
  b.flow('long-neo-svg', 'neo', 'default', false, { label: LONG });
  b.flow('long-nomin-classic-svg', 'classic', 'default', false, { label: LONG, minWidth: undefined });
  b.flow('wrap60-neo-html', 'neo', 'default', true, { label: LONG, wrappingWidth: 60, minWidth: undefined });
  b.flow('empty-classic-html', 'classic', 'default', true, { label: '' });
  b.flow('empty-neo-svg', 'neo', 'default', false, { label: '' });
  b.flow('br-classic-html', 'classic', 'default', true, { label: 'Line one<br/>the second line' });
  b.flow('br-neo-svg', 'neo', 'default', false, { label: 'Line one<br>the second line' });
  b.flow('markdown-neo-html', 'neo', 'default', true, {
    label: '**Bold** and *italic* text',
    labelType: 'markdown',
  });
  b.flow('markdown-classic-svg', 'classic', 'default', false, {
    label: 'A **bold** word\nand a *second* line',
    labelType: 'markdown',
  });
  b.flow('entity-icon-classic-svg', 'classic', 'default', false, {
    label: 'Fish &amp; chips #9829; fa:fa-car',
  });
  // Styles.
  b.flow('styles-classic-html', 'classic', 'default', true, { cssStyles: STYLES });
  b.flow('styles-neo-svg', 'neo', 'default', false, { cssStyles: STYLES });
  b.flow('label-styles-classic-html', 'classic', 'default', true, {
    cssStyles: ['color:#fff', 'font-size:20px', 'fill:#036'],
  });
  b.flow('label-styles-neo-svg', 'neo', 'default', false, {
    cssStyles: ['color:#fff', 'font-weight:bold', 'fill:#036', 'stroke:#f00'],
  });
  b.flow('compiled-neo-html', 'neo', 'default', true, {
    cssCompiledStyles: ['fill:#f00', 'color:#fff', 'stroke:#00f'],
    cssClasses: 'default foo',
  });
  // Paddings.
  b.flow('padding0-classic-html', 'classic', 'default', true, { padding: 0 });
  b.flow('padding8-neo-svg', 'neo', 'default', false, { padding: 8 });
  b.flow('padding-undefined-classic-svg', 'classic', 'default', false, { padding: undefined });
  // Explicit sizes.
  b.flow('size-200x100-classic-html', 'classic', 'default', true, { width: 200, height: 100 });
  b.flow('size-200x100-neo-svg', 'neo', 'default', false, { width: 200, height: 100 });
  b.flow('size-40x20-classic-svg', 'classic', 'default', false, { width: 40, height: 20 });
  b.flow('size-width-neo-html', 'neo', 'default', true, { width: 300, label: LONG });
  b.flow('size-height-classic-html', 'classic', 'default', true, { height: 150 });
  // Link, tooltip, callback, and missing optional properties.
  b.flow('link-classic-html', 'classic', 'default', true, { ...LINK, cssClasses: 'default clickable' });
  b.flow('bare-neo-html', 'neo', 'default', true, BARE);
  // The redux-color theme.
  b.flow('redux-neo-html', 'neo', 'redux-color', true);
  b.flow('redux-neo-svg', 'neo', 'redux-color', false);
  b.flow('redux-classic-html', 'classic', 'redux-color', true);
  b.flow('redux-long-styles-neo-svg', 'neo', 'redux-color', false, { label: LONG, cssStyles: STYLES });
  hand(b);
}

// The hand-drawn cases of a shape that draws a label. The markup of a
// hand-drawn shape is large, so three of the four cases have a small shape.
// The last case has a fixed seed: roughjs then uses its own random numbers.
function hand(b) {
  b.flow('hand-redux-html', 'handDrawn', 'redux-color', true);
  b.flow('hand-styles-svg', 'handDrawn', 'default', false, {
    label: 'Hi',
    minWidth: undefined,
    cssStyles: [...STYLES, 'stroke-dasharray: 5 5', 'color:#00f'],
  });
  b.flow('hand-size-compiled-html', 'handDrawn', 'default', true, {
    width: 100,
    height: 56,
    cssCompiledStyles: ['fill:#f00', 'color:#fff'],
    cssClasses: 'default foo',
  });
  b.flow('hand-seed-empty-svg', 'handDrawn', 'default', false, { label: '' }, { handDrawnSeed: 42 });
}

// A smaller set for a shape whose markup is large in every look.
function labelledCompact(b) {
  b.flow('classic-html', 'classic', 'default', true);
  b.flow('classic-svg', 'classic', 'default', false);
  b.flow('neo-html', 'neo', 'default', true);
  b.flow('neo-svg', 'neo', 'default', false);
  b.flow('long-classic-html', 'classic', 'default', true, { label: LONG });
  b.flow('long-neo-svg', 'neo', 'default', false, { label: LONG });
  b.flow('wrap60-neo-html', 'neo', 'default', true, { label: LONG, wrappingWidth: 60, minWidth: undefined });
  b.flow('empty-classic-html', 'classic', 'default', true, { label: '' });
  b.flow('empty-neo-svg', 'neo', 'default', false, { label: '' });
  b.flow('br-neo-svg', 'neo', 'default', false, { label: 'Line one<br>the second line' });
  b.flow('markdown-neo-html', 'neo', 'default', true, {
    label: '**Bold** and *italic* text',
    labelType: 'markdown',
  });
  b.flow('styles-classic-html', 'classic', 'default', true, { cssStyles: STYLES });
  b.flow('styles-neo-svg', 'neo', 'default', false, { cssStyles: STYLES });
  b.flow('label-styles-classic-html', 'classic', 'default', true, {
    cssStyles: ['color:#fff', 'font-size:20px', 'fill:#036'],
  });
  b.flow('compiled-neo-html', 'neo', 'default', true, {
    cssCompiledStyles: ['fill:#f00', 'color:#fff', 'stroke:#00f'],
    cssClasses: 'default foo',
  });
  b.flow('padding0-classic-html', 'classic', 'default', true, { padding: 0 });
  b.flow('padding8-neo-svg', 'neo', 'default', false, { padding: 8 });
  b.flow('padding-undefined-classic-svg', 'classic', 'default', false, { padding: undefined });
  b.flow('size-200x100-classic-html', 'classic', 'default', true, { width: 200, height: 100 });
  b.flow('size-40x20-neo-svg', 'neo', 'default', false, { width: 40, height: 20 });
  b.flow('link-classic-html', 'classic', 'default', true, { ...LINK, cssClasses: 'default clickable' });
  b.flow('bare-neo-html', 'neo', 'default', true, BARE);
  b.flow('redux-neo-html', 'neo', 'redux-color', true);
  b.flow('redux-classic-svg', 'classic', 'redux-color', false);
  b.flow('redux-long-styles-neo-svg', 'neo', 'redux-color', false, { label: LONG, cssStyles: STYLES });
  hand(b);
}

// The cases of a shape that draws no label.
function unlabelled(b) {
  b.flow('classic', 'classic', 'default', true);
  b.flow('neo', 'neo', 'default', true);
  b.flow('styles-classic', 'classic', 'default', true, { cssStyles: STYLES });
  b.flow('styles-neo', 'neo', 'default', true, { cssStyles: STYLES });
  b.flow('label-styles-classic', 'classic', 'default', true, { cssStyles: ['color:#fff', 'fill:#036'] });
  b.flow('compiled-neo', 'neo', 'default', true, {
    cssCompiledStyles: ['fill:#f00', 'color:#fff', 'stroke:#00f'],
    cssClasses: 'default foo',
  });
  b.flow('size-200x100-classic', 'classic', 'default', true, { width: 200, height: 100 });
  b.flow('size-200x100-neo', 'neo', 'default', true, { width: 200, height: 100 });
  b.flow('size-10x8-classic', 'classic', 'default', true, { width: 10, height: 8 });
  b.flow('size-10x8-neo', 'neo', 'default', true, { width: 10, height: 8 });
  b.flow('size-width-classic', 'classic', 'default', true, { width: 40 });
  b.flow('size-height-neo', 'neo', 'default', true, { height: 40 });
  b.flow('size-zero-classic', 'classic', 'default', true, { width: 0, height: 0 });
  b.flow('link-classic', 'classic', 'default', true, LINK);
  b.flow('bare-neo', 'neo', 'default', true, { ...BARE, padding: undefined });
  b.state('state-classic', 'classic', 'default', true);
  b.state('state-neo', 'neo', 'default', true);
  b.state('state-size-neo', 'neo', 'default', true, { width: 30, height: 30 });
  b.flow('redux-neo', 'neo', 'redux-color', true);
  b.flow('redux-classic', 'classic', 'redux-color', true);
  b.state('state-redux-neo', 'neo', 'redux-color', true);
  b.flow('redux-link-neo', 'neo', 'redux-color', true, LINK);
  b.flow('hand-redux', 'handDrawn', 'redux-color', true);
  b.flow('hand-styles', 'handDrawn', 'default', true, {
    cssStyles: [...STYLES, 'stroke-dasharray: 5 5'],
  });
  b.flow('hand-size', 'handDrawn', 'default', true, { width: 60, height: 40 });
  b.flow('hand-seed', 'handDrawn', 'default', true, {}, { handDrawnSeed: 42 });
  b.state('state-hand', 'handDrawn', 'default', true);
}

// Other themes, for the shapes that read theme variables.
function themes(b) {
  b.flow('dark-classic', 'classic', 'dark', true);
  b.flow('redux-dark-neo', 'neo', 'redux-dark-color', true);
  b.flow('neutral-hand', 'handDrawn', 'neutral', true, { label: '' });
}

// The nodes of a mindmap: an explicit width and a height of 0.
function mindmap(b) {
  b.mindmap('mindmap-classic-html', 'classic', 'default', true);
  b.mindmap('mindmap-neo-svg', 'neo', 'default', false);
  b.mindmap('mindmap-long-classic-html', 'classic', 'default', true, { label: LONG });
  b.mindmap('mindmap-padding-neo-html', 'neo', 'default', true, { padding: 20, type: 2 });
  b.mindmap('mindmap-hand-html', 'handDrawn', 'default', true);
}

const SHAPES = {
  stadium: { ids: ['stadium', 'terminal', 'pill'], build: labelledCompact },
  subroutine: { ids: ['subroutine', 'fr-rect', 'subprocess'], build: labelled },
  cylinder: { ids: ['cylinder', 'cyl', 'db', 'database'], build: labelled },
  datastore: {
    ids: ['datastore', 'data-store'],
    build(b) {
      labelled(b);
      b.flow('label-padding-classic-html', 'classic', 'default', true, { labelPaddingX: 4, labelPaddingY: 2 });
      b.flow('rounded-neo-html', 'neo', 'default', true, { rx: 10, ry: 10 });
      b.flow('hand-label-padding-html', 'handDrawn', 'default', true, { labelPaddingX: 40, label: '' });
      b.flow('hand-rounded-html', 'handDrawn', 'default', true, { rx: 10, ry: 10, label: '' });
      b.flow('hand-fill-none-html', 'handDrawn', 'default', true, { cssStyles: ['fill:none'], label: '' });
    },
  },
  folder: { ids: ['folder', 'directory'], build: labelled },
  bucket: {
    ids: ['bucket'],
    build(b) {
      labelled(b);
      themes(b);
    },
  },
  consoleWindow: {
    ids: ['console'],
    build(b) {
      labelled(b);
      themes(b);
    },
  },
  browser: {
    ids: ['browser'],
    build(b) {
      labelled(b);
      themes(b);
    },
  },
  person: {
    ids: ['person'],
    build(b) {
      labelled(b);
      b.flow('size-tall-classic-html', 'classic', 'default', true, { height: 400 });
      b.flow('size-wide-neo-svg', 'neo', 'default', false, { width: 500, height: 60 });
    },
  },
  circle: {
    ids: ['circle', 'circ'],
    build(b) {
      labelled(b);
      mindmap(b);
    },
  },
  bang: {
    ids: ['bang'],
    build(b) {
      labelled(b);
      mindmap(b);
    },
  },
  cloud: {
    ids: ['cloud'],
    build(b) {
      labelled(b);
      mindmap(b);
    },
  },
  question: { ids: ['diamond', 'diam', 'decision', 'question'], build: labelled },
  hexagon: {
    ids: ['hexagon', 'hex', 'prepare'],
    build(b) {
      labelled(b);
      mindmap(b);
    },
  },
  lean_right: { ids: ['lean_right', 'lean-r', 'in-out', 'lean-right'], build: labelled },
  lean_left: { ids: ['lean_left', 'lean-l', 'out-in', 'lean-left'], build: labelled },
  trapezoid: { ids: ['trapezoid', 'trap-b', 'priority', 'trapezoid-bottom'], build: labelled },
  inv_trapezoid: { ids: ['inv_trapezoid', 'trap-t', 'manual', 'inv-trapezoid'], build: labelled },
  doublecircle: { ids: ['doublecircle', 'dbl-circ', 'double-circle'], build: labelled },
  text: { ids: ['text'], build: labelled },
  card: { ids: ['card', 'notch-rect', 'notched-rectangle'], build: labelled },
  shadedProcess: { ids: ['shaded-process', 'lin-rect', 'lin-proc', 'lined-process'], build: labelled },
  stateStart: {
    ids: ['stateStart', 'start', 'sm-circ', 'small-circle'],
    build(b) {
      unlabelled(b);
      themes(b);
    },
  },
  stateEnd: {
    ids: ['stateEnd', 'stop', 'fr-circ', 'framed-circle'],
    build(b) {
      unlabelled(b);
      themes(b);
    },
  },
  forkJoin: {
    ids: ['fork', 'join', 'forkJoin'],
    build(b) {
      unlabelled(b);
      themes(b);
      for (const dir of ['LR', 'RL', 'TB', 'BT']) {
        b.state(`state-dir-${dir}-classic`, 'classic', 'default', true, {}, {}, dir);
      }
      b.flow('dir-LR-neo', 'neo', 'default', true, {}, {}, 'LR');
      b.flow('dir-LR-size-classic', 'classic', 'default', true, { width: 30, height: 120 }, {}, 'LR');
      b.flow('hand-dir-RL', 'handDrawn', 'default', true, {}, {}, 'RL');
      b.state('state-padding-classic', 'classic', 'default', true, {}, { state: { padding: 20 } });
      b.state('state-padding0-neo', 'neo', 'default', true, {}, { state: { padding: 0 } }, 'LR');
    },
  },
  hourglass: { ids: ['hourglass', 'collate'], build: unlabelled },
};

const argv = process.argv.slice(2);
const onlyCount = argv.includes('--cases');
const names = argv.filter((a) => !a.startsWith('--'));
for (const name of names) {
  if (!SHAPES[name]) {
    console.error(`unknown shape ${name}; known: ${Object.keys(SHAPES).join(', ')}`);
    process.exit(2);
  }
}
let total = 0;
for (const handler of names.length ? names : Object.keys(SHAPES)) {
  const b = new Builder(handler, SHAPES[handler].ids);
  SHAPES[handler].build(b);
  // Order the cases so that cases with the same config and style sheet are
  // neighbors. gzip then stores a small theme once for a run of cases.
  const rank = (c) => {
    const hand = c.directive.look === 'handDrawn';
    const small = c.directive.theme === 'default';
    if (c.type === 'flowchart-v2' && small) {
      return hand ? 1 : 0;
    }
    if (small && !hand) {
      return 2;
    }
    return hand ? 4 : 3;
  };
  const key = (c) => `${rank(c)} ${c.type} ${c.directive.theme}`;
  b.cases = b.cases
    .map((c, i) => ({ c, i }))
    .sort((x, y) => (key(x.c) < key(y.c) ? -1 : key(x.c) > key(y.c) ? 1 : x.i - y.i))
    .map((x) => x.c);
  total += b.cases.length;
  if (onlyCount) {
    console.log(`${handler}: ${b.cases.length}`);
    continue;
  }
  const tmp = path.join(os.tmpdir(), `mmdg-set1-${handler}-${process.pid}.json`);
  fs.writeFileSync(tmp, JSON.stringify({ cases: b.cases }));
  try {
    execFileSync('node', [path.join(here, 'gen-shapes.mjs'), tmp, path.join(outDir, `${handler}.json.gz`)], {
      stdio: 'inherit',
    });
  } finally {
    fs.rmSync(tmp);
  }
}
console.error(`${total} cases`);
