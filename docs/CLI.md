# The mmdg command

`mmdg` renders Mermaid diagrams to SVG and PNG files. It takes the options of mermaid-cli (`mmdc`) and writes files with the content of the files that `mmdc` writes. It does not start a browser.

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
| `-b`, `--backgroundColor` | CSS color | The background of an SVG or a PNG. The default is `white`. `transparent` gives a PNG without a background. |
| `-c`, `--configFile` | file | A JSON file with the mermaid config, which is the argument of `mermaid.initialize`. A `theme` in the file replaces `--theme`. |
| `-C`, `--cssFile` | file | A CSS file. Its text becomes `themeCSS` of the mermaid config, and replaces a `themeCSS` of the config file. |
| `-I`, `--svgId` | id | The `id` of the `svg` element. The default is `my-svg`. |
| `-s`, `--scale` | number | The number of PNG pixels for each px. The default is 1. |
| `--size` | px | The largest width and height of the diagram. A PNG fills this size. An SVG gets the size as `max-width` and `max-height`, and loses its `height` attribute. |
| `-a`, `--artefacts` | directory | The directory for the images of a Markdown input. `mmdg` creates it. The default is the directory of the output file. |
| `-j`, `--jobs` | number | The number of diagrams of a Markdown input that render at the same time. The default is half of the CPUs. |
| `-q`, `--quiet` | | Do not print the messages about the progress. |
| `-p`, `--puppeteerConfigFile` | file | Accepted and not read. `mmdg` uses no browser. |
| `--no-font-embed` | | Accepted. `mmdg` never embeds fonts. See [Fonts of the page of mmdc](#fonts-of-the-page-of-mmdc). |
| `--pdf-paper-format` | name | Accepted. PDF output is not supported. |
| `--iconPacks`, `--iconPacksNamesAndUrls` | names | Not supported. `mmdg` stops with an error. |
| `-V`, `--version` | | Print the version of `mmdg` and the version of mermaid that it ports. |
| `-h`, `--help` | | Print the help. |

## Options of mermaid-cli 11

mermaid-cli 12 removed these options. `mmdg` accepts them, so that scripts for mermaid-cli 11 continue to work.

| Option | Value | Effect |
| --- | --- | --- |
| `-w`, `--width` | px | The width of the page. The default is 800. A diagram that fills the page, such as a gantt chart, uses this width. A PNG is not wider than the page minus 16 px. `--size` replaces this option. |
| `-H`, `--height` | px | Accepted. The height of the page has no effect on a diagram. |
| `-f`, `--pdfFit` | | Accepted. PDF output is not supported. |

## Options of mmdg only

| Option | Value | Effect |
| --- | --- | --- |
| `--fonts` | `embedded`, `system` | The fonts that measure the text. `embedded` is the default: the Liberation fonts that are part of `mmdg`, with the rules of Chromium on Linux. The output is then the same on every machine. `system` uses the fonts of this machine, with the rules of Chromium on this operating system. |
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

## What is the same as mmdc

These points were compared with real `mmdc` 12.0.0 runs on macOS. The tests in `cmd/mmdg` repeat the comparison from recorded results.

- The SVG file. `mmdc` does not write the string that `mermaid.render()` returns. It puts the SVG into its page, sets the background color on the `svg` element, and writes the element with `XMLSerializer`. `mmdg` does the same steps. For all 2080 reference SVGs of the mermaid test corpus, the result is byte for byte what Chromium returns for these steps. This was checked with the background `white`, the background `transparent`, `--size`, and without a background.
- The size of a PNG. `mmdc` takes a screenshot of the `svg` element in a page that is 800 px wide. The diagram is not wider than 784 px. `mmdc` then makes the page as large as the diagram before it takes the screenshot, and the body has a margin of 8 px. A diagram with `width="100%"`, which is what mermaid makes, becomes 8 px narrower in an image of the first size. The image shows a strip of background at the right and at the bottom. `mmdg` makes the same image size and the same diagram size. The sizes were compared for 20 cases: 6 diagrams, the scales 1.25, 1.5, 2 and 3, `--size 300`, and two backgrounds.
- The exit code and the text on stderr for 16 wrong command lines, and the names of the output files for 9 command lines.
- The Markdown output, with the escapes of the alt text and of the title.

### SVG files of a sample of the corpus

`mmdc` rendered 92 diagrams of the mermaid test corpus, from 33 of its directories, with six sets of options. The table shows how many files of `mmdg --fonts system` are byte for byte the files of `mmdc`, and why the other files differ.

| Options of mmdc | Same bytes | mmdc embedded fonts | Fonts of the page | Sequence diagram | KaTeX math |
| --- | --- | --- | --- | --- | --- |
| none | 74 | 14 | 0 | 3 | 1 |
| `--no-font-embed` | 77 | 0 | 11 | 3 | 1 |
| `-b transparent` | 74 | 14 | 0 | 3 | 1 |
| `-C style.css` | 74 | 14 | 0 | 3 | 1 |
| `-I diagram` | 74 | 14 | 0 | 3 | 1 |
| `-t dark` | 76 | 10 | 2 | 3 | 1 |

Every file that differs belongs to a diagram that uses a font of the page of `mmdc`, to a sequence diagram, or to a diagram with KaTeX math, which `mmdg` does not render. The next section explains the fonts. Of the 92 diagrams, 75 name no font and no style sheet of the page of `mmdc`. 72 of these are the same in every set of options. The other 3 are the sequence diagrams.

The sample has no gantt chart, no diagram that draws random numbers and no diagram with an image from the network. `mmdc` gives another result for these in each run or on each machine.

## What differs from mmdc

### Fonts of the page of mmdc

The page of `mmdc` loads web fonts and style sheets that a plain page has not: Recursive Variable, Open Sans, Font Awesome and KaTeX. `mmdg` has none of them. It measures text as mermaid does in a page without these fonts, and falls back to the next font of the diagram, which is Arial or the default sans-serif font.

This changes the layout of a diagram that names one of the fonts. In mermaid 12, the default theme of several diagram types, such as flowcharts and sequence diagrams, names Recursive Variable. The config of a sequence diagram names Open Sans for the names of its actors in every theme. In one comparison with `-t default`, the sequence diagram of `mmdc` was 6 px lower than the diagram of `mmdg`. The fonts of the page are the probable cause. A label with a Font Awesome icon has the width of the icon in `mmdc`, and no width in `mmdg`. The themes `default`, `dark`, `forest`, `neutral` and `base` name fonts of the machine for most diagram types, so `-t default` gives the layout of `mmdc` for those types.

`mmdc` also embeds the fonts that a diagram uses into the SVG file, as a second `style` element with the font data. `mmdg` does not. Its SVG is then the SVG of `mmdc --no-font-embed`, when the layout is the same.

### Other differences

- KaTeX math in a label is not supported. `mmdg` stops with an error for such a diagram.
- PDF output is not supported. `mmdg` has no PDF writer. It stops with an error for `-o file.pdf` and for `-e pdf`.
- Icon packs are not supported. `--iconPacks` and `--iconPacksNamesAndUrls` stop with an error. A diagram that names an icon of a pack shows the icon for an unknown icon.
- `--backgroundColor` reads these forms of a CSS color: a name, a hex color, `rgb()`, `rgba()`, `hsl()` and `hsla()`. A hex color can come without the `#`, as in Chromium. Other forms that Chromium reads, such as `hwb()`, `lab()` and `color-mix()`, have no effect.
- A PNG background that is not a color, such as a gradient, has no effect. `mmdc` paints it on the page around the diagram.
- `mmdg` rounds the size of a PNG to whole pixels for a scale that is not a whole number. The cases that were compared with `mmdc` agree. Sizes with a fraction below one half were not compared.
- `mmdc` renders with the device scale factor of `--scale`. `mmdg` lays out the diagram at scale 1 for every scale. The image sizes that were compared agree.
- Random numbers have a fixed seed in `mmdg`. A diagram that draws random numbers, for example for the hand-drawn look, is the same in each run. In `mmdc` it differs between runs.
- `--version` prints the version of `mmdg`, not a version of mermaid-cli.

## What was not compared with mmdc

- The pixels of a PNG. Only the size of the image was compared, and the position of the diagram in it for one diagram.
- `--scale` with SVG output, `--width`, and `--artefacts` together with `--size`.
- Diagrams that show the current time, such as a gantt chart with a today marker.
