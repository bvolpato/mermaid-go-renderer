// Reports which code of cytoscape.bundle.js the layouts of mermaid reach.
//
//   node cytoscape-native-coverage.mjs [--only cose-bilkent|fcose] [--random N] [--general] [--out file]
//
// It runs the 53 recorded layouts, and N random inputs of each kind, with
// the block coverage of V8. One realm runs every layout, and the counters
// start after its load, so the report has the code of the layouts and not
// the code that runs when the script loads. The output file is the bundle
// with a mark on each line: '+' for a line that ran, '~' for a line that ran
// in part, '-' for a line that did not run inside a function that ran, and
// ' ' for a line of a function that never ran.
import fs from 'node:fs';
import path from 'node:path';
import inspector from 'node:inspector';
import { pkgDir, loadGolden, recordInput, recordRealm, checkRecord, newRealm, mulberry32 } from './cytoscape-native-lib.mjs';
import { randomMindmap, randomArchitecture } from './cytoscape-native-random.mjs';

const args = process.argv.slice(2);
const flag = (name, def) => {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : def;
};
const randomCount = Number(flag('--random', '0'));
const out = flag('--out', '/tmp/cytoscape-coverage.txt');
const only = flag('--only', '');
const general = args.includes('--general');

const session = new inspector.Session();
session.connect();
const post = (method, params) => new Promise((resolve, reject) => session.post(method, params, (err, res) => (err ? reject(err) : resolve(res))));
await post('Profiler.enable');
await post('Profiler.startPreciseCoverage', { callCount: true, detailed: true });

const golden = loadGolden();
let bad = 0;
for (const rec of golden.records) {
  if (only && rec.layout !== only) {
    continue;
  }
  const realm = recordRealm(rec);
  const { fn, arg } = recordInput(rec);
  const res = realm.api[fn](arg);
  const diffs = checkRecord(rec, res);
  if (diffs.length > 0 || realm.random.calls !== rec.random) {
    bad++;
    console.log(rec.name, 'differs from the record:', diffs.slice(0, 3), realm.random.calls, rec.random);
  }
}
console.log(`${golden.records.length} records, ${bad} differ in node`);

// The report is about the code that a layout runs, not about the code that
// runs when the script loads: one realm runs every layout, and the counters
// start after its load.
let random = mulberry32(1);
let widths = null;
const shared = newRealm({
  random: () => random(),
  measureText: (font, text) => (widths && widths.has(font + '\n' + text) ? widths.get(font + '\n' + text) : text.length * 7.5),
});
shared.api.coseBilkent({ nodes: [], edges: [] });
shared.api.architecture({
  config: { iconSize: 80, fontSize: 16, padding: 40, idealEdgeLengthMultiplier: 1.5, edgeElasticity: 0.45, randomize: false, nodeSeparation: 75, numIter: 2500, seed: 1 },
  groups: [],
  services: [],
  junctions: [],
  edges: [],
  alignmentConstraint: { horizontal: [], vertical: [] },
  relativePlacementConstraint: [],
});
await post('Profiler.takePreciseCoverage');
for (const rec of golden.records) {
  if (only && rec.layout !== only) {
    continue;
  }
  random = mulberry32(0x6d6d6467);
  widths = new Map(rec.measure.map((m) => [m[0] + '\n' + m[1], m[2]]));
  const { fn, arg } = recordInput(rec);
  shared.api[fn](arg);
}
widths = null;
let failed = 0;
for (let i = 0; i < randomCount; i++) {
  for (const kind of ['mindmap', 'architecture']) {
    if (only && (only === 'fcose') !== (kind === 'architecture')) {
      continue;
    }
    const rng = mulberry32(1000 + i);
    const input = kind === 'mindmap' ? randomMindmap(rng, { general: general && i % 2 === 1 }) : randomArchitecture(rng);
    random = mulberry32(7 + i);
    try {
      shared.api[kind === 'mindmap' ? 'coseBilkent' : 'architecture'](input);
    } catch (err) {
      failed++;
      if (failed < 5) {
        console.log('random input', i, kind, 'threw', String(err).slice(0, 200));
      }
    }
  }
}
if (randomCount > 0) {
  console.log(`${randomCount} random inputs of each kind, ${failed} threw`);
}

const { result } = await post('Profiler.takePreciseCoverage');
const bundleFile = path.join(pkgDir, 'cytoscape.bundle.js');
const source = fs.readFileSync(bundleFile, 'utf8');
// The state of each character: 1 ran, 2 did not run inside a function that
// ran, 3 is in a function that never ran. Each context that ran the script
// can report its own counts: a character ran when it ran in any of them.
const state = new Uint8Array(source.length);
for (const s of result) {
  if (s.url !== bundleFile && s.url !== 'file://' + bundleFile) {
    continue;
  }
  const local = new Uint8Array(source.length);
  // The ranges of nested functions come outer first, so an inner one wins.
  const fns = s.functions.slice().sort((a, b) => a.ranges[0].startOffset - b.ranges[0].startOffset || b.ranges[0].endOffset - a.ranges[0].endOffset);
  for (const f of fns) {
    if (f.ranges[0].count === 0) {
      local.fill(3, f.ranges[0].startOffset, f.ranges[0].endOffset);
      continue;
    }
    for (const r of f.ranges) {
      local.fill(r.count > 0 ? 1 : 2, r.startOffset, r.endOffset);
    }
  }
  for (let i = 0; i < source.length; i++) {
    const v = local[i];
    if (v !== 0 && (state[i] === 0 || v < state[i])) {
      state[i] = v;
    }
  }
}
const lines = source.split('\n');
let offset = 0;
const marks = [];
for (const line of lines) {
  let ran = 0;
  let not = 0;
  let never = 0;
  for (let i = 0; i < line.length; i++) {
    const c = line.charCodeAt(i);
    if (c === 32 || c === 9) {
      continue;
    }
    const v = state[offset + i];
    if (v === 1) {
      ran++;
    } else if (v === 2) {
      not++;
    } else if (v === 3) {
      never++;
    }
  }
  marks.push(ran + not + never === 0 ? ' ' : ran > 0 && not + never === 0 ? '+' : ran > 0 ? '~' : not > 0 ? '-' : 'x');
  offset += line.length + 1;
}
fs.writeFileSync(out, lines.map((l, i) => `${marks[i]}${String(i + 1).padStart(6)} ${l}`).join('\n'));
const sections = [
  ['cytoscape', 35, 35716],
  ['layout-base 1.0.2', 35717, 40054],
  ['cose-base 1.0.3', 40055, 41505],
  ['cytoscape-cose-bilkent 4.1.0', 41506, 41967],
  ['layout-base 2.0.1', 41968, 47201],
  ['cose-base 2.2.0', 47202, 50419],
  ['cytoscape-fcose 2.2.0', 50420, 51982],
];
for (const [name, from, to] of sections) {
  let n = 0;
  for (let i = from - 1; i < to; i++) {
    if (marks[i] === '+' || marks[i] === '~') {
      n++;
    }
  }
  console.log(`${name}: ${n} of ${to - from + 1} lines ran`);
}
console.log('wrote', out);
