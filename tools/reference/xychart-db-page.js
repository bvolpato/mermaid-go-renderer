// Body of the page function of gen-xychart-db.mjs. The oracle runs it with
// `args` in scope: { cases: [{ name, text }] }. It renders every case with
// the real mermaid and returns the content of the xychart db as JSON data:
// the drawable elements that the chart builder returns during the render,
// the chart config, the theme config, the chart data, the title and the
// accessibility values. It also returns the SVG without its style element.
//
// The xychart db is one object for the page, so that the script can wrap
// getDrawableElem to see the value that the renderer gets.

// enc makes a value that JSON can hold without loss: a number that is not
// finite and the negative zero become {"$num": text}, and undefined in an
// array becomes {"$undefined": true}. A property with the value undefined is
// left out.
const enc = (v) => {
  if (typeof v === 'number') {
    if (Number.isNaN(v)) return { $num: 'NaN' };
    if (v === Infinity) return { $num: 'Infinity' };
    if (v === -Infinity) return { $num: '-Infinity' };
    if (Object.is(v, -0)) return { $num: '-0' };
    return v;
  }
  if (v === undefined) return { $undefined: true };
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return Array.from(v, enc);
  const o = {};
  for (const k of Object.keys(v)) {
    if (v[k] !== undefined) o[k] = enc(v[k]);
  }
  return o;
};

const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  const rec = { name: c.name };
  try {
    mermaid.initialize({ startOnLoad: false });
    const probe = await mermaid.mermaidAPI.getDiagramFromText(c.text);
    rec.type = probe.type;
    if (probe.type !== 'xychart') {
      out.push(rec);
      continue;
    }
    const db = probe.db;
    const original = db.getDrawableElem;
    db.getDrawableElem = function () {
      const shapes = original.apply(this, arguments);
      rec.drawable = enc(shapes);
      return shapes;
    };
    try {
      const { svg } = await mermaid.render('my-svg', c.text, container);
      rec.svg = svg.replace(/<style>[\s\S]*?<\/style>/, '<style></style>');
    } finally {
      db.getDrawableElem = original;
    }
    rec.chartConfig = enc(db.getChartConfig());
    rec.themeConfig = enc(db.getChartThemeConfig());
    rec.data = enc(db.getXYChartData());
    rec.title = db.getDiagramTitle();
    rec.accTitle = db.getAccTitle();
    rec.accDescription = db.getAccDescription();
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  container.innerHTML = '';
  out.push(rec);
}
return out;
