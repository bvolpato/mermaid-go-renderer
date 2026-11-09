// Writes the tables and the golden traces of small grammars that use the
// parts of jison that no mermaid grammar uses: error recovery, the lexer
// options "flex", "backtrack_lexer" and "ranges", and the lexer methods more,
// less, unput, input, reject, topState and stateStackSize. The test
// internal/jison/synthetic_test.go has the actions of these grammars in Go
// and compares the Go driver with the traces of the generated JavaScript.
//
// Usage (in tools/reference): node gen-jison-synthetic.mjs
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { analyzeRule, buildGrammar, grammarTables, jisonVersion, repoRoot, tracer } from './jison-lib.mjs';

const outDir = path.join(repoRoot, 'internal/jison');

const grammars = {
  // Error recovery with the "error" symbol, operator precedence, and "ranges".
  calc: {
    src: String.raw`
%lex
%options ranges
%%
\s+                   /* skip whitespace */
[0-9]+("."[0-9]+)?\b  return 'NUMBER'
"*"                   return '*'
"/"                   return '/'
"-"                   return '-'
"+"                   return '+'
"("                   return '('
")"                   return ')'
";"                   return ';'
<<EOF>>               return 'EOF'
"@"                   return 'INVALID'
/lex

%left '+' '-'
%left '*' '/'
%left UMINUS
%start program
%%
program
    : lines EOF  { return $1.join(' '); }
    ;
lines
    : /* empty */ { $$ = []; }
    | lines line  { $1.push($2); $$ = $1; }
    ;
line
    : e ';'      { $$ = String($1) + '@' + @1.first_line + ':' + @1.first_column + '-' + @1.last_line + ':' + @1.last_column + '[' + @1.range + ']'; }
    | error ';'  { $$ = 'error(' + yytext + ',' + yyleng + ',' + yylineno + ')@' + @1.first_column + '[' + @$.range + ']'; }
    ;
e
    : e '+' e  { $$ = $1 + $3; }
    | e '-' e  { $$ = $1 - $3; }
    | e '*' e  { $$ = $1 * $3; }
    | e '/' e  { $$ = $1 / $3; }
    | '-' e %prec UMINUS { $$ = -$2; }
    | '(' e ')' { $$ = $2; }
    | NUMBER   { $$ = Number(yytext); }
    ;
`,
    inputs: [
      '1+2*3;',
      '1 + 2 * 3; (4 - 1) / 2;\n-3 * -(2 + 1);\n',
      '',
      ';',
      '1+;2;',
      '1 + + 2; 3*4;',
      '(1+2; 5;',
      '1 2 3; 7;',
      '@; 1;',
      '1+2',
      '1+2; 3 +',
      '1+2; 3 + ;',
      ') ) ); 8/4;',
      '1;;2;',
      '1/0; 0/0; 2.5*2;',
      '1 @ 2 @ 3; 4 @; 5;',
      '1 # 2;',
      '1+2;\n3+;\n\n4*;5;\n6',
      '((((1))));',
      '1 +\n 2 @\n ;\n 3;',
      '-;-1;--1;',
    ],
  },
  // The "flex" option: the longest match wins, and jison adds a rule for ".".
  flexlex: {
    src: String.raw`
%lex
%options flex
%%
"if"        return 'IF'
"iffy"      return 'IFFY'
[a-z]+      return 'ID'
[0-9]+      return 'NUM'
"=="        return 'EQ'
"="         return 'ASSIGN'
[ \t\n]+    /* skip */
<<EOF>>     return 'EOF'
/lex
%start start
%%
start : toks EOF { return $1; } ;
toks : /* empty */ { $$ = ''; } | toks tok { $$ = $1 + '(' + $2 + ')'; } ;
tok : IF | IFFY | ID | NUM | EQ | ASSIGN ;
`,
    inputs: ['if iffy ifs i', 'if==iffy=1', 'a = 12 == b', '', 'if', 'iffyx iffy9 if9', 'a # b', '=== ==', 'if\niffy\n\tx'],
  },
  // The "backtrack_lexer" option with reject().
  backtrack: {
    src: String.raw`
%lex
%options backtrack_lexer
%x special
%%
[a-z]+      { if (yytext === 'end' || yytext.length > 5) { this.begin('special'); this.reject(); } else { return 'WORD'; } }
"end"       return 'END'
[a-z]{3}    return 'TRI'
[a-z]       return 'CHAR'
[0-9]+      { if (yytext.length > 2) this.reject(); else return 'NUM'; }
[0-9]       return 'DIGIT'
\s+         /* skip */
<special>.  return 'NEVER'
<<EOF>>     return 'EOF'
/lex
%start start
%%
start : toks EOF { return $1; } ;
toks : /* empty */ { $$ = ''; } | toks tok { $$ = $1 + '(' + $2 + '@' + @2.first_line + ':' + @2.first_column + '-' + @2.last_line + ':' + @2.last_column + ')'; } ;
tok : WORD | END | TRI | CHAR | NUM | DIGIT | NEVER ;
`,
    inputs: ['abc end', 'end', 'abcdefgh 12 1234 9', 'ab\nend\nabcdefg\n', '', 'abc # d', 'endx end endings', '123456 12'],
  },
  // more, less, unput, input, topState, stateStackSize, inclusive and
  // exclusive conditions, a condition that does not exist, and "ranges".
  moreless: {
    src: String.raw`
%lex
%options ranges
%x str
%s incl
%%
"\""            { this.begin('str'); this.more(); }
<str>[^"\\\n]+  this.more();
<str>\n         this.more();
<str>"\\".      this.more();
<str>"\""       { this.popState(); return 'STRING'; }
"abc"           { this.less(1); return 'A'; }
"<<"            { this.unput('>>x'); return 'OPEN'; }
">>"            return 'CLOSE'
"#"             { var c = this.input(); yytext = yytext + '|' + c + '|' + this.topState() + '|' + this.topState(1) + '|' + this.stateStackSize(); return 'HASH'; }
"@"             { this.begin('incl'); return 'AT'; }
<incl>"!"       { this.popState(); return 'BANG'; }
"?"             { yytext = this.pastInput() + '^' + this.upcomingInput() + '^' + this.showPosition(); return 'Q'; }
"$"             { this.begin('nosuch'); return 'DOLLAR'; }
"%"             { this.begin(''); return 'PCT'; }
"~"             return 'TILDE_UNKNOWN'
"&"             { this.unput('\n\n'); return 'AMP'; }
"line"\n"two"   { this.less(4); return 'LINE'; }
[a-z]+          return 'WORD'
[0-9]+          return 'NUM'
[ \t\n\r]+      /* skip */
<<EOF>>         return 'EOF'
/lex
%start start
%%
start : items EOF { return $1; } ;
items : /* empty */ { $$ = ''; } | items item { $$ = $1 + '<' + $2 + '>'; } ;
item : STRING | A | OPEN | CLOSE | HASH | AT | BANG | Q | DOLLAR | PCT | AMP | LINE | WORD | NUM ;
`,
    inputs: [
      'abc def',
      '"a string" x',
      '"multi\nline \\" quoted\\\\" y "z"',
      '"never closed',
      'x << y',
      'a #b #\nc #',
      '#',
      '@ a ! b ! c',
      '@ @ # !',
      'one two three four five six seven ? eight nine ten eleven twelve thirteen',
      'a\nb\n?\nc',
      '?',
      'x $ y',
      'x % abc % y',
      'x ~ y',
      'abc abc\nabc',
      'a & b',
      'line\ntwo x',
      'q "s1" << 12 >> "s2\n" # @!',
      '\r\n abc \r "x\r\ny" ? \r\n',
      String.fromCodePoint(0x1f600) + ' a',
      'a "' + String.fromCodePoint(0x1f600) + '" ?',
    ],
  },
  // Rules that never give a token and never shorten the input: lexer.lex
  // calls itself until V8 stops it.
  stall: {
    src: String.raw`
%lex
%%
[a-z]*      /* matches the empty string */
"1"         { this.unput('1'); }
[0-9]+      return 'NUM'
\s+         /* skip */
<<EOF>>     return 'EOF'
/lex
%start start
%%
start : toks EOF { return $1; } ;
toks : /* empty */ { $$ = ''; } | toks NUM { $$ = $1 + '(' + $2 + ')'; } ;
`,
    inputs: ['abc', 'abc 1', '2', 'abc 22 1 2', ''],
  },
};

