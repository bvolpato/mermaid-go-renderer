#!/usr/bin/env node
// Writes internal/diagrams/architecture/testdata/ast.json.gz: parser goldens
// for the architecture diagram from the real @mermaid-js/parser (the Langium
// grammar packages/parser/src/language/architecture/architecture.langium).
//
// Each case is a text with the AST of parse('architecture', text), or the
// fact that the parser fails. The texts are the corpus fixtures,
// hand-written inputs for the branches of the grammar and of the value
// converters, and mutated inputs from a seeded generator.
//
// Run it with tools/reference as the working directory:
//
//   node gen-architecture-ast.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { parse } from '@mermaid-js/parser';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e/architecture');
const OUT = path.join(ROOT, 'internal/diagrams/architecture/testdata/ast.json.gz');
const MUTANTS = 2400;

// mulberry32, the same generator as gen-jsnum.mjs.
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

// plain is the AST without the properties of Langium ($container, $cstNode
// and so on). JSON leaves out a property that is undefined.
function plain(ast) {
  const out = {};
  for (const key of ['title', 'accTitle', 'accDescr']) {
    if (ast[key] !== undefined) out[key] = ast[key];
  }
  const pick = (node, keys) => {
    const o = {};
    for (const k of keys) {
      if (node[k] !== undefined) o[k] = node[k];
    }
    return o;
  };
  out.groups = ast.groups.map((g) => pick(g, ['id', 'icon', 'title', 'in']));
  out.services = ast.services.map((s) => pick(s, ['id', 'iconText', 'icon', 'title', 'in']));
  out.junctions = ast.junctions.map((j) => pick(j, ['id', 'in']));
  out.edges = ast.edges.map((e) =>
    pick(e, ['lhsId', 'lhsGroup', 'lhsDir', 'lhsInto', 'title', 'rhsInto', 'rhsDir', 'rhsId', 'rhsGroup'])
  );
  out.alignments = ast.alignments.map((a) => ({ direction: a.direction, members: [...a.members] }));
  for (const [list, type] of [
    [ast.groups, 'Group'],
    [ast.services, 'Service'],
    [ast.junctions, 'Junction'],
    [ast.edges, 'Edge'],
    [ast.alignments, 'Alignment'],
  ]) {
    for (const node of list) {
      if (node.$type !== type) throw new Error(`unexpected node type ${node.$type}`);
    }
  }
  return out;
}

async function run(text) {
  try {
    return { text, ast: plain(await parse('architecture', text)) };
  } catch {
    return { text, error: true };
  }
}

// encodeEntities is the function of utils.ts.
function encodeEntities(text) {
  let txt = text;
  txt = txt.replace(/style.*:\S*#.*;/g, (s) => s.substring(0, s.length - 1));
  txt = txt.replace(/classDef.*:\S*#.*;/g, (s) => s.substring(0, s.length - 1));
  txt = txt.replace(/#\w+;/g, (s) => {
    const innerTxt = s.substring(1, s.length - 1);
    const isInt = /^\+?\d+$/.test(innerTxt);
    return isInt ? 'ﬂ°°' + innerTxt + '¶ß' : 'ﬂ°' + innerTxt + '¶ß';
  });
  return txt;
}

// The text that mermaid gives to the parser: without the front matter and
// the comments, and with a line break at the end.
function prepared(source) {
  let text = source.replace(/\r\n?/g, '\n').replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, '');
  text = text.replace(/\s*%%.*\n/gm, '').trimStart();
  return encodeEntities(text) + '\n';
}

