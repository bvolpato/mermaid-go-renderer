#!/usr/bin/env node
// Runs the whole performance measurement and writes docs/PERFORMANCE.md and
// docs/performance.json. The website and the README take their numbers from
// the JSON file, so this script is the way to update them.
//
//   node tools/reference/bench.mjs [--corpus NAME] [--dir DIR] [--restart]
//                                  [--batch 250] [--repeat 7] [--runs 9]
//
// The steps:
//
//   1. mermaid, warm. For each batch of diagrams, one Chromium page stays
//      open and renders each diagram --repeat times (oracle.mjs bench).
//   2. mermaid, first render. Each diagram has a new page (oracle.mjs bench
//      --fresh).
//   3. The command-line tools mmdc and mmdg (bench-cli.mjs), with a mmdg
//      that this script builds from the tree.
//   4. mmdg in a Go process, and the two output files (go run ./tools/perfdoc).
//
// Run it when the machine has no other work: mermaid and mmdg are measured
// one after the other, and other work changes the ratio between them.
//
// The results of the steps are in DIR (default .parity/perf/bench, which
// git ignores). A step that has a result there does not run again, so a run
// that was stopped can continue. --restart measures everything again.
//
// After the script, run tools/update-docs.sh. It writes the numbers into the
// README and makes the charts again.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');

const argv = process.argv.slice(2);
const option = (name, fallback) => {
  const i = argv.indexOf(name);
  return i >= 0 && argv[i + 1] !== undefined ? argv[i + 1] : fallback;
};
const corpusName = option('--corpus', 'mermaid-e2e');
const corpus = path.join(root, 'testdata/corpus', corpusName);
const dir = path.resolve(root, option('--dir', '.parity/perf/bench'));
const batchSize = Number(option('--batch', 250));
const repeat = String(option('--repeat', 7));
const runs = String(option('--runs', 9));
const restart = argv.includes('--restart');

function run(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, stdio: ['ignore', 'inherit', 'inherit'] });
  if (result.status !== 0) {
    process.stderr.write(`bench: ${command} ${args.join(' ')} ended with status ${result.status}\n`);
    process.exit(1);
  }
}

function walk(d, out = []) {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) {
      walk(p, out);
    } else if (e.name.endsWith('.mmd')) {
      out.push(p);
    }
  }
  return out;
}

fs.mkdirSync(dir, { recursive: true });
const files = walk(corpus).sort();
const batches = [];
for (let i = 0; i < files.length; i += batchSize) {
  const name = String(batches.length).padStart(2, '0');
  const list = path.join(dir, `batch-${name}.list`);
  fs.writeFileSync(list, files.slice(i, i + batchSize).join('\n') + '\n');
  batches.push({ name, list });
}
process.stderr.write(`bench: ${files.length} diagrams of ${corpusName} in ${batches.length} batches, results in ${path.relative(root, dir)}\n`);

// Steps 1 and 2. Each batch is one run of the oracle, which takes the lock
// of the browser and releases it.
for (const [mode, extra] of [
  ['warm', ['--repeat', repeat]],
  ['cold', ['--repeat', '1', '--fresh']],
]) {
  for (const batch of batches) {
    const out = path.join(dir, `${mode}-${batch.name}.json`);
    if (!restart && fs.existsSync(out) && fs.statSync(out).size > 0) {
      continue;
    }
    process.stderr.write(`bench: mermaid ${mode}, batch ${batch.name}\n`);
    run(process.execPath, ['oracle.mjs', 'bench', '--base', corpus, '--list', batch.list, '--out', out, '--quiet', ...extra], here);
  }
}

// Step 3.
const cli = path.join(dir, 'cli.json');
if (restart || !fs.existsSync(cli)) {
  const mmdg = path.join(dir, process.platform === 'win32' ? 'mmdg.exe' : 'mmdg');
  run('go', ['build', '-o', mmdg, './cmd/mmdg'], root);
  process.stderr.write('bench: the command-line tools\n');
  run(process.execPath, ['bench-cli.mjs', '--mmdg', mmdg, '--out', cli, '--runs', runs], here);
}

// Step 4.
process.stderr.write('bench: mmdg, and the documents\n');
run(
  'go',
  ['run', './tools/perfdoc', '-corpus', path.relative(root, corpus), '-repeat', repeat, '-warm', path.join(dir, 'warm-*.json'), '-cold', path.join(dir, 'cold-*.json'), '-cli', cli],
  root
);
