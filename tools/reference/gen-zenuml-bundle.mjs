// Builds internal/diagrams/zenuml/zenuml.bundle.js.gz: the code that the
// ZenUML plugin of mermaid-cli runs for a render, as one script for goja.
//
//   node gen-zenuml-bundle.mjs [--check] [--plain FILE] [--no-speed] [--out FILE]
//
// mmdc 12.0.0 loads dist/mermaid-zenuml.esm.mjs of @mermaid-js/mermaid-zenuml
// 1.0.1. That file contains @zenuml/core 3.49.2 as one minified module of
// 2 MB, with React, the HTML renderer and 500 cloud icons. The draw function
// of the plugin calls one function of it, renderToSvg, which uses a small
// part: the ANTLR parser, the layout and the SVG string builder.
//
// The source maps of the plugin (dist/chunks/mermaid-zenuml.esm/*.mjs.map)
// hold every source file of that build unchanged in sourcesContent. This
// script bundles renderToSvg from those files with esbuild, so the bundle
// has the versions that the plugin ships and not the versions that npm
// installed (node_modules has @zenuml/core 3.50.1). For antlr4 and marked,
// which node_modules has in the same version, the script checks that the
// bytes are the same.
//
// What the script changes, and nothing else:
//
//   - The package pino (the logger) becomes a logger that prints nothing.
//     The package fs is empty, as in the browser build.
//   - The patches of PATCHES. Each one names its reason. A patch of the
//     group "speed" changes how fast the code runs and not its result, and
//     --no-speed leaves that group out: the tests of the Go package compare
//     both bundles.
//
// The script also writes zenuml.bundle.json (the versions, the modules with
// their hashes, the patches) and LICENSES.txt.
//
// --check builds the files and compares them with the files on disk,
// without a write. --plain FILE writes the bundle without minification, to
// read it. --out FILE writes the bundle to another file and writes nothing
// else.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import esbuild from 'esbuild';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const outDir = path.join(root, 'internal/diagrams/zenuml');
const pluginDir = path.join(here, 'node_modules/@mermaid-js/mermaid-zenuml');
const chunkDir = path.join(pluginDir, 'dist/chunks/mermaid-zenuml.esm');

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const option = (name) => (args.includes(name) ? args[args.indexOf(name) + 1] : undefined);
const check = flag('--check');
const speed = !flag('--no-speed');
const plainFile = option('--plain');
const outFile = option('--out');

const PLUGIN_VERSION = '1.0.1';
const CORE_VERSION = '3.49.2';
const CORE_LICENSE_VERSION = '3.49.2';
const ANTLR_LICENSE_VERSION = '4.11.0';
const EXCLUDED_ASSETS = [
  {
    provider: 'AWS',
    participantType: 'ec2',
    file: 'src/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Compute/Res_Amazon-EC2_Instance_48.svg',
  },
  {
    provider: 'AWS',
    participantType: 'iam',
    file: 'src/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Security-Identity-Compliance/Res_AWS-Identity-Access-Management_IAM-Access-Analyzer_48.svg',
  },
  {
    provider: 'AWS',
    participantType: 'lambda',
    file: 'src/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Compute/Res_AWS-Lambda_Lambda-Function_48.svg',
  },
  {
    provider: 'AWS',
    participantType: 'sns',
    file: 'src/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Application-Integration/Res_Amazon-Simple-Notification-Service_Topic_48.svg',
  },
  {
    provider: 'AWS',
    participantType: 'sqs',
    file: 'src/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Application-Integration/Res_Amazon-Simple-Queue-Service_Queue_48.svg',
  },
  {
    provider: 'Azure',
    participantType: 'azurefunction',
    file: 'src/assets/Azure_Public_Service_Icons/Icons/Compute/10029-icon-service-Function-Apps.svg',
  },
];
if (CORE_VERSION !== CORE_LICENSE_VERSION) {
  throw new Error(`update the @zenuml/core license text for ${CORE_VERSION}`);
}

const sha256 = (s) => crypto.createHash('sha256').update(s).digest('hex');

// ---- the sources of the build ----

