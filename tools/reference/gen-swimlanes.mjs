// Records what mermaid's swimlane layout does, as golden data for
// internal/ru/layout/swimlanes.
//
//   node gen-swimlanes.mjs [--raw DIR] [--pack-only] [--from-batch N] [--synthetic N]
//
// It runs probes/swimlanes.js in the reference browser (oracle.mjs esm-eval)
// for three sets of cases and writes
// internal/ru/layout/swimlanes/testdata/layout.jsonl.gz, one JSON line per
// case:
//
//   corpus     every fixture of testdata/corpus/mermaid-e2e/swimlanes,
//              rendered by mermaid (the sizes are measured by the browser)
//   extra      diagram texts for the branches that the corpus does not
//              reach, rendered by mermaid
//   synthetic  layout data from a seeded random generator, with sizes from
//              the generator: no DOM is used
//   edge       diagram texts at the limits of the layout, rendered by
//              mermaid; mermaid fails for some of them
//
// See probes/swimlanes.js for the fields of a line. The browser runs in
// batches, so that the oracle lock is free between them. --raw keeps the
// output of the batches in a directory, --pack-only builds the golden
// file from that directory without a browser, and --from-batch runs only
// the batches from the given index (the others are read from --raw).
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const corpus = path.join(root, 'testdata/corpus/mermaid-e2e/swimlanes');
const outFile = path.join(root, 'internal/ru/layout/swimlanes/testdata/layout.jsonl.gz');

const opts = { synthetic: 420, batch: 120, fromBatch: 0 };
const argv = process.argv.slice(2);
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--raw') {
    opts.raw = argv[++i];
  } else if (argv[i] === '--pack-only') {
    opts.packOnly = true;
  } else if (argv[i] === '--synthetic') {
    opts.synthetic = Number(argv[++i]);
  } else if (argv[i] === '--from-batch') {
    opts.fromBatch = Number(argv[++i]);
  } else {
    console.error('usage: node gen-swimlanes.mjs [--raw DIR] [--pack-only] [--from-batch N] [--synthetic N]');
    process.exit(2);
  }
}

function walk(dir, out) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      walk(p, out);
    } else if (e.name.endsWith('.mmd')) {
      out.push(p);
    }
  }
  return out;
}

// ---- extra diagram texts ----

