// Gives the golden generators access to the real mermaid 12.2.0 functions
// that the package does not export.
//
// The built ESM (node_modules/mermaid/dist/mermaid.core.mjs) is loaded in
// plain node. A module load hook appends one export statement to that file so
// that the internal functions processAndSetConfigs and preprocessDiagram are
// reachable. No other line of mermaid changes. The config and theme functions
// come from the chunk that mermaid.core.mjs itself imports, so they share the
// module state of the mermaid instance.
//
// mermaid.core.mjs and its chunks import their dependencies from node_modules.
// use-bundled.mjs sends the imports of stylis, es-toolkit and d3-format to the
// versions that the mermaid bundle contains (4.3.6, 1.45.1 and 3.1.0), because
// node_modules has newer versions that give other results.
import './use-bundled.mjs';
import { registerHooks } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const DIST = path.join(HERE, 'node_modules/mermaid/dist');
const CHUNKS = path.join(DIST, 'chunks/mermaid.core');

registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/mermaid/dist/mermaid.core.mjs')) {
      const source =
        result.source.toString() +
        '\nexport { processAndSetConfigs as __processAndSetConfigs, preprocessDiagram as __preprocessDiagram };\n';
      return { ...result, source };
    }
    return result;
  },
});

// findChunk returns the chunk that has `name` in its export list. The chunk
// file names are build hashes.
const findChunk = (name) => {
  for (const file of fs.readdirSync(CHUNKS)) {
    if (!file.endsWith('.mjs')) continue;
    const source = fs.readFileSync(path.join(CHUNKS, file), 'utf8');
    const exports = source.slice(source.lastIndexOf('\nexport {'));
    if (new RegExp(`\\b${name}\\b`).test(exports)) return path.join(CHUNKS, file);
  }
  throw new Error(`no mermaid chunk exports ${name}`);
};

let cached;

// internals loads mermaid once and returns its API and internal functions.
export const internals = async () => {
  if (cached) return cached;
  const core = await import(pathToFileURL(path.join(DIST, 'mermaid.core.mjs')).href);
  const config = await import(pathToFileURL(findChunk('setDiagramConfigScope')).href);
  const utils = await import(pathToFileURL(findChunk('cleanAndMerge')).href);
  cached = {
    mermaid: core.default,
    processAndSetConfigs: core.__processAndSetConfigs,
    preprocessDiagram: core.__preprocessDiagram,
    config,
    themes: config.themes_default,
    utils: utils.utils_default,
    removeDirectives: utils.removeDirectives,
    cleanAndMerge: utils.cleanAndMerge,
  };
  return cached;
};

// encode turns a JavaScript value into JSON that keeps what JSON.stringify
// loses: undefined is {"$u":1}, and NaN, the infinities and -0 are
// {"$n":"<text>"}. A function-valued property is left out, as JSON.stringify
// does. The key order of objects is the JavaScript property order.
export const encode = (v) => {
  if (v === undefined) return { $u: 1 };
  if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
    return { $n: Object.is(v, -0) ? '-0' : String(v) };
  }
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return Array.from(v, encode);
  const o = {};
  for (const k of Object.keys(v)) {
    if (typeof v[k] === 'function') continue;
    Object.defineProperty(o, k, { value: encode(v[k]), enumerable: true, writable: true, configurable: true });
  }
  return o;
};

// mulberry32 is the seeded PRNG that the generators share.
export const mulberry32 = (seed) => {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
};
