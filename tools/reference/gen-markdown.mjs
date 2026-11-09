// Writes the golden files of internal/markdown. The oracle is the label
// markdown code of the mermaid 12.2.0 bundle: handle-markdown-text.ts,
// splitText.ts, marked 16.3.0 and ts-dedent 2.2.0.
//
//   cd tools/reference && node gen-markdown.mjs
//
// markdown.json.gz holds one case for each input:
//   in | units | file   the input: a string, UTF-16 code units (for a
//                       surrogate that has no partner), or the path of a
//                       corpus fixture whose whole text is the input
//   want                JSON.stringify of the array
//                         [ markdownToLines(input, config),
//                           markdownToHTML(input, { markdownAutoWrap: true }),
//                           markdownToHTML(input, { markdownAutoWrap: false }),
//                           nonMarkdownToLines(input),
//                           nonMarkdownToHTML(input) ]
//   sum                 in place of want, to keep the file small: the first
//                       16 hex digits of the SHA-256 of want. The fixtures
//                       have it, the inputs that marked reads as one
//                       paragraph with one text token, and three of four
//                       generated inputs.
//
// split.json.gz holds the cases of splitTextToChars and splitLineToFitWidth.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import {
  REPO,
  marked,
  markdownToHTML,
  markdownToLines,
  nonMarkdownToHTML,
  nonMarkdownToLines,
  splitLineToFitWidth,
  splitTextToChars,
  versions,
} from './markdown-lib.mjs';
import { corpusFiles, inputGroups, mulberry32, surrogateInputs } from './markdown-inputs.mjs';

if (versions.marked !== '16.3.0') throw new Error(`unexpected marked ${versions.marked}`);

const sum = (s) => crypto.createHash('sha256').update(s, 'utf8').digest('hex').slice(0, 16);
const unitsOf = (s) => Array.from({ length: s.length }, (_, i) => s.charCodeAt(i));

const run = (input) => {
  const wrap = { markdownAutoWrap: true };
  const noWrap = { markdownAutoWrap: false };
  const lines = markdownToLines(input, wrap);
  // markdownToLines reads markdownAutoWrap and does not use it.
  for (const config of [noWrap, {}, undefined]) {
    if (JSON.stringify(markdownToLines(input, config)) !== JSON.stringify(lines)) {
      throw new Error(`markdownToLines depends on the config for ${JSON.stringify(input)}`);
    }
  }
  const html = markdownToHTML(input, wrap);
  for (const config of [{}, undefined, { markdownAutoWrap: null }]) {
    if (markdownToHTML(input, config) !== html) {
      throw new Error(`markdownToHTML differs without markdownAutoWrap for ${JSON.stringify(input)}`);
    }
  }
  return JSON.stringify([lines, html, markdownToHTML(input, noWrap), nonMarkdownToLines(input), nonMarkdownToHTML(input)]);
};

// isPlain reports whether marked reads the input as one paragraph with one
// text token.
const isPlain = (s) =>
  JSON.stringify(marked.lexer(s)) ===
  JSON.stringify([
    { type: 'paragraph', raw: s, text: s, tokens: [{ type: 'text', raw: s, text: s, escaped: false }] },
  ]);

const groups = inputGroups(REPO);
const inputs = [...groups.written, ...groups.generated];
const cases = [];
let plain = 0;
for (const input of groups.written) {
  const want = run(input);
  if (isPlain(input)) {
    plain++;
    cases.push({ in: input, sum: sum(want) });
  } else {
    cases.push({ in: input, want });
  }
}
// One generated input in four has the values, and the others have the sum.
groups.generated.forEach((input, i) => {
  const want = run(input);
  cases.push(i % 4 === 0 ? { in: input, want } : { in: input, sum: sum(want) });
});
for (const input of surrogateInputs()) cases.push({ units: unitsOf(input), want: run(input) });
for (const file of corpusFiles(REPO)) {
  cases.push({ file, sum: sum(run(fs.readFileSync(path.join(REPO, file), 'utf8'))) });
}

