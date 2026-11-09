// Body of the page function of gen-timeline-journey-db.mjs. The oracle runs
// it with `args` in scope: { cases: [{ name, type, text }] }. It parses every
// case with the real mermaid and returns the content of the timeline db or
// of the journey db as JSON data.
mermaid.initialize({ startOnLoad: false });

// The db of a jison diagram is one module object for the whole page. Get it
// one time, and read it after each mermaid.parse call.
const dbs = {};
for (const type of ['timeline', 'journey']) {
  dbs[type] = (await mermaid.mermaidAPI.getDiagramFromText(type + '\n')).db;
}

// JSON has no NaN and no Infinity. Write such a number as its name.
const plain = (v) =>
  JSON.parse(
    JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? String(x) : x))
  );

const out = [];
for (const c of args.cases) {
  const rec = { name: c.name, type: c.type, text: c.text };
  try {
    // parse runs processAndSetConfigs and then Diagram.fromText on the
    // cleaned text, as render does, without the title of the front matter.
    const res = await mermaid.parse(c.text);
    if (res.diagramType !== c.type) {
      rec.error = 'detected type ' + res.diagramType;
    } else if (c.type === 'timeline') {
      const db = dbs.timeline;
      const common = db.getCommonDb();
      rec.db = plain({
        direction: db.getDirection(),
        sections: db.getSections(),
        tasks: db.getTasks(),
        title: common.getDiagramTitle(),
        accTitle: common.getAccTitle(),
        accDescription: common.getAccDescription(),
      });
    } else {
      const db = dbs.journey;
      rec.db = plain({
        sections: db.getSections(),
        tasks: db.getTasks(),
        actors: db.getActors(),
        title: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
      });
    }
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  out.push(rec);
}
return out;
