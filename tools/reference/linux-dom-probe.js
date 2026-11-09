// Page script of gen-browser-linux-probes.mjs. It is .parity/probe/dom.js
// for several case sets in one page: args.sets is a list of {name, cases},
// and the result maps each name to the results of its cases.
const container = document.getElementById('container');
const out = {};
for (const set of args.sets) {
  const results = [];
  for (const c of set.cases) {
    container.innerHTML = c.html;
    const values = [];
    for (const q of c.queries) {
      const els = q.all ? [...container.querySelectorAll(q.sel)] : [container.querySelector(q.sel)];
      const vals = els.map((el) => {
        if (!el) return null;
        try {
          switch (q.op) {
            case 'bbox': { const b = el.getBBox(); return [b.x, b.y, b.width, b.height]; }
            case 'rect': { const b = el.getBoundingClientRect(); return [b.x, b.y, b.width, b.height]; }
            case 'textlen': return el.getComputedTextLength();
            case 'len': return el.getTotalLength();
            case 'pt': { const p = el.getPointAtLength(q.arg); return [p.x, p.y]; }
            case 'style': return getComputedStyle(el).getPropertyValue(q.arg);
            case 'attr': return el.getAttribute(q.arg);
            case 'html': return el.innerHTML;
            case 'outer': return el.outerHTML;
            case 'setstyle': { for (const [k, v] of q.arg) el.style.setProperty(k, v); return el.getAttribute('style'); }
          }
        } catch (e) { return 'ERR ' + e.message; }
        return null;
      });
      values.push(q.all ? vals : vals[0]);
    }
    results.push({ name: c.name, results: values });
  }
  out[set.name] = results;
}
container.innerHTML = '';
return out;
