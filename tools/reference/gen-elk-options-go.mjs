// Emits the Go declarations of the ELK layout options for
// internal/elk/native, from the option objects of elkjs 0.9.3 at run time:
//
//   internal/elk/native/core/enums_gen.go       the enums of
//                                               org.eclipse.elk.core.options,
//                                               and the id, type and legacy
//                                               ids of every option that
//                                               elkjs registers
//   internal/elk/native/coreopts/options_gen.go CoreOptions
//   internal/elk/native/lenum/enums_gen.go      the enums of
//                                               org.eclipse.elk.alg.layered.options
//   internal/elk/native/lopts/options_gen.go    LayeredOptions
//
// An enum type and an option can have the same name (the enum Direction and
// CoreOptions.DIRECTION), so the options have their own packages.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-options-go.mjs
//
// The Java sources of these classes are generated from Core.melk and
// Layered.melk and are not in the ELK repository, so the script reads the
// values from the compiled engine.
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadElk, WORKER_PATH } from './elk-phases-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const nativeDir = resolve(here, '../../internal/elk/native');

const elk = loadElk();
const S = elk.hook.scope;
const src = readFileSync(WORKER_PATH, 'utf8');
const getName = S('$getName');
const getClass = S('getClass__Ljava_lang_Class___devirtual$');
const className = (o) => {
  try {
    return getName(getClass(o));
  } catch (e) {
    return '?';
  }
};
const iterate = (coll) => {
  const out = [];
  for (const it = coll.iterator_0(); it.hasNext_0(); ) out.push(it.next_1());
  return out;
};

function body(fn) {
  const i = src.indexOf(`\nfunction ${fn}(){`);
  if (i < 0) throw new Error(`no function ${fn}`);
  return src.slice(i, src.indexOf('\n}\n', i));
}

// ---------------------------------------------------------------------------
// Names.

const camel = (s) =>
  s
    .toLowerCase()
    .split('_')
    .filter(Boolean)
    .map((w) => w[0].toUpperCase() + w.slice(1))
    .join('');
const simpleName = (cls) => cls.replace(/^.*\./, '').replace(/\$/g, '');
const isLayeredClass = (cls) => cls.startsWith('org.eclipse.elk.alg.layered.');

// ---------------------------------------------------------------------------
// Enums. enums maps the Java class name to { go, pkg, names }.

const enums = new Map();
function useEnum(classLit) {
  const cls = getName(classLit);
  if (!enums.has(cls)) {
    const constants = classLit.enumConstantsFunc();
    enums.set(cls, { cls, go: simpleName(cls), pkg: isLayeredClass(cls) ? 'lenum' : 'core', names: Array.from(constants, (c) => c.name_0) });
  }
  return enums.get(cls);
}
// A constant with a body has its own class, whose enumSuperclass is the enum.
const enumOf = (value) => {
  let c = getClass(value);
  if (!c.enumConstantsFunc && c.enumSuperclass && c.enumSuperclass.enumConstantsFunc) c = c.enumSuperclass;
  return useEnum(c);
};

// ---------------------------------------------------------------------------
// The option metadata of the layout meta data service.

const service = S('SERVICE');
const optionData = new Map();
for (const en of iterate(service.layoutOptionMap.entrySet_0())) {
  const d = en.getValue();
  optionData.set(d.id_0, d);
}

