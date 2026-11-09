// Generates the goldens of internal/purify from DOMPurify 3.4.12 as mermaid
// 12.2.0 bundles it, running in Chromium (oracle.mjs eval, one browser run).
//
//   internal/purify/clobber_names.go          names for the clobbering check
//   internal/purify/testdata/svg_clean.json.gz      reference SVGs, final-SVG config
//   internal/purify/testdata/final_hostile.json.gz  hostile inputs, final-SVG config
//   internal/purify/testdata/labels.json.gz         label texts, label configs
//   internal/purify/testdata/configs.json.gz        hostile inputs, config variants
//   internal/purify/testdata/hooks.json.gz          hook traces and hooks that change the result
//   internal/dom/testdata/purify_parse.json.gz      DOMParser trees of the inputs
//
// A result is 0 when the output equals the input, a string for another
// output, {"throw": message} when sanitize throws, {"node": toString, "html":
// markup} for RETURN_DOM, and {"value": text} for null.
//
// Usage: node gen-purify.mjs
//        PURIFY_REF_DIR=../../.parity/ref.old/mermaid-e2e node gen-purify.mjs
import fs from 'node:fs';
import path from 'node:path';
import { ROOT, mulberry32, runJobs, walk, writeGolden } from './purify-oracle-lib.mjs';
import { corpusLabels, generated, handWritten, svgMutations, unique } from './purify-hostile.mjs';

const OUT = path.join(ROOT, 'internal/purify');
const TESTDATA = path.join(OUT, 'testdata');
// The reference SVGs. PURIFY_REF_DIR selects another complete copy, for
// example .parity/ref.old/mermaid-e2e while .parity/ref is being regenerated.
// The goldens store the SVG text, so the tests do not read this directory.
const REF = process.env.PURIFY_REF_DIR
  ? path.resolve(process.env.PURIFY_REF_DIR)
  : path.join(ROOT, '.parity/ref/mermaid-e2e');

// The configuration of mermaidAPI.ts for the final SVG.
const FINAL = {
  ADD_TAGS: ['foreignobject'],
  ADD_ATTR: ['dominant-baseline'],
  HTML_INTEGRATION_POINTS: { foreignobject: true },
};

// ---- inputs ----

// sampleSvgs picks count reference SVGs: round robin over the diagram types,
// in a seeded order inside each type, so that every type is present.
function sampleSvgs(count, maxBytes) {
  const rnd = mulberry32(0x73766773);
  const byType = new Map();
  for (const file of walk(REF, '.svg')) {
    if (fs.statSync(file).size > maxBytes) {
      continue;
    }
    const type = path.relative(REF, file).split(path.sep)[0];
    if (!byType.has(type)) {
      byType.set(type, []);
    }
    byType.get(type).push(file);
  }
  for (const list of byType.values()) {
    for (let i = list.length - 1; i > 0; i--) {
      const j = Math.floor(rnd() * (i + 1));
      [list[i], list[j]] = [list[j], list[i]];
    }
  }
  const types = [...byType.keys()].sort();
  const out = [];
  for (let round = 0; out.length < count; round++) {
    let any = false;
    for (const type of types) {
      const list = byType.get(type);
      if (round < list.length && out.length < count) {
        out.push(list[round]);
        any = true;
      }
    }
    if (!any) {
      break;
    }
  }
  return { files: out.sort(), types };
}

const { files: svgFiles, types: svgTypes } = sampleSvgs(150, 60 * 1024);
const svgs = svgFiles.map((file) => ({
  name: path.relative(REF, file).split(path.sep).join('/'),
  in: fs.readFileSync(file, 'utf8'),
}));

const smallSvgs = svgs.filter((s) => s.in.length < 6000);
const mutated = [];
smallSvgs.slice(0, 40).forEach((s, i) => {
  mutated.push(...svgMutations(s.in, 10, 0x6d757400 + i));
});
const hostile = unique([...handWritten, ...generated(3200, 0x686f7374)]);
const finalInputs = unique([...hostile, ...mutated]);

const labels = unique([...corpusLabels(), ...handWritten, ...generated(1200, 0x6c61626c)]);
const labelConfigs = [null, { FORBID_TAGS: ['style'] }, { ADD_ATTR: ['target'] }];

