// Records katex.renderToString from KaTeX 0.18.11 dist/katex.mjs (MIT).
// Run from tools/reference: node gen-katex.mjs.
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import installedKatex from './node_modules/katex/dist/katex.mjs';
import katex from '../../internal/katex/katexjs/katex.mjs';

const version = '0.18.11';
if (katex.version !== version) throw new Error(`unexpected KaTeX ${katex.version}`);
const source = new URL('../../internal/katex/katexjs/katex.mjs', import.meta.url);
const installedSource = new URL('./node_modules/katex/dist/katex.mjs', import.meta.url);
const destination = new URL('../../internal/katex/testdata/render.json.gz', import.meta.url);
const outputs = ['mathml', 'htmlAndMathml'];
const inputs = [];
const add = (name, tex) => inputs.push({ name, tex });

for (const [name, tex] of [
  ['empty', ''],
  ['whitespace', ' \t\n '],
  ['comment', '% ignored'],
  ['comment-newline', 'x% ignored\n+y'],
  ['power', String.raw`x^2+y_1`],
  ['fraction', String.raw`\frac{a+b}{c-d}`],
  ['root', String.raw`\sqrt[3]{x+1}`],
  ['sum', String.raw`\sum_{i=0}^{n} i^2`],
  ['integral', String.raw`\int_{-\infty}^{\infty} e^{-x^2}\,dx`],
  ['fourier', String.raw`f(\relax{x}) = \int_{-\infty}^\infty \hat{f}(\xi)\,e^{2 \pi i \xi x}\,d\xi`],
  ['delimiters', String.raw`\left\langle \frac{x}{y} \middle| z \right\rangle`],
  ['delimiter-sizes', String.raw`\Bigg(\bigg(\Big(\big((\frac{-b\pm\sqrt{b^2-4ac}}{2a})\big)\Big)\bigg)\Bigg)`],
  ['continued-fraction', String.raw`1+\frac{e^{-2\pi}}{1+\frac{e^{-4\pi}}{1+\frac{e^{-6\pi}}{1+\cdots}}}`],
  ['overbrace', String.raw`\overbrace{a+b+c}^{\text{note}}`],
  ['phase', String.raw`\phase{-78^\circ}`],
  ['cases', String.raw`x=\begin{cases}a&\text{if }b\\c&\text{if }d\end{cases}`],
  ['matrix', String.raw`\begin{pmatrix}a&b\\c&d\end{pmatrix}`],
  ['array', String.raw`\begin{array}{r|cl}1&2&3\\[2pt]4&5&6\end{array}`],
  ['aligned', String.raw`\begin{aligned}a&=b+c\\d&=e\end{aligned}`],
  ['greek-lower', String.raw`\alpha\beta\gamma\delta\epsilon\zeta\eta\theta\iota\kappa\lambda\mu\nu\xi\omicron\pi\rho\sigma\tau\upsilon\phi\chi\psi\omega`],
  ['greek-upper', String.raw`\Alpha\Beta\Gamma\Delta\Epsilon\Zeta\Eta\Theta\Iota\Kappa\Lambda\Mu\Nu\Xi\Omicron\Pi\Rho\Sigma\Tau\Upsilon\Phi\Chi\Psi\Omega`],
  ['sets', String.raw`\forall\complement\therefore\emptyset\exists\subset\because\empty\exist\supset\mapsto\varnothing\nexists\mid\to\implies\in\land\gets\impliedby\isin\lor\leftrightarrow\iff\notin\ni\notni\lnot`],
  ['fonts', String.raw`\mathbf{Ab}\mathbb{R}\mathcal{F}\mathfrak{g}\mathsf{x}\mathtt{y}`],
  ['accents', String.raw`\hat{x}\widehat{xyz}\vec{v}\tilde{a}\dot{x}\ddot{y}`],
  ['spacing', String.raw`a\!b\,c\:d\;e\quad f\qquad g\kern1.25em h`],
  ['styles', String.raw`\displaystyle\frac{x}{y}+\textstyle\frac{x}{y}+\scriptstyle x`],
  ['color', String.raw`\textcolor{red}{x}+\color{#123abc}y`],
  ['rule', String.raw`\rule[-0.2em]{1.25em}{0.4em}`],
  ['phantom', String.raw`\phantom{x^2}\hphantom{a}\vphantom{b}`],
  ['html-escape', String.raw`\text{<&>"'}`],
  ['text-ligatures', "\\text{a---b--c ``d''}"],
  ['untrusted-link', String.raw`\href{javascript:alert(1)}{x}`],
  ['untrusted-html', String.raw`\htmlClass{example}{x}`],
  ['console-message', String.raw`\message{hello}x`],
  ['console-error', String.raw`\errmessage{hello}x`],
  ['console-show', String.raw`\show\alpha x`],
  ['macro-local', String.raw`\def\oraclelocal#1{#1^2}\oraclelocal{x}`],
  ['macro-global', String.raw`\gdef\oracleglobal{z}\oracleglobal`],
  ['macro-global-absent', String.raw`\oracleglobal`],
  ['macro-local-absent', String.raw`\oraclelocal{x}`],
  ['macro-renew', String.raw`\newcommand{\oraclearg}[2]{#1+#2}\renewcommand{\oraclearg}[2]{#1-#2}\oraclearg{a}{b}`],
  ['macro-group', String.raw`{\def\oracleinner{x}\oracleinner}\oracleinner`],
  ['macro-global-before-error', String.raw`\gdef\oraclefailed{x}\undefinedoracle`],
  ['macro-failed-absent', String.raw`\oraclefailed`],
  ['error-undefined', String.raw`x+\undefinedoracle`],
  ['error-missing-group', String.raw`\frac{a}{`],
  ['error-extra-group', 'x}'],
  ['error-double-superscript', 'x^2^3'],
  ['error-double-subscript', 'x_2_3'],
  ['error-missing-right', String.raw`\left(x`],
  ['error-mismatched-environment', String.raw`\begin{matrix}x\end{cases}`],
  ['error-environment', String.raw`\begin{unknown}x\end{unknown}`],
  ['error-color', String.raw`\textcolor{not-a-color}{x}`],
  ['error-dimension', String.raw`\kern 1zz x`],
  ['error-macro-loop', String.raw`\def\oracleloop{\oracleloop}\oracleloop`],
  ['error-newcommand-existing', String.raw`\newcommand{\frac}{x}`],
  ['unicode-greek', 'α+β=Γ'],
  ['unicode-operators', '∀x∈ℝ: x≤∞'],
  ['unicode-accents', 'é+x\u0301+Å'],
  ['unicode-scripts', 'x²+y₁'],
  ['unicode-text', String.raw`\text{日本語 Кириллица العربية}`],
  ['unicode-astral-math', '𝐀+𝑥+𝔸+𝟙'],
  ['unicode-astral-accent', '𝑥\u0301+𝐀\u0302\u0301'],
  ['unicode-astral-after-macro', String.raw`\alpha 𝑥`],
  ['unicode-astral-control-symbol', '\\😀'],
  ['unicode-astral-verb', String.raw`\verb|a😀b|`],
  ['unicode-emoji-text', String.raw`\text{a😀b}`],
  ['unicode-emoji-math', '😀+x'],
  ['unicode-error-position', '𝑥+\\undefinedoracle'],
  ['unicode-error-context', 'x'.repeat(28) + 'é😀\\undefinedoracle' + 'y'.repeat(28)],
  ['unicode-replacement', '\ufffd'],
  ['unicode-replacement-double', '\ufffd\ufffd'],
  ['unicode-lone-high', 'a\ud800b'],
  ['unicode-lone-low-text', '\\text{a\udc00b}'],
  ['unicode-broken-pair', '\ud835x\udc00'],
  ['unicode-high-high', '\ud800\udbff'],
  ['unicode-low-low', '\udc00\udfff'],
  ['unicode-high-high-low', '\ud800\ud835\udc65'],
  ['unicode-low-high-low', '\udc00\ud835\udc65'],
  ['unicode-pair-boundaries', '\ud800\udc00+\udbff\udfff'],
  ['unicode-lone-error-context', '\ud800\\undefinedoracle'],
  ['unicode-char-surrogate', String.raw`\char"D800`],
  ['unicode-nul', '\0'],
  ['nested-groups', '{'.repeat(24) + 'x' + '}'.repeat(24)],
  ['error-stack-overflow', '{'.repeat(20000) + 'x' + '}'.repeat(20000)],
]) add(name, tex);

