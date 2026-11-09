// Loads the unminified elkjs 0.9.3 worker (lib/elk-worker.js, the GWT "pretty"
// output with readable names) with hooks around each layout processor of the
// layered algorithm. gen-elk-phases.mjs uses it to write the phase goldens of
// internal/elk/native.
//
// The hooks are text patches of the worker source. Each patch must match
// exactly one time, so a different elkjs build fails loudly.
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';

const require = createRequire(import.meta.url);
export const WORKER_PATH = require.resolve('elkjs/lib/elk-worker.js');
export const WORKER_MIN_PATH = require.resolve('elkjs/lib/elk-worker.min.js');

// The deterministic environment of gen-elk.mjs and of the Go bridge.
export const FIXED_NOW = 1700000000000;
const RANDOM_SEED = 0x9e3779b9;

const PATCHES = [
  {
    // ElkLayered.layout(LGraph, IElkProgressMonitor): the processors of one
    // component of a graph without hierarchy.
    name: 'ElkLayered.layout',
    old: `    processor.process(lgraph, monitor.subTask(monitorProgress));
    ++slotIndex;`,
    new: `    $$hook.before('layout', processor, lgraph, slotIndex);
    processor.process(lgraph, monitor.subTask(monitorProgress));
    $$hook.after('layout', processor, lgraph, slotIndex);
    ++slotIndex;`,
  },
  {
    // ElkLayered.hierarchicalLayout: a hierarchy aware processor, which runs
    // for the root graph only.
    name: 'ElkLayered.hierarchicalLayout (hierarchy aware)',
    old: `          if (!graph.parentNode) {
            processor.process(graph, monitor.subTask(1));
            ++slotIndex;
            break;
          }`,
    new: `          if (!graph.parentNode) {
            $$hook.before('hierarchical', processor, graph, slotIndex);
            processor.process(graph, monitor.subTask(1));
            $$hook.after('hierarchical', processor, graph, slotIndex);
            ++slotIndex;
            break;
          }`,
  },
  {
    name: 'ElkLayered.hierarchicalLayout',
    old: `         else {
          processor.process(graph, monitor.subTask(1));
          ++slotIndex;
        }`,
    new: `         else {
          $$hook.before('hierarchical', processor, graph, slotIndex);
          processor.process(graph, monitor.subTask(1));
          $$hook.after('hierarchical', processor, graph, slotIndex);
          ++slotIndex;
        }`,
  },
  {
    // ElkLayered.doCompoundLayout: the compound graph preprocessor and
    // postprocessor are not in the processor list.
    name: 'ElkLayered.doCompoundLayout',
    old: `  $process_1(this$static.compoundGraphPreprocessor, lgraph, theMonitor.subTask(1));
  $hierarchicalLayout(this$static, lgraph, theMonitor.subTask(1));
  $process_0(lgraph, theMonitor.subTask(1));`,
    new: `  $$hook.before('compound', this$static.compoundGraphPreprocessor, lgraph, -1);
  $process_1(this$static.compoundGraphPreprocessor, lgraph, theMonitor.subTask(1));
  $$hook.after('compound', this$static.compoundGraphPreprocessor, lgraph, -1);
  $hierarchicalLayout(this$static, lgraph, theMonitor.subTask(1));
  $$hook.before('compound', 'CompoundGraphPostprocessor', lgraph, -2);
  $process_0(lgraph, theMonitor.subTask(1));
  $$hook.after('compound', 'CompoundGraphPostprocessor', lgraph, -2);`,
  },
  {
    // LayeredLayoutProvider.layout: one run of the layered algorithm.
    name: 'LayeredLayoutProvider.layout',
    old: `  layeredGraph = $importGraph_1(new ElkGraphImporter, elkgraph);
  maskUndefined($getProperty_0(elkgraph, ($clinit_LayeredOptions() , HIERARCHY_HANDLING))) === maskUndefined(($clinit_HierarchyHandling() , INCLUDE_CHILDREN))?$doCompoundLayout(this.elkLayered, layeredGraph, progressMonitor):$doLayout(this.elkLayered, layeredGraph, progressMonitor);`,
    new: `  $$hook.runStart(elkgraph);
  layeredGraph = $importGraph_1(new ElkGraphImporter, elkgraph);
  $$hook.imported(elkgraph, layeredGraph);
  maskUndefined($getProperty_0(elkgraph, ($clinit_LayeredOptions() , HIERARCHY_HANDLING))) === maskUndefined(($clinit_HierarchyHandling() , INCLUDE_CHILDREN))?$doCompoundLayout(this.elkLayered, layeredGraph, progressMonitor):$doLayout(this.elkLayered, layeredGraph, progressMonitor);
  $$hook.laidOut(elkgraph, layeredGraph);`,
  },
  {
    // ElkLayered.doLayout: the split into connected components.
    name: 'ElkLayered.doLayout',
    old: `  components = $split_2(this$static.componentsProcessor, lgraph);
  if (components.size_1() == 1) {`,
    new: `  components = $split_2(this$static.componentsProcessor, lgraph);
  $$hook.components(lgraph, components);
  if (components.size_1() == 1) {`,
  },
  {
    // GraphConfigurator.prepareGraphForLayout for a graph of a hierarchy.
    name: 'ElkLayered.hierarchicalLayout (configure)',
    old: `    $prepareGraphForLayout(this$static.graphConfigurator, g);`,
    new: `    $$hook.before('configure', 'GraphConfigurator', g, -3);
    $prepareGraphForLayout(this$static.graphConfigurator, g);
    $$hook.after('configure', 'GraphConfigurator', g, -3);`,
  },
  {
    // GraphConfigurator.prepareGraphForLayout for a graph without hierarchy.
    name: 'ElkLayered.doLayout (configure)',
    old: `  $prepareGraphForLayout(this$static.graphConfigurator, lgraph);
  components = $split_2(this$static.componentsProcessor, lgraph);
  $$hook.components(lgraph, components);`,
    new: `  $$hook.before('configure-plain', 'GraphConfigurator', lgraph, -3);
  $prepareGraphForLayout(this$static.graphConfigurator, lgraph);
  $$hook.after('configure-plain', 'GraphConfigurator', lgraph, -3);
  components = $split_2(this$static.componentsProcessor, lgraph);
  $$hook.components(lgraph, components);`,
  },
  {
    // ComponentsProcessor.combine and ElkLayered.resizeGraph.
    name: 'ElkLayered.doLayout (combine)',
    old: `  $combine(this$static.componentsProcessor, components, lgraph);
  $resizeGraph(lgraph);`,
    new: `  $$hook.combine(lgraph, components, false);
  $combine(this$static.componentsProcessor, components, lgraph);
  $$hook.combine(lgraph, components, true);
  $resizeGraph(lgraph);`,
  },
  {
    // java.util.Random.nextInternal: each draw of the generator.
    name: 'Random.nextInternal',
    old: `function $nextInternal(this$static, bits){
  var carry, dval, h, hi, l, lo;`,
    new: `function $nextInternal(this$static, bits){
  var carry, dval, h, hi, l, lo;
  $$hook.random(this$static, bits);`,
  },
  {
    // The layout algorithm that the recursive engine runs for a node.
    name: 'RecursiveGraphLayoutEngine.executeAlgorithm',
    old: `function $executeAlgorithm(layoutNode, algorithmData, progressMonitor){
  var exception, layoutProvider;`,
    new: `function $executeAlgorithm(layoutNode, algorithmData, progressMonitor){
  var exception, layoutProvider;
  $$hook.algorithm(layoutNode, algorithmData);`,
  },
];

