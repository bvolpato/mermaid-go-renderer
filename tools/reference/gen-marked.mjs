// Writes the golden files of internal/marked: the token trees of the marked
// lexer that the mermaid 12.2.0 bundle inlines (marked 16.3.0).
//
//   cd tools/reference && node gen-marked-rules.mjs && node gen-marked.mjs
//
// lexer.json.gz holds one case for each input:
//   in | units | file   the input: a string, UTF-16 code units (for a
//                       surrogate that has no partner), or the path of a
//                       corpus fixture whose whole text is the input
//   want                JSON.stringify(marked.lexer(input))
//   sum                 in place of want, to keep the file small: the first
//                       16 hex digits of the SHA-256 of want. The fixtures
//                       have it, and the inputs whose tree is one paragraph
//                       with one text token.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { REPO, marked, versions } from './markdown-lib.mjs';
import { allInputs, corpusFiles, surrogateInputs } from './markdown-inputs.mjs';

if (versions.marked !== '16.3.0') throw new Error(`unexpected marked ${versions.marked}`);

const sum = (s) => crypto.createHash('sha256').update(s, 'utf8').digest('hex').slice(0, 16);
const lex = (input) => JSON.stringify(marked.lexer(input));
const plainTree = (s) =>
  JSON.stringify([
    { type: 'paragraph', raw: s, text: s, tokens: [{ type: 'text', raw: s, text: s, escaped: false }] },
  ]);

const cases = [];
let plain = 0;
for (const input of allInputs(REPO)) {
  const want = lex(input);
  if (want === plainTree(input)) {
    plain++;
    cases.push({ in: input, sum: sum(want) });
  } else {
    cases.push({ in: input, want });
  }
}
for (const input of surrogateInputs()) {
  cases.push({ units: Array.from({ length: input.length }, (_, i) => input.charCodeAt(i)), want: lex(input) });
}
for (const file of corpusFiles(REPO)) {
  cases.push({ file, sum: sum(lex(fs.readFileSync(path.join(REPO, file), 'utf8'))) });
}

const file = path.join(REPO, 'internal/marked/testdata/lexer.json.gz');
const data = zlib.gzipSync(JSON.stringify(cases), { level: 9 });
fs.writeFileSync(file, data);
console.log(`wrote ${file}: ${cases.length} cases (${plain} plain), ${data.length} bytes`);
