// Writes internal/diagrams/c4/testdata/db.json.gz: the content of the C4 db
// of real mermaid (shapes, boundaries, relationships, title, type, layout
// config, accessibility values) after it parses
//   - every fixture of testdata/corpus/mermaid-e2e/c4,
//   - hand-written inputs for the branches of c4Db.ts and of the grammar,
//   - diagrams that a seeded generator makes from all statements,
//   - seeded mutations of the fixtures.
// The db needs a browser (DOMPurify), so the script runs c4-db-page.js with
// the eval command of the browser oracle, one time.
//
// Usage (in tools/reference): node gen-c4-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const family = 'testdata/corpus/mermaid-e2e/c4';
const outFile = path.join(repoRoot, 'internal/diagrams/c4/testdata/db.json.gz');
const generatedCount = 260;
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

const cases = [];
const add = (name, text) => cases.push({ name, text });

// ---------------------------------------------------------------------------
// The fixtures of the corpus.

const fixtures = fs
  .readdirSync(path.join(repoRoot, family), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => family + '/' + f.split(path.sep).join('/'))
  .sort();
const fixtureTexts = fixtures.map((f) => fs.readFileSync(path.join(repoRoot, f), 'utf8'));
fixtures.forEach((f, i) => add(f, fixtureTexts[i]));

// ---------------------------------------------------------------------------
// Hand-written inputs.

