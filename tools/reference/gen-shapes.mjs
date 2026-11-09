// Runs shape cases through upstream mermaid in Chromium and writes a golden
// file for internal/ru/rutest.
//
//   node gen-shapes.mjs <cases.json> <golden.json[.gz]>
//
// The cases file is { cases: [{ name, type, directive, node, dir }] }. See
// probes/shapes.js. The golden file has the cases, each with the result of
// upstream, in the form of shape-golden.mjs.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { writeGolden } from './shape-golden.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const [casesFile, outFile] = process.argv.slice(2);
if (!casesFile || !outFile) {
  console.error('usage: node gen-shapes.mjs <cases.json> <golden.json[.gz]>');
  process.exit(2);
}
const { cases } = JSON.parse(fs.readFileSync(casesFile, 'utf8'));
const tmp = path.join(os.tmpdir(), `mmdg-shapes-${process.pid}.json`);
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/shapes.js'), '--args', casesFile, '--out', tmp], { stdio: 'inherit' });
const results = JSON.parse(fs.readFileSync(tmp, 'utf8'));
fs.rmSync(tmp);
const golden = cases.map((c, i) => ({ ...c, directive: undefined, ...results[i] }));
// Cases share configs and style sheets by index: see shape-golden.mjs.
writeGolden(outFile, golden);
console.error(`${golden.length} cases, ${golden.filter((g) => g.error).length} errors -> ${outFile}`);
