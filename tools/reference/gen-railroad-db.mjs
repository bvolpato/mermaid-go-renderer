#!/usr/bin/env node
// Writes internal/diagrams/railroad/testdata/db.json.gz: what the db of the
// railroad diagrams of real mermaid holds after a real render (the title,
// the accessibility texts, the rules in their order and the rule map), for
//   - every fixture of the corpus family railroad,
//   - texts of the parser golden (testdata/ast.json.gz, from
//     gen-railroad-ast.mjs): the hand-written inputs of the four grammars
//     and a sample of the random sentences and of the mutated inputs,
//   - hand-written texts for the branches of railroadDb.ts and of the four
//     parsers that need a config, a front matter or text that the sanitizer
//     changes.
// A case that mermaid rejects has the error message instead.
//
// The db sanitizes its texts with DOMPurify, which needs a browser, so the
// script runs railroad-db-page.js with the esm-eval command of the browser
// oracle, one time. A corpus fixture runs in a new page, as in the
// reference renders, and so does a case with a site config.
//
// Usage (in tools/reference): node gen-railroad-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpusDir = 'testdata/corpus/mermaid-e2e';
const family = 'railroad/';
const astFile = path.join(repoRoot, 'internal/diagrams/railroad/testdata/ast.json.gz');
const outFile = path.join(repoRoot, 'internal/diagrams/railroad/testdata/db.json.gz');
// The number of leading cases of each grammar in the parser golden that are
// all in the db golden: the fixtures and the hand-written inputs. One of
// every `step` of the other cases is in the db golden too.
const leading = 330;
const step = 10;

const cases = [];

// ---------------------------------------------------------------------------
// The fixtures of the corpus.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => f.startsWith(family))
  .sort();
