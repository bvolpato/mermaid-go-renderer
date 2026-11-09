// Writes the Chromium measurements for the Linux tests of internal/font:
// internal/font/testdata/linux-family.json.gz, linux-weight.json.gz,
// linux-glyph.json.gz, linux-missing.json.gz and linux-autohint.json.gz.
//
// The reference environment is Chromium on Linux with only the Liberation
// fonts: oracle-linux.mjs --fonts liberation. The tests replay the cases
// with font.Embedded().
//
// With --image the script measures in the Playwright image as it is, with
// all of its fonts (Liberation, WenQuanYi Zen Hei, IPAGothic, Noto Color
// Emoji, FreeFont and others), and writes linux-image-*.json.gz. The tests
// of that data need the font files of the image (.parity/linux/image/fonts).
//
// Usage (in tools/reference): node gen-font-linux-probes.mjs [--image] [family|weight|glyph|missing|autohint ...]
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outDir = path.join(repoRoot, 'internal/font/testdata');
const workDir = path.join(repoRoot, '.parity/linux/work');
fs.mkdirSync(workDir, { recursive: true });
const argv = process.argv.slice(2);
const fonts = argv[0] === '--image' ? 'image' : 'liberation';
if (fonts === 'image') {
  argv.shift();
}

// run measures cases in the reference environment and returns the result of
// font-linux-probe.js.
function run(name, args) {
  const argsFile = path.join(workDir, `font-${name}-args.json`);
  const outFile = path.join(workDir, `font-${name}-out.json`);
  fs.writeFileSync(argsFile, JSON.stringify(args));
  execFileSync('node', ['oracle-linux.mjs', '--fonts', fonts, 'eval', 'font-linux-probe.js', '--bare', '--args', argsFile, '--out', outFile], { cwd: here, stdio: 'inherit' });
  return JSON.parse(fs.readFileSync(outFile, 'utf8'));
}

function write(name, value) {
  const file = path.join(outDir, `linux-${fonts === 'image' ? 'image-' : ''}${name}.json.gz`);
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify(value), { level: 9 }));
  console.error(`${file}: ${Array.isArray(value) ? value.length : Object.keys(value).length} cases`);
}

const fingerprint = 'Hamburgefonstiv 0123 WiMl';

// family: which font a font-family value gives.
function family() {
  const families = [
    // Sans: Arial and the families that fontconfig or Blink map to it.
    'arial', 'Arial', 'ARIAL', '"Arial"', 'helvetica', 'Helvetica', '"helvetica neue"', 'arimo', '"liberation sans"',
    '"Liberation Sans"', 'albany', '"albany amt"', '"nimbus sans"', '"nimbus sans l"', '"tex gyre heros"',
    'verdana', 'tahoma', '"trebuchet ms"', '"dejavu sans"', '"bitstream vera sans"', '"segoe ui"', 'roboto',
    '"noto sans"', '"open sans"', '"arial narrow"', '"liberation sans narrow"', '"arial black"',
    // Serif.
    '"times new roman"', 'times', 'Times', 'tinos', '"liberation serif"', 'thorndale', '"thorndale amt"', 'georgia',
    '"nimbus roman"', '"nimbus roman no9 l"', '"dejavu serif"', 'cambria', 'caladea', 'calibri', 'carlito', 'palatino',
    // Mono.
    '"courier new"', 'courier', 'Courier', 'cousine', '"liberation mono"', 'cumberland', '"cumberland amt"',
    '"nimbus mono"', '"nimbus mono ps"', 'menlo', 'consolas', 'monaco', '"lucida console"', '"dejavu sans mono"',
    '"andale mono"', '"source code pro"',
    // Other names.
    '"comic sans ms"', 'impact', 'symbol', '"ms gothic"', '"ms pgothic"', '"noto sans cjk jp"', 'simsun',
    '"nonexistent font"', 'fixed', 'system', 'default',
    // Names that fontconfig takes as a request for a default font.
    'sans', 'Sans', '"sans"', 'serif', 'Serif', 'SERIF', '"serif"', '"Serif"', 'monospace', 'Monospace', '"monospace"',
    '"Monospace"', 'mono', '"sans serif"', '"sans-serif"', 'sans-serif', 'Sans-Serif',
    // Generic families.
    'cursive', 'fantasy', 'system-ui', '"system-ui"', 'ui-serif', 'ui-sans-serif', 'ui-monospace', 'ui-rounded',
    'math', 'emoji', 'fangsong', '-webkit-body', '-apple-system', 'BlinkMacSystemFont', '"cursive"', '"fantasy"',
    // Lists.
    '"nonexistent font", monospace', '"nonexistent font", "liberation mono", arial', 'cursive, arial',
    'fantasy, courier', 'system-ui, arial', 'ui-sans-serif, times', 'math, arial', 'emoji, arial',
    '"trebuchet ms", verdana, arial, sans-serif', '"Recursive Variable", arial, sans-serif',
    'verdana, sans-serif', 'verdana, serif', 'verdana, monospace', 'verdana, cursive', 'verdana, fantasy',
    'verdana, system-ui', 'verdana, ui-monospace', 'verdana, math', 'verdana, "sans"',
    'courier, arial', 'times, courier', 'ui-monospace, arial', 'fangsong, arial',
  ];
  const cases = [];
  for (const fam of families) {
    for (const [weight, style] of [['normal', 'normal'], ['bold', 'normal'], ['normal', 'italic'], ['bold', 'italic']]) {
      cases.push({ text: fingerprint, family: fam, size: 16, weight, style });
    }
  }
  write('family', run('family', { text: cases }).text);
}

