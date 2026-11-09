// Phase oracle for internal/elk/native: runs the unminified elkjs 0.9.3 and
// records the state of the LGraph before and after each layout processor of
// the layered algorithm.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-phases.mjs             write the phase goldens
//   node gen-elk-phases.mjs --scope     print the options, the features and
//                                       the processors of the mermaid graphs
//   node gen-elk-phases.mjs --verify    check that the unminified worker and
//                                       the minified one give the same output
//   node gen-elk-phases.mjs --dump FILE print the steps of one graph (a JSON
//                                       file with an ELK graph) as JSON
//
// The goldens are internal/elk/native/nativetest/testdata/phases/*.json.gz,
// one file for each graph, and index.json. A file has the ELK graph, the
// layout result and, for each run of the layered algorithm, the list of
// steps. A step names the processor and has the state before and after it.
// A state that is equal to the state before it is stored one time.
//
// The state format is documented in internal/elk/native/nativetest/state.go.
import { gunzipSync, gzipSync } from 'node:zlib';
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadElk, env, stripHash } from './elk-phases-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const outDir = resolve(root, 'internal/elk/native/nativetest/testdata/phases');

// ---------------------------------------------------------------------------
// The state dump.

const JAVA = {
  LGraph: 'org.eclipse.elk.alg.layered.graph.LGraph',
  LNode: 'org.eclipse.elk.alg.layered.graph.LNode',
  LPort: 'org.eclipse.elk.alg.layered.graph.LPort',
  LEdge: 'org.eclipse.elk.alg.layered.graph.LEdge',
  LLabel: 'org.eclipse.elk.alg.layered.graph.LLabel',
  Layer: 'org.eclipse.elk.alg.layered.graph.Layer',
};
const KIND = { [JAVA.LGraph]: 'g', [JAVA.LNode]: 'n', [JAVA.LPort]: 'p', [JAVA.LEdge]: 'e', [JAVA.LLabel]: 'l' };
const ELK_KIND = {
  'org.eclipse.elk.graph.impl.ElkNodeImpl': 'n',
  'org.eclipse.elk.graph.impl.ElkPortImpl': 'p',
  'org.eclipse.elk.graph.impl.ElkEdgeImpl': 'e',
  'org.eclipse.elk.graph.impl.ElkLabelImpl': 'l',
};
const SPACING = new Set([
  'org.eclipse.elk.core.math.ElkPadding',
  'org.eclipse.elk.core.math.ElkMargin',
  'org.eclipse.elk.alg.layered.graph.LPadding',
  'org.eclipse.elk.alg.layered.graph.LMargin',
]);
// Properties that are not state: the processor list of a graph.
const SKIP_PROPS = new Set(['processors']);

// num keeps a double exact in JSON.
function num(x) {
  if (Number.isFinite(x) && !Object.is(x, -0)) return x;
  return { $f: Object.is(x, -0) ? '-0' : String(x) };
}
const vec = (v) => [num(v.x_0), num(v.y_0)];
const isZeroVec = (v) => Object.is(v.x_0, 0) && Object.is(v.y_0, 0);
const spacing = (s) => [num(s.top_0), num(s.right), num(s.bottom), num(s.left)];
const isZeroSpacing = (s) => Object.is(s.top_0, 0) && Object.is(s.right, 0) && Object.is(s.bottom, 0) && Object.is(s.left, 0);

