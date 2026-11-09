// Generates the Go grammar packages under internal/grammar from the *.jison
// grammars of mermaid 12.2.0.
//
// For each grammar it builds the parser with jison as the mermaid build does,
// checks that the tables are the tables in the mermaid dist, and writes:
//   tables.go            the LR tables, the lexer rules and the conditions
//   lexactions_gen.go    the lexer actions that have a mechanical translation
//   lexactions_manual.go stubs for the other lexer actions (written only when
//                        the file does not exist, then maintained by hand)
//   actions.js.txt       the JavaScript of every parser and lexer action
//
// Usage (in tools/reference): node gen-jison.mjs [grammar ...]
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import {
  analyzeRule,
  distParser,
  goString,
  grammarTables,
  jisonVersion,
  lexerRules,
  loadGrammars,
  parserActions,
  repoRoot,
  sameTables,
} from './jison-lib.mjs';

const outRoot = path.join(repoRoot, 'internal/grammar');
const modulePath = 'github.com/bvolpato/mermaid-go-renderer';
const only = process.argv.slice(2);

const isIdent = (s) => /^[A-Za-z_][A-Za-z0-9_]*$/.test(s);
// ---------------------------------------------------------------------------
// Lexer action translation

// stripComments removes JavaScript comments outside of string literals.
function stripComments(js) {
  let out = '';
  for (let i = 0; i < js.length; i++) {
    const c = js[i];
    if (c === "'" || c === '"') {
      let j = i + 1;
      while (j < js.length && js[j] !== c) {
        j += js[j] === '\\' ? 2 : 1;
      }
      out += js.slice(i, j + 1);
      i = j;
    } else if (c === '/' && js[i + 1] === '*') {
      const j = js.indexOf('*/', i + 2);
      i = j < 0 ? js.length : j + 1;
      out += ' ';
    } else if (c === '/' && js[i + 1] === '/') {
      const j = js.indexOf('\n', i);
      i = j < 0 ? js.length : j - 1;
    } else {
      out += c;
    }
  }
  return out;
}

// splitStatements splits at each ";" that is outside of strings and brackets.
// It returns undefined when the code has a block.
function splitStatements(js) {
  const out = [];
  let depth = 0;
  let cur = '';
  for (let i = 0; i < js.length; i++) {
    const c = js[i];
    if (c === "'" || c === '"') {
      let j = i + 1;
      while (j < js.length && js[j] !== c) {
        j += js[j] === '\\' ? 2 : 1;
      }
      cur += js.slice(i, j + 1);
      i = j;
    } else if (c === '{' || c === '}') {
      return undefined;
    } else if (c === '(' || c === '[') {
      depth++;
      cur += c;
    } else if (c === ')' || c === ']') {
      depth--;
      cur += c;
    } else if (c === ';' && depth === 0) {
      out.push(cur.trim());
      cur = '';
    } else {
      cur += c;
    }
  }
  out.push(cur.trim());
  return out.filter((s) => s !== '');
}

const stringLit = `(?:'([^'\\\\]*)'|"([^"\\\\]*)")`;

// translateExpr translates the right side of "yytext = ...".
function translateExpr(expr) {
  let m = new RegExp(`^${stringLit}`).exec(expr);
  let go;
  let rest;
  if (m) {
    go = goString(m[1] ?? m[2]);
    rest = expr.slice(m[0].length);
  } else if (expr.startsWith('yytext')) {
    go = 'lx.Yytext';
    rest = expr.slice('yytext'.length);
  } else {
    return undefined;
  }
  while (rest !== '') {
    if ((m = /^\.trim\(\)/.exec(rest))) {
      go = `js.TrimSpace(${go})`;
    } else if ((m = /^\.slice\(\s*(-?\d+)\s*,\s*(-?\d+)\s*\)/.exec(rest))) {
      go = `js.StringSlice(${go}, ${m[1]}, ${m[2]})`;
    } else if ((m = /^\.(?:slice|substr)\(\s*(-?\d+)\s*\)/.exec(rest))) {
      go = `js.StringSliceFrom(${go}, ${m[1]})`;
    } else {
      return undefined;
    }
    rest = rest.slice(m[0].length);
  }
  return go;
}

