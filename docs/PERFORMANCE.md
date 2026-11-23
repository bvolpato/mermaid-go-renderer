# Performance

This file is generated. Do not edit it. `node tools/reference/bench.mjs` runs the measurement and writes this file and `docs/performance.json`, which has the same numbers for the website and the README.

The numbers compare `mmdg` with mermaid 12.2.0 in Chromium 156.0.8078.4, for 2144 diagrams of the corpus `testdata/corpus/mermaid-e2e`: the diagrams that both sides rendered on the day of the measurement. Both ran on the same machine (darwin/arm64, 18 CPUs) on 2026-11-22, one diagram at a time. Times are in milliseconds.

## Summary

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| Warm render, median | 7.80 | 0.53 | 15x |
| Warm render, 90th percentile | 18.80 | 1.82 | 10x |
| Warm render, all diagrams, in seconds | 21.98 | 1.91 | 12x |
| First render, median | 132.3 | 17.0 | 8x |
| First render, 90th percentile | 196.2 | 23.9 | 8x |

What each measurement is:

- **Warm render, mermaid**: the fastest of 7 `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.
- **Warm render, mmdg**: the fastest of 7 `mermaid.Render` calls for the diagram, in a process that stays running.
- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.
- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.

The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.

## By diagram type

| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |
|---|---:|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 408 | 9.50 | 0.70 | 2.13 | 14x | 163.5 | 17.3 |
| `classDiagram` | 285 | 13.60 | 0.90 | 3.16 | 15x | 175.9 | 24.4 |
| `er` | 206 | 8.50 | 0.87 | 1.91 | 10x | 158.1 | 19.3 |
| `sequence` | 193 | 5.50 | 0.31 | 0.63 | 18x | 26.2 | 16.5 |
| `requirement` | 172 | 8.20 | 0.66 | 1.63 | 12x | 135.9 | 20.7 |
| `stateDiagram` | 144 | 14.10 | 0.68 | 1.57 | 21x | 164.4 | 16.4 |
| `gitGraph` | 135 | 5.10 | 0.14 | 0.23 | 36x | 47.2 | 12.3 |
| `block` | 58 | 6.90 | 0.26 | 0.59 | 27x | 40.5 | 12.2 |
| `gantt` | 52 | 2.00 | 0.13 | 0.32 | 15x | 17.4 | 12.6 |
| `xychart` | 48 | 2.90 | 0.19 | 0.29 | 15x | 18.3 | 13.0 |
| `swimlane` | 46 | 17.20 | 1.73 | 17.25 | 10x | 73.4 | 18.8 |
| `agentflow` | 38 | 11.50 | 0.77 | 1.74 | 15x | 197.8 | 16.3 |
| `mindmap` | 33 | 8.10 | 0.39 | 4.51 | 21x | 47.8 | 16.9 |
| `c4` | 32 | 3.30 | 0.38 | 1.04 | 9x | 28.5 | 16.7 |
| `usecase` | 32 | 13.10 | 0.81 | 1.31 | 16x | 155.4 | 16.3 |
| `timeline` | 30 | 3.70 | 0.72 | 0.85 | 5x | 21.6 | 16.9 |
| `treemap` | 26 | 2.00 | 0.21 | 0.35 | 10x | 37.8 | 19.4 |
| `pie` | 24 | 1.20 | 0.06 | 0.15 | 20x | 36.8 | 14.5 |
| `venn` | 21 | 2.00 | 0.33 | 0.47 | 6x | 14.5 | 11.5 |
| `architecture` | 20 | 9.10 | 0.78 | 2.37 | 12x | 131.8 | 13.8 |
| `journey` | 18 | 2.90 | 0.15 | 0.30 | 19x | 16.9 | 14.9 |
| `sankey` | 18 | 1.30 | 0.05 | 0.10 | 26x | 15.2 | 11.5 |
| `treeView` | 15 | 1.40 | 0.08 | 0.17 | 18x | 44.8 | 15.6 |
| `quadrantChart` | 14 | 2.60 | 0.10 | 0.19 | 26x | 11.7 | 9.1 |
| `ishikawa` | 13 | 2.20 | 0.08 | 0.28 | 28x | 14.4 | 15.6 |
| `cynefin` | 12 | 1.90 | 0.13 | 0.17 | 15x | 39.9 | 13.6 |
| `kanban` | 11 | 2.90 | 0.13 | 0.17 | 22x | 18.5 | 12.0 |
| `wardley` | 10 | 1.80 | 0.15 | 0.25 | 12x | 43.5 | 9.1 |
| `eventmodeling` | 6 | 1.80 | 0.15 | 0.21 | 12x | 56.5 | 12.6 |
| `packet` | 6 | 1.20 | 0.04 | 0.11 | 30x | 33.6 | 8.8 |
| `radar` | 6 | 1.50 | 0.04 | 0.10 | 38x | 36.6 | 10.8 |
| `railroad` | 5 | 3.80 | 0.15 | 0.30 | 25x | 41.9 | 11.7 |
| `info` | 3 | 0.80 | 0.01 | 0.01 | 80x | 30.5 | 8.8 |
| `railroadEbnf` | 2 | 3.80 | 0.13 | 0.13 | 29x | 44.2 | 11.8 |
| `railroadAbnf` | 1 | 3.10 | 0.10 | 0.10 | 31x | 41.7 | 11.7 |
| `railroadPeg` | 1 | 4.60 | 0.17 | 0.17 | 27x | 45.7 | 12.2 |
| **All** | 2144 | 7.80 | 0.53 | 1.82 | 15x | 132.3 | 17.0 |

The columns have the median of the diagrams of the type, except where the heading says otherwise.

## Command-line tools

The table compares `mmdg` with `mmdc` (mermaid-cli 12.2.0). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the fastest of 9 runs, after 3 runs of each tool that do not count. `mmdc` starts a browser in each run.

| Input | Output | mmdc | mmdg | Ratio |
|---|---|---:|---:|---:|
| Flowchart, 8 nodes | SVG | 529 | 30.5 | 17x |
| Flowchart, 8 nodes | PNG | 568 | 35.0 | 16x |
| Sequence diagram, 4 actors | SVG | 387 | 26.2 | 15x |
| Sequence diagram, 4 actors | PNG | 432 | 35.4 | 12x |
| Class diagram, 5 classes | SVG | 551 | 40.8 | 14x |
| Class diagram, 5 classes | PNG | 549 | 36.0 | 15x |