function makeDumper(S) {
  const getName = S('$getName');
  const getClass = S('getClass__Ljava_lang_Class___devirtual$');
  const className = (o) => {
    try {
      return getName(getClass(o));
    } catch (e) {
      return '?';
    }
  };
  const iterate = (coll) => {
    const out = [];
    for (const it = coll.iterator_0(); it.hasNext_0(); ) out.push(it.next_1());
    return out;
  };

  // dump returns the state of the LGraph hierarchy below graph.
  return function dump(graph) {
    const tables = { g: [], n: [], p: [], e: [], l: [] };
    const ids = new Map();
    const queue = [];
    const objects = new Map();

    const ref = (o) => {
      if (o === null || o === undefined) return -1;
      let id = ids.get(o);
      if (id === undefined) {
        const kind = KIND[className(o)];
        id = tables[kind].length;
        tables[kind].push(null);
        ids.set(o, id);
        queue.push([kind, id, o]);
      }
      return id;
    };
    const refs = (list) => list.array.map(ref);

    // elkPath names an element of the ELK graph.
    const elkPath = (o, kind) => {
      if (kind !== 'l') return `${kind}:${o.identifier}`;
      const parent = o.eContainer_0 ? o.eContainer_0() : null;
      let index = -1;
      let parentPath = '?';
      if (parent) {
        const pk = ELK_KIND[className(parent)];
        parentPath = elkPath(parent, pk);
        index = iterate(parent.labels).indexOf(o);
      }
      return `l:${parentPath}#${index}`;
    };

    const value = (v, depth = 0) => {
      if (v === null || v === undefined) return null;
      switch (typeof v) {
        case 'boolean':
          return v;
        case 'number':
          return num(v);
        case 'string':
          return { $s: v };
        case 'function':
          return { $fn: 1 };
      }
      if (Array.isArray(v)) return { $arr: v.map((x) => value(x, depth + 1)) };
      const cls = className(v);
      const kind = KIND[cls];
      if (kind) return { ['$' + kind]: ref(v) };
      if (cls === JAVA.Layer) return { $layer: [ref(v.owner), v.owner.layers.array.indexOf(v)] };
      if (ELK_KIND[cls]) return { $elk: elkPath(v, ELK_KIND[cls]) };
      if (cls === 'java.lang.Integer' || cls === 'java.lang.Short' || cls === 'java.lang.Byte') return v.value_0;
      if (cls === 'org.eclipse.elk.core.math.KVector') return { $v: vec(v) };
      if (cls === 'org.eclipse.elk.core.math.KVectorChain') return { $vc: iterate(v).map(vec) };
      if (SPACING.has(cls)) return { $sp: spacing(v) };
      if (cls === 'java.util.Random') return { $rnd: [v.seedhi, v.seedlo] };
      if (cls === 'org.eclipse.elk.alg.layered.options.Spacings') return { $spacings: 1 };
      if (cls === 'org.eclipse.elk.core.data.LayoutAlgorithmData') return { $algo: v.id_0 };
      if (cls === 'org.eclipse.elk.core.util.Pair') return { $pair: [value(v.first, depth + 1), value(v.second, depth + 1)] };
      if (typeof v.ordinal === 'number' && typeof v.name_0 === 'string') return v.name_0;
      if (cls === 'java.util.EnumSet$EnumSetImpl') return { $es: iterate(v).map((x) => x.name_0) };
      if (typeof v.asMap_0 === 'function') {
        const entries = iterate(v.asMap_0().entrySet_0());
        return { $mm: entries.map((en) => [value(en.getKey(), depth + 1), iterate(en.getValue()).map((x) => value(x, depth + 1))]) };
      }
      if (typeof v.entrySet_0 === 'function') {
        return { $map: iterate(v.entrySet_0()).map((en) => [value(en.getKey(), depth + 1), value(en.getValue(), depth + 1)]) };
      }
      if (typeof v.iterator_0 === 'function') {
        const items = iterate(v).map((x) => value(x, depth + 1));
        return /Set/.test(cls) ? { $set: items } : { $list: items };
      }
      // Any other Java object: its fields, one time for each object.
      let oid = objects.get(v);
      if (oid !== undefined) return { $o: oid };
      oid = objects.size;
      objects.set(v, oid);
      const out = { $o: oid, c: cls.replace(/^org\.eclipse\.elk\./, '') };
      if (depth > 12) return out;
      const f = {};
      for (const key of Object.keys(v)) {
        if (key === '$H' || key === '___clazz' || key === 'propertyMap') continue;
        const enc = value(v[key], depth + 1);
        if (enc !== null) f[key] = enc;
      }
      if (v.propertyMap) f.$props = props(v, depth + 1);
      out.f = f;
      return out;
    };

    const props = (holder, depth = 0) => {
      if (!holder.propertyMap) return undefined;
      const rows = [];
      for (const en of iterate(holder.propertyMap.entrySet_0())) {
        const id = en.getKey().id_0;
        if (SKIP_PROPS.has(id)) continue;
        rows.push([id, en.getValue()]);
      }
      if (rows.length === 0) return undefined;
      rows.sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0));
      const out = {};
      for (const [id, v] of rows) {
        const enc = value(v, depth);
        if (enc !== null) out[id] = enc;
      }
      return out;
    };

    const put = (out, key, v, skip) => {
      if (v !== undefined && !skip) out[key] = v;
    };

    const fill = {
      g(o) {
        const out = {};
        put(out, 'id', o.id_0, o.id_0 === 0);
        put(out, 'sz', vec(o.size_0), isZeroVec(o.size_0));
        put(out, 'pad', spacing(o.padding), isZeroSpacing(o.padding));
        put(out, 'off', vec(o.offset), isZeroVec(o.offset));
        put(out, 'll', refs(o.layerlessNodes), o.layerlessNodes.array.length === 0);
        const layers = o.layers.array.map((layer) => {
          const lo = {};
          put(lo, 'id', layer.id_0, layer.id_0 === 0);
          put(lo, 'sz', vec(layer.size_0), isZeroVec(layer.size_0));
          lo.n = refs(layer.nodes);
          put(lo, 'pr', props(layer));
          return lo;
        });
        put(out, 'ly', layers, layers.length === 0);
        put(out, 'pn', ref(o.parentNode), !o.parentNode);
        put(out, 'pr', props(o));
        return out;
      },
      n(o) {
        const out = {};
        put(out, 'id', o.id_0, o.id_0 === 0);
        put(out, 't', o.type_0.name_0, o.type_0.name_0 === 'NORMAL');
        put(out, 'pos', vec(o.pos), isZeroVec(o.pos));
        put(out, 'sz', vec(o.size_0), isZeroVec(o.size_0));
        put(out, 'mg', spacing(o.margin), isZeroSpacing(o.margin));
        put(out, 'pad', spacing(o.padding), isZeroSpacing(o.padding));
        out.g = ref(o.graph_0);
        // The layer of the node: the graph of the layer and the index of the
        // layer in that graph, or -1 when the graph does not have the layer.
        if (o.layer) out.ly = [ref(o.layer.owner), o.layer.owner.layers.array.indexOf(o.layer)];
        put(out, 'po', refs(o.ports), o.ports.array.length === 0);
        put(out, 'lb', refs(o.labels), o.labels.array.length === 0);
        put(out, 'ng', ref(o.nestedGraph), !o.nestedGraph);
        put(out, 'psc', 1, !o.portSidesCached);
        if (o.portSideIndices) {
          const rows = [];
          for (const en of iterate(o.portSideIndices.entrySet_0())) {
            rows.push([en.getKey().name_0, en.getValue().first.value_0, en.getValue().second.value_0]);
          }
          out.psi = rows;
        }
        put(out, 'pr', props(o));
        return out;
      },
      p(o) {
        const out = {};
        put(out, 'id', o.id_0, o.id_0 === 0);
        out.o = ref(o.owner);
        put(out, 'sd', o.side.name_0, o.side.name_0 === 'UNDEFINED');
        put(out, 'pos', vec(o.pos), isZeroVec(o.pos));
        put(out, 'sz', vec(o.size_0), isZeroVec(o.size_0));
        put(out, 'an', vec(o.anchor), isZeroVec(o.anchor));
        put(out, 'ea', 1, !o.explicitlySuppliedPortAnchor);
        put(out, 'nx', 1, o.connectedToExternalNodes);
        put(out, 'lb', refs(o.labels), o.labels.array.length === 0);
        put(out, 'in', refs(o.incomingEdges), o.incomingEdges.array.length === 0);
        put(out, 'out', refs(o.outgoingEdges), o.outgoingEdges.array.length === 0);
        put(out, 'pr', props(o));
        return out;
      },
      e(o) {
        const out = {};
        put(out, 'id', o.id_0, o.id_0 === 0);
        out.s = ref(o.source);
        out.t = ref(o.target);
        const bends = iterate(o.bendPoints).map(vec);
        put(out, 'bp', bends, bends.length === 0);
        put(out, 'lb', refs(o.labels), o.labels.array.length === 0);
        put(out, 'pr', props(o));
        return out;
      },
      l(o) {
        const out = {};
        put(out, 'id', o.id_0, o.id_0 === 0);
        put(out, 'tx', o.text_0, o.text_0 === null || o.text_0 === undefined);
        put(out, 'pos', vec(o.pos), isZeroVec(o.pos));
        put(out, 'sz', vec(o.size_0), isZeroVec(o.size_0));
        put(out, 'pr', props(o));
        return out;
      },
    };

    ref(graph);
    while (queue.length > 0) {
      const [kind, id, o] = queue.shift();
      tables[kind][id] = fill[kind](o);
    }
    return tables;
  };
}

