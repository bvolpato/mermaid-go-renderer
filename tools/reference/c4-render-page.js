// Page script of gen-c4-render.mjs. The eval command of the browser oracle
// runs it as the body of an async function, with `args` in scope.
//
// It renders each case as the render command of the oracle does, in one
// page. The state that a page keeps between two renders is reset before
// each case, so that every case renders as in a new page:
//   - Math.random is the seeded generator of the oracle, in its first state,
//     and it has made the draws that the mermaid bundle makes while it loads,
//   - the site config is the default config with the config of the case,
//   - the accessible title and description of commonDb.ts are empty.
const mermaid = globalThis.mermaid;
const seedRandom = () => {
  let a = args.seed >>> 0;
  Math.random = function () {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  for (let i = 0; i < args.randomBefore; i++) {
    Math.random();
  }
};
mermaid.initialize({ startOnLoad: false });
const seed = await mermaid.mermaidAPI.getDiagramFromText('C4Context\nPerson(a, "A")');
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  const entry = { name: c.name };
  seed.db.setAccTitle('');
  seed.db.setAccDescription('');
  mermaid.initialize({ startOnLoad: false, ...(c.config || {}) });
  seedRandom();
  try {
    const { svg, diagramType } = await mermaid.render('my-svg', c.text, container);
    entry.svg = svg;
    entry.type = diagramType;
  } catch (e) {
    entry.error = String((e && e.message) || e);
    // mermaid leaves the element of a failed render in the page.
    container.replaceChildren();
    for (const el of document.querySelectorAll('#dmy-svg, #my-svg')) {
      el.remove();
    }
  }
  out.push(entry);
}
return out;
