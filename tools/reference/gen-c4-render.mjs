// Writes internal/diagrams/c4/testdata/render.json.gz: the SVG that real
// mermaid renders in Chromium for C4 diagrams that exercise the code that
// the fixtures of testdata/corpus/mermaid-e2e/c4 do not reach: the looks
// handDrawn and neo, the framed rectangle, shape keywords, wrapping, the
// text placements, font config, boundaries with sprites and colours, and
// render errors.
//
// The script runs c4-render-page.js with the eval command of the browser
// oracle, one time. The page script renders all cases in one page and resets
// the page state before each case. To prove that this gives the result of
// the reference procedure (one page for each diagram), the script also
// renders fixtures of the corpus and compares them with .parity/ref.
//
// Usage (in tools/reference): node gen-c4-render.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const family = 'testdata/corpus/mermaid-e2e/c4';
const refDir = path.join(repoRoot, '.parity/ref/mermaid-e2e/c4');
const outFile = path.join(repoRoot, 'internal/diagrams/c4/testdata/render.json.gz');

// The values of the oracle (oracle.mjs) and of the reference metadata.
const seed = 0x6d6d6467;
const randomBefore = 1;

const cases = [];
const add = (name, text, config) => cases.push(config ? { name, text, config } : { name, text });
const front = (config, body) => `---\nconfig:\n${config}\n---\n${body}`;

const elements = `C4Context
title All shapes
Person(p, "A person", "The description of the person")
System(s, "A system", "The description of the system")
SystemDb(d, "A database", "The description of the database")
SystemQueue_Ext(q, "A queue", "The description of the queue")
Enterprise_Boundary(b, "A boundary") {
  Container(c, "A container", "Go", "The description of the container")
}
BiRel(p, s, "Uses", "HTTPS")
Rel(s, d, "Reads and writes")
Rel_Back(q, c, "Sends to")
UpdateElementStyle(c, $shape="component")`;

add('look-handdrawn', front('  look: handDrawn', elements));
add('look-handdrawn-seed', front('  look: handDrawn\n  handDrawnSeed: 7', elements));
add('look-handdrawn-forest', front('  look: handDrawn\n  theme: forest', elements));
add('look-neo', front('  look: neo', elements));
add('look-classic-fr-rect', elements);
add('theme-dark', front('  theme: dark', elements));
add('loose', front('  securityLevel: loose', elements));
add('no-html-labels', front('  htmlLabels: false', elements));

add(
  'handdrawn-styles',
  front(
    '  look: handDrawn',
    `C4Container
Person(p, "Person")
Container(c, "Container", "T")
ContainerDb(d, "Db", "T")
ContainerQueue(q, "Queue", "T")
Component(f, "Framed", "T")
UpdateElementStyle(p, $bgColor="red", $borderColor="blue", $fontColor="yellow")
UpdateElementStyle(c, $bgColor="red", $borderColor="blue")
UpdateElementStyle(d, $bgColor="#ff0", $borderColor="rgb(1,2,3)")
UpdateElementStyle(q, $fontColor="black")
UpdateElementStyle(f, $shape="component", $bgColor="green")`
  )
);

