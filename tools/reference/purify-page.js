// Page script for gen-purify.mjs and gen-sanitize.mjs. oracle.mjs eval runs
// it as the body of an async function in Chromium, with `args` in scope.
//
// The mermaid IIFE bundle does not export DOMPurify or the helpers of
// common.ts. So this script loads the ESM build of mermaid 12.2.0 from the
// chunk sources in args.chunks. The chunk that defines sanitizeText also has
// the bundled DOMPurify. One extra export statement is appended to the chunk
// source to reach the functions that the chunk keeps private. The function
// bodies are the real ones, not copies.
const blob = (src) => URL.createObjectURL(new Blob([src], { type: 'text/javascript' }));
const helpersUrl = blob(args.chunks.helpers);
const rewrite = (src, map) => {
  for (const [from, to] of Object.entries(map)) {
    src = src.split(JSON.stringify('./' + from)).join(JSON.stringify(to));
  }
  return src;
};
const logUrl = blob(rewrite(args.chunks.log, { [args.names.helpers]: helpersUrl }));
const urlUrl = blob(rewrite(args.chunks.url, { [args.names.helpers]: helpersUrl }));
const PRIVATE = [
  'sanitizeMore',
  'breakToPlaceholder',
  'placeholderToBreak',
  'countOccurrence',
  'shouldCombineSets',
  'processSet',
  'setupDompurifyHooks',
  'removeScript',
  'getRows',
  'hasBreaks',
  'splitBreaks',
  'sanitizeTextOrArray',
  'getMax',
  'getMin',
  'katexRegex',
];
// load returns a new instance of the mermaid modules: a new DOMPurify with no
// hooks, and the common.ts state that says that the hooks are not set up.
async function load() {
  const mainSrc =
    rewrite(args.chunks.main, { [args.names.log]: logUrl, [args.names.helpers]: helpersUrl }) +
    '\nexport { ' +
    PRIVATE.map((n) => `${n} as __${n}`).join(', ') +
    ' };\n';
  const mainUrl = blob(mainSrc);
  const main = await import(mainUrl);
  const utils = await import(
    blob(
      rewrite(args.chunks.utils, {
        [args.names.url]: urlUrl,
        [args.names.main]: mainUrl,
        [args.names.log]: logUrl,
        [args.names.helpers]: helpersUrl,
      })
    )
  );
  const sanitizeUrl = (await import(urlUrl)).require_dist().sanitizeUrl;
  return { main, utils, sanitizeUrl };
}

const instances = {};
// instance returns the shared instance for a hook state. With hooks true,
// setupDompurifyHooks of common.ts has run once, as it has after the first
// removeScript call of a page.
async function instance(hooks) {
  const key = hooks ? 'on' : 'off';
  if (!instances[key]) {
    const m = await load();
    if (hooks) {
      m.main.__setupDompurifyHooks();
    }
    instances[key] = m;
  }
  return instances[key];
}

const attempt = (fn) => {
  try {
    return fn();
  } catch (e) {
    return { throw: String((e && e.message) || e), name: e && e.name };
  }
};

// describe turns the result of sanitize into JSON. A node result (RETURN_DOM)
// is serialized in its own inert document, so that the scripting state is the
// one of the DOMPurify document.
function describe(result) {
  if (typeof result === 'string') {
    return result;
  }
  if (result === null || result === undefined) {
    return { value: String(result) };
  }
  if (typeof result === 'object' && typeof result.nodeType === 'number') {
    if (result.nodeType === 11) {
      const t = result.ownerDocument.createElement('template');
      const str = Object.prototype.toString.call(result);
      t.content.appendChild(result);
      return { node: str, html: t.innerHTML };
    }
    return { node: Object.prototype.toString.call(result), html: result.outerHTML };
  }
  // A TrustedHTML object (RETURN_TRUSTED_TYPE): its string.
  return String(result);
}

// Getters of Node.prototype. A form returns a control for a property that
// has the name of the control, so the hooks of the hook jobs read these
// properties through the prototype, as DOMPurify does.
const protoGetter = (name) => Object.getOwnPropertyDescriptor(Node.prototype, name).get;
const nodeNameOf = (node) => protoGetter('nodeName').call(node);
const nodeTypeOf = (node) => protoGetter('nodeType').call(node);
const parentNodeOf = (node) => protoGetter('parentNode').call(node);

