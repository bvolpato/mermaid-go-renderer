#!/usr/bin/env node
// Writes internal/diagrams/railroad/testdata/render.json.gz: the SVG that
// mermaid.render of mermaid 12.2.0 returns in Chromium for hand-written
// railroad texts, or the error. The texts reach the branches of
// railroadRenderer.ts and the config options that the corpus fixtures do
// not have. The Go test renders the same texts and compares the SVG text.
//
// The script runs the render command of the browser oracle in a temporary
// directory: one new page for each text, as in the reference renders. The
// cases with a site config (the argument of mermaid.initialize) take one
// more run of the oracle for each site config.
//
// Usage (in tools/reference): node gen-railroad-render.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/diagrams/railroad/testdata/render.json.gz');

const cases = [];
const add = (text, site) => cases.push({ name: `render-${cases.length}`, text, ...(site ? { site } : {}) });
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';

// One text for each syntax, with every kind of node.
const ir =
  'railroad-beta\n' +
  'expr = sequence(nonterminal("term"), zeroOrMore(sequence(choice(terminal("+"), terminal("-")), nonterminal("term")))) ;\n' +
  'term = choice(nonterminal("number"), sequence(terminal("("), nonterminal("expr"), terminal(")")), special("any text")) ;\n' +
  'number = sequence(optional(terminal("-")), oneOrMore(nonterminal("digit")), optional(sequence(terminal("."), oneOrMore(nonterminal("digit"))))) ;\n' +
  'digit = choice(terminal("0"), terminal("1"), terminal("2")) ;\n';
const ebnf =
  'railroad-ebnf-beta\n' +
  'expression = term , { ( "+" | "-" ) , term } ;\n' +
  'term = factor , [ "*" , factor ] | ? special sequence ? ;\n' +
  'factor = number+ | letter* | "(" , expression , ")" | name? | letter - "x" ;\n' +
  'number = digit , { digit } ;\n';
const abnf =
  'railroad-abnf-beta\n' +
  'scheme = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ;\n' +
  'ALPHA = %x41-5A / %x61-7A ;\n' +
  'counts = 3DIGIT 1*2ALPHA *1"x" 0*1"y" 2*ALPHA [ scheme ] 4( ALPHA / DIGIT ) ;\n' +
  'crlf = %d13.10 ;\n';
const peg =
  'railroad-peg-beta\n' +
  'Expression <- Term (("+" / "-") Term)* ;\n' +
  'Term <- &Factor Factor / !"x" . / &( Term Term ) "y"? ;\n' +
  'Factor <- Number / "(" Expression ")" / .+ ;\n' +
  'Number <- Digit+ ;\n';

