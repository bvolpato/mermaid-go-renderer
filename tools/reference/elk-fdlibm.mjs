// The fdlibm functions of v8/src/base/ieee754.cc (V8 14.6) in JavaScript:
// sin, cos, acos, atan, atan2, log and pow.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// gen-elk.mjs installs these functions as Math.sin and so on before it runs
// elkjs. JavaScript rounds after each operation, so the results are those of
// a V8 build that does not contract a multiplication and an addition into one
// fused operation. The native functions of a build are not a stable
// reference: node for arm64 is built with contraction, and its Math.sin
// differs from node for x64 in the last bit for about 1 argument in 250.
// internal/elk/fdlibm.go is the same code in Go.

const f64 = new Float64Array(1);
const u32 = new Uint32Array(f64.buffer);
const i32 = new Int32Array(f64.buffer);
// The byte order of every platform that node supports is little endian.
const HI = 1;
const LO = 0;

const hiWord = (x) => {
  f64[0] = x;
  return i32[HI];
};
const loWord = (x) => {
  f64[0] = x;
  return u32[LO];
};
const withLoWord = (x, lo) => {
  f64[0] = x;
  u32[LO] = lo;
  return f64[0];
};
const withHiWord = (x, hi) => {
  f64[0] = x;
  i32[HI] = hi;
  return f64[0];
};
const fromWords = (hi, lo) => {
  i32[HI] = hi;
  u32[LO] = lo;
  return f64[0];
};
// scalbn of fdlibm (s_scalbn.c).
function scalbn(x, n) {
  const two54 = 1.8014398509481984e16;
  const twom54 = 5.55111512312578270212e-17;
  const huge = 1.0e300;
  const tiny = 1.0e-300;
  let hx = hiWord(x);
  const lx = loWord(x);
  let k = (hx & 0x7ff00000) >> 20;
  if (k === 0) {
    if ((lx | (hx & 0x7fffffff)) === 0) return x;
    x *= two54;
    hx = hiWord(x);
    k = ((hx & 0x7ff00000) >> 20) - 54;
    if (n < -50000) return tiny * x;
  }
  if (k === 0x7ff) return x + x;
  k = k + n;
  if (k > 0x7fe) return huge * (x < 0 ? -huge : huge);
  if (k > 0) return withHiWord(x, (hx & 0x800fffff) | (k << 20));
  if (k <= -54) {
    if (n > 50000) return huge * (x < 0 ? -huge : huge);
    return tiny * (x < 0 ? -tiny : tiny);
  }
  k += 54;
  x = withHiWord(x, (hx & 0x800fffff) | (k << 20));
  return x * twom54;
}

const twoOverPi = [
  0xa2f983, 0x6e4e44, 0x1529fc, 0x2757d1, 0xf534dd, 0xc0db62, 0x95993c, 0x439041, 0xfe5163, 0xabdebb, 0xc561b7, 0x246e3a, 0x424dd2, 0xe00649, 0x2eea09, 0xd1921c,
  0xfe1deb, 0x1cb129, 0xa73ee8, 0x8235f5, 0x2ebb44, 0x84e99c, 0x7026b4, 0x5f7e41, 0x3991d6, 0x398353, 0x39f49c, 0x845f8b, 0xbdf928, 0x3b1ff8, 0x97ffde, 0x05980f,
  0xef2f11, 0x8b5a0a, 0x6d1f6d, 0x367ecf, 0x27cb09, 0xb74f46, 0x3f669e, 0x5fea2d, 0x7527ba, 0xc7ebe5, 0xf17b3d, 0x0739f7, 0x8a5292, 0xea6bfb, 0x5fb11f, 0x8d5d08,
  0x560330, 0x46fc7b, 0x6babf0, 0xcfbc20, 0x9af436, 0x1da9e3, 0x91615e, 0xe61b08, 0x659985, 0x5f14a0, 0x68408d, 0xffd880, 0x4d7327, 0x310606, 0x1556ca, 0x73a8c9,
  0x60e27b, 0xc08c6b,
];

