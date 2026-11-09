// Writes internal/diagrams/class/testdata/db.json.gz: the content of the
// class db of real mermaid (the nodes and edges of getData, the classes, the
// relations, the notes, the namespaces, the interfaces, the style classes,
// the direction, the title, the accessibility values) and the config, at the
// moment when the renderer draws, for
//   - every fixture of testdata/corpus/mermaid-e2e/class-diagram,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of classDb.ts, classTypes.ts and
//     of the grammar.
// A case that mermaid rejects has the error message instead.
//
// The db needs a browser (DOMPurify), so the script runs class-db-page.js
// with the esm-eval command of the browser oracle, one time. A corpus
// fixture runs in a new page, as in the reference renders.
//
// Usage (in tools/reference): node gen-class-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpusDir = 'testdata/corpus/mermaid-e2e';
const family = 'class-diagram';
const outFile = path.join(repoRoot, 'internal/diagrams/class/testdata/db.json.gz');
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
// The fixtures of the corpus.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir, family), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => family + '/' + f.split(path.sep).join('/'))
  .sort();

const cases = [];
for (const f of fixtures) {
  cases.push({ name: `${corpusDir}/${f}`, corpus: true, fresh: true, text: fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8') });
}

// ---------------------------------------------------------------------------
// Seeded mutations.

const snippets = [
  'class Alpha',
  'class Beta~T~',
  'class Gamma["A label"]',
  'class Delta:::styled',
  'class Epsilon {\n  +int count\n  -String name$\n  #run(int n) bool*\n}',
  'class Zeta:::styled {\n  +go()\n}',
  'class Eta <<interface>>',
  'class Theta <<service>> {\n  +call() Result~T~\n}',
  'class `Quoted name`',
  'class `Quoted name`~V~',
  'class A.B.C',
  'class Shape~List~int~~',
  'Alpha <|-- Beta',
  'Alpha *-- Gamma : owns',
  'Alpha o-- Delta',
  'Alpha --> Epsilon',
  'Alpha -- Zeta',
  'Alpha ..> Eta',
  'Alpha ..|> Theta',
  'Alpha .. Beta',
  'Alpha <|--|> Beta',
  'Alpha "1" --> "many" Beta : has',
  'Alpha "1..*" *-- "0..1" Gamma',
  'Alpha ()-- Beta',
  'Alpha --() Beta',
  'Alpha ()..() Beta',
  'Alpha ()--* Beta',
  'Alpha : +int age',
  'Alpha : +isAlive() bool',
  'Alpha : -List~String~ names',
  'Alpha : #compute(List~int~ values, Map~K, V~ m) Map~K, V~$',
  'Alpha : makeAbstract()*',
  'Alpha : ~internal()',
  'Alpha : <<abstract>>',
  '<<interface>> Alpha',
  '<<enumeration>> Missing',
  'note "A free note"',
  'note for Alpha "A note\\nwith two lines"',
  'note for Missing "No such class"',
  'namespace Space {\n  class Inner\n  class Other {\n    +x\n  }\n}',
  'namespace Outer.Mid.Leaf {\n  class Deep\n}',
  'namespace Named["A namespace label"] {\n  class InNamed\n  note for InNamed "inside"\n}',
  'namespace N1 {\n  namespace N2 {\n    class Nested\n  }\n  class Sibling\n}',
  'direction LR',
  'direction TB',
  'direction RL',
  'direction BT',
  'classDef styled fill:#f9f,stroke:#333,stroke-width:4px',
  'classDef a,b color:#fff,fill:#00f',
  'cssClass "Alpha,Beta" styled',
  'cssClass "Alpha" a',
  'style Alpha fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5',
  'Alpha:::styled',
  'callback Alpha "onAlpha"',
  'callback Alpha "onAlpha" "A tooltip"',
  'link Alpha "https://example.com"',
  'link Alpha "https://example.com" "Link tip" _blank',
  'click Alpha call onAlpha()',
  'click Alpha call onAlpha("x", 2) "Call tip"',
  'click Alpha href "https://example.com/a"',
  'click Alpha href "https://example.com/a" "Href tip" _self',
  'accTitle: The title',
  'accDescr: The description',
  'accDescr {\n  multi\n  line\n}',
  '%% comment',
  'Alpha : #35; hash #lt;b#gt;',
  '}',
  '{',
];

// mutate changes the lines after the line of the diagram keyword.
function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its config can have a line that starts with "classDiagram".
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*classDiagram/.test(l));
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

const rnd = prng(0x636c6173);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const cd = (body, front = '') => front + 'classDiagram\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const extras = [
  // Headers.
  'classDiagram\nclass A\n',
  'classDiagram-v2\nclass A\n',
  '\n\n   classDiagram   \n\n\nclass A\n\n',
  'classDiagram\n',
  'classDiagram',
  'classDiagram\n\n',
  'classDiagram class A\n',
  'classDiagram;\nclass A\n',
  'classDiagram-v3\nclass A\n',
  'classDiagramX\nclass A\n',
  // Class statements.
  cd('class A\nclass B\nclass A\nclass C'),
  cd('class A~T~\nclass B~K, V~\nclass C~List~int~~\nclass D~~\nclass E~T'),
  cd('class A["Label of A"]\nclass B["<b>bold</b> label"]\nclass C[""]\nclass D~T~["Generic label"]\nclass E["`md **label**`"]'),
  cd('class A["first"]\nclass A["second"]\nclass A'),
  cd('class A:::foo\nclass B:::foo:::bar\nclass C:::foo {\n  +x\n}\nclassDef foo fill:#f00\nclassDef bar stroke:#0f0'),
  cd('class A {\n}\nclass B {\n\n}\nclass C { }\nclass D {}\nclass E{\n+x\n}'),
  cd('class A {\n  +int x\n  -String y\n  #List~int~ z\n  ~pkg w\n  plain\n}'),
  cd('class A {\n  +x()\n  -y(int a) void\n  #z(List~int~ a, Map~K,V~ b) List~T~\n  ~w()$\n  v()*\n  u(a, b) int$\n  t() int*\n}'),
  cd('class A {\n  <<interface>>\n  +x\n}\nclass B {\n  <<abstract>>\n}\nclass C {\n  +y\n  <<service>>\n  <<second>>\n}'),
  cd('class A <<interface>>\nclass B <<abstract>> {\n  +x()\n}\nclass C <<empty>> {\n}\nclass D <<a>> { }'),
  cd('class A {\n  +x : int\n  +y() : int\n  z(a: int, b: String) : Map~K, V~\n}'),
  cd('class A {\n  x$\n  y*\n  $\n  *\n  +\n  -\n  +$\n  ~\n  #*\n}'),
  cd('class A {\n   leading spaces\n\ttab\n  trailing   \n  +  spaced id\n  +  spaced()  ret  \n}'),
  cd('class A {\n  foo()bar\n  foo() bar\n  foo()$bar\n  foo()* bar\n  foo() $\n  foo() *\n  foo()$\n  foo()*\n  foo() bar$\n  foo() bar*\n  foo(a) List~T~$\n}'),
  cd('class A {\n  a(b(c))\n  a(b)(c)\n  (x)y()\n  -(x)\n  +a()\n  +()\n  a ()\n  a( )\n  a(  x  )  r  \n}'),
  cd('class A {\n  foo)\n}'),
  cd('class A {\n  (x)\n}'),
  cd('class A {\n  )x(\n}'),
  cd('class A {\n  x)\n}'),
  cd('A : foo)'),
  cd('class A {\n  a) b(c)\n}'),
  cd('class A {\n  +List~int~ x\n  +Map~K, V~ y\n  +f(List~int~ a) Map~K, V~\n  +g(Map~K, V~ a, List~Map~K, V~~ b)\n  ~T~ odd\n  a~b\n  ~a~b~c~\n}'),
  cd('class A {\n  <b>bold</b>\n  +<i>it</i>()\n  <script>alert(1)</script>\n  x <y> z\n  a & b\n  "quoted"\n  a = b\n}'),
  cd('class A {\n  #35; hash\n  #quot;q#quot;\n  +f() #lt;T#gt;\n  #9829; heart\n}'),
  cd('class A {\n  你好\n  +Ünï()\n  😀 emoji\n  +😀()\n  $😀\n}'),
  cd('class A {\n  <<interface>> extra\n  <<>>\n  << spaced >>\n  <<a>><<b>>\n}'),
  cd('class A {\n  +x\n  {\n}'),
  cd('class A {\n  +x'),
  cd('class A {\n  +x\n}}'),
  cd('class A { +x }'),
  cd('class A {+x\n-y}'),
  cd('class `A B`\nclass `A-B`\nclass `!@#`\nclass `A B`~T~\n`A B` --> `A-B`\n`A B` : +x'),
  cd('class A.B\nclass A.B.C\nclass A-B\nclass A_B\nclass 1A\nclass A1\nclass 123\nclass -\nclass A-\nclass -A'),
  cd('class Ünï\nclass 你好\nclass Привет\nÜnï --> 你好'),
  cd('class A B'),
  cd('class'),
  cd('class A~'),
  cd('class A["x"'),
  cd('class A[x]'),
  cd('class "A"'),
  cd('class A:::'),
  cd('class A::: foo'),
  cd('class A <<x'),
  cd('class A <<x y>>'),
  cd('class A <<>>'),
  // Member statements.
  cd('A : +int x\nA : -y()\nA : #z(a) T\nB : plain'),
  cd('A : x\nA: y\nA :z\nA:w\nA   :   v   '),
  cd('A : +x : int\nA : +f() : int'),
  cd('A : <<interface>>\nA : <<a>>\nA : << b >>\nA : <<>>'),
  cd('A~T~ : +x\nA : +y\n`Q W` : +z'),
  cd('A :\nA : '),
  cd('A : a;b'),
  cd('A : a:b'),
  cd('A'),
  cd('A\nB\nA.B\n`C D`'),
  cd('A~T~'),
  // Relations.
  cd('A <|-- B\nA --|> B\nA *-- B\nA --* B\nA o-- B\nA --o B\nA <-- B\nA --> B\nA -- B'),
  cd('A <|.. B\nA ..|> B\nA *.. B\nA ..* B\nA o.. B\nA ..o B\nA <.. B\nA ..> B\nA .. B'),
  cd('A <|--|> B\nA *--* B\nA o--o B\nA <--> B\nA <|--* B\nA o--> B\nA <..> B\nA *..o B'),
  cd('A ()-- B\nC --() D\nE ()..() F\nG ()--* H\nI *--() J\nK ()--o L\nM <|--() N\nO ()--> P\nQ ()--|> R\nS ()--() T'),
  cd('A ()-- B\nA ()-- B\nC ()-- B\nB --() A'),
  cd('A <|-- B : inherits\nA --> B : uses\nA -- B:nolabelspace\nA .. B :   padded   \nA --> B : with <b>html</b>\nA --> B : #35; hash'),
  cd('A "1" --> "*" B\nA "1" --> B\nA --> "*" B\nA "1..n" *-- "0..1" B : label\nA "" --> "" B\nA " " --> " " B\nA "none" --> "none" B'),
  cd('A "<b>1</b>" --> "<i>2</i>" B : x'),
  cd('A~T~ <|-- B~U~\nA~T~ --> C\nD --> A~T~ : gen'),
  cd('A.B --> C.D\nA-B --> C-D\n`A B` --> `C D` : q'),
  cd('A-->B\nA<|--B\nA--|>B\nA..>B\nA*--B\nA--oB\nAo--B'),
  cd('A --> A\nA --> A : self'),
  cd('A o-- B\nA o--B\nAo-- B\nA --oB'),
  cd('A <-- B --> C'),
  cd('A --> '),
  cd('--> B'),
  cd('A -> B'),
  cd('A --- B'),
  cd('A ... B'),
  cd('A --> B : '),
  cd('A --> B :'),
  cd('A "1 --> B'),
  // Namespaces.
  cd('namespace N {\n  class A\n  class B\n}'),
  cd('namespace N {\n  class A {\n    +x\n  }\n  class B~T~\n  class C["label"]\n  class D:::foo\n}'),
  cd('namespace N {\n}'),
  cd('namespace N { }'),
  cd('namespace N {\n  class A\n}\nnamespace M {\n  class B\n}\nA --> B'),
  cd('namespace N {\n  class A\n}\nnamespace N {\n  class B\n}'),
  cd('namespace N["Label"] {\n  class A\n}\nnamespace N["Second label"] {\n  class B\n}\nnamespace N {\n  class C\n}'),
  cd('namespace A.B.C {\n  class X\n}'),
  cd('namespace A.B.C {\n  class X\n}\nnamespace A.B {\n  class Y\n}\nnamespace A {\n  class Z\n}'),
  cd('namespace A {\n  class Z\n}\nnamespace A.B {\n  class Y\n}\nnamespace A.B.C["Leaf"] {\n  class X\n}'),
  cd('namespace Outer {\n  namespace Inner {\n    class A\n  }\n  class B\n}'),
  cd('namespace Outer {\n  class B\n  namespace Inner {\n    class A\n  }\n}'),
  cd('namespace Outer {\n  namespace Inner {\n    namespace Deep {\n      class A\n    }\n  }\n}'),
  cd('namespace Outer {\n  namespace Inner.Deep {\n    class A\n  }\n  namespace Inner {\n    class B\n  }\n}'),
  cd('namespace Outer["O"] {\n  namespace Inner["I"] {\n    class A\n  }\n}\nnamespace Outer.Inner {\n  class C\n}'),
  cd('namespace N {\n  class A\n  note "in namespace"\n  note for A "about A"\n}'),
  cd('namespace N {\n  note "only a note"\n}'),
  cd('namespace N {\n  class A\n}\nnote for A "outside"\nnote "free"'),
  cd('namespace N {\n  class A\n  A --> B\n}'),
  cd('namespace N {\n  A : +x\n}'),
  cd('namespace N {\n  class A'),
  cd('namespace N\n  class A\n'),
  cd('namespace {\n  class A\n}'),
  cd('namespace `Quoted N` {\n  class A\n}'),
  cd('namespace N-1 {\n  class A\n}\nnamespace 2N {\n  class B\n}\nnamespace Ünï {\n  class C\n}'),
  cd('namespace N {\n  class A~T~ {\n    +x\n  }\n  class `B C`\n}'),
  cd('class A\nnamespace N {\n  class A\n}'),
  cd('namespace N {\n  class A\n}\nnamespace M {\n  class A\n}'),
  cd('namespace N {\n  class A\n}', cfg('class:\n  hierarchicalNamespaces: false')),
  cd('namespace A.B.C {\n  class X\n  note for X "n"\n}\nnamespace A.D {\n  class Y\n}\nnamespace E {\n  namespace F.G {\n    class Z\n  }\n}', cfg('class:\n  hierarchicalNamespaces: false')),
  cd('namespace A.B.C {\n  class X\n}\nnamespace A {\n  class Y\n}', cfg('class:\n  hierarchicalNamespaces: false')),
  cd('namespace A.B.C["Leaf"] {\n  class X\n}', cfg('class:\n  hierarchicalNamespaces: true')),
  // Notes.
  cd('note "A note"'),
  cd('class A\nnote for A "About A"\nnote "Free"\nnote for A "Second about A"'),
  cd('note for A "Before the class"\nclass A'),
  cd('note for Missing "No class"'),
  cd('class A~T~\nnote for A~T~ "generic"\nnote for A "plain"'),
  cd('class A\nnote for A "line one\\nline two"\nnote "with <br/> break"\nnote "<b>bold</b> <script>x</script>"\nnote ""'),
  cd('class A\nnote for A "`md **bold**`"\nnote "#35; hash"'),
  cd('note'),
  cd('note for A'),
  cd('note for "x"'),
  cd('class `A B`\nnote for `A B` "quoted"'),
  // Direction.
  cd('direction TB\nclass A'),
  cd('direction BT\nclass A'),
  cd('direction RL\nclass A'),
  cd('direction LR\nclass A'),
  cd('direction LR\ndirection TB\nclass A'),
  cd('class A\n  direction   RL  \nclass B'),
  cd('direction XX\nclass A'),
  cd('direction\nclass A'),
  cd('class directionLR'),
  cd('A : direction LR'),
  // Styles.
  cd('class A\nclassDef foo fill:#f9f,stroke:#333,stroke-width:4px\ncssClass "A" foo'),
  cd('class A\nclass B\ncssClass "A,B" foo\nclassDef foo fill:red,color:white'),
  cd('class A\nclass B\nclassDef foo fill:red,color:white\ncssClass "A,B" foo'),
  cd('class A\nclassDef foo,bar fill:red\nclassDef baz color:blue,font-size:12px\ncssClass "A" foo\ncssClass "A" baz'),
  cd('class A:::foo\nclassDef foo fill:red\nclassDef foo stroke:blue\nclassDef default fill:#eee'),
  cd('class A\nstyle A fill:#f9f,stroke:#333,stroke-width:4px'),
  cd('class A\nstyle A fill:#bbf,stroke:#f66,stroke-width:2px,color:#fff,stroke-dasharray: 5 5'),
  cd('style A fill:red\nclass A'),
  cd('class A\nstyle Missing fill:red'),
  cd('class A~T~\nstyle A fill:red\ncssClass "A~T~" foo\ncssClass "A" bar'),
  cd('class 1A\ncssClass "1A" foo\nclass A\ncssClass "9" foo\ncssClass "" foo'),
  cd('class A\ncssClass "A, B" foo\ncssClass " A" bar'),
  cd('class A\nA:::foo'),
  cd('class A\nstyle A fill:#f9f;'),
  cd('class A\nstyle A font-family:Arial\\,sans-serif'),
  cd('class A\nclassDef foo color:red,fill:blue\nclassDef bar background-color:black\ncssClass "A" foo\ncssClass "A" bar'),
  cd('class A:::default\nclass B\nclassDef default stroke:red\nclassDef fa fill:green\nclass C:::fa'),
  cd('class A\nstyle A\n'),
  cd('classDef\n'),
  cd('classDef foo\n'),
  cd('cssClass A foo'),
  cd('cssClass "A"'),
  // Interaction.
  cd('class A\nclass B\ncallback A "cb"\ncallback B "cb" "Tooltip for B"'),
  cd('class A\nclass B\nlink A "https://example.com"\nlink B "https://example.com" "Tooltip"'),
  cd('class A\nclass B\nclass C\nclass D\nlink A "https://example.com" _blank\nlink B "https://example.com" "Tip" _self\nlink C "https://example.com" _parent\nlink D "https://example.com" "Tip" _top'),
  cd('class A\nclass B\nclass C\nclass D\nclick A call cb()\nclick B call cb() "Tip"\nclick C call cb("a", 2)\nclick D call cb("a, b", c) "Tip"'),
  cd('class A\nclass B\nclass C\nclass D\nclick A href "https://example.com"\nclick B href "https://example.com" _blank\nclick C href "https://example.com" "Tip"\nclick D href "https://example.com" "Tip" _parent'),
  cd('class A\nlink A "javascript:alert(1)"\nclass B\nlink B "  "\nclass C\nlink C "relative/path?a=b&c=d#frag"\nclass D\nlink D "data:text/html,x"'),
  cd('class A\ncallback A "cb" "first"\nlink A "https://example.com" "second"\nclick A call other() "third"'),
  cd('class A\nlink A "https://a.example"\ncallback A "cb"\nlink A "https://b.example" _self'),
  cd('callback Missing "cb" "tip"\nlink Missing "https://example.com"\nclick Missing call cb()'),
  cd('class A~T~\nlink A~T~ "https://example.com" "tip"\ncallback A "cb"'),
  cd('class 1A\nlink 1A "https://example.com"\ncallback 1A "cb"'),
  cd('class A\ncallback A "cb" "<b>bold</b> tip <script>x</script>"\nclass B\ncallback B "cb" ""'),
  cd('class A\nclick A call cb(\n)'),
  cd('class A\nclick A'),
  cd('class A\nclick A href'),
  cd('class A\nlink A'),
  cd('class A\ncallback A'),
  cd('namespace N {\n  class A\n}\nlink A "https://example.com" "ns tip"\ncallback A "cb"'),
  // Annotations.
  cd('class A\n<<interface>> A\n<<second>> A'),
  cd('<<interface>> A'),
  cd('class A~T~\n<<interface>> A~T~\n<<x>> A'),
  cd('class A\n<<Ünï>> A\n<<1>> A\n<<->> A'),
  cd('class A\n<<two words>> A'),
  // Accessibility and title.
  cd('accTitle: The accessible title\naccDescr: The accessible description\nclass A'),
  cd('accDescr {\n  A description\n  on several lines\n}\nclass A'),
  cd('accTitle:    padded   \naccDescr:   padded too  \nclass A'),
  cd('accTitle: <b>html</b> #35;\naccDescr: <script>x</script>y\nclass A'),
  cd('class A', '---\ntitle: The title\n---\n'),
  cd('class A', '---\ntitle: "<b>Title</b> with #35;"\n---\n'),
  // Comments and separators.
  cd('%% a comment\nclass A %% trailing\n%% another\nA --> B'),
  cd('class A;class B'),
  cd('class A\n\n\n\nclass B\n   \n\t\nA --> B'),
  cd('A --> B\n[*] --> A'),
  cd('A : [*]'),
  // Entities.
  cd('class A["#35; hash"]\nA : #quot;x#quot;\nA --> B : #9829;\nnote "#lt;b#gt;"'),
  cd('class A#35;B'),
  // Config.
  cd('class A\nA --> B', cfg('look: handDrawn')),
  cd('class A\nA --> B', cfg('look: classic\ntheme: forest')),
  cd('class A\nA --> B', cfg('look: neo\ntheme: redux-color')),
  cd('class A\nA --> B', cfg('theme: dark')),
  cd('class A\nA --> B', cfg('theme: neutral')),
  cd('class A["<b>x</b>"] {\n  +<i>m</i>()\n}\nnote "<b>n</b>"', cfg('htmlLabels: false')),
  cd('class A["<b>x</b>"] {\n  +<i>m</i>()\n}\nnote "<b>n</b>"', cfg('htmlLabels: true')),
  cd('class A["<b>x</b>"] {\n  +<i>m</i>()\n}', cfg('flowchart:\n  htmlLabels: false')),
  cd('class A["<b>x</b>"]', cfg('class:\n  htmlLabels: false')),
  cd('class A\nnote "n"\nnamespace N {\n  class B\n}', cfg('class:\n  padding: 20')),
  cd('class A\nnote "n"\nnamespace N {\n  class B\n}', cfg('class:\n  padding: 0')),
  cd('class A', cfg('class:\n  hideEmptyMembersBox: true')),
  cd('class A', cfg('class:\n  nodeSpacing: 10\n  rankSpacing: 20\n  defaultRenderer: dagre-wrapper')),
  cd('class A\nA --> B', cfg('layout: dagre')),
  cd('class A\nA --> B', cfg('layout: elk')),
  cd('class A\nA --> B', cfg('layout: nosuchlayout')),
  cd('class A\nA --> B', '%%{init: {"class": {"padding": 30}, "look": "classic"}}%%\n'),
  cd('class A\nA --> B', '%%{init: {"arrowMarkerAbsolute": true}}%%\n'),
  cd('class A\nA --> B', cfg('arrowMarkerAbsolute: true')),
  cd('class A\ncallback A "cb"\nlink A "javascript:alert(1)"', cfg('securityLevel: loose')),
  cd('class A\nnote "n"', cfg('themeVariables:\n  noteBkgColor: "#ff0000"\n  noteBorderColor: "#00ff00"')),
  // Statements with errors.
  cd('class A {'),
  cd('}'),
  cd('{'),
  cd('class A }'),
  cd('A <|-- '),
  cd('A : '),
  cd('"str"'),
  cd('A "1"'),
  cd('<<x>>'),
  cd('<<x'),
  cd('x>>'),
  cd('[x]'),
  cd('A ~T'),
  cd('`unterminated'),
  cd('"unterminated'),
  cd('class A["unterminated'),
  cd('accDescr {\n  unterminated'),
  cd('click A call cb('),
];
extras.forEach((text, i) => cases.push({ name: `extra${i}`, text }));

