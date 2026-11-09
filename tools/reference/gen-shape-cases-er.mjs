// Makes the shape cases of internal/diagrams/er (the shape erBox) and
// records them from upstream mermaid in Chromium.
//
//   node gen-shape-cases-er.mjs
//
// It writes two golden files with gen-shapes.mjs (probes/shapes.js):
//
//   internal/diagrams/er/testdata/erBox.json.gz         hand-made nodes for the branches of the shape
//   internal/diagrams/er/testdata/erBox_corpus.json.gz  nodes of db.getData() of corpus fixtures
//
// The nodes of the corpus come from internal/diagrams/er/testdata/db.json.gz,
// which gen-er-db.mjs writes: run that script first.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(here, '../../internal/diagrams/er/testdata');
const type = 'er';

// The look of an ER diagram when the config sets none.
const defaultLook = 'neo';

const attr = (type, name, keys = [], comment = '') => ({ type, name, keys, comment });

// entity is a node of an entity, as erDb.getData() makes it and as
// render() changes it (the DOM id).
const entity = (label, attributes, look) => ({
  id: `entity-${label}-0`,
  label,
  attributes,
  alias: '',
  shape: 'erBox',
  look,
  cssClasses: 'default',
  cssStyles: [],
  labelType: 'markdown',
  cssCompiledStyles: [],
  colorIndex: 0,
  isGroup: false,
  domId: `my-svg-entity-${label}-0`,
});

const sets = {
  plain: [attr('string', 'name'), attr('int', 'age')],
  one: [attr('string', 'name')],
  keys: [attr('string', 'id', ['PK']), attr('int', 'ref', ['FK']), attr('date', 'day', ['PK', 'FK']), attr('x', 'y', ['UK'])],
  comments: [attr('string', 'name', [], 'the name'), attr('int', 'age', [], '')],
  full: [
    attr('string', 'id', ['PK'], 'the identifier'),
    attr('varchar(99)', 'name', [], 'a longer comment about the name'),
    attr('int', 'count', ['FK', 'UK']),
    attr('date', 'created'),
  ],
  generics: [attr('type~T~', 'generic'), attr('List~int~', 'numbers'), attr('Map~K,V~', 'pairs', ['PK'], 'has ~tilde~ text'), attr('a~b', 'c~d~e~')],
  many: Array.from({ length: 7 }, (_, i) => attr(`type${i}`, `name${i}`, i % 3 === 0 ? ['PK'] : [], i % 2 === 0 ? `comment ${i}` : '')),
  markdown: [attr('**bold**', '*italic*', [], 'a `code` and **bold** comment'), attr('_under_', 'a_b_c', ['PK'], '__strong__ text')],
  unicode: [attr('строка', 'имя', [], 'комментарий ζήτα 😀 ぼ'), attr('Ünï', 'cödé', ['FK'])],
  // Han and Hangul text: its width in a serif font needs the serif CJK
  // fonts of the system.
  han: [attr('文字列', '名前', [], '日本語 한국어')],
  html: [attr('<b>t</b>', 'n<br/>m', [], 'a <i>c</i> & d'), attr('a&lt;b', 'c&amp;d', [], 'x < y > z')],
  special: [attr('string[]', 'list'), attr('decimal(10,2)', 'price'), attr('a.b', 'c-d'), attr('*star', '_under'), attr('string?', 'nullable')],
  long: [attr('a_very_long_type_name_for_the_first_column', 'an_even_longer_name_for_the_second_column_of_the_table', ['PK', 'FK', 'UK'], 'A comment that is long enough to make the last column the widest column of the table')],
  empties: [attr('', ''), attr('t', '', [], 'c'), attr('', 'n', ['PK'])],
};

// The hand-drawn look fills each box with hatching lines: its markup is
// large and does not compress. Most cases of that look have a small table
// and small paddings.
const tiny = [attr('a', 'b', ['PK'], 'c'), attr('d', 'e')];
const small = { er: { diagramPadding: 2, entityPadding: 2 } };

const cases = [];
const add = (name, directive, node = {}, extra = {}) => {
  const look = directive.look ?? defaultLook;
  const handDrawn = look === 'handDrawn';
  const label = node.label ?? (handDrawn ? 'E' : 'CUSTOMER');
  const merged = { ...entity(label, node.attributes ?? (handDrawn ? tiny : sets.full), look), ...node };
  for (const key of Object.keys(merged)) {
    if (merged[key] === undefined) {
      delete merged[key];
    }
  }
  cases.push({ name: `erBox/${name}`, type, directive, node: merged, ...extra });
};

