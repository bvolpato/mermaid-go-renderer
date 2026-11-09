// Writes internal/diagrams/agentflow/jsyaml/testdata/yaml.json.gz: the
// results of yaml.load(text, { schema: yaml.JSON_SCHEMA }) of the js-yaml
// that mermaid 12.2.0 bundles, for hand-written YAML texts, for the shape
// data of the agentflow fixtures, and for seeded random texts and
// mutations. A text that js-yaml rejects has the reason, the message and
// the mark of the YAMLException.
//
// The Go test of internal/diagrams/agentflow/jsyaml replays the texts.
//
// Usage (in tools/reference): node gen-agentflow-yaml.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/diagrams/agentflow/jsyaml/testdata/yaml.json.gz');
const chunks = path.join(here, 'node_modules/mermaid/dist/chunks/mermaid.core');

// The chunk of mermaid that has js-yaml exports load and JSON_SCHEMA.
const chunk = fs
  .readdirSync(chunks)
  .filter((f) => f.endsWith('.mjs'))
  .find((f) => {
    const src = fs.readFileSync(path.join(chunks, f), 'utf8');
    return src.includes('js-yaml/dist/js-yaml.mjs') && /export \{[^}]*\bJSON_SCHEMA\b[^}]*\bload\b/.test(src);
  });
if (!chunk) {
  throw new Error('no chunk of mermaid exports the loader of js-yaml');
}
const { load, JSON_SCHEMA } = await import(pathToFileURL(path.join(chunks, chunk)).href);

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

const hasLoneSurrogate = (s) => /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?:[^\uD800-\uDBFF]|^)[\uDC00-\uDFFF]/.test(s);
const encodeString = (s) => (hasLoneSurrogate(s) ? { $s: Array.from({ length: s.length }, (_, i) => s.charCodeAt(i)) } : s);

// encode keeps what JSON.stringify loses: undefined is {"$u":1}, a number
// that is not finite and -0 are {"$n":"<text>"}, a string with a lone
// surrogate is {"$s":[code units]}, and an object is {"$o":[[key, value]]},
// so that the order and the names of its keys stay. A document that
// contains itself is {"$cyclic":1}.
function encode(v, stack = []) {
  if (v === undefined) {
    return { $u: 1 };
  }
  if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
    return { $n: Object.is(v, -0) ? '-0' : String(v) };
  }
  if (typeof v === 'string') {
    return encodeString(v);
  }
  if (v === null || typeof v !== 'object') {
    return v;
  }
  if (stack.includes(v)) {
    throw new Error('cyclic');
  }
  stack.push(v);
  let out;
  if (Array.isArray(v)) {
    out = v.map((e) => encode(e, stack));
  } else {
    out = { $o: Object.keys(v).map((k) => [encodeString(k), encode(v[k], stack)]) };
  }
  stack.pop();
  return out;
}

const run = (input) => {
  try {
    const doc = load(input, { schema: JSON_SCHEMA });
    try {
      return { input: encodeString(input), result: encode(doc) };
    } catch (e) {
      if (e.message === 'cyclic') {
        return { input: encodeString(input), cyclic: true };
      }
      throw e;
    }
  } catch (e) {
    if (e.name !== 'YAMLException') {
      throw e;
    }
    const rec = { input: encodeString(input), error: { reason: encodeString(e.reason), message: encodeString(e.message) } };
    if (e.mark) {
      rec.error.mark = { line: e.mark.line, column: e.mark.column, position: e.mark.position, buffer: encodeString(e.mark.buffer), snippet: e.mark.snippet === null ? null : encodeString(e.mark.snippet) };
    }
    return rec;
  }
};

// ---------------------------------------------------------------------------
// Hand-written texts.

