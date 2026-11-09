// Writes internal/diagrams/state/testdata/db.json.gz: the content of the
// state db of real mermaid (the root document, the states, the relations,
// the classes, the links, the direction, the title, the accessibility
// values) after the parse and again at the moment when the renderer draws,
// with the nodes and edges of getData and the config, for
//   - every fixture of the corpus that mermaid renders as stateDiagram (the
//     families state-diagram and state-diagram-v2),
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of stateDb.ts, dataFetcher.ts and
//     the grammar,
//   - calls of the functions of the db that the grammar does not reach.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs state-db-page.js
// with the esm-eval command of the browser oracle, one time. A corpus
// fixture runs in a new page, as in the reference renders.
//
// Usage (in tools/reference): node gen-state-db.mjs
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
const outFile = path.join(repoRoot, 'internal/diagrams/state/testdata/db.json.gz');
const stateTypes = new Set(['stateDiagram', 'state']);
const mutantsPerFixture = 5;
// The seed of Math.random in the oracle page (RANDOM_SEED of oracle.mjs).
const randomSeed = 0x6d6d6467;

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
// The fixtures of the corpus: every fixture of the state families, and the
// fixtures of other families that the reference rendered as a state diagram.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => {
    if (f.startsWith('state-diagram/') || f.startsWith('state-diagram-v2/')) {
      return true;
    }
    try {
      const meta = JSON.parse(fs.readFileSync(path.join(repoRoot, refDir, f.replace(/\.mmd$/, '.json')), 'utf8'));
      return stateTypes.has(meta.diagramType);
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
  '[*] --> A',
  'A --> [*]',
  '[*] --> [*]',
  '[*]',
  'A --> B',
  'A --> B: a label',
  'B --> C : spaced label',
  'A: a description',
  'A: another description',
  'B : spaced description',
  'state "A long name" as L1',
  'state "With: colon" as L2: more',
  'L1 --> L2',
  'state C1 {',
  'state "Composite name" as C2 {',
  '}',
  '--',
  'state f1 <<fork>>',
  'state j1 <<join>>',
  'state c1 <<choice>>',
  'state f2 [[fork]]',
  'state j2 [[join]]',
  'state c2 [[choice]]',
  'note right of A: A note',
  'note left of B : Another note',
  'note right of A\n  first line\n  second line\nend note',
  'note left of C1\n  on a composite\nend note',
  'note "floating" as N1',
  'classDef foo fill:#f00,color:white,font-weight:bold',
  'classDef bar stroke:#333,stroke-width:4px;',
  'classDef default fill:#eee',
  'class A foo',
  'class A,B bar',
  'class C1 foo',
  'A:::foo --> B:::bar',
  '[*]:::foo --> A',
  'C:::foo',
  'style A fill:#bbf,stroke:#f66,stroke-width:2px',
  'style A,B color:red;',
  'style C1 fill:#afa',
  'direction LR',
  'direction TB',
  'direction RL',
  'direction BT',
  'accTitle: The title',
  'accDescr: The description',
  'accDescr {\n  multi\n  line\n}',
  'click A "https://example.com" "A tooltip"',
  'click B href "https://example.com/b"',
  'hide empty description',
  'scale 350 width',
  '%% comment',
  '# another comment',
  'A --> B %% trailing comment',
  'A: #9829; and <b>bold</b>',
  'root --> A',
  'nl',
  'state nl',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its config can have a line that starts with "state".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*stateDiagram/.test(l));
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

