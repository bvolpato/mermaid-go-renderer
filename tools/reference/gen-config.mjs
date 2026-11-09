// Emits the data and the golden cases of internal/config from the real
// mermaid 12.2.0.
//
// Usage (from tools/reference):
//   node gen-config.mjs defaults   writes internal/config/default_config.json and testdata/defaults.json.gz
//   node gen-config.mjs corpus     writes testdata/corpus.json.gz   (every corpus fixture)
//   node gen-config.mjs site       writes testdata/site.json.gz     (site configs crossed with fixtures)
//   node gen-config.mjs text       writes testdata/text.json.gz     (preprocessing and detection cases)
//   node gen-config.mjs yaml       writes testdata/yaml.json.gz     (front matter parsing cases)
//   node gen-config.mjs all
//
// Source of the libraries: lib-mermaid-internals.mjs loads mermaid.core.mjs,
// which imports es-toolkit from node_modules. node_modules has es-toolkit
// 1.53.1, and the mermaid bundle that the reference renders with has 1.45.1.
// use-bundled.mjs sends the import to the source of 1.45.1, which comes from
// the source map of the bundle. The two versions differ in cleanAndMerge
// (front matter config merged with the directives): 1.45.1 reads a null
// element of a source array as undefined, and 1.53.1 keeps the null. The text
// cases with a null in an array are those where the two versions differ.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { bundledVersion } from './use-bundled.mjs';
import { encode, internals, mulberry32 } from './lib-mermaid-internals.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const OUT = path.join(ROOT, 'internal/config');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e');

const { mermaid, processAndSetConfigs, preprocessDiagram, config: api, cleanAndMerge } = await internals();

// es-toolkit 1.45.1 gives {"a":[1,3]} here, and 1.53.1 gives {"a":[null,3]}.
// This check stops the script when mermaid did not load the bundled version.
if (bundledVersion('es-toolkit') !== '1.45.1' || JSON.stringify(cleanAndMerge({ a: [1, 2] }, { a: [null, 3] })) !== '{"a":[1,3]}') {
  throw new Error('gen-config.mjs: es-toolkit is not the version of the mermaid bundle (1.45.1)');
}

// Each golden file records the version of es-toolkit that made it.
const writeGz = (name, value) => {
  const file = path.join(OUT, 'testdata', name);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const data = zlib.gzipSync(JSON.stringify({ esToolkit: bundledVersion('es-toolkit'), ...value }), { level: 9 });
  fs.writeFileSync(file, data);
  process.stderr.write(`${name}: ${data.length} bytes\n`);
};

const walk = (dir) =>
  fs.readdirSync(dir).flatMap((name) => {
    const p = path.join(dir, name);
    return fs.statSync(p).isDirectory() ? walk(p) : p.endsWith('.mmd') ? [p] : [];
  });
const corpusFiles = () => walk(CORPUS).sort();

// A pool stores each distinct JSON value once. A config object is then a list
// of [key, pool index] pairs, which keeps the golden files small: most
// fixtures share most top-level sections.
class Pool {
  constructor() {
    this.index = new Map();
    this.values = [];
  }
  add(value) {
    const text = JSON.stringify(value);
    let i = this.index.get(text);
    if (i === undefined) {
      i = this.values.length;
      this.index.set(text, i);
      this.values.push(value);
    }
    return i;
  }
  // object stores an object as pairs of key and pool index.
  object(obj) {
    return this.add(Object.keys(obj).map((k) => [k, this.add(obj[k])]));
  }
}

// errorOf describes a thrown value.
const errorOf = (e) => ({ name: e?.name ?? typeof e, message: String(e?.message ?? e) });

// frontmatterMetadata is diagram-api/frontmatter.ts extractFrontMatter. That
// function is not reachable from outside, and preprocessDiagram does not
// return displayMode on its own, so this is a copy of its few lines built
// from the real matchFrontMatter and the real js-yaml. run() checks its title
// against the title that the real preprocessDiagram returns.
let yamlModule;
const loadYaml = async () => {
  if (yamlModule) return yamlModule;
  const dir = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.core');
  for (const file of fs.readdirSync(dir)) {
    const source = fs.readFileSync(path.join(dir, file), 'utf8');
    const exports = source.slice(source.lastIndexOf('\nexport {'));
    if (/\bJSON_SCHEMA\b/.test(exports) && /\bload\b/.test(exports)) {
      yamlModule = await import(pathToFileURL(path.join(dir, file)).href);
      return yamlModule;
    }
  }
  throw new Error('js-yaml chunk not found');
};
// cleanupText is a copy of the function of that name in preprocess.ts, which
// runs before the front matter is read.
const cleanupText = (code) =>
  code
    .replace(/\r\n?/g, '\n')
    .replace(
      /<(\w+)([^>]*)>/g,
      (match, tag, attributes) => '<' + tag + attributes.replace(/="([^"]*)"/g, "='$1'") + '>'
    );
const frontmatterMetadata = (yaml, code) => {
  const normalized = cleanupText(code);
  const match = api.matchFrontMatter(normalized);
  if (!match) return {};
  const indent = match.indent;
  const body = indent
    ? match.body
        .split('\n')
        .map((line) => (line.startsWith(indent) ? line.slice(indent.length) : line))
        .join('\n')
    : match.body;
  let parsed = yaml.load(body, { schema: yaml.JSON_SCHEMA }) ?? {};
  parsed = typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
  const metadata = {};
  if (parsed.displayMode) metadata.displayMode = parsed.displayMode.toString();
  if (parsed.title) metadata.title = parsed.title.toString();
  return metadata;
};

// run is mermaid.initialize(site) followed by processAndSetConfigs(text), as
// mermaidAPI.render does it. It records everything that the Go port computes.
const run = (yaml, pool, text, site) => {
  const rec = {};
  // The reference renderer loads a new page for every diagram. reset() drops
  // the directives and the diagram scope that an earlier case can leave behind
  // when it throws, and initialize() replaces the rest of the module state.
  api.reset();
  try {
    mermaid.initialize(site === undefined ? undefined : structuredClone(site));
  } catch (e) {
    rec.initError = errorOf(e);
    return rec;
  }
  try {
    const processed = processAndSetConfigs(text);
    const cfg = api.getConfig();
    let diagramType;
    try {
      diagramType = api.detectType(processed.code.cleaned, api.getConfig());
    } catch {
      diagramType = undefined;
    }
    let meta;
    try {
      meta = frontmatterMetadata(yaml, text);
    } catch (e) {
      throw new Error(`frontmatterMetadata threw where preprocessDiagram did not: ${e}`);
    }
    if (meta.title !== processed.title) {
      throw new Error(`frontmatterMetadata title ${meta.title} differs from ${processed.title}`);
    }
    rec.cleaned = processed.code.cleaned;
    rec.withComments = processed.code.withComments;
    rec.frontmatterLineOffset = processed.code.frontmatterLineOffset;
    if (processed.title !== undefined) rec.title = processed.title;
    if (meta.displayMode !== undefined) rec.displayMode = meta.displayMode;
    if (diagramType !== undefined) rec.diagramType = diagramType;
    rec.directive = pool.add(encode(processed.config));
    rec.config = pool.object(encode(cfg));
  } catch (e) {
    rec.error = errorOf(e);
  } finally {
    // render() and parse() leave the diagram scope when they finish. That
    // call throws the same error again when a directive breaks the theme.
    try {
      api.setDiagramConfigScope(undefined);
    } catch {
      // The error is already recorded.
    }
  }
  return rec;
};

