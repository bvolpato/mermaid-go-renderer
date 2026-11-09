// Builds the cases of the requirementBox shape and records them with
// upstream mermaid in Chromium (probes/requirement-shapes.js). The golden
// file has the cases of gen-shapes.mjs, with the configs and the style
// sheets in shared lists:
// internal/diagrams/requirement/testdata/requirementbox.json.gz.
//
//   node gen-requirement-shapes.mjs            write the golden file
//   node gen-requirement-shapes.mjs --cases    only print the number of cases
//
// The nodes are the nodes that the real requirement db builds for a diagram
// text (db.getData()), with the config of the case. The cases cover the
// looks, the themes, both label modes, both alignments of the body text,
// the lines of a box (type, name, id, text, risk, verification method, or
// type and doc ref), markdown and HTML in the texts, styles, the color
// slots, fonts, and nodes with missing properties. A second group has the
// nodes of corpus fixtures, each with the config of its fixture: the
// fixtures of the pr-check directory, and of the nightly-check directory
// the variants that differ in look, label mode or theme family. Of the
// hand-drawn variants it has the fixtures with small boxes. A third group
// draws all the nodes of a diagram together, as a layout does.
//
// Run gen-requirement-db.mjs first: the corpus cases take the node ids of
// each fixture from its golden file.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/diagrams/requirement/testdata/requirementbox.json.gz');
const dbGolden = path.join(repoRoot, 'internal/diagrams/requirement/testdata/db.json.gz');

const rd = (body) => 'requirementDiagram\n' + body + '\n';

const full = 'requirement test_req {\nid: 1\ntext: the test text.\nrisk: high\nverifymethod: test\n}';
const element = 'element test_entity {\ntype: simulation\n}';

