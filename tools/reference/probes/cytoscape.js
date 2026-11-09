// Records what mermaid does with cytoscape, as golden data for
// internal/cytoscape. Run with:
//
//   node oracle.mjs esm-eval probes/cytoscape.js --args cases.json --out out.json
//
// args is { cases: [{ name, text, randomBefore }] }. See gen-cytoscape.mjs.
//
// For each case the probe renders the diagram with the real mermaid and
// records every cytoscape instance that the render creates:
//
//   style     the style sheet of the instance (cy.style().json())
//   initial   the layout option of the constructor
//   adds      the argument of every cy.add call, in call order
//   layouts   the options of every cy.layout call. A function option is
//             recorded as { $fn: { <edge id>: value } }: its value for each
//             edge. runs is the number of run calls of that layout.
//   measure   every canvas measureText call: [the font as cytoscape assigns
//             it, text, width, the font as the context serializes it]
//   random    the number of Math.random calls from the first add to the read
//   result    what mermaid reads after the layout, read at the same moment
//             (a microtask after cy.ready, when the renderer is destroyed)
//             and in the same order:
//               cose-bilkent: the rscratch of every edge and the position of
//               every node (field reads, no calls)
//               fcose: sourceEndpoint, midpoint and targetEndpoint of every
//               edge, then boundingBox of every group node, then the
//               position of every node
//   after     when the render is done: boundingBox of every node and the
//             endpoints of every edge again. mermaid does not read these.
//
// For an architecture diagram the case also has `architecture`: the config
// fields, services, junctions, groups and edges that layoutArchitecture gets.
//
// The record also has the SVG of the render. The generator compares it with
// the reference SVG, which proves that the hooks do not change the result
// and that the state resets below give the state of a new page.
//
// A reference render runs in a new page. Here all cases run in one page, so
// the probe resets the state that a layout reads from the page: the seeded
// Math.random (and the number of calls before the render), and the random
// seed of layout-base, which is a module variable.
const { cytoscape } = await internals('cytoscape');
const layoutBases = [];
for (const chunk of ['cose-bilkent', 'architectureDiagram']) {
  const req = await internalsFrom(chunk, 'require_layout_base');
  layoutBases.push(req());
}
const seedState = layoutBases.map((lb) => ({ seed: lb.RandomSeed.seed, x: lb.RandomSeed.x }));

// clean copies a value as JSON data. A number that JSON cannot hold becomes
// { $num: "NaN" } and so on. A function, a DOM node, a d3 selection and a
// reference back to an object that contains the value are left out.
const clean = (v) => {
  const path = [];
  const walk = (x) => {
    if (typeof x === 'number') {
      if (!Number.isFinite(x)) {
        return { $num: String(x) };
      }
      return Object.is(x, -0) ? { $num: '-0' } : x;
    }
    if (x === null || typeof x === 'string' || typeof x === 'boolean') {
      return x;
    }
    if (typeof x !== 'object') {
      return undefined;
    }
    if ((typeof Node !== 'undefined' && x instanceof Node) || (x._groups && x._parents) || path.includes(x)) {
      return undefined;
    }
    path.push(x);
    let out;
    if (Array.isArray(x)) {
      out = x.map((e) => {
        const c = walk(e);
        return c === undefined ? null : c;
      });
    } else {
      out = {};
      for (const k of Object.keys(x)) {
        const c = walk(x[k]);
        if (c !== undefined) {
          out[k] = c;
        }
      }
    }
    path.pop();
    return out;
  };
  return walk(v);
};
const pt = (p) => clean({ x: p.x, y: p.y });

let current = null;
const instances = new Map(); // cy -> record

const coreProto = Object.getPrototypeOf(cytoscape({ headless: true }));
const instanceRecord = (cy) => {
  let r = instances.get(cy);
  if (!r) {
    const container = cy.container();
    r = {
      style: clean(cy.style().json()),
      initial: clean(cy._private.options.layout),
      container: container ? { id: container.id, style: container.getAttribute('style'), connected: container.isConnected } : null,
      headless: cy.headless(),
      styleEnabled: cy.styleEnabled(),
      adds: [],
      layouts: [],
      measure: [],
      randomStart: globalThis.__mmdgRandomCalls,
    };
    instances.set(cy, r);
    if (current) {
      current.instances.push(r);
    }
  }
  return r;
};

