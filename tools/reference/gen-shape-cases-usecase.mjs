// Makes the shape cases of internal/diagrams/usecase (the shapes
// usecaseActor, usecaseActorHollow, usecaseActorAwesome, usecaseActorIcon,
// usecaseEllipse, usecaseBusiness and usecaseJsonTable) and records them from
// upstream mermaid in Chromium.
//
//   node gen-shape-cases-usecase.mjs
//
// It runs the shape probe (probes/shapes.js) one time and writes
//
//   internal/diagrams/usecase/testdata/shapes.json.gz
//
// The nodes have the properties that usecaseDb.getData() and
// prepareUsecaseLayoutData give them. The cases cover each look, themes with
// and without a color palette, HTML and SVG labels, short, long, empty,
// multi-line and markdown labels, styles, paddings and the properties that
// only one shape reads.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(here, '../../internal/diagrams/usecase/testdata');
const type = 'usecase';

// The config variants: the directive of the case, and the look that the db
// puts on the nodes for it.
const variants = [
  { name: 'neo', directive: {}, look: 'neo' },
  { name: 'neo-svg', directive: { htmlLabels: false }, look: 'neo' },
  { name: 'dark', directive: { theme: 'redux-dark-color' }, look: 'neo' },
  { name: 'classic', directive: { look: 'classic', theme: 'default' }, look: 'classic' },
  { name: 'classic-svg', directive: { look: 'classic', theme: 'default', htmlLabels: false }, look: 'classic' },
  { name: 'hand', directive: { look: 'handDrawn', theme: 'default' }, look: 'handDrawn' },
  { name: 'hand-svg', directive: { look: 'handDrawn', theme: 'forest', htmlLabels: false }, look: 'handDrawn' },
  { name: 'hand-color', directive: { look: 'handDrawn', theme: 'redux-color' }, look: 'handDrawn' },
];

const labels = {
  short: { label: 'Customer', labelType: 'text' },
  long: { label: 'A fairly long label that has to wrap over several lines of text', labelType: 'text' },
  markdown: { label: '**Bold** and *italic* words in a `markdown` label', labelType: 'markdown' },
  empty: { label: '', labelType: 'text' },
  br: { label: 'Line one<br/>Line two', labelType: 'text' },
  escaped: { label: '&lt;b&gt;not bold&lt;/b&gt; &amp; more', labelType: 'text' },
  unicode: { label: 'Zażółć gęślą jaźń 测试 ✓', labelType: 'text' },
};

let serial = 0;
// node is a node as getData and prepareUsecaseLayoutData make it.
const node = (shape, look, cssClasses, props) => {
  const id = `N${serial++}`;
  return {
    id,
    ...labels.short,
    shape,
    isGroup: false,
    padding: 10,
    look,
    cssClasses,
    cssStyles: [],
    cssCompiledStyles: [],
    wrappingWidth: 120,
    domId: `my-svg-usecase-${id}`,
    ...props,
  };
};

const styleSets = {
  fill: { cssStyles: ['fill:#fdf6e3', 'stroke:#6c71c4', 'stroke-width:3px'] },
  text: { cssStyles: ['color:#dc322f', 'font-weight:bold', 'fill:#eee'] },
  compiled: { cssCompiledStyles: ['fill:#fff3cd', 'stroke:#b58900'], cssStyles: ['stroke-dasharray:4 2'] },
  dash: { cssStyles: ['stroke-dasharray: 5', 'stroke-width: 2.5px', 'fill : #abc'] },
};

// The markup of a hand-drawn shape is large: its rough paths have many
// random numbers, which do not compress. So only these cases are recorded
// with the hand-drawn look. They cover the branches that draw with rough.
const actorHand = {
  hand: ['short', 'business', 'stereotype', 'style-fill', 'style-compiled', 'icon-no-prefix', 'icon-business'],
  'hand-svg': ['short', 'business-stereotype-long'],
  'hand-color': ['short'],
};
const handDrawnCases = {
  usecaseActor: actorHand,
  usecaseActorHollow: actorHand,
  usecaseActorAwesome: actorHand,
  usecaseActorIcon: actorHand,
  usecaseEllipse: {
    hand: ['short', 'long', 'style-fill', 'style-compiled', 'style-dash', 'no-css-styles'],
    'hand-svg': ['short', 'markdown'],
    'hand-color': ['short'],
  },
  usecaseBusiness: {
    hand: ['short', 'stereotype', 'style-fill', 'style-compiled'],
    'hand-svg': ['short', 'stereotype-long'],
    'hand-color': ['short'],
  },
  usecaseJsonTable: {
    hand: ['rows-none', 'rows-one', 'rows-array', 'style-fill', 'stroke-width-0'],
    'hand-svg': ['rows-scalars'],
    'hand-color': ['rows-one'],
  },
};

