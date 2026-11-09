// Body of the page function of gen-sequence-db.mjs. The oracle runs it with
// `args` in scope: { cases: [{ name, text }] }. It parses every case with the
// real mermaid and returns the content of the sequence db as JSON data.
mermaid.initialize({ startOnLoad: false });
const out = [];
for (const c of args.cases) {
  const rec = { name: c.name };
  try {
    // parse runs processAndSetConfigs, so the config of the front matter and
    // of the directives is active for getDiagramFromText, as in render.
    await mermaid.parse(c.text);
    const d = await mermaid.mermaidAPI.getDiagramFromText(c.text);
    rec.type = d.type;
    if (d.type === 'sequence') {
      const db = d.db;
      const boxes = db.getBoxes();
      rec.boxes = boxes.map((b) => ({ ...b }));
      rec.actors = [...db.getActors().entries()].map(([k, a]) => [
        k,
        { ...a, box: a.box ? boxes.indexOf(a.box) : undefined },
      ]);
      rec.actorKeys = db.getActorKeys();
      rec.messages = db.getMessages().map((m) => ({ ...m }));
      rec.createdActors = [...db.getCreatedActors().entries()];
      rec.destroyedActors = [...db.getDestroyedActors().entries()];
      rec.title = db.getDiagramTitle();
      rec.accTitle = db.getAccTitle();
      rec.accDescription = db.getAccDescription();
      rec.hasAtLeastOneBox = db.hasAtLeastOneBox();
      rec.hasAtLeastOneBoxWithTitle = db.hasAtLeastOneBoxWithTitle();
      rec.autoWrap = db.autoWrap();
    }
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  out.push(rec);
}
return out;
