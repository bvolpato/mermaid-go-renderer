#!/usr/bin/env node
// Writes internal/diagrams/cynefin/testdata/boundaries.json.gz: goldens for
// the functions of cynefinBoundaries.ts of mermaid 12.2.0, from the real
// code, for random inputs: seededRandom, hashString, resolveSeed,
// generateFoldPath, generateHorizontalBoundary, generateCliffPath and
// generateConfusionPath.
//
// The functions are not exports of the build. A module load hook appends
// one export statement to the chunk of the cynefin diagram. No other line
// of mermaid changes.
//
// Run it with tools/reference as the working directory:
//
//   node gen-cynefin-boundaries.mjs
import { registerHooks } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CHUNKS = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.core');
const OUT = path.join(ROOT, 'internal/diagrams/cynefin/testdata/boundaries.json.gz');

const NAMES = [
  'seededRandom',
  'hashString',
  'resolveSeed',
  'generateFoldPath',
  'generateHorizontalBoundary',
  'generateCliffPath',
  'generateConfusionPath',
];
const chunk = fs
  .readdirSync(CHUNKS)
  .find((f) => f.endsWith('.mjs') && fs.readFileSync(path.join(CHUNKS, f), 'utf8').includes('function generateFoldPath('));
if (!chunk) {
  throw new Error('no chunk has generateFoldPath');
}
registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/' + chunk)) {
      const source = result.source.toString() + `\nexport { ${NAMES.map((n) => `${n} as __${n}`).join(', ')} };\n`;
      return { ...result, source };
    }
    return result;
  },
});
const mod = await import(pathToFileURL(path.join(CHUNKS, chunk)).href);
const fn = Object.fromEntries(NAMES.map((n) => [n, mod['__' + n]]));

// mulberry32, the generator that the golden scripts share.
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
const rnd = prng(0x63796e62);
const pick = (a) => a[Math.floor(rnd() * a.length)];

// A number is the hex text of its bits, because a value can be NaN or -0.
const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  if (Number.isNaN(v)) return '7ff8000000000000';
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};
// encode keeps undefined and the numbers that JSON has no text for.
const encode = (v) => {
  if (v === undefined) return { $u: 1 };
  if (typeof v === 'number') return { $b: bits(v) };
  return v;
};

const randomSeed = () => {
  const k = rnd();
  if (k < 0.3) return Math.floor(rnd() * 1000) - 500;
  if (k < 0.55) return Math.floor(rnd() * 2 ** 32) - 2 ** 31;
  if (k < 0.7) return (rnd() - 0.5) * 2 ** 40;
  if (k < 0.8) return (rnd() - 0.5) * 100;
  return pick([0, -0, 1, -1, 2 ** 31, 2 ** 31 - 1, -(2 ** 31), 2 ** 32, 2 ** 53, -(2 ** 53), 1e300, -1e300, NaN, Infinity, -Infinity, 0.5, 42, -1771684999]);
};
const randomSize = () => {
  const k = rnd();
  if (k < 0.5) return Math.floor(rnd() * 1500) + 1;
  if (k < 0.85) return rnd() * 1500;
  return pick([0, -0, 1, 800, 600, 0.1, 1e-7, 123456.789, -200, NaN]);
};
const randomAmplitude = () => {
  const k = rnd();
  if (k < 0.3) return undefined;
  if (k < 0.5) return Math.floor(rnd() * 30);
  if (k < 0.85) return rnd() * 30;
  return pick([0, -0, 8, -3.5, null, 1e-9, 250, NaN]);
};
const words = ['my-svg', 'mermaid-1', '', 'a', 'é😀', 'a very long identifier of an svg element 0123456789', '\u0000', 'ｆｕｌｌ'];
const randomText = () => {
  if (rnd() < 0.4) return pick(words);
  let s = '';
  const n = Math.floor(rnd() * 40);
  for (let i = 0; i < n; i++) s += String.fromCharCode(Math.floor(rnd() * (rnd() < 0.8 ? 128 : 65536)));
  return s;
};

const cases = { seededRandom: [], hashString: [], resolveSeed: [], fold: [], horizontal: [], cliff: [], confusion: [] };
for (let i = 0; i < 1500; i++) {
  const seed = randomSeed();
  cases.seededRandom.push({ seed: bits(seed), result: bits(fn.seededRandom(seed)) });
}
for (let i = 0; i < 400; i++) {
  const text = randomText();
  // The text as code units: a random text can hold half of a surrogate pair.
  const units = Array.from({ length: text.length }, (_, k) => text.charCodeAt(k));
  cases.hashString.push({ units, result: fn.hashString(text) });
}
for (let i = 0; i < 300; i++) {
  const configured = pick([undefined, null, '42', true, false, {}, [], randomSeed(), randomSeed(), randomSeed()]);
  const id = pick(words);
  cases.resolveSeed.push({ configured: encode(configured), id, result: bits(fn.resolveSeed(configured, id)) });
}
for (let i = 0; i < 400; i++) {
  const [w, h, seed, amp] = [randomSize(), randomSize(), randomSeed(), randomAmplitude()];
  const args = { width: bits(w), height: bits(h), seed: bits(seed), amplitude: encode(amp) };
  cases.fold.push({ ...args, result: fn.generateFoldPath(w, h, seed, amp) });
  cases.horizontal.push({ ...args, result: fn.generateHorizontalBoundary(w, h, seed, amp) });
}
for (let i = 0; i < 300; i++) {
  const [w, h] = [randomSize(), randomSize()];
  cases.cliff.push({ width: bits(w), height: bits(h), result: fn.generateCliffPath(w, h) });
  const [cx, cy, rx, ry] = [randomSize(), randomSize(), randomSize(), randomSize()];
  cases.confusion.push({ cx: bits(cx), cy: bits(cy), rx: bits(rx), ry: bits(ry), result: fn.generateConfusionPath(cx, cy, rx, ry) });
}

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, zlib.gzipSync(Buffer.from(JSON.stringify(cases)), { level: 9 }));
const total = Object.values(cases).reduce((n, list) => n + list.length, 0);
console.log(`${path.relative(ROOT, OUT)}: ${total} cases, ${(fs.statSync(OUT).size / 1024).toFixed(1)} kB`);
