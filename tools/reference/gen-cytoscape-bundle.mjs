// Builds internal/cytoscape/cytoscape.bundle.js: cytoscape and the layout
// extensions cose-bilkent and fcose as one script for goja.
//
//   node gen-cytoscape-bundle.mjs [--check]
//
// The sources are the files that mermaid bundles. The source maps of the
// mermaid ESM build (node_modules/mermaid/dist/chunks/mermaid.esm/*.mjs.map)
// hold each of them unchanged in sourcesContent, with the package version in
// the path. The script takes the files from there, so the bundle has the
// versions of the mermaid build and not the versions that npm installed:
// mermaid 12.2.0 bundles cytoscape 3.34.0, and node_modules has 3.34.4.
// For each file that node_modules has in the same version, the script checks
// that the bytes are the same.
//
// The code of the libraries is not changed and not minified. Each file
// becomes one function:
//
//   - cytoscape.esm.mjs has one export statement at the end. It becomes the
//     return statement of a function.
//   - The other files are webpack UMD modules. Each one becomes a CommonJS
//     module function, as in the mermaid build (esbuild __commonJS), with
//     `this` bound to the exports object and a require for its dependency.
//
// The whole script is one function expression in strict mode, as the mermaid
// build is. It returns { cytoscape, coseBilkent, fcose, layoutBase, ... }: the
// loaders run a module on first use, so a runtime pays only for the layout
// that it runs.
//
// The script also writes LICENSES.txt: the license file of each package,
// from node_modules, and the notices of code that the packages include.
//
// --check builds the text and compares it with the file, without a write.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const outDir = path.join(root, 'internal/cytoscape');
const outFile = path.join(outDir, 'cytoscape.bundle.js');
const manifestFile = path.join(outDir, 'cytoscape.bundle.json');
const licenseFile = path.join(outDir, 'LICENSES.txt');
const chunkDir = path.join(here, 'node_modules/mermaid/dist/chunks/mermaid.esm');
const check = process.argv.includes('--check');

// The modules of the bundle. `file` is the path inside the package. `dep` is
// the module that the UMD header requires, by the name of its variable.
const WANT = [
  { key: 'cytoscape', pkg: 'cytoscape', version: '3.34.0', file: 'dist/cytoscape.esm.mjs', kind: 'esm' },
  { key: 'layoutBase1', pkg: 'layout-base', version: '1.0.2', file: 'layout-base.js', kind: 'umd' },
  { key: 'coseBase1', pkg: 'cose-base', version: '1.0.3', file: 'cose-base.js', kind: 'umd', dep: { name: 'layout-base', key: 'layoutBase1' } },
  {
    key: 'coseBilkent',
    pkg: 'cytoscape-cose-bilkent',
    version: '4.1.0',
    file: 'cytoscape-cose-bilkent.js',
    kind: 'umd',
    dep: { name: 'cose-base', key: 'coseBase1' },
  },
  { key: 'layoutBase2', pkg: 'layout-base', version: '2.0.1', file: 'layout-base.js', kind: 'umd' },
  { key: 'coseBase2', pkg: 'cose-base', version: '2.2.0', file: 'cose-base.js', kind: 'umd', dep: { name: 'layout-base', key: 'layoutBase2' } },
  { key: 'fcose', pkg: 'cytoscape-fcose', version: '2.2.0', file: 'cytoscape-fcose.js', kind: 'umd', dep: { name: 'cose-base', key: 'coseBase2' } },
];

const sha256 = (s) => crypto.createHash('sha256').update(s).digest('hex');

// Collect the third-party sources of the mermaid build: package@version/file -> text.
const bundled = new Map();
for (const f of fs.readdirSync(chunkDir).filter((f) => f.endsWith('.mjs.map')).sort()) {
  const map = JSON.parse(fs.readFileSync(path.join(chunkDir, f), 'utf8'));
  map.sources.forEach((src, i) => {
    const m = src.match(/node_modules\/\.pnpm\/((?:@[^/]+\+)?[^/@]+)@([^/_]+)[^/]*\/node_modules\/(.+)$/);
    if (!m || map.sourcesContent[i] == null) {
      return;
    }
    const [, , version, rest] = m;
    const key = `${rest.split('/')[0]}@${version}/${rest.split('/').slice(1).join('/')}`;
    const text = map.sourcesContent[i];
    if (bundled.has(key) && bundled.get(key).text !== text) {
      throw new Error(`${key} has two different texts in the mermaid build`);
    }
    bundled.set(key, { text, chunk: f.replace(/\.map$/, '') });
  });
}