const cases = {};
const add = (shape, name, variant, n) => {
  if (variant.look === 'handDrawn' && !handDrawnCases[shape][variant.name].includes(name)) {
    return;
  }
  (cases[shape] ??= []).push({ name: `${shape}/${variant.name}/${name}`, type, directive: variant.directive, node: n });
};

// ---------------------------------------------------------------------------
// The actors.

const actors = [
  ['usecaseActor', 'normal', {}],
  ['usecaseActorHollow', 'hollow', {}],
  ['usecaseActorAwesome', 'awesome', {}],
  ['usecaseActorIcon', 'icon', { icon: 'fa:bell' }],
];
for (const [shape, actorType, own] of actors) {
  const actor = (variant, props = {}) => {
    const business = props.business ?? false;
    return node(shape, variant.look, `default usecase-actor usecase-actor-${actorType}${business ? ' usecase-business' : ''}`, {
      colorIndex: 0,
      actorType,
      business,
      ...own,
      ...props,
    });
  };
  for (const variant of variants) {
    add(shape, 'short', variant, actor(variant));
    add(shape, 'long', variant, actor(variant, labels.long));
    add(shape, 'markdown', variant, actor(variant, labels.markdown));
    add(shape, 'business', variant, actor(variant, { business: true, colorIndex: 1 }));
    add(shape, 'stereotype', variant, actor(variant, { stereotype: 'Employee', colorIndex: 2 }));
  }
  for (const variant of [variants[0], variants[1], variants[3], variants[5], variants[6]]) {
    add(shape, 'empty', variant, actor(variant, labels.empty));
    add(shape, 'br', variant, actor(variant, labels.br));
    add(shape, 'escaped', variant, actor(variant, labels.escaped));
    add(shape, 'unicode', variant, actor(variant, labels.unicode));
    add(shape, 'business-stereotype-long', variant, actor(variant, { ...labels.long, business: true, stereotype: 'A long stereotype text &amp; more', colorIndex: 11 }));
    add(shape, 'stereotype-markdown', variant, actor(variant, { ...labels.markdown, stereotype: 'Role' }));
    add(shape, 'style-fill', variant, actor(variant, styleSets.fill));
    add(shape, 'style-text', variant, actor(variant, { ...styleSets.text, stereotype: 'Styled' }));
    add(shape, 'style-compiled', variant, actor(variant, { ...styleSets.compiled, business: true }));
    add(shape, 'padding-0', variant, actor(variant, { padding: 0 }));
    add(shape, 'padding-default', variant, actor(variant, { padding: undefined }));
    add(shape, 'padding-30', variant, actor(variant, { padding: 30, stereotype: 'P' }));
    add(shape, 'narrow-wrap', variant, actor(variant, { ...labels.long, wrappingWidth: 60 }));
    add(shape, 'no-color-index', variant, actor(variant, { colorIndex: undefined }));
    add(shape, 'accessible-name', variant, actor(variant, { accessibleName: 'business actor Customer' }));
    add(shape, 'no-label', variant, actor(variant, { label: undefined }));
  }
  add(shape, 'link', variants[0], actor(variants[0], { link: 'https://example.com', linkTarget: '_blank', tooltip: 'A tooltip' }));
  add(shape, 'callback', variants[3], actor(variants[3], { haveCallback: true }));
}
// The icons of the icon actor.
for (const variant of [variants[0], variants[3], variants[5]]) {
  const icon = (name, props) =>
    add('usecaseActorIcon', name, variant, node('usecaseActorIcon', variant.look, 'default usecase-actor usecase-actor-icon', { colorIndex: 0, actorType: 'icon', business: false, ...props }));
  icon('icon-no-prefix', { icon: 'bell' });
  icon('icon-other-pack', { icon: 'mdi:account' });
  icon('icon-undefined', {});
  icon('icon-odd-name', { icon: 'fa:not registered!' });
  icon('icon-business', { icon: 'fa:user', business: true, stereotype: 'Bot' });
}

// ---------------------------------------------------------------------------
// The use cases.

