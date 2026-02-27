#!/usr/bin/env python3
"""
Compatibility sweep against Mermaid docs examples.

This script fetches Mermaid syntax docs from the upstream repository, extracts
```mermaid``` and ```mermaid-example``` code blocks, and renders each snippet
with both mmdc and mmdg.

Outputs:
  /tmp/mmdg_docs_compat/results.csv
  /tmp/mmdg_docs_compat/summary.txt
  /tmp/mmdg_docs_compat/cases/*.mmd
  /tmp/mmdg_docs_compat/logs/*.log
"""

from __future__ import annotations

import csv
import os
import re
import shlex
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from urllib.request import urlopen


DOC_URLS = [
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/flowchart.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/sequenceDiagram.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/classDiagram.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/stateDiagram.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/entityRelationshipDiagram.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/pie.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/mindmap.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/userJourney.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/timeline.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/gantt.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/requirementDiagram.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/gitgraph.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/c4.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/sankey.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/quadrantChart.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/zenuml.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/block.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/packet.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/kanban.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/architecture.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/radar.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/treemap.md",
    "https://raw.githubusercontent.com/mermaid-js/mermaid/develop/packages/mermaid/src/docs/syntax/xyChart.md",
]

BLOCK_RE = re.compile(r"```(?:mermaid|mermaid-example)\n(.*?)\n```", re.S)


@dataclass
class Case:
    case_id: str
    source_file: str
    index_in_file: int
    body: str


def require_tool(cmd: str) -> None:
    result = subprocess.run(["bash", "-lc", f"command -v {shlex.quote(cmd)}"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if result.returncode != 0:
        raise RuntimeError(f"required tool not found in PATH: {cmd}")


def fetch_text(url: str) -> str:
    with urlopen(url, timeout=30) as resp:
        return resp.read().decode("utf-8", errors="replace")


def extract_cases() -> list[Case]:
    max_per_file = int(os.environ.get("MAX_EXAMPLES_PER_FILE", "12"))
    only_file = os.environ.get("ONLY_FILE", "").strip().lower()

    cases: list[Case] = []
    global_idx = 0
    for url in DOC_URLS:
        file_name = url.rsplit("/", 1)[-1]
        if only_file and only_file != file_name.lower():
            continue
        content = fetch_text(url)
        blocks = BLOCK_RE.findall(content)
        for i, body in enumerate(blocks[:max_per_file], start=1):
            global_idx += 1
            cases.append(
                Case(
                    case_id=f"case_{global_idx:04d}",
                    source_file=file_name,
                    index_in_file=i,
                    body=body.strip() + "\n",
                )
            )
    return cases


def run_cmd(cmd: list[str], log_path: Path, timeout_s: int) -> bool:
    with log_path.open("w", encoding="utf-8") as log_fp:
        try:
            proc = subprocess.run(cmd, stdout=log_fp, stderr=log_fp, timeout=timeout_s)
            return proc.returncode == 0
        except subprocess.TimeoutExpired:
            log_fp.write("\nTIMEOUT\n")
            return False


def main() -> int:
    repo_root = Path(__file__).resolve().parent.parent
    out_root = Path("/tmp/mmdg_docs_compat")
    cases_dir = out_root / "cases"
    logs_dir = out_root / "logs"
    svg_dir = out_root / "svg"
    out_root.mkdir(parents=True, exist_ok=True)
    cases_dir.mkdir(parents=True, exist_ok=True)
    logs_dir.mkdir(parents=True, exist_ok=True)
    svg_dir.mkdir(parents=True, exist_ok=True)

    require_tool("mmdc")
    require_tool("go")

    mmdg_bin = out_root / "mmdg"
    build = subprocess.run(["go", "build", "-o", str(mmdg_bin), "./cmd/mmdg"], cwd=repo_root)
    if build.returncode != 0:
        print("failed to build mmdg", file=sys.stderr)
        return 1

    timeout_s = int(os.environ.get("EXAMPLE_TIMEOUT_SECONDS", "25"))
    cases = extract_cases()
    if not cases:
        print("no cases found")
        return 0

    rows: list[dict[str, str]] = []
    for case in cases:
        mmd_path = cases_dir / f"{case.case_id}.mmd"
        mmd_path.write_text(case.body, encoding="utf-8")

        out_mmdg_svg = svg_dir / f"{case.case_id}_mmdg.svg"
        out_mmdc_svg = svg_dir / f"{case.case_id}_mmdc.svg"

        mmdg_ok = run_cmd(
            [str(mmdg_bin), "-i", str(mmd_path), "-o", str(out_mmdg_svg), "--allowApproximate"],
            logs_dir / f"{case.case_id}_mmdg.log",
            timeout_s,
        )
        mmdc_ok = run_cmd(
            ["mmdc", "-i", str(mmd_path), "-o", str(out_mmdc_svg), "-e", "svg", "-w", "2200", "-H", "1600", "-b", "white", "-q"],
            logs_dir / f"{case.case_id}_mmdc.log",
            timeout_s,
        )

        rows.append(
            {
                "case_id": case.case_id,
                "source_file": case.source_file,
                "index_in_file": str(case.index_in_file),
                "mmdg_status": "ok" if mmdg_ok else "fail",
                "mmdc_status": "ok" if mmdc_ok else "fail",
                "mmd_path": str(mmd_path),
                "mmdg_svg": str(out_mmdg_svg),
                "mmdc_svg": str(out_mmdc_svg),
                "mmdg_log": str(logs_dir / f"{case.case_id}_mmdg.log"),
                "mmdc_log": str(logs_dir / f"{case.case_id}_mmdc.log"),
            }
        )

    csv_path = out_root / "results.csv"
    with csv_path.open("w", newline="", encoding="utf-8") as fp:
        writer = csv.DictWriter(
            fp,
            fieldnames=[
                "case_id",
                "source_file",
                "index_in_file",
                "mmdg_status",
                "mmdc_status",
                "mmd_path",
                "mmdg_svg",
                "mmdc_svg",
                "mmdg_log",
                "mmdc_log",
            ],
        )
        writer.writeheader()
        writer.writerows(rows)

    total = len(rows)
    both_ok = sum(1 for r in rows if r["mmdg_status"] == "ok" and r["mmdc_status"] == "ok")
    mmdg_fail_mmdc_ok = sum(1 for r in rows if r["mmdg_status"] == "fail" and r["mmdc_status"] == "ok")
    mmdg_ok_mmdc_fail = sum(1 for r in rows if r["mmdg_status"] == "ok" and r["mmdc_status"] == "fail")
    both_fail = sum(1 for r in rows if r["mmdg_status"] == "fail" and r["mmdc_status"] == "fail")

    summary_path = out_root / "summary.txt"
    with summary_path.open("w", encoding="utf-8") as fp:
        fp.write(f"total_cases={total}\n")
        fp.write(f"both_ok={both_ok}\n")
        fp.write(f"mmdg_fail_mmdc_ok={mmdg_fail_mmdc_ok}\n")
        fp.write(f"mmdg_ok_mmdc_fail={mmdg_ok_mmdc_fail}\n")
        fp.write(f"both_fail={both_fail}\n")
        fp.write(f"results_csv={csv_path}\n")

    print(f"summary: {summary_path}")
    print(f"results: {csv_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
