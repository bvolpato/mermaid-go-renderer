// Writes the extra diagram texts of gen-swimlanes.mjs as a small corpus
// with reference SVGs, for the parity command:
//
//   node swimlanes-extra-corpus.mjs <dir>
//   go run ./internal/diagrams/flowchart/paritycmd -corpus <dir>/corpus -ref <dir>/ref
//
// The texts and the SVGs that mermaid rendered for them in Chromium are in
// internal/ru/layout/swimlanes/testdata/extra-svg.json.gz.
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const out = process.argv[2];
if (!out) {
  console.error('usage: node swimlanes-extra-corpus.mjs <dir>');
  process.exit(2);
}
const file = path.join(root, 'internal/ru/layout/swimlanes/testdata/extra-svg.json.gz');
const extra = JSON.parse(zlib.gunzipSync(fs.readFileSync(file)).toString('utf8'));
const corpus = path.join(out, 'corpus/swimlanes-extra');
const ref = path.join(out, 'ref/swimlanes-extra');
fs.mkdirSync(corpus, { recursive: true });
fs.mkdirSync(ref, { recursive: true });
let n = 0;
for (const { name, text, svg, diagramType, error } of extra) {
  if (!svg && !error) {
    continue;
  }
  fs.writeFileSync(path.join(corpus, name + '.mmd'), text + '\n');
  if (svg) {
    fs.writeFileSync(path.join(ref, name + '.svg'), svg);
    fs.writeFileSync(path.join(ref, name + '.json'), JSON.stringify({ diagramType, mermaid: '12.2.0' }) + '\n');
  } else {
    // mermaid fails for this text: the parity command expects an error.
    fs.writeFileSync(path.join(ref, name + '.json'), JSON.stringify({ error, mermaid: '12.2.0' }) + '\n');
  }
  n++;
}
console.error(`wrote ${n} fixtures to ${out}`);
