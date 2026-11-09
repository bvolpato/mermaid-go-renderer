// Translates mermaid's theme classes (packages/mermaid/src/themes/theme-*.js)
// to Go source in internal/theme. The themes are long lists of assignments in
// a small subset of JavaScript, so a mechanical translation is the most
// faithful port. The script fails on any syntax that it does not know.
//
// Usage (from tools/reference):
//   node gen-theme-go.mjs && gofmt -w ../../internal/theme
//
// The parser is the Babel bundle that ships inside playwright. It is used at
// generation time only.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';

const require = createRequire(import.meta.url);
const { babelParse } = require('./node_modules/playwright/lib/transform/babelBundle.js');

const SRC = '../../.upstream/mermaid-12.2.0/packages/mermaid/src/themes';
const OUT = '../../internal/theme';

// name in themes/index.js -> source file and Go identifier suffix.
const THEMES = [
  ['base', 'theme-base.js', 'Base'],
  ['dark', 'theme-dark.js', 'Dark'],
  ['default', 'theme-default.js', 'Default'],
  ['forest', 'theme-forest.js', 'Forest'],
  ['neutral', 'theme-neutral.js', 'Neutral'],
  ['neo', 'theme-neo.js', 'Neo'],
  ['neo-dark', 'theme-neo-dark.js', 'NeoDark'],
  ['redux', 'theme-redux.js', 'Redux'],
  ['redux-dark', 'theme-redux-dark.js', 'ReduxDark'],
  ['redux-color', 'theme-redux-color.js', 'ReduxColor'],
  ['redux-dark-color', 'theme-redux-dark-color.js', 'ReduxDarkColor'],
];

const KHROMA = new Set(['adjust', 'darken', 'lighten', 'invert', 'isDark', 'rgba']);
const IMPORTED_CONSTS = {
  oldAttributeBackgroundColorOdd: { go: 'oldAttributeBackgroundColorOdd', t: 'str' },
  oldAttributeBackgroundColorEven: { go: 'oldAttributeBackgroundColorEven', t: 'str' },
};
const GO_KEYWORDS = new Set(['type', 'func', 'var', 'range', 'map', 'chan', 'go', 'select', 'package', 'import', 'defer', 'fallthrough', 'interface', 'struct', 'len', 'string', 'any']);

class Translator {
  constructor(source, file, suffix) {
    this.source = source;
    this.file = file;
    this.suffix = suffix;
    this.scopes = [{ ...IMPORTED_CONSTS }];
    this.lines = [];
    this.indent = 1;
    this.consumed = new Set();
  }

  fail(node, msg) {
    const loc = node.loc ? `${this.file}:${node.loc.start.line}` : this.file;
    throw new Error(`${loc}: ${msg} (${node.type})`);
  }

  emit(line) {
    this.lines.push('\t'.repeat(this.indent) + line);
  }

  lookup(name) {
    for (let i = this.scopes.length - 1; i >= 0; i--) {
      if (Object.hasOwn(this.scopes[i], name)) return this.scopes[i][name];
    }
    return undefined;
  }

  declare(name, t) {
    const go = GO_KEYWORDS.has(name) ? name + '_' : name;
    this.scopes[this.scopes.length - 1][name] = { go, t, uses: 0 };
    return go;
  }

  commentText(c) {
    return c.value
      .split('\n')
      .map((line) => line.replace(/^\s*\*?\s?/, '').replace(/\s+$/, '').replaceAll('\u2014', '--'));
  }

  // sameLineComment returns the text of a comment that follows a node on its
  // last line, or ''.
  sameLineComment(node, next) {
    const candidates = [...(node.trailingComments ?? []), ...(next?.leadingComments ?? [])];
    const c = candidates.find((x) => x.loc.start.line === node.loc.end.line);
    if (!c) return '';
    this.consumed.add(c.start);
    return ' // ' + this.commentText(c).join(' ').trim();
  }

  comments(node) {
    for (const c of node.leadingComments ?? []) {
      if (this.consumed.has(c.start)) continue;
      this.consumed.add(c.start);
      for (const line of this.commentText(c)) this.emit(line ? '// ' + line : '//');
    }
  }

  num(v) {
    if (Number.isInteger(v)) return v + '.0';
    return String(v);
  }

