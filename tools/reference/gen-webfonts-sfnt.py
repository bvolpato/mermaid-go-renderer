#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = ["fonttools==4.55.3", "brotli==1.1.0"]
# ///
"""Decodes the WOFF2 web fonts of internal/webfonts to sfnt files.

Chromium decodes a WOFF2 web font before it gives the font to Skia. The Go
module has no Brotli decoder, so this script does the same step one time:
it writes each font of internal/webfonts/files as an sfnt file, compressed
with gzip, under internal/font/webfontdata/files with the same relative
path and the extension .sfnt.gz. It also writes manifest.json there, which
has the SHA-256 of each WOFF2 file and of its sfnt file. A test of the Go
package compares the manifest with the embedded files, so a change of a
WOFF2 file without a new run of this script fails.

KaTeX ships the same fonts as TTF files in its npm package. For these
fonts the script packs the original TTF bytes, without a format conversion.
Install the pinned tools/reference dependencies before running this script.

Run:

    uv run tools/reference/gen-webfonts-sfnt.py
"""

import gzip
import hashlib
import io
import json
import pathlib
import subprocess

from fontTools.ttLib import TTFont

ROOT = pathlib.Path(__file__).resolve().parents[2]
SRC = ROOT / "internal" / "webfonts" / "files"
DST = ROOT / "internal" / "font" / "webfontdata" / "files"


def decode(woff2: bytes) -> bytes:
    """Returns the sfnt data of a WOFF2 font.

    fontTools undoes the table transforms of WOFF2 (glyf, loca, hmtx) and
    copies every other table as it is. The time stamps and the bounding
    boxes stay as the font has them.
    """
    font = TTFont(io.BytesIO(woff2), recalcBBoxes=False, recalcTimestamp=False)
    font.flavor = None
    out = io.BytesIO()
    font.save(out, reorderTables=False)
    return out.getvalue()


def pack(data: bytes) -> bytes:
    """Returns gzip data without a time stamp, so that the file is stable."""
    out = io.BytesIO()
    with gzip.GzipFile(fileobj=out, mode="wb", compresslevel=9, mtime=0) as zf:
        zf.write(data)
    return out.getvalue()


def main() -> None:
    assets = json.loads(
        subprocess.check_output(
            ["node", str(ROOT / "tools/reference/mmdc-assets.mjs")], text=True
        )
    )
    katex = pathlib.Path(assets["directory"])
    manifest = {}
    for path in sorted(SRC.rglob("*.woff2")):
        rel = path.relative_to(SRC).as_posix()
        woff2 = path.read_bytes()
        if rel.startswith("katex/"):
            original = katex / rel.removeprefix("katex/")
            if original.read_bytes() != woff2:
                raise ValueError(f"{rel} differs from the mermaid-cli dependency")
            sfnt = original.with_suffix(".ttf").read_bytes()
        else:
            sfnt = decode(woff2)
        target = DST / (rel[: -len(".woff2")] + ".sfnt.gz")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(pack(sfnt))
        manifest["files/" + rel] = {
            "woff2": hashlib.sha256(woff2).hexdigest(),
            "sfnt": hashlib.sha256(sfnt).hexdigest(),
            "size": len(sfnt),
        }
        print(f"{rel}: {len(woff2)} -> {len(sfnt)} bytes")
    text = json.dumps(manifest, indent=1, sort_keys=True) + "\n"
    (DST.parent / "manifest.json").write_text(text)


if __name__ == "__main__":
    main()