// The version of every copy of a package that mermaid bundles must be one of
// the versions that this script knows.
for (const pkg of new Set(WANT.map((w) => w.pkg))) {
  const versions = [...bundled.keys()].filter((k) => k.startsWith(pkg + '@')).map((k) => k.slice(pkg.length + 1).split('/')[0]);
  const known = WANT.filter((w) => w.pkg === pkg).map((w) => w.version);
  for (const v of new Set(versions)) {
    if (!known.includes(v)) {
      throw new Error(`mermaid bundles ${pkg}@${v}, and this script expects ${known.join(', ')}`);
    }
  }
}

// installed returns the text of the file in node_modules when a copy with the
// same version is there.
function installed(w) {
  for (const base of ['node_modules', 'node_modules/cytoscape-fcose/node_modules', 'node_modules/cytoscape-cose-bilkent/node_modules']) {
    const dir = path.join(here, base, w.pkg);
    const pj = path.join(dir, 'package.json');
    if (fs.existsSync(pj) && JSON.parse(fs.readFileSync(pj, 'utf8')).version === w.version) {
      return fs.readFileSync(path.join(dir, w.file), 'utf8');
    }
  }
  return null;
}

const manifest = { mermaid: JSON.parse(fs.readFileSync(path.join(here, 'node_modules/mermaid/package.json'), 'utf8')).version, modules: [] };
const parts = [];
for (const w of WANT) {
  const hit = bundled.get(`${w.pkg}@${w.version}/${w.file}`);
  if (!hit) {
    throw new Error(`the mermaid build has no ${w.pkg}@${w.version}/${w.file}`);
  }
  const inst = installed(w);
  if (inst !== null && inst !== hit.text) {
    throw new Error(`${w.pkg}@${w.version}/${w.file}: node_modules and the mermaid build differ`);
  }
  let text = hit.text;
  let body;
  if (w.kind === 'esm') {
    const exp = /\nexport \{ cytoscape as default \};\s*$/;
    if (!exp.test(text) || /^(?:import|export)\s/m.test(text.replace(exp, '\n'))) {
      throw new Error(`${w.pkg}: the module statements are not the single default export at the end`);
    }
    if (!text.includes(`var version = "${w.version}";`)) {
      throw new Error(`${w.pkg}: the text does not have version ${w.version}`);
    }
    body = `var ${w.key} = once(function () {\n${text.replace(exp, '\nreturn cytoscape;\n')}});\n`;
  } else {
    if (!text.startsWith('(function webpackUniversalModuleDefinition(root, factory) {') || !/\}\)\(this, function\(/.test(text)) {
      throw new Error(`${w.pkg}: not a webpack UMD module`);
    }
    const req = w.dep
      ? `function require(name) { if (name === ${JSON.stringify(w.dep.name)}) { return ${w.dep.key}(); } throw new Error('unknown module ' + name); }`
      : `function require(name) { throw new Error('unknown module ' + name); }`;
    body = `var ${w.key} = commonJS(function (exports, module) {\n${req}\n${text}\n});\n`;
  }
  parts.push(`// ---- ${w.pkg} ${w.version}: ${w.file} ----\n${body}`);
  manifest.modules.push({
    name: w.key,
    package: w.pkg,
    version: w.version,
    file: w.file,
    bytes: Buffer.byteLength(text),
    sha256: sha256(text),
    mermaidChunk: hit.chunk,
    sameAsNodeModules: inst === null ? null : true,
  });
}

const header = `// Generated by tools/reference/gen-cytoscape-bundle.mjs. Do not edit.
//
// cytoscape and the layout extensions cose-bilkent and fcose, as mermaid
// ${manifest.mermaid} bundles them:
//
${manifest.modules.map((m) => `//   ${m.package} ${m.version} (${m.file})`).join('\n')}
//
// The code of the libraries is unchanged. See LICENSES.txt for the licenses.
(function () {
'use strict';
// once runs a module function on first use and keeps the result.
function once(fn) {
  var done = false, value;
  return function () {
    if (!done) { value = fn(); done = true; }
    return value;
  };
}
// commonJS is __commonJS of esbuild, with this bound to the exports object
// as a script has it.
function commonJS(fn) {
  var mod;
  return function () {
    if (!mod) { mod = { exports: {} }; fn.call(mod.exports, mod.exports, mod); }
    return mod.exports;
  };
}
`;
const footer = `return {
${WANT.map((w) => `  ${w.key}: ${w.key}`).join(',\n')}
};
})
`;
const out = header + parts.join('') + footer;
manifest.bytes = Buffer.byteLength(out);
manifest.sha256 = sha256(out);
const manifestText = JSON.stringify(manifest, null, 1) + '\n';

// The licenses. The license file of a package is the same in every version
// that is used here, so the installed copy stands for the bundled one.
const licenseParts = [
  `Licenses of the JavaScript in cytoscape.bundle.js