// ---------------------------------------------------------------------------
// One instrumented engine. Each layout runs in the deterministic environment
// of gen-elk.mjs.

function newRecorder() {
  env.install();
  const elk = loadElk();
  const S = elk.hook.scope;
  const dump = makeDumper(S);
  const getName = S('$getName');
  const getClass = S('getClass__Ljava_lang_Class___devirtual$');
  // The name of a step: the simple name of the Java class. A class that one
  // strategy makes with a parameter gets the parameter after a colon:
  // GraphTransformer:TO_INTERNAL_LTR, GraphTransformer:TO_INPUT_DIRECTION,
  // LayerSweepCrossingMinimizer (BARYCENTER),
  // LayerSweepCrossingMinimizer:TWO_SIDED_GREEDY_SWITCH and
  // LayerSweepCrossingMinimizer:ONE_SIDED_GREEDY_SWITCH.
  const procName = (p) => {
    if (typeof p === 'string') return p;
    const name = getName(getClass(p)).replace(/^.*\./, '');
    if (name === 'GraphTransformer') return `${name}:${p.mode.name_0}`;
    if (name === 'LayerSweepCrossingMinimizer' && p.crossMinType.name_0 !== 'BARYCENTER') return `${name}:${p.crossMinType.name_0}`;
    return name;
  };

  let rec = null;
  let run = null;
  let draws = 0;
  // The state of a step is the whole hierarchy in a compound layout, and the
  // processed graph (a connected component) in a plain layout.
  const plain = (kind) => kind === 'layout' || kind === 'configure-plain';
  const stateRoot = (kind, graph) => (plain(kind) ? graph : run.root);
  elk.hook.random = () => {
    draws++;
  };
  elk.hook.algorithm = (node, data) => {
    if (rec) rec.algorithms.push(data.id_0 ?? data.id ?? String(data));
  };
  elk.hook.runStart = (elkgraph) => {
    if (!rec) return;
    run = { elk: elkgraph.identifier, steps: [], states: [], root: null, components: null };
    rec.runs.push(run);
  };
  elk.hook.imported = (elkgraph, lgraph) => {
    if (!rec) return;
    run.root = lgraph;
    if (rec.states) run.imported = pushState(dump(lgraph));
  };
  // The split into connected components: the state of the graph before, and
  // the state of each component after.
  elk.hook.components = (lgraph, comps) => {
    if (!rec) return;
    run.components = comps.size_1();
    run.comps = [];
    for (const it = comps.iterator_0(); it.hasNext_0(); ) run.comps.push(it.next_1());
    const step = { kind: 'split', proc: 'ComponentsProcessor.split', slot: -4, draws, drawsAfter: draws, graph: 0 };
    run.steps.push(step);
    if (rec.states) {
      // A graph with one component is its own component, and the split
      // changed nothing or moved nothing.
      step.afterList = run.comps.map((c) => pushState(dump(c)));
    }
  };
  // ComponentsProcessor.combine: the states of the components before, and
  // the state of the graph after.
  elk.hook.combine = (lgraph, comps, done) => {
    if (!rec) return;
    if (!done) {
      const step = { kind: 'combine', proc: 'ComponentsProcessor.combine', slot: -5, draws, graph: 0 };
      run.steps.push(step);
      if (rec.states) step.beforeList = run.comps.map((c) => pushState(dump(c)));
      return;
    }
    const step = run.steps[run.steps.length - 1];
    step.drawsAfter = draws;
    if (rec.states) step.after = pushState(dump(lgraph));
  };
  elk.hook.laidOut = (elkgraph, lgraph) => {
    if (!rec) return;
    if (rec.states) run.final = pushState(dump(lgraph));
    run = null;
  };
  // pushState stores a state one time when it is equal to the last one.
  const pushState = (state) => {
    const text = JSON.stringify(state);
    if (run.lastText === text) return run.states.length - 1;
    run.lastText = text;
    run.states.push(text);
    return run.states.length - 1;
  };
  elk.hook.before = (kind, p, graph, slot) => {
    if (!rec) return;
    const step = { kind, proc: procName(p), slot, draws };
    run.steps.push(step);
    if (rec.states) {
      const tables = dump(stateRoot(kind, graph));
      // The index of the processed graph in the table of graphs. The state of
      // a component is rooted at the component, so it is 0 there.
      step.before = pushState(tables);
      step.graph = plain(kind) ? 0 : graphIndex(run.root, graph);
      if (kind === 'layout' && run.comps) step.comp = run.comps.indexOf(graph);
    }
  };
  elk.hook.after = (kind, p, graph, slot) => {
    if (!rec) return;
    const step = run.steps[run.steps.length - 1];
    step.drawsAfter = draws;
    if (rec.states) step.after = pushState(dump(stateRoot(kind, graph)));
  };
  // graphIndex is the id that the dump gives to graph: the order of a
  // breadth-first walk over layerless nodes, layers and nested graphs is the
  // order of the dump only for simple cases, so ask the dump.
  const graphIndex = (rootGraph, graph) => {
    if (rootGraph === graph) return 0;
    const probe = graph.id_0;
    // Mark the graph through its id for one dump.
    graph.id_0 = -987654321;
    const tables = dump(rootGraph);
    graph.id_0 = probe;
    return tables.g.findIndex((g) => g && g.id === -987654321);
  };

  return {
    // layout returns { out | err, runs, algorithms }.
    layout(input, { states = true } = {}) {
      rec = { runs: [], algorithms: [], states };
      draws = 0;
      env.beginLayout();
      const r = rec;
      try {
        r.out = JSON.stringify(elk.layout(JSON.parse(input)));
      } catch (e) {
        r.err = String(e && e.message !== undefined ? e.message : e);
      }
      rec = null;
      run = null;
      r.mathRandom = env.mathRandomCalls;
      return r;
    },
  };
}