// --- splitTextToChars -------------------------------------------------------

const cp = (...points) => String.fromCodePoint(...points);
const charInputs = new Set([
  '',
  'a',
  'hello world',
  'a\r\nb\rc\nd\n\re',
  '\t\x00\x7f',
  cp(0x65, 0x301), // e + combining acute
  cp(0x65, 0x301, 0x302, 0x303),
  cp(0x301), // a combining mark alone
  cp(0x301, 0x61),
  cp(0x1f600),
  cp(0x1f44d, 0x1f3fd), // emoji + skin tone
  cp(0x1f468, 0x200d, 0x1f469, 0x200d, 0x1f467, 0x200d, 0x1f466), // ZWJ family
  cp(0x1f469, 0x1f3fd, 0x200d, 0x1f4bb), // woman technologist with skin tone
  cp(0x1f3f3, 0xfe0f, 0x200d, 0x1f308), // rainbow flag
  cp(0x1f3f4, 0xe0067, 0xe0062, 0xe0065, 0xe006e, 0xe0067, 0xe007f), // England tag flag
  cp(0x1f1e7, 0x1f1f7), // flag BR
  cp(0x1f1e7, 0x1f1f7, 0x1f1fa, 0x1f1f8), // two flags
  cp(0x1f1e7, 0x1f1f7, 0x1f1fa), // one flag and a half
  cp(0x1f1e7), // a regional indicator alone
  'a' + cp(0x1f1e7, 0x1f1f7) + 'b',
  cp(0x2764, 0xfe0f), // heart + variation selector
  '1' + cp(0xfe0f, 0x20e3), // keycap
  '#' + cp(0x20e3),
  cp(0x200d), // ZWJ alone
  'a' + cp(0x200d) + 'b',
  cp(0x1f600, 0x200d),
  cp(0x200d, 0x1f600),
  cp(0x1f600, 0x200d, 0x61),
  cp(0xa9, 0x200d, 0xae), // extended pictographic in the BMP
  cp(0x4e2d, 0x6587, 0x65e5, 0x672c, 0x8a9e),
  cp(0x304b, 0x3099), // kana + combining voiced mark
  cp(0xff76, 0xff9e), // halfwidth kana + voiced mark
  cp(0x1100, 0x1161, 0x11a8), // Hangul jamo L V T
  cp(0x1100, 0x1100, 0x1161, 0x1161, 0x11a8, 0x11a8),
  cp(0xac00, 0x11a8), // LV + T
  cp(0xac01, 0x11a8), // LVT + T
  cp(0x1161, 0x1100),
  cp(0x915, 0x94d, 0x937), // Devanagari conjunct (GB9c)
  cp(0x915, 0x94d, 0x937, 0x93f),
  cp(0x915, 0x94d, 0x200d, 0x937),
  cp(0x915, 0x93c, 0x94d, 0x937),
  cp(0x915, 0x94d, 0x61),
  cp(0x995, 0x9cd, 0x9b7), // Bengali conjunct
  cp(0xb95, 0xbcd, 0xbb7), // Tamil: virama is not a linker
  cp(0xd15, 0xd4d, 0xd15), // Malayalam conjunct
  cp(0xe01, 0xe33), // Thai + SARA AM (spacing mark)
  cp(0xe01, 0xe34, 0xe48),
  cp(0x600, 0x661), // Arabic number sign (prepend)
  cp(0x600),
  cp(0x600, 0x20),
  cp(0x645, 0x631, 0x62d, 0x628, 0x627),
  cp(0x64e, 0x651), // Arabic marks alone
  cp(0x5d0, 0x5b8, 0x5bc), // Hebrew with points
  cp(0x1f1e6, 0x200d, 0x1f1e7),
  cp(0x61, 0x308, 0x1f600, 0x308),
  cp(0x1d49c), // mathematical script A (non-BMP letter)
  cp(0x1d49c, 0x301),
  cp(0x2028) + 'a' + cp(0x2029) + 'b',
  cp(0xa0) + 'a' + cp(0x3000) + cp(0xfeff),
  cp(0xad) + 'a', // soft hyphen (control)
  'a' + cp(0xad),
  cp(0x200b) + 'a' + cp(0x200c) + 'b' + cp(0x200e),
  cp(0xe0001), // tag character alone
  cp(0x1f600, 0xe0020),
  cp(0xfe0f),
  cp(0x1f9d1, 0x200d, 0x1f91d, 0x200d, 0x1f9d1), // people holding hands
  cp(0x1f468, 0x200d, 0x2764, 0xfe0f, 0x200d, 0x1f48b, 0x200d, 0x1f468), // kiss
  cp(0x1fae9), // Unicode 16 emoji
  cp(0x1face), // Unicode 15 emoji
  cp(0x1f6dc), // Unicode 15 emoji
  cp(0x1cf00, 0x1cf00), // Znamenny combining marks
  cp(0x11f02), // Kawi prepend
  cp(0x11f02, 0x11f04),
  cp(0x16d63, 0x16d67), // Kirat Rai (Unicode 16)
  cp(0x113d1, 0x11380), // Tulu-Tigalari prepend (Unicode 16)
  'Supercalifragilisticexpialidocious-0123456789',
  'x'.repeat(200),
]);
// The inputs that have a character outside of ASCII: each one from the
// specifications and the corpus, and one in four of the generated ones.
groups.written.forEach((input) => {
  if (input.length <= 200 && /[^\x00-\x7f]/.test(input)) charInputs.add(input);
});
groups.generated.forEach((input, i) => {
  if (i % 4 === 0 && input.length <= 200 && /[^\x00-\x7f]/.test(input)) charInputs.add(input);
});
// Random sequences of the code points that the grapheme rules name.
{
  const rand = mulberry32(42);
  const pool = [
    0x61, 0x20, 0x0d, 0x0a, 0x09, 0x301, 0x308, 0x200d, 0xfe0f, 0x20e3, 0x1f600, 0x1f3fd, 0x1f468, 0x1f469, 0x1f1e7,
    0x1f1f7, 0x2764, 0x1100, 0x1161, 0x11a8, 0xac00, 0xac01, 0x915, 0x94d, 0x937, 0x93f, 0x93c, 0xe33, 0x600, 0x4e2d,
    0xa9, 0x23, 0xe0020, 0xe007f, 0x1f3f4, 0xad, 0x200c, 0x903, 0xd4e, 0x1f308, 0x11f02, 0x9cd, 0x995,
  ];
  for (let i = 0; i < 3000; i++) {
    let s = '';
    const n = 1 + Math.floor(rand() * 8);
    for (let j = 0; j < n; j++) s += cp(pool[Math.floor(rand() * pool.length)]);
    charInputs.add(s);
  }
}
const chars = [];
for (const input of charInputs) chars.push({ in: input, want: JSON.stringify(splitTextToChars(input)) });
for (const input of surrogateInputs()) {
  chars.push({ units: unitsOf(input), want: JSON.stringify(splitTextToChars(input)) });
}

