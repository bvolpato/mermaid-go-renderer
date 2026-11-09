#!/usr/bin/env node
// Writes the parser goldens of the treeView, cynefin and wardley diagrams
// from the real @mermaid-js/parser (the Langium grammars in
// packages/parser/src/language/treeView, cynefin and wardley):
//
//   internal/diagrams/treeview/testdata/ast.json.gz
//   internal/diagrams/cynefin/testdata/ast.json.gz
//   internal/diagrams/wardley/testdata/ast.json.gz
//
// Each case is a text with the AST of parse(grammar, text), or the error of
// the parser. The texts are the corpus fixtures (as mermaid gives them to the
// parser, and as they are in the file), hand-written inputs for the branches
// of the grammar and of the value converters, and mutated inputs.
//
// Run it with tools/reference as the working directory:
//
//   node gen-treeview-cynefin-wardley-ast.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { parse } from '@mermaid-js/parser';
import { internals, mulberry32 } from './lib-mermaid-internals.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(HERE, '../..');
const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e');

// plain turns an AST into JSON: it drops the links to the CST and to the
// container, and keeps the numbers that JSON has no text for.
function plain(node) {
  if (Array.isArray(node)) {
    return node.map(plain);
  }
  if (typeof node === 'number' && (!Number.isFinite(node) || Object.is(node, -0))) {
    return { $n: Object.is(node, -0) ? '-0' : String(node) };
  }
  if (node === null || typeof node !== 'object') {
    return node;
  }
  const out = {};
  for (const key of Object.keys(node)) {
    if (key.startsWith('$') && key !== '$type') {
      continue;
    }
    if (node[key] !== undefined) {
      out[key] = plain(node[key]);
    }
  }
  return out;
}

async function run(grammar, text) {
  try {
    return { text, ast: plain(await parse(grammar, text)) };
  } catch (e) {
    return { text, error: String(e?.message ?? e) || 'error' };
  }
}