// EXPORTS gives the hook access to functions and class objects of the worker
// scope. The names are the names of the pretty output.
const EXPORTS = [
  '$getName',
  'getClass__Ljava_lang_Class___devirtual$',
  '$getProperty',
  '$getAllProperties',
  'instanceOf',
  'toString_40',
];

function patchedSource() {
  let src = readFileSync(WORKER_PATH, 'utf8');
  for (const p of PATCHES) {
    const n = src.split(p.old).length - 1;
    if (n !== 1) {
      throw new Error(`patch ${p.name} matches ${n} times in elk-worker.js, want 1`);
    }
    src = src.replace(p.old, () => p.new);
  }
  // The script ends with gwtOnLoad(null, 'elk', null). The export object is
  // built after it, inside the same function scope.
  const tail = `\n;$$hook.scope = function (name) { return eval(name); };\n`;
  return src + tail;
}

let compiled;

// loadElk returns { layout(graph), hook } for a new instance of the engine.
// hook is the object whose methods the patched worker calls. Callers replace
// its methods.
export function loadElk({ minified = false } = {}) {
  const hook = {
    before() {},
    after() {},
    runStart() {},
    imported() {},
    laidOut() {},
    components() {},
    combine() {},
    random() {},
    algorithm() {},
    scope: null,
  };
  let src;
  if (minified) {
    src = readFileSync(WORKER_MIN_PATH, 'utf8');
  } else {
    compiled ??= patchedSource();
    src = compiled;
  }
  // The worker installs self.onmessage when there is no document and there is
  // a self. $wnd is the global object of node.
  env.newEngine();
  const self = {};
  const fn = new Function('self', '$$hook', 'module', 'exports', src);
  fn(self, hook, undefined, undefined);
  const dispatch = self.onmessage;
  if (typeof dispatch !== 'function') {
    throw new Error('the elkjs worker did not install onmessage');
  }
  let message;
  self.postMessage = (m) => {
    message = m;
  };
  const send = (data) => {
    message = undefined;
    dispatch({ data });
    const m = message;
    if (m === undefined) throw new Error('the elkjs worker did not answer');
    if (m.error) throw m.error;
    return m.data;
  };
  send({ id: 0, cmd: 'register', algorithms: ['layered', 'stress', 'mrtree', 'radial', 'force', 'disco', 'sporeOverlap', 'sporeCompaction', 'rectpacking'] });
  return {
    hook,
    // layout is ELK.layout of elk-api.js with the default arguments.
    layout(graph) {
      return send({ id: 0, cmd: 'layout', graph, layoutOptions: {}, options: { logging: false, measureExecutionTime: false } });
    },
  };
}

