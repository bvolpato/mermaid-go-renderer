// Body of the page function of gen-smallcharts-db.mjs. The oracle runs it
// with `args` in scope: { cases: [{ type, text }] }. It parses every case
// with the real mermaid and returns the content of the db of the pie chart,
// the packet diagram, the radar chart or the quadrant chart as JSON data.
mermaid.initialize({ startOnLoad: false });

// encode keeps what JSON.stringify loses: undefined is {"$u":1}, and NaN,
// the infinities and -0 are {"$n":"<text>"}.
const encode = (v) => {
  if (v === undefined) return { $u: 1 };
  if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
    return { $n: Object.is(v, -0) ? '-0' : String(v) };
  }
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return Array.from(v, encode);
  const o = {};
  for (const k of Object.keys(v)) {
    if (typeof v[k] !== 'function') o[k] = encode(v[k]);
  }
  return o;
};

const out = [];
for (const c of args.cases) {
  const rec = { type: c.type, text: c.text };
  try {
    // parse runs processAndSetConfigs and Diagram.fromText, as render does.
    // getDiagramFromText then parses again with the same config, without
    // the title of the front matter, and returns the diagram with its db.
    const res = await mermaid.parse(c.text);
    if (res.diagramType !== c.type) {
      throw new Error('detected type ' + res.diagramType);
    }
    const db = (await mermaid.mermaidAPI.getDiagramFromText(c.text)).db;
    const common = {
      title: db.getDiagramTitle(),
      accTitle: db.getAccTitle(),
      accDescription: db.getAccDescription(),
    };
    if (c.type === 'pie') {
      rec.db = encode({ ...common, sections: [...db.getSections()], showData: db.getShowData() });
    } else if (c.type === 'packet') {
      rec.db = encode({ ...common, packet: db.getPacket(), config: db.getConfig() });
    } else if (c.type === 'radar') {
      rec.db = encode({
        ...common,
        axes: db.getAxes(),
        curves: db.getCurves(),
        options: db.getOptions(),
        config: db.getConfig(),
      });
    } else {
      // The first statements of draw in quadrantRenderer.ts.
      const conf = mermaid.mermaidAPI.getConfig();
      const width = conf.quadrantChart?.chartWidth ?? 500;
      const height = conf.quadrantChart?.chartHeight ?? 500;
      db.setHeight(height);
      db.setWidth(width);
      rec.db = encode({ ...common, data: db.getQuadrantData() });
    }
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  out.push(rec);
}
return out;
