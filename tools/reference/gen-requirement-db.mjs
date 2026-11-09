// Writes internal/diagrams/requirement/testdata/db.json.gz: the content of
// the requirement db of real mermaid (the requirements, the elements, the
// relationships, the classes, the direction, the title, the accessibility
// values) and the layout data that the renderer hands to the layout (the
// nodes and edges of getData, the config, the direction, the spacing, the
// markers, the layout algorithm), for
//   - every fixture of the corpus that mermaid renders as a requirement
//     diagram, and every other fixture of the requirement family,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of requirementDb.ts and of the
//     grammar.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs
// requirement-db-page.js with the esm-eval command of the browser oracle,
// one time. A corpus fixture runs in a new page, as in the reference
// renders.
//
// Usage (in tools/reference): node gen-requirement-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpusDir = 'testdata/corpus/mermaid-e2e';
const refDir = '.parity/ref/mermaid-e2e';
const family = 'requirement/';
const outFile = path.join(repoRoot, 'internal/diagrams/requirement/testdata/db.json.gz');
const mutantsPerFixture = 2;

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

// ---------------------------------------------------------------------------
// The fixtures of the corpus: every fixture of the requirement family, and
// the fixtures of other families that the reference rendered as a
// requirement diagram.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => {
    if (f.startsWith(family)) {
      return true;
    }
    try {
      const meta = JSON.parse(fs.readFileSync(path.join(repoRoot, refDir, f.replace(/\.mmd$/, '.json')), 'utf8'));
      return meta.diagramType === 'requirement';
    } catch {
      return false;
    }
  })
  .sort();

const cases = [];
for (const f of fixtures) {
  cases.push({ name: `${corpusDir}/${f}`, corpus: true, fresh: true, text: fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8') });
}

// ---------------------------------------------------------------------------
// Seeded mutations.

const snippets = [
  'requirement r1 {\n  id: 1\n  text: the text.\n  risk: high\n  verifymethod: test\n}',
  'functionalRequirement fr {\n  id: 1.1\n}',
  'interfaceRequirement ir {\n}',
  'performanceRequirement pr {\n  risk: low\n}',
  'physicalRequirement phr {\n  verifymethod: inspection\n}',
  'designConstraint dc {\n  text: "quoted, text: with - chars"\n}',
  'requirement "quoted name" {\n  id: "quoted id"\n}',
  'requirement test_req {\n  id: other\n}',
  'element e1 {\n  type: simulation\n}',
  'element e2 {\n  type: "word doc"\n  docRef: reqs/test_entity\n}',
  'element e3 {\n}',
  'element test_entity {\n  docref: again\n}',
  'test_entity - contains -> test_req',
  'test_entity - copies -> test_req',
  'test_req - derives -> test_entity',
  'test_req - satisfies -> r1',
  'r1 - verifies -> e1',
  'e1 - refines -> e2',
  'e2 - traces -> nowhere',
  'test_req <- copies - test_entity',
  'r1 <- contains - r1',
  '"quoted name" - traces -> "quoted name"',
  'direction LR',
  'direction TB',
  'direction BT',
  'direction RL',
  'classDef important fill:#f96,stroke:#333,stroke-width:4px',
  'classDef c1,c2 color:#fff,fill:#00f',
  'classDef default fill:#eee,color:#111',
  'class test_req important',
  'class test_req,test_entity c1,c2',
  'style test_req fill:#ffa,stroke:#000,color:green',
  'style test_req,test_entity stroke-width:2px',
  'test_req:::important',
  'test_entity:::c1,c2',
  'requirement styled:::important {\n  id: 2\n}',
  'element styledEl:::c1,c2 {\n  type: x\n}',
  'accTitle: The title',
  'accDescr: The description',
  'accDescr {\n  multi\n  line\n}',
  '%% comment',
  '# comment',
  'requirement #35;hash#lt; {\n  text: #9829;\n}',
  '}',
  '{',
  ':',
  'title something',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its text can have a line that starts with the keyword.
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*requirementDiagram/.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 7);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind === 3 || kind === 4) {
      lines.splice(p, 0, '  ' + snippets[Math.floor(rnd() * snippets.length)]);
    } else if (kind === 5) {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    } else {
      lines.length = Math.max(start, p);
    }
  }
  return lines.join('\n');
}