const npio2Hw = [
  0x3ff921fb, 0x400921fb, 0x4012d97c, 0x401921fb, 0x401f6a7a, 0x4022d97c, 0x4025fdbb, 0x402921fb, 0x402c463a, 0x402f6a7a, 0x4031475c, 0x4032d97c, 0x40346b9c,
  0x4035fdbb, 0x40378fdb, 0x403921fb, 0x403ab41b, 0x403c463a, 0x403dd85a, 0x403f6a7a, 0x40407e4c, 0x4041475c, 0x4042106c, 0x4042d97c, 0x4043a28c, 0x40446b9c,
  0x404534ac, 0x4045fdbb, 0x4046c6cb, 0x40478fdb, 0x404858eb, 0x404921fb,
];

const two24 = 1.6777216e7;
const twon24 = 5.9604644775390625e-8;
const invpio2 = 6.36619772367581382433e-1;
const pio2_1 = 1.57079632673412561417;
const pio2_1t = 6.07710050650619224932e-11;
const pio2_2 = 6.0771005063039659766e-11;
const pio2_2t = 2.02226624879595063154e-21;
const pio2_3 = 2.0222662487111664558e-21;
const pio2_3t = 8.47842766036889956997e-32;

const pio2Table = [
  1.57079625129699707031, 7.54978941586159635335e-8, 5.39030252995776476554e-15, 3.28200341580791294123e-22, 1.27065575308067607349e-29, 1.22933308981111328932e-36,
  2.73370053816464559624e-44, 2.16741683877804819444e-51,
];

// __kernel_rem_pio2 for prec = 2 and ipio2 = two_over_pi.
function kernelRemPio2(x, y, e0) {
  const jk = 4;
  const jp = jk;
  const iq = new Int32Array(20);
  const f = new Float64Array(20);
  const fq = new Float64Array(20);
  const q = new Float64Array(20);
  const jx = x.length - 1;
  let jv = ((e0 - 3) / 24) | 0;
  if (jv < 0) jv = 0;
  let q0 = e0 - 24 * (jv + 1);
  let j = jv - jx;
  const m = jx + jk;
  for (let i = 0; i <= m; i++, j++) f[i] = j < 0 ? 0 : twoOverPi[j];
  for (let i = 0; i <= jk; i++) {
    let fw = 0;
    for (let k = 0; k <= jx; k++) fw += x[k] * f[jx + i - k];
    q[i] = fw;
  }
  let jz = jk;
  let n;
  let ih;
  let z;
  for (;;) {
    z = q[jz];
    for (let i = 0, k = jz; k > 0; i++, k--) {
      const fw = (twon24 * z) | 0;
      iq[i] = (z - two24 * fw) | 0;
      z = q[k - 1] + fw;
    }
    z = scalbn(z, q0);
    z -= 8.0 * Math.floor(z * 0.125);
    n = z | 0;
    z -= n;
    ih = 0;
    if (q0 > 0) {
      const i = iq[jz - 1] >> (24 - q0);
      n += i;
      iq[jz - 1] -= i << (24 - q0);
      ih = iq[jz - 1] >> (23 - q0);
    } else if (q0 === 0) {
      ih = iq[jz - 1] >> 23;
    } else if (z >= 0.5) {
      ih = 2;
    }
    if (ih > 0) {
      n += 1;
      let carry = 0;
      for (let i = 0; i < jz; i++) {
        const k = iq[i];
        if (carry === 0) {
          if (k !== 0) {
            carry = 1;
            iq[i] = 0x1000000 - k;
          }
        } else {
          iq[i] = 0xffffff - k;
        }
      }
      if (q0 > 0) {
        if (q0 === 1) iq[jz - 1] &= 0x7fffff;
        else if (q0 === 2) iq[jz - 1] &= 0x3fffff;
      }
      if (ih === 2) {
        z = 1.0 - z;
        if (carry !== 0) z -= scalbn(1.0, q0);
      }
    }
    if (z !== 0) break;
    let any = 0;
    for (let i = jz - 1; i >= jk; i--) any |= iq[i];
    if (any !== 0) break;
    let k = 1;
    while (iq[jk - k] === 0) k++;
    for (let i = jz + 1; i <= jz + k; i++) {
      f[jx + i] = twoOverPi[jv + i];
      let fw = 0;
      for (let l = 0; l <= jx; l++) fw += x[l] * f[jx + i - l];
      q[i] = fw;
    }
    jz += k;
  }
  if (z === 0.0) {
    jz -= 1;
    q0 -= 24;
    while (iq[jz] === 0) {
      jz--;
      q0 -= 24;
    }
  } else {
    z = scalbn(z, -q0);
    if (z >= two24) {
      const fw = (twon24 * z) | 0;
      iq[jz] = (z - two24 * fw) | 0;
      jz += 1;
      q0 += 24;
      iq[jz] = fw | 0;
    } else {
      iq[jz] = z | 0;
    }
  }
  let fw = scalbn(1.0, q0);
  for (let i = jz; i >= 0; i--) {
    q[i] = fw * iq[i];
    fw *= twon24;
  }
  for (let i = jz; i >= 0; i--) {
    fw = 0.0;
    for (let k = 0; k <= jp && k <= jz - i; k++) fw += pio2Table[k] * q[i + k];
    fq[jz - i] = fw;
  }
  fw = 0.0;
  for (let i = jz; i >= 0; i--) fw += fq[i];
  y[0] = ih === 0 ? fw : -fw;
  fw = fq[0] - fw;
  for (let i = 1; i <= jz; i++) fw += fq[i];
  y[1] = ih === 0 ? fw : -fw;
  return n & 7;
}