// Config variants: every option of _parseConfig that JSON can express.
const variantInputs = unique([...handWritten, ...generated(250, 0x76617269)]);
const variants = [
  {},
  { ALLOWED_TAGS: ['b', 'i', 'a', 'p', '#text'] },
  { ALLOWED_TAGS: [] },
  { ALLOWED_TAGS: ['B', 'TABLE', 'tr', 'td'], ALLOWED_ATTR: ['HREF', 'title'] },
  { ALLOWED_ATTR: [] },
  { ALLOWED_ATTR: ['style', 'class', 'href', 'xlink:href', 'target'] },
  { FORBID_TAGS: ['b', 'svg', 'P'], FORBID_ATTR: ['style', 'CLASS', 'href'] },
  { FORBID_TAGS: ['style', 'tbody', 'form'] },
  { ADD_TAGS: ['foo', 'x-custom', 'script', 'iframe', 'noscript', 'noembed', 'noframes', 'xmp', 'use', 'FoO'] },
  { ADD_ATTR: ['foo', 'onclick', 'target', 'form', 'srcdoc', 'shadowrootmode', 'is'] },
  { ADD_TAGS: ['noscript', 'title', 'plaintext', 'textarea', 'object', 'embed'], ADD_ATTR: ['data', 'for'] },
  { USE_PROFILES: { html: true } },
  { USE_PROFILES: { svg: true } },
  { USE_PROFILES: { svg: true, svgFilters: true } },
  { USE_PROFILES: { mathMl: true } },
  { USE_PROFILES: { html: true, svg: true, mathMl: true } },
  { USE_PROFILES: {} },
  { USE_PROFILES: true },
  { USE_PROFILES: false },
  { USE_PROFILES: { html: 1, svg: 'true' } },
  { USE_PROFILES: { html: true }, ADD_TAGS: ['svg', 'rect'], ADD_ATTR: ['viewbox'] },
  { KEEP_CONTENT: false },
  { KEEP_CONTENT: false, ALLOWED_TAGS: ['b', 'p'] },
  { FORBID_CONTENTS: ['b', 'div'] },
  { ADD_FORBID_CONTENTS: ['b', 'foo', 'UNKNOWN'] },
  { FORBID_CONTENTS: [] },
  { SAFE_FOR_TEMPLATES: true },
  { SAFE_FOR_XML: false },
  { SANITIZE_DOM: false },
  { SANITIZE_NAMED_PROPS: true },
  { SANITIZE_NAMED_PROPS: true, SANITIZE_DOM: false },
  { ALLOW_DATA_ATTR: false },
  { ALLOW_ARIA_ATTR: false },
  { ALLOW_UNKNOWN_PROTOCOLS: true },
  { ALLOW_SELF_CLOSE_IN_ATTR: false },
  { ADD_URI_SAFE_ATTR: ['href', 'SRC'] },
  { ADD_DATA_URI_TAGS: ['a', 'SVG', 'use'] },
  { FORCE_BODY: true },
  { WHOLE_DOCUMENT: true },
  { WHOLE_DOCUMENT: true, ADD_TAGS: ['!doctype', 'title', 'meta'] },
  { WHOLE_DOCUMENT: true, FORCE_BODY: true },
  { RETURN_DOM: true },
  { RETURN_DOM_FRAGMENT: true },
  { RETURN_DOM: true, SAFE_FOR_TEMPLATES: true },
  { RETURN_DOM: true, WHOLE_DOCUMENT: true },
  { RETURN_TRUSTED_TYPE: true },
  { IN_PLACE: true },
  { HTML_INTEGRATION_POINTS: {} },
  { HTML_INTEGRATION_POINTS: { foreignobject: true, desc: true, title: 1, g: 0 } },
  { MATHML_TEXT_INTEGRATION_POINTS: { mi: true } },
  { MATHML_TEXT_INTEGRATION_POINTS: {} },
  { ALLOWED_NAMESPACES: ['http://www.w3.org/1999/xhtml'] },
  { ALLOWED_NAMESPACES: ['http://www.w3.org/2000/svg', 'http://www.w3.org/1998/Math/MathML'] },
  { CUSTOM_ELEMENT_HANDLING: { allowCustomizedBuiltInElements: true } },
  { ...FINAL, FORBID_TAGS: ['style'] },
  { ...FINAL, ALLOW_DATA_ATTR: 'false', KEEP_CONTENT: 0, SAFE_FOR_XML: null, ALLOW_UNKNOWN_PROTOCOLS: 'yes' },
  { ADD_TAGS: 'foo', ADD_ATTR: 'foo', FORBID_TAGS: 'b', ALLOWED_TAGS: { b: true }, FORBID_ATTR: null },
  { ALLOWED_TAGS: ['b', 1, null, 'NaN'], ADD_ATTR: [true, 'x'] },
  { NAMESPACE: 'http://www.w3.org/1999/xhtml', PARSER_MEDIA_TYPE: 'text/html' },
  { PARSER_MEDIA_TYPE: 'text/plain' },
  { TRUSTED_TYPES_POLICY: { name: 'x' } },
  { TRUSTED_TYPES_POLICY: null },
];

// ---- jobs ----

