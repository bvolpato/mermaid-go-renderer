// Random dates for the time generators: a helper for gen-d3-time.mjs,
// gen-d3-timefmt.mjs and gen-d3-scale.mjs.
import { pick, int } from './d3-golden-lib.mjs';

const DAY = 864e5;

// transitions returns the instants at which the local offset changes.
export function transitions() {
  const out = [];
  for (let t = Date.UTC(1880, 0, 1), prev = new Date(t).getTimezoneOffset(); t < Date.UTC(2040, 0, 1); t += DAY) {
    const off = new Date(t).getTimezoneOffset();
    if (off !== prev) {
      let lo = t - DAY, hi = t;
      while (hi - lo > 1000) {
        const mid = Math.floor((lo + hi) / 2000) * 1000;
        if (new Date(mid).getTimezoneOffset() === prev) lo = mid; else hi = mid;
      }
      out.push(hi);
      prev = off;
    }
  }
  return out;
}

// dateSource returns a function that gives random time values: recent dates,
// dates near a change of the local offset, round dates, old and far dates,
// and sometimes NaN.
export function dateSource(rnd, { nan = 0.01, far = 0.04 } = {}) {
  const tr = transitions();
  return function randomTime() {
    const k = rnd();
    if (k < nan) return NaN;
    if (k < nan + far) return Math.floor((rnd() * 2 - 1) * 8.64e15);
    if (k < nan + far + 0.02) return pick(rnd, [0, -1, 1, 8.64e15, -8.64e15, 946684800000, 951782400000, 1709164800000]);
    if (tr.length && k < 0.3) return pick(rnd, tr) + int(rnd, -3 * DAY, 3 * DAY);
    if (k < 0.5) return Date.UTC(int(rnd, 2000, 2030), int(rnd, 0, 11), int(rnd, 1, 31), pick(rnd, [0, 0, 12, int(rnd, 0, 23)]), pick(rnd, [0, 0, 30, int(rnd, 0, 59)]));
    if (k < 0.6) return +new Date(int(rnd, 2000, 2030), int(rnd, 0, 11), int(rnd, 1, 28));
    if (k < 0.7) return Date.UTC(int(rnd, 2000, 2030), int(rnd, 0, 11), pick(rnd, [1, 1, 28, 29, 30, 31]), pick(rnd, [0, 23]), pick(rnd, [0, 59]), pick(rnd, [0, 59]), pick(rnd, [0, 999]));
    if (k < 0.92) return Date.UTC(1970, 0, 1) + Math.floor((rnd() * 2 - 0.4) * 80 * 365 * DAY);
    if (k < 0.97) return Date.UTC(int(rnd, -200, 3000), int(rnd, 0, 11), int(rnd, 1, 28), int(rnd, 0, 23), int(rnd, 0, 59), int(rnd, 0, 59), int(rnd, 0, 999));
    return Date.UTC(int(rnd, 1, 150), int(rnd, 0, 11), int(rnd, 1, 28)) - Date.UTC(1900, 0, 1) + Date.UTC(0, 0, 1);
  };
}

export function tzFileSuffix() {
  const tz = process.env.TZ || 'UTC';
  return tz === 'UTC' ? 'utc' : tz.replace(/.*\//, '').toLowerCase();
}