for (const variant of variants) {
  const ellipse = (props = {}) =>
    node('usecaseEllipse', variant.look, 'default usecase-element usecase-ellipse', { padding: 20, colorIndex: 0, business: false, minWidth: 120, spreadPorts: true, ...props });
  add('usecaseEllipse', 'short', variant, ellipse());
  add('usecaseEllipse', 'long', variant, ellipse(labels.long));
  add('usecaseEllipse', 'markdown', variant, ellipse(labels.markdown));
  add('usecaseEllipse', 'empty', variant, ellipse(labels.empty));
  add('usecaseEllipse', 'br', variant, ellipse(labels.br));
  add('usecaseEllipse', 'escaped', variant, ellipse(labels.escaped));
  add('usecaseEllipse', 'unicode', variant, ellipse(labels.unicode));
  add('usecaseEllipse', 'folded-stereotype', variant, ellipse({ label: '«Core»<br/>Prepare quote', labelType: 'markdown', hasFoldedStereotype: true }));
  add('usecaseEllipse', 'style-fill', variant, ellipse(styleSets.fill));
  add('usecaseEllipse', 'style-text', variant, ellipse(styleSets.text));
  add('usecaseEllipse', 'style-compiled', variant, ellipse(styleSets.compiled));
  add('usecaseEllipse', 'style-dash', variant, ellipse(styleSets.dash));
  add('usecaseEllipse', 'no-min-width', variant, ellipse({ minWidth: undefined }));
  add('usecaseEllipse', 'min-width-300', variant, ellipse({ minWidth: 300 }));
  add('usecaseEllipse', 'padding-0', variant, ellipse({ padding: 0 }));
  add('usecaseEllipse', 'padding-undefined', variant, ellipse({ padding: undefined, colorIndex: 7 }));
  add('usecaseEllipse', 'narrow-wrap', variant, ellipse({ ...labels.long, wrappingWidth: 60, minWidth: 0 }));
  add('usecaseEllipse', 'no-css-styles', variant, ellipse({ cssStyles: undefined, colorIndex: undefined }));

  const business = (props = {}) =>
    node('usecaseBusiness', variant.look, 'default usecase-element usecase-ellipse usecase-business', {
      padding: 20,
      colorIndex: 0,
      business: true,
      minWidth: 120,
      spreadPorts: true,
      ...props,
    });
  add('usecaseBusiness', 'short', variant, business());
  add('usecaseBusiness', 'long', variant, business(labels.long));
  add('usecaseBusiness', 'markdown', variant, business(labels.markdown));
  add('usecaseBusiness', 'empty', variant, business(labels.empty));
  add('usecaseBusiness', 'br', variant, business(labels.br));
  add('usecaseBusiness', 'unicode', variant, business(labels.unicode));
  add('usecaseBusiness', 'stereotype', variant, business({ stereotype: 'Core', colorIndex: 4 }));
  add('usecaseBusiness', 'stereotype-long', variant, business({ ...labels.long, stereotype: 'A stereotype that is longer than the label &amp; wraps' }));
  add('usecaseBusiness', 'stereotype-markdown', variant, business({ ...labels.markdown, stereotype: 'Md' }));
  add('usecaseBusiness', 'style-fill', variant, business(styleSets.fill));
  add('usecaseBusiness', 'style-text', variant, business({ ...styleSets.text, stereotype: 'Styled' }));
  add('usecaseBusiness', 'style-compiled', variant, business(styleSets.compiled));
  add('usecaseBusiness', 'no-min-width', variant, business({ minWidth: undefined }));
  add('usecaseBusiness', 'padding-0', variant, business({ padding: 0 }));
  add('usecaseBusiness', 'padding-2', variant, business({ padding: 2, stereotype: 'S' }));
  add('usecaseBusiness', 'padding-60', variant, business({ padding: 60 }));
  add('usecaseBusiness', 'accessible-name', variant, business({ accessibleName: 'business use case Customer' }));
  add('usecaseBusiness', 'no-label', variant, business({ label: undefined, colorIndex: undefined }));
}

// ---------------------------------------------------------------------------
// The JSON tables.

