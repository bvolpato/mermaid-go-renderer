// Writes internal/diagrams/agentflow/testdata/db.json.gz: what a layout
// algorithm of real mermaid receives from the draw function of the
// agentflow renderer (the nodes and edges of getData, the connectors and
// the layout properties), the content of the agentflow db (the vertices,
// the edges, the containers, the classes, the tooltips, the direction, the
// title, the accessibility values, the click callbacks, the diagnostics,
// the element mappings, the semantic model), the config after the init
// hook and a checksum of the text of the style element, for
//   - every fixture of the corpus that mermaid renders as agentflow,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of agentflowDb.ts and of the
//     grammar.
// A case that mermaid rejects has the error message instead. The file also
// has api cases, which call the methods of the db directly for the branches
// that the grammar does not reach, and the results of destructLink,
// stripLineTrailingCommas, stripPrototypeKeys, normaliseNodeShapes and
// containerSlot for lists of arguments.
//
// The db needs a browser (DOMPurify), so the script runs
// agentflow-db-page.js with the esm-eval command of the browser oracle, one
// time. A corpus fixture runs in a new page, as in the reference renders.
//
// Usage (in tools/reference): node gen-agentflow-db.mjs
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
const outFile = path.join(repoRoot, 'internal/diagrams/agentflow/testdata/db.json.gz');
const mutantsPerFixture = 6;

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
// The fixtures of the corpus: every fixture of the agentflow family, and the
// fixtures of other families that the reference rendered as agentflow.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => {
    if (f.startsWith('agentflow/')) {
      return true;
    }
    try {
      const meta = JSON.parse(fs.readFileSync(path.join(repoRoot, refDir, f.replace(/\.mmd$/, '.json')), 'utf8'));
      return meta.diagramType === 'agentflow';
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
  'A --> B',
  'B --x C',
  'C -.- D',
  'D -..- E',
  'E ---> F',
  'A -- yes --> B',
  'A -- no --x B',
  'A -->|label| B',
  'A --x|"quoted"| B',
  'A e1@--> B',
  'e1@{ animate: true }',
  'e1@{ animation: fast, curve: linear, instruction: "go" }',
  'A & B --> C & D',
  'A[Square] --> B(Round) --> C{Diamond}',
  'N1([Stadium])',
  'N2[[Subroutine]]',
  'N3[(Database)]',
  'N4((Circle))',
  'N5>Flag]',
  'N6{{Hexagon}}',
  'N7[/Parallelogram/]',
  'N8[\\Alt\\]',
  'N9[/Trapezoid\\]',
  'N10[\\Trapezoid/]',
  'N11(((Double)))',
  'N12@{ shape: tool, label: "Meta" }',
  'N13@{ shape: decision }',
  'N14@{ shape: refdoc, value: "x" }',
  'N15["`**bold** _text_`"]',
  'N16["A quoted (text)"]',
  'N17@{ shape: action, params: "a :: String" }',
  'N18@{ shape: input }',
  'N19@{ shape: task,\n  label: "Multi" }',
  'N20@{\n  shape: tool,\n  connectorRef: "c.x",\n}',
  'flow f9["A title"]',
  'flow f8',
  'flow',
  'global',
  'connector c1["Conn"]',
  'connector c2',
  'c1@{ protocol: "http", endpoint: "https://x.example" }',
  'direction LR',
  'direction TB',
  'end',
  'classDef foo fill:#f9f,stroke:#333,stroke-width:4px',
  'classDef bar,baz color:#fff',
  'class A foo',
  'class A,B bar',
  'A:::foo --> B:::bar',
  'style A fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5',
  'linkStyle 0 stroke:#ff3,stroke-width:4px',
  'linkStyle default stroke:blue',
  'linkStyle 0,1 interpolate basis stroke:red',
  'linkStyle default interpolate linear',
  'click A callback "Tooltip"',
  'click B call callback("x", 2)',
  'click C href "https://example.com" "Tip" _blank',
  'click D "https://example.com/d"',
  'f9@{ view: "collapsed" }',
  'f8@{ view: collapsed, instruction: "x" }',
  'accTitle: The title',
  'accDescr: The description',
  'accDescr {\n  multi\n  line\n}',
  ';',
  '%% comment',
  'A["#35; hash #lt;b#gt;"]',
  'A --> B & C --> D',
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
  let start = lines.findIndex((l, i) => i >= from && /^\s*agentflow-beta/.test(l));
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

const rnd = prng(0x6167666c);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const af = (body, front = '') => front + 'agentflow-beta TB\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const extras = [
  // Headers and directions.
  'agentflow-beta TB\nA-->B\n',
  'agentflow-beta\nA-->B\n',
  'agentflow-beta LR;A-->B;B-->C;',
  'agentflow-beta RL\nA-->B\n',
  'agentflow-beta BT\nA-->B\n',
  'agentflow-beta TD\nA-->B\n',
  'agentflow-beta BR\nA-->B\n',
  'agentflow-beta >\nA-->B\n',
  'agentflow-beta <\nA-->B\n',
  'agentflow-beta ^\nA-->B\n',
  'agentflow-beta v\nA-->B\n',
  '\n\n   agentflow-beta TB   \n\n\nA-->B\n\n',
  'agentflow-beta TB;\nA-->B;\n',
  'agentflow-beta TB\n',
  'agentflow-beta TB',
  'agentflow-beta',
  'agentflow-beta\n',
  'agentflow-beta   \n   \n',
  'agentflow-beta TB\nA\n',
  'agentflow-beta TB A-->B\n',
  'agentflow-beta TB %% a comment after the header\nA-->B\n',
  '%% a comment before the header\n%% another\nagentflow-beta LR\nA-->B\n',
  '  %% indented comment\n\n  agentflow-beta\n  %% after\nA-->B\n',
  'agentflow-beta TB\nagentflow-beta LR\nA-->B\n',
  'agentflow-beta TB\nA --> agentflow-beta\n',
  'agentflow-beta XY\nA --> B\n',
  'agentflow-beta TB LR\nA --> B\n',
  'agentflow-betaTB\nA --> B\n',
  'agentflow-beta-x TB\nA --> B\n',
  // Vertex shapes of the grammar.
  af('A[Square]\nB(Round)\nC((Circle))\nD(((Double)))\nE([Stadium])\nF[[Subroutine]]\nG[(Cylinder)]\nH{Diamond}\nI{{Hexagon}}\nJ>Odd]\nK[/Trapezoid\\]\nL[\\InvTrapezoid/]\nM[/LeanRight/]\nN[\\LeanLeft\\]\nO(-Ellipse-)\nP[|borders:lt|Props]\nQ'),
  af('A["Quoted text"]\nB("with (parens)")\nC{"a ? b : c"}\nD["`**Markdown** _text_\nsecond line`"]\nE[["`md tool`"]]'),
  af('A[ padded text ]\nB["  padded quoted  "]\nC[""]\nD[" "]'),
  af('A["#35; hash"]\nB["#quot;quoted#quot;"]\nC["#9829; heart #amp; amp"]\nD[x #lt; y]'),
  af('A[Line one<br/>line two<br>three<br />four]\nB[<b>bold</b> <i>it</i> <script>alert(1)</script>]\nC["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]\nD[<img src=x onerror=alert(1)>]'),
  af('A[你好] --> B[Ünïcödé]\nC[😀 emoji] --> Привет\nÅ --> ß'),
  af('A-B --> C.D\nE_F --> G/H\n1 --> 2\n3a --> a3\ndefault --> x\nv --> w\nA-v --> B'),
  af('end1 --> B\nflows --> C\nstyles --> classy\nA --> endpoint\nglobals --> connectors1'),
  af('A[text with keywords: flow connector global end style class click]'),
  af('A[Text] --> A\nA --> A\nA[Changed text]\nA(Round now)'),
  af('A\nA[Later text]\nB[First]\nB'),
  af('A[50% done] --> B[100 % sure]\nC[a %% a comment in the text\n still the text]\nD(round %% comment\n)\nE{{hex %% comment\n}}'),
  af('A[/lean %% comment\n right/]\nB(-ell %% c\n ipse-)\nC[\\x % y/]'),
  af('A[|field:value|Props text]\nA[|other:thing|Again]\nB[|__proto__:x|Proto]\nB[|__proto__:y|Proto2]'),
  af('P[|a:b|x] --> Q[|c:d|"quoted"]'),
  // Links.
  af('A --> B\nA --x B\nA -.- B\nA -..- B\nA -...- B\nA ---> B\nA ----> B\nA ---x B\nA --------------> B\nA -...........- B'),
  af('A -- text --> B\nA -- text --x B\nA -- "quoted" --> B\nA -- "`md **text**`" --> B\nA -- text with spaces and - dash --> B\nA -- yes ---> B'),
  af('A -->|text| B\nA --x|text| B\nA -.-|text| B\nA -->|"quoted"| B\nA -->|"`md`"| B\nA -->| padded | B'),
  af('A --> B --> C --> D\nA --> B & C\nA & B --> C\nA & B --> C & D\nA --> B & C --> D & E\nA & B & C --> D'),
  af('A-->B;B-->C;C-->A'),
  af('A   -->   B\nC-->D\nE --   text   --> F'),
  af('A --> B\nA --> B\nA --> B\nB --> A\nA --x B'),
  af('A --- B'),
  af('A ==> B'),
  af('A -.-> B'),
  af('A --o B'),
  af('A <--> B'),
  af('A ~~~ B'),
  af('A o--o B'),
  af('A x--x B'),
  af('A -- text --- B'),
  af('A -. text .- B'),
  af('A == text ==> B'),
  af('A -->'),
  af('--> B'),
  af('A -> B'),
  af('A - B'),
  // Edge ids and edge metadata.
  af('A e1@--> B\ne1@{ animate: true }'),
  af('A e1@--> B\ne1@{ animation: fast }\nB e2@--> C\ne2@{ animation: slow, curve: linear }\nC e3@--x D\ne3@{ curve: stepBefore, instruction: "escalate" }'),
  af('A e1@--> B & C\nD & E e2@--> F\nA & B e3@--> C & D'),
  af('A e1@--> B\nB e1@--> C\nC e1@--> D'),
  af('A e1@-- text --> B\nA e2@-->|text| B\ne1@{ animate: false }\ne2@{ animate: null }'),
  af('A e1@--> B\nclassDef dashed stroke-dasharray: 5 5,stroke:red\nclass e1 dashed\ne1@{ animate: true, animation: slow, curve: natural, extra: 1 }'),
  af('A L_A_B_0@--> B\nA --> B\nL_A_B_0@{ animate: true }'),
  af('A --> B\nL_A_B_0@{ animate: true, instruction: "auto id" }'),
  af('A e1@--> B\ne1@{ shape: diamond, label: "not a node" }\ne1'),
  af('A e1@--> B\ne1@{\n  animate: true\n  curve: basis\n}'),
  af('A e1@--> B\ne1@{ instruction: "one" }\ne1@{ instruction: "two", other: [1, 2] }\ne1@{ }'),
  af('A e1@-.- B\ne1@{ animate: "true" }\nB e2@--> C\ne2@{ animate: 1, animation: 5, curve: 7 }\nC e3@--> D\ne3@{ animation: [a], curve: "" }'),
  af('A e1@--> B\nB e1@{ shape: diamond }\ne1 --> C'),
  af('A @--> B'),
  af('A a@b@--> B\na@b@{ animate: true }\nab@{ animate: false }'),
  // Node metadata: the shapes.
  af('A@{ shape: task }\nB@{ shape: tool }\nC@{ shape: input }\nD@{ shape: decision }\nE@{ shape: refdoc }\nF@{ shape: action }\nG@{ shape: round }'),
  af('A@{ shape: subroutine }\nB@{ shape: subprocess }\nC@{ shape: subproc }\nD@{ shape: framed-rectangle }\nE@{ shape: lean-right }\nF@{ shape: diamond }\nG@{ shape: lin-doc }\nH@{ shape: lined-document }\nI@{ shape: hexagon }\nJ@{ shape: hex }'),
  af('A@{ shape: doc }\nB@{ shape: stadium }\nC@{ shape: terminal }\nD@{ shape: circle }\nE@{ shape: inv-trapezoid }\nF@{ shape: double-circle }\nG@{ shape: lean-left }\nH@{ shape: in-out }\nI@{ shape: cylinder }\nJ@{ shape: odd }'),
  af('A@{ shape: tag-rect }\nB@{ shape: tagged-rectangle }\nC@{ shape: delay }\nD@{ shape: half-rounded-rectangle }\nE@{ shape: lin-rect }\nF@{ shape: lined-rectangle }\nG@{ shape: win-pane }\nH@{ shape: window-pane }\nI@{ shape: curv-trap }\nJ@{ shape: curved-trapezoid }'),
  af('A@{ shape: rounded }\nB@{ shape: rect }\nC@{ shape: text }\nD@{ shape: notch-rect }\nE@{ shape: fork }\nF@{ shape: cyl }\nG@{ shape: diam }\nH@{ shape: lean-r }\nI@{ shape: fr-rect }\nJ@{ shape: prepare }\nK@{ shape: question }\nL@{ shape: event }\nM@{ shape: sm-circ }\nN@{ shape: bolt }'),
  af('A@{ shape: trapezoid }'),
  af('A@{ shape: procs }'),
  af('A@{ shape: ellipse }'),
  af('A@{ shape: connector }'),
  af('A@{ shape: collapsedGroup }'),
  af('A@{ shape: typeDeclaration }'),
  af('A@{ shape: roundedRect }'),
  af('A@{ shape: squareRect }'),
  af('A@{ shape: Diamond }'),
  af('A@{ shape: lean_right }'),
  af('A@{ shape: inv_trapezoid }'),
  af('A@{ shape: nosuchshape }'),
  af('A@{ shape: 5 }'),
  af('A@{ shape: true }'),
  af('A@{ shape: [a, b] }'),
  af('A@{ shape: { a: 1 } }'),
  af('A@{ shape: "" , label: x }'),
  af('A@{ shape: 0 }\nB@{ shape: false }\nC@{ shape: null }'),
  af('A@{ shape: constructor }'),
  af('A@{ shape: tostring }'),
  af('A@{ shape: Task }'),
  af('A@{ shape: "tool" }\nA@{ shape: decision }\nA@{ label: "third" }'),
  af('A[[Tool by grammar]]@{ shape: task }\nB(Round)@{ shape: tool }\nC{D}@{ shape: action }'),
  // Node metadata: the labels.
  af('A@{ label: 2024 }\nB@{ label: true }\nC@{ label: 1.5 }\nD@{ label: "" }\nE@{ label: null }\nF@{ label: 0 }'),
  af('A@{ label: [a, b] }'),
  af('A@{ label: [] }'),
  af('A@{ label: { a: 1 } }'),
  af('A@{ label: { toString: 1 } }'),
  af('A@{ label: [a, { toString: 1 }] }'),
  af('A@{ label: [1, null, [2, 3], { b: 1 }] }'),
  af('A@{ label: "x", labelType: string }\nB@{ label: "x", labelType: text }\nC@{ label: "x", labelType: markdown }\nD@{ label: "x", labelType: html }\nE@{ label: "x", labelType: 5 }\nF[plain]@{ labelType: string }'),
  af('A@{ shape: tool, label: "<b>html</b> <script>x</script>" }'),
  af('A@{ label: "a <br/> b", shape: task }\nB@{ label: "\\"quoted\\"" }\nC@{ label: \'"single"\' }'),
  af('A@{ icon: "fa:user" }\nB@{ icon: "fa:user", form: circle }\nC@{ img: "https://example.com/a.png", w: 60, h: 60, pos: t, constraint: on }'),
  af('A@{ shape: tool, label: "Multi\n  line\n  label" }'),
  af('A@{\n  shape: tool\n  label: "Block yaml"\n}\nB@{\n  label: |\n    literal\n    block\n}'),
  af('A@{\n  just text\n}'),
  af('A@{\n  - a\n  - b\n}'),
  af('A@{\n  5\n}'),
  af('A@{\n  0\n}\nB@{\n  false\n}\nC@{\n  ~\n}\nD@{\n  ""\n}'),
  af('A@{\n  true\n}\nB@{\n  1.5\n}'),
  af('A@{\n\n}'),
  af('A@{\n}'),
  af('A@{ }'),
  af('A@{}'),
  af('A@{ shape: diamond, shape: circle }'),
  af('A@{ : }'),
  af('A@{ not yaml: [ }'),
  af('A@{ label: "a } b" }'),
  af('A@{ label: "a: b, c" , shape: task }'),
  af('A@{ shape: task } --> B@{ shape: tool }\nC@{ shape: decision } & D@{ shape: hex } --> E'),
  af('A[Text]@{ shape: decision } --> B'),
  af('A --> B@{ shape: decision }\nA@{ shape: tool } --> B'),
  af('A@{ shape: task, unknownKey: 1, another: [1, 2], nested: { a: { b: [c] } } }'),
  af('A@{ shape: tool, connectorRef: "x.y", params: "a :: String", returns: "B", description: "d", instruction: "i", value: 5 }'),
  af('A@{ __proto__: { x: 1 }, constructor: 2, prototype: 3, keep: { __proto__: 4, constructor: { a: 1 }, ok: 5 }, list: [{ prototype: 6, fine: 7 }] }'),
  af('A@{ 2: two, 1: one, b: bee, 0: zero, a: ay }\nA@{ c: sea, 1: uno }'),
  af('A@{ shape: tool }   \n\n\n   B'),
  af('A@{ shape: tool }\t \nB@{ shape: task }\u00a0\nC'),
  af('A["a } b"] --> B\nC{x}  \n\n D'),
  af('A@{ label: "caret ^ inside" }'),
  af('A@{ note: a^b }'),
  af("A@{ label: 'single quoted', shape: task }\nB@{ \"label\": \"quoted key\" }\nC@{ label: plain text with spaces }"),
  af('A@{ shape: task, label: "a # not a comment" }\nB@{ shape: task # a comment\n}'),
  af('A --> B\nA@{ shape: decision }\nB@{ label: "new label" }\nA@{ label: "again", shape: tool }'),
  // Multi-line metadata with trailing commas, and YAML errors.
  af('A@{\n  shape: tool,\n  label: "x",\n}'),
  af('A@{\n  shape: "tool",\n  label: "x",\n  connectorRef: "a.b"\n}'),
  af('A@{\n  label: "x", # a comment\n  shape: "task", \t\n  list: [1, 2,\n    3],\n  map: { a: 1,\n    b: 2 },\n}'),
  af('A@{\n  label: "has, comma",\n  other: \'it,\',\n  text: |\n    keep, this,\n    and this,\n  after: "x",\n}'),
  af('A@{\n  folded: >-\n    one,\n    two,\n\n  after: "y",\n  esc: "a \\" b, c",\n}'),
  af('A@{\n  shape: tool,\n  label: "x"\n}'),
  af('A@{\n  label: "x",\n  bad: [1, 2\n}'),
  af('A@{\n  label: "x",\n  bad: "unterminated,\n}'),
  af('A@{\n  a: 1\n   b: 2\n}'),
  af('A@{\n  a: "x" y\n}'),
  af('A@{\n\ta: 1\n}'),
  af('A@{\n  a: 1\n  a: 2\n}'),
  af('A@{ a: 1, a: 2 }'),
  af('A@{ a: "x" y }'),
  af('A@{ a: [1, 2 }'),
  af('A@{ a: b: c }'),
  af('A@{ a: &x 1, b: *x, c: *nope }'),
  af('A@{ a: !!str 5, b: !!int "5", c: !!float 1, d: !!bool true, e: !!null "" }'),
  af('A@{ a: 0x1F, b: 0o17, c: 0b101, d: 1e3, e: .5, f: -.inf, g: .nan, h: +7, i: 1_000, j: 012, k: ~, l: Null, m: TRUE, n: yes }'),
  af('B --> C\n  A@{ a: "x" y }'),
  af('B --> C\n  A@{\n    shape: tool\n    bad: [\n}'),
  af('A@{ shape: tool } --> B@{\n  label: "x"\n  bad: {\n}', '---\ntitle: With front matter\n---\n'),
  af('A@{ a: "x" y }', '---\ntitle: With front matter\nconfig:\n  theme: forest\n---\n'),
  af('%% comment one\n%% comment two\nA@{ a: "x" y }'),
  // Styles and classes.
  af('A --> B\nstyle A fill:#f9f,stroke:#333,stroke-width:4px\nstyle B fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5'),
  af('style A fill:#f9f\nA --> B\nstyle C color:red'),
  af('A --> B\nclassDef default fill:#f9f,stroke:#333,stroke-width:4px\nclassDef foo color:red,fill:blue\nclassDef bar,baz stroke:green\nclass A foo\nclass A,B bar\nclass B baz'),
  af('A:::foo --> B:::bar\nC[Text]:::foo\nclassDef foo fill:red\nclassDef bar fill:blue,color:white,background-color:black'),
  af('classDef foo font-family:Arial\\,sans-serif,fill:red\nA:::foo'),
  af('classDef foo fill:#f00;\nclassDef bar stroke:#0f0,color:#00f;\nA:::foo --> B:::bar'),
  af('classDef node fill:red\nclassDef cluster fill:blue\nA --> B'),
  af('class A nodef\nA --> B\nclass missing foo'),
  af('A --> B\nlinkStyle 0 stroke:#ff3,stroke-width:4px,color:red\nB --> C\nlinkStyle 1 fill:blue,stroke:green'),
  af('A --> B\nB --> C\nlinkStyle default stroke:blue,stroke-width:2px\nlinkStyle 1 stroke:red'),
  af('A --> B\nB --> C\nC --> D\nlinkStyle 0,1,2 stroke:red\nlinkStyle 1 interpolate linear\nlinkStyle default interpolate cardinal stroke:blue\nlinkStyle 2 interpolate step stroke:green'),
  af('linkStyle default interpolate basis\nA --> B'),
  af('A --> B\nlinkStyle 1 stroke:red'),
  af('A --> B\nlinkStyle 0,5 stroke:red'),
  af('linkStyle 0 stroke:red\nA --> B'),
  af('A --> B\nlinkStyle 0 stroke-width:2px'),
  af('A --> B\nlinkStyle 00 stroke:red'),
  af('A --> B\nlinkStyle 5 interpolate basis'),
  af('A --> B\nlinkStyle default interpolate cardinal\nB --> C\nC e1@--> D\ne1@{ curve: linear }'),
  af('flow f["F"]\nA --> B\nend\nclassDef foo fill:red\nclass f foo\nstyle f stroke:blue,fill:#eee'),
  af('flow f["F"]\nA --> B\nend\nf:::foo\nclassDef foo fill:red'),
  af('A[[Tool]]:::foo\nclassDef foo fill:red,color:blue\nB@{ shape: decision }\nclass B foo'),
  // Interaction.
  af('A --> B\nclick A callback\nclick B callback "Tooltip for B"'),
  af('A --> B\nclick A call callback()\nclick B call callback("arg", 5) "Tooltip"\nclick A call other(a, "b, c", d)'),
  af('A --> B --> C --> D\nclick A href "https://example.com"\nclick B href "https://example.com" "Tooltip"\nclick C href "https://example.com" _blank\nclick D href "https://example.com" "Tooltip" _self'),
  af('A --> B --> C --> D\nclick A "https://example.com"\nclick B "https://example.com" "Tooltip"\nclick C "https://example.com" _parent\nclick D "https://example.com" "Tooltip" _top'),
  af('A --> B\nclick A,B callback "Shared tooltip"\nclick A,B "https://example.com/shared"'),
  af('A --> B\nclick A href "javascript:alert(1)"\nclick B href "  "'),
  af('A --> B\nclick missing callback "tip"\nclick missing "https://example.com"'),
  af('A --> B\nclick A callback "<b>bold</b> tip <script>x</script>"'),
  af('A --> B\nclick A callback "<script>only</script>"'),
  af('flow S["S"]\nA --> B\nend\nclick S "https://example.com" "Flow tip"\nclick S callback'),
  af('A e1@--> B\nclick e1 callback "edge tip"'),
  af('connector c["C"]\nclick c "https://example.com" "Connector tip" _blank'),
  // Containers.
  af('flow S["Title of S"]\nA --> B\nend'),
  af('flow S\nA --> B\nend'),
  af('flow\nA --> B\nend'),
  af('flow\nA\nend\nflow\nB\nend\nflow named\nC\nend'),
  af('flow S["Title"]\nend'),
  af('flow S\n\nend'),
  af('flow S[Plain title]\nA\nend\nflow T["`md **title**`"]\nB\nend\nflow U["Quoted"]\nC\nend'),
  af('flow my flow id\nA\nend\nflow other id ["T"]\nB\nend'),
  af('flow "quoted id"\nA\nend\nflow "`md id`"\nB\nend'),
  af('flow 1\nA\nend\nflow 2\nB\nend\n1 --> 2'),
  af('flow S-1["a-b"]\nA\nend\nflow S&T\nB\nend\nflow S:T\nC\nend\nflow S*T\nD\nend\nflow end2\nE\nend'),
  af('flow Outer["O"]\nflow Inner1["I1"]\nA\nend\nflow Inner2["I2"]\nB\nend\nA --> B\nend\nflow Sibling["S"]\nC\nend\nOuter --> Sibling\nInner1 --> C'),
  af('flow S["S"]\ndirection LR\nA --> B\nend\nflow T["T"]\ndirection BT\nC --> D\nend\nflow U["U"]\ndirection TD\nE\nend\nflow V["V"]\n  direction RL\nF\nend'),
  af('flow S["S"]\ndirection LR\ndirection TB\nA\nend'),
  af('direction LR\nA --> B'),
  af('flow S["First"]\nA\nend\nflow S\nB\nend\nflow S["Second title"]\ndirection LR\nC\nA\nend'),
  af('flow S["S"]\nA\nflow S["Inner S"]\nB\nend\nend'),
  af('flow S["S"]\nA\nend\nflow T["T"]\nA\nB\nend'),
  af('flow S["S"]\nA\nA\nB\nB\nend'),
  af('flow S["S"]\nS --> A\nA --> S\nend'),
  af('A --> B\nflow A["A as flow"]\nC\nend'),
  af('flow S["S"]\nA & B --> C\nD@{ shape: decision }\nE e1@--> F\nend'),
  af('flow S["S"]\nA;B;C\nend'),
  af('flow S["T"]\n  A\n  end\n  flow U\n  B\n  end  \n'),
  af('flow S["S"]\n;\nA\n;;\nend'),
  af('flow S["S"]\naccTitle: inside a flow\nA\naccDescr: also inside\nend'),
  af('flow S["S"]\naccDescr {\n  inside\n}\nA\nend'),
  af('flow S["S"]\nhasOwnProperty --> B\nend'),
  af('flow S["S"]\nA --> hasOwnProperty\nend'),
  af('flow S["S"]\nhasOwnProperty\nend'),
  af('flow S["S"]\n__proto__ --> A\n__proto__\nA\nend'),
  af('flow S["S"]\nconstructor --> toString\nvalueOf\nconstructor\nend'),
  af('__proto__ --> constructor\nhasOwnProperty --> toString'),
  af('flow S["<b>html</b> <script>x</script> title"]\nA\nend'),
  af('flow S["S"]\n%% a comment in the flow\nA --> B %% trailing\nend %% after end'),
  af('flow S["S"]@{ instruction: "do it" }\nA\nend'),
  af('flow S["S"]@{ view: "collapsed" }\nA --> B\nend\nC --> A'),
  af('flow S@{ view: collapsed, instruction: "x" }\nA\nend'),
  af('flow S@{ instruction: "no title" }\nA\nend\nS@{ more: 1 }'),
  af('flow S["S"]@{\n  instruction: "multi",\n  view: "collapsed",\n}\nA\nend'),
  af('flow S["S"]@{ shape: tool }\nA\nend'),
  af('flow S["S"]@{ a: "x" y }\nA\nend'),
  af('flow S["S"]\nA\nend\nS@{ shape: nosuchshape, label: "ignored" }'),
  af('flow S["S"]\nA\nend\nS'),
  af('flow S["S"]\nA\nend\nS["Vertex with the id of a flow"]\nS:::cls\nclassDef cls fill:red'),
  af('flow S["S"]\nA\nend\nS@{ }\nS@{\n}'),
  af('A --> B\nflow S["S"]\nA\nend\nflow T["T"]\nB\nS\nend\nflow U["U"]\nT\nend'),
  af('flow S'),
  af('flow S["S"]\nA'),
  af('end'),
  af('A --> B\nend'),
  af('flow S["S"] A\nend'),
  af('flow S["S"]["T"]\nA\nend'),
  af('flowS\nA\nend'),
  af('flow --> B'),
  // Collapsed containers.
  af('flow S["S"]\nA --> B\nend\nS@{ view: collapsed }\nC --> A\nB --> D'),
  af('flow Outer["O"]\nflow Inner["I"]\nA --> B\nend\nC\nend\nInner@{ view: collapsed }\nD --> A\nB --> C\nC --> D'),
  af('flow Outer["O"]\nflow Inner["I"]\nA --> B\nend\nC\nend\nOuter@{ view: collapsed }\nD --> A\nB --> C\nC --> D\nInner --> D'),
  af('flow Outer["O"]\nflow Inner["I"]\nA --> B\nend\nC\nend\nOuter@{ view: collapsed }\nInner@{ view: collapsed }\nD --> A'),
  af('flow Outer["O"]\nflow Inner["I"]\nA --> B\nend\nC\nend\nInner@{ view: collapsed }\nOuter@{ view: collapsed }\nD --> A\nInner --> Outer'),
  af('flow S["S"]\nA --> A\nend\nS@{ view: collapsed }\nB --> B\nS --> S'),
  af('flow S["S"]\nA\nend\nS@{ view: expanded }\nflow T["T"]\nB\nend\nT@{ view: "collapsed" }\nT@{ view: expanded }'),
  af('flow S["S"]\nA\nend\nclassDef foo fill:red\nclass S foo\nS@{ view: collapsed }\nstyle S stroke:blue'),
  af('S@{ view: collapsed }\nflow S["S"]\nA\nend'),
  af('flow S["S"]\nA\nend\nS@{\n  view: collapsed\n}'),
  af('flow S["S"]\nend\nS@{ view: collapsed }\nA --> S'),
  af('flow S["S"]\nA\nend\nS@{ view: collapsed, containerType: custom, other: 1 }'),
  af('flow S["S"]\ndirection LR\nA --> B\nend\nS@{ view: collapsed }'),
  af('flow A["A"]\nx\nend\nflow B["B"]\ny\nend\nflow C["C"]\nz\nend\nB@{ view: collapsed }\nx --> y\ny --> z'),
  af('flow A["A"]\nx[[tool]]\nend\nA@{ view: collapsed }\nflow B["B"]\ny\nend\nB@{ view: collapsed }\nflow C["C"]\nflow D["D"]\nw\nend\nend\nD@{ view: collapsed }'),
  // Containment cycles.
  af('flow A["A"]\nx --> B\nend\nflow B["B"]\ny --> A\nend'),
  af('flow A["A"]\nB\nend\nflow B["B"]\nA\nend'),
  af('flow A["A"]\nB\nend\nflow B["B"]\nC\nend\nflow C["C"]\nA\nend'),
  af('flow A["A"]\nB\nend\nflow B["B"]\nC\nend\nflow C["C"]\nA\nend\nA@{ view: collapsed }'),
  af('flow A["A"]\nB\nend\nflow B["B"]\nA\nend\nB@{ view: collapsed }\nA@{ view: collapsed }\nA --> B'),
  af('flow A["A"]\nx --> B\nend\nflow B["B"]\ny --> A\nend\nflow A\nz\nend'),
  // The global block.
  af('global\npolicy["Policy"]@{ shape: refdoc }\nend\nflow f["F"]\nA -.- policy\nend'),
  af('flow f["F"]\nA -.- policy\nend\nglobal\npolicy["Policy"]@{ shape: refdoc }\nend\nflow g["G"]\nB -.- policy\npolicy\nend'),
  af('global\nA --> B\nC\nend\nflow f["F"]\nA\nB\nC\nD\nend'),
  af('global\nend'),
  af('global\n\n  %% only a comment\nend\nA'),
  af('global\ndirection LR\nA\n;\naccTitle: in global\nend\nflow f\nA\n;\nin global\nend'),
  af('global\nflow inner["Inner"]\nA\nend\nend\nflow outer["Outer"]\ninner\nB\nend'),
  af('flow outer["Outer"]\nglobal\nA\nend\nA --> B\nend'),
  af('global\nconnector c["C"]\nend\nflow f\nc\nX\nend'),
  af('global A\nend'),
  af('globalX --> B'),
  af('global\nA'),
  af('global;A;end;flow f;A;B;end'),
  // Connectors.
  af('connector github["GitHub API"]\ngithub@{ protocol: "http", endpoint: "https://api.github.com", token_required: true }\nA@{ shape: tool, connectorRef: "github.list" }'),
  af('connector c\nA --> c'),
  af('connector c["C"]@{ protocol: "http" }\nconnector d@{ protocol: "grpc", shape: tool }\nc --> d'),
  af('connector c["First"]\nconnector c["Second"]\nconnector c\nconnector c@{ a: 1 }\nconnector c["Third"]@{ b: 2 }'),
  af('connector c["C"]\nc["Vertex text"]\nc(Round)\nc@{ shape: tool, label: "meta" }'),
  af('c["Vertex first"]\nA --> c\nconnector c["Then connector"]\nc@{ x: 1 }'),
  af('connector c["<b>bold</b> <script>x</script>"]\nconnector d["#35; entity"]\nconnector e["`md`"]'),
  af('flow f["F"]\nconnector c["Inside"]\nA --> c\nend'),
  af('connector c["C"]\nflow f["F"]\nc\nA\nend\nstyle c fill:red\nclass c foo\nclassDef foo stroke:blue'),
  af('connector my conn id ["T"]\nconnector other id\nA'),
  af('connector c["C"]@{ a: "x" y }'),
  af('connector c["C"]@{\n  protocol: "http",\n  endpoint: "https://x.example",\n}'),
  af('connector\nA'),
  af('connector c["C"] A'),
  af('connector connectors["Reserved"]\nconnectors@{ a: 1 }'),
  af('connectors@{ a: 1 }\nA --> connectors\nconnectors'),
  af('flow connectors["Flow with the reserved id"]\nA\nend\nconnectors@{ view: collapsed }'),
  af('connector c["C"]\nconnector d["D"]\nc -.- d\nflow f\nc\nend\nf@{ view: collapsed }'),
  // Accessibility and title.
  af('accTitle: The accessible title\naccDescr: The accessible description\nA --> B'),
  af('accDescr {\n  A description\n  on several lines\n}\nA --> B'),
  af('accTitle:    padded   \naccDescr:   padded too  \nA'),
  af('A --> B', '---\ntitle: The title\n---\n'),
  af('A --> B', '---\ntitle: "<b>Title</b> with #35;"\n---\n'),
  // Config.
  af('A --> B', cfg('look: handDrawn')),
  af('A --> B', cfg('look: classic\ntheme: forest')),
  af('A --> B', cfg('theme: dark')),
  af('A --> B', cfg('theme: redux-dark-color')),
  af('A --> B', cfg('theme: neutral')),
  af('A --> B', cfg('theme: base')),
  af('A[<b>x</b>] --> B', cfg('htmlLabels: false')),
  af('A[<b>x</b>] --> B\nC@{ label: 5 }', cfg('flowchart:\n  htmlLabels: false')),
  af('A --> B', cfg('flowchart:\n  curve: linear\n  padding: 20\n  nodeSpacing: 10\n  rankSpacing: 20')),
  af('A --> B', cfg('flowchart:\n  padding: 0')),
  af('A --> B', cfg('agentflow:\n  wrappingWidth: 300\n  minNodeWidth: 10\n  nodeSpacing: 70\n  rankSpacing: 90\n  diagramPadding: 20\n  titleTopMargin: 5\n  useMaxWidth: false')),
  af('A --> B', cfg('agentflow:\n  nodeSpacing: 0\n  rankSpacing: 0\n  minNodeWidth: 0\n  wrappingWidth: 0')),
  af('A --> B', cfg('agentflow:\n  look: classic\n  theme: forest')),
  af('flow S["S"]\nA --> B\nend\nflow T["T"]\ndirection RL\nC\nend', cfg('flowchart:\n  inheritDir: true')),
  'agentflow-beta LR\nflow S["S"]\nA --> B\nend\n',
  cfg('flowchart:\n  inheritDir: true') + 'agentflow-beta LR\nflow S["S"]\nA --> B\nend\n',
  af('A --> B', cfg('layout: dagre')),
  af('A --> B', cfg('layout: elk')),
  af('A --> B', cfg('layout: elk.stress')),
  af('A --> B', cfg('layout: nosuchlayout')),
  af('A --> B', cfg('layout: 5')),
  af('A --> B', '%%{init: {"flowchart": {"curve": "step"}, "look": "classic"}}%%\n'),
  af('A --> B', '%%{init: {"arrowMarkerAbsolute": true}}%%\n'),
  af('A --> B', cfg('arrowMarkerAbsolute: true')),
  af('A --> B', cfg('flowchart:\n  arrowMarkerAbsolute: true')),
  af('A --> B', cfg('securityLevel: loose')),
  af('A --> B', cfg('maxEdges: 1')),
  // Comments and the positions of the source.
  af('A --> B %% trailing comment\n%% a comment line\nB --> C'),
  af('%% first\nA((Circle))\n%% second\n  %% indented\nB([Stadium])\nflow f["F"]\n%% in flow\nC[(Cyl)]\nend', '---\ntitle: Positions\n---\n'),
  af('A["line one\n%% this line is a comment in the source\nline two"] --> B'),
  af('A@{\n  shape: tool\n%% a comment line in the metadata\n  label: "x"\n}'),
  af('A["`md line one\n  %% comment line\nmd line two`"]'),
  af('A -- "text\n%% comment\nmore" --> B'),
  af('%%{init: {"theme": "forest"}}%%\n%% comment\nA((x))'),
  af('A((x)) %%{ not a comment }\nB'),
  af('A %%\nB %% \nC %%x'),
  af('A --> B\n%%\nB --> C'),
  // Quirks of the grammar and of the db.
  af('A:::foo & B:::bar --> C:::foo\nclassDef foo fill:red\nclassDef bar fill:blue'),
  af('A#35;B --> C#quot;D'),
  'agentflow-beta TB\r\nA --> B\r\nflow S["S"]\r\nC@{ shape: tool }\r\nend\r\n',
  'agentflow-beta TB\n\tA --> B\n\tflow S\n\t\tC@{ shape: decision }\t\n\tend\n',
  af('A{{"hex"}}   \nB{"diamond"}  \n  C'),
  af('A -->|"a | b"| B'),
  af('A["`line one\nline two`"] --> B["plain\nnewline"]'),
  af('A[" "] --> B["  x  "]'),
  af('click A callback\nA --> B'),
  af('A --> B\nclick A href "https://example.com/a b?c=d e#f"\nclick B href "HTTPS://EXAMPLE.COM/%7Efoo/../bar"'),
  af('A[&quot;x&quot;] --> B[&quot;]\nC[&quot;&quot;] -- &quot;y&quot; --> D\nE -->|&quot;z&quot;| F'),
  af('A[&lt;b&gt;x&lt;/b&gt; &amp; y] --> B[a &nbsp; b]'),
  af('A & B'),
  af('A &B'),
  af('A@{ shape: tool } & B@{ shape: task } & C'),
  af('A@{ shape: tool }& B'),
  af('A@{ shape: tool } @{ shape: task }'),
  af('A --> B @{ shape: decision }'),
  af('@{ shape: tool }'),
  af('A@{ shape: tool'),
  af('A@{ shape: "tool }'),
  af('A["unterminated'),
  af('A{{x}'),
  af('A((x)'),
  af('A [text'),
  af('A[text]]'),
  af('class A'),
  af('classDef'),
  af('style A'),
  af('click A'),
  af('linkStyle x stroke:red'),
  af('A;;;B;\n;\n;C'),
  af('   \n\n  A  \n\n   B   \n'),
  // More ids and aliases.
  af('flow   ["Flow with an empty id"]\nA\nend\nflow   ["Again"]\nB\nend'),
  af('connector "quoted id"["T"]\nconnector "`md id`"\nA'),
  af('connector f["C"]\nflow f["F"]\nA\nend\nf@{ x: 1 }\nf@{ view: collapsed }'),
  af('flow f["F"]\nA\nend\nconnector f["C"]\nf@{ x: 1 }\nA --> f'),
  af('A@{\n  a: &x [*x]\n}'),
  af('A@{ a: &x {b: *x} }'),
  af('A@{ a: &x [1, 2], b: *x, c: {d: *x} }'),
  af('flow a["A"]\nx\nend\na@{ view: collapsed }\nstyle x fill:red\nclass x cls\nclick x "https://example.com" "tip"\nx@{ shape: tool }\nx --> y'),
  af('A@{ label: "x" }@{ shape: tool }'),
  af('A[One]@{ shape: tool }[Two]'),
  af('A --> B@{ shape: tool } --> C@{ shape: decision }\nD --> E@{ shape: action } & F@{ shape: input }'),
  af('A e1@--> B@{ shape: tool }\ne1@{ instruction: "x" }'),
  af('A@{ shape: tool }\nA@{ shape: stadium }\nB((x))\nB@{ shape: task }\nC@{ shape: delay }\nC'),
  // Values that are objects: the lexer ends the shape data at the first "}",
  // so an object needs the block style.
  af('A@{\n  label:\n    a: 1\n}'),
  af('A@{\n  label:\n    toString: 1\n}'),
  af('A@{\n  label:\n    valueOf: 1\n}'),
  af('A@{\n  label:\n    - a\n    - toString: 1\n}'),
  af('A@{\n  label:\n    - 1\n    - null\n    - [2, 3]\n    - b: 1\n    - ~\n}'),
  af('A@{ label: [1, null, [2, 3], ~, true] }'),
  af('A@{\n  shape:\n    a: 1\n}'),
  af('A@{\n  shape:\n    - tool\n}'),
  af('A@{\n  shape: tool\n  labelType:\n    a: 1\n  label: x\n}'),
  af('A@{\n  __proto__:\n    x: 1\n  constructor: 2\n  prototype: 3\n  keep:\n    __proto__: 4\n    constructor:\n      a: 1\n    ok: 5\n  list:\n    - prototype: 6\n      fine: 7\n    - - __proto__: 8\n        deep: 9\n}'),
  af('A@{\n  a: &x\n    b: *x\n}'),
  af('A@{\n  a: &x\n    - 1\n  b: *x\n  c:\n    d: *x\n}'),
  af('flow f["F"]\nA\nend\nf@{\n  view: collapsed\n  nested:\n    a: 1\n  __proto__: 1\n}'),
  af('connector c["C"]\nc@{\n  auth:\n    type: token\n    constructor: x\n}\nA e1@--> B\ne1@{\n  instruction:\n    steps:\n      - one\n      - two\n}'),
  // The index of the containers.
  af('flow none["N"]\nA\nend\nflow outer["O"]\nnone\nB\nend\nflow last["L"]\nouter\nend'),
  af('flow a["A"]\nx\nend\nflow none\ny\nend'),
  // A YAML error on line 10: the line numbers of the excerpt have two widths.
  af('n1\nn2\nn3\nn4\nn5\nn6\nA@{\n  a: 1\n  b: [1, 2\n  c: 3\n}'),
  af('n1\nn2\nn3\nn4\nn5\nn6\nn7\nA@{\n  a: "x" y\n  c: 3\n}'),
  // Two YAML documents: the exception has no mark.
  af('A@{\na: 1\n---\nb: 2\n}'),
  af('A@{\na: 1,\n---\nb: 2,\n}'),
  af('A@{\n---\na: 1\n...\n}'),
  af('A@{\n%YAML 1.2\n---\na: 1\n}'),
  af('A@{\n%YAML 1.2\na: 1\n}'),
  af('A@{\n  a: !!str 5\n  b: !!int "7"\n  c: !!oops x\n}'),
  af('A@{\n  a: |\n    literal\n    text\n  b: >-\n    folded\n    text\n  shape: tool\n}'),
  af('A@{\n  label: |\n    line one\n    line two\n}'),
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Inputs that need a site config run in their own page.
const longComments = (n) => Array.from({ length: n }, (_, i) => `%% comment line number ${i} that makes the source longer\n`).join('');
const sited = [
  { site: { securityLevel: 'loose' }, text: af('A --> B --> C\nclick A callback "Tip"\nclick B call callback("x", "y, z", 3) "Tip B"\nclick C call noargs()\nclick A href "javascript:alert(1)"\nclick B "https://example.com" _blank') },
  { site: { securityLevel: 'loose' }, text: af('A[<b>bold</b> <script>x</script>] --> B["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]\nclick A callback "<b>tip</b>"\nC@{ label: 5 }\nD@{ label: [a, b] }') },
  { site: { securityLevel: 'antiscript' }, text: af('A[<b>bold</b> <script>x</script>] --> B\nclick A callback\nclick B href "javascript:alert(1)"\nC@{ label: true }') },
  { site: { securityLevel: 'other' }, text: af('A[<b>bold</b> x=y] --> B[a<br/>b]\nC@{ label: "<i>x</i> a=b" }') },
  { site: { securityLevel: 'other' }, text: af('A@{ label: 5 }') },
  { site: { securityLevel: 'other', htmlLabels: false }, text: af('A@{ label: 5 }\nB@{ label: [1, 2] }') },
  { site: { maxEdges: 3 }, text: af('A --> B --> C --> D') },
  { site: { maxEdges: 3 }, text: af('A --> B --> C --> D --> E') },
  { site: { maxEdges: 0 }, text: af('A --> B') },
  { site: { htmlLabels: false }, text: af('A[<b>x</b>] --> B["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]') },
  { site: { flowchart: { htmlLabels: false } }, text: af('A[<b>x</b>] -->|<i>y</i>| B') },
  { site: { look: 'handDrawn', theme: 'forest' }, text: af('flow S["S"]\nA --> B\nend') },
  { site: { look: '' }, text: af('flow S["S"]\nA --> B\nend') },
  { site: { agentflow: { look: 'classic', theme: 'default' } }, text: af('flow S["S"]\nA --> B\nend\nS@{ view: collapsed }') },
  { site: { flowchart: { inheritDir: true }, direction: 'RL' }, text: 'agentflow-beta\nflow S["S"]\nA --> B\nend\n' },
  { site: { flowchart: { padding: '20', curve: 'natural' } }, text: af('A --> B') },
  { site: { agentflow: { wrappingWidth: '200', minNodeWidth: '50', nodeSpacing: '70', rankSpacing: '80' } }, text: af('A --> B') },
  { site: { layout: 'dagre' }, text: af('A --> B') },
  { site: { layout: 'elk.mrtree' }, text: af('A --> B') },
  { site: { layout: '' }, text: af('A --> B') },
  { site: { arrowMarkerAbsolute: true }, text: af('A --> B') },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: af('A[<b>x</b> <i>y</i>] --> B\nC@{ label: "<b>z</b>" }') },
  { site: { securityLevel: 'loose' }, text: af('A --> B\nclick A call callback(\"quoted, arg\", plain , \" spaced \")\nclick B call callback(\")\nclick A call a.b.c()\nclick missing call cb()') },
  { site: { securityLevel: 'loose' }, text: af('A@{ shape: decision, label: "<b>bold</b>" } --> B\nclick A,B callback "tip"\nclick A,B call two(1, 2)\nconnector c["C"]\nclick c call cb()') },
  { site: { theme: 'neutral', themeVariables: { primaryColor: '#ff0000' } }, text: af('A --> B\nclassDef foo fill:#00f\nclass A foo') },
  { site: { theme: 'redux-color', themeVariables: { borderColorArray: ['#111111', '#222222', '#333333'] } }, text: af('flow a["A"]\nx[[t]]\nend\nflow b["B"]\ny\nend\nflow c["C"]\nz{d}\nend\nflow d["D"]\nw\nend\nc@{ view: collapsed }') },
  { site: { theme: 'redux-color', themeVariables: { borderColorArray: [] } }, text: af('flow a["A"]\nx\nend\nflow b["B"]\ny\nend') },
  { site: { theme: 'redux-color', themeVariables: { borderColorArray: 'red' } }, text: af('flow a["A"]\nx\nend\nflow b["B"]\ny\nend') },
  { site: { theme: 'redux-dark-color' }, text: af(Array.from({ length: 9 }, (_, i) => `flow f${i}["F${i}"]\nn${i}\nend`).join('\n')) },
  // The text with its comments is longer than maxTextSize: the parser gets the text without them.
  { site: { maxTextSize: 400 }, text: '---\ntitle: Long comments\n---\n' + af(longComments(10) + 'A((Circle))\n%% c\nB([Stadium])') },
  { site: { maxTextSize: 2000 }, text: '---\ntitle: Long comments\n---\n' + af(longComments(10) + 'A((Circle))\n%% c\nB([Stadium])') },
  { site: { maxTextSize: 30 }, text: af('A((Circle)) --> B --> C --> D') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Api cases: the methods of the db, called directly.

const U = { $u: 1 };
const t = (text, type = 'text') => ({ text, type });
const loc = (first_line, first_column, last_line, last_column) => ({ first_line, first_column, last_line, last_column });
const api = [
  {
    name: 'gen-1',
    ops: [
      ['setGen', 'gen-1'],
      ['addVertex', 'A', t('Text A'), 'square'],
      ['addVertex', 'B', t('Text B'), 'round'],
      ['setDiagramId', 'svg1'],
      ['setTooltip', 'A,B,missing', 'tip'],
      ['addSubGraph', t('S'), [['A', 'B'], 'missing'], t('Title'), 'flow'],
      ['lookUpDomId', 'A'],
      ['lookUpDomId', 'missing'],
      ['setDiagramId', ''],
      ['lookUpDomId', 'A'],
      ['lookUpDomId', 'missing'],
      ['setDiagramId', 'svg1'],
      ['getTooltip', 'A'],
      ['getTooltip', 'svg1-agentflow-A-0'],
      ['setGen', ''],
      ['setTooltip', 'A', 'second'],
      ['getData'],
    ],
  },
  {
    name: 'addVertex-arguments',
    ops: [
      ['addVertex', '', t('ignored'), 'square'],
      ['addVertex', '   ', t('ignored'), 'square'],
      ['addVertex', 'A', t('  "quoted"  '), 'square', ['fill:red'], ['c1', 'c2'], 'LR', { a: 1 }],
      ['addVertex', 'A', U, U, ['stroke:blue'], ['c3'], 'TB', { b: 2, __proto__x: 3 }],
      ['addVertex', 'A', U, U, null, null],
      ['addVertex', 'B', t('"'), 'round', U, U, U, U, 'shape: tool, label: "From metadata"'],
      ['getData'],
      ['addVertex', 'C', U, U, U, U, U, U, 'shape: Bad'],
    ],
  },
  {
    name: 'addVertex-shape-error-with-location',
    ops: [
      ['setFrontmatterLineOffset', 4],
      ['addVertex', 'C', U, U, U, U, U, U, 'shape: nosuchshape', loc(3, 5, 3, 25)],
    ],
  },
  {
    name: 'addVertex-yaml-error-without-location',
    ops: [['addVertex', 'C', U, U, U, U, U, U, 'a: "x" y']],
  },
  {
    name: 'addVertex-yaml-error-with-location',
    ops: [
      ['setFrontmatterLineOffset', 2],
      ['addVertex', 'C', U, U, U, U, U, U, '\n  a: 1\n  b: [1, 2\n  c: 3', loc(7, 3, 11, 1)],
    ],
  },
  {
    name: 'addVertex-yaml-error-first-line',
    ops: [['addVertex', 'C', U, U, U, U, U, U, ' a: "x" y\n  b: 1', loc(12, 10, 13, 6)]],
  },
  {
    name: 'links',
    ops: [
      ['addSingleLink', 'A', 'B', { type: 'arrow_point', stroke: 'normal', length: 1, edgeSemantic: 'sequence' }],
      ['addSingleLink', 'A', 'B', { type: 'arrow_point', stroke: 'normal', length: 25, text: t('  "label"  ', 'string') }, 'myId'],
      ['addSingleLink', 'A', 'B', { type: 'arrow_cross', stroke: 'normal', length: 2, text: t('md', 'markdown') }, 'myId'],
      ['addSingleLink', 'A', 'B', { type: 'INVALID', stroke: 'INVALID' }],
      ['addSingleLink', 'A', 'C', { type: 'arrow_open', stroke: 'dotted', length: 1, text: t('x', 'other') }],
      ['addLink', ['A', 'B'], ['C', 'D'], { type: 'arrow_point', stroke: 'normal', length: 1, id: 'grp@' }],
      ['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'normal', length: 1 }],
      ['addLink', ['A'], ['B'], 'not an object'],
      ['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'invisible', length: 1, id: 5 }],
      ['addLink', ['A'], ['E'], { type: 'double_arrow_point', stroke: 'thick', length: 1, id: 'a@b@' }],
      ['addLink', ['A'], ['F'], { type: 'arrow_circle', stroke: 'normal', length: 1 }],
      ['addLink', ['A'], ['G'], { type: 'double_arrow_cross', stroke: 'normal', length: 1 }],
      ['addLink', ['A'], ['H'], { type: 'arrow_hierarchy', stroke: 'normal', length: 1 }],
      ['addLink', ['A'], ['I'], { stroke: 'normal', length: 1 }],
      ['updateLink', [0, 2], ['stroke:red']],
      ['updateLink', ['default'], ['stroke:blue', 'fill:green']],
      ['updateLinkInterpolate', [1], 'basis'],
      ['updateLinkInterpolate', ['default', 3], 'linear'],
      ['setClass', 'myId,grp,A,nothing', 'cls'],
      ['addVertex', 'myId', U, U, U, U, U, U, 'animate: true, curve: step'],
      ['getData'],
    ],
  },
  { name: 'updateLink-out-of-bounds', ops: [['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'normal', length: 1 }], ['updateLink', [0, 7], ['stroke:red']]] },
  { name: 'updateLink-empty', ops: [['updateLink', [0], ['stroke:red']]] },
  { name: 'updateLink-negative', ops: [['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'normal', length: 1 }], ['updateLink', [-1], ['stroke:red']]] },
  { name: 'updateLink-empty-style', ops: [['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'normal', length: 1 }], ['updateLink', [0], []], ['updateLink', ['0'], ['fill:red']], ['getData']] },
  { name: 'updateLinkInterpolate-out-of-bounds', ops: [['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'normal', length: 1 }], ['updateLinkInterpolate', [1], 'basis']] },
  { name: 'updateLinkInterpolate-string-position', ops: [['addLink', ['A'], ['B'], { type: 'arrow_point', stroke: 'normal', length: 1 }], ['updateLinkInterpolate', ['0'], 'basis'], ['updateLinkInterpolate', ['4'], 'basis']] },
  {
    name: 'subgraphs',
    ops: [
      ['addSubGraphSame', t('one two'), [['A']], 'flow'],
      ['addSubGraphSame', t('single'), [['B']], U],
      ['addSubGraph', U, [['C'], { stmt: 'dir', value: 'LR' }, ' ', ''], U, U],
      ['addSubGraph', t(' padded '), [['D', 'D'], 'D', ['E']], t(' <b>Title</b> ', 'string'), 'flow'],
      ['addSubGraph', t('padded'), [['F']], t('', 'text'), 'flow'],
      ['addSubGraph', t('padded'), [['G'], { stmt: 'dir', value: 'BT' }], t('New title', 'markdown'), 'flow'],
      ['addGlobal', [['A', 'G'], 'x', { stmt: 'dir', value: 'TB' }, ' ', 5]],
      ['exists', [{ nodes: ['a', 'b'] }, { nodes: [] }], 'b'],
      ['exists', [{ nodes: ['a', 'b'] }], 'c'],
      ['makeUniq', { nodes: ['a', 'b', 'c'] }, [{ nodes: ['b'] }]],
      ['getDepthFirstPos', 0],
      ['indexNodes'],
      ['getDepthFirstPos', 0],
      ['getData'],
    ],
  },
  {
    name: 'connectors',
    ops: [
      ['addConnector', U, U],
      ['addConnector', t('  '), t('x')],
      ['addConnector', t(' c1 '), U],
      ['addConnector', t('c1'), t('  <b>Title</b>  ')],
      ['addConnector', t('c1'), t('')],
      ['addConnector', t('c2'), t('Two')],
      ['addVertex', 'c2', t('Vertex text'), 'subroutine'],
      ['isToolDefinition', 'c2'],
      ['isToolDefinition', 'c1'],
      ['addVertex', 'c1', U, U, U, U, U, U, 'protocol: http'],
      ['getData'],
    ],
  },
  {
    name: 'diagnostics',
    ops: [
      ['addVertexMapping', 'n1', U, U, loc(2, 2, 2, 4)],
      ['addVertexMapping', 'n2', t('x'), 'square', loc(3, 2, 3, 9)],
      ['addEdgeMapping', ['n1'], ['n2', '', 'n3'], {}, loc(4, 0, 4, 12)],
      ['addEdgeMapping', ['n1'], [], {}, loc(5, 0, 5, 3)],
      ['addEdgeMapping', ['n1'], 'not a list', {}, loc(6, 0, 6, 3)],
      ['addEdgeMapping', ['n1'], [{ id: 'obj' }, {}, null, 'str'], {}, U],
      ['emitWarning', 'SHAPE_UNSUPPORTED', 'first', { nodeId: 'n1' }],
      ['emitWarning', 'SHAPE_UNSUPPORTED', 'first', { nodeId: 'n1' }],
      ['emitError', 'SHAPE_UNSUPPORTED', 'first', { nodeId: 'n1' }],
      ['emitWarning', 'CONTAINMENT_VIOLATION', 'edge', { edgeId: 'n2>n3' }],
      ['emitWarning', 'CONTAINMENT_VIOLATION', 'both', { nodeId: 'n2', edgeId: 'n2>n3' }],
      ['emitWarning', 'FLOW_NO_INPUT', 'no context'],
      ['emitWarning', 'FLOW_NO_INPUT', 'empty context', {}],
      ['emitWarning', 'FLOW_NO_INPUT', 'unknown node', { nodeId: 'zzz' }],
      ['emitError', 'SHAPE_REMOVED', 'empty ids', { nodeId: '', edgeId: 'n2>n3' }],
      ['getDiagnostics'],
    ],
  },
  {
    name: 'mappings',
    ops: [
      ['setFrontmatterLineOffset', 3],
      ['addVertexMapping', 'a', U, U, loc(1, 0, 1, 1)],
      ['addVertexMapping', 'a', U, null, loc(2, 0, 2, 1)],
      ['addVertexMapping', 'b', t('x'), 'square', loc(3, 0, 3, 6)],
      ['addVertexMapping', '', t('x'), 'square', loc(3, 0, 3, 6)],
      ['addVertexMapping', 'c', U, U, U],
      ['extendVertexMapping', 'a', loc(2, 1, 4, 1)],
      ['extendVertexMapping', 'a', loc(2, 1, 2, 0)],
      ['extendVertexMapping', 'b', loc(3, 6, 3, 30)],
      ['extendVertexMapping', 'b', loc(3, 6, 3, 10)],
      ['extendVertexMapping', 'zz', loc(9, 2, 9, 12)],
      ['extendVertexMapping', '', loc(9, 2, 9, 12)],
      ['extendVertexMapping', 'a', U],
      ['extendVertexMapping', 'a', loc(1, 1, 1, 8)],
      ['addSubgraphMapping', t(' sg '), t('T'), loc(5, 0, 5, 4), loc(8, 0, 8, 3)],
      ['addSubgraphMapping', 'plain', U, loc(10, 0, 10, 4), U],
      ['addSubgraphMapping', U, U, loc(11, 0, 11, 4), loc(12, 0, 12, 3)],
      ['addSubgraphMapping', t(''), U, loc(11, 0, 11, 4), loc(12, 0, 12, 3)],
      ['addSubgraphMapping', t('noloc'), U, U, U],
      ['addConnectorMapping', t(' conn '), t('T'), loc(13, 2, 13, 11), loc(13, 20, 14, 0)],
      ['addConnectorMapping', t('  '), U, loc(13, 2, 13, 11), U],
      ['addConnectorMapping', U, U, loc(13, 2, 13, 11), U],
      ['addConnectorMapping', t('c2'), U, loc(15, 2, 15, 11), U],
      ['getElementAtPosition', 6, 2],
      ['getElementAtPosition', 5, 0],
      ['getElementAtPosition', 100, 0],
      ['getElementAtPosition', 8, 1],
      ['getElementsOnLine', 6],
      ['getElementsOnLine', 0],
      ['getElementById', 'a'],
      ['getElementById', 'nothing'],
      ['getMappingStats'],
    ],
  },
  {
    name: 'transformData',
    ops: [
      ['transformData', [{ id: 'a', shape: 'circle' }, { id: 'b', shape: 'note' }, { id: 'c' }, { id: 'd', shape: 'rect', isGroup: true }, { id: 'e', shape: 'squareRect' }, { id: 'f', shape: 'tool' }], false],
      ['transformData', [{ id: 'a', shape: 'circle' }, { id: 'b', shape: 'note' }, { id: 'a', shape: 'circle' }], true],
      ['getDiagnostics'],
    ],
  },
  {
    name: 'misc',
    ops: [
      ['setDirection', ' LR '],
      ['getDirection'],
      ['setDirection', 'x<y'],
      ['getDirection'],
      ['setDirection', 'a^b>cvd'],
      ['getDirection'],
      ['setDirection', 'TD'],
      ['getDirection'],
      ['firstGraph'],
      ['firstGraph'],
      ['defaultStyle'],
      ['defaultConfig'],
      ['addClass', 'a,b', ['fill:red', 'color:fill', 'font:x\\,y', 'background-color:blue;stroke:1']],
      ['addClass', 'a', []],
      ['setAccTitle', '  the <b>title</b>'],
      ['setAccDescription', 'line one\n   line two'],
      ['setDiagramTitle', '<i>Title</i>'],
      ['addVertex', 'v', t('V'), 'square'],
      ['setLink', 'v,w', ' https://example.com ', '_blank'],
      ['setLink', 'v', '   '],
      ['setClickEvent', 'v', 'callback', 'a, "b, c"'],
      ['getData'],
    ],
  },
  {
    name: 'misc-loose',
    site: { securityLevel: 'loose' },
    ops: [
      ['addVertex', 'v', t('V <script>x</script>'), 'square'],
      ['setLink', 'v,w', ' javascript:alert(1) ', '_self'],
      ['setClickEvent', 'v,w', 'callback', 'a, "b, c" ,"d"'],
      ['setClickEvent', 'v', 'cb'],
      ['setTooltip', 'v', '<b>tip</b>'],
      ['getData'],
    ],
  },
  {
    name: 'inheritDir',
    site: { flowchart: { inheritDir: true }, direction: 'RL' },
    ops: [
      ['addSubGraph', t('a'), [['x']], t('A'), 'flow'],
      ['setDirection', 'LR'],
      ['addSubGraph', t('b'), [['y']], t('B'), 'flow'],
      ['addSubGraph', t('c'), [['z'], { stmt: 'dir', value: 'BT' }], t('C'), 'flow'],
      ['getData'],
    ],
  },
  {
    name: 'inheritDir-number',
    site: { flowchart: { inheritDir: true }, direction: 5 },
    ops: [['addSubGraph', t('a'), [['x']], t('A'), 'flow'], ['getData']],
  },
  {
    name: 'colorSlots',
    site: { theme: 'redux-color' },
    ops: [
      [
        'assignColorSlots',
        [
          { id: 'c1', isGroup: true, shape: 'flowGroup' },
          { id: 'c2', isGroup: false, shape: 'collapsedGroup', cssClasses: 'x' },
          { id: 'c3', isGroup: true },
          { id: 'c4', isGroup: true },
          { id: 'n1', isGroup: false, shape: 'roundedRect', cssClasses: 'default ' },
          { id: 'n2', isGroup: false, shape: 'diamond', cssClasses: '  a \t b  ' },
          { id: 'n3', isGroup: false, shape: 'roundedRect' },
          { id: 'n4', isGroup: false, shape: 'roundedRect', cssClasses: 'keep' },
          { id: 'n5', isGroup: false, shape: 'roundedRect', cssClasses: 'keep' },
          { id: 'n6', isGroup: false, shape: 'roundedRect', cssClasses: '' },
        ],
        { n1: 'tool', n2: 'decision', n3: 'action', n4: 'nokind', n6: 'connector' },
        [['c2', 0], ['c1', 3], ['zz', 9]],
      ],
    ],
  },
  {
    name: 'colorSlots-no-palette',
    ops: [
      [
        'assignColorSlots',
        [{ id: 'c1', isGroup: true }, { id: 'c2', isGroup: true }, { id: 'n1', isGroup: false, cssClasses: 'default' }],
        { n1: 'task' },
        [],
      ],
    ],
  },
  {
    name: 'element-at-position',
    ops: [
      ['addVertexMapping', 'wide', t('x'), 'square', loc(1, 0, 1, 30)],
      ['addVertexMapping', 'narrow', t('x'), 'square', loc(1, 5, 1, 10)],
      ['addVertexMapping', 'same', t('x'), 'square', loc(1, 6, 1, 11)],
      ['addVertexMapping', 'narrower', t('x'), 'square', loc(1, 7, 1, 9)],
      ['addSubgraphMapping', t('tall'), U, loc(1, 0, 1, 4), loc(5, 0, 5, 3)],
      ['addSubgraphMapping', t('short'), U, loc(1, 2, 1, 4), loc(2, 40, 2, 43)],
      ['getElementAtPosition', 1, 8],
      ['getElementAtPosition', 1, 6],
      ['getElementAtPosition', 1, 5],
      ['getElementAtPosition', 1, 20],
      ['getElementAtPosition', 1, 31],
      ['getElementAtPosition', 2, 0],
      ['getElementAtPosition', 2, 44],
      ['getElementAtPosition', 5, 3],
      ['getElementAtPosition', 5, 4],
      ['getElementAtPosition', 0, 0],
      ['getElementsOnLine', 1],
      ['getElementsOnLine', 2],
      ['getElementsOnLine', 6],
    ],
  },
  { name: 'clear', ops: [['addVertex', 'v', t('V'), 'square'], ['addConnector', t('c')], ['emitWarning', 'FLOW_NO_INPUT', 'x'], ['clear'], ['getData'], ['clear', 'gen-1'], ['addVertex', 'v', t('V'), 'square'], ['setTooltip', 'v', 'x']] },
];

// Arguments for destructLink(str, startStr).
const linkEnds = ['-->', '--x', '-.-', '-..-', '--->', '---x', '-...-', '----', '---', '--', '-', '', ' --> ', '  --x  ', '.', 'x', '>', '-.', '-.->', '==>', '--o', '<-->', '-->x', '-.x', '.-', '-..', '-------------->', 'a.b.c-', '😀>'];
const linkStarts = [null, '--', ' -- ', '-.', '', '==', '.', 'x--'];
const links = [];
for (const end of linkEnds) {
  for (const start of linkStarts) {
    links.push([end, start]);
  }
}

// Bodies for stripLineTrailingCommas(body).
const commas = [
  '',
  ',',
  'a: 1,',
  'a: 1, ',
  'a: 1,\t \t',
  'a: 1,\nb: 2,\nc: 3',
  '\n  shape: tool,\n  label: "x",\n',
  'a: "x,",\nb: \'y,\',',
  'a: "x,\n  still, in, string,\n  end",\nb: 2,',
  "a: 'x,\n  still,\n  end',\nb: 2,",
  'a: "esc \\" quote, ",\nb: "trailing backslash \\\\",\nc: 3,',
  'a: [1, 2,\n  3,\n  4],\nb: 5,',
  'a: { b: 1,\n  c: 2,\n},\nd: 6,',
  'a: ]],\nb: }},\nc: 1,',
  'a: [[1,\n2],\n3],\nb: 4,',
  'a: 1, # comment,\nb: 2,# not a comment,\nc: 3,\t# tab comment\n# full line comment,\nd: 4 #,',
  '#,\n #,\n\t#,\na#b,',
  'text: |\n  keep, this,\n  and, this,\nafter: 1,',
  'text: >\n  folded,\n\n  still folded,\nafter: 1,',
  'text: |-\n  a,\nb: >+\n  c,\nd: |2\n    e,\nf: >-2\n  g,\nh: | \n  i,\nj: 1,',
  '  text: |\n    indented,\n   less but deeper,\n  same indent,\n deeper: no,',
  'text: |\n\n\n  after blank lines,\nnext: 1,',
  'a: | not a block,\nb: >x,\nc: "|",\nd: x |,\ne: :|,',
  'a:|\n  x,\nb: 1,',
  'a: 1,,\nb: 2, ,\nc: , \nd:,',
  'a: "unterminated,\nb: 2,\nc: 3,',
  "it's: here,\nb: 2,",
  'a: 1,\r\nb: 2,\r\n',
  'a: 1,\u00a0\nb: 2,\u3000',
  '\u00a0\u00a0text: |\n\u00a0\u00a0\u00a0x,\n\u00a0y,',
  'emoji: "😀",\nkey😀: 1,\n"\\😀": 2,',
  'a: \\,\nb: "\\\\",\nc: 3,',
];

// Texts for stripPrototypeKeys(JSON.parse(text)).
const proto = [
  '{}',
  '[]',
  'null',
  '5',
  '"text"',
  'true',
  '{"a":1,"__proto__":{"x":1},"constructor":2,"prototype":3,"b":{"__proto__":4,"c":[{"prototype":5,"d":6},7,null,[{"constructor":8}]]}}',
  '[{"__proto__":1},{"2":"two","1":"one","b":"bee","a":"ay"}]',
  '{"toString":1,"valueOf":2,"hasOwnProperty":3}',
];

// Nodes for normaliseNodeShapes.
const shapeNames = [
  null, '', 'squareRect', 'rect', 'roundedRect', 'subroutine', 'subprocess', 'subproc', 'framed-rectangle', 'lean-right', 'diamond', 'lin-doc',
  'lined-document', 'hexagon', 'hex', 'connector', 'collapsedGroup', 'doc', 'stadium', 'terminal', 'circle', 'trapezoid', 'inv_trapezoid',
  'inv-trapezoid', 'doublecircle', 'double-circle', 'typeDeclaration', 'procs', 'lean_left', 'lean-left', 'in-out', 'cylinder', 'ellipse', 'odd',
  'tag-rect', 'tagged-rectangle', 'delay', 'half-rounded-rectangle', 'lin-rect', 'lined-rectangle', 'win-pane', 'window-pane', 'curv-trap',
  'curved-trapezoid', 'task', 'tool', 'input', 'decision', 'refdoc', 'action', 'round', 'lean_right', 'note', 'constructor', 'toString', '__proto__',
  'flowGroup', 'text', 'rounded', 'fr-rect',
];
const shapes = [];
for (const shape of shapeNames) {
  shapes.push({ shape, isGroup: false });
}
shapes.push({ shape: 'circle', isGroup: true }, { shape: null, isGroup: true }, { shape: 'flowGroup', isGroup: true });

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'afdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(
  argsFile,
  JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh: !!fresh, site })), api, links, commas, proto, shapes })
);
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'agentflow-db-page.js', '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
// The page returns a JSON text, which the oracle writes as a JSON string.
const result = JSON.parse(JSON.parse(fs.readFileSync(resultFile, 'utf8')));
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
const delta = (config) => {
  const flat = flatten(config, [], new Map());
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
  return { set, del };
};
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
    delete rec.config;
    rec.configDelta = delta(r.config);
  }
  return rec;
});
const apiGolden = result.api.map((r, i) => {
  const rec = { ...r, ops: api[i].ops };
  if (api[i].site) {
    rec.site = api[i].site;
  }
  delete rec.config;
  rec.configDelta = delta(r.config);
  return rec;
});
const doc = {
  baseConfig: base.config,
  cases: golden,
  api: apiGolden,
  links: links.map((args, i) => ({ args, result: result.links[i] })),
  commas: commas.map((body, i) => ({ body, result: result.commas[i] })),
  proto: proto.map((text, i) => ({ text, result: result.proto[i] })),
  shapes: shapes.map((node, i) => ({ node, result: result.shapes[i] })),
  slots: result.slots,
  constants: result.constants,
};
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(doc), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
const dumped = golden.filter((r) => r.nodes).length;
console.log(
  `${outFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${dumped} with a db, ${errors} with an error, ${apiGolden.length} api cases`
);