// files maps a path below the package @zenuml/core of the build to its
// text: "src/svg/renderToSvg.ts", "node_modules/antlr4/src/antlr4/Lexer.js".
const files = new Map();
const pluginVersion = JSON.parse(fs.readFileSync(path.join(pluginDir, 'package.json'), 'utf8')).version;
if (pluginVersion !== PLUGIN_VERSION) {
  throw new Error(`@mermaid-js/mermaid-zenuml is ${pluginVersion}, want ${PLUGIN_VERSION}`);
}
const coreMarker = `/@zenuml+core@${CORE_VERSION}_`;
for (const name of fs.readdirSync(chunkDir).filter((f) => f.endsWith('.mjs.map')).sort()) {
  const map = JSON.parse(fs.readFileSync(path.join(chunkDir, name), 'utf8'));
  map.sources.forEach((source, i) => {
    const at = source.indexOf('/node_modules/@zenuml/core/');
    if (at < 0 || map.sourcesContent[i] == null) {
      return;
    }
    if (!source.includes(coreMarker)) {
      throw new Error(`${name}: ${source} is not of @zenuml/core ${CORE_VERSION}`);
    }
    const key = source.slice(at + '/node_modules/@zenuml/core/'.length);
    const text = map.sourcesContent[i];
    if (files.has(key) && files.get(key) !== text) {
      throw new Error(`${key} has two texts in the source maps`);
    }
    files.set(key, text);
  });
}
if (!files.has('src/svg/renderToSvg.ts')) {
  throw new Error('the source maps have no src/svg/renderToSvg.ts');
}
for (const asset of EXCLUDED_ASSETS) {
  if (!files.has(asset.file)) {
    throw new Error(`the source maps do not have the excluded ${asset.provider} asset ${asset.file}`);
  }
}

// The class BraceBlockContext has another name in the build that the plugin
// ships: the build of @zenuml/core renames it, and the build of the plugin
// keeps that name with __name. Find it, for the patch below.
function shippedClassName(className) {
  const chunk = fs
    .readdirSync(chunkDir)
    .filter((f) => f.startsWith('zenuml-definition-') && f.endsWith('.mjs'))
    .map((f) => fs.readFileSync(path.join(chunkDir, f), 'utf8'));
  if (chunk.length !== 1) {
    throw new Error('want one zenuml-definition chunk');
  }
  const m = chunk[0].match(new RegExp(`\\.${className} = ([A-Za-z0-9_$]+);`));
  if (!m) {
    throw new Error(`the plugin build does not assign ${className}`);
  }
  const short = m[1];
  if (!new RegExp(`var ${short.replace(/\$/g, '\\$')} = class extends [^{]*\\{\\s*static \\{\\s*__name\\(this, "${short.replace(/\$/g, '\\$')}"\\);`).test(chunk[0])) {
    throw new Error(`the plugin build does not name the class ${short}`);
  }
  if (!chunk[0].includes(`this.constructor.name === "${className}"`)) {
    throw new Error(`the plugin build does not compare a class name with ${className}`);
  }
  return short;
}
const braceBlockName = shippedClassName('BraceBlockContext');
if (braceBlockName === 'BraceBlockContext') {
  throw new Error('the plugin build keeps the name BraceBlockContext');
}

// ---- patches ----

