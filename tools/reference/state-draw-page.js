// Body of the page function of gen-state-draw.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, site }], seed }
//
// For each case the script calls the real mermaid.render with the real draw
// function of the state renderer. The layouts are replaced by a stub that
// draws one group for each node of the layout data, with the properties of
// the layout data as attributes. So the result shows what the draw function
// gives to the layout, and what it does after the layout: the links, the
// title and the view box. internal/diagrams/state/draw_test.go has the same
// stub.
//
// Each case runs in a new iframe: a page that has made no other render, as
// in the reference renders. Math.random is the seeded generator of the
// oracle. Before the render it is reset and makes one draw, as the bundle of
// the reference page does.

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (seed, site) {
  let a = seed >>> 0;
  const resetRandom = () => {
    a = seed >>> 0;
  };
  Math.random = function () {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  const mermaid = (await import('/mermaid.esm.mjs')).default;

  // The attribute value of a property: none for a value that is falsy
  // (text), undefined (number) or empty (array).
  const text = (v) => (v ? String(v) : null);
  const number = (v) => (v === undefined || v === null ? null : String(v));
  const flag = (v) => (v ? 'true' : null);
  const list = (v) => (Array.isArray(v) && v.length > 0 ? v.join('|') : null);
  const stub = async (data, svg, helpers, options) => {
    const root = svg
      .append('g')
      .attr('class', 'root')
      .attr('data-type', text(data.type))
      .attr('data-layout', text(data.layoutAlgorithm))
      .attr('data-algorithm', text(options?.algorithm))
      .attr('data-direction', text(data.direction))
      .attr('data-node-spacing', number(data.nodeSpacing))
      .attr('data-rank-spacing', number(data.rankSpacing))
      .attr('data-markers', list(data.markers))
      .attr('data-diagram-id', text(data.diagramId));
    const nodes = root.append('g').attr('class', 'nodes');
    data.nodes.forEach((node, i) => {
      const g = nodes
        .append('g')
        .attr('class', node.isGroup ? 'cluster' : node.look === 'handDrawn' ? 'rough-node' : 'node')
        .attr('id', node.domId)
        .attr('data-id', node.id)
        .attr('data-shape', text(node.shape))
        .attr('data-label-type', text(node.labelType))
        .attr('data-classes', text(node.cssClasses))
        .attr('data-styles', list(node.cssStyles))
        .attr('data-compiled', list(node.cssCompiledStyles))
        .attr('data-parent', text(node.parentId))
        .attr('data-dir', text(node.dir))
        .attr('data-color-index', number(node.colorIndex))
        .attr('data-padding', number(node.padding))
        .attr('data-rx', number(node.rx))
        .attr('data-ry', number(node.ry))
        .attr('data-look', text(node.look))
        .attr('data-position', text(node.position))
        .attr('data-description', list(node.description))
        .attr('data-wrapping-width', number(node.wrappingWidth))
        .attr('data-min-width', number(node.minWidth))
        .attr('data-center-label', flag(node.centerLabel))
        .attr('data-node-type', text(node.type));
      g.append('rect')
        .attr('x', i * 100)
        .attr('y', 0)
        .attr('width', 80)
        .attr('height', 40);
      g.append('desc').text(node.label ?? '');
    });
    const edges = root.append('g').attr('class', 'edges');
    data.edges.forEach((edge) => {
      edges
        .append('path')
        .attr('id', edge.id)
        .attr('d', 'M0,0L10,10')
        .attr('data-start', text(edge.start))
        .attr('data-end', text(edge.end))
        .attr('data-label', text(edge.label))
        .attr('data-classes', text(edge.classes))
        .attr('data-pattern', text(edge.pattern))
        .attr('data-arrowhead', text(edge.arrowhead))
        .attr('data-arrow-type-end', text(edge.arrowTypeEnd))
        .attr('data-arrowhead-style', text(edge.arrowheadStyle))
        .attr('data-look', text(edge.look))
        .attr('data-thickness', text(edge.thickness))
        .attr('data-style', text(edge.style))
        .attr('data-label-type', text(edge.labelType))
        .attr('data-labelpos', text(edge.labelpos));
    });
  };
  const loader = async () => ({ render: stub });
  mermaid.registerLayoutLoaders([
    { name: 'dagre', loader },
    { name: 'elk', loader, algorithm: 'elk.layered' },
    { name: 'elk.stress', loader, algorithm: 'elk.stress' },
  ]);
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  return async (c) => {
    const rec = { name: c.name };
    resetRandom();
    Math.random();
    try {
      const res = await mermaid.render('my-svg', c.text, document.getElementById('container'));
      rec.type = res.diagramType;
      rec.svg = res.svg;
    } catch (e) {
      rec.error = String((e && e.message) || e);
    }
    return rec;
  };
};

const setupSrc = '(' + String(setup) + ')';
const out = [];
for (const c of args.cases) {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const run = await frame.contentWindow.eval(setupSrc)(args.seed, c.site);
    out.push(await run(c));
  } finally {
    frame.remove();
  }
}
return { cases: out };
