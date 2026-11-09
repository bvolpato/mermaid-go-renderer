// Writes internal/diagrams/er/testdata/db.json.gz: the content of the ER db
// of real mermaid (the entities, the relationships, the classes, the
// subgraphs, the direction, the title, the accessibility values) and the
// layout data that the renderer hands to the layout (the nodes and edges of
// getData, the config, the direction, the markers, the layout algorithm), for
//   - every fixture of the corpus that mermaid renders as an ER diagram,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of erDb.ts and of the grammar.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs er-db-page.js with
// the esm-eval command of the browser oracle, one time. A corpus fixture
// runs in a new page, as in the reference renders.
//
// Usage (in tools/reference): node gen-er-db.mjs
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
const family = 'er-diagram/';
const outFile = path.join(repoRoot, 'internal/diagrams/er/testdata/db.json.gz');
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
// The fixtures of the corpus: every fixture of the ER family, and the
// fixtures of other families that the reference rendered as an ER diagram.

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
      return meta.diagramType === 'er';
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
  'CUSTOMER ||--o{ ORDER : places',
  'ORDER ||--|{ LINE-ITEM : contains',
  'A }|..|{ B : uses',
  'A }o--o{ B : "has many"',
  'A |o--o| B : ""',
  'A }o..o| B : "a label with spaces"',
  'A one or zero to many B : label',
  'A only one to zero or more B : x',
  'A 1+ optionally to 0+ B : y',
  'A many(1) to many(0) B : z',
  'A one or many optionally to zero or one B : w',
  'A u--o{ B : md',
  'A ||--u B : md2',
  'A 1--1 B : one',
  'A 1 to 1 B : one',
  'A ||--|| A : self',
  'A ||.-|| B : dotdash',
  'A ||-.|| B : dashdot',
  '"Quoted Name" ||--o{ B : rel',
  'A ||--o{ "Other Quoted" : "quoted role"',
  'ENT { string id PK "the id" }',
  'ENT {\n  int a\n  string b FK\n  date c PK, FK "both"\n  type~T~ d UK\n}',
  'ENT {\n  string? nullable\n  varchar(99) name\n  string[] list\n  `a b` `c d`\n}',
  'ENT { }',
  'ENT {}',
  'ENT[Alias] { string x }',
  'ENT["Quoted alias"]',
  'ENT[Alias]:::cls',
  'ENT[Alias]:::cls { string z "c" }',
  'ENT:::cls',
  'ENT:::cls1,cls2 { string y }',
  'ENT:::cls {}',
  'A:::c1 ||--o{ B:::c2 : styled',
  'A:::c1 ||--o{ B : left',
  'A ||--o{ B:::c2 : right',
  'classDef cls fill:#f9f,stroke:#333,stroke-width:4px',
  'classDef cls1,cls2 color:#fff,fill:#00f',
  'classDef default fill:#eee',
  'class ENT cls',
  'class A,B cls1,cls2',
  'style ENT fill:#bbf,stroke:#f66',
  'style A,B color:red,stroke-width:2px',
  'direction LR',
  'direction TB',
  'direction BT',
  'direction RL',
  'subgraph SG',
  'subgraph SG2 [A title here]',
  'subgraph "quoted id"',
  'end',
  'accTitle: The title',
  'accDescr: The description',
  'accDescr {\n  multi\n  line\n}',
  '1 ||--o{ 2 : nums',
  '1.5 ||--|| 2.5 : dec',
  '%% comment',
  'A ||--o{ B : #35;hash#lt;',
  '}',
  '{',
  ':',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its text can have a line that starts with "erDiagram".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*erDiagram/.test(l));
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