  // key returns a Go string expression for a property key.
  key(node, computed) {
    if (!computed) {
      if (node.type !== 'Identifier') this.fail(node, 'unexpected property key');
      return { go: JSON.stringify(node.name), simple: true };
    }
    const k = this.ex(node);
    if (k.t === 'str') return { go: k.go, simple: k.simple };
    if (k.t === 'num') return { go: `js.Str(${k.go})`, simple: k.simple };
    return { go: `str(${k.go})`, simple: k.simple };
  }

  // asNum returns a Go float64 expression.
  asNum(e) {
    return e.t === 'num' ? e.go : `num(${e.go})`;
  }

  // ex translates an expression in value context.
  // The result has the Go code, a static type (num, str, bool, arr or any),
  // and whether evaluation is free of side effects and cannot throw.
  ex(node) {
    switch (node.type) {
      case 'ParenthesizedExpression':
        return this.ex(node.expression);
      case 'NumericLiteral':
        return { go: this.num(node.value), t: 'num', simple: true };
      case 'StringLiteral':
        return { go: JSON.stringify(node.value), t: 'str', simple: true };
      case 'BooleanLiteral':
        return { go: String(node.value), t: 'bool', simple: true };
      case 'Identifier': {
        if (node.name === 'undefined') return { go: 'nil', t: 'any', simple: true };
        const b = this.lookup(node.name);
        if (!b) this.fail(node, `unknown identifier ${node.name}`);
        b.uses = (b.uses ?? 0) + 1;
        return { go: b.go, t: b.t, simple: true };
      }
      case 'MemberExpression':
      case 'OptionalMemberExpression': {
        const k = this.key(node.property, node.computed);
        if (node.object.type === 'ThisExpression') {
          return { go: `t.get(${k.go})`, t: 'any', simple: k.simple };
        }
        const o = this.ex(node.object);
        if (node.optional) {
          return { go: `optMember(${o.go}, ${k.go})`, t: 'any', simple: o.simple && k.simple };
        }
        return { go: `member(${o.go}, ${k.go})`, t: 'any', simple: false };
      }
      case 'UnaryExpression': {
        const a = this.ex(node.argument);
        if (node.operator === '-') {
          if (a.t === 'num') return { go: `-${a.go}`, t: 'num', simple: a.simple };
          return { go: `-num(${a.go})`, t: 'num', simple: a.simple };
        }
        if (node.operator === '+') return { go: this.asNum(a), t: 'num', simple: a.simple };
        if (node.operator === '!') return { go: `!${this.cnd(node.argument)}`, t: 'bool', simple: a.simple };
        return this.fail(node, `unary ${node.operator}`);
      }
      case 'BinaryExpression':
        return this.binary(node);
      case 'LogicalExpression': {
        const a = this.ex(node.left);
        const b = this.ex(node.right);
        const fn = { '||': 'or', '&&': 'and', '??': 'nullish' }[node.operator];
        if (!fn) this.fail(node, `logical ${node.operator}`);
        if (b.simple) return { go: `${fn}(${a.go}, ${b.go})`, t: 'any', simple: a.simple };
        return { go: `${fn}f(${a.go}, func() any { return ${b.go} })`, t: 'any', simple: false };
      }
      case 'ConditionalExpression': {
        const test = this.cnd(node.test);
        const a = this.ex(node.consequent);
        const b = this.ex(node.alternate);
        const simple = a.simple && b.simple;
        if (simple && a.t === 'num' && b.t === 'num') return { go: `condNum(${test}, ${a.go}, ${b.go})`, t: 'num', simple: false };
        if (simple) return { go: `cond(${test}, ${a.go}, ${b.go})`, t: 'any', simple: false };
        return {
          go: `condf(${test}, func() any { return ${a.go} }, func() any { return ${b.go} })`,
          t: 'any',
          simple: false,
        };
      }
      case 'CallExpression':
        return this.call(node);
      case 'ObjectExpression': {
        const parts = [];
        for (const p of node.properties) {
          if (p.type !== 'ObjectProperty' || p.computed) this.fail(p, 'unsupported object member');
          const name = p.key.type === 'Identifier' ? p.key.name : p.key.value;
          parts.push(`${JSON.stringify(String(name))}, ${this.ex(p.value).go}`);
        }
        return { go: `obj(\n${parts.join(',\n')},\n)`, t: 'any', simple: false };
      }
      case 'ArrayExpression': {
        if (node.elements.length === 0) return { go: '[]any{}', t: 'arr', simple: false };
        const parts = node.elements.map((e, i) => `${this.ex(e).go},${this.sameLineComment(e, node.elements[i + 1])}`);
        return { go: `[]any{\n${parts.join('\n')}\n}`, t: 'arr', simple: false };
      }
    }
    return this.fail(node, 'unsupported expression');
  }