// The label modes: htmlLabels undefined (HTML labels, with the padding of
// SVG labels), true and false.
const modes = [
  ['html', {}],
  ['html-true', { htmlLabels: true }],
  ['svg', { htmlLabels: false }],
];

// Looks, themes and label modes, for a table and for a plain box.
for (const look of ['classic', 'neo']) {
  for (const theme of ['default', 'redux-color']) {
    for (const [mode, directive] of modes) {
      add(`table-${look}-${theme}-${mode}`, { look, theme, ...directive });
      add(`box-${look}-${theme}-${mode}`, { look, theme, ...directive }, { attributes: [] });
    }
  }
}
add('table-handDrawn-default-html', { look: 'handDrawn', theme: 'default', ...small });
add('table-handDrawn-default-svg', { look: 'handDrawn', theme: 'default', htmlLabels: false, ...small });
add('table-handDrawn-redux-color-html-true', { look: 'handDrawn', theme: 'redux-color', htmlLabels: true, ...small });
add('box-handDrawn-default-html', { look: 'handDrawn', theme: 'default', ...small }, { attributes: [] });
add('box-handDrawn-default-html-true', { look: 'handDrawn', theme: 'default', htmlLabels: true, ...small }, { attributes: [] });
add('box-handDrawn-redux-color-svg', { look: 'handDrawn', theme: 'redux-color', htmlLabels: false, ...small }, { attributes: [] });
// The hand-drawn look with the paddings of the default config.
add('box-handDrawn-default-padding', { look: 'handDrawn', theme: 'default' }, { attributes: [] });
add('table-no-directive', {});
add('box-no-directive', {}, { attributes: [] });
for (const theme of ['dark', 'forest', 'neutral', 'base', 'redux', 'redux-dark', 'neo-dark']) {
  add(`table-theme-${theme}`, { look: 'neo', theme });
}
for (const theme of ['dark', 'redux-dark-color']) {
  add(`box-theme-${theme}`, { look: 'neo', theme }, { attributes: [] });
}
add('table-theme-dark-handdrawn', { look: 'handDrawn', theme: 'dark', ...small });
add('box-handdrawn-seed', { look: 'handDrawn', theme: 'default', handDrawnSeed: 7, ...small }, { attributes: [] });
add('table-flowchart-html-key', { look: 'classic', theme: 'default', flowchart: { htmlLabels: false } });
add('box-flowchart-html-key', { look: 'classic', theme: 'default', flowchart: { htmlLabels: false } }, { attributes: [] });

// The attribute lists.
for (const [name, attributes] of Object.entries(sets)) {
  if (name === 'full') {
    continue;
  }
  add(`set-${name}-html`, { look: 'neo', theme: 'redux-color' }, { attributes });
}
for (const name of ['generics', 'markdown', 'han', 'html', 'empties']) {
  add(`set-${name}-svg`, { look: 'classic', theme: 'default', htmlLabels: false }, { attributes: sets[name] });
}
add('set-generics-html-true', { look: 'classic', theme: 'default', htmlLabels: true }, { attributes: sets.generics });
add('set-three-handdrawn-svg', { look: 'handDrawn', theme: 'default', htmlLabels: false, ...small }, { attributes: [attr('a', 'b'), attr('c', 'd'), attr('e', 'f')] });