const seed = 0x4b615465;
let state = seed;
const random = (n) => {
  state ^= state << 13;
  state ^= state >>> 17;
  state ^= state << 5;
  return (state >>> 0) % n;
};
const atoms = ['x', 'y', '2', String.raw`\alpha`, String.raw`\infty`, '𝑥'];
function expression(depth) {
  if (!depth) return atoms[random(atoms.length)];
  const a = expression(depth - 1);
  const b = expression(depth - 1);
  switch (random(7)) {
    case 0: return `{${a}}+{${b}}`;
    case 1: return String.raw`\frac{${a}}{${b}}`;
    case 2: return String.raw`\sqrt{${a}}+${b}`;
    case 3: return `{${a}}^{${b}}`;
    case 4: return String.raw`\left(${a}\right)_{${b}}`;
    case 5: return String.raw`\overline{${a}}-${b}`;
    default: return String.raw`\mathbf{${a}}=${b}`;
  }
}
for (let i = 0; i < 64; i++) add(`seeded-${String(i).padStart(2, '0')}`, expression(1 + random(3)));

// JSON escapes lone surrogates, but Go's JSON decoder replaces them.
// Record their code units so the test keeps the original JavaScript string.
function text(value) {
  for (let i = 0; i < value.length; i++) {
    const unit = value.charCodeAt(i);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(i + 1);
      if (next >= 0xdc00 && next <= 0xdfff) { i++; continue; }
    } else if (unit < 0xdc00 || unit > 0xdfff) continue;
    return { units: Array.from({ length: value.length }, (_, j) => value.charCodeAt(j)) };
  }
  return { text: value };
}