// weight: which face of a family a font-weight and a font-style give.
function weight() {
  const cases = [];
  const weights = ['1', '100', '200', '300', '350', '399', '400', '401', '450', '499', '500', '501', '549', '550', '551', '599', '600', '601', '650', '700', '800', '900', '950', '1000', 'normal', 'bold', 'bolder', 'lighter'];
  for (const fam of ['arial', '"times new roman"', '"courier new"', '"liberation sans"', 'serif', 'monospace', 'verdana']) {
    for (const w of weights) {
      for (const style of ['normal', 'italic', 'oblique', 'oblique 10deg', 'oblique -10deg', 'oblique 30deg']) {
        cases.push({ text: fingerprint, family: fam, size: 16, weight: w, style });
      }
    }
  }
  write('weight', run('weight', { text: cases }).text);
}

const faces = [];
for (const fam of ['Liberation Sans', 'Liberation Serif', 'Liberation Mono']) {
  for (const [weight, style] of [['normal', 'normal'], ['bold', 'normal'], ['normal', 'italic'], ['bold', 'italic']]) {
    faces.push({ family: `"${fam}"`, weight, style });
  }
}

// glyph: the advance and the ink bounds of single glyphs, for each face at
// many font sizes.
function glyph() {
  const chars = [];
  for (let c = 0x20; c < 0x7f; c++) chars.push(String.fromCodePoint(c));
  for (let c = 0xa0; c < 0x180; c += 1) chars.push(String.fromCodePoint(c));
  for (const s of 'ǺǻǼǽǾǿȘșȚțΆΈΉΊΌΎΏΐΑΒΓΔΩαβγδωϑϒЀЁЂЃЄЖЩЫЯабвгджщыяѐёђѓ‐–—‘’“”†‡•…‰‹›⁄€™ℓ№Ω℮⅛←↑→↓∂∆∏∑−√∞∫≈≠≤≥◊ﬁﬂẀẁẂẃẄẅỲỳẠạẤấẦầẨẩẪẫẬậỆệỖỗỰự') chars.push(s);
  const sizes = [7, 8, 9, 9.6, 10, 11, 12, 12.8, 13, 13.333, 14, 14.4, 15, 16, 17, 17.5, 18, 19, 20, 22, 24, 28, 32, 36, 40, 48, 64, 72, 100, 150, 200, 255, 256, 257, 300, 400];
  const cases = [];
  for (const face of faces) {
    for (const size of sizes) {
      cases.push({ ...face, size, chars });
    }
  }
  const result = run('glyph', { glyphs: cases }).glyphs;
  // The list of strings is the same for every case.
  write('glyph', { chars, cases: result.map(({ chars: _, ...rest }) => rest) });
}

