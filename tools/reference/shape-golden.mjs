// The file format of the shape golden files of internal/ru/rutest.
//
// A golden file is { format, configs, styles, cases }. Many cases have the
// same effective config (about 25 KB) and the same style sheet, so a case
// holds the index of its config in configs and of its style sheet in
// styles. rutest.LoadShapeCases reads this form, and also the older form:
// an array of cases that each hold their config and style sheet.
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';

export const FORMAT = 'shape-golden-2';

// encodeGolden returns the compact form of a list of cases.
export function encodeGolden(cases) {
  const configs = [];
  const styles = [];
  const configIndex = new Map();
  const styleIndex = new Map();
  const out = cases.map((c) => {
    const copy = { ...c };
    if (c.config !== undefined && typeof c.config !== 'number') {
      const key = JSON.stringify(c.config);
      if (!configIndex.has(key)) {
        configIndex.set(key, configs.length);
        configs.push(c.config);
      }
      copy.config = configIndex.get(key);
    }
    if (typeof c.css === 'string') {
      if (!styleIndex.has(c.css)) {
        styleIndex.set(c.css, styles.length);
        styles.push(c.css);
      }
      copy.css = styleIndex.get(c.css);
    }
    return copy;
  });
  return { format: FORMAT, configs, styles, cases: out };
}

// decodeGolden returns the list of cases of either form.
export function decodeGolden(data) {
  if (Array.isArray(data)) {
    return data;
  }
  return data.cases.map((c) => ({
    ...c,
    config: typeof c.config === 'number' ? data.configs[c.config] : c.config,
    css: typeof c.css === 'number' ? data.styles[c.css] : c.css,
  }));
}

export function readGolden(file) {
  const raw = fs.readFileSync(file);
  return decodeGolden(JSON.parse(file.endsWith('.gz') ? zlib.gunzipSync(raw) : raw));
}

export function writeGolden(file, cases) {
  const json = JSON.stringify(encodeGolden(cases));
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, file.endsWith('.gz') ? zlib.gzipSync(json, { level: 9 }) : json);
}

// As a command: node shape-golden.mjs <golden files...> rewrites files of
// the older form in the compact form.
if (process.argv[1] && path.resolve(process.argv[1]) === new URL(import.meta.url).pathname) {
  for (const file of process.argv.slice(2)) {
    const raw = fs.readFileSync(file);
    const data = JSON.parse(file.endsWith('.gz') ? zlib.gunzipSync(raw) : raw);
    const isCases = (list) => Array.isArray(list) && list.length > 0 && list.every((c) => c && typeof c === 'object' && 'name' in c && ('config' in c || 'error' in c));
    if (!isCases(Array.isArray(data) ? data : data.cases)) {
      console.error(`skip ${file}: not a shape golden file`);
      continue;
    }
    const before = raw.length;
    writeGolden(file, decodeGolden(data));
    console.error(`${file}: ${before} -> ${fs.statSync(file).size} bytes`);
  }
}