const out = { ua: navigator.userAgent, results: [] };
for (const job of args.jobs) {
  const m = await instance(Boolean(job.hooks));
  const purify = m.main.purify;
  let result;
  switch (job.kind) {
    case 'env': {
      const tdoc = document.createElement('template').content.ownerDocument;
      const names = (o) => {
        const s = new Set();
        for (; o; o = Object.getPrototypeOf(o)) {
          for (const k of Object.getOwnPropertyNames(o)) {
            s.add(k);
          }
        }
        return [...s].sort();
      };
      result = {
        version: purify.version,
        isSupported: purify.isSupported,
        documentNames: names(tdoc),
        formNames: names(tdoc.createElement('form')),
        trustedTypes: typeof window.trustedTypes,
        location: {
          protocol: location.protocol,
          host: location.host,
          pathname: location.pathname,
          search: location.search,
        },
      };
      break;
    }
    case 'purify': {
      // job.cfg is null for a call with no config argument. A new object is
      // passed for every input, as mermaid does with its object literals.
      result = job.inputs.map((input) => {
        const r = attempt(() =>
          describe(
            job.cfg === null
              ? purify.sanitize(input)
              : purify.sanitize(input, JSON.parse(JSON.stringify(job.cfg)))
          )
        );
        return r === input ? 0 : r;
      });
      break;
    }
    case 'purifyTrace': {
      // Every entry point gets a hook that records the call. The trace shows
      // the order in which DOMPurify visits nodes and attributes.
      const points = [
        'beforeSanitizeElements',
        'uponSanitizeElement',
        'afterSanitizeElements',
        'beforeSanitizeAttributes',
        'uponSanitizeAttribute',
        'afterSanitizeAttributes',
        'beforeSanitizeShadowDOM',
        'uponSanitizeShadowNode',
        'afterSanitizeShadowDOM',
      ];
      const short = ['bE', 'uE', 'aE', 'bA', 'uA', 'aA', 'bS', 'uS', 'aS'];
      result = job.inputs.map((input) => {
        const trace = [];
        points.forEach((point, k) => {
          purify.addHook(point, (node, data) => {
            let entry = short[k] + ' ' + nodeNameOf(node);
            if (point === 'uponSanitizeElement') {
              entry += ' ' + data.tagName;
            }
            if (point === 'uponSanitizeAttribute') {
              entry += ' ' + data.attrName + '=' + data.attrValue;
            }
            trace.push(entry);
          });
        });
        const r = attempt(() =>
          job.cfg === null ? purify.sanitize(input) : purify.sanitize(input, JSON.parse(JSON.stringify(job.cfg)))
        );
        purify.removeAllHooks();
        return { out: r, trace };
      });
      break;
    }
    case 'purifyMutate': {
      // Hooks that change the result through every field of the hook event.
      // internal/purify/hooks_test.go has the same hooks in Go.
      result = job.inputs.map((input) => {
        purify.addHook('uponSanitizeElement', (node, data) => {
          if (data.tagName === 'foo') {
            data.allowedTags.foo = true;
          }
          if (data.tagName === 'bar') {
            data.allowedTags.b = false;
          }
          if (data.tagName === 'gone' && parentNodeOf(node)) {
            parentNodeOf(node).removeChild(node);
          }
        });
        purify.addHook('uponSanitizeAttribute', (node, data) => {
          if (data.attrName === 'x-keep') {
            data.forceKeepAttr = true;
          }
          if (data.attrName === 'title') {
            data.attrValue = data.attrValue.toUpperCase();
          }
          if (data.attrName === 'class') {
            data.keepAttr = false;
          }
          if (data.attrName === 'x-allow') {
            data.allowedAttributes['x-allow'] = true;
          }
          if (data.attrName === 'x-forbid-id') {
            data.allowedAttributes.id = false;
          }
        });
        purify.addHook('afterSanitizeAttributes', (node) => {
          if (nodeNameOf(node) === 'P') {
            node.setAttribute('data-seen', '1');
          }
        });
        const first = purify.addHook('beforeSanitizeElements', (node) => {
          if (nodeTypeOf(node) === 3) {
            node.data = node.data.replace(/secret/g, '***');
          }
        });
        const r = attempt(() =>
          job.cfg === null ? purify.sanitize(input) : purify.sanitize(input, JSON.parse(JSON.stringify(job.cfg)))
        );
        // removeHook with no function removes the hook that was added last.
        const popped = typeof purify.removeHook('beforeSanitizeElements');
        const second = attempt(() => purify.sanitize(input));
        purify.removeHooks('uponSanitizeAttribute');
        const third = attempt(() => purify.sanitize(input));
        purify.removeAllHooks();
        const fourth = attempt(() => purify.sanitize(input));
        return { out: r, popped, second, third, fourth, first: typeof first };
      });
      break;
    }
    case 'parse': {
      // The tree that DOMParser builds, in the format of Node.DumpTree of
      // internal/dom.
      const HTML = 'http://www.w3.org/1999/xhtml';
      const NS = { [HTML]: 'h', 'http://www.w3.org/2000/svg': 's', 'http://www.w3.org/1998/Math/MathML': 'm' };
      const ATTR_NS = {
        'http://www.w3.org/1999/xlink': 'x',
        'http://www.w3.org/XML/1998/namespace': 'X',
        'http://www.w3.org/2000/xmlns/': 'n',
      };
      const esc = (str) =>
        str.replace(/[\\"\u0000-\u001f]/g, (c) =>
          c === '\\' ? '\\\\' : c === '"' ? '\\"' : '\\u' + c.charCodeAt(0).toString(16).padStart(4, '0')
        );
      // A form returns a control for a property that has the name of the
      // control. The getters of the prototypes are not affected.
      const elementGetter = (name) => Object.getOwnPropertyDescriptor(Element.prototype, name).get;
      const attributesOf = elementGetter('attributes');
      const namespaceOf = elementGetter('namespaceURI');
      const localNameOf = elementGetter('localName');
      const childNodesOf = protoGetter('childNodes');
      const kids = (node) => {
        let text = '';
        for (const c of childNodesOf.call(node)) {
          text += dump(c);
        }
        return text;
      };
      const dump = (node) => {
        switch (nodeTypeOf(node)) {
          case 1: {
            const namespaceURI = namespaceOf.call(node);
            const localName = localNameOf.call(node);
            let text = '<' + (NS[namespaceURI] || '?') + ':' + localName;
            for (const a of attributesOf.call(node)) {
              text += ' ' + a.name;
              if (a.namespaceURI) {
                text += '{' + (ATTR_NS[a.namespaceURI] || '?') + '}';
              }
              text += '="' + esc(a.value) + '"';
            }
            text += '>';
            if (namespaceURI === HTML && localName === 'template') {
              text += '{' + kids(node.content) + '}';
            }
            return text + kids(node) + '</>';
          }
          case 3:
            return '"' + esc(node.data) + '"';
          case 8:
            return '<!--"' + esc(node.data) + '"-->';
          case 7:
            // A processing instruction: target and data.
            return '<?"' + esc(node.target) + '" "' + esc(node.data) + '">';
          default:
            return '<#' + nodeTypeOf(node) + '>';
        }
      };
      result = job.inputs.map((input) =>
        attempt(() => dump(new DOMParser().parseFromString(input, 'text/html').documentElement))
      );
      break;
    }
    case 'sanitizeText': {
      result = job.cases.map(([text, config]) => attempt(() => m.main.sanitizeText(text, config)));
      break;
    }
    case 'sanitizeTextFresh': {
      // Each call runs on a new module instance: the state of a page whose
      // first sanitize call it is.
      result = [];
      for (const [text, config] of job.cases) {
        const fresh = await load();
        result.push(attempt(() => fresh.main.sanitizeText(text, config)));
      }
      break;
    }
    case 'fn': {
      const table = {
        ...Object.fromEntries(PRIVATE.map((n) => [n, m.main['__' + n]])),
        sanitizeText: m.main.sanitizeText,
        evaluate: m.main.evaluate,
        getEffectiveHtmlLabels: m.main.getEffectiveHtmlLabels,
        parseGenericTypes: m.main.parseGenericTypes,
        hasKatex: m.main.hasKatex,
        getUrl: m.main.getUrl,
        encodeEntities: m.utils.encodeEntities,
        decodeEntities: m.utils.decodeEntities,
        formatUrl: m.utils.utils_default.formatUrl,
        sanitizeUrl: m.sanitizeUrl,
        urlHref: (value) => new URL(value).href,
        getMax: (...values) => m.main.__getMax(...values.map(Number)),
        getMin: (...values) => m.main.__getMin(...values.map(Number)),
        lineBreakRegexSplit: (s) => s.split(m.main.lineBreakRegex),
        cssEscape: (s) => CSS.escape(s),
      };
      const fn = table[job.name];
      result = job.calls.map((call) =>
        attempt(() => {
          const r = fn(...call);
          // JSON has no undefined, NaN or Infinity.
          if (r === undefined) {
            return { value: 'undefined' };
          }
          if (typeof r === 'number' && !Number.isFinite(r)) {
            return { value: String(r) };
          }
          return r;
        })
      );
      break;
    }
    default:
      result = { throw: 'unknown job kind ' + job.kind };
  }
  out.results.push(result);
}
return out;
