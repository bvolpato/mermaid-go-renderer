// MathML measurement probe. Each case has `html`, which goes into the
// container, and an optional selector `sel` (default: every element in the
// container). The result of a case is one row per element: the local name,
// the rectangle of getBoundingClientRect, and the computed values of the
// properties in `props`, when the case has them.
//
// Run it with the eval command of the browser oracle and --bare, or with
// oracle-linux.mjs --fonts liberation. gen-browser-mathml-probe.mjs does
// both.
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  container.innerHTML = c.html;
  const els = [...container.querySelectorAll(c.sel || '*')];
  const rows = els.map((el) => {
    const b = el.getBoundingClientRect();
    const row = [el.localName, b.x, b.y, b.width, b.height];
    if (c.props) {
      const cs = getComputedStyle(el);
      for (const p of c.props) row.push(cs.getPropertyValue(p));
    }
    if (c.client) {
      row.push(el.clientWidth, el.clientHeight);
    }
    return row;
  });
  out.push({ name: c.name, rows });
}
container.innerHTML = '';
return out;