// The texts with no config.
for (const text of [
  ir,
  ebnf,
  abnf,
  peg,
  // No rules: the fixed size.
  'railroad-beta\n',
  'railroad-ebnf-beta\ntitle Only a title\n',
  'railroad-abnf-beta\n',
  'railroad-peg-beta\naccTitle: Empty\naccDescr: No rules\n',
  // Every kind of node alone.
  'railroad-beta\na = terminal("x") ;\n',
  'railroad-beta\na = nonterminal("x") ;\n',
  'railroad-beta\na = special("x") ;\n',
  'railroad-beta\na = optional(terminal("x")) ;\n',
  'railroad-beta\na = oneOrMore(terminal("x")) ;\n',
  'railroad-beta\na = zeroOrMore(terminal("x")) ;\n',
  'railroad-beta\na = sequence(terminal("x"), terminal("y")) ;\n',
  'railroad-beta\na = choice(terminal("x"), terminal("y")) ;\n',
  'railroad-beta\na = sequence(terminal("x")) ;\nb = choice(nonterminal("y")) ;\n',
  // Choices: an odd and an even number of alternatives, unequal widths
  // and heights, and choices in choices.
  'railroad-beta\na = choice(terminal("1"), terminal("2"), terminal("3")) ;\n',
  'railroad-beta\na = choice(terminal("1"), terminal("2"), terminal("3"), terminal("4")) ;\n',
  'railroad-beta\na = choice(terminal("short"), nonterminal("a much longer alternative"), special("mid")) ;\n',
  'railroad-beta\na = choice(optional(terminal("1")), terminal("2"), zeroOrMore(terminal("3"))) ;\n',
  'railroad-beta\na = choice(choice(terminal("1"), terminal("2")), choice(terminal("3"), terminal("4"), terminal("5"))) ;\n',
  'railroad-beta\na = choice(sequence(terminal("1"), terminal("2")), terminal("3"), sequence(terminal("4"), terminal("5"), terminal("6"))) ;\n',
  'railroad-beta\na = choice(terminal("1"), choice(terminal("2"), terminal("3"))) ;\n',
  // Sequences of parts with unequal heights.
  'railroad-beta\na = sequence(terminal("1"), optional(terminal("2")), zeroOrMore(terminal("3")), oneOrMore(terminal("4")), choice(terminal("5"), terminal("6"), terminal("7"))) ;\n',
  'railroad-beta\na = sequence(sequence(terminal("1"), terminal("2")), sequence(terminal("3"), sequence(terminal("4"), terminal("5")))) ;\n',
  // Nested options and repetitions.
  'railroad-beta\na = optional(optional(optional(terminal("x")))) ;\n',
  'railroad-beta\na = zeroOrMore(zeroOrMore(terminal("x"))) ;\nb = oneOrMore(oneOrMore(terminal("x"))) ;\nc = oneOrMore(zeroOrMore(optional(terminal("x")))) ;\n',
  'railroad-beta\na = optional(choice(terminal("1"), terminal("2"))) ;\nb = zeroOrMore(sequence(terminal("1"), terminal("2"))) ;\nc = oneOrMore(choice(terminal("1"), terminal("2"), terminal("3"))) ;\n',
  'railroad-beta\na = sequence(optional(sequence(nonterminal("member"), zeroOrMore(sequence(terminal(","), nonterminal("member"))))), choice(optional(choice(terminal("1"), zeroOrMore(terminal("2")))), oneOrMore(sequence(terminal("3"), optional(special("4")))))) ;\n',
  // Texts.
  'railroad-beta\na = terminal("") ;\nb = nonterminal("") ;\nc = special("") ;\n',
  'railroad-beta\na = terminal(" ") ;\nb = terminal("  padded  ") ;\nc = terminal("a   b") ;\n',
  'railroad-beta\na = terminal("Ünïcödé") ;\nb = nonterminal("日本語") ;\nc = special("emoji 😀") ;\n',
  'railroad-beta\na = terminal("line\\nbreak") ;\nb = terminal("tab\\tstop") ;\n',
  'railroad-beta\na = terminal("<b>bold</b>") ;\nb = nonterminal("a < b & c > d") ;\nc = special("<script>alert(1)</script>x") ;\n',
  'railroad-beta\na = terminal("#35; #lt;x#gt; #9829;") ;\n',
  'railroad-beta\na = sequence(terminal("same"), terminal("same"), nonterminal("same"), special("same")) ;\nsame = terminal("same") ;\n',
  'railroad-beta\na-very-long-rule-name-that-goes-on-and-on = terminal("x") ;\nb = terminal("y") ;\n_c1 = nonterminal("a-very-long-rule-name-that-goes-on-and-on") ;\n',
  'railroad-beta\na = terminal("a very long terminal text that makes the diagram wider than the viewport of the page, which is 800 pixels wide, and more") ;\n',
  'railroad-beta\na = terminal("x") ;\na = terminal("y") ;\na = terminal("z") ;\n',
  'railroad-beta\nWWWW = terminal("WWWW") ;\niiii = nonterminal("iiii") ;\nAV = special("AVAV To fi fl") ;\n',
  // The title and the accessibility texts.
  'railroad-beta\ntitle The title\naccTitle: The acc title\naccDescr: The acc description\na = terminal("x") ;\n',
  'railroad-ebnf-beta\naccDescr {\n  a description\n  on two lines\n}\na = "x" ;\n',
  '---\ntitle: Front matter title\n---\nrailroad-beta\na = terminal("x") ;\n',
  'railroad-beta\naccTitle: <b>bold</b> title\naccDescr: a < b\na = terminal("x") ;\n',
  // EBNF.
  'railroad-ebnf-beta\na = b ;\n',
  'railroad-ebnf-beta\na ::= b c , d | e ;\n',
  'railroad-ebnf-beta\na = ( b | c ) , [ d ] , { e } ;\n',
  'railroad-ebnf-beta\na = b? , c* , d+ , e?*+ ;\n',
  'railroad-ebnf-beta\na = b - c ;\nd = e - "x" - ( f | g ) ;\nh = i* - j ;\n',
  'railroad-ebnf-beta\na = ? special ? | ?   padded   ? | "x" ? y ? ;\n',
  'railroad-ebnf-beta\n(* an ISO comment *)\na = b (* inside *) , c ; /* block */\n',
  'railroad-ebnf-beta\na = { { b } } | [ [ c ] ] | ( ( d ) ) ;\n',
  // ABNF.
  'railroad-abnf-beta\na = b ;\n',
  'railroad-abnf-beta\na = b c / d / "e" %x41 ;\n',
  'railroad-abnf-beta\na = *b ;\nc = 1*d ;\ne = *1f ;\ng = 0*1h ;\ni = 3j ;\nk = 0l ;\nm = 2*3n ;\no = 0*5p ;\n',
  'railroad-abnf-beta\na = [ b ] ;\nc = *[ d ] ;\ne = 2( f g ) ;\nh = *( i / j ) ;\n',
  'railroad-abnf-beta\n; a comment line\na = b ;\n; another comment\nc = %d13.10 / %b1010 / %x41-5A.61 ;\n',
  // PEG.
  'railroad-peg-beta\nA <- B ;\n',
  'railroad-peg-beta\nA <- B C / D / "e" . ;\n',
  'railroad-peg-beta\nA <- B? C* D+ ;\n',
  'railroad-peg-beta\nA <- &B !C &"x" !. &( B C ) !B* &B? ;\n',
  'railroad-peg-beta\n# a comment\nA <- ( B / C )* ( D )+ ( "x" )? ; # trailing\n',
  'railroad-peg-beta\nA <- !"\\"" . ;\n',
]) {
  add(text);
}

