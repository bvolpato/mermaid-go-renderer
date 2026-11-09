// Probe: which DOM calls read the attributes of the svg element of a
// usecase diagram while its style is set through the CSSOM and its class
// attribute is not set yet. Such a read makes Blink write the style
// attribute, which decides the order of style and class in the result.
// Run with oracle.mjs eval (IIFE build) and esm-eval (ESM build).
// args: { text }.
const log = [];
const isRoot = (el) => el && el.id === 'my-svg' && el.localName === 'svg';
const active = (el) => isRoot(el) && el.style.length > 0 && !Element.prototype.hasAttribute.__orig.call(el, 'class');
const wrap = (proto, name, describe, covers) => {
  const orig = proto[name];
  const wrapped = function (...a) {
    try {
      const root = covers(this);
      if (root && active(root)) {
        log.push(`${name}(${describe(a)}) on ${this.localName || this.nodeName}${this.id ? '#' + this.id : ''} ${new Error().stack.split('\n').slice(2, 5).map((s) => s.trim().replace(/\(.*\/([^/]+)\)$/, '($1)')).join(' < ')}`);
      }
    } catch {}
    return orig.apply(this, a);
  };
  wrapped.__orig = orig;
  proto[name] = wrapped;
};
const self = (el) => (isRoot(el) ? el : null);
const within = (el) => (el.querySelector ? (isRoot(el) ? el : Element.prototype.querySelector.__orig ? Element.prototype.querySelector.__orig.call(el, 'svg#my-svg') : el.querySelector('svg#my-svg')) : null);
wrap(Element.prototype, 'hasAttribute', (a) => a[0], self);
wrap(Element.prototype, 'getAttribute', (a) => a[0], self);
wrap(Element.prototype, 'getAttributeNS', (a) => a.join(','), self);
wrap(Element.prototype, 'getAttributeNames', () => '', self);
wrap(Element.prototype, 'hasAttributes', () => '', self);
wrap(Element.prototype, 'setAttribute', (a) => a[0], self);
wrap(Element.prototype, 'removeAttribute', (a) => a[0], self);
wrap(Element.prototype, 'matches', (a) => a[0], self);
wrap(Element.prototype, 'closest', (a) => a[0], self);
wrap(Node.prototype, 'cloneNode', (a) => a[0], within);
wrap(Element.prototype, 'querySelector', (a) => a[0], (el) => el.querySelector.__orig ? null : null);
for (const prop of ['attributes', 'outerHTML', 'innerHTML']) {
  const d = Object.getOwnPropertyDescriptor(Element.prototype, prop);
  Object.defineProperty(Element.prototype, prop, {
    ...d,
    get() {
      try {
        const root = prop === 'attributes' ? self(this) : this.querySelector && (isRoot(this) ? this : this.querySelector('svg#my-svg'));
        if (root && active(root)) {
          log.push(`get ${prop} on ${this.localName}${this.id ? '#' + this.id : ''} ${new Error().stack.split('\n').slice(2, 5).map((s) => s.trim().replace(/\(.*\/([^/]+)\)$/, '($1)')).join(' < ')}`);
        }
      } catch {}
      return d.get.call(this);
    },
  });
}
const mermaid = globalThis.mermaid;
mermaid.initialize({ startOnLoad: false });
const container = document.getElementById('container');
const { svg } = await mermaid.render('my-svg', args.text, container);
const counts = {};
for (const line of log) {
  const key = line.replace(/:\d+:\d+/g, '');
  counts[key] = (counts[key] || 0) + 1;
}
return { head: svg.slice(0, svg.indexOf('>')).replace(/style="[^"]*"/, 'style="..."').slice(100, 190), reads: Object.entries(counts).map(([k, n]) => `${n}x ${k}`.slice(0, 330)).slice(0, 30) };
