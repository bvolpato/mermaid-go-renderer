// Page script of gen-jsmath.mjs. oracle.mjs runs it as the body of an async
// function in a Chromium page. args.funcs is a list of {name, arity, inputs}
// and inputs is the base64 text of the little-endian float64 arguments. The
// result has the bits of Math.<name>(...) for every case, as base64 text. A
// NaN result is the canonical NaN.
const decode = (text) => {
  const bin = atob(text);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) {
    bytes[i] = bin.charCodeAt(i);
  }
  return new Float64Array(bytes.buffer);
};
const encode = (floats) => {
  const bytes = new Uint8Array(floats.buffer);
  let bin = '';
  for (let i = 0; i < bytes.length; i += 0x8000) {
    bin += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
  }
  return btoa(bin);
};
const out = { userAgent: navigator.userAgent, funcs: [] };
for (const f of args.funcs) {
  const xs = decode(f.inputs);
  const arity = f.arity;
  const n = xs.length / arity;
  const ys = new Float64Array(n);
  // hypot1, hypot3 and hypot4 are Math.hypot with 1, 3 and 4 arguments.
  const fn = Math[f.name.startsWith('hypot') ? 'hypot' : f.name];
  for (let i = 0; i < n; i++) {
    let y;
    switch (arity) {
      case 1:
        y = fn(xs[i]);
        break;
      case 2:
        y = fn(xs[2 * i], xs[2 * i + 1]);
        break;
      case 3:
        y = fn(xs[3 * i], xs[3 * i + 1], xs[3 * i + 2]);
        break;
      default:
        y = fn(xs[4 * i], xs[4 * i + 1], xs[4 * i + 2], xs[4 * i + 3]);
    }
    ys[i] = y === y ? y : NaN;
  }
  out.funcs.push({ name: f.name, outputs: encode(ys) });
}
return out;
