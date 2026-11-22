# Performance

This file is generated. Do not edit it. `node tools/reference/bench.mjs` runs the measurement and writes this file and `docs/performance.json`, which has the same numbers for the website and the README.

The numbers compare `mmdg` with mermaid 12.2.0 in Chromium 156.0.8078.4, for 2144 diagrams of the corpus `testdata/corpus/mermaid-e2e`: the diagrams that both sides rendered on the day of the measurement. Both ran on the same machine (darwin/arm64, 18 CPUs) on 2026-11-21, one diagram at a time. Times are in milliseconds.

## Summary

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| Warm render, median | 13.80 | 1.00 | 14x |
| Warm render, 90th percentile | 33.00 | 3.09 | 11x |
| Warm render, all diagrams, in seconds | 38.84 | 3.29 | 12x |
| First render, median | 132.3 | 31.4 | 4x |
| First render, 90th percentile | 196.2 | 48.5 | 4x |

What each measurement is:

- **Warm render, mermaid**: the fastest of 7 `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.
- **Warm render, mmdg**: the fastest of 7 `mermaid.Render` calls for the diagram, in a process that stays running.
- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.
- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.

The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.

## By diagram type

| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |
|---|---:|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 408 | 16.30 | 1.31 | 3.66 | 12x | 163.5 | 32.3 |
| `classDiagram` | 285 | 23.90 | 1.50 | 5.26 | 16x | 175.9 | 44.2 |
| `er` | 206 | 15.20 | 1.52 | 3.24 | 10x | 158.1 | 35.2 |
| `sequence` | 193 | 9.40 | 0.57 | 1.16 | 16x | 26.2 | 26.7 |
| `requirement` | 172 | 16.50 | 1.32 | 3.02 | 12x | 135.9 | 34.9 |
| `stateDiagram` | 144 | 21.70 | 1.28 | 2.57 | 17x | 164.4 | 28.6 |
| `gitGraph` | 135 | 9.90 | 0.22 | 0.43 | 45x | 47.2 | 24.0 |
| `block` | 58 | 15.70 | 0.45 | 1.12 | 35x | 40.5 | 26.3 |
| `gantt` | 52 | 4.00 | 0.23 | 0.55 | 17x | 17.4 | 23.7 |
| `xychart` | 48 | 4.90 | 0.35 | 0.54 | 14x | 18.3 | 25.0 |
| `swimlane` | 46 | 29.40 | 2.88 | 24.29 | 10x | 73.4 | 30.7 |
| `agentflow` | 38 | 17.90 | 1.48 | 2.91 | 12x | 197.8 | 29.2 |
| `mindmap` | 33 | 15.40 | 0.61 | 7.76 | 25x | 47.8 | 35.6 |
| `c4` | 32 | 6.60 | 0.69 | 1.69 | 10x | 28.5 | 34.9 |
| `usecase` | 32 | 20.80 | 1.57 | 2.54 | 13x | 155.4 | 32.5 |
| `timeline` | 30 | 5.90 | 1.16 | 1.35 | 5x | 21.6 | 31.7 |
| `treemap` | 26 | 4.50 | 0.40 | 0.70 | 11x | 37.8 | 37.3 |
| `pie` | 24 | 2.40 | 0.10 | 0.22 | 24x | 36.8 | 27.1 |
| `venn` | 21 | 3.60 | 0.55 | 0.75 | 7x | 14.5 | 23.7 |
| `architecture` | 20 | 18.00 | 1.23 | 3.31 | 15x | 131.8 | 24.1 |
| `journey` | 18 | 5.00 | 0.24 | 0.54 | 21x | 16.9 | 28.7 |
| `sankey` | 18 | 3.10 | 0.10 | 0.18 | 31x | 15.2 | 20.5 |
| `treeView` | 15 | 2.90 | 0.14 | 0.32 | 21x | 44.8 | 28.3 |
| `quadrantChart` | 14 | 4.70 | 0.15 | 0.29 | 31x | 11.7 | 18.5 |
| `ishikawa` | 13 | 4.30 | 0.14 | 0.46 | 31x | 14.4 | 29.2 |
| `cynefin` | 12 | 3.70 | 0.19 | 0.28 | 19x | 39.9 | 29.5 |
| `kanban` | 11 | 5.40 | 0.22 | 0.27 | 25x | 18.5 | 29.7 |
| `wardley` | 10 | 3.90 | 0.25 | 0.37 | 16x | 43.5 | 20.1 |
| `eventmodeling` | 6 | 3.70 | 0.21 | 0.31 | 18x | 56.5 | 41.5 |
| `packet` | 6 | 2.80 | 0.07 | 0.19 | 40x | 33.6 | 18.1 |
| `radar` | 6 | 2.90 | 0.08 | 0.20 | 36x | 36.6 | 23.9 |
| `railroad` | 5 | 6.30 | 0.25 | 0.50 | 25x | 41.9 | 24.6 |
| `info` | 3 | 1.60 | 0.02 | 0.02 | 80x | 30.5 | 17.3 |
| `railroadEbnf` | 2 | 6.80 | 0.21 | 0.21 | 32x | 44.2 | 24.3 |
| `railroadAbnf` | 1 | 6.20 | 0.20 | 0.20 | 31x | 41.7 | 24.5 |
| `railroadPeg` | 1 | 7.20 | 0.29 | 0.29 | 25x | 45.7 | 25.1 |
| **All** | 2144 | 13.80 | 1.00 | 3.09 | 14x | 132.3 | 31.4 |

The columns have the median of the diagrams of the type, except where the heading says otherwise.

## Command-line tools

The table compares `mmdg` with `mmdc` (mermaid-cli 12.2.0). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the fastest of 9 runs, after 3 runs of each tool that do not count. `mmdc` starts a browser in each run.

| Input | Output | mmdc | mmdg | Ratio |
|---|---|---:|---:|---:|
| Flowchart, 8 nodes | SVG | 1233 | 61.1 | 20x |
| Flowchart, 8 nodes | PNG | 1302 | 65.5 | 20x |
| Sequence diagram, 4 actors | SVG | 909 | 53.3 | 17x |
| Sequence diagram, 4 actors | PNG | 922 | 68.1 | 14x |
| Class diagram, 5 classes | SVG | 1215 | 74.7 | 16x |
| Class diagram, 5 classes | PNG | 1267 | 64.4 | 20x |
