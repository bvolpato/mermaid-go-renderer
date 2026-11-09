#!/usr/bin/env node
// Renders the corpus with upstream mermaid in Chromium on Linux, with only
// the Liberation fonts installed, into .parity/ref-linux/mermaid-e2e. That
// directory has the layout of .parity/ref/mermaid-e2e (the macOS
// reference), and it is the reference for the embedded font library:
//
//   go run ./cmd/parity -ref .parity/ref-linux/mermaid-e2e -fonts embedded
//
// The script renders in batches (60 files by default, at most 150), one
// browser per batch, through oracle-linux.mjs, which takes the oracle lock
// for each batch. It skips a fixture that already has a result, so a run
// that was stopped can be started again. --force renders every fixture
// again, and --filter TEXT renders only the fixtures whose path contains
// TEXT.
//
// Usage (in the repository root): node tools/reference/render-linux-ref.mjs [--corpus NAME] [--force] [--filter TEXT] [--batch N]
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const workDir = path.join(root, '.parity/linux/work');

const argv = process.argv.slice(2);
const force = argv.includes('--force');
const option = (name, fallback) => {
  const i = argv.indexOf(name);
  return i >= 0 && argv[i + 1] !== undefined ? argv[i + 1] : fallback;
};
// --corpus NAME selects another directory of testdata/corpus, for example
// mermaid-docs. The reference goes to .parity/ref-linux/NAME.
const corpusName = option('--corpus', 'mermaid-e2e');
const corpus = `testdata/corpus/${corpusName}`;
const out = `.parity/ref-linux/${corpusName}`;
const filter = option('--filter', '');
const batchSize = Math.min(150, Math.max(1, Number(option('--batch', 60))));

function walk(dir, files) {
  for (const entry of fs.readdirSync(path.join(root, dir), { withFileTypes: true })) {
    const rel = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(rel, files);
    } else if (entry.name.endsWith('.mmd')) {
      files.push(rel);
    }
  }
  return files;
}

const all = walk(corpus, []).sort().filter((file) => file.includes(filter));
const resultOf = (file) => path.join(root, out, path.relative(corpus, file).replace(/\.mmd$/, '.json'));
const isPending = (file) => !fs.existsSync(resultOf(file));
let pending = all.filter((file) => force || isPending(file));
process.stderr.write(`${all.length} fixtures, ${pending.length} to render\n`);

fs.mkdirSync(workDir, { recursive: true });
// A browser that stops in the middle of a batch leaves the rest of the
// batch without results. The next batch starts with those fixtures. The
// script gives up when two batches in a row render nothing.
let idle = 0;
let batchNumber = 0;
while (pending.length > 0) {
  const batch = pending.slice(0, batchSize);
  const list = path.join(workDir, `render-linux-ref-${process.pid}.list`);
  fs.writeFileSync(list, batch.join('\n') + '\n');
  if (force) {
    for (const file of batch) {
      fs.rmSync(resultOf(file), { force: true });
    }
  }
  const began = Date.now();
  // The working directory is the repository root, so that the paths in the
  // result files are the paths of the macOS reference.
  const run = spawnSync(
    'node',
    [path.join(here, 'oracle-linux.mjs'), '--fonts', 'liberation', 'render', '--quiet', '--base', corpus, '--out', out, '--list', path.relative(root, list)],
    { cwd: root, stdio: 'inherit' }
  );
  fs.rmSync(list, { force: true });
  const done = batch.filter((file) => !isPending(file)).length;
  const rest = new Set(batch.filter(isPending));
  pending = pending.filter((file) => !batch.includes(file) || rest.has(file));
  batchNumber++;
  const seconds = ((Date.now() - began) / 1000).toFixed(0);
  process.stderr.write(`batch ${batchNumber}: ${done} of ${batch.length} fixtures in ${seconds}s, ${pending.length} left\n`);
  if (run.status !== 0) {
    process.stderr.write(`oracle-linux.mjs stopped with status ${run.status}\n`);
  }
  idle = done === 0 ? idle + 1 : 0;
  if (idle >= 2) {
    process.stderr.write(`no result for ${batch[0]} in two runs\n`);
    process.exit(1);
  }
}
