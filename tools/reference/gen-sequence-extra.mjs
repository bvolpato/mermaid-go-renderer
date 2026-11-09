// Writes internal/diagrams/sequence/testdata/extra_render.json.gz: the SVG
// that mermaid.render() returns in Chromium for hand-written sequence
// diagrams. They cover the parts of sequenceRenderer.ts and svgDraw.js that
// no fixture of the corpus uses: the links and properties menus, the icon
// and class properties, rightAngles, textPlacement, forceMenus,
// arrowMarkerAbsolute, mirrorActors: false, the title, and config values of
// the sequence section.
//
// The script renders with the render command of the browser oracle, one
// page for each diagram, as the reference of the corpus is rendered.
//
// Usage (in tools/reference): node gen-sequence-extra.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/diagrams/sequence/testdata/extra_render.json.gz');

const front = (config) => `---\nconfig:\n${config.replace(/^/gm, '  ')}\n---\n`;
const sd = (body, config = '') => (config ? front(config) : '') + 'sequenceDiagram\n' + body.replace(/^/gm, '  ') + '\n';

const basic = `participant Alice
participant Bob
actor Carol
Alice->>Bob: Hello Bob
Bob-->>Alice: Hi Alice
Alice->>Alice: Think<br/>twice
Bob-)Carol: Async
Note right of Carol: A note
Note over Alice,Bob: Over two`;

const menus = `participant Alice
participant Bob as Bob the builder
links Alice: {"Dashboard": "https://dashboard.contoso.com/alice", "Wiki": "https://wiki.contoso.com/alice"}
link Alice: Endpoint @ https://alice.contoso.com
link Bob: A long label for the menu entry of Bob @ https://bob.contoso.com/a?b=c&d=e
links Bob: {"Repo": "javascript:alert(1)", "Two words": "relative/path"}
Alice->>Bob: Hello
Bob-->>Alice: Bye`;