const PATCHES = [
  {
    name: 'omit AWS and Azure participant artwork imports',
    group: 'license',
    file: 'src/svg/icons.ts',
    reason:
      'The distribution terms for these six vendor artwork files are not confirmed. ' +
      'The participant renderer shows a labeled box when getIcon has no matching icon.',
    old: `import Ec2Svg from "@/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Compute/Res_Amazon-EC2_Instance_48.svg?raw";
import IamSvg from "@/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Security-Identity-Compliance/Res_AWS-Identity-Access-Management_IAM-Access-Analyzer_48.svg?raw";
import LambdaSvg from "@/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Compute/Res_AWS-Lambda_Lambda-Function_48.svg?raw";
import SnsSvg from "@/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Application-Integration/Res_Amazon-Simple-Notification-Service_Topic_48.svg?raw";
import SqsSvg from "@/assets/AWS-Asset-Package_02062024/Resource-Icons_01312024/Res_Application-Integration/Res_Amazon-Simple-Queue-Service_Queue_48.svg?raw";
import AzureFunctionSvg from "@/assets/Azure_Public_Service_Icons/Icons/Compute/10029-icon-service-Function-Apps.svg?raw";
`,
    new: '',
  },
  {
    name: 'omit AWS and Azure participant artwork mappings',
    group: 'license',
    file: 'src/svg/icons.ts',
    reason:
      'Without these mappings, AWS and Azure participant types use the existing plain-box and text-label path.',
    old: `  ec2: parseRawSvg(Ec2Svg),
  lambda: parseRawSvg(LambdaSvg),
  azurefunction: parseRawSvg(AzureFunctionSvg),
  sqs: parseRawSvg(SqsSvg),
  sns: parseRawSvg(SnsSvg),
  iam: parseRawSvg(IamSvg),
`,
    new: '',
  },
  {
    name: 'remove reserved space for omitted vendor artwork',
    group: 'license',
    file: 'src/parser/OrderedParticipants.ts',
    reason:
      'The six participant types have no icon in this bundle, so they must not reserve the icon column width.',
    old: `  hasIcon(): boolean {
    // Only participants with a defined type property have icons
    return this.type !== undefined;
  }`,
    new: `  hasIcon(): boolean {
    // The AWS and Azure types use the plain participant layout in this bundle.
    return this.type !== undefined && !["ec2", "iam", "lambda", "sns", "sqs", "azurefunction"].includes(this.type.toLowerCase());
  }`,
  },
  {
    name: 'class name of BraceBlockContext',
    group: 'build',
    file: 'src/parser/index.js',
    reason:
      `getComment compares this.constructor.name with "BraceBlockContext". In the build that the plugin ships the ` +
      `class has the name "${braceBlockName}", so the comparison is false for every context. The patch writes that result.`,
    old: `  if (this.constructor.name === "BraceBlockContext") {`,
    new: `  if (false) {`,
  },
  {
    name: 'reset of the width cache',
    group: 'reuse',
    file: 'src/utils/RenderingCache.ts',
    reason:
      'The module keeps measured widths for the life of the page, and mmdc renders one diagram in a page. ' +
      'resetCache empties the module, so that a runtime that renders a second diagram starts as a new page does.',
    old: `//Need call clearCache before rendering.`,
    new: `export const resetCache = () => {
  dic = {};
  for (const key of Object.keys(persistDic)) {
    delete persistDic[key];
  }
};
//Need call clearCache before rendering.`,
  },
  {
    name: 'two-stage parse',
    group: 'speed',
    file: 'src/parser/index.js',
    reason:
      'The parser first runs with the prediction mode SLL. ANTLR guarantees that an SLL parse that reports no syntax ' +
      'error returns the parse tree of the default mode LL. When the SLL parse reports a syntax error, the text is ' +
      'parsed again in the default mode, as upstream does. The grammar needs full-context prediction for most ' +
      'statements, which the default mode computes again for every statement and SLL keeps in its DFA.',
    old: `function rootContext(code) {
  const chars = new antlr4.InputStream(code);
  const lexer = new sequenceLexer(chars);
  const tokens = new antlr4.CommonTokenStream(lexer);
  const parser = new sequenceParser(tokens);
  parser.addErrorListener(new SeqErrorListener());
  return parser._syntaxErrors ? null : parser.prog();
}`,
    new: `class SilentErrorListener extends antlr4.error.ErrorListener {}
function rootContext(code) {
  {
    const chars = new antlr4.InputStream(code);
    const lexer = new sequenceLexer(chars);
    lexer.removeErrorListeners();
    const tokens = new antlr4.CommonTokenStream(lexer);
    const parser = new sequenceParser(tokens);
    parser.removeErrorListeners();
    parser.addErrorListener(new SilentErrorListener());
    parser._interp.predictionMode = antlr4.atn.PredictionMode.SLL;
    const tree = parser.prog();
    if (parser._syntaxErrors === 0) {
      return tree;
    }
  }
  const chars = new antlr4.InputStream(code);
  const lexer = new sequenceLexer(chars);
  const tokens = new antlr4.CommonTokenStream(lexer);
  const parser = new sequenceParser(tokens);
  parser.addErrorListener(new SeqErrorListener());
  return parser._syntaxErrors ? null : parser.prog();
}`,
  },
];

// ---- the modules that replace a package ----

const STUBS = {
  // src/logger/logger.ts calls pino({ level }) and wraps the result. With
  // this logger no level is enabled, so the wrappers print nothing.
  pino: `export default function pino() {
  const logger = { child() { return logger; }, isLevelEnabled() { return false; } };
  return logger;
}
`,
  // The browser build of @zenuml/core has an empty module for fs.
  fs: `export default {};\n`,
  // Only types are imported from react.
  react: `export default {};\n`,
};

const PACKAGES = {
  antlr4: 'node_modules/antlr4/src/antlr4/index.js',
  marked: 'node_modules/marked/lib/marked.esm.js',
};