const commands = {
  // The default config is data: the defaults of the JSON schema plus the
  // values that defaultConfig.ts adds. Function-valued properties are marked
  // {"$f":1}, undefined ones {"$u":1}. themeVariables is left to the Go theme
  // port and marked {"$theme":"default"}.
  defaults() {
    const mark = (v) => {
      if (typeof v === 'function') return { $f: 1 };
      if (v === undefined) return { $u: 1 };
      if (v === null || typeof v !== 'object') return v;
      if (Array.isArray(v)) return v.map(mark);
      return Object.fromEntries(Object.keys(v).map((k) => [k, mark(v[k])]));
    };
    const marked = mark(api.defaultConfig);
    const full = encode(api.defaultConfig);
    marked.themeVariables = { $theme: 'default' };
    fs.writeFileSync(path.join(OUT, 'default_config.json'), JSON.stringify(marked, null, 1) + '\n');

    // configKeys is not exported. sanitizeDirective deletes exactly the keys
    // that are not in it, so probe it with every key of the default config.
    const candidates = new Set();
    const collect = (v) => {
      if (v === null || typeof v !== 'object') return;
      for (const k of Object.keys(v)) {
        candidates.add(k);
        collect(v[k]);
      }
    };
    collect(api.defaultConfig);
    const configKeys = [...candidates].filter((k) => {
      const probe = { [k]: 'x' };
      api.sanitizeDirective(probe);
      return Object.hasOwn(probe, k);
    });
    writeGz('defaults.json.gz', {
      defaultConfig: full,
      configKeys,
      notConfigKeys: [...candidates].filter((k) => !configKeys.includes(k)),
      detectors: (mermaid.initialize({ startOnLoad: false }), Object.keys(api.detectors)),
    });
  },

  // Golden set 1: every corpus fixture after mermaid.initialize({startOnLoad:false}).
  async corpus() {
    const yaml = await loadYaml();
    const pool = new Pool();
    const site = { startOnLoad: false };
    const cases = corpusFiles().map((file) => {
      const text = fs.readFileSync(file, 'utf8');
      return { file: path.relative(CORPUS, file), ...run(yaml, pool, text, site) };
    });
    writeGz('corpus.json.gz', { site, pool: pool.values, cases });
    const errors = cases.filter((c) => c.error || c.initError);
    process.stderr.write(`corpus: ${cases.length} cases, ${errors.length} throw\n`);
  },
};

// Site configs for golden set 4. Each one is merged over { startOnLoad: false },
// as the reference renderer does.
const SITES = [
  {},
  { theme: 'dark' },
  { theme: 'forest' },
  { theme: 'neutral' },
  { theme: 'base' },
  { theme: 'default' },
  { theme: 'neo' },
  { theme: 'neo-dark' },
  { theme: 'redux' },
  { theme: 'redux-dark' },
  { theme: 'redux-color' },
  { theme: 'redux-dark-color' },
  {
    theme: 'base',
    themeVariables: {
      primaryColor: '#BB2528',
      primaryTextColor: '#fff',
      primaryBorderColor: '#7C0000',
      lineColor: '#F8B229',
      secondaryColor: '#006100',
      tertiaryColor: '#fff',
    },
  },
  { themeVariables: { primaryColor: '#ff0000' } },
  { theme: 'dark', themeVariables: { darkMode: true, fontSize: '20px' } },
  { theme: 'redux-color', themeVariables: { borderColorArray: ['#111', '#222', '#111'], nodeBorder: 'red' } },
  { look: 'handDrawn' },
  { look: 'classic' },
  { look: 'neo' },
  { look: 'bogus' },
  { fontFamily: 'Comic Sans MS' },
  { fontFamily: 'arial', themeVariables: { fontFamily: 'courier' } },
  { layout: 'dagre' },
  { layout: 'elk' },
  { layout: 'tidy-tree' },
  { securityLevel: 'loose' },
  { securityLevel: 'sandbox' },
  { flowchart: { htmlLabels: false } },
  { htmlLabels: false },
  { flowchart: { htmlLabels: true, curve: 'linear', theme: 'forest', look: 'classic', layout: 'dagre' } },
  { theme: 'unknown-theme' },
  { theme: 'null' },
  { theme: null },
  { theme: '' },
  { theme: false },
  { theme: 5 },
  { theme: ['dark'] },
  { sequence: { theme: 'dark', showSequenceNumbers: true, mirrorActors: false } },
  { class: { look: 'handDrawn' }, state: { layout: 'dagre' }, er: { theme: 'neutral' } },
  { secure: ['theme', 'fontFamily'] },
  { themeCSS: '.node rect { fill: red; }', darkMode: true, logLevel: 'fatal', maxTextSize: 100 },
  { deterministicIds: true, deterministicIDSeed: 'abc', arrowMarkerAbsolute: true, wrap: true, fontSize: 20 },
  {
    gantt: { displayMode: 'compact', tickInterval: '1week', useWidth: 500 },
    xyChart: { xAxis: { showLabel: false }, useWidth: 300 },
    themeVariables: { xyChart: { titleColor: 'red' } },
  },
  { theme: 'forest', flowchart: { theme: 'bogus' } },
  { flowchart: { theme: 'null' } },
  { unknownKey: { a: { b: { c: 1 } } }, journey: { sectionFills: ['#000'], actorColours: ['#8FBC8F', '#111'] } },
  { layout: null, look: null },
  { startOnLoad: true },
  // Site configs that make initialize() throw.
  { theme: 'dark', themeVariables: { primaryColor: 'notacolor' } },
  { fontFamily: 'x', themeVariables: 'oops' },
];