// Inputs that need a site config run in their own page.
const sited = [
  { site: { securityLevel: 'loose' }, text: cd('class A\nclass B\nclass C\ncallback A "cb" "Tip"\nclick B call cb("x", "y, z", 3) "Tip B"\nclick C call noargs()\nlink A "javascript:alert(1)"\nlink B "https://example.com" _blank') },
  { site: { securityLevel: 'loose' }, text: cd('class A["<b>bold</b> <script>x</script>"] {\n  +<i>m</i>() <b>T</b>\n}\ncallback A "cb" "<b>tip</b>"\nnote "<a href=\'https://x.example\' target=\'_blank\'>link</a>"') },
  { site: { securityLevel: 'loose' }, text: cd('class A~T~\nclick A~T~ call cb()\nclick Missing call cb()\nclick A call cb(  " a " , "b" )') },
  { site: { securityLevel: 'antiscript' }, text: cd('class A["<b>bold</b> <script>x</script>"]\ncallback A "cb"\nlink A "javascript:alert(1)"') },
  { site: { securityLevel: 'sandbox' }, text: cd('class A\nlink A "https://example.com" _blank\ncallback A "cb"') },
  { site: { securityLevel: 'other' }, text: cd('class A["<b>bold</b> x=y"] {\n  +a<br/>b\n}') },
  { site: { htmlLabels: false }, text: cd('class A["<b>x</b>"] {\n  +<i>m</i>()\n}\nnote "<a href=\'https://x.example\' target=\'_blank\'>link</a>"') },
  { site: { class: { htmlLabels: false } }, text: cd('class A["<b>x</b>"]') },
  { site: { look: 'handDrawn', theme: 'forest' }, text: cd('namespace N {\n  class A\n}\nA --> B\nnote "n"') },
  { site: { class: { padding: '20' } }, text: cd('class A\nnote "n"\nnamespace N {\n  class B\n}') },
  { site: { class: { hierarchicalNamespaces: false } }, text: cd('namespace A.B {\n  class X\n}') },
  { site: { layout: 'elk' }, text: cd('class A\nA --> B') },
  { site: { arrowMarkerAbsolute: true }, text: cd('class A\nA --> B') },
  { site: { dompurifyConfig: { FORBID_TAGS: ['b'] } }, text: cd('class A["<b>x</b> <i>y</i>"] {\n  +<b>m</b>\n}') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site, fresh: true }));

// ---------------------------------------------------------------------------
// Run the oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'classdb-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases: cases.map(({ name, text, fresh, site }) => ({ name, text, fresh: !!fresh, site })) }));
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'class-db-page.js', '--args', argsFile, '--out', resultFile], {
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
const other = golden.length - errors - dumped;
console.log(`${outFile}: ${golden.length} cases (${fixtures.length} corpus fixtures), ${dumped} with a db, ${errors} with an error, ${other} of another type`);