// ---------------------------------------------------------------------------
// The graphs: the ELK inputs that mermaid produced for the corpus, and the
// graphs of gen-elk.mjs.

function readLines(file) {
  return gunzipSync(readFileSync(file)).toString().split('\n').filter(Boolean);
}

function mermaidPopulation() {
  const out = [];
  for (const line of readLines(resolve(root, 'internal/ru/layout/elk/testdata/fixtures.jsonl.gz'))) {
    const recd = JSON.parse(line);
    for (const [ri, r] of (recd.renders ?? []).entries()) {
      for (const [ci, c] of (r.elk ?? []).entries()) {
        if (c.in !== undefined) out.push({ name: `${recd.name}#${ri}.${ci}`, input: c.in, out: c.out, source: 'mermaid' });
      }
    }
  }
  return out;
}

function goldenGraphs() {
  const out = [];
  for (const line of readLines(resolve(root, 'internal/elk/testdata/golden.jsonl.gz'))) {
    // The input is raw JSON in the line. Keep its bytes.
    const m = /^\{"name":("(?:[^"\\]|\\.)*"),"nodes":(\d+),"ms":([\d.e+-]+),"in":/.exec(line);
    const c = JSON.parse(line);
    if (c.hang || c.err !== undefined) continue;
    out.push({ name: JSON.parse(m[1]), input: JSON.stringify(c.in), out: JSON.stringify(c.out), source: 'golden', ms: c.ms });
  }
  return out;
}