const texts = [
  '',
  ' ',
  '   ',
  '\n',
  '\n\n',
  '  \n',
  '# comment',
  '# comment\n',
  '---',
  '---\n',
  '--- \n...\n',
  '...',
  '---\n---\n',
  'a\n---\nb\n',
  '--- a\n--- b',
  '~',
  'null',
  'Null',
  'NULL',
  'nUll',
  'true',
  'True',
  'TRUE',
  'false',
  'False',
  'FALSE',
  'yes',
  'no',
  'on',
  'off',
  '0',
  '-0',
  '+0',
  '7',
  '-7',
  '+7',
  '012',
  '0b101',
  '-0b101',
  '0b',
  '0b12',
  '0x1F',
  '0xfF',
  '-0x1f',
  '0x',
  '0xg',
  '0o17',
  '0o8',
  '0o',
  '1_000',
  '123456789012345678901234567890',
  '0x' + 'f'.repeat(400),
  '9'.repeat(400),
  '1.5',
  '-1.5',
  '+1.5',
  '1.',
  '.5',
  '-.5',
  '1e3',
  '1E3',
  '1e+3',
  '1.5e-3',
  '1e400',
  '-1e400',
  '.inf',
  '-.inf',
  '+.inf',
  '.Inf',
  '.INF',
  '.nan',
  '.NaN',
  '.NAN',
  '-.nan',
  '.iNf',
  '-0.0',
  '0.0',
  '1e',
  '.e3',
  '.',
  '-',
  '- ',
  '-\n',
  '+',
  '0.1.2',
  '2001-01-01',
  '12:30:45',
  'plain text',
  'plain: text',
  'key: value\nother: 5\n',
  'a: 1\nb: true\nc: null\nd: ~\ne: text\nf: "quoted"\ng: \'single\'\nh:\n',
  'a:\n  b:\n    c: deep\n  d: 2\ne: 3\n',
  'a: b: c',
  'a:\n- 1\n- 2\nb:\n  - 3\n  - 4\n',
  '- a\n- b\n- - c\n  - d\n- e: f\n  g: h\n',
  '-\n- a\n-\n',
  '- - - a\n',
  '[a, b, c]',
  '[a, b, c,]',
  '[]',
  '{}',
  '[ ]',
  '{ }',
  '[[]]',
  '[{}]',
  '{a: []}',
  '[a, [b, [c, [d]]], {e: {f: [g]}}]',
  '{a: 1, b: 2}',
  '{a: 1, b: 2,}',
  '{a: 1,, b: 2}',
  '[a,, b]',
  '[,]',
  '{,}',
  '{a}',
  '{a, b: c}',
  '{a:}',
  '{a: , b: }',
  '{: a}',
  '{:}',
  '{ : }',
  '[: a]',
  '[a: b]',
  '[a: b, c: d]',
  '[? a : b]',
  '[? a]',
  '{? a : b}',
  '{? a}',
  '{? }',
  '{a: b c}',
  '{a: "x" y}',
  '{a: [1, 2}',
  '{a: 1',
  '[a, b',
  '[a b]',
  '[a\nb]',
  '{a: 1\nb: 2}',
  '{a: 1,\n b: 2\n}',
  '{\na: 1,\nb: 2\n}',
  '{\n  shape: tool, label: "x"\n}',
  '{\nshape: tool\n}',
  '{\n\n}',
  '{\n \n}',
  '{\na: "x" y\n}',
  '{\na: 1, a: 2\n}',
  '{\na: [1, 2\n}',
  '{\na: b: c\n}',
  '{\n: \n}',
  '{\nkey: value: more\n}',
  '{\nurl: http://example.com/a?b=c\n}',
  '{\nlabel: a, b\n}',
  '{\na: -, b: - x\n}',
  '{\na: ? b\n}',
  '{\na: "b", "c": d, \'e\': f\n}',
  '{\n"a b": 1, ? c d : 2\n}',
  '{\na: &x 1, b: *x\n}',
  '{\na: *nope\n}',
  '{\na: &x [1, 2], b: *x, c: &x 3, d: *x\n}',
  '{\na: !!str 5, b: !!int "5", c: !!float 1, d: !!bool "true", e: !!null "~"\n}',
  '{\na: !!null ""\n}',
  '{\na: !!int x\n}',
  '{\na: !!bool 1\n}',
  '{\na: !!float x\n}',
  '{\na: !!str\n}',
  '{\na: !!int\n}',
  '{\na: !!seq, b: !!map, c: !!null\n}',
  '{\na: !!seq [1], b: !!map {c: 1}\n}',
  '{\na: !!seq {b: 1}\n}',
  '{\na: !!map [1]\n}',
  '{\na: !!str [1]\n}',
  '{\na: !!set {b}\n}',
  '{\na: !!binary aGVsbG8=\n}',
  '{\na: !!timestamp 2001-01-01\n}',
  '{\na: !!merge <<\n}',
  '{\na: !local x\n}',
  '{\na: ! x\n}',
  '{\na: !<tag:yaml.org,2002:str> 5\n}',
  '{\na: !<tag:yaml.org,2002:int> 5\n}',
  '{\na: !<> 5\n}',
  '{\na: !<?> 5\n}',
  '{\na: !<?> [5]\n}',
  '{\na: !<!> 5\n}',
  '{\na: !<unclosed 5\n}',
  '{\na: !!str !!int 5\n}',
  '{\na: &x &y 5\n}',
  '{\na: & 5\n}',
  '{\na: * \n}',
  '{\na: !!str *x\n}',
  '{\na: !e!x 5\n}',
  '{\na: !e!x!y 5\n}',
  '{\na: !e x! 5\n}',
  '{\na: !!x{y 5\n}',
  '{\na: !x%zz 5\n}',
  '{\na: !x%ff 5\n}',
  '{\na: !x%41%c3%a9 5\n}',
  '{\na: !!s%74r 5\n}',
  '{\na: !! 5\n}',
  '<<: {a: 1}\nb: 2\n',
  'base: &b {a: 1}\nd:\n  <<: *b\n  c: 2\n',
  'a: &a [1, *a]\n',
  '&a [*a]',
  '&a {b: *a}',
  '&a\nb: *a\n',
  'a: &x\n  b: 1\nc: *x\nd: [*x, *x]\n',
  '&x a: 1\n',
  '&x\n&y a: 1\n',
  '!!map\na: 1\n',
  '!!map\n!!str a: 1\n',
  '!!str\n!!str a: 1\n',
  '&a !!str x',
  '!!str &a x',
  '&a',
  '!!str',
  '!!int',
  '!!null',
  'a: !!str\nb: 1\n',
  'a: &x\nb: *x\n',
  '"double"',
  '"with \\n \\t \\" \\\\ \\/ escapes"',
  '"\\0\\a\\b\\e\\v\\f\\r\\ \\N\\_\\L\\P"',
  '"\\x41\\u00e9\\U0001F600"',
  '"\\uD83D"',
  '"\\uDE00\\uD83D"',
  '"\\UFFFFFFFF"',
  '"\\U00110000"',
  '"\\x4"',
  '"\\xZZ"',
  '"\\q"',
  '"\\',
  '"unterminated',
  '"multi\n  line\n\n  folded"',
  '"trailing space   \n   leading"',
  '"escaped \\\n   newline"',
  '"a\n---\nb"',
  '"tab\there"',
  "'single'",
  "'it''s'",
  "''",
  "''''",
  "'unterminated",
  "'multi\n  line\n\n  folded'",
  "'a\n...\nb'",
  "'a' b",
  '"a" b',
  '"a": b',
  "'a': b",
  '"a":b',
  'a:b',
  'a :b',
  'a : b',
  'a:\tb',
  'a: b # comment',
  'a: b#not comment',
  'a: #comment\n  b',
  '#c\na: 1 #c\n#c\nb: 2\n',
  'a: |\n  literal\n  block\n',
  'a: |-\n  strip\n\n',
  'a: |+\n  keep\n\n\nb: 1\n',
  'a: >\n  folded\n  text\n\n  para\n',
  'a: >-\n  folded\n   more indented\n  back\n',
  'a: >+\n  x\n\n',
  'a: |2\n    indented\n',
  'a: |1\n  x\n',
  'a: |0\n  x\n',
  'a: |12\n  x\n',
  'a: |+-\n  x\n',
  'a: |-2\n   x\n',
  'a: | # comment\n  x\n',
  'a: |x\n  y\n',
  'a: |\nb: 1\n',
  'a: |\n',
  'a: |',
  'a: >',
  '|\n text\n',
  '>\n text\n more\n',
  '|\ntext',
  '- |\n  a\n- >\n  b\n',
  'a: |\n  x\n y\n',
  'a: |\n\n\n  after blanks\n\n\n',
  'a: >\n\n  x\n\n\n  y\n   z\n\n  w\n',
  'a: |\n  \ttab\n',
  'a: >\n \tmore\n x\n',
  'a: multi\n  line plain\n  scalar\nb: 1\n',
  'a: multi\n\n  para\n',
  'multi\nline\n\ntop',
  'a\n b\n  c\n',
  'a: 1\n b: 2\n',
  'a: 1\n  b: 2\n',
  '  a: 1\n  b: 2\n',
  '  a: 1\n b: 2\n',
  ' a: 1\nb: 2\n',
  'a:\n  b: 1\n c: 2\n',
  'a:\n  - 1\n - 2\n',
  '- a\n - b\n',
  '- a\n-b\n',
  '-a\n',
  '- a\nb: 1\n',
  'a: 1\n- b\n',
  'a: 1\nb\n',
  'a: 1\nb\nc: 2\n',
  'a\nb: 1\n',
  'a: "x" y\n',
  'a: [1, 2\n',
  'a: 1\na: 2\n',
  'a: 1\nb: 2\na: 3\n',
  '1: a\n1: b\n',
  '1: a\n"1": b\n',
  'true: a\nTrue: b\n',
  '~: a\nnull: b\n',
  'null: a\n~: b\n',
  '2: two\n1: one\nb: bee\n0: zero\na: ay\n-1: neg\n1.5: float\n01: lead\n',
  '4294967294: a\n4294967295: b\n4294967296: c\n',
  '__proto__: 1\nconstructor: 2\ntoString: 3\nhasOwnProperty: 4\n',
  '__proto__:\n  a: 1\n__proto__x: 2\n',
  '{\n__proto__: {x: 1}, __proto__: 2\n}',
  '[a]: b\n',
  '[a, b]: c\n',
  '[[a]]: b\n',
  '[{a: 1}]: b\n',
  '{a: 1}: b\n',
  '{a: 1}: b\n{c: 2}: d\n',
  '? a\n: b\n',
  '? a\n',
  '? a\n? b\n',
  '? - a\n  - b\n: c\n',
  '? |\n  block key\n: v\n',
  ': a\n',
  '?\n',
  '? a\n: b\n: c\n',
  'a: b\n? c\n: d\ne: f\n',
  '? a: b\n: c\n',
  '\ta: 1\n',
  'a:\n\tb: 1\n',
  'a: 1\n\tb: 2\n',
  'a:\t1\n',
  'a: 1\t\n',
  '-\ta\n',
  '- a\n\t- b\n',
  'a:\n  -\tb\n',
  '\t',
  '\t# comment\n',
  '\t[a]\n',
  '\t"a"\n',
  'a: [\n\t1\n]\n',
  '%YAML 1.2\n---\na: 1\n',
  '%YAML 1.1\n---\na: 1\n',
  '%YAML 1.3\n---\na\n',
  '%YAML 2.0\n---\na\n',
  '%YAML 1.2\n%YAML 1.2\n---\na\n',
  '%YAML\n---\na\n',
  '%YAML 1.2 3\n---\na\n',
  '%YAML x\n---\na\n',
  '%YAML 1.2\na: 1\n',
  '%TAG !e! tag:example.com,2000:\n---\n!e!x a\n',
  '%TAG !e! tag:yaml.org,2002:\n---\n!e!int 5\n',
  '%TAG ! tag:yaml.org,2002:\n---\n!int 5\n',
  '%TAG !! tag:example.com,2000:\n---\n!!int 5\n',
  '%TAG !e!\n---\na\n',
  '%TAG e tag:x\n---\na\n',
  '%TAG !e! {x\n---\na\n',
  '%TAG !e! tag:x%zz\n---\na\n',
  '%TAG !e! tag:x%ff\n---\na\n',
  '%TAG !e! a\n%TAG !e! b\n---\na\n',
  '%FOO bar baz # comment\n---\na\n',
  '%\n---\na\n',
  '% YAML\n---\na\n',
  ' %YAML 1.2\n---\na\n',
  '%YAML 1.2 # comment\n---\na\n',
  '%YAML 1.2',
  'a: 1\n...\nb: 2\n',
  'a: 1\n... # end\n',
  'a: 1\n...\n',
  'a: 1\n---\n',
  'a: 1\n--- b\n',
  '---a\n',
  '--- |\n  x\n',
  '--- >\n x\n',
  '---\n# only a comment\n',
  '\ufeffa: 1\n',
  '\ufeff',
  'a: \ufeffb\n',
  'a: 1\r\nb: 2\r\n',
  'a: 1\rb: 2\r',
  'a: |\r\n  x\r\n  y\r\n',
  'a: "x\r\n  y"\r\n',
  'a: 1\0\n',
  '\0',
  'a: \x01\n',
  'a: "\x01"\n',
  'a: \'\x01\'\n',
  'a: \x7f\n',
  'a: \x85b\n',
  'a: \u2028b\n',
  'a: "\u2028"\n',
  'a: \ufffe\n',
  'a: 😀\n😀: b\n',
  'é: ü\n日本: 語\n',
  'a: "é😀"\n',
  'a: |\n  é😀\n',
  '😀😀😀: "x" y\n',
  'ééé: [1\n',
  'a: @b\n',
  'a: `b\n',
  'a: %b\n',
  'a: b@c\n',
  'a: ,b\n',
  'a: [b]c\n',
  'a: {b}c\n',
  'a: ]\n',
  'a: }\n',
  ']',
  '}',
  ',',
  'a: b, c\n',
  'a: b]c\n',
  'a: -\n',
  'a: - b\n',
  'a: -b\n',
  'a: ?b\n',
  'a: ? b\n',
  'a: :b\n',
  'a: : b\n',
  'a::\n',
  'a:: b\n',
  ':a\n',
  ': \n',
  ':',
  '?',
  '? ',
  'a: !\n',
  'a: &\n',
  'a: *\n',
  'a: |\n  x\nb: >\n  y\nc: "z"\n',
  'key with spaces: value with spaces\n',
  '  leading: spaces\n',
  'trailing: spaces   \n',
  'a:    b   \n',
  'a: b   # c\n',
  'a:\n\n\n  b: 1\n\n\n',
  'a: 1\n\n\nb: 2\n\n\n',
  'a: [1, 2] # c\nb: {c: d} # e\n',
  'a: [1, 2] x\n',
  'a: {b: c} x\n',
  '[1, 2] x\n',
  '[1, 2]: x\n',
  '{a: 1}: x\n',
  '[1, 2]\n[3]\n',
  '{a: 1}\n{b: 2}\n',
  'a: [b: c, d]\n',
  'a: [b, c: d, e: [f, g: h]]\n',
  'a: {b: [c, {d: e}], f: g}\n',
  'a: [[[[[[[[[[1]]]]]]]]]]\n',
  '['.repeat(99) + ']'.repeat(99),
  '['.repeat(100) + ']'.repeat(100),
  '['.repeat(101) + ']'.repeat(101),
  '{a: '.repeat(60) + '1' + '}'.repeat(60),
  Array.from({ length: 105 }, (_, i) => ' '.repeat(i) + '- ').join('\n') + 'x\n',
  Array.from({ length: 60 }, (_, i) => ' '.repeat(i) + 'k:').join('\n') + ' v\n',
  'a: ' + 'x'.repeat(200) + ' "y\n',
  'aaaa: 1\n' + 'b'.repeat(120) + ': [\n',
  'k: ' + 'w '.repeat(60) + '"x" y ' + 'z '.repeat(60) + '\n',
  'l1: 1\nl2: 2\nl3: 3\nl4: 4\nl5: [\nl6: 6\nl7: 7\nl8: 8\n',
  'l1: 1\nl2: 2\nl3: 3\nl4: 4\nl5: 5\nl6: 6\nl7: 7\nl8: 8\nl9: 9\nl10: "x" y\nl11: 11\nl12: 12\n',
  '\tx: "a\tb" c\n',
  'x: "a\tb" c\td\n',
  // The texts that the agentflow db builds: the single-line wrapper and the
  // multi-line body.
  '{\n shape: tool \n}',
  '{\n view: "collapsed" \n}',
  '{\n protocol: "http", endpoint: "https://api.github.com", token_required: true \n}',
  '{\n shape: tool, connectorRef: "llm.chat", returns: "Intent" \n}',
  '{\n label: "Multi<br/>line<br/>label" \n}',
  '{\n   \n}',
  '{\n\n}',
  '\n  shape: tool\n  label: "Block yaml"\n\n',
  '\n  shape: tool,\n  label: "x",\n\n',
  '\n\n',
  '\n  just text\n\n',
  '\n  - a\n  - b\n\n',
  ' shape: tool\n  label: "x"\n\n',
  ' shape: tool,\n label: x\n\n',
  '\n  label: "x",\n  bad: [1, 2\n\n',
  '\n  a: 1\n   b: 2\n\n',
  '\n\ta: 1\n\n',
];

