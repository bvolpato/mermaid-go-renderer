// Writes the golden files of internal/diagrams/block/testdata:
//
//   db.json.gz      the content of the block db of real mermaid after the
//                   parse (the block hierarchy, the flat block list, the
//                   columns, the edges, the classes, the config), the block
//                   sizes that calculateBlockSizes measured, and the block
//                   sizes and the bounds after layout()
//   render.json.gz  the SVG of mermaid.render for the cases that are not in
//                   the corpus (the corpus has its own reference directory)
//
// for
//   - every fixture of the corpus that mermaid renders as a block diagram,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of blockDB.ts, layout.ts,
//     renderHelpers.ts and the grammar.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify, and the sizes are measured), so the
// script runs block-db-page.js with the esm-eval command of the browser
// oracle, one time. A corpus fixture runs in a new page, as in the
// reference renders.
//
// Usage (in tools/reference): node gen-block-db.mjs
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
const outDir = path.join(repoRoot, 'internal/diagrams/block/testdata');
const mutantsPerFixture = 3;
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
// The fixtures of the corpus: every fixture of the block family, and the
// fixtures of other families that the reference rendered as a block diagram.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => {
    if (f.startsWith('block/')) {
      return true;
    }
    try {
      const meta = JSON.parse(fs.readFileSync(path.join(repoRoot, refDir, f.replace(/\.mmd$/, '.json')), 'utf8'));
      return meta.diagramType === 'block';
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
  'columns 3',
  'columns auto',
  'columns 1',
  'columns 2',
  'space',
  'space:2',
  'space:3',
  'A',
  'B["a label"]',
  'C("round")',
  'D(("circle"))',
  'E{"diamond"}',
  'F{{"hexagon"}}',
  'G(["stadium"])',
  'H[["subroutine"]]',
  'I[("cylinder")]',
  'J((("double")))',
  'K>"odd"]',
  'L[/"lean right"/]',
  'M[\\"lean left"\\]',
  'N[/"trapezoid"\\]',
  'O[\\"inverted"/]',
  'P<["arrow"]>(right)',
  'Q<["both"]>(x, y)',
  'R<[" "]>(down)',
  'A:2',
  'B["wide"]:3',
  'A --> B',
  'A --- B',
  'A -- "label" --> B',
  'A == "thick" ==> B',
  'A -. "dotted" .-> B',
  'A x--x B',
  'A o--o B',
  'A <--> B',
  'A ==> B',
  'A -.-> B',
  'A ~~~ B',
  'A --> B --> C',
  'block',
  'block:group1',
  'block:group2:2',
  'end',
  'classDef blue fill:#66f,stroke:#333,stroke-width:2px;',
  'classDef green fill:#6f6,color:#000',
  'class A blue',
  'class A,B green',
  'class A, B green',
  'style A fill:#f9f,stroke:#333',
  'style B color:red',
  'A["a #9829; b"]',
  'A["<b>bold</b> and a=b"]',
  'root',
  'undefined',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its text can have a line that starts with "block".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*block(-beta)?\s*$/.test(l));
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