const hand = {
  'types-context': 'C4Context\nPerson(a, "A")',
  'types-container': 'C4Container\nPerson(a, "A")',
  'types-component': 'C4Component\nPerson(a, "A")',
  'types-dynamic': 'C4Dynamic\nPerson(a, "A")',
  'types-deployment': 'C4Deployment\nPerson(a, "A")',
  'title-only': 'C4Context\ntitle Only a title',
  'title-twice': 'C4Context\ntitle First\ntitle Second\nPerson(a, "A")',
  'acc-title-is-title': 'C4Context\naccTitle: The acc title\nPerson(a, "A")',
  'acc-title-then-title': 'C4Context\naccTitle: The acc title\ntitle The title\nPerson(a, "A")',
  'acc-descr': 'C4Context\naccDescr: The description\nPerson(a, "A")',
  'acc-descr-multiline': 'C4Context\naccDescr {\n  line one\n   line two\n}\nPerson(a, "A")',
  'acc-description-keyword': 'C4Context\naccDescription The old keyword\nPerson(a, "A")',
  'title-sanitize': 'C4Context\ntitle A <b>bold</b> <script>alert(1)</script> title\nPerson(a, "A")',
  'title-entities': 'C4Context\ntitle A #lt;b#gt; and #35; title\nPerson(a, "A")',
  'comment-lines': 'C4Context\n%% a comment\nPerson(a, "A")\n%% another\nSystem(b, "B")',
  'person-all-positional': 'C4Context\nPerson(a, "Label", "Descr", "sprite1", "tag1", "http://link")',
  'person-ext': 'C4Context\nPerson_Ext(a, "Label", "Descr")',
  'person-unquoted': 'C4Context\nPerson(a, Label, "Descr")',
  'person-no-label': 'C4Context\nPerson(a)',
  'person-empty-label': 'C4Context\nPerson(a, "")',
  'person-empty-descr': 'C4Context\nPerson(a, "A", "")',
  'person-skipped-args': 'C4Context\nPerson(a, "A",, "sprite")',
  'person-leading-empty': 'C4Context\nPerson(, "A", "D")',
  'person-named-link': 'C4Context\nPerson(a, "A", "D", $link="https://example.com")',
  'person-named-tags': 'C4Context\nPerson(a, "A", "D", $tags="v1.0")',
  'person-named-sprite': 'C4Context\nPerson(a, "A", "D", $sprite="users")',
  'person-named-in-descr-slot': 'C4Context\nPerson(a, "A", $tags="t")',
  'person-named-descr': 'C4Context\nPerson(a, "A", $descr="The description")',
  'person-named-label-later': 'C4Context\nPerson(a, "A", "D", $label="Other label")',
  'person-named-type': 'C4Context\nPerson(a, "A", "D", $type="t")',
  'person-named-techn': 'C4Context\nPerson(a, "A", "D", $techn="t")',
  'person-named-unknown': 'C4Context\nPerson(a, "A", "D", $custom="value")',
  'person-named-alias': 'C4Context\nPerson($alias="x", "A")',
  'person-named-label': 'C4Context\nPerson(a, $label="x")',
  'person-named-many': 'C4Context\nPerson(a, "A", $descr="d", $sprite="s", $tags="t", $link="l")',
  'person-named-extra-ignored': 'C4Context\nPerson(a, "A", "D", "s", "t", "l", "seventh", $eighth="x")',
  'person-named-spaces': 'C4Context\nPerson(a, "A", "D", $link= "https://example.com")',
  'person-named-proto': 'C4Context\nPerson(a, "A", $__proto__="x")',
  'person-named-empty-key': 'C4Context\nPerson(a, "A", $="x")',
  'person-named-index-key': 'C4Context\nPerson(a, "A", $0="x", $1="y")',
  'person-duplicate': 'C4Context\nPerson(a, "First", "D1", "s1")\nPerson(a, "Second")',
  'person-duplicate-other-type': 'C4Context\nPerson(a, "First", "D1")\nSystem(a, "Second", $tags="t")',
  'person-trim': 'C4Context\nPerson( a ,  "  Label  " , "D" )',
  'system-variants': 'C4Context\nSystem(a, "A", "D")\nSystem_Ext(b, "B", "D")\nSystemDb(c, "C", "D")\nSystemDb_Ext(d, "D", "D")\nSystemQueue(e, "E", "D")\nSystemQueue_Ext(f, "F", "D")',
  'container-all-positional': 'C4Container\nContainer(a, "Label", "Techn", "Descr", "sprite1", "tag1", "http://link")',
  'container-variants': 'C4Container\nContainer(a, "A", "T", "D")\nContainer_Ext(b, "B", "T", "D")\nContainerDb(c, "C", "T", "D")\nContainerDb_Ext(d, "D", "T", "D")\nContainerQueue(e, "E", "T", "D")\nContainerQueue_Ext(f, "F", "T", "D")',
  'container-no-techn': 'C4Container\nContainer(a, "A")',
  'container-empty-techn': 'C4Container\nContainer(a, "A", "", "D")',
  'container-named-techn-slot': 'C4Container\nContainer(a, "A", $descr="d")',
  'container-named-descr-slot': 'C4Container\nContainer(a, "A", "T", $techn="t2")',
  'container-named-sprite': 'C4Container\nContainer(a, "A", "T", "D", $sprite="browser")',
  'container-duplicate': 'C4Container\nContainer(a, "A", "T", "D")\nContainer(a, "B")',
  'component-all-positional': 'C4Component\nComponent(a, "Label", "Techn", "Descr", "sprite1", "tag1", "http://link")',
  'component-variants': 'C4Component\nComponent(a, "A", "T", "D")\nComponent_Ext(b, "B", "T", "D")\nComponentDb(c, "C", "T", "D")\nComponentDb_Ext(d, "D", "T", "D")\nComponentQueue(e, "E", "T", "D")\nComponentQueue_Ext(f, "F", "T", "D")',
  'component-named': 'C4Component\nComponent(a, "A", $techn="t", $descr="d", $link="l")',
  'boundary-generic': 'C4Context\nBoundary(b, "B") {\n  Person(a, "A")\n}',
  'boundary-generic-type': 'C4Context\nBoundary(b, "B", "group") {\n  Person(a, "A")\n}',
  'boundary-generic-all': 'C4Context\nBoundary(b, "B", "group", "tags", "link") {\n  Person(a, "A")\n}',
  'boundary-generic-named-type': 'C4Context\nBoundary(b, "B", $type="named") {\n  Person(a, "A")\n}',
  'boundary-generic-named-tags': 'C4Context\nBoundary(b, "B", $tags="t") {\n  Person(a, "A")\n}',
  'boundary-enterprise': 'C4Context\nEnterprise_Boundary(b, "B") {\n  Person(a, "A")\n}',
  'boundary-enterprise-extra': 'C4Context\nEnterprise_Boundary(b, "B", "tags", "link") {\n  Person(a, "A")\n}',
  'boundary-enterprise-named-type': 'C4Context\nEnterprise_Boundary(b, "B", $type="named") {\n  Person(a, "A")\n}',
  'boundary-enterprise-alias-only': 'C4Context\nEnterprise_Boundary(b) {\n  Person(a, "A")\n}',
  'boundary-system': 'C4Context\nSystem_Boundary(b, "B") {\n  Person(a, "A")\n}',
  'boundary-system-named-link': 'C4Context\nSystem_Boundary(b, "B", $link="l") {\n  Person(a, "A")\n}',
  'boundary-container': 'C4Container\nContainer_Boundary(b, "B") {\n  Container(a, "A")\n}',
  'boundary-container-extra': 'C4Container\nContainer_Boundary(b, "B", "tags", "link") {\n  Container(a, "A")\n}',
  'boundary-brace-next-line': 'C4Context\nBoundary(b, "B")\n{\n  Person(a, "A")\n}',
  'boundary-nested': 'C4Context\nEnterprise_Boundary(e, "E") {\n  System_Boundary(s, "S") {\n    Person(a, "A")\n  }\n  Person(b, "B")\n}\nPerson(c, "C")',
  'boundary-siblings': 'C4Context\nBoundary(b1, "B1") {\n  Person(a, "A")\n}\nBoundary(b2, "B2") {\n  Person(b, "B")\n}\nPerson(c, "C")',
  'boundary-deep': 'C4Context\nBoundary(b1, "B1") {\nBoundary(b2, "B2") {\nBoundary(b3, "B3") {\nBoundary(b4, "B4") {\nPerson(a, "A")\n}\nPerson(b, "B")\n}\nPerson(c, "C")\n}\nPerson(d, "D")\n}\nPerson(e, "E")',
  'boundary-duplicate': 'C4Context\nBoundary(b, "First", "t1") {\n  Person(a, "A")\n}\nBoundary(b, "Second") {\n  Person(c, "C")\n}',
  'boundary-self-nested': 'C4Context\nBoundary(b, "First") {\n  Boundary(b, "Second") {\n    Person(a, "A")\n  }\n}',
  'boundary-named-global': 'C4Context\nBoundary(global, "G") {\n  Person(a, "A")\n}',
  'node': 'C4Deployment\nNode(n, "N") {\n  Container(a, "A")\n}',
  'node-all': 'C4Deployment\nDeployment_Node(n, "N", "type", "descr", "sprite", "tags", "link") {\n  Container(a, "A")\n}',
  'node-lr': 'C4Deployment\nNode_L(l, "L", "tl") {\n  Container(a, "A")\n}\nNode_R(r, "R", "tr", "dr") {\n  Container(b, "B")\n}',
  'node-named': 'C4Deployment\nNode(n, "N", $type="t", $descr="d", $sprite="s", $tags="g", $link="l") {\n  Container(a, "A")\n}',
  'node-named-in-type-slot': 'C4Deployment\nNode(n, "N", $descr="d") {\n  Container(a, "A")\n}',
  'node-named-nodetype': 'C4Deployment\nBoundary(n, "N", $nodeType="x") {\n  Container(a, "A")\n}',
  'node-duplicate': 'C4Deployment\nNode(n, "N", "t", "d") {\n  Container(a, "A")\n}\nNode(n, "N2") {\n  Container(b, "B")\n}',
  'rel': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "Label")',
  'rel-all': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "Label", "Techn", "Descr", "sprite", "tags", "link")',
  'rel-variants': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nPerson(c, "C")\nBiRel(a, b, "birel")\nRel_U(a, c, "u")\nRel_Up(b, c, "up")\nRel_D(c, a, "d")\nRel_Down(c, b, "down")\nRel_L(b, a, "l")\nRel_Left(a, a, "left")\nRel_R(b, b, "r")\nRel_Right(c, c, "right")\nRel_Back(c, d, "back")',
  'rel-index': 'C4Dynamic\nPerson(a, "A")\nPerson(b, "B")\nRelIndex(1, a, b, "Label", "Techn")',
  'rel-no-label': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b)',
  'rel-empty-label': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "")',
  'rel-duplicate': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "First", "T1", "D1", "s1")\nBiRel(a, b, "Second")',
  'rel-reverse-is-new': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "First")\nRel(b, a, "Second")',
  'rel-unknown-ends': 'C4Context\nRel(x, y, "Label")',
  'rel-named': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "Label", $techn="t", $descr="d", $sprite="s", $tags="g", $link="l")',
  'rel-named-in-techn-slot': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "Label", $tags="g")',
  'rel-named-type': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "Label", $type="t")',
  'rel-named-colors': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "Label", $textColor="red", $lineColor="blue", $offsetX="5", $offsetY="x")',
  'rel-named-label': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, $label="x")',
  'update-el-style-all': 'C4Context\nPerson(a, "A")\nUpdateElementStyle(a, "bg", "font", "border", "shadow", "shape", "sprite", "techn", "legendText", "legendSprite")',
  'update-el-style-named': 'C4Context\nPerson(a, "A")\nUpdateElementStyle(a, $bgColor="grey", $fontColor="red", $borderColor="blue")',
  'update-el-style-named-all': 'C4Context\nPerson(a, "A")\nUpdateElementStyle(a, $shadowing="true", $shape="cylinder", $sprite="s", $techn="t", $legendText="lt", $legendSprite="ls")',
  'update-el-style-named-text-fields': 'C4Context\nPerson(a, "A", "D")\nUpdateElementStyle(a, $label="L2", $descr="D2", $type="T2", $techn="X2")',
  'update-el-style-boundary': 'C4Context\nBoundary(b, "B") {\n  Person(a, "A")\n}\nUpdateElementStyle(b, $bgColor="grey", $fontColor="red", $borderColor="blue")',
  'update-el-style-global': 'C4Context\nPerson(a, "A")\nUpdateElementStyle(global, $bgColor="grey")',
  'update-el-style-unknown': 'C4Context\nPerson(a, "A")\nUpdateElementStyle(zzz, $bgColor="grey")',
  'update-el-style-before': 'C4Context\nUpdateElementStyle(a, $bgColor="grey")\nPerson(a, "A")',
  'update-el-style-empty-slots': 'C4Context\nPerson(a, "A")\nUpdateElementStyle(a, "", "font",, "shadow")',
  'update-rel-style-all': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(a, b, "red", "blue", "5", "-10")',
  'update-rel-style-named': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(a, b, $textColor="red", $lineColor="blue", $offsetX="5", $offsetY="-10")',
  'update-rel-style-named-shifted': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(a, b, $offsetY="-40")',
  'update-rel-style-offsets': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(a, b, "red", "blue", "0x10", "12.7px")',
  'update-rel-style-nan': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(a, b, "red", "blue", "abc", "")',
  'update-rel-style-named-offset-slot': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(a, b, "red", "blue", $textColor="7px", $offsetX="abc")',
  'update-rel-style-unknown': 'C4Context\nPerson(a, "A")\nPerson(b, "B")\nRel(a, b, "L")\nUpdateRelStyle(b, a, $textColor="red")',
  'update-layout-config': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig("3", "1")',
  'update-layout-config-named': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig($c4ShapeInRow="2", $c4BoundaryInRow="5")',
  'update-layout-config-named-swapped': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig($c4BoundaryInRow="5", $c4ShapeInRow="2")',
  'update-layout-config-one': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig("3")',
  'update-layout-config-zero': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig("0", "-1")',
  'update-layout-config-nan': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig("x", "")',
  'update-layout-config-float': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig("2.9", "0x3")',
  'update-layout-config-huge': 'C4Context\nPerson(a, "A")\nUpdateLayoutConfig("99999999999999999999999", "1e3")',
  'frontmatter-wrap': '---\nconfig:\n  wrap: true\n---\nC4Context\nPerson(a, "A")\nBoundary(b, "B") {\n  System(s, "S")\n}\nRel(a, s, "L")',
  'frontmatter-wrap-false': '---\nconfig:\n  wrap: false\n---\nC4Context\nPerson(a, "A")\nRel(a, a, "L")',
  'frontmatter-title': '---\ntitle: Front matter title\n---\nC4Context\nPerson(a, "A")',
  'frontmatter-loose': '---\nconfig:\n  securityLevel: loose\n---\nC4Context\ntitle A <b>bold</b> title\nPerson(a, "A <i>x</i>")',
  'frontmatter-no-html-labels': '---\nconfig:\n  htmlLabels: false\n---\nC4Context\ntitle A <b>bold</b> title\nPerson(a, "A")',
  'frontmatter-antiscript': '---\nconfig:\n  securityLevel: antiscript\n---\nC4Context\ntitle A <a href="x" target="_blank">link</a> title\nPerson(a, "A")',
  'directive-wrap': '%%{init: {"wrap": true}}%%\nC4Context\nPerson(a, "A")',
  'unicode': 'C4Context\ntitle Ünïcödé \u{1F600} title\nPerson(a, "Fränz \u{1F600}", "中文 description")',
  'entities-in-label': 'C4Context\nPerson(a, "A #amp; B #lt;x#gt;", "D #35;1")',
  'html-in-label': 'C4Context\nPerson(a, "A<br/>B", "D <b>bold</b>")',
  'unquoted-last': 'C4Context\nPerson(a, A)\nPerson(b, "B", "D")',
  'spaces-before-paren': 'C4Context\nPerson (a, "A")',
  'leading-blank-lines': '\n\n  C4Context\n\n\nPerson(a, "A")\n\n\n',
  'crlf': 'C4Context\r\ntitle T\r\nPerson(a, "A")\r\nSystem(b, "B")\r\n',
  'tabs': 'C4Context\n\tPerson(a, "A")\n\tSystem(b, "B")',
  'semicolon-title': 'C4Context\ntitle A; B\nPerson(a, "A")',
  'hash-title': 'C4Context\ntitle A # B\nPerson(a, "A")',
  // Errors.
  'error-empty': 'C4Context',
  'error-no-newline-statements': 'C4Context Person(a, "A")',
  'error-unclosed-boundary': 'C4Context\nBoundary(b, "B") {\n  Person(a, "A")',
  'error-extra-brace': 'C4Context\nPerson(a, "A")\n}',
  'error-empty-boundary': 'C4Context\nBoundary(b, "B") {\n}',
  'error-unknown-statement': 'C4Context\nFoo(a, "A")',
  'error-no-attributes': 'C4Context\nPerson()',
  'error-unclosed-paren': 'C4Context\nPerson(a, "A"',
  'error-direction': 'C4Context\ndirection TB\nPerson(a, "A")',
  'error-direction-first': 'direction LR',
  'error-lowercase': 'c4context\nPerson(a, "A")',
  'error-named-no-value': 'C4Context\nPerson(a, "A", $link)',
};
for (const [name, text] of Object.entries(hand)) {
  add('hand/' + name, text);
}