// The shape data of the corpus fixtures of agentflow.
const corpus = path.join(repoRoot, 'testdata/corpus/mermaid-e2e/agentflow');
const shapeData = new Set();
for (const f of fs.readdirSync(corpus, { recursive: true })) {
  if (!String(f).endsWith('.mmd')) {
    continue;
  }
  const text = fs.readFileSync(path.join(corpus, String(f)), 'utf8');
  for (const m of text.matchAll(/@\{([^}]*)\}/g)) {
    const body = m[1];
    shapeData.add(body.includes('\n') ? body + '\n' : '{\n' + body + '\n}');
  }
}
texts.push(...[...shapeData].sort());

// ---------------------------------------------------------------------------
// Seeded random texts: a soup of the tokens that the loader reacts to, and
// mutations of the texts above.

const tokens = [
  'a', 'b', 'key', 'value', '1', '0', '-1', '1.5', 'true', 'null', '~', ' ', ' ', ' ', '  ', '\n', '\n', '\n  ', '\n    ', '\n- ', '\t',
  ': ', ': ', ':', ', ', ',', '[', ']', '{', '}', '"', "'", '#', ' #', '- ', '-', '? ', '?', '|', '>', '|-', '>+', '&x ', '*x', '!!str ',
  '!!int ', '!', '!!', '%', '@', '`', '---', '...', '\\', '\\n', '"x"', "'y'", 'é', '😀', '<<', '__proto__', '\r\n', '\r', '.inf', '0x1F', '0o7', '=',
];
const rnd = prng(0x79616d6c);
const pick = (list) => list[Math.floor(rnd() * list.length)];
for (let i = 0; i < 900; i++) {
  const n = 1 + Math.floor(rnd() * 14);
  let s = '';
  for (let k = 0; k < n; k++) {
    s += pick(tokens);
  }
  // Half of the texts are in the wrapper of the single-line shape data.
  texts.push(rnd() < 0.5 ? '{\n' + s.replace(/\n/g, ' ') + '\n}' : s);
}
const base = texts.slice(0, texts.length - 900).filter((t) => t.length > 0 && t.length < 200);
for (let i = 0; i < 900; i++) {
  let s = pick(base);
  const edits = 1 + Math.floor(rnd() * 3);
  for (let k = 0; k < edits; k++) {
    const at = Math.floor(rnd() * (s.length + 1));
    const kind = Math.floor(rnd() * 4);
    if (kind === 0) {
      s = s.slice(0, at) + s.slice(at + 1 + Math.floor(rnd() * 3));
    } else if (kind === 1 || kind === 2) {
      s = s.slice(0, at) + pick(tokens) + s.slice(at);
    } else {
      s = s.slice(0, at);
    }
  }
  texts.push(s);
}

const cases = texts.map(run);
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases }), { level: 9 }));
const errors = cases.filter((c) => c.error).length;
const cyclic = cases.filter((c) => c.cyclic).length;
const reasons = new Set(cases.filter((c) => c.error).map((c) => (typeof c.error.reason === 'string' ? c.error.reason.replace(/"[^"]*"/g, '""').replace(/<[^>]*>/g, '<>') : '')));
console.log(`${outFile}: ${cases.length} cases, ${errors} with an error (${reasons.size} kinds of reasons), ${cyclic} cyclic`);