const rnd = prng(0x626c6f63);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const bd = (body, front = '') => front + 'block\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const extras = [
  // Headers.
  'block\n  A\n',
  'block-beta\n  A\n',
  'block',
  'block\n',
  'block-beta',
  '\n\n   block-beta   \n\n\n  A\n\n',
  'block A B C',
  'block-beta A',
  'blockA',
  'block-betaA B',
  'block:\n  A\n',
  'block:A\n  B\nend\n',
  'BLOCK\n  A\n',
  'block-Beta\n  A\n',
  'block\nblock\n  A\nend\n',
  'block\nblock-beta\n  A\nend\n',
  'block\r\n  A\r\n  B\r\n',
  'block\n\tA\tB\n',
  // Node shapes.
  bd('A'),
  bd('A B C'),
  bd('A\nB\nC'),
  bd('A["square"]'),
  bd('A("round")'),
  bd('A(("circle"))'),
  bd('A((("double circle")))'),
  bd('A>"odd"]'),
  bd('A{"diamond"}'),
  bd('A{{"hexagon"}}'),
  bd('A(["stadium"])'),
  bd('A[["subroutine"]]'),
  bd('A[("cylinder")]'),
  bd('A[/"lean right"/]'),
  bd('A[\\"lean left"\\]'),
  bd('A[/"trapezoid"\\]'),
  bd('A[\\"inverted trapezoid"/]'),
  bd('A<["block arrow"]>(right)'),
  bd('A<["left"]>(left)\nB<["up"]>(up)\nC<["down"]>(down)\nD<["x"]>(x)\nE<["y"]>(y)\nF<["x y"]>(x, y)'),
  bd('A<["all"]>(right, left, up, down)'),
  bd('A<["rd"]>(right, down) B<["ru"]>(right, up) C<["ld"]>(left, down) D<["lu"]>(left, up)'),
  bd('A<["three"]>(right, left, up) B<["three"]>(right, left, down) C<["three"]>(right, up, down) D<["three"]>(left, up, down)'),
  bd('A<["twice"]>(right, right)'),
  bd('A<["spaces"]>(  right ,  left  )'),
  bd('A<["no comma"]>(right left)'),
  bd('A<["none"]>()'),
  bd('A<["bad"]>(diagonal)'),
  bd('A<[" "]>(down)'),
  bd('A<["wide"]>(right):3\nB C D'),
  bd('columns 3\nA<["wide"]>(right):3\nB C D'),
  bd('columns 3\nA<["wide"]>(x):2 B\nC D E'),
  // Shapes whose start and end do not match.
  bd('A("x"]'),
  bd('A["x")'),
  bd('A[("x"))'),
  bd('A(-"x"-)'),
  bd('A-)"x"(-'),
  bd('A))"x"(('),
  bd('A)"x"('),
  bd('A)))"x"((('),
  bd('A[|"x"]'),
  bd('A{"x"}}'),
  bd('A{{"x"}'),
  bd('A(["x"]]'),
  bd('A[["x"])'),
  bd('A["x"]>'),
  bd('A>"x")'),
  bd('A[/"x"]'),
  bd('A<["x"]'),
  bd('A<["x"]>'),
  bd('A<["x"]>(right'),
  // Labels.
  bd('A[""]'),
  bd('A[" "]'),
  bd('A[text]'),
  bd('A["unterminated]'),
  bd('A["`markdown **bold**`"]'),
  bd('A["two\nlines"]'),
  bd('A["a <br/> break"] B["a<br>break"] C["a <br /> break"]'),
  bd('A["<b>bold</b> <i>it</i> <u>u</u>"]'),
  bd('A["<script>alert(1)</script>after"]'),
  bd('A["a = b < c > d"]'),
  bd('A["#9829; heart #amp; amp #35; hash"]'),
  bd('A["#quot;quoted#quot;"]'),
  bd('A["你好"] Ünï["Привет"] C["😀 emoji"]'),
  bd('A["fa:fa-car car"]'),
  bd('A["a very long label that goes on and on and on and on and on and on and on and on and on and on"]'),
  bd('A["A"]\nA["B"]'),
  bd('A["A"]\nA'),
  bd('A\nA["B"]'),
  bd('A["x"]\nA("y")\nA'),
  bd('A("x")\nA["A"]'),
  bd('A("x")\nA'),
  bd('<a href="https://x.example" target="_blank">l</a>'),
  bd('A["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]'),
  // Ids.
  bd('a.b a_b a/b a|b a,b a;b a&b a*b a+b a!b'),
  bd('1 2 3'),
  bd('123["n"] 45'),
  bd('A-B'),
  bd('A=B'),
  bd('A:B'),
  bd('A<B'),
  bd('"quoted"'),
  bd('default'),
  bd('linkStyle'),
  bd('interpolate'),
  bd('end'),
  bd('endA'),
  bd('A end'),
  bd('columns'),
  bd('columnsx'),
  bd('spacex'),
  bd('spaceship enterprise'),
  bd('classy'),
  bd('stylex'),
  bd('classDefx'),
  bd('blocker'),
  bd('root'),
  bd('root["the root"]'),
  bd('root("r") A'),
  bd('undefined'),
  bd('undefined A --> B --> C'),
  bd('null NaN true false'),
  bd('__proto__ constructor toString'),
  bd('__proto__ --> constructor'),
  bd('😀 👍🏽 é'),
  // Widths.
  bd('A:1'),
  bd('A:2 B'),
  bd('columns 3\nA:2 B\nC:3\nD E F'),
  bd('columns 3\nA:4 B'),
  bd('A:0 B'),
  bd('A:10'),
  bd('A["label"]:2 B'),
  bd('A:2:3'),
  bd('A :2'),
  bd('A: 2'),
  bd('A:x'),
  bd('columns 2\nA:2\nB:2\nC D'),
  bd('columns 4\nA:3 B\nC D:3'),
  // Columns.
  bd('columns 1\nA B C'),
  bd('columns 2\nA B C'),
  bd('columns 3\nA B C D E F G'),
  bd('columns 10\nA B C'),
  bd('columns auto\nA B C'),
  bd('columns 0\nA B C'),
  bd('columns  2\nA B C'),
  bd('columns\n2\nA B C'),
  bd('columns\t3\nA B C'),
  bd('columns automatic\nA B C'),
  bd('columns autoA B C'),
  bd('columns -1\nA'),
  bd('columns 2.5\nA B C'),
  bd('columns 2\nA B C\ncolumns 3'),
  bd('columns 2\ncolumns 1\nA B C'),
  bd('A B C\ncolumns 1'),
  bd('columns 3'),
  bd('columns 007\nA B C D E F G H'),
  bd('columns 2\nblock\n  columns 3\n  A B C D\nend\nE F'),
  // Spaces.
  bd('space'),
  bd('A space B'),
  bd('A space:2 B'),
  bd('A space:0 B'),
  bd('space:3'),
  bd('space space'),
  bd('columns 3\nA space B\nspace:2 C\nspace D space'),
  bd('space:\nA'),
  bd('space :2\nA'),
  bd('space:2:3\nA'),
  bd('space:12 A'),
  bd('columns 2\nspace:3 A'),
  bd('block\n  space\nend\nA'),
  bd('space --> A'),
  bd('A --> space'),
  // Edges.
  bd('A --> B'),
  bd('A --- B'),
  bd('A --x B'),
  bd('A --o B'),
  bd('A <--> B'),
  bd('A x--x B'),
  bd('A o--o B'),
  bd('A <-- B'),
  bd('A ----> B'),
  bd('A ==> B'),
  bd('A === B'),
  bd('A <==> B'),
  bd('A ==x B'),
  bd('A ==o B'),
  bd('A x==x B'),
  bd('A ====> B'),
  bd('A -.-> B'),
  bd('A -.- B'),
  bd('A .-> B'),
  bd('A .- B'),
  bd('A -..-> B'),
  bd('A <-.-> B'),
  bd('A x-.-x B'),
  bd('A o.-o B'),
  bd('A ~~~ B'),
  bd('A ~~~~~ B'),
  bd('A ~~ B'),
  bd('A --> B\nA --> B\nB --> A'),
  bd('A --> A'),
  bd('A-->B'),
  bd('A  -->  B'),
  bd('A -->\n  B'),
  bd('A\n  --> B'),
  bd('A --> B --> C'),
  bd('A --> B --> C --> D'),
  bd('A --> B\nB --> C\nC --> A'),
  bd('A["a"] --> B(("b"))'),
  bd('A --> B["late label"]'),
  bd('A["first"]\nA --> B'),
  bd('A:2 --> B'),
  bd('A --> B:2'),
  bd('columns 3\nA space B\nA --> B'),
  bd('columns 1\nA\nspace\nB\nA --> B'),
  bd('A space:2 B\nB --> A'),
  bd('A -- "label" --> B'),
  bd('A -- "label" --- B'),
  bd('A -- "label" --x B'),
  bd('A == "label" ==> B'),
  bd('A -. "label" .-> B'),
  bd('A x-- "label" --x B'),
  bd('A <-- "label" --> B'),
  bd('A o== "label" ==o B'),
  bd('A -- "label" ==> B'),
  bd('A --"label"--> B'),
  bd('A -- "" --> B'),
  bd('A -- " " --> B'),
  bd('A -- label --> B'),
  bd('A -- "`md`" --> B'),
  bd('A -- "a <b>bold</b> = label" --> B'),
  bd('A -- "#9829; l" --> B'),
  bd('A -- "a long edge label that wraps because it is very long indeed, longer than the width" --> B'),
  bd('A -- "l" --> B -- "m" --> C'),
  bd('A space B\nA -- "one" --> B\nA -- "two" --> B'),
  bd('columns 3\nA space B\nspace:3\nC space D\nA --> D\nC --> B\nA --> C\nB --> D'),
  bd('columns 3\nA space B\nA --x B\nB --o A'),
  bd('A(("a")) space B{"b"}\nA --> B'),
  bd('A[("a")] space B{{"b"}}\nA --> B\nB --> A'),
  bd('A>"a"] space B[/"b"/]\nA <--> B'),
  bd('A<["a"]>(right) space B((("b")))\nA --> B'),
  bd('A --> E'),
  bd('E --> A\nA'),
  bd('A --> root'),
  bd('root --> A'),
  bd('A --> undefined'),
  bd('A -- B'),
  bd('A - B'),
  bd('A -> B'),
  bd('A > B'),
  bd('A --> '),
  bd('--> B'),
  bd('A -->'),
  // Composite blocks.
  bd('block\n  A\nend'),
  bd('block\n  A B\nend\nC'),
  bd('block:g1\n  A B\nend\nC'),
  bd('block:g1:2\n  A B\nend\nC D'),
  bd('block:g1["a label"]\n  A B\nend'),
  bd('block:g1("round label"):3\n  A\nend\nB C D'),
  bd('block:g1\n  columns 1\n  A B\nend\nblock:g2\n  columns 2\n  C D\nend'),
  bd('columns 1\nblock:g1\n  A\nend\nblock:g2\n  B C\nend\nblock:g3\n  D E F\nend'),
  bd('block:g1\n  block:g2\n    block:g3\n      A\n    end\n  end\nend'),
  bd('block\n  block\n    block\n      A\n    end\n    B\n  end\n  C\nend\nD'),
  bd('block:g1\n  A\nend\nblock:g1\n  B\nend'),
  bd('A\nblock:A\n  B\nend'),
  bd('block:A\n  B\nend\nA'),
  bd('block:g1\n  g1\nend'),
  bd('block:root\n  A\nend\nB'),
  bd('block end'),
  bd('block:g1 end'),
  bd('block:g1\nend'),
  bd('block\n  A'),
  bd('block:g1\n  A'),
  bd('A\nend'),
  bd('block\n  A\nend\nend'),
  bd('block:g1 A B end C'),
  bd('block A end'),
  bd('block:g1 --> B\n  C\nend'),
  bd('block:g1 --> B --> C\n  D\nend'),
  bd('block:g1\n  A --> B\nend\ng1 --> C'),
  bd('columns 3\nblock:g1\n  A\nend\nspace\nblock:g2\n  B\nend\ng1 --> g2\nA --> B'),
  bd('columns 3\nblock:g1:2\n  A B\nend\nC\nA --> C\nC --> g1'),
  bd('block:g1\n  space\nend\nblock:g2\n  space:2\n  A\nend'),
  bd('block:g1\n  columns 2\n  A B C D\nend\nE:2'),
  bd('columns 2\nblock:g1\n  A\nend\nB["a much much wider block than the group"]\nC D'),
  bd('columns 1\nA["a much much wider block than the group below it"]\nblock:g1\n  B C\nend'),
  bd('columns 1\nA["a much much wider block than the group below it"]\nblock:g1\n  columns 1\n  B C\nend'),
  bd('columns 1\nA["a much much wider block than the group below it"]\nblock:g1\n  columns 3\n  B C\nend'),
  bd('columns 1\nA["a much much wider block than the group below it"]\nblock:g1\n  columns 2\n  B C D\nend'),
  bd('columns 1\nA["wide wide wide wide wide wide wide"]\nblock:g1\n  block:g2\n    B\n  end\n  C\nend'),
  bd('block:g1\n  A["tall<br/>tall<br/>tall"] B\nend\nC'),
  bd('columns 2\nA["tall<br/>tall<br/>tall"] B\nC D["two<br/>lines"]\nE'),
  bd('columns 3\nA:2 B["tall<br/>tall"]\nC D:2'),
  // Classes and styles.
  bd('A B\nclassDef blue fill:#66f,stroke:#333,stroke-width:2px\nclass A blue'),
  bd('A B\nclassDef blue fill:#66f,stroke:#333,stroke-width:2px;\nclass A,B blue'),
  bd('A B\nclassDef blue fill:#66f\nclass A, B blue'),
  bd('A B\nclassDef blue fill:#66f\nclass A,  B,C blue'),
  bd('A B\nclassDef c1 fill:#f00\nclassDef c2 stroke:#0f0\nclass A c1\nclass A c2\nclass B c2'),
  bd('A\nclassDef c fill:#f00,color:white,font-weight:bold\nclass A c'),
  bd('A\nclassDef c color:red,background-color:blue,fill:green\nclass A c'),
  bd('A\nclassDef c fill:red;stroke:blue;\nclass A c'),
  bd('A\nclassDef c fill:red;,stroke:blue;;,color:#fff;\nclass A c'),
  bd('A\nclassDef c fill:red\nclassDef c stroke:blue\nclass A c'),
  bd('A\nclassDef c  fill:red , stroke:blue  \nclass A c'),
  bd('A\nclassDef c\nclass A c'),
  bd('A\nclassDef c \nB'),
  bd('A\nclassDef c\n  B\nclass A c'),
  bd('A\nclassDef default fill:#eee,stroke:#333'),
  bd('A\nclassDef DEFAULT fill:#eee'),
  bd('A\nclassDef a-b fill:red'),
  bd('A\nclassDef 1 fill:red\nclass A 1'),
  bd('A\nclassDef c font-family:Arial\\, sans-serif\nclass A c'),
  bd('A\nclassDef'),
  bd('A\nclass A nodef'),
  bd('A\nclass A'),
  bd('A\nclass A \nB'),
  bd('A\nclass A c d\nclassDef c fill:red'),
  bd('A\nclass Missing c\nclassDef c fill:red'),
  bd('class A c\nA B\nclassDef c fill:red'),
  bd('class A c\nA["label"] B\nclassDef c fill:red'),
  bd('class A c\nA("round") B\nclassDef c fill:red'),
  bd('A\nclass A,A c\nclassDef c fill:red'),
  bd('A\nclass root c\nclassDef c fill:red'),
  bd('A\nclass A-B c'),
  bd('A\nclass'),
  bd('block:g1\n  A\nend\nclassDef c fill:#f00\nclass g1 c'),
  bd('block:g1\n  A\n  classDef c fill:#f00\n  class A c\nend'),
  bd('A space B\nA --> B\nclassDef c fill:#f00,stroke:#00f\nclass A,B c'),
  bd('A B\nstyle A fill:#f9f'),
  bd('A B\nstyle A fill:#f9f,stroke:#333,stroke-width:4px'),
  bd('A B\nstyle A fill:#f9f;'),
  bd('A B\nstyle A color:red,fill:blue,text-align:left'),
  bd('A B\nstyle A fill:#f9f\nstyle A stroke:#333'),
  bd('A B\nstyle A,B fill:#f9f'),
  bd('A B\nstyle A, B fill:#f9f'),
  bd('A\nstyle Missing fill:red'),
  bd('style A fill:red\nA'),
  bd('A\nstyle A'),
  bd('A\nstyle A \nB'),
  bd('A\nstyle'),
  bd('A\nstyle root fill:red'),
  bd('block:g1\n  A\nend\nstyle g1 fill:#f9f,stroke:#333'),
  bd('A(("circle")) B{"diamond"} C(["stadium"])\nstyle A fill:#f9f\nstyle B stroke:#333,stroke-width:4px\nstyle C color:red'),
  bd('A B\nclassDef c fill:#f00\nclass A c\nstyle A stroke:#00f'),
  bd('A<["arrow"]>(right)\nstyle A fill:#969'),
  // Statements that the grammar does not have.
  bd('accTitle: The title\nA'),
  bd('accDescr: The description\nA'),
  bd('accDescr {\n  multi\n  line\n}\nA'),
  bd('title A title\nA'),
  bd('linkStyle 0 stroke:red\nA --> B'),
  bd('click A "https://example.com"\nA'),
  bd('direction LR\nA B'),
  bd('subgraph s\n  A\nend'),
  // Comments.
  bd('%% a comment\nA B %% trailing\n%% another'),
  bd('A %% B\nC'),
  // Title.
  bd('A B', '---\ntitle: The title\n---\n'),
  // Config.
  bd('A B\nA --> B', cfg('look: handDrawn')),
  bd('A B\nA --> B', cfg('look: classic')),
  bd('A space B\nA --> B', cfg('look: neo')),
  bd('A space B\nA -.-> B\nB ==> A', cfg('look: neo\ntheme: neo')),
  bd('block:g1\n  A\nend\nblock:g2\n  B\nend', cfg('look: neo\ntheme: redux-color')),
  bd('block:g1\n  A\nend\nblock:g2\n  B\nend', cfg('theme: redux-dark-color')),
  bd('A B\nA --> B', cfg('theme: dark')),
  bd('A B\nA --> B', cfg('theme: forest')),
  bd('A B\nA --> B', cfg('theme: neutral')),
  bd('A B\nA --> B', cfg('theme: base\nthemeVariables:\n  primaryColor: "#ff0000"')),
  bd('A["<b>x</b>"] space B\nA -- "<i>l</i>" --> B', cfg('htmlLabels: false')),
  bd('A["<b>x</b>"] space B\nA -- "<i>l</i>" --> B', cfg('flowchart:\n  htmlLabels: false')),
  bd('A["<b>x</b> a=b"] space B\nA -- "<i>l</i>" --> B', cfg('securityLevel: loose')),
  bd('A["<b>x</b> a=b"] space B\nA -- "<i>l</i>" --> B', cfg('securityLevel: antiscript')),
  bd('A B C', cfg('block:\n  padding: 20')),
  bd('A B C', cfg('block:\n  padding: 0')),
  bd('block:g1\n  A B\nend\nC', cfg('block:\n  padding: 3')),
  bd('A B C', cfg('block:\n  padding: "12"')),
  bd('A B C', cfg('block:\n  useMaxWidth: false')),
  bd('A B C', cfg('block: x')),
  bd('A B C', cfg('block: null')),
  bd('A space B\nA --> B', cfg('flowchart:\n  curve: linear')),
  bd('A space B\nA --> B', cfg('flowchart:\n  curve: stepAfter')),
  bd('columns 1\nA\nspace\nB\nA --> B', cfg('flowchart:\n  curve: rounded')),
  bd('A["a long label that should wrap at the wrapping width of the flowchart config"]', cfg('flowchart:\n  wrappingWidth: 100')),
  bd('A space B\nA --> B', cfg('layout: elk')),
  bd('A space B\nA --> B', cfg('layout: swimlane')),
  bd('A space B\nA --> B', cfg('arrowMarkerAbsolute: true')),
  bd('A space B\nA --> B', cfg('flowchart:\n  arrowMarkerAbsolute: true')),
  bd('A space B\nA --> B', '%%{init: {"block": {"padding": 16}, "look": "classic"}}%%\n'),
  bd('A space B\nA --> B', '%%{init: {"theme": "base", "themeVariables": {"primaryColor": "#ff0000"}}}%%\n'),
  bd('A B', cfg('look: 5')),
  bd('A B', cfg('look: nosuchlook')),
  bd('A["<b>x</b> <i>y</i>"]', cfg('dompurifyConfig:\n  FORBID_TAGS: [b]')),
  bd('A["text"] B', cfg('fontFamily: courier\nfontSize: 20')),
  bd('A["text"] B', cfg('themeVariables:\n  fontSize: 24px')),
  // Larger diagrams.
  bd('columns 5\n' + Array.from({ length: 23 }, (_, i) => 'n' + i).join(' ')),
  bd('columns 4\n' + Array.from({ length: 12 }, (_, i) => `n${i}["node ${i}"]`).join('\n') + '\n' + Array.from({ length: 11 }, (_, i) => `n${i} --> n${i + 1}`).join('\n')),
  bd('columns 3\nblock:a:3\n  columns 3\n  a1 a2 a3\nend\nblock:b:2\n  b1 b2\nend\nc\nd space e\na1 --> d\nb2 --> e\nc --> a'),
  bd('space:40 A'),
  bd('A:40 B'),
  // Numbers that are too large for a float.
  bd('A:' + '9'.repeat(400) + ' B'),
  bd('columns ' + '9'.repeat(400) + '\nA B'),
  bd('A:99999999999999999999 B'),
  bd('columns 99999999999999999999\nA B'),
  // A class statement with a space after the comma replaces a block of the
  // db: the block of the hierarchy and the block of the db are then two
  // objects.
  bd('A space B\nA --> B\nclassDef c fill:#f00\nclass A, B c'),
  bd('A B\nclassDef c fill:#f00\nclass A, root c'),
  bd('block:g1\n  A\nend\nB\nclassDef c fill:#f00\nclass B, g1 c'),
  bd('A B\nclass A, B c\nstyle B fill:#f00'),
  // Edges of diamond blocks: the intersect function of the shape fails.
  bd('A{"a"} space B\nA --> B'),
  bd('A space B{"b"}\nA --> B'),
  bd('A{"a"} B\nB --> A', cfg('layout: swimlane')),
  bd('A{"a"}\nA --> A', cfg('layout: swimlane')),
  bd('A{"a"} space B\nA -- "label" --> B'),
  // The same id in several places.
  bd('block:g1\n  A\nend\nblock:g2\n  A\nend'),
  bd('A\nblock:g1\n  A["inner"]\nend\nA --> g1'),
  bd('A B A B A'),
  bd('A --> B\nA --> B\nblock\n  A --> B\nend'),
];
// A hand-written input runs in a new page, like a corpus fixture, so that
// the test can compare its SVG with the result of a plain render.
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text, fresh: true }));