// ENTRY is the module that the bundle exports.
const ENTRY = `export { renderToSvg } from "@/svg/renderToSvg";
export { resetCache } from "@/utils/RenderingCache";
import Parser from "@/parser/index.js";
export const parserErrors = [Parser.Errors, Parser.ErrorDetails];
`;

function resolveFile(target) {
  for (const ext of ['', '.ts', '.tsx', '.js', '/index.ts', '/index.js']) {
    if (files.has(target + ext)) {
      return target + ext;
    }
  }
  return null;
}

function bundlePlugin(applied, used) {
  return {
    name: 'zenuml-sources',
    setup(build) {
      build.onResolve({ filter: /.*/ }, (a) => {
        if (a.kind === 'entry-point') {
          return { path: 'entry.js', namespace: 'entry' };
        }
        const spec = a.path.replace(/\?raw$/, '');
        if (Object.hasOwn(STUBS, spec)) {
          return { path: spec, namespace: 'stub' };
        }
        let target;
        if (Object.hasOwn(PACKAGES, spec)) {
          target = PACKAGES[spec];
        } else if (spec.startsWith('@/')) {
          target = 'src/' + spec.slice(2);
        } else if (spec.startsWith('.')) {
          target = path.posix.normalize(path.posix.join(path.posix.dirname(a.importer), spec));
        } else {
          return { errors: [{ text: `${a.importer} imports the package ${spec}, which the bundle does not know` }] };
        }
        const file = resolveFile(target);
        if (!file) {
          return { errors: [{ text: `${a.importer} imports ${a.path}, which the source maps do not have` }] };
        }
        return { path: file, namespace: 'core' };
      });
      build.onLoad({ filter: /.*/, namespace: 'entry' }, () => ({ contents: ENTRY, loader: 'js' }));
      build.onLoad({ filter: /.*/, namespace: 'stub' }, (a) => ({ contents: STUBS[a.path], loader: 'js' }));
      build.onLoad({ filter: /.*/, namespace: 'core' }, (a) => {
        let text = files.get(a.path);
        used.set(a.path, text);
        for (const p of PATCHES) {
          if (p.file !== a.path || (p.group === 'speed' && !speed)) {
            continue;
          }
          const n = text.split(p.old).length - 1;
          if (n !== 1) {
            throw new Error(`patch "${p.name}" matches ${n} times in ${a.path}, want 1`);
          }
          text = text.replace(p.old, () => p.new);
          applied.add(p.name);
        }
        const ext = path.extname(a.path);
        return { contents: text, loader: ext === '.ts' ? 'ts' : ext === '.tsx' ? 'tsx' : 'js' };
      });
    },
  };
}

async function bundle(minify) {
  const applied = new Set();
  const used = new Map();
  const result = await esbuild.build({
    entryPoints: ['entry.js'],
    bundle: true,
    write: false,
    format: 'iife',
    globalName: '__zenumlCore',
    // goja runs this level without a change of the syntax of the classes
    // and of the optional chains.
    target: 'es2022',
    minify,
    charset: 'utf8',
    legalComments: 'none',
    metafile: true,
    logLevel: 'warning',
    plugins: [bundlePlugin(applied, used)],
  });
  for (const p of PATCHES) {
    if (!applied.has(p.name) && !(p.group === 'speed' && !speed)) {
      throw new Error(`patch "${p.name}" was not applied: the bundle does not load ${p.file}`);
    }
  }
  const output = Object.values(result.metafile.outputs)[0];
  return { text: result.outputFiles[0].text, used, inputs: output.inputs };
}

// gzip writes the same bytes on every machine: no time and no system in the
// header.
function gzip(text) {
  const data = zlib.gzipSync(Buffer.from(text, 'utf8'), { level: 9 });
  data.writeUInt32LE(0, 4);
  data[9] = 255;
  return data;
}

const plain = plainFile ? await bundle(false) : null;
if (plainFile) {
  fs.writeFileSync(plainFile, plain.text);
}
const built = await bundle(true);
if (outFile) {
  fs.writeFileSync(outFile, built.text);
  process.stderr.write(`${outFile}: ${built.text.length} bytes${speed ? '' : ' (no speed patches)'}\n`);
  process.exit(0);
}
if (!speed) {
  throw new Error('--no-speed needs --out FILE: the embedded bundle has every patch');
}

