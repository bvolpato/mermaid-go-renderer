// Reads the source maps of the built mermaid and gives the source of the
// libraries that the bundle really contains.
//
// The byte-exact reference is the output of node_modules/mermaid/dist/mermaid.min.js
// in Chromium. That file contains fixed versions of its dependencies. npm
// installs the newest version that the ranges of mermaid allow, so
// node_modules/<package> can be a newer version than the code in the bundle.
// The source map of the bundle has the exact text of every bundled module in
// sourcesContent, and the path of each module has the package name and the
// version (node_modules/.pnpm/<name>@<version>/node_modules/<name>/<file>).
//
// The parser package (@mermaid-js/parser) is in the mermaid bundle as built
// chunks. The libraries inside those chunks (langium, chevrotain) are in the
// source maps of the parser package, which this script reads too.
//
// Usage (in tools/reference):
//   node extract-bundled-source.mjs list
//       Prints one line per bundled package: the version in the bundle, the
//       number of bundled files, the version in node_modules, and how many of
//       the bundled files have the same text in node_modules.
//   node extract-bundled-source.mjs extract <package> [<package> ...]
//       Writes the bundled files of each package to
//       .upstream/bundled/<package>@<version>/<file>.
//   node extract-bundled-source.mjs diff <package> [<package> ...]
//       Prints, for each bundled file of the package, if node_modules has the
//       same text, another text, or no such file.
//   node extract-bundled-source.mjs json
//       The data of "list" as JSON, for other scripts.
//
// Other scripts import bundledDir() to load a library at the bundled version.
// It extracts the package and returns the directory:
//   const dir = bundledDir('stylis');
//   const parser = await import(path.join(dir, 'src/Parser.js'));
// use-bundled.mjs does this for the import of a package name.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const REPO = path.resolve(HERE, '../..');
export const BUNDLED_ROOT = path.join(REPO, '.upstream/bundled');
const NODE_MODULES = path.join(HERE, 'node_modules');

// The maps to read. The first one is the bundle that the oracle loads. The
// maps of the parser package give the libraries inside the parser chunks.
function mapFiles() {
  const files = [path.join(NODE_MODULES, 'mermaid/dist/mermaid.min.js.map')];
  const parserDist = path.join(NODE_MODULES, '@mermaid-js/parser/dist');
  files.push(path.join(parserDist, 'mermaid-parser.core.mjs.map'));
  const chunkDir = path.join(parserDist, 'chunks/mermaid-parser.core');
  for (const f of fs.readdirSync(chunkDir).sort()) {
    if (f.endsWith('.map')) files.push(path.join(chunkDir, f));
  }
  return files;
}

const PNPM = /node_modules\/\.pnpm\/([^/]+)\/node_modules\/((?:@[^/]+\/)?[^/]+)\/(.*)$/;

// versionOf returns the version in a pnpm directory name such as
// "roughjs@4.6.6_patch_hash=35..." or "@iconify+utils@3.0.2". The second
// result says if pnpm applied a patch of the mermaid repository to the package.
function versionOf(pnpmDir) {
  const at = pnpmDir.indexOf('@', 1);
  const rest = pnpmDir.slice(at + 1);
  const cut = rest.indexOf('_');
  const version = cut < 0 ? rest : rest.slice(0, cut);
  return { version, patched: rest.includes('patch_hash=') };
}

// readBundle returns Map<"name@version", {name, version, patched, files: Map<file, text>, maps: Set}>.
export function readBundle() {
  const packages = new Map();
  for (const mapFile of mapFiles()) {
    const map = JSON.parse(fs.readFileSync(mapFile, 'utf8'));
    for (let i = 0; i < map.sources.length; i++) {
      const hit = PNPM.exec(map.sources[i]);
      if (!hit) continue;
      const [, pnpmDir, name, file] = hit;
      const { version, patched } = versionOf(pnpmDir);
      const key = `${name}@${version}`;
      let pkg = packages.get(key);
      if (!pkg) {
        pkg = { name, version, patched, files: new Map(), maps: new Set() };
        packages.set(key, pkg);
      }
      pkg.maps.add(path.relative(NODE_MODULES, mapFile));
      const text = map.sourcesContent[i];
      const have = pkg.files.get(file);
      if (have !== undefined && have !== text) {
        throw new Error(`${key}/${file} has two different texts in the source maps`);
      }
      pkg.files.set(file, text);
    }
  }
  return packages;
}

// installed returns every copy of a package under node_modules: the top level
// copy and the copies that npm nested under another package.
function installed(name) {
  const out = [];
  const add = (dir) => {
    const pj = path.join(dir, 'package.json');
    if (fs.existsSync(pj)) out.push({ dir, version: JSON.parse(fs.readFileSync(pj, 'utf8')).version });
  };
  add(path.join(NODE_MODULES, name));
  for (const parent of fs.readdirSync(NODE_MODULES)) {
    if (parent.startsWith('.')) continue;
    const parents = parent.startsWith('@')
      ? fs.readdirSync(path.join(NODE_MODULES, parent)).map((p) => path.join(parent, p))
      : [parent];
    for (const p of parents) add(path.join(NODE_MODULES, p, 'node_modules', name));
  }
  return out;
}

