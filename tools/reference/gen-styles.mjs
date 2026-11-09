// Emits golden data for internal/styles from the built mermaid 12.2.0.
//
// For each fixture of the corpus, the script does what mermaidAPI.render does
// up to the <style> element: processAndSetConfigs(text), getConfig(), load the
// diagram, then createUserStyles(config, type, undefined, '#my-svg'). It
// stores the effective config, the text that getStyles returns and the text
// that compileCSS returns. It adds cases that the corpus does not have: every
// diagram type with every theme and look, configs that reach the rare
// branches, and direct calls of getStyles with odd options.
//
// It also writes the theme variables of the default theme, which five style
// templates merge into the active theme variables.
//
// Node has no CSSOM, so styles-lib.mjs supplies a small CSSStyleSheet. See the
// note there for what that means for the font rules.
//
// Source of the libraries: mermaid.core.mjs imports stylis and es-toolkit from
// node_modules, which has newer versions than the mermaid bundle that the
// reference renders with. use-bundled.mjs (imported by styles-lib.mjs) sends
// those imports to the bundled versions, stylis 4.3.6 and es-toolkit 1.45.1,
// whose source comes from the source map of the bundle. The merge of
// es-toolkit 1.45.1 reads a null in an array of the source as undefined, and
// 1.53.1 does not. The merge cases with "newer" are those where the two
// versions differ.
//
// Usage (in tools/reference): node gen-styles.mjs
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { pathToFileURL } from 'node:url';
import { bundledVersion } from './use-bundled.mjs';
import { HERE, REPO, encode, loadDiagram, loadMermaid, mulberry32 } from './styles-lib.mjs';

// This is a dynamic import, because use-bundled.mjs must run before node
// resolves "es-toolkit/compat".
const { merge } = await import('es-toolkit/compat');
// The merge of the es-toolkit in node_modules. The import by path does not go
// through the hook.
const newerEsToolkit = path.join(HERE, 'node_modules/es-toolkit');
const { merge: newerMerge } = await import(pathToFileURL(path.join(newerEsToolkit, 'dist/compat/object/merge.mjs')).href);
const newerVersion = JSON.parse(fs.readFileSync(path.join(newerEsToolkit, 'package.json'), 'utf8')).version;
// es-toolkit 1.45.1 gives {"a":[1,3]} here, and 1.53.1 gives {"a":[null,3]}.
if (bundledVersion('es-toolkit') !== '1.45.1' || JSON.stringify(merge({}, { a: [1, 2] }, { a: [null, 3] })) !== '{"a":[1,3]}') {
  throw new Error('gen-styles.mjs: es-toolkit is not the version of the mermaid bundle (1.45.1)');
}

const SVG_ID = '#my-svg';
const CORPUS = path.join(REPO, 'testdata/corpus/mermaid-e2e');
const OUT_DIR = path.join(REPO, 'internal/styles');

const api = await loadMermaid();

// --- value tables ----------------------------------------------------------
// An effective config is about 50 kB of JSON and most of it is the same in
// every case. The file stores each distinct top-level value one time.
const values = [];
const valueIndex = new Map();
function valueId(v) {
  const json = JSON.stringify(v);
  let id = valueIndex.get(json);
  if (id === undefined) {
    id = values.length;
    values.push(v);
    valueIndex.set(json, id);
  }
  return id;
}
const configs = [];
const configIndex = new Map();
function configId(config) {
  const enc = encode(config);
  const entry = Object.keys(enc).map((k) => [k, valueId(enc[k])]);
  const key = JSON.stringify(entry);
  let id = configIndex.get(key);
  if (id === undefined) {
    id = configs.length;
    configs.push(entry);
    configIndex.set(key, id);
  }
  return id;
}

const sha = (s) => crypto.createHash('sha256').update(s, 'utf8').digest('hex');

