// Shared code for gen-stylis.mjs and gen-styles.mjs.
//
// It loads the built mermaid (dist/mermaid.core.mjs) in node and exposes the
// internal functions that make the <style> text. Run with tools/reference as
// the working directory.
//
// mermaid.core.mjs imports its dependencies from node_modules. use-bundled.mjs
// sends the imports of stylis and es-toolkit to the versions that the mermaid
// bundle contains (stylis 4.3.6, es-toolkit 1.45.1), because node_modules has
// newer versions that give other results.
import './use-bundled.mjs';
import { registerHooks } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const HERE = path.dirname(fileURLToPath(import.meta.url));
export const REPO = path.resolve(HERE, '../..');

// The functions below are module-level bindings of mermaid.core.mjs that the
// module does not export. A load hook adds the export statement in memory.
const INTERNALS = [
  'compileCSS',
  'createUserStyles',
  'createCssStyles',
  'processAndSetConfigs',
  'addDiagrams',
  'getConfig',
  'detectType',
  'getDiagram',
  'getDiagramLoader',
  'registerDiagram',
  'styles_default',
  'themes_default',
];

registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/mermaid/dist/mermaid.core.mjs')) {
      const names = INTERNALS.map((n) => `${n} as __${n}`).join(', ');
      return { ...result, source: `${result.source.toString()}\nexport { ${names} };\n` };
    }
    return result;
  },
});

// ---------------------------------------------------------------------------
// CSSStyleSheet stub.
//
// Node has no CSSOM. Mermaid uses it in two places on the way to the <style>
// text: createCssStyles (the ":root" font variables, classDefs, themeCSS) and
// the c4 style template (font rules per element type). The stub covers the
// rule texts that mermaid makes for the font variables and for c4. It has no
// replaceSync, so mermaid takes its documented branch for hosts without it and
// adds config.themeCSS as written. internal/styles ports the same rules.

const CSS_WIDE = new Set(['inherit', 'initial', 'unset', 'revert', 'revert-layer']);
const GENERIC_FAMILIES = new Set([
  'serif',
  'sans-serif',
  'cursive',
  'fantasy',
  'monospace',
  'system-ui',
  'math',
  'ui-serif',
  'ui-sans-serif',
  'ui-monospace',
  'ui-rounded',
]);
const IDENT = /^-?(?:[A-Za-z_]|[^\x00-\x7f])(?:[\w-]|[^\x00-\x7f])*$|^--(?:[\w-]|[^\x00-\x7f])*$/;
const FONT_SIZE_KEYWORDS = new Set([
  'xx-small',
  'x-small',
  'small',
  'medium',
  'large',
  'x-large',
  'xx-large',
  'xxx-large',
  'larger',
  'smaller',
  'math',
]);
const LENGTH_UNITS = new Set([
  'px', 'em', 'rem', 'ex', 'rex', 'ch', 'rch', 'ic', 'ric', 'lh', 'rlh', 'cap', 'rcap',
  'vw', 'vh', 'vi', 'vb', 'vmin', 'vmax', 'svw', 'svh', 'svi', 'svb', 'svmin', 'svmax',
  'lvw', 'lvh', 'lvi', 'lvb', 'lvmin', 'lvmax', 'dvw', 'dvh', 'dvi', 'dvb', 'dvmin', 'dvmax',
  'cqw', 'cqh', 'cqi', 'cqb', 'cqmin', 'cqmax', 'cm', 'mm', 'q', 'in', 'pt', 'pc',
]);
const NUMBER = /^([+-]?(?:\d+\.?\d*|\.\d+)(?:e[+-]?\d+)?)([a-z%]*)$/i;

const cssString = (s) => `"${s.replace(/[\\"]/g, '\\$&')}"`;

// Each function returns the text that Chrome gives back for the value, or
// null when Chrome ignores the declaration.
function fontFamilyValue(input) {
  const value = input.trim();
  if (value === '') return null;
  if (value.includes('var(')) return value;
  if (CSS_WIDE.has(value.toLowerCase())) return value.toLowerCase();
  const out = [];
  let i = 0;
  const n = value.length;
  const skip = () => {
    while (i < n && /\s/.test(value[i])) i++;
  };
  for (;;) {
    skip();
    if (i >= n) return null;
    const c = value[i];
    if (c === '"' || c === "'") {
      let name = '';
      i++;
      for (;;) {
        if (i >= n) return null;
        if (value[i] === c) {
          i++;
          break;
        }
        if (value[i] === '\\') {
          i++;
          if (i >= n) return null;
        }
        name += value[i++];
      }
      out.push(IDENT.test(name) && !CSS_WIDE.has(name.toLowerCase()) && !GENERIC_FAMILIES.has(name.toLowerCase()) && name.toLowerCase() !== 'default' ? name : cssString(name));
    } else {
      const start = i;
      while (i < n && value[i] !== ',') i++;
      const words = value.slice(start, i).trim().split(/\s+/);
      if (words.some((w) => !IDENT.test(w))) return null;
      if (words.length === 1) {
        const lower = words[0].toLowerCase();
        if (CSS_WIDE.has(lower) || lower === 'default') return null;
        out.push(GENERIC_FAMILIES.has(lower) ? lower : words[0]);
      } else {
        out.push(cssString(words.join(' ')));
      }
    }
    skip();
    if (i >= n) break;
    if (value[i] !== ',') return null;
    i++;
  }
  return out.join(', ');
}

