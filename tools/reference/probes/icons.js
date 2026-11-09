// Icon oracle: calls the real registerIconPacks, isIconAvailable and
// getIconSVG of mermaid (rendering-util/icons.ts), and the functions of
// @iconify/utils that the build contains. Run with:
//
//   node oracle.mjs esm-eval probes/icons.js --args cases.json --out out.json
//
// args is { packs: [{ name, icons | loader | loaderFails }], units: [...],
// groups: [{ directive, steps }] }.
//
// A unit is a direct call of one @iconify/utils function: { fn, ... }. See
// the switch below for the arguments. Its result is the JSON text of the
// return value, or the error.
//
// A step is { op: 'available', name } or { op: 'svg', name, customisations,
// extraAttributes }. The units and then the steps of all groups run in
// order in one page, because replaceIDs numbers the ids with a counter of
// the page. The result has the prefix of those ids, and for each group the
// effective config and the results of its steps.
const { getIconSVG, isIconAvailable, setDiagramConfigScope, addDirective, getConfig } = await internals(
  'getIconSVG',
  'isIconAvailable',
  'setDiagramConfigScope',
  'addDirective',
  'getConfig'
);
const reset = await internalsFrom('chunk-DLUCIBNR', 'reset');
// The chunk that has rendering-util/icons.ts has the @iconify/utils code.
const [iconChunk] = internalsWhere('getIconSVG');
const iconify = {};
for (const name of ['randomPrefix', 'stringToIcon', 'getIconData', 'iconToSVG', 'calculateSize', 'wrapSVGContent', 'replaceIDs', 'iconToHTML']) {
  iconify[name] = await internalsFrom(iconChunk, name);
}
const randomPrefix = iconify.randomPrefix;
mermaid.initialize({ startOnLoad: false });
const out = { randomPrefix, randomCallsAtLoad: globalThis.__mmdgRandomCalls, registerErrors: [], units: [], groups: [] };
for (const pack of args.packs) {
  try {
    // A loader is a function, which JSON cannot hold: the pack has the
    // icon set that the loader returns.
    const loader = pack.loaderFails
      ? { name: pack.name, loader: async () => { throw new Error('no network'); } }
      : pack.loader
        ? { name: pack.name, loader: async () => pack.loader }
        : pack;
    mermaid.registerIconPacks([loader]);
    out.registerErrors.push(null);
  } catch (e) {
    out.registerErrors.push(String((e && e.message) || e));
  }
}
globalThis.__mmdgResetRandom();
for (const unit of args.units || []) {
  const before = globalThis.__mmdgRandomCalls;
  const r = {};
  try {
    let v;
    switch (unit.fn) {
      case 'stringToIcon':
        v = iconify.stringToIcon(unit.value, unit.validate, unit.allowSimpleName);
        break;
      case 'getIconData':
        v = iconify.getIconData(args.packs[unit.pack].icons, unit.name);
        break;
      case 'iconToSVG':
        v = iconify.iconToSVG(unit.icon, unit.customisations);
        break;
      case 'calculateSize':
        v = iconify.calculateSize(unit.size, unit.num / unit.den);
        v = { type: typeof v, text: String(v) };
        break;
      case 'wrapSVGContent':
        v = iconify.wrapSVGContent(unit.body, unit.start, unit.end);
        break;
      case 'replaceIDs':
        v = iconify.replaceIDs(unit.body);
        break;
      case 'iconToHTML':
        v = iconify.iconToHTML(unit.body, unit.attributes);
        break;
      default:
        throw new Error('unknown unit ' + unit.fn);
    }
    r.json = JSON.stringify(v === undefined ? null : v);
  } catch (e) {
    r.error = String((e && e.message) || e);
  }
  r.randomCalls = globalThis.__mmdgRandomCalls - before;
  out.units.push(r);
}
for (const group of args.groups) {
  reset();
  setDiagramConfigScope('flowchart-v2');
  addDirective(group.directive || {});
  const res = { config: JSON.parse(JSON.stringify(getConfig())), steps: [] };
  for (const step of group.steps) {
    const before = globalThis.__mmdgRandomCalls;
    const r = {};
    try {
      if (step.op === 'available') {
        r.available = await isIconAvailable(step.name);
      } else {
        // A customisations value that is absent stays undefined.
        r.svg = await getIconSVG(step.name, step.customisations, step.extraAttributes);
      }
    } catch (e) {
      r.error = String((e && e.message) || e);
    }
    r.randomCalls = globalThis.__mmdgRandomCalls - before;
    res.steps.push(r);
  }
  out.groups.push(res);
}
return out;