add(
  'shape-keywords',
  `C4Container
title Shape keywords
Container(a, "person", "T", "d")
Container(b, "box", "T", "d")
Container(c, "rounded", "T", "d")
Container(d, "cylinder", "T", "d")
Container(e, "database", "T", "d")
Container(f, "db", "T", "d")
Container(g, "queue", "T", "d")
Container(h, "pipe", "T", "d")
Container(i, "component", "T", "d")
Container(j, "unknown", "T", "d")
Person(k, "person as box", "d")
UpdateElementStyle(a, $shape="person")
UpdateElementStyle(b, $shape="box")
UpdateElementStyle(c, $shape="Rounded")
UpdateElementStyle(d, $shape="CYLINDER")
UpdateElementStyle(e, $shape="database")
UpdateElementStyle(f, $shape="db")
UpdateElementStyle(g, $shape="queue")
UpdateElementStyle(h, $shape="pipe")
UpdateElementStyle(i, $shape="component")
UpdateElementStyle(j, $shape="folder")
UpdateElementStyle(k, $shape="box")`
);
add(
  'sprite-and-tags-keywords',
  `C4Container
Container(a, "sprite db", "T", "d", $sprite="db")
Container(b, "sprite person", "T", "d", "person")
Container(c, "tags queue", "T", "d", $tags="v1, queue")
Container(d, "tags none", "T", "d", $tags="v1, v2")
ContainerDb(e, "shape wins", "T", "d", $sprite="queue", $tags="person")
Container(f, "shape and sprite", "T", "d", $sprite="queue")
UpdateElementStyle(f, $shape="nothing")
UpdateElementStyle(e, $shape="box")`
);
add(
  'element-style-colors',
  `C4Context
Person(a, "A", "d")
Person_Ext(b, "B", "d")
System(c, "C", "d")
SystemDb_Ext(d, "D", "d")
SystemQueue(e, "E", "d")
UpdateElementStyle(a, $bgColor="red")
UpdateElementStyle(b, $borderColor="#00f")
UpdateElementStyle(c, $fontColor="rgb(0, 0, 0)")
UpdateElementStyle(d, "pink", "black", "green")
UpdateElementStyle(e, $bgColor="url(#x)", $fontColor="a:b")`
);
add(
  'labels',
  `C4Context
Person(a, "Line one<br/>line two", "First<br>second<br />third")
System(b, "", "Only a description")
System(c, "Only a label")
System(d, "A #lt;tag#gt; #amp; an entity", "x #35; y")
System(e, "Averyveryveryveryveryveryverylongwordthatcannotbreak", "another_very_very_very_very_very_long_identifier_name with words")
System(f, "<b>bold</b> and <script>alert(1)</script>", "a < b > c")
System(g, "Ünïcödé çà", "Łódź naïve façade")
Container(h, "Techn only", "A very long technology name that needs more than one line")
Container(i, "Spaces   inside", "T", "  padded  description  ")`
);
// Text that needs fallback fonts: an emoji, CJK characters, a combining
// mark and an emoji sequence with zero width joiners.
add(
  'labels-fallback-fonts',
  `C4Context
System(a, "Smile \u{1F600} \u4E2D\u6587\u5B57\u7B26", "e\u0301 combining \u{1F468}\u200D\u{1F469}\u200D\u{1F467}")
System(b, "\u4E2D\u6587\u5B57\u7B26\u4E2D\u6587\u5B57\u7B26\u4E2D\u6587\u5B57\u7B26\u4E2D\u6587\u5B57\u7B26\u4E2D\u6587\u5B57\u7B26\u4E2D\u6587\u5B57\u7B26", "d")`
);
add(
  'labels-no-wrap',
  front(
    '  c4:\n    wrap: false',
    `C4Context
Person(a, "A person with a long label that does not wrap", "The description of the person is long and it does not wrap at all")
System(b, "B", "short")
SystemDb(c, "A database with a long label that does not wrap", "d")
SystemQueue(d, "A queue with a long label that does not wrap", "d")`
  )
);
add(
  'wrap-rels',
  front(
    '  wrap: true',
    `C4Context
title Wrapped texts
Person(a, "A")
System(b, "B")
System(c, "C")
Boundary(x, "A boundary with a long label that is wider than the content", "a long type of the boundary that is wide") {
  System(d, "D")
}
Rel(a, b, "A long label of a relationship with several words", "A long technology of the relationship")
Rel(b, c, "Averyveryveryveryveryverylongword second", "T")
Rel(c, d, "Line one<br/>line two", "T1<br/>T2")
Rel(a, d, "x")`
  )
);
add(
  'rels',
  `C4Context
Person(a, "A")
System(b, "B")
System(c, "C")
SystemDb(d, "D")
SystemQueue(e, "E")
Boundary(x, "X") {
  System(f, "F")
}
Rel(a, b, "first", "t1", "descr")
BiRel(b, c, "second")
Rel_Back(c, d, "third", "t3")
Rel(d, e, "fourth<br/>line", "t4<br/>line")
Rel(e, f, "fifth")
Rel(f, a, "sixth")
Rel(x, a, "from a boundary")
Rel(d, x, "to a boundary")
Rel(a, a, "self")
Rel(a, e, "")
Rel(b, e, "named", $techn="tech", $textColor="red", $lineColor="green", $offsetX="12", $offsetY="-7")
UpdateRelStyle(a, b, "blue", "orange", "10", "20")
UpdateRelStyle(b, c, $offsetX="0x10", $offsetY="3.9px")
UpdateRelStyle(c, d, "", "")`
);
add(
  'dynamic',
  `C4Dynamic
title Dynamic
Person(a, "A")
System(b, "B")
RelIndex(7, a, b, "indexed", "T")
Rel(b, a, "plain")
Rel(a, a, "")`
);
add(
  'boundaries',
  `C4Deployment
title Boundaries
Deployment_Node(n1, "Node one", "type one", "The description of node one") {
  Container(a, "A", "T")
  Node_L(n2, "Node two", "", "Only a description") {
    Container(b, "B", "T")
  }
  Node_R(n3, "", "type three") {
    Container(c, "C", "T")
  }
}
Boundary(b1, "With sprite", "kind", $sprite="users") {
  System(d, "D")
}
Enterprise_Boundary(b2, "Line one<br/>line two") {
  System(e, "E")
  System_Boundary(b3, "Inner", $type="inner type") {
    System(f, "F")
  }
}
Container_Boundary(b4, "Styled") {
  Container(g, "G", "T")
}
Boundary(b5, "Solid", $nodeType="x") {
  System(h, "H")
}
UpdateElementStyle(b4, $bgColor="#eee", $borderColor="red", $fontColor="blue")
UpdateElementStyle(n1, $bgColor="lightyellow")`
);
add(
  'layout-rows',
  `C4Context
System(a, "A")
System(b, "B")
System(c, "C")
Boundary(x, "X") {
  System(d, "D")
  System(e, "E")
  System(f, "F")
}
Boundary(y, "Y") {
  System(g, "G")
}
Boundary(z, "Z") {
  System(h, "H")
  Boundary(w, "W") {
    System(i, "I")
    System(j, "J")
  }
}
UpdateLayoutConfig($c4ShapeInRow="1", $c4BoundaryInRow="3")`
);
add(
  'layout-wide',
  `C4Context
System(a, "A")
System(b, "B")
System(c, "C")
System(d, "D")
System(e, "E")
System(f, "F")
System(g, "G")
Boundary(x, "X") {
  System(h, "H")
}
Boundary(y, "Y") {
  System(i, "I")
}
Boundary(z, "Z") {
  System(j, "J")
}
UpdateLayoutConfig("9", "9")`
);
add(
  'config-fonts',
  front(
    '  c4:\n    personFontSize: 20\n    personFontFamily: Georgia\n    personFontWeight: bold\n    systemFontSize: "18px"\n    boundaryFontSize: 20\n    boundaryFontFamily: "Courier New"\n    boundaryFontWeight: 700\n    messageFontSize: 16\n    messageFontFamily: Verdana\n    messageFontWeight: bold',
    `C4Context
Person(a, "A person", "With a description")
System(b, "A system", "With a description")
Boundary(x, "The boundary", "the type") {
  SystemDb(c, "C", "d")
}
Rel(a, b, "A label", "A technology")`
  )
);
add(
  'config-layout',
  front(
    '  c4:\n    diagramMarginX: 20\n    diagramMarginY: 30\n    c4ShapeMargin: 30\n    c4ShapePadding: 10\n    width: 150\n    height: 40\n    nextLinePaddingX: 15\n    useMaxWidth: false',
    `C4Context
title Layout config
Person(a, "A person", "With a description")
System(b, "A system", "With a description")
SystemDb(c, "C", "d")
SystemQueue(d, "D", "d")
System(e, "E", "d")
Boundary(x, "X") {
  System(f, "F", "d")
}
Rel(a, f, "label")`
  )
);
add(
  'config-colors',
  front(
    '  c4:\n    person_bg_color: "#111111"\n    person_border_color: red\n    system_bg_color: ""\n    external_system_border_color: ""',
    `C4Context
Person(a, "A")
System(b, "B")
System_Ext(c, "C")`
  )
);
add(
  'config-font-family',
  front(
    '  fontFamily: Georgia\n  fontSize: 20\n  c4:\n    fontFamily: Courier\n    fontSize: 18\n    fontWeight: bold',
    `C4Context
title Fonts of the c4 section
Person(a, "A person", "With a description")
System(b, "A system", "With a description")
Boundary(x, "The boundary") {
  System(c, "C")
}
Rel(a, b, "A label", "A technology")`
  )
);
add('text-placement-old', `C4Context\nPerson(a, "A")\nBoundary(x, "X", "t") {\n  System(b, "B")\n}\nRel(a, b, "label<br/>two", "techn")`, {
  c4: { textPlacement: 'old' },
});
add('text-placement-fo', `C4Context\nPerson(a, "A")\nBoundary(x, "X", "t") {\n  System(b, "B")\n}\nRel(a, b, "label<br/>two", "techn")`, {
  c4: { textPlacement: 'fo' },
});
add(
  'site-config',
  `C4Context\ntitle Site config\nPerson(a, "A", "d")\nSystemDb(b, "B", "d")\nRel(a, b, "label")`,
  { look: 'handDrawn', handDrawnSeed: 3, c4: { wrap: false, width: 300 }, themeVariables: { radius: 20 } }
);
add('accessibility', `C4Context\naccTitle: An acc title\naccDescr: An acc description\ntitle The title\nPerson(a, "A")`);
add('acc-descr-only', `C4Context\naccDescr {\n  A long\n  description\n}\nPerson(a, "A")`);
add('named-alias', `C4Context\nPerson($alias="x", "A")\nSystem(b, $link="l")`);
add('boundary-only-type', `C4Context\nBoundary(x, "", "") {\n  System(a, "A")\n}\nBoundary(y, "Y", "") {\n  System(b, "B")\n}`);
// Render errors.
add('error-rel-unknown', `C4Context\nPerson(a, "A")\nRel(a, zzz, "label")`);
add('error-rel-global', `C4Context\nPerson(a, "A")\nRel(a, global, "label")`);
add('error-rel-undrawn', `C4Context\nPerson(a, "A")\nBoundary(x, "X") {\n  Boundary(x, "X2") {\n    System(b, "B")\n  }\n}\nRel(a, b, "label")`);
add('error-shape-constructor', `C4Context\nPerson(a, "A")\nUpdateElementStyle(a, $shape="constructor")`);
add('error-shape-proto', `C4Context\nPerson(a, "A")\nUpdateElementStyle(a, $shape="__proto__")`);
add('error-type-string', `C4Context\nPerson(a, "A")\nUpdateElementStyle(a, $typeC4Shape="x")`);
add('error-rel-label-object', `C4Context\nPerson(a, "A")\nRel(a, a, $link="x")`);
add('error-boundary-type-string', `C4Context\nBoundary(x, "X") {\n  Person(a, "A")\n}\nUpdateElementStyle(x, $type="x")`);
add('error-parse', `C4Context\nPerson(a, "A"`);
add('element-label-string', `C4Context\nPerson(a, "A", "d")\nUpdateElementStyle(a, $label="x", $descr="y", $techn="z")`);
add('element-label-object', `C4Context\nPerson(a, $link="x")\nSystem(b, $0="zero")`);

