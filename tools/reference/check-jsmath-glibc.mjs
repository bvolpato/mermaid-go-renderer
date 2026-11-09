#!/usr/bin/env node
// Compares pow and tanh of the C library of the Linux reference image with
// Chromium on Linux and with the package crmath. It starts no browser.
//
// V8 calls std::pow and std::tanh of the C library for Math.pow and
// Math.tanh. The script checks two things:
//
//   1. The C library of the image (glibc) gives the results of the golden
//      file of Chromium on Linux for the arguments of that file. So the
//      browser calls that library.
//   2. The functions of the package crmath for Linux (linux_libm_pow.go,
//      linux_libm_tanh.go) give the results of that C library for random
//      arguments.
//
// Run it in tools/reference:
//
//   node check-jsmath-glibc.mjs [--n 2000000] [--seed 1] [--golden FILE]
//
// --n is the number of random arguments of each function. The script
// compiles jsmath-glibc.c in the image gcc:14 and runs the program in the
// Playwright image, which has the C library of the reference browser. The
// Go test TestMathEval gives the results of the package crmath. The
// temporary files are in .parity/jsmath, because the containers see only
// the repository.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');

const opts = { n: '2000000', seed: '1', golden: path.join(ROOT, 'internal/js/crmath/testdata/chromium_math_linux.bin.gz') };
for (let i = 2; i < process.argv.length; i += 2) {
  opts[process.argv[i].replace(/^--/, '')] = process.argv[i + 1];
}

const version = JSON.parse(fs.readFileSync(path.join(HERE, 'node_modules/playwright/package.json'), 'utf8')).version;
const image = `mcr.microsoft.com/playwright:v${version}-noble`;
fs.mkdirSync(path.join(ROOT, '.parity/jsmath'), { recursive: true });
const tmp = fs.mkdtempSync(path.join(ROOT, '.parity/jsmath/glibc-'));
const docker = (img, ...cmd) =>
  execFileSync('docker', ['run', '--rm', '-v', `${ROOT}:${ROOT}`, '-w', tmp, img, ...cmd], { stdio: 'inherit' });
const goTest = (env) =>
  execFileSync('go', ['test', './internal/js/crmath', '-run', '^(TestMathWriteInputs|TestMathEval)$', '-count=1'], {
    cwd: ROOT,
    env: { ...process.env, ...env },
    stdio: ['ignore', 'ignore', 'inherit'],
  });

// The results of the golden file. The format is in
// internal/js/crmath/golden_test.go.
const golden = new Map();
{
  const data = zlib.gunzipSync(fs.readFileSync(opts.golden));
  let p = 8;
  const userAgent = data.subarray(p + 2, p + 2 + data.readUInt16LE(p)).toString('utf8');
  if (!userAgent.includes('Linux')) {
    throw new Error(`${opts.golden} is not from Chromium on Linux: ${userAgent}`);
  }
  p += 2 + data.readUInt16LE(p);
  const count = data.readUInt32LE(p);
  p += 4;
  for (let i = 0; i < count; i++) {
    const name = data.subarray(p + 1, p + 1 + data[p]).toString('latin1');
    p += 1 + data[p];
    const cases = data.readUInt32LE(p + 1);
    p += 13;
    const out = Buffer.alloc(cases * 8);
    for (let k = 0; k < 8; k++) {
      for (let j = 0; j < cases; j++) {
        out[j * 8 + k] = data[p + k * cases + j];
      }
    }
    p += cases * 8;
    golden.set(name, { cases, out });
  }
}

// The arguments of the golden file, from the Go test.
const inputsFile = path.join(tmp, 'inputs.bin');
goTest({ JSMATH_INPUTS: inputsFile, JSMATH_N: String(golden.get('pow').cases), JSMATH_N_EXACT: '1' });
{
  const data = fs.readFileSync(inputsFile);
  let p = 8;
  const count = data.readUInt32LE(p);
  p += 4;
  for (let i = 0; i < count; i++) {
    const name = data.subarray(p + 1, p + 1 + data[p]).toString('latin1');
    p += 1 + data[p];
    const size = data.readUInt32LE(p + 1) * data[p] * 8;
    p += 13;
    if (name === 'pow' || name === 'tanh') {
      fs.writeFileSync(path.join(tmp, `golden-${name}.in`), data.subarray(p, p + size));
    }
    p += size;
  }
}

docker('gcc:14', 'gcc', '-O1', '-fno-builtin', '-o', 'jsmath-glibc', path.join(HERE, 'jsmath-glibc.c'), '-lm');
docker(
  image,
  'sh',
  '-c',
  './jsmath-glibc eval pow golden-pow.in golden-pow.out && ./jsmath-glibc eval tanh golden-tanh.in golden-tanh.out && ' +
    `./jsmath-glibc gen pow ${Number(opts.n)} ${Number(opts.seed)} random-pow.in random-pow.out && ` +
    `./jsmath-glibc gen tanh ${Number(opts.n)} ${Number(opts.seed) + 1} random-tanh.in random-tanh.out`
);

// differ counts the float64 results that are not the same in two buffers.
const differ = (a, b) => {
  if (a.length !== b.length) {
    throw new Error(`${a.length} and ${b.length} bytes of results`);
  }
  let n = 0;
  for (let i = 0; i < a.length; i += 8) {
    if (a.readBigUInt64LE(i) !== b.readBigUInt64LE(i)) {
      n++;
    }
  }
  return n;
};

let failed = false;
for (const name of ['pow', 'tanh']) {
  const g = golden.get(name);
  const bad = differ(fs.readFileSync(path.join(tmp, `golden-${name}.out`)), g.out);
  failed ||= bad !== 0;
  process.stdout.write(`${name}: the C library differs from Chromium on Linux for ${bad} of ${g.cases} arguments of the golden file\n`);
}
for (const name of ['pow', 'tanh']) {
  const goOut = path.join(tmp, `go-${name}.out`);
  goTest({
    MMDG_MATH: 'linux',
    JSMATH_EVAL: name === 'pow' ? 'pow.libm' : 'tanh',
    JSMATH_EVAL_IN: path.join(tmp, `random-${name}.in`),
    JSMATH_EVAL_OUT: goOut,
  });
  const bad = differ(fs.readFileSync(goOut), fs.readFileSync(path.join(tmp, `random-${name}.out`)));
  failed ||= bad !== 0;
  process.stdout.write(`${name}: the package crmath differs from the C library for ${bad} of ${Number(opts.n)} random arguments\n`);
}
fs.rmSync(tmp, { recursive: true, force: true });
process.exit(failed ? 1 : 0);