const jobs = [{ kind: 'env' }];
const add = (job) => jobs.push(job) - 1;
const jSvg = add({ kind: 'purify', hooks: false, cfg: FINAL, inputs: svgs.map((s) => s.in) });
const jSvgHooks = add({ kind: 'purify', hooks: true, cfg: FINAL, inputs: svgs.map((s) => s.in) });
const jFinal = add({ kind: 'purify', hooks: false, cfg: FINAL, inputs: finalInputs });
const jFinalHooks = add({ kind: 'purify', hooks: true, cfg: FINAL, inputs: finalInputs });
const labelRuns = [];
for (const cfg of labelConfigs) {
  for (const hooks of [false, true]) {
    labelRuns.push({ cfg, hooks, job: add({ kind: 'purify', hooks, cfg, inputs: labels }) });
  }
}
const variantRuns = variants.map((cfg) => ({
  cfg,
  hooks: false,
  job: add({ kind: 'purify', hooks: false, cfg, inputs: variantInputs }),
}));

// Hooks: a trace of every hook call, and hooks that change the result.
const hookInputs = unique([
  ...handWritten,
  ...generated(300, 0x686f6f6b),
  '<foo>kept</foo><bar><b>x</b></bar><b>y</b>',
  '<gone>a<b>b</b></gone>c<gone/>d',
  '<p x-keep="1" title="lower" class="c" id="i">secret text</p>',
  '<p x-allow="1">a</p><p x-allow="2" id="j" x-forbid-id>b</p><i id="k">c</i>',
  '<div><template><p title="t" class="c">secret<gone>x</gone></p><foo>y</foo></template></div>',
  '<svg><foo x-keep="javascript:x"></foo><rect title=" a " class="c"/></svg>',
]);
const jTrace = add({ kind: 'purifyTrace', hooks: false, cfg: null, inputs: hookInputs });
const jTraceFinal = add({ kind: 'purifyTrace', hooks: false, cfg: FINAL, inputs: hookInputs.filter((_, i) => i % 2 === 0) });
const jMutate = add({ kind: 'purifyMutate', hooks: false, cfg: null, inputs: hookInputs });

// The parser golden of internal/dom: the tree that DOMParser builds for every
// input that has markup, and for every SVG and MathML name whose case the
// parser can adjust.
const SVG_TAG_CANDIDATES = [
  'altGlyph', 'altGlyphDef', 'altGlyphItem', 'animateColor', 'animateMotion', 'animateTransform',
  'clipPath', 'feBlend', 'feColorMatrix', 'feComponentTransfer', 'feComposite', 'feConvolveMatrix',
  'feDiffuseLighting', 'feDisplacementMap', 'feDistantLight', 'feDropShadow', 'feFlood', 'feFuncA',
  'feFuncB', 'feFuncG', 'feFuncR', 'feGaussianBlur', 'feImage', 'feMerge', 'feMergeNode',
  'feMorphology', 'feOffset', 'fePointLight', 'feSpecularLighting', 'feSpotLight', 'feTile',
  'feTurbulence', 'foreignObject', 'glyphRef', 'linearGradient', 'radialGradient', 'textPath',
];
const SVG_ATTR_CANDIDATES = [
  'attributeName', 'attributeType', 'baseFrequency', 'baseProfile', 'calcMode', 'clipPathUnits',
  'contentScriptType', 'contentStyleType', 'diffuseConstant', 'edgeMode', 'externalResourcesRequired',
  'filterRes', 'filterUnits', 'glyphRef', 'gradientTransform', 'gradientUnits', 'kernelMatrix',
  'kernelUnitLength', 'keyPoints', 'keySplines', 'keyTimes', 'lengthAdjust', 'limitingConeAngle',
  'markerHeight', 'markerUnits', 'markerWidth', 'maskContentUnits', 'maskUnits', 'numOctaves',
  'pathLength', 'patternContentUnits', 'patternTransform', 'patternUnits', 'pointsAtX', 'pointsAtY',
  'pointsAtZ', 'preserveAlpha', 'preserveAspectRatio', 'primitiveUnits', 'refX', 'refY',
  'repeatCount', 'repeatDur', 'requiredExtensions', 'requiredFeatures', 'specularConstant',
  'specularExponent', 'spreadMethod', 'startOffset', 'stdDeviation', 'stitchTiles', 'surfaceScale',
  'systemLanguage', 'tableValues', 'targetX', 'targetY', 'textLength', 'viewBox', 'viewTarget',
  'xChannelSelector', 'yChannelSelector', 'zoomAndPan',
];
const adjustInputs = [
  ...SVG_TAG_CANDIDATES.map((t) => `<svg><${t.toLowerCase()}></${t.toLowerCase()}></svg>`),
  ...SVG_ATTR_CANDIDATES.map((a) => `<svg ${a.toLowerCase()}="1"><g ${a}="2"></g></svg>`),
  '<math definitionurl="1"><mi DEFINITIONURL="2"></mi></math>',
  '<svg xlink:actuate="1" xlink:arcrole="2" xlink:href="3" xlink:role="4" xlink:show="5" xlink:title="6" xlink:type="7" xml:base="8" xml:lang="9" xml:space="10" xmlns="11" xmlns:xlink="12" xmlns:foo="13" xlink:foo="14" xml:id="15"></svg>',
  '<div xlink:href="1" xml:lang="2" xmlns="3" xmlns:xlink="4" viewbox="5"></div>',
];
const parseInputs = unique([
  ...adjustInputs,
  ...[...finalInputs, ...labels, ...variantInputs, ...hookInputs].filter((s) => s.includes('<')),
]);
const jParse = add({ kind: 'parse', inputs: parseInputs });