// sampleFixtures picks n corpus fixtures: every fixture that sets a config in
// its front matter or has a directive gets a higher weight, and every
// top-level directory is represented.
const sampleFixtures = (n) => {
  const rnd = mulberry32(0x73697465); // "site"
  const files = corpusFiles();
  const byDir = new Map();
  for (const f of files) {
    const dir = path.relative(CORPUS, f).split(path.sep)[0];
    if (!byDir.has(dir)) byDir.set(dir, []);
    byDir.get(dir).push(f);
  }
  const chosen = new Set();
  const take = (list) => {
    const rest = list.filter((f) => !chosen.has(f));
    if (rest.length) chosen.add(rest[Math.floor(rnd() * rest.length)]);
  };
  for (const list of byDir.values()) take(list);
  const interesting = files.filter((f) => /^config:|%%\{/m.test(fs.readFileSync(f, 'utf8')));
  while (chosen.size < Math.min(n, files.length)) take(rnd() < 0.6 ? interesting : files);
  return [...chosen].sort();
};

commands.site = async () => {
  const yaml = await loadYaml();
  const pool = new Pool();
  const fixtures = sampleFixtures(100);
  const sites = SITES.map((site) => ({ startOnLoad: false, ...site }));
  const cases = [];
  for (let si = 0; si < sites.length; si++) {
    for (const file of fixtures) {
      const text = fs.readFileSync(file, 'utf8');
      cases.push({ file: path.relative(CORPUS, file), site: si, ...run(yaml, pool, text, sites[si]) });
    }
  }
  writeGz('site.json.gz', { sites: sites.map(encode), pool: pool.values, cases });
  const errors = cases.filter((c) => c.error || c.initError);
  process.stderr.write(`site: ${sites.length} sites x ${fixtures.length} fixtures = ${cases.length} cases, ${errors.length} throw\n`);
};

// Characters that the generators need and that are easy to lose in a source
// file.
const BOM = String.fromCodePoint(0xfeff);
const NBSP = String.fromCodePoint(0xa0);
const LS = String.fromCodePoint(0x2028);
const EMSP = String.fromCodePoint(0x2003);

// Hand-written texts for the preprocessing steps: front matter, directives,
// comments, sanitization and type detection.
const TEXT_CASES = [
  // Front matter.
  '---\ntitle: T\n---\ngraph TD\nA-->B\n',
  '---\ntitle: Gantt\ndisplayMode: compact\n---\ngantt\n  section A\n  t1: 2020-01-01, 1d\n',
  '---\ndisplayMode: compact\nconfig:\n  gantt:\n    displayMode: other\n    topAxis: true\n---\ngantt\n',
  '---\ndisplayMode: compact\nconfig:\n  gantt: text\n---\ngantt\n',
  '---\ndisplayMode: compact\nconfig: abc\n---\ngantt\n',
  '---\ndisplayMode: compact\nconfig: 5\n---\ngantt\n',
  '---\ndisplayMode: compact\nconfig: true\n---\ngantt\n',
  '---\ndisplayMode: compact\nconfig: [1, 2]\n---\ngantt\n',
  '---\nconfig: abc\n---\ngantt\n',
  '---\nconfig: [{theme: dark}, {look: classic}]\n---\ngraph TD\n',
  '---\ndisplayMode: 0\ntitle: 0\n---\ngantt\n',
  '---\ndisplayMode: false\ntitle: false\n---\ngantt\n',
  '---\ndisplayMode: [a, b]\ntitle: [x, [y, z]]\n---\ngantt\n',
  '---\ndisplayMode: {a: 1}\ntitle: {b: 2}\n---\ngantt\n',
  '---\ntitle: {toString: 1}\n---\ngantt\n',
  '---\ndisplayMode: [[]]\ntitle: [[]]\n---\ngantt\n',
  '---\ntitle: 123\n---\ngraph TD\n',
  '---\ntitle: 1.50\n---\ngraph TD\n',
  '---\ntitle: true\n---\ngraph TD\n',
  '---\ntitle: null\n---\ngraph TD\n',
  "---\ntitle: ''\n---\ngraph TD\n",
  '---\ntitle: >-\n  folded\n  title\n---\ngraph TD\n',
  '  ---\n  title: indented\n  config:\n    theme: dark\n  ---\n  graph TD\n  A-->B\n',
  '\t---\n\ttitle: tabbed\n\tconfig:\n\t  theme: dark\n\t---\n\tgraph TD\n',
  '---\r\ntitle: crlf\r\nconfig:\r\n  theme: forest\r\n---\r\ngraph TD\r\n  A-->B\r\n',
  '---\rtitle: cr\r---\rgraph TD\rA-->B\r',
  '---\ntitle: x\ngraph TD\n',
  '---\n---\ngraph TD\n',
  '---\n\n---\n\nMORE\n---\ngraph TD\n',
  '---\ntitle: a\n---\n\n\n\ngraph TD\n',
  '---   \ntitle: a\n---   \n  \ngraph TD\n',
  '\n---\ntitle: a\n---\ngraph TD\n',
  '---\ntitle: a\n---',
  '---\ntitle: a\n---\n',
  '---\ntitle: [unclosed\n---\ngraph TD\n',
  '---\ntitle: a\ntitle: b\n---\ngraph TD\n',
  '---\na: 1\n---\nb: 2\n---\ngraph TD\n',
  '---\n- a\n- b\n---\ngraph TD\n',
  '---\njust text\n---\ngraph TD\n',
  '---\n# only a comment\n---\ngraph TD\n',
  '---\ntitle: "<b class="x">bold</b>"\n---\ngraph TD\n',
  '---\ntitle: \'<b class="x">bold</b>\'\n---\ngraph TD\nA["<i id="a" class="b c">x</i>"]\n',
  // Config in front matter.
  '---\nconfig:\n  theme: dark\n---\ngraph TD\n',
  '---\nconfig:\n  theme: forest\n  look: handDrawn\n  layout: dagre\n---\nflowchart LR\n',
  '---\nconfig:\n  theme: bogus\n  look: bogus\n  layout: bogus\n---\nflowchart LR\n',
  "---\nconfig:\n  theme: 'null'\n---\nflowchart LR\n",
  '---\nconfig:\n  theme: null\n  look: null\n  layout: null\n---\nflowchart LR\n',
  '---\nconfig:\n  flowchart:\n    theme: dark\n    look: classic\n    layout: dagre\n    curve: linear\n---\nflowchart LR\n',
  '---\nconfig:\n  flowchart:\n    theme: bogus\n  theme: neutral\n---\nflowchart LR\n',
  '---\nconfig:\n  flowchart:\n    theme: dark\n---\nsequenceDiagram\n',
  '---\nconfig:\n  sequence:\n    theme: dark\n    mirrorActors: false\n---\nsequenceDiagram\n',
  '---\nconfig:\n  class:\n    look: handDrawn\n  state:\n    layout: dagre\n---\nclassDiagram\n',
  '---\nconfig:\n  class:\n    look: handDrawn\n  state:\n    layout: dagre\n---\nstateDiagram-v2\n',
  '---\nconfig:\n  xyChart:\n    theme: forest\n    width: 300\n---\nxychart-beta\n',
  '---\nconfig:\n  railroad:\n    theme: dark\n---\nrailroad-ebnf-beta\n',
  '---\nconfig:\n  theme: base\n  themeVariables:\n    primaryColor: "#BB2528"\n    primaryTextColor: "#fff"\n    lineColor: "#F8B229"\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    primaryColor: "#ff0000"\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    primaryColor: "#ff0000"\n---\npie\n',
  '---\nconfig:\n  theme: dark\n  themeVariables:\n    darkMode: true\n    fontSize: 20px\n    xyChart:\n      titleColor: red\n      bogusKey: 1\n---\nxychart-beta\n',
  '---\nconfig:\n  themeVariables:\n    primaryColor: "red; background: url(x)"\n    secondaryColor: "rgb(1, 2, 3)"\n    tertiaryColor: "hsl(10, 20%, 30%)"\n    lineColor: "a/b"\n    notAVariable: red\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    primaryColor: notacolor\n---\ngraph TD\n',
  '---\nconfig:\n  theme: forest\n  themeVariables:\n    primaryColor: 5\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables: text\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables: "a-b"\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables: 5\n---\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    - red\n    - "a:b"\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: monospace\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: monospace\n  themeVariables:\n    fontFamily: serif\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: monospace\n  themeVariables: abc\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: 12\n---\ngraph TD\n',
  '---\nconfig:\n  altFontFamily: true\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: "a { b } c"\n  altFontFamily: "a } b {"\n  themeCSS: ".x { fill: red }"\n---\ngraph TD\n',
  '---\nconfig:\n  themeCSS: ".x { fill: red"\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: "<script>"\n  look: "a>b"\n  layout: "url(data:x)"\n---\ngraph TD\n',
  '---\nconfig:\n  securityLevel: loose\n  startOnLoad: true\n  maxTextSize: 10\n  maxEdges: 1\n  suppressErrorRendering: true\n  secure: [theme]\n  theme: dark\n---\ngraph TD\n',
  '---\nconfig:\n  __proto__:\n    theme: dark\n  __x: 1\n  prototype: 2\n  constructor: 3\n  unknownKey: 4\n  theme: forest\n---\ngraph TD\n',
  '---\nconfig:\n  sankey:\n    nodeColors:\n      a: "#fff"\n      b: "rgb(1, 2, 3)"\n      c: "url(x)"\n      d: 5\n      __e: red\n      proto: red\n      f: Red\n    width: 300\n---\nsankey-beta\n',
  '---\nconfig:\n  treeView:\n    filenameIcons:\n      a.txt: "mdi:file"\n      b: "bad icon"\n    extensionIcons:\n      ts: vscode-icons:file-type-typescript\n      js: 5\n---\ntreeView-beta\n',
  '---\nconfig:\n  journey:\n    sectionFills: ["#000"]\n    actorColours: ["#111"]\n    width: 99\n---\njourney\n',
  '---\nconfig:\n  htmlLabels: false\n  flowchart:\n    htmlLabels: true\n---\ngraph TD\n',
  '---\nconfig:\n  flowchart:\n    htmlLabels: null\n    curve: null\n---\ngraph TD\n',
  '---\nconfig:\n  flowchart: text\n  sequence: 5\n  gantt: [1]\n---\ngraph TD\n',
  '---\nconfig:\n  wrap: true\n  darkMode: true\n  logLevel: 5\n  deterministicIds: true\n  deterministicIDSeed: x\n  elk:\n    mergeEdges: true\n    nodePlacementStrategy: SIMPLE\n---\ngraph TD\n',
  '---\nconfig:\n  gantt:\n    tickInterval: 1week\n    useWidth: 400\n  pie:\n    useWidth: 100\n  c4:\n    flowchart:\n      curve: step\n      subGraphTitleMargin:\n        top: 9\n---\ngantt\n',
  '---\nconfig:\n  sequence:\n    messageFont: text\n    actorFontSize: 20\n---\nsequenceDiagram\n',
  '---\nconfig:\n  theme: dark\n  theme2: x\n  10: a\n  2: b\n---\ngraph TD\n',
  // Directives.
  '%%{init: {"theme": "dark"}}%%\ngraph TD\nA-->B\n',
  "%%{init: {'theme': 'forest'}}%%\ngraph TD\nA-->B\n",
  '%%{initialize: {"theme": "neutral"}}%%\ngraph TD\n',
  '%%{ init : { "theme": "dark" } }%%\ngraph TD\n',
  '%%{reinit: {"theme": "dark"}}%%\ngraph TD\n',
  '%%{initx: {"theme": "dark"}}%%\ngraph TD\n',
  '%%{init: {"config": {"curve": "linear"}}}%%\nflowchart LR\n',
  '%%{init: {"config": {"mirrorActors": false}, "theme": "dark"}}%%\nsequenceDiagram\n',
  '%%{init: {"config": {"x": 1}}}%%\nclassDiagram\n',
  '%%{init: {"config": {"barHeight": 9}, "gantt": {"barGap": 1}}}%%\ngantt\n',
  '%%{init: {"theme": "dark"}}%%\n%%{init: {"look": "classic"}}%%\ngraph TD\n',
  '%%{init: {"theme": "dark", "flowchart": {"curve": "linear"}}}%%\n%%{init: {"theme": "forest", "flowchart": {"padding": 3}}}%%\n%%{init: {"fontFamily": "x"}}%%\ngraph TD\n',
  '%%{init: {"theme": "dark"}}%%\n%%{init: {"fontFamily": 5}}%%\ngraph TD\n',
  '%%{init: {"theme": "dark"}}%%\n%%{init: dark}%%\ngraph TD\n',
  '%%{init: {theme: dark}}%%\ngraph TD\n',
  '%%{wrap}%%\n%%{init: {bad json}}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{wrap}%%\n%%{init: {bad json}}%%\ngraph TD\n',
  '%%{init: {bad json}}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{init: {"theme": "dark"}}%%\n%%{init: {bad}}%%\n%%{wrap}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{init: {bad}}%%\n%%{init: {bad}}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{wrap}%%\ngraph TD\n',
  '%%{wrap}%%\n%%{wrap}%%\nsequenceDiagram\n',
  '%%{nowrap}%%\ngraph TD\n',
  '%%{wrapper}%%\n%%{nowrap}%%\ngraph TD\n',
  '%%{wrap: true}%%\ngraph TD\n',
  '%%{init: dark}%%\ngraph TD\n',
  '%%{init: dark}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{init: 5}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{init: "str"}%%\ngraph TD\n',
  '%%{init: [1, 2]}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{init: [{"theme": "dark"}]}%%\ngraph TD\n',
  '%%{init: true}%%\n%%{wrap}%%\ngraph TD\n',
  '%%{init: null}%%\ngraph TD\n',
  '%%{init: 0}%%\ngraph TD\n',
  '%%{init: {}}%%\ngraph TD\n',
  '%%{init}%%\ngraph TD\n',
  '%%{init: {"theme": "dark"}}%%\nnot a diagram\n',
  '%%{init: {}}%%\nnot a diagram\n',
  '%%{init: {bad}}%%\nnot a diagram\n',
  '%%{wrap}%%\nnot a diagram\n',
  'not a diagram\n',
  'zenuml\n  A.method()\n',
  '',
  '   \n\n',
  'graph TD\nA-->B\n%%{init: {"theme": "dark"}}%%\nB-->C\n',
  '%%{init: {\n  "theme": "dark",\n  "themeVariables": {\n    "primaryColor": "#ff0000"\n  }\n}}%%\ngraph TD\n',
  '%%{init: {"theme": "dark"}\ngraph TD\nA-->B\n',
  '%%{init: {"theme": "dark"}} %%\ngraph TD\n',
  'graph TD\nA-->B %%{init: {"look": "classic"}}%% C-->D\n',
  '%%{init: {"themeVariables": {"primaryColor": "#ff0000"}}}%%\ngraph TD\n',
  '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#ff0000", "darkMode": true}}}%%\nsequenceDiagram\n',
  '%%{init: {"themeVariables": {"primaryColor": "url(x)"}}}%%\ngraph TD\n',
  '%%{init: {"themeVariables": {"primaryColor": "nope"}}}%%\ngraph TD\n',
  '%%{init: {"fontFamily": "monospace"}}%%\ngraph TD\n',
  '%%{init: {"securityLevel": "loose", "theme": "dark", "secure": ["x"]}}%%\ngraph TD\n',
  '%%{init: {"flowchart": {"curve": "linear", "htmlLabels": true, "theme": "forest"}}}%%\ngraph TD\n',
  '%%{init: {"sequence": {"mirrorActors": false, "look": "classic"}}}%%\nsequenceDiagram\n',
  '%%{init: {"look": "handDrawn", "layout": "elk"}}%%\nstateDiagram-v2\n',
  '%%{init: {"theme": "dark"}}%%\n---\ntitle: late\n---\ngraph TD\n',
  '---\nconfig:\n  theme: forest\n  look: classic\n---\n%%{init: {"theme": "dark"}}%%\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    primaryColor: "#00ff00"\n    lineColor: "#0000ff"\n---\n%%{init: {"themeVariables": {"primaryColor": "#ff0000"}}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart:\n    curve: linear\n    padding: 1\n  secure: [a, b]\n---\n%%{init: {"flowchart": {"curve": "step"}, "secure": ["c"]}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart: [1, 2, 3]\n---\n%%{init: {"flowchart": [9]}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart:\n    curve: linear\n---\n%%{init: {"flowchart": null, "theme": null}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart: text\n---\n%%{init: {"flowchart": {"curve": "step"}}}%%\ngraph TD\n',
  '---\nconfig:\n  theme: &t dark\n  flowchart: &f\n    curve: linear\n  sequence: *f\n  look: *t\n---\ngraph TD\n',
  '---\ntitle: T\n---\n%%{init: {"config": {"curve": "linear"}}}%%\n%%{wrap}%%\nflowchart TD\n',
  // A null in an array of the front matter or of a directive. es-toolkit
  // 1.45.1 (the mermaid bundle) reads it as undefined when it merges the two,
  // so it does not replace a value. es-toolkit 1.53.1 (node_modules) keeps it.
  '---\nconfig:\n  flowchart: [1, 2, 3]\n---\n%%{init: {"flowchart": [null, 9]}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart: [1, null, 3]\n---\ngraph TD\n',
  '---\nconfig:\n  flowchart: [1, ~, 3]\n---\n%%{init: {"flowchart": [null, null, null, 4]}}%%\ngraph TD\n',
  '%%{init: {"flowchart": [null]}}%%\ngraph TD\n',
  '%%{init: {"flowchart": {"curve": [null, "step"]}}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart:\n    curve: [linear, basis]\n---\n%%{init: {"flowchart": {"curve": [null, "step"]}}}%%\ngraph TD\n',
  '---\nconfig:\n  flowchart:\n    curve: [[null, a], null]\n---\n%%{init: {"flowchart": {"curve": [["b", null]]}}}%%\ngraph TD\n',
  '---\nconfig:\n  secure: [a, null, b]\n---\ngraph TD\n',
  '%%{init: {"secure": [null]}}%%\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    cScale0: [red, blue]\n---\n%%{init: {"themeVariables": {"cScale0": [null, "green"]}}}%%\ngraph TD\n',
  '---\nconfig:\n  themeVariables:\n    primaryColor: [~, "#ff0000"]\n---\ngraph TD\n',
  '---\nconfig:\n  fontFamily: [~, serif]\n---\ngraph TD\n',
  '---\nconfig:\n  theme: [dark]\n---\n%%{init: {"theme": [null]}}%%\ngraph TD\n',
  '%%{init: {"theme": [null, "dark"]}}%%\ngraph TD\n',
  '---\nconfig:\n  sequence:\n    messageFontFamily: [a, b]\n---\n%%{init: {"sequence": {"messageFontFamily": [null, null, "c"]}}}%%\nsequenceDiagram\n',
  '---\nconfig:\n  flowchart:\n    padding: {a: [1, {b: [2, 3]}]}\n---\n%%{init: {"flowchart": {"padding": {"a": [null, {"b": [null, 4]}]}}}}%%\ngraph TD\n',
  // A null that is not in an array replaces the value in both versions.
  '---\nconfig:\n  flowchart:\n    curve: [linear]\n---\n%%{init: {"flowchart": {"curve": null}}}%%\ngraph TD\n',
  // Comments.
  '%% a comment\ngraph TD\nA-->B\n',
  '   %% indented comment\n\n\n  %% another\ngraph TD\n  %% inner\nA-->B %% trailing\n%%last',
  'graph TD\n%%\nA-->B\n%%x\n',
  'graph TD\n%%{ not a directive\nA-->B\n',
  '%%}%% odd\ngraph TD\n',
  'graph TD\n%%}%% odd line\nA-->B\n%%{init: {"theme": "dark"}}%%\n',
  '%% comment with %%{init: {"theme": "dark"}}%% inside\ngraph TD\n',
  '\n\n%% c1\n\n%% c2\n\ngraph TD\n',
  'graph TD' + LS + '%% after line separator\nA-->B\n',
  'graph TD\r\n%% crlf comment\r\nA-->B\r\n',
  '%% only a comment',
  '%% only a comment\n',
  // Type detection.
  'graph TD', 'flowchart LR', 'flowchart-elk TD', '  \n\t flowchart-elk TD', 'flowchart-elkx', 'graphx', 'Graph TD',
  'sequenceDiagram', 'classDiagram', 'classDiagram-v2', 'stateDiagram', 'stateDiagram-v2', 'erDiagram', 'gantt',
  'pie', 'pie title x', 'journey', 'gitGraph', 'gitGraph:', 'info', 'information', 'quadrantChart', 'xychart',
  'xychart-beta', 'requirement', 'requirementDiagram', 'mindmap', 'timeline', 'kanban', 'sankey', 'sankey-beta',
  'packet', 'packet-beta', 'radar-beta', 'radar', 'block', 'block-beta', 'treeView-beta', 'treeView',
  'architecture-beta', 'architecture', 'eventmodeling', 'ishikawa', 'ishikawa-beta', 'Ishikawa-Beta', 'ISHIKAWA',
  'ishikawax', 'ishikawa-betax', 'venn-beta', 'venn', 'treemap', 'treemap-beta', 'usecase-beta', 'usecase-beta\n',
  'usecase-betax', 'usecase-beta x', 'wardley-beta', 'WARDLEY-BETA', 'wardley', 'cynefin-beta', 'cynefin-beta:',
  'cynefin-beta x', 'cynefin-betax', 'railroad-beta', 'RAILROAD-BETA', 'railroad-ebnf-beta', 'Railroad-Ebnf-Beta',
  'railroad-abnf-beta', 'railroad-peg-beta', 'RAILROAD-PEG-BETA', 'swimlane-beta', 'swimlane-betax',
  'swimlane-beta-x', 'agentflow-beta', 'agentflow-beta TB', 'agentflow-betax', 'C4Context', 'C4Container',
  'C4Component', 'C4Dynamic', 'C4Deployment', '  C4Context', 'graph TD\nA[C4Container]', 'x C4Context',
  'error', ' Error \n', 'ERROR', 'errors', '---', '  ---x', '--', 'zenuml', 'unknown',
  BOM + 'graph TD', NBSP + 'graph TD', EMSP + '\n' + LS + 'sequenceDiagram', '\u000bgraph TD', '\u200bgraph TD',
  '\n\n\n   pie', '%%{init: {}}%%\n  mindmap', '%% c\n%% d\n   timeline', 'graph TD\n%% c', '\n%% c\ngraph',
  '---\ntitle: x\n---\n   \n  erDiagram', 'stateDiagram-v2\n---\n', 'x\n%% no newline graph',
];

// randomText composes a diagram text from fragments. Most results are valid.
// The others exercise the paths where upstream throws.
const randomText = (rnd) => {
  const pick = (a) => a[Math.floor(rnd() * a.length)];
  const maybe = (p, v) => (rnd() < p ? v : '');
  const yamlLines = [
    'title: Random title', 'title: 42', 'displayMode: compact', 'description: >-\n  two\n  lines',
    'config:\n  theme: dark', 'config:\n  theme: forest\n  look: classic', 'config:\n  look: handDrawn',
    'config:\n  layout: dagre', 'config:\n  theme: base\n  themeVariables:\n    primaryColor: "#aabbcc"',
    'config:\n  themeVariables:\n    lineColor: "#123456"\n    fontSize: 18px', 'config:\n  fontFamily: courier',
    'config:\n  flowchart:\n    curve: linear\n    htmlLabels: false', 'config:\n  sequence:\n    mirrorActors: false',
    'config:\n  securityLevel: loose\n  theme: neutral', 'config:\n  themeVariables:\n    primaryColor: "a:b"',
    'config:\n  gantt:\n    barHeight: 30', 'config:\n  state:\n    theme: forest\n    layout: dagre',
    'config:\n  class:\n    look: neo\n    theme: redux-dark', 'config:\n  theme: redux-dark-color',
    'config:\n  theme: neo\n  darkMode: true', 'config: text', 'x: [1, 2', 'config:\n  htmlLabels: false',
  ];
  const directives = [
    '%%{init: {"theme": "dark"}}%%', "%%{init: {'theme': 'forest'}}%%", '%%{init: {"look": "classic"}}%%',
    '%%{init: {"layout": "dagre"}}%%', '%%{init: {"themeVariables": {"primaryColor": "#ff8800"}}}%%',
    '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#00ff88", "darkMode": true}}}%%',
    '%%{init: {"fontFamily": "verdana"}}%%', '%%{init: {"config": {"curve": "step"}}}%%', '%%{wrap}%%',
    '%%{init: {"flowchart": {"curve": "basis"}, "sequence": {"wrap": true}}}%%', '%%{init: {bad}}%%',
    '%%{initialize: {"theme": "neutral", "securityLevel": "loose"}}%%', '%%{init: dark}%%', '%%{nowrap}%%',
    '%%{init: {"theme": "redux"}}%%', '%%{init: {"state": {"look": "classic"}, "class": {"theme": "dark"}}}%%',
  ];
  const diagrams = [
    'graph TD\n  A-->B', 'flowchart LR\n  A --> B', 'sequenceDiagram\n  A->>B: hi', 'classDiagram\n  class A',
    'stateDiagram-v2\n  [*] --> A', 'erDiagram\n  A ||--o{ B : has', 'gantt\n  title G', 'pie\n  "a": 1',
    'journey\n  title J', 'gitGraph\n  commit', 'mindmap\n  root', 'timeline\n  title T', 'xychart-beta\n  bar [1]',
    'quadrantChart\n  title Q', 'requirementDiagram\n  requirement r {}', 'kanban\n  todo', 'block-beta\n  a',
    'architecture-beta\n  group g', 'sankey-beta\n  a,b,1', 'packet-beta\n  0-1: "x"', 'radar-beta\n  axis a',
    'treemap\n  "a"', 'venn-beta\n  set A', 'C4Context\n  Person(a, "A")', 'flowchart-elk TD\n  A-->B', 'info',
    'usecase-beta\n  actor A', 'cynefin-beta\n  clear', 'swimlane-beta\n  lane a', 'agentflow-beta TB\n  a',
    'not a diagram', 'zenuml\n  A.b()', 'railroad-beta\n  a = b', 'eventmodeling\n  x', 'ishikawa-beta\n  x',
    'treeView-beta\n  a', 'wardley-beta\n  title w',
  ];
  const eol = pick(['\n', '\n', '\n', '\r\n']);
  let text = maybe(0.1, pick(['\n', '  \n', '\t']));
  if (rnd() < 0.6) {
    const indent = maybe(0.1, '  ');
    // One line for each key, so that most front matter is valid YAML.
    const byKey = new Map();
    const n = 1 + Math.floor(rnd() * 3);
    for (let i = 0; i < n; i++) {
      const line = pick(yamlLines);
      const key = line.slice(0, line.indexOf(':'));
      if (!byKey.has(key) || rnd() < 0.03) byKey.set(key + (byKey.has(key) ? ' ' : ''), line);
    }
    const lines = [...byKey.values()];
    text += `${indent}---\n${lines.join('\n').split('\n').map((l) => indent + l).join('\n')}\n${indent}---\n`;
    text += maybe(0.2, '\n');
  }
  const nd = rnd() < 0.5 ? 0 : 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < nd; i++) text += pick(directives) + maybe(0.9, '\n') + maybe(0.2, '%% note\n');
  text += maybe(0.2, '  %% leading comment\n') + maybe(0.1, '   ');
  text += pick(diagrams) + '\n' + maybe(0.3, '  %% inner comment\n') + maybe(0.15, pick(directives) + '\n') + maybe(0.5, '  X --> Y\n');
  return text.replaceAll('\n', eol);
};