// ---------------------------------------------------------------------------
// Deterministic environment (see gen-elk.mjs).

const native = { random: Math.random, now: Date.now };
let randomState = RANDOM_SEED;
let nowCalls = 0;
let nowBase = 0;
export const env = {
  mathRandomCalls: 0,
  install() {
    Math.random = () => {
      env.mathRandomCalls++;
      randomState = (randomState + 0x6d2b79f5) >>> 0;
      let t = randomState;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
    Date.now = () => {
      nowCalls++;
      return FIXED_NOW - nowBase;
    };
  },
  restore() {
    Math.random = native.random;
    Date.now = native.now;
  },
  // newEngine resets the count of Date.now calls. A new worker starts its
  // uniqueSeed counter of java.util.Random at 0, and the clock of this
  // environment goes back by one for each earlier call, so both must start
  // together (gen-elk.mjs does the same in freshELK).
  newEngine() {
    nowCalls = 0;
    nowBase = 0;
  },
  // beginLayout resets the state that the Go bridge resets before a layout.
  beginLayout() {
    randomState = RANDOM_SEED;
    env.mathRandomCalls = 0;
    nowBase = nowCalls;
  },
};

export const stripHash = (s) => (s === undefined ? s : s.replace(/,"\$H":\d+/g, ''));
