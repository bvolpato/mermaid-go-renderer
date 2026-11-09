// Golden generator for the gantt db of internal/diagrams/gantt in a local
// time zone that is not UTC. The browser oracle runs in UTC only, so this
// generator runs the real mermaid 12.2.0 gantt code in plain node:
//
//   TZ=America/New_York node gen-gantt-db-tz.mjs
//
// For each case it does the config step and the parse step of
// mermaid.render (processAndSetConfigs, then what Diagram.fromText does with
// the gantt diagram definition), and it stores the dump of the db. The
// gantt module keeps state that it never clears (taskDb), so each case
// loads a new instance of the gantt chunk.
//
// The clock is the clock of the reference page. node has no DOM, so
// DOMPurify does nothing there: the generator installs DOMPurify.sanitize as
// the identity function, which is what the real function does for a string
// that has no "<", and it leaves out a case that sanitizes a string with
// "<".
//
// Output: internal/diagrams/gantt/testdata/db_new_york.json.gz
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';

const FIXED_TIME = Date.parse('2026-01-15T12:00:00Z');
const RealDate = Date;
class FixedDate extends RealDate {
  constructor(...args) {
    if (args.length === 0) super(FIXED_TIME);
    else super(...args);
  }
  static now() {
    return FIXED_TIME;
  }
}
globalThis.Date = FixedDate;

const { default: DOMPurify } = await import('dompurify');
const { internals } = await import('./lib-mermaid-internals.mjs');

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '../..');
const CHUNKS = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.core');
const tz = process.env.TZ || 'UTC';

class StubError extends Error {}
DOMPurify.sanitize = (dirty) => {
  if (String(dirty).includes('<')) throw new StubError('markup');
  return dirty;
};
DOMPurify.addHook = () => {};

const { mermaid, processAndSetConfigs } = await internals();
mermaid.initialize({ startOnLoad: false });

const findChunk = (test) => {
  for (const file of fs.readdirSync(CHUNKS)) {
    if (file.endsWith('.mjs') && test(file, fs.readFileSync(path.join(CHUNKS, file), 'utf8'))) return path.join(CHUNKS, file);
  }
  throw new Error('chunk not found');
};
const exportsName = (name) => (file, source) => new RegExp(`\\b${name}\\b`).test(source.slice(source.lastIndexOf('\nexport {')));
const ganttChunk = pathToFileURL(findChunk((file) => file.startsWith('ganttDiagram-'))).href;
const { encodeEntities } = await import(pathToFileURL(findChunk(exportsName('encodeEntities'))).href);
if (typeof encodeEntities !== 'function') throw new Error('encodeEntities not found');

const date = (d) => {
  if (d === undefined) return { undefined: true };
  if (d === null) return { null: true };
  if (typeof d.getTime !== 'function') return { other: String(d) };
  const ms = d.getTime();
  return Number.isNaN(ms) ? { invalid: true } : { ms, iso: d.toISOString() };
};
const flag = (v) => (v === undefined ? false : v);

let instance = 0;
async function dump(text) {
  const out = {};
  let processed;
  let code;
  try {
    processed = processAndSetConfigs(text);
    // The text that Diagram.fromText gets: the code without the comments.
    code = processed.code.cleaned;
    if (mermaid.detectType(code) !== 'gantt') return null;
  } catch {
    return null;
  }
  const { diagram } = await import(`${ganttChunk}?instance=${instance++}`);
  const { db, parser } = diagram;
  try {
    parser.parser.yy = db;
    db.clear();
    if (processed.title) db.setDiagramTitle(processed.title);
    parser.parse(encodeEntities(code) + '\n');
  } catch (e) {
    if (e instanceof StubError) return null;
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
}

function walk(dir) {
  const out = [];
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...walk(p));
    else if (e.name.endsWith('.mmd')) out.push(p);
  }
  return out.sort();
}