// translateLexAction returns the Go statements for a lexer action, or
// undefined when the action needs a manual port.
function translateLexAction(action, terminals) {
  let js = stripComments(action).trim();
  if (js.startsWith('{') && js.endsWith('}')) {
    js = js.slice(1, -1);
  }
  const statements = splitStatements(js);
  if (!statements) {
    return undefined;
  }
  const lines = [];
  let returns = false;
  let usesJS = false;
  for (const st of statements) {
    let m;
    if ((m = new RegExp(`^this\\.(?:begin|pushState)\\(\\s*${stringLit}\\s*\\)$`).exec(st))) {
      lines.push(`lx.Begin(${goString(m[1] ?? m[2])})`);
    } else if (new RegExp(`^this\\.popState\\(\\s*(?:${stringLit})?\\s*\\)$`).test(st)) {
      lines.push('lx.PopState()');
    } else if ((m = new RegExp(`^return\\s*${stringLit}$`).exec(st))) {
      const name = m[1] ?? m[2];
      if (name === '') {
        return undefined;
      }
      if (name in terminals) {
        lines.push(`return ${terminals[name]}, true`);
      } else {
        lines.push(`return lx.TokenNamed(${goString(name)})`);
      }
      returns = true;
      break; // The statements after a return do not run.
    } else if ((m = /^yytext\s*=\s*(.+)$/.exec(st))) {
      const go = translateExpr(m[1].trim());
      if (!go) {
        return undefined;
      }
      usesJS ||= go.includes('js.');
      lines.push(`lx.Yytext = ${go}`);
    } else if (
      (m = /^yy\.getLogger\(\)\.(?:trace|debug|info|warn|error|fatal)\((.*)\)$/s.exec(st)) &&
      /^(?:\s|,|\+|S|yytext)*$/.test(m[1].replace(new RegExp(stringLit, 'g'), 'S'))
    ) {
      // A log call has no effect on the lexer: its arguments are string
      // literals and yytext.
    } else if ((m = /^this\.less\(\s*(\d+)\s*\)$/.exec(st))) {
      lines.push(`lx.Less(${m[1]})`);
    } else if (/^this\.unput\(\s*yytext\s*\)$/.test(st)) {
      lines.push('lx.Unput(lx.Yytext)');
    } else {
      return undefined;
    }
  }
  if (!returns) {
    lines.push('return 0, false');
  }
  return { lines, usesJS };
}

const oneLine = (jsText) => jsText.replace(/\s+/g, ' ').trim();

// ---------------------------------------------------------------------------
// Files

function tablesFile(grammar, rules) {
  const { parser, name, rel } = grammar;
  const tables = grammarTables(
    grammar,
    rules.map((r) => r.info)
  );
  const out = [];
  out.push(
    `// Code generated by tools/reference/gen-jison.mjs from ${rel} (mermaid 12.2.0, jison ${jisonVersion}). DO NOT EDIT.`,
    '',
    `// Package ${name} holds the parse tables and the lexer rules that jison`,
    `// generates from the mermaid grammar ${rel}.`,
    '//',
    '// actions.js.txt in this directory has the JavaScript of every semantic',
    '// action and of every lexer action, with the numbers that the driver uses.',
    `package ${name}`,
    '',
    `import "${modulePath}/internal/jison"`,
    '',
    '// Numbers of the terminal symbols (terminals_ of the generated parser). A',
    '// terminal whose name is not an identifier has no constant.',
    'const ('
  );
  for (const id of tables.terminalIDs) {
    if (isIdent(tables.symbols[id])) {
      out.push(`\tTok${tables.symbols[id]} = ${id}`);
    }
  }
  out.push(')', '');
  out.push(
    `// Grammar is the grammar of ${path.basename(rel)}: ${parser.table.length} states, ${parser.productions_.length} productions, ${rules.length} lexer rules.`,
    'var Grammar = &jison.Grammar{',
    ...tables.lines,
    '}',
    ''
  );
  out.push(
    '// Parse parses input with the semantic actions of the caller. See jison.Grammar.Parse.',
    'func Parse(input string, yy any, performAction jison.PerformActionFunc) (any, error) {',
    '\treturn Grammar.Parse(input, yy, performAction)',
    '}',
    ''
  );
  return out.join('\n');
}

function commentLines(text, prefix) {
  return text
    .replace(/^\s*\n/, '')
    .replace(/\s+$/, '')
    .split('\n')
    .map((l) => (prefix + l.replace(/\t/g, '    ')).trimEnd());
}