// __ieee754_rem_pio2. Returns [n, y0, y1].
function remPio2(x) {
  const hx = hiWord(x);
  const ix = hx & 0x7fffffff;
  let y0;
  let y1;
  if (ix <= 0x3fe921fb) return [0, x, 0];
  if (ix < 0x4002d97c) {
    if (hx > 0) {
      let z = x - pio2_1;
      if (ix !== 0x3ff921fb) {
        y0 = z - pio2_1t;
        y1 = z - y0 - pio2_1t;
      } else {
        z -= pio2_2;
        y0 = z - pio2_2t;
        y1 = z - y0 - pio2_2t;
      }
      return [1, y0, y1];
    }
    let z = x + pio2_1;
    if (ix !== 0x3ff921fb) {
      y0 = z + pio2_1t;
      y1 = z - y0 + pio2_1t;
    } else {
      z += pio2_2;
      y0 = z + pio2_2t;
      y1 = z - y0 + pio2_2t;
    }
    return [-1, y0, y1];
  }
  if (ix <= 0x413921fb) {
    let t = Math.abs(x);
    const n = (t * invpio2 + 0.5) | 0;
    const fn = n;
    let r = t - fn * pio2_1;
    let w = fn * pio2_1t;
    if (n < 32 && ix !== npio2Hw[n - 1]) {
      y0 = r - w;
    } else {
      const j = ix >> 20;
      y0 = r - w;
      let i = j - ((hiWord(y0) >> 20) & 0x7ff);
      if (i > 16) {
        t = r;
        w = fn * pio2_2;
        r = t - w;
        w = fn * pio2_2t - (t - r - w);
        y0 = r - w;
        i = j - ((hiWord(y0) >> 20) & 0x7ff);
        if (i > 49) {
          t = r;
          w = fn * pio2_3;
          r = t - w;
          w = fn * pio2_3t - (t - r - w);
          y0 = r - w;
        }
      }
    }
    y1 = r - y0 - w;
    if (hx < 0) return [-n, -y0, -y1];
    return [n, y0, y1];
  }
  if (ix >= 0x7ff00000) {
    y0 = x - x;
    return [0, y0, y0];
  }
  const e0 = (ix >> 20) - 1046;
  let z = fromWords(ix - (e0 << 20), loWord(x));
  const tx = [0, 0, 0];
  for (let i = 0; i < 2; i++) {
    tx[i] = z | 0;
    z = (z - tx[i]) * two24;
  }
  tx[2] = z;
  let nx = 3;
  while (tx[nx - 1] === 0) nx--;
  const y = [0, 0];
  const n = kernelRemPio2(tx.slice(0, nx), y, e0);
  if (hx < 0) return [-n, -y[0], -y[1]];
  return [n, y[0], y[1]];
}