const cases = [];
let failures = 0;
let installedDifferences = 0;
function result(api, tex, output) {
  try {
    return { want: text(api.renderToString(tex, { throwOnError: true, displayMode: true, output })) };
  } catch (error) {
    const parse = error instanceof api.ParseError;
    const hasPosition = parse && typeof error.position === 'number' && typeof error.length === 'number';
    const value = { parse, name: error.name, message: text(error.message) };
    if (parse) Object.assign(value, {
      rawMessage: text(error.rawMessage), hasPosition,
      position: hasPosition ? error.position : 0, length: hasPosition ? error.length : 0,
    });
    return { error: value };
  }
}
const originalConsole = { log: console.log, warn: console.warn, error: console.error };
try {
  console.log = console.warn = console.error = () => {};
  // Each mode replays the full sequence in the same upstream module.
  for (const output of outputs) {
    for (const { name, tex } of inputs) {
      const expected = result(katex, tex, output);
      const c = { name, tex: text(tex), output, ...expected };
      if (c.error) failures++;
      const installed = result(installedKatex, tex, output);
      if (JSON.stringify(installed) !== JSON.stringify(expected)) {
        installedDifferences++;
        c.installed = installed;
      }
      cases.push(c);
    }
  }
} finally {
  Object.assign(console, originalConsole);
}
const golden = {
  source: 'internal/katex/katexjs/katex.mjs', version,
  sourceSHA256: createHash('sha256').update(readFileSync(source)).digest('hex'),
  installed: {
    source: 'tools/reference/node_modules/katex/dist/katex.mjs', version: installedKatex.version,
    sourceSHA256: createHash('sha256').update(readFileSync(installedSource)).digest('hex'),
    differences: installedDifferences,
  },
  options: { throwOnError: true, displayMode: true }, seed, cases,
};
const data = gzipSync(JSON.stringify(golden, null, 2) + '\n', { level: 9 });
writeFileSync(destination, data);
console.log(`wrote ${destination.pathname}: ${cases.length} cases (${failures} errors), ${data.length} bytes; installed ${installedKatex.version}: ${installedDifferences} differences`);
