// Golden generator for the gitGraph db of internal/diagrams/git.
//
// It runs the real mermaid 12.2.0 in plain node: the config step and the
// parse step of mermaid.render (processAndSetConfigs, then Diagram.fromText
// with the front matter title), which fill the db of gitGraphAst.ts. For
// every input it stores the db, and the error that upstream throws.
//
// node has no DOM, so DOMPurify does nothing there. The generator installs
// DOMPurify.sanitize as the identity function. That is what the real
// function does for a string that has no "<". An input for which the
// function gets a string with "<" is left out. The browser golden file
// (gitgraph-db-oracle.mjs) covers the real DOMPurify.
//
// Math.random is the seeded generator of the reference page, with one
// number drawn before each case, as the mermaid bundle does in the page.
//
// Run with tools/reference as the working directory:
//
//   node gen-gitgraph-db.mjs
//
// Output: internal/diagrams/git/testdata/db_node.json.gz
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import DOMPurify from 'dompurify';
import { internals } from './lib-mermaid-internals.mjs';
import { TESTDATA, corpusFiles, stripFrontMatter, mulberry32, mutations } from './gitgraph-cases.mjs';

const RANDOM_SEED = 0x6d6d6467;

class StubError extends Error {}
DOMPurify.sanitize = (dirty) => {
  if (String(dirty).includes('<')) throw new StubError('markup');
  return dirty;
};
DOMPurify.addHook = () => {};

const { mermaid, processAndSetConfigs } = await internals();
mermaid.initialize({ startOnLoad: false });

// dump reads the db as the Go test compares it.
const dump = (db) => ({
  commits: [...db.getCommits().entries()].map(([key, commit]) => ({ key, ...commit })),
  branches: db.getBranchesAsObjArray().map((b) => b.name),
  branchHeads: [...db.getBranches().entries()],
  direction: db.getDirection(),
  currentBranch: db.getCurrentBranch(),
  head: db.getHead(),
  title: db.getDiagramTitle(),
  accTitle: db.getAccTitle(),
  accDescr: db.getAccDescription(),
});

// The db object of the gitGraph diagram. It is one object for the page.
const gitDb = (await mermaid.mermaidAPI.getDiagramFromText('gitGraph')).db;

const run = async (text) => {
  const rand = mulberry32(RANDOM_SEED);
  Math.random = rand;
  Math.random();
  let processed;
  try {
    processed = processAndSetConfigs(text);
  } catch {
    return null;
  }
  let type;
  try {
    type = mermaid.detectType(processed.code.cleaned ?? processed.code);
  } catch {
    return null;
  }
  if (type !== 'gitGraph') return null;
  try {
    const diagram = await mermaid.mermaidAPI.getDiagramFromText(text, { title: processed.title });
    return { text, db: dump(diagram.db) };
  } catch (e) {
    if (e instanceof StubError) return null;
    const message = String((e && e.message) || e);
    if (message.startsWith('Parsing failed')) return { text, parseError: true };
    return { text, error: message, db: dump(gitDb) };
  }
};

