// Shared code of gen-purify.mjs and gen-sanitize.mjs: it runs purify-page.js
// in Chromium through oracle.mjs, once for a list of jobs.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

export const HERE = path.dirname(fileURLToPath(import.meta.url));
export const ROOT = path.join(HERE, '../..');
const CHUNKS = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.esm');

// The chunks of the mermaid ESM build that the page script loads.
const NAMES = {
  helpers: 'chunk-35ZOQMXA.mjs', // esbuild runtime helpers
  log: 'chunk-OHNBJXQ2.mjs', // logger and d3
  url: 'chunk-445WJIJ4.mjs', // @braintree/sanitize-url
  main: 'chunk-DLUCIBNR.mjs', // config.ts, DOMPurify, common.ts
  utils: 'chunk-PIRVTUE5.mjs', // utils.ts
};

// runJobs runs the jobs in one browser and returns { ua, results }.
export function runJobs(jobs) {
  const chunks = {};
  for (const [key, name] of Object.entries(NAMES)) {
    chunks[key] = fs.readFileSync(path.join(CHUNKS, name), 'utf8');
  }
  if (!/DOMPurify\.version = "3\.4\.12"/.test(chunks.main)) {
    throw new Error('the mermaid chunk does not bundle DOMPurify 3.4.12');
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'purify-oracle-'));
  const argsFile = path.join(dir, 'args.json');
  const outFile = path.join(dir, 'out.json');
  fs.writeFileSync(argsFile, JSON.stringify({ chunks, names: NAMES, jobs }));
  try {
    execFileSync(
      process.execPath,
      [path.join(HERE, 'oracle.mjs'), 'eval', path.join(HERE, 'purify-page.js'), '--args', argsFile, '--out', outFile],
      { cwd: HERE, stdio: ['ignore', 'inherit', 'inherit'], maxBuffer: 1 << 30 }
    );
    return JSON.parse(fs.readFileSync(outFile, 'utf8'));
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// writeGolden writes JSON, gzipped when the name ends in .gz. The gzip header
// has no time stamp, so that the same data gives the same file.
export function writeGolden(file, data) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  const json = JSON.stringify(data);
  if (file.endsWith('.gz')) {
    fs.writeFileSync(file, zlib.gzipSync(json, { level: 9 }));
  } else {
    fs.writeFileSync(file, json + '\n');
  }
  const size = fs.statSync(file).size;
  console.log(`${path.relative(ROOT, file)}: ${(size / 1024).toFixed(1)} kB`);
}

// mulberry32 is the seeded PRNG of the generators.
export function mulberry32(seed) {
  let a = seed >>> 0;
  return function () {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// walk lists the files under dir that end with ext, sorted.
export function walk(dir, ext) {
  const out = [];
  const visit = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) {
        visit(p);
      } else if (e.name.endsWith(ext)) {
        out.push(p);
      }
    }
  };
  visit(dir);
  return out;
}