commands.text = async () => {
  const yaml = await loadYaml();
  const pool = new Pool();
  const sites = [
    { startOnLoad: false },
    { startOnLoad: false, theme: 'forest', themeVariables: { primaryColor: '#00ff00', lineColor: 'red' }, secure: ['look'] },
    { startOnLoad: false, fontFamily: 'fantasy', look: 'handDrawn', flowchart: { theme: 'dark' }, sequence: { look: 'classic' } },
    undefined, // mermaid.initialize() with no argument
  ];
  const rnd = mulberry32(0x74657874); // "text"
  const inputs = TEXT_CASES.map((text) => ({ text, site: 0 }));
  for (const text of TEXT_CASES.slice(0, 60)) inputs.push({ text, site: 3 });
  for (const text of TEXT_CASES.filter((t) => /config:|init/.test(t))) {
    inputs.push({ text, site: 1 }, { text, site: 2 });
  }
  for (let i = 0; i < 1500; i++) inputs.push({ text: randomText(rnd), site: Math.floor(rnd() * 3) });
  // ORDER=reverse runs the cases backwards. The records must not change:
  // that shows that no state leaks from one case to the next.
  const order = inputs.map((_, i) => i);
  if (process.env.ORDER === 'reverse') order.reverse();
  const cases = new Array(inputs.length);
  for (const i of order) {
    cases[i] = { text: inputs[i].text, site: inputs[i].site, ...run(yaml, pool, inputs[i].text, sites[inputs[i].site]) };
  }
  // ORDER=reverse prints one digest for each case instead of the golden file.
  // DIGEST=1 prints the same digests for the normal order.
  if (process.env.ORDER === 'reverse' || process.env.DIGEST) {
    const digest = (c) => {
      const config = c.config === undefined ? undefined : pool.values[c.config].map(([k, i]) => [k, pool.values[i]]);
      const full = { ...c, directive: pool.values[c.directive], config };
      return crypto.createHash('sha1').update(JSON.stringify(full)).digest('hex');
    };
    process.stdout.write(cases.map(digest).join('\n') + '\n');
    return;
  }
  writeGz('text.json.gz', { sites: sites.map(encode), pool: pool.values, cases });
  const errors = cases.filter((c) => c.error || c.initError);
  const names = {};
  for (const c of errors) names[(c.error ?? c.initError).name] = (names[(c.error ?? c.initError).name] ?? 0) + 1;
  process.stderr.write(`text: ${cases.length} cases, ${errors.length} throw ${JSON.stringify(names)}\n`);
};