function* walk(dir) {
  for (const e of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, e);
    if (fs.statSync(p).isDirectory()) yield* walk(p);
    else if (p.endsWith('.mmd')) yield p;
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

const COMMON_HAND = (kw) => [
  kw,
  `${kw}\n`,
  `\n\n${kw}\n`,
  `  ${kw}\n`,
  `${kw}x\n`,
  `${kw} %% c\n`,
  `${kw}%%c\n`,
  `%% c\n${kw}\n`,
  `${kw}\ntitle T\n`,
  `${kw}\ntitle\n`,
  `${kw}\ntitle  a   b \t c  \n`,
  `${kw} title T`,
  `${kw}\ntitle a\ntitle b\n`,
  `${kw}\naccTitle: A\naccDescr: D\n`,
  `${kw}\naccTitle:   a   b  %% c\n`,
  `${kw}\naccDescr {\n  multi\n   line\n}\n`,
  `${kw}\naccDescr {a} title x\n`,
  `${kw}\naccDescr {\n unclosed\n`,
  `${kw}\naccDescr{x}accDescr:y\n`,
  `${kw}\naccTitle x\n`,
  `${kw}\ntitle T\n\n\naccTitle: A`,
  `${kw}\r\ntitle x\r\n`,
  `${kw}\u00a0\n`,
  `---\ntitle: x\n---\n${kw}\n`,
  `%%{init: {"theme":"dark"}}%%\n${kw}\n`,
  '',
  '\n',
  kw.toUpperCase(),
  `${kw} ${kw}`,
  `${kw}\n${kw}\n`,
];

const HAND = {
  treeView: [
    ...COMMON_HAND('treeView-beta'),
    'treeView-beta\n  src/\n      App.tsx :::highlight icon(fa:bell) ## main component\n      index.js ## entry point\n',
    'treeView-beta\na\n',
    'treeView-beta\n a\n  b\n   c\n',
    'treeView-beta\n\ta\n\t\tb\n \t c\n',
    'treeView-beta\n"quoted name"\n  \'single quoted\'\n',
    'treeView-beta\n"a" "b"\n',
    'treeView-beta\n"a"   \n"b"\n',
    'treeView-beta\n"a"\n   \n"b"\n',
    'treeView-beta\na   \nb\n',
    'treeView-beta\na\n   \nb\n',
    'treeView-beta\n  a\n  ',
    'treeView-beta\n  a\n\n\n  b\n\n',
    'treeView-beta\n""\n',
    "treeView-beta\n''\n",
    'treeView-beta\n"unterminated\n',
    'treeView-beta\n"multi\nline"\n',
    'treeView-beta\n"a"b\n',
    'treeView-beta\na"b\n',
    "treeView-beta\na'b c\n",
    'treeView-beta\n"a" :::cls\n',
    'treeView-beta\n"a":::cls\n',
    'treeView-beta\na:::cls\n',
    'treeView-beta\na ::: cls\n',
    'treeView-beta\na :::\n',
    'treeView-beta\na :::1x\n',
    'treeView-beta\na :::_x-1 :::y\n',
    'treeView-beta\na :::x:::y\n',
    'treeView-beta\na :::x\t:::y  icon(a)\ticon(b:c) ## d ## e\n',
    'treeView-beta\na icon()\n',
    'treeView-beta\na icon(none)\n',
    'treeView-beta\na icon(fa:bell)\n',
    'treeView-beta\na icon(fa:)\n',
    'treeView-beta\na icon(:bell)\n',
    'treeView-beta\na icon(a:b:c)\n',
    'treeView-beta\na icon(a b)\n',
    'treeView-beta\na icon(é)\n',
    'treeView-beta\na icon(x\n',
    'treeView-beta\na icon (x)\n',
    'treeView-beta\naicon(x)\n',
    'treeView-beta\nicon(x)\n',
    'treeView-beta\n  icon(x)\n',
    'treeView-beta\n:::x\n',
    'treeView-beta\n##x\n',
    'treeView-beta\n ## only a description\n',
    'treeView-beta\na ## d\n',
    'treeView-beta\na ##\n',
    'treeView-beta\na ##   \n',
    'treeView-beta\na ##  d  e  \n',
    'treeView-beta\na ##\u00a0d\u00a0\n',
    'treeView-beta\na##d\n',
    'treeView-beta\na #d\n',
    'treeView-beta\na ## d :::x icon(y)\n',
    'treeView-beta\na :::x ## d icon(y)\n',
    'treeView-beta\na  b   c/\n',
    'treeView-beta\ndir/\n  sub/\n    f.txt\n',
    'treeView-beta\n/\n//\n',
    'treeView-beta\n"dir/"\n"a/b"\n',
    'treeView-beta\na\\b\n',
    'treeView-beta\n🚀 rocket-app/\n    📦 packages/\n',
    'treeView-beta\né\u00a0x\n',
    'treeView-beta\n\u00a0a\n',
    'treeView-beta\na\u2028b\n',
    'treeView-beta\ntitle.txt\n',
    'treeView-beta\n  title.txt\n',
    'treeView-beta\ntitle\n',
    'treeView-beta\n  a\ntitle T\n',
    'treeView-beta\ntitle T\n  a\n',
    'treeView-beta\ntitle T\naccTitle: A\naccDescr: D\n  a\n',
    'treeView-beta\ntitle T\ntitle U\n  a\n',
    'treeView-beta\naccDescr {\n a\n b\n}\n  a\n',
    'treeView-beta\naccDescr {a} b\n',
    'treeView-beta title T\n a\n',
    'treeView-beta a\n',
    'treeView-beta   \n a\n',
    'treeView-beta\t\n a\n',
    'treeView-beta%% c\n a\n',
    'treeView-beta %% c\n a\n',
    'treeView-beta\n a %% c\n',
    'treeView-beta\n%% c\n a\n',
    'treeView-beta\n  %% c\n a\n',
    'treeView-beta\n a\n%%c',
    'treeView-beta\r\n  a\r\n    b\r\n',
    'treeView-beta\r  a\r',
    'treeView-beta\n  accTitle.md\n',
    'treeView-beta\n  accDescr: x\n',
    'treeView-beta\n  a\n  accTitle: x\n',
    'treeView\n a\n',
    'treeView-beta-x\n a\n',
    'treeView-betaa\n',
    'TreeView-beta\n a\n',
    'treeView-beta\n a\n treeView-beta\n',
    'treeView-beta\n a\ntreeView-beta\n',
    'treeView-beta\n a b :::c d\n',
    'treeView-beta\n a icon(x) b\n',
    'treeView-beta\n a\t:::c\n',
    'treeView-beta\n a \t :::c\n',
    'treeView-beta\n a :::c \n',
    'treeView-beta\n a icon(x) \n',
    'treeView-beta\n a :::c\n\n b\n',
    'treeView-beta\n ﬂ°quot¶ß name\n',
    'treeView-beta\n' + ' '.repeat(300) + 'deep\n',
  ],
  cynefin: [
    ...COMMON_HAND('cynefin-beta'),
    'cynefin-beta:',
    'cynefin-beta:\n',
    'cynefin-beta :\n',
    'cynefin-beta::\n',
    'cynefin-beta: complex\n',
    'cynefin-beta:complex\n',
    'cynefin-beta complex\n',
    'cynefin-beta:\ntitle T\n',
    'cynefin-beta: title T\n',
    'cynefin-beta\ncomplex\n',
    'cynefin-beta\ncomplex\ncomplicated\nclear\nchaotic\nconfusion\n',
    'cynefin-beta\ncomplex\n"a"\n"b"\n\n"c"\n',
    'cynefin-beta\ncomplex "a" "b"\n',
    'cynefin-beta\ncomplex "a" complicated "b"\n',
    'cynefin-beta\ncomplex\n"a"\ncomplex\n"b"\n',
    'cynefin-beta\n"orphan"\n',
    'cynefin-beta\ncomplex\n\'single\'\n"with \\"escape\\" \\n x"\n',
    'cynefin-beta\ncomplex\n""\n',
    'cynefin-beta\ncomplex\n"unterminated\n',
    'cynefin-beta\ncomplex\nbare\n',
    'cynefin-beta\ncomplex --> clear\n',
    'cynefin-beta\ncomplex --> clear',
    'cynefin-beta\ncomplex-->clear\n',
    'cynefin-beta\ncomplex --> clear : "label"\n',
    'cynefin-beta\ncomplex --> clear:"label"\n',
    "cynefin-beta\ncomplex --> clear : 'label'\n",
    'cynefin-beta\ncomplex --> clear : ""\n',
    'cynefin-beta\ncomplex --> clear :\n',
    'cynefin-beta\ncomplex --> clear "label"\n',
    'cynefin-beta\ncomplex --> clear : "a" "b"\n',
    'cynefin-beta\ncomplex --> clear : "a" complex\n',
    'cynefin-beta\ncomplex --> complex\n',
    'cynefin-beta\ncomplex --> complex : "self"\n',
    'cynefin-beta\ncomplex -->\n',
    'cynefin-beta\ncomplex --> other\n',
    'cynefin-beta\ncomplex -> clear\n',
    'cynefin-beta\ncomplex --- clear\n',
    'cynefin-beta\n--> clear\n',
    'cynefin-beta\ncomplex clear\n',
    'cynefin-beta\ncomplex\nclear --> chaotic\n',
    'cynefin-beta\ncomplex\n"a"\nclear --> chaotic\n"b"\n',
    'cynefin-beta\ncomplex --> clear\n"b"\n',
    'cynefin-beta\ncomplex --> clear\n\n\ncomplex\n"b"\n',
    'cynefin-beta\ncomplex --> clear complex --> chaotic\n',
    'cynefin-beta\ncomplex --> clear\nclear --> chaotic\nchaotic --> confusion : "x"\n',
    'cynefin-beta\ncomplex\n"a"\ntitle Late title\n',
    'cynefin-beta\ncomplex\ntitle Late title\n"a"\n',
    'cynefin-beta\ncomplex title x\n',
    'cynefin-beta\ncomplex "a" title x\n',
    'cynefin-beta\ncomplex --> clear title x\n',
    'cynefin-beta\ntitle A\ncomplex\ntitle B\n',
    'cynefin-beta\naccDescr {\n a\n}\ncomplex\n',
    'cynefin-beta\naccDescr {a} complex\n',
    'cynefin-beta\ncomplexity\n',
    'cynefin-beta\nComplex\n',
    'cynefin-beta\ncomplexcomplicated\n',
    'cynefin-beta\ncomplex"a"clear"b"\n',
    'cynefin-beta\ncomplex %% c\n"a" %% d\n',
    'cynefin-beta\r\ncomplex\r\n"a"\r\ncomplex --> clear : "x"\r\n',
    'cynefin-beta\ncomplex\n:\n',
    'cynefin-beta\ncomplex : "a"\n',
    'cynefin-beta\ncomplex\n"é 😀"\n',
    'cynefin-beta\ncomplex\n"a\nb"\n',
    'cynefin-beta\n\n\ncomplex\n\n\n"a"\n\n\n"b"\n\n\nclear\n\n',
    'cynefin-beta\nconfusion\n"1"\n"2"\n"3"\n"4"\n"5"\n',
    'cynefin-beta\ncomplex --> clear : "x"\n\n',
    'cynefin-beta\ncomplex\n"ﬂ°quot¶ß"\n',
    'cynefin-beta\ncomplex\n"a" -->\n',
  ],
  wardley: [
    ...COMMON_HAND('wardley-beta'),
    'wardley-beta\ntitle Tea Shop\nanchor Business [0.95, 0.63]\ncomponent Cup of Tea [0.79, 0.61] label [19, -4]\nBusiness -> Cup of Tea\n',
    'wardley-beta\nsize [1100, 800]\n',
    'wardley-beta\nsize [1100,800]\nsize [5, 6]\n',
    'wardley-beta\nsize [1100]\n',
    'wardley-beta\nsize [1.5, 2]\n',
    'wardley-beta\nsize [-1, 2]\n',
    'wardley-beta\nsize [01, 2]\n',
    'wardley-beta\nsize [0, 0]\n',
    'wardley-beta\nsize 1, 2\n',
    'wardley-beta\nsize [1, 2] size [3, 4]\n',
    'wardley-beta\nsize [1, 2]',
    'wardley-beta\nsize [123456789012345678901234567890, 2]\n',
    'wardley-beta\nevolution Genesis -> Custom -> Product -> Commodity\n',
    'wardley-beta\nevolution Genesis / Concept -> Custom / Emerging -> Product / Converging -> Commodity / Accepted\n',
    'wardley-beta\nevolution Genesis@0.25 -> Custom@0.5 -> Product@0.7 -> Commodity@1.0\n',
    'wardley-beta\nevolution A@0.2 / B -> C / D@0.4 -> E\n',
    'wardley-beta\nevolution A\n',
    'wardley-beta\nevolution A -> B\n',
    'wardley-beta\nevolution A ->\n',
    'wardley-beta\nevolution -> A\n',
    'wardley-beta\nevolution "Quoted A" -> \'Quoted B\' -> c_1 -> D d\n',
    'wardley-beta\nevolution A@1 -> B\n',
    'wardley-beta\nevolution A@ -> B\n',
    'wardley-beta\nevolution A / -> B\n',
    'wardley-beta\nevolution A / B / C -> D\n',
    'wardley-beta\nevolution A -> B\nevolution C -> D -> E\n',
    'wardley-beta\nevolution A --> B\n',
    'wardley-beta\nevolution A->B->C\n',
    'wardley-beta\nevolution A-B -> C\n',
    'wardley-beta\nevolution 1 -> 2 -> 3\n',
    'wardley-beta\nanchor Business [0.95, 0.63]\n',
    'wardley-beta\nanchor "Quoted" [0.95, 0.63]\nanchor id_1 [0.1, 0.2]\nanchor Two Words [0.3, 0.4]\n',
    'wardley-beta\nanchor Business [95, 63]\n',
    'wardley-beta\nanchor Business [0.95]\n',
    'wardley-beta\nanchor [0.95, 0.63]\n',
    'wardley-beta\nanchor Business [0.95, 0.63] label [1, 2]\n',
    'wardley-beta\nanchor Business [1.5, 0.63]\nanchor B [95.5, 100.0]\nanchor C [100.5, 0.0]\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5]',
    'wardley-beta\ncomponent A [0.5,0.5]\n',
    'wardley-beta\ncomponent A[0.5, 0.5]\n',
    'wardley-beta\ncomponent "A b" [0.5, 0.5]\ncomponent \'c\' [0.1, 0.1]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [10, 5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [-10, -5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [- 10, -5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [--10, 5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [1.5, 5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [10]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [0, 0]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (build)\ncomponent B [0.5, 0.5] (buy)\ncomponent C [0.5, 0.5] (outsource)\ncomponent D [0.5, 0.5] (market)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (other)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] build\n',
    'wardley-beta\ncomponent A [0.5, 0.5] inertia\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (inertia)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (build) inertia\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (build) (inertia)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] inertia (build)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (inertia) (build)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] label [1, 2] (buy) inertia\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (buy) label [1, 2]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] inertia inertia\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (build) (build)\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (\n',
    'wardley-beta\ncomponent A [0.5, 0.5] ()\n',
    'wardley-beta\ncomponent A [0.5, 0.5] (build\n',
    'wardley-beta\ncomponent A [0.5, 0.5] extra\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\ncomponent A [0.2, 0.3] label [1, 1]\n',
    'wardley-beta\ncomponent A [5, 5]\n',
    'wardley-beta\ncomponent A [0.5, 5]\n',
    'wardley-beta\ncomponent A [.5, 0.5]\n',
    'wardley-beta\ncomponent A [0.5., 0.5]\n',
    'wardley-beta\ncomponent A [0.5.5, 0.5]\n',
    'wardley-beta\ncomponent A [1e1, 0.5]\n',
    'wardley-beta\ncomponent A [00.50, 000.250]\n',
    'wardley-beta\ncomponent A [0.' + '9'.repeat(400) + ', ' + '9'.repeat(400) + '.0]\n',
    'wardley-beta\ncomponent [0.5, 0.5]\n',
    'wardley-beta\ncomponent component [0.5, 0.5]\n',
    'wardley-beta\ncomponent size [0.5, 0.5]\n',
    'wardley-beta\ncomponent sizeable thing [0.5, 0.5]\n',
    'wardley-beta\ncomponent builder [0.5, 0.5]\n',
    'wardley-beta\ncomponent My builder [0.5, 0.5]\n',
    'wardley-beta\ncomponent My note [0.5, 0.5]\n',
    'wardley-beta\ncomponent Label Maker [0.5, 0.5]\n',
    'wardley-beta\ncomponent the label [0.5, 0.5]\n',
    'wardley-beta\ncomponent real-time processing [0.5, 0.5]\n',
    'wardley-beta\ncomponent byte pair encoding (BPE) [0.5, 0.5]\n',
    'wardley-beta\ncomponent R&D [0.5, 0.5]\n',
    'wardley-beta\ncomponent A1 b2 [0.5, 0.5]\n',
    'wardley-beta\ncomponent A 1b [0.5, 0.5]\n',
    'wardley-beta\ncomponent 1A [0.5, 0.5]\n',
    'wardley-beta\ncomponent a-b_c [0.5, 0.5]\n',
    'wardley-beta\ncomponent _x [0.5, 0.5]\n',
    'wardley-beta\ncomponent x- [0.5, 0.5]\n',
    'wardley-beta\ncomponent é [0.5, 0.5]\n',
    'wardley-beta\ncomponent "é 😀 \\"q\\"" [0.5, 0.5]\n',
    'wardley-beta\ncomponent title [0.5, 0.5]\n',
    'wardley-beta\ncomponent titled [0.5, 0.5]\n',
    'wardley-beta\ncomponent accTitle [0.5, 0.5]\n',
    'wardley-beta\ncomponent the title [0.5, 0.5]\n',
    'wardley-beta\nA -> B\n',
    'wardley-beta\nA->B\n',
    'wardley-beta\nA --> B\n',
    'wardley-beta\nA -.-> B\n',
    'wardley-beta\nA > B\n',
    'wardley-beta\nA B\n',
    'wardley-beta\nA\n',
    'wardley-beta\nA ->\n',
    'wardley-beta\n-> B\n',
    'wardley-beta\nA -> B -> C\n',
    'wardley-beta\nA +> B\n',
    'wardley-beta\nA +< B\n',
    'wardley-beta\nA +<> B\n',
    'wardley-beta\nA -> B +>\n',
    'wardley-beta\nA +> -> B +<\n',
    'wardley-beta\nA +> B; +>\n',
    'wardley-beta\nA -> B; constraint\n',
    'wardley-beta\nA -> B;\n',
    'wardley-beta\nA -> B;   spaced  label  \n',
    'wardley-beta\nA -> B ; x ; y\n',
    'wardley-beta\nA; x\n',
    "wardley-beta\nA +'backup'> B\n",
    "wardley-beta\nA +'backup'< B\n",
    "wardley-beta\nA +'backup'<> B\n",
    "wardley-beta\nA +''> B\n",
    "wardley-beta\nA +'x' B\n",
    "wardley-beta\nA +'x'> B; y\n",
    "wardley-beta\nA +> +'x'> B\n",
    'wardley-beta\nA + B\n',
    'wardley-beta\nA - B\n',
    'wardley-beta\nA -- B\n',
    'wardley-beta\nA .-. B\n',
    'wardley-beta\n"Quoted A" -> \'Quoted B\'\n',
    'wardley-beta\nTwo Words -> Three Word Name\n',
    'wardley-beta\nid_1 -> id-2\n',
    'wardley-beta\n1 -> 2\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\ncomponent B [0.1, 0.1]\nA -> B\nB -> A\nA -> A\nA -> Missing\n',
    'wardley-beta\nevolve A 0.8\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\nevolve A 0.8\nevolve A 0.9\n',
    'wardley-beta\nevolve A 8\n',
    'wardley-beta\nevolve A\n',
    'wardley-beta\nevolve 0.8\n',
    'wardley-beta\nevolve "A b" 0.8\nevolve Two Words 0.5\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\nevolve A 1.5\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\nevolve A 100.5\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35] label [-60, 35]\ncomponent B [0.53]\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\n\n\ncomponent A [0.35]\n\n\ncomponent B [0.53]\n\n}\n\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K { component A [0.35]\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35] }\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35]\n}',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35]\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35, 0.2]\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35] (build)\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35] inertia\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\nA -> B\n}\n',
    'wardley-beta\npipeline Missing {\ncomponent A [0.35]\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K\n{\ncomponent A [0.35]\n}\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\npipeline K {\ncomponent A [0.35]\n}\npipeline K {\ncomponent B [0.6]\n}\nA -> K\nB -> K\n',
    'wardley-beta\ncomponent K [0.57, 0.45]\ncomponent A [0.2, 0.2]\npipeline K {\ncomponent A [0.35]\n}\nA -> K\n',
    'wardley-beta\ncomponent Two Words [0.57, 0.45]\npipeline Two Words {\ncomponent Sub One [0.35]\ncomponent "Sub 2" [1.5]\n}\nSub One -> Two Words\n',
    'wardley-beta\nnote "A note" [0.5, 0.5]\n',
    'wardley-beta\nnote A note [0.5, 0.5]\n',
    'wardley-beta\nnote "A note"\n',
    'wardley-beta\nnote "A note" [50, 50]\n',
    "wardley-beta\nnote 'single' [0.5, 0.5]\nnote \"\" [0.1, 0.1]\n",
    'wardley-beta\nannotations [0.72, 0.03]\n',
    'wardley-beta\nannotations [72, 3]\n',
    'wardley-beta\nannotations [0.72, 3]\nannotations [1, 0.5]\n',
    'wardley-beta\nannotations [0.72]\n',
    'wardley-beta\nannotations [0, 0]\n',
    'wardley-beta\nannotation 1,[0.48, 0.48] "Standardising power"\n',
    'wardley-beta\nannotation 1, [0.48, 0.48] "Standardising power"\n',
    'wardley-beta\nannotation 1 [0.48, 0.48] "x"\n',
    'wardley-beta\nannotation 1,[0.48, 0.48]\n',
    'wardley-beta\nannotation 1,[0.48, 0.48] x\n',
    'wardley-beta\nannotation 1,[48, 0.48] "x"\nannotation 2,[0.3, 5] "y"\nannotation 10,[1, 1] ""\n',
    'wardley-beta\nannotation 1.5,[0.48, 0.48] "x"\n',
    'wardley-beta\nannotation -1,[0.48, 0.48] "x"\n',
    'wardley-beta\nannotation 01,[0.48, 0.48] "x"\n',
    'wardley-beta\nannotation 0,[0.48, 0.48] "x"\n',
    'wardley-beta\nannotation a,[0.48, 0.48] "x"\n',
    'wardley-beta\nannotation 2,[0.1, 0.1] "b"\nannotation 1,[0.2, 0.2] "a"\nannotations [0.5, 0.5]\n',
    'wardley-beta\naccelerator Fast [0.5, 0.5]\n',
    'wardley-beta\naccelerator "Fast one" [0.5, 0.5]\naccelerator Two Words [0.1, 0.9]\n',
    'wardley-beta\naccelerator Fast [5, 5]\n',
    'wardley-beta\naccelerator [0.5, 0.5]\n',
    'wardley-beta\ndeaccelerator Slow [0.5, 0.5]\n',
    'wardley-beta\ndeaccelerator "Slow one" [0.5, 0.5]\ndeaccelerator x [0.1, 0.9]\n',
    'wardley-beta\ndeaccelerator Slow [0.5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] %% c\nA -> B %% d\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\n%% c\nA -> B\n',
    'wardley-beta\r\ncomponent A [0.5, 0.5]\r\nA -> B\r\n',
    'wardley-beta\n\n\ncomponent A [0.5, 0.5]\n\n\nA -> B\n\n\n',
    'wardley-beta component A [0.5, 0.5]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] component B [0.1, 0.1]\n',
    'wardley-beta\ncomponent A [0.5, 0.5] A -> B\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\ntitle Late\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\naccTitle: Late\naccDescr: Late d\n',
    'wardley-beta\ntitle A\ncomponent A [0.5, 0.5]\ntitle B\n',
    'WARDLEY-BETA\ncomponent A [0.5, 0.5]\n',
    'Wardley-Beta\n',
    'wardley-beta-x\n',
    'wardley\n',
    'wardley-beta\nfoo bar baz\n',
    'wardley-beta\n[0.5, 0.5]\n',
    'wardley-beta\n{\n}\n',
    'wardley-beta\n@\n',
    'wardley-beta\n/\n',
    'wardley-beta\n(build)\n',
    'wardley-beta\nbuild -> buy\n',
    'wardley-beta\nlabel -> inertia\n',
    'wardley-beta\ncomponent A [0.5, 0.5]\nnote -> A\n',
    'wardley-beta\nanchor A [0.9, 0.9]\ncomponent A [0.5, 0.5] label [1, 2] (buy) inertia\nanchor A [0.1, 0.1]\n',
    'wardley-beta\ncomponent ﬂ°quot¶ß [0.5, 0.5]\n',
    'wardley-beta\ncomponent "ﬂ°quot¶ß" [0.5, 0.5]\n',
  ],
};