const count = (m, k, n = 1) => m.set(k, (m.get(k) ?? 0) + n);
const table = (title, m, byKey = false) => {
  console.log(title);
  const rows = [...m].sort(byKey ? (a, b) => (a[0] < b[0] ? -1 : 1) : (a, b) => b[1] - a[1] || (a[0] < b[0] ? -1 : 1));
  for (const [k, v] of rows) console.log(String(v).padStart(7), k);
};

// ---------------------------------------------------------------------------
// --scope

function scope() {
  const recorder = newRecorder();
  const pop = mermaidPopulation();
  const procs = new Map();
  const algos = new Map();
  const options = new Map();
  const features = new Map();
  const pipelines = new Map();
  const totals = { graphs: 0, layeredRuns: 0, compoundRuns: 0, plainRuns: 0, multiComponent: 0, random: 0, mathRandom: 0, errors: 0 };
  for (const c of pop) {
    totals.graphs++;
    const r = recorder.layout(c.input, { states: false });
    if (r.err !== undefined) totals.errors++;
    if (c.out !== undefined && stripHash(r.out) !== stripHash(c.out)) throw new Error(`${c.name}: the unminified worker differs from the recorded output`);
    for (const a of new Set(r.algorithms)) count(algos, a);
    const seen = new Set();
    let draws = 0;
    for (const run of r.runs) {
      totals.layeredRuns++;
      if (run.steps.some((s) => s.kind === 'compound')) totals.compoundRuns++;
      else totals.plainRuns++;
      if ((run.components ?? 1) > 1) totals.multiComponent++;
      for (const s of run.steps) {
        seen.add(s.proc);
        draws = Math.max(draws, s.drawsAfter ?? 0);
      }
      count(pipelines, [...new Set(run.steps.map((s) => s.proc))].join(' '));
    }
    for (const p of seen) count(procs, p);
    if (draws > 0) totals.random++;
    if (r.mathRandom > 0) totals.mathRandom++;

    const f = new Set();
    const optionValue = (k, v) => (/^nodeSize\.minimum$|minimum$/.test(k) ? '(w, h)' : /padding$/i.test(k) && !/portsSurrounding/.test(k) ? String(v).replace(/\d+(\.\d+)?/g, 'N') : String(v));
    const walk = (node, depth) => {
      const where = depth === 0 ? 'root' : Array.isArray(node.children) ? 'group' : 'node';
      for (const [k, v] of Object.entries(node.layoutOptions ?? {})) count(options, `${where}: ${k} = ${optionValue(k, v)}`);
      if (node.ports?.length) f.add('ports');
      if (depth > 0 && node.labels?.length) f.add(Array.isArray(node.children) ? 'label on a group' : 'label on a node');
      if (depth > 0 && node.children?.length) f.add('hierarchy (a node with children)');
      if (depth > 1 && node.children?.length) f.add('hierarchy with 2 levels or more');
      if (depth > 0 && Array.isArray(node.children) && node.children.length === 0) f.add('group with an empty list of children');
      if (depth > 0 && node.edges?.length) f.add('edges that live in a child node');
      if (depth > 0 && (node.x !== undefined || node.y !== undefined)) f.add('node with x or y in the input');
      if (depth > 0 && node.children?.length && node.width !== undefined) f.add('group with a width in the input');
      for (const e of node.edges ?? []) {
        if (e.sources?.length !== 1 || e.targets?.length !== 1) f.add('hyperedge');
        if (e.sources?.[0] === e.targets?.[0]) f.add('self loop');
        for (const [k, v] of Object.entries(e.layoutOptions ?? {})) count(options, `edge: ${k} = ${v}`);
        if (e.sections) f.add('edge sections in the input');
        let sized = 0;
        for (const l of e.labels ?? []) {
          for (const [k, v] of Object.entries(l.layoutOptions ?? {})) count(options, `edge label: ${k} = ${v}`);
          if (l.text) sized++;
        }
        if (sized > 0) f.add('edge label with a text');
        if (sized > 1) f.add('edge with 2 labels or more with a text');
        if ((e.labels?.length ?? 0) > sized) f.add('edge label with an empty text');
      }
      for (const ch of node.children ?? []) walk(ch, depth + 1);
    };
    walk(JSON.parse(c.input), 0);
    for (const k of f) count(features, k);
  }
  console.log(JSON.stringify(totals));
  table('ALGORITHMS (graphs)', algos);
  table('PROCESSORS (graphs)', procs);
  table('FEATURES (graphs)', features);
  table('OPTIONS (elements)', options, true);
  table('PIPELINES (runs)', pipelines);
}