// The config options of the renderer, and config that changes the font.
for (const config of [
  'railroad:\n  padding: 0',
  'railroad:\n  padding: 20',
  'railroad:\n  padding: 3.5',
  'railroad:\n  padding: "15"',
  'railroad:\n  padding: -5',
  'railroad:\n  padding: abc',
  'railroad:\n  verticalSeparation: 0',
  'railroad:\n  verticalSeparation: 30',
  'railroad:\n  horizontalSeparation: 0',
  'railroad:\n  horizontalSeparation: 25.5',
  'railroad:\n  arcRadius: 0',
  'railroad:\n  arcRadius: 4',
  'railroad:\n  arcRadius: 20',
  'railroad:\n  arcRadius: 7.3',
  'railroad:\n  fontSize: 10',
  'railroad:\n  fontSize: 24',
  'railroad:\n  fontSize: "18px"',
  'railroad:\n  fontSize: 0',
  'railroad:\n  fontSize: -3',
  'railroad:\n  fontFamily: monospace',
  'railroad:\n  fontFamily: courier',
  'railroad:\n  fontFamily: "\'Courier New\', Courier, monospace"',
  'railroad:\n  fontFamily: "bad;family"',
  'railroad:\n  fontFamily: 5',
  'railroad:\n  markerRadius: 0',
  'railroad:\n  markerRadius: 10',
  'railroad:\n  showMarkers: false\n  compactMode: true',
  'railroad:\n  useMaxWidth: false',
  'railroad:\n  useMaxWidth: true',
  'railroad:\n  useMaxWidth: 0',
  'railroad:\n  padding: 6\n  verticalSeparation: 12\n  horizontalSeparation: 14\n  arcRadius: 8\n  fontSize: 13\n  markerRadius: 3\n  useMaxWidth: false',
  'railroad:\n  terminalFill: "#ff0000"\n  lineColor: blue\n  strokeWidth: 4',
  'theme: dark',
  'theme: forest',
  'theme: neutral',
  'theme: base',
  'theme: redux',
  'theme: redux-dark',
  'theme: neo',
  'look: handDrawn',
  'themeVariables:\n  fontSize: 20px',
  'themeVariables:\n  fontSize: 12',
  'themeVariables:\n  fontSize: big',
  'themeVariables:\n  fontFamily: courier',
  'themeVariables:\n  fontFamily: "bad;family"',
  'themeVariables:\n  fontSize: 22px\nrailroad:\n  fontSize: 11',
  'fontFamily: courier',
  'fontSize: 20',
  'securityLevel: loose',
  'htmlLabels: false',
]) {
  add(cfg(config) + ir);
}
add(cfg('railroad:\n  useMaxWidth: false') + 'railroad-beta\n');
add(cfg('railroad:\n  padding: 5\n  arcRadius: 5') + ebnf);
add(cfg('railroad:\n  fontSize: 12\n  horizontalSeparation: 4') + abnf);
add(cfg('theme: dark\nrailroad:\n  verticalSeparation: 2\n  markerRadius: 8') + peg);
add('%%{init: {"railroad": {"padding": 12, "useMaxWidth": false}, "theme": "forest"}}%%\n' + peg);