// goType returns { type, kind, enum } for an option id and a default value.
function goType(id, def, pkg) {
  const d = optionData.get(id);
  const q = (e) => (e.pkg === pkg ? e.go : `${e.pkg}.${e.go}`);
  const kind = d ? d.type_0.name_0 : undefined;
  switch (kind) {
    case 'BOOLEAN':
      return { type: 'bool', kind };
    case 'INT':
      return { type: 'int', kind };
    case 'DOUBLE':
      return { type: 'float64', kind };
    case 'STRING':
      return { type: 'string', kind };
    case 'ENUM': {
      const e = useEnum(d.clazz);
      return { type: q(e), kind, enum: e };
    }
    case 'ENUMSET': {
      const e = useEnum(d.clazz);
      return { type: `*prop.EnumSet[${q(e)}]`, kind, enum: e };
    }
  }
  const cls = d && d.clazz ? getName(d.clazz) : def !== null && def !== undefined && typeof def === 'object' ? className(def) : '';
  switch (cls) {
    case 'org.eclipse.elk.core.math.KVector':
      return { type: '*emath.KVector', kind: 'OBJECT', cls };
    case 'org.eclipse.elk.core.math.KVectorChain':
      return { type: '*emath.KVectorChain', kind: 'OBJECT', cls };
    case 'org.eclipse.elk.core.math.ElkPadding':
      return { type: '*emath.ElkPadding', kind: 'OBJECT', cls };
    case 'org.eclipse.elk.core.math.ElkMargin':
      return { type: '*emath.ElkMargin', kind: 'OBJECT', cls };
  }
  if (typeof def === 'number') return { type: 'float64', kind: 'DOUBLE' };
  if (typeof def === 'boolean') return { type: 'bool', kind: 'BOOLEAN' };
  if (typeof def === 'string') return { type: 'string', kind: 'STRING' };
  if (def && className(def) === 'java.lang.Integer') return { type: 'int', kind: 'INT' };
  if (def && typeof def.ordinal === 'number' && typeof def.name_0 === 'string') {
    const e = enumOf(def);
    return { type: q(e), kind: 'ENUM', enum: e };
  }
  return { type: 'any', kind: 'OBJECT', cls };
}

const num = (x) => {
  if (Object.is(x, -0)) return 'math.Copysign(0, -1)';
  if (x === Infinity) return 'math.Inf(1)';
  if (x === -Infinity) return 'math.Inf(-1)';
  if (Number.isNaN(x)) return 'math.NaN()';
  return String(x);
};

// goDefault returns the Go expression of a default value, or null when the
// option has no default.
function goDefault(def, t, pkg) {
  if (def === null || def === undefined) return null;
  const q = (e) => (e.pkg === pkg ? '' : `${e.pkg}.`);
  switch (t.kind) {
    case 'BOOLEAN':
      return String(def);
    case 'INT':
      return String(typeof def === 'number' ? def : def.value_0);
    case 'DOUBLE':
      return num(def);
    case 'STRING':
      return JSON.stringify(def);
    case 'ENUM':
      return `${q(t.enum)}${t.enum.go}${camel(def.name_0)}`;
    case 'ENUMSET':
      return `prop.Of[${q(t.enum)}${t.enum.go}](${iterate(def)
        .map((x) => `${q(t.enum)}${t.enum.go}${camel(x.name_0)}`)
        .join(', ')})`;
  }
  switch (t.type) {
    case '*emath.KVector':
      return `emath.NewKVector(${num(def.x_0)}, ${num(def.y_0)})`;
    case '*emath.KVectorChain':
      if (iterate(def).length !== 0) throw new Error('a KVectorChain default with points');
      return 'emath.NewKVectorChain()';
    case '*emath.ElkPadding':
      return `emath.NewElkPadding(${num(def.top_0)}, ${num(def.right)}, ${num(def.bottom)}, ${num(def.left)})`;
    case '*emath.ElkMargin':
      return `emath.NewElkMargin(${num(def.top_0)}, ${num(def.right)}, ${num(def.bottom)}, ${num(def.left)})`;
  }
  return undefined;
}

// properties lists the Property fields that a class initializer assigns.
function properties(clinit) {
  S(`${clinit}()`);
  const rows = [];
  const seen = new Set();
  for (const m of body(clinit).matchAll(/^\s+([A-Z][A-Z0-9_]*) = /gm)) {
    const jsName = m[1];
    let p;
    try {
      p = S(jsName);
    } catch (e) {
      continue;
    }
    if (!p || typeof p !== 'object' || !('id_0' in p) || typeof p.id_0 !== 'string') continue;
    const javaName = jsName.replace(/_\d+$/, '');
    if (seen.has(javaName)) throw new Error(`duplicate ${javaName}`);
    seen.add(javaName);
    rows.push({ javaName, id: p.id_0, def: p.defaultValue });
  }
  return rows;
}