// --- one render ------------------------------------------------------------
// styleCase does the config and style steps of renderDiagram for a text.
const outputs = [];
const outputIndex = new Map();
async function styleCase(text, forceType) {
  const processed = api.processAndSetConfigs(text);
  const config = api.getConfig();
  let type = forceType;
  if (type === undefined) {
    try {
      type = api.detectType(processed.code.cleaned, config);
    } catch (e) {
      return { error: String(e.message).split('\n')[0] };
    }
  }
  const cfg = configId(config);
  const key = `${type}\u0001${cfg}`;
  let id = outputIndex.get(key);
  if (id === undefined) {
    const rec = { type, config: cfg };
    try {
      // Diagram.fromText loads the diagram and calls its init with the config.
      // A forced type can be one that mermaid has no diagram for.
      if (forceType === undefined) {
        const diagram = await loadDiagram(api, type);
        diagram.init?.(config);
      }
      const userStyles = api.createCssStyles(config, undefined);
      rec.user = userStyles;
      rec.raw = api.styles_default(type, userStyles, { ...config.themeVariables, theme: config.theme, look: config.look }, SVG_ID);
      const css = api.createUserStyles(config, type, undefined, SVG_ID);
      rec.css = css;
    } catch (e) {
      delete rec.raw;
      rec.error = String(e && e.message).split('\n')[0];
    }
    id = outputs.length;
    outputs.push(rec);
    outputIndex.set(key, id);
  }
  return { output: id };
}

// --- corpus ----------------------------------------------------------------
function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(p, out);
    else if (entry.name.endsWith('.mmd')) out.push(p);
  }
  return out;
}
const fixtures = [];
for (const file of walk(CORPUS)) {
  const rel = path.relative(CORPUS, file).replace(/\.mmd$/, '');
  const res = await styleCase(fs.readFileSync(file, 'utf8'));
  fixtures.push(res.error !== undefined ? { path: rel, error: res.error } : { path: rel, output: res.output });
}

// --- cases outside the corpus ----------------------------------------------
// A keyword that detectType maps to each diagram type.
const KEYWORDS = {
  'flowchart-v2': 'flowchart TD\n A-->B',
  'flowchart-elk': 'flowchart-elk TD\n A-->B',
  mindmap: 'mindmap\n root',
  architecture: 'architecture-beta\n service a(server)[A]',
  agentflow: 'agentflow-beta\n',
  c4: 'C4Context\n',
  kanban: 'kanban\n a',
  classDiagram: 'classDiagram\n class A',
  er: 'erDiagram\n A',
  gantt: 'gantt\n',
  info: 'info',
  pie: 'pie\n "a": 1',
  requirement: 'requirementDiagram\n',
  sequence: 'sequenceDiagram\n A->>B: x',
  swimlane: 'swimlane-beta\n',
  timeline: 'timeline\n',
  gitGraph: 'gitGraph\n commit',
  stateDiagram: 'stateDiagram-v2\n a',
  journey: 'journey\n',
  quadrantChart: 'quadrantChart\n',
  sankey: 'sankey-beta\n',
  packet: 'packet-beta\n',
  xychart: 'xychart-beta\n',
  block: 'block-beta\n a',
  eventmodeling: 'eventmodeling\n',
  treeView: 'treeView-beta\n',
  radar: 'radar-beta\n',
  ishikawa: 'ishikawa-beta\n',
  treemap: 'treemap-beta\n',
  railroad: 'railroad-beta\n',
  railroadEbnf: 'railroad-ebnf-beta\n',
  railroadAbnf: 'railroad-abnf-beta\n',
  railroadPeg: 'railroad-peg-beta\n',
  venn: 'venn-beta\n',
  wardley: 'wardley-beta\n',
  cynefin: 'cynefin-beta\n',
  usecase: 'usecase-beta\n',
};
const TYPES = Object.keys(KEYWORDS);
const THEMES = Object.keys(api.themes_default);
const LOOKS = ['classic', 'neo', 'handDrawn'];
const frontmatter = (config, body) => `---\nconfig: ${JSON.stringify(config)}\n---\n${body}`;

// Check that each keyword gives its type, so that the cases mean what they say.
for (const type of TYPES) {
  api.processAndSetConfigs(KEYWORDS[type]);
  const detected = api.detectType(KEYWORDS[type], api.getConfig());
  if (detected !== type) throw new Error(`keyword of ${type} is detected as ${detected}`);
}

// A config that makes the theme calculation itself throw never gets to the
// style step, so such a case is left out.
const extra = [];
const skipped = [];
async function addExtra(name, config, types) {
  for (const type of types) {
    let res;
    try {
      res = await styleCase(frontmatter(config, KEYWORDS[type]));
    } catch (e) {
      skipped.push(`${name}/${type}: ${String(e && e.message).split('\n')[0]}`);
      continue;
    }
    extra.push({ name: `${name}/${type}`, output: res.output });
  }
}