// Site configs.
for (const site of [
  { theme: 'forest' },
  { railroad: { padding: 5, arcRadius: 6, useMaxWidth: false } },
  { themeVariables: { fontSize: '12px', fontFamily: 'courier' } },
  { securityLevel: 'loose', fontFamily: 'verdana' },
]) {
  add(ir, site);
  add(cfg('railroad:\n  padding: 8\n  fontSize: 15') + ebnf, site);
}

// ---------------------------------------------------------------------------
// Run the oracle: one run for the cases with the same site config.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-railroad-render-'));
const groups = new Map();
cases.forEach((c, i) => {
  const key = JSON.stringify(c.site ?? null);
  if (!groups.has(key)) {
    groups.set(key, []);
  }
  groups.get(key).push(i);
});
let run = 0;
for (const [key, indexes] of groups) {
  const dir = path.join(tmp, `run-${run++}`);
  fs.mkdirSync(path.join(dir, 'out'), { recursive: true });
  const files = indexes.map((i) => {
    const file = path.join(dir, `case-${i}.mmd`);
    fs.writeFileSync(file, cases[i].text);
    return file;
  });
  const listFile = path.join(dir, 'files.list');
  fs.writeFileSync(listFile, files.join('\n') + '\n');
  const args = [path.join(here, 'oracle.mjs'), 'render', '--out', path.join(dir, 'out'), '--list', listFile, '--quiet'];
  if (key !== 'null') {
    const configFile = path.join(dir, 'site.json');
    fs.writeFileSync(configFile, key);
    args.push('--config', configFile);
  }
  execFileSync('node', args, { stdio: 'inherit' });
  for (const i of indexes) {
    const meta = JSON.parse(fs.readFileSync(path.join(dir, 'out', `case-${i}.json`), 'utf8'));
    if (meta.error !== undefined) {
      cases[i].error = meta.error;
    } else {
      cases[i].type = meta.diagramType;
      cases[i].svg = fs.readFileSync(path.join(dir, 'out', `case-${i}.svg`), 'utf8');
    }
  }
}
fs.rmSync(tmp, { recursive: true, force: true });

fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases }), { level: 9 }));
const errors = cases.filter((c) => c.error !== undefined).length;
console.error(`${path.relative(repoRoot, outFile)}: ${cases.length} cases, ${errors} errors, ${fs.statSync(outFile).size} bytes`);