// The files of antlr4 and marked in node_modules, when they have the same
// version, must have the same bytes.
function sameAsNodeModules(key) {
  let file;
  if (key.startsWith('node_modules/antlr4/')) {
    file = path.join(here, key);
  } else if (key.startsWith('node_modules/marked/')) {
    file = path.join(pluginDir, key);
  } else {
    return null;
  }
  if (!fs.existsSync(file)) {
    return null;
  }
  return fs.readFileSync(file, 'utf8') === files.get(key);
}
const antlrVersion = JSON.parse(fs.readFileSync(path.join(here, 'node_modules/antlr4/package.json'), 'utf8')).version;
const markedVersion = JSON.parse(fs.readFileSync(path.join(pluginDir, 'node_modules/marked/package.json'), 'utf8')).version;
const runtimeVersion = (files.get('node_modules/antlr4/src/antlr4/Recognizer.js').match(/runtimeVersion = "([^"]+)"/) || [])[1];
const markedHeader = (files.get('node_modules/marked/lib/marked.esm.js').match(/marked v([0-9.]+)/) || [])[1];
if (runtimeVersion !== antlrVersion || markedHeader !== markedVersion) {
  throw new Error(`the build has antlr4 ${runtimeVersion} and marked ${markedHeader}; node_modules has ${antlrVersion} and ${markedVersion}`);
}
if (runtimeVersion !== ANTLR_LICENSE_VERSION) {
  throw new Error(`update the antlr4 license text for ${runtimeVersion}`);
}

const modules = [];
for (const [key, text] of [...built.used.entries()].sort((a, b) => (a[0] < b[0] ? -1 : 1))) {
  const same = sameAsNodeModules(key);
  if (same === false) {
    throw new Error(`${key} differs from the file in node_modules`);
  }
  const input = built.inputs[`core:${key}`];
  modules.push({
    file: key,
    bytes: Buffer.byteLength(text),
    sha256: sha256(text),
    bytesInBundle: input ? input.bytesInOutput : 0,
    sameAsNodeModules: same,
  });
}
const group = (prefix) => modules.filter((m) => m.file.startsWith(prefix)).reduce((n, m) => n + m.bytesInBundle, 0);
const bundledAssets = modules.filter((m) => m.file.startsWith('src/assets/') && m.bytesInBundle > 0);
const coreAssetFiles = new Set([
  'src/assets/Robustness_Diagram_Boundary.svg',
  'src/assets/Robustness_Diagram_Control.svg',
  'src/assets/Robustness_Diagram_Entity.svg',
  'src/assets/actor.svg',
  'src/assets/database.svg',
]);
const coreAssets = bundledAssets.filter((m) => coreAssetFiles.has(m.file));
const unclassifiedAssets = bundledAssets.filter((m) => !coreAssetFiles.has(m.file));
if (unclassifiedAssets.length > 0) {
  throw new Error(`classify the license terms for bundled assets: ${unclassifiedAssets.map((m) => m.file).join(', ')}`);
}
if (coreAssets.length !== coreAssetFiles.size) {
  throw new Error(`the bundle has ${coreAssets.length} core icons, want ${coreAssetFiles.size}`);
}
const assetSources = (assets) =>
  assets.map((m) => `- ${m.file} (source SHA-256 ${m.sha256})`).join('\n');
const gz = gzip(built.text);
const manifest = {
  plugin: { package: '@mermaid-js/mermaid-zenuml', version: PLUGIN_VERSION, file: 'dist/mermaid-zenuml.esm.mjs' },
  core: { package: '@zenuml/core', version: CORE_VERSION, entry: 'src/svg/renderToSvg.ts (renderToSvg)' },
  packages: { antlr4: runtimeVersion, marked: markedHeader },
  esbuild: esbuild.version,
  bundle: { bytes: Buffer.byteLength(built.text), sha256: sha256(built.text), gzipBytes: gz.length },
  bytesInBundle: {
    'antlr4 (runtime)': group('node_modules/antlr4/'),
    'generated lexer and parser': group('src/generated-parser/'),
    marked: group('node_modules/marked/'),
    'icons (src/assets)': group('src/assets/'),
    '@zenuml/core (other)': group('src/') - group('src/generated-parser/') - group('src/assets/'),
  },
  excludedAssets: {
    reason: 'Distribution terms for the AWS and Azure artwork are not confirmed.',
    items: EXCLUDED_ASSETS,
  },
  stubs: Object.keys(STUBS),
  patches: PATCHES.map((p) => ({ name: p.name, group: p.group, file: p.file, reason: p.reason })),
  modules,
};
const manifestText = JSON.stringify(manifest, null, 1) + '\n';