const { ua, results } = runJobs(jobs);
const chrome = /HeadlessChrome\/[\d.]+/.exec(ua)[0];
const env = results[0];
if (env.version !== '3.4.12' || !env.isSupported) {
  throw new Error('unexpected DOMPurify: ' + JSON.stringify({ v: env.version, s: env.isSupported }));
}

// ---- clobber_names.go ----

{
  const names = [...new Set([...env.documentNames, ...env.formNames])].sort();
  let src =
    '// Code generated by tools/reference/gen-purify.mjs. DO NOT EDIT.\n\npackage purify\n\n' +
    '// clobberingNames is the set of property names for which the check of\n' +
    '// _isValidAttribute, `value in document || value in formElement`, is true in\n' +
    `// ${chrome}. The document is the owner document of the contents\n` +
    '// of a template, and the form is an empty form of that document. An id or a\n' +
    '// name attribute with one of these values is removed.\n' +
    'var clobberingNames = newSet([]string{\n';
  let line = '\t';
  for (const v of names) {
    const item = JSON.stringify(v) + ', ';
    if (line.length + item.length > 100) {
      src += line.trimEnd() + '\n';
      line = '\t';
    }
    line += item;
  }
  src += line.trimEnd() + '\n})\n';
  fs.writeFileSync(path.join(OUT, 'clobber_names.go'), src);
  console.log(`clobber_names.go: ${names.length} names`);
}

// ---- goldens ----

// sameAs replaces the entries of b that equal the entry of a with null.
const sameAs = (a, b) => b.map((v, i) => (JSON.stringify(v) === JSON.stringify(a[i]) ? null : v));

const meta = { dompurify: env.version, browser: chrome };

writeGolden(path.join(TESTDATA, 'svg_clean.json.gz'), {
  ...meta,
  config: FINAL,
  types: svgTypes,
  cases: svgs.map((s, i) => ({ name: s.name, in: s.in, out: results[jSvg][i] })),
  outHooks: sameAs(results[jSvg], results[jSvgHooks]),
});

writeGolden(path.join(TESTDATA, 'final_hostile.json.gz'), {
  ...meta,
  config: FINAL,
  inputs: finalInputs,
  out: results[jFinal],
  outHooks: sameAs(results[jFinal], results[jFinalHooks]),
});

writeGolden(path.join(TESTDATA, 'labels.json.gz'), {
  ...meta,
  inputs: labels,
  runs: labelRuns.map((r) => ({ config: r.cfg, hooks: r.hooks, out: results[r.job] })),
});

writeGolden(path.join(TESTDATA, 'configs.json.gz'), {
  ...meta,
  inputs: variantInputs,
  runs: variantRuns.map((r) => ({ config: r.cfg, hooks: r.hooks, out: results[r.job] })),
});

for (const tree of results[jParse]) {
  if (typeof tree !== 'string') {
    throw new Error('DOMParser failed: ' + JSON.stringify(tree));
  }
}
writeGolden(path.join(ROOT, 'internal/dom/testdata/purify_parse.json.gz'), {
  browser: chrome,
  inputs: parseInputs,
  trees: results[jParse],
});

writeGolden(path.join(TESTDATA, 'hooks.json.gz'), {
  ...meta,
  inputs: hookInputs,
  trace: results[jTrace],
  traceFinal: { config: FINAL, every: 2, out: results[jTraceFinal] },
  mutate: results[jMutate],
});

const count = (list) => list.filter((v) => v !== 0).length;
console.log(
  `svg: ${svgs.length} (${count(results[jSvg])} changed), final hostile: ${finalInputs.length}, ` +
    `labels: ${labels.length} x ${labelRuns.length}, variants: ${variantInputs.length} x ${variants.length}`
);
