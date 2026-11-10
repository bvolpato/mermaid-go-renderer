// Records complete Mermaid math diagrams for label structure checks.
const out = [];
for (const c of args.cases) {
  mermaid.initialize({ startOnLoad: false });
  try {
    const result = await mermaid.render('my-svg', c.text, document.getElementById('container'));
    out.push({ ...c, svg: result.svg });
  } catch (e) {
    out.push({ ...c, error: e.message });
  }
  document.getElementById('container').innerHTML = '';
}
return out;