const EXTRA = {
  'cycle-three': `swimlane-beta TB
  subgraph L1
    A --> B
  end
  subgraph L2
    C
  end
  B --> C --> A`,
  'cycle-two-lanes-lr': `swimlane-beta LR
  subgraph One
    A --> B --> C
  end
  subgraph Two
    D --> E
  end
  C --> D
  E --> A
  E --> B`,
  'long-edges': `swimlane-beta TB
  subgraph L1
    A --> B --> C --> D --> E
  end
  subgraph L2
    X
    Y
  end
  A --> E
  A --> Y
  X --> E
  B --> D`,
  'long-edges-lr': `swimlane-beta LR
  subgraph L1
    A --> B --> C --> D --> E
  end
  subgraph L2
    X
    Y
  end
  A --> E
  A --> Y
  X --> E
  B --> D`,
  'labels-tb': `swimlane-beta TB
  subgraph Sales
    A[Start] -->|go| B{Decide}
    B -->|yes| C[Ship]
  end
  subgraph Ops
    D[Pack]
    E[Bill]
  end
  B -->|no| D
  D -->|a longer label here| E
  E -->|back| A`,
  'labels-lr': `swimlane-beta LR
  subgraph Sales
    A[Start] -->|go| B{Decide}
    B -->|yes| C[Ship]
  end
  subgraph Ops
    D[Pack]
    E[Bill]
  end
  B -->|no| D
  D -->|a longer label here| E
  E -->|back| A`,
  'nested-subgraphs': `swimlane-beta TB
  subgraph Outer
    subgraph Inner
      A --> B
    end
    C
  end
  subgraph Other
    subgraph Deep1
      subgraph Deep2
        D
      end
      E
    end
  end
  B --> C --> D --> E
  A --> E`,
  'nested-subgraphs-lr': `swimlane-beta LR
  subgraph Outer
    subgraph Inner
      A --> B
    end
    C
  end
  subgraph Other
    subgraph Deep1
      D
    end
    E
  end
  B --> C --> D --> E`,
  'direction-bt': `swimlane-beta BT
  subgraph L1
    A --> B
    B --> C
  end
  subgraph L2
    D --> E
  end
  A --> D
  E --> C
  C -->|up| A`,
  'direction-rl': `swimlane-beta RL
  subgraph L1
    A --> B
    B --> C
  end
  subgraph L2
    D --> E
  end
  A --> D
  E --> C
  C -->|left| A`,
  'direction-td': `swimlane-beta TD
  subgraph L1
    A --> B
  end
  subgraph L2
    C --> D
  end
  B --> C`,
  'single-lane': `swimlane-beta TB
  subgraph Only
    A --> B
    A --> C
    B --> D
    C --> D
    D --> A
  end`,
  'single-lane-lr': `swimlane-beta LR
  subgraph Only
    A --> B
    A --> C
    B --> D
    C --> D
    D --> A
  end`,
  'many-lanes': `swimlane-beta TB
  subgraph L1
    A
  end
  subgraph L2
    B
  end
  subgraph L3
    C
  end
  subgraph L4
    D
  end
  subgraph L5
    E
  end
  subgraph L6
    F
  end
  A --> B --> C --> D --> E --> F
  A --> F
  F --> C
  B --> E`,
  'many-lanes-lr': `swimlane-beta LR
  subgraph L1
    A
  end
  subgraph L2
    B
  end
  subgraph L3
    C
  end
  subgraph L4
    D
  end
  subgraph L5
    E
  end
  A --> B --> C --> D --> E
  A --> E
  E --> C
  B --> D`,
  'disconnected': `swimlane-beta TB
  subgraph L1
    A --> B
    P
    Q
  end
  subgraph L2
    C --> D
    R
  end
  subgraph L3
    S
  end`,
  'disconnected-lr': `swimlane-beta LR
  subgraph L1
    A --> B
    P
    Q
  end
  subgraph L2
    C --> D
    R
  end
  subgraph L3
    S
  end`,
  'no-lanes': `swimlane-beta TB
  A --> B --> C
  A --> C`,
  'mixed-loose-nodes': `swimlane-beta LR
  subgraph L1
    A --> B
  end
  X --> A
  B --> Y
  X --> Y`,
  'fan-out': `swimlane-beta TB
  subgraph L1
    A --> B
    A --> C
    A --> D
    A --> E
  end
  subgraph L2
    F
    G
  end
  B --> F
  C --> F
  D --> G
  E --> G
  F --> G`,
  'fan-in-lr': `swimlane-beta LR
  subgraph L1
    A
    B
    C
  end
  subgraph L2
    D{Join}
    E
  end
  A --> D
  B --> D
  C --> D
  D --> E
  E --> A`,
  'multi-edges': `swimlane-beta TB
  subgraph L1
    A --> B
    A --> B
    B --> A
  end
  subgraph L2
    C
  end
  A --> C
  A --> C
  C --> B`,
  'shapes': `swimlane-beta TB
  subgraph L1
    A([Round]) --> B{Rhombus}
    B --> C((Circle))
    B --> D[/Lean/]
  end
  subgraph L2
    E[(Database)]
    F{{Hexagon}}
  end
  C --> E
  D --> F
  E --> F
  F --> B`,
  'case-ids': `swimlane-beta TB
  subgraph lane_b
    b1 --> B2
    a10 --> a2
  end
  subgraph Lane_A
    Zed --> alpha
    n_1 --> N_2
  end
  B2 --> Zed
  a2 --> n_1
  alpha --> b1
  N_2 --> a10`,
  'automatic-lane-order': `---
config:
  swimlane:
    automaticLaneOrdering: true
---
swimlane-beta TB
  subgraph L1
    A
  end
  subgraph L2
    B
  end
  subgraph L3
    C
  end
  subgraph L4
    D
  end
  A --> C
  A --> C
  C --> A
  B --> D
  D --> B
  A --> D`,
  'gravity-layering': `---
config:
  swimlane:
    ignoreCrossLaneEdges: false
---
swimlane-beta TB
  subgraph L1
    A --> B --> C
  end
  subgraph L2
    D --> E
  end
  A --> D
  B --> E
  E --> C
  D --> C`,
  'gravity-no-optimize-lr': `---
config:
  swimlane:
    ignoreCrossLaneEdges: false
    optimizeRanksByCrossings: false
---
swimlane-beta LR
  subgraph L1
    A --> B --> C
  end
  subgraph L2
    D --> E
  end
  A --> D
  B --> E
  E --> C
  D --> C`,
  'spacing': `---
config:
  flowchart:
    nodeSpacing: 20
    rankSpacing: 80
---
swimlane-beta TB
  subgraph L1
    A --> B
    A --> C
  end
  subgraph L2
    D --> E
  end
  B --> D
  C --> E`,
  'line-hops-gap': `---
config:
  swimlane:
    lineHops: gap
---
swimlane-beta TB
  subgraph L1
    A --> B
    C --> D
  end
  subgraph L2
    E --> F
  end
  A --> F
  E --> B
  C --> F
  E --> D`,
  'flowchart-with-layout': `---
config:
  layout: swimlane
---
flowchart LR
  subgraph One
    A --> B
  end
  subgraph Two
    C --> D
  end
  B --> C
  D --> A`,
  'handdrawn-labels': `---
config:
  look: handDrawn
---
swimlane-beta LR
  subgraph Sales
    A[Start] -->|go| B{Decide}
    B -->|yes| C[Ship]
  end
  subgraph Ops
    D[Pack]
  end
  B -->|no| D
  D --> A`,
  'tight-spacing': `---
config:
  flowchart:
    nodeSpacing: 5
    rankSpacing: 10
---
swimlane-beta TB
  subgraph L1
    A --> B --> C
    A --> C
  end
  subgraph L2
    D --> E
  end
  B --> D
  E --> C
  C --> A`,
  'html-labels': `---
config:
  flowchart:
    htmlLabels: true
---
swimlane-beta TB
  subgraph Sales
    A[Start] -->|go| B{Decide}
  end
  subgraph Ops
    D[Pack]
  end
  B -->|no| D`,
};

