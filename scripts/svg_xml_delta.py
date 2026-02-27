#!/usr/bin/env python3
"""
Compute XML-structure deltas between mmdc and mmdg SVG outputs.

Default input locations:
  /tmp/*_mmdc.svg
  /tmp/*_mmdg.svg

Outputs:
  /tmp/svg_xml_delta_report.txt
  /tmp/svg_xml_delta_report.csv
  /tmp/svg_xml_delta_details/<name>.txt
"""

from __future__ import annotations

import csv
import glob
import os
import re
import xml.etree.ElementTree as ET
from collections import Counter, defaultdict
from dataclasses import dataclass
from pathlib import Path


REPORT_TXT = Path("/tmp/svg_xml_delta_report.txt")
REPORT_CSV = Path("/tmp/svg_xml_delta_report.csv")
DETAILS_DIR = Path("/tmp/svg_xml_delta_details")


def local_name(tag: str) -> str:
    if "}" in tag:
        return tag.split("}", 1)[1]
    return tag


def parse_float(value: str) -> float | None:
    try:
        return float(value.strip())
    except Exception:
        return None


def normalize_attr_value(value: str) -> str:
    v = value.strip()
    # Normalize excessive float precision to improve meaningful diffing.
    def _fmt(m: re.Match[str]) -> str:
        raw = m.group(0)
        try:
            num = float(raw)
        except ValueError:
            return raw
        return f"{num:.4f}".rstrip("0").rstrip(".")

    return re.sub(r"-?\d+\.\d+|-?\d+", _fmt, v)


@dataclass
class SVGStats:
    element_counts: Counter[str]
    attr_presence: Counter[str]
    attr_values: Counter[str]
    text_values: Counter[str]
    root_width: float | None
    root_height: float | None
    root_viewbox: tuple[float, float, float, float] | None


def extract_viewbox(root: ET.Element) -> tuple[float, float, float, float] | None:
    vb = root.attrib.get("viewBox", "").strip()
    if not vb:
        return None
    parts = re.split(r"[,\s]+", vb)
    if len(parts) != 4:
        return None
    vals = [parse_float(p) for p in parts]
    if any(v is None for v in vals):
        return None
    return vals[0], vals[1], vals[2], vals[3]  # type: ignore[index]


def collect_stats(path: Path) -> SVGStats:
    tree = ET.parse(path)
    root = tree.getroot()

    element_counts: Counter[str] = Counter()
    attr_presence: Counter[str] = Counter()
    attr_values: Counter[str] = Counter()
    text_values: Counter[str] = Counter()

    for elem in root.iter():
        tag = local_name(elem.tag)
        element_counts[tag] += 1
        for k, v in sorted(elem.attrib.items()):
            key = f"{tag}@{k}"
            attr_presence[key] += 1
            attr_values[f"{key}={normalize_attr_value(v)}"] += 1
        text = (elem.text or "").strip()
        if text:
            text_values[text] += 1

    width = parse_float(root.attrib.get("width", ""))
    height = parse_float(root.attrib.get("height", ""))
    vb = extract_viewbox(root)

    return SVGStats(
        element_counts=element_counts,
        attr_presence=attr_presence,
        attr_values=attr_values,
        text_values=text_values,
        root_width=width,
        root_height=height,
        root_viewbox=vb,
    )


def counter_abs_delta(a: Counter[str], b: Counter[str]) -> int:
    keys = set(a.keys()) | set(b.keys())
    return sum(abs(a.get(k, 0) - b.get(k, 0)) for k in keys)


def counter_intersection(a: Counter[str], b: Counter[str]) -> int:
    keys = set(a.keys()) | set(b.keys())
    return sum(min(a.get(k, 0), b.get(k, 0)) for k in keys)


def jaccard_multiset(a: Counter[str], b: Counter[str]) -> float:
    keys = set(a.keys()) | set(b.keys())
    inter = sum(min(a.get(k, 0), b.get(k, 0)) for k in keys)
    union = sum(max(a.get(k, 0), b.get(k, 0)) for k in keys)
    if union == 0:
        return 1.0
    return inter / union


def fmt_vb(vb: tuple[float, float, float, float] | None) -> str:
    if vb is None:
        return "-"
    return ",".join(f"{x:.3f}" for x in vb)


def root_dim_delta(a: float | None, b: float | None) -> float:
    if a is None or b is None:
        return 0.0
    return abs(a - b)


def common_pairs() -> list[tuple[str, Path, Path]]:
    mmdc_files = sorted(glob.glob("/tmp/*_mmdc.svg"))
    out: list[tuple[str, Path, Path]] = []
    for mmdc in mmdc_files:
        name = Path(mmdc).name.replace("_mmdc.svg", "")
        mmdg = Path(f"/tmp/{name}_mmdg.svg")
        if mmdg.exists():
            out.append((name, Path(mmdc), mmdg))
    return out