  binary(node) {
    const a = this.ex(node.left);
    const b = this.ex(node.right);
    const simple = a.simple && b.simple;
    const bothNum = a.t === 'num' && b.t === 'num';
    switch (node.operator) {
      case '+':
        if (bothNum) return { go: `(${a.go} + ${b.go})`, t: 'num', simple };
        if (a.t === 'str' && b.t === 'str') return { go: `${a.go} + ${b.go}`, t: 'str', simple };
        if (a.t === 'str' && b.t === 'num') return { go: `${a.go} + js.Str(${b.go})`, t: 'str', simple };
        if (a.t === 'num' && b.t === 'str') return { go: `js.Str(${a.go}) + ${b.go}`, t: 'str', simple };
        return { go: `add(${a.go}, ${b.go})`, t: 'any', simple };
      case '-':
        return { go: `(${this.asNum(a)} - ${this.asNum(b)})`, t: 'num', simple };
      case '*':
        // The conversion stops the compiler from fusing the product with a
        // later addition into one FMA instruction.
        return { go: `float64(${this.asNum(a)} * ${this.asNum(b)})`, t: 'num', simple };
      case '/':
        return { go: `(${this.asNum(a)} / ${this.asNum(b)})`, t: 'num', simple };
      case '%':
        return { go: `math.Mod(${this.asNum(a)}, ${this.asNum(b)})`, t: 'num', simple };
      case '<':
      case '>':
      case '<=':
      case '>=':
        if (bothNum) return { go: `(${a.go} ${node.operator} ${b.go})`, t: 'bool', simple };
        return { go: `compare(${a.go}, ${JSON.stringify(node.operator)}, ${b.go})`, t: 'bool', simple };
      case '===':
        return { go: `strictEquals(${a.go}, ${b.go})`, t: 'bool', simple };
      case '!==':
        return { go: `!strictEquals(${a.go}, ${b.go})`, t: 'bool', simple };
    }
    return this.fail(node, `binary ${node.operator}`);
  }

  call(node) {
    if (
      node.callee.type === 'MemberExpression' &&
      node.callee.object.type === 'ThisExpression' &&
      node.callee.property.name === 'updateColors' &&
      node.arguments.length === 0
    ) {
      return { go: `updateColors${this.suffix}(t)`, t: 'void', simple: false };
    }
    if (node.callee.type !== 'Identifier') this.fail(node, 'unsupported callee');
    const name = node.callee.name;
    const args = node.arguments;
    if (name === 'mkBorder' && args.length === 2) {
      return { go: `mkBorder(${this.ex(args[0]).go}, ${this.ex(args[1]).go})`, t: 'any', simple: false };
    }
    if (!KHROMA.has(name)) this.fail(node, `unknown function ${name}`);
    switch (name) {
      case 'adjust': {
        if (args.length !== 2 || args[1].type !== 'ObjectExpression') this.fail(node, 'adjust shape');
        const chans = args[1].properties.map((p) => {
          if (p.type !== 'ObjectProperty' || p.computed || p.key.type !== 'Identifier' || !/^[rgbhsla]$/.test(p.key.name)) {
            this.fail(p, 'adjust channel');
          }
          return `khroma.${p.key.name.toUpperCase()}(${this.asNum(this.ex(p.value))})`;
        });
        return { go: `adjust(${this.ex(args[0]).go}, ${chans.join(', ')})`, t: 'any', simple: false };
      }
      case 'darken':
      case 'lighten': {
        if (args.length < 1 || args.length > 2) this.fail(node, `${name} shape`);
        // An omitted amount is undefined, which is NaN in arithmetic.
        const amount = args.length === 2 ? this.asNum(this.ex(args[1])) : 'math.NaN()';
        return { go: `${name}(${this.ex(args[0]).go}, ${amount})`, t: 'any', simple: false };
      }
      case 'invert':
        if (args.length !== 1) this.fail(node, 'invert shape');
        return { go: `invert(${this.ex(args[0]).go})`, t: 'any', simple: false };
      case 'isDark':
        if (args.length !== 1) this.fail(node, 'isDark shape');
        return { go: `isDark(${this.ex(args[0]).go})`, t: 'bool', simple: false };
      case 'rgba': {
        if (args.length !== 4) this.fail(node, 'rgba shape');
        const nums = args.map((a) => this.ex(a));
        if (nums.some((n) => n.t !== 'num')) this.fail(node, 'rgba with a value that is not a number');
        return { go: `khroma.RGBA(${nums.map((n) => n.go).join(', ')})`, t: 'str', simple: false };
      }
    }
    return this.fail(node, 'unreachable');
  }

