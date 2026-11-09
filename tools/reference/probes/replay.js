// Replay probe: measures every element of a finished diagram.
//
// The script is the body of an async function for `oracle.mjs eval`. Each
// document of `args.docs` is `{ name, html, style }`: markup that the page
// puts into div#container, the place where mermaid renders. The probe then
// reads, for each element, the values that the code of mermaid reads while
// it renders:
//
//   bbox  getBBox() of each SVG graphics element that is rendered
//   rect  getBoundingClientRect() of each foreignObject, each HTML element
//         in a foreignObject, each text element and the root svg
//   tlen  getComputedTextLength() of each text content element
//   font  the computed font-size, font-family, font-weight and font-style of
//         each text content element and each HTML element (and display for
//         an HTML element)
//
// An element is named by its index in the document order of all elements in
// the container. `n` is the number of elements and `sig` is a hash of their
// names, so that the Go side can check that its parser made the same tree.
//
// With `args.keys`, each record also gets a key: a hash of the style sheet,
// the element with its context and its ancestors. The generator uses the
// key to drop a record that an earlier document has with the same markup.
const container = document.getElementById('container');
const SVG_NS = 'http://www.w3.org/2000/svg';
const NOT_RENDERED = new Set([
  'defs', 'marker', 'symbol', 'clipPath', 'mask', 'pattern', 'linearGradient', 'radialGradient', 'filter', 'style',
  'title', 'desc', 'metadata', 'script',
]);
const GRAPHICS = new Set([
  'svg', 'g', 'a', 'text', 'tspan', 'textPath', 'path', 'rect', 'circle', 'ellipse', 'line', 'polygon', 'polyline',
  'foreignObject', 'image', 'use', 'switch',
]);
const TEXT_CONTENT = new Set(['text', 'tspan', 'textPath']);

const fnv = (h, s) => {
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h;
};
const hash2 = (s) => fnv(2166136261, s).toString(36) + '.' + fnv(0x9747b28c, s).toString(36);

const out = [];
for (const doc of args.docs) {
  container.innerHTML = doc.html;
  const res = { name: doc.name, n: 0, sig: 0, bbox: [], rect: [], tlen: [], font: [] };
  if (args.keys) {
    res.keys = { bbox: [], rect: [], tlen: [], font: [] };
  }
  const outerCache = new Map();
  const chainCache = new Map();
  const openTag = (el) => {
    let s = '<' + el.localName;
    for (const a of el.attributes) {
      s += ' ' + a.name + '=' + a.value;
    }
    return s + '>';
  };
  const chainOf = (el) => {
    if (!el || el === container) {
      return '';
    }
    let c = chainCache.get(el);
    if (c === undefined) {
      c = chainOf(el.parentElement) + openTag(el);
      chainCache.set(el, c);
    }
    return c;
  };
  // keyOf is the key of an element: the style sheet, the ancestors of its
  // context, the markup of its context and its place in the context. The
  // context of an element in a text or in a foreignObject is that text or
  // foreignObject, because the layout of the whole context gives its values.
  const keyOf = (el, ctx, path) => {
    let outer = outerCache.get(ctx);
    if (outer === undefined) {
      outer = hash2(ctx.outerHTML);
      outerCache.set(ctx, outer);
    }
    return hash2(doc.style + '\n' + chainOf(ctx.parentElement) + '\n' + outer + '\n' + path);
  };
  let sig = 2166136261;
  let index = -1;
  const record = (kind, idx, values, el, ctx, path) => {
    res[kind].push([idx, ...values]);
    if (args.keys) {
      res.keys[kind].push(keyOf(el, ctx, path));
    }
  };
  const fontOf = (el, html) => {
    const cs = getComputedStyle(el);
    const v = [cs.fontSize, cs.fontFamily, cs.fontWeight, cs.fontStyle];
    if (html) {
      v.push(cs.display);
    }
    return v;
  };
  const walk = (el, skip, ctx, path) => {
    index++;
    const idx = index;
    const name = el.localName;
    const svg = el.namespaceURI === SVG_NS;
    sig = fnv(sig, name + '|');
    if (!skip && svg && NOT_RENDERED.has(name)) {
      skip = true;
    }
    if (!skip) {
      const own = ctx || el;
      try {
        if (svg && GRAPHICS.has(name) && typeof el.getBBox === 'function') {
          const b = el.getBBox();
          record('bbox', idx, [b.x, b.y, b.width, b.height], el, own, path);
          if (TEXT_CONTENT.has(name)) {
            record('tlen', idx, [el.getComputedTextLength()], el, own, path);
            record('font', idx, fontOf(el, false), el, own, path);
          }
          if (name === 'text' || name === 'foreignObject' || (name === 'svg' && el.parentElement.namespaceURI !== SVG_NS)) {
            const r = el.getBoundingClientRect();
            record('rect', idx, [r.x, r.y, r.width, r.height], el, own, path);
          }
        } else if (!svg && ctx) {
          const r = el.getBoundingClientRect();
          record('rect', idx, [r.x, r.y, r.width, r.height], el, own, path);
          record('font', idx, fontOf(el, true), el, own, path);
        }
      } catch (e) {
        record('bbox', idx, ['ERR ' + e.message], el, own, path);
      }
    }
    if (!ctx && svg && (name === 'text' || name === 'foreignObject')) {
      ctx = el;
      path = '';
    }
    let k = 0;
    for (const c of el.children) {
      walk(c, skip, ctx, ctx ? path + '/' + k : '');
      k++;
    }
  };
  for (const c of container.children) {
    walk(c, false, null, '');
  }
  res.n = index + 1;
  res.sig = sig;
  out.push(res);
}
container.innerHTML = '';
return out;
