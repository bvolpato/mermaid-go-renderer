// DOM probe for the page of mermaid-cli: computed styles, also of the
// pseudo-elements ::before and ::after, and boxes. Run it in a page that has
// the style sheets and the web fonts of mermaid-cli:
//
//   node oracle.mjs eval probes/pagesheets.js --bare --webfonts --args cases.json --out results.json
//
// args is { cases: [{ name, html, queries }] }. The html goes into the
// container. A query is { sel, op, arg, all }:
//
//   rect    getBoundingClientRect as [x, y, width, height]
//   rects   getClientRects as a list of such rectangles
//   bbox    getBBox as [x, y, width, height]
//   style   getComputedStyle(el).getPropertyValue(arg)
//   pstyle  getComputedStyle(el, arg[0]).getPropertyValue(arg[1])
//   styles  every property of getComputedStyle(el) as { name: value }; with
//           arg, of getComputedStyle(el, arg)
//
// The result is [{ name, results }], with one result for each query.
const container = document.getElementById('container');
const box = (b) => [b.x, b.y, b.width, b.height];
const out = [];
for (const c of args.cases) {
  container.innerHTML = c.html;
  const results = [];
  for (const q of c.queries) {
    const els = q.all ? [...container.querySelectorAll(q.sel)] : [container.querySelector(q.sel)];
    const vals = els.map((el) => {
      if (!el) return null;
      try {
        switch (q.op) {
          case 'rect': return box(el.getBoundingClientRect());
          case 'rects': return [...el.getClientRects()].map(box);
          case 'bbox': return box(el.getBBox());
          case 'style': return getComputedStyle(el).getPropertyValue(q.arg);
          case 'pstyle': return getComputedStyle(el, q.arg[0]).getPropertyValue(q.arg[1]);
          case 'styles': {
            const s = q.arg ? getComputedStyle(el, q.arg) : getComputedStyle(el);
            const all = {};
            for (let i = 0; i < s.length; i++) all[s[i]] = s.getPropertyValue(s[i]);
            return all;
          }
        }
      } catch (e) { return 'ERR ' + e.message; }
      return null;
    });
    results.push(q.all ? vals : vals[0]);
  }
  out.push({ name: c.name, results });
}
container.innerHTML = '';
return out;
