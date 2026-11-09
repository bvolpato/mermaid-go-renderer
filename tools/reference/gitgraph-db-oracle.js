// Browser side of the gitGraph db golden file. See gitgraph-db-oracle.mjs.
//
// This is the body of an async function that oracle.mjs eval runs in a page
// with mermaid loaded. For each case it builds a new page in an iframe, as
// oracle.mjs render does for one diagram: the seeded Math.random, then the
// mermaid bundle. It then runs mermaid.render and reads the db between the
// parse step and the draw step.
const SEEDED_RANDOM = `(() => {
  let a = ${args.seed} >>> 0;
  globalThis.__mmdgRandomCalls = 0;
  Math.random = function () {
    globalThis.__mmdgRandomCalls++;
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
})();`;
const PAGE_HTML =
  '<!doctype html><html><head><meta charset="utf-8"></head>' +
  '<body><div id="container"></div></body></html>';

// The source of the mermaid bundle: the script that oracle.mjs added.
const mermaidSource = [...document.scripts].map((s) => s.text).find((t) => t.length > 100000);
if (!mermaidSource) {
  return { error: 'the mermaid script is not in the page' };
}

const dump = (db) => ({
  commits: [...db.getCommits().entries()].map(([key, commit]) => ({ key, ...commit })),
  branches: db.getBranchesAsObjArray().map((b) => b.name),
  branchHeads: [...db.getBranches().entries()],
  direction: db.getDirection(),
  currentBranch: db.getCurrentBranch(),
  head: db.getHead(),
  title: db.getDiagramTitle(),
  accTitle: db.getAccTitle(),
  accDescr: db.getAccDescription(),
});

document.body.style.margin = '0';
const out = [];
for (const c of args.cases) {
  const iframe = document.createElement('iframe');
  iframe.style.cssText = 'border:0;width:800px;height:600px;position:absolute;left:0;top:0';
  document.body.appendChild(iframe);
  const win = iframe.contentWindow;
  const doc = iframe.contentDocument;
  doc.open();
  doc.write(PAGE_HTML);
  doc.close();
  win.eval(SEEDED_RANDOM);
  const script = doc.createElement('script');
  script.text = mermaidSource;
  doc.head.appendChild(script);

  const mermaid = win.mermaid;
  const result = { name: c.name, text: c.text };
  if (c.config) result.config = c.config;
  try {
    mermaid.initialize({ startOnLoad: false, ...(c.config || {}) });
    // The db and the renderer of the gitGraph diagram are one object each
    // for the page. An empty diagram gives them without a random number.
    const loadCalls = win.__mmdgRandomCalls;
    const empty = await mermaid.mermaidAPI.getDiagramFromText('gitGraph');
    result.randomInSetup = win.__mmdgRandomCalls - loadCalls;
    const db = empty.db;
    const renderer = empty.renderer;
    let snapshot = null;
    const draw = renderer.draw;
    renderer.draw = function (...a) {
      snapshot = JSON.parse(JSON.stringify(dump(db)));
      return draw.apply(this, a);
    };
    const randomBefore = win.__mmdgRandomCalls;
    result.randomBefore = randomBefore;
    try {
      const { svg, diagramType } = await mermaid.render('my-svg', c.text, doc.getElementById('container'));
      result.diagramType = diagramType;
      result.svg = svg;
      result.db = snapshot;
    } catch (e) {
      result.error = String((e && e.message) || e);
      result.db = snapshot || dump(db);
    }
    result.randomCalls = win.__mmdgRandomCalls - randomBefore;
  } catch (e) {
    result.setupError = String((e && e.message) || e);
  }
  out.push(result);
  iframe.remove();
}
return { cases: out };