// --- splitLineToFitWidth ----------------------------------------------------

// The checkFit functions. internal/markdown/split_text_test.go has the same
// functions.
const fits = {
  // The line has at most n UTF-16 code units, with one more for each gap
  // between two words.
  units: (n) => (line) => line.reduce((sum, w) => sum + w.content.length, 0) + Math.max(line.length - 1, 0) <= n,
  // The line has at most n graphemes.
  graphemes: (n) => (line) => line.reduce((sum, w) => sum + splitTextToChars(w.content).length, 0) <= n,
  // The line has at most n words.
  words: (n) => (line) => line.length <= n,
  never: () => () => false,
  always: () => () => true,
};
const fitSpecs = [
  ['units', 1], ['units', 3], ['units', 6], ['units', 10], ['units', 16], ['units', 40],
  ['graphemes', 1], ['graphemes', 2], ['graphemes', 4], ['graphemes', 7], ['graphemes', 12], ['graphemes', 30],
  ['words', 1], ['words', 2], ['words', 5],
  ['never', 0], ['always', 0],
];

const w = (content, type = 'normal') => ({ content, type });
const lineInputs = [
  [],
  [w('')],
  [w(' ')],
  [w(' '), w(' ')],
  [w('a')],
  [w('a'), w(' '), w('b')],
  [w(' '), w('a')],
  [w('a'), w(' ')],
  [w('a'), w(' '), w(' '), w('b')],
  [w(''), w('a'), w(''), w('b'), w('')],
  [w('Hello'), w('world')],
  [w('Hello', 'strong'), w('big', 'em'), w('world')],
  [w('Supercalifragilisticexpialidocious'), w('is'), w('long')],
  [w('a'), w('Supercalifragilisticexpialidocious', 'em'), w('b')],
  [w('x'.repeat(100), 'strong')],
  [w(cp(0x1f468, 0x200d, 0x1f469, 0x200d, 0x1f467)), w(cp(0x1f1e7, 0x1f1f7, 0x1f1fa, 0x1f1f8)), w(cp(0x65, 0x301, 0x65, 0x301))],
  [w(cp(0x1f600).repeat(9)), w(cp(0x4e2d, 0x6587).repeat(6))],
  [w(cp(0x915, 0x94d, 0x937, 0x93f, 0x915, 0x94d, 0x937)), w(cp(0x1100, 0x1161, 0x11a8, 0xac00))],
  [w('<br/>'), w('<span class="a b">'), w('text'), w('</span>')],
  [w('a\r\nb')],
  [w('line\nbreak')],
  [w('ok'), w('a\nb'), w('ok')],
];
// The lines that markdownToLines and nonMarkdownToLines make for the
// inputs. One input in 25 keeps the file small.
{
  const seen = new Set(lineInputs.map((l) => JSON.stringify(l)));
  const add = (line) => {
    const key = JSON.stringify(line);
    if (line.length > 0 && line.length <= 40 && key.length <= 600 && !seen.has(key)) {
      seen.add(key);
      lineInputs.push(line);
    }
  };
  inputs.forEach((input, i) => {
    if (i % 25 !== 0) return;
    markdownToLines(input, {}).forEach(add);
    nonMarkdownToLines(input).forEach(add);
  });
}