  // cnd translates an expression in boolean context (ToBoolean).
  cnd(node) {
    if (node.type === 'ParenthesizedExpression') return this.cnd(node.expression);
    if (node.type === 'LogicalExpression' && (node.operator === '||' || node.operator === '&&')) {
      return `(${this.cnd(node.left)} ${node.operator} ${this.cnd(node.right)})`;
    }
    if (node.type === 'UnaryExpression' && node.operator === '!') return `!${this.cnd(node.argument)}`;
    const e = this.ex(node);
    switch (e.t) {
      case 'bool':
        return e.go;
      case 'num':
        return `truthyNum(${e.go})`;
      case 'str':
        return `(${e.go} != "")`;
      case 'arr':
        return 'true';
    }
    return `truthy(${e.go})`;
  }

  assignTarget(node) {
    if (
      (node.type !== 'MemberExpression' && node.type !== 'OptionalMemberExpression') ||
      node.object.type !== 'ThisExpression'
    ) {
      this.fail(node, 'unsupported assignment target');
    }
    return this.key(node.property, node.computed).go;
  }

  // blank emits a placeholder after the declaration of a local variable. Go
  // rejects an unused local variable and JavaScript does not, so block()
  // turns the placeholder into `_ = name` when nothing reads the variable.
  blank(binding) {
    binding.marker = `\u0000blank:${binding.go}`;
    this.emit(binding.marker);
  }

  block(body, bindings = []) {
    this.scopes.push({});
    const start = this.lines.length;
    for (const b of bindings) this.blank(b);
    for (const s of body) this.stmt(s);
    const scope = this.scopes.pop();
    const all = [...bindings, ...Object.values(scope)];
    for (let i = this.lines.length - 1; i >= start; i--) {
      const line = this.lines[i].trim();
      const b = all.find((x) => x.marker === line);
      if (!b) continue;
      if (b.uses) this.lines.splice(i, 1);
      else this.lines[i] = this.lines[i].replace(line, `_ = ${b.go}`);
    }
  }

  stmt(node) {
    this.comments(node);
    const before = this.lines.length;
    this.stmtBody(node);
    // A comment at the end of a one-line statement stays on that line.
    if (this.lines.length === before + 1 && !this.lines[before].includes('\n')) {
      this.lines[before] += this.sameLineComment(node);
    }
  }

