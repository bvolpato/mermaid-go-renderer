#!/usr/bin/env node
// Writes testdata/corpus/mermaid-large: large diagrams from a seeded
// generator.
//
// The diagrams of the mermaid test suite are small: no layout graph has
// more than 22 nodes. Large graphs take other paths in the layout engines
// (for example the network simplex layering removes subtrees from 40 nodes
// on). This corpus has flowcharts, class, state and ER diagrams with 30 to
// 150 nodes, so that those paths are compared with mermaid end to end.
//
// Usage (in tools/reference): node gen-large-corpus.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(here, '../../testdata/corpus/mermaid-large');

// mulberry32
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

const WORDS = ['order', 'payment', 'queue', 'worker', 'cache', 'index', 'report', 'user', 'session', 'token', 'batch', 'event', 'stream', 'shard', 'replica', 'gateway', 'router', 'ledger', 'invoice', 'audit', 'schema', 'bucket', 'policy', 'alert', 'metric', 'trace', 'span', 'probe', 'lease', 'quota'];

function makeRand(seed) {
  const r = prng(seed);
  return {
    next: r,
    int: (lo, hi) => lo + Math.floor(r() * (hi - lo + 1)),
    pick: (list) => list[Math.floor(r() * list.length)],
    chance: (p) => r() < p,
    label(maxWords = 3) {
      const n = 1 + Math.floor(r() * maxWords);
      const words = [];
      for (let i = 0; i < n; i++) {
        words.push(WORDS[Math.floor(r() * WORDS.length)]);
      }
      const text = words.join(' ');
      return text[0].toUpperCase() + text.slice(1);
    },
  };
}

function frontMatter(config) {
  if (!config) {
    return '';
  }
  return `---\nconfig:\n${config}\n---\n`;
}

const SHAPES = [
  (l) => `[${l}]`,
  (l) => `(${l})`,
  (l) => `{${l}}`,
  (l) => `([${l}])`,
  (l) => `[[${l}]]`,
  (l) => `[(${l})]`,
  (l) => `((${l}))`,
  (l) => `{{${l}}}`,
  (l) => `[/${l}/]`,
];
const LINKS = ['-->', '---', '-.->', '==>', '--o', '--x'];

function flowchart(seed, n, dir, layout) {
  const R = makeRand(seed);
  const lines = [];
  lines.push(`flowchart ${dir}`);
  const groups = R.int(0, Math.min(6, Math.floor(n / 12)));
  const groupOf = [];
  for (let i = 0; i < n; i++) {
    groupOf.push(groups > 0 && R.chance(0.6) ? R.int(0, groups - 1) : -1);
  }
  const decl = (i) => `  n${i}${R.pick(SHAPES)(R.label())}`;
  for (let g = 0; g < groups; g++) {
    lines.push(`  subgraph g${g} [${R.label(2)}]`);
    for (let i = 0; i < n; i++) {
      if (groupOf[i] === g) {
        lines.push('  ' + decl(i));
      }
    }
    lines.push('  end');
  }
  for (let i = 0; i < n; i++) {
    if (groupOf[i] === -1) {
      lines.push(decl(i));
    }
  }
  // A spanning structure first, so that the graph is mostly connected.
  for (let i = 1; i < n; i++) {
    if (R.chance(0.92)) {
      const from = R.int(Math.max(0, i - 9), i - 1);
      const label = R.chance(0.25) ? `|${R.label(2)}|` : '';
      lines.push(`  n${from} ${R.pick(LINKS)}${label} n${i}`);
    }
  }
  const extra = Math.floor(n * 0.3);
  for (let k = 0; k < extra; k++) {
    const a = R.int(0, n - 1);
    const b = R.int(0, n - 1);
    if (a !== b) {
      lines.push(`  n${a} ${R.pick(LINKS)} n${b}`);
    }
  }
  if (groups > 1 && R.chance(0.5)) {
    lines.push(`  g0 --> g${groups - 1}`);
  }
  return frontMatter(layout ? `  layout: ${layout}` : '') + lines.join('\n') + '\n';
}

function classDiagram(seed, n, layout) {
  const R = makeRand(seed);
  const lines = ['classDiagram'];
  const RELS = ['<|--', '*--', 'o--', '-->', '--', '..>', '..|>'];
  for (let i = 0; i < n; i++) {
    lines.push(`  class C${i} {`);
    const members = R.int(0, 4);
    for (let m = 0; m < members; m++) {
      if (R.chance(0.5)) {
        lines.push(`    +${R.pick(['int', 'string', 'bool', 'List~int~'])} ${R.pick(WORDS)}${m}`);
      } else {
        lines.push(`    +${R.pick(WORDS)}${m}(${R.chance(0.5) ? 'id' : ''}) ${R.pick(['void', 'int', 'bool'])}`);
      }
    }
    lines.push('  }');
  }
  for (let i = 1; i < n; i++) {
    if (R.chance(0.9)) {
      const from = R.int(Math.max(0, i - 7), i - 1);
      const label = R.chance(0.3) ? ` : ${R.pick(WORDS)}` : '';
      lines.push(`  C${from} ${R.pick(RELS)} C${i}${label}`);
    }
  }
  for (let k = 0; k < Math.floor(n * 0.2); k++) {
    const a = R.int(0, n - 1);
    const b = R.int(0, n - 1);
    if (a !== b) {
      lines.push(`  C${a} ${R.pick(RELS)} C${b}`);
    }
  }
  return frontMatter(layout ? `  layout: ${layout}` : '') + lines.join('\n') + '\n';
}