// Hand-written YAML documents for the front matter parser. The Go port reads
// YAML with another library than upstream, so these cover the places where
// YAML parsers differ: scalar types, keys, block scalars, tags and errors.
const YAML_CASES = [
  // Numbers.
  'a: 1', 'a: -5', 'a: +5', 'a: 1.5', 'a: -1.5', 'a: 1e3', 'a: 1E3', 'a: 1e+3', 'a: 1.5e-3', 'a: .5', 'a: -.5',
  'a: +.5', 'a: 5.', 'a: 0', 'a: -0', 'a: +0', 'a: 0.0', 'a: -0.0', 'a: 00', 'a: 007', 'a: 0777', 'a: 08',
  'a: 0x1F', 'a: 0X1F', 'a: 0xff', 'a: -0x10', 'a: 0x', 'a: 0xg', 'a: 0o17', 'a: 0o8', 'a: 0o', 'a: 0b101',
  'a: 0b102', 'a: 0b', 'a: 1_000', 'a: 1_000.5', 'a: 0x_1F', 'a: .inf', 'a: -.INF', 'a: +.Inf', 'a: .NaN',
  'a: .nan', 'a: .NAN', 'a: .iNf', 'a: inf', 'a: NaN', 'a: Infinity', 'a: 1e400', 'a: -1e400', 'a: 1e-400',
  'a: 123456789012345678901234567890', 'a: 9007199254740993', 'a: 0x20000000000001', 'a: 1.7976931348623157e308',
  'a: 12e', 'a: e3', 'a: 1.2.3', 'a: 1:30', 'a: 1:30:00', 'a: 190:20:30.15', 'a: 12:30', 'a: 1,000', 'a: 0.1',
  'a: 3.14159265358979', 'a: 100%', 'a: 16px', 'a: #fff', 'a: "#fff"', 'a: 1 2', 'a: - 1', 'a: 1 # c',
  // Booleans and null.
  'a: true', 'a: True', 'a: TRUE', 'a: tRue', 'a: false', 'a: False', 'a: FALSE', 'a: yes', 'a: Yes', 'a: no',
  'a: on', 'a: off', 'a: y', 'a: n', 'a: Y', 'a: N', 'a: ~', 'a: null', 'a: Null', 'a: NULL', 'a: nUll', 'a:',
  'a: ', 'a: # only a comment', 'a:\nb: 1', '~: 1', 'null: 1', 'a: nil', 'a: none', 'a: undefined',
  // Dates, which YAML 1.1 parsers turn into timestamps.
  'a: 2001-01-01', 'a: 2001-01-01T10:00:00Z', 'a: 2001-1-1', 'a: 2001-01-01 10:00:00', 'a: 2024-02-30',
  // Strings.
  'a: text', 'a: "1"', "a: '1'", 'a: "true"', "a: 'null'", 'a: ""', "a: ''", 'a: "a\\nb"', 'a: "a\\tb\\\\c\\"d"',
  'a: "\\x41\\u00e9\\U0001F600"', 'a: "\\/"', 'a: "\\0\\a\\b\\e\\f\\v\\ \\_\\N\\L\\P"', 'a: "\\q"', "a: 'it''s'",
  "a: 'a\\nb'", 'a: a "quoted" word', "a: it's", 'a: a: b', 'a: a:b', 'a: a :b', 'a: a #b', 'a: a#b',
  'a: http://x.y/z?q=1#frag', 'a: [not, closed', 'a: {not: closed', "a: 'unterminated", 'a: "unterminated',
  'a: @at', 'a: `tick`', 'a: %pct', 'a: !bang', 'a: &anchor', 'a: *alias', 'a: |', 'a: >', 'a: -', 'a: ?',
  'a: :', 'a: ,', 'a: -x', 'a: ?x', 'a: :x', 'a: [', 'a: ]', 'a: {', 'a: }', 'a: "x" y', "a: 'x' y",
  'a: plain\n  continued\n  lines', 'a: plain\n\n  after blank', 'a: "double\n  folded\n\n  lines"',
  "a: 'single\n  folded\n\n  lines'", 'a: "trailing \\\n  escape"', 'a: h\u00e9llo \u2603', 'a: \ud83d\ude00',
  '"\ud83d\ude00": 1', 'a: "tab\there"', 'a: text with trailing spaces   ', 'a:    leading spaces',
  'a: text\t# tab before comment', 'key with spaces: v', '"quoted key": v', "'single key': v", '? complex\n: v',
  '? a\n? b', 'a b: c d', 'a.b: 1', 'a-b: 1', 'a_b: 1', '@a: 1', 'a: b\nc', 'a\nb: 1',
  // Block scalars.
  'a: |\n  l1\n  l2\n', 'a: |-\n  l1\n  l2\n', 'a: |+\n  l1\n  l2\n\n', 'a: |\n  l1\n\n  l3\n', 'a: |2\n    x\n',
  'a: |1\n  x\n', 'a: |-\n\n  x\n\n', 'a: >\n  f1\n  f2\n', 'a: >-\n  f1\n  f2\n', 'a: >+\n  f1\n  f2\n\n',
  'a: >\n  f1\n\n  f2\n', 'a: >\n  f1\n    more\n  f2\n', 'a: >-\n  two\n  lines\nb: 1', 'a: |\n  x\nb: |\n  y\n',
  'a: |\n\tx\n', 'a: >\n x\n  y\n z\n', 'a: |\n  x # not a comment\n', 'a: | # comment\n  x\n', 'a: |\nb: 1',
  'a: >\n\n\n  x\n', 'a: |0\n  x\n', 'a: |10\n  x\n', 'a: |-+\n  x\n', 'a: |\n  x\n y\n',
  // Collections.
  'a: [1, 2, 3]', 'a: [1, [2, [3]]]', 'a: []', 'a: [ ]', 'a: {}', 'a: { }', 'a: {b: 1, c: [1, {d: x}]}',
  'a: [a: 1, b: 2]', 'a: {b}', 'a: {b, c: 1}', 'a: [1, 2,]', 'a: [,]', 'a: {b: 1,}', 'a: [1 2]', 'a: {b: 1 c: 2}',
  'a:\n  - 1\n  - 2', 'a:\n- 1\n- 2', 'a:\n  - b: 1\n    c: 2\n  - d', '- a\n- b', '- - a\n  - b\n- c', '- a: 1\n  b: 2',
  'a:\n  b:\n    c:\n      d: 1', 'a:\n    b: 1\n  c: 2', 'a:\n  b: 1\n c: 2', 'a:\n  b: 1\n    c: 2', 'a: 1\n b: 2',
  'a: [\n  1,\n  2\n]', 'a: {\n  b: 1\n}', 'a: [1, 2] # c', '[a, b]: c', '{a: 1}: c', '[[a]]: c', '? [a, b]\n: c',
  // Keys.
  '1: a', '1.5: b', '1.0: c', 'true: c', 'True: c', 'false: d', '0x10: f', '0o17: g', '1e3: h', '.5: i',
  '-0: j', '0: a\n-0: b', '1: a\n1.0: b', '1: a\n"1": b', 'a: 1\na: 2', 'a: 1\n"a": 2', 'a: {b: 1, b: 2}',
  'b: 1\n10: x\n2: y\na: z\n-1: w\n1.5: v', '9007199254740993: x', '123456789012345678901234567890: x',
  '__proto__: {x: 1}', '__proto__: 1\na: 2', 'constructor: x', 'toString: y', 'hasOwnProperty: z', 'length: 3',
  '"": empty', '? \n: v', ': v', 'a: 1\n: 2',
  // Anchors and aliases.
  'a: &x {b: 1}\nc: *x', 'a: &x [1, 2]\nb: *x', 'a: &s str\nb: *s', 'a: &n 12\nb: *n', 'a: *missing',
  'a: &x\n  b: 1\nc: *x\nd: *x', 'base: &b {x: 1}\nd:\n  <<: *b\n  y: 2', '<<: {a: 1}\nb: 2', 'a: &x {b: *x}',
  'a: &x [*x]', '&k key: v\nother: *k', 'a: &x 1\nb: &x 2\nc: *x',
  // Tags.
  'a: !!str 123', 'a: !!str true', 'a: !!str', 'a: !!int "12"', 'a: !!int 0x1F', 'a: !!int abc', 'a: !!int 1.5',
  'a: !!int', 'a: !!float 1', 'a: !!float "1.5"', 'a: !!float abc', 'a: !!bool "true"', 'a: !!bool yes',
  'a: !!bool', 'a: !!null ""', 'a: !!null null', 'a: !!null x', 'a: !!null', 'a: !custom x', 'a: !custom {b: 1}',
  'a: !!binary aGk=', 'a: !!timestamp 2001-01-01', 'a: !!set {x}', 'a: !!omap [a: 1]', 'a: !!pairs [a: 1]',
  'a: ! 12', 'a: ! "12"', 'a: !!map {b: 1}', 'a: !!seq [1]', 'a: !!map [1]', 'a: !!seq {b: 1}', 'a: !!map',
  'a: !!seq', 'a: !!map x', 'a: !!str [1]', 'a: !<tag:yaml.org,2002:int> 5', 'a: !!merge x', '!!str a: 1',
  '!!map\na: 1', 'a: !!js/function x', 'a: !!python/object x',
  // Documents and directives.
  '', ' ', '\n\n', '# c', '# c\n# d\n', '---', '--- ', '---\na: 1', '--- a', 'a: 1\n...', 'a: 1\n...\n',
  'a: 1\n---\nb: 2', 'a: 1\n...\n---\nb: 2', '--- \n...\n', '%YAML 1.2\n---\na: 1', '%YAML 1.1\n---\na: 1',
  '%YAML 2.0\n---\na: 1', '%TAG !e! tag:example.com,2000:\n---\na: !e!x 1', '%FOO bar\n---\na: 1', 'just a string',
  '12', 'true', 'null', '~', '[1, 2]', '{a: 1}', '"str"', '- 1', '? a', '|\n  block', 'a: 1\n# trailing comment',
  '\ufeffa: 1', 'a: 1\u0000', 'a: \u0007', 'a: "\u0007"', 'a: \u0085x', 'a: x\u2028y', 'a: x\u00a0y', '\u00a0a: 1',
  // Places where js-yaml and yaml.v3 are known to differ (see yaml_test.go).
  '- [x\n- x]', '- - \n- |+\n  x: y', '- -  - # c\n  - #x\n    -  \n    -  x', '-\tx', '-\t- x', '-\ta: 1',
  '-\t{a: 1}', '-\t"a: b"', '-\t"a": b', '\t # comment\n- a', '- -\n- x', '- -\n-\n- x', '-\n  -\n- x', '- a:\n  -\n- x',
  // Tabs and indentation.
  'a:\t1', 'a: 1\t', '\ta: 1', 'a:\n\tb: 1', 'a:\n  b:\t1\n  c: 2', 'a:\n  - 1\n\t- 2', '- a\n\t- b', 'a: [1,\t2]',
  'a: "x\ty"', 'a:\n  b: 1\n\t\n  c: 2', ' a: 1', '  a: 1\n  b: 2', '  a: 1\nb: 2',
];