// missing: characters that no Liberation font has, and characters that only
// some of the Liberation fonts have.
function missing() {
  const texts = ['日本語', '中文测试', '한국어', '😀', '😀 🎉', 'a😀b', 'a日b', '✓ ✗ ★', '→ ← ↑ ↓', '☺', '♥', '⚠', '①', '⌘', 'ﬃ', 'ℵ', '́', 'á', '​', 'a​b', '﻿', '­', 'a­b', '\t', 'a\tb', ' ', '�', '', '\u{1f1fa}\u{1f1f8}', 'אב', 'ال', 'ก', 'अ', 'Ω', 'ℓ', '⅓', '∮', '▲', '░', '┌─┐', '♪', '✔', '❤', '☃', '𝒜', '\u{10000}'];
  const cases = [];
  for (const fam of ['arial', '"times new roman"', '"courier new"', 'verdana', 'monospace']) {
    for (const [weight, style] of [['normal', 'normal'], ['bold', 'normal'], ['normal', 'italic'], ['bold', 'italic']]) {
      for (const t of texts) {
        for (const size of fam === 'arial' && weight === 'normal' && style === 'normal' ? [16, 14, 13.333, 20] : [16]) {
          cases.push({ text: t, family: fam, size, weight, style });
        }
      }
    }
  }
  write('missing', run('missing', { text: cases }).text);
}

// autohint: the ink bounds of the single glyphs that the set glyph does not
// have, for the test of the automatic hinter. The characters are the
// characters that all twelve Liberation faces have, without the characters
// of the set glyph and without the format characters: more Latin, Greek and
// Cyrillic, Hebrew, combining marks, superscripts, subscripts and symbols.
// Each face has three font sizes, and the four faces of a family have twelve
// font sizes together.
const autohintRanges = [
  [0x0180, 0x01f9], [0x0200, 0x0217], [0x021c, 0x0236], [0x0238, 0x036f], [0x0374, 0x0375], [0x037a, 0x037e],
  [0x0384, 0x0385], [0x0387, 0x0387], [0x0395, 0x03a1], [0x03a3, 0x03a8], [0x03aa, 0x03b0], [0x03b5, 0x03c8],
  [0x03ca, 0x03ce], [0x03d0, 0x03d0], [0x03d3, 0x03ff], [0x0405, 0x0415], [0x0417, 0x0428], [0x042a, 0x042a],
  [0x042c, 0x042e], [0x0435, 0x0435], [0x0437, 0x0448], [0x044a, 0x044a], [0x044c, 0x044e], [0x0454, 0x0513],
  [0x051a, 0x051d], [0x0591, 0x05c7], [0x05d0, 0x05ea], [0x05f0, 0x05f4], [0x1d00, 0x1dca], [0x1dfe, 0x1e7f],
  [0x1e86, 0x1e9b], [0x1e9e, 0x1e9e], [0x1ea2, 0x1ea3], [0x1eae, 0x1ec5], [0x1ec8, 0x1ed5], [0x1ed8, 0x1eef],
  [0x1ef4, 0x1ef9], [0x1f00, 0x1f15], [0x1f18, 0x1f1d], [0x1f20, 0x1f45], [0x1f48, 0x1f4d], [0x1f50, 0x1f57],
  [0x1f59, 0x1f59], [0x1f5b, 0x1f5b], [0x1f5d, 0x1f5d], [0x1f5f, 0x1f7d], [0x1f80, 0x1fb4], [0x1fb6, 0x1fc4],
  [0x1fc6, 0x1fd3], [0x1fd6, 0x1fdb], [0x1fdd, 0x1fef], [0x1ff2, 0x1ff4], [0x1ff6, 0x1ffe], [0x2007, 0x2007],
  [0x2012, 0x2012], [0x2015, 0x2015], [0x2017, 0x2017], [0x201a, 0x201b], [0x201e, 0x201f], [0x2032, 0x2034],
  [0x203c, 0x203c], [0x203e, 0x203e], [0x205e, 0x205e], [0x2074, 0x2079], [0x207f, 0x2089], [0x2090, 0x2094],
  [0x20a0, 0x20ab], [0x20ad, 0x20b5], [0x20bf, 0x20bf], [0x20f0, 0x20f0], [0x2105, 0x2105], [0x2117, 0x2117],
  [0x2126, 0x2126], [0x214d, 0x214e], [0x2153, 0x2154], [0x215c, 0x215e], [0x2184, 0x2184], [0x2194, 0x2195],
  [0x21a8, 0x21a8], [0x21d4, 0x21d4], [0x2215, 0x2215], [0x2219, 0x2219], [0x221f, 0x221f], [0x2229, 0x2229],
  [0x2261, 0x2262], [0x2302, 0x2302], [0x2310, 0x2310], [0x2320, 0x2321], [0x2500, 0x2500], [0x2502, 0x2502],
  [0x250c, 0x250c], [0x2510, 0x2510], [0x2514, 0x2514], [0x2518, 0x2518], [0x251c, 0x251c], [0x2524, 0x2524],
  [0x252c, 0x252c], [0x2534, 0x2534], [0x253c, 0x253c], [0x2550, 0x256c], [0x2580, 0x2580], [0x2584, 0x2584],
  [0x2588, 0x2588], [0x258c, 0x258c], [0x2590, 0x2593], [0x25a0, 0x25a1], [0x25aa, 0x25ac], [0x25b2, 0x25b2],
  [0x25ba, 0x25ba], [0x25bc, 0x25bc], [0x25c4, 0x25c4], [0x25cb, 0x25cc], [0x25cf, 0x25d9], [0x25e6, 0x25e6],
  [0x263a, 0x263c], [0x2640, 0x2640], [0x2642, 0x2642], [0x2660, 0x2660], [0x2663, 0x2663], [0x2665, 0x2666],
  [0x2669, 0x266c], [0x266f, 0x266f], [0x2c60, 0x2c6d], [0x2c71, 0x2c77], [0x2e17, 0x2e17], [0xa717, 0xa721],
  [0xa788, 0xa78c], [0xfb1d, 0xfb36], [0xfb38, 0xfb3c], [0xfb3e, 0xfb3e], [0xfb40, 0xfb41], [0xfb43, 0xfb44],
  [0xfb46, 0xfb4f], [0xfe20, 0xfe23],
];