def write_detail(name: str, mmdc_path: Path, mmdg_path: Path, c_stats: SVGStats, g_stats: SVGStats) -> None:
    DETAILS_DIR.mkdir(parents=True, exist_ok=True)
    detail_path = DETAILS_DIR / f"{name}.txt"

    lines: list[str] = []
    lines.append(f"name={name}")
    lines.append(f"mmdc={mmdc_path}")
    lines.append(f"mmdg={mmdg_path}")
    lines.append("")
    lines.append(f"root_width:  mmdc={c_stats.root_width} mmdg={g_stats.root_width}")
    lines.append(f"root_height: mmdc={c_stats.root_height} mmdg={g_stats.root_height}")
    lines.append(f"root_viewBox: mmdc={fmt_vb(c_stats.root_viewbox)} mmdg={fmt_vb(g_stats.root_viewbox)}")
    lines.append("")

    all_tags = sorted(set(c_stats.element_counts.keys()) | set(g_stats.element_counts.keys()))
    lines.append("element_count_delta:")
    for tag in all_tags:
        cv = c_stats.element_counts.get(tag, 0)
        gv = g_stats.element_counts.get(tag, 0)
        if cv != gv:
            lines.append(f"  {tag}: mmdc={cv} mmdg={gv} delta={gv-cv}")
    lines.append("")

    all_attr_presence = sorted(set(c_stats.attr_presence.keys()) | set(g_stats.attr_presence.keys()))
    lines.append("attribute_presence_delta (top 80 by abs delta):")
    pres_rows = []
    for k in all_attr_presence:
        cv = c_stats.attr_presence.get(k, 0)
        gv = g_stats.attr_presence.get(k, 0)
        if cv != gv:
            pres_rows.append((abs(gv - cv), k, cv, gv))
    for _, k, cv, gv in sorted(pres_rows, reverse=True)[:80]:
        lines.append(f"  {k}: mmdc={cv} mmdg={gv} delta={gv-cv}")
    lines.append("")

    all_text = sorted(set(c_stats.text_values.keys()) | set(g_stats.text_values.keys()))
    lines.append("text_value_delta:")
    for t in all_text:
        cv = c_stats.text_values.get(t, 0)
        gv = g_stats.text_values.get(t, 0)
        if cv != gv:
            lines.append(f"  {t!r}: mmdc={cv} mmdg={gv} delta={gv-cv}")

    detail_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    pairs = common_pairs()
    if not pairs:
        print("no /tmp/*_mmdc.svg and /tmp/*_mmdg.svg pairs found")
        return 1

    rows: list[dict[str, object]] = []

    for name, mmdc_path, mmdg_path in pairs:
        c_stats = collect_stats(mmdc_path)
        g_stats = collect_stats(mmdg_path)

        tag_delta = counter_abs_delta(c_stats.element_counts, g_stats.element_counts)
        attr_presence_delta = counter_abs_delta(c_stats.attr_presence, g_stats.attr_presence)
        attr_values_delta = counter_abs_delta(c_stats.attr_values, g_stats.attr_values)
        text_delta = counter_abs_delta(c_stats.text_values, g_stats.text_values)

        tag_similarity = jaccard_multiset(c_stats.element_counts, g_stats.element_counts)
        text_similarity = jaccard_multiset(c_stats.text_values, g_stats.text_values)

        width_delta = root_dim_delta(c_stats.root_width, g_stats.root_width)
        height_delta = root_dim_delta(c_stats.root_height, g_stats.root_height)

        row = {
            "name": name,
            "tag_delta": tag_delta,
            "attr_presence_delta": attr_presence_delta,
            "attr_values_delta": attr_values_delta,
            "text_delta": text_delta,
            "tag_similarity": f"{tag_similarity:.4f}",
            "text_similarity": f"{text_similarity:.4f}",
            "width_delta": f"{width_delta:.3f}",
            "height_delta": f"{height_delta:.3f}",
            "mmdc_viewbox": fmt_vb(c_stats.root_viewbox),
            "mmdg_viewbox": fmt_vb(g_stats.root_viewbox),
            "detail_path": str(DETAILS_DIR / f"{name}.txt"),
        }
        rows.append(row)

        write_detail(name, mmdc_path, mmdg_path, c_stats, g_stats)

    # Higher score means more different.
    def severity(r: dict[str, object]) -> float:
        return (
            float(r["tag_delta"]) * 4
            + float(r["attr_presence_delta"]) * 1.5
            + float(r["attr_values_delta"]) * 0.2
            + float(r["text_delta"]) * 3
            + float(r["width_delta"]) * 0.1
            + float(r["height_delta"]) * 0.1
        )

    rows.sort(key=severity, reverse=True)

    with REPORT_CSV.open("w", newline="", encoding="utf-8") as fp:
        fieldnames = [
            "name",
            "tag_delta",
            "attr_presence_delta",
            "attr_values_delta",
            "text_delta",
            "tag_similarity",
            "text_similarity",
            "width_delta",
            "height_delta",
            "mmdc_viewbox",
            "mmdg_viewbox",
            "detail_path",
        ]
        writer = csv.DictWriter(fp, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)

    summary_lines = [
        f"pairs={len(rows)}",
        f"csv={REPORT_CSV}",
        f"details_dir={DETAILS_DIR}",
        "",
        "top_deltas:",
    ]
    for r in rows[:10]:
        summary_lines.append(
            "  {name}: tag_delta={tag_delta} attr_presence_delta={attr_presence_delta} "
            "text_delta={text_delta} tag_similarity={tag_similarity} text_similarity={text_similarity}".format(**r)
        )
    REPORT_TXT.write_text("\n".join(summary_lines) + "\n", encoding="utf-8")

    print(str(REPORT_TXT))
    print(str(REPORT_CSV))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
