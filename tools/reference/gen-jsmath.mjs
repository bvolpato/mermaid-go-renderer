#!/usr/bin/env node
// Emits the golden files of the Math functions: the results of headless
// Chromium for the arguments of internal/js/crmath/gen_test.go.
//
// Run it in tools/reference:
//
//   node gen-jsmath.mjs [--platform darwin|linux] [--n 20000] [--n-exact 5000] [--out FILE] [--chunk-mb 24]
//
// --platform darwin (the default) runs Chromium of this machine with
// oracle.mjs. Run it on macOS on arm64. It writes
// internal/js/crmath/testdata/chromium_math.bin.gz.
//
// --platform linux runs Chromium in the Playwright Linux image with
// oracle-linux.mjs. It writes
// internal/js/crmath/testdata/chromium_math_linux.bin.gz. The temporary
// files are in .parity/jsmath, because the container sees only the
// repository.
//
// The script takes the arguments from the Go test TestMathWriteInputs,
// evaluates them in Chromium with jsmath-page.js, and writes the golden
// file. The Go tests TestMathChromium and TestMathChromiumLinux read the
// files. The format is in internal/js/crmath/golden_test.go. --n is the
// number of cases of a transcendental function and --n-exact that of the
// other functions. The script starts the oracle once for each chunk of
// arguments.
//
// A larger file for a validation outside of the repository:
//
//   node gen-jsmath.mjs --n 220000 --n-exact 220000 --chunk-mb 200 --out /tmp/chromium_math_large.bin.gz
//   JSMATH_GOLDEN=/tmp/chromium_math_large.bin.gz go test ./internal/js/crmath -run TestMathChromium
//
//   node gen-jsmath.mjs --platform linux --n 220000 --n-exact 220000 --out ../../.parity/jsmath/chromium_math_linux_large.bin.gz
//   JSMATH_GOLDEN_LINUX=$PWD/.parity/jsmath/chromium_math_linux_large.bin.gz go test ./internal/js/crmath -run TestMathChromiumLinux
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');

const opts = { platform: 'darwin', n: '20000', 'n-exact': '5000', out: '', 'chunk-mb': '24' };
for (let i = 2; i < process.argv.length; i += 2) {
  opts[process.argv[i].replace(/^--/, '')] = process.argv[i + 1];
}
if (opts.platform !== 'darwin' && opts.platform !== 'linux') {
  throw new Error('--platform must be darwin or linux');
}
const linux = opts.platform === 'linux';
if (!opts.out) {
  opts.out = path.join(ROOT, 'internal/js/crmath/testdata', linux ? 'chromium_math_linux.bin.gz' : 'chromium_math.bin.gz');
}
opts.out = path.resolve(opts.out);

// The container of oracle-linux.mjs sees only the repository.
const tmpParent = linux ? path.join(ROOT, '.parity/jsmath') : os.tmpdir();
fs.mkdirSync(tmpParent, { recursive: true });
const tmp = fs.mkdtempSync(path.join(tmpParent, 'jsmath-'));
const inputsFile = path.join(tmp, 'inputs.bin');
execFileSync('go', ['test', './internal/js/crmath', '-run', '^TestMathWriteInputs$', '-count=1'], {
  cwd: ROOT,
  env: { ...process.env, JSMATH_INPUTS: inputsFile, JSMATH_N: opts.n, JSMATH_N_EXACT: opts['n-exact'] },
  stdio: 'inherit',
});

// The file of the Go test: the magic "JSMATHIN", the number of functions,
// and for each function the name, the arity, the number of cases, the hash
// of the arguments and the arguments.
const data = fs.readFileSync(inputsFile);
if (data.subarray(0, 8).toString('latin1') !== 'JSMATHIN') {
  throw new Error('bad inputs file');
}
let p = 8;
const count = data.readUInt32LE(p);
p += 4;
const funcs = [];
for (let i = 0; i < count; i++) {
  const nameLen = data[p];
  const name = data.subarray(p + 1, p + 1 + nameLen).toString('latin1');
  p += 1 + nameLen;
  const arity = data[p];
  const cases = data.readUInt32LE(p + 1);
  const hash = data.subarray(p + 5, p + 13);
  p += 13;
  const size = cases * arity * 8;
  funcs.push({ name, arity, cases, hash, inputs: data.subarray(p, p + size) });
  p += size;
}

