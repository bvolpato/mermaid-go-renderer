// Body of the page function of gen-browser-svgscale.mjs. The oracle runs it
// with `args` in scope: { cases: [{ name, html, queries: [{ sel, op }] }] }.
// Each case puts its markup in the container and reads values from Chromium:
//   rect     getBoundingClientRect()  [x, y, width, height]
//   bbox     getBBox()                [x, y, width, height]
//   textlen  getComputedTextLength()
//   ctm      getScreenCTM()           [a, b, c, d, e, f]
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  container.innerHTML = c.html;
  const results = [];
  for (const q of c.queries) {
    const el = container.querySelector(q.sel);
    let value = null;
    try {
      if (el) {
        switch (q.op) {
          case 'rect': {
            const b = el.getBoundingClientRect();
            value = [b.x, b.y, b.width, b.height];
            break;
          }
          case 'bbox': {
            const b = el.getBBox();
            value = [b.x, b.y, b.width, b.height];
            break;
          }
          case 'textlen':
            value = el.getComputedTextLength();
            break;
          case 'ctm': {
            const m = el.getScreenCTM();
            value = m ? [m.a, m.b, m.c, m.d, m.e, m.f] : null;
            break;
          }
        }
      }
    } catch (e) {
      value = 'ERR ' + e.message;
    }
    results.push(value);
  }
  out.push({ name: c.name, results });
}
container.innerHTML = '';
return out;
