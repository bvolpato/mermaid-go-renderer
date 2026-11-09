// Writes internal/diagrams/usecase/testdata/parser.json.gz: what the real
// lexer, parser, CST visitor and model builder of the usecase diagram of
// mermaid 12.2.0 make of a diagram text, in plain node:
//   - the tokens of the Chevrotain lexer, or its first error,
//   - the model that the builder commits to the db (the actors, the use
//     cases, the system boundaries, the relationships, the notes, the JSON
//     nodes, the class definitions, the symbols, the direction, the counters
//     and the accessibility texts), or the message of the error that
//     parser.parse throws,
// for the fixtures of the corpus, hand-written inputs for the branches of the
// grammar and of the builder, and seeded mutations. The file also has the
// results of JSON.parse of V8 and of parseOrderedJsonObject for JSON texts,
// valid and not valid: the error of a JSON block quotes the message of V8.
//
// db.commit is replaced by a function that keeps the model: the real commit
// sanitizes the accessibility texts with DOMPurify, which needs a browser.
// gen-usecase-db.mjs covers that part with the browser oracle.
//
// Usage (in tools/reference): node gen-usecase-parser.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { mulberry32, usecase } from './usecase-lib.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const corpusDir = path.join(repoRoot, 'testdata/corpus/mermaid-e2e/usecase');
const outFile = path.join(repoRoot, 'internal/diagrams/usecase/testdata/parser.json.gz');

const { lexer, db, parserDefinition, tokens: vocabulary, parseOrderedJsonObject } = await usecase();
const typeIndex = new Map(vocabulary.map((t, i) => [t.name, i]));

// ---------------------------------------------------------------------------
// The dump of one text.

// ordered keeps the property order of an object: { $o: [[key, value], ...] }.
// A number that JSON cannot hold is { $n: text }.
const ordered = (v) => {
  if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
    return { $n: Object.is(v, -0) ? '-0' : String(v) };
  }
  if (v === null || typeof v !== 'object') {
    return v;
  }
  if (Array.isArray(v)) {
    return v.map(ordered);
  }
  return { $o: Object.keys(v).map((k) => [k, ordered(v[k])]) };
};

const dumpModel = (m) => ({
  actors: [...m.actors].map(([key, a]) => [
    key,
    a.id,
    a.label,
    a.labelType,
    a.type,
    a.icon ?? null,
    a.business,
    a.stereotype ?? null,
    a.parentId ?? null,
    a.classes,
    a.styles,
  ]),
  useCases: [...m.useCases].map(([key, u]) => [
    key,
    u.id,
    u.label,
    u.labelType,
    u.shape,
    u.business,
    u.stereotype ?? null,
    u.parentId ?? null,
    u.classes,
    u.styles,
  ]),
  systemBoundaries: [...m.systemBoundaries].map(([key, b]) => [key, b.id, b.label, b.labelType, b.type, b.members, b.classes, b.styles]),
  relationships: m.relationships.map((r) => [
    r.id,
    r.explicitId,
    r.source,
    r.target,
    r.type,
    r.arrowType,
    r.label ?? null,
    r.labelType ?? null,
    r.minlen,
    r.classes,
    r.styles,
    r.animate,
    r.animation ?? null,
  ]),
  notes: [...m.notes].map(([key, n]) => [key, n.id, n.target, n.label, n.labelType]),
  jsonNodes: [...m.jsonNodes].map(([key, j]) => [key, j.id, ordered(j.value), ordered(j.propertyOrder), j.classes, j.styles]),
  classDefs: [...m.classDefs].map(([key, c]) => [key, c.id, c.styles]),
  symbols: [...m.symbols],
  direction: m.direction,
  relationshipCounter: m.relationshipCounter,
  noteCounter: m.noteCounter,
  accTitle: m.accTitle,
  accDescription: m.accDescription,
});

// keysAgree reports whether the property names of every object of a parsed
// JSON value are the names that the property order collector of upstream
// read from the text. V8 14.6 (node 26) can give a property a wrong name:
// the parser guesses the next key from the shape of an object that it made
// before, and a key that is written with an escape (\u0041) can take the
// name of that guess. The result depends on the objects that the process
// made before, so a text where it happens is left out of the golden file.
const keysAgree = (value, propertyOrder, pointer) => {
  if (Array.isArray(value)) {
    return value.every((item, i) => keysAgree(item, propertyOrder, `${pointer}/${i}`));
  }
  if (value === null || typeof value !== 'object') {
    return true;
  }
  const order = propertyOrder[pointer];
  const keys = Object.keys(value);
  if (!order || order.length !== keys.length || !order.every((k) => Object.hasOwn(value, k))) {
    return false;
  }
  return keys.every((k) => keysAgree(value[k], propertyOrder, `${pointer}/${k.replaceAll('~', '~0').replaceAll('/', '~1')}`));
};
let inconsistent = 0;

let committed;
db.commit = (model) => {
  committed = [...model.jsonNodes.values()].every((j) => keysAgree(j.value, j.propertyOrder, '')) ? dumpModel(model) : 'inconsistent';
};

const run = async (text) => {
  const rec = { text };
  const lex = lexer.tokenize(text);
  if (lex.errors.length > 0) {
    const e = lex.errors[0];
    rec.lexError = [e.offset, e.line, e.column, e.length, e.message];
  } else {
    // A token is [type, start offset, length, line, column]. The image is
    // the text of the range.
    rec.tokens = lex.tokens.map((t) => {
      if (t.image !== text.slice(t.startOffset, t.endOffset + 1)) {
        throw new Error('the image of a token is not its source range');
      }
      return [typeIndex.get(t.tokenType.name), t.startOffset, t.image.length, t.startLine, t.startColumn];
    });
  }
  committed = undefined;
  try {
    await parserDefinition.parse(text);
    if (!committed) {
      throw new Error('the parser did not commit a model');
    }
    rec.model = committed;
  } catch (e) {
    rec.error = String(e?.message ?? e);
    if (e?.name && e.name !== 'Error') {
      rec.errorName = e.name;
    }
  }
  return rec;
};

// ---------------------------------------------------------------------------
// The inputs.

const rnd = mulberry32(0x75636173);
const pick = (list) => list[Math.floor(rnd() * list.length)];
const int = (n) => Math.floor(rnd() * n);