// Inputs that need a site config run in their own page.
const sited = [
  { site: { securityLevel: 'loose' }, text: bd('A["<b>bold</b> <script>x</script> a=b"] space B\nA -- "<i>l</i>" --> B') },
  { site: { securityLevel: 'antiscript' }, text: bd('A["<b>bold</b> <script>x</script> a=b"] space B\nA -- "<i>l</i>" --> B') },
  { site: { securityLevel: 'other' }, text: bd('A["<b>bold</b> x=y a<br/>b"] B') },
  { site: { htmlLabels: false }, text: bd('A["<b>x</b>"] space B(("round"))\nA -- "<i>l</i>" --> B') },
  { site: { look: 'handDrawn', theme: 'forest' }, text: bd('block:g1\n  A B\nend\nC') },
  { site: { look: 'classic' }, text: bd('A space B\nA --> B') },
  { site: { look: 'neo' }, text: bd('A space B\nA --> B\nB -.-> A') },
  { site: { block: { padding: 20 } }, text: bd('A B\nblock:g\n  C\nend') },
  { site: { block: { padding: '20' } }, text: bd('A B\nblock:g\n  C\nend') },
  { site: { block: { useMaxWidth: false } }, text: bd('A B') },
  { site: { flowchart: { curve: 'linear' } }, text: bd('A space B\nA --> B') },
  { site: { layout: 'elk' }, text: bd('A space B\nA --> B') },
  { site: { arrowMarkerAbsolute: true }, text: bd('A space B\nA --> B') },
  { site: { block: 'x' }, text: bd('A B') },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: bd('A["<b>x</b> <i>y</i>"] space B\nA -- "<b>l</b>" --> B') },
  { site: { maxTextSize: 20 }, text: bd('A B C D E F G H I J K L') },
  { site: { theme: 'redux-color' }, text: bd('block:g1\n  A\nend\nblock:g2\n  block:g3\n    B\n  end\nend') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'blockdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(
  argsFile,
  JSON.stringify({
    seed: randomSeed,
    cases: cases.map(({ name, text, fresh, site, corpus }) => ({ name, text, fresh: !!fresh, site, svg: !corpus })),
  })
);
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'block-db-page.js', '--args', argsFile, '--out', resultFile], {
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
const base = records.find((r) => r.parsed);
const baseFlat = flatten(base.parsed.config, [], new Map());
const svgs = [];
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
  if (rec.svg !== undefined) {
    svgs.push({ name: rec.name, svg: rec.svg });
    delete rec.svg;
  }
  if (r.parsed) {
    const flat = flatten(r.parsed.config, [], new Map());
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
    rec.parsed = { ...r.parsed };
    delete rec.parsed.config;
    rec.parsed.configDelta = { set, del };
  }
  return rec;
});
fs.mkdirSync(outDir, { recursive: true });
const dbFile = path.join(outDir, 'db.json.gz');
fs.writeFileSync(dbFile, zlib.gzipSync(JSON.stringify({ baseConfig: base.parsed.config, cases: golden }), { level: 9 }));
const renderFile = path.join(outDir, 'render.json.gz');
fs.writeFileSync(renderFile, zlib.gzipSync(JSON.stringify({ cases: svgs }), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
const parsed = golden.filter((r) => r.parsed).length;
const laidOut = golden.filter((r) => r.drawn).length;
console.log(
  `${dbFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${parsed} with a db, ${laidOut} with a layout, ${errors} with an error`
);
console.log(`${renderFile}: ${svgs.length} SVGs, ${fs.statSync(renderFile).size} bytes`);