const rnd = prng(0x72657144);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const rd = (body, front = '') => front + 'requirementDiagram\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const req = (name, body = 'id: 1') => `requirement ${name} {\n${body}\n}`;
const el = (name, body = 'type: sim') => `element ${name} {\n${body}\n}`;
const reqTypes = ['requirement', 'functionalRequirement', 'interfaceRequirement', 'performanceRequirement', 'physicalRequirement', 'designConstraint'];
const relTypes = ['contains', 'copies', 'derives', 'satisfies', 'verifies', 'refines', 'traces'];
const two = req('a') + '\n' + el('b');
const extras = [
  // Headers.
  'requirementDiagram',
  'requirementDiagram\n',
  'requirementDiagram\n\n\n',
  '\n\n   requirementDiagram   \n\n\n' + req('a') + '\n\n',
  'requirementDiagram ' + req('a') + '\n',
  'requirementDiagram;\n' + req('a') + '\n',
  'REQUIREMENTDIAGRAM\n' + req('a') + '\n',
  'requirementdiagram\n' + req('a') + '\n',
  'requirementDiagramX\n' + req('a') + '\n',
  'requirement\n' + req('a') + '\n',
  req('a') + '\n',
  'requirement a {\nid: 1\n}\nrequirementDiagram\n',
  'requirementDiagram\nrequirementDiagram\n' + req('a') + '\n',
  // Requirement types.
  rd(reqTypes.map((t, i) => `${t} r${i} {\nid: ${i}\ntext: the text ${i}.\nrisk: high\nverifymethod: test\n}`).join('\n')),
  rd(reqTypes.map((t, i) => `${t.toUpperCase()} R${i} {\n}`).join('\n')),
  rd(reqTypes.map((t, i) => `${t.toLowerCase()} l${i} {\nID: ${i}\n}`).join('\n')),
  rd('Requirement a {\nId: 1\nText: t\nRisk: High\nVerifyMethod: Test\n}'),
  rd('requirementx a {\nid: 1\n}'),
  rd('functionalrequirement a {\n}\nFunctionalRequirement b {\n}'),
  // Risk levels and verification methods.
  rd(['low', 'medium', 'high', 'LOW', 'Medium', 'HIGH'].map((r, i) => req('r' + i, 'risk: ' + r)).join('\n')),
  rd(['analysis', 'demonstration', 'inspection', 'test', 'ANALYSIS', 'Demonstration'].map((v, i) => req('v' + i, 'verifymethod: ' + v)).join('\n')),
  rd(req('a', 'verifyMethod: test') + '\n' + req('b', 'VERIFYMETHOD: analysis') + '\n' + req('c', 'verifymethod:inspection')),
  rd(req('a', 'risk: none')),
  rd(req('a', 'risk: "high"')),
  rd(req('a', 'risk:')),
  rd(req('a', 'verifymethod: other')),
  rd(req('a', 'verifymethod: "test"')),
  rd(req('a', 'risk: highest')),
  rd(req('a', 'verifymethod: testing')),
  rd(req('a', 'risk: high low')),
  // The body of a requirement.
  rd(req('a', '')),
  rd('requirement a {\n}'),
  rd('requirement a {\n\n\n}'),
  rd('requirement a {}'),
  rd('requirement a { }'),
  rd('requirement a { id: 1 }'),
  rd('requirement a {\nid: 1 }'),
  rd('requirement a {\nid: 1\n} element b {\n}'),
  rd('requirement a\n{\nid: 1\n}'),
  rd('requirement a {\nid: 1'),
  rd('requirement a {\nid: 1\n'),
  rd('requirement a\n'),
  rd('requirement {\nid: 1\n}'),
  rd(req('a', 'id: 1\nid: 2\ntext: one\ntext: two\nrisk: low\nrisk: high\nverifymethod: test\nverifymethod: analysis')),
  rd(req('a', 'verifymethod: test\nrisk: high\ntext: the text\nid: 1')),
  rd(req('a', '\n\nid: 1\n\n\ntext: t\n\n')),
  rd(req('a', '  id:   spaced id   \n  text:    spaced   text   ')),
  rd(req('a', 'id:1\ntext:t')),
  rd(req('a', 'id: 1.2.3\ntext: the text.')),
  rd(req('a', 'id: "quoted id"\ntext: "quoted text"')),
  rd(req('a', 'id: "with: colon, comma - dash = eq { brace < lt > gt"\ntext: "x"')),
  rd(req('a', 'id: ""')),
  rd(req('a', 'text: ""')),
  rd(req('a', 'id: " "')),
  rd(req('a', 'id: "unterminated')),
  rd(req('a', 'id: a, b')),
  rd(req('a', 'id: a: b')),
  rd(req('a', 'id: a-b')),
  rd(req('a', 'id: a = b')),
  rd(req('a', 'id: a <b>')),
  rd(req('a', 'id: a {b}')),
  rd(req('a', 'id:')),
  rd(req('a', 'id')),
  rd(req('a', 'text')),
  rd(req('a', 'name: x')),
  rd(req('a', 'type: x')),
  rd(req('a', 'docref: x')),
  rd(req('a', 'id: id\ntext: text')),
  rd(req('a', 'id: test\ntext: high')),
  rd(req('a', 'id: requirement\ntext: element')),
  rd(req('a', 'id: type\ntext: docref')),
  rd(req('a', 'id: 1 text: 2')),
  rd(req('a', 'text: Ünïcödé 日本語 😀')),
  rd(req('a', 'text: "*italic* **bold** `code`"')),
  rd(req('a', 'text: *italic* **bold**')),
  rd(req('a', 'text: "<b>html</b> <script>alert(1)</script>"')),
  rd(req('a', 'text: "line<br/>break"')),
  rd(req('a', 'text: #35; hash #lt;b#gt; #9829;')),
  rd(req('a', 'text: "#35; hash #lt;b#gt; #9829;"')),
  rd(req('a', 'text: a; b')),
  rd(req('a', 'text: a # b')),
  rd(req('a', 'text: a %% b')),
  rd(req('a', 'text: a %  b')),
  rd(req('a', 'text: 100%')),
  rd(req('a', 'text: don\'t')),
  rd(req('a', 'text: a\\b')),
  rd(req('a', 'text: a "quoted" word')),
  rd(req('a', 'text: "a" b')),
  rd(req('a', 'text: 5')),
  rd(req('a', 'id: 007\ntext: 1e5')),
  rd(req('a', 'id: _under\ntext: .dot')),
  rd(req('a', 'id: +plus')),
  // Names.
  rd(req('test_req') + '\n' + req('test_req2') + '\n' + req('TestReq3')),
  rd(req('a b c')),
  rd(req('  padded  ')),
  rd(req('"quoted name"')),
  rd(req('"quoted: with, chars - = { } < >"')),
  rd(req('""')),
  rd(req('" "')),
  rd(req('"a" "b"')),
  rd(req('"unterminated')),
  rd(req('a.b') + '\n' + req('a/b') + '\n' + req('a+b') + '\n' + req('a_b') + '\n' + req('a(b)')),
  rd(req('a-b')),
  rd(req('a:b')),
  rd(req('a,b')),
  rd(req('a=b')),
  rd(req('a<b>')),
  rd(req('1') + '\n' + req('2') + '\n' + req('33') + '\n' + req('4.5') + '\n' + req('007')),
  rd(req('10') + '\n' + req('b') + '\n' + req('2') + '\n' + el('1x') + '\n' + el('0')),
  rd(req('Ünïcödé') + '\n' + req('日本語') + '\n' + req('емкость')),
  rd(req('😀')),
  rd(req('a😀b')),
  rd(req('_under') + '\n' + req('__proto__') + '\n' + req('constructor') + '\n' + req('toString')),
  rd(req('.dot')),
  rd(req('-dash')),
  rd(req('#hash')),
  rd(req('test') + '\n'),
  rd(req('testing') + '\n' + req('test_x') + '\n' + req('highest') + '\n' + req('lower') + '\n' + req('typed')),
  rd(req('element')),
  rd(req('elements')),
  rd(req('requirement')),
  rd(req('id')),
  rd(req('idx') + '\n' + req('text2') + '\n' + req('risky') + '\n' + req('typed') + '\n' + req('docrefs')),
  rd(req('style')),
  rd(req('styled') + '\n' + req('classy') + '\n' + req('classDefined')),
  rd(req('class')),
  rd(req('contains')),
  rd(req('containsx') + '\n' + req('copies2') + '\n' + req('tracesz')),
  rd(req('direction')),
  rd(req('my direction LR')),
  rd(req('title')),
  rd(req('title x')),
  rd(req('accTitle')),
  rd(req('a') + '\n' + req('a', 'id: 2\ntext: second')),
  rd(req('a') + '\nfunctionalRequirement a {\nid: 3\n}'),
  rd(req('a') + '\n' + el('a')),
  rd(el('a') + '\n' + req('a')),
  rd(req('a #35; b')),
  rd(req('"a #35; b #quot;q#quot;"')),
  // Elements.
  rd(el('e')),
  rd(el('e', '')),
  rd('element e {\n}'),
  rd('element e {}'),
  rd('element e { type: x }'),
  rd(el('e', 'type: simulation\ndocref: reqs/test_entity')),
  rd(el('e', 'docRef: reqs/test_entity\ntype: word doc')),
  rd(el('e', 'DOCREF: a\nTYPE: b')),
  rd(el('e', 'docref: one\ndocref: two\ntype: t1\ntype: t2')),
  rd(el('e', 'type: "quoted type"\ndocref: "quoted, ref: x"')),
  rd(el('e', 'type: ""\ndocref: ""')),
  rd(el('e', 'type:')),
  rd(el('e', 'docref:')),
  rd(el('e', 'type: a-b')),
  rd(el('e', 'docref: http://example.com/a')),
  rd(el('e', 'docref: "http://example.com/a?b=c"')),
  rd(el('e', 'id: 1')),
  rd(el('e', 'text: x')),
  rd(el('e', 'risk: high')),
  rd(el('e', 'type: test\ndocref: high')),
  rd(el('e', 'type: type\ndocref: docref')),
  rd(el('"quoted element"', 'type: x')),
  rd(el('a b c', 'type: x')),
  rd(el('e', '\n\ntype: x\n\n')),
  rd(el('e', 'type: x') + '\n' + el('e', 'type: y\ndocref: z')),
  rd('element {\ntype: x\n}'),
  rd('element e\n'),
  rd('element e {\ntype: x'),
  rd('ELEMENT E {\nType: X\nDocRef: Y\n}'),
  rd('elementx e {\n}'),
  rd(el('e', 'type: <b>bold</b>')),
  rd(el('e', 'type: "<b>bold</b> *it* #35;"')),
  // Relationships: every type in both directions.
  rd(two + '\n' + relTypes.map((t) => `a - ${t} -> b`).join('\n')),
  rd(two + '\n' + relTypes.map((t) => `a <- ${t} - b`).join('\n')),
  rd(two + '\n' + relTypes.map((t) => `b - ${t.toUpperCase()} -> a`).join('\n')),
  rd(two + '\na-contains->b\nb<-traces-a'),
  rd(two + '\na   -   contains   ->   b'),
  rd(two + '\na - contains -> a\nb <- traces - b'),
  rd(two + '\na - contains -> b\na - contains -> b\nb - contains -> a'),
  rd(two + '\na - contains -> c'),
  rd(two + '\nc - contains -> a'),
  rd(two + '\nc - traces -> d'),
  rd(two + '\nc <- copies - d'),
  rd('a - contains -> b'),
  rd('a - contains -> b\n' + two),
  rd(req('"my req"') + '\n' + el('"my el"') + '\n"my req" - satisfies -> "my el"\n"my el" <- traces - "my req"'),
  rd(req('my req') + '\n' + el('my el') + '\nmy req - satisfies -> my el'),
  rd(two + '\n"a" - contains -> "b"'),
  rd(req('a') + '\n' + el('a') + '\na - traces -> a'),
  rd(two + '\na - contain -> b'),
  rd(two + '\na - containsx -> b'),
  rd(two + '\na - contains - b'),
  rd(two + '\na -> contains -> b'),
  rd(two + '\na <- contains -> b'),
  rd(two + '\na - contains ->'),
  rd(two + '\n- contains -> b'),
  rd(two + '\na contains b'),
  rd(two + '\na -- contains --> b'),
  rd(two + '\na - contains -> b - traces -> a'),
  rd(two + '\na - contains -> b c'),
  rd(two + '\na - id -> b'),
  rd(two + '\ntest - contains -> b'),
  rd(two + '\na - contains -> test'),
  rd(two + '\n1 - contains -> 2'),
  rd(req('1') + '\n' + el('2') + '\n1 - contains -> 2\n2 <- traces - 1'),
  rd(req('__proto__') + '\n' + el('constructor') + '\n__proto__ - contains -> constructor\nconstructor - traces -> toString'),
  rd(two + '\na - contains -> b;'),
  rd(two + '\na - contains -> b # c'),
  rd(two + '\na - contains -> b %% c'),
  // Direction.
  rd('direction LR\n' + two),
  rd('direction TB\n' + two),
  rd('direction BT\n' + two),
  rd('direction RL\n' + two),
  rd('direction lr\n' + two),
  rd('Direction Rl\n' + two),
  rd('direction LR\ndirection RL\n' + two),
  rd(two + '\n  direction   BT  \n'),
  rd('direction TD\n' + two),
  rd('direction XY\n' + two),
  rd('direction\n' + two),
  rd('direction LRX\n' + two),
  rd('direction LR extra words\n' + two),
  rd('some direction LR\n' + two),
  rd('# direction LR\n' + two),
  rd('requirement my direction RL thing {\nid: 1\n}'),
  rd(req('a', 'text: go in direction BT now')),
  rd('direction TB direction LR\n' + two),
  rd('direction\tLR\n' + two),
  'direction LR\nrequirementDiagram\n' + two + '\n',
  // Styles.
  rd(two + '\nstyle a fill:#f9f,stroke:#333,stroke-width:4px'),
  rd(two + '\nstyle a fill:#ffa,stroke:#000,color:green\nstyle b color:red'),
  rd(two + '\nstyle a,b fill:red\nstyle a stroke:blue'),
  rd('style a fill:red\n' + two),
  rd(two + '\nstyle missing fill:red'),
  rd(two + '\nstyle missing,a fill:red'),
  rd(two + '\nstyle a,missing,b fill:red'),
  rd(two + '\nstyle a'),
  rd(two + '\nstyle'),
  rd(two + '\nstyle a fill:#f00;'),
  rd(two + '\nstyle a fill:#f00;stroke:#00f'),
  rd(two + '\nstyle a fill : red , stroke : blue'),
  rd(two + '\nstyle a fill:red color:blue'),
  rd(two + '\nstyle a font-size:12px,stroke-dasharray:5 5'),
  rd(two + '\nstyle a opacity:0.5'),
  rd(two + '\nstyle a width:50%'),
  rd(two + '\nstyle a fill:rgb(1,2,3)'),
  rd(two + '\nstyle a fill:red,'),
  rd(two + '\nstyle a ,fill:red'),
  rd(two + '\nstyle a fill:red,,stroke:blue'),
  rd(two + '\nstyle a fill:red\nstyle a fill:blue\nstyle a fill:red'),
  rd(two + '\nstyle "a" fill:red'),
  rd(req('"my req"') + '\nstyle "my req" fill:red'),
  rd(req('my req') + '\nstyle my req fill:red'),
  rd(two + '\nSTYLE a fill:red'),
  rd(two + '\nstyle a fill:red\n\n' + req('c')),
  rd(two + '\nstyle a fill:red\n' + req('c') + '\nstyle c stroke:blue'),
  rd(two + '\nstyle a fill:#ABCDEF,stroke:#123'),
  rd(two + '\nstyle a 12:34'),
  rd(two + '\nstyle a color:#fff,background-color:#000'),
  rd(req('a') + '\n' + el('a') + '\nstyle a fill:red'),
  rd(two + '\nstyle a fill:red\na - contains -> b'),
  rd(two + '\nstyle a fill:red direction LR'),
  // classDef and class.
  rd(two + '\nclassDef important fill:#f96,stroke:#333,stroke-width:4px\nclass a important'),
  rd('classDef important fill:#f96,stroke:#333\n' + two + '\nclass a,b important'),
  rd(two + '\nclass a important\nclassDef important fill:#f96,stroke:#333'),
  rd(two + '\nclassDef c1 fill:red\nclassDef c2 stroke:blue\nclass a,b c1,c2'),
  rd(two + '\nclassDef c1,c2 fill:red,color:blue\nclass a c1\nclass b c2'),
  rd(two + '\nclassDef cls color:red,fill:blue,background-color:green,stroke:colorful\nclass a cls'),
  rd(two + '\nclassDef cls fill:red\nclassDef cls stroke:blue\nclassDef cls color:green\nclass a cls'),
  rd(two + '\nclass a cls\nclassDef cls fill:red\nclassDef cls stroke:blue,color:green'),
  rd(two + '\nclassDef cls font-size:20px,font-weight:bold\nclass a cls'),
  rd(two + '\nclassDef cls fill:#f00;\nclass a cls'),
  rd(two + '\nclassDef cls'),
  rd(two + '\nclassDef'),
  rd(two + '\nclassDef cls fill'),
  rd(two + '\nclassDef cls fill:\nclass a cls'),
  rd(two + '\nclassDef cls fill:red,\nclass a cls'),
  rd('classDef default fill:#eee,stroke:#111\n' + two),
  rd(two + '\nclassDef default fill:#eee,stroke:#111'),
  rd(two + '\nclassDef default fill:#eee\n' + req('c') + '\nclassDef default color:#111'),
  rd(two + '\nclassDef default fill:#eee\nstyle a fill:#f00\nclass b default'),
  rd(two + '\nclassDef Ünï fill:red\nclass a Ünï'),
  rd(two + '\nclass a cls other'),
  rd(two + '\nclass a'),
  rd(two + '\nclass'),
  rd(two + '\nclass missing cls\nclass a nodef'),
  rd(two + '\nclass a,missing,b cls\nclassDef cls fill:red'),
  rd(two + '\nclass a cls\nclass a cls\nclassDef cls fill:red'),
  rd(two + '\nCLASSDEF cls fill:red\nCLASS a cls'),
  rd(two + '\nclassDef a fill:red\nclass a a'),
  rd(two + '\nclassDef cls fill:red,stroke:blue,color:white\nclass a,b cls\nstyle a fill:yellow'),
  rd(req('"my req"') + '\nclassDef cls fill:red\nclass "my req" cls'),
  rd(req('a') + '\n' + el('a') + '\nclassDef cls fill:red\nclass a cls'),
  rd(two + '\nclassDef cls fill:red\nclass a cls\n\n' + req('c')),
  rd(two + '\nclassDef cls color:fill,fill:color\nclass a cls'),
  rd(two + '\nclassDef cls fill:red,fill:blue,colorfill:1\nclass a cls'),
  // The shorthand :::.
  rd('requirement a:::cls {\nid: 1\n}\nclassDef cls fill:red'),
  rd('classDef cls fill:red\nrequirement a:::cls {\nid: 1\n}\nelement b:::cls {\ntype: x\n}'),
  rd('classDef c1 fill:red\nclassDef c2 stroke:blue\nrequirement a:::c1,c2 {\nid: 1\n}\nelement b:::c2,c1 {\n}'),
  rd('requirement a:::c1, c2 {\nid: 1\n}'),
  rd('requirement a ::: cls {\nid: 1\n}'),
  rd('requirement a::: {\nid: 1\n}'),
  rd('requirement a:: cls {\nid: 1\n}'),
  rd('requirement "my req":::cls {\nid: 1\n}\nclassDef cls fill:red'),
  rd('requirement a:::"quoted class" {\nid: 1\n}'),
  rd(two + '\nclassDef cls fill:red\na:::cls\nb:::cls'),
  rd(two + '\na:::cls\nclassDef cls fill:red'),
  rd(two + '\na:::c1,c2\nclassDef c1 fill:red\nclassDef c2 color:blue'),
  rd(two + '\nmissing:::cls'),
  rd(two + '\na:::'),
  rd(two + '\na:::cls b:::cls'),
  rd(two + '\n"a":::cls'),
  rd('requirement a:::cls {\nid: 1\n}\nrequirement a:::other {\nid: 2\n}\nclassDef cls fill:red\nclassDef other fill:blue'),
  rd('classDef cls fill:red\nrequirement a:::cls {\nid: 1\n}\nrequirement a:::cls {\nid: 2\n}'),
  rd('requirement a:::default {\nid: 1\n}\nclassDef default fill:red'),
  // Accessibility and title.
  rd('accTitle: The accessible title\naccDescr: The accessible description\n' + two),
  rd('accDescr {\n  A description\n  on several lines\n}\n' + two),
  rd('accTitle:    padded   \naccDescr:   padded too  \n' + two),
  rd('accTitle: one\naccTitle: two\naccDescr: one\naccDescr { two }\n' + two),
  rd('accTitle: <b>bold</b> #35; title\naccDescr: <script>x</script> descr\n' + two),
  rd('accTitle: a <a href="https://example.com">link</a>\naccDescr: b <a href="javascript:x">l</a>\n' + two),
  rd('accTitle:\n' + two),
  rd('accDescr:\n' + two),
  rd('accDescr {}\n' + two),
  rd('accDescr {\nunterminated\n' + two),
  rd('acctitle: lower\naccdescr: lower\n' + two),
  rd('accTitle : spaced colon\naccDescr : spaced colon\n' + two),
  rd(two + '\naccTitle: after the nodes\naccDescr {\n after\n}'),
  rd(req('a', 'id: 1\naccTitle: inside')),
  'accTitle: before the keyword\nrequirementDiagram\n' + two + '\n',
  'accDescr: before the keyword\n\nrequirementDiagram\n' + two + '\n',
  'accDescr {\n before\n}\nrequirementDiagram\n' + two + '\n',
  'accTitle: one\naccDescr: two\nrequirementDiagram\n' + two + '\n',
  rd(two, '---\ntitle: The title\n---\n'),
  rd(two, '---\ntitle: "<b>Title</b> with #35;"\n---\n'),
  rd('title The title\n' + two),
  rd(two + '\ntitle The title'),
  // Comments and separators.
  rd('%% a comment\n' + two + '\n  %% another'),
  rd('# a comment\n' + two + '\n  # another'),
  rd(req('a', 'id: 1 %% trailing')),
  rd(req('a', 'id: 1\n# inside\n%% inside')),
  rd(two + ';'),
  rd(two + '\n;'),
  rd(two + '\n@'),
  rd(two + '\n$'),
  rd(req('a').replace(/\n/g, '\r\n') + '\r\n' + el('b').replace(/\n/g, '\r\n') + '\r\n'),
  rd(req('a').replace(/\n/g, '\n\t')),
  rd(req('a') + '\n}'),
  rd(req('a') + '\n{'),
  rd(req('a') + '\n:'),
  rd(req('a') + '\n,'),
  rd(req('a') + '\n-'),
  rd(req('a') + '\n->'),
  rd(req('a') + '\n<-'),
  rd(req('a') + '\n"'),
  rd(req('a') + '\n""'),
  rd(req('a') + '\nb'),
  rd(req('a') + '\n5'),
  // Config.
  rd(two + '\na - contains -> b', cfg('look: handDrawn')),
  rd(two + '\na - contains -> b', cfg('look: neo')),
  rd(two + '\na - contains -> b', cfg('look: classic')),
  rd(two + '\na - contains -> b', cfg('look: classic\ntheme: forest')),
  rd(two + '\na - contains -> b', cfg('look: other')),
  rd(two + '\na - traces -> b', cfg('theme: dark')),
  rd(two + '\na - traces -> b', cfg('theme: redux-dark-color')),
  rd(two + '\na - traces -> b', cfg('htmlLabels: false')),
  rd(two + '\na - traces -> b', cfg('flowchart:\n  htmlLabels: false')),
  rd(two + '\na - traces -> b', cfg('state:\n  nodeSpacing: 10\n  rankSpacing: 20')),
  rd(two + '\na - traces -> b', cfg('state:\n  nodeSpacing: 0\n  rankSpacing: 0')),
  rd(two + '\na - traces -> b', cfg('state:\n  nodeSpacing: "30"\n  rankSpacing: "40"')),
  rd(two + '\na - traces -> b', cfg('state:\n  titleTopMargin: 50\n  useMaxWidth: false')),
  rd(two + '\na - traces -> b', cfg('requirement:\n  nodeSpacing: 7\n  rankSpacing: 9\n  useMaxWidth: false\n  fontSize: 20')),
  rd(two + '\na - traces -> b', cfg('requirement:\n  look: classic\n  theme: forest\n  layout: dagre')),
  rd(two + '\na - traces -> b', cfg('flowchart:\n  nodeSpacing: 7\n  rankSpacing: 9')),
  rd(two + '\na - traces -> b', cfg('nodeSpacing: 11\nrankSpacing: 13')),
  rd(two + '\na - traces -> b', cfg('layout: dagre')),
  rd(two + '\na - traces -> b', cfg('layout: elk')),
  rd(two + '\na - traces -> b', cfg('layout: elk.stress')),
  rd(two + '\na - traces -> b', cfg('layout: nosuchlayout')),
  rd(two + '\na - traces -> b', cfg('layout: 5')),
  rd(two + '\na - contains -> b', cfg('look: handDrawn\nlayout: dagre\nhandDrawnSeed: 7')),
  rd(two + '\na - traces -> b', '%%{init: {"state": {"nodeSpacing": 99}, "look": "classic"}}%%\n'),
  rd(two + '\na - traces -> b', '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#ff0000"}}}%%\n'),
  rd(two + '\na - traces -> b', cfg('securityLevel: loose')),
  rd(two + '\na - traces -> b', cfg('fontFamily: courier\nfontSize: 20')),
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Inputs that need a site config run in their own page.
const sited = [
  { site: { securityLevel: 'loose' }, text: rd('accTitle: <i>t</i> <script>y</script>\n' + req('a', 'text: "<b>bold</b> <script>x</script>"')) },
  { site: { securityLevel: 'antiscript' }, text: rd('accDescr: <script>y</script> d\n' + req('a', 'text: "<b>bold</b> <script>x</script>"')) },
  { site: { securityLevel: 'other' }, text: rd('accTitle: x=y\n' + two) },
  { site: { look: 'handDrawn', theme: 'forest' }, text: rd(two + '\na - contains -> b') },
  { site: { look: 'neo', theme: 'redux-dark-color' }, text: rd(two + '\na - traces -> b') },
  { site: { look: 'classic' }, text: rd(two + '\na - traces -> b', cfg('look: neo')) },
  { site: { htmlLabels: false }, text: rd(two + '\na - traces -> b') },
  { site: { state: { nodeSpacing: 55, rankSpacing: 66 } }, text: rd(two + '\na - traces -> b') },
  { site: { requirement: { look: 'handDrawn', layout: 'dagre' } }, text: rd(two + '\na - traces -> b') },
  { site: { layout: 'dagre' }, text: rd(two + '\na - traces -> b') },
  { site: { layout: 'dagre' }, text: rd(two + '\na - traces -> b', cfg('layout: elk')) },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: rd('accTitle: <b>x</b> <i>y</i>\n' + two) },
  { site: { state: { useMaxWidth: false } }, text: rd(two) },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'reqdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh: !!fresh, site })) }));
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'requirement-db-page.js', '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (result.cases.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${result.cases.length}`);
}