const built = [];
const trace = ['jison-synthetic 1'];
for (const [name, def] of Object.entries(grammars)) {
  const grammar = buildGrammar(name, name + '.jison', def.src);
  const rules = grammar.parser.lexer.rules.map((re) => analyzeRule(re));
  built.push({ name, grammar, tables: grammarTables(grammar, rules) });
  const run = tracer(grammar, grammar.parser);
  trace.push(`G ${name}`);
  for (const input of def.inputs) {
    const log = console.log;
    const events = run(input, {
      result: true,
      // The rule that "flex" adds calls console.log(yytext).
      yy: (ev) => {
        console.log = (...args) => ev.push(`LOG ${JSON.stringify(args.join(' '))}`);
        return {};
      },
    });
    console.log = log;
    trace.push(`F ${name}`, `I ${JSON.stringify(input)}`, 'LOC', ...events);
  }
}

const go = [
  `// Code generated by tools/reference/gen-jison-synthetic.mjs (jison ${jisonVersion}). DO NOT EDIT.`,
  '',
  'package jison_test',
  '',
  'import "github.com/bvolpato/mermaid-go-renderer/internal/jison"',
  '',
  '// syntheticGrammars holds the tables of the grammars in gen-jison-synthetic.mjs.',
  'var syntheticGrammars = map[string]*jison.Grammar{',
];
for (const b of built) {
  go.push(`\t${JSON.stringify(b.name)}: {`, ...b.tables.lines.map((l) => '\t' + l), '\t},');
}
go.push('}', '');
// The symbol numbers, for the actions in the test.
go.push('// syntheticSymbols holds the symbol numbers of each grammar by name.', 'var syntheticSymbols = map[string]map[string]int{');
for (const b of built) {
  go.push(`\t${JSON.stringify(b.name)}: {`);
  for (const [sym, id] of Object.entries(b.grammar.parser.symbols_)) {
    go.push(`\t\t${JSON.stringify(sym)}: ${id},`);
  }
  go.push('\t},');
}
go.push('}', '');
const goPath = path.join(outDir, 'synthetic_tables_test.go');
fs.writeFileSync(goPath, go.join('\n'));
execFileSync('gofmt', ['-w', goPath], { stdio: 'inherit' });
fs.mkdirSync(path.join(outDir, 'testdata'), { recursive: true });
fs.writeFileSync(path.join(outDir, 'testdata/synthetic.txt'), trace.join('\n') + '\n');
for (const b of built) {
  const p = b.grammar.parser;
  console.log(
    `${b.name}: ${p.table.length} states, ${p.lexer.rules.length} rules, options ${JSON.stringify(p.lexer.options)}, error recovery ${b.grammar.gen.hasErrorRecovery}, ${grammars[b.name].inputs.length} inputs`
  );
  // The semantic actions and the lexer actions, as a reference for the Go test.
  if (process.argv.includes('--actions')) {
    console.log(b.grammar.gen.performAction);
    console.log(b.grammar.gen.lexer.performAction);
  }
}