// Statement lists for each branch of the db: the errors of merge,
// cherry-pick, checkout and branch, and the cases next to them.
const SCENARIOS = [
  'commit\nmerge main',
  'branch dev\nmerge main',
  'commit\nbranch dev\nmerge main',
  'commit\nbranch dev\ncheckout main\nmerge dev',
  'commit\nbranch dev\ncommit\ncheckout main\nmerge dev',
  'commit\nbranch dev\ncommit\nmerge main',
  'commit\nmerge nothere',
  'merge nothere',
  'branch dev\ncommit\ncheckout main\nmerge dev',
  'branch dev\ncheckout main\ncommit\nmerge dev',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "a"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m" tag: "t1" tag: "t2" type: REVERSE',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev type: HIGHLIGHT',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev type: NORMAL',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev\nmerge dev',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\nmerge dev\ncheckout dev\nmerge main',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\nmerge dev\ncheckout dev\ncommit\nmerge main',
  'commit\nbranch dev\nbranch dev',
  'commit\nbranch main',
  'branch main',
  'commit\ncheckout nothere',
  'checkout main',
  'commit\nbranch dev\ncheckout main\ncheckout dev\nswitch main',
  'branch a\nbranch b\nbranch c\ncommit',
  'branch a order: 3\nbranch b order: 1\nbranch c order: 2\ncommit',
  'branch a order: 3\nbranch b\nbranch c order: 0\ncommit',
  'commit\nbranch a order: 2\ncommit\nbranch b order: 2\ncommit\nbranch c order: 1',
  'commit id: "a"\ncommit id: "a"',
  'commit id: "a"\ncommit id: "a"\ncommit id: "a"',
  'commit id: "a"\nbranch dev\ncommit id: "a"',
  'commit id: ""\ncommit id: "" tag: "" msg: ""',
  'commit id: "a=b" tag: "x>y" msg: "m = n"',
  'commit id: "a" tag: "t1" tag: "t2" tag: "" type: REVERSE "the message"',
  'commit type: HIGHLIGHT\ncommit type: REVERSE\ncommit type: NORMAL',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b" tag: "t"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b" tag: ""',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b" tag: "t" tag: "" tag: "u"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b" parent: "a"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b" parent: "x"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "x"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncherry-pick tag: "t"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncherry-pick id: "b"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncherry-pick id: "a"',
  'commit id: "a"\ncherry-pick id: "a"',
  'branch dev\ncommit id: "b"\ncheckout main\ncherry-pick id: "b"',
  'cherry-pick id: "a"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\nbranch rel\ncommit id: "r"\ncherry-pick id: "m"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\nbranch rel\ncheckout dev\ncommit id: "r"\ncherry-pick id: "m"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\ncheckout dev\ncommit id: "r"\ncherry-pick id: "m" parent: "b"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\ncheckout dev\ncommit id: "r"\ncherry-pick id: "m" parent: "c"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\ncheckout dev\ncommit id: "r"\ncherry-pick id: "m" parent: "c" tag: "v"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\ncheckout dev\ncommit id: "r"\ncherry-pick id: "m" parent: "a"',
  'commit id: "a"\nbranch dev\ncommit id: "b"\ncheckout main\ncommit id: "c"\nmerge dev id: "m"\ncheckout dev\ncommit id: "r"\ncherry-pick id: "m"',
  'commit\ncommit\nbranch dev\ncommit\ncommit\ncheckout main\ncommit\nmerge dev\ncommit\nbranch "feature x"\ncommit\ncheckout main\ncherry-pick id: "nope"',
  'commit\nbranch "a b"\ncommit\ncheckout "a b"\ncheckout main\nmerge "a b"',
  'commit\nbranch a.b/c-d\ncommit\ncheckout main\nmerge a.b/c-d',
  'title the title\naccTitle: the acc title\naccDescr: the acc descr\ncommit',
  'accDescr {\n  line one\n     line   two\n\n}\ncommit',
  'commit\ntitle  late   title',
];
const HEADERS = ['gitGraph', 'gitGraph LR:', 'gitGraph TB:', 'gitGraph BT:'];
const CONFIGS = [
  '',
  '---\nconfig:\n  gitGraph:\n    mainBranchName: trunk\n---\n',
  '---\nconfig:\n  gitGraph:\n    mainBranchOrder: 2\n---\n',
  '---\nconfig:\n  gitGraph:\n    mainBranchName: dev\n    mainBranchOrder: 1\n---\n',
  '---\ntitle: Front matter title\nconfig:\n  gitGraph:\n    mainBranchOrder: 3.5\n---\n',
  '%%{init: {"gitGraph": {"mainBranchName": "MetroLine1", "mainBranchOrder": 10}}}%%\n',
];

const texts = new Map();
const add = (text) => {
  if (text.isWellFormed() && !text.includes('<') && !texts.has(text)) texts.set(text, null);
};

for (const config of CONFIGS) {
  SCENARIOS.forEach((scenario, i) => {
    const header = HEADERS[i % HEADERS.length];
    add(`${config}${header}\n  ${scenario.split('\n').join('\n  ')}\n`);
    // The main branch of the scenario under the configured name.
    const named = scenario.replace(/\bmain\b/g, config.includes('trunk') ? 'trunk' : 'main');
    add(`${config}${header}\n${named}`);
  });
}

const rand = mulberry32(0x67697432);
for (const f of corpusFiles()) {
  add(f.text);
  const body = stripFrontMatter(f.text);
  const frontMatter = f.text.slice(0, f.text.length - body.length);
  for (const m of mutations(rand, body, 30)) add(frontMatter + m);
}

const cases = [];
let skipped = 0;
let parseErrors = 0;
let dbErrors = 0;
const messages = new Map();
for (const text of texts.keys()) {
  const result = await run(text);
  if (!result) {
    skipped++;
    continue;
  }
  if (result.parseError) parseErrors++;
  if (result.error) {
    dbErrors++;
    const key = result.error.replace(/\(.*?\)|'.*?'|id:\S+/g, '_');
    messages.set(key, (messages.get(key) ?? 0) + 1);
  }
  cases.push(result);
}

fs.mkdirSync(TESTDATA, { recursive: true });
const file = path.join(TESTDATA, 'db_node.json.gz');
fs.writeFileSync(file, zlib.gzipSync(JSON.stringify({ cases }), { level: 9 }));
process.stderr.write(
  `${cases.length} cases (${parseErrors} parse errors, ${dbErrors} db errors, ${skipped} inputs left out) -> ${file} (${fs.statSync(file).size} bytes)\n`
);
for (const [message, n] of [...messages.entries()].sort((a, b) => b[1] - a[1])) {
  process.stderr.write(`  ${n}  ${message}\n`);
}
