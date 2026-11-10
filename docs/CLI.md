# The mmdg command

`mmdg` renders Mermaid diagrams to SVG, PNG, and vector PDF files. It takes the options of mermaid-cli (`mmdc`) and writes SVG and PNG files in the formats that `mmdc` writes. It does not start a browser. By default it uses the `mmdc` page, which has web fonts and the ZenUML diagram type. A plain page does not register the ZenUML plugin.

The options follow mermaid-cli 12.0.0. That is the newest version that npm accepted when this document was written, and it asks for mermaid `^12.0.0`. The source of mermaid-cli 12.2.0 differs from it in one line of the help text.

## Usage

```sh
mmdg [options]
```

`mmdg` takes no other arguments. An option takes its value in each form of `mmdc`: `-i file`, `-ifile`, `--input file` and `--input=file`. Options without a value can share one dash: `-qi file`.

## Examples

Render one file. The output name is the input name with `.svg`.

```sh
mmdg -i diagram.mmd
mmdg -i diagram.mmd -o diagram.svg
```

Render a PNG with two pixels for each px.

```sh
mmdg -i diagram.mmd -o diagram.png -s 2
```

Render a vector PDF. By default, the page fits the diagram. Choose a paper size with `--pdf-paper-format`.

```sh
mmdg -i diagram.mmd -e pdf -o diagram.pdf
mmdg -i diagram.mmd -o x.pdf --pdf-paper-format A4
```

Render every mermaid block of a Markdown file. This command writes `out-1.svg`, `out-2.svg` and so on.

```sh
mmdg -i README.md -o out.svg
```

Write a Markdown file that has an image in place of every mermaid block. The images go to the directory `images`.

```sh
mmdg -i README.md -o README.out.md -a images
```

Read from stdin and write to stdout.

```sh
cat diagram.mmd | mmdg -i - -o - -e svg > diagram.svg
```

Use a mermaid config file and a theme.

```sh
mmdg -i diagram.mmd -o diagram.svg -c mermaid.json -t dark
```

Write a small SVG, without the fonts of the page.

```sh
mmdg -i diagram.mmd -o diagram.svg --no-font-embed
```

Render as a web site with only mermaid renders, without the web fonts of `mmdc`.

```sh
mmdg -i diagram.mmd -o diagram.svg --page plain
```

Measure text with the fonts of this machine, as a browser on this machine does.

```sh
mmdg -i diagram.mmd -o diagram.svg --fonts system
```

Add a directory of font files to the embedded fonts, for example for Chinese text.

```sh
mmdg -i diagram.mmd -o diagram.svg --fontDir /usr/share/fonts/noto
```

## Options of mermaid-cli