// ---- licenses ----

const rule = '-'.repeat(72);
const coreLicense = `MIT License

Copyright (c) 2022 Peng Xiao

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`;
const antlrLicense = `Copyright (c) 2012-2022 The ANTLR Project. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:

1. Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright
notice, this list of conditions and the following disclaimer in the
documentation and/or other materials provided with the distribution.

3. Neither name of copyright holders nor the names of its contributors
may be used to endorse or promote products derived from this software
without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
\`\`AS IS'' AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED.  IN NO EVENT SHALL THE REGENTS OR
CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF
LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
`;
const licenses = [
  {
    title: `@zenuml/core ${CORE_VERSION} (https://github.com/mermaid-js/zenuml-core)`,
    note:
      `License: MIT. The code comes from @zenuml/core ${CORE_VERSION} in the source maps of ` +
      `@mermaid-js/mermaid-zenuml ${PLUGIN_VERSION}. The license text is the LICENSE file at ` +
      `https://raw.githubusercontent.com/mermaid-js/zenuml-core/v${CORE_LICENSE_VERSION}/LICENSE. ` +
      'The bundle source paths and hashes are in zenuml.bundle.json. The bundle includes these core icons:\n' +
      assetSources(coreAssets) +
      '\nThe bundle omits the six AWS and Azure artwork files listed in zenuml.bundle.json. Their participant ' +
      'types render as plain boxes with text labels. The corpus has no AWS or Azure sample.',
    text: coreLicense,
  },
  {
    title: `@mermaid-js/mermaid-zenuml ${PLUGIN_VERSION} (https://github.com/mermaid-js/mermaid)`,
    note:
      'License: MIT. This license covers the Go port in internal/diagrams/zenuml/zenuml.go. ' +
      'The plugin renderer code is not part of zenuml.bundle.js.gz. The text is the LICENSE file of the npm package.',
    text: fs.readFileSync(path.join(pluginDir, 'LICENSE'), 'utf8'),
  },
  {
    title: `antlr4 ${runtimeVersion} (https://github.com/antlr/antlr4)`,
    note:
      `License: BSD-3-Clause. The npm package has no license file. The text is the file ` +
      `LICENSE.txt of the tag ${ANTLR_LICENSE_VERSION} of the repository ` +
      `(https://github.com/antlr/antlr4/blob/${ANTLR_LICENSE_VERSION}/LICENSE.txt).`,
    text: antlrLicense,
  },
  {
    title: `marked ${markedHeader} (https://github.com/markedjs/marked)`,
    note: 'License: MIT. The text is the file LICENSE.md of the npm package.',
    text: fs.readFileSync(path.join(pluginDir, 'node_modules/marked/LICENSE.md'), 'utf8'),
  },
];
const licenseText =
  'License notices for the ZenUML integration\n\n' +
  'The script tools/reference/gen-zenuml-bundle.mjs writes this file.\n' +
  'The source paths and hashes for zenuml.bundle.js.gz are in zenuml.bundle.json. That manifest also lists the six omitted AWS and Azure artwork files.\n' +
  'The bundle uses @zenuml/core source-map files, the antlr4 runtime, and marked. The script replaces pino, fs, and react with local stubs. esbuild is a build tool and is not included in the bundle.\n\n' +
  licenses
    .map((l) => `${rule}\n${l.title}\n${rule}\n\n${l.note}${l.text.trim() ? `\n\n${l.text.trim()}` : ''}\n`)
    .join('\n');

// ---- write or check ----

const outputs = [
  ['zenuml.bundle.js.gz', gz],
  ['zenuml.bundle.json', Buffer.from(manifestText)],
  ['LICENSES.txt', Buffer.from(licenseText)],
];
let stale = 0;
for (const [name, data] of outputs) {
  const file = path.join(outDir, name);
  if (check) {
    if (!fs.existsSync(file) || !fs.readFileSync(file).equals(data)) {
      process.stderr.write(`${name} is not up to date\n`);
      stale++;
    }
  } else {
    fs.mkdirSync(outDir, { recursive: true });
    fs.writeFileSync(file, data);
  }
}
if (stale > 0) {
  process.exit(1);
}
process.stderr.write(
  `zenuml bundle: ${modules.length} modules, ${manifest.bundle.bytes} bytes, ${gz.length} bytes with gzip` + (check ? ' (up to date)\n' : '\n')
);