const rnd = prng(0x65724462);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const er = (body, front = '') => front + 'erDiagram\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const cards = ['||', '|o', 'o|', '}o', 'o{', '}|', '|{', 'u'];
const extras = [
  // Headers.
  'erDiagram\nA\n',
  'erDiagram',
  'erDiagram\n',
  'erDiagram\n\n\n',
  '\n\n   erDiagram   \n\n\nA\n\n',
  'erDiagram A\n',
  'erDiagram A ||--o{ B : x',
  'ERDIAGRAM\nA\n',
  'erdiagram\nA\n',
  'erDiagramX\nA\n',
  'erDiagram;\nA\n',
  // Entities.
  er('A\nB\nA\nC'),
  er('"Quoted Entity"\n"another one"\n"with-dash"\n"with.dot"'),
  er('A-B\nA_B\nA.B\nA*B\n*\n-\n.'),
  er('1\n2\n33\n4.5\n1.0\n007'),
  er('Ünïcödé\n日本語\nемкость\n😀'),
  er('"A % B"'),
  er('"A\\B"'),
  er('""'),
  er('"a" "b"'),
  er('A B'),
  er('one\nmany\nto\nend\nclass\nstyle\nu'),
  // Aliases.
  er('A[Alias]\nB["Quoted alias"]\nC[with-dash]\nD[1]\nE[2.5]\nF[42]'),
  er('A[First]\nA[Second]\nA'),
  er('A\nA[Later alias]'),
  er('A[Alias] {\n  string x\n}\nA {\n  int y\n}'),
  er('A[Alias]:::cls\nB[Alias2]:::c1,c2 {\n  string x\n}\nC[Alias3]:::cls {}\nD[Alias4] {}\nclassDef cls fill:red'),
  er('A[Alias with spaces]'),
  er('A[]'),
  er('A["`md **alias**`"]'),
  er('A["<b>html</b> alias <script>x</script>"]'),
  // Attributes.
  er('A {\n  string name\n}'),
  er('A {\n  string name\n  int age\n  date born\n}'),
  er('A {\n  string name PK\n  int age FK\n  date born UK\n  x y PK, FK\n  z w PK,FK,UK\n}'),
  er('A {\n  string name "a comment"\n  int age PK "key and comment"\n  date born PK, FK "two keys"\n  e f ""\n}'),
  er('A {\n  string name pk\n  int age Fk\n  date born uK "lower"\n}'),
  er('A { string name }'),
  er('A { string name int age }'),
  er('A { string name PK "c" int age FK }'),
  er('A {\n}'),
  er('A {}'),
  er('A { }'),
  er('A {\n\n\n}'),
  er('A {\n  string a\n}\nA {\n  int b\n}\nA {\n}'),
  er('A {\n  type~T~ generic\n  List~int~ numbers\n  Map~K,V~ pairs\n  a~b~c d~e~f\n}'),
  er('A {\n  string[] list\n  varchar(99) name\n  decimal(10,2) price\n  a.b c.d\n  a-b c-d\n  *star *name\n  _under _score\n}'),
  er('A {\n  string? nullable\n  int? count PK\n}'),
  er('A {\n  `a b` `c d`\n  `x-y` z "bq"\n  t `quoted name` PK\n}'),
  er('A {\n  `unterminated name\n}'),
  er('A {\n  string 1name\n}'),
  er('A {\n  1type name\n}'),
  er('A {\n  string\n}'),
  er('A {\n  string name extra\n}'),
  er('A {\n  string name PK FK\n}'),
  er('A {\n  string name "c" PK\n}'),
  er('A {\n  string name PK,\n}'),
  er('A {\n  string name PKX\n}'),
  er('A {\n  Ünï côd "ça"\n  日本 語\n}'),
  er('A {\n  string name "a ""\n}'),
  er('A {\n  string name "unterminated\n}'),
  er('A {\n  string name # c\n}'),
  er('A {\n  string name'),
  er('A {\n  string name PK "#35; hash #lt;b#gt;"\n}'),
  er('A {\n  string name "<b>bold</b>"\n}'),
  // Relationships: every pair of cardinalities and both identifications.
  er(cards.flatMap((l, i) => cards.map((r, j) => `L${i} ${l}--${r} R${j} : c${i}${j}`)).join('\n')),
  er(cards.flatMap((l, i) => cards.map((r, j) => `L${i} ${l}..${r} R${j} : d${i}${j}`)).join('\n')),
  er('A ||.-|| B : a\nC ||-.|| D : b'),
  er('A only one to one or more B : a\nC zero or one to zero or more D : b\nE one or zero to one or many F : c\nG zero or many to many(1) H : d\nI many(0) to 1+ J : e\nK 0+ to many L : f\nM one to only one N : g'),
  er('A only one optionally to one or more B : a\nC zero or one optionally to zero or more D : b\nE many optionally to one F : c'),
  er('A 1 to 1 B : a\nC 1--1 D : b\nE 1..1 F : c\nG 1 optionally to 1 H : d\nI 1 to many J : e'),
  er('1 ||--|| 2 : a\n1 1--1 1 : b\n1 one to one 1 : c'),
  er('1.5 ||--|| 2.5 : a\n3 ||--o{ 4.0 : b'),
  er('u ||--|| v : a\nA u--o{ B : b\nA ||--u B : c\nu u--u u : d\nA u..u B : e'),
  er('A ||--|| A : self\nA }o--o{ A : self2'),
  er('A ||--o{ B : one\nA ||--o{ B : two\nB }o--|| A : three'),
  er('A ||--o{ B : "quoted role"\nC ||--o{ D : ""\nE ||--o{ F : " "\nG ||--o{ H : "with % percent"\nI ||--o{ J : "with \\\\ backslash"'),
  er('A ||--o{ B : role-with-dash\nC ||--o{ D : role.with.dot\nE ||--o{ F : 5\nG ||--o{ H : 1'),
  er('A ||--o{ B : Ünï\nC ||--o{ D : "日本語 role"'),
  er('A ||--o{ B : "line<br/>break"\nC ||--o{ D : "`md **role**`"'),
  er('"Q A" ||--o{ "Q B" : "q"\n"Q A" ||--o{ C : r'),
  er('A ||--o{ B'),
  er('A ||--o{ B :'),
  er('A ||--o{ : x'),
  er('||--o{ B : x'),
  er('A || B : x'),
  er('A -- B : x'),
  er('A ||--o{ B : two words'),
  er('A ||--o{ B : x y'),
  er('A||--o{B:x'),
  er('A ||--o{B : x\nC}o--o{D : y'),
  er('A }|--|{ B:x'),
  // Classes and styles.
  er('A:::cls\nclassDef cls fill:#f9f,stroke:#333,stroke-width:4px'),
  er('classDef cls fill:#f9f\nA:::cls\nB:::cls { string x }\nC:::cls {}\nD:::nodef'),
  er('A:::c1,c2\nB:::c1, c2\nclassDef c1 fill:red\nclassDef c2 stroke:blue'),
  er('A:::c1 ||--o{ B:::c2 : both\nC:::c1 ||--o{ D : left\nE ||--o{ F:::c2 : right\nclassDef c1 fill:red\nclassDef c2 color:white,fill:black'),
  er('A:::c1,c2 ||--o{ B:::c2,c1 : lists'),
  er('classDef a,b,c fill:red,color:blue\nA:::a\nB:::b\nC:::c'),
  er('classDef cls color:red,fill:blue,background-color:green,stroke:colorful\nA:::cls'),
  er('classDef cls fill:red\nclassDef cls stroke:blue\nclassDef cls color:green\nA:::cls'),
  er('classDef cls font-size:20px,font-weight:bold,font-family:Arial\nA:::cls'),
  er('classDef cls stroke-dasharray: 5 5,stroke-width:2px\nA:::cls'),
  er('classDef cls fill:#f00;\nA:::cls'),
  er('classDef cls fill:#f00;classDef other fill:#0f0;A:::cls'),
  er('classDef cls'),
  er('classDef'),
  er('classDef cls fill'),
  er('classDef cls fill:\nA:::cls'),
  er('classDef cls fill:red,\nA'),
  er('classDef default fill:#eee,stroke:#111\nA\nB'),
  er('classDef Ünï fill:red\nA:::Ünï'),
  er('A\nB\nclass A cls\nclass A,B c1,c2\nclassDef cls fill:red\nclassDef c1 fill:blue\nclassDef c2 color:white'),
  er('class A cls\nA\nclass missing cls'),
  er('class A\nA'),
  er('class\nA'),
  er('A\nclass A cls other'),
  er('A\nB\nstyle A fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5\nstyle B color:red'),
  er('A\nB\nstyle A,B fill:red\nstyle A stroke:blue'),
  er('style A fill:red\nA'),
  er('A\nstyle missing fill:red'),
  er('A\nstyle A'),
  er('A\nstyle A fill:#f00;'),
  er('A\nstyle A fill:#f00;B'),
  er('A\nstyle A fill : red , stroke : blue'),
  er('A\nstyle A fill:red color:blue'),
  er('A\nstyle A font-size:12px,x:1.5,y:#abc'),
  er('A:::cls\nstyle A fill:yellow\nclassDef cls fill:red,stroke:blue'),
  er('CLASSDEF cls fill:red\nA:::cls\nCLASS A cls\nSTYLE A fill:blue'),
  // Direction.
  er('direction LR\nA ||--o{ B : x'),
  er('direction TB\nA'),
  er('direction BT\nA'),
  er('direction RL\nA'),
  er('direction lr\nA'),
  er('direction LR\ndirection RL\nA'),
  er('A\n  direction   LR  \nB'),
  er('direction XY\nA'),
  er('direction\nA'),
  er('direction LRX\nA'),
  er('A direction LR B'),
  er('direction TD\nA'),
  // Accessibility and title.
  er('accTitle: The accessible title\naccDescr: The accessible description\nA'),
  er('accDescr {\n  A description\n  on several lines\n}\nA'),
  er('accTitle:    padded   \naccDescr:   padded too  \nA'),
  er('accTitle: one\naccTitle: two\naccDescr: one\naccDescr { two }\nA'),
  er('accTitle: <b>bold</b> #35; title\naccDescr: <script>x</script> descr\nA'),
  er('accTitle:\nA'),
  er('accDescr {}\nA'),
  er('accDescr {\nunterminated\nA'),
  er('acctitle: lower\naccdescr: lower\nA'),
  er('A', '---\ntitle: The title\n---\n'),
  er('A', '---\ntitle: "<b>Title</b> with #35;"\n---\n'),
  er('title The title\nA'),
  // Subgraphs.
  er('subgraph S\nA\nB\nend'),
  er('subgraph S\nA ||--o{ B : x\nend'),
  er('subgraph S [The title]\nA\nend'),
  er('subgraph S [Title with many words 1 2.5]\nA\nend'),
  er('subgraph S ["Quoted title"]\nA\nend'),
  er('subgraph "Quoted id"\nA\nend'),
  er('subgraph "Quoted id" ["Quoted title"]\nA\nend'),
  er('subgraph S\nend'),
  er('subgraph S\n\nend'),
  er('subgraph S;A;end'),
  er('subgraph S\nA\nend\nsubgraph T\nB\nend\nS ||--o{ T : between'),
  er('subgraph S\nA\nend\nsubgraph T\nB\nend\nA ||--o{ B : across\nS ||--|| B : mixed\nA ||--|| T : mixed2'),
  er('S ||--o{ T : before\nsubgraph S\nA\nend\nsubgraph T\nB\nend'),
  er('subgraph Outer\nsubgraph Inner1\nA\nend\nsubgraph Inner2\nB\nend\nA ||--o{ B : x\nend\nsubgraph Sibling\nC\nend'),
  er('subgraph Outer\nsubgraph Mid\nsubgraph Inner\nA\nend\nB\nend\nC\nend'),
  er('subgraph S\ndirection LR\nA ||--o{ B : x\nend\nsubgraph T\ndirection BT\nC\nend\nsubgraph U\ndirection TB\nD\nend\nsubgraph V\n  direction RL\nE\nend'),
  er('subgraph S\ndirection LR\ndirection TB\nA\nend'),
  er('direction RL\nsubgraph S\ndirection LR\nA\nend\ndirection BT'),
  er('subgraph S\nsubgraph T\ndirection LR\nA\nend\ndirection BT\nB\nend'),
  er('subgraph S\nA\nend\nsubgraph S\nB\nend'),
  er('subgraph S\nA\nend\nsubgraph T\nA\nB\nend'),
  er('subgraph S\nA\nA\nB\nB\nend'),
  er('subgraph S\nA ||--|| A : self\nA ||--o{ B : x\nB }o--|| A : y\nend'),
  er('subgraph S\nA[Alias]\nB[Alias2] { string x }\nC:::cls\nD { int y }\nE {}\nend'),
  er('subgraph S\nA\nend\nclassDef cls fill:red,color:white\nclass S cls\nstyle S stroke:blue,fill:#eee'),
  er('subgraph S\nA\nclassDef cls fill:red\nclass A cls\nstyle A fill:blue\nend'),
  er('subgraph S\naccTitle: in subgraph\naccDescr: descr in subgraph\nA\nend'),
  er('subgraph S\naccDescr {\n  multi\n}\nA\nend'),
  er('subgraph S\nA\nend\nS:::cls\nclassDef cls fill:red'),
  er('subgraph S\nS\nend'),
  er('A\nsubgraph A\nB\nend'),
  er('A ||--o{ B : x\nsubgraph A\nC\nend\nA ||--o{ B : y'),
  er('subgraph 1\nA\nend\nsubgraph 2.5\nB\nend\n1 ||--o{ 2.5 : nums'),
  er('subgraph S-1 [a-b]\nA\nend\nsubgraph S.T\nB\nend\nsubgraph S*T\nC\nend'),
  er('subgraph S ["`md **title**`"]\nA\nend'),
  er('subgraph S ["<b>html</b> title <script>x</script>"]\nA\nend'),
  er('subgraph S [T]\n  A\n  end\n  subgraph U\n  B\n  end  \n'),
  er('subgraph S\nA\nend\nstyle S fill:red\nstyle S stroke:blue'),
  er('subgraph S\nA\nend\nclass S c1,c2\nclassDef c1 fill:red\nclassDef c2 stroke:blue,color:green'),
  er('subgraph  S   [  Padded   title  ]  \nA\nend'),
  er('subgraph " padded id "\nA\nend'),
  er('subgraph S\n"  spaced  "\n" spaced "\nend'),
  er('subgraph S [T]:::cls\nA\nend'),
  er('subgraph\nA\nend'),
  er('subgraph S\nA'),
  er('subgraph S [T\nA\nend'),
  er('subgraph S []\nA\nend'),
  er('end'),
  er('A\nend'),
  er('subgraph S\nA\nend\nend'),
  er('subgraph S A\nend'),
  er('subgraph S\nA\nend B'),
  er('subgraph S\nA\nendX\nend'),
  er('SUBGRAPH S\nA\nEND'),
  er('subgraph S\nA\nend\nsubgraph T\nS\nend'),
  // Entity codes and special characters.
  er('A ||--o{ B : "#35; hash #lt;b#gt;"'),
  er('"A#35;B"\n"#quot;q#quot;"'),
  er('A {\n  string name "#9829; heart"\n}'),
  er('A["#lt;alias#gt;"]'),
  er('%% a comment\nA\n  %% another\nB %% trailing'),
  er('A ||--o{ B : x %% trailing comment'),
  er('A;B'),
  er('A #\nB'),
  er('A,B'),
  er('A:B'),
  er('A : B'),
  er('A ::: B'),
  er('A:::'),
  er('A\t\tB\r\nC\r\n'),
  er('@\nA'),
  er('A\n$'),
  // Config.
  er('A ||--o{ B : x', cfg('look: handDrawn')),
  er('A ||--o{ B : x', cfg('look: neo')),
  er('A ||--o{ B : x', cfg('look: classic\ntheme: forest')),
  er('A ||--o{ B : x', cfg('theme: dark')),
  er('A ||--o{ B : x', cfg('theme: redux-color')),
  er('A ||--o{ B : x', cfg('htmlLabels: false')),
  er('A ||--o{ B : x', cfg('flowchart:\n  htmlLabels: false')),
  er('A ||--o{ B : x', cfg('er:\n  nodeSpacing: 10\n  rankSpacing: 20')),
  er('A ||--o{ B : x', cfg('er:\n  nodeSpacing: 0\n  rankSpacing: 0')),
  er('A ||--o{ B : x', cfg('er:\n  nodeSpacing: "30"\n  rankSpacing: "40"')),
  er('A ||--o{ B : x', cfg('er:\n  layoutDirection: LR\n  diagramPadding: 30\n  entityPadding: 20\n  minEntityWidth: 200\n  minEntityHeight: 100\n  fontSize: 20\n  useMaxWidth: false\n  titleTopMargin: 50')),
  er('A ||--o{ B : x', cfg('flowchart:\n  nodeSpacing: 7\n  rankSpacing: 9')),
  er('A ||--o{ B : x', cfg('nodeSpacing: 11\nrankSpacing: 13')),
  er('A ||--o{ B : x', cfg('layout: dagre')),
  er('A ||--o{ B : x', cfg('layout: elk')),
  er('A ||--o{ B : x', cfg('layout: elk.stress')),
  er('A ||--o{ B : x', cfg('layout: nosuchlayout')),
  er('A ||--o{ B : x', cfg('layout: 5')),
  er('subgraph S\nA ||--o{ B : x\nend', cfg('look: handDrawn\nlayout: elk')),
  er('A ||--o{ B : x', '%%{init: {"er": {"nodeSpacing": 99}, "look": "classic"}}%%\n'),
  er('A ||--o{ B : x', '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#ff0000"}}}%%\n'),
  er('A ||--o{ B : x', cfg('securityLevel: loose')),
  er('A ||--o{ B : x', cfg('fontFamily: courier\nfontSize: 20')),
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Inputs that need a site config run in their own page.
const sited = [
  { site: { securityLevel: 'loose' }, text: er('subgraph S ["<b>bold</b> <script>x</script>"]\nA\nend\naccTitle: <i>t</i> <script>y</script>') },
  { site: { securityLevel: 'antiscript' }, text: er('subgraph S ["<b>bold</b> <script>x</script>"]\nA\nend\naccDescr: <script>y</script> d') },
  { site: { securityLevel: 'other' }, text: er('subgraph S ["<b>bold</b> x=y"]\nA\nend') },
  { site: { look: 'handDrawn', theme: 'forest' }, text: er('subgraph S\nA ||--o{ B : x\nend') },
  { site: { look: 'neo', theme: 'redux-dark-color' }, text: er('A ||--o{ B : x\nC') },
  { site: { htmlLabels: false }, text: er('A ||--o{ B : x') },
  { site: { er: { nodeSpacing: 55, rankSpacing: 66 } }, text: er('A ||--o{ B : x') },
  { site: { layout: 'elk' }, text: er('A ||--o{ B : x') },
  { site: { layout: 'dagre' }, text: er('A ||--o{ B : x', cfg('layout: elk')) },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: er('subgraph S ["<b>x</b> <i>y</i>"]\nA\nend') },
  { site: { er: { useMaxWidth: false } }, text: er('A') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'erdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh: !!fresh, site })) }));
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'er-db-page.js', '--args', argsFile, '--out', resultFile], {
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