  stmtBody(node) {
    switch (node.type) {
      case 'ExpressionStatement': {
        const e = node.expression;
        if (e.type === 'AssignmentExpression' && e.operator === '=') {
          const key = this.assignTarget(e.left);
          this.emit(`t.set(${key}, ${this.ex(e.right).go})`);
          return;
        }
        if (e.type === 'CallExpression') {
          // journeyFills.forEach((fill, i) => { ... })
          if (
            e.callee.type === 'MemberExpression' &&
            e.callee.property.name === 'forEach' &&
            e.callee.object.type === 'Identifier' &&
            e.arguments.length === 1 &&
            e.arguments[0].type === 'ArrowFunctionExpression'
          ) {
            const arr = this.ex(e.callee.object);
            if (arr.t !== 'arr') this.fail(e, 'forEach on a value that is not a local array');
            const fn = e.arguments[0];
            if (fn.params.length !== 2 || fn.body.type !== 'BlockStatement') this.fail(e, 'forEach callback shape');
            this.scopes.push({});
            const item = this.declare(fn.params[0].name, 'any');
            const idx = this.declare(fn.params[1].name, 'num');
            this.emit(`for ${idx}Index, ${item} := range ${arr.go} {`);
            this.indent++;
            this.emit(`${idx} := float64(${idx}Index)`);
            this.block(fn.body.body, [this.lookup(fn.params[0].name), this.lookup(fn.params[1].name)]);
            this.indent--;
            this.emit('}');
            this.scopes.pop();
            return;
          }
          const c = this.call(e);
          if (c.t !== 'void') this.fail(e, 'call statement with a discarded result');
          this.emit(c.go);
          return;
        }
        return this.fail(e, 'unsupported expression statement');
      }
      case 'VariableDeclaration': {
        if (node.kind !== 'const' || node.declarations.length !== 1) this.fail(node, 'unsupported declaration');
        const d = node.declarations[0];
        if (d.id.type !== 'Identifier') this.fail(d, 'unsupported declaration pattern');
        const init = this.ex(d.init);
        const go = this.declare(d.id.name, init.t);
        // A string or a number keeps its static Go type. Other values are dynamic.
        const typed = init.t === 'any' ? `var ${go} any = ${init.go}` : `${go} := ${init.go}`;
        this.emit(typed);
        this.blank(this.lookup(d.id.name));
        return;
      }
      case 'ForStatement': {
        const { init, test, update } = node;
        if (
          init?.type !== 'VariableDeclaration' ||
          init.kind !== 'let' ||
          init.declarations.length !== 1 ||
          init.declarations[0].init?.type !== 'NumericLiteral' ||
          update?.type !== 'UpdateExpression' ||
          update.operator !== '++' ||
          update.argument.name !== init.declarations[0].id.name ||
          node.body.type !== 'BlockStatement'
        ) {
          this.fail(node, 'unsupported for loop');
        }
        this.scopes.push({});
        const name = this.declare(init.declarations[0].id.name, 'num');
        const staticBound =
          test.type === 'BinaryExpression' && test.left.type === 'Identifier' && test.right.type === 'NumericLiteral';
        this.emit(`for ${name} := ${this.num(init.declarations[0].init.value)}; ${this.cnd(test)}; ${name}++ {`);
        this.indent++;
        // The bound is a theme variable that a user can set. See themeVars.guard.
        if (!staticBound) this.emit('t.guard()');
        this.block(node.body.body);
        this.indent--;
        this.emit('}');
        this.scopes.pop();
        return;
      }
      case 'IfStatement': {
        this.emit(`if ${this.cnd(node.test)} {`);
        this.indent++;
        this.block(node.consequent.type === 'BlockStatement' ? node.consequent.body : [node.consequent]);
        this.indent--;
        if (node.alternate) {
          this.emit('} else {');
          this.indent++;
          this.block(node.alternate.type === 'BlockStatement' ? node.alternate.body : [node.alternate]);
          this.indent--;
        }
        this.emit('}');
        return;
      }
      case 'EmptyStatement':
        return;
    }
    return this.fail(node, 'unsupported statement');
  }

  method(node) {
    this.lines = [];
    this.indent = 1;
    this.block(node.body.body);
    return this.lines.join('\n');
  }
}

const normalize = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '').replace(/\s+/g, '');

const PLAIN_CALCULATE = normalize(`calculate(overrides) {
    if (typeof overrides !== 'object') {
      this.updateColors();
      return;
    }
    const keys = Object.keys(overrides);
    keys.forEach((k) => {
      this[k] = overrides[k];
    });
    this.updateColors();
    keys.forEach((k) => {
      this[k] = applyOverride(this[k], overrides[k]);
    });
  }`);
const BASE_CALCULATE = PLAIN_CALCULATE.slice(0, -1) + normalize(`
    if (Object.hasOwn(overrides, 'nodeBorder') && !Object.hasOwn(overrides, 'useGradient')) {
      this.useGradient = false;
    }
  }`);
