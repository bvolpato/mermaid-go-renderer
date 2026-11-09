// Browser side of gen-browser-transform-probe.mjs. For each case it puts a
// rect in a group that has the transform attribute, inside a group with no
// transform. It returns the matrix that Chromium uses for the group
// (getCTM, six doubles) and the bounding box of the outer group, which is
// the box of the rect mapped by that matrix.
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  container.innerHTML =
    '<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg">' +
    `<g id="p"><g id="t" transform="${c.transform}"><rect x="${c.rect[0]}" y="${c.rect[1]}" width="${c.rect[2]}" height="${c.rect[3]}"/></g></g></svg>`;
  const m = container.querySelector('#t').getCTM();
  const b = container.querySelector('#p').getBBox();
  out.push({ name: c.name, matrix: [m.a, m.b, m.c, m.d, m.e, m.f], bbox: [b.x, b.y, b.width, b.height] });
}
container.innerHTML = '';
return out;
