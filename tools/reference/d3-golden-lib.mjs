// Shared helpers for the gen-d3-*.mjs golden generators.
//
// Import this module before any d3 module. It wraps the transcendental Math
// functions so that every call that d3 makes is recorded. The Go tests use
// the recorded calls to prove that a mismatch comes only from a last-bit
// difference between Go's math package and V8.
import { gzipSync } from 'node:zlib';
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

// mulberry32, the same generator as gen-jsnum.mjs.
export function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const view = new DataView(new ArrayBuffer(8));

// hex encodes one number as the 16 hex digits of its bits.
export function hex(x) {
  view.setFloat64(0, +x);
  return view.getBigUint64(0).toString(16).padStart(16, '0');
}

// floats encodes a list of numbers as base64 of the little-endian bytes.
export function floats(arr) {
  const f = new Float64Array(arr.length);
  for (let i = 0; i < arr.length; i++) f[i] = +arr[i];
  return Buffer.from(f.buffer, f.byteOffset, f.byteLength).toString('base64');
}

// nums encodes a list of numbers as JSON numbers, which round-trip exactly.
// The values that JSON cannot hold are strings: "NaN", "Infinity",
// "-Infinity" and "-0". This form is shorter than floats for the numbers
// with few digits that diagrams mostly have.
export function nums(arr) {
  return Array.from(arr, (x) => (Number.isNaN(x) ? 'NaN' : x === Infinity ? 'Infinity' : x === -Infinity ? '-Infinity' : Object.is(x, -0) ? '-0' : x));
}

const recorded = new Map();
let recording = true;
const unary = ['sin', 'cos', 'tan', 'asin', 'acos', 'atan', 'exp', 'log', 'log10', 'log2', 'cbrt'];
const binary = ['atan2', 'pow'];
for (const name of unary) {
  const f = Math[name];
  const m = new Map();
  recorded.set(name, m);
  Math[name] = function (x) {
    const r = f(x);
    if (recording && typeof x === 'number') m.set(hex(x), [x, r]);
    return r;
  };
}
for (const name of binary) {
  const f = Math[name];
  const m = new Map();
  recorded.set(name, m);
  Math[name] = function (x, y) {
    const r = f(x, y);
    if (recording && typeof x === 'number' && typeof y === 'number') m.set(hex(x) + hex(y), [x, y, r]);
    return r;
  };
}

// pauseMath stops the recording while the generator itself uses Math.
export function pauseMath(fn) {
  const saved = recording;
  recording = false;
  try {
    return fn();
  } finally {
    recording = saved;
  }
}

// mathTable returns the recorded calls and clears the record.
export function mathTable() {
  const out = {};
  for (const [name, m] of recorded) {
    if (m.size === 0) continue;
    const flat = [];
    for (const v of m.values()) flat.push(...v);
    out[name] = floats(flat);
    m.clear();
  }
  return out;
}

// writeGolden writes obj as gzip-compressed JSON. The path is relative to the
// repository root.
export function writeGolden(relPath, obj) {
  const file = resolve(repoRoot, relPath);
  mkdirSync(dirname(file), { recursive: true });
  const json = JSON.stringify(obj);
  const gz = gzipSync(Buffer.from(json), { level: 9 });
  writeFileSync(file, gz);
  const counts = Object.entries(obj.groups ?? {}).map(([k, v]) => `${k}=${v.length}`).join(' ');
  console.log(`${relPath}: ${(gz.length / 1024).toFixed(0)} KiB gzip, ${(json.length / 1024).toFixed(0)} KiB json; ${counts}`);
}

// Random number helpers. Each takes the generator as the first argument.

// pick returns one element of the list.
export function pick(rnd, list) {
  return list[Math.floor(rnd() * list.length)];
}

// uniform returns a number in [lo, hi).
export function uniform(rnd, lo, hi) {
  return lo + rnd() * (hi - lo);
}

// int returns an integer in [lo, hi].
export function int(rnd, lo, hi) {
  return lo + Math.floor(rnd() * (hi - lo + 1));
}

// magnitude returns a number with a random sign and a random decimal exponent
// in [minExp, maxExp].
export function magnitude(rnd, minExp, maxExp) {
  return pauseMath(() => (rnd() - 0.5) * 2 * Math.pow(10, minExp + rnd() * (maxExp - minExp)));
}

// coord returns a coordinate as diagrams have them: often an integer or a
// number with few decimals, sometimes any double, sometimes huge or tiny.
export function coord(rnd) {
  const k = rnd();
  if (k < 0.3) return Math.round((rnd() - 0.3) * 1000);
  if (k < 0.55) return Math.round((rnd() - 0.3) * 100000) / 100;
  if (k < 0.9) return (rnd() - 0.3) * 1000;
  if (k < 0.95) return magnitude(rnd, -12, 12);
  if (k < 0.98) return magnitude(rnd, -300, 300);
  return pick(rnd, [0, -0, 1, -1, 0.5, 1e21, 1e-7, 123456789.123456789]);
}
