// Style attribute probe: when does Chromium add the style attribute of an
// element whose inline style was written through the CSSOM
// (element.style.setProperty, as d3 selection.style does)? Run with:
//
//   node oracle.mjs eval probes/style-attr-recalc.js
//
// Blink keeps such a style out of the attribute list until something reads
// the attributes. This probe shows one more trigger: a style recalculation
// (getBBox, getComputedStyle, offsetWidth, a rendered frame) adds the
// attribute of an svg, rect and foreignObject element, and not of a g, text,
// div or p element. The usecase renderer depends on it: it writes font
// properties on the root svg before the layout and sets the class attribute
// after it, and the style attribute of the reference SVG comes first.
//
// For each element and trigger the result lists the properties that were
// set, with S when the style attribute came before the class attribute that
// was set after the trigger, and - when it came after it.
const out = {};
const build = () => {
  const host = document.createElement('div');
  document.body.appendChild(host);
  host.innerHTML =
    '<svg id="s" width="100%"><g id="g"><rect id="r" width="10" height="10"></rect><text id="t">hello</text>' +
    '<svg id="inner" width="5" height="5"></svg>' +
    '<foreignObject id="fo" width="50" height="20"><div id="d">text</div></foreignObject></g></svg><p id="p">para</p>';
  return host;
};
const triggers = {
  none: () => {},
  bbox: (host) => host.querySelector('#g').getBBox(),
  computedSelf: (host, el) => getComputedStyle(el).getPropertyValue('color'),
  offsetWidth: (host) => host.offsetWidth,
  getId: (host, el) => el.getAttribute('id'),
  query: (host) => host.querySelector('[id="s"]'),
};
const values = { '--a': '1px', fill: 'red', color: 'red', display: 'block', 'max-width': '10px' };
for (const id of ['s', 'g', 'r', 't', 'inner', 'fo', 'd', 'p']) {
  for (const [name, trigger] of Object.entries(triggers)) {
    const marks = [];
    for (const [prop, value] of Object.entries(values)) {
      const host = build();
      const el = host.querySelector('#' + id);
      el.style.setProperty(prop, value);
      trigger(host, el);
      el.setAttribute('class', 'c');
      const names = el.getAttributeNames();
      marks.push(prop + '=' + (names.indexOf('style') < names.indexOf('class') ? 'S' : '-'));
      host.remove();
    }
    out[`${id} ${name}`] = marks.join(' ');
  }
}
return out;