function* walk(dir) {
  for (const e of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (fs.statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.mmd')) yield p;
  }
}

const texts = [];
const fixtures = [];
for (const file of walk(CORPUS)) {
  const source = fs.readFileSync(file, 'utf8');
  fixtures.push(prepared(source));
  // The parser itself also takes the front matter and comments.
  texts.push(source);
}
texts.push(...fixtures);

const a = (body) => 'architecture-beta\n' + body;

// Hand-written inputs: one for each branch of the grammar and of the value
// converters, and the inputs on which the lexer decides.
texts.push(
  '',
  '\n',
  'architecture-beta',
  'architecture-beta\n',
  '\n\n  architecture-beta  \n\n',
  'architecture',
  'architecture\nservice a\n',
  'architecture-betax\nservice a\n',
  'architecture-beta-x\nservice a\n',
  'architecture-beta service a',
  'architecture-beta service a service b',
  'architecture-beta%% c\nservice a\n',
  'architecture-beta %% c\nservice a %% d\n',
  '%% c\narchitecture-beta\nservice a\n',
  '---\ntitle: x\n---\narchitecture-beta\nservice a\n',
  '%%{init: {"theme":"dark"}}%%\narchitecture-beta\nservice a\n',
  'Architecture-beta\nservice a\n',
  'architecture-beta\r\nservice a\r\nservice b\r\n',
  'architecture-beta\u00a0\nservice a\n',
  // Title and accessibility.
  a('title T\naccTitle: A\naccDescr: D\nservice a\n'),
  a('title T\n\n\naccTitle: A'),
  a('accDescr {\n  multi\n   line\n}\nservice a\n'),
  a('service a\ntitle late\naccTitle: late too\n'),
  a('title a\ntitle b\n'),
  a('title\n'),
  a('title'),
  a('titlex\n'),
  a('title a %% c\n'),
  a('accTitle:   a   b  %% c\n'),
  a('accDescr {a} service b\n'),
  a('accDescr {a}\nservice b\n'),
  a('service a title x\n'),
  a('service title\n'),
  a('service title1\n'),
  a('service accTitle\n'),
  a('service atitle\n'),
  'architecture-beta title x\nservice a\n',
  // Groups.
  a('group g\n'),
  a('group g'),
  a('group g(cloud)\n'),
  a('group g[Title]\n'),
  a('group g(cloud)[Title]\n'),
  a('group g[Title](cloud)\n'),
  a('group g(cloud)[Title] in h\n'),
  a('group g in h\n'),
  a('group g in\n'),
  a('group g in h i\n'),
  a('group\n'),
  a('group (cloud)\n'),
  a('group g (cloud) [Title] in h\n'),
  a('group g(cloud)(server)\n'),
  a('group g[a][b]\n'),
  a('group g "text"\n'),
  a('group g\ngroup h in g\ngroup i in h\n'),
  a('group g group h\n'),
  a('groupg\n'),
  a('group group\n'),
  a('group groups\n'),
  a('group in\n'),
  a('group inner in outer\n'),
  // Services.
  a('service s\n'),
  a('service s'),
  a('service s(server)\n'),
  a('service s[Title]\n'),
  a('service s(server)[Title]\n'),
  a('service s(server)[Title] in g\n'),
  a('service s "txt"\n'),
  a('service s "txt"[Title]\n'),
  a('service s "txt" [Title] in g\n'),
  a("service s 'txt'\n"),
  a('service s ""\n'),
  a('service s "a\\"b\\n\\t\\\\c\\x"\n'),
  a('service s "txt"(server)\n'),
  a('service s (server)"txt"\n'),
  a('service s "a" "b"\n'),
  a('service s "unterminated\n'),
  a('service s "multi\nline"\n'),
  a('service s in g\n'),
  a('service s in\n'),
  a('service\n'),
  a('service s t\n'),
  a('service 1\n'),
  a('service 1a\n'),
  a('service _a\n'),
  a('service a-b\n'),
  a('service a-\n'),
  a('service -a\n'),
  a('service a_b-c_d\n'),
  a('service a.b\n'),
  a('service é\n'),
  a('service s(é)\n'),
  a('service s[é 日本語 😀]\n'),
  // Keywords and terminals that win over ID.
  a('service in\n'),
  a('service row\n'),
  a('service column\n'),
  a('service align\n'),
  a('service junction\n'),
  a('service service\n'),
  a('service group\n'),
  a('service rows\n'),
  a('service columns\n'),
  a('service aligned\n'),
  a('service junctions\n'),
  a('service services\n'),
  a('service L\n'),
  a('service R\n'),
  a('service T\n'),
  a('service B\n'),
  a('service Left\n'),
  a('service Right\n'),
  a('service Top\n'),
  a('service Bottom\n'),
  a('service aL\n'),
  a('service lb\n'),
  a('service architecture-beta\n'),
  a('service architecture-beta2\n'),
  a('architecture-beta\n'),
  // Icons.
  a('service s()\n'),
  a('service s( )\n'),
  a('service s(a b)\n'),
  a('service s(a:b)\n'),
  a('service s(a:b-c_d)\n'),
  a('service s(logos:aws-s3)\n'),
  a('service s(:::)\n'),
  a('service s(-)\n'),
  a('service s((a))\n'),
  a('service s(a\n'),
  a('service s(a.b)\n'),
  a('service s(a/b)\n'),
  // Titles.
  a('service s[]\n'),
  a('service s[ ]\n'),
  a('service s[  spaced   title  ]\n'),
  a('service s[""]\n'),
  a("service s['']\n"),
  a('service s["]\n'),
  a("service s[']\n"),
  a('service s["quoted"]\n'),
  a("service s['quoted']\n"),
  a('service s["with ] bracket"]\n'),
  a('service s["with [ bracket"]\n'),
  a("service s['with ] bracket']\n"),
  a('service s["esc \\" quote"]\n'),
  a("service s['esc \\' quote']\n"),
  a('service s["esc \\\' other"]\n'),
  a('service s["a\\\\b"]\n'),
  a('service s["a\\nb"]\n'),
  a('service s[ "padded" ]\n'),
  a('service s["a" b]\n'),
  a('service s[a "b"]\n'),
  a('service s["a" "b"]\n'),
  a('service s["a\'b"]\n'),
  a('service s[\'a"b\']\n'),
  a('service s["mixed\']\n'),
  a('service s[a]b]\n'),
  a('service s[a[b]\n'),
  a('service s[[a]]\n'),
  a('service s[a\nb]\n'),
  a('service s["a\nb"]\n'),
  a('service s[a (b) {c} <d> :e -f]\n'),
  a('service s[#35; ent #lt;b#gt;]\n'),
  a('service s[<b>bold</b>]\n'),
  a('service s[title]\n'),
  a('service s[accTitle: x]\n'),
  a('service s[%% not a comment]\n'),
  a('service s[a %% b]\n'),
  // Junctions.
  a('junction j\n'),
  a('junction j'),
  a('junction j in g\n'),
  a('junction j in\n'),
  a('junction\n'),
  a('junction j(server)\n'),
  a('junction j[Title]\n'),
  a('junction j k\n'),
  // Edges.
  a('a:L -- R:b\n'),
  a('a:L -- R:b'),
  a('a:L--R:b\n'),
  a('a : L -- R : b\n'),
  a('a:L --> R:b\n'),
  a('a:L <-- R:b\n'),
  a('a:L <--> R:b\n'),
  a('a:L>--<R:b\n'),
  a('a:L >--> R:b\n'),
  a('a:L <<--> R:b\n'),
  a('a:L -[Label]- R:b\n'),
  a('a:L-[Label]-R:b\n'),
  a('a:L <-[Label]-> R:b\n'),
  a('a:L -["quoted label"]- R:b\n'),
  a('a:L -[ ]- R:b\n'),
  a('a:L -[]- R:b\n'),
  a('a:L -[a]-- R:b\n'),
  a('a:L --[a]- R:b\n'),
  a('a:L --[a]-- R:b\n'),
  a('a:L - R:b\n'),
  a('a:L --- R:b\n'),
  a('a:L ---- R:b\n'),
  a('a:L - - R:b\n'),
  a('a:L -(icon)- R:b\n'),
  a('a{group}:L -- R:b\n'),
  a('a:L -- R:b{group}\n'),
  a('a{group}:L -- R:b{group}\n'),
  a('a {group} :L -- R:b {group}\n'),
  a('a{group}{group}:L -- R:b\n'),
  a('a{groups}:L -- R:b\n'),
  a('a{ group }:L -- R:b\n'),
  a('a:L{group} -- R:b\n'),
  a('a:T -- B:b\n'),
  a('a:B -- T:b\n'),
  a('a:R -- L:b\n'),
  a('a:L -- L:b\n'),
  a('a:T -- L:b\n'),
  a('a:X -- R:b\n'),
  a('a:l -- r:b\n'),
  a('a:LR -- R:b\n'),
  a('a:L -- RL:b\n'),
  a('a:L -- :b\n'),
  a('a: -- R:b\n'),
  a('a:L -- R:\n'),
  a(':L -- R:b\n'),
  a('a -- b\n'),
  a('a:L -- R b\n'),
  a('a L -- R:b\n'),
  a('a:L -- R:b c:L -- R:d\n'),
  a('a:L -- R:b\nb:T -- B:c\n'),
  a('a:L -- R:b in g\n'),
  a('a:L -- R:b[x]\n'),
  a('a\n'),
  a('a b\n'),
  a('a:L\n'),
  a('a:L --\n'),
  a('a:L -- R\n'),
  a('1:L -- R:2\n'),
  a('a-b:L -- R:c-d\n'),
  a('a-:L -- R:b\n'),
  a('a:L\n-- R:b\n'),
  a('a:L --\nR:b\n'),
  a('L:L -- R:R\n'),
  a('in:L -- R:b\n'),
  a('a:L -- R:in\n'),
  a('group:L -- R:b\n'),
  a('a:L -- R:row\n'),
  a('a:L ---\nx\n--- R:b\n'),
  a('a:L --\n---\nfoo\n---\nR:b\n'),
  // Alignments.
  a('align row a b\n'),
  a('align row a b'),
  a('align column a b\n'),
  a('align row a b c d e f\n'),
  a('align row a\n'),
  a('align row\n'),
  a('align\n'),
  a('align a b\n'),
  a('align rows a b\n'),
  a('align Row a b\n'),
  a('align row column a b\n'),
  a('align row a b row\n'),
  a('align row a\nb\n'),
  a('align row a, b\n'),
  a('align row a b in g\n'),
  a('align row a a\n'),
  a('align row L R\n'),
  a('align row 1 2\n'),
  a('align column a b align row c d\n'),
  a('alignrow a b\n'),
  a('align rowa b c\n'),
  // Everything together, and the order of the statements.
  a(
    'title All\n  group g(cloud)[G]\n  group h[H] in g\n  service s1(server)[S1] in h\n  service s2 "txt" [S2]\n' +
      '  junction j in g\n  s1:R -- L:s2\n  s1{group}:B -[lbl]-> T:j\n  align row s1 s2\n  align column s1 j\n'
  ),
  a('align row a b\na:L -- R:b\njunction j\nservice a\ngroup g\n'),
  a('service a\n\n\n\nservice b\n\n'),
  a('  service a\n\tservice b\n'),
  a('service a\n  %% comment\nservice b\n'),
  a('service a %% comment\nservice b\n'),
  a('service a\nfoo bar\n'),
  a('service a\n;\n'),
  a('service a;\n'),
  a('service a,\n'),
  a('service a\n{group}\n'),
  a('service a\n<\n'),
  a('service a\n:\n'),
  a('service a\n-\n'),
  a('service a\n--\n'),
  a('service a\n"str"\n'),
  a('service a\n(icon)\n'),
  a('service a\n[title]\n'),
  a('service a\nL\n')
);

// Mutated inputs: random edits of the fixtures and of some hand-written
// inputs, with pieces of the token vocabulary.
const pieces = [
  'architecture-beta', 'group', 'service', 'junction', 'align', 'row', 'column', 'in', ' in ', ':', '--', '-', '---',
  'L', 'R', 'T', 'B', '{group}', '<', '>', '(server)', '(cloud)', '(a:b)', '()', '(', ')', '[Title]', '["q"]', "['q']",
  '[]', '[ ]', '[', ']', '"', "'", '"txt"', "'txt'", '\\', '\n', '\n\n', '\r\n', ' ', '  ', '\t', 'x', 'id1', '_', '9',
  'title', 'title X', 'accTitle: Y', 'accDescr: Z', 'accDescr {', '}', '{', '%%', '%% c', ';', ',', '.', 'é', '😀',
  ':L', 'R:', ':T -- B:', ' -[x]- ', ' --> ', ' <-- ',
];
const seeds = [
  ...fixtures,
  a('group g(cloud)[G]\n  service s(server)[S] in g\n  service t "txt" [T]\n  junction j in g\n  s:R -[l]-> L:t\n  t{group}:B <-- T:j{group}\n  align row s t\n'),
  a('service a\nservice b\na:L -- R:b\n'),
  a('title T\naccTitle: A\naccDescr {\n d\n}\nservice a["q \\" r"]\n'),
];
const rnd = prng(0xa4c817ec);
const intn = (n) => Math.floor(rnd() * n);
function mutate(text) {
  let chars = Array.from(text);
  for (let edits = 1 + intn(3); edits > 0; edits--) {
    const at = intn(chars.length + 1);
    const k = rnd();
    if (k < 0.4) {
      chars.splice(at, 0, ...Array.from(pieces[intn(pieces.length)]));
    } else if (k < 0.7) {
      chars.splice(at, 1 + intn(4));
    } else if (k < 0.9) {
      chars.splice(at, 1, ...Array.from(pieces[intn(pieces.length)]));
    } else {
      // Swap two lines.
      const lines = chars.join('').split('\n');
      const i = intn(lines.length);
      const j = intn(lines.length);
      [lines[i], lines[j]] = [lines[j], lines[i]];
      chars = Array.from(lines.join('\n'));
    }
  }
  return chars.join('');
}
for (let i = 0; i < MUTANTS; i++) {
  texts.push(mutate(seeds[intn(seeds.length)]));
}

// Generated inputs: statements from the grammar with random parts. Most of
// them parse, so they compare the values of the AST.
const pick = (list) => list[intn(list.length)];
// clean is true for a text that takes only the parts that the grammar
// accepts. The lists have those parts first.
let clean = true;
const of = (list, valid) => (clean ? list[intn(valid)] : list[intn(list.length)]);
const idList = ['a', 'b', 'c', 'db', 'web-1', 'x_y', '1st', '_u', 'gr', 'inner', 'row1', 'Lx', 'title', 'in', 'é'];
const iconList = ['(server)', '(cloud)', '(logos:aws)', '(a-b)', '(:x)', '( x )', '()', '(a b)'];
const titleList = ['[T]', '[ spaced  out ]', '["q"]', "['q']", '["a \\" b"]', "['a \\' b']", '[""]', '[ ]', '["x]y"]', '[#35;]', '[😀 é]', '["]', '[a "b"]', '[a[b]', '[]'];
const stringList = ['"txt"', "'t'", '""', '"a\\nb"', '"a\\"b"', '"\\\\"', '"x'];
const dirList = ['L', 'R', 'T', 'B', 'X', 'l'];
const ws = () => pick(['', ' ', ' ', '  ', '\t']);
const eol = () => (clean ? pick(['\n', '\n', '\n', '\n\n', '\r\n', ' %% c\n', '  \n']) : pick(['\n', '\n', '\n\n', '\r\n', ' %% c\n', '  \n', ' ', '']));
function statement() {
  const id = () => of(idList, 11);
  const icon = () => of(iconList, 5);
  const title = () => of(titleList, 13);
  const dir = () => of(dirList, 4);
  const k = rnd();
  if (k < 0.2) {
    return `group ${id()}${rnd() < 0.5 ? ws() + icon() : ''}${rnd() < 0.6 ? ws() + title() : ''}${rnd() < 0.4 ? ' in ' + id() : ''}`;
  }
  if (k < 0.45) {
    const ic = rnd() < 0.3 ? ws() + of(stringList, 6) : rnd() < 0.6 ? ws() + icon() : '';
    return `service ${id()}${ic}${rnd() < 0.6 ? ws() + title() : ''}${rnd() < 0.4 ? ' in ' + id() : ''}`;
  }
  if (k < 0.55) {
    return `junction ${id()}${rnd() < 0.4 ? ' in ' + id() : ''}`;
  }
  if (k < 0.85) {
    const grp = () => (rnd() < 0.25 ? ws() + '{group}' : '');
    const into = (c) => (rnd() < 0.35 ? c : '');
    const line = rnd() < 0.6 || (clean && rnd() < 0.2) ? '--' : clean || rnd() < 0.9 ? `-${title()}-` : pick(['-', '---', '- -']);
    return `${id()}${grp()}${ws()}:${ws()}${dir()}${ws()}${into('<')}${line}${into('>')}${ws()}${dir()}${ws()}:${ws()}${id()}${grp()}`;
  }
  if (k < 0.93) {
    const n = clean ? 2 + intn(4) : intn(5);
    return `align ${clean ? pick(['row', 'column']) : pick(['row', 'column', 'rows', ''])} ${Array.from({ length: n }, id).join(' ')}`;
  }
  return pick(['title A  title', 'accTitle: An  acc title', 'accDescr: A description', 'accDescr {\n  two\n  lines\n}', 'title']);
}
for (let i = 0; i < 1200; i++) {
  clean = rnd() < 0.7;
  let text = pick(['architecture-beta\n', 'architecture-beta\n', 'architecture-beta ', '\narchitecture-beta\n\n', '  architecture-beta\n']);
  for (let n = 1 + intn(7); n > 0; n--) {
    text += ws() + statement() + eol();
  }
  texts.push(text);
}

const seen = new Set();
const cases = [];
for (const text of texts) {
  if (seen.has(text)) continue;
  seen.add(text);
  cases.push(await run(text));
}
const errors = cases.filter((c) => c.error).length;
fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, zlib.gzipSync(JSON.stringify(cases), { level: 9 }));
console.error(`${path.relative(ROOT, OUT)}: ${cases.length} cases (${cases.length - errors} parse, ${errors} fail), ${fs.statSync(OUT).size} bytes`);