function kernelSin(x, y, iy) {
  const half = 0.5;
  const S1 = -1.66666666666666324348e-1;
  const S2 = 8.33333333332248946124e-3;
  const S3 = -1.98412698298579493134e-4;
  const S4 = 2.75573137070700676789e-6;
  const S5 = -2.50507602534068634195e-8;
  const S6 = 1.58969099521155010221e-10;
  const ix = hiWord(x) & 0x7fffffff;
  if (ix < 0x3e400000) {
    if ((x | 0) === 0) return x;
  }
  const z = x * x;
  const v = z * x;
  const r = S2 + z * (S3 + z * (S4 + z * (S5 + z * S6)));
  if (iy === 0) return x + v * (S1 + z * r);
  return x - (z * (half * y - v * r) - y - v * S1);
}

function kernelCos(x, y) {
  const C1 = 4.16666666666666019037e-2;
  const C2 = -1.38888888888741095749e-3;
  const C3 = 2.48015872894767294178e-5;
  const C4 = -2.75573143513906633035e-7;
  const C5 = 2.0875723212981748279e-9;
  const C6 = -1.13596475577881948265e-11;
  const ix = hiWord(x) & 0x7fffffff;
  if (ix < 0x3e400000) {
    if ((x | 0) === 0) return 1;
  }
  const z = x * x;
  const r = z * (C1 + z * (C2 + z * (C3 + z * (C4 + z * (C5 + z * C6)))));
  if (ix < 0x3fd33333) return 1 - (0.5 * z - (z * r - x * y));
  let qx;
  if (ix > 0x3fe90000) {
    qx = 0.28125;
  } else {
    qx = fromWords(ix - 0x00200000, 0);
  }
  const iz = 0.5 * z - qx;
  const a = 1 - qx;
  return a - (iz - (z * r - x * y));
}

export function sin(x) {
  x = +x;
  const ix = hiWord(x) & 0x7fffffff;
  if (ix <= 0x3fe921fb) return kernelSin(x, 0, 0);
  if (ix >= 0x7ff00000) return x - x;
  const [n, y0, y1] = remPio2(x);
  switch (n & 3) {
    case 0:
      return kernelSin(y0, y1, 1);
    case 1:
      return kernelCos(y0, y1);
    case 2:
      return -kernelSin(y0, y1, 1);
    default:
      return -kernelCos(y0, y1);
  }
}

export function cos(x) {
  x = +x;
  const ix = hiWord(x) & 0x7fffffff;
  if (ix <= 0x3fe921fb) return kernelCos(x, 0);
  if (ix >= 0x7ff00000) return x - x;
  const [n, y0, y1] = remPio2(x);
  switch (n & 3) {
    case 0:
      return kernelCos(y0, y1);
    case 1:
      return -kernelSin(y0, y1, 1);
    case 2:
      return -kernelCos(y0, y1);
    default:
      return kernelSin(y0, y1, 1);
  }
}

export function acos(x) {
  x = +x;
  const pi = 3.141592653589793116;
  const pio2Hi = 1.570796326794896558;
  const pio2Lo = 6.12323399573676603587e-17;
  const pS0 = 1.66666666666666657415e-1;
  const pS1 = -3.25565818622400915405e-1;
  const pS2 = 2.01212532134862925881e-1;
  const pS3 = -4.00555345006794114027e-2;
  const pS4 = 7.91534994289814532176e-4;
  const pS5 = 3.4793310759602116757e-5;
  const qS1 = -2.40339491173441421878;
  const qS2 = 2.02094576023350569471;
  const qS3 = -6.8828397160545329303e-1;
  const qS4 = 7.70381505559019352791e-2;
  const hx = hiWord(x);
  const ix = hx & 0x7fffffff;
  if (ix >= 0x3ff00000) {
    if (((ix - 0x3ff00000) | loWord(x)) === 0) {
      if (hx > 0) return 0.0;
      return pi + 2.0 * pio2Lo;
    }
    return (x - x) / (x - x);
  }
  if (ix < 0x3fe00000) {
    if (ix <= 0x3c600000) return pio2Hi + pio2Lo;
    const z = x * x;
    const p = z * (pS0 + z * (pS1 + z * (pS2 + z * (pS3 + z * (pS4 + z * pS5)))));
    const q = 1 + z * (qS1 + z * (qS2 + z * (qS3 + z * qS4)));
    const r = p / q;
    return pio2Hi - (x - (pio2Lo - r * x));
  }
  if (hx < 0) {
    const z = (1 + x) * 0.5;
    const p = z * (pS0 + z * (pS1 + z * (pS2 + z * (pS3 + z * (pS4 + z * pS5)))));
    const q = 1 + z * (qS1 + z * (qS2 + z * (qS3 + z * qS4)));
    const s = Math.sqrt(z);
    const r = p / q;
    const w = r * s - pio2Lo;
    return pi - 2.0 * (s + w);
  }
  const z = (1 - x) * 0.5;
  const s = Math.sqrt(z);
  const df = withLoWord(s, 0);
  const c = (z - df * df) / (s + df);
  const p = z * (pS0 + z * (pS1 + z * (pS2 + z * (pS3 + z * (pS4 + z * pS5)))));
  const q = 1 + z * (qS1 + z * (qS2 + z * (qS3 + z * qS4)));
  const r = p / q;
  const w = r * s + c;
  return 2.0 * (df + w);
}