// Evaluate the functions in chunks, so that one message to the browser stays
// below the chunk size.
const chunkBytes = Number(opts['chunk-mb']) * 1024 * 1024;
const results = new Map();
let userAgent = '';
let batch = [];
let batchSize = 0;
let run = 0;
const flush = () => {
  if (batch.length === 0) {
    return;
  }
  const argsFile = path.join(tmp, `args-${run}.json`);
  const outFile = path.join(tmp, `result-${run}.json`);
  run++;
  fs.writeFileSync(
    argsFile,
    JSON.stringify({ funcs: batch.map((f) => ({ name: f.name, arity: f.arity, inputs: f.inputs.toString('base64') })) })
  );
  const oracle = path.join(HERE, linux ? 'oracle-linux.mjs' : 'oracle.mjs');
  execFileSync('node', [oracle, 'eval', path.join(HERE, 'jsmath-page.js'), '--bare', '--args', argsFile, '--out', outFile], {
    cwd: HERE,
    stdio: 'inherit',
    maxBuffer: 1 << 30,
  });
  const result = JSON.parse(fs.readFileSync(outFile, 'utf8'));
  userAgent = result.userAgent;
  // The golden file of a platform must come from the browser of that
  // platform: pow and tanh differ.
  if (!userAgent.includes(linux ? 'Linux' : 'Macintosh')) {
    throw new Error(`--platform ${opts.platform} needs Chromium on ${linux ? 'Linux' : 'macOS'}, the browser is ${userAgent}`);
  }
  for (const f of result.funcs) {
    results.set(f.name, Buffer.from(f.outputs, 'base64'));
  }
  process.stderr.write(`oracle run ${run}: ${batch.map((f) => f.name).join(' ')}\n`);
  batch = [];
  batchSize = 0;
};
for (const f of funcs) {
  if (batchSize > 0 && batchSize + f.inputs.length > chunkBytes) {
    flush();
  }
  batch.push(f);
  batchSize += f.inputs.length;
}
flush();

const parts = [Buffer.from('JSMATHGD', 'latin1')];
const ua = Buffer.from(userAgent, 'utf8');
const head = Buffer.alloc(2);
head.writeUInt16LE(ua.length);
parts.push(head, ua);
const n = Buffer.alloc(4);
n.writeUInt32LE(funcs.length);
parts.push(n);
for (const f of funcs) {
  const out = results.get(f.name);
  if (!out || out.length !== f.cases * 8) {
    throw new Error(`no results for ${f.name}`);
  }
  const h = Buffer.alloc(1 + f.name.length + 1 + 4);
  h[0] = f.name.length;
  h.write(f.name, 1, 'latin1');
  h[1 + f.name.length] = f.arity;
  h.writeUInt32LE(f.cases, 2 + f.name.length);
  parts.push(h, f.hash);
  // 8 planes: plane k has byte k of every result. The planes of the sign and
  // the exponent compress well.
  const planes = Buffer.alloc(f.cases * 8);
  for (let k = 0; k < 8; k++) {
    for (let j = 0; j < f.cases; j++) {
      planes[k * f.cases + j] = out[j * 8 + k];
    }
  }
  parts.push(planes);
}
const gz = zlib.gzipSync(Buffer.concat(parts), { level: 9 });
fs.mkdirSync(path.dirname(opts.out), { recursive: true });
fs.writeFileSync(opts.out, gz);
fs.rmSync(tmp, { recursive: true, force: true });
process.stderr.write(`${opts.out}: ${funcs.length} functions, ${gz.length} bytes, ${userAgent}\n`);