// randomYaml builds a YAML document from random fragments: block and flow
// collections, scalars of every style, comments, anchors and odd spacing.
const randomYaml = (rnd) => {
  const pick = (a) => a[Math.floor(rnd() * a.length)];
  const int = (n) => Math.floor(rnd() * n);
  const keys = ['title', 'config', 'theme', 'a', 'b', 'key', 'displayMode', 'x1', 'fontFamily', 'flowchart', '1', '2.5', 'true', '"q k"', "'s'", 'null', 'a b', 'look', 'k-1', 'k_2'];
  const scalars = [
    'text', 'two words', '12', '-3', '1.5', '0x1F', '0o17', '1e3', '.5', '007', '1_000', 'true', 'False', 'yes', 'no', 'null', '~', '',
    '"double"', "'single'", '"with: colon"', "'it''s'", '"esc \\n \\t \\" \\\\"', '"\\u0041"', 'dark', 'forest', 'handDrawn',
    '#fff', '"#fff"', 'rgb(1, 2, 3)', 'a:b', 'http://x.y/z', '2001-01-01', '12:30', '100%', '16px', '-', '.inf', '.NaN',
    'x # comment', 'x#y', '"x # y"', '@x', '!!str 5', '!!int "7"', '&anc val', '*anc', 'a, b', '[x', 'x]', '{x', 'x}', '> x', '| x',
    '"multi\n    line"', "'multi\n    line'", 'plain\n    continued', '"trail  "', '  lead',
  ];
  const flow = (depth) => {
    const k = int(depth > 1 ? 4 : 7);
    if (k < 4) return pick(['1', 'x', '"q"', "'s'", 'true', 'null', 'a b', '0x10', '1.5', '']);
    if (k === 4) return '[' + Array.from({ length: int(4) }, () => flow(depth + 1)).join(pick([', ', ',', ' , '])) + ']';
    if (k === 5) return '{' + Array.from({ length: int(3) }, (_, i) => `${pick(['a', 'b', 'c', '1', '"k"'])}${i}${pick([': ', ': ', ':  '])}${flow(depth + 1)}`).join(', ') + '}';
    return '{' + pick(['a', 'b']) + '}';
  };
  const block = (indent, depth) => {
    const pad = ' '.repeat(indent);
    const n = 1 + int(4);
    const lines = [];
    const seq = rnd() < 0.2;
    const used = new Set();
    for (let i = 0; i < n; i++) {
      let key = pick(keys);
      if (used.has(key) && rnd() < 0.9) key += i;
      used.add(key);
      const head = seq ? pad + '-' + pick([' ', ' ', '  ']) : pad + key + pick([':', ':', ':', ' :']) ;
      const kind = int(12);
      if (rnd() < 0.08) lines.push(pad + pick(['# comment', '', '  ', '#x']));
      if (kind < 5) lines.push(head + (seq ? '' : ' ') + pick(scalars) + pick(['', '', '', ' ', ' # c']));
      else if (kind < 7) lines.push(head + (seq ? '' : ' ') + flow(0));
      else if (kind < 9 && depth < 3) {
        const step = pick([2, 2, 2, 4, 1, 3]);
        lines.push(seq ? head.trimEnd() + ' ' + block(0, depth + 1).split('\n').map((l, j) => (j ? pad + '  ' + l : l)).join('\n') : head + '\n' + block(indent + step, depth + 1));
      } else if (kind === 9) {
        const style = pick(['|', '|-', '|+', '>', '>-', '>+', '|2', '>1']);
        const body = Array.from({ length: 1 + int(3) }, () => pad + '  ' + pick(['line', 'two words', '', '  indented', 'x: y', '# not comment', '- item']));
        lines.push(head + (seq ? '' : ' ') + style + '\n' + body.join('\n'));
      } else if (kind === 10 && !seq) lines.push(head + '\n' + pad + pick(['- 1', '- a\n' + pad + '- b', '  - x\n' + pad + '  - y: 1']));
      else lines.push(head + (seq ? ' x' : ''));
    }
    return lines.join('\n');
  };
  let doc = block(pick([0, 0, 0, 0, 2]), 0);
  if (rnd() < 0.05) doc = '---\n' + doc;
  if (rnd() < 0.1) doc += '\n';
  if (rnd() < 0.03) doc = doc.replace(' ', '\t');
  return doc;
};