// The texts that mutate inserts, by grammar.
const COMMON_PIECES = [
  '"', '"', "'", '\n', '\n', ' ', ' ', '  ', '\t', '\r\n', '%%', '%% c\n', ':', ',', '.', '-', '0', '1', '7', '42',
  '0.5', 'x', 'ab', '_', 'title ', 'title T\n', 'accTitle: A\n', 'accDescr: D\n', 'accDescr {\n d\n}\n', '}', '{',
  '\\', '\u00a0', 'é', '😀', '"s"', '#', ';',
];
const PIECES = {
  treeView: [
    ...COMMON_PIECES,
    'treeView-beta', ' :::cls', ':::', ' icon(x)', ' icon()', ' icon(a:b)', 'icon(', ')', ' ## desc', '##', '/', '    ',
    '      ', 'name.txt', 'dir/', '"q name"',
  ],
  cynefin: [
    ...COMMON_PIECES,
    'cynefin-beta', 'cynefin-beta:', 'complex', 'complicated', 'clear', 'chaotic', 'confusion', '-->', ' --> ', ' : ',
    '"item"', 'complex --> clear\n', 'clear\n',
  ],
  wardley: [
    ...COMMON_PIECES,
    'wardley-beta', '[', ']', '[', ']', '(', ')', '@', '/', '->', ' -> ', '-->', '-.->', '>', '+>', '+<', '+<>',
    "+'x'>", "+'x'<>", '; lbl', 'build', 'buy', 'outsource', 'market', 'size ', 'evolution ', 'anchor ', 'component ',
    'label ', 'label [1, 2]', 'inertia', '(inertia)', '(build)', 'evolve ', 'pipeline ', 'note ', 'annotations ',
    'annotation ', 'accelerator ', 'deaccelerator ', '[0.5, 0.5]', '[0.3]', 'Name', 'Two Words', '0.25', '10', '100',
  ],
};