// The name of the entity: wider than the columns, with and without the
// optional columns.
const longName = 'AN_ENTITY_WITH_A_NAME_THAT_IS_WIDER_THAN_ALL_OF_ITS_COLUMNS';
for (const set of ['plain', 'keys', 'comments', 'full', 'one']) {
  add(`long-name-${set}-html`, { look: 'neo', theme: 'redux-color' }, { label: longName, attributes: sets[set] });
}
add('long-name-plain-svg', { look: 'neo', theme: 'redux-color', htmlLabels: false }, { label: longName, attributes: sets.plain });
add('long-name-full-svg', { look: 'neo', theme: 'redux-color', htmlLabels: false }, { label: longName, attributes: sets.full });
for (const [mode, directive] of [modes[0], modes[2]]) {
  add(`long-name-box-${mode}`, { look: 'neo', theme: 'redux-color', ...directive }, { label: longName, attributes: [] });
  add(`name-generics-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: 'List~Item~', attributes: sets.one });
  add(`name-markdown-box-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: 'A **bold** and *italic* name', attributes: [] });
  add(`name-br-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: 'First line<br/>Second line', attributes: sets.plain });
  add(`name-unicode-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: 'Ünïcödé ζήτα 😀 ぼ', attributes: sets.one });
  add(`name-han-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: '日本語 한국어', attributes: sets.one });
  add(`name-empty-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: '', attributes: sets.plain });
  add(`name-empty-no-rows-${mode}`, { look: 'classic', theme: 'default', ...directive }, { label: '', attributes: [] });
  add(`alias-${mode}`, { look: 'neo', theme: 'redux-color', ...directive }, { label: 'p', alias: 'Photograph', attributes: sets.plain });
  add(`alias-box-${mode}`, { look: 'neo', theme: 'redux-color', ...directive }, { label: 'p', alias: 'Photograph', attributes: [] });
}
add('short-name-box', { look: 'classic', theme: 'default' }, { label: 'A', attributes: [] });
add('name-generics-box', { look: 'classic', theme: 'default' }, { label: 'List~Item~', attributes: [] });
add('name-markdown', { look: 'classic', theme: 'default' }, { label: 'A **bold** and *italic* name', attributes: sets.one });
add('name-br-box', { look: 'classic', theme: 'default' }, { label: 'First line<br/>Second line', attributes: [] });
add('name-entities-box', { look: 'classic', theme: 'default' }, { label: 'a &amp; b #quot;c#quot; <b>d</b>', attributes: [] });
add('name-text-type-box', { look: 'classic', theme: 'default' }, { label: 'Not **bold** here', labelType: 'text', attributes: [] });
add('alias-empty-label-box', { look: 'classic', theme: 'default' }, { label: '', alias: 'Only alias', attributes: [] });
// The label of the node is undefined.
cases.push({
  name: 'erBox/label-undefined',
  type,
  directive: { look: 'classic', theme: 'default' },
  node: (() => {
    const n = entity('X', sets.plain, 'classic');
    delete n.label;
    return n;
  })(),
});

// Styles.
const nodeStyles = ['fill:#f9f', 'stroke:#333', 'stroke-width:4px'];
const labelStyles = ['color:red', 'font-weight:bold', 'font-size:20px', 'fill:#ff0'];
const compiled = ['fill:#f00', 'color:white', 'stroke-width:2px', 'stroke:yellow'];
add('css-styles-classic-default', { look: 'classic', theme: 'default' }, { cssStyles: nodeStyles });
add('css-styles-neo-redux-color', { look: 'neo', theme: 'redux-color' }, { cssStyles: nodeStyles });
add('css-styles-handDrawn-default', { look: 'handDrawn', theme: 'default', ...small }, { cssStyles: nodeStyles });
add('css-styles-box-classic', { look: 'classic', theme: 'default' }, { cssStyles: nodeStyles, attributes: [] });
add('css-styles-box-handDrawn', { look: 'handDrawn', theme: 'default', ...small }, { cssStyles: nodeStyles, attributes: [] });
add('css-styles-svg', { look: 'classic', theme: 'default', htmlLabels: false }, { cssStyles: nodeStyles });
add('css-styles-redux-svg', { look: 'neo', theme: 'redux', htmlLabels: false }, { cssStyles: nodeStyles });
add('css-label-styles', { look: 'classic', theme: 'default' }, { cssStyles: labelStyles });
add('css-label-styles-svg', { look: 'classic', theme: 'default', htmlLabels: false }, { cssStyles: labelStyles });
add('css-label-styles-box', { look: 'neo', theme: 'redux-color' }, { cssStyles: labelStyles, attributes: [] });
add('css-compiled', { look: 'neo', theme: 'redux-color' }, { cssCompiledStyles: compiled, cssClasses: 'default cls' });
add('css-compiled-classic', { look: 'classic', theme: 'default' }, { cssCompiledStyles: compiled, cssClasses: 'default cls' });
add('css-compiled-handdrawn', { look: 'handDrawn', theme: 'default', ...small }, { cssCompiledStyles: [...compiled, 'stroke-dasharray: 5 5'], cssClasses: 'default cls' });
add('css-compiled-box', { look: 'classic', theme: 'default' }, { cssCompiledStyles: compiled, cssClasses: 'default cls', attributes: [] });
add('css-compiled-and-styles', { look: 'classic', theme: 'forest' }, { cssCompiledStyles: compiled, cssStyles: ['fill:yellow', 'stroke-dasharray:3'], cssClasses: 'default a b' });
add('css-stroke-only', { look: 'classic', theme: 'default' }, { cssStyles: ['stroke:blue', 'stroke-width:3px'], attributes: sets.many });
add('css-fill-only', { look: 'classic', theme: 'default' }, { cssStyles: ['fill:#ccf'], attributes: sets.many });
add('css-fill-only-redux', { look: 'neo', theme: 'redux-dark' }, { cssStyles: ['fill:#ccf'], attributes: sets.many });
add('css-no-colon', { look: 'classic', theme: 'default' }, { cssStyles: ['fill', 'stroke:', 'color: blue ; '] });
add('css-classes-undefined', { look: 'classic', theme: 'default' }, { cssClasses: undefined });
add('css-compiled-undefined', { look: 'classic', theme: 'default' }, { cssCompiledStyles: undefined });
// cssStyles is undefined: the table throws, and the plain box does not.
add('css-styles-undefined', { look: 'classic', theme: 'default' }, { cssStyles: undefined });
add('css-styles-undefined-box', { look: 'classic', theme: 'default' }, { cssStyles: undefined, attributes: [] });