commands.yaml = async () => {
  const yaml = await loadYaml();
  const bodies = new Set();
  // The front matter of every corpus fixture, as extractFrontMatter passes
  // it to js-yaml.
  for (const file of corpusFiles()) {
    const match = api.matchFrontMatter(cleanupText(fs.readFileSync(file, 'utf8')));
    if (!match) continue;
    const indent = match.indent;
    bodies.add(
      indent
        ? match.body
            .split('\n')
            .map((line) => (line.startsWith(indent) ? line.slice(indent.length) : line))
            .join('\n')
        : match.body
    );
  }
  const corpusCount = bodies.size;
  for (const text of YAML_CASES) bodies.add(text);
  const handCount = bodies.size - corpusCount;
  // YAML_SEED and YAML_RANDOM make a larger sample for a one-off check.
  const rnd = mulberry32(Number(process.env.YAML_SEED ?? 0x79616d6c)); // "yaml"
  const randomCount = Number(process.env.YAML_RANDOM ?? 3000);
  while (bodies.size < corpusCount + handCount + randomCount) bodies.add(randomYaml(rnd));
  const cases = [...bodies].map((text) => {
    try {
      return { text, value: encode(yaml.load(text, { schema: yaml.JSON_SCHEMA })) };
    } catch (e) {
      return { text, error: errorOf(e) };
    }
  });
  writeGz('yaml.json.gz', { corpusCount, handCount, cases });
  process.stderr.write(`yaml: ${corpusCount} corpus documents, ${handCount} hand-written, ${randomCount} random, ${cases.filter((c) => c.error).length} throw\n`);
};