// ---------------------------------------------------------------------------
// --verify: the unminified worker against the minified one. Each engine gets
// its own pass, because Date.now of the environment counts calls.

function verify() {
  const cases = [...mermaidPopulation(), ...goldenGraphs().filter((c) => c.ms <= 400)];
  let differ = 0;
  for (const minified of [false, true]) {
    env.install();
    const elk = loadElk({ minified });
    for (const c of cases) {
      env.beginLayout();
      let out;
      try {
        out = JSON.stringify(elk.layout(JSON.parse(c.input)));
      } catch (e) {
        out = undefined;
      }
      if (stripHash(out) !== stripHash(c.out)) {
        differ++;
        if (differ <= 10) console.error(`${minified ? 'minified' : 'unminified'} differs from the recorded output: ${c.name}`);
      }
    }
  }
  console.log(`${cases.length} graphs, 2 engines, ${differ} outputs differ from the recorded output of the minified worker`);
  if (differ > 0) process.exitCode = 1;
}

// ---------------------------------------------------------------------------
// The goldens.

// The number of cases that each processor gets at least, when enough graphs
// run it.
const WANT_MERMAID = 28;
const WANT_OTHER = 10;
// A state larger than this is too large for a golden.
const MAX_STATE_BYTES = 60000;
const MAX_CASE_BYTES = 1500000;