// Every type with every theme and look.
for (const theme of THEMES) {
  for (const look of LOOKS) {
    await addExtra(`matrix/${theme}/${look}`, { theme, look }, TYPES);
  }
}
// Types that mermaid has no style function for. '---' has an object where
// the function must be, so getStyles throws.
for (const type of ['error', 'zenuml', 'no-such-type', '---']) {
  const res = await styleCase('flowchart TD\n A-->B', type);
  extra.push({ name: `type/${type}`, output: res.output });
}

const PALETTE_TYPES = ['flowchart-v2', 'swimlane', 'block', 'agentflow', 'classDiagram', 'er', 'requirement', 'stateDiagram', 'usecase', 'timeline', 'gitGraph', 'mindmap', 'kanban', 'radar', 'sequence'];
const VARIANTS = [
  // Loop limits and palettes.
  ['limit-0', { themeVariables: { THEME_COLOR_LIMIT: 0 } }, PALETTE_TYPES],
  ['limit-3', { themeVariables: { THEME_COLOR_LIMIT: 3 } }, PALETTE_TYPES],
  ['limit-20', { themeVariables: { THEME_COLOR_LIMIT: 20 } }, PALETTE_TYPES],
  ['limit-2.5', { themeVariables: { THEME_COLOR_LIMIT: 2.5 } }, PALETTE_TYPES],
  ['limit-string', { themeVariables: { THEME_COLOR_LIMIT: '5' } }, PALETTE_TYPES],
  ['limit-65', { themeVariables: { THEME_COLOR_LIMIT: 65 } }, PALETTE_TYPES],
  ['limit-negative', { themeVariables: { THEME_COLOR_LIMIT: -1 } }, PALETTE_TYPES],
  ['limit-word', { themeVariables: { THEME_COLOR_LIMIT: 'many' } }, PALETTE_TYPES],
  ['color-limit-3', { theme: 'redux-color', themeVariables: { THEME_COLOR_LIMIT: 3 } }, PALETTE_TYPES],
  ['color-limit-20', { theme: 'redux-dark-color', look: 'neo', themeVariables: { THEME_COLOR_LIMIT: 20 } }, PALETTE_TYPES],
  ['color-palette-short', { theme: 'redux-color', themeVariables: { borderColorArray: ['#111111', '#222222', '#333333'], bkgColorArray: ['#aaaaaa', '#bbbbbb'] } }, PALETTE_TYPES],
  ['color-palette-long', { theme: 'redux-color', look: 'handDrawn', themeVariables: { borderColorArray: Array.from({ length: 15 }, (_, i) => `#${String(i).padStart(2, '0')}0000`), bkgColorArray: ['#aaaaaa'] } }, PALETTE_TYPES],
  ['color-palette-empty', { theme: 'redux-color', themeVariables: { borderColorArray: [] } }, PALETTE_TYPES],
  ['color-palette-string', { theme: 'redux-dark-color', themeVariables: { borderColorArray: 'red', bkgColorArray: 'blue' } }, PALETTE_TYPES],
  ['color-bkg-empty', { theme: 'redux-color', themeVariables: { bkgColorArray: [] } }, PALETTE_TYPES],
  ['palette-on-default-theme', { theme: 'default', themeVariables: { borderColorArray: ['#111111', '#222222'], bkgColorArray: ['#aaaaaa'] } }, PALETTE_TYPES],
  // Looks and themes that are not in the lists.
  ['look-unknown', { look: 'sketchy' }, PALETTE_TYPES],
  ['look-number', { look: 7 }, ['flowchart-v2', 'er', 'usecase', 'sequence', 'requirement']],
  ['look-injection', { theme: 'redux-color', look: 'neo"] { fill: red } [x="' }, ['flowchart-v2', 'er', 'usecase', 'stateDiagram']],
  ['theme-null', { theme: 'null' }, TYPES],
  ['theme-unknown', { theme: 'pink' }, ['flowchart-v2', 'gitGraph', 'timeline', 'mindmap']],
  ['theme-diagram-scoped', { flowchart: { theme: 'forest', look: 'handDrawn' }, sequence: { theme: 'dark' }, gitGraph: { theme: 'redux-color' } }, ['flowchart-v2', 'sequence', 'gitGraph', 'er']],
  // Neo features.
  ['gradient', { look: 'neo', themeVariables: { useGradient: true } }, TYPES],
  ['gradient-redux', { theme: 'redux', look: 'neo', themeVariables: { useGradient: true, THEME_COLOR_LIMIT: 4 } }, ['mindmap', 'timeline', 'gitGraph', 'stateDiagram', 'flowchart-v2']],
  ['gradient-neutral', { theme: 'neutral', look: 'neo', themeVariables: { useGradient: true } }, ['mindmap', 'timeline', 'gitGraph']],
  ['drop-shadow', { look: 'neo', themeVariables: { dropShadow: 'drop-shadow(1px 2px 3px red) url(#drop-shadow) url(#drop-shadow)' } }, TYPES],
  ['drop-shadow-empty', { look: 'neo', themeVariables: { dropShadow: '' } }, ['flowchart-v2', 'stateDiagram', 'mindmap', 'timeline', 'sequence', 'gitGraph']],
  ['drop-shadow-number', { themeVariables: { dropShadow: 5 } }, ['flowchart-v2', 'stateDiagram', 'mindmap', 'timeline', 'sequence', 'gitGraph']],
  ['stroke-width', { themeVariables: { strokeWidth: 3 } }, TYPES],
  ['stroke-width-zero', { look: 'neo', themeVariables: { strokeWidth: 0 } }, TYPES],
  ['stroke-width-string', { themeVariables: { strokeWidth: '2.5' } }, ['flowchart-v2', 'stateDiagram', 'classDiagram', 'er', 'requirement', 'gitGraph', 'block', 'agentflow']],
  // Theme variable overrides, including values of the wrong type.
  ['base-colors', { theme: 'base', themeVariables: { primaryColor: '#ff0000', lineColor: 'rgb(1, 2, 3)', secondaryColor: 'hsl(120, 50%, 50%)', tertiaryColor: '#00f' } }, TYPES],
  ['dark-mode', { theme: 'base', themeVariables: { darkMode: true, primaryColor: '#123456' } }, TYPES],
  ['font', { fontFamily: 'courier', fontSize: 20 }, TYPES],
  ['font-quoted', { fontFamily: '"Comic Sans MS", \'Zapf Chancery\', cursive ; ', themeVariables: { fontSize: '13.5px' } }, ['flowchart-v2', 'railroad', 'venn', 'pie', 'usecase', 'ishikawa']],
  ['font-odd', { fontFamily: 'a{b}c', altFontFamily: ' Times ' }, ['flowchart-v2', 'railroad']],
  ['font-unicode', { fontFamily: '微软雅黑, "ヒラギノ", 😀' }, ['flowchart-v2', 'railroad', 'usecase']],
  ['font-null', { fontFamily: null }, ['flowchart-v2']],
  ['numbers', { themeVariables: { textColor: 12, lineColor: 0, mainBkg: false, nodeBorder: null, titleColor: 1.5e21, fontSize: 14 } }, ['flowchart-v2', 'classDiagram', 'stateDiagram', 'gantt', 'pie', 'journey', 'sankey', 'requirement', 'sequence', 'usecase', 'ishikawa', 'venn', 'architecture']],
  ['objects', { themeVariables: { textColor: ['a', 'b'], lineColor: { a: 1 }, titleColor: [] } }, ['classDiagram', 'stateDiagram', 'gantt', 'pie', 'journey', 'sankey', 'sequence', 'ishikawa', 'venn', 'architecture']],
  ['state-fallbacks', { themeVariables: { transitionLabelColor: '', stateBkg: '', stateBorder: '', altBackground: '', compositeBackground: '', strokeWidth: '' } }, ['stateDiagram']],
  ['falsy-colors', { themeVariables: { nodeTextColor: '', titleColor: '', mainBkg: '', faceColor: '', fillType0: '', actor0: '', commitLineColor: '#123' } }, ['flowchart-v2', 'block', 'agentflow', 'journey', 'sankey', 'gantt', 'gitGraph', 'er', 'requirement', 'classDiagram']],
  ['journey-colors', { themeVariables: { faceColor: '#abc', fillType0: '#000', fillType3: '#333', actor0: '#111', actor5: '#555' } }, ['journey']],
  ['bad-color-edge-label', { themeVariables: { edgeLabelBackground: 'not-a-color' } }, ['flowchart-v2', 'agentflow', 'swimlane', 'block', 'er']],
  ['bad-color-cluster', { themeVariables: { clusterBkg: 'nope', clusterBorder: '#12' } }, ['block', 'flowchart-v2']],
  ['bad-color-tertiary', { themeVariables: { tertiaryColor: '' } }, ['er', 'flowchart-v2']],
  ['er-label', { theme: 'redux-color', themeVariables: { erEdgeLabelBackground: '#fafafa' } }, ['er']],
  ['er-label-default-theme', { themeVariables: { erEdgeLabelBackground: '#fafafa' } }, ['er']],
  ['requirement-label', { themeVariables: { requirementEdgeLabelBackground: '#fafafa' } }, ['requirement']],
  ['line-colors', { themeVariables: { lineColor0: '#ffffff', lineColor1: '#000000', lineColor2: 'not-a-color', cScaleInv3: '' } }, ['mindmap', 'kanban', 'timeline']],
  ['line-colors-ok', { themeVariables: { lineColor0: '#ffffff', lineColor1: '#000000', lineColor2: 'hsl(10, 20%, 30%)' } }, ['mindmap', 'kanban', 'timeline']],
  ['line-colors-number', { themeVariables: { lineColor0: 5 } }, ['mindmap', 'kanban', 'timeline']],
  ['kanban-dark', { theme: 'dark' }, ['kanban']],
  ['cscale-missing', { theme: 'base', themeVariables: { THEME_COLOR_LIMIT: 14 } }, ['mindmap', 'kanban', 'timeline', 'radar']],
  // Per-diagram option objects in themeVariables.
  ['treemap-options', { themeVariables: { treemap: { sectionStrokeColor: 'red', leafFillColor: 'blue', titleColor: 'green', labelColor: null, valueFontSize: 9 } } }, ['treemap']],
  ['treemap-options-odd', { themeVariables: { treemap: 'x', titleColor: null, textColor: 3 } }, ['treemap']],
  ['packet-options', { themeVariables: { packet: { byteFontSize: '11px', blockStrokeWidth: 2, extra: 'x', labelColor: null } } }, ['packet']],
  ['packet-options-array', { themeVariables: { packet: ['a', 'b'] } }, ['packet']],
  ['treeview-options', { themeVariables: { treeView: { labelFontSize: '12px', lineColor: 'red', highlightBg: null } } }, ['treeView']],
  ['radar-options', { themeVariables: { radar: { axisColor: 'red', curveOpacity: 0.3, legendFontSize: 9 }, cScale0: 'pink', THEME_COLOR_LIMIT: 3 } }, ['radar']],
  ['radar-options-odd', { theme: 'forest', themeVariables: { radar: { axisColor: { a: 1 }, graticuleColor: ['x'] }, THEME_COLOR_LIMIT: '2' } }, ['radar']],
  ['wardley-options', { themeVariables: { wardley: { axisColor: 'red', componentFill: null, linkStroke: 5 } } }, ['wardley']],
  ['wardley-themes', { theme: 'dark', themeVariables: { wardley: { backgroundColor: '#000' } } }, ['wardley']],
  ['cynefin-options', { themeVariables: { cynefin: { domainFontSize: 20, itemFontSize: '11', boundaryWidth: 3, textColor: 'red' } } }, ['cynefin']],
  ['cynefin-options-odd', { themeVariables: { cynefin: { domainFontSize: 'big', itemFontSize: null, boundaryColor: ['a', 'b'], arrowWidth: false } } }, ['cynefin']],
  ['cynefin-options-array', { themeVariables: { cynefin: { domainFontSize: [1, 2], itemFontSize: [], labelColor: { x: 1 } } } }, ['cynefin']],
  // Config sections that style functions read.
  ['railroad-config', { railroad: { fontSize: 18, fontFamily: 'Fira Code, monospace', terminalFill: '#abc', lineColor: 'rgb(1, 2, 3)', strokeWidth: '3', padding: 4 } }, ['railroad', 'railroadEbnf', 'railroadAbnf', 'railroadPeg']],
  ['railroad-config-invalid', { railroad: { fontSize: -1, fontFamily: 'a;b', terminalFill: 'url(x)', lineColor: 'rgb(1; 2)', strokeWidth: 'x', commentFill: '#12', specialStroke: '  #ABCDEF12  ', ruleNameColor: 7, markerFill: 'Red' } }, ['railroad']],
  ['railroad-theme-vars', { themeVariables: { railroad: { terminalFill: '#010203', fontSize: '11' }, fontFamily: 'a;b', fontSize: '0px', secondBkg: null, lineColor: 'not a color', titleColor: 'hsl(1 2% 3% / 0.5)' } }, ['railroad']],
  ['railroad-themes', { theme: 'dark', themeVariables: { fontSize: '12.5px' } }, ['railroad']],
  ['usecase-rotate', { theme: 'redux-color', usecase: { colorScheme: 'rotate' } }, ['usecase']],
  ['usecase-rotate-handdrawn', { theme: 'redux-dark-color', look: 'handDrawn', usecase: { colorScheme: 'rotate' } }, ['usecase']],
  ['usecase-rotate-default-theme', { usecase: { colorScheme: 'rotate' } }, ['usecase']],
  ['usecase-tokens', { themeVariables: { usecaseActorBkg: '#111', usecaseActorBorder: '#222', usecaseBkg: '#333', usecaseBorder: '#444', usecaseBoundaryBkg: '#555', usecaseBoundaryBorder: '#666', usecaseIncludeLine: '#777', usecaseExtendLine: '#888', actorTextColor: '#999' } }, ['usecase']],
  ['usecase-nulls', { themeVariables: { actorBkg: null, actorBorder: null, nodeBorder: null, titleColor: null, actorTextColor: null } }, ['usecase']],
  ['c4-fonts', { c4: { personFontFamily: 'Arial', personFontSize: '12px', personFontWeight: 'bold', systemFontSize: 0, systemFontFamily: '', systemFontWeight: '', external_systemFontSize: 16, containerFontWeight: 700 } }, ['c4']],
  ['c4-null', { c4: { personFontFamily: null, personFontSize: null, personFontWeight: null } }, ['c4']],
  ['sequence-neo', { look: 'neo', sequence: { wrap: true } }, ['sequence']],
  ['git-themes', { theme: 'neo', themeVariables: { THEME_COLOR_LIMIT: 10, noteFontWeight: 600 } }, ['gitGraph']],
  ['git-color-no-palette', { theme: 'redux-color', themeVariables: { borderColorArray: null } }, ['gitGraph']],
  ['git-color-palette-empty', { theme: 'redux-dark-color', themeVariables: { borderColorArray: [] } }, ['gitGraph']],
  ['theme-css', { themeCSS: '.a { fill: #ff0000 }\n.b > .c { stroke: blue; }' }, ['flowchart-v2', 'sequence']],
  ['security-loose', { securityLevel: 'loose', htmlLabels: false }, ['flowchart-v2']],
];
for (const [name, config, types] of VARIANTS) {
  await addExtra(`variant/${name}`, config, types);
}