function mutate(text, rnd, pieces, maxEdits) {
  const pick = (a) => a[Math.floor(rnd() * a.length)];
  const units = Array.from(text);
  const edits = 1 + Math.floor(rnd() * maxEdits);
  for (let e = 0; e < edits; e++) {
    const at = Math.floor(rnd() * (units.length + 1));
    const k = rnd();
    if (k < 0.35) units.splice(at, 0, pick(pieces));
    else if (k < 0.6) units.splice(at, 1 + Math.floor(rnd() * 3));
    else if (k < 0.8) units.splice(at, 1, pick(pieces));
    else if (k < 0.9) {
      // Move a line.
      const lines = units.join('').split('\n');
      const from = Math.floor(rnd() * lines.length);
      const [line] = lines.splice(from, 1);
      lines.splice(Math.floor(rnd() * (lines.length + 1)), 0, line);
      units.length = 0;
      units.push(...Array.from(lines.join('\n')));
    } else {
      // Duplicate a line.
      const lines = units.join('').split('\n');
      const from = Math.floor(rnd() * lines.length);
      lines.splice(Math.floor(rnd() * (lines.length + 1)), 0, lines[from]);
      units.length = 0;
      units.push(...Array.from(lines.join('\n')));
    }
  }
  return units.join('');
}