const cases = {
  'empty': sd(''),
  'only-participants': sd('participant A\nactor B\nparticipant C as A long description of C'),
  'basic': sd(basic),
  'menus': sd(menus),
  'menus-force': sd(menus, 'sequence:\n  forceMenus: true'),
  'menus-fo': sd(menus, 'sequence:\n  textPlacement: fo'),
  'menus-old': sd(menus, 'sequence:\n  textPlacement: old'),
  'menus-classic': sd(menus, 'look: classic\ntheme: default'),
  'menus-shapes': sd(`participant A@{ "type": "queue" }
participant B@{ "type": "database" }
participant C@{ "type": "collections" }
links A: {"Queue": "https://q.example"}
links B: {"Database": "https://db.example", "Second": "https://db.example/2"}
links C: {"Collections": "https://c.example"}
A->>B: x
B->>C: y`),
  'menus-actor-error': sd('actor Alice\nlinks Alice: {"Docs": "https://example.com"}\nAlice->>Bob: x'),
  'menus-boundary-error': sd('participant A@{ "type": "boundary" }\nlink A: Docs @ https://example.com\nA->>B: x'),
  'properties': sd(`participant Alice
participant Bob
participant Carol
participant Dave@{ "type": "queue" }
participant Eve@{ "type": "collections" }
participant Fay@{ "type": "database" }
properties Alice: {"class": "internal-service-actor", "icon": "@computer"}
properties Bob: {"class": "external-service-actor", "icon": "@database", "type": "x"}
properties Carol: {"icon": "https://example.com/icon.png"}
properties Dave: {"class": "q", "icon": " @clock "}
properties Eve: {"icon": "javascript:alert(1)"}
properties Fay: {"class": "db"}
Alice->>Bob: Hello
Bob->>Carol: World
Carol->>Dave: x
Dave->>Eve: y
Eve->>Fay: z`),
  'properties-icon-error': sd('participant A\nproperties A: {"icon": 5}\nA->>B: x'),
  'right-angles': sd(basic + '\nBob->>Bob: wrap: A long self message that is wrapped over lines', 'sequence:\n  rightAngles: true'),
  'right-angles-autonumber': sd('autonumber\nA->>A: one\nA<<->>A: two\nA/|-A: three\nA()->>()A: four', 'sequence:\n  rightAngles: true'),
  'text-placement-fo': sd(basic, 'sequence:\n  textPlacement: fo'),
  'text-placement-old': sd(basic + '\nparticipant Dan as Two<br/>lines', 'sequence:\n  textPlacement: old'),
  'text-placement-fo-shapes': sd(`participant A@{ "type": "boundary" }
participant B@{ "type": "control" }
participant C@{ "type": "entity" }
participant D@{ "type": "database" }
participant E@{ "type": "collections" }
participant F@{ "type": "queue" }
actor G as Two<br/>lines
A->>G: x`, 'sequence:\n  textPlacement: fo'),
  'arrow-marker-absolute': sd(basic + '\nautonumber\nAlice-xBob: cross\nBob--)Alice: point\nAlice-|\\Bob: half\nBob//-Alice: stick', 'arrowMarkerAbsolute: true'),
  'no-mirror': sd(basic, 'sequence:\n  mirrorActors: false'),
  'no-mirror-destroy': sd(`participant Alice
participant Bob
create participant Carl
Alice->>Carl: Hi Carl
create actor Dan
Carl->>Dan: Hi Dan
destroy Carl
Alice-xCarl: Bye
destroy Bob
Bob->>Alice: I go
Alice->>Dan: Still here`, 'sequence:\n  mirrorActors: false'),
  'create-destroy-shapes': sd(`participant Alice
create participant Q@{ "type": "queue" }
Alice->>Q: q
create participant D@{ "type": "database" }
Alice->>D: d
create participant E@{ "type": "entity" }
Q->>E: e
create participant K@{ "type": "control" }
E->>K: k
create participant B@{ "type": "boundary" }
K->>B: b
create participant C@{ "type": "collections" }
B->>C: c
destroy E
Alice-xE: bye
destroy K
K->>Alice: bye
destroy D
Alice-xD: bye`),
  'title': sd('title The title of the diagram\n' + basic),
  'title-legacy': sd('title: A legacy title\nAlice->>Bob: x'),
  'title-frontmatter': '---\ntitle: Title of the front matter\n---\n' + sd('Alice->>Bob: x\nBob->>Alice: y'),
  'title-boxes': sd(`title With boxes
box Aqua Left group
participant A
participant B
end
box rgb(200,220,255)
participant C
end
participant D
A->>D: x
D->>B: y`),
  'boxes-wrap': sd(`box LightYellow wrap: A very long title of a box that has to wrap over more than one line of text
participant A
end
box transparent nowrap: Another quite long title that stays on one line
participant B
participant C
end
box Purple
actor D
end
A->>D: x`),
  'boxes-split': sd(`box Aqua One
participant A
end
participant B
box Aqua One
end
A->>B: x
participant C`),
  'hide-unused-boxes': sd(`box Aqua One
participant A
participant B
end
box Pink Two
participant C
participant D
end
participant E
A->>C: x
Note over E: unused but noted`, 'sequence:\n  hideUnusedParticipants: true'),
  'unknown-type': sd('participant A@{ "type": "nothing" }\nparticipant B\nA->>B: x\nB->>A: y\nNote over A: n'),
  'unknown-type-number': sd('participant A@{ "type": 5 }\nA->>B: x'),
  'alias-number-error': sd('participant A@{ "alias": 5 }\nA->>B: x'),
  'loose': sd(menus + '\nAlice->>Bob: <b>bold</b> & <i>it</i>', 'securityLevel: loose'),
  'antiscript': sd(basic, 'securityLevel: antiscript'),
  'font-size': sd(basic, 'fontSize: 22'),
  'font-size-string': sd(basic, 'fontSize: "13px"'),
  'font-zero': sd(basic, 'fontSize: 0\nsequence:\n  actorFontSize: "18px"\n  noteFontSize: 11\n  messageFontSize: "1em"'),
  'font-family': sd(basic, 'fontFamily: arial\nsequence:\n  actorFontFamily: courier'),
  'font-family-empty': sd(basic, 'fontFamily: ""\nsequence:\n  actorFontFamily: courier\n  noteFontFamily: verdana\n  messageFontFamily: times\n  actorFontWeight: bold\n  noteFontWeight: 700\n  messageFontWeight: 100'),
  'font-weight': sd(basic, 'fontWeight: bold'),
  'align-left': sd(basic + '\nNote over Bob: two<br/>lines of note', 'sequence:\n  messageAlign: left\n  noteAlign: left'),
  'align-right': sd(basic + '\nNote over Bob: two<br/>lines of note', 'sequence:\n  messageAlign: right\n  noteAlign: right'),
  'align-bogus': sd(basic, 'sequence:\n  messageAlign: bogus\n  noteAlign: justify'),
  'wrap-global': sd(`participant Alice as Alice with a very long name that wraps around
Alice->>Bob: A very long message that needs to be wrapped because it is so long that it does not fit
Note over Alice,Bob: A very long note that needs to be wrapped because it is so long that it does not fit
loop A very long loop title that needs to be wrapped because it is so long
Bob->>Bob: A very long self message that needs to be wrapped because it is long
end`, 'wrap: true'),
  'wrap-sequence': sd(`box Aqua A long box title that should wrap because of the config
participant Alice
end
Alice->>Bob: nowrap: A very long message that is not wrapped because it says so explicitly
Bob->>Alice: A very long message that needs to be wrapped because it is so long that it does not fit
Note left of Alice: A very long note that needs to be wrapped because it is so long`, 'sequence:\n  wrap: true'),
  'activations': sd(`Alice->>+Bob: one
Alice->>+Bob: two
Alice->>+Bob: three
Alice->>+Bob: four
Bob-->>-Alice: r4
Bob-->>-Alice: r3
Bob->>+Carol: nested
Carol-->>-Bob: back
Bob-->>-Alice: r2
Bob-->>-Alice: r1
activate Alice
activate Alice
Alice->>Alice: self
deactivate Alice
deactivate Alice`),
  'activation-short': sd('activate Alice\ndeactivate Alice\nAlice->>Bob: x\nactivate Bob\ndeactivate Bob'),
  'autonumber-sizes': sd(`autonumber 98 1
A->>B: two digits
A->>B: two digits
A->>B: three digits
autonumber 9999 1
A->>B: four digits
B->>A: five digits
autonumber 99999 0.5
A->>A: five digits
B<<->>A: seven chars
autonumber 0.25 0.25
A->>B: decimals
B-->>A: decimals
autonumber off
A->>B: off
autonumber
A->>B: on again`),
  'max-width-false': sd(basic, 'sequence:\n  useMaxWidth: false'),
  'margins': sd(basic + `
loop L
Alice->>+Bob: x
Bob-->>-Alice: y
end
box Aqua B
participant Dan
end
Bob->>Dan: z`, `sequence:
  diagramMarginX: 12
  diagramMarginY: 33
  actorMargin: 80
  width: 120
  height: 40
  boxMargin: 6
  boxTextMargin: 9
  noteMargin: 14
  messageMargin: 20
  activationWidth: 16
  labelBoxWidth: 70
  labelBoxHeight: 30
  bottomMarginAdj: 7
  wrapPadding: 4`),
  'margins-classic': sd(basic, `look: classic
theme: default
sequence:
  width: 60
  height: 100
  labelBoxWidth: 20
  labelBoxHeight: 0`),
  'notes': sd(`participant A
participant B
participant C
Note left of A: left of the first
Note right of C: right of the last
Note over C,A: right to left
Note over B: over one
Note left of B: left of the middle with more text than fits
Note right of B: right of the middle<br/>two lines
Note over A,C: wrap: A long note over all three actors that is wrapped and then some more words
Note right of A:
A->>C: x`),
  'blocks': sd(`par_over Outer
A->>B: one
and Other
par Inner
B->>C: two
and
C->>A: three
end
end
critical Establish a connection
A-->B: connect
option Network timeout
A-->A: Log error
option
A-->A: Log other
end
break when something
B->>A: stop
end
opt
A->>B: no title
end
loop
B->>A: no title
end
rect
A->>B: default fill
rect rgba(0, 0, 255, .1)
B->>A: nested
end
end`),
  'entities': sd('participant A as #9829; Heart #amp; #lt;tag#gt;\nA->>B: #35;hash #quot;q#quot; #9731;\nNote over A: #60;b#62;x#60;/b#62;'),
  'markup': sd('participant A as <b>Bold</b>\nA->>B: <script>alert(1)</script> <i>i</i> & "q" \'s\'\nNote over A,B: <img src=x onerror=y> text'),
  'names': sd('participant Long Name With Spaces\nparticipant x-y_z.1\nLong Name With Spaces->>x-y_z.1: hi\nx-y_z.1-->>Long Name With Spaces: bye'),
  'unicode': sd('participant A as Ünïcödé\nA->>B: 你好，世界\nB->>A: مرحبا\nNote over A: emoji 😀 text'),
  'acc': sd('accTitle: The accessible title\naccDescr: The accessible description\nA->>B: x'),
  'acc-multiline': sd('accDescr {\n  A description\n  on two lines\n}\ntitle T\nA->>B: x'),
  'dark-menus': sd(menus, 'theme: dark'),
  'theme-variables': sd(basic + '\nrect\nAlice->>Bob: in rect\nend', `theme: base
themeVariables:
  actorBkg: "#ff0000"
  actorBorder: "#00ff00"
  rectBkgColor: "#0000ff"
  noteBkgColor: "#ffff00"`),
  'redux-dark-color-activations': sd('participant A\nactor B\nparticipant C@{ "type": "queue" }\nA->>+B: x\nB->>+C: y\nC-->>-B: z\nB-->>-A: w', 'theme: redux-dark-color'),
  'no-html-labels': sd(basic, 'htmlLabels: false'),
  'show-sequence-numbers': sd(basic + '\nBob<<->>Alice: both\nCarol/|-Bob: reverse', 'sequence:\n  showSequenceNumbers: true'),
  'central-create': sd('participant A\ncreate participant B\nA->>()B: hi\nA()->>()B: again\ndestroy B\nB()-xA: bye'),
  'self-note-only': sd('Note over A: only a note'),
  'messages-only-self': sd('A->>A: self\nA-->>A: self again<br/>two lines<br/>three lines'),
  'wide-message': sd('A->>B: A very wide message between two neighbours that makes the margin of the actor grow a lot\nB->>A: short'),
  'deactivate-error': sd('A->>B: x\ndeactivate B'),
};

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'seqextra-'));
const inDir = path.join(tmp, 'in');
const outDir = path.join(tmp, 'out');
fs.mkdirSync(inDir);
const names = Object.keys(cases);
for (const name of names) {
  fs.writeFileSync(path.join(inDir, name + '.mmd'), cases[name]);
}
execFileSync(
  process.execPath,
  ['oracle.mjs', 'render', '--quiet', '--out', outDir, '--base', inDir, ...names.map((n) => path.join(inDir, n + '.mmd'))],
  { cwd: here, stdio: 'inherit' }
);
const golden = names.map((name) => {
  const meta = JSON.parse(fs.readFileSync(path.join(outDir, name + '.json'), 'utf8'));
  const rec = { name, text: cases[name] };
  if (meta.error) {
    rec.error = meta.error;
  } else {
    rec.svg = fs.readFileSync(path.join(outDir, name + '.svg'), 'utf8');
  }
  return rec;
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
console.log(`${outFile}: ${golden.length} diagrams, ${golden.filter((g) => g.error).length} with an error`);
