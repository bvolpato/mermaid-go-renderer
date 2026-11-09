// Body of the page function of gen-venn-ishikawa-db.mjs. The oracle runs it
// with `args` in scope: { cases: [{ name, type, text }] }. It parses every
// case with the real mermaid and returns the content of the venn db or of
// the ishikawa db as JSON data.
mermaid.initialize({ startOnLoad: false });

// The venn db is one module object for the whole page. Get it one time, and
// read it after each mermaid.parse call. The ishikawa diagram makes a new db
// for each parse, so the script parses the text a second time with
// getDiagramFromText, under the config that mermaid.parse has set.
const vennDb = (await mermaid.mermaidAPI.getDiagramFromText('venn-beta\n')).db;

const tree = (node) => ({ text: node.text, children: node.children.map(tree) });

const out = [];
for (const c of args.cases) {
  const rec = { name: c.name, type: c.type, text: c.text };
  try {
    // parse runs processAndSetConfigs and then Diagram.fromText on the
    // cleaned text, as render does, without the title of the front matter.
    const res = await mermaid.parse(c.text);
    if (res.diagramType !== c.type) {
      rec.error = 'detected type ' + res.diagramType;
    } else if (c.type === 'venn') {
      const db = vennDb;
      rec.db = {
        // A size can be Infinity, which JSON does not have: write the text.
        subsets: db.getSubsetData().map((s) => ({
          sets: s.sets,
          size: String(s.size),
          label: s.label === undefined ? null : s.label,
        })),
        textNodes: db.getTextData().map((t) => ({
          sets: t.sets,
          id: t.id,
          label: t.label === undefined ? null : t.label,
        })),
        // The entries keep the order of the keys of the styles object.
        styles: db.getStyleData().map((s) => ({
          targets: s.targets,
          styles: Object.entries(s.styles),
        })),
        currentSets: db.getCurrentSets() === undefined ? null : db.getCurrentSets(),
        indentMode: db.getIndentMode(),
        title: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
      };
    } else {
      const db = (await mermaid.mermaidAPI.getDiagramFromText(c.text)).db;
      const root = db.getRoot();
      rec.db = {
        root: root === undefined ? null : tree(root),
        // stack and baseLevel are private in TypeScript only.
        stack: db.stack.map((s) => s.level),
        baseLevel: db.baseLevel === undefined ? null : db.baseLevel,
        title: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
      };
    }
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  out.push(rec);
}
return out;