// Diagram texts. The first node of a text is the node of the case.
const D = {
  full,
  functional: 'functionalRequirement test_req2 {\nid: 1.1\ntext: the second test text.\nrisk: low\nverifymethod: inspection\n}',
  performance: 'performanceRequirement test_req3 {\nid: 1.2\ntext: the third test text.\nrisk: medium\nverifymethod: demonstration\n}',
  interface: 'interfaceRequirement test_req4 {\nid: 1.2.1\ntext: the fourth test text.\nrisk: medium\nverifymethod: analysis\n}',
  physical: 'physicalRequirement test_req5 {\nid: 1.2.2\ntext: the fifth test text.\nrisk: medium\nverifymethod: analysis\n}',
  constraint: 'designConstraint test_req6 {\nid: 1.2.3\ntext: the sixth test text.\nrisk: medium\nverifymethod: analysis\n}',
  nameOnly: 'requirement only_name {\n}',
  idOnly: 'requirement r {\nid: 42\n}',
  textOnly: 'requirement r {\ntext: only the text\n}',
  riskOnly: 'requirement r {\nrisk: medium\n}',
  verifyOnly: 'requirement r {\nverifymethod: analysis\n}',
  idVerify: 'requirement r {\nid: 7\nverifymethod: demonstration\n}',
  textRisk: 'requirement r {\ntext: a text and a risk\nrisk: low\n}',
  long: 'requirement test_req {\nid: 1\ntext: the test text that is long and takes up a lot of space and then goes on for some more words.\nrisk: high\nverifymethod: test\n}',
  longName: 'requirement test_requirement_name_that_is_extra_long_and_wider_than_the_body {\nid: 1\n}',
  quoted: 'requirement "a name with spaces" {\nid: "an id with spaces"\ntext: "text: with, punctuation - and = signs {x} <y>"\n}',
  markdown: 'requirement "__my bolded name__" {\nid: 1\ntext: "**Bolded text** _italicized text_"\nrisk: high\nverifymethod: test\n}',
  markdownLong:
    'requirement "*italic* and **bold** name" {\ntext: "A **long bolded text that is wider when it is bold than when it is plain so that the label may wrap at its end** x"\n}',
  markdownBlock: 'requirement "# heading" {\nid: "- item"\ntext: "1. numbered `code` [link](https://example.com)"\n}',
  html: 'requirement "<b>bold</b> name" {\nid: "<u>u</u>"\ntext: "a <i>tag</i> and a <br/> break"\n}',
  script: 'requirement "<script>alert(1)</script>name" {\ntext: "<img src=x onerror=alert(1)> text"\n}',
  entities: 'requirement "a #35; b" {\ntext: "#lt;tag#gt; #9829; &amp; &lt; #quot;q#quot;"\n}',
  unicode: 'requirement "Ünïcödé name" {\nid: "ñ"\ntext: "емкость ελληνικά"\n}',
  emoji: 'requirement "name 😀" {\ntext: "text 👍 and ❤"\n}',
  cjk: 'requirement "日本語" {\ntext: "한국어 中文"\n}',
  icon: 'requirement icon_req {\ntext: "fa:fa-user an icon"\n}',
  numbers: 'requirement 123 {\nid: 4.5\ntext: 678\n}',
  spaces: 'requirement "  padded name  " {\ntext: "  padded   text  "\n}',
  element,
  elementBoth: 'element test_entity2 {\ntype: word doc\ndocRef: reqs/test_entity\n}',
  elementDocOnly: 'element e {\ndocref: only/the/ref\n}',
  elementEmpty: 'element empty_element {\n}',
  elementLong: 'element test_entity_name_that_is_extra_long {\n}',
  elementMarkdown: 'element "*my italicized name*" {\ntype: "**Bolded type** _italicized type_"\ndocref: "*Italicized* __Bolded__"\n}',
  elementQuoted: 'element "an element name" {\ntype: "a type: with, punctuation"\ndocref: "https://example.com/a?b=c"\n}',
  styled: full + '\nstyle test_req fill:#ffa,stroke:#000,color:green',
  styledStroke: full + '\nstyle test_req stroke:#f00,stroke-width:4px',
  styledFont: full + '\nstyle test_req font-size:20px,font-weight:bold,color:red',
  styledFill: full + '\nstyle test_req fill:#036',
  styledColor: full + '\nstyle test_req color:#f0f',
  classDef: 'requirement test_req:::important {\nid: 1\ntext: the test text.\n}\nclassDef important fill:#f96,stroke:#333,stroke-width:4px,color:#00f',
  classDefault: full + '\nclassDef default fill:pink,color:#333',
  classStatement: full + '\nclassDef a fill:#eee\nclassDef b stroke:blue,color:white\nclass test_req a,b',
  styledElement: element + '\nstyle test_entity fill:#bbf,stroke:#f66,color:#fff',
  classElement: 'classDef c fill:#036,color:#fff\nelement e:::c {\ntype: x\n}',
  styledNameOnly: 'requirement only_name {\n}\nstyle only_name fill:#0f0,color:#00f,stroke:#f00',
};

const directives = {
  neo: {},
  'neo-svg': { htmlLabels: false },
  'neo-dagre': { layout: 'dagre' },
  'neo-dagre-svg': { layout: 'dagre', htmlLabels: false },
  classic: { look: 'classic', theme: 'default' },
  'classic-svg': { look: 'classic', theme: 'default', htmlLabels: false },
  hand: { look: 'handDrawn' },
  'hand-svg': { look: 'handDrawn', htmlLabels: false },
};

const cases = [];
const add = (name, text, directive = {}, extra = {}) => cases.push({ name: `requirementBox/${name}`, text: rd(text), directive, ...extra });

// The markup of a hand-drawn box is large: its fill is made of many lines
// with random numbers. So only a part of the cases has the hand-drawn look.
const handKeys = {
  hand: ['full', 'nameOnly', 'verifyOnly', 'element', 'elementBoth', 'elementEmpty', 'styled', 'classDef'],
  'hand-svg': ['full', 'idOnly', 'element', 'styledStroke'],
};

