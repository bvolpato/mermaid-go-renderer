// The fdlibm functions of V8 (v8/src/base/ieee754.cc at the tag 14.6.202) in
// JavaScript: sin, cos, tan, atan and asin.
//
// JavaScript rounds after each operation. A C++ compiler can contract a
// multiplication and an addition into one fused multiply-add
// (-ffp-contract=on, the default of clang). The Node.js build of Homebrew for
// darwin/arm64 has such contractions, and its Math.sin differs from the
// result of the source code in the last bit for about 1 argument in 130. The
// functions in this file give the result of the source code without
// contraction on every platform. The golden generators for internal/rough use
// them in place of the native Math functions, so the golden files do not
// depend on the build of Node.js.
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
// The original source code covered by the above license above has been
// modified significantly by Google Inc.
// Copyright 2016 the V8 project authors. All rights reserved.

const view = new DataView(new ArrayBuffer(8));
const highWord = (x) => {
  view.setFloat64(0, x);
  return view.getInt32(0);
};
const lowWord = (x) => {
  view.setFloat64(0, x);
  return view.getUint32(4);
};
const setLowWord = (x, lo) => {
  view.setFloat64(0, x);
  view.setUint32(4, lo >>> 0);
  return view.getFloat64(0);
};
const setHighWord = (x, hi) => {
  view.setFloat64(0, x);
  view.setInt32(0, hi | 0);
  return view.getFloat64(0);
};
const insertWords = (hi, lo) => {
  view.setInt32(0, hi | 0);
  view.setUint32(4, lo >>> 0);
  return view.getFloat64(0);
};
// scalbn(x, n) for a result that is a normal number.
const scalbn = (x, n) => x * insertWords((n + 1023) << 20, 0);

const two_over_pi = [
  0xa2f983, 0x6e4e44, 0x1529fc, 0x2757d1, 0xf534dd, 0xc0db62, 0x95993c, 0x439041, 0xfe5163, 0xabdebb, 0xc561b7, 0x246e3a, 0x424dd2, 0xe00649, 0x2eea09, 0xd1921c, 0xfe1deb, 0x1cb129, 0xa73ee8, 0x8235f5, 0x2ebb44, 0x84e99c, 0x7026b4, 0x5f7e41, 0x3991d6, 0x398353, 0x39f49c, 0x845f8b, 0xbdf928, 0x3b1ff8, 0x97ffde, 0x05980f, 0xef2f11, 0x8b5a0a, 0x6d1f6d, 0x367ecf, 0x27cb09, 0xb74f46, 0x3f669e, 0x5fea2d, 0x7527ba, 0xc7ebe5, 0xf17b3d, 0x0739f7, 0x8a5292, 0xea6bfb, 0x5fb11f, 0x8d5d08, 0x560330, 0x46fc7b, 0x6babf0, 0xcfbc20, 0x9af436, 0x1da9e3, 0x91615e, 0xe61b08, 0x659985, 0x5f14a0, 0x68408d, 0xffd880, 0x4d7327, 0x310606, 0x1556ca, 0x73a8c9, 0x60e27b, 0xc08c6b,
];
const npio2_hw = [
  0x3ff921fb, 0x400921fb, 0x4012d97c, 0x401921fb, 0x401f6a7a, 0x4022d97c, 0x4025fdbb, 0x402921fb, 0x402c463a, 0x402f6a7a, 0x4031475c, 0x4032d97c, 0x40346b9c, 0x4035fdbb, 0x40378fdb, 0x403921fb, 0x403ab41b, 0x403c463a, 0x403dd85a, 0x403f6a7a, 0x40407e4c, 0x4041475c, 0x4042106c, 0x4042d97c, 0x4043a28c, 0x40446b9c, 0x404534ac, 0x4045fdbb, 0x4046c6cb, 0x40478fdb, 0x404858eb, 0x404921fb,
];

