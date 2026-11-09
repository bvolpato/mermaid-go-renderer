// Writes golden cases from V8 for internal/js/str_kanban.go:
// String.prototype.replace with a search value that is a string.
//
// Usage (in tools/reference): node gen-kanban-jsreplace.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

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
const rnd = prng(0x6b616e62);
const int = (n) => Math.floor(rnd() * n);
const pick = (a) => a[int(a.length)];
const toUnits = (s) => Array.from({ length: s.length }, (_, i) => s.charCodeAt(i));

const pieces = ['#TICKET#', '$', '$$', '$&', '$`', "$'", '$1', '$0', '$<a>', '$<', 'a', 'b', '/', 'x', 'é', '😀', '\ud83d', '\ude00', '#', 'TICKET', ' ', ''];
const text = (n) => {
  let s = '';
  for (let i = int(n); i > 0; i--) {
    s += pick(pieces);
  }
  return s;
};

const cases = [];
const add = (s, search, replacement) => {
  cases.push({ s: toUnits(s), search: toUnits(search), replacement: toUnits(replacement), out: toUnits(s.replace(search, replacement)) });
};
add('https://example.com/#TICKET#', '#TICKET#', 'MC-2037');
add('https://example.com/#TICKET#/#TICKET#', '#TICKET#', 'MC-2037');
add('https://example.com/browse', '#TICKET#', 'MC-2037');
add('a#TICKET#b', '#TICKET#', '[$&|$`|$\'|$$|$1|$<x>|$]');
add('', '', 'x');
add('abc', '', 'x$&y');
add('abc', 'abc', '');
add('abc', 'abcd', 'x');
add('a$b', '$', '$$$$');
add('x', 'x', '$');
add('x', 'x', '$$');
add('x', 'x', '$$$');
for (let i = 0; i < 800; i++) {
  const s = text(8);
  const search = rnd() < 0.5 ? '#TICKET#' : rnd() < 0.5 ? pick(pieces) : text(3);
  add(s, search, text(6));
}
const out = path.join(repoRoot, 'internal/js/testdata/str_kanban.json');
fs.writeFileSync(out, JSON.stringify({ cases }) + '\n');
console.log(`${path.relative(repoRoot, out)}: ${cases.length} cases`);
