// Writes internal/diagrams/sequence/testdata/db.json.gz: the content of the
// sequence db of real mermaid (actors, messages, boxes, created and destroyed
// actors, title, accessibility values) after it parses
//   - every fixture of testdata/corpus/mermaid-e2e/sequence,
//   - seeded mutations of the body of those fixtures,
//   - hand-written inputs for the branches of sequenceDb.ts.
// The db needs a browser (DOMPurify, CSS.supports), so the script runs
// sequence-db-page.js with the eval command of the browser oracle, one time.
//
// Usage (in tools/reference): node gen-sequence-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const family = 'testdata/corpus/mermaid-e2e/sequence';
const outFile = path.join(repoRoot, 'internal/diagrams/sequence/testdata/db.json.gz');
const mutantsPerFixture = 3;

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

const fixtures = fs
  .readdirSync(path.join(repoRoot, family), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => family + '/' + f.split(path.sep).join('/'))
  .sort();

const snippets = [
  'Alice->>Bob: hi',
  'Bob-->>-Alice: bye',
  'Alice->>+Bob: start',
  'activate Alice',
  'deactivate Alice',
  'deactivate Bob',
  'Note over Alice,Bob: wrap: a note',
  'Note left of Alice: nowrap: left',
  'loop every minute',
  'alt :wrap: maybe',
  'else otherwise',
  'opt',
  'par one',
  'and two',
  'critical danger',
  'option fallback',
  'break stop',
  'rect rgb(200, 220, 255)',
  'end',
  'autonumber',
  'autonumber 5 2',
  'autonumber off',
  'create participant Zed',
  'destroy Bob',
  'box Aqua Group',
  'participant Alice as wrap:The first one',
  'actor Q@{ "type": "queue" }',
  'links Alice: {"Docs": "https://example.com"}',
  'link Alice: Wiki @ https://example.org/wiki',
  'properties Alice: {"class": "internal"}',
  'title A title',
  'accTitle: Acc',
  'Alice ()->>() Bob: both',
  'Bob //-- Alice: stick',
  ';',
  '#35; text',
  '%% comment',
];

// mutate changes the lines after the "sequenceDiagram" line.
function mutate(rnd, text) {
  const lines = text.split('\n');
  let start = lines.findIndex((l) => /^\s*sequenceDiagram/.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 6);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind === 3) {
      lines.splice(p, 0, '  ' + snippets[Math.floor(rnd() * snippets.length)]);
    } else if (kind === 4) {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    } else {
      lines.length = Math.max(start, p);
    }
  }
  return lines.join('\n');
}