// Direct calls of getStyles. They skip the config sanitizer, so the options
// can be anything.
const direct = [];
const DIRECT_OPTIONS = [
  ['empty', {}],
  ['theme-only', { theme: 'redux-color', look: 'neo' }],
  ['theme-number', { theme: 5, look: null, THEME_COLOR_LIMIT: 2 }],
  ['theme-array', { theme: ['redux'], look: ['neo'], THEME_COLOR_LIMIT: 2 }],
  ['theme-object', { theme: { a: 1 }, THEME_COLOR_LIMIT: 2 }],
  ['nulls', { fontFamily: null, fontSize: null, textColor: null, lineColor: null, strokeWidth: null, nodeBorder: null, mainBkg: null, THEME_COLOR_LIMIT: 2, edgeLabelBackground: '#fff', tertiaryColor: '#fff', clusterBkg: '#fff', clusterBorder: '#fff' }],
  ['gradient', { useGradient: 1, dropShadow: 'url(#drop-shadow)', nodeBorder: 'red', THEME_COLOR_LIMIT: 2, mainBkg: '#fff', edgeLabelBackground: '#fff', tertiaryColor: '#fff', clusterBkg: '#fff', clusterBorder: '#fff' }],
  ['drop-shadow-true', { dropShadow: true, edgeLabelBackground: '#fff' }],
  ['drop-shadow-array', { dropShadow: ['url(#drop-shadow)'], edgeLabelBackground: '#fff' }],
  ['limit-infinity', { THEME_COLOR_LIMIT: 1e9, edgeLabelBackground: '#fff', tertiaryColor: '#fff', clusterBkg: '#fff', clusterBorder: '#fff' }],
  ['palette', { theme: 'redux-color', look: 'handDrawn', borderColorArray: ['a', 5, null, undefined, ['x', 'y'], { k: 1 }], bkgColorArray: [true, 1.5], edgeLabelBackground: '#fff', tertiaryColor: '#fff', clusterBkg: '#fff', clusterBorder: '#fff', THEME_COLOR_LIMIT: 3 }],
  ['railroad-shape', { railroad: { terminalFill: '#123456' }, fontFamily: 'x y', fontSize: 9 }],
  ['railroad-shape-null', { railroad: null, lineColor: '#123' }],
  ['colors', { edgeLabelBackground: 'rgba(1, 2, 3, 0.4)', tertiaryColor: 'hsla(10, 20%, 30%, 0.5)', clusterBkg: 'transparent', clusterBorder: '#abcd', mainBkg: 'red', THEME_COLOR_LIMIT: 3, cScale0: '#111', cScale1: '#eee', cScale2: 'hsl(1, 2%, 3%)', cScaleInv0: '#fff', cScaleInv1: '#000', cScaleInv2: 'rgb(9, 9, 9)', lineColor0: '#777' }],
  ['dark-mode', { darkMode: true, THEME_COLOR_LIMIT: 2, cScale0: '#111', cScale1: '#eee', cScaleInv0: '#fff', cScaleInv1: '#000' }],
];
const DIRECT_SVG_IDS = [SVG_ID, '#a$&b', "#x$'y", '', 'no-hash'];
api.processAndSetConfigs('flowchart TD\n A-->B');
const directConfig = api.getConfig();
const directConfigId = configId(directConfig);
for (const type of [...TYPES, 'error', '---', 'no-such-type']) {
  if (type !== 'no-such-type') await loadDiagram(api, type);
  for (let i = 0; i < DIRECT_OPTIONS.length; i++) {
    const [name, options] = DIRECT_OPTIONS[i];
    const svgId = DIRECT_SVG_IDS[i % DIRECT_SVG_IDS.length];
    const userStyles = i % 3 === 0 ? '' : '.user { fill: red }';
    const rec = { name: `${name}/${type}`, type, config: directConfigId, options: encode(options), svgId, user: userStyles };
    if (name === 'limit-infinity' && type === 'gitGraph') {
      // This loop runs on THEME_COLOR_LIMIT with no bound. The port returns an
      // error for a limit above its maximum. Mermaid would make a very large
      // string here, so the script does not run it.
      rec.error = 'not run: unbounded loop';
    } else {
      try {
        // The options object is copied first: kanban, mindmap and timeline
        // write to the copy that getStyles makes, never to this object.
        rec.raw = api.styles_default(type, userStyles, structuredClone(options), svgId);
        rec.css = api.compileCSS(svgId, rec.raw);
      } catch (e) {
        delete rec.raw;
        rec.error = String(e && e.message).split('\n')[0];
      }
    }
    direct.push(rec);
  }
}