// The config is large and almost the same for every case. The golden file
// has the config of the first case, and for each other case the leaves that
// differ: [path, value] to set and paths to delete.
const flatten = (v, prefix, out) => {
  if (v !== null && typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length > 0) {
    for (const k of Object.keys(v)) {
      flatten(v[k], [...prefix, k], out);
    }
  } else {
    out.set(JSON.stringify(prefix), v);
  }
  return out;
};
const base = result.cases.find((r) => r.config);
const baseFlat = flatten(base.config, [], new Map());
const golden = result.cases.map((r, i) => {
  const c = cases[i];
  // The text of a corpus fixture is read from the corpus by the test.
  const rec = c.corpus ? { ...r } : { ...r, text: c.text };
  if (c.fresh) {
    rec.fresh = true;
  }
  if (c.site) {
    rec.site = c.site;
  }
  if (r.config) {
    const flat = flatten(r.config, [], new Map());
    const set = [];
    const del = [];
    for (const [k, v] of flat) {
      if (!baseFlat.has(k) || JSON.stringify(baseFlat.get(k)) !== JSON.stringify(v)) {
        set.push([JSON.parse(k), v]);
      }
    }
    for (const k of baseFlat.keys()) {
      if (!flat.has(k)) {
        del.push(JSON.parse(k));
      }
    }
    delete rec.config;
    rec.configDelta = { set, del };
  }
  return rec;
});
const doc = { baseConfig: base.config, cases: golden };
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(doc), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
const dumped = golden.filter((r) => r.nodes).length;
console.log(`${outFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${dumped} with a db, ${errors} with an error`);