function emitProperties(rows, pkg, javaClass) {
  const lines = [];
  lines.push(`// The fields of ${javaClass}. A field that has the id of a core option and`);
  lines.push('// another default value is a second Property object with the same id, as in');
  lines.push('// Java.');
  lines.push('var (');
  for (const r of rows) {
    const t = goType(r.id, r.def, pkg);
    const def = goDefault(r.def, t, pkg);
    const name = camel(r.javaName);
    lines.push(`\t// ${name} is ${javaClass}.${r.javaName}.`);
    if (def === null) {
      lines.push(`\t${name} = prop.New[${t.type}](${JSON.stringify(r.id)})`);
    } else if (def === undefined) {
      lines.push(`\t// The default value has the class ${className(r.def)}, which is not ported.`);
      lines.push(`\t${name} = prop.New[${t.type}](${JSON.stringify(r.id)})`);
    } else if (t.kind === 'ENUMSET') {
      // The EnumSet of GWT is not Cloneable: getProperty returns the default
      // set and does not store it.
      lines.push(`\t${name} = prop.NewFresh[${t.type}](${JSON.stringify(r.id)}, ${def})`);
    } else if (t.type.startsWith('*')) {
      lines.push(`\t${name} = prop.NewCloneable[${t.type}](${JSON.stringify(r.id)}, ${def})`);
    } else {
      lines.push(`\t${name} = prop.NewDefault[${t.type}](${JSON.stringify(r.id)}, ${def})`);
    }
  }
  lines.push(')');
  return lines.join('\n');
}

function emitEnums(pkg) {
  const lines = [];
  const list = [...enums.values()].filter((e) => e.pkg === pkg).sort((a, b) => (a.go < b.go ? -1 : 1));
  for (const e of list) {
    lines.push(`// ${e.go} is the enum ${e.cls}.`);
    lines.push(`type ${e.go} int`);
    lines.push('');
    lines.push(`// The constants of ${e.go}, in the order of the Java enum.`);
    lines.push('const (');
    e.names.forEach((n, i) => lines.push(`\t${e.go}${camel(n)}${i === 0 ? ` ${e.go} = iota` : ''}`));
    lines.push(')');
    lines.push('');
    lines.push(`var names${e.go} = []string{${e.names.map((n) => JSON.stringify(n)).join(', ')}}`);
    lines.push('');
    lines.push(`// String returns the name of the Java constant.`);
    lines.push(`func (e ${e.go}) String() string { return prop.EnumName(names${e.go}, int(e)) }`);
    lines.push('');
    lines.push(`// Values${e.go} returns the constants in the order of the Java enum.`);
    lines.push(`func Values${e.go}() []${e.go} {`);
    lines.push(`\tout := make([]${e.go}, ${e.names.length})`);
    lines.push('\tfor i := range out {');
    lines.push(`\t\tout[i] = ${e.go}(i)`);
    lines.push('\t}');
    lines.push('\treturn out');
    lines.push('}');
    lines.push('');
  }
  lines.push('func init() {');
  for (const e of list) lines.push(`\tprop.RegisterEnum[${e.go}](${JSON.stringify(e.cls)}, names${e.go})`);
  lines.push('}');
  return lines.join('\n');
}

// ---------------------------------------------------------------------------

