// C program of check-jsmath-glibc.mjs. It calls pow and tanh of the C
// library that it runs with, so that a script can compare them with the
// browser and with the package crmath.
//
//   jsmath-glibc eval pow|tanh IN OUT
//       the results for the arguments of the file IN
//   jsmath-glibc gen pow|tanh N SEED IN OUT
//       N random arguments, and the results for them
//
// The files have little-endian float64 numbers. A NaN result is the
// canonical NaN. "eval pow" has the special cases of math::pow of V8 before
// the call of pow, as Math.pow has. "gen pow" calls pow only.
#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static uint64_t state;

// splitmix64
static uint64_t u64(void) {
  state += 0x9e3779b97f4a7c15ULL;
  uint64_t z = state;
  z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9ULL;
  z = (z ^ (z >> 27)) * 0x94d049bb133111ebULL;
  return z ^ (z >> 31);
}

static double frombits(uint64_t b) {
  double d;
  memcpy(&d, &b, 8);
  return d;
}

static uint64_t bits(double d) {
  uint64_t b;
  memcpy(&b, &d, 8);
  return d != d ? 0x7ff8000000000000ULL : b;
}

static double unit(void) { return (double)(u64() >> 11) * 0x1p-53; }
static double uniform(double lo, double hi) { return lo + unit() * (hi - lo); }
static double sign(double d) { return (u64() & 1) ? -d : d; }

// A positive number with a random mantissa and a binary exponent from lo to
// hi.
static double pexp(int lo, int hi) {
  uint64_t e = lo + u64() % (hi - lo + 1) + 1023;
  return frombits(e << 52 | u64() >> 12);
}

static double subnormal(void) { return frombits(u64() >> (12 + u64() % 52)); }

// A number that is at most span float64 steps away from c.
static double near(double c, int span) {
  return frombits(bits(c) + (int64_t)(u64() % (2 * span + 1)) - span);
}

static void pow_args(long i, double *x, double *y) {
  switch (i % 14) {
  case 0:
    *x = pexp(-5, 5);
    *y = uniform(-25, 25);
    break;
  case 1: {
    // A result near the overflow or the underflow limit.
    int e = (int)(u64() % 2046) - 1022;
    *x = frombits((uint64_t)(e + 1023) << 52 | u64() >> 12);
    *y = uniform(-1100, 1050) / (e + 0.5);
    break;
  }
  case 2:
    *x = frombits(u64());
    *y = frombits(u64());
    break;
  case 3:
    *x = pexp(-1022, 1023);
    *y = sign(pexp(-70, 12));
    break;
  case 4:
    *x = subnormal();
    *y = uniform(-2, 2);
    break;
  case 5:
    *x = sign(pexp(-3, 3));
    *y = (double)((int)(u64() % 121) - 60) + (double)(u64() % 2) / 2;
    break;
  case 6:
    *x = pexp(-40, 40);
    *y = uniform(-30, 30);
    break;
  case 7:
  case 8: {
    // A subnormal result, or a result near the overflow limit.
    *x = pexp(-3, 3);
    double l = log2(*x);
    if (l == 0) l = 1;
    *y = (i % 14 == 7 ? uniform(-1076, -1020) : uniform(1020, 1025)) / l;
    break;
  }
  case 9:
    *x = near(1, 1 << (u64() % 40));
    *y = sign(pexp(0, 62));
    break;
  case 10:
    *x = sign(pexp(-1022, 1023));
    *y = sign(pexp(-80, 80));
    break;
  case 11:
    *x = uniform(0, 1);
    *y = uniform(0, 4);
    break;
  case 12:
    *x = (double)((int)(u64() % 2000001) - 1000000) / 1000.0;
    *y = (double)((int)(u64() % 20001) - 10000) / 100.0;
    break;
  default:
    *x = uniform(0, 300);
    *y = uniform(-4, 4);
  }
}

static double tanh_arg(long i) {
  switch (i % 8) {
  case 0:
    return sign(pexp(-6, 5));
  case 1:
    return sign(pexp(-60, -6));
  case 2:
    return frombits(u64());
  case 3:
    return sign(pexp(5, 10));
  case 4:
    return uniform(-25, 25);
  case 5:
    return sign(near((double)(u64() % 64) * 0.34657359027997264, 1 << (u64() % 16)));
  case 6:
    return uniform(-2, 2);
  default:
    return (double)(int64_t)(u64() % 4500001 - 2250000) / 100000.0;
  }
}

// Math.pow of V8 (math::pow of src/numbers/ieee754.cc).
static double v8_pow(double x, double y) {
  if (isnan(y)) return NAN;
  if (isinf(y) && (x == 1 || x == -1)) return NAN;
  if (y == 2.0) return x * x;
  if (y == 0.5) {
    if (isinf(x)) return INFINITY;
    return sqrt(x + 0.0);
  }
  volatile double vx = x, vy = y;
  return pow(vx, vy);
}

static void put(FILE *f, double d) {
  uint64_t b = bits(d);
  fwrite(&b, 8, 1, f);
}

int main(int argc, char **argv) {
  if (argc < 5) return 2;
  int is_pow = !strcmp(argv[2], "pow");
  if (!is_pow && strcmp(argv[2], "tanh")) return 2;
  if (!strcmp(argv[1], "eval") && argc == 5) {
    FILE *in = fopen(argv[3], "rb"), *out = fopen(argv[4], "wb");
    if (!in || !out) return 1;
    double a[2];
    size_t arity = is_pow ? 2 : 1;
    while (fread(a, 8, arity, in) == arity) {
      volatile double x = a[0];
      put(out, is_pow ? v8_pow(a[0], a[1]) : tanh(x));
    }
    return fclose(out) ? 1 : 0;
  }
  if (!strcmp(argv[1], "gen") && argc == 7) {
    long n = atol(argv[3]);
    state = strtoull(argv[4], 0, 0);
    FILE *in = fopen(argv[5], "wb"), *out = fopen(argv[6], "wb");
    if (!in || !out) return 1;
    for (long i = 0; i < n; i++) {
      if (is_pow) {
        double x, y;
        pow_args(i, &x, &y);
        volatile double vx = x, vy = y;
        fwrite(&x, 8, 1, in);
        fwrite(&y, 8, 1, in);
        put(out, pow(vx, vy));
      } else {
        double x = tanh_arg(i);
        volatile double vx = x;
        fwrite(&x, 8, 1, in);
        put(out, tanh(vx));
      }
    }
    return fclose(in) || fclose(out) ? 1 : 0;
  }
  return 2;
}
