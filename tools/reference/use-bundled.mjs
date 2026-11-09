// Makes node load stylis, d3-format and es-toolkit at the version that the
// mermaid bundle contains, in place of the newer version in node_modules.
//
// Why: the byte-exact reference is mermaid.min.js in Chromium. It contains
// stylis 4.3.6, d3-format 3.1.0 and es-toolkit 1.45.1. npm installed stylis
// 4.4.0, d3-format 3.1.2 and es-toolkit 1.53.1, and those versions give other
// results for some inputs. A golden file must come from the bundled code.
//
// How: extract-bundled-source.mjs writes the exact text of the bundled
// modules, from the source map of the bundle, to .upstream/bundled. A resolve
// hook sends the import of a package below to those files. The hook works for
// an import in a generator and for an import inside another package
// (mermaid.core.mjs imports stylis and es-toolkit, d3-scale imports d3-format).
//
// Usage: import this file first, then load everything else with a dynamic
// import. A static import is resolved before this file runs, so it would get
// the version in node_modules.
//
//   import { bundledVersion } from './use-bundled.mjs';
//   const { format } = await import('d3-format');
//
// The bundle has only the modules that mermaid reaches. Two things follow.
// 1. A bundled module can import a module that the bundle does not contain
//    (stylis/src/Middleware.js imports Prefixer.js). The hook gives a stub for
//    it. Each export of the stub throws when it is called.
// 2. The index of a package can be absent (stylis) or can name modules that
//    are absent (es-toolkit/compat). The hook makes the index from the modules
//    that are present.
import fs from 'node:fs';
import path from 'node:path';
import { registerHooks } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { BUNDLED_ROOT, bundledDir, readBundle } from './extract-bundled-source.mjs';

// ENTRIES maps an import specifier to the bundled module that answers it.
//   file    a bundled file that is complete: the hook loads it as it is.
//   index   a bundled index file. The hook removes the re-exports of modules
//           that the bundle does not contain.
//   all     no index file is in the bundle. The hook makes "export *" lines
//           for these files, which are the lines of the index of the package
//           without the modules that are absent.
const ENTRIES = {
  stylis: {
    pkg: 'stylis',
    all: ['src/Enum.js', 'src/Utility.js', 'src/Parser.js', 'src/Tokenizer.js', 'src/Serializer.js', 'src/Middleware.js'],
  },
  'd3-format': { pkg: 'd3-format', file: 'src/index.js' },
  'es-toolkit/compat': { pkg: 'es-toolkit', index: 'dist/compat/index.mjs' },
};
const PACKAGES = [...new Set(Object.values(ENTRIES).map((e) => e.pkg))];

const bundle = readBundle();
const dirs = new Map(PACKAGES.map((pkg) => [pkg, bundledDir(pkg)]));
const BUNDLED_URL = pathToFileURL(BUNDLED_ROOT).href + '/';
const ENTRY_NAME = '__bundled_entry__.mjs';

// bundledVersion returns the version of a package in the mermaid bundle.
export function bundledVersion(pkg) {
  const hits = [...bundle.values()].filter((p) => p.name === pkg);
  if (hits.length !== 1) throw new Error(`${pkg}: ${hits.length} versions in the bundle`);
  return hits[0].version;
}

const exists = (url) => fs.existsSync(fileURLToPath(url));

// filterIndex keeps the re-export lines of an index file whose module is in
// the bundle.
function filterIndex(url, source) {
  const out = [];
  for (const line of source.split('\n')) {
    const m = /^export\s.*\sfrom\s+['"](\.[^'"]+)['"];?\s*$/.exec(line);
    if (m && !exists(new URL(m[1], url))) continue;
    out.push(line);
  }
  return out.join('\n');
}

// stubFor makes the source of a module that the bundle does not contain. Its
// exports are the names that the bundled modules of the package import from
// it.
function stubFor(url) {
  const file = fileURLToPath(url);
  const pkgDir = [...dirs.values()].find((d) => file.startsWith(d + path.sep));
  const names = new Set();
  const walk = (dir) => {
    for (const name of fs.readdirSync(dir)) {
      const p = path.join(dir, name);
      if (fs.statSync(p).isDirectory()) walk(p);
      else if (/\.m?js$/.test(name)) {
        const text = fs.readFileSync(p, 'utf8');
        for (const m of text.matchAll(/^import\s*\{([^}]*)\}\s*from\s*['"](\.[^'"]+)['"]/gm)) {
          if (path.resolve(path.dirname(p), m[2]) !== file) continue;
          for (const part of m[1].split(',')) {
            const name = part.trim().split(/\s+as\s+/)[0];
            if (name) names.add(name);
          }
        }
      }
    }
  };
  walk(pkgDir);
  const rel = path.relative(BUNDLED_ROOT, file);
  return [...names]
    .map((n) => `export function ${n}() { throw new Error(${JSON.stringify(`${n} of ${rel} is not in the mermaid bundle`)}); }`)
    .join('\n');
}

registerHooks({
  resolve(specifier, context, nextResolve) {
    const entry = ENTRIES[specifier];
    if (entry) {
      const dir = dirs.get(entry.pkg);
      const file = entry.file ?? entry.index ?? ENTRY_NAME;
      return { url: pathToFileURL(path.join(dir, file)).href, format: 'module', shortCircuit: true };
    }
    for (const pkg of PACKAGES) {
      if (specifier === pkg || specifier.startsWith(pkg + '/')) {
        throw new Error(`use-bundled.mjs has no entry for "${specifier}": the import would load the version in node_modules`);
      }
    }
    // A bundled module that imports a module that the bundle does not contain.
    if (specifier.startsWith('.') && context.parentURL?.startsWith(BUNDLED_URL)) {
      const url = new URL(specifier, context.parentURL).href;
      if (!exists(url)) return { url, format: 'module', shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (!url.startsWith(BUNDLED_URL)) return nextLoad(url, context);
    for (const [, entry] of Object.entries(ENTRIES)) {
      const dir = dirs.get(entry.pkg);
      if (entry.all && url === pathToFileURL(path.join(dir, ENTRY_NAME)).href) {
        const source = entry.all.map((f) => `export * from './${f}';`).join('\n');
        return { format: 'module', source, shortCircuit: true };
      }
      if (entry.index && url === pathToFileURL(path.join(dir, entry.index)).href) {
        const source = filterIndex(url, fs.readFileSync(fileURLToPath(url), 'utf8'));
        return { format: 'module', source, shortCircuit: true };
      }
    }
    if (!exists(url)) return { format: 'module', source: stubFor(url), shortCircuit: true };
    return nextLoad(url, context);
  },
});