// Texts at the limits of the layout. mermaid fails for some of them; then
// the message of the error is the reference.
const EDGE = {
  'empty': `swimlane-beta TB`,
  'single-node': `swimlane-beta LR
  A`,
  'self-loop': `swimlane-beta TB
  subgraph L
    A --> A
  end`,
  'self-loop-in-cycle': `swimlane-beta TB
  subgraph L
    A --> A
    A --> B
    B --> A
  end`,
  'empty-lanes': `swimlane-beta LR
  subgraph L1
  end
  subgraph L2
  end`,
  'empty-lane-beside-lane': `swimlane-beta TB
  subgraph L1
  end
  subgraph L2
    A --> B
  end`,
  'subgraph-cycle': `swimlane-beta TB
  subgraph A
    B
  end
  subgraph B
    A
  end
  X --> Y`,
  'node-named-as-lane': `swimlane-beta TB
  subgraph A
    A
  end
  A --> A`,
  'edges-to-lanes': `swimlane-beta TB
  subgraph L1
    A
  end
  subgraph L2
    B
  end
  A --> L2
  L1 --> B
  L1 --> L2`,
  'deep-nesting': `swimlane-beta BT
  subgraph s1
  subgraph s2
  subgraph s3
  subgraph s4
  subgraph s5
    a --> b
  end
  end
  end
  end
  end
  b --> c`,
  'label-pairs': `swimlane-beta TB
  subgraph L
    a -->|x| b
    b -->|y| a
    a -->|x| b
    b -->|y| a
  end`,
  'negative-spacing': `---
config:
  flowchart:
    nodeSpacing: -50
    rankSpacing: 0
---
swimlane-beta TB
  subgraph L
    A --> B --> C
    A --> C
  end`,
  'markdown-and-wrapping': `swimlane-beta LR
  subgraph Lane one with a long title that wraps
    A["\`**Bold** and a long label that the renderer wraps over lines\`"] -->|"\`a *markdown* label\`"| B[Short]
  end
  subgraph Two
    C[Another node with a longer text inside]
  end
  B --> C`,
  'direction-in-subgraph': `swimlane-beta TB
  subgraph L1
    direction LR
    A --> B
  end
  subgraph L2
    direction BT
    C --> D
  end
  B --> C`,
  'styles-and-classes': `swimlane-beta LR
  subgraph L1
    A:::hot --> B
  end
  subgraph L2
    C --> D
  end
  B -->|styled| C
  classDef hot fill:#f96,stroke:#333
  style L2 fill:#eef,stroke:#00f
  linkStyle 0 stroke:red,stroke-width:3px
  linkStyle 2 color:blue`,
};

