// Writes internal/mmdc/testdata/fontembed.json.gz: what the font embedder
// of real mermaid-cli adds to the SVG of small diagrams. The Go port in
// internal/mmdc/fontembed.go is compared with it.
//
// The script renders each diagram with mmdc-oracle.mjs and stores:
//
//   name         the name of the case
//   source       the text of the diagram
//   svg          the .svg file of mmdc --no-font-embed
//   embedSha256  the SHA-256 of the .svg file of mmdc with its default
//                options, which has the fonts
//   embedStyles  the style elements that the embedder added, in the order
//                of the document: keys of "styles", which has each text one
//                time, with "sha256:" and the hash of the bytes of each
//                font in place of the base64 text
//
// Usage (in tools/reference): node gen-mmdc-embed.mjs
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

// Each case uses other fonts and style sheets of the page of mmdc.
const cases = {
  // The default theme of a flowchart names Recursive Variable.
  'recursive': 'flowchart LR\n  A[Hello] --> B(World)\n',
  // The theme "default" names fonts of the machine: nothing is embedded.
  'no-page-font': '---\nconfig:\n  theme: default\n---\nflowchart LR\n  A[Hello] --> B(World)\n',
  // A label with a solid icon of Font Awesome.
  'fa-solid': 'flowchart LR\n  A[fa:fa-car Car] --> B(World)\n',
  'fa-solid-default-theme': '---\nconfig:\n  theme: default\n---\nflowchart LR\n  A[fa:fa-car Car] --> B(World)\n',
  // The classes far and fab select the regular and the brands font.
  'fa-regular': 'flowchart LR\n  A[far:fa-bell Bell] --> B(World)\n',
  'fa-brands': 'flowchart LR\n  A[fab:fa-github GitHub] --> B(World)\n',
  'fa-all': 'flowchart LR\n  A[fa:fa-car Car] --> B[fab:fa-github GitHub] --> C[far:fa-bell Bell] --> D[fas:fa-ban No]\n',
  // A diagram that sets its font to Open Sans, normal and italic.
  'open-sans': '---\nconfig:\n  theme: default\n  themeVariables:\n    fontFamily: Open Sans\n---\nflowchart LR\n  A[Hello] --> B(World)\n',
  'open-sans-quoted': '---\nconfig:\n  theme: default\n  fontFamily: \'"Open Sans", sans-serif\'\n---\nflowchart LR\n  A["`Hello *italic*`"] --> B(World)\n',
  // The name of the family in another case is not the family of the page.
  'recursive-lower-case': '---\nconfig:\n  theme: default\n  themeVariables:\n    fontFamily: recursive variable\n---\nflowchart LR\n  A[Hello] --> B(World)\n',
  // The family of the page is not the first one.
  'recursive-second': '---\nconfig:\n  theme: default\n  themeVariables:\n    fontFamily: arial, Recursive Variable\n---\nflowchart LR\n  A[Hello] --> B(World)\n',
  // Sequence diagrams name Open Sans for the names of the actors.
  'sequence': 'sequenceDiagram\n  Alice->>Bob: Hello Bob, how are you?\n  Bob-->>Alice: Fine\n',
  'class': 'classDiagram\n  class Animal {\n    +int age\n    +eat()\n  }\n',
  'pie': 'pie title Pets\n  "Dogs" : 386\n  "Cats" : 85\n',
  'c4': 'C4Context\n  title System Context\n  Person(customer, "Customer", "A customer")\n  System(system, "System", "The system")\n  Rel(customer, system, "Uses")\n',
};

const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdc-embed-'));
const files = Object.entries(cases).map(([name, source]) => {
  const file = path.join(dir, `${name}.mmd`);
  fs.writeFileSync(file, source);
  return file;
});
execFileSync(process.execPath, ['mmdc-oracle.mjs', 'render', '--quiet', '--out', path.join(dir, 'out'), ...files], {
  cwd: here,
  stdio: ['ignore', 'inherit', 'inherit'],
});
// The same style text comes back in many cases: the file has each text one
// time, and a case names its texts by their SHA-256.
const styles = {};
const golden = Object.entries(cases).map(([name, source]) => {
  const meta = JSON.parse(fs.readFileSync(path.join(dir, 'out', `${name}.json`), 'utf8'));
  if (meta.error) {
    throw new Error(`${name}: ${meta.error}`);
  }
  return {
    name,
    source,
    svg: fs.readFileSync(path.join(dir, 'out', `${name}.svg`), 'utf8'),
    sha256: meta.sha256,
    embedSha256: meta.embedSha256,
    embedStyles: meta.embedStyles.map((text) => {
      const id = crypto.createHash('sha256').update(text).digest('hex').slice(0, 16);
      styles[id] = text;
      return id;
    }),
  };
});
const file = path.join(repoRoot, 'internal/mmdc/testdata/fontembed.json.gz');
fs.mkdirSync(path.dirname(file), { recursive: true });
fs.writeFileSync(file, zlib.gzipSync(JSON.stringify({ styles, cases: golden }), { level: 9 }));
for (const c of golden) {
  process.stderr.write(`${c.name}: ${c.embedStyles.length} style elements (${c.embedStyles.map((id) => styles[id].length).join(', ')})\n`);
}
process.stderr.write(`wrote ${path.relative(repoRoot, file)}: ${golden.length} cases, ${Object.keys(styles).length} style texts, ${fs.statSync(file).size} bytes\n`);