const origAdd = coreProto.add;
coreProto.add = function (opts) {
  if (current && this.container()) {
    instanceRecord(this).adds.push(clean(opts));
  }
  return origAdd.apply(this, arguments);
};

const origLayout = coreProto.layout;
coreProto.layout = function (opts) {
  const layout = origLayout.apply(this, arguments);
  // The first layout call comes from the constructor, before the instance
  // is ready: it is the initial layout, recorded as `initial`.
  if (current && this.container() && this._private.ready) {
    const cy = this;
    const r = instanceRecord(cy);
    const options = {};
    for (const k of Object.keys(opts)) {
      if (typeof opts[k] === 'function') {
        const values = {};
        cy.edges().forEach((e) => {
          values[e.id()] = opts[k](e);
        });
        options[k] = { $fn: clean(values) };
      } else if (k !== 'eles' && k !== 'cy') {
        options[k] = clean(opts[k]);
      }
    }
    const rec = { options, runs: 0 };
    r.layouts.push(rec);
    const origRun = layout.run;
    layout.run = function () {
      rec.runs++;
      return origRun.apply(this, arguments);
    };
  }
  return layout;
};

const readResult = (cy, r) => {
  const res = { destroyed: !!cy.renderer().destroyed, width: clean(cy.width()), height: clean(cy.height()), zoom: cy.zoom(), pan: pt(cy.pan()) };
  const fcose = r.layouts.some((l) => l.options.name === 'fcose');
  if (fcose) {
    res.edges = cy.edges().map((edge) => {
      const s = edge[0].sourceEndpoint();
      const m = edge[0].midpoint();
      const t = edge[0].targetEndpoint();
      return { id: edge.id(), sourceEndpoint: pt(s), midpoint: pt(m), targetEndpoint: pt(t), hasRscratch: !!edge[0]._private.rscratch };
    });
    res.nodes = cy.nodes().map((node) => {
      const n = { id: node.id() };
      if (node.data().type === 'group') {
        const { h, w, x1, y1, x2, y2 } = node.boundingBox();
        n.boundingBox = clean({ x1, y1, x2, y2, w, h });
      }
      return n;
    });
    cy.nodes().forEach((node, i) => {
      res.nodes[i].position = pt(node.position());
    });
  } else {
    res.edges = cy.edges().map((edge) => {
      const rs = edge._private.rscratch;
      return clean({
        id: edge.id(),
        startX: rs.startX,
        startY: rs.startY,
        midX: rs.midX,
        midY: rs.midY,
        endX: rs.endX,
        endY: rs.endY,
        edgeType: rs.edgeType,
      });
    });
    res.nodes = cy.nodes().map((node) => ({ id: node.id(), position: pt(node.position()) }));
  }
  return res;
};

const origReady = coreProto.ready;
coreProto.ready = function (fn) {
  if (current && this.container() && instances.has(this)) {
    const cy = this;
    const r = instances.get(cy);
    r.readyDestroyed = !!cy.renderer().destroyed;
    // mermaid reads the result in the continuation of an await. This
    // microtask is queued before that continuation and after the mutation
    // observer notification of the container removal.
    Promise.resolve().then(() => {
      r.random = globalThis.__mmdgRandomCalls - r.randomStart;
      try {
        r.result = readResult(cy, r);
      } catch (e) {
        r.resultError = String((e && e.stack) || e);
      }
    });
  }
  return origReady.apply(this, arguments);
};

// The font that cytoscape assigns, as text. The getter gives the serialized
// form.
const fontDesc = Object.getOwnPropertyDescriptor(CanvasRenderingContext2D.prototype, 'font');
const assignedFont = new WeakMap();
Object.defineProperty(CanvasRenderingContext2D.prototype, 'font', {
  configurable: true,
  enumerable: fontDesc.enumerable,
  get() {
    return fontDesc.get.call(this);
  },
  set(v) {
    assignedFont.set(this, String(v));
    fontDesc.set.call(this, v);
  },
});
const origMeasure = CanvasRenderingContext2D.prototype.measureText;
CanvasRenderingContext2D.prototype.measureText = function (text) {
  const m = origMeasure.apply(this, arguments);
  if (current) {
    const last = current.instances[current.instances.length - 1];
    if (last) {
      last.measure.push([assignedFont.get(this), text, m.width, this.font]);
    }
  }
  return m;
};

