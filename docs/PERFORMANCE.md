# Performance

This file is generated. Do not edit it. `node tools/reference/bench.mjs` runs the measurement and writes this file and `docs/performance.json`, which has the same numbers for the website and the README.

The numbers compare `mmdg` with mermaid 12.2.0 in Chromium 156.0.8078.4, for 2144 diagrams of the corpus `testdata/corpus/mermaid-e2e`: the diagrams that both sides rendered on the day of the measurement. Both ran on the same machine (linux/amd64, 32 CPUs) on 2026-11-24, one diagram at a time. Times are in milliseconds.

## Summary

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| Warm render, median | 9.60 | 0.59 | 16x |
| Warm render, 90th percentile | 22.40 | 2.24 | 10x |
| Warm render, all diagrams, in seconds | 26.46 | 2.13 | 12x |
| First render, median | 156.2 | 13.1 | 12x |
| First render, 90th percentile | 216.3 | 23.4 | 9x |

What each measurement is:

- **Warm render, mermaid**: the fastest of 7 `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.
- **Warm render, mmdg**: the fastest of 7 `mermaid.Render` calls for the diagram, in a process that stays running.
- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.
- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.

The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.

## By diagram type

| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |
|---|---:|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 408 | 12.10 | 0.85 | 2.62 | 14x | 182.3 | 10.9 |
| `classDiagram` | 285 | 15.60 | 0.94 | 3.31 | 17x | 168.6 | 21.5 |
| `er` | 206 | 13.40 | 0.87 | 2.20 | 15x | 185.6 | 16.5 |
| `sequence` | 193 | 6.10 | 0.30 | 0.66 | 20x | 30.3 | 12.9 |
| `requirement` | 172 | 9.60 | 0.70 | 1.80 | 14x | 183.3 | 18.4 |
| `stateDiagram` | 144 | 16.00 | 0.89 | 2.13 | 18x | 200.2 | 11.6 |
| `gitGraph` | 135 | 5.40 | 0.11 | 0.21 | 49x | 62.6 | 9.2 |
| `block` | 58 | 7.80 | 0.22 | 0.53 | 35x | 27.9 | 9.4 |
| `gantt` | 52 | 2.80 | 0.11 | 0.30 | 25x | 18.8 | 10.3 |
| `xychart` | 48 | 3.10 | 0.19 | 0.29 | 16x | 18.1 | 8.5 |
| `swimlane` | 46 | 18.70 | 2.32 | 19.68 | 8x | 77.0 | 21.5 |
| `agentflow` | 38 | 15.20 | 0.86 | 2.03 | 18x | 158.9 | 12.1 |
| `mindmap` | 33 | 9.20 | 0.39 | 5.72 | 24x | 78.7 | 11.6 |
| `c4` | 32 | 4.40 | 0.31 | 0.87 | 14x | 22.5 | 15.6 |
| `usecase` | 32 | 14.40 | 0.95 | 2.05 | 15x | 215.4 | 14.7 |
| `timeline` | 30 | 5.20 | 0.68 | 0.77 | 8x | 23.4 | 49.2 |
| `treemap` | 26 | 2.70 | 0.24 | 0.40 | 11x | 53.7 | 29.9 |
| `pie` | 24 | 1.60 | 0.04 | 0.11 | 40x | 50.0 | 8.7 |
| `venn` | 21 | 2.70 | 0.35 | 0.51 | 8x | 21.3 | 5.3 |
| `architecture` | 20 | 11.80 | 0.79 | 2.42 | 15x | 90.0 | 11.7 |
| `journey` | 18 | 3.80 | 0.11 | 0.22 | 35x | 17.3 | 8.6 |
| `sankey` | 18 | 1.60 | 0.03 | 0.07 | 53x | 14.9 | 7.5 |
| `treeView` | 15 | 1.80 | 0.07 | 0.18 | 26x | 55.4 | 56.0 |
| `quadrantChart` | 14 | 2.80 | 0.08 | 0.17 | 35x | 14.7 | 5.1 |
| `ishikawa` | 13 | 2.50 | 0.07 | 0.26 | 36x | 14.5 | 11.7 |
| `cynefin` | 12 | 3.20 | 0.10 | 0.13 | 32x | 47.4 | 9.5 |
| `kanban` | 11 | 3.60 | 0.10 | 0.14 | 36x | 20.0 | 8.8 |
| `wardley` | 10 | 2.30 | 0.18 | 0.24 | 13x | 51.5 | 4.8 |
| `eventmodeling` | 6 | 3.10 | 0.13 | 0.19 | 24x | 67.9 | 8.3 |
| `packet` | 6 | 1.50 | 0.03 | 0.09 | 50x | 91.1 | 4.6 |
| `radar` | 6 | 2.10 | 0.03 | 0.10 | 70x | 53.1 | 5.6 |
| `railroad` | 5 | 4.30 | 0.12 | 0.23 | 36x | 58.0 | 8.4 |
| `info` | 3 | 1.00 | 0.01 | 0.01 | 100x | 42.7 | 5.1 |
| `railroadEbnf` | 2 | 4.30 | 0.10 | 0.10 | 43x | 60.7 | 9.3 |
| `railroadAbnf` | 1 | 3.70 | 0.08 | 0.08 | 46x | 58.3 | 9.3 |
| `railroadPeg` | 1 | 5.20 | 0.12 | 0.12 | 43x | 62.8 | 8.6 |
| **All** | 2144 | 9.60 | 0.59 | 2.24 | 16x | 156.2 | 13.1 |

The columns have the median of the diagrams of the type, except where the heading says otherwise.

## Command-line tools

The table compares `mmdg` with `mmdc` (mermaid-cli 12.2.0). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the fastest of 9 runs, after 3 runs of each tool that do not count. `mmdc` starts a browser in each run.

| Input | Output | mmdc | mmdg | Ratio |
|---|---|---:|---:|---:|
| Flowchart, 8 nodes | SVG | 579 | 14.5 | 40x |
| Flowchart, 8 nodes | PNG | 652 | 22.2 | 29x |
| Sequence diagram, 4 actors | SVG | 441 | 17.3 | 25x |
| Sequence diagram, 4 actors | PNG | 507 | 22.7 | 22x |
| Class diagram, 5 classes | SVG | 631 | 23.6 | 27x |
| Class diagram, 5 classes | PNG | 606 | 27.5 | 22x |
