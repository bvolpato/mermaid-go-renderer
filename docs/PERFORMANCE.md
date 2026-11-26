# Performance

This file is generated. Do not edit it. `node tools/reference/bench.mjs` runs the measurement and writes this file and `docs/performance.json`, which has the same numbers for the website and the README.

The numbers compare `mmdg` with mermaid 12.2.0 in Chromium 156.0.8078.4, for 2144 diagrams of the corpus `testdata/corpus/mermaid-e2e`: the diagrams that both sides rendered on the day of the measurement. Both ran on the same machine (linux/amd64, 32 CPUs) on 2026-11-25, one diagram at a time. Times are in milliseconds.

## Summary

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| Warm render, median | 9.60 | 0.55 | 17x |
| Warm render, 90th percentile | 22.40 | 1.93 | 12x |
| Warm render, all diagrams, in seconds | 26.46 | 1.86 | 14x |
| First render, median | 156.2 | 13.0 | 12x |
| First render, 90th percentile | 216.3 | 21.0 | 10x |

What each measurement is:

- **Warm render, mermaid**: the fastest of 7 `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.
- **Warm render, mmdg**: the fastest of 7 `mermaid.Render` calls for the diagram, in a process that stays running.
- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.
- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.

The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.

## By diagram type

| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |
|---|---:|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 408 | 12.10 | 0.79 | 2.12 | 15x | 182.3 | 12.1 |
| `classDiagram` | 285 | 15.60 | 0.87 | 2.96 | 18x | 168.6 | 21.2 |
| `er` | 206 | 13.40 | 0.80 | 1.63 | 17x | 185.6 | 17.1 |
| `sequence` | 193 | 6.10 | 0.24 | 0.56 | 25x | 30.3 | 13.0 |
| `requirement` | 172 | 9.60 | 0.63 | 1.68 | 15x | 183.3 | 17.3 |
| `stateDiagram` | 144 | 16.00 | 0.81 | 1.96 | 20x | 200.2 | 11.1 |
| `gitGraph` | 135 | 5.40 | 0.10 | 0.24 | 54x | 62.6 | 9.4 |
| `block` | 58 | 7.80 | 0.21 | 0.52 | 37x | 27.9 | 8.7 |
| `gantt` | 52 | 2.80 | 0.11 | 0.29 | 25x | 18.8 | 10.6 |
| `xychart` | 48 | 3.10 | 0.15 | 0.22 | 21x | 18.1 | 8.4 |
| `swimlane` | 46 | 18.70 | 1.87 | 11.46 | 10x | 77.0 | 13.0 |
| `agentflow` | 38 | 15.20 | 0.84 | 1.93 | 18x | 158.9 | 10.5 |
| `mindmap` | 33 | 9.20 | 0.36 | 5.52 | 26x | 78.7 | 9.5 |
| `c4` | 32 | 4.40 | 0.33 | 0.89 | 13x | 22.5 | 13.7 |
| `usecase` | 32 | 14.40 | 0.91 | 1.66 | 16x | 215.4 | 12.2 |
| `timeline` | 30 | 5.20 | 0.62 | 0.66 | 8x | 23.4 | 12.9 |
| `treemap` | 26 | 2.70 | 0.18 | 0.33 | 15x | 53.7 | 15.6 |
| `pie` | 24 | 1.60 | 0.05 | 0.11 | 32x | 50.0 | 8.3 |
| `venn` | 21 | 2.70 | 0.31 | 0.55 | 9x | 21.3 | 5.0 |
| `architecture` | 20 | 11.80 | 0.80 | 2.34 | 15x | 90.0 | 9.7 |
| `journey` | 18 | 3.80 | 0.11 | 0.19 | 35x | 17.3 | 9.4 |
| `sankey` | 18 | 1.60 | 0.03 | 0.06 | 53x | 14.9 | 8.1 |
| `treeView` | 15 | 1.80 | 0.07 | 0.18 | 26x | 55.4 | 11.7 |
| `quadrantChart` | 14 | 2.80 | 0.08 | 0.18 | 35x | 14.7 | 4.9 |
| `ishikawa` | 13 | 2.50 | 0.06 | 0.22 | 42x | 14.5 | 12.0 |
| `cynefin` | 12 | 3.20 | 0.13 | 0.15 | 25x | 47.4 | 9.1 |
| `kanban` | 11 | 3.60 | 0.09 | 0.13 | 40x | 20.0 | 8.8 |
| `wardley` | 10 | 2.30 | 0.13 | 0.22 | 18x | 51.5 | 5.0 |
| `eventmodeling` | 6 | 3.10 | 0.12 | 0.18 | 26x | 67.9 | 9.6 |
| `packet` | 6 | 1.50 | 0.03 | 0.09 | 50x | 91.1 | 4.2 |
| `radar` | 6 | 2.10 | 0.03 | 0.10 | 70x | 53.1 | 5.0 |
| `railroad` | 5 | 4.30 | 0.10 | 0.22 | 43x | 58.0 | 8.7 |
| `info` | 3 | 1.00 | 0.01 | 0.01 | 100x | 42.7 | 5.0 |
| `railroadEbnf` | 2 | 4.30 | 0.09 | 0.09 | 48x | 60.7 | 8.3 |
| `railroadAbnf` | 1 | 3.70 | 0.07 | 0.07 | 53x | 58.3 | 8.9 |
| `railroadPeg` | 1 | 5.20 | 0.12 | 0.12 | 43x | 62.8 | 8.0 |
| **All** | 2144 | 9.60 | 0.55 | 1.93 | 17x | 156.2 | 13.0 |

The columns have the median of the diagrams of the type, except where the heading says otherwise.

## Command-line tools

The table compares `mmdg` with `mmdc` (mermaid-cli 12.2.0). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the fastest of 9 runs, after 3 runs of each tool that do not count. `mmdc` starts a browser in each run.

| Input | Output | mmdc | mmdg | Ratio |
|---|---|---:|---:|---:|
| Flowchart, 8 nodes | SVG | 616 | 11.6 | 53x |
| Flowchart, 8 nodes | PNG | 662 | 16.1 | 41x |
| Sequence diagram, 4 actors | SVG | 434 | 17.0 | 26x |
| Sequence diagram, 4 actors | PNG | 466 | 22.7 | 21x |
| Class diagram, 5 classes | SVG | 676 | 20.9 | 32x |
| Class diagram, 5 classes | PNG | 682 | 23.3 | 29x |
