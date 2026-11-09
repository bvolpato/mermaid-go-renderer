# Performance

This file is generated. Do not edit it. `node tools/reference/bench.mjs` runs the measurement and writes this file and `docs/performance.json`, which has the same numbers for the website and the README.

The numbers compare `mmdg` with mermaid 12.2.0 in Chromium 156.0.8078.4, for 2076 diagrams of the corpus `testdata/corpus/mermaid-e2e`: the diagrams that both sides rendered on the day of the measurement. Both ran on the same machine (darwin/arm64, 18 CPUs) on 2026-11-09, one diagram at a time. Times are in milliseconds.

## Summary

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| Warm render, median | 11.50 | 0.88 | 13x |
| Warm render, 90th percentile | 25.80 | 2.55 | 10x |
| Warm render, all diagrams, in seconds | 29.00 | 2.79 | 10x |
| First render, median | 130.4 | 36.7 | 4x |
| First render, 90th percentile | 189.5 | 79.0 | 2x |

What each measurement is:

- **Warm render, mermaid**: the fastest of 7 `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.
- **Warm render, mmdg**: the fastest of 7 `mermaid.Render` calls for the diagram, in a process that stays running.
- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.
- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.

The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.

## By diagram type

| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |
|---|---:|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 404 | 13.90 | 1.34 | 3.93 | 10x | 163.8 | 32.1 |
| `classDiagram` | 227 | 16.00 | 1.19 | 3.19 | 13x | 166.0 | 76.9 |
| `er` | 206 | 13.50 | 1.59 | 3.39 | 8x | 158.1 | 33.6 |
| `sequence` | 193 | 8.60 | 0.48 | 1.01 | 18x | 26.2 | 36.4 |
| `requirement` | 170 | 13.30 | 1.06 | 2.51 | 13x | 135.9 | 42.6 |
| `stateDiagram` | 144 | 21.00 | 1.06 | 2.42 | 20x | 164.4 | 34.7 |
| `gitGraph` | 135 | 7.90 | 0.19 | 0.32 | 42x | 47.2 | 30.2 |
| `block` | 58 | 9.40 | 0.39 | 0.95 | 24x | 40.5 | 76.8 |
| `gantt` | 52 | 3.00 | 0.18 | 0.42 | 17x | 17.4 | 32.6 |
| `xychart` | 48 | 3.80 | 0.25 | 0.38 | 15x | 18.3 | 73.9 |
| `swimlane` | 46 | 21.70 | 2.47 | 19.73 | 9x | 73.4 | 34.0 |
| `agentflow` | 38 | 13.80 | 1.48 | 3.50 | 9x | 197.8 | 77.7 |
| `mindmap` | 33 | 13.90 | 0.55 | 5.61 | 25x | 47.8 | 36.1 |
| `c4` | 32 | 3.90 | 0.68 | 1.68 | 6x | 28.5 | 80.5 |
| `timeline` | 30 | 3.70 | 0.94 | 1.22 | 4x | 21.6 | 71.7 |
| `usecase` | 28 | 12.30 | 1.26 | 1.92 | 10x | 153.4 | 76.8 |
| `treemap` | 26 | 2.20 | 0.27 | 0.64 | 8x | 37.8 | 82.2 |
| `pie` | 24 | 2.10 | 0.10 | 0.22 | 21x | 36.8 | 32.9 |
| `venn` | 21 | 2.20 | 0.46 | 0.64 | 5x | 14.5 | 71.9 |
| `architecture` | 20 | 14.30 | 1.20 | 3.00 | 12x | 131.8 | 76.0 |
| `journey` | 18 | 4.80 | 0.26 | 0.52 | 18x | 16.9 | 34.0 |
| `sankey` | 18 | 2.60 | 0.07 | 0.13 | 37x | 15.2 | 39.4 |
| `treeView` | 15 | 1.70 | 0.12 | 0.21 | 14x | 44.8 | 70.9 |
| `quadrantChart` | 14 | 4.10 | 0.12 | 0.24 | 34x | 11.7 | 27.4 |
| `ishikawa` | 13 | 3.70 | 0.12 | 0.42 | 31x | 14.4 | 32.3 |
| `cynefin` | 12 | 3.20 | 0.18 | 0.24 | 18x | 39.9 | 23.3 |
| `kanban` | 11 | 4.10 | 0.22 | 0.29 | 19x | 18.5 | 31.5 |
| `wardley` | 10 | 2.00 | 0.19 | 0.28 | 11x | 43.5 | 72.2 |
| `eventmodeling` | 6 | 2.60 | 0.28 | 0.31 | 9x | 56.5 | 25.8 |
| `packet` | 6 | 2.60 | 0.06 | 0.15 | 43x | 33.6 | 30.6 |
| `radar` | 6 | 2.40 | 0.11 | 0.20 | 22x | 36.6 | 30.3 |
| `railroad` | 5 | 6.00 | 0.26 | 0.47 | 23x | 41.9 | 28.4 |
| `info` | 3 | 1.30 | 0.02 | 0.03 | 65x | 30.5 | 29.9 |
| `railroadEbnf` | 2 | 6.20 | 0.24 | 0.24 | 26x | 44.2 | 29.8 |
| `railroadAbnf` | 1 | 5.30 | 0.20 | 0.20 | 26x | 41.7 | 30.2 |
| `railroadPeg` | 1 | 7.20 | 0.29 | 0.29 | 25x | 45.7 | 28.9 |
| **All** | 2076 | 11.50 | 0.88 | 2.55 | 13x | 130.4 | 36.7 |

The columns have the median of the diagrams of the type, except where the heading says otherwise.

## Command-line tools

The table compares `mmdg` with `mmdc` (mermaid-cli 12.0.0). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the fastest of 9 runs, after 3 runs of each tool that do not count. `mmdc` starts a browser in each run.

| Input | Output | mmdc | mmdg | Ratio |
|---|---|---:|---:|---:|
| Flowchart, 8 nodes | SVG | 729 | 46.8 | 16x |
| Flowchart, 8 nodes | PNG | 711 | 52.5 | 14x |
| Sequence diagram, 4 actors | SVG | 489 | 42.4 | 12x |
| Sequence diagram, 4 actors | PNG | 538 | 53.1 | 10x |
| Class diagram, 5 classes | SVG | 675 | 52.9 | 13x |
| Class diagram, 5 classes | PNG | 810 | 49.9 | 16x |