// Sizes, the color slot and the config of the diagram.
add('width', { look: 'classic', theme: 'default' }, { width: 400 });
add('height', { look: 'neo', theme: 'redux-color' }, { height: 300 });
add('width-height', { look: 'handDrawn', theme: 'default', ...small }, { width: 90, height: 80 });
add('width-height-classic', { look: 'classic', theme: 'default' }, { width: 650, height: 350 });
add('small-size', { look: 'classic', theme: 'default' }, { width: 10, height: 5 });
add('width-box', { look: 'classic', theme: 'default' }, { width: 400, attributes: [] });
add('width-height-box', { look: 'neo', theme: 'redux-color', htmlLabels: false }, { width: 30, height: 200, attributes: [] });
for (const colorIndex of [3, 25, undefined]) {
  add(`color-index-${colorIndex}`, { look: 'neo', theme: 'redux-color' }, { colorIndex });
}
add('color-index-11-box', { look: 'neo', theme: 'redux-dark-color' }, { colorIndex: 11, attributes: [] });
add('color-index-other-theme', { look: 'neo', theme: 'redux' }, { colorIndex: 5 });
add('er-padding', { look: 'classic', theme: 'default', er: { diagramPadding: 40, entityPadding: 30 } });
add('er-padding-svg', { look: 'classic', theme: 'default', htmlLabels: false, er: { diagramPadding: 40, entityPadding: 30 } });
add('er-padding-box', { look: 'classic', theme: 'default', er: { diagramPadding: 40, entityPadding: 30 } }, { attributes: [] });
add('er-padding-zero', { look: 'neo', theme: 'redux-color', er: { diagramPadding: 0, entityPadding: 0 } });
add('er-min-width', { look: 'classic', theme: 'default', er: { minEntityWidth: 300 } });
add('er-min-width-box', { look: 'classic', theme: 'default', er: { minEntityWidth: 300 } }, { attributes: [] });
add('er-min-width-small-box-svg', { look: 'classic', theme: 'default', htmlLabels: false, er: { minEntityWidth: 10 } }, { attributes: [] });
add('font-courier', { look: 'classic', theme: 'default', fontFamily: 'courier' });
add('font-courier-box', { look: 'classic', theme: 'default', fontFamily: 'courier' }, { attributes: [] });
add('font-size', { look: 'classic', theme: 'default', fontSize: 24, themeVariables: { fontSize: '20px' } });
add('font-size-box-svg', { look: 'classic', theme: 'default', htmlLabels: false, fontSize: 24, themeVariables: { fontSize: '20px' } }, { attributes: [] });
add('theme-variables', { look: 'classic', theme: 'base', themeVariables: { primaryColor: '#ffcc00', lineColor: '#0000ff', background: '#eeeeee' } });
add('theme-variables-handdrawn-box', { look: 'handDrawn', theme: 'base', themeVariables: { primaryColor: '#ffcc00', lineColor: '#0000ff', background: '#eeeeee' }, ...small }, { attributes: [] });

// The other properties of a node.
add('link', { look: 'classic', theme: 'default' }, { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'A tip' });
add('link-box', { look: 'neo', theme: 'redux-color' }, { link: 'https://example.com/', tooltip: 'A tip', attributes: [] });
add('tooltip-callback', { look: 'neo', theme: 'redux-color' }, { tooltip: 'A tip', haveCallback: true });
add('dir-lr', { look: 'classic', theme: 'default' }, {}, { dir: 'LR' });
add('loose', { look: 'classic', theme: 'default', securityLevel: 'loose' }, { attributes: sets.html });
add('no-dom-id', { look: 'classic', theme: 'default' }, { domId: undefined });
add('no-dom-id-handdrawn-box', { look: 'handDrawn', theme: 'default', ...small }, { domId: undefined, attributes: [] });
add('look-undefined', { look: 'classic', theme: 'default' }, { look: undefined });
add('look-default', { theme: 'default' }, { look: 'default' });
add('parent', { look: 'classic', theme: 'default' }, { parentId: 'S' });
// attributes is undefined: the shape throws.
cases.push({
  name: 'erBox/attributes-undefined',
  type,
  directive: { look: 'classic', theme: 'default' },
  node: (() => {
    const n = entity('X', [], 'classic');
    delete n.attributes;
    return n;
  })(),
});