const atanHi = [4.63647609000806093515e-1, 7.85398163397448278999e-1, 9.82793723247329054082e-1, 1.570796326794896558];
const atanLo = [2.26987774529616870924e-17, 3.06161699786838301793e-17, 1.39033110312309984516e-17, 6.12323399573676603587e-17];
const aT = [
  3.33333333333329318027e-1, -1.99999999998764832476e-1, 1.42857142725034663711e-1, -1.1111110405462355788e-1, 9.09088713343650656196e-2, -7.69187620504482999495e-2,
  6.66107313738753120669e-2, -5.83357013379057348645e-2, 4.97687799461593236017e-2, -3.6531572744216915527e-2, 1.62858201153657823623e-2,
];

export function atan(x) {
  x = +x;
  const hx = hiWord(x);
  const ix = hx & 0x7fffffff;
  let id;
  if (ix >= 0x44100000) {
    if (ix > 0x7ff00000 || (ix === 0x7ff00000 && loWord(x) !== 0)) return x + x;
    if (hx > 0) return atanHi[3] + atanLo[3];
    return -atanHi[3] - atanLo[3];
  }
  if (ix < 0x3fdc0000) {
    if (ix < 0x3e400000) return x;
    id = -1;
  } else {
    x = Math.abs(x);
    if (ix < 0x3ff30000) {
      if (ix < 0x3fe60000) {
        id = 0;
        x = (2.0 * x - 1) / (2.0 + x);
      } else {
        id = 1;
        x = (x - 1) / (x + 1);
      }
    } else if (ix < 0x40038000) {
      id = 2;
      x = (x - 1.5) / (1 + 1.5 * x);
    } else {
      id = 3;
      x = -1.0 / x;
    }
  }
  let z = x * x;
  const w = z * z;
  const s1 = z * (aT[0] + w * (aT[2] + w * (aT[4] + w * (aT[6] + w * (aT[8] + w * aT[10])))));
  const s2 = w * (aT[1] + w * (aT[3] + w * (aT[5] + w * (aT[7] + w * aT[9]))));
  if (id < 0) return x - x * (s1 + s2);
  z = atanHi[id] - (x * (s1 + s2) - atanLo[id] - x);
  return hx < 0 ? -z : z;
}

