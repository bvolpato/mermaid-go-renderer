// Collation probe for internal/js LocaleCompare. It asks the ICU collator of
// Chromium (locale en-US, the locale of the reference page) for the order of
// single code points and derives a table of collation weights:
//
//   p  the rank of the primary weight (0: the code point is ignored)
//   s  the rank of the secondary weight among the code points with that p
//   t  the rank of the tertiary weight among the code points with that p, s
//
// It also returns pairs of strings with the result of String.localeCompare
// and with the result of the table model, as golden data.
//
// Run with:
//
//   node oracle.mjs eval probes/locale-compare.js --bare --args args.json
//
// args is { max, seed, pairs }. See gen-js-localecompare.mjs.
const max = args.max;
const variant = (a, b) => a.localeCompare(b);
const base = new Intl.Collator(undefined, { sensitivity: 'base' }).compare;
const accent = new Intl.Collator(undefined, { sensitivity: 'accent' }).compare;

const P = new Array(max).fill(0);
const S = new Array(max).fill(0);
const T = new Array(max).fill(0);
// 1: ignored on every level. 2: ignored on the primary level only (the
// model ignores it on every level).
const ignorable = new Array(max).fill(0);

const list = [];
for (let cp = 0; cp < max; cp++) {
  const ch = String.fromCodePoint(cp);
  if (variant(ch, '') === 0) {
    ignorable[cp] = 1;
  } else if (base(ch, '') === 0) {
    ignorable[cp] = 2;
  } else {
    list.push(cp);
  }
}
const chr = (cp) => String.fromCodePoint(cp);
list.sort((a, b) => base(chr(a), chr(b)) || a - b);
let p = 0;
let group = [];
const closeGroup = () => {
  if (group.length === 0) {
    return;
  }
  group.sort((a, b) => accent(chr(a), chr(b)) || a - b);
  let s = 0;
  let sub = [];
  const closeSub = () => {
    if (sub.length === 0) {
      return;
    }
    sub.sort((a, b) => variant(chr(a), chr(b)) || a - b);
    let t = 0;
    for (let i = 0; i < sub.length; i++) {
      if (i > 0 && variant(chr(sub[i - 1]), chr(sub[i])) !== 0) {
        t++;
      }
      P[sub[i]] = p;
      S[sub[i]] = s;
      T[sub[i]] = t;
    }
    sub = [];
  };
  for (let i = 0; i < group.length; i++) {
    if (i > 0 && accent(chr(group[i - 1]), chr(group[i])) !== 0) {
      closeSub();
      s++;
    }
    sub.push(group[i]);
  }
  closeSub();
  group = [];
};
for (let i = 0; i < list.length; i++) {
  if (i === 0 || base(chr(list[i - 1]), chr(list[i])) !== 0) {
    closeGroup();
    p++;
  }
  group.push(list[i]);
}
closeGroup();

// The model: what the Go code computes from the table.
const keys = (str) => {
  const out = [];
  for (const ch of str) {
    const cp = ch.codePointAt(0);
    if (cp < max) {
      if (P[cp] !== 0) {
        out.push([P[cp], S[cp], T[cp]]);
      }
    } else {
      out.push([0x10000 + cp, 0, 0]);
    }
  }
  return out;
};
const model = (a, b) => {
  const ka = keys(a);
  const kb = keys(b);
  for (let level = 0; level < 3; level++) {
    const n = Math.min(ka.length, kb.length);
    for (let i = 0; i < n; i++) {
      if (ka[i][level] !== kb[i][level]) {
        return ka[i][level] < kb[i][level] ? -1 : 1;
      }
    }
    if (level === 0 && ka.length !== kb.length) {
      return ka.length < kb.length ? -1 : 1;
    }
  }
  return 0;
};

// A seeded generator, so that the golden file is reproducible.
let state = args.seed >>> 0;
const random = () => {
  state = (state + 0x6d2b79f5) >>> 0;
  let t = state;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pick = (alphabet) => alphabet[Math.floor(random() * alphabet.length)];
const word = (alphabet, maxLen) => {
  const n = Math.floor(random() * (maxLen + 1));
  let s = '';
  for (let i = 0; i < n; i++) {
    s += pick(alphabet);
  }
  return s;
};

const ascii = [];
for (let cp = 0; cp < 0x80; cp++) {
  ascii.push(chr(cp));
}
// The characters of the ids that flowcharts have in practice.
const idChars = [...'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-. #:'];
const latin = [];
for (let cp = 0x20; cp < 0x250; cp++) {
  latin.push(chr(cp));
}
const all = [];
for (let cp = 0; cp < max; cp++) {
  all.push(chr(cp));
}
// Code points outside the table.
const beyond = [...'中文字日本語한국어ひらがなカタカナ€√∑😀🙂'];

const sets = [
  { name: 'ascii', alphabet: ascii, exact: true },
  { name: 'ids', alphabet: idChars, exact: true },
  { name: 'latin', alphabet: latin, exact: false },
  { name: 'table', alphabet: all, exact: false },
  { name: 'beyond', alphabet: [...idChars, ...beyond], exact: false },
];
const result = { max, P, S, T, ignorable, sets: [] };
for (const set of sets) {
  const pairs = [];
  let agree = 0;
  const add = (a, b) => {
    const real = Math.sign(variant(a, b));
    const m = model(a, b);
    if (real === m) {
      agree++;
    }
    pairs.push([a, b, real, m]);
  };
  // Every pair of single characters of the small alphabets.
  if (set.alphabet.length <= 128) {
    for (const a of set.alphabet) {
      for (const b of set.alphabet) {
        add(a, b);
      }
    }
  }
  for (let i = 0; i < args.pairs; i++) {
    const a = word(set.alphabet, 6);
    // Half of the pairs differ in a few characters only, so that the
    // secondary and tertiary levels decide.
    let b;
    if (random() < 0.5) {
      b = word(set.alphabet, 6);
    } else {
      const chars = [...a];
      const edits = 1 + Math.floor(random() * 2);
      for (let e = 0; e < edits && chars.length > 0; e++) {
        const at = Math.floor(random() * chars.length);
        const r = random();
        if (r < 0.4) {
          const c = chars[at];
          chars[at] = c === c.toLowerCase() ? c.toUpperCase() : c.toLowerCase();
        } else if (r < 0.7) {
          chars[at] = pick(set.alphabet);
        } else if (r < 0.85) {
          chars.splice(at, 1);
        } else {
          chars.splice(at, 0, pick(set.alphabet));
        }
      }
      b = chars.join('');
    }
    add(a, b);
  }
  result.sets.push({ name: set.name, exact: set.exact, agree, total: pairs.length, pairs });
}
return result;