function lexActionsFile(grammar, rules) {
  const { name, rel } = grammar;
  const manual = rules.filter((r) => !r.go);
  const usesJS = rules.some((r) => r.go?.usesJS);
  const out = [];
  out.push(
    `// Code generated by tools/reference/gen-jison.mjs from ${rel} (mermaid 12.2.0, jison ${jisonVersion}). DO NOT EDIT.`,
    '',
    `package ${name}`,
    ''
  );
  if (usesJS) {
    out.push('import (', `\t"${modulePath}/internal/jison"`, `\t"${modulePath}/internal/js"`, ')', '');
  } else {
    out.push(`import "${modulePath}/internal/jison"`, '');
  }
  out.push('func init() { Grammar.LexAction = LexAction }', '');
  out.push(
    '// LexAction runs the action of a lexer rule. It is the port of the generated',
    '// lexer.performAction. The generator translated ' +
      `${rules.length - manual.length} of the ${rules.length} actions.`
  );
  if (manual.length) {
    out.push(
      `// The actions of the rules ${manual.map((r) => r.index).join(', ')} are the`,
      '// functions lexRuleN in lexactions_manual.go.'
    );
  }
  out.push(
    'func LexAction(yy any, lx *jison.Lexer, rule int, startCondition string) (int, bool) {',
    '\tswitch rule {'
  );
  for (const r of rules) {
    out.push(`\tcase ${r.index}:`);
    const js = oneLine(stripComments(r.action));
    if (js !== '') {
      out.push(`\t\t// ${js}`);
    }
    if (r.go) {
      for (const line of r.go.lines) {
        out.push('\t\t' + line);
      }
    } else {
      out.push(`\t\treturn lexRule${r.index}(yy, lx, startCondition)`);
    }
  }
  out.push('\t}', '\treturn 0, false', '}', '');
  return out.join('\n');
}

function manualStub(grammar, r) {
  return [
    `// lexRule${r.index} is the action of lexer rule ${r.index}: /${oneLine(r.info.source)}/${r.info.flags}`,
    '//',
    ...commentLines(r.action, '//\t'),
    '//',
    '// TODO(port): translate the JavaScript above and remove this line.',
    `func lexRule${r.index}(yy any, lx *jison.Lexer, startCondition string) (int, bool) {`,
    `\tlx.Throw(jison.NotPorted(${r.index}))`,
    '\treturn 0, false',
    '}',
    '',
  ].join('\n');
}

function manualFileHeader(grammar) {
  const { name, rel, ast } = grammar;
  const out = [
    `// Lexer actions of ${rel} that tools/reference/gen-jison.mjs does not`,
    '// translate. The generator writes a stub for each such action one time.',
    '// This file is then maintained by hand, and the generator does not change it.',
    '',
    `package ${name}`,
    '',
    `import "${modulePath}/internal/jison"`,
    '',
  ];
  if (ast.lex.actionInclude?.trim()) {
    out.push(
      '// The grammar has this code in front of the lexer actions:',
      '//',
      ...commentLines(ast.lex.actionInclude, '//\t'),
      ''
    );
  }
  return out.join('\n');
}

function actionsText(grammar, rules, productions) {
  const { rel, ast, parser } = grammar;
  const out = [];
  out.push(
    `Actions of ${rel} (mermaid 12.2.0, jison ${jisonVersion}).`,
    'Generated by tools/reference/gen-jison.mjs. Do not edit.',
    '',
    'PARSER ACTIONS',
    '',
    'The number of a production is the argument "production" of PerformAction',
    '(yystate in the generated performAction). In the JavaScript:',
    '  $$        is ParserState.Result (this.$). Its value before the action is $1.',
    '  $1 .. $n  are ParserState.V(1) .. V(n). The value of a token is its yytext.',
    '  @$        is ParserState.Loc (this._$), and @1 .. @n are ParserState.L(1) .. L(n).',
    '  yytext, yyleng, yylineno are the fields of ParserState.',
    '  A "return" statement ends the parse: PerformAction returns that value.',
    'jison already replaced the named references ($name) by numbers here.',
    'A production without an action keeps $$ = $1.',
    ''
  );
  if (ast.actionInclude?.trim()) {
    out.push('Code in front of the parser actions:', ast.actionInclude, '');
  }
  for (const p of productions) {
    const rhs = p.rhs.length ? p.rhs.join(' ') : '/* empty */';
    out.push(`production ${p.id}: ${p.lhs} -> ${rhs}`);
    if (p.action === null) {
      out.push(p.id === 0 ? '    (the accept production; it has no action)' : '    (no action)');
    } else {
      out.push(...commentLines(p.action, '    '));
    }
    out.push('');
  }
  out.push(
    'LEXER ACTIONS',
    '',
    'The number of a rule is the argument "rule" of LexAction',
    '($avoiding_name_collisions in the generated lexer.performAction). In the JavaScript:',
    '  this      is the Lexer. this.begin, this.pushState, this.popState, this.unput,',
    '            this.less and this.more are the methods of Lexer with the same names.',
    '  yytext    is Lexer.Yytext.',
    "  return 'X' returns the terminal X. An action without a return gives no token.",
    `Lexer options: ${JSON.stringify(parser.lexer.options ?? {})}`,
    ''
  );
  if (ast.lex.actionInclude?.trim()) {
    out.push('Code in front of the lexer actions:', ast.lex.actionInclude.replace(/\s+$/, ''), '');
  }
  for (const r of rules) {
    const kind = r.go ? 'translated in lexactions_gen.go' : `manual: lexRule${r.index} in lexactions_manual.go`;
    out.push(`rule ${r.index}: <${r.conditions.join(',')}> /${r.info.source}/${r.info.flags}   (${kind})`);
    const text = r.action.replace(/^\s*\n/, '').replace(/\s+$/, '');
    if (text.trim() === '') {
      out.push('    (no action)');
    } else {
      out.push(...commentLines(text, '    '));
    }
    out.push('');
  }
  return out.join('\n');
}