function fontSizeValue(input) {
  const value = input.trim();
  if (value === '') return null;
  if (value.includes('var(')) return value;
  const lower = value.toLowerCase();
  if (CSS_WIDE.has(lower) || FONT_SIZE_KEYWORDS.has(lower)) return lower;
  const m = NUMBER.exec(value);
  if (!m) return null;
  const num = Number(m[1]);
  const unit = m[2].toLowerCase();
  if (!Number.isFinite(num) || num < 0) return null;
  if (unit === '') return num === 0 ? '0px' : null;
  if (unit !== '%' && !LENGTH_UNITS.has(unit)) return null;
  return `${num}${unit}`;
}

function fontWeightValue(input) {
  const value = input.trim();
  if (value === '') return null;
  if (value.includes('var(')) return value;
  const lower = value.toLowerCase();
  if (CSS_WIDE.has(lower) || ['normal', 'bold', 'bolder', 'lighter'].includes(lower)) return lower;
  const m = NUMBER.exec(value);
  if (!m || m[2] !== '') return null;
  const num = Number(m[1]);
  if (!Number.isFinite(num) || num < 1 || num > 1000) return null;
  return `${num}`;
}

const VALUE_FUNCS = {
  'font-family': fontFamilyValue,
  'font-size': fontSizeValue,
  'font-weight': fontWeightValue,
};

class StubStyleDeclaration {
  constructor() {
    this.decls = [];
  }
  get length() {
    return this.decls.length;
  }
  setProperty(name, value) {
    const fn = VALUE_FUNCS[name];
    if (!fn) throw new Error(`CSSStyleSheet stub: setProperty(${name}) is not supported`);
    const text = fn(String(value));
    if (text === null) return;
    const found = this.decls.find((d) => d[0] === name);
    if (found) found[1] = text;
    else this.decls.push([name, text]);
  }
  get cssText() {
    return this.decls.map(([n, v]) => `${n}: ${v};`).join(' ');
  }
}

class StubStyleRule {
  constructor(selectorText) {
    this.selectorText = selectorText;
    this.style = new StubStyleDeclaration();
  }
  get cssText() {
    const body = this.style.cssText;
    return body === '' ? `${this.selectorText} { }` : `${this.selectorText} { ${body} }`;
  }
}

// rootVariableRule is the cssText of ":root { --name: value}" for a value
// that has no ";", "{", "}" or "!" in the middle. A custom property keeps its
// value as written, without the white space around it.
const trimCss = (v) => v.replace(/^[ \t\n\r\f]+|[ \t\n\r\f]+$/g, '');
export function rootVariableRule(name, value) {
  let v = trimCss(String(value));
  if (v.endsWith(';')) v = trimCss(v.slice(0, -1));
  return `:root { ${name}: ${v}; }`;
}

class StubCSSStyleSheet {
  constructor() {
    this.cssRules = [];
  }
  insertRule(text, index = 0) {
    let rule;
    let m;
    if ((m = /^:root \{ (--mermaid-(?:alt-)?font-family): ([\s\S]*)\}$/.exec(text))) {
      rule = { cssText: rootVariableRule(m[1], m[2]) };
    } else if ((m = /^([^{}]+) \{\}$/.exec(text))) {
      rule = new StubStyleRule(m[1]);
    } else {
      throw new Error(`CSSStyleSheet stub: insertRule(${JSON.stringify(text)}) is not supported`);
    }
    this.cssRules.splice(index, 0, rule);
    return index;
  }
}

globalThis.CSSStyleSheet = StubCSSStyleSheet;

// ---------------------------------------------------------------------------

export async function loadMermaid() {
  const m = await import(path.join(HERE, 'node_modules/mermaid/dist/mermaid.core.mjs'));
  const api = { mermaid: m.default };
  for (const name of INTERNALS) {
    api[name] = m[`__${name}`];
  }
  api.mermaid.initialize({ startOnLoad: false });
  api.addDiagrams();
  return api;
}

// loadDiagram does what Diagram.fromText does before it parses: it loads the
// diagram of the type and registers it, which also registers its styles.
export async function loadDiagram(api, type) {
  try {
    return api.getDiagram(type);
  } catch {
    const loader = api.getDiagramLoader(type);
    if (!loader) {
      throw new Error(`Diagram ${type} not found.`);
    }
    const { id, diagram } = await loader();
    api.registerDiagram(id, diagram);
  }
  return api.getDiagram(type);
}

// encode makes a JSON-safe copy of a config value. JSON has no NaN, Infinity,
// -0 or undefined, so they get the tagged form { "\u0001js": "NaN" } that the
// Go side decodes. A property that holds a function is left out, as
// JSON.stringify does: the default config has font getters such as
// sequence.messageFont, and the Go config model has no functions.
export const TAG = '\u0001js';
export function encode(v) {
  if (v === undefined) return { [TAG]: 'undefined' };
  if (typeof v === 'number' && !Number.isFinite(v)) return { [TAG]: String(v) };
  if (typeof v === 'number' && Object.is(v, -0)) return { [TAG]: '-0' };
  if (v === null || typeof v !== 'object') {
    if (typeof v === 'function' || typeof v === 'symbol' || typeof v === 'bigint') {
      throw new Error(`cannot encode a ${typeof v}`);
    }
    return v;
  }
  // Array.from reads a hole as undefined, which is what a read of it gives.
  if (Array.isArray(v)) return Array.from(v, encode);
  const out = {};
  for (const k of Object.keys(v)) {
    if (k === TAG) throw new Error('cannot encode the tag key');
    if (typeof v[k] === 'function') continue;
    // defineProperty also works for the key "__proto__".
    Object.defineProperty(out, k, { value: encode(v[k]), enumerable: true, writable: true, configurable: true });
  }
  return out;
}

// mulberry32 is the seeded PRNG that the golden generators share.
export function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
