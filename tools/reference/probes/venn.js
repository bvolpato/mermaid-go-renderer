// Runs the functions of @upsetjs/venn.js 2.0.0 for the golden files of
// internal/venn. The file is the body of an async function with `args` in
// scope: { cases: [...] }. It returns one result for each case.
//
// gen-venn.mjs runs it in two places:
//   - in node, with globalThis.__vennLib set to the module build/venn.esm.js
//     (with all its top-level functions exported), with the fdlibm functions
//     as Math.sin and so on, and with the seeded Math.random of the oracle;
//   - in Chromium, with the esm-eval command of the oracle: the functions are
//     then the ones in the vennDiagram chunk of mermaid, and Math is the Math
//     of the browser.
//
// Every number of a case and of a result is the hexadecimal text of the 64
// bits of the double, so that a file has NaN, the infinities and -0.
const names = [
  'venn',
  'layout',
  'normalizeSolution',
  'scaleSolution',
  'computeTextCentres',
  'computeTextCentre',
  'intersectionArea',
  'intersectionAreaPath',
  'circleOverlap',
  'circleArea',
  'circleCircleIntersection',
  'distanceFromIntersectArea',
  'greedyLayout',
  'bestInitialLayout',
  'constrainedMDSLayout',
  'lossFunction',
  'addMissingAreas',
  'VennDiagram',
];
let lib = globalThis.__vennLib;
let select = null;
if (!lib) {
  lib = {};
  for (const n of names) {
    lib[n] = await internalsFrom('vennDiagram', n);
  }
  select = await internalsFrom('vennDiagram', 'd3select').catch(() => null);
  if (!select) {
    const where = internalsWhere('select_default');
    select = await internalsFrom(where[0].replace(/\.mjs$/, ''), 'select_default');
  }
}

const view = new DataView(new ArrayBuffer(8));
const hex = (v) => {
  if (typeof v !== 'number') {
    return 'undefined';
  }
  if (v !== v) {
    return '7ff8000000000000';
  }
  view.setFloat64(0, v);
  return view.getUint32(0).toString(16).padStart(8, '0') + view.getUint32(4).toString(16).padStart(8, '0');
};
const unhex = (s) => {
  view.setUint32(0, parseInt(s.slice(0, 8), 16));
  view.setUint32(4, parseInt(s.slice(8), 16));
  return view.getFloat64(0);
};
const message = (e) => String((e && e.message) || e);
const reset = () => globalThis.__mmdgResetRandom();
const randomCalls = () => globalThis.__mmdgRandomCalls;

// An area of a case is [sets, size, label, weight]. label and weight are
// null when the property is undefined.
const decodeAreas = (areas) =>
  areas.map(([sets, size, label, weight]) => {
    const a = { sets: sets.slice(), size: unhex(size), label: label === null ? undefined : label };
    if (weight !== null) {
      a.weight = unhex(weight);
    }
    return a;
  });
// A circle of a case is [x, y, radius], or [id, x, y, radius] in a solution.
const decodeCircles = (list) => list.map(([x, y, r]) => ({ x: unhex(x), y: unhex(y), radius: unhex(r) }));
const decodeSolution = (list) => {
  const o = {};
  for (const [id, x, y, r] of list) {
    o[id] = { x: unhex(x), y: unhex(y), radius: unhex(r) };
  }
  return o;
};
const solutionOut = (o) => Object.keys(o).map((k) => [k, hex(o[k].x), hex(o[k].y), hex(o[k].radius)]);
const centreOut = (c) => (c ? [hex(c.x), hex(c.y), c.disjoint === true] : null);
const arcOut = (a, id) => [id(a.circle), hex(a.width), hex(a.p1.x), hex(a.p1.y), hex(a.p2.x), hex(a.p2.y), !!a.large, !!a.sweep];

function runLayout(c) {
  const out = {};
  const width = unhex(c.width);
  const height = unhex(c.height);
  const padding = unhex(c.padding);

  // The steps of layout(), one by one.
  reset();
  try {
    const data = decodeAreas(c.areas);
    const solution = lib.venn(data, { lossFunction: lib.lossFunction, distinct: undefined });
    out.venn = solutionOut(solution);
    out.vennRandom = randomCalls();
    const normalized = lib.normalizeSolution(solution, Math.PI / 2, undefined);
    out.normalized = solutionOut(normalized);
    const scaled = lib.scaleSolution(normalized, width, height, padding, false);
    out.scaled = solutionOut(scaled);
    const centres = lib.computeTextCentres(scaled, data, false);
    out.centres = data.map((a) => centreOut(centres[a.sets]));
  } catch (e) {
    out.stepsError = message(e);
  }

  // layout() as mermaid calls it.
  reset();
  try {
    const res = lib.layout(decodeAreas(c.areas), { width, height, padding });
    out.layout = res.map((a) => [
      centreOut(a.text),
      a.circles.map((k) => (k ? [k.set, hex(k.x), hex(k.y), hex(k.radius)] : null)),
      a.arcs.map((arc) => arcOut(arc, (k) => k.set)),
      a.path,
      a.distinctPath,
    ]);
    out.layoutRandom = randomCalls();
  } catch (e) {
    out.layoutError = message(e);
  }

  // The numbers of the chart of VennDiagram, without the DOM: the chart
  // removes the sets of size 0, has the padding 15 and does not round.
  reset();
  try {
    let data = decodeAreas(c.areas);
    const toRemove = new Set();
    data.forEach((datum) => {
      if (datum.size == 0 && datum.sets.length == 1) {
        toRemove.add(datum.sets[0]);
      }
    });
    data = data.filter((datum) => !datum.sets.some((set) => toRemove.has(set)));
    out.chart = { circles: [], paths: [], centres: [] };
    if (data.length > 0) {
      let solution = lib.venn(data, { lossFunction: lib.lossFunction, distinct: false });
      solution = lib.normalizeSolution(solution, Math.PI / 2, null);
      const circles = lib.scaleSolution(solution, width, height, 15, null);
      const centres = lib.computeTextCentres(circles, data, false);
      out.chart.circles = solutionOut(circles);
      out.chart.paths = data.map((d) => lib.intersectionAreaPath(d.sets.map((set) => circles[set])));
      out.chart.centres = data.map((d) => [String(Math.floor(centres[d.sets].x)), String(Math.floor(centres[d.sets].y))]);
    }
  } catch (e) {
    out.chartError = message(e);
  }
  return out;
}