// __ieee754_rem_pio2: returns [n, y0, y1].
function rem_pio2(x) {
  const half = 5.0e-1;
  const two24 = 1.6777216e7;
  const invpio2 = 6.36619772367581382433e-1;
  const pio2_1 = 1.57079632673412561417;
  const pio2_1t = 6.07710050650619224932e-11;
  const pio2_2 = 6.0771005063039659766e-11;
  const pio2_2t = 2.02226624879595063154e-21;
  const pio2_3 = 2.0222662487111664558e-21;
  const pio2_3t = 8.47842766036889956997e-32;
  let z, w, t, r, fn, y0, y1, n;
  const hx = highWord(x);
  const ix = hx & 0x7fffffff;
  if (ix <= 0x3fe921fb) return [0, x, 0];
  if (ix < 0x4002d97c) {
    if (hx > 0) {
      z = x - pio2_1;
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
    z = x + pio2_1;
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
    t = Math.abs(x);
    n = (t * invpio2 + half) | 0;
    fn = n;
    r = t - fn * pio2_1;
    w = fn * pio2_1t;
    if (n < 32 && ix !== npio2_hw[n - 1]) {
      y0 = r - w;
    } else {
      const j = ix >> 20;
      y0 = r - w;
      let high = highWord(y0) >>> 0;
      let i = j - ((high >>> 20) & 0x7ff);
      if (i > 16) {
        t = r;
        w = fn * pio2_2;
        r = t - w;
        w = fn * pio2_2t - (t - r - w);
        y0 = r - w;
        high = highWord(y0) >>> 0;
        i = j - ((high >>> 20) & 0x7ff);
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
  z = setLowWord(0, lowWord(x));
  const e0 = (ix >> 20) - 1046;
  z = setHighWord(z, ix - (e0 << 20));
  const tx = [0, 0, 0];
  for (let i = 0; i < 2; i++) {
    tx[i] = z | 0;
    z = (z - tx[i]) * two24;
  }
  tx[2] = z;
  let nx = 3;
  while (tx[nx - 1] === 0) nx--;
  const y = [0, 0, 0];
  n = kernel_rem_pio2(tx, y, e0, nx, 2);
  if (hx < 0) return [-n, -y[0], -y[1]];
  return [n, y[0], y[1]];
}

const init_jk = [2, 3, 4, 6];
const PIo2 = [1.57079625129699707031, 7.54978941586159635335e-8, 5.39030252995776476554e-15, 3.28200341580791294123e-22, 1.27065575308067607349e-29, 1.22933308981111328932e-36, 2.73370053816464559624e-44, 2.16741683877804819444e-51];

function kernel_rem_pio2(x, y, e0, nx, prec) {
  const two24 = 1.6777216e7;
  const twon24 = 5.9604644775390625e-8;
  let jz, jx, jv, jp, jk, carry, n, i, j, k, m, q0, ih, z, fw;
  const iq = new Array(20).fill(0);
  const f = new Array(20).fill(0);
  const fq = new Array(20).fill(0);
  const q = new Array(20).fill(0);

  jk = init_jk[prec];
  jp = jk;

  jx = nx - 1;
  jv = ((e0 - 3) / 24) | 0;
  if (jv < 0) jv = 0;
  q0 = e0 - 24 * (jv + 1);

  j = jv - jx;
  m = jx + jk;
  for (i = 0; i <= m; i++, j++) f[i] = j < 0 ? 0 : two_over_pi[j];

  for (i = 0; i <= jk; i++) {
    for (j = 0, fw = 0.0; j <= jx; j++) fw += x[j] * f[jx + i - j];
    q[i] = fw;
  }

  jz = jk;
  for (;;) {
    // recompute:
    for (i = 0, j = jz, z = q[jz]; j > 0; i++, j--) {
      fw = (twon24 * z) | 0;
      iq[i] = (z - two24 * fw) | 0;
      z = q[j - 1] + fw;
    }

    z = scalbn(z, q0);
    z -= 8.0 * Math.floor(z * 0.125);
    n = z | 0;
    z -= n;
    ih = 0;
    if (q0 > 0) {
      i = iq[jz - 1] >> (24 - q0);
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
      carry = 0;
      for (i = 0; i < jz; i++) {
        j = iq[i];
        if (carry === 0) {
          if (j !== 0) {
            carry = 1;
            iq[i] = 0x1000000 - j;
          }
        } else {
          iq[i] = 0xffffff - j;
        }
      }
      if (q0 > 0) {
        switch (q0) {
          case 1:
            iq[jz - 1] &= 0x7fffff;
            break;
          case 2:
            iq[jz - 1] &= 0x3fffff;
            break;
        }
      }
      if (ih === 2) {
        z = 1 - z;
        if (carry !== 0) z -= scalbn(1, q0);
      }
    }

    if (z === 0) {
      j = 0;
      for (i = jz - 1; i >= jk; i--) j |= iq[i];
      if (j === 0) {
        for (k = 1; jk >= k && iq[jk - k] === 0; k++) {
          // k = no. of terms needed
        }
        for (i = jz + 1; i <= jz + k; i++) {
          f[jx + i] = two_over_pi[jv + i];
          for (j = 0, fw = 0.0; j <= jx; j++) fw += x[j] * f[jx + i - j];
          q[i] = fw;
        }
        jz += k;
        continue;
      }
    }
    break;
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
      fw = (twon24 * z) | 0;
      iq[jz] = (z - two24 * fw) | 0;
      jz += 1;
      q0 += 24;
      iq[jz] = fw | 0;
    } else {
      iq[jz] = z | 0;
    }
  }

  fw = scalbn(1, q0);
  for (i = jz; i >= 0; i--) {
    q[i] = fw * iq[i];
    fw *= twon24;
  }

  for (i = jz; i >= 0; i--) {
    for (fw = 0.0, k = 0; k <= jp && k <= jz - i; k++) fw += PIo2[k] * q[i + k];
    fq[jz - i] = fw;
  }

  // prec is 2.
  fw = 0.0;
  for (i = jz; i >= 0; i--) fw += fq[i];
  y[0] = ih === 0 ? fw : -fw;
  fw = fq[0] - fw;
  for (i = 1; i <= jz; i++) fw += fq[i];
  y[1] = ih === 0 ? fw : -fw;
  return n & 7;
}

function kernel_cos(x, y) {
  const one = 1.0;
  const C1 = 4.16666666666666019037e-2;
  const C2 = -1.38888888888741095749e-3;
  const C3 = 2.48015872894767294178e-5;
  const C4 = -2.75573143513906633035e-7;
  const C5 = 2.0875723212981748279e-9;
  const C6 = -1.13596475577881948265e-11;
  const ix = highWord(x) & 0x7fffffff;
  if (ix < 0x3e400000) {
    if ((x | 0) === 0) return one;
  }
  const z = x * x;
  const r = z * (C1 + z * (C2 + z * (C3 + z * (C4 + z * (C5 + z * C6)))));
  if (ix < 0x3fd33333) {
    return one - (0.5 * z - (z * r - x * y));
  }
  let qx;
  if (ix > 0x3fe90000) {
    qx = 0.28125;
  } else {
    qx = insertWords(ix - 0x00200000, 0);
  }
  const iz = 0.5 * z - qx;
  const a = one - qx;
  return a - (iz - (z * r - x * y));
}

function kernel_sin(x, y, iy) {
  const half = 5.0e-1;
  const S1 = -1.66666666666666324348e-1;
  const S2 = 8.33333333332248946124e-3;
  const S3 = -1.98412698298579493134e-4;
  const S4 = 2.75573137070700676789e-6;
  const S5 = -2.50507602534068634195e-8;
  const S6 = 1.58969099521155010221e-10;
  const ix = highWord(x) & 0x7fffffff;
  if (ix < 0x3e400000) {
    if ((x | 0) === 0) return x;
  }
  const z = x * x;
  const v = z * x;
  const r = S2 + z * (S3 + z * (S4 + z * (S5 + z * S6)));
  if (iy === 0) {
    return x + v * (S1 + z * r);
  }
  return x - (z * (half * y - v * r) - y - v * S1);
}

const T = [
  3.33333333333334091986e-1, 1.33333333333201242699e-1, 5.39682539762260521377e-2, 2.18694882948595424599e-2, 8.86323982359930005737e-3, 3.59207910759131235356e-3, 1.45620945432529025516e-3, 5.88041240820264096874e-4, 2.46463134818469906812e-4, 7.817944429395570923e-5, 7.14072491382608190305e-5, -1.85586374855275456654e-5,
  2.59073051863633712884e-5,
];

function kernel_tan(x, y, iy) {
  const one = 1.0;
  const pio4 = 7.85398163397448278999e-1;
  const pio4lo = 3.06161699786838301793e-17;
  let z, r, v, w, s, a, t;
  const hx = highWord(x);
  const ix = hx & 0x7fffffff;
  if (ix < 0x3e300000) {
    if ((x | 0) === 0) {
      const low = lowWord(x);
      if (((ix | low) | (iy + 1)) === 0) {
        return one / Math.abs(x);
      }
      if (iy === 1) return x;
      w = x + y;
      z = setLowWord(w, 0);
      v = y - (z - x);
      a = -one / w;
      t = setLowWord(a, 0);
      s = one + t * z;
      return t + a * (s + t * v);
    }
  }
  if (ix >= 0x3fe59428) {
    if (hx < 0) {
      x = -x;
      y = -y;
    }
    z = pio4 - x;
    w = pio4lo - y;
    x = z + w;
    y = 0.0;
  }
  z = x * x;
  w = z * z;
  r = T[1] + w * (T[3] + w * (T[5] + w * (T[7] + w * (T[9] + w * T[11]))));
  v = z * (T[2] + w * (T[4] + w * (T[6] + w * (T[8] + w * (T[10] + w * T[12])))));
  s = z * x;
  r = y + z * (s * (r + v) + y);
  r += T[0] * s;
  w = x + r;
  if (ix >= 0x3fe59428) {
    v = iy;
    return (1 - ((hx >> 30) & 2)) * (v - 2.0 * (x - ((w * w) / (w + v) - r)));
  }
  if (iy === 1) return w;
  z = setLowWord(w, 0);
  v = r - (z - x);
  a = -1.0 / w;
  t = setLowWord(a, 0);
  s = 1.0 + t * z;
  return t + a * (s + t * v);
}

export function sin(x) {
  x = +x;
  const ix = highWord(x) & 0x7fffffff;
  if (ix <= 0x3fe921fb) return kernel_sin(x, 0, 0);
  if (ix >= 0x7ff00000) return x - x;
  const [n, y0, y1] = rem_pio2(x);
  switch (n & 3) {
    case 0:
      return kernel_sin(y0, y1, 1);
    case 1:
      return kernel_cos(y0, y1);
    case 2:
      return -kernel_sin(y0, y1, 1);
    default:
      return -kernel_cos(y0, y1);
  }
}

export function cos(x) {
  x = +x;
  const ix = highWord(x) & 0x7fffffff;
  if (ix <= 0x3fe921fb) return kernel_cos(x, 0);
  if (ix >= 0x7ff00000) return x - x;
  const [n, y0, y1] = rem_pio2(x);
  switch (n & 3) {
    case 0:
      return kernel_cos(y0, y1);
    case 1:
      return -kernel_sin(y0, y1, 1);
    case 2:
      return -kernel_cos(y0, y1);
    default:
      return kernel_sin(y0, y1, 1);
  }
}

export function tan(x) {
  x = +x;
  const ix = highWord(x) & 0x7fffffff;
  if (ix <= 0x3fe921fb) return kernel_tan(x, 0, 1);
  if (ix >= 0x7ff00000) return x - x;
  const [n, y0, y1] = rem_pio2(x);
  return kernel_tan(y0, y1, 1 - ((n & 1) << 1));
}

const atanhi = [4.63647609000806093515e-1, 7.85398163397448278999e-1, 9.82793723247329054082e-1, 1.570796326794896558];
const atanlo = [2.26987774529616870924e-17, 3.06161699786838301793e-17, 1.39033110312309984516e-17, 6.12323399573676603587e-17];
const aT = [3.33333333333329318027e-1, -1.99999999998764832476e-1, 1.42857142725034663711e-1, -1.1111110405462355788e-1, 9.09088713343650656196e-2, -7.69187620504482999495e-2, 6.66107313738753120669e-2, -5.83357013379057348645e-2, 4.97687799461593236017e-2, -3.6531572744216915527e-2, 1.62858201153657823623e-2];

export function atan(x) {
  x = +x;
  const one = 1.0;
  const huge = 1.0e300;
  let w, s1, s2, z, id;
  const hx = highWord(x);
  const ix = hx & 0x7fffffff;
  if (ix >= 0x44100000) {
    const low = lowWord(x);
    if (ix > 0x7ff00000 || (ix === 0x7ff00000 && low !== 0)) return x + x;
    if (hx > 0) return atanhi[3] + atanlo[3];
    return -atanhi[3] - atanlo[3];
  }
  if (ix < 0x3fdc0000) {
    if (ix < 0x3e400000) {
      if (huge + x > one) return x;
    }
    id = -1;
  } else {
    x = Math.abs(x);
    if (ix < 0x3ff30000) {
      if (ix < 0x3fe60000) {
        id = 0;
        x = (2.0 * x - one) / (2.0 + x);
      } else {
        id = 1;
        x = (x - one) / (x + one);
      }
    } else {
      if (ix < 0x40038000) {
        id = 2;
        x = (x - 1.5) / (one + 1.5 * x);
      } else {
        id = 3;
        x = -1.0 / x;
      }
    }
  }
  z = x * x;
  w = z * z;
  s1 = z * (aT[0] + w * (aT[2] + w * (aT[4] + w * (aT[6] + w * (aT[8] + w * aT[10])))));
  s2 = w * (aT[1] + w * (aT[3] + w * (aT[5] + w * (aT[7] + w * aT[9]))));
  if (id < 0) {
    return x - x * (s1 + s2);
  }
  z = atanhi[id] - (x * (s1 + s2) - atanlo[id] - x);
  return hx < 0 ? -z : z;
}

export function asin(x) {
  x = +x;
  const one = 1.0;
  const huge = 1.0e300;
  const pio2_hi = 1.570796326794896558;
  const pio2_lo = 6.12323399573676603587e-17;
  const pio4_hi = 7.85398163397448278999e-1;
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
  let t = 0;
  let w, p, q, c, r, s;
  const hx = highWord(x);
  const ix = hx & 0x7fffffff;
  if (ix >= 0x3ff00000) {
    const lx = lowWord(x);
    if (((ix - 0x3ff00000) | lx) === 0) {
      return x * pio2_hi + x * pio2_lo;
    }
    return NaN;
  } else if (ix < 0x3fe00000) {
    if (ix < 0x3e400000) {
      if (huge + x > one) return x;
    } else {
      t = x * x;
    }
    p = t * (pS0 + t * (pS1 + t * (pS2 + t * (pS3 + t * (pS4 + t * pS5)))));
    q = one + t * (qS1 + t * (qS2 + t * (qS3 + t * qS4)));
    w = p / q;
    return x + x * w;
  }
  w = one - Math.abs(x);
  t = w * 0.5;
  p = t * (pS0 + t * (pS1 + t * (pS2 + t * (pS3 + t * (pS4 + t * pS5)))));
  q = one + t * (qS1 + t * (qS2 + t * (qS3 + t * qS4)));
  s = Math.sqrt(t);
  if (ix >= 0x3fef3333) {
    w = p / q;
    t = pio2_hi - (2.0 * (s + s * w) - pio2_lo);
  } else {
    w = setLowWord(s, 0);
    c = (t - w * w) / (s + w);
    r = p / q;
    p = 2.0 * s * r - (pio2_lo - 2.0 * c);
    q = pio4_hi - 2.0 * w;
    t = pio4_hi - (p - q);
  }
  return hx > 0 ? t : -t;
}

// install replaces the functions of the Math object. It returns the native
// functions.
export function install() {
  const native = { sin: Math.sin, cos: Math.cos, tan: Math.tan, atan: Math.atan, asin: Math.asin };
  Math.sin = sin;
  Math.cos = cos;
  Math.tan = tan;
  Math.atan = atan;
  Math.asin = asin;
  return native;
}