function autohint() {
  if (fonts === 'image') {
    autohintImage();
    return;
  }
  const chars = [];
  for (const [first, last] of autohintRanges) {
    for (let c = first; c <= last; c++) chars.push(String.fromCodePoint(c));
  }
  const sizes = [8.5, 10.5, 11.5, 12, 13.5, 14, 16, 18.5, 21, 26, 34, 57];
  const cases = [];
  faces.forEach((face, i) => {
    for (let k = 0; k < 3; k++) {
      cases.push({ ...face, size: sizes[(i % 4) + 4 * k], chars });
    }
  });
  const result = run('autohint', { glyphs: cases }).glyphs;
  // The list of strings is the same for every case.
  write('autohint', { chars, cases: result.map(({ chars: _, ...rest }) => rest) });
}

// imageFontDir has the font files of the Playwright image (its
// /usr/share/fonts). The files are not in the repository.
const imageFontDir = path.join(repoRoot, '.parity/linux/image/fonts');

// fontChars returns the characters of the character map of a font file, in
// order. It reads the first face of a collection, and the subtable of
// format 12 for Windows.
function fontChars(file) {
  const d = fs.readFileSync(file);
  const base = d.toString('latin1', 0, 4) === 'ttcf' ? d.readUInt32BE(12) : 0;
  let cmap = -1;
  for (let i = 0; i < d.readUInt16BE(base + 4); i++) {
    const rec = base + 12 + 16 * i;
    if (d.toString('latin1', rec, rec + 4) === 'cmap') cmap = d.readUInt32BE(rec + 8);
  }
  let sub = -1;
  for (let i = 0; cmap >= 0 && i < d.readUInt16BE(cmap + 2); i++) {
    const rec = cmap + 4 + 8 * i;
    const off = cmap + d.readUInt32BE(rec + 4);
    if (d.readUInt16BE(rec) === 3 && d.readUInt16BE(rec + 2) === 10 && d.readUInt16BE(off) === 12) sub = off;
  }
  if (sub < 0) {
    throw new Error(`${file}: no character map of format 12`);
  }
  const chars = [];
  for (let g = 0; g < d.readUInt32BE(sub + 12); g++) {
    const rec = sub + 16 + 12 * g;
    const first = d.readUInt32BE(rec);
    const last = d.readUInt32BE(rec + 4);
    const gid = d.readUInt32BE(rec + 8);
    for (let c = first; c <= last; c++) {
      if (gid + (c - first) !== 0) chars.push(c);
    }
  }
  return chars;
}