function runIntersectionArea(c) {
  const circles = decodeCircles(c.circles);
  const stats = {};
  const area = lib.intersectionArea(circles, stats);
  return {
    area: hex(area),
    arcArea: hex(stats.arcArea),
    polygonArea: hex(stats.polygonArea),
    arcs: stats.arcs.map((a) => arcOut(a, (k) => circles.indexOf(k))),
    inner: stats.innerPoints.map((p) => [hex(p.x), hex(p.y), p.parentIndex[0], p.parentIndex[1]]),
    points: stats.intersectionPoints.length,
    path: lib.intersectionAreaPath(circles),
    path2: lib.intersectionAreaPath(circles, 2),
    path0: lib.intersectionAreaPath(circles, 0),
  };
}

function runScalar(c) {
  const v = c.values.map(unhex);
  switch (c.fn) {
    case 'circleOverlap':
      return { value: hex(lib.circleOverlap(v[0], v[1], v[2])) };
    case 'circleArea':
      return { value: hex(lib.circleArea(v[0], v[1])) };
    case 'distanceFromIntersectArea':
      return { value: hex(lib.distanceFromIntersectArea(v[0], v[1], v[2])) };
    case 'circleCircleIntersection': {
      const pts = lib.circleCircleIntersection({ x: v[0], y: v[1], radius: v[2] }, { x: v[3], y: v[4], radius: v[5] });
      return { points: pts.map((p) => [hex(p.x), hex(p.y)]) };
    }
  }
  throw new Error('unknown function ' + c.fn);
}

function runTextCentre(c) {
  const centre = lib.computeTextCentre(decodeCircles(c.interior), decodeCircles(c.exterior), false);
  return { centre: centreOut(centre) };
}

function runNormalize(c) {
  const out = {};
  const normalized = lib.normalizeSolution(decodeSolution(c.solution), Math.PI / 2, undefined);
  out.normalized = solutionOut(normalized);
  const scaled = lib.scaleSolution(normalized, unhex(c.width), unhex(c.height), unhex(c.padding), false);
  out.scaled = solutionOut(scaled);
  return out;
}

function runInitial(c) {
  const out = {};
  const stage = (name, fn) => {
    reset();
    try {
      const params = { lossFunction: lib.lossFunction, maxIterations: 500 };
      const areas = lib.addMissingAreas(decodeAreas(c.areas), params);
      const circles = fn(areas, params);
      out[name] = solutionOut(circles);
      out[name + 'Loss'] = hex(lib.lossFunction(circles, areas));
      out[name + 'Random'] = randomCalls();
    } catch (e) {
      out[name + 'Error'] = message(e);
    }
  };
  stage('greedy', (areas, params) => lib.greedyLayout(areas, params));
  stage('mds', (areas, params) => lib.constrainedMDSLayout(areas, params));
  stage('best', (areas, params) => lib.bestInitialLayout(areas, params));
  try {
    out.areas = lib.addMissingAreas(decodeAreas(c.areas), {}).map((a) => [a.sets, hex(a.size)]);
  } catch (e) {
    out.areasError = message(e);
  }
  return out;
}

// runChart calls the real chart on a new div. The div is detached, as it is
// in mermaid, or a child of the container of the page.
function runChart(c) {
  if (!select) {
    throw new Error('the chart needs a browser');
  }
  reset();
  const container = document.getElementById('container');
  const div = document.createElement('div');
  if (c.attach) {
    container.appendChild(div);
  }
  try {
    const chart = lib.VennDiagram().width(unhex(c.width)).height(unhex(c.height));
    select(div).datum(decodeAreas(c.areas)).call(chart);
    return { html: div.innerHTML, random: randomCalls() };
  } finally {
    div.remove();
  }
}

const runners = {
  layout: runLayout,
  intersectionArea: runIntersectionArea,
  scalar: runScalar,
  textCentre: runTextCentre,
  normalize: runNormalize,
  initial: runInitial,
  chart: runChart,
};

const results = [];
for (const c of args.cases) {
  let out;
  try {
    out = runners[c.kind](c);
  } catch (e) {
    out = { error: message(e) };
  }
  results.push(out);
}
return results;
