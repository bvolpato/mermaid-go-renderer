// Writes golden cases from V8 for internal/js/str_jison.go: the string
// functions that count UTF-16 code units and keep lone surrogates.
//
// Usage (in tools/reference): node gen-jison-str.mjs
import fs from 'node:fs';
import path from 'node:path';
import { prng, repoRoot } from './jison-lib.mjs';

const rnd = prng(0x3c6ef372);
const int = (n) => Math.floor(rnd() * n);
const pools = [
  () => 0x20 + int(0x5f), // printable ASCII
  () => [0x09, 0x0a, 0x0d, 0x22, 0x5c, 0x00, 0x1f, 0x7f, 0x08, 0x0c][int(10)],
  () => 0xa0 + int(0x700), // two-byte UTF-8
  () => 0x800 + int(0xd000), // three-byte UTF-8 below the surrogates
  () => 0xe000 + int(0x2000),
  () => [0x2028, 0x2029, 0xfeff, 0xfffd, 0xffff, 0xd7ff, 0xe000, 0xed, 0xa0, 0x80][int(10)],
  () => 0xd800 + int(0x400), // a high surrogate
  () => 0xdc00 + int(0x400), // a low surrogate
];
const units = () => {
  const n = int(12);
  const out = [];
  for (let i = 0; i < n; i++) {
    const r = rnd();
    if (r < 0.15) {
      // A surrogate pair.
      out.push(0xd800 + int(0x400), 0xdc00 + int(0x400));
    } else {
      out.push(pools[r < 0.6 ? 0 : 1 + int(pools.length - 1)]());
    }
  }
  return out;
};
const toUnits = (s) => Array.from({ length: s.length }, (_, i) => s.charCodeAt(i));
const index = () => (rnd() < 0.1 ? [-100, 100, 0][int(3)] : int(31) - 15);

const cases = [];
for (let i = 0; i < 600; i++) {
  const u = units();
  const s = String.fromCharCode(...u);
  const t = String.fromCharCode(...units());
  const [a, b, c] = [index(), index(), index()];
  cases.push({
    u,
    t: toUnits(t),
    a,
    b,
    c,
    slice: toUnits(s.slice(a, b)),
    sliceFrom: toUnits(s.slice(a)),
    substr: toUnits(s.substr(a, b)),
    charAt: toUnits(s.charAt(c)),
    concat: toUnits(s + t),
    wellFormed: toUnits(s.toWellFormed()),
    json: JSON.stringify(s),
  });
}
const out = path.join(repoRoot, 'internal/js/testdata/str_jison.json');
fs.writeFileSync(out, JSON.stringify({ cases }) + '\n');
console.log(`${cases.length} cases, ${fs.statSync(out).size} bytes`);
