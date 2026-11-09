#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = ["mpmath"]
# ///
"""Emits golden cases for the correctly rounded functions of internal/js:
SinCR, CosCR, TanCR, AsinCR, AtanCR, AcosCR and Atan2CR. mpmath computes the
values with 600 bits.

Run it from tools/reference: uv run --script gen-d3-trig-cr.py
"""
import base64
import gzip
import json
import math
import random
import struct
from pathlib import Path

import mpmath as mp

mp.mp.prec = 600
rnd = random.Random(0x51C0)


def ulps(x, k):
    for _ in range(abs(k)):
        x = math.nextafter(x, math.inf if k > 0 else -math.inf)
    return x


xs = [0.0, -0.0, 1.0, -1.0, 0.5, 2.0, 1e22, -1e22, 1e300, 1.7976931348623157e308, 5e-324, 2.2250738585072014e-308,
      math.pi, math.pi / 2, math.pi / 4, 3 * (math.pi / 2), 2 * math.pi, 100.0, 1e6, 2.0**-27, 2.0**-26, 2.0**-26.5,
      # The float64 that is closest to a multiple of pi/2.
      6381956970095103.0 * 2.0**797]
for k in range(-40, 41):
    for d in range(-3, 4):
        xs.append(ulps(k * (math.pi / 2), d))
for _ in range(2500):
    xs.append(rnd.uniform(-2 * math.pi, 2 * math.pi))
for _ in range(800):
    xs.append(rnd.uniform(-1000, 1000))
for _ in range(900):
    xs.append(math.ldexp(rnd.uniform(0.5, 1), rnd.randint(10, 1024)) * rnd.choice([1, -1]))
for _ in range(300):
    xs.append(math.ldexp(rnd.uniform(0.5, 1), rnd.randint(-1073, -5)) * rnd.choice([1, -1]))
for e in range(-40, 1):
    xs.append(ulps(2.0**e, rnd.randint(-2, 2)))
# Angles as the diagrams have them: fractions of a turn.
for n in range(1, 41):
    for i in range(0, n + 1):
        xs.append((2 * i * math.pi) / n - math.pi / 2)



def odd(f, x):
    # mpmath has no negative zero. An odd function keeps the sign of a zero.
    return x if x == 0 else float(f(mp.mpf(x)))


def pack(values):
    return base64.b64encode(struct.pack("<%dd" % len(values), *values)).decode()


trig = []
for x in xs:
    trig += [x, odd(mp.sin, x), float(mp.cos(mp.mpf(x))), odd(mp.tan, x)]

# Arguments of asin: the interval [-1, 1], dense near the ends and near zero.
us = [0.0, -0.0, 1.0, -1.0, 0.5, -0.5, 2.0**-30, 1 - 2.0**-53, -1 + 2.0**-53, 2.0**-600]
for _ in range(1500):
    us.append(rnd.uniform(-1, 1))
for _ in range(300):
    us.append(rnd.choice([1, -1]) * (1 - math.ldexp(rnd.uniform(0.5, 1), rnd.randint(-52, -1))))
    us.append(rnd.choice([1, -1]) * math.ldexp(rnd.uniform(0.5, 1), rnd.randint(-60, -1)))
asin = []
for u in us:
    asin += [u, odd(mp.asin, u), float(mp.acos(mp.mpf(u)))]

# Arguments of atan.
ts = [0.0, -0.0, 1.0, -1.0, 1e300, -1e300, 2.0**-40, 1e16, math.inf, -math.inf]
for _ in range(1500):
    ts.append(rnd.uniform(-50, 50))
for _ in range(600):
    ts.append(rnd.choice([1, -1]) * math.ldexp(rnd.uniform(0.5, 1), rnd.randint(-200, 200)))
atan = []
for t in ts:
    atan += [t, odd(mp.atan, t)]



def atan2(y, x):
    """Math.atan2 with correct rounding, by the rules of ECMAScript."""
    if y == 0:
        if x > 0 or (x == 0 and math.copysign(1, x) > 0):
            return y
        return math.copysign(float(mp.pi), y)
    if x == 0:
        return math.copysign(float(mp.pi / 2), y)
    if math.isinf(y):
        if math.isinf(x):
            return math.copysign(float(mp.pi / 4 if x > 0 else 3 * mp.pi / 4), y)
        return math.copysign(float(mp.pi / 2), y)
    if math.isinf(x):
        return math.copysign(0.0 if x > 0 else float(mp.pi), y)
    return float(mp.atan2(mp.mpf(y), mp.mpf(x)))


special = [0.0, -0.0, 1.0, -1.0, math.inf, -math.inf, 5e-324, 1.7976931348623157e308, 2.5]
pairs = [(y, x) for y in special for x in special]
for _ in range(2500):
    pairs.append((rnd.uniform(-500, 500), rnd.uniform(-500, 500)))
for _ in range(700):
    pairs.append((rnd.choice([1, -1]) * math.ldexp(rnd.uniform(0.5, 1), rnd.randint(-1070, 1023)),
                  rnd.choice([1, -1]) * math.ldexp(rnd.uniform(0.5, 1), rnd.randint(-1070, 1023))))
for _ in range(300):
    # Coordinates as the diagrams have them.
    pairs.append((float(rnd.randint(-300, 300)), float(rnd.randint(-300, 300))))
atan2s = []
for y, x in pairs:
    atan2s += [y, x, atan2(y, x)]

out = Path(__file__).resolve().parents[2] / "internal/js/testdata/d3_trig_cr.json.gz"
with gzip.GzipFile(out, "wb", mtime=0) as f:
    f.write(json.dumps({"trig": pack(trig), "asin": pack(asin), "atan": pack(atan), "atan2": pack(atan2s)}).encode())
print(out, len(xs), "trig,", len(us), "asin and acos,", len(ts), "atan,", len(pairs), "atan2 cases;", out.stat().st_size // 1024, "KiB")