// ---- synthetic layout data ----

function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const NODE_IDS = [
  'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P',
  'a', 'b', 'c', 'n1', 'n2', 'n10', 'N_1', 'x-y', 'Zeta', 'alpha', 'Beta', 'node_3', 'Q1', 'q1',
  'Start', 'End', 'check', 'Check2', 'z', 'Z', '1', '2', '10', 'A1', 'a1', 'A_b', 'a.B',
];
const LANE_IDS = ['lane1', 'Lane2', 'L3', 'sales', 'Ops', 'legal', 'Zone', 'B_lane', '7'];
const SHAPES = ['squareRect', 'squareRect', 'squareRect', 'squareRect', 'squareRect', 'roundedRect', 'diamond', 'question', 'circle', 'stadium'];
const LABELS = ['Yes', 'No', 'ok', 'a longer label', 'x', 'maybe so', 'retry'];
const SWIMLANE_CONFIGS = [
  {},
  {},
  {},
  { ignoreCrossLaneEdges: true, optimizeRanksByCrossings: true, automaticLaneOrdering: false },
  { ignoreCrossLaneEdges: false },
  { ignoreCrossLaneEdges: false, optimizeRanksByCrossings: false },
  { ignoreCrossLaneEdges: false, optimizeRanksByCrossings: true, automaticLaneOrdering: true },
  { automaticLaneOrdering: true },
];