// The lines of a box, in every look, label mode and alignment.
for (const key of ['full', 'nameOnly', 'idOnly', 'textOnly', 'riskOnly', 'verifyOnly', 'idVerify', 'textRisk', 'element', 'elementBoth', 'elementDocOnly', 'elementEmpty', 'styled', 'styledStroke', 'classDef', 'styledElement']) {
  for (const [dname, directive] of Object.entries(directives)) {
    if (handKeys[dname] && !handKeys[dname].includes(key)) {
      continue;
    }
    add(`${key}-${dname}`, D[key], directive);
  }
}
// The other texts, in both label modes of the default look.
for (const key of Object.keys(D)) {
  if (cases.some((c) => c.name === `requirementBox/${key}-neo`)) {
    continue;
  }
  add(`${key}-neo`, D[key], directives.neo);
  add(`${key}-neo-svg`, D[key], directives['neo-svg']);
}
// A few of them in the other looks.
for (const key of ['long', 'markdown', 'markdownLong', 'html', 'entities', 'unicode', 'elementMarkdown', 'styledFont', 'classDefault', 'classElement', 'functional']) {
  add(`${key}-classic`, D[key], directives.classic);
  add(`${key}-classic-svg`, D[key], directives['classic-svg']);
  add(`${key}-neo-dagre`, D[key], directives['neo-dagre']);
}
for (const key of ['entities', 'elementMarkdown', 'classElement']) {
  add(`${key}-hand-svg`, D[key], directives['hand-svg']);
}
// Themes. A theme with border colors gives each box a color slot.
for (const theme of ['default', 'neo', 'neo-dark', 'dark', 'forest', 'neutral', 'base', 'redux', 'redux-dark', 'redux-color', 'redux-dark-color', 'null']) {
  add(`full-theme-${theme}`, D.full, { look: 'neo', theme });
  add(`styled-theme-${theme}-classic-svg`, D.styled, { look: 'classic', theme, htmlLabels: false });
  add(`element-theme-${theme}-classic`, D.elementBoth, { look: 'classic', theme });
}
for (const theme of ['default', 'neo', 'redux-dark-color']) {
  add(`element-theme-${theme}-hand`, D.element, { look: 'handDrawn', theme });
}
// Theme variables that the shape reads.
add('styled-noborders', D.styled, { look: 'classic', theme: 'redux-color', themeVariables: { borderColorArray: [] } });
add('styled-edgebkg', D.styled, { look: 'classic', theme: 'default', themeVariables: { requirementEdgeLabelBackground: '#eee' } });
add('styled-borders-default', D.styled, { look: 'classic', theme: 'default', themeVariables: { borderColorArray: ['#f00', '#0f0', '#00f'] } });
add('full-borders-string', D.full, { look: 'neo', theme: 'default', themeVariables: { borderColorArray: 'abcde' } }, { patch: { colorIndex: 7 } });
add('full-mainbkg', D.full, { look: 'handDrawn', theme: 'default', themeVariables: { mainBkg: '#fed', nodeBorder: '#123' } });
// The layout decides the alignment of the body text.
for (const layout of ['elk', 'dagre', 'elk.stress', 'nosuchlayout']) {
  add(`full-layout-${layout}`, D.full, { layout });
  add(`element-layout-${layout}-svg`, D.elementBoth, { layout, htmlLabels: false });
}
// Label modes.
add('full-htmllabels-true', D.full, { htmlLabels: true });
add('full-flowchart-htmllabels-false', D.full, { flowchart: { htmlLabels: false } });
add('full-htmllabels-string', D.full, { htmlLabels: 'false' });
// Fonts.
add('full-fontsize-neo', D.full, { fontSize: 24 });
add('full-fontsize-classic-svg', D.full, { ...directives['classic-svg'], fontSize: 10 });
add('full-fontfamily-neo', D.full, { fontFamily: 'courier, monospace' });
add('full-fontfamily-neo-svg', D.long, { fontFamily: 'courier, monospace', htmlLabels: false });
add('full-themefont-neo-svg', D.full, { htmlLabels: false, themeVariables: { fontFamily: 'arial', fontSize: '20px' } });
add('long-fontfamily-arial', D.long, { fontFamily: 'arial' });
add('markdownLong-fontfamily-arial', D.markdownLong, { fontFamily: 'arial' });
add('markdownLong-fontfamily-arial-svg', D.markdownLong, { fontFamily: 'arial', htmlLabels: false });
add('full-themecss', D.full, { themeCSS: '.label { font-size: 20px; }' });
// The hand-drawn look with a fixed seed: roughjs then uses its own random
// numbers.
add('full-hand-seed', D.full, { look: 'handDrawn', handDrawnSeed: 42 });
add('element-hand-seed-svg', D.element, { look: 'handDrawn', htmlLabels: false, handDrawnSeed: 42 });
add('styled-hand-seed', D.styled, { look: 'handDrawn', handDrawnSeed: 7 });
add('full-neo-seed', D.full, { handDrawnSeed: 42 });
// The color slots.
add('full-color-13', D.full, directives.neo, { patch: { colorIndex: 13 } });
add('full-color-none', D.full, directives.neo, { patch: { colorIndex: null } });
add('element-color-5-classic', D.element, { look: 'classic' }, { patch: { colorIndex: 5 } });
// Styles of the node on top of the styles of the diagram.
add('full-compiled-neo', D.full, directives.neo, { patch: { cssCompiledStyles: ['fill:#0f0', 'color:#00f', 'stroke:#f00'] } });
add('styled-compiled-neo-svg', D.styled, directives['neo-svg'], { patch: { cssCompiledStyles: ['color:#00f', 'font-size:18px'] } });
add('full-labelstyle-classic', D.full, directives.classic, { patch: { labelStyle: 'color:#f00;font-weight:bold' } });
add('full-nostyles', D.full, directives.neo, { patch: { cssStyles: null } });
add('full-noclasses', D.full, directives.neo, { patch: { cssClasses: null } });
add('full-styles-nocolon', D.full, directives.neo, { patch: { cssStyles: ['fill', 'stroke:#f00', 'color'] } });
// Nodes with other properties than the db gives.
add('full-nodomid', D.full, directives.neo, { patch: { domId: null } });
add('full-emptyname', D.full, directives.neo, { patch: { name: '' } });
add('full-emptyname-svg', D.full, directives['neo-svg'], { patch: { name: '' } });
add('full-noname', D.full, directives.neo, { patch: { name: null } });
add('full-blankname', D.full, directives.neo, { patch: { name: ' ' } });
add('full-notype', D.full, directives.neo, { patch: { type: null } });
add('full-numbers', D.full, directives.neo, { patch: { requirementId: 5, text: 1.5, risk: 0, verifyMethod: true } });
add('full-numbertype', D.full, directives['neo-svg'], { patch: { type: 3 } });
add('full-link', D.full, directives.neo, { patch: { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'tip', haveCallback: true } });
add('full-size', D.full, directives.classic, { patch: { width: 400, height: 300 } });
add('full-look-other', D.full, directives.neo, { patch: { look: 'classic' } });
add('full-look-none', D.full, directives.neo, { patch: { look: null } });
add('element-verifymethod', D.element, directives.neo, { patch: { verifyMethod: '' } });
add('element-notype', D.elementBoth, directives.neo, { patch: { type: null } });
add('element-numbertype', D.elementBoth, directives.neo, { patch: { type: 9 } });
add('full-labelclass', D.full, directives.neo, { patch: { name: '<span class="label">inner label</span> name' } });
// A node of another diagram type that uses the shape.
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
  shape: 'requirementBox',
  ...extra,
});
const addNode = (name, node, directive = {}) => cases.push({ name: `requirementBox/${name}`, node, directive });
addNode('plain-neo', plain('neo'), directives.neo);
addNode('plain-name-neo', plain('neo', { name: 'A plain name' }), directives.neo);
addNode('plain-name-svg', plain('neo', { name: 'A plain name' }), directives['neo-svg']);
addNode('plain-name-hand', plain('handDrawn', { name: 'A plain name', type: 'a type', docRef: 'a ref' }), directives.hand);
addNode('bare', { id: 'A', name: 'Bare', shape: 'requirementBox' }, directives.neo);
addNode('bare-svg', { id: 'A', name: 'Bare', shape: 'requirementBox' }, directives['neo-svg']);

