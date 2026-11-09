// The page side of the gantt oracle. Run it with oracle.mjs eval; see
// gen-gantt-oracle.mjs. args is { cases: [{ name, text }] }.
//
// mermaid keeps the state of the gantt db in module variables, and one of
// them (taskDb) is never cleared. So each case runs in new frames that load
// mermaid again: one frame for the dump of the db after the parse, and one
// frame for the render. A frame has the size of the viewport of the oracle,
// and it uses the clock and the seeded Math.random of the oracle page.
const mermaidSource = [...document.scripts].map((s) => s.text).find((t) => t.includes('mermaid'));
const SEEDED_RANDOM = `(() => {
  let a = 0x6d6d6467 >>> 0;
  Math.random = function () {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
})();`;

async function newFrame() {
  const frame = document.createElement('iframe');
  frame.style.cssText = 'position:absolute;left:0;top:0;width:800px;height:600px;border:0';
  frame.srcdoc = '<!doctype html><html><head><meta charset="utf-8"></head><body><div id="container"></div></body></html>';
  await new Promise((done) => {
    frame.onload = done;
    document.body.appendChild(frame);
  });
  const win = frame.contentWindow;
  // The fixed clock of the oracle page.
  win.Date = window.Date;
  win.eval(SEEDED_RANDOM);
  const script = win.document.createElement('script');
  script.text = mermaidSource;
  win.document.head.appendChild(script);
  win.mermaid.initialize({ startOnLoad: false });
  return { frame, win };
}

const date = (d) => {
  if (d === undefined) return { undefined: true };
  if (d === null) return { null: true };
  if (typeof d.getTime !== 'function') return { other: String(d) };
  const ms = d.getTime();
  return Number.isNaN(ms) ? { invalid: true } : { ms, iso: d.toISOString() };
};
const flag = (v) => (v === undefined ? false : v);

async function dump(text) {
  const { frame, win } = await newFrame();
  const out = {};
  try {
    // The db is a module singleton: a diagram of an empty chart gives it.
    const db = (await win.mermaid.mermaidAPI.getDiagramFromText('gantt')).db;
    try {
      await win.mermaid.parse(text);
    } catch (e) {
      out.parseError = String((e && e.message) || e);
      return out;
    }
    out.dateFormat = db.getDateFormat();
    out.axisFormat = db.getAxisFormat();
    out.tickInterval = db.getTickInterval() ?? null;
    out.todayMarker = db.getTodayMarker();
    out.excludes = db.getExcludes();
    out.includes = db.getIncludes();
    out.sections = db.getSections();
    out.links = [...db.getLinks().entries()];
    out.title = db.getDiagramTitle();
    out.accTitle = db.getAccTitle();
    out.accDescription = db.getAccDescription();
    out.displayMode = db.getDisplayMode();
    out.weekday = db.getWeekday();
    out.inclusiveEndDates = db.endDatesAreInclusive();
    out.topAxis = db.topAxisEnabled();
    try {
      out.tasks = db.getTasks().map((t) => ({
        id: t.id ?? null,
        hasId: t.id !== undefined,
        section: t.section,
        type: t.type,
        task: t.task,
        classes: t.classes,
        active: flag(t.active),
        done: flag(t.done),
        crit: flag(t.crit),
        milestone: flag(t.milestone),
        vert: flag(t.vert),
        order: t.order,
        processed: t.processed,
        manualEndTime: t.manualEndTime,
        startTime: date(t.startTime),
        endTime: date(t.endTime),
        renderEndTime: date(t.renderEndTime),
        prevTaskId: t.prevTaskId ?? null,
        hasPrevTaskId: t.prevTaskId !== undefined,
        raw: t.raw.data,
      }));
    } catch (e) {
      out.tasksError = String((e && e.message) || e);
    }
    return out;
  } finally {
    frame.remove();
  }
}

async function render(text) {
  const { frame, win } = await newFrame();
  const container = win.document.getElementById('container');
  const out = { now: new win.Date().toISOString(), width: container.offsetWidth };
  try {
    const { svg } = await win.mermaid.render('my-svg', text, container);
    out.svg = svg;
  } catch (e) {
    out.error = String((e && e.message) || e);
  } finally {
    frame.remove();
  }
  return out;
}

const results = [];
for (const c of args.cases) {
  results.push({ name: c.name, db: await dump(c.text), ...(await render(c.text)) });
}
return { now: new Date().toISOString(), timezone: Intl.DateTimeFormat().resolvedOptions().timeZone, results };