// Every enum of the option packages, also the ones that no option uses (the
// internal properties use them).
for (const m of src.matchAll(/^var (Lorg_eclipse_elk_(?:core_options|alg_layered_options)_[A-Za-z0-9$]+_2_classLit) = createForEnum\(/gm)) {
  // A constant with a body has a class of its own without constants.
  const lit = S(m[1]);
  if (typeof lit.enumConstantsFunc === 'function') useEnum(lit);
}
for (const name of ['Lorg_eclipse_elk_core_options_LabelSide_2_classLit', 'Lorg_eclipse_elk_alg_layered_components_ComponentOrderingStrategy_2_classLit']) {
  try {
    useEnum(S(name));
  } catch (e) {
    // The class is not in this build.
  }
}

const coreRows = properties('$clinit_CoreOptions');
const layeredRows = properties('$clinit_LayeredOptions');
const coreProps = emitProperties(coreRows, 'coreopts', 'org.eclipse.elk.core.options.CoreOptions');
const layeredProps = emitProperties(layeredRows, 'lopts', 'org.eclipse.elk.alg.layered.options.LayeredOptions');

// The metadata of every registered option.
const meta = [];
for (const id of [...optionData.keys()].sort()) {
  const d = optionData.get(id);
  const kind = d.type_0.name_0;
  let cls = '';
  if ((kind === 'ENUM' || kind === 'ENUMSET' || kind === 'OBJECT') && d.clazz) cls = getName(d.clazz);
  const targets = iterate(d.targets).map((t) => t.name_0);
  meta.push({ id, kind, cls, legacy: d.legacyIds ? Array.from(d.legacyIds) : [], targets });
}

const header = (pkg, what) => `// Code generated by tools/reference/gen-elk-options-go.mjs from elkjs 0.9.3. DO NOT EDIT.
//
// Ported from ${what} of Eclipse ELK 0.9.1 (EPL-2.0). The values are read from
// the compiled engine, because the Java classes are generated from .melk
// files.
`;
const imp = (...pkgs) => `import (\n${pkgs.map((p) => `\t"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/${p}"`).join('\n')}\n)\n`;

const coreOut = `${header('core', 'the enums of org.eclipse.elk.core.options')}
// Package core has the enums of org.eclipse.elk.core.options and the list of
// the layout options that elkjs registers.
package core

${imp('prop')}
${emitEnums('core')}

// OptionMeta describes one layout option that elkjs registers.
type OptionMeta struct {
	// ID is the full id of the option.
	ID string
	// Kind is the name of LayoutOptionData.Type: BOOLEAN, INT, STRING, DOUBLE,
	// ENUM, ENUMSET or OBJECT.
	Kind string
	// Class is the Java class of an enum or of an object value.
	Class string
	// Legacy has the ids that the option had in earlier versions.
	Legacy []string
	// Targets has the names of LayoutOptionData.Target.
	Targets []string
}

// Options has every layout option of the algorithms that elkjs registers, in
// the order of the ids.
var Options = []OptionMeta{
${meta.map((m) => `\t{ID: ${JSON.stringify(m.id)}, Kind: ${JSON.stringify(m.kind)}, Class: ${JSON.stringify(m.cls)}, Legacy: ${m.legacy.length ? `[]string{${m.legacy.map((x) => JSON.stringify(x)).join(', ')}}` : 'nil'}, Targets: []string{${m.targets.map((x) => JSON.stringify(x)).join(', ')}}},`).join('\n')}
}
`;

const lenumOut = `${header('lenum', 'the enums of org.eclipse.elk.alg.layered.options')}
// Package lenum has the enums of org.eclipse.elk.alg.layered.options.
package lenum

${imp('prop')}
${emitEnums('lenum')}
`;

const needs = (text, pkg) => text.includes(`${pkg}.`);
const optsOut = (pkg, what, doc, propsText) => {
  const pkgs = ['core', 'emath', 'lenum', 'prop'].filter((p) => needs(propsText, p));
  return `${header(pkg, what)}
// Package ${pkg} has ${doc}.
package ${pkg}

${imp(...pkgs)}
${propsText}
`;
};

mkdirSync(resolve(nativeDir, 'core'), { recursive: true });
mkdirSync(resolve(nativeDir, 'coreopts'), { recursive: true });
mkdirSync(resolve(nativeDir, 'lenum'), { recursive: true });
mkdirSync(resolve(nativeDir, 'lopts'), { recursive: true });
writeFileSync(resolve(nativeDir, 'core/enums_gen.go'), coreOut);
writeFileSync(resolve(nativeDir, 'lenum/enums_gen.go'), lenumOut);
writeFileSync(resolve(nativeDir, 'coreopts/options_gen.go'), optsOut('coreopts', 'org.eclipse.elk.core.options.CoreOptions', 'the layout options of org.eclipse.elk.core.options.CoreOptions', coreProps));
writeFileSync(resolve(nativeDir, 'lopts/options_gen.go'), optsOut('lopts', 'org.eclipse.elk.alg.layered.options.LayeredOptions', 'the layout options of org.eclipse.elk.alg.layered.options.LayeredOptions, with the default values of the layered algorithm', layeredProps));
console.log(`core: ${coreRows.length} properties, ${[...enums.values()].filter((e) => e.pkg === 'core').length} enums, ${meta.length} registered options`);
console.log(`lopts: ${layeredRows.length} properties, ${[...enums.values()].filter((e) => e.pkg === 'lenum').length} enums`);