| Option | Value | Effect |
| --- | --- | --- |
| `-i`, `--input` | file | The input file. A name that ends in `.md` or `.markdown` is a Markdown file. `-` is stdin. Without the option, `mmdg` prints a warning and reads stdin. |
| `-o`, `--output` | file | The output file. The name must end in `.svg`, `.png`, `.pdf`, `.md` or `.markdown`. `-` is stdout. The default is the input name with `.svg`, or `out.svg` for stdin. |
| `-e`, `--outputFormat` | `svg`, `png`, `pdf` | The output format. The default comes from the name of the output file. The option wins over the name: `-o a.svg -e png` writes a PNG into `a.svg`. |
| `-t`, `--theme` | name | The theme: `base`, `dark`, `default`, `forest`, `neutral`, `neo`, `neo-dark`, `redux`, `redux-dark`, `redux-color`, `redux-dark-color`, `null`. Without the option, mermaid selects the theme for the diagram type. |
| `-b`, `--backgroundColor` | CSS color | The background of an SVG, PNG, or PDF. The default is `white`. `transparent` gives a PNG without a background. |
| `-c`, `--configFile` | file | A JSON file with the mermaid config, which is the argument of `mermaid.initialize`. A `theme` in the file replaces `--theme`. |
| `-C`, `--cssFile` | file | A CSS file. Its text becomes `themeCSS` of the mermaid config, and replaces a `themeCSS` of the config file. |
| `-I`, `--svgId` | id | The `id` of the `svg` element. The default is `my-svg`. |
| `-s`, `--scale` | number | The number of PNG pixels for each px. The default is 1. |
| `--size` | px | The largest width and height of the diagram. A PNG fills this size. An SVG gets the size as `max-width` and `max-height`, and loses its `height` attribute. |
| `-a`, `--artefacts` | directory | The directory for the images of a Markdown input. `mmdg` creates it. The default is the directory of the output file. |
| `-j`, `--jobs` | number | The number of diagrams of a Markdown input that render at the same time. The default is half of the CPUs. |
| `-q`, `--quiet` | | Do not print the messages about the progress. |
| `-p`, `--puppeteerConfigFile` | file | Accepted and not read. `mmdg` uses no browser. |
| `--no-font-embed` | | Do not embed the fonts of the page into an SVG. By default an SVG has the fonts of the page that the diagram uses, as a second `style` element. See [The page of mmdc](#the-page-of-mmdc). |
| `--pdf-paper-format` | name | Paper size for PDF output: `Letter`, `Legal`, `Tabloid`, `Ledger`, or `A0` through `A6`. Without this option, the page fits the diagram. |
| `--iconPacks`, `--iconPacksNamesAndUrls` | names | Not supported. `mmdg` stops with an error. |
| `-V`, `--version` | | Print the version of `mmdg` and the version of mermaid that it ports. |
| `-h`, `--help` | | Print the help. |

## Options of mermaid-cli 11

mermaid-cli 12 removed these options. `mmdg` accepts them, so that scripts for mermaid-cli 11 continue to work.

| Option | Value | Effect |
| --- | --- | --- |
| `-w`, `--width` | px | The width of the page. The default is 800. A diagram that fills the page, such as a gantt chart, uses this width. A PNG is not wider than the page minus 16 px. `--size` replaces this option. |
| `-H`, `--height` | px | Accepted. The height of the page has no effect on a diagram. |
| `-f`, `--pdfFit` | | Accepted for compatibility with mermaid-cli 11. PDF output fits the diagram by default. |

## Options of mmdg only

| Option | Value | Effect |
| --- | --- | --- |
| `--page` | `mermaid-cli`, `plain` | The web page that the render reproduces. `mermaid-cli` is the default: the page of `mmdc`, which has the web fonts Recursive Variable, Open Sans and Font Awesome and their style sheets. `plain` is a page with only mermaid, which is what `mermaid.render()` gives on a web site that loads no such fonts. See [The page of mmdc](#the-page-of-mmdc). |
| `--fonts` | `embedded`, `system` | The fonts that stand for the fonts of the machine. `embedded` is the default: the Liberation fonts that are part of `mmdg`, with the rules of Chromium on Linux. The output is then the same on every machine. `system` uses the fonts of this machine, with the rules of Chromium on this operating system. |
| `--fontDir` | directory | A directory with more font files for `--fonts embedded`. Repeat the option for more directories. |
| `--imageSizes` | file | A JSON file with the size of the images that diagrams name by URL: `{"https://example.com/a.png": [40, 30]}`. `mmdg` never reads from the network. Without a size, an image counts as an image that failed to load. |
| `--timeout` | duration | The longest time for one diagram, for example `30s`. The default is no limit. |
| `--now` | time | The time that diagrams see as now, for example `2026-01-15T12:00:00Z`. The time zone of the value is the local time zone of the render. The default is the clock and the time zone of this machine, as in a browser. A gantt chart shows this time as its today marker. |

## Markdown input

`mmdg` finds the blocks with the regular expression of `mmdc`. A block starts with a line that has three characters from `` ` `` and `:`, then the word `mermaid`. It ends at the next three such characters that end a line.

Each block becomes one file. The name is the output name with a number: `out.svg` gives `out-1.svg`. When the output is a Markdown file, the images have the output format, which is `svg` by default: `out.md` gives `out-1.svg`. `mmdg` then writes the Markdown file with an image in place of each block. The alt text of the image is the accessible description of the diagram (`accDescr`), or `diagram`. The title of the image is the accessible title (`accTitle`).

A Markdown input cannot go to stdout.

## Messages and exit codes

`mmdg` prints the messages of `mmdc`: the progress on stdout, and warnings and errors on stderr. The exit code is 0 after success and 1 after an error. An error in the options has the form of `mmdc`, for example `error: unknown option '--bogus'`.

When a diagram has an error, `mmdg` prints the message of the error. `mmdc` prints the same message with a JavaScript stack.

## The page of mmdc

`mmdc` renders a diagram in a web page of its own. Before it calls mermaid, it adds 8 style sheets to the page and loads every font that they name: Recursive Variable, Open Sans, Font Awesome Free, and KaTeX. A plain web page has none of these web fonts.

The fonts change the layout. In mermaid 12 the default theme of flowcharts, class diagrams, sequence diagrams and other types names Recursive Variable, and several diagram types name Open Sans. In a plain page Chromium falls back to Arial. In the page of `mmdc` it measures the text with the web font. A label with a Font Awesome icon, such as `fa:fa-car`, has the size of the icon in the page of `mmdc` and no size in a plain page.

`mmdg` has the fonts and style sheets of the `mmdc` page as embedded data, including KaTeX styles. KaTeX math in labels renders on both `--page` modes, with the resulting MathML laid out by the browser engine. Some stretched delimiters, accents, and matrices differ from KaTeX's browser layout. Legacy KaTeX HTML sizing is approximate. With `--page mermaid-cli`, which is the default, the ZenUML diagram type is available. With `--page plain`, the external ZenUML plugin is not registered.

`mmdc` also registers the `tidy-tree` layout from `@mermaid-js/layout-tidy-tree` and the external ZenUML diagram plugin. `mmdg` supports both on `--page mermaid-cli`. A plain page registers neither plugin, so Mermaid uses the default layout for `layout: tidy-tree` and rejects the ZenUML diagram type.

After the render, `mmdc` puts the fonts that the diagram uses into the SVG file: for each style sheet of the page that has a used font, or a rule for an element of the diagram, it adds one `style` element with the license of the font package, the rules of the style sheet and the font data. `mmdg` does the same. A flowchart with the default theme is then about 170 kB larger. `--no-font-embed` writes the SVG without the fonts, as the same option of `mmdc` does.

## What is the same as mmdc

### The .svg files of the corpus

The reference is the code of `mmdc` 12.0.0 itself, with mermaid 12.2.0: the function `renderMermaid`, which the `mmdc` command calls for each diagram, in Chromium 156. `tools/reference/mmdc-oracle.mjs` runs it for every diagram of three corpora and records both `.svg` files: the file with the fonts and the file of `--no-font-embed`. The test `TestCorpusParity` compares the files of this module with them, byte for byte.

The table has the result for Chromium on Linux with only the Liberation fonts as system fonts, which is what `--fonts embedded` reproduces. [PARITY-MMDC.md](PARITY-MMDC.md) has the tables for each diagram type.

<!-- generated:corpus start -->
| Corpus | Diagrams | Pass | Other outcomes |
| --- | ---: | ---: | ---: |
| The end-to-end tests of mermaid | 2145 | 2143 | 2 |
| The examples of the mermaid documentation | 596 | 594 | 2 |
| Generated large diagrams, 15 to 150 nodes | 70 | 70 | 0 |

Pass requires both mmdc output files to match, or both tools to return an error. Other outcomes include partial matches and different output. See [What differs from mmdc](#what-differs-from-mmdc).

With `--fonts system` on macOS, the reference is Chromium on macOS with its system fonts. There 2134 of 2145, 594 of 596 and 70 of 70 diagrams pass. 9 more diagrams of the first corpus do not match both output files. [PARITY-MMDC-MACOS.md](PARITY-MMDC-MACOS.md) has the tables.
<!-- generated:corpus end -->

The reference makes `mmdc` repeatable in four points, where the `mmdc` command is not: the random numbers have a fixed seed, the clock shows a fixed time, the time zone is UTC, and the style sheets of the page have the order of the source of `mmdc`. `mmdg` has the same seed. Give `--now 2026-01-15T12:00:00Z` for the same time.

### The mmdc command

These points were compared with runs of the real `mmdc` command on macOS. The tests in `cmd/mmdg` repeat the comparison from recorded results.

- The steps after the render. `mmdc` does not write the string that `mermaid.render()` returns. It puts the SVG into its page, sets the background color on the `svg` element, and writes the element with `XMLSerializer`. For all 2080 reference SVGs of the mermaid test corpus, the result of `mmdg` is byte for byte what Chromium returns for these steps. This was checked with the background `white`, the background `transparent`, `--size`, and without a background.
- The size of a PNG. `mmdc` takes a screenshot of the `svg` element in a page that is 800 px wide. The diagram is not wider than 784 px. `mmdc` then makes the page as large as the diagram before it takes the screenshot, and the body has a margin of 8 px. A diagram with `width="100%"`, which is what mermaid makes, becomes 8 px narrower in an image of the first size. The image shows a strip of background at the right and at the bottom. `mmdg` makes the same image size and the same diagram size. The sizes were compared for 20 cases: 6 diagrams, the scales 1.25, 1.5, 2 and 3, `--size 300`, and two backgrounds.
- The exit code and the text on stderr for 16 wrong command lines, and the names of the output files for 9 command lines.
- The Markdown output, with the escapes of the alt text and of the title.
- The embedded fonts. For 15 small diagrams that use Recursive Variable, Open Sans, the three fonts of Font Awesome, or none of them, the added `style` elements of `mmdg` are those of `mmdc`, character for character.
- A sample of 92 diagrams of the corpus, from 33 of its directories, that the `mmdc` command rendered with six sets of options. With `--fonts system` on the same machine, 90 to 92 of the 92 outputs of `mmdg` match `mmdc` for each set. The KaTeX math example matches in all six sets. The remaining differences are two diagrams with another layout under `-t dark`, and two cases where only the order of embedded `style` elements differs.

## What differs from mmdc

### Diagram features

- **KaTeX math.** Math in labels (`$$...$$`) renders on both page modes. Stretched delimiters, accents, and matrices can differ from KaTeX's browser layout. Legacy KaTeX HTML sizing is approximate.
- **ZenUML.** This diagram type is available on the mermaid-cli page. Use `--page mermaid-cli`, which is the default. The plain page does not register the plugin. EC2, IAM, Lambda, SNS, SQS and Azure Function participants use labeled boxes in place of vendor artwork.

### The order of the style sheets

`mmdc` adds the 8 style sheets to its page from calls that run at the same time. Their order in the page changes from run to run. The order decides the order of the embedded `style` elements when a diagram uses fonts of two or more style sheets, for example a diagram with a Font Awesome icon, or a diagram with Open Sans, which has one style sheet for normal text and one for italic text. Two runs of `mmdc` can then write two different files. `mmdg` always uses the order of the list in the source of `mmdc`. In one test, `mmdc` rendered 4 diagrams, and each of the 4 pages had another order.

### The scale and the SVG

`mmdc` renders with the device scale factor of `--scale`, also for an SVG. `mmdg` lays out the diagram at scale 1 for every scale. In a sample of 71 diagrams, the SVG of `mmdc --scale 2` was the SVG of scale 1 for 69. The other 2 are gantt charts with section titles of several lines. The image sizes of PNG output that were compared agree.

### Other differences

- PDF output uses vector paths and TrueType fonts or Type 3 glyph subsets where possible. Unsupported fonts become outlines, and some contextual glyphs or ligatures may not extract as the original text. Drop shadows are flat silhouettes. Other filter graphs and image elements, including data URLs, are omitted.
- Icon packs are not supported. `--iconPacks` and `--iconPacksNamesAndUrls` stop with an error. A diagram that names an icon of a pack shows the icon for an unknown icon.
- `--backgroundColor` reads these forms of a CSS color: a name, a hex color, `rgb()`, `rgba()`, `hsl()` and `hsla()`. A hex color can come without the `#`, as in Chromium. Other forms that Chromium reads, such as `hwb()`, `lab()` and `color-mix()`, have no effect.
- A PNG background that is not a color, such as a gradient, has no effect. `mmdc` paints it on the page around the diagram.
- `mmdg` rounds the size of a PNG to whole pixels for a scale that is not a whole number. The cases that were compared with `mmdc` agree. Sizes with a fraction below one half were not compared.
- Random numbers have a fixed seed in `mmdg`. A diagram that draws random numbers, for example for the hand-drawn look, is the same in each run. In `mmdc` it differs between runs.
- `--version` prints the version of `mmdg`, not a version of mermaid-cli.

## What was not compared with mmdc

- The pixels of a PNG. Only the size of the image was compared, and the position of the diagram in it for one diagram.
- PDF output is not included in the corpus manifests. The manifests compare SVG files.
- `--width`, and `--artefacts` together with `--size`.
- The corpus with other options than the defaults in the page of `mmdc`: `--theme`, `--cssFile`, `--svgId` and `--configFile` were compared only on the sample of 92 diagrams.
