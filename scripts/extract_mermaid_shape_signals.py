#!/usr/bin/env python3
"""
Extract anonymized Mermaid usage signals from markdown files.

This script intentionally outputs only aggregate counts (diagram types, arrows,
shape token categories) and never prints raw Mermaid block content.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from collections import Counter
from pathlib import Path


FENCE_START_RE = re.compile(r"^(```|~~~)\s*mermaid(?:-example|js)?\s*$", re.I)
FENCE_END_RE = re.compile(r"^(```|~~~)\s*$")

DIAGRAM_TYPE_RE = re.compile(
    r"^(flowchart|graph|sequenceDiagram|classDiagram|stateDiagram(?:-v2)?|erDiagram|journey|timeline|gantt|pie|mindmap|gitGraph|quadrantChart|xychart-beta|C4Context|sankey(?:-beta)?|zenuml|kanban|packet|block|architecture-beta|radar-beta|treemap-beta|requirementDiagram)\b",
    re.I,
)

FLOWCHART_SHAPE_PATTERNS = {
    "square[]": re.compile(r"\[[^\]]+\]"),
    "double_square[[]]": re.compile(r"\[\[[^\]]+\]\]"),
    "round()": re.compile(r"\([^()]+\)"),
    "double_round(())": re.compile(r"\(\([^()]+\)\)"),
    "diamond{}": re.compile(r"\{[^{}]+\}"),
    "stadium([])": re.compile(r"\(\[[^\]]+\]\)"),
}

EDGE_TOKENS = ["-->", "---", "-.->", "==>", "<-->", "<->", "<|--", "--|>", "*--", "--*", "o--", "--o", "..>", "<.."]
SEQUENCE_ARROWS = ["->>", "-->>", "->", "-->", "-x", "--x"]
CLASS_RELATION_TOKENS = ["<|--", "--|>", "*--", "o--", "..>", "<.."]
ER_CARDINALITY_TOKENS = ["||--o{", "||--|{", "}|..|{", "}|--||", "o{--||"]


def collect_markdown_files(roots: list[str]) -> list[Path]:
    cmd = ["rg", "-l", r"```\s*mermaid|~~~\s*mermaid|```\s*mermaidjs|~~~\s*mermaidjs"]
    for root in roots:
        cmd.extend(["-g", "*.md", root])
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode not in (0, 1):
        raise RuntimeError(result.stderr.strip() or "rg failed")
    return [Path(line.strip()) for line in result.stdout.splitlines() if line.strip()]


def iter_mermaid_blocks(lines: list[str]) -> list[list[str]]:
    blocks: list[list[str]] = []
    i = 0
    while i < len(lines):
        if not FENCE_START_RE.match(lines[i].strip()):
            i += 1
            continue
        i += 1
        block: list[str] = []
        while i < len(lines) and not FENCE_END_RE.match(lines[i].strip()):
            block.append(lines[i])
            i += 1
        if i < len(lines):
            i += 1
        if any(line.strip() for line in block):
            blocks.append(block)
    return blocks


def summarize(files: list[Path]) -> dict[str, object]:
    diagram_types: Counter[str] = Counter()
    flowchart_shapes: Counter[str] = Counter()
    edge_tokens: Counter[str] = Counter()
    sequence_arrows: Counter[str] = Counter()
    class_relations: Counter[str] = Counter()
    er_cardinalities: Counter[str] = Counter()
    block_count = 0

    for fp in files:
        try:
            lines = fp.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError:
            continue

        for block in iter_mermaid_blocks(lines):
            block_count += 1
            first_nonempty = next((line.strip() for line in block if line.strip()), "")
            match = DIAGRAM_TYPE_RE.match(first_nonempty)
            if match:
                diagram = match.group(1)
                if diagram.lower() == "graph":
                    diagram = "flowchart"
                diagram_types[diagram] += 1

            body = "\n".join(block)
            for name, pat in FLOWCHART_SHAPE_PATTERNS.items():
                flowchart_shapes[name] += len(pat.findall(body))
            for token in EDGE_TOKENS:
                edge_tokens[token] += body.count(token)
            for token in SEQUENCE_ARROWS:
                sequence_arrows[token] += body.count(token)
            for token in CLASS_RELATION_TOKENS:
                class_relations[token] += body.count(token)
            for token in ER_CARDINALITY_TOKENS:
                er_cardinalities[token] += body.count(token)

    def non_zero(counter: Counter[str]) -> dict[str, int]:
        items = [(k, v) for k, v in counter.items() if v > 0]
        items.sort(key=lambda kv: (-kv[1], kv[0]))
        return dict(items)

    return {
        "files_scanned": len(files),
        "mermaid_blocks": block_count,
        "diagram_types": non_zero(diagram_types),
        "shape_signals": {
            "flowchart_shapes": non_zero(flowchart_shapes),
            "edge_tokens": non_zero(edge_tokens),
            "sequence_arrows": non_zero(sequence_arrows),
            "class_relation_tokens": non_zero(class_relations),
            "er_cardinality_tokens": non_zero(er_cardinalities),
        },
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "roots",
        nargs="*",
        help="Directories to scan (defaults to ~/dd/dd-source ~/dd/logs-backend ~/dd/dd-go)",
    )
    parser.add_argument("--output", default="", help="Optional output JSON path")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    roots = args.roots or [
        str(Path.home() / "dd" / "dd-source"),
        str(Path.home() / "dd" / "logs-backend"),
        str(Path.home() / "dd" / "dd-go"),
    ]
    files = collect_markdown_files(roots)
    report = summarize(files)

    payload = json.dumps(report, indent=2)
    print(payload)
    if args.output:
        Path(args.output).write_text(payload, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

