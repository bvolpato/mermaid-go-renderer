// Page script of gen-font-linux-probes.mjs. It measures text in Chromium
// for the Linux tests of internal/font.
//
// args.text: cases {text, family, size, weight, style}. The result of a
// case has the width of an HTML span, the height of its line, the length of
// an SVG text and its bounding box, as .parity/probe/text.js gives them.
//
// args.glyphs: cases {family, size, weight, style, chars}. The result of a
// case has, for each string of chars, what the canvas reports: the advance
// and the ink bounds [width, left, right, ascent, descent].
const svgNS = 'http://www.w3.org/2000/svg';
const host = document.createElement('div');
document.body.appendChild(host);
const svg = document.createElementNS(svgNS, 'svg');
document.body.appendChild(svg);
const text = [];
for (const c of args.text || []) {
  const style = `font-family:${c.family};font-size:${c.size}px;font-weight:${c.weight};font-style:${c.style}`;
  const span = document.createElement('span');
  span.style.cssText = style + ';white-space:pre;position:absolute;line-height:normal';
  span.textContent = c.text;
  host.appendChild(span);
  const r = span.getBoundingClientRect();
  const t = document.createElementNS(svgNS, 'text');
  t.setAttribute('style', style + ';white-space:pre');
  t.textContent = c.text;
  svg.appendChild(t);
  const b = t.getBBox();
  text.push({ ...c, htmlW: r.width, htmlH: r.height, svgLen: t.getComputedTextLength(), bx: b.x, by: b.y, bw: b.width, bh: b.height });
  span.remove();
  t.remove();
}
const glyphs = [];
const ctx = document.createElement('canvas').getContext('2d');
for (const c of args.glyphs || []) {
  ctx.font = `${c.style} ${c.weight} ${c.size}px ${c.family}`;
  const rows = c.chars.map((ch) => {
    const m = ctx.measureText(ch);
    return [m.width, m.actualBoundingBoxLeft, m.actualBoundingBoxRight, m.actualBoundingBoxAscent, m.actualBoundingBoxDescent];
  });
  const m = ctx.measureText('x');
  glyphs.push({ ...c, font: ctx.font, fontAscent: m.fontBoundingBoxAscent, fontDescent: m.fontBoundingBoxDescent, rows });
}
host.remove();
svg.remove();
return { text, glyphs };
