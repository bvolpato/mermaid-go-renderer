#!/usr/bin/env node
// Emits internal/js/testdata/math_sankey.json: golden cases for js.Pow with
// an integer exponent. The d3-sankey port calls it for Math.pow(0.99, i).
//
// Run it in tools/reference:
//
//   node gen-sankey-pow.mjs
//
// Each case has the base, the exponent, the result of Math.pow of this
// Node.js build, and the correctly rounded result. The correctly rounded
// result comes from exact arithmetic with BigInt in this script: the power of
// a double is a fraction of two integers, and the script rounds the fraction
// to the nearest double with ties to even. The Go test requires the result
// of Node.js, and uses the correctly rounded result to count the cases in
// which the pow of the C library is one unit off.
//
// "sankey" has the calls that d3-sankey makes: Math.pow(0.99, i).
import { writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.join(HERE, '../../internal/js/testdata/math_sankey.json');

const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  if (Number.isNaN(v)) return '7ff8000000000000';
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};

// mulberry32, the same generator as gen-jsnum.mjs.
function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// decompose returns [mant, exp] with |x| = mant * 2^exp for a finite x that
// is not zero. mant is a BigInt.
function decompose(x) {
  f64.setFloat64(0, Math.abs(x));
  const u = f64.getBigUint64(0);
  const e = Number(u >> 52n);
  const frac = u & ((1n << 52n) - 1n);
  if (e === 0) return [frac, -1074];
  return [frac | (1n << 52n), e - 1075];
}

const bitLength = (v) => v.toString(2).length;

// roundFraction returns the double that is nearest to num / den (both are
// positive BigInt values), with ties to even.
function roundFraction(num, den) {
  // Choose e so that 2^52 <= num / den / 2^e < 2^53, or e = -1074 for a
  // subnormal result.
  let e = bitLength(num) - bitLength(den) - 53;
  const scaled = (k) => (k >= 0 ? [num, den << BigInt(k)] : [num << BigInt(-k), den]);
  for (;;) {
    const [n, d] = scaled(e);
    const q = n / d;
    if (q >= 1n << 53n) e++;
    else if (q < 1n << 52n) e--;
    else break;
  }
  if (e < -1074) e = -1074;
  const [n, d] = scaled(e);
  let q = n / d;
  const r = n - q * d;
  if (2n * r > d || (2n * r === d && (q & 1n) === 1n)) q++;
  if (q === 1n << 53n) {
    q >>= 1n;
    e++;
  }
  if (e > 971) return Infinity;
  // q * 2^e is a double, so each product below is exact.
  let v = Number(q);
  while (e > 0) {
    const s = Math.min(e, 1000);
    v *= 2 ** s;
    e -= s;
  }
  while (e < 0) {
    const s = Math.min(-e, 1000);
    v /= 2 ** s;
    e += s;
  }
  return v;
}

// powExact is the correctly rounded power of a finite, nonzero x.
function powExact(x, n) {
  const [mant, exp] = decompose(x);
  const an = BigInt(Math.abs(n));
  let num = mant ** an;
  let den = 1n;
  const e = BigInt(exp) * an;
  if (e >= 0n) num <<= e;
  else den <<= -e;
  if (n < 0) [num, den] = [den, num];
  const r = roundFraction(num, den);
  return x < 0 && n % 2 !== 0 ? -r : r;
}

const rnd = prng(0x5a4e);
const cases = [];
const add = (x, n) => {
  // The exact power needs |n| * 53 bits. Skip a power that overflows or
  // underflows by far: the result is then clear.
  const l = Math.abs(n) * Math.log2(Math.abs(x));
  const far = Number.isFinite(l) && Math.abs(l) > 1200;
  const cr = x === 0 || !Number.isFinite(x) || far ? Math.pow(x, n) : powExact(x, n);
  cases.push([bits(x), n, bits(Math.pow(x, n)), bits(cr)]);
};

// Special values.
for (const x of [0, -0, Infinity, -Infinity, NaN, 1, -1, 2, -2, 0.5, -0.5, 5e-324, -5e-324, 1.7976931348623157e308, 2.2250738585072014e-308])
  for (const n of [0, 1, -1, 2, -2, 3, -3, 7, 52, 53, 1023, 1024, 1074, 1075, -1022, -1023, -1074, -1075, 100000, -100001]) add(x, n);
// Powers that are exactly a double, or the middle of two doubles.
for (const x of [3, 5, 7, 9, 1.5, 2.5, 0.75, 10, 0.1, 12345, 1 + 2 ** -20, 1 - 2 ** -27, 94906267, 6074001, 208065])
  for (let n = -40; n <= 60; n++) add(x, n);
// Random bases with small exponents.
for (let i = 0; i < 6000; i++) {
  const k = rnd();
  let x;
  if (k < 0.4) x = rnd() * 2;
  else if (k < 0.6) x = (rnd() - 0.5) * 20;
  else if (k < 0.8) x = Math.round(rnd() * 1000) / 100;
  else if (k < 0.9) x = (rnd() - 0.5) * 2 * 10 ** (rnd() * 40 - 20);
  else x = 1 + (rnd() - 0.5) * 2 ** -Math.floor(rnd() * 30);
  const n = Math.floor((rnd() - 0.4) * (rnd() < 0.8 ? 40 : 600));
  add(x, n);
}
// Results near overflow, underflow and in the subnormal range.
for (let i = 0; i < 600; i++) {
  const x = 1 + rnd() * 3;
  const want = (rnd() < 0.5 ? 1 : -1) * (1000 + rnd() * 90);
  add(x, Math.round(want / Math.log2(x)));
}
// Bases close to one with large exponents.
for (let i = 0; i < 300; i++) {
  const x = 1 + (rnd() - 0.5) * 0.02;
  add(x, Math.floor((rnd() - 0.5) * 20000));
}

// The calls of d3-sankey.
const sankey = [];
for (let i = 0; i < 1000; i++) sankey.push([bits(0.99), i, bits(Math.pow(0.99, i)), bits(powExact(0.99, i))]);

writeFileSync(OUT, JSON.stringify({ node: process.version, v8: process.versions.v8, arch: process.arch, cases, sankey }) + '\n');
console.log(`${path.relative(path.join(HERE, '../..'), OUT)}: ${cases.length} cases, ${sankey.length} sankey calls`);