// The fixtures that prove the procedure.
const proof = [
  'pr-check/c4-1-should-render-a-simple-c4context-diagram',
  'pr-check/component-variants',
  'nightly-check/c4-5-should-render-a-simple-c4deployment-diagram',
  'nightly-check/description-wrapping',
];
for (const name of proof) {
  add('proof/' + name, fs.readFileSync(path.join(repoRoot, family, name + '.mmd'), 'utf8'));
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'c4-render-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases, seed, randomBefore }));
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'c4-render-page.js'), '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

const golden = [];
cases.forEach((c, i) => {
  const r = results[i];
  if (r.name !== c.name) {
    throw new Error(`result ${i} is for ${r.name}, not ${c.name}`);
  }
  if (c.name.startsWith('proof/')) {
    const want = fs.readFileSync(path.join(refDir, c.name.slice('proof/'.length) + '.svg'), 'utf8');
    if (r.svg !== want) {
      throw new Error(`${c.name}: the page script does not give the SVG of the reference`);
    }
    return;
  }
  const entry = { name: c.name, text: c.text };
  if (c.config) {
    entry.config = c.config;
  }
  if (r.error !== undefined) {
    entry.error = r.error;
  } else {
    entry.type = r.type;
    entry.svg = r.svg;
  }
  golden.push(entry);
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden, null, 1) + '\n', { level: 9 }));
const errors = golden.filter((g) => g.error !== undefined);
console.log(`${outFile}: ${golden.length} cases, ${errors.length} errors, ${proof.length} proofs equal to the reference`);
for (const g of errors) {
  console.log(`  ${g.name}: ${g.error.split('\n')[0]}`);
}
