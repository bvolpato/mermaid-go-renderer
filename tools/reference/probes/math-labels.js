// Records the real common.ts and createText.ts label output of Mermaid 12.2.0.
const { renderKatexSanitized, createText, getConfig } = await internals(
  'renderKatexSanitized', 'createText', 'getConfig'
);
const select = await internalsFrom('chunk-OHNBJXQ2', 'select_default');
const out = [];
for (const c of args.cases) {
  mermaid.initialize({ startOnLoad: false, ...c.directive });
  const config = getConfig();
  const row = { ...c, config: JSON.parse(JSON.stringify(config)) };
  try {
    row.markup = await renderKatexSanitized(c.text, config);
  } catch (e) {
    row.error = e.message;
  }
  try {
    const container = document.getElementById('container');
    container.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg"><g></g></svg>';
    const g = select(container.querySelector('g'));
    await createText(g, c.text, { useHtmlLabels: true, markdown: false, width: Infinity }, config);
    row.label = g.node().innerHTML;
    container.innerHTML = '';
  } catch (e) {
    row.labelError = e.message;
  }
  out.push(row);
}
return out;
