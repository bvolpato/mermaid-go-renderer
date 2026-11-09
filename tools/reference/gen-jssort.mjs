// Emits golden cases for js.SortV8 (internal/js/sort_dagre.go) from the
// Array.prototype.sort of the running V8.
//
// Usage (from tools/reference): node gen-jssort.mjs
//
// The output file depends on the V8 version, because V8 14.9 replaced TimSort
// with PowerSort:
//   V8 <= 14.8: internal/js/testdata/v8_sort_timsort.json
//   V8 >= 14.9: internal/js/testdata/v8_sort_powersort.json
//
// Each case is [n, seed, kind, shape, hash]. The Go test builds the same
// input and comparator from n, seed, kind and shape, and compares the hash of
// the sorted ids. Most comparators are not consistent on purpose: for those
// the result depends on the exact sequence of comparator calls.
import { writeFileSync } from 'node:fs';

const mix = (x) => {
  x = Math.imul(x ^ (x >>> 16), 0x85ebca6b);
  x = Math.imul(x ^ (x >>> 13), 0xc2b2ae35);
  return (x ^ (x >>> 16)) >>> 0;
};

function makeInput(n, seed, shape) {
  const arr = [];
  const runLen = 5 + (seed % 60);
  for (let i = 0; i < n; i++) {
    const h = mix((Math.imul(seed, 0x9e3779b1) + i) | 0);
    let key;
    switch (shape) {
      case 0: key = h % (n || 1); break;
      case 1: key = h % 4; break;
      case 2: key = i % runLen; break;
      case 3: key = h % 10 === 0 ? h % (n || 1) : n - i; break;
      case 4: key = Math.floor(i / 3); break;
      default: key = Math.floor(i / runLen) % 2 === 0 ? i % runLen : runLen - (i % runLen); break;
    }
    arr.push({ id: i, key, nan: mix((i ^ Math.imul(seed, 7)) | 0) % 5 === 0 });
  }
  return arr;
}

function compareAscending(value, other) {
  // lodash compareAscending for numbers.
  if (value !== other) {
    const valIsReflexive = value === value;
    const othIsReflexive = other === other;
    if (value > other || !valIsReflexive) return 1;
    if (value < other || !othIsReflexive) return -1;
  }
  return 0;
}

function makeCompare(seed, kind) {
  const pair = (a, b) => mix((Math.imul(a.id, 0x1f123bb5) ^ Math.imul(b.id, 0x5f356495) ^ seed) | 0);
  const k = (e) => (e.nan ? NaN : e.key);
  switch (kind) {
    case 0: return (a, b) => a.key - b.key;
    case 1: return (a, b) => (pair(a, b) % 3) - 1;
    case 2: return (a, b) => (k(a) < k(b) ? -1 : k(a) > k(b) ? 1 : a.id - b.id);
    case 3: return (a, b) => (k(a) < k(b) ? -1 : k(a) > k(b) ? 1 : b.id - a.id);
    case 4: return (a, b) => (pair(a, b) % 8 === 0 ? b.key - a.key : a.key - b.key);
    case 5: return (a, b) => compareAscending(k(a), k(b)) || a.id - b.id;
    default: return (a, b) => (pair(a, b) % 7 === 0 ? NaN : a.key - b.key);
  }
}

const KINDS = 7;
const SHAPES = 6;

function hashIds(arr) {
  let h = 0x811c9dc5;
  for (const e of arr) {
    h = Math.imul(h ^ (e.id & 0xff), 0x01000193);
    h = Math.imul(h ^ ((e.id >>> 8) & 0xff), 0x01000193);
    h = Math.imul(h ^ ((e.id >>> 16) & 0xff), 0x01000193);
  }
  return h >>> 0;
}

const cases = [];
let seed = 1;
function add(n, kind, shape) {
  seed = (seed + 0x632be5ab) >>> 0;
  const arr = makeInput(n, seed, shape);
  arr.sort(makeCompare(seed, kind));
  cases.push([n, seed, kind, shape, hashIds(arr)]);
}

for (let kind = 0; kind < KINDS; kind++) {
  for (let n = 0; n <= 80; n++) {
    add(n, kind, n % SHAPES);
    add(n, kind, (n + 3) % SHAPES);
    add(n, kind, 0);
  }
  for (const n of [100, 127, 128, 129, 200, 255, 256, 257, 500, 777, 1000, 1023, 1024, 1025, 2000, 4097, 10000]) {
    for (let shape = 0; shape < SHAPES; shape++) add(n, kind, shape);
  }
}

const v8 = process.versions.v8;
const [major, minor] = v8.split('.').map(Number);
const powersort = major > 14 || (major === 14 && minor >= 9);
const file = `../../internal/js/testdata/v8_sort_${powersort ? 'powersort' : 'timsort'}.json`;
writeFileSync(new URL(file, import.meta.url), JSON.stringify({ v8, cases }) + '\n');
console.log(`${cases.length} cases, V8 ${v8}, wrote ${file}`);