for (const f of fixtures) {
  cases.push({ name: `${corpusDir}/${f}`, corpus: true, fresh: true, text: fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8') });
}

// ---------------------------------------------------------------------------
// Texts of the parser golden.

const ast = JSON.parse(zlib.gunzipSync(fs.readFileSync(astFile)));
const seen = new Set();
for (const grammar of ['railroad', 'railroadEbnf', 'railroadAbnf', 'railroadPeg']) {
  ast[grammar].cases.forEach((c, i) => {
    if ((i < leading || i % step === 0) && !seen.has(c.text)) {
      seen.add(c.text);
      cases.push({ name: `ast-${grammar}-${i}`, text: c.text });
    }
  });
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const ir = 'railroad-beta\ntitle <b>Bold</b> title <script>alert(1)</script>\naccTitle: <i>Acc</i> title\naccDescr: <b>Acc</b> description <img src=x onerror=alert(1)>\na = sequence(terminal("<b>bold</b> <script>alert(1)</script>"), nonterminal("<img src=x onerror=alert(1)>"), special("a<br/>b <i>c</i>")) ;\nb = choice(terminal("x < y"), terminal("a = b"), terminal("a & b"), nonterminal("<unknown>tag</unknown>")) ;\nc = optional(zeroOrMore(oneOrMore(terminal("#35; #lt;x#gt; #quot;")))) ;\n';
const ebnfText = 'railroad-ebnf-beta\ntitle "Quoted <b>title</b>"\naccDescr {\n  multi <b>line</b>\n     description\n}\nexpr = term , { ( "+" | "-" ) , term } | ? <b>special</b> text ? ;\nterm = "<i>x</i>" - "y" , [ factor ]? , digit+ , letter* ;\nexpr = "duplicate" ;\n';
const abnfText = 'railroad-abnf-beta\ntitle \'single <b>quoted</b>\'\naccTitle: ABNF\nrule = *a 1*b *1c 0*1d 1*2e 3*5f 0*g 0*0h 3i 0j 1k 007l 12345678901234567890123m 2*12345678901234567890123n ;\nother = "<b>x</b>" %x41-5A [ rule ] 2( rule / "y" ) *[ rule ] ;\n';
const pegText = 'railroad-peg-beta\ntitle PEG <i>title</i>\naccTitle: PEG acc\naccDescr: PEG descr\nA <- &B !"<b>x</b>" &. !( B C ) &B* !"y"+ &( B )? ;\nB <- . .* "x"? ( B / C )+ / !. ;\nA <- B ;\n';
const extras = [
  // The title of the front matter does not stay: the parser clears the db.
  '---\ntitle: Front matter title\n---\nrailroad-beta\na = terminal("x") ;\n',
  '---\ntitle: Front matter title\n---\nrailroad-beta\ntitle Text title\na = terminal("x") ;\n',
  '---\ntitle: Front matter title\n---\nrailroad-ebnf-beta\na = "x" ;\n',
  '---\ntitle: Front matter title\n---\nrailroad-abnf-beta\na = "x" ;\n',
  '---\ntitle: Front matter title\n---\nrailroad-peg-beta\nA <- "x" ;\n',
  '---\ntitle: Front matter title\n---\nrailroad-beta\n',
  // Texts that the sanitizer changes, with the security levels.
  ir,
  cfg('securityLevel: loose') + ir,
  cfg('securityLevel: antiscript') + ir,
  cfg('securityLevel: strict') + ir,
  cfg('htmlLabels: false') + ir,
  cfg('flowchart:\n  htmlLabels: false') + ir,
  cfg('securityLevel: loose\nhtmlLabels: false') + ir,
  '%%{init: {"dompurifyConfig": {"ALLOWED_TAGS": ["b"]}}}%%\n' + ir,
  '%%{init: {"dompurifyConfig": {"FORBID_TAGS": ["b", "i"], "KEEP_CONTENT": false}}}%%\n' + ir,
  '%%{init: {"dompurifyConfig": {"RETURN_DOM": true}}}%%\n' + ir,
  '%%{init: {"dompurifyConfig": {"RETURN_DOM": true}}}%%\n---\ntitle: T\n---\nrailroad-beta\n',
  '%%{init: {"dompurifyConfig": {"RETURN_DOM": true}}}%%\nrailroad-beta\n',
  ebnfText,
  cfg('securityLevel: loose') + ebnfText,
  cfg('htmlLabels: false') + ebnfText,
  abnfText,
  cfg('securityLevel: loose') + abnfText,
  pegText,
  cfg('securityLevel: loose') + pegText,
  cfg('htmlLabels: false') + pegText,
  // Config and directives that do not change the db.
  cfg('theme: dark\nrailroad:\n  padding: 20\n  fontSize: 20') + 'railroad-beta\na = terminal("x") ;\n',
  '%%{init: {"theme": "forest"}}%%\nrailroad-ebnf-beta\na = "x" | b ;\n',
  '%%{init: {"railroad": {"useMaxWidth": false}}}%%\nrailroad-peg-beta\nA <- "x" / B ;\n',
  // The text that mermaid gives to the parser: without comments, with
  // entities encoded.
  'railroad-beta\n%% comment\na = terminal("x") ; %% trailing\n  %% indented\nb = terminal("%% in a string") ;\n',
  'railroad-beta %% comment\na = terminal("x") ;\n',
  '%% first\n\n   railroad-beta\na = terminal("x") ;\n',
  'railroad-beta\r\na = terminal("x") ;\r\nb = terminal("y") ;\r\n',
  'railroad-beta\na = terminal("#9829; #heart; #35;") ;\nb = nonterminal("#lt;b#gt;") ;\n',
  'railroad-ebnf-beta\na = "#35;" | ? #quot; ? ;\n',
  'railroad-abnf-beta\na = "#35;" ;\n',
  'railroad-abnf-beta\na = b ; a comment\n ;\nc = d ;\n',
  'railroad-abnf-beta\na = b ;\n; #35; in a comment\nc = d ;\n',
  'railroad-peg-beta\nA <- "x" ; # comment\nB <- "#35;" ; #x; y\n',
  'railroad-peg-beta\nA <- "x" ; #x;\n',
  'RAILROAD-BETA\na = terminal("x") ;\n',
  'Railroad-Ebnf-Beta\na = "x" ;\n',
  'railroad-beta\nclassDef x fill:#f00;\n',
  'railroad-beta\na = terminal("style x fill:#f00;") ;\n',
  // No rules.
  'railroad-beta',
  'railroad-beta\n',
  'railroad-ebnf-beta\ntitle Only a title\n',
  'railroad-abnf-beta\naccTitle: Only acc\naccDescr: Only descr\n',
  'railroad-peg-beta\n\n\n',
  // Names that the grammars share with JavaScript objects.
  'railroad-beta\nconstructor = terminal("x") ;\n__proto__ = terminal("y") ;\ntoString = nonterminal("constructor") ;\nconstructor = terminal("z") ;\n',
  'railroad-ebnf-beta\nconstructor = __proto__ | toString ;\n__proto__ = "y" ;\n',
  // White space in the accessibility texts.
  'railroad-beta\naccTitle:    padded   title   \naccDescr:    padded   description   \na = terminal("x") ;\n',
  'railroad-beta\naccDescr {\n\n   first\n\n\n   second   line\n\t third \n}\na = terminal("x") ;\n',
  'railroad-beta\ntitle     \na = terminal("x") ;\n',
  'railroad-beta\ntitle ""\naccTitle:\naccDescr:\na = terminal("x") ;\n',
  'railroad-beta\ntitle "  inner   spaces  "\na = terminal("x") ;\n',
  'railroad-beta\ntitle "esc \\"aped\\" \\n new \\t tab"\na = terminal("x") ;\n',
  "railroad-abnf-beta\ntitle 'esc \\'aped\\' \\n'\na = b ;\n",
  // Unicode.
  'railroad-beta\ntitle Ünïcödé 日本語 😀\na = sequence(terminal("Ünïcödé"), nonterminal("日本語"), special("😀")) ;\n',
  'railroad-peg-beta\nA <- "Ünïcödé 日本語 😀" ;\n',
];
extras.forEach((text, i) => cases.push({ name: `extra-${i}`, text }));

// Site configs: the argument of mermaid.initialize. Such a case runs in a
// new page.
const sites = [
  { securityLevel: 'loose' },
  { securityLevel: 'antiscript' },
  { htmlLabels: false },
  { theme: 'dark', railroad: { padding: 4 } },
  { dompurifyConfig: { ALLOWED_TAGS: ['i'] } },
];
sites.forEach((site, i) => {
  cases.push({ name: `site-${i}-ir`, fresh: true, site, text: ir });
  cases.push({ name: `site-${i}-peg`, fresh: true, site, text: pegText });
  cases.push({ name: `site-${i}-directive`, fresh: true, site, text: cfg('securityLevel: strict\nhtmlLabels: true') + ebnfText });
});

// ---------------------------------------------------------------------------
// Run the page script.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-railroad-db-'));
const argsFile = path.join(tmp, 'args.json');
const rawFile = path.join(tmp, 'raw.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh, site })) }));
const started = Date.now();
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'railroad-db-page.js'), '--args', argsFile, '--out', rawFile], {
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });
if (raw.cases.length !== cases.length) {
  throw new Error(`${raw.cases.length} results for ${cases.length} cases`);
}

const stats = { total: cases.length, errors: 0, types: {}, seconds: Math.round((Date.now() - started) / 100) / 10 };
const out = raw.cases.map((res, i) => {
  const c = cases[i];
  if (res.name !== c.name) {
    throw new Error(`case ${i}: ${res.name} is not ${c.name}`);
  }
  const rec = { ...res };
  if (!c.corpus) {
    rec.text = c.text;
  }
  if (c.site) {
    rec.site = c.site;
  }
  if (res.error !== undefined) {
    stats.errors++;
  } else {
    stats.types[res.type] = (stats.types[res.type] || 0) + 1;
  }
  return rec;
});

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases: out }), { level: 9 }));
console.error(JSON.stringify(stats));
console.error(`${path.relative(repoRoot, outFile)}: ${out.length} cases, ${fs.statSync(outFile).size} bytes`);
