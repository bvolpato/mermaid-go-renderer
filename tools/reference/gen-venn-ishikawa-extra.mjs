// Writes internal/diagrams/venn/testdata/extra-ref.json.gz and
// internal/diagrams/ishikawa/testdata/extra-ref.json.gz: the SVG that real
// mermaid renders in Chromium for the fixtures in testdata/extra of the two
// packages, or the message of the error when mermaid does not render a
// fixture. These fixtures cover config options and branches of the renderers
// that the corpus does not have: for the venn diagram four and more sets
// (the random restarts of the layout), disjoint and contained sets, sets of
// size zero, the debug layout, the size options, every style property with
// both looks, the themes, and inputs that make venn.js throw; for the
// ishikawa diagram the hand-drawn look with and without a seed, line breaks,
// long labels, the font size, the padding, unbalanced trees and deep trees.
//
// The script runs venn-ishikawa-render-page.js with the eval command of the
// browser oracle, one time for both diagram types.
//
// Usage (in tools/reference): node gen-venn-ishikawa-extra.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const names = ['venn', 'ishikawa'];

const fixtures = [];
for (const name of names) {
  const dir = path.join(repoRoot, 'internal/diagrams', name, 'testdata/extra');
  for (const f of fs.readdirSync(dir).filter((f) => f.endsWith('.mmd')).sort()) {
    fixtures.push({ name: `${name}/${path.basename(f, '.mmd')}`, text: fs.readFileSync(path.join(dir, f), 'utf8') });
  }
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'venn-ishikawa-extra-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ fixtures }));
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'venn-ishikawa-render-page.js'), '--args', argsFile, '--out', resultFile],
  { stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

for (const name of names) {
  const svgs = {};
  const errors = {};
  const randomCalls = {};
  for (const r of results.filter((r) => r.name.startsWith(name + '/'))) {
    const base = r.name.slice(name.length + 1);
    if (r.error !== undefined) {
      errors[base] = r.error;
    } else {
      if (r.diagramType !== name) {
        throw new Error(`${r.name}: mermaid detects the type ${r.diagramType}`);
      }
      svgs[base] = r.svg;
      randomCalls[base] = r.randomCalls;
    }
  }
  const data = JSON.stringify({ mermaid: '12.2.0', svgs, errors, randomCalls }, null, 1) + '\n';
  const outFile = path.join(repoRoot, 'internal/diagrams', name, 'testdata/extra-ref.json.gz');
  fs.writeFileSync(outFile, zlib.gzipSync(data, { level: 9 }));
  console.error(
    `${path.relative(repoRoot, outFile)}: ${Object.keys(svgs).length} fixtures render, ${Object.keys(errors).length} fail`
  );
}
