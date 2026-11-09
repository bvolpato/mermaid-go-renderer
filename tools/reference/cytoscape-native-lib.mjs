// Runs internal/cytoscape (env.js, cytoscape.bundle.js, glue.js) in node, one
// vm context for each layout, as the Go package runs it in goja.
// cytoscape-native-coverage.mjs uses it to find the code that the layouts of
// mermaid reach: the Go ports of internal/cytoscape/native cover that code.
//
// node is not the oracle of the ports. Its Math.sin and Math.cos are not
// those of Chromium for some arguments, and the radial layout of
// cose-bilkent depends on them. The oracle is the interpreter of the Go
// package, with the Math functions of internal/js: see
// internal/cytoscape/native_golden_write_test.go.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
export const pkgDir = path.resolve(here, '../../internal/cytoscape');

const sources = {};
function script(name) {
  if (!sources[name]) {
    const file = path.join(pkgDir, name);
    sources[name] = new vm.Script(fs.readFileSync(file, 'utf8'), { filename: file });
  }
  return sources[name];
}

// mulberry32 is the generator of tools/reference/oracle.mjs.
export function mulberry32(seed) {
  let state = seed >>> 0;
  const fn = function () {
    fn.calls++;
    state = (state + 0x6d2b79f5) >>> 0;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  fn.calls = 0;
  return fn;
}

// newRealm loads the scripts in a new context. opts.random is Math.random,
// opts.measureText(font, text) the width of a label, opts.patch(modules,
// context) runs before the glue.
export function newRealm(opts = {}) {
  const context = vm.createContext({});
  const host = {
    math: {},
    random: opts.random || (() => { throw new Error('Math.random is not set'); }),
    now: () => 1768478400000,
    measureText: opts.measureText || (() => { throw new Error('measureText is not set'); }),
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/156.0.8078.4 Safari/537.36',
  };
  const env = script('env.js').runInContext(context)(host);
  const modules = script('cytoscape.bundle.js').runInContext(context)();
  if (opts.patch) {
    opts.patch(modules, context);
  }
  const api = script(opts.glue || 'glue.js').runInContext(context)(modules, env);
  return { context, modules, api, env };
}

export function loadGolden() {
  return JSON.parse(zlib.gunzipSync(fs.readFileSync(path.join(pkgDir, 'testdata/layouts.json.gz'))).toString('utf8'));
}

function decode(v) {
  if (v !== null && typeof v === 'object' && typeof v.$num === 'string') {
    return Number(v.$num === '-0' ? -0 : v.$num);
  }
  return v;
}

// recordInput builds the argument of the glue from a record of the golden
// file, as golden_test.go does.
export function recordInput(rec) {
  if (rec.layout === 'cose-bilkent') {
    const nodes = [];
    const edges = [];
    for (const a of rec.adds) {
      if (a.group === 'nodes') {
        nodes.push({ id: a.data.id, width: decode(a.data.width), height: decode(a.data.height), x: decode(a.position.x), y: decode(a.position.y) });
      } else {
        edges.push({ id: a.data.id, start: a.data.source, end: a.data.target });
      }
    }
    return { fn: 'coseBilkent', arg: { nodes, edges } };
  }
  const a = rec.architecture;
  const options = rec.layouts[0].options;
  const conv = (list) => list.map((n) => ({ id: n.id, icon: n.icon, title: n.title, in: n.in }));
  return {
    fn: 'architecture',
    arg: {
      config: a.config,
      groups: conv(a.groups),
      services: conv(a.services),
      junctions: conv(a.junctions),
      edges: a.edges.map((e) => ({ lhsId: e.lhsId, rhsId: e.rhsId, lhsDir: e.lhsDir, rhsDir: e.rhsDir, title: e.title })),
      alignmentConstraint: options.alignmentConstraint || { horizontal: [], vertical: [] },
      relativePlacementConstraint: (options.relativePlacementConstraint || []).map((r) => (r.left !== undefined || r.right !== undefined ? { left: r.left, right: r.right, gap: r.gap } : { top: r.top, bottom: r.bottom, gap: r.gap })),
    },
  };
}

// recordRealm is the page of the reference oracle for one record.
export function recordRealm(rec, opts = {}) {
  const random = mulberry32(0x6d6d6467);
  for (let i = 0; i < rec.randomBefore; i++) {
    random();
  }
  random.calls = 0;
  const widths = new Map();
  for (const m of rec.measure) {
    widths.set(m[0] + '\n' + m[1], m[2]);
  }
  const realm = newRealm({
    ...opts,
    random,
    measureText: (font, text) => {
      const w = widths.get(font + '\n' + text);
      if (w === undefined) {
        throw new Error('no width for ' + JSON.stringify([font, text]));
      }
      return w;
    },
  });
  realm.random = random;
  return realm;
}

const f64 = new Float64Array(1);
const u64 = new BigUint64Array(f64.buffer);
export function bits(x) {
  f64[0] = x;
  return u64[0].toString(16).padStart(16, '0');
}

// sameNumber compares a result with a number of the golden file.
export function sameNumber(got, want) {
  want = decode(want);
  if (want === undefined || want === null) {
    return got === undefined || got === null;
  }
  if (typeof got !== 'number') {
    return false;
  }
  return bits(got) === bits(want) || (Number.isNaN(got) && Number.isNaN(want));
}

// checkRecord compares the result of the glue with the record. It returns the
// differences.
export function checkRecord(rec, res) {
  const diffs = [];
  const num = (what, got, want) => {
    if (!sameNumber(got, want)) {
      diffs.push(`${what}: got ${got}, want ${JSON.stringify(want)}`);
    }
  };
  if (res.nodes.length !== rec.result.nodes.length || res.edges.length !== rec.result.edges.length) {
    return ['the number of elements differs'];
  }
  rec.result.nodes.forEach((want, i) => {
    const got = res.nodes[i];
    if (want.boundingBox && rec.layout === 'fcose') {
      for (const k of ['x1', 'y1', 'x2', 'y2', 'w', 'h']) {
        num(`node ${want.id} boundingBox.${k}`, got.boundingBox[k], want.boundingBox[k]);
      }
    } else {
      num(`node ${want.id} x`, got.x, want.position.x);
      num(`node ${want.id} y`, got.y, want.position.y);
    }
  });
  rec.result.edges.forEach((want, i) => {
    const got = res.edges[i];
    if (rec.layout === 'fcose') {
      for (const k of ['sourceEndpoint', 'midpoint', 'targetEndpoint']) {
        num(`edge ${want.id} ${k}.x`, got[k].x, want[k].x);
        num(`edge ${want.id} ${k}.y`, got[k].y, want[k].y);
      }
    } else {
      for (const k of ['startX', 'startY', 'midX', 'midY', 'endX', 'endY']) {
        num(`edge ${want.id} ${k}`, got[k], want[k]);
      }
    }
  });
  return diffs;
}