// sampleChars takes up to perPage characters of each page of 128 characters,
// at even steps. It leaves out the control, space and format characters,
// the variation selectors and the private use characters.
function sampleChars(chars, perPage) {
  const skip = (c) =>
    c < 0x21 || (c >= 0x7f && c <= 0xa0) || c === 0xad || (c >= 0x2000 && c <= 0x200f) || (c >= 0x2028 && c <= 0x202f) ||
    (c >= 0x205f && c <= 0x206f) || c === 0x3000 || (c >= 0xd800 && c <= 0xf8ff) || (c >= 0xfe00 && c <= 0xfe0f) ||
    c === 0xfeff || c >= 0xe0000;
  const pages = new Map();
  for (const c of chars) {
    if (skip(c)) continue;
    const page = c >> 7;
    if (!pages.has(page)) pages.set(page, []);
    pages.get(page).push(c);
  }
  const out = [];
  for (const list of pages.values()) {
    const n = Math.min(list.length, perPage);
    for (let k = 0; k < n; k++) out.push(String.fromCodePoint(list[Math.floor((k * list.length) / n)]));
  }
  return out;
}

// autohintImage: the ink bounds of single glyphs of fonts of the Playwright
// image that have other scripts than the Liberation fonts, for the test of
// the automatic hinter: the FreeFont families (Arabic, Armenian, Cherokee,
// Ethiopic, Georgian, Thai, the scripts of India and others) and two fonts
// for Chinese and Japanese. The characters are a sample of the character
// map of each font.
//
// The fontconfig files of the image set the hint style none for WenQuanYi
// Zen Hei, and for IPAGothic up to 18px (25-wqy-zenhei.conf and
// 65-fonts-ipafont-gothic.conf). Chromium does not hint those glyphs. The
// other cases have slight hinting.
function autohintImage() {
  const fontCases = [
    { family: 'FreeSerif', file: 'truetype/freefont/FreeSerif.ttf', perPage: 12, sizes: [11, 16, 22] },
    { family: 'FreeSans', file: 'truetype/freefont/FreeSans.ttf', perPage: 10, sizes: [10, 14] },
    { family: 'FreeMono', file: 'truetype/freefont/FreeMono.ttf', perPage: 6, sizes: [12, 17] },
    { family: 'IPAGothic', file: 'opentype/ipafont-gothic/ipag.ttf', perPage: 4, sizes: [14, 20, 26, 36] },
    { family: '"WenQuanYi Zen Hei"', file: 'truetype/wqy/wqy-zenhei.ttc', perPage: 2, sizes: [15] },
  ];
  const cases = [];
  for (const f of fontCases) {
    const chars = sampleChars(fontChars(path.join(imageFontDir, f.file)), f.perPage);
    for (const size of f.sizes) {
      cases.push({ family: f.family, weight: 'normal', style: 'normal', size, chars });
    }
  }
  write('autohint', run('autohint', { glyphs: cases }).glyphs);
}

const sets = { family, weight, glyph, missing, autohint };
const wanted = argv.length > 0 ? argv : Object.keys(sets);
for (const name of wanted) {
  if (!sets[name]) {
    console.error(`unknown set ${name}`);
    process.exit(2);
  }
  sets[name]();
}
