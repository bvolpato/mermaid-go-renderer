#!/usr/bin/env node
// Emits internal/js/testdata/math_rough.json: golden cases for the correctly
// rounded functions of internal/js/math_rough.go.
//
// Run it in tools/reference:
//
//   node gen-rough-crmath.mjs
//
// The file has two parts.
//
// "chromium": the stadium nodes of the reference SVGs (.parity/ref), which
// mermaid 12.2.0 made in headless Chromium. mermaid computes the 100 arc
// points of a stadium with Math.cos and Math.sin (generateCirclePoints in
// shapes/util.ts), and the points are in the SVG with all their digits. So
// these are results of Math.cos and Math.sin of the reference browser.
//
// "bc": results of sin, cos, tan, atan and asin from the calculator bc with
// 80 decimal places, rounded to the nearest double. bc is an independent
// program with arbitrary precision. Node.js is not the oracle here: its Math
// functions are the fdlibm functions, which are not correctly rounded.
import { execFileSync } from 'node:child_process';
import { readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REF = path.join(HERE, '../../.parity/ref');
const OUT = path.join(HERE, '../../internal/js/testdata/math_rough.json');

const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  if (Number.isNaN(v)) return '7ff8000000000000';
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};

// ---------------------------------------------------------------------------
// Part 1: the stadium nodes of the reference SVGs.
function* walk(dir) {
  for (const e of readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.svg')) yield p;
  }
}
const chromium = [];
const seen = new Set();
let stadiums = 0;
for (const file of walk(REF)) {
  const html = readFileSync(file, 'utf8');
  for (const m of html.matchAll(/<path d="(M[^"]*)"/g)) {
    const d = m[1];
    const segs = d.split(' C');
    if (segs.length !== 104 || d.indexOf('M', 1) >= 0) continue;
    const start = segs[0].slice(1).trim().split(' ');
    const ends = segs.slice(1).map((c) => c.split(', ')[2].trim().split(' '));
    const cx1 = start[0];
    const cx2 = ends[0][0];
    if (!(Math.abs(Number(cx1) + Number(cx2)) < 1e-9)) continue;
    stadiums++;
    // The radius is h / 2, and the first point has y = -h / 2.
    const radius = String(-Number(start[1]));
    const key = `${cx1} ${radius}`;
    if (seen.has(key) || chromium.length >= 8) continue;
    seen.add(key);
    // Points 1 to 50 are the first arc. Point 51 is a corner. Points 52 to
    // 101 are the second arc.
    chromium.push({ source: path.relative(REF, file), centerX1: cx1, centerX2: cx2, radius, arc1: ends.slice(1, 51), arc2: ends.slice(52, 102) });
  }
}

// ---------------------------------------------------------------------------
// Part 2: bc.
let state = 0x63726d61;
function u32() {
  state = (state + 0x6d2b79f5) >>> 0;
  let t = state;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return (t ^ (t >>> 14)) >>> 0;
}
const rnd = () => (u32() * 2 ** 21 + (u32() >>> 11)) / 2 ** 53;
const between = (a, b) => a + rnd() * (b - a);
// neighbor returns the double that is n steps away from x in magnitude.
const neighbor = (x, n) => {
  f64.setFloat64(0, x);
  f64.setBigUint64(0, f64.getBigUint64(0) + BigInt(n));
  return f64.getFloat64(0);
};

// exactDecimal is the exact decimal expansion of a finite double.
function exactDecimal(x) {
  if (x === 0) return '0';
  f64.setFloat64(0, Math.abs(x));
  const b = f64.getBigUint64(0);
  const expBits = Number(b >> 52n);
  let mant = b & ((1n << 52n) - 1n);
  let exp;
  if (expBits === 0) {
    exp = -1074;
  } else {
    mant |= 1n << 52n;
    exp = expBits - 1075;
  }
  let s;
  if (exp >= 0) {
    s = (mant << BigInt(exp)).toString();
  } else {
    // mant / 2^k = mant * 5^k / 10^k
    const k = -exp;
    const digits = (mant * 5n ** BigInt(k)).toString().padStart(k + 1, '0');
    s = digits.slice(0, digits.length - k) + '.' + digits.slice(digits.length - k);
  }
  return (x < 0 ? '-' : '') + s;
}

