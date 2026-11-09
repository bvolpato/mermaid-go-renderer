// Body of the page function of gen-dom-type-selector-probe.mjs. The oracle runs it
// with `args` in scope: { cases: [{ name, html, target, ops }] }. Each case
// puts html into the container, runs the operations on the element that
// target selects, and returns what the operations return, and the markup of
// the container at the end.
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  container.innerHTML = c.html;
  const el = container.querySelector(c.target);
  const results = [];
  for (const [op, a, b] of c.ops) {
    let r = null;
    try {
      switch (op) {
        case 'attr': el.setAttribute(a, b); break;
        case 'rmattr': el.removeAttribute(a); break;
        case 'getattr': r = el.getAttribute(a); break;
        case 'hasattr': r = el.hasAttribute(a); break;
        case 'style': el.style.setProperty(a, b); break;
        case 'rmstyle': el.style.removeProperty(a); break;
        case 'classed': el.classList.add(a); break;
        case 'outer': r = el.outerHTML; break;
        case 'bbox': { const box = el.getBBox ? el.getBBox() : el.getBoundingClientRect(); r = [box.width > 0, box.height > 0]; break; }
        case 'computed': r = getComputedStyle(el).getPropertyValue(a); break;
        case 'query': r = container.querySelectorAll(a).length; break;
        case 'matches': r = el.matches(a); break;
        case 'closest': r = el.closest(a) !== null; break;
      }
    } catch (e) {
      r = 'ERR ' + e.message;
    }
    results.push(r);
  }
  out.push({ name: c.name, results, html: container.innerHTML });
}
container.innerHTML = '';
return out;
