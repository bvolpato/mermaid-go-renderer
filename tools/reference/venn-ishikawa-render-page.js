// Body of the page function of gen-venn-ishikawa-extra.mjs. The oracle runs
// it with `args` in scope: { fixtures: [{ name, text }] }. It renders every
// fixture with the real mermaid and returns the SVG, or the message of the
// error.
//
// The reference renders of the corpus use a new page for each diagram. This
// script uses one page, and puts the page in the same state before each
// render: an empty container, the site config of the oracle, and the seeded
// Math.random after the one number that mermaid draws when it loads. The
// venn diagram and the ishikawa diagram keep no other state between renders.
const container = document.getElementById('container');
const out = [];
for (const f of args.fixtures) {
  container.innerHTML = '';
  mermaid.initialize({ startOnLoad: false });
  globalThis.__mmdgResetRandom();
  Math.random();
  const rec = { name: f.name };
  try {
    const before = globalThis.__mmdgRandomCalls;
    const { svg, diagramType } = await mermaid.render('my-svg', f.text, container);
    rec.svg = svg;
    rec.diagramType = diagramType;
    rec.randomCalls = globalThis.__mmdgRandomCalls - before;
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  out.push(rec);
}
return out;
