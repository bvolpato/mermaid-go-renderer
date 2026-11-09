// Records how V8 in Chromium answers for the parts of JavaScript that the
// cytoscape layouts depend on and that goja implements in its own way:
// Array.prototype.sort with a comparator that is not consistent, number to
// string conversion, the order of object keys, parseInt and parseFloat.
//
//   node gen-cytoscape-engine.mjs
//
// It runs probes/cytoscape-js.js in a bare page (oracle.mjs eval --bare) and
// writes internal/cytoscape/testdata/engine.json.gz. Node is not the oracle
// for the sort: Node 26 has the TimSort of V8 14.8, and Chromium 156 has
// PowerSort.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const outFile = path.join(root, 'internal/cytoscape/testdata/engine.json.gz');
const raw = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-cytoscape-engine-')), 'raw.json');
execFileSync('node', [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'probes/cytoscape-js.js'), '--bare', '--out', raw], { stdio: 'inherit' });
const data = JSON.parse(fs.readFileSync(raw, 'utf8'));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(data), { level: 9 }));
console.error(
  `${path.relative(root, outFile)}: ${data.sort.length} sorts, ${data.numbers.length} numbers, ${data.keys.length} key orders, ${data.parse.length} parses, ${fs.statSync(outFile).size} bytes`
);