const inputs = {
  sin: [],
  cos: [],
  tan: [],
  atan: [],
  asin: [],
};
for (const fn of ['sin', 'cos', 'tan']) {
  const list = inputs[fn];
  for (let i = 0; i < 120; i++) list.push(between(-10, 10));
  for (let i = 0; i < 60; i++) list.push(between(-1e4, 1e4));
  for (let i = 0; i < 40; i++) list.push(between(-1e-3, 1e-3));
  for (let i = 0; i < 30; i++) list.push(between(-1e6, 1e6));
  // Doubles next to a multiple of pi/2: the reduction cancels many bits.
  for (let k = -12; k <= 12; k++) {
    if (k === 0) continue;
    const base = k * (Math.PI / 2);
    for (const n of [-2, -1, 0, 1, 2]) list.push(neighbor(base, n));
  }
  list.push(1, -1, 0.5, 2, 3, 4, 5, 6, 7, 22, 355, 1e5, 4.584160708809596, 0.7853981633974483, 1.5707963267948966, 3.141592653589793, 6.283185307179586, 102.10176124166829);
}
for (let i = 0; i < 150; i++) inputs.atan.push(between(-5, 5));
for (let i = 0; i < 80; i++) inputs.atan.push(between(-1e-3, 1e-3));
for (let i = 0; i < 80; i++) inputs.atan.push(between(-1e5, 1e5));
inputs.atan.push(1, -1, 0.5, 2, 0.4375, 0.6875, 1.1875, 2.4375, 1e6, 1e-5, 0.9999999999999999, 1.0000000000000002);
for (let i = 0; i < 200; i++) inputs.asin.push(between(-1, 1));
for (let i = 0; i < 60; i++) inputs.asin.push(between(-1e-3, 1e-3));
for (let k = 1; k <= 40; k++) inputs.asin.push(1 - 2 ** -k, -(1 - 2 ** -k), Math.round(between(-1e9, 1e9)) / 1e9);
inputs.asin.push(0.5, -0.5, 0.975, 0.9999999999999999, -0.9999999999999999, 0.123456789, 0.999999999, 1e-5);

const EXPR = {
  sin: (x) => `s(${x})`,
  cos: (x) => `c(${x})`,
  tan: (x) => `s(${x})/c(${x})`,
  atan: (x) => `a(${x})`,
  asin: (x) => `a((${x})/sqrt(1-(${x})*(${x})))`,
};
const bc = {};
let skipped = 0;
for (const [fn, list] of Object.entries(inputs)) {
  const program = ['scale=80', ...list.map((x) => EXPR[fn](exactDecimal(x)))].join('\n') + '\n';
  const out = execFileSync('bc', ['-l'], { input: program, encoding: 'utf8', env: { ...process.env, BC_LINE_LENGTH: '0' }, maxBuffer: 1 << 26 })
    .replace(/\\\n/g, '')
    .trim()
    .split('\n');
  if (out.length !== list.length) throw new Error(`bc printed ${out.length} lines for ${list.length} inputs of ${fn}`);
  bc[fn] = [];
  list.forEach((x, i) => {
    const text = out[i];
    // The result has about 80 digits. Round it to a double two times, with
    // 45 and with all the digits. When the two results differ, the value is
    // too near to the middle of two doubles for this check.
    const digits = text.replace(/^-?0*\.?0*/, '').length;
    const short = text.slice(0, text.length - Math.max(0, digits - 45));
    const up = short.replace(/\d$/, (ch) => String(Number(ch) + 1));
    if (digits < 60 || /9$/.test(short) || Number(short) !== Number(text) || Number(up) !== Number(text)) {
      skipped++;
      return;
    }
    bc[fn].push([bits(x), bits(Number(text))]);
  });
}

writeFileSync(OUT, JSON.stringify({ meta: { chromium: 'stadium nodes of .parity/ref (mermaid 12.2.0 in headless Chromium 156)', bc: 'bc -l with scale=80' }, chromium, bc }) + '\n');
console.error(`${stadiums} stadium nodes in the reference SVGs, ${chromium.length} written (${chromium.length * 200} coordinates)`);
console.error(Object.entries(bc).map(([fn, list]) => `${fn}: ${list.length}`).join(', '), `(${skipped} inputs skipped)`);