// ---------------------------------------------------------------------------
// The nodes of the corpus fixtures, each with the config of its fixture.

const corpusDir = 'testdata/corpus/mermaid-e2e';
const corpusVariants =
  /\/pr-check\/|-(default|without-htmllabels-default|neo-look-neo-dark-theme)\.mmd$|\/(a-simple-requirement-diagram|requirements-and-elements-with-and-without-information|requirements-and-elements-with-styles-applied-from-style-statement)-(without-htmllabels-)?handdrawn-look\.mmd$/;
const db = JSON.parse(zlib.gunzipSync(fs.readFileSync(dbGolden)));
const seen = new Set();
let corpusCases = 0;
for (const c of db.cases) {
  if (!c.name.startsWith(corpusDir) || c.name.includes('#') || !c.nodes || !corpusVariants.test(c.name)) {
    continue;
  }
  const source = fs.readFileSync(path.join(repoRoot, c.name), 'utf8');
  // The front matter without the description decides the config.
  const frontMatter = (source.match(/^---\n([\s\S]*?)\n---\n/) ?? ['', ''])[1].replace(/^description:.*\n?(?:[ \t]+.*\n?)*/m, '');
  for (const node of c.nodes) {
    const { domId, ...rest } = node;
    const key = frontMatter + '\n' + JSON.stringify(rest);
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    const short = c.name.slice(corpusDir.length + 1).replace(/\.mmd$/, '');
    cases.push({ name: `corpus/${short}#${node.id}`, source, pick: node.id });
    corpusCases++;
  }
}