const row = (key, value, accessibleKey = key) => ({ key, accessibleKey, value });
const rowSets = {
  none: [],
  one: [row('region', 'eu')],
  scalars: [row('2', 'two'), row('1', 'one'), row('flag', 'true'), row('nothing', 'null'), row('n', '1.5e-7')],
  array: [row('colors', 'Red'), row('', 'Green', 'colors'), row('', 'Blue', 'colors'), row('address.city', 'Oslo'), row('empty', '{}'), row('list', '[]')],
  long: [row('a.very.long.property.path[3].name', 'short'), row('k', 'A long value that is wider than the title of the table and does not wrap')],
  escaped: [row('&lt;key&gt;', '&lt;b&gt;bold&lt;/b&gt; &amp; x'), row('', '')],
  markdown: [row('**bold** key', '*italic* value'), row('multi', 'line one<br/>line two')],
};
for (const variant of variants) {
  const table = (rows, props = {}) =>
    node('usecaseJsonTable', variant.look, 'default usecase-json-table', { label: 'Payload', minWidth: 120, jsonRows: rows, ...props });
  for (const [name, rows] of Object.entries(rowSets)) {
    add('usecaseJsonTable', `rows-${name}`, variant, table(rows));
  }
  add('usecaseJsonTable', 'no-rows-property', variant, table(undefined));
  add('usecaseJsonTable', 'long-title', variant, table(rowSets.one, { label: 'A title that is wider than the cells of the table' }));
  add('usecaseJsonTable', 'empty-title', variant, table(rowSets.scalars, { label: '' }));
  add('usecaseJsonTable', 'style-fill', variant, table(rowSets.array, styleSets.fill));
  add('usecaseJsonTable', 'style-text', variant, table(rowSets.scalars, styleSets.text));
  add('usecaseJsonTable', 'style-compiled', variant, table(rowSets.one, styleSets.compiled));
  add('usecaseJsonTable', 'stroke-width-0', variant, table(rowSets.one, { cssStyles: ['stroke-width:0'] }));
  add('usecaseJsonTable', 'stroke-width-bad', variant, table(rowSets.one, { cssStyles: ['stroke-width:thick', 'stroke:#123456'] }));
  add('usecaseJsonTable', 'stroke-width-negative', variant, table(rowSets.one, { cssStyles: ['stroke-width:-2px'] }));
  add('usecaseJsonTable', 'stroke-width-em', variant, table(rowSets.scalars, { cssStyles: ['stroke-width: 1.5em'] }));
  add('usecaseJsonTable', 'no-min-width', variant, table(rowSets.one, { minWidth: undefined }));
  add('usecaseJsonTable', 'accessible-name', variant, table(rowSets.one, { accessibleName: 'Payload: region: eu' }));
  add('usecaseJsonTable', 'narrow-wrap', variant, table(rowSets.long, { wrappingWidth: 60 }));
}

// ---------------------------------------------------------------------------
// Record the cases: one run of the shape probe for all of them.

const all = Object.values(cases).flat();
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'usecase-shapes-'));
const casesFile = path.join(tmp, 'cases.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(casesFile, JSON.stringify({ cases: all, samples: { usecase: 'usecase-beta\n  actor A\n' } }));
execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/shapes.js'), '--args', casesFile, '--out', resultFile], {
  stdio: 'inherit',
});
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });
if (results.length !== all.length) {
  throw new Error(`expected ${all.length} results, got ${results.length}`);
}

// The golden file is the list of cases of gen-shapes.mjs, in a compact form:
// the config and the style sheet of a case are the same for every case of a
// config variant, so the file has each of them one time, and a case has
// their indexes.
const table = (list) => {
  const index = new Map();
  return {
    list,
    add(value) {
      const key = JSON.stringify(value);
      if (!index.has(key)) {
        index.set(key, list.length);
        list.push(value);
      }
      return index.get(key);
    },
  };
};
const configs = table([]);
const sheets = table([]);
const golden = all.map((c, i) => {
  const { config, css, ...result } = results[i];
  const rec = { ...c, directive: undefined, ...result };
  if (config !== undefined) {
    rec.config = configs.add(config);
  }
  if (css !== undefined) {
    rec.css = sheets.add(css);
  }
  return rec;
});
const outFile = path.join(outDir, 'shapes.json.gz');
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ configs: configs.list, css: sheets.list, cases: golden }), { level: 9 }));
const perShape = Object.entries(cases).map(([shape, list]) => `${shape} ${list.length}`).join(', ');
console.log(
  `${outFile}: ${golden.length} cases (${perShape}), ${golden.filter((g) => g.error).length} errors upstream, ` +
    `${configs.list.length} configs, ${(fs.statSync(outFile).size / 1024).toFixed(0)} KiB`
);