// Scripts of config API calls, as diagram code makes them after
// processAndSetConfigs: the init function of a diagram calls setConfig, and
// mindmap calls getUserDefinedConfig. Each step that reads something records
// it.
commands.api = async () => {
  const pool = new Pool();
  const rnd = mulberry32(0x61706973); // "apis"
  const pick = (a) => a[Math.floor(rnd() * a.length)];
  const fixtures = sampleFixtures(40);
  const sites = [
    { startOnLoad: false },
    { startOnLoad: false, theme: 'forest', themeVariables: { primaryColor: '#00ff00' }, flowchart: { curve: 'linear' } },
    { startOnLoad: false, look: 'classic', layout: 'dagre', fontFamily: 'courier', secure: ['layout'] },
    { startOnLoad: false, theme: 'base', sequence: { theme: 'dark' }, htmlLabels: false },
  ];
  const objects = [
    { layout: 'elk' },
    { layout: 'dagre' },
    { flowchart: { arrowMarkerAbsolute: true } },
    { flowchart: { arrowMarkerAbsolute: false, htmlLabels: false } },
    { theme: 'dark' },
    { theme: 'neutral', themeVariables: { primaryColor: '#abcdef' } },
    { look: 'handDrawn' },
    { fontSize: 20, fontFamily: 'serif' },
    { securityLevel: 'loose', secure: ['x'] },
    { sequence: { look: 'classic', mirrorActors: false }, state: { theme: 'forest' } },
    { themeVariables: { lineColor: '#ff0000' } },
    { theme: 'bogus', look: 'bogus' },
    { __proto: 1, title: '<b>x</b>', nested: { url: 'url(data:x)', ok: 1 } },
    {},
    // A theme variable that is not a color makes the theme code throw.
    { theme: 'forest', themeVariables: { primaryColor: 'nope' } },
  ];
  const types = ['flowchart-v2', 'sequence', 'classDiagram', 'stateDiagram', 'xychart', 'info', 'railroadPeg', null];
  const reads = ['getConfig', 'getSiteConfig', 'getUserDefinedConfig', 'getEffectiveHtmlLabels'];
  const scripts = [];
  for (let n = 0; n < 240; n++) {
    const file = pick(fixtures);
    const steps = [['initialize', pick(sites)]];
    if (rnd() < 0.85) steps.push(['process', path.relative(CORPUS, file)], ['getConfig']);
    const count = 3 + Math.floor(rnd() * 6);
    for (let i = 0; i < count; i++) {
      const k = Math.floor(rnd() * 10);
      if (k < 4) steps.push(['setConfig', pick(objects)]);
      else if (k < 6) steps.push(['updateSiteConfig', pick(objects)]);
      else if (k < 7) steps.push(['addDirective', pick(objects)]);
      else if (k < 8) steps.push(['setDiagramConfigScope', pick(types)]);
      else if (k < 9) steps.push(['reset']);
      else steps.push(['setSiteConfig', pick(objects)]);
      steps.push([pick(reads)]);
    }
    steps.push(['getConfig'], ['getSiteConfig'], ['getUserDefinedConfig']);
    scripts.push(steps);
  }
  const results = scripts.map((steps) => {
    api.reset();
    const out = [];
    for (const [op, arg] of steps) {
      try {
        const input = arg !== null && typeof arg === 'object' ? structuredClone(arg) : arg;
        switch (op) {
          case 'initialize':
            mermaid.initialize(input);
            out.push(null);
            break;
          case 'process':
            processAndSetConfigs(fs.readFileSync(path.join(CORPUS, arg), 'utf8'));
            out.push(null);
            break;
          case 'setConfig':
            out.push({ config: pool.object(encode(api.setConfig(input))) });
            break;
          case 'updateSiteConfig':
            out.push({ config: pool.object(encode(api.updateSiteConfig(input))) });
            break;
          case 'setSiteConfig':
            out.push({ config: pool.object(encode(api.setSiteConfig(input))) });
            break;
          case 'addDirective':
            api.addDirective(input);
            out.push({ value: pool.add(encode(input)) });
            break;
          case 'setDiagramConfigScope':
            api.setDiagramConfigScope(arg === null ? undefined : arg);
            out.push(null);
            break;
          case 'reset':
            api.reset();
            out.push(null);
            break;
          case 'getConfig':
            out.push({ config: pool.object(encode(api.getConfig())) });
            break;
          case 'getSiteConfig':
            out.push({ config: pool.object(encode(api.getSiteConfig())) });
            break;
          case 'getUserDefinedConfig':
            out.push({ value: pool.add(encode(api.getUserDefinedConfig())) });
            break;
          case 'getEffectiveHtmlLabels':
            out.push({ value: pool.add(api.getEffectiveHtmlLabels(api.getConfig())) });
            break;
          default:
            throw new Error(`unknown step ${op}`);
        }
      } catch (e) {
        out.push({ error: errorOf(e) });
      }
    }
    try {
      api.setDiagramConfigScope(undefined);
    } catch {
      // A directive that breaks the theme is dropped by the next reset().
    }
    return out;
  });
  // evaluate() is a pure function.
  const values = [true, false, 'true', 'false', ' False ', 'null', 'NULL', '0', 0, 1, '', 'x', null, undefined, [], [0], ['false'], {}];
  const evaluate = values.map((v) => [encode(v), api.evaluate(v)]);
  writeGz('api.json.gz', { pool: pool.values, scripts: scripts.map((steps) => steps.map(([op, arg]) => [op, encode(arg)])), results, evaluate });
  const steps = results.flat();
  process.stderr.write(`api: ${scripts.length} scripts, ${steps.length} steps, ${steps.filter((r) => r?.error).length} throw\n`);
};

const which = process.argv[2] ?? 'all';
for (const [name, fn] of Object.entries(commands)) {
  if (which === 'all' || which === name) await fn();
}
