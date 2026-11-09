// Builds the cases of the classBox shape and records them with upstream
// mermaid in Chromium (probes/class-shapes.js). The golden file is
// internal/diagrams/class/testdata/classbox.json.gz. It has the cases in
// the format of gen-shapes.mjs, which internal/ru/rutest replays, with one
// difference: the config and the style sheet of a case are indexes into the
// lists `configs` and `styles` of the file, because many cases share them.
//
//   node gen-class-shapes.mjs            write the golden file
//   node gen-class-shapes.mjs --cases    only print the number of cases
//
// The nodes are the nodes that the real class db builds for a diagram text
// (db.getData()), with the config of the case. The cases cover the looks,
// the themes, both label modes, the parts of a class box (annotation,
// title, members, methods), the classifiers, generics, styles, links, the
// config of the class diagram, explicit sizes, and nodes of other diagram
// types that use the shape.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.join(here, '../../internal/diagrams/class/testdata/classbox.json.gz');

const cd = (body) => 'classDiagram\n' + body + '\n';

// Diagram texts. The first class of a text is the node of the case.
const D = {
  empty: 'class Animal',
  generic: 'class Stack~T~',
  label: 'class A["A label with several words"]',
  labelHtml: 'class A["<b>Bold</b> and <i>italic</i>"]',
  labelMarkdown: 'class A["`**Bold** and *italic*`"]',
  members: 'class Animal {\n  +int age\n  -String name\n  #List~int~ ids\n  ~pkg thing\n  plain\n}',
  methods: 'class Animal {\n  +isMammal() bool\n  -mate(Animal other) Animal\n  #run(List~int~ steps, Map~K, V~ m) List~T~\n  ~hide()\n  plain()\n}',
  both: 'class Animal {\n  +int age\n  +String gender\n  +isMammal()\n  +mate()\n}',
  classifiers: 'class Shape {\n  +int count$\n  +String kind*\n  +area() double*\n  +create() Shape$\n  +noReturn()$\n  +abs()*\n}',
  annotation: 'class Shape {\n  <<interface>>\n}',
  annotationMembers: 'class Shape {\n  <<interface>>\n  +int sides\n}',
  annotationMethods: 'class Shape {\n  <<abstract>>\n  +draw()\n}',
  annotationBoth: 'class Shape {\n  <<service>>\n  +int sides\n  +draw() void\n}',
  annotationLong: 'class S {\n  <<a very long annotation text here>>\n  +x\n}',
  twoAnnotations: 'class Shape {\n  <<interface>>\n  <<second>>\n  +draw()\n}',
  longName: 'class AVeryLongClassNameThatIsWiderThanItsMembers {\n  +x\n  +y()\n}',
  longMember:
    'class A {\n  +aVeryLongMemberNameThatMakesTheBoxWide(String firstParameter, int secondParameter) Map~String, List~int~~\n  +s\n}',
  generics: 'class A~T~ {\n  +List~T~ items\n  +get(int i) T\n  +put(T item) void\n}',
  styled: 'class A {\n  +x\n  +y()\n}\nstyle A fill:#f9f,stroke:#333,stroke-width:4px',
  styledColor: 'class A {\n  +x\n  +y()\n}\nstyle A fill:#036,color:#fff,stroke:#f00,stroke-dasharray: 5 5',
  styledFont: 'class A {\n  +x\n}\nstyle A color:red,font-size:20px,font-weight:bold',
  styledEmpty: 'class A\nstyle A fill:#f9f,stroke:#333,color : blue',
  classDef: 'class A:::foo {\n  +x\n  +y()\n}\nclassDef foo fill:#f00,color:#fff,stroke:#00f',
  cssClass: 'class A\ncssClass "A" bar',
  link: 'class A {\n  +x\n}\nlink A "https://example.com" "A tooltip"',
  linkSelf: 'class A\nlink A "https://example.com" _self',
  callback: 'class A\ncallback A "cb" "A tooltip"',
  markdownish:
    'class A {\n  *italic*\n  **bold**\n  _under_\n  - dash item\n  + plus item\n  1. numbered\n  # heading\n  `code`\n  a_b_c\n  a * b * c\n}',
  specials: 'class A {\n  +x < y\n  +a > b\n  +a & b\n  +"quoted"\n  + spaced out\n  -  two spaces\n  +equals = 5\n}',
  html: 'class A {\n  +<b>bold</b>\n  +<i>it</i>()\n  +a<br/>b\n  +c<br>d() e\n}',
  entities: 'class A {\n  +#35; hash\n  +f() #lt;T#gt;\n  +#9829; heart\n}',
  unicode: 'class Ünï {\n  +你好 世界\n  +Привет()\n  +😀 emoji\n}',
  visibilityOnly: 'class A {\n  +\n  -\n  #\n  ~\n}',
  methodsNoVisibility: 'class A {\n  foo()\n  bar(int a) int\n}',
  tilde: 'class A {\n  ~internal int x\n  ~go()\n}',
  namespaced: 'namespace N {\n  class A {\n    +x\n  }\n}',
  many: 'class A {\n' + Array.from({ length: 12 }, (_, i) => `  +int field${i}\n  +method${i}(int a) int`).join('\n') + '\n}',
};

