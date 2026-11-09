#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# dependencies = []
# ///
"""Emits internal/js/crmath/tables.go: the tables of the double functions of
LLVM libc that the package crmath ports.

The argument is a checkout of the libc directory of LLVM at the revision that
the reference browser has. For Chromium 156.0.8078.4 that is the revision
b8b28272d32b75f0ee71acd2fa1144c8b5b95460 of
https://chromium.googlesource.com/external/github.com/llvm/llvm-project/libc
(llvm_libc_revision in the DEPS file of Chromium).

Run it from tools/reference: uv run --script gen-jsmath-tables.py <libc dir>
"""
import re
import subprocess
import sys
from pathlib import Path

root = sys.argv[1]
out = str(Path(__file__).resolve().parents[2] / "internal/js/crmath/tables.go")
NUM = re.compile(r'-?0x[0-9a-fA-F]+(?:\.[0-9a-fA-F]*)?p[-+]?\d+|-?\d+\.\d*|-?\b\d+\b')
def table(path, name):
    s = open(root + '/' + path).read()
    s = re.sub(r'//[^\n]*', '', s)
    m = re.search(r'\b' + re.escape(name) + r'\b(?:\[[^\]]*\])*\s*=\s*\{', s)
    if not m:
        raise SystemExit('no table ' + name + ' in ' + path)
    i = m.end(); depth = 1
    while depth:
        c = s[i]
        depth += (c == '{') - (c == '}')
        i += 1
    body = s[m.end():i-1]
    return [t for t in NUM.findall(body)]
def lit(t):
    if t.startswith('0x') or t.startswith('-0x'):
        return t
    if '.' in t:
        return t.rstrip('0').rstrip('.') if float(t) != 0 else '0'
    return t
def emit_flat(gname, doc, vals, per=4):
    rows = ['\t' + ', '.join(lit(v) for v in vals[i:i+per]) + ',' for i in range(0, len(vals), per)]
    return '%s\nvar %s = [%d]float64{\n%s\n}\n\n' % (doc, gname, len(vals), '\n'.join(rows))
def emit_rows(gname, doc, vals, width, typ=None):
    n = len(vals) // width
    assert n * width == len(vals), (gname, len(vals), width)
    rows = ['\t{' + ', '.join(lit(v) for v in vals[i*width:(i+1)*width]) + '},' for i in range(n)]
    t = typ or ('[%d]float64' % width)
    return '%s\nvar %s = [%d]%s{\n%s\n}\n\n' % (doc, gname, n, t, '\n'.join(rows))
M = 'src/__support/math/'
go = '''// Ported from libc/src/__support/math of LLVM libc: the tables of the files
// range_reduction_double_common.h, range_reduction_double_fma.h,
// atan_utils.h, exp_constants.h, common_constants.h, log2.h, log1p.h,
// asin_utils.h and cbrt.h. tools/reference/gen-jsmath-tables.py made this
// file at the revision that Chromium 156 has (llvm-libc b8b28272d32b). Do not
// edit it.
//
// The files of LLVM have no copyright line. Each file says:
//
//	Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.
//	See https://llvm.org/LICENSE.txt for license information.
//	SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
//
// This file has the numbers of those files in the syntax of Go. It is a
// changed version of them. LICENSE-LLVM.txt in this directory has the
// license.

package crmath

'''
t = table(M + 'range_reduction_double_common.h', 'ONE_TWENTY_EIGHT_OVER_PI'); assert len(t) == 256
go += emit_rows('crInvPi128', '// crInvPi128 is ONE_TWENTY_EIGHT_OVER_PI: 128/pi in parts of 52 bits, one\n// row for each 16 binary exponents of the argument.', t, 4)
t = table(M + 'range_reduction_double_fma.h', 'SIN_K_PI_OVER_128'); assert len(t) == 512, len(t)
go += emit_rows('crSinKPi128', '// crSinKPi128 is SIN_K_PI_OVER_128: sin(k pi/128) as a low and a high part.', t, 2, 'dd')
t = table(M + 'atan_utils.h', 'ATAN_I'); assert len(t) == 130, len(t)
go += emit_rows('crAtanI', '// crAtanI is ATAN_I: atan(i/64) as a low and a high part.', t, 2, 'dd')
t = table(M + 'exp_constants.h', 'EXP2_MID1'); assert len(t) == 192, len(t)
go += emit_rows('crExp2Mid1', '// crExp2Mid1 is EXP2_MID1: 2^(i/64) as a low, a middle and a high part.', t, 3)
t = table(M + 'exp_constants.h', 'EXP2_MID2'); assert len(t) == 192, len(t)
go += emit_rows('crExp2Mid2', '// crExp2Mid2 is EXP2_MID2: 2^(i/4096) as a low, a middle and a high part.', t, 3)
t = table(M + 'common_constants.h', 'RD'); assert len(t) == 128, len(t)
go += emit_flat('crLogRD', '// crLogRD is RD: the reciprocals r(i) of the log functions.', t)
t = table(M + 'common_constants.h', 'LOG_R_DD'); assert len(t) == 256, len(t)
go += emit_rows('crLogRDD', '// crLogRDD is LOG_R_DD: -log(r(i)) as a low and a high part.', t, 2, 'dd')
t = table(M + 'common_constants.h', 'LOG_COEFFS'); assert len(t) == 6, len(t)
go += emit_flat('crLogCoeffs', '// crLogCoeffs is LOG_COEFFS: the polynomial of log(1 + u) - u.', t, 3)
t = table(M + 'log2.h', 'LOG_R1'); assert len(t) == 256, len(t)
go += emit_rows('crLog2R1', '// crLog2R1 is LOG_R1 of log2.h: -log(r(i)) as a low and a high part.', t, 2, 'dd')
t = table(M + 'log1p.h', 'R1'); assert len(t) == 129, len(t)
go += emit_flat('crLog1pR1', '// crLog1pR1 is R1 of log1p.h: the reciprocals of log1p.', t)
t = table(M + 'log1p.h', 'LOG_R1_DD'); assert len(t) == 258, len(t)
go += emit_rows('crLog1pLogR1', '// crLog1pLogR1 is LOG_R1_DD of log1p.h: -log(r(i)) as a low and a high part.', t, 2, 'dd')
t = table(M + 'log1p.h', 'P_COEFFS'); assert len(t) == 6, len(t)
go += emit_flat('crLog1pCoeffs', '// crLog1pCoeffs is P_COEFFS of log1p.h.', t, 3)
t = table(M + 'asin_utils.h', 'ASIN_COEFFS'); assert len(t) == 108, len(t)
go += emit_rows('crAsinCoeffs', '// crAsinCoeffs is ASIN_COEFFS: the Taylor polynomials of asin(sqrt(u))/sqrt(u)\n// at u = i/32.', t, 12)
t = table(M + 'cbrt.h', 'COEFFS'); assert len(t) == 8, len(t)
go += emit_flat('crCbrtCoeffs', '// crCbrtCoeffs is COEFFS of cbrt.h: a polynomial for x^(-2/3) on [1, 2].', t)
open(out, 'w').write(go.rstrip('\n') + '\n')
subprocess.run(['gofmt', '-w', out], check=True)
print(out)
