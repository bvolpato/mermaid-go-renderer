// Writes internal/mmdc/testdata/tokenizecss.json: what the functions of
// src/browser/tokenizeCSS.js of mermaid-cli return for many texts. The Go
// port in internal/mmdc/tokenizecss.go is compared with it.
//
// The functions are plain JavaScript, so this script needs no browser. It
// runs the file of the installed mermaid-cli.
//
// For each text:
//   families   parseFontFamily(text), or error
//   sources    tokenizeCSSSrc(text), or error
//   src        stringifyCSSSrc(sources)
//
// Usage (in tools/reference): node gen-mmdc-tokenize.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseFontFamily, tokenizeCSSSrc, stringifyCSSSrc } from './node_modules/@mermaid-js/mermaid-cli/src/browser/tokenizeCSS.js';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

// The texts of the page of mermaid-cli and the corners of the parser.
const texts = [
  '',
  ' ',
  '"Recursive Variable", arial, sans-serif',
  '"trebuchet ms", verdana, arial, sans-serif',
  '"Font Awesome 7 Free"',
  '"Font Awesome 7 Brands"',
  '"Open Sans", sans-serif',
  'Times New Roman',
  '"Hello, World", Hello, World, Hello World, \\22 toto\\22',
  "'single', \"double\"",
  'a,b,,c',
  'a , b ',
  ' a',
  'a\tb\nc',
  '-apple-system, BlinkMacSystemFont',
  '--custom, -x, -',
  '1abc',
  '"unterminated',
  '"esc\\"aped"',
  '\\41 BC',
  '\\000041BC',
  '\\1F600 x',
  '\\110000 x',
  '\\',
  'a\\',
  '\\z\\é',
  'é, 😀 x',
  'url("../webfonts/fa-brands-400.woff2")',
  'url("./files/recursive-latin-wght-normal.woff2") format("woff2-variations")',
  'url("./files/open-sans-latin-400-normal.woff2") format("woff2"), url("./files/open-sans-latin-400-normal.woff") format("woff")',
  'url(a.woff2) format(woff2)',
  'url( a.woff2 ) format( "woff2" ) tech( variations ), local( Arial )',
  "url('a.woff') format('woff'), local('My Font')",
  'local(Arial), url(a.ttf)',
  'url(a b.woff2)',
  'url(a(b).woff2)',
  'url(a\\29 b.woff2)',
  'url()',
  'url("")',
  'URL(a.woff2)',
  'url (a.woff2)',
  'format("woff2")',
  'tech(color-COLRv1) url(a.woff2)',
  'unknown(x) url(a.woff2)',
  'url(a.woff2),',
  ',url(a.woff2)',
  'url(a.woff2) format("a\\"b")',
  'url("a\nb")',
  'url(data:font/woff2;base64,AAAA) format("woff2")',
  'a(b',
  '(a)',
  'url(a.woff2) !',
];

// More texts from a seeded generator: short texts of the characters that
// the parser tests.
let seed = 0x6d6d6467;
const random = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pieces = [
  'a', 'B', 'z', '0', '9', 'f', '_', '-', '--', ' ', ' ', '\t', '\n', '"', "'", '\\', '(', ')', ',', ',', 'url', 'url(', 'format(',
  'local(', 'tech(', 'woff2', '.', '/', ':', ';', 'é', '😀', '\\2', '\\"', '\u007f', '\u0001', '!',
];
for (let n = 0; n < 2500; n++) {
  const length = Math.floor(random() * 9);
  let text = '';
  for (let i = 0; i < length; i++) {
    text += pieces[Math.floor(random() * pieces.length)];
  }
  texts.push(text);
}

const attempt = (fn) => {
  try {
    return { value: fn() };
  } catch (e) {
    return { error: String(e.message) };
  }
};
const cases = texts.map((text) => {
  const families = attempt(() => parseFontFamily(text));
  const sources = attempt(() => tokenizeCSSSrc(text));
  const c = { text };
  if (families.error !== undefined) {
    c.familiesError = families.error;
  } else {
    c.families = families.value;
  }
  if (sources.error !== undefined) {
    c.sourcesError = sources.error;
  } else {
    c.sources = sources.value;
    c.src = stringifyCSSSrc(sources.value);
  }
  return c;
});
const file = path.join(repoRoot, 'internal/mmdc/testdata/tokenizecss.json');
fs.mkdirSync(path.dirname(file), { recursive: true });
fs.writeFileSync(file, JSON.stringify(cases) + '\n');
const errors = cases.filter((c) => c.familiesError !== undefined || c.sourcesError !== undefined).length;
process.stderr.write(`wrote ${path.relative(repoRoot, file)}: ${cases.length} texts, ${errors} with an error\n`);
