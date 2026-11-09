// The page side of the d3-axis oracle. Run it with oracle.mjs eval; see
// gen-d3-axis-cases.mjs. args is { d3: the source of d3.min.js, cases }.
(0, eval)(args.d3);
const d3 = globalThis.d3;

function makeScale(s) {
  let scale;
  if (s.type === 'linear') scale = d3.scaleLinear().domain(s.domain);
  else if (s.type === 'time') scale = d3.scaleTime().domain(s.domain.map((t) => new Date(t)));
  else if (s.type === 'band') scale = d3.scaleBand().domain(s.domain).round(s.round).paddingInner(s.paddingInner).paddingOuter(s.paddingOuter).align(s.align);
  else scale = d3.scalePoint().domain(s.domain).round(s.round).padding(s.padding).align(s.align);
  return s.rangeRound ? scale.rangeRound(s.range) : scale.range(s.range);
}

const intervals = {
  millisecond: d3.timeMillisecond, second: d3.timeSecond, minute: d3.timeMinute, hour: d3.timeHour, day: d3.timeDay, week: d3.timeWeek,
  monday: d3.timeMonday, thursday: d3.timeThursday, month: d3.timeMonth, year: d3.timeYear,
};

const results = [];
for (const c of args.cases) {
  const numeric = c.scale.type === 'linear' || c.scale.type === 'time';
  const axis = { top: d3.axisTop, right: d3.axisRight, bottom: d3.axisBottom, left: d3.axisLeft }[c.orient](makeScale(c.scale));
  if (c.ticks) axis.ticks(...c.ticks.map((a) => (a && typeof a === 'object' ? intervals[a.interval].every(a.every) : a)));
  if (c.tickValues) axis.tickValues(c.scale.type === 'time' ? c.tickValues.map((t) => new Date(t)) : c.tickValues);
  if (c.tickFormat) {
    if (c.tickFormat.kind === 'time') axis.tickFormat(d3.timeFormat(c.tickFormat.spec));
    else if (c.tickFormat.kind === 'number') axis.tickFormat(d3.format(c.tickFormat.spec));
    else axis.tickFormat((d) => '<' + (numeric ? +d : d) + '>');
  }
  if (c.tickSize !== undefined) axis.tickSize(c.tickSize);
  if (c.tickSizeInner !== undefined) axis.tickSizeInner(c.tickSizeInner);
  if (c.tickSizeOuter !== undefined) axis.tickSizeOuter(c.tickSizeOuter);
  if (c.tickPadding !== undefined) axis.tickPadding(c.tickPadding);
  if (c.offset !== undefined) axis.offset(c.offset);

  const svg = d3.select(document.body).append('svg');
  const g = svg.append('g');
  const html = [];
  try {
    g.call(axis);
    html.push(g.node().outerHTML);
    for (const step of c.steps ?? []) {
      axis.scale(makeScale(step.scale));
      g.call(axis);
      html.push(g.node().outerHTML);
    }
    results.push({ html });
  } catch (e) {
    results.push({ html, error: String(e) });
  }
  svg.remove();
}
return { devicePixelRatio: window.devicePixelRatio, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone, cases: args.cases, results };