const splits = [];
lineInputs.forEach((line, i) => {
  // Each line runs with three of the checkFit functions, and the first
  // lines run with all of them.
  const specs = i < 22 ? fitSpecs : [0, 1, 2].map((k) => fitSpecs[(i * 5 + k * 7) % fitSpecs.length]);
  for (const [kind, n] of specs) {
    const calls = [];
    const checkFit = (l) => {
      const ok = fits[kind](n)(l);
      calls.push(JSON.stringify(l));
      return ok;
    };
    const item = { line: JSON.stringify(line), fit: kind, n };
    try {
      // The function consumes its argument, so it gets a copy.
      item.want = JSON.stringify(splitLineToFitWidth(structuredClone(line), checkFit));
    } catch (e) {
      item.error = String(e.message);
    }
    // The lines that checkFit got, in order: their number and their sum.
    item.calls = calls.length;
    item.callsSum = sum(JSON.stringify(calls));
    splits.push(item);
  }
});

const write = (name, value, count) => {
  const file = path.join(REPO, 'internal/markdown/testdata', name);
  const data = zlib.gzipSync(JSON.stringify(value), { level: 9 });
  fs.writeFileSync(file, data);
  console.log(`wrote ${file}: ${count} cases, ${data.length} bytes`);
};
write('markdown.json.gz', cases, `${cases.length} (${plain} plain)`);
write('split.json.gz', { chars, splits }, `${chars.length} + ${splits.length}`);
