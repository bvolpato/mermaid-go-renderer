#!/usr/bin/env node
// Emits internal/rough/testdata/fdlibm.json: results of the functions of
// rough-fdlibm.mjs. The Go test of internal/rough has the same functions in
// Go (fdlibm_test.go), and this file checks that the two versions agree.
//
// Run it in tools/reference:
//
//   node gen-rough-fdlibm.mjs           write the golden file
//   node gen-rough-fdlibm.mjs --eval    read lines "fn hexbits" on stdin, print the result bits
//
// The native Math functions of Node.js are not the reference, because a
// build of Node.js can contract operations into fused multiply-adds. The
// script counts the arguments for which the native function differs and
// stores the counts in the meta data of the golden file.
//
// The inputs come from a seeded PRNG with exact arithmetic, so the Go test
// makes the same inputs. The golden file holds one SHA-256 for each block of
// inputs.
import { createHash } from 'node:crypto';
import { writeFileSync, mkdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as fdlibm from './rough-fdlibm.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT_DIR = path.join(HERE, '../../internal/rough/testdata');

const FUNCS = { sin: fdlibm.sin, cos: fdlibm.cos, tan: fdlibm.tan, atan: fdlibm.atan, asin: fdlibm.asin };
const NATIVE = { sin: Math.sin, cos: Math.cos, tan: Math.tan, atan: Math.atan, asin: Math.asin };
const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  if (Number.isNaN(v)) return '7ff8000000000000';
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};
const fromBits = (h) => {
  f64.setBigUint64(0, BigInt('0x' + h));
  return f64.getFloat64(0);
};

if (process.argv[2] === '--eval') {
  for (const line of readFileSync(0, 'utf8').split('\n')) {
    const [fn, h] = line.trim().split(/\s+/);
    if (FUNCS[fn]) console.log(fn, h, bits(FUNCS[fn](fromBits(h))), bits(NATIVE[fn](fromBits(h))));
  }
  process.exitCode = 0;
  await new Promise((resolve) => process.stdout.write('', resolve));
  process.exit();
}

let state = 0x76386d61;
function u32() {
  state = (state + 0x6d2b79f5) >>> 0;
  let t = state;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return (t ^ (t >>> 14)) >>> 0;
}
// input returns a number in [-2^expo, 2^expo) with 53 random bits. Every
// operation is exact.
function input(expo) {
  const hi = u32() >>> 11;
  const lo = u32();
  const m = hi * 4294967296 + lo;
  return (m - 4503599627370496) * 2 ** (expo - 52);
}

const BLOCK = 500;
const BLOCKS = 20;
const EXPONENTS = {
  sin: [-30, -8, -2, -1, 0, 1, 2, 3, 4, 6, 10, 19, 21, 30, 52, 100, 300, 1023],
  cos: [-30, -8, -2, -1, 0, 1, 2, 3, 4, 6, 10, 19, 21, 30, 52, 100, 300, 1023],
  tan: [-30, -8, -2, -1, 0, 1, 2, 3, 4, 6, 10, 19, 21, 30, 52, 100, 300, 1023],
  atan: [-30, -5, -2, -1, 0, 1, 2, 5, 20, 67, 300],
  asin: [-30, -5, -2, -1, 0, 1],
};
const random = [];
// nativeDiffers counts the arguments for which the native function of this
// Node.js build differs from the reference.
const nativeDiffers = {};
const count = {};
for (const [fn, exps] of Object.entries(EXPONENTS)) {
  nativeDiffers[fn] = 0;
  count[fn] = 0;
  for (const expo of exps) {
    const hashes = [];
    for (let b = 0; b < BLOCKS; b++) {
      const h = createHash('sha256');
      for (let i = 0; i < BLOCK; i++) {
        const x = input(expo);
        const y = bits(FUNCS[fn](x));
        h.update(y);
        count[fn]++;
        if (y !== bits(NATIVE[fn](x))) nativeDiffers[fn]++;
      }
      hashes.push(h.digest('hex').slice(0, 16));
    }
    random.push({ fn, expo, hashes });
  }
}
// asin of k / 1e9: the arguments that path-data-parser makes with toFixed(9).
const fixed = [];
for (let b = 0; b < 40; b++) {
  const h = createHash('sha256');
  for (let i = 0; i < BLOCK; i++) {
    const k = (u32() % 2000000001) - 1000000000;
    h.update(bits(fdlibm.asin(k / 1000000000)));
  }
  fixed.push(h.digest('hex').slice(0, 16));
}
const SPECIAL = [0, -0, 1, -1, 0.5, -0.5, Infinity, -Infinity, NaN, 5e-324, -5e-324, 2.2250738585072014e-308, 1e-300, 7.450580596923828e-9, 3.725290298461914e-9, 1.4901161193847656e-8, Math.PI, -Math.PI, Math.PI / 2, -Math.PI / 2, Math.PI / 4, 3 * Math.PI / 4, 2 * Math.PI, Math.PI * 1e5, 0.7853981633974483, 0.7853981633974484, 0.7853981633974482, 2.356194490192345, 1.5707963267948966, 1.5707963267948968, 1.5707963267948963, 0.6744, 0.67434, 0.3, 0.2999999, 0.78125, 0.4375, 0.6875, 1.1875, 2.4375, 0.975, 0.9750000001, 0.999999999, 1e22, 1e300, 1.7976931348623157e308, 7.3786976294838206e19, 7.378697629483821e19, 823549.6645826427, 823549.6645826428, 1647099.3291652855, 1e6, 123456789, 9007199254740992, 102.10176124166829, 6.123233995736766e-17, 4.71238898038469, 100, 1000, 3, 4, 5, 6, 7, 22, 355, 0.1, 0.2, 1.5, 2.5, 10];
const special = [];
for (const fn of Object.keys(FUNCS)) for (const x of SPECIAL) special.push([fn, bits(x), bits(FUNCS[fn](x))]);

mkdirSync(OUT_DIR, { recursive: true });
const golden = { meta: { reference: 'rough-fdlibm.mjs (v8/src/base/ieee754.cc 14.6.202 without contraction)', node: process.versions.node, v8: process.versions.v8, platform: `${process.platform}/${process.arch}`, block: BLOCK, seed: 0x76386d61, count, nativeDiffers }, random, fixed, special };
writeFileSync(path.join(OUT_DIR, 'fdlibm.json'), JSON.stringify(golden) + '\n');
console.error(`${random.length * BLOCKS * BLOCK + fixed.length * BLOCK} random inputs, ${special.length} special inputs`);
console.error('arguments for which the native Math function of this Node.js differs from the reference:', JSON.stringify(nativeDiffers), 'of', JSON.stringify(count));