The bundle holds these packages unchanged, in the versions that mermaid
${manifest.mermaid} bundles. All of them have the MIT license. The license file of each
package follows.
`,
];
const seenLicense = new Map();
for (const w of WANT) {
  let text = null;
  for (const base of ['node_modules', 'node_modules/cytoscape-fcose/node_modules']) {
    const f = path.join(here, base, w.pkg, 'LICENSE');
    const pj = path.join(here, base, w.pkg, 'package.json');
    if (fs.existsSync(f) && fs.existsSync(pj)) {
      const pkg = JSON.parse(fs.readFileSync(pj, 'utf8'));
      if (pkg.license !== 'MIT') {
        throw new Error(`${w.pkg}: license ${pkg.license}`);
      }
      text = fs.readFileSync(f, 'utf8').trim();
      if (pkg.version === w.version) {
        break;
      }
    }
  }
  if (text === null) {
    throw new Error(`${w.pkg}: no LICENSE file`);
  }
  const title = `${w.pkg} ${w.version}`;
  if (seenLicense.has(text)) {
    seenLicense.get(text).push(title);
  } else {
    seenLicense.set(text, [title]);
  }
}
for (const [text, titles] of seenLicense) {
  licenseParts.push(`${'-'.repeat(72)}\n${titles.join(', ')}\n${'-'.repeat(72)}\n\n${text}\n`);
}
// Code inside the packages that has its own notice.
const cy = bundled.get('cytoscape@3.34.0/dist/cytoscape.esm.mjs').text;
const notices = [...cy.matchAll(/\/\*!([\s\S]*?)\*\//g)].map((m) => m[1].replace(/^[ \t]*\*?[ \t]?/gm, '').trim()).filter(Boolean);
const lb2 = bundled.get('layout-base@2.0.1/layout-base.js').text;
const apacheStart = lb2.indexOf('Apache License');
if (notices.length === 0 || apacheStart < 0 || !lb2.includes('https://github.com/dragonfly-ai/JamaJS') || !cy.includes('lodash') || !cy.includes('heappush')) {
  throw new Error('the notices of included code are not where this script expects them');
}
licenseParts.push(`${'-'.repeat(72)}
Code that the packages include
${'-'.repeat(72)}

cytoscape includes code with these notices:

${[...new Set(notices)].map((n) => n.split('\n').map((l) => '  ' + l).join('\n')).join('\n\n')}

The build of cytoscape also inlines functions of the packages lodash and
heap, without a notice. Their license files are not here: the packages are
build dependencies of cytoscape and are not installed with it.

layout-base 2.0.1 includes a singular value decomposition that is adopted
from JamaJS (https://github.com/dragonfly-ai/JamaJS), which has the Apache
License, Version 2.0. The text of that license is in layout-base.js and so
in the bundle, in the comment after the notice of that code.
`);
const licenseText = licenseParts.join('\n');

if (check) {
  const same =
    fs.existsSync(outFile) &&
    fs.readFileSync(outFile, 'utf8') === out &&
    fs.readFileSync(manifestFile, 'utf8') === manifestText &&
    fs.readFileSync(licenseFile, 'utf8') === licenseText;
  console.log(same ? 'the bundle is up to date' : 'the bundle differs');
  process.exit(same ? 0 : 1);
}
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(outFile, out);
fs.writeFileSync(manifestFile, manifestText);
fs.writeFileSync(licenseFile, licenseText);
console.log(`${path.relative(root, outFile)}: ${manifest.bytes} bytes, sha256 ${manifest.sha256}`);
for (const m of manifest.modules) {
  console.log(`  ${m.package}@${m.version} ${m.bytes} bytes (${m.mermaidChunk})`);
}