export function atan2(y, x) {
  y = +y;
  x = +x;
  const tiny = 1.0e-300;
  const piO4 = 7.85398163397448279e-1;
  const piO2 = 1.570796326794896558;
  const pi = 3.141592653589793116;
  const piLo = 1.2246467991473531772e-16;
  const hx = hiWord(x);
  const lx = loWord(x);
  const ix = hx & 0x7fffffff;
  const hy = hiWord(y);
  const ly = loWord(y);
  const iy = hy & 0x7fffffff;
  if ((ix | ((lx | -lx) >>> 31)) >>> 0 > 0x7ff00000 || (iy | ((ly | -ly) >>> 31)) >>> 0 > 0x7ff00000) return x + y;
  if ((((hx - 0x3ff00000) | 0) | lx) === 0) return atan(y);
  let m = ((hy >> 31) & 1) | ((hx >> 30) & 2);
  if ((iy | ly) === 0) {
    switch (m) {
      case 0:
      case 1:
        return y;
      case 2:
        return pi + tiny;
      case 3:
        return -pi - tiny;
    }
  }
  if ((ix | lx) === 0) return hy < 0 ? -piO2 - tiny : piO2 + tiny;
  if (ix === 0x7ff00000) {
    if (iy === 0x7ff00000) {
      switch (m) {
        case 0:
          return piO4 + tiny;
        case 1:
          return -piO4 - tiny;
        case 2:
          return 3.0 * piO4 + tiny;
        case 3:
          return -3.0 * piO4 - tiny;
      }
    } else {
      switch (m) {
        case 0:
          return 0;
        case 1:
          return -0;
        case 2:
          return pi + tiny;
        case 3:
          return -pi - tiny;
      }
    }
  }
  if (iy === 0x7ff00000) return hy < 0 ? -piO2 - tiny : piO2 + tiny;
  let z;
  const k = (iy - ix) >> 20;
  if (k > 60) {
    z = piO2 + 0.5 * piLo;
    m &= 1;
  } else if (hx < 0 && k < -60) {
    z = 0.0;
  } else {
    z = atan(Math.abs(y / x));
  }
  switch (m) {
    case 0:
      return z;
    case 1:
      return -z;
    case 2:
      return pi - (z - piLo);
    default:
      return z - piLo - pi;
  }
}

export function log(x) {
  x = +x;
  const ln2Hi = 6.9314718036912381649e-1;
  const ln2Lo = 1.90821492927058770002e-10;
  const two54 = 1.8014398509481984e16;
  const Lg1 = 6.66666666666673513e-1;
  const Lg2 = 3.999999999940941908e-1;
  const Lg3 = 2.857142874366239149e-1;
  const Lg4 = 2.222219843214978396e-1;
  const Lg5 = 1.818357216161805012e-1;
  const Lg6 = 1.531383769920937332e-1;
  const Lg7 = 1.479819860511658591e-1;
  let hx = hiWord(x);
  const lx = loWord(x);
  let k = 0;
  if (hx < 0x00100000) {
    if (((hx & 0x7fffffff) | lx) === 0) return -Infinity;
    if (hx < 0) return NaN;
    k -= 54;
    x *= two54;
    hx = hiWord(x);
  }
  if (hx >= 0x7ff00000) return x + x;
  k += (hx >> 20) - 1023;
  hx &= 0x000fffff;
  let i = (hx + 0x95f64) & 0x100000;
  x = withHiWord(x, hx | (i ^ 0x3ff00000));
  k += i >> 20;
  const f = x - 1.0;
  if ((0x000fffff & (2 + hx)) < 3) {
    if (f === 0) {
      if (k === 0) return 0;
      const dk = k;
      return dk * ln2Hi + dk * ln2Lo;
    }
    const R = f * f * (0.5 - 0.33333333333333333 * f);
    if (k === 0) return f - R;
    const dk = k;
    return dk * ln2Hi - (R - dk * ln2Lo - f);
  }
  const s = f / (2.0 + f);
  const dk = k;
  const z = s * s;
  i = hx - 0x6147a;
  const w = z * z;
  const j = 0x6b851 - hx;
  const t1 = w * (Lg2 + w * (Lg4 + w * Lg6));
  const t2 = z * (Lg1 + w * (Lg3 + w * (Lg5 + w * Lg7)));
  i |= j;
  const R = t2 + t1;
  if (i > 0) {
    const hfsq = 0.5 * f * f;
    if (k === 0) return f - (hfsq - s * (hfsq + R));
    return dk * ln2Hi - (hfsq - (s * (hfsq + R) + dk * ln2Lo) - f);
  }
  if (k === 0) return f - s * (f - R);
  return dk * ln2Hi - (s * (f - R) - dk * ln2Lo - f);
}