// --- cleanAndMerge ---------------------------------------------------------
// cleanAndMerge(defaultData, data) is merge({}, defaultData, data) of
// es-toolkit. The cases are random value trees with keys that collide.
const rnd = mulberry32(0x6d657267);
const pick = (list) => list[Math.floor(rnd() * list.length)];
const MERGE_KEYS = ['a', 'b', 'c', 'titleColor', 'x', '0', '1', '2', '10', '__proto__'];
function randomValue(depth) {
  const r = rnd();
  if (depth > 0 && r < 0.3) {
    const o = {};
    const n = Math.floor(rnd() * 5);
    for (let i = 0; i < n; i++) {
      Object.defineProperty(o, pick(MERGE_KEYS), { value: randomValue(depth - 1), enumerable: true, writable: true, configurable: true });
    }
    return o;
  }
  if (depth > 0 && r < 0.45) {
    return Array.from({ length: Math.floor(rnd() * 4) }, () => randomValue(depth - 1));
  }
  return pick([null, undefined, true, false, 0, 1, -2.5, 1e21, '', 'red', '#fff', '12px']);
}
const mergeCases = [];
let mergeDiffer = 0;
function addMerge(a, b) {
  const rec = { a: encode(a), b: encode(b), out: encode(merge({}, a, b)) };
  const newer = encode(newerMerge({}, a, b));
  if (JSON.stringify(newer) !== JSON.stringify(rec.out)) {
    rec.newer = newer;
    mergeDiffer++;
  }
  mergeCases.push(rec);
}
for (let i = 0; i < 600; i++) {
  const a = i % 7 === 0 ? randomValue(0) : randomValue(3);
  const b = i % 11 === 0 ? randomValue(0) : randomValue(3);
  addMerge(a, b);
}
// Cases where es-toolkit 1.45.1 (the mermaid bundle) and 1.53.1 (node_modules)
// differ: 1.45.1 reads a null element of a source array as undefined, so the
// element does not replace a value of the target.
const MERGE_VERSION = [
  [{ a: [1, 2] }, { a: [null, 3] }],
  [{ a: [1, 2] }, { a: [null] }],
  [{}, { a: [null] }],
  [{ a: [null] }, {}],
  [{ a: [null, 1] }, undefined],
  [{ a: [1, 2, 3] }, { a: [null, null] }],
  [{ a: [1] }, { a: [null, null, 4] }],
  [{ a: { b: [1, { c: 2 }] } }, { a: { b: [null, null] } }],
  [{ a: [[1], [2]] }, { a: [[null], null] }],
  [{ a: [{ b: 1 }, 'x'] }, { a: [null, null] }],
  [{ a: 'x' }, { a: [null, 'y'] }],
  [{ a: null }, { a: [null] }],
  [{ a: { 0: 'x', 1: 'y' } }, { a: [null, null] }],
  [{ a: { length: 2, 0: 'x', 1: 'y' } }, { a: [null, 'z'] }],
  [{ a: [null, null] }, { a: [null, null] }],
  [{ a: [undefined, 1] }, { a: [null, null] }],
  [{ a: [1, 2] }, { a: [undefined, null] }],
  [[1, 2], [null, 3]],
  [[null], [null]],
  [{ a: [1, [2, [3]]] }, { a: [null, [null, [null]]] }],
  [{ titleColor: ['red', 'blue'] }, { titleColor: [null] }],
  // A null that is not in an array is the same in both versions.
  [{ a: 1 }, { a: null }],
  [{ a: [1] }, { a: null }],
  [{ a: { b: 1 } }, { a: { b: null } }],
];
for (const [a, b] of MERGE_VERSION) {
  addMerge(a, b);
}
// Random trees where arrays and null are frequent.
function randomArrayValue(depth) {
  const r = rnd();
  if (depth > 0 && r < 0.3) {
    const o = {};
    const n = 1 + Math.floor(rnd() * 3);
    for (let i = 0; i < n; i++) {
      Object.defineProperty(o, pick(['a', 'b', '0', '1']), { value: randomArrayValue(depth - 1), enumerable: true, writable: true, configurable: true });
    }
    return o;
  }
  if (depth > 0 && r < 0.75) {
    return Array.from({ length: Math.floor(rnd() * 4) }, () => randomArrayValue(depth - 1));
  }
  return pick([null, null, null, undefined, 0, 1, 'red', true]);
}
for (let i = 0; i < 600; i++) {
  addMerge(randomArrayValue(3), i % 13 === 0 ? randomArrayValue(0) : randomArrayValue(3));
}
if (bundledVersion('es-toolkit') !== newerVersion && mergeDiffer < 100) {
  throw new Error(`gen-styles.mjs: only ${mergeDiffer} merge cases tell es-toolkit ${bundledVersion('es-toolkit')} from ${newerVersion}`);
}

