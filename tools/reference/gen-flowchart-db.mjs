// Writes internal/diagrams/flowchart/testdata/db.json.gz: what a layout
// algorithm of real mermaid receives from the draw function of the
// flowchart renderer (the nodes and edges of getData and the layout
// properties), the content of the flowchart db (the vertices, the edges, the
// subgraphs, the classes, the tooltips, the direction, the title, the
// accessibility values, the click callbacks), the config after the init
// hook and a checksum of the text of the style element, for
//   - every fixture of the corpus that mermaid renders as flowchart-v2,
//     flowchart-elk or swimlane (all of testdata/corpus/mermaid-e2e/flowchart
//     and some fixtures of other families),
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of flowDb.ts and of the grammar.
// A case that mermaid rejects has the error message instead. The file also
// has the results of destructLink for a list of link texts.
//
// The db needs a browser (DOMPurify), so the script runs
// flowchart-db-page.js with the esm-eval command of the browser oracle, one
// time. A corpus fixture runs in a new page, as in the reference renders.
//
// Usage (in tools/reference): node gen-flowchart-db.mjs
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
const outFile = path.join(repoRoot, 'internal/diagrams/flowchart/testdata/db.json.gz');
const flowTypes = new Set(['flowchart-v2', 'flowchart-elk', 'swimlane']);
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
// The fixtures of the corpus: every fixture of the flowchart family, and the
// fixtures of other families that the reference rendered as a flowchart.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => {
    if (f.startsWith('flowchart/')) {
      return true;
    }
    try {
      const meta = JSON.parse(fs.readFileSync(path.join(repoRoot, refDir, f.replace(/\.mmd$/, '.json')), 'utf8'));
      return flowTypes.has(meta.diagramType);
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
  'B --- C',
  'C -.-> D',
  'D ==> E',
  'E ~~~ F',
  'A --o B',
  'B --x C',
  'C <--> D',
  'D o--o E',
  'E x--x F',
  'A -- text --> B',
  'A == text ==> B',
  'A -. text .-> B',
  'A -->|label| B',
  'A ---->|long| B',
  'A e1@--> B',
  'e1@{ animate: true }',
  'e1@{ animation: fast, curve: linear }',
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
  'N12@{ shape: cyl, label: "Meta" }',
  'N13@{ icon: "fa:user", form: circle, pos: t, h: 60 }',
  'N14@{ img: "https://example.com/a.png", w: 100, h: 80, constraint: on }',
  'N15["`**bold** _text_`"]',
  'N16["A quoted (text)"]',
  'subgraph S1 [A title]',
  'subgraph S2',
  'subgraph "Quoted title"',
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
  'S1@{ view: collapsed }',
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
  // Skip the front matter: its config can have a line that starts with "flowchart".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*(graph|flowchart|swimlane-beta)/.test(l));
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

const rnd = prng(0x666c6f77);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const fc = (body, front = '') => front + 'flowchart TD\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const extras = [
  // Headers and directions.
  'graph TD\nA-->B\n',
  'graph\nA-->B\n',
  'flowchart\nA-->B\n',
  'graph LR;A-->B;B-->C;',
  'graph RL\nA-->B\n',
  'graph BT\nA-->B\n',
  'graph TB\nA-->B\n',
  'graph BR\nA-->B\n',
  'graph >\nA-->B\n',
  'graph <\nA-->B\n',
  'graph ^\nA-->B\n',
  'graph v\nA-->B\n',
  'flowchart-elk LR\nA-->B\n',
  'swimlane-beta LR\nsubgraph L1\nA-->B\nend\nsubgraph L2\nC\nend\nB-->C\n',
  '\n\n   flowchart TD   \n\n\nA-->B\n\n',
  'flowchart TD;\nA-->B;\n',
  'flowchart TD\n',
  'flowchart TD\nA\n',
  'flowchart TD A-->B\n',
  'graph TD\ngraph LR\nA-->B\n',
  // Vertex shapes of the grammar.
  fc('A[Square]\nB(Round)\nC((Circle))\nD(((Double)))\nE([Stadium])\nF[[Subroutine]]\nG[(Cylinder)]\nH{Diamond}\nI{{Hexagon}}\nJ>Odd]\nK[/Trapezoid\\]\nL[\\InvTrapezoid/]\nM[/LeanRight/]\nN[\\LeanLeft\\]\nO(-Ellipse-)\nP[|borders:lt|Props]\nQ'),
  fc('A["Quoted text"]\nB("with (parens)")\nC{"a ? b : c"}\nD["`**Markdown** _text_\nsecond line`"]\nE(["`md stadium`"])'),
  fc('A[ padded text ]\nB["  padded quoted  "]\nC[""]\nD[" "]'),
  fc('A["#35; hash"]\nB["#quot;quoted#quot;"]\nC["#9829; heart #amp; amp"]\nD[x #lt; y]'),
  fc('A[Line one<br/>line two<br>three<br />four]\nB[<b>bold</b> <i>it</i> <script>alert(1)</script>]\nC["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]\nD[<img src=x onerror=alert(1)>]'),
  fc('A[你好] --> B[Ünïcödé]\nC[😀 emoji] --> Привет\nÅ --> ß'),
  fc('A-B --> C.D\nE_F --> G/H\n1 --> 2\n3a --> a3\ndefault --> x\nv --> w\nA-v --> B'),
  fc('end1 --> B\nsubgraphs --> C\nstyles --> classy\nA --> endpoint'),
  fc('A[text with keywords: graph subgraph end style class click]'),
  fc('A[Text] --> A\nA --> A\nA[Changed text]\nA(Round now)'),
  fc('A\nA[Later text]\nB[First]\nB'),
  // Links.
  fc('A --> B\nA --- B\nA -.-> B\nA -.- B\nA ==> B\nA === B\nA ~~~ B\nA --o B\nA --x B\nA o--o B\nA x--x B\nA <--> B\nA <-.-> B\nA <==> B\nA o-.-o B\nA x==x B'),
  fc('A ---> B\nA ----> B\nA -----> B\nA ---- B\nA ----- B\nA -..-> B\nA -...-> B\nA -..- B\nA ===> B\nA ====> B\nA ==== B\nA ~~~~ B\nA --------------> B\nA -...........-> B\nA ==============> B'),
  fc('A -- text --> B\nA -- text --- B\nA == text ==> B\nA == text === B\nA -. text .-> B\nA -. text .- B\nA -- "quoted" --> B\nA -- "`md **text**`" --> B\nA -- text with spaces and - dash --> B'),
  fc('A -->|text| B\nA ---|text| B\nA -.->|text| B\nA ==>|text| B\nA -->|"quoted"| B\nA -->|"`md`"| B\nA --o|circle| B\nA --x|cross| B\nA <-->|both| B\nA -->| padded | B'),
  fc('A x-- text --x B\nA o-- text --o B\nA <-- text --> B\nA x== text ==x B\nA o-. text .-o B\nA <-. text .-> B'),
  fc('A x-- text --> B'),
  fc('A o-- text --x B'),
  fc('A -- text ==> B'),
  fc('A == text --> B'),
  fc('A -. text --> B'),
  fc('A --> B --> C --> D\nA --> B & C\nA & B --> C\nA & B --> C & D\nA --> B & C --> D & E\nA & B & C --> D'),
  fc('A-->B;B-->C;C-->A'),
  fc('A   -->   B\nC-->D\nE --   text   --> F'),
  fc('A --> B\nA --> B\nA --> B\nB --> A\nA --- B'),
  // Edge ids and edge metadata.
  fc('A e1@--> B\ne1@{ animate: true }'),
  fc('A e1@--> B\ne1@{ animation: fast }\nB e2@--> C\ne2@{ animation: slow, curve: linear }\nC e3@==> D\ne3@{ curve: stepBefore }'),
  fc('A e1@--> B & C\nD & E e2@--> F\nA & B e3@--> C & D'),
  fc('A e1@--> B\nB e1@--> C\nC e1@--> D'),
  fc('A e1@-- text --> B\nA e2@-->|text| B\ne1@{ animate: false }\ne2@{ animate: null }'),
  fc('A e1@--> B\nclassDef dashed stroke-dasharray: 5 5,stroke:red\nclass e1 dashed\ne1@{ animate: true, animation: slow, curve: natural, extra: 1 }'),
  fc('A L_A_B_0@--> B\nA --> B\nL_A_B_0@{ animate: true }'),
  fc('A --> B\nL_A_B_0@{ animate: true }'),
  fc('A e1@--> B\ne1@{ shape: diamond, label: "not a node" }\ne1'),
  fc('A e1@--> B\ne1@{\n  animate: true\n  curve: basis\n}'),
  // Node metadata.
  fc('A@{ shape: diamond }\nB@{ shape: rounded, label: "Rounded" }\nC@{ shape: cyl }\nD@{ shape: rect, label: "Text" }\nE@{ label: "Only label" }\nF@{ shape: circle, label: "`**md**`" }'),
  fc('A@{ shape: notched-rectangle }\nB@{ shape: lin-cyl }\nC@{ shape: sm-circ }\nD@{ shape: fr-circ }\nE@{ shape: fork }\nF@{ shape: hourglass }\nG@{ shape: comment }\nH@{ shape: bolt }\nI@{ shape: doc }\nJ@{ shape: delay }\nK@{ shape: das }\nL@{ shape: curv-trap }\nM@{ shape: div-rect }\nN@{ shape: win-pane }\nO@{ shape: f-circ }\nP@{ shape: lin-doc }\nQ@{ shape: notch-pent }\nR@{ shape: flip-tri }\nS@{ shape: sl-rect }\nT@{ shape: docs }\nU@{ shape: st-rect }\nV@{ shape: flag }\nW@{ shape: bow-rect }\nX@{ shape: cross-circ }\nY@{ shape: tag-doc }\nZ@{ shape: tag-rect }'),
  fc('A@{ shape: text, label: "Text block" }\nB@{ shape: odd }\nC@{ shape: tri }\nD@{ shape: brace }\nE@{ shape: brace-r }\nF@{ shape: braces }\nG@{ shape: hex }\nH@{ shape: lean-r }\nI@{ shape: lean-l }\nJ@{ shape: trap-b }\nK@{ shape: trap-t }\nL@{ shape: dbl-circ }\nM@{ shape: stadium }\nN@{ shape: subproc }\nO@{ shape: db }\nP@{ shape: diam }\nQ@{ shape: event }\nR@{ shape: bang }\nS@{ shape: cloud }'),
  fc('A@{ shape: squareRect }\nB@{ shape: roundedRect }\nC@{ shape: state }\nD@{ shape: stateStart }\nE@{ shape: labelRect }\nF@{ shape: choice }\nG@{ shape: note }\nH@{ shape: rectWithTitle }\nI@{ shape: classBox }\nJ@{ shape: erBox }\nK@{ shape: kanbanItem }\nL@{ shape: mindmapCircle }\nM@{ shape: defaultMindmapNode }\nN@{ shape: collapsedGroup }'),
  fc('A@{ shape: Diamond }'),
  fc('A@{ shape: lean_right }'),
  fc('A@{ shape: nosuchshape }'),
  fc('A@{ shape: 5 }'),
  fc('A@{ shape: true }'),
  fc('A@{ shape: [a, b] }'),
  fc('A@{ shape: "" , label: x }'),
  fc('A@{ label: 2024 }\nB@{ label: true }\nC@{ label: 1.5 }\nD@{ label: "" }\nE@{ label: null }\nF@{ label: 0 }'),
  fc('A@{ label: [a, b] }'),
  fc('A@{ label: { a: 1 } }'),
  fc('A@{ label: "x", labelType: string }\nB@{ label: "x", labelType: text }\nC@{ label: "x", labelType: markdown }\nD@{ label: "x", labelType: html }\nE@{ label: "x", labelType: 5 }\nF[plain]@{ labelType: string }'),
  fc('A@{ icon: "fa:user" }\nB@{ icon: "fa:user", form: circle }\nC@{ icon: "fa:user", form: square, label: "Square" }\nD@{ icon: "fa:user", form: rounded, pos: t }\nE@{ icon: "fa:user", form: other, pos: b, h: 80 }\nF[Has text]@{ icon: "fa:user" }\nG@{ icon: "fa:user", label: "  " }'),
  fc('A@{ img: "https://example.com/a.png" }\nB@{ img: "https://example.com/a.png", label: "Image", pos: t, w: 60, h: 60, constraint: off }\nC@{ img: "x.png", w: "100", h: abc, constraint: on }\nD@{ img: "x.png", icon: "fa:x", form: circle }\nE[Text]@{ img: "x.png" }'),
  fc('A@{ icon: 5, form: 6, pos: 7, constraint: 8 }'),
  fc('A@{ img: 5, w: [5], h: true }'),
  fc('A@{ icon: "fa:x", label: 5 }'),
  fc('A@{ w: 100, h: 50 }\nB@{ w: 0, h: 0 }\nC@{ w: -5, h: 1e3 }\nD@{ w: 0x10, h: .5 }'),
  fc('A@{ shape: diamond, label: "Multi\n  line\n  label" }'),
  fc('A@{\n  shape: diamond\n  label: "Block yaml"\n}\nB@{\n  label: |\n    literal\n    block\n}'),
  fc('A@{\n  just text\n}'),
  fc('A@{\n  - a\n  - b\n}'),
  fc('A@{\n\n}'),
  fc('A@{ }'),
  fc('A@{}'),
  fc('A@{ shape: diamond, shape: circle }'),
  fc('A@{ : }'),
  fc('A@{ not yaml: [ }'),
  fc('A@{ label: "a } b" }'),
  fc('A@{ label: "a: b, c" , shape: rect }'),
  fc('A@{ shape: rect } --> B@{ shape: circle }\nC@{ shape: diamond } & D@{ shape: hex } --> E'),
  fc('A[Text]@{ shape: diamond } --> B'),
  fc('A --> B@{ shape: diamond }\nA@{ shape: circle } --> B'),
  fc('A@{ shape: rect, label: "<b>html</b> <script>x</script>" }'),
  fc('A@{ shape: rect, unknownKey: 1, another: [1, 2] }'),
  // Styles and classes.
  fc('A --> B\nstyle A fill:#f9f,stroke:#333,stroke-width:4px\nstyle B fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5'),
  fc('style A fill:#f9f\nA --> B\nstyle C color:red'),
  fc('A --> B\nclassDef default fill:#f9f,stroke:#333,stroke-width:4px\nclassDef foo color:red,fill:blue\nclassDef bar,baz stroke:green\nclass A foo\nclass A,B bar\nclass B baz'),
  fc('A:::foo --> B:::bar\nC[Text]:::foo\nD:::foo:::bar\nclassDef foo fill:red\nclassDef bar fill:blue,color:white,background-color:black'),
  fc('classDef foo font-family:Arial\\,sans-serif,fill:red\nA:::foo'),
  fc('classDef foo fill:#f00;\nclassDef bar stroke:#0f0,color:#00f;\nA:::foo --> B:::bar'),
  fc('classDef node fill:red\nclassDef cluster fill:blue\nA --> B'),
  fc('class A nodef\nA --> B\nclass missing foo'),
  fc('A --> B\nlinkStyle 0 stroke:#ff3,stroke-width:4px,color:red\nB --> C\nlinkStyle 1 fill:blue,stroke:green'),
  fc('A --> B\nB --> C\nlinkStyle default stroke:blue,stroke-width:2px\nlinkStyle 1 stroke:red'),
  fc('A --> B\nB --> C\nC --> D\nlinkStyle 0,1,2 stroke:red\nlinkStyle 1 interpolate linear\nlinkStyle default interpolate cardinal stroke:blue\nlinkStyle 2 interpolate step stroke:green'),
  fc('linkStyle default interpolate basis\nA --> B'),
  fc('A --> B\nlinkStyle 1 stroke:red'),
  fc('A --> B\nlinkStyle 0,5 stroke:red'),
  fc('linkStyle 0 stroke:red\nA --> B'),
  fc('A --> B\nlinkStyle 0 stroke-width:2px'),
  fc('A --> B\nlinkStyle 00 stroke:red'),
  // Interaction.
  fc('A --> B\nclick A callback\nclick B callback "Tooltip for B"'),
  fc('A --> B\nclick A call callback()\nclick B call callback("arg", 5) "Tooltip"\nclick A call other(a, "b, c", d)'),
  fc('A --> B --> C --> D\nclick A href "https://example.com"\nclick B href "https://example.com" "Tooltip"\nclick C href "https://example.com" _blank\nclick D href "https://example.com" "Tooltip" _self'),
  fc('A --> B --> C --> D\nclick A "https://example.com"\nclick B "https://example.com" "Tooltip"\nclick C "https://example.com" _parent\nclick D "https://example.com" "Tooltip" _top'),
  fc('A --> B\nclick A,B callback "Shared tooltip"\nclick A,B "https://example.com/shared"'),
  fc('A --> B\nclick A href "javascript:alert(1)"\nclick B href "  "'),
  fc('A --> B\nclick A href "data:text/html,x"\nclick B "relative/path?a=b&c=d#frag"'),
  fc('A --> B\nclick missing callback "tip"\nclick missing "https://example.com"'),
  fc('A --> B\nclick A callback "<b>bold</b> tip <script>x</script>"'),
  fc('A --> B\nclick A callback ""'),
  fc('subgraph S\nA --> B\nend\nclick S "https://example.com" "Subgraph tip"\nclick S callback'),
  fc('A e1@--> B\nclick e1 callback "edge tip"'),
  // Subgraphs.
  fc('subgraph S\nA --> B\nend'),
  fc('subgraph S [Title of S]\nA --> B\nend\nsubgraph T ["Quoted title"]\nC\nend\nsubgraph U ["`md **title**`"]\nD\nend'),
  fc('subgraph Title with spaces\nA --> B\nend\nsubgraph "Quoted with spaces"\nC\nend\nsubgraph "`md title`"\nD\nend'),
  fc('subgraph\nA --> B\nend'),
  fc('subgraph S\nend'),
  fc('subgraph S\n\nend'),
  fc('subgraph Outer\nsubgraph Inner1\nA\nend\nsubgraph Inner2\nB\nend\nA --> B\nend\nsubgraph Sibling\nC\nend\nOuter --> Sibling\nInner1 --> C'),
  fc('subgraph S\ndirection LR\nA --> B\nend\nsubgraph T\ndirection BT\nC --> D\nend\nsubgraph U\ndirection TD\nE\nend\nsubgraph V\n  direction RL\nF\nend'),
  fc('subgraph S\ndirection LR\ndirection TB\nA\nend'),
  fc('direction LR\nA --> B'),
  fc('subgraph S\nA\nend\nsubgraph S\nB\nend\nsubgraph S [Second title]\nC\nend'),
  fc('subgraph S\nA\nsubgraph S\nB\nend\nend'),
  fc('subgraph S\nA\nend\nsubgraph T\nA\nB\nend'),
  fc('subgraph S\nA\nA\nB\nB\nend'),
  fc('subgraph S\nS --> A\nend'),
  fc('A --> B\nsubgraph A\nC\nend'),
  fc('subgraph S\nA --> B\nend\nclassDef foo fill:red\nclass S foo\nstyle S stroke:blue,fill:#eee'),
  fc('subgraph S\nA --> B\nend\nS:::foo\nclassDef foo fill:red'),
  fc('subgraph 1\nA\nend\nsubgraph 2\nB\nend\n1 --> 2'),
  fc('subgraph S-1 [a-b]\nA\nend\nsubgraph S&T\nB\nend\nsubgraph S:T\nC\nend\nsubgraph S*T\nD\nend'),
  fc('subgraph S\nA & B --> C\nD@{ shape: diamond }\nE e1@--> F\nend'),
  fc('subgraph S\nA;B;C\nend'),
  fc('subgraph S [T]\n  A\n  end\n  subgraph U\n  B\n  end  \n'),
  // Collapsed subgraphs.
  fc('subgraph S\nA --> B\nend\nS@{ view: collapsed }\nC --> A\nB --> D'),
  fc('subgraph Outer\nsubgraph Inner\nA --> B\nend\nC\nend\nInner@{ view: collapsed }\nD --> A\nB --> C\nC --> D'),
  fc('subgraph Outer\nsubgraph Inner\nA --> B\nend\nC\nend\nOuter@{ view: collapsed }\nD --> A\nB --> C\nC --> D\nInner --> D'),
  fc('subgraph Outer\nsubgraph Inner\nA --> B\nend\nC\nend\nOuter@{ view: collapsed }\nInner@{ view: collapsed }\nD --> A'),
  fc('subgraph S\nA --> A\nend\nS@{ view: collapsed }\nB --> B'),
  fc('subgraph S\nA\nend\nS@{ view: expanded, algorithm: elk.box }\nsubgraph T\nB\nend\nT@{ algorithm: elk.rectpacking }\nT@{ view: collapsed }'),
  fc('subgraph S\nA\nend\nS@{ }'),
  fc('subgraph S\nA\nend\nclassDef foo fill:red\nclass S foo\nS@{ view: collapsed }'),
  fc('S@{ view: collapsed }\nsubgraph S\nA\nend'),
  fc('subgraph S\nA\nend\nS@{\n  view: collapsed\n}'),
  // Accessibility and title.
  fc('accTitle: The accessible title\naccDescr: The accessible description\nA --> B'),
  fc('accDescr {\n  A description\n  on several lines\n}\nA --> B'),
  fc('accTitle:    padded   \naccDescr:   padded too  \nA'),
  fc('A --> B', '---\ntitle: The title\n---\n'),
  fc('A --> B', '---\ntitle: "<b>Title</b> with #35;"\n---\n'),
  // Config.
  fc('A --> B', cfg('look: handDrawn')),
  fc('A --> B', cfg('look: classic\ntheme: forest')),
  fc('A --> B', cfg('theme: dark')),
  fc('A[<b>x</b>] --> B', cfg('htmlLabels: false')),
  fc('A["<a href=\'https://x.example\' target=\'_blank\'>link</a>"] --> B', cfg('htmlLabels: false')),
  fc('A[<b>x</b>] --> B', cfg('flowchart:\n  htmlLabels: false')),
  fc('A --> B', cfg('flowchart:\n  curve: linear\n  padding: 20\n  minNodeWidth: 50\n  nodeSpacing: 10\n  rankSpacing: 20')),
  fc('A --> B', cfg('flowchart:\n  padding: 0\n  minNodeWidth: 0')),
  fc('subgraph S\nA --> B\nend\nsubgraph T\ndirection RL\nC\nend', cfg('flowchart:\n  inheritDir: true')),
  'flowchart LR\nsubgraph S\nA --> B\nend\n',
  cfg('flowchart:\n  inheritDir: true') + 'flowchart LR\nsubgraph S\nA --> B\nend\n',
  fc('A --> B', cfg('layout: dagre')),
  fc('A --> B', cfg('layout: elk')),
  fc('A --> B', cfg('layout: elk.stress')),
  fc('A --> B', cfg('layout: nosuchlayout')),
  fc('A --> B', cfg('layout: 5')),
  'flowchart-elk TD\nA --> B\n',
  cfg('layout: dagre') + 'flowchart-elk TD\nA --> B\n',
  fc('A --> B', '%%{init: {"flowchart": {"curve": "step"}, "look": "classic"}}%%\n'),
  fc('A --> B', '%%{init: {"arrowMarkerAbsolute": true}}%%\n'),
  fc('A --> B', cfg('arrowMarkerAbsolute: true')),
  fc('A --> B', cfg('flowchart:\n  arrowMarkerAbsolute: true')),
  fc('A --> B', cfg('securityLevel: loose')),
  fc('A --> B', cfg('maxEdges: 1')),
  // Quirks of the grammar and of the db.
  fc('subgraph S\nhasOwnProperty --> B\nend'),
  fc('subgraph S\nA --> hasOwnProperty\nend'),
  fc('subgraph S\nhasOwnProperty\nend'),
  fc('subgraph S\n__proto__ --> A\n__proto__\nA\nend'),
  fc('subgraph S\nconstructor --> toString\nvalueOf\nconstructor\nend'),
  fc('subgraph S\n;\nA\n;;\nend'),
  fc('subgraph S\naccTitle: inside a subgraph\nA\naccDescr: also inside\nend'),
  fc('subgraph S\naccDescr {\n  inside\n}\nA\nend'),
  fc('A@{ shape: constructor }'),
  fc('A@{ shape: sm-circ, label: 5 }'),
  fc('__proto__ --> constructor\nhasOwnProperty --> toString'),
  fc('A:::foo & B:::bar --> C:::foo\nclassDef foo fill:red\nclassDef bar fill:blue'),
  fc('A#35;B --> C#quot;D'),
  'flowchart TD\r\nA --> B\r\nsubgraph S\r\nC\r\nend\r\n',
  'flowchart TD\n\tA --> B\n\tsubgraph S\n\t\tC@{ shape: diamond }\t\n\tend\n',
  fc('A --> B %% trailing comment\n%% a comment line\nB --> C'),
  fc('A@{ shape: diamond }   \n\n\n   B'),
  fc('A{{"hex"}}   \nB{"diamond"}  \n  C'),
  fc('A["a } b"] --> B\nC{x}  \n\n D'),
  fc('A -->|"a | b"| B'),
  fc('A["`line one\nline two`"] --> B["plain\nnewline"]'),
  fc('A[" "] --> B["  x  "]'),
  fc('A e1@--> B\ne1@{ animate: "true" }\nB e2@--> C\ne2@{ animate: 1, animation: 5, curve: 7 }\nC e3@--> D\ne3@{ animation: [a], curve: "" }'),
  fc('A --> B\nlinkStyle 0 interpolate basis\nlinkStyle 0 interpolate\n'),
  fc('A --> B\nlinkStyle default interpolate cardinal\nB --> C\nC e1@--> D\ne1@{ curve: linear }'),
  fc('click A callback\nA --> B'),
  fc('A --> B\nclick A call callback(\"quoted, arg\", plain , \" spaced \")'),
  fc('A --> B\nclick A call callback(\")'),
  fc('A --> B\nclick A href "https://example.com/a b?c=d e#f"\nclick B href "HTTPS://EXAMPLE.COM/%7Efoo/../bar"'),
  fc('A --> B\nclick A href "mailto:a@example.com"\nclick B href "//example.com/x"'),
  fc('A --> B\nclick A href "ftp://example.com"\nclick B href "vbscript:x"'),
  fc('subgraph S ["`**md** title`"]\nA\nend\nsubgraph T ["plain string"]\nB\nend\nsubgraph U [text title]\nC\nend'),
  fc('subgraph S [<b>html</b> <script>x</script> title]\nA\nend'),
  fc('A --> B\nsubgraph S\nA\nend\nsubgraph T\nB\nS\nend\nsubgraph U\nT\nend'),
  fc('subgraph A\nB\nend\nsubgraph B\nA\nend'),
  fc('A[&quot;x&quot;] --> B[&quot;]\nC[&quot;&quot;] -- &quot;y&quot; --> D\nE -->|&quot;z&quot;| F'),
  fc('A[&lt;b&gt;x&lt;/b&gt; &amp; y] --> B[a &nbsp; b]'),
  fc('A --> B\nA@{ shape: diamond }\nB@{ label: "new label" }\nA@{ label: "again", shape: circle }'),
  fc("A@{ label: 'single quoted', shape: rect }\nB@{ \"label\": \"quoted key\" }\nC@{ label: plain text with spaces }"),
  fc('A@{ shape: rect, label: "a # not a comment" }\nB@{ shape: rect # a comment\n}'),
  fc('A e1@--> B\nB e1@{ shape: diamond }\ne1 --> C'),
  // Statements with errors.
  fc('A --> '),
  fc('--> B'),
  fc('A -> B'),
  fc('A [text'),
  fc('A[text]]'),
  fc('subgraph S\nA'),
  fc('end'),
  fc('A --> B\nend'),
  fc('class A'),
  fc('classDef'),
  fc('style A'),
  fc('click A'),
  fc('linkStyle x stroke:red'),
  fc('A@{ shape: diamond'),
  fc('A --> B @{ shape: diamond }'),
  fc('A["unterminated'),
  fc('A{{x}'),
  fc('A((x)'),
  'flowchart XY\nA --> B\n',
  'flowchart TD LR\nA --> B\n',
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Inputs that need a site config run in their own page.
const sited = [
  { site: { securityLevel: 'loose' }, text: fc('A --> B --> C\nclick A callback "Tip"\nclick B call callback("x", "y, z", 3) "Tip B"\nclick C call noargs()\nclick A href "javascript:alert(1)"\nclick B "https://example.com" _blank') },
  { site: { securityLevel: 'loose' }, text: fc('A[<b>bold</b> <script>x</script>] --> B["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]\nclick A callback "<b>tip</b>"') },
  { site: { securityLevel: 'antiscript' }, text: fc('A[<b>bold</b> <script>x</script>] --> B\nclick A callback\nclick B href "javascript:alert(1)"') },
  { site: { securityLevel: 'other' }, text: fc('A[<b>bold</b> x=y] --> B[a<br/>b]') },
  { site: { maxEdges: 3 }, text: fc('A --> B --> C --> D') },
  { site: { maxEdges: 3 }, text: fc('A --> B --> C --> D --> E') },
  { site: { maxEdges: 0 }, text: fc('A --> B') },
  { site: { htmlLabels: false }, text: fc('A[<b>x</b>] --> B["<a href=\'https://x.example\' target=\'_blank\'>link</a>"]') },
  { site: { flowchart: { htmlLabels: false } }, text: fc('A[<b>x</b>] -->|<i>y</i>| B') },
  { site: { look: 'handDrawn', theme: 'forest' }, text: fc('subgraph S\nA --> B\nend') },
  { site: { flowchart: { inheritDir: true }, direction: 'RL' }, text: 'flowchart\nsubgraph S\nA --> B\nend\n' },
  { site: { flowchart: { padding: '20', curve: 'natural' } }, text: fc('A --> B') },
  { site: { layout: 'dagre' }, text: fc('A --> B') },
  { site: { layout: 'dagre' }, text: 'flowchart-elk TD\nA --> B\n' },
  { site: { arrowMarkerAbsolute: true }, text: fc('A --> B') },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: fc('A[<b>x</b> <i>y</i>] --> B') },
  { site: { markdownAutoWrap: false, flowchart: { wrappingWidth: 50 } }, text: fc('A["`long markdown text that wraps`"] --> B') },
  { site: { securityLevel: 'loose' }, text: fc('A --> B\nclick A call callback(\"quoted, arg\", plain , \" spaced \")\nclick B call callback(\")\nclick A call a.b.c()\nclick missing call cb()') },
  { site: { securityLevel: 'loose' }, text: fc('A@{ shape: diamond, label: "<b>bold</b>" } --> B\nclick A,B callback "tip"\nclick A,B call two(1, 2)') },
  { site: { securityLevel: 'loose', htmlLabels: false }, text: fc('A["<a href=\'https://x.example\' target=\'_blank\'>link</a> <b>b</b>"] --> B') },
  { site: { flowchart: { defaultRenderer: 'elk' }, look: 'classic' }, text: fc('A --> B') },
  { site: { theme: 'neutral', themeVariables: { primaryColor: '#ff0000' } }, text: fc('A --> B\nclassDef foo fill:#00f\nclass A foo') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// Arguments for destructLink(str, startStr).
const linkEnds = ['-->', '---', '--x', '--o', '<-->', 'x--x', 'o--o', '==>', '===', '==x', '==o', '<==>', 'x==x', 'o==o', '-.->', '-.-', '-.-x', '-.-o', '<-.->', 'x-.-x', 'o-.-o', '~~~', '~~~~~', '---->', '------', '=====>', '-...->', '-....-', ' --> ', '  ---  ', '-->x', '>', '', 'x', '-', '--', 'o-->', 'x-->', '<--', '<---x', '.->', '-.....', '--..-->'];
const linkStarts = [null, '--', '==', '-.', '<--', 'x--', 'o--', '<==', 'x==', 'o==', '<-.', 'x-.', 'o-.', ' -- ', '', '~~', 'x', '-'];
const links = [];
for (const end of linkEnds) {
  for (const start of linkStarts) {
    links.push([end, start]);
  }
}

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'flowdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(
  argsFile,
  JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh: !!fresh, site })), links })
);
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'flowchart-db-page.js', '--args', argsFile, '--out', resultFile], {
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
const doc = {
  baseConfig: base.config,
  cases: golden,
  links: links.map((args, i) => ({ args, result: result.links[i] })),
  defaultStyle: result.defaultStyle,
};
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(doc), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
const dumped = golden.filter((r) => r.nodes).length;
console.log(`${outFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${dumped} with a db, ${errors} with an error`);