// ---------------------------------------------------------------------------
// Generated diagrams.

const rnd = prng(0xc4c4c4c4);
const pick = (list) => list[Math.floor(rnd() * list.length)];
const chance = (p) => rnd() < p;

const aliases = ['a', 'b', 'c', 'd', 'e', 'sys1', 'Alias_2', 'x-y', 'global', '1'];
const words = ['Alpha', 'Beta gamma', 'A longer label with several words', 'x', 'Ünï', 'a,b', 'tag1, db', 'queue', 'person', '#lt;b#gt;', '<br/>', '50', '-7', '0'];
const keys = ['link', 'tags', 'sprite', 'descr', 'techn', 'type', 'label', 'shape', 'bgColor', 'fontColor', 'borderColor', 'shadowing', 'legendText', 'legendSprite', 'textColor', 'lineColor', 'offsetX', 'offsetY', 'c4ShapeInRow', 'c4BoundaryInRow', 'nodeType', 'alias', 'wrap', 'custom', 'parentBoundary', 'typeC4Shape', 'x', 'width'];
const shapeMacros = [
  'Person', 'Person_Ext', 'System', 'System_Ext', 'SystemDb', 'SystemDb_Ext', 'SystemQueue', 'SystemQueue_Ext',
  'Container', 'Container_Ext', 'ContainerDb', 'ContainerDb_Ext', 'ContainerQueue', 'ContainerQueue_Ext',
  'Component', 'Component_Ext', 'ComponentDb', 'ComponentDb_Ext', 'ComponentQueue', 'ComponentQueue_Ext',
];
const boundaryMacros = ['Boundary', 'Enterprise_Boundary', 'System_Boundary', 'Container_Boundary', 'Deployment_Node', 'Node', 'Node_L', 'Node_R'];
const relMacros = ['Rel', 'BiRel', 'Rel_U', 'Rel_Up', 'Rel_D', 'Rel_Down', 'Rel_L', 'Rel_Left', 'Rel_R', 'Rel_Right', 'Rel_Back'];