// --- write -----------------------------------------------------------------
// The compiled text is checked through its hash, which keeps the file small.
// internal/stylis has its own golden cases for the compile step.
function pack(rec) {
  const out = { ...rec };
  if (out.css !== undefined) {
    out.cssHash = sha(out.css);
    out.cssLength = out.css.length;
    delete out.css;
  }
  return out;
}

const themeDefault = encode(api.themes_default.default.getThemeVariables());
fs.writeFileSync(path.join(OUT_DIR, 'theme_default.json'), `${JSON.stringify(themeDefault)}\n`);

const golden = {
  mermaid: '12.2.0',
  // The versions of the libraries that made the expected values, and the
  // version of es-toolkit that made the "newer" values of the merge cases.
  stylis: bundledVersion('stylis'),
  esToolkit: bundledVersion('es-toolkit'),
  newerEsToolkit: newerVersion,
  svgId: SVG_ID,
  values,
  configs,
  outputs: outputs.map(pack),
  fixtures,
  extra,
  direct: direct.map(pack),
  merge: mergeCases,
};
const json = JSON.stringify(golden);
const file = path.join(OUT_DIR, 'testdata/styles.json.gz');
fs.mkdirSync(path.dirname(file), { recursive: true });
fs.writeFileSync(file, zlib.gzipSync(json, { level: 9 }));

for (const line of skipped) console.error(`skipped ${line}`);
const errors = (list) => list.filter((r) => r.error !== undefined).length;
console.error(
  `wrote ${file}: ${fixtures.length} fixtures (${errors(fixtures)} with no type), ${extra.length} extra, ` +
    `${outputs.length} distinct outputs (${errors(outputs)} throw), ${direct.length} direct (${errors(direct)} throw), ` +
    `${mergeCases.length} merge cases (${mergeDiffer} differ from es-toolkit ${newerVersion}), ${configs.length} configs, ${values.length} values, ` +
    `${json.length} bytes of JSON, ${fs.statSync(file).size} bytes on disk`
);