// What the page gives for a container as mermaid makes it.
const env = {};
{
  const el = document.createElement('div');
  el.setAttribute('id', 'cy-probe');
  el.setAttribute('style', 'display:none');
  document.body.appendChild(el);
  const names = [
    'position',
    'padding-left',
    'padding-right',
    'padding-top',
    'padding-bottom',
    'border-left-width',
    'border-right-width',
    'border-top-width',
    'border-bottom-width',
    'font-size',
    'width',
    'height',
  ];
  const read = () => {
    const cs = getComputedStyle(el);
    const rect = el.getBoundingClientRect();
    return {
      computed: Object.fromEntries(names.map((n) => [n, cs.getPropertyValue(n)])),
      clientWidth: el.clientWidth,
      clientHeight: el.clientHeight,
      rect: { left: rect.left, top: rect.top, width: rect.width, height: rect.height },
    };
  };
  env.connected = read();
  el.className = ' __________cytoscape_container';
  env.connectedWithClass = read();
  el.remove();
  env.removed = read();
  env.userAgent = navigator.userAgent;
  env.devicePixelRatio = devicePixelRatio;
  env.hasResizeObserver = typeof ResizeObserver !== 'undefined';
  env.hasMutationObserver = typeof MutationObserver !== 'undefined';
  env.stylesheet = !!document.getElementById('__________cytoscape_stylesheet');
  env.now = Date.now();
  env.performanceNow = performance.now();
  env.cytoscapeVersion = cytoscape.version;
}

const container = document.getElementById('container');
mermaid.initialize({ startOnLoad: false });
const out = { env, cases: [] };
for (const c of args.cases) {
  const rec = { name: c.name, instances: [] };
  container.innerHTML = '';
  globalThis.__mmdgResetRandom();
  for (let i = 0; i < (c.randomBefore || 0); i++) {
    Math.random();
  }
  layoutBases.forEach((lb, i) => {
    lb.RandomSeed.seed = seedState[i].seed;
    lb.RandomSeed.x = seedState[i].x;
  });
  current = rec;
  try {
    const { svg, diagramType } = await mermaid.render('my-svg', c.text, container);
    rec.type = diagramType;
    rec.svg = svg;
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  current = null;
  if (rec.type === 'architecture') {
    // The arguments of layoutArchitecture, from a second parse of the text.
    try {
      const db = (await mermaid.mermaidAPI.getDiagramFromText(c.text)).db;
      const config = {};
      for (const k of ['iconSize', 'fontSize', 'padding', 'idealEdgeLengthMultiplier', 'edgeElasticity', 'randomize', 'nodeSeparation', 'numIter', 'seed']) {
        config[k] = db.getConfigField(k);
      }
      rec.architecture = clean({
        config,
        services: db.getServices(),
        junctions: db.getJunctions(),
        groups: db.getGroups(),
        edges: db.getEdges(),
        layoutHints: db.getLayoutHints(),
      });
    } catch (e) {
      rec.architectureError = String((e && e.stack) || e);
    }
  }
  // What mermaid does not read: every bounding box and endpoint, after the render.
  for (const [cy, r] of instances) {
    if (!rec.instances.includes(r)) {
      continue;
    }
    try {
      r.after = {
        destroyed: !!cy.renderer().destroyed,
        nodes: cy.nodes().map((node) => {
          const { h, w, x1, y1, x2, y2 } = node.boundingBox();
          return { id: node.id(), position: pt(node.position()), boundingBox: clean({ x1, y1, x2, y2, w, h }) };
        }),
        edges: cy.edges().map((edge) => {
          const rs = edge._private.rscratch;
          return clean({
            id: edge.id(),
            sourceEndpoint: pt(edge.sourceEndpoint()),
            midpoint: pt(edge.midpoint()),
            targetEndpoint: pt(edge.targetEndpoint()),
            rscratch: { startX: rs.startX, startY: rs.startY, midX: rs.midX, midY: rs.midY, endX: rs.endX, endY: rs.endY },
          });
        }),
      };
    } catch (e) {
      r.afterError = String((e && e.stack) || e);
    }
  }
  instances.clear();
  document.querySelectorAll('body > :not(#container)').forEach((el) => el.remove());
  out.cases.push(rec);
}
out.seedState = seedState;
return out;
