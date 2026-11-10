#!/usr/bin/env node
// Renders a corpus with real mermaid-cli (mmdc-oracle.mjs) into
// .parity/ref-mmdc/<platform>/<corpus>: the reference for the mermaid-cli
// page mode (WithPage(PageMermaidCLI), mmdg --page mermaid-cli).
//
//   linux   Chromium in the Playwright image with only the Liberation fonts
//           as system fonts (oracle-linux.mjs --fonts liberation). It is the
//           reference for the embedded font library.
//   macos   Chromium on this machine with its system fonts.
//
// The script renders in batches (100 files by default, at most 150), one
// browser per batch. Each batch takes the oracle lock and releases it, so
// that other callers of the oracle get a turn. It skips a fixture only
// when its source, toolchain, platform, and SVG hash match the result.
// The script waits 3 seconds between batches (--pause MS), so that the
// other callers get the lock. --force renders every fixture again, and --filter TEXT renders only the
// fixtures whose path contains TEXT.
//
// Usage (in the repository root):
//   node tools/reference/render-mmdc-ref.mjs --platform linux|macos [--corpus NAME] [--force] [--filter TEXT] [--batch N] [--max-batches N] [--pause MS]
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import crypto from 'node:crypto';
import { referenceContract } from './mmdc-oracle.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');

const argv = process.argv.slice(2);
const force = argv.includes('--force');
const option = (name, fallback) => {
  const i = argv.indexOf(name);
  return i >= 0 && argv[i + 1] !== undefined ? argv[i + 1] : fallback;
};
const platform = option('--platform', '');
if (platform !== 'linux' && platform !== 'macos') {
  process.stderr.write('usage: render-mmdc-ref.mjs --platform linux|macos [--corpus NAME] [--force] [--filter TEXT] [--batch N] [--max-batches N]\n');
  process.exit(2);
}
if (platform === 'macos' && process.platform !== 'darwin') {
  throw new Error('macos references require a macOS host');
}
const corpusName = option('--corpus', 'mermaid-e2e');
const corpus = `testdata/corpus/${corpusName}`;
const out = `.parity/ref-mmdc/${platform}/${corpusName}`;
const filter = option('--filter', '');
const batchSize = Math.min(150, Math.max(1, Number(option('--batch', 100))));
const maxBatches = Number(option('--max-batches', Infinity));
const pauseMs = Math.max(0, Number(option('--pause', 3000)));
if (!Number.isInteger(batchSize) || !((Number.isInteger(maxBatches) && maxBatches >= 0) || maxBatches === Infinity) || !Number.isFinite(pauseMs)) {
  throw new Error('batch, max-batches, and pause must be valid nonnegative numbers');
}
const workDir = path.join(root, '.parity/ref-mmdc/work');
const contract = await referenceContract();
const sha256 = (data) => crypto.createHash('sha256').update(data).digest('hex');

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
const isPending = (file) => {
  try {
    const result = resultOf(file);
    const meta = JSON.parse(fs.readFileSync(result, 'utf8'));
    if (meta.page !== 'mermaid-cli' || meta.mermaid !== contract.mermaid || meta.mmdc !== contract.mmdc ||
        meta.platform !== (platform === 'macos' ? 'darwin' : 'linux') ||
        !Object.entries(contract).every(([key, value]) => meta.contract?.[key] === value) ||
        meta.sourceSha256 !== sha256(fs.readFileSync(path.join(root, file))) || !meta.browserVersion) {
      return true;
    }
    if (meta.error) {
      return meta.error.startsWith('oracle:');
    }
    return !/^[a-f0-9]{64}$/.test(meta.embedSha256 || '') ||
      sha256(fs.readFileSync(result.replace(/\.json$/, '.svg'))) !== meta.sha256;
  } catch {
    return true;
  }
};
let pending = all.filter((file) => force || isPending(file));
process.stderr.write(`${platform} ${corpusName}: ${all.length} fixtures, ${pending.length} to render\n`);

fs.mkdirSync(workDir, { recursive: true });
// A browser that stops in the middle of a batch leaves the rest of the
// batch without results. The next batch starts with those fixtures. The
// script gives up when two batches in a row render nothing.
let idle = 0;
let batchNumber = 0;
const started = Date.now();
while (pending.length > 0 && batchNumber < maxBatches) {
  const batch = pending.slice(0, batchSize);
  const list = path.join(workDir, `render-mmdc-ref-${process.pid}.list`);
  fs.writeFileSync(list, batch.join('\n') + '\n');
  if (force) {
    for (const file of batch) {
      fs.rmSync(resultOf(file), { force: true });
    }
  }
  const began = Date.now();
  const render = ['render', '--quiet', '--base', corpus, '--out', out, '--list', path.relative(root, list)];
  // The working directory is the repository root, so that the paths in the
  // result files are the same on both platforms.
  const run =
    platform === 'linux'
      ? spawnSync('node', [path.join(here, 'oracle-linux.mjs'), '--fonts', 'liberation', '--script', 'mmdc-oracle.mjs', ...render], { cwd: root, stdio: ['ignore', 'inherit', 'inherit'] })
      : spawnSync('node', [path.join(here, 'mmdc-oracle.mjs'), ...render], { cwd: root, stdio: ['ignore', 'inherit', 'inherit'] });
  fs.rmSync(list, { force: true });
  const done = batch.filter((file) => !isPending(file)).length;
  const rest = new Set(batch.filter(isPending));
  pending = pending.filter((file) => !batch.includes(file) || rest.has(file));
  batchNumber++;
  const seconds = ((Date.now() - began) / 1000).toFixed(0);
  process.stderr.write(`batch ${batchNumber}: ${done} of ${batch.length} fixtures in ${seconds}s, ${pending.length} left\n`);
  if (run.status !== 0) {
    process.stderr.write(`the oracle stopped with status ${run.status}\n`);
    if (run.error || run.signal) {
      process.stderr.write(`${run.error || run.signal}\n`);
    }
  }
  idle = done === 0 ? idle + 1 : 0;
  if (idle >= 2) {
    process.stderr.write(`no result for ${batch[0]} in two runs\n`);
    process.exit(1);
  }
  // Other callers of the oracle look for the lock four times in a second.
  // A pause lets them take it before the next batch does.
  if (pending.length > 0) {
    spawnSync(process.execPath, ['-e', `setTimeout(() => {}, ${pauseMs})`]);
  }
}
process.stderr.write(`${platform} ${corpusName}: ${all.length - pending.length} current results, ${pending.length} pending, ${((Date.now() - started) / 1000).toFixed(1)}s total\n`);