const directives = {
  neo: {},
  'neo-svg': { htmlLabels: false },
  classic: { look: 'classic', theme: 'default' },
  'classic-svg': { look: 'classic', theme: 'default', htmlLabels: false },
  hand: { look: 'handDrawn', theme: 'default' },
  'hand-svg': { look: 'handDrawn', theme: 'default', htmlLabels: false },
};

const cases = [];
const add = (name, text, directive = {}, extra = {}) => cases.push({ name: `classBox/${name}`, text: cd(text), directive, ...extra });

// The parts of a class box, in every look and label mode.
for (const key of ['empty', 'generic', 'label', 'members', 'methods', 'both', 'classifiers', 'annotation', 'annotationMembers', 'annotationMethods', 'annotationBoth', 'styled', 'styledColor']) {
  for (const [dname, directive] of Object.entries(directives)) {
    add(`${key}-${dname}`, D[key], directive);
  }
}
// Label and member variants, in both label modes of the default look.
for (const key of ['labelHtml', 'labelMarkdown', 'annotationLong', 'twoAnnotations', 'longName', 'longMember', 'generics', 'styledFont', 'styledEmpty', 'classDef', 'cssClass', 'link', 'linkSelf', 'callback', 'markdownish', 'specials', 'html', 'entities', 'unicode', 'visibilityOnly', 'methodsNoVisibility', 'tilde', 'namespaced', 'many']) {
  add(`${key}-neo`, D[key], directives.neo);
  add(`${key}-neo-svg`, D[key], directives['neo-svg']);
}
// A few of them in the other looks.
for (const key of ['generics', 'classDef', 'link', 'markdownish', 'html', 'unicode']) {
  add(`${key}-classic`, D[key], directives.classic);
  add(`${key}-hand-svg`, D[key], directives['hand-svg']);
}
// The config of the class diagram.
for (const key of ['empty', 'annotation', 'both', 'members', 'generic']) {
  add(`${key}-hide-neo`, D[key], { class: { hideEmptyMembersBox: true } });
  add(`${key}-hide-classic-svg`, D[key], { ...directives['classic-svg'], class: { hideEmptyMembersBox: true } });
}
for (const key of ['empty', 'annotationBoth', 'members', 'methods']) {
  add(`${key}-padding20-neo`, D[key], { class: { padding: 20 } });
  add(`${key}-padding0-classic-svg`, D[key], { ...directives['classic-svg'], class: { padding: 0 } });
  add(`${key}-padding5-hand`, D[key], { ...directives.hand, class: { padding: 5 } });
}
// Explicit sizes: a node that is wider or higher than its content.
for (const key of ['empty', 'annotation', 'members', 'methods', 'annotationBoth']) {
  add(`${key}-size-neo`, D[key], directives.neo, { patch: { width: 300, height: 400 } });
  add(`${key}-size-classic-svg`, D[key], directives['classic-svg'], { patch: { width: 300, height: 400 } });
  add(`${key}-width-neo-svg`, D[key], directives['neo-svg'], { patch: { width: 300 } });
  add(`${key}-height-hand`, D[key], directives.hand, { patch: { height: 400 } });
  add(`${key}-small-classic`, D[key], directives.classic, { patch: { width: 10, height: 10 } });
}
add('both-size-hide-neo', D.both, { class: { hideEmptyMembersBox: true } }, { patch: { width: 300, height: 400 } });
add('empty-size-hide-neo-svg', D.empty, { htmlLabels: false, class: { hideEmptyMembersBox: true } }, { patch: { width: 300, height: 400 } });
// The label mode of the node wins over the config.
add('both-node-svg-in-html', D.both, directives.neo, { patch: { useHtmlLabels: false } });
add('both-node-html-in-svg', D.both, directives['neo-svg'], { patch: { useHtmlLabels: true } });
add('annotationBoth-node-svg-in-html-classic', D.annotationBoth, directives.classic, { patch: { useHtmlLabels: false } });
add('empty-node-html-in-svg-hand', D.empty, directives['hand-svg'], { patch: { useHtmlLabels: true } });
// Other themes. The neo theme has a gradient, so the dividers have no
// neo-line class.
for (const theme of ['neo', 'dark', 'forest', 'neutral', 'base', 'redux', 'redux-dark-color', 'neo-dark']) {
  add(`both-theme-${theme}`, D.annotationBoth, { look: 'neo', theme });
  add(`both-theme-${theme}-classic-svg`, D.both, { look: 'classic', theme, htmlLabels: false });
}
add('styled-theme-neo', D.styled, { look: 'neo', theme: 'neo' });
add('empty-theme-neo-svg', D.empty, { look: 'neo', theme: 'neo', htmlLabels: false });
// Fonts.
add('both-fontsize-neo', D.both, { fontSize: 24 });
add('both-fontsize-classic-svg', D.both, { ...directives['classic-svg'], fontSize: 10 });
add('both-fontfamily-neo', D.annotationBoth, { fontFamily: 'courier, monospace' });
add('both-fontfamily-neo-svg', D.annotationBoth, { fontFamily: 'courier, monospace', htmlLabels: false });
add('both-themefont-neo-svg', D.both, { htmlLabels: false, themeVariables: { fontFamily: 'arial', fontSize: '20px' } });
// Text that is anchored in the middle moves the title to x = 0.
add('both-anchor-middle-svg', D.annotationBoth, { htmlLabels: false, themeCSS: '.label text { text-anchor: middle; }' });
add('empty-anchor-middle-classic-svg', D.empty, { ...directives['classic-svg'], themeCSS: 'text { text-anchor: middle; }' });
// The hand-drawn look with a fixed seed: roughjs then uses its own random
// numbers.
add('both-hand-seed', D.both, { ...directives.hand, handDrawnSeed: 42 });
add('empty-hand-seed-svg', D.empty, { ...directives['hand-svg'], handDrawnSeed: 42 });
add('styled-hand-seed', D.styledColor, { ...directives.hand, handDrawnSeed: 7 });
// The color slots of the redux-color theme.
add('both-color-13', D.both, directives.neo, { patch: { colorIndex: 13 } });
add('both-color-none', D.both, directives.neo, { patch: { colorIndex: null } });
// Styles of the node on top of the styles of the class.
add('both-cssstyles-neo', D.both, directives.neo, { patch: { cssStyles: ['fill:#0f0', 'color:#00f'] } });
add('both-compiled-neo-svg', D.both, directives['neo-svg'], { patch: { cssCompiledStyles: ['fill:#0f0', 'color:#00f', 'stroke:#f00'] } });
add('both-labelstyle-classic', D.both, directives.classic, { patch: { labelStyle: 'color:#f00;font-weight:bold' } });
add('styled-compiled-classic-svg', D.styled, directives['classic-svg'], { patch: { cssCompiledStyles: ['color:#00f', 'font-size:18px'] } });
// Nodes of other diagram types that use the shape: no class properties.
const plain = (look, extra = {}) => ({
  id: 'A',
  label: 'A plain node',
  labelType: 'text',
  padding: 15,
  cssStyles: [],
  cssCompiledStyles: [],
  cssClasses: 'default',
  domId: 'my-svg-flowchart-A-0',
  look,
  isGroup: false,
  shape: 'classBox',
  ...extra,
});
const addNode = (name, node, directive = {}) => cases.push({ name: `classBox/${name}`, node, directive });
addNode('plain-neo', plain('neo'), directives.neo);
addNode('plain-neo-svg', plain('neo'), directives['neo-svg']);
addNode('plain-classic', plain('classic'), directives.classic);
addNode('plain-hand-svg', plain('handDrawn'), directives['hand-svg']);
addNode('plain-styles-neo', plain('neo', { cssStyles: ['fill:#f9f', 'stroke:#333', 'color:#00f'] }), directives.neo);
addNode('plain-styles-classic-svg', plain('classic', { cssStyles: ['fill:#f9f', 'stroke:#333', 'color:#00f'] }), directives['classic-svg']);
addNode('plain-compiled-neo-svg', plain('neo', { cssCompiledStyles: ['fill:#f00', 'color:#fff', 'stroke:#00f'], cssClasses: 'default foo' }), directives['neo-svg']);
addNode('plain-long-neo', plain('neo', { label: 'This is a long label that must wrap over several lines to fit' }), directives.neo);
addNode('plain-bare-neo', { id: 'A', label: 'Bare', shape: 'classBox' }, directives.neo);
addNode('plain-bare-svg', { id: 'A', label: 'Bare', shape: 'classBox' }, directives['neo-svg']);
addNode('plain-empty-neo', plain('neo', { label: '' }), directives.neo);
addNode('plain-empty-svg', plain('neo', { label: '' }), directives['neo-svg']);
addNode('plain-nolabel-neo', { id: 'A', shape: 'classBox', look: 'neo', cssClasses: 'default' }, directives.neo);
addNode('plain-nolabel-svg', { id: 'A', shape: 'classBox', look: 'neo', cssClasses: 'default' }, directives['neo-svg']);
addNode('plain-link-neo', plain('neo', { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'tip', haveCallback: true }), directives.neo);
addNode('plain-size-classic', plain('classic', { width: 200, height: 150 }), directives.classic);
// A class node whose text is empty.
addNode(
  'emptytext-neo-svg',
  { id: 'A', label: '', text: '', shape: 'classBox', cssClasses: 'default', methods: [], members: [], annotations: [], styles: [], domId: 'my-svg-classId-A-0', isGroup: false, look: 'neo', colorIndex: 0 },
  directives['neo-svg']
);
addNode(
  'emptytext-neo',
  { id: 'A', label: '', text: '', shape: 'classBox', cssClasses: 'default', methods: [], members: [], annotations: [], styles: [], domId: 'my-svg-classId-A-0', isGroup: false, look: 'neo', colorIndex: 0 },
  directives.neo
);

if (process.argv.includes('--cases')) {
  console.log(`${cases.length} cases`);
  process.exit(0);
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'classshapes-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/class-shapes.js'), '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
// Share the configs and the style sheets.
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
const styles = table([]);
const golden = results.map((r) => {
  const rec = { ...r };
  if (r.config !== undefined) {
    rec.config = configs.add(r.config);
  }
  if (r.css !== undefined) {
    rec.css = styles.add(r.css);
  }
  return rec;
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ configs: configs.list, styles: styles.list, cases: golden }), { level: 9 }));
const errors = results.filter((r) => r.error);
console.error(`${results.length} cases, ${errors.length} errors -> ${outFile}`);
for (const r of errors) {
  console.error(`  ${r.name}: ${r.error}`);
}
