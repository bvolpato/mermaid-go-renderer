#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = []
# ///
"""Emits internal/js/crmath/linux_libm_pow_tables.go: the tables of the pow
function of Arm Optimized Routines.

V8 calls std::pow of the C library for Math.pow. On Linux that is the pow of
glibc, and glibc 2.28 and later has the pow of Arm Optimized Routines. The
function is in linux_libm_pow.go of the package crmath. This script takes the
tables from the Arm source, which has the license MIT OR Apache-2.0 WITH
LLVM-exception. It does not read glibc.

The argument is the directory `math` of a checkout of
https://github.com/ARM-software/optimized-routines at the tag v23.01. The
script runs the C preprocessor on pow_log_data.c and exp_data.c, which selects
the tables of 128 entries, and reads the two initializers. It does not compile
or run the C files.

Run it from tools/reference: uv run --script gen-jsmath-linux-libm.py <math dir>
"""
import re
import struct
import subprocess
import sys
from pathlib import Path

OUT = Path(__file__).resolve().parents[2] / "internal/js/crmath/linux_libm_pow_tables.go"

HEADER = """// Ported from math/pow_log_data.c and math/exp_data.c of Arm Optimized
// Routines v23.01 (https://github.com/ARM-software/optimized-routines).
// tools/reference/gen-jsmath-linux-libm.py made this file. See
// linux_libm_pow.go.
//
// Copyright (c) 2018, Arm Limited.
// SPDX-License-Identifier: MIT OR Apache-2.0 WITH LLVM-exception
//
// The MIT license of Arm Optimized Routines:
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package crmath

"""

HEX_FLOAT = r"-?0x[0-9a-fA-F]+(?:\.[0-9a-fA-F]*)?p[-+]?\d+"


def preprocess(root, name):
    """Returns the text of the initializer of the variable in a data file."""
    text = subprocess.run(
        ["cc", "-E", "-P", "-I", str(root), str(root / name)], check=True, capture_output=True, text=True
    ).stdout
    m = re.search(r"const struct \w+ __\w+ = \{(.*)\};", text, re.S)
    if not m:
        sys.exit("no initializer in " + name)
    return m.group(1)


def number(expr):
    """Evaluates a constant of the tables: a hexadecimal float, or a product or
    a quotient of one with a power of two. The result is exact."""
    expr = expr.strip().replace("(1 << 7)", "128")
    m = re.fullmatch(r"(%s)(?:\s*([*/])\s*(-?\d+))?" % HEX_FLOAT, expr)
    if not m:
        sys.exit("cannot read the constant " + expr)
    value = float.fromhex(m.group(1))
    if m.group(2):
        scale = int(m.group(3))
        if abs(scale) & (abs(scale) - 1):
            sys.exit("the scale of " + expr + " is not a power of two")
        value = value * scale if m.group(2) == "*" else value / scale
    return value


def field(body, name):
    """Returns the text of a field of an initializer: a scalar or a list."""
    m = re.search(r"\.%s = (\{.*?\n\}|[^,{]+)," % re.escape(name), body, re.S)
    if not m:
        sys.exit("no field " + name)
    return m.group(1)


def items(text):
    """Splits the text of a list of constants."""
    return [t for t in text.strip("{}").replace("\n", " ").split(",") if t.strip()]


def lit(value):
    """A Go literal with the exact bits of a float64."""
    if struct.unpack("<d", struct.pack("<d", value))[0] != value:
        sys.exit("not a float64")
    return value.hex()


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    root = Path(sys.argv[1])
    log = preprocess(root, "pow_log_data.c")
    exp = preprocess(root, "exp_data.c")

    poly = [number(t) for t in items(field(log, "poly"))]
    rows = re.findall(r"\{(%s), 0, (%s), (%s)\}" % (HEX_FLOAT, HEX_FLOAT, HEX_FLOAT), field(log, "tab"))
    exp_poly = [number(t) for t in items(field(exp, "poly"))]
    exp_tab = [int(t, 16) for t in items(field(exp, "tab"))]
    if len(poly) != 7 or len(rows) != 128 or len(exp_poly) != 4 or len(exp_tab) != 256:
        sys.exit("the tables do not have the sizes of the tables of 128 entries")

    out = HEADER
    out += "// The constants of __pow_log_data: log(2) in two parts.\nconst (\n"
    out += "\tlinuxPowLn2hi = %s\n" % lit(number(field(log, "ln2hi")))
    out += "\tlinuxPowLn2lo = %s\n)\n\n" % lit(number(field(log, "ln2lo")))
    out += "// linuxPowLogPoly is __pow_log_data.poly: the coefficients of log1p(r) - r\n"
    out += "// from the term r^2, scaled to match the scaling during evaluation.\n"
    out += "var linuxPowLogPoly = [7]float64{\n"
    out += "".join("\t%s,\n" % lit(v) for v in poly)
    out += "}\n\n"
    out += "// linuxPowLogTab is __pow_log_data.tab. An entry is for a number c near the\n"
    out += "// center of a subinterval of [0x1.69555p-1, 0x1.69555p0): invc is 1/c, logc\n"
    out += "// is log(c) with the last bits rounded away, and logctail is the rest of\n"
    out += "// log(c).\n"
    out += "var linuxPowLogTab = [128]struct{ invc, logc, logctail float64 }{\n"
    out += "".join("\t{%s},\n" % ", ".join(lit(float.fromhex(v)) for v in row) for row in rows)
    out += "}\n\n"
    out += "// The constants of __exp_data: 128/log(2), and -log(2)/128 in two parts.\nconst (\n"
    out += "\tlinuxExpInvLn2N   = %s\n" % lit(number(field(exp, "invln2N")))
    out += "\tlinuxExpNegLn2hiN = %s\n" % lit(number(field(exp, "negln2hiN")))
    out += "\tlinuxExpNegLn2loN = %s\n)\n\n" % lit(number(field(exp, "negln2loN")))
    out += "// linuxExpPoly is __exp_data.poly: the coefficients of exp(r) - 1 - r from\n"
    out += "// the term r^2 to the term r^5.\n"
    out += "var linuxExpPoly = [4]float64{\n"
    out += "".join("\t%s,\n" % lit(v) for v in exp_poly)
    out += "}\n\n"
    out += "// linuxExpTab is __exp_data.tab: two words for each k from 0 to 127, with\n"
    out += "// 2^(k/128) ~= H(k) (1 + T(k)). The first word is the bits of T(k). The\n"
    out += "// second word is the bits of H(k) minus k << 45.\n"
    out += "var linuxExpTab = [256]uint64{\n"
    out += "".join(
        "\t%s,\n" % ", ".join("0x%016x" % w for w in exp_tab[i : i + 2]) for i in range(0, len(exp_tab), 2)
    )
    out += "}\n"
    OUT.write_text(out)
    subprocess.run(["gofmt", "-w", str(OUT)], check=True)
    print(OUT)


main()