// attribute makes one attribute that is not the last one of the list: an
// unquoted last attribute takes the text up to the next comma of the input.
function attribute(first) {
  const r = rnd();
  if (first) {
    return r < 0.9 ? pick(aliases) : `"${pick(aliases)}"`;
  }
  if (r < 0.55) {
    return `"${pick(words)}"`;
  }
  if (r < 0.8) {
    return `$${pick(keys)}="${pick(words)}"`;
  }
  if (r < 0.87) {
    return '""';
  }
  if (r < 0.92) {
    return '';
  }
  return pick(aliases);
}

function attributes(min, max) {
  const n = min + Math.floor(rnd() * (max - min + 1));
  const list = [];
  for (let i = 0; i < n; i++) {
    list.push(attribute(i === 0));
  }
  // The last attribute is quoted or named, so that the list ends at the ")".
  if (n > 1 && !/["]$/.test(list[n - 1])) {
    list[n - 1] = `"${pick(words)}"`;
  }
  if (n === 1 && !/["]$/.test(list[0])) {
    list[0] = `"${list[0]}"`;
  }
  return list.join(chance(0.8) ? ', ' : ',');
}

function statements(depth, lines, indent) {
  const n = 1 + Math.floor(rnd() * 5);
  for (let i = 0; i < n; i++) {
    const r = rnd();
    if (r < 0.45) {
      lines.push(`${indent}${pick(shapeMacros)}(${attributes(1, 8)})`);
    } else if (r < 0.6 && depth < 4) {
      const open = chance(0.85) ? ' {' : '\n' + indent + '{';
      lines.push(`${indent}${pick(boundaryMacros)}(${attributes(1, 8)})${open}`);
      statements(depth + 1, lines, indent + '  ');
      lines.push(`${indent}}`);
    } else if (r < 0.8) {
      lines.push(`${indent}${pick(relMacros)}(${pick(aliases)}, ${pick(aliases)}, ${attributes(1, 6)})`);
    } else if (r < 0.84) {
      lines.push(`${indent}RelIndex(${1 + Math.floor(rnd() * 9)}, ${pick(aliases)}, ${pick(aliases)}, ${attributes(1, 4)})`);
    } else if (r < 0.91) {
      lines.push(`${indent}UpdateElementStyle(${pick(aliases)}, ${attributes(1, 10)})`);
    } else if (r < 0.97) {
      lines.push(`${indent}UpdateRelStyle(${pick(aliases)}, ${pick(aliases)}, ${attributes(1, 5)})`);
    } else {
      lines.push(`${indent}UpdateLayoutConfig(${attributes(1, 3)})`);
    }
  }
}

for (let i = 0; i < generatedCount; i++) {
  const lines = [];
  if (chance(0.15)) {
    lines.push('---', 'config:', `  wrap: ${chance(0.5)}`, '---');
  }
  lines.push(pick(['C4Context', 'C4Container', 'C4Component', 'C4Dynamic', 'C4Deployment']));
  if (chance(0.5)) {
    lines.push(pick(['title A title', 'title  Two  spaces ', 'accTitle: An acc title', 'accDescr: An acc description', 'accDescr {\n multi\n line\n}']));
  }
  if (chance(0.15)) {
    lines.push('title Second title');
  }
  statements(0, lines, '');
  add(`generated/${i}`, lines.join('\n') + (chance(0.5) ? '\n' : ''));
}

// ---------------------------------------------------------------------------
// Mutations of the fixtures.

const specials = ['"', '$', '=', '(', ')', '{', '}', ',', ',,', '""', '\n', ' ', '%%', '#', ';', '<', '>', 'Rel', 'Person', 'title ', 'é', '\u{1F600}'];
function mutate(base) {
  let s = base;
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const p = Math.floor(rnd() * (s.length + 1));
    const kind = Math.floor(rnd() * 4);
    if (kind === 0) {
      s = s.slice(0, p) + s.slice(p + 1 + Math.floor(rnd() * 6));
    } else if (kind === 1) {
      s = s.slice(0, p) + pick(specials) + s.slice(p);
    } else if (kind === 2) {
      const q = Math.floor(rnd() * (s.length + 1));
      const [lo, hi] = p < q ? [p, q] : [q, p];
      s = s.slice(0, lo) + s.slice(lo, Math.min(hi, lo + 30)) + s.slice(lo);
    } else {
      const lines = s.split('\n');
      const a = Math.floor(rnd() * lines.length);
      const b = Math.floor(rnd() * lines.length);
      [lines[a], lines[b]] = [lines[b], lines[a]];
      s = lines.join('\n');
    }
  }
  return s;
}
fixtures.forEach((f, i) => {
  // Mutate the body only, so that the text stays a C4 diagram.
  const text = fixtureTexts[i];
  const end = text.startsWith('---') ? text.indexOf('\n---', 3) + 4 : 0;
  const headerEnd = text.indexOf('\n', text.indexOf('C4', end)) + 1;
  for (let m = 0; m < mutantsPerFixture; m++) {
    add(`${f}#mutant${m}`, text.slice(0, headerEnd) + mutate(text.slice(headerEnd)));
  }
});

// ---------------------------------------------------------------------------
// Run the page script in the browser oracle.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'c4-db-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(
  process.execPath,
  [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'c4-db-page.js'), '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

const golden = cases.map((c, i) => {
  const r = results[i];
  if (r.name !== c.name) {
    throw new Error(`result ${i} is for ${r.name}, not ${c.name}`);
  }
  const entry = { name: c.name, text: c.text };
  if (r.error !== undefined) {
    entry.error = r.error;
  } else {
    entry.type = r.type;
    entry.db = r.db;
  }
  return entry;
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden, null, 1) + '\n', { level: 9 }));
const errors = golden.filter((g) => g.error !== undefined).length;
const notC4 = golden.filter((g) => g.type !== undefined && g.type !== 'c4').length;
console.log(`${outFile}: ${golden.length} cases, ${errors} errors, ${notC4} of another diagram type`);