function stateDiagram(seed, n, layout) {
  const R = makeRand(seed);
  const lines = ['stateDiagram-v2'];
  const composites = R.int(1, Math.min(5, Math.floor(n / 8)));
  const groupOf = [];
  for (let i = 0; i < n; i++) {
    groupOf.push(R.chance(0.5) ? R.int(0, composites - 1) : -1);
  }
  lines.push('  [*] --> s0');
  for (let c = 0; c < composites; c++) {
    lines.push(`  state Group${c} {`);
    const inside = [];
    for (let i = 0; i < n; i++) {
      if (groupOf[i] === c) {
        inside.push(i);
      }
    }
    for (let k = 1; k < inside.length; k++) {
      lines.push(`    s${inside[k - 1]} --> s${inside[k]}${R.chance(0.3) ? ` : ${R.pick(WORDS)}` : ''}`);
    }
    if (inside.length === 1) {
      lines.push(`    s${inside[0]}`);
    }
    lines.push('  }');
  }
  for (let i = 1; i < n; i++) {
    if (R.chance(0.7)) {
      const from = R.int(Math.max(0, i - 6), i - 1);
      lines.push(`  s${from} --> s${i}${R.chance(0.3) ? ` : ${R.label(2)}` : ''}`);
    }
  }
  lines.push(`  s${n - 1} --> [*]`);
  return frontMatter(layout ? `  layout: ${layout}` : '') + lines.join('\n') + '\n';
}

function erDiagram(seed, n, layout) {
  const R = makeRand(seed);
  const lines = ['erDiagram'];
  const CARD = ['||--o{', '||--||', '}o--o{', '||--|{', '}|..|{', '|o--o|'];
  for (let i = 0; i < n; i++) {
    lines.push(`  E${i} {`);
    const attrs = R.int(1, 5);
    for (let a = 0; a < attrs; a++) {
      lines.push(`    ${R.pick(['int', 'string', 'date', 'float'])} ${R.pick(WORDS)}_${a}${a === 0 ? ' PK' : R.chance(0.2) ? ' FK' : ''}`);
    }
    lines.push('  }');
  }
  for (let i = 1; i < n; i++) {
    if (R.chance(0.9)) {
      const from = R.int(Math.max(0, i - 6), i - 1);
      lines.push(`  E${from} ${R.pick(CARD)} E${i} : ${R.pick(WORDS)}`);
    }
  }
  for (let k = 0; k < Math.floor(n * 0.15); k++) {
    const a = R.int(0, n - 1);
    const b = R.int(0, n - 1);
    if (a !== b) {
      lines.push(`  E${a} ${R.pick(CARD)} E${b} : ${R.pick(WORDS)}`);
    }
  }
  return frontMatter(layout ? `  layout: ${layout}` : '') + lines.join('\n') + '\n';
}

fs.rmSync(outDir, { recursive: true, force: true });
let count = 0;
function write(dir, name, text) {
  const file = path.join(outDir, dir, `${name}.mmd`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, text);
  count++;
}
const pad = (n) => String(n).padStart(3, '0');
const flowSizes = [30, 36, 40, 41, 48, 55, 64, 72, 80, 96, 110, 128, 150];
let seed = 0x6d6d6467;
for (const dir of ['TD', 'LR']) {
  for (const n of flowSizes) {
    write('flowchart-elk', `${dir.toLowerCase()}-${pad(n)}`, flowchart(seed++, n, dir, ''));
  }
}
for (const n of [30, 40, 48, 64, 80, 110]) {
  write('flowchart-dagre', `td-${pad(n)}`, flowchart(seed++, n, 'TD', 'dagre'));
  write('flowchart-dagre', `lr-${pad(n)}`, flowchart(seed++, n, 'LR', 'dagre'));
}
for (const n of [20, 28, 36, 44, 52, 60]) {
  write('class', `elk-${pad(n)}`, classDiagram(seed++, n, ''));
  write('class', `dagre-${pad(n)}`, classDiagram(seed++, n, 'dagre'));
}
for (const n of [24, 32, 40, 50, 64]) {
  write('state', `elk-${pad(n)}`, stateDiagram(seed++, n, ''));
  write('state', `dagre-${pad(n)}`, stateDiagram(seed++, n, 'dagre'));
}
for (const n of [15, 22, 30, 40, 48]) {
  write('er', `elk-${pad(n)}`, erDiagram(seed++, n, ''));
  write('er', `dagre-${pad(n)}`, erDiagram(seed++, n, 'dagre'));
}
console.log(`${count} diagrams in ${outDir}`);
