// Records what mermaid's dagre layout does for real diagrams, as the golden
// file of internal/ru/layout/dagre (layer 1 of its tests).
//
//   node gen-dagre-layout.mjs [--list]
//
// It renders the fixtures below with upstream mermaid in Chromium through
// probes/dagre-layout-spy.js and writes
// internal/ru/layout/dagre/testdata/layout-record.json.gz. With --list it
// only prints the names of the fixtures.
//
// The fixtures are:
//   - every corpus fixture that names `layout: dagre` in its config,
//   - 50 flowchart fixtures without a layout in their config, picked for
//     subgraphs, self-loops, edge labels and directions,
//   - state, ER, requirement and class fixtures without a layout in their
//     config,
//   - the diagrams of CUSTOM below, for branches that the corpus does not
//     reach,
//   - random flowcharts from a seeded generator: nested subgraphs, edges
//     that end at subgraphs, self-loops, edge labels, directions, title
//     margins and spacings.
//
// The default layout of mermaid 12.2.0 is elk. The fixtures that do not
// name a layout are rendered with `mermaid.initialize({ layout: 'dagre' })`.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, '../..');
const corpus = path.join(root, 'testdata/corpus/mermaid-e2e');
const outFile = path.join(root, 'internal/ru/layout/dagre/testdata/layout-record.json.gz');

const walk = (dir) => {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...walk(p));
    } else if (entry.name.endsWith('.mmd')) {
      out.push(p);
    }
  }
  return out;
};