const GRAMMARS = [
  { name: 'treeView', pkg: 'treeview', family: 'tree-view', mutants: 3000, seed: 0x74726565 },
  { name: 'cynefin', pkg: 'cynefin', family: 'cynefin', mutants: 2600, seed: 0x63796e65 },
  { name: 'wardley', pkg: 'wardley', family: 'wardley', mutants: 5000, seed: 0x77617264 },
];

const { preprocessDiagram } = await internals();

for (const g of GRAMMARS) {
  const fixtures = [];
  for (const file of walk(path.join(CORPUS, g.family))) {
    const source = fs.readFileSync(file, 'utf8');
    // The text that mermaid gives to the parser.
    fixtures.push(encodeEntities(preprocessDiagram(source).code.cleaned) + '\n');
    // The text of the file without the front matter: the parser itself
    // also takes comments.
    fixtures.push(source.replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, ''));
  }
  const texts = [...fixtures, ...HAND[g.name]];
  const seeds = [...new Set(texts)];
  // The seeds that the parser accepts. One edit of such a text is often
  // valid too, so these mutants compare many ASTs.
  const validSeeds = [];
  for (const text of seeds) {
    if ((await run(g.name, text)).error === undefined) validSeeds.push(text);
  }
  const seen = new Set(seeds);
  const rnd = mulberry32(g.seed);
  const all = [...seeds];
  for (let i = 0; i < g.mutants; i++) {
    // A mutant of a fixture or of a hand-written input: one edit of a valid
    // text, or one to three edits of any text.
    const text =
      i % 2 === 0
        ? mutate(validSeeds[Math.floor(rnd() * validSeeds.length)], rnd, PIECES[g.name], 1)
        : mutate(seeds[Math.floor(rnd() * seeds.length)], rnd, PIECES[g.name], 3);
    if (seen.has(text)) continue;
    seen.add(text);
    all.push(text);
  }
  const cases = [];
  for (const text of all) cases.push(await run(g.name, text));
  const failed = cases.filter((c) => c.error !== undefined).length;
  const out = path.join(ROOT, 'internal/diagrams', g.pkg, 'testdata/ast.json.gz');
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, zlib.gzipSync(Buffer.from(JSON.stringify({ fixtures: fixtures.length, cases })), { level: 9 }));
  console.log(
    `${path.relative(ROOT, out)}: ${cases.length} cases, ${failed} that the parser rejects, ` +
      `${fixtures.length} from fixtures, ${(fs.statSync(out).size / 1024).toFixed(1)} kB`
  );
}