const sd = (body, front = '') => front + 'sequenceDiagram\n' + body + '\n';
const extras = [
  sd(''),
  sd('Alice->>Bob: Hello'),
  sd('Alice->>Bob: wrap: Hello there'),
  sd('Alice->>Bob: nowrap: Hello there'),
  sd('Alice->>Bob::wrap: Hello there'),
  sd('Alice->>Bob:'),
  sd('participant A as wrap:A long description\nparticipant B as nowrap: B\nA->>B: x'),
  sd('participant A as Alice\nparticipant A as Again\nparticipant A\nA->>A: self'),
  sd('actor A\nparticipant A\nA->>B: implicit'),
  sd('participant A@{ "type": "boundary" }\nparticipant B@{ "type": "control" } as Ctl\nparticipant C@{ "type" : "entity", "alias": "Ent" }\nparticipant D@{ "alias": "Dee" } as External\nactor E@{ "type": "database" }\nparticipant F@{ type: queue }\nparticipant G@{ "type": "collections" }\nA->>B: x'),
  sd('participant A@{\n  "type": "database",\n  "alias": "Multi"\n}\nA->>A: y'),
  sd('participant A@{ "type": 5 }\nA->>A: y'),
  sd('participant A@{ not yaml: [ }\nA->>A: y'),
  sd('box Aqua Group\nparticipant A\nparticipant B\nend\nbox rgb(1,2,3)\nparticipant C\nend\nbox rgba(1,2,3,0.5) Title here\nparticipant D\nend\nbox hsl(120, 50%, 50%) wrap: Wrapped\nparticipant E\nend\nbox\nparticipant F\nend\nbox transparent T\nparticipant G\nend\nbox NotAColor\nparticipant H\nend\nA->>H: x'),
  sd('box red One\nparticipant A\nend\nbox blue Two\nparticipant A\nend'),
  sd('box red One\nparticipant A\nend\nparticipant A\nA->>B: ok'),
  sd('participant A\nbox red One\nparticipant A\nend\nA->>B: ok'),
  sd('box hsla(0, 100%, 50%, 0.3) :nowrap: x\nactor A\nend'),
  sd('box LightYellow\nactor A\nend\nbox #35;fff Hex\nactor B\nend'),
  sd('Alice->>+Bob: a\nAlice->>+Bob: b\nBob-->>-Alice: c\nBob-->>-Alice: d'),
  sd('Alice->>Bob: a\nBob-->>-Alice: c'),
  sd('deactivate Alice'),
  sd('activate Alice\ndeactivate Alice\ndeactivate Alice'),
  sd('activate Alice\nAlice->>Bob: x\ndeactivate Alice'),
  sd('Note left of Alice: left\nNote right of Alice: right\nNote over Alice: over\nNote over Alice,Bob: both\nNote over Bob,Alice: wrap: both wrapped'),
  sd('note over A, B, C: three'),
  sd('loop L\nA->>B: x\nend\nopt O\nA->>B: x\nend\nalt A1\nA->>B: x\nelse A2\nA->>B: y\nelse\nA->>B: z\nend'),
  sd('par P1\nA->>B: x\nand P2\nA->>B: y\nand\nA->>B: z\nend\npar_over PO\nA->>B: x\nand\nA->>B: y\nend'),
  sd('critical C\nA->>B: x\noption O1\nA->>B: y\noption\nA->>B: z\nend\nbreak B\nA->>B: x\nend'),
  sd('rect rgb(1,2,3)\nA->>B: x\nrect\nA->>B: y\nend\nend\nrect wrap: red\nA->>B: z\nend'),
  sd('loop wrap: long loop title\nA->>B: x\nend\nloop :nowrap:t\nA->>B: x\nend'),
  sd('autonumber\nA->>B: x\nautonumber 10\nA->>B: x\nautonumber 10 5\nA->>B: x\nautonumber off\nA->>B: x\nautonumber 1.5 .25\nA->>B: x'),
  sd('autonumber 1.234 2\nA->>B: x'),
  sd('create participant C\nA->>C: hi\ndestroy C\nC->>A: bye'),
  sd('create actor C as Carl\nA->>C: hi\ndestroy A\nC-xA: bye'),
  sd('create participant C\nA->>B: wrong'),
  sd('participant C\ncreate participant C\nA->>C: dup'),
  sd('A->>B: x\ndestroy B\nA->>C: wrong'),
  sd('create participant C@{ "type": "queue" } as Q\nA->>C: hi'),
  sd('links A: {"Repo": "https://github.com/x", "Wiki": "https://w.example"}\nlinks A: {"Repo": "https://other"}\nlink A: Dash @ https://d.example/a?b=c&d=e\nlink A: NoAt\nproperties A: {"class": "internal-service-actor", "icon": "@computer"}\nproperties B: not json\nlinks B: [1,2]\ndetails A: someId'),
  sd('participant A\nlinks A: {"x": "javascript:alert(1)"}\nlink A: <b>bold</b> @ https://x.example'),
  sd('title My title\nA->>B: x'),
  sd('title: Legacy title\nA->>B: x'),
  sd('accTitle: The acc title\naccDescr: The description\nA->>B: x'),
  sd('accDescr {\n  multi\n  line\n}\nA->>B: x'),
  sd('A->>B: x\nA-->>B: x\nA->B: x\nA-->B: x\nA-xB: x\nA--xB: x\nA-)B: x\nA--)B: x\nA<<->>B: x\nA<<-->>B: x'),
  sd('A-|\\B: x\nA-|/B: x\nA-\\\\B: x\nA-//B: x\nA--|\\B: x\nA--|/B: x\nA--\\\\B: x\nA--//B: x'),
  sd('A/|-B: x\nA\\|-B: x\nA//-B: x\nA\\\\-B: x\nA/|--B: x\nA\\|--B: x\nA//--B: x\nA\\\\--B: x'),
  sd('A->>()B: x\nA()->>B: y\nA()->>()B: z\nA()<<->>()B: w\nA-->>()A: self'),
  sd('A->>B: #35; hash #lt;tag#gt; #9829; #amp;'),
  sd('A->>B: line1<br/>line2<br>line3<br />line4'),
  sd('A->>B: <script>alert(1)</script> <b>bold</b> <img src=x onerror=y>'),
  sd('participant A as <b>Bold</b><br/>Second\nA->>B: x'),
  sd('participant Alice;participant Bob;Alice->>Bob: semi;Bob->>Alice: colons'),
  sd('%% a comment\nA->>B: x %% not a comment\n# hash comment\nA->>B: y # trailing'),
  sd('A->>B: x', '---\nconfig:\n  wrap: true\n---\n'),
  sd('A->>B: x\nNote over A: n\nparticipant C as nowrap:C', '---\nconfig:\n  sequence:\n    wrap: true\n---\n'),
  sd('A->>B: x', '%%{init: {"wrap": true}}%%\n'),
  sd('A->>B: x', '%%{wrap}%%\n'),
  sd('A->>B: x', '---\ntitle: Front title\n---\n'),
  sd('participant link\nlink->>B: x\nlink B: L @ https://x.example'),
  sd('participant A B C\nA B C->>D E: spaces'),
  sd('Alice-->>John: Hi\nJohn-->>Alice: 你好 😀 é'),
  sd('participant 😀 as Emoji\n😀->>B: x'),
  sd('A->>B: x\nend'),
  sd('loop\nA->>B: x'),
  sd('A->>'),
  sd('participant'),
  sd('box\nA->>B: x\nend'),
  sd('participant A<br>\nA->>B: x'),
  'SEQUENCEDIAGRAM\nparticipant A\nA->>B: caps\nNOTE OVER A: caps\nLOOP x\nA->>B: y\nEND\n',
  'sequenceDiagram;A->>B: one line;',
  '\n\n  sequenceDiagram\n\n\n  A->>B: blank lines\n\n\n',
];

const rnd = prng(0x73657164);
const cases = [];
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, f), 'utf8');
  cases.push({ name: f, text, corpus: true });
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'seqdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text }) => ({ name, text })) }));
execFileSync(
  process.execPath,
  ['oracle.mjs', 'eval', 'sequence-db-page.js', '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
const golden = results.map((r, i) => {
  // The text of a corpus fixture is read from the corpus by the test.
  const rec = cases[i].corpus ? { ...r } : { ...r, text: cases[i].text };
  return rec;
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
console.log(`${outFile}: ${golden.length} cases, ${errors} with an error`);