// ---------------------------------------------------------------------------
// Diagrams whose nodes are drawn together, as a layout draws them. The
// boxes have different numbers of lines, so they do not finish in the order
// of the nodes.

const mixedList = [
  'requirement r_full {\nid: 1\ntext: the test text.\nrisk: high\nverifymethod: test\n}',
  'requirement r_id {\nid: 1\n}',
  'requirement r_none {\n}',
  'requirement r_verify {\nverifymethod: analysis\n}',
  'element e_type {\ntype: sim\n}',
  'element e_both {\ntype: word doc\ndocref: reqs/x\n}',
  'element e_none {\n}',
  'requirement r_text {\ntext: only a text here\n}',
];
const mixed = mixedList.join('\n');
const together = (name, text, directive = {}) => cases.push({ name: `together/${name}`, text: rd(text), directive, all: true });
together('mixed-neo', mixed, directives.neo);
together('mixed-neo-svg', mixed, directives['neo-svg']);
together('mixed-neo-dagre', mixed, directives['neo-dagre']);
together('mixed-classic', mixed, directives.classic);
together('mixed-classic-svg', mixed, directives['classic-svg']);
together('mixed-hand', 'requirement r_id {\nid: 1\n}\nelement e_none {\n}\nrequirement r_none {\n}\nelement e_type {\ntype: sim\n}', directives.hand);
together('mixed-hand-svg', 'element e_both {\ntype: word doc\ndocref: reqs/x\n}\nrequirement r_verify {\nverifymethod: analysis\n}\nelement e_none {\n}', directives['hand-svg']);
together('reversed-neo', [...mixedList].reverse().join('\n'), directives.neo);
together('icons-neo', 'requirement r_icon {\ntext: "fa:fa-user an icon"\n}\nrequirement r_plain {\ntext: no icon\n}\nelement e_none {\n}\nrequirement r_two {\nid: "fa:fa-a"\ntext: "fa:fa-b fa:fa-c"\n}', directives.neo);
together('empty-neo', '', directives.neo);
together('one-neo', D.full, directives.neo);
for (const fixture of ['pr-check/sample', 'nightly-check/a-not-so-simple-requirement-diagram-default', 'nightly-check/a-not-so-simple-requirement-diagram-without-htmllabels-default', 'nightly-check/requirements-and-elements-with-and-without-information-default', 'nightly-check/a-complex-diagram-with-multiple-requirements-and-elements-neo-look-neo-dark-theme']) {
  const source = fs.readFileSync(path.join(repoRoot, corpusDir, 'requirement', fixture + '.mmd'), 'utf8');
  cases.push({ name: `together/corpus/${fixture}`, source, all: true });
}

if (process.argv.includes('--cases')) {
  console.log(`${cases.length} cases (${corpusCases} of the corpus)`);
  process.exit(0);
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'reqshapes-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/requirement-shapes.js'), '--args', argsFile, '--out', resultFile], {
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
console.error(`${results.length} cases (${corpusCases} of the corpus), ${errors.length} errors -> ${outFile}`);
for (const r of errors) {
  console.error(`  ${r.name}: ${r.error}`);
}