const rnd = prng(0x73746174);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const sd = (body, front = '') => front + 'stateDiagram-v2\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const extras = [
  // Headers.
  'stateDiagram\n[*] --> A\n',
  'stateDiagram-v2\n[*] --> A\n',
  'stateDiagram-v2\n',
  'stateDiagram\n',
  'stateDiagram-v2',
  '\n\n   stateDiagram-v2   \n\n\n  A --> B\n\n',
  'stateDiagram-v2 A --> B',
  'stateDiagram-v2;A --> B',
  'stateDiagram-v3\nA --> B\n',
  'stateDiagram-v2\nstateDiagram-v2\nA --> B\n',
  'STATEDIAGRAM-V2\nA --> B\n',
  'stateDiagram-V2\nA --> B\n',
  // Start and end states.
  sd('[*] --> A\nA --> [*]'),
  sd('[*] --> [*]'),
  sd('[*]'),
  sd('[*] --> A\n[*] --> B\nA --> [*]\nB --> [*]'),
  sd('A --> [*]: done\n[*] --> A: begin'),
  sd('state C {\n  [*] --> A\n  A --> [*]\n}\n[*] --> C\nC --> [*]'),
  sd('state C {\n  state D {\n    [*] --> A\n    A --> [*]\n  }\n  [*] --> D\n  D --> [*]\n}'),
  sd('[*]:::foo --> A\nA --> [*]:::bar\nclassDef foo fill:red\nclassDef bar fill:blue'),
  sd('[*]: a description\n[*] --> A'),
  sd('root_start --> A\n[*] --> A'),
  sd('start1 --> A\n[*] --> A\nend2 --> A'),
  // Descriptions.
  sd('A'),
  sd('A\nB\nA'),
  sd('A: one'),
  sd('A: one\nA: two'),
  sd('A: one\nA: two\nA: three'),
  sd('A : spaced\nA :not spaced\nA:tight'),
  sd('A: A'),
  sd('A: A\nA: A'),
  sd('a: x'),
  sd('a\na: x\na: y'),
  sd('ab: a'),
  sd('A: one\nA --> B\nA: two'),
  sd('A --> B\nA: one\nB: one\nB: two'),
  sd('A: :leading colon\nB:: double'),
  sd('A: with: inner colon\nB: a::b'),
  sd('A: trailing ;semicolon\nB: x; y'),
  sd('state "A long description" as A'),
  sd('state "A long description" as A\nA --> B'),
  sd('state "A long description" as A\nA: second'),
  sd('state "A long description" as A\nA: second\nA: third'),
  sd('A: first\nstate "A long description" as A'),
  sd('state "desc" as A: extra'),
  sd('state "desc" as A: extra\nA: third'),
  sd('state "desc" as A:\nA --> B'),
  sd('state "desc" as A: b: c'),
  sd('state "  padded  " as   A  \nA --> B'),
  sd('state "" as A'),
  sd('state "desc" as A\nstate "other" as A'),
  sd('state "desc" as A %% comment\nA --> B'),
  sd('state "<b>bold</b> and <script>alert(1)</script>" as A'),
  sd('A: <b>bold</b> <i>it</i> <br/> line <br> two <br /> three'),
  sd('A: #9829; heart #amp; amp #35; hash'),
  sd('A: x = y <z> "q"'),
  sd('A: `code` **bold** _it_'),
  sd('A: 你好\nÜnï --> Привет: 😀 emoji\nÅ --> ß'),
  sd('A --> B: <a href="https://x.example" target="_blank">link</a>'),
  // Relations.
  sd('A --> B'),
  sd('A --> B\nA --> B\nB --> A'),
  sd('A --> A'),
  sd('A --> B: label'),
  sd('A --> B : spaced label \nB-->C:tight'),
  sd('A --> B: with: colon'),
  sd('A --> B: <b>html</b>'),
  sd('A-->B\nC  -->  D'),
  sd('A --> B --> C'),
  sd('A -> B'),
  sd('A --> '),
  sd('--> B'),
  sd('A -- B'),
  sd('A:::x --> B:::y: label\nclassDef x fill:red\nclassDef y fill:blue,color:white'),
  // Composite states.
  sd('state A {\n  B --> C\n}'),
  sd('state A {\n}'),
  sd('state A {}'),
  sd('state A {\n  B\n}\nA --> D'),
  sd('state A {\n  B --> C\n}\nstate A {\n  D --> E\n}'),
  sd('A --> B\nstate A {\n  C --> D\n}'),
  sd('state A {\n  B --> C\n}\nA: a description'),
  sd('A: a description\nstate A {\n  B --> C\n}'),
  sd('state "Composite description" as A {\n  B --> C\n}'),
  sd('state "Composite description" as A {\n  B --> C\n}\nA: extra'),
  sd('state "Composite description" as A  {\n  B --> C\n}\nA --> D'),
  sd('state A {\n  state B {\n    state C {\n      D --> E\n    }\n  }\n}\nA --> F\nB --> F\nC --> F'),
  sd('state A {\n  B --> C\n}\nstate D {\n  E --> F\n}\nstate G {\n  state H {\n    I\n  }\n}\nA --> D\nD --> G'),
  sd('state A {\n  A --> B\n}'),
  sd('state A {\n  B --> C\n}\nB --> E'),
  sd('state A {\n  direction LR\n  B --> C\n}'),
  sd('state A {\n  direction LR\n  direction BT\n  B --> C\n}\nstate D {\n  E\n  direction RL\n}'),
  sd('direction LR\nstate A {\n  direction TB\n  B --> C\n}'),
  sd('state A {\n  B: one\n  B: two\n  note right of B: n\n}'),
  sd('state A B {\n  C\n}'),
  sd('state A {\n  B'),
  sd('}'),
  sd('state A {\n  B\n}\n}'),
  sd('state root {\n  B --> C\n}\nroot --> D'),
  sd('root --> A\nB --> root'),
  sd('state "desc" as {\n  B\n}'),
  sd('state A{\n  B\n}'),
  sd('state A\n{\n  B\n}'),
  sd('state A'),
  sd('state A\nA --> B'),
  // Concurrency.
  sd('state A {\n  B --> C\n  --\n  D --> E\n}'),
  sd('state A {\n  B --> C\n  --\n  D --> E\n  --\n  F --> G\n}'),
  sd('state A {\n  B --> C\n  --\n}'),
  sd('state A {\n  --\n  B --> C\n}'),
  sd('state A {\n  --\n}'),
  sd('state A {\n  --\n  --\n  B\n}'),
  sd('state A {\n  [*] --> B\n  --\n  [*] --> C\n  C --> [*]\n}'),
  sd('state A {\n  B --> C\n  --\n  state D {\n    E --> F\n    --\n    G --> H\n  }\n}'),
  sd('state A {\n  B --> C\n  --\n  D --> E\n}\nstate F {\n  G --> H\n  --\n  I --> J\n}'),
  sd('state A {\n  B --> C\n  --\n  D --> E\n}\nclassDef foo fill:red\nclass A foo'),
  sd('state A {\n  B --> C\n  --\n  D --> E\n}\nstyle A fill:red'),
  sd('state A {\n  B --> C\n}\nstate D {\n  E --> F\n}\nclassDef foo fill:red\nclass A foo\nstate G {\n  H\n}'),
  sd('--\nA --> B'),
  sd('A --> B\n--\nC --> D'),
  sd('state A {\n  direction LR\n  B --> C\n  --\n  direction TB\n  D --> E\n}'),
  sd('state A {\n  note right of B: n\n  B --> C\n  --\n  D --> E\n}'),
  // Forks, joins and choices.
  sd('state f <<fork>>\n[*] --> f\nf --> A\nf --> B\nstate j <<join>>\nA --> j\nB --> j\nj --> [*]'),
  sd('state f [[fork]]\nstate j [[join]]\nf --> j'),
  sd('state c <<choice>>\n[*] --> A\nA --> c\nc --> B: if x\nc --> C: else'),
  sd('state c [[choice]]\nA --> c'),
  sd('state   spaced   <<fork>>\nA --> spaced'),
  sd('state f <<fork>>\nf: description'),
  sd('state f <<fork>>\nstate f <<join>>\nA --> f'),
  sd('A --> f\nstate f <<fork>>'),
  sd('state X {\n  state f <<fork>>\n  f --> A\n  state c <<choice>>\n  A --> c\n}'),
  sd('state f <<FORK>>\nA --> f'),
  sd('state "desc" as f <<fork>>'),
  sd('state </join></fork> <<fork>>'),
  sd('</join></fork> --> A\n</choice> --> A'),
  // Notes.
  sd('A --> B\nnote right of A: A note'),
  sd('A --> B\nnote left of A: A note'),
  sd('A --> B\nnote right of A\n  line one\n  line two\nend note'),
  sd('A --> B\nnote left of B\n  line one\n\n  line three\n  end note'),
  sd('note right of A: note first\nA --> B'),
  sd('note right of A: only a note'),
  sd('A\nnote right of A: one\nnote left of A: two'),
  sd('A\nnote right of A: one\nnote right of A: two'),
  sd('A: desc\nA: desc two\nnote right of A: n'),
  sd('state A {\n  B --> C\n}\nnote right of A: on a composite'),
  sd('state A {\n  B --> C\n  note right of B: inside\n}'),
  sd('state A {\n  B --> C\n  note left of B\n    inside\n    two\n  end note\n}'),
  sd('A\nnote right of A: <b>html</b> <script>x</script> a=b'),
  sd('A\nnote right of A : spaced : more'),
  sd('A\nnote right of A:tight'),
  sd('A\nnote right of   A  : spaces'),
  sd('A\nnote top of A: wrong'),
  sd('A\nnote right of A'),
  sd('A\nnote "floating note" as N1'),
  sd('A\nnote "floating note" as N1\nN1 --> A'),
  sd('A\nnote right of A: n\nclassDef foo fill:red\nclass A foo'),
  sd('[*] --> A\nnote right of [*]: on start'),
  sd('A----parent --> B\nA --> B\nnote right of A: n'),
  sd('A\nnote right of A: n\nA----note-0 --> B'),
  sd('A --> B: l\nnote right of B: n\nB --> C\nnote left of C: m\nC --> D'),
  sd('A\nNOTE RIGHT OF A: upper'),
  sd('A\nnote right of A: text %% not a comment\nB'),
  // Classes and styles.
  sd('A --> B\nclassDef foo fill:#f00\nclass A foo'),
  sd('A --> B\nclassDef foo fill:#f00,color:white,font-weight:bold,stroke-width:2px,stroke:yellow\nclass A foo'),
  sd('A --> B\nclassDef foo fill:#f00;\nclassDef bar stroke:#0f0;,color:#00f;;\nclass A foo\nclass B bar'),
  sd('A --> B\nclassDef foo color:red,background-color:blue,fill:green\nclass A foo'),
  sd('A --> B\nclassDef foo fill:red,color:blue\nclassDef foo stroke:green\nclass A foo'),
  sd('classDef foo fill:red\nA:::foo --> B:::foo\nC:::foo'),
  sd('classDef foo fill:red\nclassDef bar stroke:blue\nA:::foo\nA:::bar\nclass A foo'),
  sd('A --> B\nclass A,B foo\nclassDef foo fill:red'),
  sd('A --> B\nclass A, B foo\nclassDef foo fill:red'),
  sd('A --> B\nclass A nodef'),
  sd('class Missing foo\nclassDef foo fill:red'),
  sd('A --> B\nclass A foo bar\nclassDef foo fill:red'),
  sd('classDef default fill:#eee,stroke:#333\nA --> B'),
  sd('classDef DEFAULT fill:#eee\nA --> B'),
  sd('classDef foo\nA:::foo'),
  sd('classDef foo \nA:::foo'),
  sd('classDef foo-bar fill:red\nA:::foo-bar'),
  sd('classDef foo font-family:Arial\\, sans-serif\nA:::foo'),
  sd('classDef statediagram-state fill:red\nclassDef statediagram-cluster fill:blue\nA --> B\nstate C {\n  D\n}'),
  sd('A --> B\nstyle A fill:#bbf'),
  sd('A --> B\nstyle A fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5'),
  sd('A --> B\nstyle A,B fill:red;,stroke:blue;'),
  sd('A --> B\nstyle A, B fill:red'),
  sd('style Missing fill:red\nA --> B'),
  sd('A --> B\nstyle A fill:red\nstyle A stroke:blue'),
  sd('A --> B\nstyle A fill:red\nclassDef foo stroke:blue\nclass A foo'),
  sd('state C {\n  A --> B\n}\nstyle C fill:red\nstate D {\n  E\n}'),
  sd('state C {\n  A --> B\n}\nclassDef foo fill:red\nclass C foo\nstate D {\n  E\n}\nstate F {\n  G\n}'),
  sd('state C {\n  A:::foo --> B\n  classDef foo fill:red\n  class B foo\n  style A stroke:blue\n}'),
  sd('A --> B\nstyle A'),
  sd('A --> B\nclass A'),
  sd('A:::\nB'),
  sd('A:::foo:::bar'),
  sd('state f <<fork>>\nclassDef foo fill:red\nclass f foo'),
  // Direction.
  sd('direction LR\nA --> B'),
  sd('direction TB\nA --> B'),
  sd('direction RL\nA --> B'),
  sd('direction BT\nA --> B'),
  sd('A --> B\ndirection LR'),
  sd('direction LR\nA --> B\ndirection BT'),
  sd('direction lr\nA --> B'),
  sd('direction XX\nA --> B'),
  sd('direction\nA --> B'),
  sd('  direction   LR  \nA --> B'),
  sd('A --> B: direction LR'),
  // Links.
  sd('A --> B\nclick A "https://example.com" "A tooltip"\n'),
  sd('A --> B\nclick A href "https://example.com"\n'),
  sd('A --> B\nclick A "https://example.com" "A tooltip"\nclick B href "https://example.com/b"\nclick A href "https://example.com/again"\n'),
  sd('click Missing "https://example.com" "tip"\nA --> B\n'),
  sd('state C {\n  A --> B\n  click A "https://example.com" "tip"\n}\n'),
  sd('state C {\n  A --> B\n}\nclick C "https://example.com" "tip"\n'),
  sd('A --> B\nclick A "https://example.com"\n'),
  sd('A --> B\nclick A href\n'),
  sd('A --> B\nclick [*] href "https://example.com"\n'),
  sd('A --> B\nclick A:::foo href "https://example.com"\n'),
  sd('A --> B\nclick A "javascript:alert(1)" "<b>tip</b>"\n'),
  sd('A --> B\nclick A """quoted""" ""\n'),
  sd('A --> B\nCLICK A HREF "https://example.com"\n'),
  // Accessibility and title.
  sd('accTitle: The accessible title\naccDescr: The accessible description\nA --> B'),
  sd('accDescr {\n  A description\n  on several lines\n}\nA --> B'),
  sd('accTitle:    padded   \naccDescr:   padded too  \nA'),
  sd('accTitle: nl\nA'),
  sd('accDescr: nl\nA'),
  sd('accDescr {nl}\nA'),
  sd('accTitle: <b>bold</b> title\nA'),
  sd('state C {\n  accTitle: inside\n  A\n}'),
  sd('A --> B', '---\ntitle: The title\n---\n'),
  sd('A --> B', '---\ntitle: "<b>Title</b> with #35;"\n---\n'),
  // Other statements.
  sd('hide empty description\nA --> B'),
  sd('scale 350 width\nA --> B'),
  sd('scale 350\nA --> B'),
  sd('A --> B\nhide empty description\nscale 10 width'),
  sd('nl'),
  sd('state nl'),
  sd('nl --> A'),
  sd('state nl\nnl --> A'),
  // Comments and ids.
  sd('%% a comment\nA --> B %% trailing\n%% another'),
  sd('# a comment\nA --> B'),
  sd('A%%x --> B'),
  sd('A --> B%% c\nB --> C'),
  sd('state "desc" as A%%c\nA --> B'),
  sd('A_1 --> B.2\nC/3 --> D&4\n5 --> 6\n7a --> a7'),
  sd('state --> default\nclass --> style\nnote --> end'),
  sd('A;B'),
  sd('A --> B;\nC --> D;'),
  sd('A{B'),
  sd('A - B'),
  sd('A-B --> C'),
  sd('"quoted" --> B'),
  sd('A\tB\n'),
  sd('A\r\nB --> C\r\n'),
  // Config.
  sd('A --> B', cfg('look: handDrawn')),
  sd('A --> B', cfg('look: classic\ntheme: forest')),
  sd('A --> B', cfg('look: neo')),
  sd('A --> B', cfg('theme: dark')),
  sd('A --> B: <b>x</b>\nA: <i>d</i>', cfg('htmlLabels: false')),
  sd('A --> B: <b>x</b>\nA: <i>d</i>\nnote right of A: <u>n</u>', cfg('securityLevel: loose')),
  sd('A --> B: <b>x</b> a=b\nA: <i>d</i>', cfg('securityLevel: antiscript')),
  sd('A --> B', cfg('state:\n  nodeSpacing: 10\n  rankSpacing: 20\n  titleTopMargin: 5\n  useMaxWidth: false')),
  sd('A --> B', cfg('state:\n  nodeSpacing: 0\n  rankSpacing: 0')),
  sd('A --> B\nstate C {\n  D\n}\nnote right of A: n', cfg('state:\n  wrappingWidth: 100\n  minNodeWidth: 80')),
  sd('A --> B\nnote right of A: n', cfg('flowchart:\n  padding: 20')),
  sd('A --> B\nnote right of A: n', cfg('flowchart:\n  padding: "20"')),
  sd('A --> B', cfg('layout: dagre')),
  sd('A --> B', cfg('layout: elk')),
  sd('A --> B', cfg('layout: elk.stress')),
  sd('A --> B', cfg('layout: nosuchlayout')),
  sd('A --> B', cfg('layout: 5')),
  sd('A --> B', cfg('state: x')),
  sd('A --> B', cfg('state: 5')),
  sd('A --> B', cfg('state: true')),
  sd('A --> B', cfg('state: false')),
  sd('A --> B', cfg('state: null')),
  sd('A --> B', cfg('state: [1, 2]')),
  sd('A --> B', cfg('arrowMarkerAbsolute: true')),
  sd('A --> B', cfg('state:\n  arrowMarkerAbsolute: true')),
  sd('A --> B', '%%{init: {"state": {"nodeSpacing": 5}, "look": "classic"}}%%\n'),
  sd('A --> B', '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#ff0000"}}}%%\n'),
  sd('A --> B', cfg('look: 5')),
  sd('A --> B', cfg('look: nosuchlook')),
  sd('A --> B: l\nnote right of A: n', cfg('dompurifyConfig:\n  FORBID_TAGS: [b]')),
  sd('A: <b>x</b> <i>y</i>', cfg('dompurifyConfig:\n  FORBID_TAGS: [b]')),
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Inputs that need a site config run in their own page.
const sited = [
  { site: { securityLevel: 'loose' }, text: sd('A --> B: <b>bold</b> <script>x</script>\nA: <i>d</i> a=b\nnote right of A: <u>n</u>\nclick A "javascript:alert(1)" "<b>tip</b>"\n') },
  { site: { securityLevel: 'antiscript' }, text: sd('A --> B: <b>bold</b> <script>x</script>\nA: <i>d</i> a=b') },
  { site: { securityLevel: 'other' }, text: sd('A --> B: <b>bold</b> x=y\nA: a<br/>b') },
  { site: { htmlLabels: false }, text: sd('A --> B: <b>x</b>\nA: <i>d</i>\nnote right of A: <u>n</u>') },
  { site: { look: 'handDrawn', theme: 'forest' }, text: sd('state C {\n  A --> B\n}') },
  { site: { look: 'classic' }, text: sd('A --> B\nnote right of A: n') },
  { site: { state: { nodeSpacing: '10', rankSpacing: '20', wrappingWidth: '100', minNodeWidth: '50' } }, text: sd('A --> B') },
  { site: { flowchart: { padding: '20' } }, text: sd('A --> B\nnote right of A: n') },
  { site: { layout: 'dagre' }, text: sd('A --> B') },
  { site: { layout: 'elk' }, text: sd('A --> B') },
  { site: { arrowMarkerAbsolute: true }, text: sd('A --> B') },
  { site: { state: 'x' }, text: sd('A --> B') },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: sd('A: <b>x</b> <i>y</i>\nA --> B: <b>l</b>') },
  { site: { maxTextSize: 20 }, text: sd('A --> B\nB --> C\nC --> D') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Calls of the functions of the db that the grammar does not reach: the
// relations of two ids, the db with version 1, the styles and text styles of
// addState, and the helper functions. Each case is a list of calls on a new
// db, see state-db-page.js. A null argument stands for undefined.

const st = (id, extra = {}) => ({ stmt: 'state', id, type: 'default', description: '', ...extra });
const rel = (a, b, description) =>
  description === undefined
    ? { stmt: 'relation', state1: st(a), state2: st(b) }
    : { stmt: 'relation', state1: st(a), state2: st(b), description };
const divider = (n) => ({ stmt: 'state', id: `divider-id-${n}`, type: 'divider' });
const noteStmt = (id, position, text) => ({ stmt: 'state', id, note: { position, text } });
const apiScripts = [
  // Relations of two ids.
  [2, ['addRelation', '[*]', 'A', 'a title'], ['addRelation', 'A', '[*]'], ['addRelation', ' A ', ' B ', ''], ['addRelation', 'A', 'B', '<b>t</b> <script>x</script>'], ['getData']],
  [2, ['addRelation', '[*]', '[*]'], ['addRelation', '[*]', '[*]', 'again']],
  [2, ['addRelation', 'A', st('B')], ['addRelation', st('A'), 'B'], ['addRelation', st('A'), st('B')], ['addRelation', st('A'), st('B'), 'title']],
  [2, ['addRelation', st('[*]'), st('[*]'), 't'], ['addRelation', st(' [*] '), st('A', { type: 'fork' })], ['addRelation', st('A', { description: ':d' }), st('B', { description: ['x', ' :y '] })]],
  [2, ['addRelation', st('A', { classes: ['c1', ' c2 '] }), st('B', { classes: 'c3' })], ['addRelation', st('A', { styles: ['fill:red', ' stroke:blue '] }), st('B', { styles: 'fill:green' })], ['addRelation', st('A', { textStyles: ['color:red'] }), st('B', { textStyles: ' color:blue ' })]],
  [2, ['addRelation', st('A', { note: { position: 'left of', text: '<b>n</b> <script>x</script>' } }), st('B', { doc: [st('C')] })], ['getData']],
  // The db with version 1: the root document is extracted as it is.
  [1, ['setRootDoc', [rel('[*]', 'A'), rel('A', '[*]', 'done'), st('B', { description: 'desc' })]], ['getData']],
  [1, ['setRootDoc', [st('C', { doc: [rel('[*]', 'A'), divider(1), rel('B', '[*]')] }), rel('[*]', 'C')]], ['getData'], ['getDirection']],
  [1, ['setRootDoc', [{ stmt: 'dir', value: 'LR' }, rel('A', 'B')]], ['getDirection'], ['setDirection', 'BT'], ['getDirection'], ['getData']],
  // The db with version 2.
  [2, ['setRootDoc', [rel('[*]', 'A'), rel('A', '[*]', 'done'), st('B', { description: 'desc' })]], ['getData'], ['getData']],
  [2, ['setRootDoc', [st('C', { doc: [rel('[*]', 'A'), divider(1), rel('B', '[*]')] }), rel('[*]', 'C')]], ['getData']],
  [2, ['setRootDoc', [st('C', { doc: [rel('[*]', 'A'), divider(1), rel('B', '[*]'), divider(2)] })]], ['getData']],
  [2, ['setRootDoc', [st('C', { doc: [divider(1), rel('B', '[*]')] })]], ['getData']],
  [2, ['setRootDoc', [st('C', { doc: [divider(1)] })]], ['getData']],
  [2, ['setRootDoc', [rel('A', 'B'), divider(1), rel('C', 'D')]], ['getData']],
  [2, ['setRootDoc', [rel('A', 'B'), divider(1), rel('C', 'D'), divider(2)]], ['getData']],
  [2, ['setRootDoc', [st('C', { doc: [st('D', { doc: [st('E'), divider(1), st('F')] }), divider(2), st('G')] })]], ['getData']],
  [2, ['setRootDoc', [{ stmt: 'default', id: 'A', type: 'default', description: 'd' }, st('B')]], ['getData']],
  [2, ['setRootDoc', ['a line', st(' A '), 'nl', rel(' B ', ' C ')]], ['getData']],
  [2, ['setRootDoc', [st('A'), st('A', { description: 'one' }), st('A', { description: 'two' }), st('A', { description: ['three', 'four'] })]], ['getData']],
  [2, ['setRootDoc', [st('A', { description: ['one', 'two'] }), st('A', { description: ['three', 'four'] })]], ['getData']],
  [2, ['setRootDoc', [st('A', { description: 'A' }), st('B', { description: ['B'] }), st('c', { description: 'c' }), st('d', { description: 'x' })]], ['getData']],
  [2, ['setRootDoc', [st('A', { doc: [st('B')], description: 'one' }), st('A', { description: 'two' })]], ['getData']],
  [2, ['setRootDoc', [st('A', { description: 'one' }), st('A', { description: 'two' }), st('A', { doc: [st('B')] })]], ['getData']],
  [2, ['setRootDoc', [st('A'), noteStmt('A', 'right of', 'a <b>note</b>'), noteStmt('A', 'left of', 'second'), noteStmt('B', 'left of', 'first')]], ['getData']],
  [2, ['setRootDoc', [st('A'), noteStmt('A', 'right of', 'n'), st('A----parent'), st('A----note-0'), noteStmt('A----parent', 'left of', 'm')]], ['getData']],
  [2, ['setRootDoc', [st('root', { doc: [st('B')] }), rel('root', 'A'), st(''), st('</join></fork>'), st('</choice>')]], ['getData']],
  [2, ['setRootDoc', [st('A', { type: 'fork' }), st('B', { type: 'join' }), st('C', { type: 'choice' }), st('D', { type: 'divider' }), st('E', { type: 'other' }), st('F', { start: true }), st('G', { start: false }), { stmt: 'state', id: 'H' }]], ['getData']],
  [2, ['setRootDoc', [{ stmt: 'dir', value: 'LR' }, st('C', { doc: [{ stmt: 'dir', value: 'RL' }, st('A'), { stmt: 'dir', value: 'BT' }] })]], ['getDirection'], ['getData'], ['setDirection', 'TB'], ['getDirection'], ['getData']],
  [2, ['setDirection', 'LR'], ['getDirection'], ['setDirection', 'RL'], ['getDirection'], ['setRootDoc', [st('A')]], ['getDirection'], ['setDirection', 'BT'], ['getDirection'], ['getData']],
  // extract.
  [2, ['extract', [st('A'), rel('A', 'B', 'l'), { stmt: 'classDef', id: ' foo ', classes: 'fill:red,color:blue' }, { stmt: 'applyClass', id: ' A,B ', styleClass: 'foo' }, { stmt: 'style', id: ' A, C ', styleClass: 'fill:red;, stroke:blue ' }, { stmt: 'click', id: st('A'), url: '"u"', tooltip: '"t"' }, 'a line']], ['getData']],
  [2, ['extract', { doc: [st('A'), rel('A', 'B')] }], ['extract', { doc: [st('C')] }]],
  [2, ['extract', [{ stmt: 'classDef', id: 'foo' }, { stmt: 'classDef', id: 'foo', classes: '' }, { stmt: 'classDef', id: 'bar', classes: 'color:red;,background-color:blue,fill:green;;,x' }]]],
  [2, ['extract', [st('A', { doc: [st('B')], description: ['one', 'two'] })]]],
  [2, ['extract', [{ stmt: 'state' }]]],
  [2, ['extract', [{ stmt: 'relation', state1: st('A') }]]],
  [2, ['extract', [{ stmt: 'style', id: 'A' }]]],
  // addState.
  [2, ['addState', 'A'], ['addState', ' A '], ['addState', 'B', 'fork'], ['addState', 'B', 'join'], ['addState', 'C', ''], ['addState', 'C', 'choice'], ['getState', 'A'], ['getState', 'Z'], ['getStates']],
  [2, ['addState', 'A', null, [st('B')]], ['addState', 'A', null, [st('C')]], ['addState', 'D'], ['addState', 'D', null, [st('E')]]],
  [2, ['addState', 'A', null, null, 'one'], ['addState', 'A', null, null, [' two ', ':three', ' : four', '']], ['addState', 'A', null, null, ''], ['addState', 'A', null, null, []]],
  [2, ['addState', 'A', null, null, null, { position: 'right of', text: ' <i>n</i> ' }], ['addState', 'A', null, null, null, { text: 'second' }], ['addState', 'B', null, null, null, { position: 'left of' }]],
  [2, ['addState', 'A', null, null, null, null, 'c1'], ['addState', 'A', null, null, null, null, [' c2 ', 'c3,c4']], ['addState', 'A', null, null, null, null, null, 'fill:red'], ['addState', 'A', null, null, null, null, null, [' stroke:blue ', 'x']], ['addState', 'A', null, null, null, null, null, null, ' color:red '], ['addState', 'A', null, null, null, null, null, null, ['a', ' b ']]],
  [2, ['addState', 'A', null, null, [5]]],
  // Classes and styles.
  [2, ['addStyleClass', 'foo'], ['addStyleClass', 'foo', 'fill:red'], ['addStyleClass', 'foo', 'color:blue;,background-color:white, fill:green ;x;'], ['addStyleClass', 'bar', ''], ['addStyleClass', 'baz', 'colorfill:1,fillcolor:2,color:color,fill:fill:color'], ['getClasses']],
  [2, ['setCssClass', 'A', 'c1'], ['setCssClass', 'A,B', 'c2'], ['setCssClass', ' A , C ', 'c3'], ['setCssClass', '', 'c4'], ['getStates']],
  [2, ['setStyle', 'A', 'fill:red'], ['addState', 'A'], ['setStyle', 'A', 'fill:red'], ['setTextStyle', 'A', 'color:blue'], ['setTextStyle', 'B', 'color:blue'], ['getStates']],
  [2, ['addDescription', 'A', 'x'], ['addState', 'A'], ['addDescription', 'A', 'one'], ['addDescription', 'A', ':two'], ['addDescription', 'A', '::three'], ['addDescription', 'A', ' : four '], ['addDescription', 'A', ':'], ['addDescription', 'A', '<b>five</b> <script>x</script>'], ['getStates']],
  // Links.
  [2, ['addLink', 'A', '"u"', '"t"'], ['addLink', 'A', '"v"', ''], ['addLink', { id: 'B' }, 'w', 't'], ['getLinks'], ['clear', true], ['getLinks'], ['clear'], ['getLinks']],
  // Helper functions.
  [2, ['trimColon', ':  x '], ['trimColon', ' x '], ['trimColon', '::x'], ['trimColon', ''], ['trimColon', ' :x'], ['cleanupLabel', ':  x '], ['cleanupLabel', ' x '], ['cleanupLabel', '::x'], ['cleanupLabel', ':x'], ['cleanupLabel', ':'], ['cleanupLabel', '']],
  [2, ['getDividerId'], ['getDividerId'], ['clear'], ['getDividerId']],
  [2, ['startIdIfNeeded', '[*]'], ['startIdIfNeeded', 'A'], ['startIdIfNeeded'], ['endIdIfNeeded', '[*]'], ['endIdIfNeeded', 'A'], ['endIdIfNeeded'], ['startIdIfNeeded', '[*]'], ['clear', true], ['startIdIfNeeded', '[*]']],
  [2, ['startTypeIfNeeded', '[*]', 'fork'], ['startTypeIfNeeded', 'A', 'fork'], ['startTypeIfNeeded', 'A'], ['startTypeIfNeeded'], ['endTypeIfNeeded', '[*]', 'fork'], ['endTypeIfNeeded', 'A', 'fork'], ['endTypeIfNeeded', 'A'], ['endTypeIfNeeded']],
  [2, ['stateDomId'], ['stateDomId', 'A'], ['stateDomId', 'A', 3], ['stateDomId', 'A', 3, 'note'], ['stateDomId', 'A', 3, ''], ['stateDomId', '', 0, 'parent']],
  [2, ['getData'], ['getDirection'], ['getStates'], ['getRelations'], ['getClasses'], ['getLinks']],
];
apiScripts.forEach(([version, ...ops], i) => cases.push({ name: `api${i}`, text: 'stateDiagram-v2\n', api: { version, ops } }));

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'statedb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(
  argsFile,
  JSON.stringify({ seed: randomSeed, cases: cases.map(({ name, text, fresh, site, api }) => ({ name, text, fresh: !!fresh, site, api })) })
);
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'state-db-page.js', '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (result.cases.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${result.cases.length}`);
}
// Each result is a JSON text, so that the order of the properties is kept.
const records = result.cases.map((text) => JSON.parse(text));

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
const base = records.find((r) => r.config);
const baseFlat = flatten(base.config, [], new Map());
const golden = records.map((r, i) => {
  const c = cases[i];
  // The text of a corpus fixture is read from the corpus by the test.
  const rec = c.corpus ? { ...r } : { ...r, text: c.text };
  if (c.fresh) {
    rec.fresh = true;
  }
  if (c.site) {
    rec.site = c.site;
  }
  if (c.api) {
    rec.api = c.api;
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
const api = golden.filter((r) => r.apiResults).length;
console.log(`${outFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${dumped} with a db, ${errors} with an error, ${api} with calls of the db`);