// ---------------------------------------------------------------------------

const grammars = loadGrammars().filter((g) => only.length === 0 || only.includes(g.name));
const written = [];
const report = [];
let failed = false;
for (const grammar of grammars) {
  const { name, parser } = grammar;
  const dist = distParser(grammar.rel);
  if (!dist) {
    console.error(`${name}: the mermaid dist has no parser for ${grammar.rel}`);
    failed = true;
    continue;
  }
  const diff = sameTables(parser, dist.parser);
  if (diff.length) {
    console.error(`${name}: differs from the mermaid dist (${dist.chunk}) in ${diff.join(', ')}`);
    failed = true;
    continue;
  }
  const terminals = {};
  for (const [id, sym] of Object.entries(parser.terminals_)) {
    terminals[sym] = isIdent(sym) ? `Tok${sym}` : `${id} /* ${sym.replace(/\*\//g, '* /')} */`;
  }
  const rules = lexerRules(grammar).map((r) => ({
    ...r,
    info: analyzeRule(r.re),
    go: translateLexAction(r.action, terminals),
  }));
  const productions = parserActions(grammar);

  const dir = path.join(outRoot, name);
  fs.mkdirSync(dir, { recursive: true });
  const put = (file, text) => {
    fs.writeFileSync(path.join(dir, file), text);
    if (file.endsWith('.go')) {
      written.push(path.join(dir, file));
    }
  };
  put('tables.go', tablesFile(grammar, rules));
  put('lexactions_gen.go', lexActionsFile(grammar, rules));
  put('actions.js.txt', actionsText(grammar, rules, productions));

  // The manual file: write the stubs that are missing, keep everything else.
  const manual = rules.filter((r) => !r.go);
  const manualPath = path.join(dir, 'lexactions_manual.go');
  let manualText = fs.existsSync(manualPath) ? fs.readFileSync(manualPath, 'utf8') : undefined;
  let pending = 0;
  if (manual.length) {
    let text = manualText ?? manualFileHeader(grammar);
    for (const r of manual) {
      if (!new RegExp(`\\bfunc lexRule${r.index}\\(`).test(text)) {
        text += (text.endsWith('\n\n') ? '' : '\n') + manualStub(grammar, r);
      }
    }
    if (text !== manualText) {
      fs.writeFileSync(manualPath, text);
      written.push(manualPath);
      manualText = text;
    }
    for (const r of manual) {
      const start = manualText.search(new RegExp(`// lexRule${r.index} is the action`));
      const end = manualText.indexOf(`func lexRule${r.index}(`);
      if (start < 0 || manualText.slice(start, end).includes('TODO(port)')) {
        pending++;
      }
    }
  }
  report.push({
    grammar: name,
    states: parser.table.length,
    productions: parser.productions_.length,
    rules: rules.length,
    translated: rules.length - manual.length,
    manual: manual.length,
    manualPending: pending,
    manualRules: manual.map((r) => r.index).join(' '),
  });
}
if (written.length) {
  execFileSync('gofmt', ['-w', ...written], { stdio: 'inherit' });
}
console.table(report);
const sum = (k) => report.reduce((a, r) => a + r[k], 0);
console.log(
  `${report.length} grammars, tables equal to the mermaid dist. Lexer actions: ${sum('rules')} total, ${sum('translated')} translated, ${sum('manual')} manual (${sum('manualPending')} still stubs).`
);
process.exit(failed ? 1 : 0);