function fileName(name) {
  return name.replace(/[^A-Za-z0-9_.-]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 90);
}

function generate() {
  const recorder = newRecorder();
  const mermaid = mermaidPopulation();
  const golden = goldenGraphs().filter((c) => c.ms <= 60);
  // Small graphs first: a small state is easy to debug.
  const order = (list) => list.map((c, i) => [c, i]).sort((a, b) => a[0].input.length - b[0].input.length || a[1] - b[1]).map((x) => x[0]);
  const candidates = [];
  const om = order(mermaid);
  const og = order(golden);
  // Interleave: two mermaid graphs, one other graph.
  for (let i = 0, j = 0; i < om.length || j < og.length; ) {
    if (i < om.length) candidates.push(om[i++]);
    if (i < om.length) candidates.push(om[i++]);
    if (j < og.length) candidates.push(og[j++]);
  }
  const mermaidProcs = new Set();
  const have = new Map();
  const index = { cases: [], processors: {} };
  rmSync(outDir, { recursive: true, force: true });
  mkdirSync(outDir, { recursive: true });
  let total = 0;
  const used = new Set();
  let scanned = 0;
  for (const c of candidates) {
    scanned++;
    // First pass without states: which processors run.
    const probe = recorder.layout(c.input, { states: false });
    if (probe.err !== undefined || probe.runs.length === 0) continue;
    if (stripHash(probe.out) !== stripHash(c.out)) throw new Error(`${c.name}: the unminified worker differs from the recorded output`);
    const procs = new Set(probe.runs.flatMap((r) => r.steps.map((s) => s.proc)));
    if (c.source === 'mermaid') for (const p of procs) mermaidProcs.add(p);
    const need = [...procs].some((p) => (have.get(p) ?? 0) < (mermaidProcs.has(p) ? WANT_MERMAID : WANT_OTHER));
    if (!need) continue;
    const r = recorder.layout(c.input);
    let bytes = 0;
    let tooLarge = false;
    for (const run of r.runs) {
      for (const s of run.states) {
        bytes += s.length;
        if (s.length > MAX_STATE_BYTES) tooLarge = true;
      }
    }
    if (tooLarge || bytes > MAX_CASE_BYTES) continue;
    let file = fileName(c.name);
    while (used.has(file)) file += '_';
    used.add(file);
    const runs = r.runs.map((run) => ({
      elk: run.elk,
      components: run.components ?? undefined,
      imported: run.imported,
      final: run.final,
      steps: run.steps.map((s) => ({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: s.before, after: s.after, beforeList: s.beforeList, afterList: s.afterList, draws: s.draws, drawsAfter: s.drawsAfter })),
      // The states are JSON texts already.
      states: '@@STATES@@',
    }));
    let body = JSON.stringify({ name: c.name, source: c.source, in: '@@IN@@', out: '@@OUT@@', runs });
    let k = 0;
    body = body.replace(/"@@STATES@@"/g, () => `[${r.runs[k++].states.join(',')}]`);
    body = body.replace('"@@IN@@"', () => c.input).replace('"@@OUT@@"', () => stripHash(r.out));
    const gz = gzipSync(Buffer.from(body), { level: 9 });
    writeFileSync(resolve(outDir, `${file}.json.gz`), gz);
    total += gz.length;
    index.cases.push({ file: `${file}.json.gz`, name: c.name, source: c.source, bytes: gz.length, runs: r.runs.length, steps: r.runs.reduce((a, run) => a + run.steps.length, 0) });
    for (const p of procs) {
      count(have, p);
      (index.processors[p] ??= []).push(`${file}.json.gz`);
    }
  }
  index.processors = Object.fromEntries(Object.entries(index.processors).sort((a, b) => (a[0] < b[0] ? -1 : 1)));
  writeFileSync(resolve(outDir, 'index.json'), JSON.stringify(index, null, 1) + '\n');
  console.log(`${index.cases.length} cases of ${scanned} graphs, ${(total / 1e6).toFixed(2)} MB gzip`);
  table('cases for each processor', have);
}

function dumpOne(file) {
  const recorder = newRecorder();
  const r = recorder.layout(readFileSync(file, 'utf8'));
  for (const run of r.runs) {
    run.states = run.states.map((s) => JSON.parse(s));
    delete run.root;
    delete run.comps;
    delete run.lastText;
  }
  console.log(JSON.stringify(r, null, 1));
}

if (process.argv.includes('--scope')) {
  scope();
} else if (process.argv.includes('--verify')) {
  verify();
} else if (process.argv.includes('--dump')) {
  dumpOne(process.argv[process.argv.indexOf('--dump') + 1]);
} else {
  generate();
}