function genSynthetic(index) {
  const rnd = mulberry32(0x5317 + index * 7919);
  const int = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));
  const pick = (list) => list[Math.floor(rnd() * list.length)];
  const chance = (p) => rnd() < p;
  const shuffle = (list) => {
    const a = [...list];
    for (let i = a.length - 1; i > 0; i--) {
      const j = Math.floor(rnd() * (i + 1));
      [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
  };
  // A size as the browser measures it: a multiple of 1/64.
  const size = (lo, hi) => Math.round((lo + rnd() * (hi - lo)) * 64) / 64;

  // Small cases come first; the later ones grow.
  const scale = index < 60 ? 0 : index < 200 ? 1 : 2;
  const direction = pick(['TB', 'TB', 'LR', 'LR', 'BT', 'RL', undefined]);
  const laneCount = pick([[0, 1, 1, 2, 2], [1, 2, 2, 3, 3, 4], [2, 3, 3, 4, 5, 6]][scale]);
  const nodeCount = index % 97 === 96 ? 0 : [int(1, 5), int(3, 9), int(6, 16)][scale];
  const labelChance = pick([0.1, 0.22, 0.22, 0.5]);

  const groups = [];
  const laneIds = shuffle(LANE_IDS).slice(0, laneCount);
  laneIds.forEach((id, i) => {
    groups.push({
      id,
      label: id,
      labelStyle: '',
      labelType: 'text',
      ...(chance(0.05) ? {} : { padding: chance(0.1) ? pick([0, 20, 30]) : 8 }),
      shape: 'rect',
      isGroup: true,
      look: 'neo',
      colorIndex: i,
    });
  });
  // A node with the id of the default lane, rarely: a group or a plain node.
  if (chance(0.02)) {
    groups.push({ id: '__swimlane_default__', label: 'given', isGroup: chance(0.7), shape: 'rect', padding: 8 });
  }
  // Nested groups inside lanes, sometimes two levels deep.
  const containers = [...laneIds];
  if (laneCount > 0) {
    const nested = scale === 0 ? (chance(0.2) ? 1 : 0) : int(0, 3);
    for (let k = 0; k < nested; k++) {
      const parentId = pick(containers);
      const id = `sub${k}_${parentId}`;
      groups.push({
        id,
        label: id,
        labelStyle: '',
        labelType: 'text',
        padding: 8,
        shape: 'rect',
        isGroup: true,
        look: 'neo',
        parentId,
        ...(chance(0.3) ? { dir: pick(['LR', 'TB']) } : {}),
      });
      containers.push(id);
    }
  }

  const ids = shuffle(NODE_IDS).slice(0, nodeCount);
  const sizes = {};
  const nodes = ids.map((id) => {
    const shape = pick(SHAPES);
    const w = chance(0.05) ? size(170, 320) : size(30, 170);
    const h = shape === 'diamond' || shape === 'question' || shape === 'circle' ? w : size(21, 80);
    // A node without a size, rarely.
    if (!chance(0.015)) {
      sizes[id] = chance(0.01) ? [0, 0] : [w, h];
    }
    const loose = laneCount === 0 || chance(0.07);
    // A parent that is not a group or does not exist, rarely.
    const parentId = chance(0.01) ? 'no-such-group' : pick(containers);
    return {
      id,
      label: id,
      labelStyle: '',
      labelType: 'text',
      ...(loose ? {} : { parentId }),
      padding: 15,
      look: 'neo',
      isGroup: false,
      shape,
      ...(chance(0.04) ? { dir: pick(['LR', 'TB']) } : {}),
    };
  });
  if (nodes.length > 1 && chance(0.01)) {
    nodes[1].parentId = nodes[0].id;
  }

  const edges = [];
  const pairCount = {};
  const addEdge = (start, end, label) => {
    const key = `${start}_${end}`;
    const n = pairCount[key] ?? 0;
    pairCount[key] = n + 1;
    const id = `L_${start}_${end}_${n}`;
    edges.push({
      id,
      isUserDefinedId: false,
      start,
      end,
      type: 'arrow_point',
      label: label ?? '',
      labelType: 'text',
      labelpos: 'c',
      thickness: 'normal',
      minlen: 1,
      arrowTypeStart: 'none',
      arrowTypeEnd: 'arrow_point',
      labelStyle: chance(0.05) ? ['color:red'] : chance(0.03) ? 'color:blue' : [],
      style: [],
      pattern: 'normal',
      look: 'neo',
      curve: chance(0.1) ? pick(['linear', 'stepAfter', undefined]) : 'basis',
    });
    if (label) {
      // The label node gets the size of its text.
      const empty = chance(0.04);
      sizes[`edge-label-${start}-${end}-${id}`] = empty ? [0, 0] : [size(8, 12) * label.length, pick([21, 21, 24])];
    }
  };
  const edgeCount = nodeCount < 2 ? (chance(0.3) ? 1 : 0) : int(Math.max(0, nodeCount - 2), Math.floor(nodeCount * [1.2, 1.5, 1.7][scale]));
  const style = pick(['random', 'random', 'forward', 'chain']);
  for (let k = 0; k < edgeCount; k++) {
    let a = int(0, nodeCount - 1);
    let b = int(0, nodeCount - 1);
    if (style === 'forward' && a > b) {
      [a, b] = [b, a];
    }
    if (style === 'chain' && k < nodeCount - 1) {
      a = k;
      b = k + 1;
    }
    if (a === b && !chance(0.08)) {
      b = (a + 1) % nodeCount;
    }
    if (nodeCount === 1) {
      b = a;
    }
    addEdge(ids[a], ids[b], chance(labelChance) ? pick(LABELS) : undefined);
  }
  // Edges that the prepare phase and the graph view must skip, rarely: no
  // start, a label node that is set already, a layout-only edge with a
  // label.
  if (edges.length > 0 && chance(0.02)) {
    delete edges[edges.length - 1].start;
  }
  if (edges.length > 1 && chance(0.02)) {
    edges[0].labelNodeId = 'no-such-label-node';
  }
  if (edges.length > 2 && chance(0.02)) {
    edges[1].isLayoutOnly = true;
    edges[1].label = 'hidden';
  }
  // An edge to a node that does not exist, rarely.
  if (chance(0.03) && nodeCount > 0) {
    addEdge(ids[0], 'missing-node');
  }

  // Groups come first, as flowDb.getData returns them. Sometimes the order
  // is mixed.
  let allNodes = [...groups.slice().reverse(), ...nodes];
  if (chance(0.15)) {
    allNodes = shuffle(allNodes);
  }

  const flowchart = {};
  const nodeSpacing = pick([50, 50, 50, 30, 80, undefined, 10, 5]);
  const rankSpacing = pick([50, 50, 50, 40, 100, undefined, 10, 20]);
  if (nodeSpacing !== undefined) {
    flowchart.nodeSpacing = nodeSpacing;
  }
  if (rankSpacing !== undefined) {
    flowchart.rankSpacing = rankSpacing;
  }
  const data = {
    nodes: allNodes,
    edges,
    config: { look: 'neo', flowchart, swimlane: pick(SWIMLANE_CONFIGS) },
    ...(direction ? { direction } : {}),
  };
  return { name: `synthetic/${String(index).padStart(4, '0')}`, data, sizes };
}

// ---- run ----

const cases = [];
for (const file of walk(corpus, [])) {
  cases.push({ name: 'corpus/' + path.relative(corpus, file).replace(/\.mmd$/, ''), text: fs.readFileSync(file, 'utf8') });
}
for (const [name, text] of Object.entries(EXTRA)) {
  cases.push({ name: 'extra/' + name, text, keepSvg: true });
}
for (let i = 0; i < opts.synthetic; i++) {
  cases.push(genSynthetic(i));
}
// The edge texts come last, so that a change here leaves the earlier
// batches as they are.
for (const [name, text] of Object.entries(EDGE)) {
  cases.push({ name: 'edge/' + name, text, keepSvg: true });
}

const rawDir = opts.raw ?? fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-swimlanes-'));
fs.mkdirSync(rawDir, { recursive: true });
const batches = [];
for (let i = 0; i < cases.length; i += opts.batch) {
  batches.push(cases.slice(i, i + opts.batch));
}
if (!opts.packOnly) {
  batches.forEach((batch, i) => {
    if (i < opts.fromBatch) {
      return;
    }
    const argsFile = path.join(rawDir, `args-${i}.json`);
    const resultFile = path.join(rawDir, `out-${i}.json`);
    fs.writeFileSync(argsFile, JSON.stringify({ cases: batch }));
    execFileSync('node', ['oracle.mjs', 'esm-eval', 'probes/swimlanes.js', '--args', argsFile, '--out', resultFile], {
      cwd: here,
      stdio: 'inherit',
      maxBuffer: 1 << 30,
    });
    console.error(`batch ${i + 1}/${batches.length}: ${batch.length} cases`);
  });
}

const lines = [];
const extraSvgs = {};
const stats = { total: 0, errors: 0, inconsistent: 0, skipped: 0, steps: 0 };
batches.forEach((batch, i) => {
  const { records } = JSON.parse(fs.readFileSync(path.join(rawDir, `out-${i}.json`), 'utf8'));
  for (const record of records) {
    stats.total++;
    if (record.svg !== undefined || record.outcomeError !== undefined) {
      extraSvgs[record.name] = { svg: record.svg ?? null, diagramType: record.diagramType ?? null, error: record.outcomeError ?? null };
      delete record.svg;
      delete record.diagramType;
      delete record.outcomeError;
    }
    if (record.input === undefined) {
      // The layout did not run: the render failed before it, or the diagram
      // has another layout.
      stats.skipped++;
      console.error(`no layout for ${record.name}: ${record.skipped ?? record.renderError}`);
      continue;
    }
    // A render that fails after the layout is still a record of the layout.
    delete record.renderError;
    if (record.error !== undefined) {
      stats.errors++;
    }
    if (record.consistent === false) {
      stats.inconsistent++;
      console.error(`inconsistent ${record.name}: ${record.traceError ?? record.error ?? ''}`);
      continue;
    }
    stats.steps += record.steps ? record.steps.length : 0;
    lines.push(JSON.stringify(record));
  }
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(lines.join('\n') + '\n', { level: 9 }));
// The texts and the SVGs of the extra cases, for an end-to-end check.
const extraFile = path.join(root, 'internal/ru/layout/swimlanes/testdata/extra-svg.json.gz');
const extra = [
  ...Object.entries(EXTRA).map(([name, text]) => ({ name, text, ...extraSvgs['extra/' + name] })),
  ...Object.entries(EDGE).map(([name, text]) => ({ name: 'edge-' + name, text, ...extraSvgs['edge/' + name] })),
];
fs.writeFileSync(extraFile, zlib.gzipSync(JSON.stringify(extra), { level: 9 }));
if (!opts.raw) {
  fs.rmSync(rawDir, { recursive: true, force: true });
}
console.error(
  `${stats.total} cases: ${lines.length} recorded (${stats.errors} with a layout error), ` +
    `${stats.inconsistent} inconsistent, ${stats.skipped} skipped, ${stats.steps} steps`
);
console.error(`wrote ${outFile} (${fs.statSync(outFile).size} bytes), ${extraFile} (${fs.statSync(extraFile).size} bytes)`);
