#!/usr/bin/env node
// Rewrites fixtures of the corpus as the text that the test page of mermaid
// gives to mermaid.
//
// The fixtures of testdata/corpus/mermaid-e2e are the .mmd files of the
// end-to-end tests of mermaid. A test does not call mermaid.render() with
// the text of its file. Its page (e2e/platform/viewer.js) puts the text
// into a div as HTML and calls mermaid.run(), which reads the HTML of the
// div back, decodes the HTML entities and removes the indentation
// (packages/mermaid/src/mermaid.ts, runThrowsErrors):
//
//   txt = element.innerHTML;
//   txt = dedent(utils.entityDecode(txt)).trim().replace(/<br\s*\/?>/gi, '<br/>');
//
// The reference oracle calls mermaid.render() with the text of the file. So
// a fixture that needs these steps fails in the oracle, and renders in the
// test of mermaid. Two kinds of fixtures need them: a class or usecase
// diagram that writes <<interface>> as &lt;&lt;interface&gt;&gt;, and a
// diagram whose front matter is indented.
//
// This script runs the same steps in Chromium, with the functions of the
// mermaid build (entityDecode and dedent), and writes the result into the
// fixture file. Then the oracle and the Go renderer get the text that
// mermaid gets in its own test.
//
// Usage (in tools/reference):
//   node gen-run-text.mjs [--write] file.mmd...     the files that are named
//   node gen-run-text.mjs [--write] --failing DIR   the fixtures of ../../testdata/corpus/mermaid-e2e
//                                                   that have an error in the reference directory DIR
//
// Without --write the script only prints which files would change.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const corpus = path.resolve(here, '../../testdata/corpus/mermaid-e2e');

const argv = process.argv.slice(2);
const write = argv.includes('--write');
let files = argv.filter((a, i) => !a.startsWith('--') && argv[i - 1] !== '--failing');
const failing = argv.indexOf('--failing');
if (failing >= 0) {
  const ref = path.resolve(argv[failing + 1]);
  const walk = (dir) => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        walk(p);
      } else if (e.name.endsWith('.json') && JSON.parse(fs.readFileSync(p, 'utf8')).error) {
        files.push(path.join(corpus, path.relative(ref, p).replace(/\.json$/, '.mmd')));
      }
    }
  };
  walk(ref);
}
files = [...new Set(files.map((f) => path.resolve(f)))].sort();
if (files.length === 0) {
  process.stderr.write('usage: gen-run-text.mjs [--write] file.mmd... | --failing REFERENCE_DIR\n');
  process.exit(2);
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-run-text-'));
const script = path.join(tmp, 'run-text.js');
const argsFile = path.join(tmp, 'args.json');
const outFile = path.join(tmp, 'out.json');
fs.writeFileSync(
  script,
  `const { entityDecode, dedent } = await internals('entityDecode', 'dedent');
const out = [];
for (const code of args.codes) {
  // e2e/platform/viewer.js
  const div = document.createElement('div');
  div.className = 'mermaid';
  div.innerHTML = code;
  document.body.appendChild(div);
  // packages/mermaid/src/mermaid.ts, runThrowsErrors
  let txt = div.innerHTML;
  txt = dedent(entityDecode(txt))
    .trim()
    .replace(/<br\\s*\\/?>/gi, '<br/>');
  div.remove();
  out.push(txt);
}
return out;
`
);
fs.writeFileSync(argsFile, JSON.stringify({ codes: files.map((f) => fs.readFileSync(f, 'utf8')) }));
execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'esm-eval', script, '--args', argsFile, '--out', outFile], { stdio: 'inherit' });
const texts = JSON.parse(fs.readFileSync(outFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

let changed = 0;
files.forEach((file, i) => {
  const next = texts[i] + '\n';
  if (next === fs.readFileSync(file, 'utf8')) {
    return;
  }
  changed++;
  process.stderr.write(`${write ? 'wrote' : 'would change'} ${path.relative(corpus, file)}\n`);
  if (write) {
    fs.writeFileSync(file, next);
  }
});
process.stderr.write(`${changed} of ${files.length} files ${write ? 'changed' : 'would change'}\n`);