// ---------------------------------------------------------------------------
// The nodes of the corpus: the first entity of the corpus for each kind of
// content, and the first entity for each combination of look, theme and
// label mode.

const db = JSON.parse(zlib.gunzipSync(fs.readFileSync(path.join(outDir, 'db.json.gz'))).toString('utf8'));
const baseFlat = new Map();
const flatten = (v, prefix, out) => {
  if (v !== null && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length > 0) {
    for (const k of Object.keys(v)) {
      flatten(v[k], [...prefix, k], out);
    }
  } else {
    out.set(JSON.stringify(prefix), v);
  }
  return out;
};
flatten(db.baseConfig, [], baseFlat);
// configValue reads a leaf of the config of a case.
const configValue = (c, keys) => {
  const key = JSON.stringify(keys);
  for (const [p, v] of c.configDelta.set) {
    if (JSON.stringify(p) === key) {
      return v;
    }
  }
  if (c.configDelta.del.some((p) => JSON.stringify(p) === key)) {
    return undefined;
  }
  return baseFlat.get(key);
};

const corpusCases = [];
const seen = new Set();
// The markup of the hand-drawn look is large (see tiny): the cases of that
// look are a few small tables and a few plain boxes.
let handDrawnTables = 2;
let handDrawnBoxes = 3;
for (const c of db.cases) {
  if (!c.name.endsWith('.mmd') || !c.nodes) {
    continue;
  }
  // The config of the fixtures of the corpus sets these keys only.
  const directive = { look: configValue(c, ['look']), theme: configValue(c, ['theme']) };
  const htmlLabels = configValue(c, ['htmlLabels']);
  if (htmlLabels !== undefined) {
    directive.htmlLabels = htmlLabels;
  }
  const flowchartHtmlLabels = configValue(c, ['flowchart', 'htmlLabels']);
  if (flowchartHtmlLabels !== undefined) {
    directive.flowchart = { htmlLabels: flowchartHtmlLabels };
  }
  const handDrawn = directive.look === 'handDrawn';
  for (const node of c.nodes) {
    if (node.shape !== 'erBox') {
      continue;
    }
    const a = node.attributes;
    const text = [node.label, node.alias, ...a.flatMap((x) => [x.type, x.name, x.comment])].join(' ');
    const content = JSON.stringify([
      handDrawn,
      htmlLabels,
      a.length === 0 ? 0 : a.length === 1 ? 1 : 2,
      a.some((x) => x.keys.length > 0),
      a.some((x) => x.comment !== ''),
      node.alias !== '',
      node.cssStyles.length > 0,
      node.cssCompiledStyles.length > 0,
      /~/.test(text),
      /[^\x20-\x7e]/.test(text),
      /[*_`<&#]/.test(text),
      node.label.length > 20,
      node.parentId !== undefined,
    ]);
    const config = JSON.stringify([directive, a.length > 0]);
    if (seen.has(content) && seen.has(config)) {
      continue;
    }
    if (handDrawn && a.length > 0) {
      if (a.length > 2 || text.length > 60 || handDrawnTables === 0) {
        continue;
      }
      handDrawnTables--;
    }
    if (handDrawn && a.length === 0) {
      if (handDrawnBoxes === 0) {
        continue;
      }
      handDrawnBoxes--;
    }
    seen.add(content);
    seen.add(config);
    const fixture = c.name.replace(/^.*\/er-diagram\//, '').replace(/\.mmd$/, '');
    corpusCases.push({ name: `erBox/corpus/${fixture}/${node.id}`, type, directive, node });
  }
}

// ---------------------------------------------------------------------------
// Record.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ershape-'));
for (const [file, list] of [
  ['erBox.json.gz', cases],
  ['erBox_corpus.json.gz', corpusCases],
]) {
  const casesFile = path.join(tmp, 'cases.json');
  fs.writeFileSync(casesFile, JSON.stringify({ cases: list }));
  execFileSync(process.execPath, [path.join(here, 'gen-shapes.mjs'), casesFile, path.join(outDir, file)], { cwd: here, stdio: 'inherit' });
}
fs.rmSync(tmp, { recursive: true, force: true });