const all = walk(corpus).map((file) => ({ name: path.relative(corpus, file), text: fs.readFileSync(file, 'utf8') }));
const explicitDagre = (f) => /layout['"]?: *['"]?dagre/.test(f.text);
// No layout in the config: the layout of the site config applies.
const defaultLayout = (f) => !/layout['"]?\s*:/.test(f.text) && !/\belk\b/i.test(f.name);
const small = (f) => f.text.length < 5000;

const FORCE_DAGRE = { layout: 'dagre' };
const picked = new Map();
const add = (f, init) => {
  if (!picked.has(f.name)) {
    picked.set(f.name, init ? { ...f, init } : f);
    return true;
  }
  return false;
};
// take adds up to n fixtures of the list that pass the test.
const take = (list, n, test) => {
  let count = 0;
  for (const f of list) {
    if (count >= n) {
      break;
    }
    if (!picked.has(f.name) && test(f) && add(f, FORCE_DAGRE)) {
      count++;
    }
  }
  return count;
};

for (const f of all.filter(explicitDagre)) {
  add(f);
}
const explicitCount = picked.size;

const family = (prefix) => all.filter((f) => f.name.startsWith(prefix) && defaultLayout(f) && small(f));
const selfLoop = (f) => /(^|[\s;])([A-Za-z_][\w]*)\s*(-->|---|==>|-\.->|--[^>\n|]*-->)\s*(\|[^|\n]*\|\s*)?\2([\s;]|$)/m.test(f.text);
const count = (f, re) => (f.text.match(re) || []).length;
const dirOf = (f) => (f.text.match(/^\s*(?:graph|flowchart)\s+(TB|TD|BT|LR|RL)/m) || [])[1];

// 50 flowcharts.
const flow = family('flowchart/');
const before = picked.size;
take(flow, 8, selfLoop);
take(flow, 12, (f) => count(f, /^\s*subgraph\b/gm) >= 2);
take(flow, 8, (f) => /^\s*direction\s+(TB|BT|LR|RL)/m.test(f.text));
take(flow, 8, (f) => /subgraph/.test(f.text) && /\|[^|\n]+\|/.test(f.text));
for (const dir of ['BT', 'RL', 'LR', 'TB', 'TD']) {
  take(flow, 2, (f) => dirOf(f) === dir && /\|[^|\n]+\|/.test(f.text));
}
take(flow, 50 - (picked.size - before), (f) => /subgraph/.test(f.text));
take(flow, 50 - (picked.size - before), () => true);
const flowCount = picked.size - before;

// The margins of subgraph titles shift the nodes and the edges.
take(flow, 10, (f) => /subGraphTitleMargin/.test(f.text));
// A spacing that is a string in the config.
take(flow, 10, (f) => /(rank|node)Spacing: *['"]/.test(f.text));

// State diagrams: composite states, notes and directions.
const state = [...family('state-diagram-v2/'), ...family('state-diagram/')];
take(state, 12, (f) => count(f, /\bstate\b[^\n]*\{/g) >= 2);
take(state, 6, (f) => /^\s*direction\s+(TB|BT|LR|RL)/m.test(f.text));
take(state, 6, (f) => /note (left|right) of/.test(f.text));
take(state, 6, () => true);
// ER diagrams and requirement diagrams.
const er = family('er-diagram/');
take(er, 6, (f) => /^\s*direction\s+(TB|BT|LR|RL)/m.test(f.text));
take(er, 14, () => true);
take(family('requirement/'), 15, () => true);
// Class diagrams with namespaces, which are clusters.
const cls = family('class-diagram/');
take(cls, 8, (f) => /namespace/.test(f.text));
take(cls, 4, (f) => /note for/.test(f.text));

// Diagrams for the branches that the corpus does not reach.
const CUSTOM = {
  'self-loop-td': 'flowchart TD\n  A --> A\n  A --> B',
  'self-loop-label-lr': 'flowchart LR\n  A -->|loop| A\n  A --> B',
  'self-loop-label-bt': 'flowchart BT\n  A -- again --> A\n  B --> A',
  'self-loop-label-rl': 'flowchart RL\n  A -->|loop| A\n  B --> A --> C',
  'self-loop-two-on-one-node': 'flowchart TD\n  A -->|one| A\n  A -->|two| A\n  A --> B',
  'self-loop-only': 'flowchart TD\n  A --> A',
  'self-loop-in-subgraph': 'flowchart TD\n  subgraph S\n    A -->|in| A\n    A --> B\n  end\n  C --> D',
  'self-loop-in-connected-subgraph': 'flowchart LR\n  subgraph S\n    A -->|in| A\n    A --> B\n  end\n  B --> C',
  'self-loop-on-subgraph': 'flowchart TD\n  subgraph S\n    A --> B\n  end\n  S --> S',
  'self-loop-on-nested-subgraph': 'flowchart TD\n  subgraph O\n    subgraph S\n      A --> B\n    end\n    C\n  end\n  S --> S\n  C --> D',
  'subgraph-to-subgraph': 'flowchart TD\n  subgraph S1\n    A --> B\n  end\n  subgraph S2\n    C --> D\n  end\n  S1 --> S2',
  'subgraph-to-node-and-back': 'flowchart LR\n  subgraph S1\n    A --> B\n  end\n  X --> S1\n  S1 -->|out| Y\n  B --> Y',
  'nested-three-levels': 'flowchart TB\n  subgraph L1\n    subgraph L2\n      subgraph L3\n        A --> B\n      end\n      C --> D\n    end\n    E --> F\n  end\n  G --> H',
  'nested-with-external': 'flowchart TB\n  subgraph L1\n    subgraph L2\n      A --> B\n    end\n    subgraph L2b\n      C --> D\n    end\n    B --> C\n  end\n  D --> E',
  'nested-direction': 'flowchart LR\n  subgraph TOP\n    direction TB\n    subgraph B1\n      direction RL\n      i1 --> f1\n    end\n    subgraph B2\n      direction BT\n      i2 --> f2\n    end\n  end\n  A --> TOP --> B\n  B1 --> B2',
  'anchor-in-extractable-sibling': 'flowchart TD\n  subgraph Outer\n    subgraph Inner\n      a --> b\n    end\n    c\n  end\n  Outer --> d',
  'anchor-only-extractable': 'flowchart TD\n  subgraph Outer\n    subgraph Inner\n      a --> b\n    end\n  end\n  Outer --> d\n  e --> Outer',
  'empty-subgraph': 'flowchart TD\n  subgraph S\n  end\n  A --> B',
  'numeric-ids': 'flowchart TD\n  10 --> 2\n  2 --> 1\n  subgraph 7\n    3 --> 4\n  end\n  1 --> 3',
  'title-margins': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 10\n      bottom: 5\n---\nflowchart LR\n  subgraph S1\n    A -->|x| A\n    A --> B\n  end\n  subgraph S2\n    subgraph S3\n      C --> D\n    end\n  end\n  B --> C',
  'spacing-config': '---\nconfig:\n  flowchart:\n    nodeSpacing: 20\n    rankSpacing: 90\n---\nflowchart TD\n  A --> B & C\n  subgraph S\n    D --> E & F\n  end\n  C --> G',
  'top-level-spacing': '---\nconfig:\n  nodeSpacing: 15\n  rankSpacing: 30\n---\nflowchart TD\n  A --> B & C\n  subgraph S\n    D --> E & F\n  end',
  'edge-lengths': 'flowchart TD\n  A --> B\n  A ---> C\n  A -----> D\n  B -->|long label on this edge| D',
  'state-self-transition': 'stateDiagram-v2\n  [*] --> A\n  A --> A : again\n  A --> [*]',
  'state-self-transition-in-composite': 'stateDiagram-v2\n  state Outer {\n    A --> A : again\n    A --> B\n  }\n  [*] --> Outer',
  'state-composite-self': 'stateDiagram-v2\n  state Outer {\n    A --> B\n  }\n  Outer --> Outer : loop\n  [*] --> Outer',
  'er-self-relationship': 'erDiagram\n  EMPLOYEE ||--o{ EMPLOYEE : manages\n  EMPLOYEE }|--|| DEPARTMENT : in',
  'class-self-relation': 'classDiagram\n  class Node\n  Node "1" --> "0..*" Node : children\n  Node --> Tree',
  'class-namespace-self': 'classDiagram\n  namespace N {\n    class A\n    class B\n  }\n  A --> A : self\n  A --> B\n  B --> C',
  'safe-anchor': 'flowchart TD\n  subgraph Outer\n    subgraph Inner\n      a --> b\n    end\n    c\n  end\n  c --> d\n  Outer --> e',
  'safe-anchor-in-closed-cluster': 'flowchart TD\n  subgraph World\n    subgraph Outer\n      subgraph Inner\n        a --> b\n      end\n      c\n    end\n    d\n    e\n  end\n  c --> d\n  Outer --> e',
  'safe-anchor-second-inner': 'flowchart LR\n  subgraph Outer\n    subgraph Inner1\n      a --> b\n    end\n    subgraph Inner2\n      c --> x\n    end\n  end\n  x --> d\n  Outer --> e',
  'safe-anchor-none': 'flowchart TD\n  subgraph Outer\n    subgraph Inner1\n      a --> b\n    end\n    subgraph Inner2\n      c --> x\n    end\n    Inner1 --> Inner2\n  end\n  Outer --> e\n  f --> Outer',
  'safe-anchor-incoming-only': 'flowchart TD\n  subgraph Outer\n    subgraph Inner\n      a --> b\n    end\n    c\n  end\n  c --> d\n  e --> Outer',
  'safe-anchor-deep': 'flowchart TD\n  subgraph Outer\n    subgraph Mid\n      subgraph Inner\n        a --> b\n      end\n      m\n    end\n    c\n  end\n  m --> d\n  Outer --> e\n  Mid --> f',
  'margins-state': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 12\n      bottom: 3\n---\nstateDiagram-v2\n  state Outer {\n    A --> A : again\n    A --> B\n    state Inner {\n      C --> D\n    }\n  }\n  [*] --> Outer\n  B --> E',
  'margins-class': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 7\n      bottom: 9\n---\nclassDiagram\n  namespace N {\n    class A\n    class B\n  }\n  A --> A : self\n  A --> B\n  B --> C\n  C "1" --> "n" C : again',
  'margins-er': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 4\n      bottom: 4\n---\nerDiagram\n  direction LR\n  EMPLOYEE ||--o{ EMPLOYEE : manages\n  EMPLOYEE }|--|| DEPARTMENT : in',
  'margins-requirement': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 6\n      bottom: 2\n---\nrequirementDiagram\n  direction LR\n  requirement r1 {\n    id: 1\n    text: one\n  }\n  element e1 {\n    type: sim\n  }\n  e1 - satisfies -> r1\n  r1 - traces -> r1\n  e1 - contains -> e1',
  'requirement-self-bt': 'requirementDiagram\n  direction BT\n  requirement r1 {\n    id: 1\n    text: one\n  }\n  requirement r2 {\n    id: 2\n    text: two\n  }\n  r1 - derives -> r2\n  r2 - refines -> r2',
  'margins-self-loops-rl': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 15\n      bottom: 5\n---\nflowchart RL\n  A -->|top| A\n  subgraph S\n    B -->|in| B\n    subgraph T\n      C -->|deep| C\n    end\n    B --> C\n  end\n  A --> B\n  S --> S',
  'margins-self-loops-bt': '---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      top: 3\n      bottom: 11\n---\nflowchart BT\n  A -->|top| A\n  subgraph S\n    direction LR\n    B -->|in| B\n    B --> C\n  end\n  A --> D\n  S --> D',
  'state-note-closed': 'stateDiagram-v2\n  A : alone\n  note right of A : a note\n  B --> C',
  'state-note-closed-left-lr': 'stateDiagram-v2\n  direction LR\n  A : alone\n  note left of A\n    two lines\n    of note\n  end note\n  B --> C\n  C --> C : self',
  'state-note-in-composite': 'stateDiagram-v2\n  state Outer {\n    A : alone\n    note right of A : a note\n    B --> C\n  }\n  [*] --> Outer',
  'state-concurrency': 'stateDiagram-v2\n  [*] --> Active\n  state Active {\n    [*] --> A\n    A --> B\n    --\n    [*] --> C\n    C --> C : stay\n    C --> D\n  }',
  'string-spacing': "---\nconfig:\n  flowchart:\n    rankSpacing: '100'\n    nodeSpacing: '30'\n---\nflowchart TD\n  A --> B & C\n  subgraph S\n    D --> E & F\n    subgraph T\n      G --> H\n    end\n  end\n  C --> G2",
  'string-spacing-state': "---\nconfig:\n  state:\n    rankSpacing: '60'\n    nodeSpacing: '20'\n---\nstateDiagram-v2\n  state Outer {\n    A --> B\n    state Inner {\n      C --> D\n    }\n  }\n  [*] --> X",
  'string-spacing-top-level': "---\nconfig:\n  rankSpacing: '70'\n  nodeSpacing: '25'\n---\nflowchart LR\n  A --> B & C\n  subgraph S\n    D --> E & F\n  end",
  'requirement-self': 'requirementDiagram\n  requirement r1 {\n    id: 1\n    text: one\n  }\n  element e1 {\n    type: sim\n  }\n  e1 - satisfies -> r1\n  r1 - traces -> r1',
};
for (const [name, text] of Object.entries(CUSTOM)) {
  add({ name: `custom/${name}`, text }, FORCE_DAGRE);
}

// ---- random flowcharts ----
let prngState = 0x666c6f77 >>> 0; // "flow"
const R = () => {
  prngState = (prngState + 0x6d2b79f5) >>> 0;
  let t = prngState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const ri = (a, b) => a + Math.floor(R() * (b - a + 1));
const pick = (arr) => arr[Math.floor(R() * arr.length)];
const chance = (p) => R() < p;

// randomFlowchart writes a flowchart with nested subgraphs. Every node is
// declared once, in the body of its subgraph, and the edges follow after
// the subgraphs, so that the edges do not change which subgraph a node is
// in. pClusterEdge is the chance that an end of an edge is a subgraph.
const randomFlowchart = ({ leaves, clusters, maxDepth, edges, pClusterEdge, pSelfLoop, pLabel, pMargins, pSpacing, pDir }) => {
  const cluster = [];
  for (let i = 0; i < clusters; i++) {
    const c = { id: `S${i}`, parent: null, depth: 0, children: [], leaves: [] };
    if (i > 0 && chance(0.6)) {
      const p = cluster[ri(0, i - 1)];
      if (p.depth + 1 < maxDepth) {
        c.parent = p;
        c.depth = p.depth + 1;
        p.children.push(c);
      }
    }
    if (chance(pDir)) {
      c.dir = pick(['TB', 'BT', 'LR', 'RL']);
    }
    cluster.push(c);
  }
  const rootLeaves = [];
  const leafIds = [];
  for (let i = 0; i < leaves; i++) {
    const id = `n${i}`;
    leafIds.push(id);
    if (cluster.length > 0 && chance(0.75)) {
      pick(cluster).leaves.push(id);
    } else {
      rootLeaves.push(id);
    }
  }
  const lines = [];
  const front = [];
  if (chance(pMargins)) {
    front.push('    subGraphTitleMargin:', `      top: ${pick([0, 5, 10, 20])}`, `      bottom: ${pick([0, 5, 10])}`);
  }
  if (chance(pSpacing)) {
    front.push(`    nodeSpacing: ${pick([10, 30, 80])}`, `    rankSpacing: ${pick([20, 60, 100])}`);
  }
  if (front.length > 0) {
    lines.push('---', 'config:', '  flowchart:', ...front, '---');
  }
  lines.push(`flowchart ${pick(['TB', 'TD', 'BT', 'LR', 'RL'])}`);
  const emit = (c, indent) => {
    lines.push(`${indent}subgraph ${c.id}${chance(0.3) ? ` [Title of ${c.id}]` : ''}`);
    if (c.dir) {
      lines.push(`${indent}  direction ${c.dir}`);
    }
    for (const id of c.leaves) {
      lines.push(`${indent}  ${id}`);
    }
    for (const child of c.children) {
      emit(child, indent + '  ');
    }
    lines.push(`${indent}end`);
  };
  for (const c of cluster.filter((c) => !c.parent)) {
    emit(c, '  ');
  }
  for (const id of rootLeaves) {
    lines.push(`  ${id}`);
  }
  const clusterIds = cluster.map((c) => c.id);
  const end = () => (clusterIds.length > 0 && chance(pClusterEdge) ? pick(clusterIds) : pick(leafIds));
  for (let k = 0; k < edges; k++) {
    const v = end();
    const w = chance(pSelfLoop) ? v : end();
    const arrow = pick(['-->', '-->', '-->', '--->', '---', '-.->', '==>', '<-->']);
    lines.push(`  ${v} ${arrow}${chance(pLabel) ? `|${pick(['yes', 'no', 'a longer label', 'x'])}|` : ''} ${w}`);
  }
  return lines.join('\n');
};

const randomBatches = [
  // Subgraphs whose edges stay between leaves.
  [35, () => ({ leaves: ri(2, 9), clusters: ri(0, 4), maxDepth: 3, edges: ri(1, 10), pClusterEdge: 0, pSelfLoop: 0.08, pLabel: 0.3, pMargins: 0.3, pSpacing: 0.15, pDir: 0.3 })],
  // Edges that end at subgraphs.
  [55, () => ({ leaves: ri(3, 9), clusters: ri(1, 5), maxDepth: 4, edges: ri(2, 10), pClusterEdge: 0.3, pSelfLoop: 0.08, pLabel: 0.3, pMargins: 0.3, pSpacing: 0.15, pDir: 0.3 })],
  // Many subgraphs, few nodes.
  [30, () => ({ leaves: ri(2, 6), clusters: ri(3, 6), maxDepth: 5, edges: ri(1, 7), pClusterEdge: 0.5, pSelfLoop: 0.15, pLabel: 0.4, pMargins: 0.5, pSpacing: 0.1, pDir: 0.5 })],
];
let randomCount = 0;
for (const [n, params] of randomBatches) {
  for (let i = 0; i < n; i++) {
    add({ name: `random/flowchart-${String(randomCount).padStart(3, '0')}`, text: randomFlowchart(params()) }, FORCE_DAGRE);
    randomCount++;
  }
}

const fixtures = [...picked.values()];
if (process.argv.includes('--list')) {
  for (const f of fixtures) {
    console.log(f.name);
  }
  process.exit(0);
}
console.error(`${fixtures.length} fixtures: ${explicitCount} with layout: dagre, ${flowCount} flowcharts, ${Object.keys(CUSTOM).length} custom, ${randomCount} random`);

const tmpArgs = path.join(os.tmpdir(), `mmdg-dagre-layout-args-${process.pid}.json`);
const tmpOut = path.join(os.tmpdir(), `mmdg-dagre-layout-out-${process.pid}.json`);
fs.writeFileSync(tmpArgs, JSON.stringify({ fixtures }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/dagre-layout-spy.js'), '--args', tmpArgs, '--out', tmpOut], {
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const results = JSON.parse(fs.readFileSync(tmpOut, 'utf8'));
fs.rmSync(tmpArgs);
fs.rmSync(tmpOut);

const recorded = results.filter((r) => r.input);
const json = JSON.stringify({ generator: 'tools/reference/gen-dagre-layout.mjs', mermaid: '12.2.0', fixtures: results });
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(json, { level: 9 }));
const graphs = recorded.reduce((n, r) => n + Object.keys(r.graphs).length, 0);
console.error(
  `${results.length} fixtures: ${recorded.length} recorded with ${graphs} graphs, ${results.filter((r) => r.error).length} errors, ${results.filter((r) => r.skipped).length} skipped -> ${path.relative(root, outFile)} (${fs.statSync(outFile).size} bytes)`
);
for (const r of results) {
  if (r.error) {
    console.error(`  error ${r.name}: ${r.error}`);
  } else if (r.skipped) {
    console.error(`  skipped ${r.name}: ${r.skipped}`);
  }
}