function ieee754Pow(x, y) {
  const two53 = 9007199254740992.0;
  const huge = 1.0e300;
  const tiny = 1.0e-300;
  const L1 = 5.99999999999994648725e-1;
  const L2 = 4.28571428578550184252e-1;
  const L3 = 3.33333329818377432918e-1;
  const L4 = 2.72728123808534006489e-1;
  const L5 = 2.30660745775561754067e-1;
  const L6 = 2.06975017800338417784e-1;
  const P1 = 1.66666666666666019037e-1;
  const P2 = -2.77777777770155933842e-3;
  const P3 = 6.61375632143793436117e-5;
  const P4 = -1.6533902205465251539e-6;
  const P5 = 4.13813679705723846039e-8;
  const lg2 = 6.93147180559945286227e-1;
  const lg2H = 6.93147182464599609375e-1;
  const lg2L = -1.90465429995776804525e-9;
  const ovt = 8.0085662595372944372e-17;
  const cp = 9.61796693925975554329e-1;
  const cpH = 9.61796700954437255859e-1;
  const cpL = -7.02846165095275826516e-9;
  const ivln2 = 1.442695040888963387;
  const ivln2H = 1.44269502162933349609;
  const ivln2L = 1.92596299112661746302e-8;
  const bp = [1.0, 1.5];
  const dpH = [0.0, 5.84962487220764160156e-1];
  const dpL = [0.0, 1.35003920212974897128e-8];

  const hx = hiWord(x);
  const lx = loWord(x);
  const hy = hiWord(y);
  const ly = loWord(y);
  let ix = hx & 0x7fffffff;
  const iy = hy & 0x7fffffff;

  if ((iy | ly) === 0) return 1;
  if (ix > 0x7ff00000 || (ix === 0x7ff00000 && lx !== 0) || iy > 0x7ff00000 || (iy === 0x7ff00000 && ly !== 0)) return x + y;

  let yisint = 0;
  if (hx < 0) {
    if (iy >= 0x43400000) {
      yisint = 2;
    } else if (iy >= 0x3ff00000) {
      const k = (iy >> 20) - 0x3ff;
      if (k > 20) {
        const j = ly >>> (52 - k);
        if (j << (52 - k) >>> 0 === ly) yisint = 2 - (j & 1);
      } else if (ly === 0) {
        const j = iy >> (20 - k);
        if (j << (20 - k) === iy) yisint = 2 - (j & 1);
      }
    }
  }

  if (ly === 0) {
    if (iy === 0x7ff00000) {
      if (((ix - 0x3ff00000) | lx) === 0) return y - y;
      if (ix >= 0x3ff00000) return hy >= 0 ? y : 0;
      return hy < 0 ? -y : 0;
    }
    if (iy === 0x3ff00000) {
      if (hy < 0) return 1 / x;
      return x;
    }
    if (hy === 0x40000000) return x * x;
    if (hy === 0x3fe00000) {
      if (hx >= 0) return Math.sqrt(x);
    }
  }

  let ax = Math.abs(x);
  if (lx === 0) {
    if (ix === 0x7ff00000 || ix === 0 || ix === 0x3ff00000) {
      let z = ax;
      if (hy < 0) z = 1 / z;
      if (hx < 0) {
        if (((ix - 0x3ff00000) | yisint) === 0) {
          z = (z - z) / (z - z);
        } else if (yisint === 1) {
          z = -z;
        }
      }
      return z;
    }
  }

  let n = (hx >>> 31) - 1;
  if ((n | yisint) === 0) return (x - x) / (x - x);
  let s = 1.0;
  if ((n | (yisint - 1)) === 0) s = -1.0;

  let t1;
  let t2;
  if (iy > 0x41e00000) {
    if (iy > 0x43f00000) {
      if (ix <= 0x3fefffff) return hy < 0 ? huge * huge : tiny * tiny;
      if (ix >= 0x3ff00000) return hy > 0 ? huge * huge : tiny * tiny;
    }
    if (ix < 0x3fefffff) return hy < 0 ? s * huge * huge : s * tiny * tiny;
    if (ix > 0x3ff00000) return hy > 0 ? s * huge * huge : s * tiny * tiny;
    const t = ax - 1;
    const w = t * t * (0.5 - t * (0.3333333333333333333333 - t * 0.25));
    const u = ivln2H * t;
    const v = t * ivln2L - w * ivln2;
    t1 = withLoWord(u + v, 0);
    t2 = v - (t1 - u);
  } else {
    n = 0;
    if (ix < 0x00100000) {
      ax *= two53;
      n -= 53;
      ix = hiWord(ax);
    }
    n += (ix >> 20) - 0x3ff;
    const j = ix & 0x000fffff;
    ix = j | 0x3ff00000;
    let k;
    if (j <= 0x3988e) {
      k = 0;
    } else if (j < 0xbb67a) {
      k = 1;
    } else {
      k = 0;
      n += 1;
      ix -= 0x00100000;
    }
    ax = withHiWord(ax, ix);
    let u = ax - bp[k];
    let v = 1 / (ax + bp[k]);
    const ss = u * v;
    const sH = withLoWord(ss, 0);
    let tH = fromWords(((ix >> 1) | 0x20000000) + 0x00080000 + (k << 18), 0);
    let tL = ax - (tH - bp[k]);
    const sL = v * (u - sH * tH - sH * tL);
    let s2 = ss * ss;
    let r = s2 * s2 * (L1 + s2 * (L2 + s2 * (L3 + s2 * (L4 + s2 * (L5 + s2 * L6)))));
    r += sL * (sH + ss);
    s2 = sH * sH;
    tH = withLoWord(3.0 + s2 + r, 0);
    tL = r - (tH - 3.0 - s2);
    u = sH * tH;
    v = sL * tH + tL * ss;
    const pH = withLoWord(u + v, 0);
    const pL = v - (pH - u);
    const zH = cpH * pH;
    const zL = cpL * pH + pL * cp + dpL[k];
    const t = n;
    t1 = withLoWord(zH + zL + dpH[k] + t, 0);
    t2 = zL - (t1 - t - dpH[k] - zH);
  }

  const y1 = withLoWord(y, 0);
  let pL = (y - y1) * t1 + y * t2;
  let pH = y1 * t1;
  let z = pL + pH;
  let j = hiWord(z);
  let i = loWord(z) | 0;
  if (j >= 0x40900000) {
    if (((j - 0x40900000) | i) !== 0) return s * huge * huge;
    if (pL + ovt > z - pH) return s * huge * huge;
  } else if ((j & 0x7fffffff) >= 0x4090cc00) {
    if ((((j - 0xc090cc00) | 0) | i) !== 0) return s * tiny * tiny;
    if (pL <= z - pH) return s * tiny * tiny;
  }
  i = j & 0x7fffffff;
  let k = (i >> 20) - 0x3ff;
  n = 0;
  if (i > 0x3fe00000) {
    n = (j + (0x00100000 >> (k + 1))) | 0;
    k = ((n & 0x7fffffff) >> 20) - 0x3ff;
    const t = fromWords(n & ~(0x000fffff >> k), 0);
    n = ((n & 0x000fffff) | 0x00100000) >> (20 - k);
    if (j < 0) n = -n;
    pH -= t;
  }
  let t = withLoWord(pL + pH, 0);
  const u = t * lg2H;
  const v = (pL - (t - pH)) * lg2 + t * lg2L;
  z = u + v;
  const w = v - (z - u);
  t = z * z;
  t1 = z - t * (P1 + t * (P2 + t * (P3 + t * (P4 + t * P5))));
  const r = (z * t1) / (t1 - 2) - (w + z * w);
  z = 1 - (r - z);
  j = hiWord(z);
  j = (j + (n << 20)) | 0;
  if (j >> 20 <= 0) {
    z = scalbn(z, n);
  } else {
    z = withHiWord(z, (hiWord(z) + (n << 20)) | 0);
  }
  return s * z;
}

// Math.pow: fdlibm's pow after the two cases in which ECMAScript differs
// from C.
export function pow(x, y) {
  x = +x;
  y = +y;
  if (Number.isNaN(y)) return NaN;
  if ((x === 1 || x === -1) && (y === Infinity || y === -Infinity)) return NaN;
  return ieee754Pow(x, y);
}

export const functions = { sin, cos, acos, atan2, log, pow };