// compare reports how the bundled files of pkg relate to the copy of the
// package in node_modules. It prefers a copy with the bundled version.
export function compare(pkg) {
  const copies = installed(pkg.name);
  const copy = copies.find((c) => c.version === pkg.version) ?? copies[0];
  const res = { installed: copy ? copy.version : null, dir: copy ? copy.dir : null, same: [], differ: [], missing: [] };
  if (!copy) return res;
  for (const [file, text] of pkg.files) {
    const p = path.join(copy.dir, file);
    if (!fs.existsSync(p)) res.missing.push(file);
    else if (fs.readFileSync(p, 'utf8') === text) res.same.push(file);
    else res.differ.push(file);
  }
  return res;
}

function find(packages, spec) {
  const hits = [...packages.values()].filter((p) => p.name === spec || `${p.name}@${p.version}` === spec);
  if (hits.length === 0) throw new Error(`the bundle has no package ${spec}`);
  return hits;
}

// extract writes the bundled files of one package and returns the directory.
export function extract(pkg) {
  const dir = path.join(BUNDLED_ROOT, `${pkg.name}@${pkg.version}`);
  for (const [file, text] of pkg.files) {
    const p = path.join(dir, file);
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, text);
  }
  // A package.json makes node load the .js files of the package as it does in
  // node_modules. The source maps do not have the package.json of a package,
  // so the module type comes from the copy in node_modules.
  const copy = installed(pkg.name)[0];
  const type = copy ? JSON.parse(fs.readFileSync(path.join(copy.dir, 'package.json'), 'utf8')).type : undefined;
  const meta = { name: pkg.name, version: pkg.version, private: true };
  if (type) meta.type = type;
  fs.writeFileSync(path.join(dir, 'package.json'), JSON.stringify(meta, null, 2) + '\n');
  return dir;
}

// bundledDir extracts the bundled files of a package and returns the
// directory. A package with two bundled versions needs "name@version".
export function bundledDir(spec) {
  const hits = find(readBundle(), spec);
  if (hits.length > 1) {
    throw new Error(`${spec} has ${hits.length} versions in the bundle: ${hits.map((h) => h.version).join(', ')}`);
  }
  return extract(hits[0]);
}

// bundledImport imports one file of a package at the bundled version.
export async function bundledImport(spec, file) {
  return import(pathToFileURL(path.join(bundledDir(spec), file)).href);
}

function rows(packages) {
  const out = [];
  for (const pkg of [...packages.values()].sort((a, b) => (a.name + a.version).localeCompare(b.name + b.version))) {
    const cmp = compare(pkg);
    out.push({
      name: pkg.name,
      bundled: pkg.version,
      patched: pkg.patched,
      files: pkg.files.size,
      installed: cmp.installed,
      same: cmp.same.length,
      differ: cmp.differ,
      missing: cmp.missing,
      maps: [...pkg.maps].map((m) => (m.includes('parser') ? 'parser' : 'mermaid')).filter((v, i, a) => a.indexOf(v) === i),
    });
  }
  return out;
}

function main(argv) {
  const [cmd, ...args] = argv;
  const packages = readBundle();
  if (cmd === 'list') {
    const pad = (s, n) => String(s).padEnd(n);
    console.log(`${pad('package', 36)}${pad('bundled', 10)}${pad('node_modules', 14)}${pad('files', 7)}${pad('same', 6)}${pad('differ', 8)}missing  note`);
    for (const r of rows(packages)) {
      const notes = [];
      if (r.patched) notes.push('patched by mermaid');
      if (r.installed === null) notes.push('not installed');
      else if (r.installed !== r.bundled) notes.push('VERSION MISMATCH');
      if (r.maps.join() === 'parser') notes.push('parser package');
      console.log(
        `${pad(r.name, 36)}${pad(r.bundled, 10)}${pad(r.installed ?? '-', 14)}${pad(r.files, 7)}${pad(r.same, 6)}${pad(r.differ.length, 8)}${pad(r.missing.length, 9)}${notes.join(', ')}`,
      );
    }
    return;
  }
  if (cmd === 'json') {
    console.log(JSON.stringify(rows(packages), null, 2));
    return;
  }
  if (cmd === 'extract' && args.length > 0) {
    for (const spec of args) {
      for (const pkg of find(packages, spec)) {
        const dir = extract(pkg);
        console.log(`${pkg.name}@${pkg.version}: ${pkg.files.size} files -> ${path.relative(REPO, dir)}`);
      }
    }
    return;
  }
  if (cmd === 'diff' && args.length > 0) {
    for (const spec of args) {
      for (const pkg of find(packages, spec)) {
        const cmp = compare(pkg);
        console.log(`${pkg.name}: bundled ${pkg.version}${pkg.patched ? ' (patched by mermaid)' : ''}, node_modules ${cmp.installed ?? 'not installed'}`);
        for (const f of cmp.same) console.log(`  same     ${f}`);
        for (const f of cmp.differ) console.log(`  DIFFERS  ${f}`);
        for (const f of cmp.missing) console.log(`  missing  ${f}`);
      }
    }
    return;
  }
  console.error('usage: node extract-bundled-source.mjs list | json | extract <package>... | diff <package>...');
  process.exitCode = 2;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2));
}