const texts = [];
const seen = new Set();
const add = (text) => {
  if (!seen.has(text)) {
    seen.add(text);
    texts.push(text);
  }
};

// The fixtures of the corpus, without the front matter, with the line break
// that mermaid appends.
const stripFrontMatter = (text) => text.replace(/^\s*---\n[\s\S]*?\n---\n/, '');
const corpus = fs
  .readdirSync(corpusDir, { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .sort()
  .map((f) => stripFrontMatter(fs.readFileSync(path.join(corpusDir, f), 'utf8')) + '\n');
corpus.forEach(add);

const uc = (...lines) => 'usecase-beta\n' + lines.join('\n') + '\n';

// Hand-written inputs: each statement form, each decision of the grammar and
// each branch of the model builder.
const hand = [
  'usecase-beta',
  'usecase-beta\n',
  'usecase-beta\n\n\n',
  '  usecase-beta  \n',
  'usecase-beta x\n',
  'usecase-betax\n',
  'usecase-beta-x\n',
  'usecase\n',
  '\nusecase-beta\n',
  '%% c\nusecase-beta\n',
  'usecase-beta %% c\n',
  'usecase-beta\r\nactor A\r\nA --> B\r\n',
  'usecase-beta\ractor A\rA --> B\r',
  uc('%% comment', '  %% indented comment', 'actor A %% not a comment'),
  uc('actor A'),
  uc('actor A, B, C'),
  uc('actor A,'),
  uc('actor A B'),
  uc('actor A(Alice Smith)'),
  uc('actor A("Alice")'),
  uc('actor A("`**Alice**`")'),
  uc('actor A()'),
  uc('actor A(a(b)c)'),
  uc('actor A(Alice'),
  uc('actor "Quoted Actor"'),
  uc("actor 'Single'"),
  uc('actor "`Markdown *actor*`"'),
  uc('actor A@{ type: hollow }'),
  uc('actor A@{ type: awesome, business: false }'),
  uc('actor A@{ icon: "fa:user" }'),
  uc('actor A@{ icon: user }'),
  uc('actor A@{ icon: "" }'),
  uc('actor A@{ icon: "fa:user", type: normal }'),
  uc('actor A@{ icon: "fa:user", type: hollow }'),
  uc('actor A@{ business: true }'),
  uc('actor A@{ business: true, type: awesome }'),
  uc('actor A@{ business: true, icon: "x" }'),
  uc('actor A@{ business: "true" }'),
  uc('actor A@{ type: square }'),
  uc('actor A@{ type: true }'),
  uc('actor A@{ color: red }'),
  uc('actor A@{}'),
  uc('actor A@{ }'),
  uc('actor A@{', '  type: hollow', '}'),
  uc('actor A@{', '  type: hollow,', '  business: true', '}'),
  uc('actor A@{', '  type: hollow', '  business: true', '}'),
  uc('actor A@{ type: hollow,, business: true }'),
  uc('actor A@{ type: hollow, }'),
  uc('actor A@{ type: hollow', ', business: true }'),
  uc('actor A@{', '', '  type: hollow', '', ',', '', '  business: true', '', '}'),
  uc('actor A@{ "type": "hollow" }'),
  uc('actor A@{ type hollow }'),
  uc('actor A@{ type: }'),
  uc('actor A@{ : hollow }'),
  uc('actor A@{ type: hollow'),
  uc('actor A <<Human>>'),
  uc('actor A << Human Being >>'),
  uc('actor A <<>>'),
  uc('actor A <<  >>'),
  uc('actor A <<Human'),
  uc('actor A <<Human>'),
  uc('actor A <<', 'Human>>'),
  uc('actor A <<a>>b>>'),
  uc('actor A <<Human>> <<Again>>'),
  uc('actor A:::one'),
  uc('actor A:::one,two'),
  uc('actor A:::one, two , three'),
  uc('actor A:::'),
  uc('actor A:::one,'),
  uc('actor A@{ type: hollow } <<S>>:::c1,c2'),
  uc('actor A <<S>>@{ type: hollow }'),
  uc('actor A:::c <<S>>'),
  uc('actor A("L")@{ type: hollow } <<S>>:::c, B@{ type: awesome }, "C c"'),
  uc('actor A --> B'),
  uc('actor A --> B(Label)'),
  uc('actor A, B --> C'),
  uc('actor A e1@--> B'),
  uc('actor A <<S>> --> B[Rect]'),
  uc('actor A', 'actor A'),
  uc('actor A(One)', 'actor A(Two)'),
  uc('actor A("One")', 'actor A("`One`")'),
  uc('actor A@{ type: hollow }', 'actor A@{ type: awesome }'),
  uc('actor A@{ type: hollow }', 'actor A@{ type: hollow }'),
  uc('actor A@{ icon: "a" }', 'actor A@{ icon: "b" }'),
  uc('actor A@{ business: true }', 'actor A@{ business: false }'),
  uc('actor A <<S>>', 'actor A <<T>>'),
  uc('actor A <<S>>', 'actor A <<S>>'),
  uc('actor A:::a', 'actor A:::b,a'),
  uc('actor A', 'A(Use case)'),
  uc('A(Use case)', 'actor A'),
  uc('actor "A B"', 'actor A_B'),
  uc('actor A_B', 'actor "A B"'),
  uc('actor "A B"', 'actor "A-B"'),
  uc('actor "A B"', '"A B"'),
  uc('A'),
  uc('A(Do something)'),
  uc('A[Do something]'),
  uc('A("Quoted")'),
  uc('A["Quoted"]'),
  uc('A("`**bold**`")'),
  uc('A(Do something'),
  uc('A[Do something)'),
  uc('A(Do [something])'),
  uc('A(x, y: z; w - v. 50% #fff a-b \\, ! ? * + / = ^ _ | ~ < > ` é ✓ 😀)'),
  uc('A(end)'),
  uc('A(actor note for json class style TD)'),
  uc('A(--)'),
  uc('A(-->)'),
  uc('A(a -- b)'),
  uc('A(a {b})'),
  uc('A(a "b" c)'),
  uc("A(it's)"),
  uc('A(1.5px 10 .5 1e3 0x10)'),
  uc('"Quoted use case"'),
  uc('"`Markdown use case`"'),
  uc('"Quoted"@{ business: true }'),
  uc('"Quoted" <<S>>:::c'),
  uc('A@{ business: true }'),
  uc('A(L)@{ business: true }'),
  uc('A[L]@{ business: true }'),
  uc('A(L)@{ business: true }', 'A@{ business: false }'),
  uc('A(L)@{ business: true }', 'A(L)@{ business: false }'),
  uc('A(L)@{ type: hollow }'),
  uc('A(L) <<Core>>'),
  uc('A(L) <<Core>>:::c1'),
  uc('A(L):::c1,c2'),
  uc('A(L)', 'A[L]'),
  uc('A(L)', 'A(M)'),
  uc('A(L)', 'A(L)'),
  uc('A(L)', 'A'),
  uc('A', 'A(L)'),
  uc('A --> B'),
  uc('A --> B', 'B --> C', 'C --> A'),
  uc('A --> A'),
  uc('A ---> B', 'A ----> C', 'A -----------> D'),
  uc('A <-- B', 'A <--- C', 'A <------ D'),
  uc('A -- B', 'A --- C', 'A ------ D'),
  uc('A --o B', 'A o-- B', 'A --x B', 'A x-- B'),
  uc('A --oB', 'Ao-- B', 'A --xB', 'Ax-- B', 'Ax--B', 'Ao--B'),
  uc('x--> B', 'o--> B', 'xo-- B'),
  uc('A ---o B'),
  uc('A ---x B'),
  uc('A o--- B'),
  uc('A <--> B'),
  uc('A <-- B --> C'),
  uc('A -> B'),
  uc('A - B'),
  uc('A --> B --> C'),
  uc('A -- label --> B'),
  uc('A -- "quoted label" --> B'),
  uc('A -- "`md **label**`" --> B'),
  uc('A -- two words --> B'),
  uc('A -- a, b: c --> B'),
  uc('A -- label ---> B'),
  uc('A -- label ----> B'),
  uc('A -- label -- B'),
  uc('A -- label --- B'),
  uc('A -- label --o B'),
  uc('A -- label --x B'),
  uc('A -- label <-- B'),
  uc('A -- label o-- B'),
  uc('A -- label'),
  uc('A -- label B'),
  uc('A --- label --> B'),
  uc('A -- (label) --> B'),
  uc('A -- "a" "b" --> B'),
  uc('A -- "" --> B'),
  uc('A -- end --> B'),
  uc('A <-- label -- B'),
  uc('A <-- "quoted" --- B'),
  uc('A <-- label --> B'),
  uc('A <--- label -- B'),
  uc('A <-- label'),
  uc('A o-- label -- B'),
  uc('A o-- label --- B'),
  uc('A o-- label --> B'),
  uc('A x-- label -- B'),
  uc('A x-- "q" -- B'),
  uc('A x-- label'),
  uc('A ..> : include B'),
  uc('A ..> : extend B'),
  uc('A ..> : INCLUDE B'),
  uc('A ..> : Extend B'),
  uc('A ..>: include B'),
  uc('A ..> :include B'),
  uc('A ..> include B'),
  uc('A ..> : includes B'),
  uc('A ..> : B'),
  uc('A ..> B'),
  uc('A .> : include B'),
  uc('A ...> : include B'),
  uc('A --|> B'),
  uc('A ---|> B'),
  uc('A --| > B'),
  uc('actor A', 'actor B', 'A --|> B'),
  uc('actor A', 'A --|> B'),
  uc('actor A', 'B --|> A'),
  uc('actor A', 'A ..> : include B'),
  uc('actor A', 'B ..> : extend A'),
  uc('A e1@--> B'),
  uc('A e1 @ --> B'),
  uc('A e1@ --> B'),
  uc('A e1@-- label --> B'),
  uc('A e1@..> : include B'),
  uc('A e1@--|> B'),
  uc('A e1@--> B', 'C e1@--> D'),
  uc('A e1@--> B', 'actor e1'),
  uc('actor e1', 'A e1@--> B'),
  uc('A A@--> B'),
  uc('A e1@ B'),
  uc('A @--> B'),
  uc('A e1@e2@--> B'),
  uc('A --> B(Label)'),
  uc('A --> B[Label]'),
  uc('A(Label) --> B'),
  uc('A(L1) --> B(L2)', 'A(L1) --> C'),
  uc('A(L1) --> B', 'A(L2) --> C'),
  uc('A --> B:::c'),
  uc('A:::c --> B'),
  uc('A(L):::c --> B(M):::d'),
  uc('A --> B(M):::d'),
  uc('A --> B@{ business: true }'),
  uc('A --> B <<S>>'),
  uc('A <<S>> --> B'),
  uc('"A a" --> "B b"'),
  uc('"A a" --> "`B b`"'),
  uc('"A a" --> "A-a"'),
  uc('"A a" -- "label" --> "B b"'),
  uc('A --> "B b"@{ business: true }'),
  uc('systemBoundary S', 'end'),
  uc('systemBoundary S', '  A', '  B(Label)', '  C[Rect]', 'end'),
  uc('systemBoundary S', '  actor X', '  actor Y, Z', 'end'),
  uc('systemBoundary S', '  actor X --> A', 'end'),
  uc('systemBoundary S', '  A --> B', 'end'),
  uc('systemBoundary S', '', '  %% c', '  A', '', 'end'),
  uc('systemBoundary S', '  A'),
  uc('systemBoundary S', '  A', 'end x'),
  uc('systemBoundary S A', 'end'),
  uc('systemBoundary', 'end'),
  uc('systemBoundary S(System Title)', '  A', 'end'),
  uc('systemBoundary S[System Title]', '  A', 'end'),
  uc('systemBoundary S("Quoted")', 'end'),
  uc('systemBoundary "Quoted Boundary"', '  A', 'end'),
  uc('systemBoundary "`Markdown **Boundary**`"', '  A', 'end'),
  uc('systemBoundary ""', '  A', 'end'),
  uc('systemBoundary ""', '  A', 'end', 'systemBoundary T', '  A', 'end'),
  uc('systemBoundary ""', '  A', '  A', 'end', 'systemBoundary T', '  A', 'end'),
  uc('systemBoundary S@{ type: package }', '  A', 'end'),
  uc('systemBoundary S@{ type: rect }', 'end'),
  uc('systemBoundary S@{ type: hollow }', 'end'),
  uc('systemBoundary S@{ type: package, type: rect }', 'end'),
  uc('systemBoundary S@{ business: true }', 'end'),
  uc('systemBoundary S:::c1,c2', 'end'),
  uc('systemBoundary S@{ type: package }:::c1', 'end'),
  uc('systemBoundary S:::c1@{ type: package }', 'end'),
  uc('systemBoundary S <<x>>', 'end'),
  uc('systemBoundary S', 'end', 'S@{ type: package }'),
  uc('S@{ type: package }', 'systemBoundary S', 'end'),
  uc('systemBoundary S', 'end', 'systemBoundary S', 'end'),
  uc('systemBoundary S(One)', 'end', 'systemBoundary S(Two)', 'end'),
  uc('systemBoundary S:::a', '  A', 'end', 'systemBoundary S:::b', '  B', 'end'),
  uc('systemBoundary S', '  A', 'end', 'systemBoundary T', '  A', 'end'),
  uc('systemBoundary S', '  A', 'end', 'systemBoundary T', '  B', 'end', 'A --> B'),
  uc('systemBoundary S', '  systemBoundary T', '  end', 'end'),
  uc('systemBoundary S', '  A', '  A', 'end'),
  uc('A', 'systemBoundary S', '  A', 'end'),
  uc('systemBoundary S', '  A', 'end', 'A(Label)'),
  uc('actor S', 'systemBoundary S', 'end'),
  uc('systemBoundary S', 'end', 'actor S'),
  uc('systemBoundary S', 'end', 'S --> A'),
  uc('systemBoundary S', 'end', 'A --> S'),
  uc('systemBoundary S', 'end', 'note for S hello'),
  uc('systemBoundary "A b"', 'end', 'systemBoundary "A-b"', 'end'),
  uc('systemBoundary "A b"', 'end', '"A b"@{ type: package }'),
  uc('systemBoundary S', '  note for A x', 'end'),
  uc('systemBoundary S', '  classDef a fill:red', 'end'),
  uc('systemBoundary S', '  json J@{}', 'end'),
  uc('systemBoundary S', '  A@{ business: true } <<x>>:::c', 'end'),
  uc('systemBoundary S', '  "Quoted in boundary"', '  "`md in boundary`"', 'end'),
  uc('systemBoundary end', 'end'),
  uc('systemBoundary S', '  end', 'end'),
  uc('systemBoundary S', '  endpoint', 'end'),
  uc('A', 'note for A hello world'),
  uc('A', 'note for A "quoted note"'),
  uc('A', 'note for A "`markdown **note**`"'),
  uc('actor A', 'note for A hello'),
  uc('A', 'note for A a', 'note for A b'),
  uc('note for A hello', 'A'),
  uc('note for A hello'),
  uc('A', 'note for A'),
  uc('A', 'note A hello'),
  uc('A', 'note for "A" hello'),
  uc('A', 'note for A (x)'),
  uc('A', 'note for A x --> y'),
  uc('A e1@--> B', 'note for e1 hello'),
  uc('json J@{}', 'note for J hello'),
  uc('A', 'note for A hello', 'note-0 --> A'),
  uc('A', 'note for A hello', 'style note-0 fill:red'),
  uc('json J@{}'),
  uc('json J@{ }'),
  uc('json J@{"a": 1}'),
  uc('json J @ {"a": 1}'),
  uc('json J@ {"a": 1}'),
  uc('json   J@{"a": 1}'),
  uc('json J@{', '  "a": 1,', '  "b": [1, 2, 3],', '  "c": {"d": null, "e": true}', '}'),
  uc('json J@{"a": 1}:::c1,c2'),
  uc('json J@{"a": 1} :::c1'),
  uc('json J@{"a": 1} x'),
  uc('json J@{"a": 1'),
  uc('json J@{"a": "}"}'),
  uc('json J@{"a": "\\"}"}'),
  uc('json J@{"a": {"b": {"c": {}}}}'),
  uc('json J@{"a": 1}}'),
  uc('json J@{a: 1}'),
  uc('json J@{"a": 1,}'),
  uc('json J@{"a": tru}'),
  uc('json J@{', '  "a": 1,', '  "b": x', '}'),
  uc('json J@{"position 3": x}'),
  uc('json J@{"line 2 column 5": x, "aaaaaaaaaaaaaaaaaaaaaaaaaa": 1}'),
  uc('json J@{"unexpected end": x, "aaaaaaaaaaaaaaaaaaaaaaaaaa": 1}'),
  uc('json J@{"end of JSON input": ?}'),
  uc('json J@[1]'),
  uc('json J'),
  uc('json J@'),
  uc('json'),
  uc('json J@{"a": 1}', 'json J@{"b": 2}'),
  uc('json J@{"a": 1}', 'actor J'),
  uc('actor J', 'json J@{"a": 1}'),
  uc('json J@{"a": 1}', 'A --> J', 'J --> A', 'A -- J', 'A <-- J'),
  uc('json J@{"a": 1}', 'A --o J'),
  uc('json J@{"a": 1}', 'J x-- A'),
  uc('json J@{"a": 1}', 'A ..> : include J'),
  uc('json J@{"a": 1}', 'A --|> J'),
  uc('json J@{"a": 1}', 'J@{ business: true }'),
  uc('json J@{"a": 1}', 'J@{}'),
  uc('json json@{"a": 1}'),
  uc('json o@{"a": 1}', 'json son@{"b": 2}'),
  uc('json 1@{"a": 1}'),
  uc('jsonJ@{"a": 1}'),
  uc('json J@{"2": "two", "1": "one", "a": {"y": 1, "x": 2}, "a": 3, "dup": {"k": 1}, "dup": {"l": 2}}'),
  uc('direction TB'),
  uc('direction TD'),
  uc('direction BT'),
  uc('direction LR'),
  uc('direction RL'),
  uc('direction TB', 'direction RL'),
  uc('direction'),
  uc('direction XX'),
  uc('direction tb'),
  uc('direction TB x'),
  uc('directionTB'),
  uc('TB'),
  uc('classDef a fill:#f00'),
  uc('classDef a fill:#f00,stroke:#333,stroke-width:2px'),
  uc('classDef a,b fill:#f00'),
  uc('classDef a, b , c fill:#f00'),
  uc('classDef a fill:#f00, stroke: #333 , stroke-width : 2px'),
  uc('classDef a border:1px solid red'),
  uc('classDef a font-family:"Open Sans"\\, sans-serif'),
  uc('classDef a font-family:Arial\\,Helvetica,color:red'),
  uc('classDef a --my-var:10px'),
  uc('classDef a --my-var:calc(100% - 10px)'),
  uc('classDef a -- x:1'),
  uc('classDef a transform:rotate(45deg) translate(1px, 2px)'),
  uc('classDef a fill:url(#grad)'),
  uc('classDef a content:[x]{y}@z'),
  uc('classDef a stroke-dasharray:5 5'),
  uc('classDef a fill:rgb(1,2,3)'),
  uc('classDef a fill:red;stroke:blue'),
  uc('classDef a fill'),
  uc('classDef a fill:'),
  uc('classDef a :red'),
  uc('classDef a'),
  uc('classDef'),
  uc('classDef a fill:red,'),
  uc('classDef a fill:red,,stroke:blue'),
  uc('classDef default fill:red', 'A'),
  uc('classDef a fill:red', 'classDef a stroke:blue'),
  uc('classDef a 1px:2'),
  uc('classDef a end:1'),
  uc('classDef a x:end'),
  uc('classDef a x:a.b%c!d-e--f'),
  uc('classDef a x:<y>'),
  uc('classDef a x:é'),
  uc('classDef a opacity:0.5,stroke-width:1.5px,margin:.5em'),
  uc('classDef a margin:-1px - 2px,width:calc(100% - 10px)'),
  uc('classDef a x:1.5 - .5 - a -b'),
  uc('A(L)', 'A@{ type: hollow }'),
  uc('A(L)', 'A@{ business: "yes" }'),
  uc('A(L)', 'A@{ business: true, icon: "x" }'),
  uc('json J@{"":line 1 column 1}'),
  uc('json J@{"":line 2 column 3}', 'A'),
  uc('  json J@{"":line 9 column 9}'),
  uc('json J@{"":unexpected end}'),
  uc('json J@{"":Unexpected End}'),
  uc('json J@{"":position 7}'),
  uc('json J@{"":position 9999999999999}'),
  uc('json J@{"k\\b\\f\\n\\r\\t\\/\\\\\\"": {"x": 1}, "\\u00e9\\u0041": [{"\\t": 2}]}'),
  uc('A', 'class A a'),
  uc('A', 'B', 'class A,B a'),
  uc('A', 'B', 'class A, B a, b'),
  uc('A', 'class A a b'),
  uc('A', 'class A'),
  uc('A', 'class A,'),
  uc('A', 'class A, a'),
  uc('A', 'class A a,'),
  uc('class A a'),
  uc('class A a', 'A'),
  uc('A e1@--> B', 'class e1 a'),
  uc('A --> B', 'class edge-0 a'),
  uc('systemBoundary S', 'end', 'class S a'),
  uc('json J@{}', 'class J a'),
  uc('A', 'class A a', 'class A a', 'class A b'),
  uc('A', 'style A fill:#f00'),
  uc('A', 'style A fill:#f00,stroke:#333'),
  uc('A', 'style A fill:#f00', 'style A stroke:#333'),
  uc('style A fill:#f00'),
  uc('A', 'style A'),
  uc('A', 'style A,B fill:red'),
  uc('actor A', 'style A fill:red', 'A@{ type: hollow }'),
  uc('A e1@--> B', 'style e1 stroke:red'),
  uc('systemBoundary S', 'end', 'style S fill:red'),
  uc('json J@{}', 'style J fill:red'),
  uc('A@{ business: true }'),
  uc('A', 'A@{ business: true }'),
  uc('A@{ business: true }', 'A'),
  uc('actor A', 'A@{ type: hollow }'),
  uc('actor A@{ type: hollow }', 'A@{ type: awesome }'),
  uc('actor A', 'A@{ icon: "x" }', 'A@{ type: hollow }'),
  uc('actor A', 'A@{ icon: "x", business: true }'),
  uc('A@{ type: hollow }'),
  uc('X@{ icon: "x" }'),
  uc('X@{ animate: true }'),
  uc('X@{ animation: fast }'),
  uc('X@{ type: rect }'),
  uc('X@{ type: hollow }'),
  uc('X@{ type: other }'),
  uc('X@{ business: true }'),
  uc('X@{ unknown: 1 }'),
  uc('X@{}'),
  uc('"Q q"@{ business: true }'),
  uc('"Q q"', '"Q q"@{ business: true }'),
  uc('"`Q q`"@{ business: true }'),
  uc('A e1@--> B', 'e1@{ animate: true }'),
  uc('A e1@--> B', 'e1@{ animate: false }'),
  uc('A e1@--> B', 'e1@{ animation: fast }'),
  uc('A e1@--> B', 'e1@{ animation: slow }'),
  uc('A e1@--> B', 'e1@{ animation: medium }'),
  uc('A e1@--> B', 'e1@{ animate: false, animation: slow }'),
  uc('A e1@--> B', 'e1@{ animation: slow, animate: false }'),
  uc('A e1@--> B', 'e1@{ animate: "true" }'),
  uc('A e1@--> B', 'e1@{ type: rect }'),
  uc('A e1@--> B', 'e1@{}'),
  uc('e1@{ animate: true }', 'A e1@--> B'),
  uc('A --> B', 'edge-0@{ animate: true }'),
  uc('accTitle: The title'),
  uc('accTitle:The title'),
  uc('accTitle : The title'),
  uc('  accTitle: indented'),
  uc('accTitle:'),
  uc('accTitle: a: b: c'),
  uc('accTitle: one', 'accTitle: two'),
  uc('accDescr: The description'),
  uc('accDescr { multi', 'line', 'description }'),
  uc('accDescr{x}'),
  uc('accDescr {', '}'),
  uc('accDescr { unclosed'),
  uc('accDescr { a } b'),
  uc('accDescr { a } }'),
  uc('accDescr: one', 'accDescr { two }'),
  uc('A accTitle: not at line start'),
  uc('accTitle'),
  uc('accDescr'),
  uc('acctitle: x'),
  uc('title My diagram'),
  uc('package P'),
  uc('Package P'),
  uc('rectangle R'),
  uc('skinparam x'),
  uc('allowmixing'),
  uc('newpage'),
  uc('package'),
  uc('packages'),
  uc('package --> A'),
  uc('A --> package'),
  uc('actor package'),
  uc('end'),
  uc('for'),
  uc('true'),
  uc('note'),
  uc('include'),
  uc('actor'),
  uc('actor actor'),
  uc('actor end'),
  uc('actors'),
  uc('Actor A'),
  uc('ACTOR A'),
  uc('A --> end'),
  uc('ends --> A'),
  uc('1 --> 2'),
  uc('1a --> 2b'),
  uc('1.5 --> 2'),
  uc('_a --> __'),
  uc('a-b --> c'),
  uc('a.b'),
  uc('é'),
  uc('A(é)'),
  uc('"unterminated'),
  uc("'unterminated"),
  uc('A("unterminated)'),
  uc('"`unterminated'),
  uc('A("`unterminated)', 'B'),
  uc('"`multi', 'line`"'),
  uc('A("`multi', 'line`") --> B'),
  uc('"multi', 'line"'),
  uc('A --> B;'),
  uc('A; B'),
  uc('A -->'),
  uc('--> B'),
  uc('A B'),
  uc('A, B'),
  uc('A --> B, C'),
  uc('{'),
  uc('}'),
  uc('('),
  uc('@{}'),
  uc('@'),
  uc(':::c'),
  uc('<<x>>'),
  uc('#fff'),
  uc('\\'),
  uc('\t'),
  uc('A\t-->\tB'),
  uc('A\u00a0--> B'),
  uc('A \u3000 B'),
  uc('😀 --> A'),
  uc('A(😀 emoji)'),
  uc('"😀 emoji"'),
  uc('actor "😀"', 'actor "__"'),
  uc('actor "😀 a"', '"😀 a" --> B'),
];
hand.forEach(add);

// ---------------------------------------------------------------------------
// Seeded mutations.

// The pool of statement lines: the lines of the corpus and of the
// hand-written inputs. validPool has the lines of the inputs that upstream
// accepts: most of the composed texts take their lines from it, so that
// many of them have a model.
const lineSplit = /\r\n|\n|\r/;
const pool = [];
const validPool = [];
for (const text of [...corpus, ...hand]) {
  const lines = text.split(lineSplit).filter((line) => line.trim() !== '' && line.trim() !== 'usecase-beta');
  pool.push(...lines);
  if ((await run(text)).model && !/systemBoundary|\bend\b|[{}]|"`/.test(text)) {
    validPool.push(...lines);
  }
}
// The names of the valid lines are changed at random, so that two lines
// declare the same element less often.
const names = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'User', 'Admin', 'Login', 'Pay'];
const rename = (line) => line.replace(/\b[A-C]\b/g, () => pick(names));

const compose = () => {
  const lines = ['usecase-beta'];
  const count = 1 + int(8);
  const strict = rnd() < 0.7;
  const line = () => (strict || rnd() < 0.6 ? rename(pick(validPool)) : pick(pool)).trim();
  for (let i = 0; i < count; i++) {
    const r = rnd();
    if (r < 0.08) {
      lines.push(`systemBoundary ${pick(['S', 'T', '"Sys tem"', 'S(Title)'])}`);
      const inner = int(4);
      for (let j = 0; j < inner; j++) {
        lines.push('  ' + pick(['X', 'Y(In boundary)', 'actor Z', 'W[Rect]', 'actor V@{ type: hollow }', '"Quoted member"', line()]));
      }
      lines.push('end');
    } else {
      lines.push(pick(['', '  ', '    ', '\t']) + line());
    }
  }
  return lines.join(pick(['\n', '\n', '\n', '\r\n'])) + '\n';
};

const alphabet = [
  ...'"\'`<>-.:@{}[](),#%\\|&!?*+/=^_~; \n\t',
  'x', 'o', 'A', '1', '0', 'é', '😀', '--', '-->', '<--', '..>', ':::', '<<', '>>', '@{', '\r\n', ' end ', ' actor ', '%%', 'json ', ' for ', 'true', 'TB',
];
// mutate changes a text at random. Most of the time it keeps the first
// line, the keyword of the diagram: a text without it fails at once.
const mutate = (text) => {
  const headerEnd = rnd() < 0.9 ? text.indexOf('\n') + 1 : 0;
  const header = text.slice(0, headerEnd);
  let out = text.slice(headerEnd);
  const edits = 1 + int(3);
  for (let i = 0; i < edits; i++) {
    const at = int(out.length + 1);
    const r = rnd();
    if (r < 0.3) {
      out = out.slice(0, at) + out.slice(at + 1 + int(3));
    } else if (r < 0.65) {
      out = out.slice(0, at) + pick(alphabet) + out.slice(at);
    } else if (r < 0.8) {
      const len = 1 + int(4);
      out = out.slice(0, at) + out.slice(at, at + len) + out.slice(at);
    } else {
      const lines = out.split('\n');
      const a = int(lines.length);
      const b = int(lines.length);
      [lines[a], lines[b]] = [lines[b], lines[a]];
      out = lines.join('\n');
    }
  }
  return header + out;
};

const words = [
  'usecase-beta', 'actor', 'systemBoundary', 'end', 'direction', 'TD', 'TB', 'BT', 'LR', 'RL', 'note', 'for', 'json', 'classDef', 'class', 'style',
  'include', 'extend', 'true', 'false', '--|>', '..>', '<<', '>>', ':::', '-->', '--->', '<--', '<---', '--o', 'o--', '--x', 'x--', '--', '---', '@{', '@',
  '{', '}', '[', ']', '(', ')', ',', ':', '#f00', '"str"', "'str'", '"`md`"', 'stroke-width', '--var', 'A', 'B', 'e1', '1', '1.5px', '\\,', '-', '.', '%',
  '!', ';', 'é', '\n', '\n', '\n', ' ', ' ', 'type', 'hollow', 'business', 'animate', 'fill', 'red', 'J@{"a":1}', 'accTitle: t', 'accDescr { d }', '%% c',
];
const soup = () => {
  let out = 'usecase-beta\n';
  const count = 2 + int(14);
  for (let i = 0; i < count; i++) {
    out += pick(words) + pick(['', ' ', ' ', ' ']);
  }
  return out + '\n';
};

for (let i = 0; i < 900; i++) {
  add(compose());
}
for (let i = 0; i < 600; i++) {
  add(mutate(compose()));
}
for (const text of corpus) {
  for (let i = 0; i < 8; i++) {
    add(mutate(text));
  }
}
for (const text of hand) {
  for (let i = 0; i < 2; i++) {
    add(mutate(text));
  }
}
for (let i = 0; i < 450; i++) {
  add(soup());
}

// ---------------------------------------------------------------------------
// JSON texts: JSON.parse of V8 and parseOrderedJsonObject.

const jsonTexts = [];
const jsonSeen = new Set(['NaN', 'Infinity', 'undefined', '[object Object]']);
const addJson = (text) => {
  if (!jsonSeen.has(text)) {
    jsonSeen.add(text);
    jsonTexts.push(text);
  }
};
[
  '{', '{"a"', '{"a":', '{"a":1', '{"a":1,', '{"a":1,}', '{,}', '{"a" 1}', '{a:1}', "{'a':1}", '{"a":-}', '{"a":-x}', '{"a":01}', '{"a":1.}',
  '{"a":1.x}', '{"a":1e}', '{"a":1e+}', '{"a":1ex}', '{"a":tru}', '{"a":trux}', '{"a":t}', '{"a":nul}', '{"a":fals1}', '{"a":tr"}', '{"a":tr1}',
  '{"a":tr u}', '{"a":[1,2}', '{"a":[1 2]}', '{"a":[,]}', '{"a":[1,]}', '{"a":"x\ny"}', '{"a":"x\\qy"}', '{"a":"x\\u12g4"}', '{"a":"\\u12"}',
  '{"a":"\\', '{"a":"abc', '{"a":"\t"}', '{"a":1} x', '{"a":1}}', '{"a":}', '{"a"::1}', '{"a":1 "b":2}', '{"a":1,,"b":2}', '{"a":.5}', '{"a":+1}',
  '{"a":0x10}', '{"a":1e5,"b":00}', '{"a":-0}', '{"a":-01}', '{"a":- 1}', '{"a":NaN}', '{"a":Infinity}', '{"a":undefined}', '{"a":\u00e9}',
  '{"a":\ud83d\ude00}', '{\n "a": 1,\n "b": x\n}', '{\r\n "a": 1,\r\n "b" 2\r\n}', '{"a":1,\n\n"b":[1,\n2,\n}',
  '{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": x, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbb": 1}', '{"a": x}',
  '{ "abcdefghij": [1, 2, 3], "k": ? , "zzzzzzzzzzzzzzzzzzzzzzz": 1}', '', ' ', '[', '[1', '"abc', 'tru', 't', 'nul', '-', '1.', '1e', 'x',
  '{"a":"\u0000"}', '{"a":"\\u00"}', '{"a":"\\x41"}', '{"a":1e999}', '{"a":-1e999}', '{"a":1E+2}', '{"a":12345678901234567890}', '{"a":0.1e-7}',
  '{"a":0e}', '{"a":0.}', '{"a":0e1}', '{"a":00}', '{"a":0 1}', '{"a":1 2}', '{"a":"b" "c"}', '{"a":"b" 1}', '{"a":[1 "x"]}', '{1:2}',
  '{"a":1, 2:3}', '{"a":1, "b"}', '{"a":1, "b" "c"}', '{"a":"\\\u20ac"}', '{"a":"\\\u00e9"}', '{"a":"\\u12', '{"a":"\\u', '{"a":"\\u1\u20ac23"}',
  '{"a":tr\u20ac}', '{"a":tr-}', '{"a":tr\n}', '{"a":t\u00e9}', '{"a":1\u20ac}', '{"a":-\u20ac}', '{"a":1.\u20ac}', '{"a":1e\u20ac}', '{"a":0\u20ac}',
  '{"a":0.5.5}', '{"a":1-}', '{"a":1+}', '{"a":1E}', '{"a":1.5e+}', '{"a":1.5E-x}', '{"a":2e5x}', '{"a":1x}', '{"a":1.5x}', '{"a":123456789012x}',
  '{"a":1234567890.}', '{"a":1234567890e}', '{"a":0123456789}', '{"a":--1}', '{"a":-.5}', '{"\u20ac":1}', '{"a":"\u20ac"}', '{\u20ac}', '{"a"\u20ac}',
  '{"a":1\u00a0}', '{"a":1\u000b}', '{"a":1\f}', '{"a" :\t1\r\n}', '{"a":1,\u00a0"b":2}', '{"a":[}', '{"a":[]]}', '{"a":{"b":}}', '{"a":{"b":1,}}',
  '{"a":{,}}', '{"a":{"b":1 "c":2}}', '{"a":[[1,2] [3]]}', '{"a":[{"b":1} {"c":2}]}', '{"__proto__": 1, "a": 2}', '{"a":1,"a":2,"b":3}',
  '{"2":"x","1":"y","a":"z"}', '{"a":"\\ud83d\\ude00"}', '{"a\\u0000b":1}', '{"":1}', '{"a":"\\/\\b\\f\\n\\r\\t\\"\\\\"}', '{"a":"\u007f"}',
  '{"a":"\u001f"}', '{"a":"\r"}', '{}', '{ }', '[]', '[1,2,3]', 'null', 'true', 'false', '0', '-0', '1.5e3', '"s"', ' {"a": 1} ', '{"a":{}}',
  '{"a":[]}', '{"a":[[],{}]}', '{"a":{"b":{"c":[1,{"d":2}]}}}', '{"a/b":{"c~d":1},"a~1b":2}', '{"a":{"x":1},"a":5}', '{"a":{"x":1},"a":{"y":2}}',
  '{"a":[{"x":1}],"a":[{"y":2},{"z":3}]}', '{"":{"":{"":1}},"":2}', '{"a":1,"b":{"a":1,"a":2},"a":{"b":1}}', '{"position 5": x}',
  '{"line 3 column 9": x}', '{"x":"unexpected end"} y', '{"end of json input": tru}', '{"END OF JSON INPUT": ?, "aaaaaaaaaaaaaaaaaaaa": 1}',
  '{"position 99999999999999999999999": x}', '{"a": 1, "position 2": [x]}', '{"k":"line 1 column 1"} x', '[1,]', '[,]', '[1 2]', '{"a":1}{"b":2}',
  '{"":line 1 column 1}', '{"":line 2 column 3}', '{"":line 9 column 9}', '{"":unexpected end}', '{"":Unexpected End}', '[end of json input]',
  '[END OF JSON INPUT]', '{"":position 7}', '{"":position 99999999}', '{"":position 9999999999999}', '{"":line 0 column 0}', '{"":line 1 column 0}',
  '{"k\\b\\f\\n\\r\\t\\/\\\\\\"": {"x": 1}, "\\u00e9\\u0041": [{"\\t": 2, "\\r": 3, "\\f": 4, "\\b": 5}]}',
  '{"a": tru\u20ac}', '{"a": n}', '{"a": nu', '{"a": f', '{"a": fa1se}', '{"a": "\\u0041\\u00e9\\uD83D\\uDE00"}', '{"A":1,"a":2}', '{"10":1,"9":2,"-1":3,"01":4,"1.5":5}',
].forEach(addJson);

// Random JSON values, and mutations of their texts.
const randomString = () => {
  const parts = ['a', 'b', 'key', 'x y', '', '1', '10', 'é', '😀', 'a/b', 'a~b', '\\n', '\\"', '\\u0041', '\\\\', 'position 3', 'line 1 column 2', '__proto__', '{', '}'];
  let s = '';
  const n = int(3);
  for (let i = 0; i <= n; i++) {
    s += pick(parts);
  }
  return '"' + s + '"';
};
const randomValue = (depth) => {
  const r = rnd();
  if (depth <= 0 || r < 0.45) {
    return pick([
      () => String(int(1000)),
      () => '-' + int(100),
      () => int(10) + '.' + int(1000),
      () => int(10) + 'e' + pick(['', '+', '-']) + int(30),
      () => '0',
      () => '-0',
      () => 'true',
      () => 'false',
      () => 'null',
      randomString,
      randomString,
    ])();
  }
  const ws = () => pick(['', '', ' ', '\n', '\n  ', '\t', '\r\n']);
  if (r < 0.7) {
    const n = int(4);
    const items = [];
    for (let i = 0; i < n; i++) {
      items.push(ws() + randomValue(depth - 1) + ws());
    }
    return '[' + items.join(',') + (n === 0 ? ws() : '') + ']';
  }
  const n = int(5);
  const items = [];
  for (let i = 0; i < n; i++) {
    items.push(ws() + randomString() + ws() + ':' + ws() + randomValue(depth - 1) + ws());
  }
  return '{' + items.join(',') + (n === 0 ? ws() : '') + '}';
};
const randomObject = () => {
  let text = randomValue(3);
  while (!text.startsWith('{')) {
    text = randomValue(3);
  }
  return text;
};
const jsonAlphabet = [...'{}[]",:\\ \n\t-+.eE0123456789tfnrulasx', 'é', '€', '😀', '\u0001', '\r\n', 'true', 'null', '""', '\\u', '\\q'];
const mutateJson = (text) => {
  let out = text;
  const edits = 1 + int(2);
  for (let i = 0; i < edits; i++) {
    const at = int(out.length + 1);
    const r = rnd();
    if (r < 0.4) {
      out = out.slice(0, at) + out.slice(at + 1 + int(2));
    } else if (r < 0.85) {
      out = out.slice(0, at) + pick(jsonAlphabet) + out.slice(at);
    } else {
      out = out.slice(0, at);
    }
  }
  return out;
};
for (let i = 0; i < 300; i++) {
  addJson(randomObject());
}
for (let i = 0; i < 1800; i++) {
  addJson(mutateJson(randomObject()));
}

const jsonCases = jsonTexts.map((text, i) => {
  const rec = { text };
  try {
    const value = JSON.parse(text);
    let order;
    try {
      order = parseOrderedJsonObject(text, 1, 1).propertyOrder;
    } catch {
      // The root is not an object.
    }
    if (order && !keysAgree(value, order, '')) {
      inconsistent++;
      return undefined;
    }
    rec.value = ordered(value);
  } catch (e) {
    rec.message = e.message;
  }
  // parseOrderedJsonObject, with a start position that changes with the case.
  const startLine = 1 + (i % 7);
  const startColumn = 1 + (i % 13);
  try {
    const parsed = parseOrderedJsonObject(text, startLine, startColumn);
    rec.ordered = [startLine, startColumn, ordered(parsed.value), ordered(parsed.propertyOrder)];
  } catch (e) {
    rec.orderedError = [startLine, startColumn, e.name, e.message, e.line ?? null, e.column ?? null];
  }
  return rec;
}).filter(Boolean);

// JSON blocks in a diagram: some of the texts above as the body of a json
// statement, when the lexer takes the whole text as one object literal.
for (let i = 0; i < jsonTexts.length; i += 3) {
  const text = jsonTexts[i];
  if (text.startsWith('{')) {
    add(`usecase-beta\n  json Data@${text}\n  A --> Data\n`);
  }
}

// ---------------------------------------------------------------------------

const cases = [];
for (const text of texts) {
  const rec = await run(text);
  if (rec.model === 'inconsistent') {
    inconsistent++;
    continue;
  }
  cases.push(rec);
}

const doc = { tokenTypes: vocabulary.map((t) => t.name), cases, json: jsonCases };
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(doc), { level: 9 }));
const count = (f) => cases.filter(f).length;
console.log(
  `${outFile}: ${cases.length} texts (${count((c) => c.model)} with a model, ${count((c) => c.lexError)} with a lexer error, ` +
    `${count((c) => c.error)} with an error), ${jsonCases.length} JSON texts (${jsonCases.filter((c) => c.message).length} not valid), ` +
    `${(fs.statSync(outFile).size / 1024).toFixed(0)} KiB`
);
const kinds = new Map();
for (const c of cases) {
  if (c.error) {
    const kind = c.error.replace(/'[^']*'/g, "'…'").replace(/\d+/g, 'N').slice(0, 60);
    kinds.set(kind, (kinds.get(kind) ?? 0) + 1);
  }
}
console.log([...kinds].sort((a, b) => b[1] - a[1]).map(([k, n]) => `${String(n).padStart(5)}  ${JSON.stringify(k)}`).join('\n'));
console.log(`${inconsistent} texts left out: V8 gave a JSON property a wrong name`);
// The mermaid chunk keeps a timer alive.
process.exit(0);