// Cases for local time: dates near the changes of the offset of
// America/New_York (2024-03-10 and 2024-11-03), dates that the Date
// constructor reads as UTC or as local time, and "today".
const local = {
  'dst-days': `gantt
  dateFormat YYYY-MM-DD
  excludes weekends
  section Spring
  Over the change :a, 2024-03-08, 5d
  Hours after :b, after a, 48h
  Weeks :c, 2024-03-01, 2w
  Month :d, 2024-02-29, 1M
  section Fall
  Over the change :e, 2024-11-01, 5d
  Hours :f, 2024-11-02, 36h
  Until :g, 2024-10-30, until e
  Manual :h, 2024-11-02, 2024-11-05
`,
  'dst-times': `gantt
  dateFormat YYYY-MM-DD HH:mm
  excludes sunday
  Before the gap :a, 2024-03-10 01:30, 2h
  In the gap :b, 2024-03-10 02:30, 1d
  After the gap :c, 2024-03-10 03:00, 90m
  Repeated hour :d, 2024-11-03 01:30, 3h
  Day over it :e, 2024-11-02 12:00, 1d
  Next :1d
  Midnight :f, 2024-11-03 00:00, 2024-11-04 00:00
`,
  'no-format': `gantt
  ISO date is UTC :a, 2024-03-10, 3d
  Legacy date is local :b, 2024-3-10, 3d
  Text date :c, Mar 10 2024 10:00, 2024-03-12
  ISO date and time is local :d, 2024-11-03T01:30, 1d
  With a zone :e, 2024-11-03T01:30:00Z, 2024-11-03T09:00:00+02:00
  Zone name :f, Nov 3 2024 01:30 EST, 5h
`,
  'today': `gantt
  dateFormat YYYY-MM-DD
  inclusiveEndDates
  excludes weekends
  After nothing :a, after missing, 3d
  Until nothing :b, 2026-01-10, until missing
  Inclusive :c, 2026-01-12, 2026-01-16
`,
  'zone-token': `gantt
  dateFormat YYYY-MM-DDTHH:mm:ssZ
  Winter :a, 2024-01-09T22:00:00-05:00, 1d
  Summer :b, 2024-07-09T22:00:00-04:00, 2024-07-10T22:00:00-04:00
  Other zone :c, 2024-07-09T22:00:00+00:00, 1d
`,
  'timestamps': `gantt
  dateFormat x
  excludes 2024-03-10
  A :a, 1710054000000, 1710140400000
  B :b, 1710054000000, 2d
  C :c, after a, 12h
`,
  'time-only': `gantt
  dateFormat HH:mm
  Wake :a, 06:30, 30m
  Late :b, 23:30, 2h
  Day :c, 00:00, 1d
`,
  'excludes-format-with-time': `gantt
  dateFormat YYYY-MM-DD HH:mm
  excludes 2024-03-11 00:00, 2024-03-12
  includes 2024-03-12 08:00
  A :a, 2024-03-09 08:00, 5d
  B :b, 2024-03-10 00:00, 4d
`,
  'end-of-month': `gantt
  dateFormat YYYY-MM-DD
  A :a, 2024-01-31, 1M
  B :b, 2024-10-31, 1M
  C :c, 2024-02-29, 1y
  D :d, 2023-11-05, 0.5M
`,
};

const cases = [];
let skipped = 0;
async function add(name, text, inline) {
  const db = await dump(text);
  if (db === null) {
    skipped++;
    return;
  }
  const c = { name, db };
  if (inline) c.text = text;
  cases.push(c);
}
for (const [name, text] of Object.entries(local)) await add('local/' + name, text, true);
const corpus = path.join(ROOT, 'testdata/corpus/mermaid-e2e');
for (const file of walk(path.join(corpus, 'gantt'))) {
  await add('corpus/' + path.relative(corpus, file).replace(/\.mmd$/, ''), fs.readFileSync(file, 'utf8'));
}
const casesDir = path.join(ROOT, 'internal/diagrams/gantt/testdata/cases');
for (const file of walk(casesDir)) {
  await add('cases/' + path.relative(casesDir, file).replace(/\.mmd$/, ''), fs.readFileSync(file, 'utf8'));
}

const file = path.join(ROOT, 'internal/diagrams/gantt/testdata', 'db_' + (tz === 'UTC' ? 'utc' : tz.replace(/.*\//, '').toLowerCase()) + '.json.gz');
const gz = zlib.gzipSync(Buffer.from(JSON.stringify({ tz, now: new Date().toISOString(), offsetNow: new Date().getTimezoneOffset(), cases })), { level: 9 });
fs.writeFileSync(file, gz);
console.log(`${cases.length} cases (${skipped} left out), ${cases.filter((c) => c.db.parseError || c.db.tasksError).length} with an error -> ${file} (${(gz.length / 1024).toFixed(0)} KiB)`);
