// Page script of gen-c4-db.mjs. The eval command of the browser oracle runs
// it as the body of an async function, with `args` in scope.
//
// For each case it parses the text as mermaid.parse does (the config of the
// front matter is set), and then reads the db of the C4 diagram.
const mermaid = globalThis.mermaid;
mermaid.initialize({ startOnLoad: false });
// c4Db.clear() does not clear the accessible title and description of
// commonDb.ts. A page that renders one diagram never sees values of an
// earlier diagram, so the script clears them before each case. The C4 type
// also stays, and every parse that succeeds sets it again.
const seed = await mermaid.mermaidAPI.getDiagramFromText('C4Context\nPerson(a, "A")');
const out = [];
for (const c of args.cases) {
  const entry = { name: c.name };
  seed.db.setAccTitle('');
  seed.db.setAccDescription('');
  try {
    // mermaid.parse sets the config of the text. getDiagramFromText parses
    // again with that config and returns the diagram with its db.
    await mermaid.parse(c.text);
    const diagram = await mermaid.mermaidAPI.getDiagramFromText(c.text);
    const db = diagram.db;
    entry.type = diagram.type;
    entry.db = JSON.stringify({
      c4Type: db.getC4Type(),
      title: db.getTitle(),
      accTitle: db.getAccTitle(),
      accDescription: db.getAccDescription(),
      c4ShapeInRow: db.getC4ShapeInRow(),
      c4BoundaryInRow: db.getC4BoundaryInRow(),
      wrap: db.autoWrap(),
      currentBoundaryParse: db.getCurrentBoundaryParse(),
      parentBoundaryParse: db.getParentBoundaryParse(),
      shapes: db.getC4ShapeArray(),
      boundaries: db.getBoundaries(),
      rels: db.getRels(),
    });
  } catch (e) {
    entry.error = String((e && e.message) || e);
  }
  out.push(entry);
}
return out;