const DEFAULT_CALCULATE =
  normalize(`calculate(overrides) {
    Object.keys(this).forEach((k) => {
      if (this[k] === 'calculated') {
        this[k] = undefined;
      }
    });`) + PLAIN_CALCULATE.slice('calculate(overrides){'.length);
const GET_THEME_VARIABLES = normalize(`export const getThemeVariables = (userOverrides) => {
  const theme = new Theme();
  theme.calculate(userOverrides);
  return theme;
};`);

const registry = [];
for (const [name, file, suffix] of THEMES) {
  const source = fs.readFileSync(path.join(SRC, file), 'utf8');
  const ast = babelParse(source, file, true);
  const cls = ast.program.body.find((n) => n.type === 'ClassDeclaration' && n.id.name === 'Theme');
  if (!cls) throw new Error(`${file}: class Theme not found`);
  const methods = Object.fromEntries(cls.body.body.map((m) => [m.key.name, m]));
  for (const m of Object.keys(methods)) {
    if (!['constructor', 'updateColors', 'calculate'].includes(m)) throw new Error(`${file}: unexpected method ${m}`);
  }
  const calcSrc = normalize(source.slice(methods.calculate.start, methods.calculate.end));
  const variant =
    calcSrc === PLAIN_CALCULATE ? 'calculate' : calcSrc === BASE_CALCULATE ? 'calculateBase' : calcSrc === DEFAULT_CALCULATE ? 'calculateDefault' : null;
  if (!variant) throw new Error(`${file}: the calculate method is not one of the known variants; update internal/theme/theme.go`);
  const exp = ast.program.body.find((n) => n.type === 'ExportNamedDeclaration');
  if (normalize(source.slice(exp.start, exp.end)) !== GET_THEME_VARIABLES) {
    throw new Error(`${file}: getThemeVariables has an unexpected shape`);
  }
  const imports = ast.program.body.filter((n) => n.type === 'ImportDeclaration');
  for (const imp of imports) {
    for (const spec of imp.specifiers) {
      const n = spec.local.name;
      if (!KHROMA.has(n) && !IMPORTED_CONSTS[n] && n !== 'applyOverride' && n !== 'mkBorder') {
        throw new Error(`${file}: unknown import ${n}`);
      }
    }
  }

  const tr = new Translator(source, file, suffix);
  const ctor = tr.method(methods.constructor);
  const update = tr.method(methods.updateColors);
  const body = `${ctor}\n${update}`;
  const needs = (pkg, re) => (re.test(body) ? `\t"${pkg}"\n` : '');
  const go = `// Ported from packages/mermaid/src/themes/${file} (mermaid 12.2.0).
// Code generated by tools/reference/gen-theme-go.mjs; DO NOT EDIT.

package theme

import (
${needs('math', /\bmath\./)}
${needs('github.com/bvolpato/mermaid-go-renderer/internal/js', /\bjs\./)}${needs('github.com/bvolpato/mermaid-go-renderer/internal/khroma', /\bkhroma\./)})

// new${suffix} is the constructor of class Theme in ${file}.
func new${suffix}() *themeVars {
	t := newThemeVars()
${ctor}
	return t
}

// updateColors${suffix} is Theme.updateColors in ${file}.
func updateColors${suffix}(t *themeVars) {
${update}
}
`;
  fs.writeFileSync(path.join(OUT, `theme_${name.replaceAll('-', '_')}.go`), go);
  registry.push({ name, suffix, variant });
}

// The registry keeps the order of themes/index.js.
const reg = `// Ported from packages/mermaid/src/themes/index.js (mermaid 12.2.0).
// Code generated by tools/reference/gen-theme-go.mjs; DO NOT EDIT.

package theme

// themeNames lists the themes in the order of the object in themes/index.js.
var themeNames = []string{
${registry.map((r) => `\t${JSON.stringify(r.name)},`).join('\n')}
}

// themes is the default export of themes/index.js.
var themes = map[string]themeClass{
${registry.map((r) => `\t${JSON.stringify(r.name)}: {construct: new${r.suffix}, updateColors: updateColors${r.suffix}, calculate: ${r.variant}},`).join('\n')}
}
`;
fs.writeFileSync(path.join(OUT, 'index.go'), reg);
console.error(`wrote ${registry.length} themes: ${registry.map((r) => `${r.name}(${r.variant})`).join(', ')}`);
