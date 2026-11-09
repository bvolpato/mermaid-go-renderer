// Dumps the layout nodes that a diagram db builds, for making realistic
// shape cases. Run with:
//
//   node oracle.mjs esm-eval probes/dump-data.js --args texts.json --out out.json
//
// args is { texts: [{ name, text }] }. For each text the result has the
// diagram type and the nodes of db.getData(). For a block diagram, which has
// no getData, the result has the nodes that getNodeFromBlock builds, before
// and after the blocks are positioned.
mermaid.initialize({ startOnLoad: false });
const clean = (v) => JSON.parse(JSON.stringify(v, (k, x) => (x === Infinity ? 'Infinity' : typeof x === 'function' ? undefined : x)));
const out = [];
for (const { name, text } of args.texts) {
  const res = { name };
  try {
    const diag = await mermaid.mermaidAPI.getDiagramFromText(text);
    res.type = diag.type;
    if (typeof diag.db.getData === 'function') {
      const data = diag.db.getData();
      res.nodes = clean(data.nodes);
    } else if (diag.type === 'block') {
      const getNodeFromBlock = await internalsFrom('blockDiagram', 'getNodeFromBlock');
      const blocks = diag.db.getBlocksFlat();
      res.nodes = clean(blocks.map((b) => getNodeFromBlock(b, diag.db, false)));
      // Render once, so that the blocks have their sizes.
      const container = document.getElementById('container');
      await mermaid.render('my-svg', text, container);
      const diag2 = await mermaid.mermaidAPI.getDiagramFromText(text);
      res.blocks = clean(diag2.db.getBlocksFlat());
    }
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  out.push(res);
}
return out;
