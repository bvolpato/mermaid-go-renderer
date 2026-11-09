// The extra inputs of gen-treeview-cynefin-wardley-db.mjs: texts that reach
// the branches of the db and of the renderer of the treeView, cynefin and
// wardley diagrams that the corpus fixtures do not reach.

// fm puts a config in the front matter of a diagram text.
const fm = (config, body) => `---\nconfig:\n${config}\n---\n${body}`;

const cynefinFull =
  'cynefin-beta\n' +
  '  title Software Delivery\n' +
  '  complex\n  "Feature discovery"\n  "User research"\n' +
  '  complicated\n  "Refactoring"\n' +
  '  clear\n  "Hotfix"\n  "Deploy"\n  "Rollback"\n' +
  '  chaotic\n  "Incident"\n' +
  '  confusion\n  "Unknown"\n' +
  '  complex --> complicated : "Pattern identified"\n' +
  '  clear --> chaotic : "Complacency"\n' +
  '  chaotic --> complex\n';

const cynefin = [
  'cynefin-beta\n',
  'cynefin-beta:\n  complex\n  "a"\n',
  cynefinFull,
  'cynefin-beta\n  confusion\n  "one"\n  "two"\n  "three"\n  "four"\n  "five"\n  "six"\n',
  'cynefin-beta\n  confusion\n  "one"\n  "two"\n  "three"\n',
  'cynefin-beta\n  confusion\n  "one"\n  "two"\n  "three"\n  "four"\n',
  fm('  cynefin:\n    showDomainDescriptions: false', 'cynefin-beta\n  confusion\n  "one"\n  "two"\n  "three"\n  "four"\n  complex\n  "x"\n'),
  fm('  cynefin:\n    showDomainDescriptions: false', cynefinFull),
  'cynefin-beta\n  complex --> complicated\n  complex --> clear : "a"\n  complex --> chaotic : "b"\n  complex --> confusion : "c"\n' +
    '  complicated --> complex : "d"\n  clear --> complex\n  chaotic --> clear : "e"\n  confusion --> chaotic : "f"\n  confusion --> complicated\n',
  'cynefin-beta\n  complex --> complex : "self"\n  clear --> clear\n',
  'cynefin-beta\n  complex --> complex : "self"\n  clear --> chaotic : ""\n',
  'cynefin-beta\n  title In the text\n  accTitle: The acc title\n  accDescr: The description\n  complex\n  "a"\n',
  'cynefin-beta\n  accDescr {\n    multi\n    line  text\n  }\n  complex\n  "a"\n  title Late  title\n',
  '---\ntitle: Front matter title\n---\ncynefin-beta\n  complex\n  "a"\n',
  '---\ntitle: Front matter title\n---\ncynefin-beta\n  title Text title\n  complex\n  "a"\n',
  fm('  cynefin:\n    width: 400\n    height: 300\n    padding: 10', cynefinFull),
  fm('  cynefin:\n    width: 1000\n    height: 500\n    padding: 0', cynefinFull),
  fm('  cynefin:\n    useMaxWidth: false', cynefinFull),
  fm('  cynefin:\n    useMaxWidth: null', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  cynefin:\n    boundaryAmplitude: 0', 'cynefin-beta\n'),
  fm('  cynefin:\n    boundaryAmplitude: 25', 'cynefin-beta\n'),
  fm('  cynefin:\n    boundaryAmplitude: null', 'cynefin-beta\n'),
  fm('  cynefin:\n    boundaryAmplitude: "12"', 'cynefin-beta\n'),
  fm('  cynefin:\n    boundaryAmplitude: -3.5', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: 42', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: 0', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: -7', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: 1.5', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: 10000000000', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: 4294967295', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: 1e300', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: "42"', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: .nan', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: .inf', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: null', 'cynefin-beta\n'),
  fm('  cynefin:\n    seed: true', 'cynefin-beta\n'),
  fm('  cynefin:\n    width: "800"\n    height: "600"\n    padding: "40"', cynefinFull),
  fm('  cynefin:\n    width: "500"\n    useMaxWidth: false', 'cynefin-beta\n  title T\n'),
  fm('  cynefin:\n    width: 0\n    height: 0', 'cynefin-beta\n  title T\n  complex\n  "a"\n  complex --> clear : "x"\n'),
  fm('  cynefin:\n    width: null\n    height: true', 'cynefin-beta\n  complex --> clear : "x"\n'),
  fm('  cynefin:\n    padding: 0', 'cynefin-beta\n  title Zero padding\n'),
  fm('  cynefin:\n    padding: 33.3', 'cynefin-beta\n  title Odd padding\n  complex\n  "a"\n'),
  fm('  cynefin: 5', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  cynefin: "ab"', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  cynefin: null', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  cynefin:\n    unknownKey: 1', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  theme: dark', cynefinFull),
  fm('  theme: forest', cynefinFull),
  fm('  theme: neutral', cynefinFull),
  fm('  theme: base\n  themeVariables:\n    cynefin:\n      complexBg: "#FFE4B5"\n      confusionBg: red\n      itemFontSize: 20\n      domainFontSize: 24', cynefinFull),
  fm('  themeVariables:\n    cynefin:\n      chaoticBg: 5\n      clearBg: null\n      complicatedBg: ""', cynefinFull),
  fm('  themeVariables:\n    cynefin: null', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  themeVariables:\n    cynefin: "x"', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  themeVariables:\n    cynefin: 7', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  themeVariables:\n    cynefin: []', 'cynefin-beta\n  complex\n  "a"\n'),
  fm('  fontFamily: courier', cynefinFull),
  fm('  themeVariables:\n    fontFamily: "Times New Roman"\n    fontSize: 20px', cynefinFull),
  '%%{init: {"cynefin": {"width": 400, "seed": 3}}}%%\ncynefin-beta\n  complex\n  "a"\n',
  'cynefin-beta\n  complex\n  "<b>bold</b>"\n  "a &amp; b < c"\n  "quote \\" and \\\\ backslash"\n  \'single\'\n  ""\n' +
    '  complicated\n  "é ü 😀 日本語"\n  "a very long item label that is wider than the quadrant of the diagram itself, really"\n' +
    '  clear\n  "entity #quot;x#quot; #35; #lt;"\n  "  spaces  "\n',
  'cynefin-beta\n  complex\n  "first"\n  complex\n  "second"\n  "third"\n  complex\n',
  'cynefin-beta\n  complex\n  "a"\n  clear\n  complex\n  "b"\n',
  'cynefin-beta\n  complex --> clear : "<i>markup</i> & #amp; entity"\n  clear --> complex : "é 😀"\n',
  'cynefin-beta\n  title <b>Bold</b> & title #35;\n  accTitle: <i>acc</i>\n  accDescr: <script>alert(1)</script>descr\n',
  'cynefin-beta\n\n\n  complex\n\n  "a"\n\n\n  "b"\n\n  complex --> clear\n\n\n',
  'cynefin-beta\r\n  complex\r\n  "a"\r\n  complex --> clear : "x"\r\n',
  'cynefin-beta %% comment\n  complex %% c\n  "a"\n',
  // Errors.
  'cynefin-beta\n  complex --> clear "a"\n',
  'cynefin-beta\n  "orphan"\n',
  'cynefin-beta\n  complexity\n',
  'cynefin-beta\n  complex -> clear\n',
  'cynefin-beta\n  complex\n  "unterminated\n',
  'cynefin-beta x\n',
];

const tree =
  'treeView-beta\n' +
  '  my-project/\n' +
  '      src/\n' +
  '          App.tsx :::highlight icon(fa:bell) ## main component\n' +
  '          index.js ## entry point\n' +
  '          styles.css\n' +
  '      docs/\n' +
  '      .env ## environment variables\n' +
  '      package.json :::highlight\n' +
  '  README.md\n';

const treeview = [
  'treeView-beta\n',
  'treeView-beta\n  a\n',
  'treeView-beta\na\nb\nc/\n',
  tree,
  'treeView-beta\n  a/\n    b/\n      c/\n        d/\n          e.txt\n    f.txt\n  g/\n  h\n',
  'treeView-beta\n\ta/\n\t\tb\n\t\t\tc\n \t d\n',
  'treeView-beta\n        deep first\n    less\n  least\n      more\n',
  'treeView-beta\n  "quoted name"\n    \'single quoted/\'\n      "nested"\n  ""\n  "/"\n  "a/b/"\n',
  'treeView-beta\n  a  b   c.txt\n  name with trailing spaces   \n  é ü 日本語/\n    🚀 rocket.md\n',
  'treeView-beta\n  a :::highlight\n  b :::other\n  c :::highlight ## with description\n    d :::highlight icon(file)\n  e/ :::highlight\n',
  'treeView-beta\n  a :::highlight :::other\n  b :::other :::highlight\n',
  'treeView-beta\n  a icon()\n  b icon(none)\n  c icon(folder)\n  d icon(file)\n  e icon(fa:bell)\n  f icon(bell)\n  g/ icon(file)\n  h/ icon()\n',
  'treeView-beta\n  a icon(constructor)\n  b icon(toString)\n  c icon(__proto__)\n  d icon(a:b)\n  e icon(mermaid-treeview:folder)\n  f icon(mermaid-treeview:missing)\n',
  'treeView-beta\n  a icon(folder) icon(file)\n  b icon(file) icon()\n  c icon() icon(folder)\n',
  fm('  treeView:\n    showIcons: true', tree),
  fm('  treeView:\n    showIcons: true', 'treeView-beta\n  a icon()\n  b icon(none)\n  c/\n    d\n  e/ icon(file)\n  f icon(folder)\n  g icon(x:y)\n'),
  fm('  treeView:\n    showIcons: true\n    defaultIconPack: material-icon-theme', tree),
  fm('  treeView:\n    defaultIconPack: fa', 'treeView-beta\n  a icon(bell)\n  b icon(folder)\n  c icon(x:y)\n  d\n'),
  fm(
    '  treeView:\n    showIcons: true\n    defaultIconPack: mi\n    filenameIcons:\n      README.md: "fa:bell"\n      Makefile: none\n      LICENSE: file\n      five: 5\n      empty: ""\n' +
      '    extensionIcons:\n      .ts: typescript\n      py: none\n      .xyz: folder\n      md: markdown\n      .tar.gz: archive\n      gz: zip',
    'treeView-beta\n  README.md\n  Makefile\n  LICENSE\n  empty\n  index.ts\n  UPPER.TS\n  main.py\n  data.xyz\n  notes.md\n  a.tar.gz\n  unmapped.bin\n  .ts\n  noext\n  dir.ts/\n'
  ),
  fm('  treeView:\n    showIcons: true\n    filenameIcons:\n      five: 5', 'treeView-beta\n  five\n'),
  fm('  treeView:\n    showIcons: true', 'treeView-beta\n  constructor\n'),
  fm('  treeView:\n    showIcons: true', 'treeView-beta\n  a.constructor\n'),
  fm('  treeView:\n    showIcons: true', 'treeView-beta\n  x.toString\n  valueOf.txt\n'),
  fm('  treeView:\n    showIcons: false', 'treeView-beta\n  constructor\n  toString\n'),
  fm('  treeView:\n    showIcons: true\n    filenameIcons: text\n    extensionIcons: 5', 'treeView-beta\n  a.b\n  x\n'),
  fm('  treeView:\n    showIcons: true\n    filenameIcons: null\n    extensionIcons: null', 'treeView-beta\n  a.b\n  x\n'),
  fm('  treeView:\n    showIcons: true\n    filenameIcons:\n      a.b: [":"]\n      c: ["x"]', 'treeView-beta\n  a.b\n  c\n'),
  fm('  treeView:\n    rowIndent: 30\n    paddingX: 12\n    paddingY: 9\n    lineThickness: 3', tree),
  fm('  treeView:\n    rowIndent: 0\n    paddingX: 0\n    paddingY: 0\n    lineThickness: 0', tree),
  fm('  treeView:\n    rowIndent: 4.5\n    paddingX: 2.25\n    paddingY: 1.75\n    lineThickness: 0.5', tree),
  fm('  treeView:\n    lineThickness: -2', 'treeView-beta\n  a/\n    b\n'),
  fm('  treeView:\n    useMaxWidth: false', tree),
  fm('  treeView:\n    useMaxWidth: false\n    showIcons: true', 'treeView-beta\n'),
  fm('  treeView:\n    paddingX: "5"\n    paddingY: "5"\n    rowIndent: "10"\n    lineThickness: "1"', tree),
  fm('  treeView:\n    paddingX: "7"\n    showIcons: true', 'treeView-beta\n  a/ ## d\n    b ## e\n'),
  fm('  treeView:\n    paddingX: null\n    paddingY: null\n    rowIndent: null\n    lineThickness: null', 'treeView-beta\n  a/\n    b ## d\n'),
  fm('  treeView:\n    paddingX: true\n    lineThickness: true', 'treeView-beta\n  a/\n    b ## d\n'),
  fm('  treeView:\n    paddingX: x', 'treeView-beta\n  a/\n    b :::highlight ## d\n'),
  fm('  treeView: 5', 'treeView-beta\n  a/\n    b\n'),
  fm('  theme: dark', tree),
  fm('  theme: forest\n  treeView:\n    showIcons: true', tree),
  fm('  theme: neutral', tree),
  fm('  themeVariables:\n    treeView:\n      labelFontSize: 24px\n      labelColor: red\n      lineColor: blue\n      iconColor: green\n      descriptionColor: purple\n      highlightBg: yellow\n      highlightStroke: "#000"', tree),
  fm('  themeVariables:\n    treeView:\n      labelFontSize: 10px', tree),
  fm('  fontFamily: courier', tree),
  fm('  themeVariables:\n    fontFamily: "Times New Roman"', tree),
  'treeView-beta\ntitle The title\naccTitle: The acc title\naccDescr: The description\n  a/\n    b\n',
  'treeView-beta\naccDescr {\n  multi\n  line  text\n}\ntitle A  title\n  a\n',
  '---\ntitle: Front matter title\n---\ntreeView-beta\n  a\n',
  '---\ntitle: Front matter title\n---\ntreeView-beta\ntitle Text title\n  a\n',
  'treeView-beta\n  a ## <b>bold</b> description\n  b ## <script>alert(1)</script>\n  c ## a &amp; b < c #quot;q#quot; #35;\n  d ##   spaced   text  \n  e ##\n  f ## é 😀 日本語\n',
  'treeView-beta\n  <b>bold</b>.txt\n  a&amp;b #quot;q#quot;/\n  "<i>x</i>" ## d\n',
  'treeView-beta\n  short ## one\n  a much longer file name.txt ## two\n    nested/ ## three\n  x\n',
  'treeView-beta\n  a ## only description here\n',
  'treeView-beta\n  a :::highlight ## description beyond the highlight\n    a much longer nested file name than the others.txt\n',
  'treeView-beta\n  a\n  %% a comment\n  b %% not a comment\n',
  'treeView-beta\r\n  a/\r\n    b ## d\r\n',
  '%%{init: {"treeView": {"rowIndent": 20, "showIcons": true}}}%%\ntreeView-beta\n  a/\n    b\n',
  // Box-drawing input.
  'treeView-beta\nroot/\n├── src/\n│   ├── index.js\n│   └── app.js :::highlight ## the app\n└── README.md\n',
  'treeView-beta\nroot/\n┣━━ src/\n┃   ┣━━ index.js\n┃   ┗━━ app.js\n┗━━ README.md\n',
  'treeView-beta\ntitle Box tree\nroot/\n├─ a\n│  ├─ b\n│  │  └─ c\n│\n└─ d icon(file)\n',
  'treeView-beta\n├── a\n└── b\n',
  'treeView-beta\nroot/\n├── "quoted name"\n└── 🚀 emoji.md\n',
  'treeView-beta\nroot/\n\t├── a\n\t\t└── b\n',
  'treeView-beta\nroot/\n├── a\nSection ─ A.txt\nplain\n',
  'treeView-beta\nroot/\n├── a\n%% comment\n\n└── b\n',
  // Errors.
  'treeView-beta\nroot/\n├── \n',
  'treeView-beta\nroot/\n├── a\n  plain indented\n',
  'treeView-beta\nroot/\n├── a\n├── title.txt\n',
  'treeView-beta\n  title.txt\n',
  'treeView-beta\n  a\ntitle Late\n',
  'treeView-beta\n  "unterminated\n',
  'treeView-beta\n  a icon(x\n',
  'treeView-beta x\n  a\n',
  'treeView-beta\n  "a" "b"\n',
  'treeView-beta\n  :::cls\n',
];

const map =
  'wardley-beta\n' +
  '  title Tea Shop\n' +
  '  anchor Business [0.95, 0.63]\n' +
  '  anchor Public [0.95, 0.78]\n' +
  '  component Cup of Tea [0.79, 0.61] label [19, -4]\n' +
  '  component Cup [0.73, 0.78]\n' +
  '  component Tea [0.63, 0.81]\n' +
  '  component Hot Water [0.52, 0.80]\n' +
  '  component Water [0.38, 0.82]\n' +
  '  component Kettle [0.43, 0.35] label [-57, 4]\n' +
  '  component Power [0.10, 0.70] label [-27, 20]\n' +
  '  Business -> Cup of Tea\n' +
  '  Public -> Cup of Tea\n' +
  '  Cup of Tea -> Cup\n' +
  '  Cup of Tea -> Tea\n' +
  '  Cup of Tea -> Hot Water\n' +
  '  Hot Water -> Water\n' +
  '  Hot Water -> Kettle; limited by\n' +
  '  Kettle -> Power\n' +
  '  evolve Kettle 0.62\n' +
  '  evolve Power 0.89\n' +
  '  annotation 1,[0.43, 0.49] "Standardising power allows Kettles to evolve faster"\n' +
  '  annotation 2,[0.08, 0.79] "Hot water is obvious and well known"\n' +
  '  annotations [0.72, 0.03]\n' +
  '  note "+a generic note appeared" [0.23, 0.33]\n';

const strategies =
  'wardley-beta\n' +
  '  component Built [0.80, 0.20] (build)\n' +
  '  component Bought [0.60, 0.40] (buy)\n' +
  '  component Out [0.40, 0.60] (outsource)\n' +
  '  component Mkt [0.20, 0.80] (market)\n' +
  '  component Stuck [0.85, 0.75] inertia\n' +
  '  component Paren [0.70, 0.85] (inertia)\n' +
  '  component Both [0.50, 0.15] (buy) inertia\n' +
  '  component BothParen [0.30, 0.25] (market) (inertia)\n' +
  '  component Labeled [0.15, 0.45] label [-40, 12] (outsource) inertia\n' +
  '  component LabeledBuild [0.10, 0.10] label [5, 5] (build)\n' +
  '  Built -> Bought\n' +
  '  Bought -> Out\n' +
  '  Out -> Mkt\n';

const pipelines =
  'wardley-beta\n' +
  '  size [1100, 800]\n' +
  '  component Kettle [0.57, 0.45]\n' +
  '  component Power [0.10, 0.70]\n' +
  '  component User [0.95, 0.50]\n' +
  '  Kettle -> Power\n' +
  '  User -> Electric Kettle\n' +
  '  pipeline Kettle {\n' +
  '  component Smart Kettle [0.72] label [-30, 35]\n' +
  '  component Campfire Kettle [0.35] label [-60, 35]\n' +
  '  component Electric Kettle [0.53]\n' +
  '  }\n' +
  '  Campfire Kettle -> Kettle\n' +
  '  Electric Kettle -> Kettle\n' +
  '  Kettle -> Smart Kettle\n';

const links =
  'wardley-beta\n' +
  '  component A [0.90, 0.10]\n' +
  '  component B [0.90, 0.50]\n' +
  '  component C [0.90, 0.90]\n' +
  '  component D [0.50, 0.10]\n' +
  '  component E [0.50, 0.50]\n' +
  '  component F [0.50, 0.90]\n' +
  '  component G [0.10, 0.10]\n' +
  '  component H [0.10, 0.50]\n' +
  '  component I [0.10, 0.90]\n' +
  '  A -> B\n' +
  '  B --> C\n' +
  '  A -.-> D\n' +
  '  B > E\n' +
  '  "C" "F"\n' +
  '  D +> E\n' +
  '  E +< F\n' +
  '  D +<> G\n' +
  '  E -> H +>\n' +
  '  F -> I +<\n' +
  '  G +> -> H +<\n' +
  "  H +'backup'> I\n" +
  "  G +'both'<> I\n" +
  "  A +'back'< E\n" +
  "  B +''> F; ignored\n" +
  '  C -> I; constraint\n' +
  '  A -> I;   spaced  label  \n' +
  '  D -> H; <b>bold</b> & #quot;x#quot;\n' +
  '  G -.-> E; dashed label\n';

const wardleyConfig = (config) => fm('  wardley-beta:\n' + config, map);

const wardley = [
  'wardley-beta\n',
  'wardley-beta\n  title Only a title\n',
  map,
  strategies,
  pipelines,
  links,
  'wardley-beta\n  size [400, 300]\n  component A [0.5, 0.5]\n',
  'wardley-beta\n  size [0, 0]\n  component A [0.5, 0.5]\n  component B [0.2, 0.2]\n  A -> B\n',
  'wardley-beta\n  size [1100, 800]\n  size [500, 400]\n  component A [0.5, 0.5]\n',
  'wardley-beta\n  evolution Genesis -> Custom -> Product -> Commodity\n  component A [0.5, 0.5]\n',
  'wardley-beta\n  evolution Genesis / Concept -> Custom / Emerging -> Product / Converging -> Commodity / Accepted\n',
  'wardley-beta\n  evolution Genesis@0.25 -> Custom@0.5 -> Product@0.7 -> Commodity@1.0\n',
  'wardley-beta\n  evolution A@0.2 / B -> C / D@0.4 -> E\n',
  'wardley-beta\n  evolution One -> Two\n',
  'wardley-beta\n  evolution "Quoted A" -> \'Quoted B\' -> _c1 -> D d -> "" -> X / ""\n',
  'wardley-beta\n  evolution A -> B\n  evolution C@0.1 -> D@0.9 -> E@1.0\n',
  'wardley-beta\n  evolution A@0.5 -> B@0.25 -> C@2.5\n',
  'wardley-beta\n  anchor Business [0.95, 0.63]\n  anchor "Quoted anchor" [0.5, 0.5]\n  anchor _id [0.1, 0.2]\n',
  'wardley-beta\n  anchor A [0.9, 0.9]\n  component A [0.5, 0.5] label [10, 20] (buy) inertia\n  anchor A [0.1, 0.1]\n  A -> B\n',
  'wardley-beta\n  component A [0.5, 0.5] label [10, 20] (buy) inertia\n  component A [0.2, 0.3]\n  component A [0.6, 0.7] label [-1, -2]\n',
  'wardley-beta\n  component A [0.5, 0.5] label [-0, 0]\n  component B [0.2, 0.2] label [0, -0]\n',
  'wardley-beta\n  component Low [0.0, 0.0]\n  component High [1.0, 1.0]\n  component Pct [50.5, 99.9]\n  component Mixed [0.5, 75.0]\n  component Hundred [100.0, 100.0]\n  component JustOver [1.5, 1.01]\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  component B [0.2, 0.8]\n  A -> A\n  A -> Missing\n  Missing -> B\n  B -> A\n  A -> B\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  component B [0.5, 0.5]\n  A -> B; same place\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  evolve A 0.8\n  evolve A 0.9\n  evolve Missing 0.5\n  component B [0.2, 0.2]\n  evolve B 0.21\n  evolve "A" 0.1\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  evolve A 0.5\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  evolve A 80.0\n',
  'wardley-beta\n  component Parent [0.6, 0.4] inertia\n  pipeline Parent {\n  component One [0.2]\n  }\n  One -> Parent\n  Parent -> One\n',
  'wardley-beta\n  component Parent [0.6, 0.4] (build) inertia\n  component Other [0.3, 0.2]\n  pipeline Parent {\n  component B [0.8] label [5, 5]\n  component A [0.1]\n  component C [0.5]\n  }\n  Other -> Parent\n  A -> Other\n  Parent_B -> Other\n',
  'wardley-beta\n  component P [0.6, 0.4]\n  pipeline P {\n  component A [0.3]\n  }\n  pipeline P {\n  component B [0.7]\n  }\n  A -> P\n  B -> P\n',
  'wardley-beta\n  component P [0.6, 0.4]\n  component A [0.2, 0.2]\n  pipeline P {\n  component A [0.3]\n  component A [0.5]\n  }\n  A -> P\n',
  'wardley-beta\n  component P [0.6, 0.4]\n  component Q [0.3, 0.6]\n  pipeline P {\n  component X [0.3]\n  component Y [0.5]\n  }\n  pipeline Q {\n  component X [0.6]\n  component Z [80.0]\n  }\n  X -> Q\n  evolve P 0.9\n',
  'wardley-beta\n  component Two Words [0.57, 0.45]\n  pipeline Two Words {\n  component Sub One [0.35]\n  component "Sub 2" [0.75]\n  }\n  Sub One -> Two Words\n  "Two Words_Sub One" -> "Sub 2"\n',
  'wardley-beta\n  anchor P [0.6, 0.4]\n  pipeline P {\n  component A [0.3]\n  component B [0.6]\n  }\n',
  'wardley-beta\n  note "A note" [0.5, 0.5]\n  note \'single\' [0.1, 0.9]\n  note "" [0.9, 0.1]\n  note "<b>bold</b> & #quot;q#quot;" [0.3, 0.3]\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  annotation 2,[0.1, 0.1] "second"\n  annotation 1,[0.2, 0.2] "first"\n  annotation 10,[0.3, 0.3] "tenth"\n  annotation 3,[0.4, 0.4] ""\n  annotations [0.5, 0.5]\n',
  'wardley-beta\n  annotation 1,[0.2, 0.2] "first"\n  annotation 2,[0.9, 0.9] "second with a much longer text than the first one has"\n  annotations [0.99, 0.99]\n',
  'wardley-beta\n  annotation 1,[0.2, 0.2] "first"\n  annotations [0.0, 0.0]\n',
  'wardley-beta\n  annotation 1,[0.2, 0.2] "first"\n  annotations [1.0, 0.0]\n  annotations [0.5, 0.5]\n',
  'wardley-beta\n  annotation 1,[20, 30] "percent"\n  annotation 2,[1, 1] "ones"\n  annotations [72, 3]\n',
  'wardley-beta\n  annotation 1,[0.2, 0.2] ""\n  annotations [0.5, 0.5]\n',
  'wardley-beta\n  annotation 1,[0.2, 0.2] "no box"\n  annotation 1,[0.4, 0.4] "same number"\n',
  'wardley-beta\n  annotations [0.5, 0.5]\n',
  'wardley-beta\n  annotation 5,[0.5, 0.5] "é 😀 日本語 <i>x</i>"\n  annotations [0.3, 0.6]\n',
  'wardley-beta\n  accelerator Fast [0.5, 0.5]\n  accelerator "Fast one" [0.1, 0.9]\n  deaccelerator Slow [0.3, 0.3]\n  deaccelerator Two Words [80.0, 20.0]\n',
  'wardley-beta\n  accelerator Edge [0.0, 1.0]\n  deaccelerator Edge [1.0, 0.0]\n',
  'wardley-beta\n  title <b>Bold</b> & title #35;\n  accTitle: The acc title\n  accDescr: The description\n  component A [0.5, 0.5]\n',
  'wardley-beta\n  accDescr {\n    multi\n    line  text\n  }\n  component A [0.5, 0.5]\n  title Late  title\n',
  '---\ntitle: Front matter title\n---\nwardley-beta\n  component A [0.5, 0.5]\n',
  '---\ntitle: Front matter title\n---\nwardley-beta\n  title Text title\n  component A [0.5, 0.5]\n',
  'wardley-beta\n  component "<b>bold</b>" [0.5, 0.5]\n  component "a &amp; b" [0.2, 0.2]\n  component "é 😀 日本語" [0.8, 0.8]\n  component R&D (lab) [0.3, 0.7]\n  component real-time processing [0.6, 0.3]\n',
  'wardley-beta\r\n  component A [0.5, 0.5]\r\n  component B [0.2, 0.2]\r\n  A -> B\r\n',
  'wardley-beta %% comment\n  component A [0.5, 0.5] %% c\n  %% line comment\n  A -> B %% d\n',
  'WARDLEY-BETA\n',
  wardleyConfig('    width: 700\n    height: 500\n    padding: 30'),
  wardleyConfig('    nodeRadius: 10\n    nodeLabelOffset: 14\n    axisFontSize: 16\n    labelFontSize: 13'),
  wardleyConfig('    showGrid: true'),
  wardleyConfig('    useMaxWidth: false\n    showGrid: true\n    width: 650.5\n    height: 444.4\n    padding: 22.2'),
  wardleyConfig('    padding: 0\n    nodeRadius: 0'),
  wardleyConfig('    width: "900"\n    height: "600"'),
  wardleyConfig('    padding: "48"'),
  wardleyConfig('    nodeRadius: "6"\n    nodeLabelOffset: "8"\n    axisFontSize: "12"\n    labelFontSize: "10px"'),
  wardleyConfig('    padding: null\n    nodeRadius: null\n    showGrid: null\n    useMaxWidth: null\n    width: null'),
  wardleyConfig('    padding: true\n    showGrid: 1\n    useMaxWidth: 0'),
  wardleyConfig('    padding: x'),
  fm('  wardley-beta:\n    nodeRadius: 9\n    padding: "40"', strategies),
  fm('  wardley-beta:\n    nodeRadius: "7"', strategies),
  fm('  wardley-beta:\n    nodeRadius: 12', pipelines),
  fm('  wardley-beta:\n    padding: "20"', pipelines),
  fm('  wardley-beta:\n    padding: "20"', links),
  fm('  wardley-beta:\n    padding: "20"\n    showGrid: true', 'wardley-beta\n  evolution A@0.3 -> B@1.0\n  accelerator Fast [0.5, 0.5]\n  deaccelerator Slow [0.3, 0.3]\n  note "n" [0.2, 0.2]\n  component A [0.5, 0.5]\n  evolve A 0.9\n  annotation 1,[0.2, 0.2] "first"\n  annotations [0.5, 0.5]\n'),
  fm('  wardley-beta: 5', 'wardley-beta\n  component A [0.5, 0.5]\n'),
  fm('  wardley-beta: text', 'wardley-beta\n  component A [0.5, 0.5]\n'),
  fm('  wardley-beta:\n    width: 300', 'wardley-beta\n  size [800, 200]\n  component A [0.5, 0.5]\n'),
  fm('  theme: dark', map),
  fm('  theme: forest', strategies),
  fm('  theme: neutral', pipelines),
  fm('  theme: base', links),
  fm('  theme: redux-color', map),
  fm('  theme: dark', strategies),
  fm(
    '  themeVariables:\n    wardley:\n      backgroundColor: "#eef"\n      axisColor: red\n      axisTextColor: blue\n      gridColor: green\n      componentFill: yellow\n' +
      '      componentStroke: purple\n      componentLabelColor: "#123"\n      linkStroke: orange\n      evolutionStroke: teal\n      annotationStroke: pink\n      annotationTextColor: brown\n      annotationFill: gray\n' +
      '  wardley-beta:\n    showGrid: true',
    map
  ),
  fm('  themeVariables:\n    wardley:\n      axisColor: 5\n      linkStroke: null\n      componentFill: ""\n      evolutionStroke: false', strategies),
  fm('  themeVariables:\n    wardley: null', map),
  fm('  themeVariables:\n    wardley: text', map),
  fm('  themeVariables:\n    background: "#fafafa"\n    primaryTextColor: "#0a0"', map),
  fm('  fontFamily: courier', map),
  fm('  themeVariables:\n    fontFamily: "Times New Roman"\n    fontSize: 20px', map),
  '%%{init: {"wardley-beta": {"width": 500, "showGrid": true}}}%%\nwardley-beta\n  component A [0.5, 0.5]\n',
  // Errors.
  'wardley-beta\n  component A [1.5, 150.0]\n',
  'wardley-beta\n  component A [150.0, 0.5]\n',
  'wardley-beta\n  anchor A [0.5, 100.5]\n',
  'wardley-beta\n  note "n" [0.5, 200.0]\n',
  'wardley-beta\n  component A [0.5, 0.5]\n  evolve A 100.5\n',
  'wardley-beta\n  pipeline Missing {\n  component A [0.3]\n  }\n',
  'wardley-beta\n  component P [0.5, 0.5]\n  pipeline P {\n  component A [300.0]\n  }\n',
  'wardley-beta\n  annotations [0.5, 150]\n',
  'wardley-beta\n  annotation 1,[150, 0.5] "x"\n',
  'wardley-beta\n  accelerator A [0.5, 101.0]\n',
  'wardley-beta\n  deaccelerator A [101.0, 0.5]\n',
  'wardley-beta\n  component A [5, 5]\n',
  'wardley-beta\n  component label [0.5, 0.5]\n',
  'wardley-beta\n  component A [0.5, 0.5] (other)\n',
  'wardley-beta\n  A ->\n',
  'wardley-beta\n  evolution A\n',
  'wardley-beta x\n',
];

// Random inputs. The generator is seeded, so the cases are the same in each
// run.
function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = prng(0x74637772);
const int = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));
const pick = (a) => a[Math.floor(rnd() * a.length)];
const chance = (p) => rnd() < p;
const words = [
  'Alpha', 'beta', 'Gamma ray', 'delta-v', 'Epsilon', 'zeta', 'Eta', 'theta', 'Iota', 'kappa', 'Lambda', 'mu', 'Nu', 'xi',
  'Omicron', 'pi', 'Rho', 'sigma', 'Tau', 'upsilon', 'Phi', 'chi', 'Psi', 'omega', 'Wave', 'AVATAR', 'fjord', 'Typography',
  'To Yo', 'ffi', 'WWW', 'il1', 'Quick brown', 'lazy dog',
];
const frac = () => (int(0, 100) / 100).toFixed(2);

// A random Wardley map: components at random places with random labels,
// strategies and inertia, links of every kind, evolves, and sometimes a
// pipeline, annotations, notes, accelerators, a size and evolution stages.
function randomWardley() {
  const lines = ['wardley-beta'];
  if (chance(0.5)) lines.push(`title ${pick(words)} ${pick(words)}`);
  if (chance(0.4)) lines.push(`size [${int(300, 1400)}, ${int(200, 1000)}]`);
  if (chance(0.3)) {
    const n = int(2, 6);
    const stages = [];
    const bounded = chance(0.5);
    for (let i = 0; i < n; i++) {
      let stage = `S${i}`;
      if (bounded) stage += `@${(((i + 1) / n) * (i === n - 1 ? 1 : 0.9 + rnd() * 0.1)).toFixed(2)}`;
      if (chance(0.3)) stage += ` / T${i}`;
      stages.push(stage);
    }
    lines.push('evolution ' + stages.join(' -> '));
  }
  const names = [];
  const count = int(2, 9);
  for (let i = 0; i < count; i++) {
    const name = `N${i}`;
    names.push(name);
    if (chance(0.15)) {
      lines.push(`anchor ${name} [${frac()}, ${frac()}]`);
      continue;
    }
    let line = `component ${name} [${frac()}, ${frac()}]`;
    if (chance(0.35)) line += ` label [${pick(['', '-'])}${int(0, 60)}, ${pick(['', '-'])}${int(0, 40)}]`;
    if (chance(0.3)) line += ` (${pick(['build', 'buy', 'outsource', 'market'])})`;
    if (chance(0.2)) line += pick([' inertia', ' (inertia)']);
    lines.push(line);
  }
  const arrows = ['->', '->', '->', '-->', '-.->', '>', '+>', '+<', '+<>', "+'f'>", "+'g'<", "+'h'<>"];
  const linkCount = int(0, 10);
  for (let i = 0; i < linkCount; i++) {
    const from = pick(names);
    const to = pick(names);
    if (from === to) continue;
    let line = `${from} ${pick(arrows)} ${to}`;
    if (chance(0.25)) line += `; ${pick(words)}`;
    lines.push(line);
  }
  if (chance(0.35)) {
    const parent = pick(names);
    lines.push(`pipeline ${parent} {`);
    const n = int(1, 4);
    for (let i = 0; i < n; i++) {
      lines.push(`component P${i} [${frac()}]${chance(0.3) ? ` label [${int(0, 30)}, -${int(0, 30)}]` : ''}`);
    }
    lines.push('}');
    if (chance(0.5)) lines.push(`P0 -> ${parent}`);
    if (chance(0.5)) lines.push(`${pick(names)} -> P0`);
  }
  const evolves = int(0, 3);
  for (let i = 0; i < evolves; i++) lines.push(`evolve ${pick(names)} ${frac()}`);
  if (chance(0.3)) {
    const n = int(1, 4);
    for (let i = 0; i < n; i++) lines.push(`annotation ${int(1, 12)},[${frac()}, ${frac()}] "${pick(words)} ${pick(words)}"`);
    if (chance(0.8)) lines.push(`annotations [${frac()}, ${frac()}]`);
  }
  if (chance(0.25)) lines.push(`note "${pick(words)}" [${frac()}, ${frac()}]`);
  if (chance(0.2)) lines.push(`accelerator ${pick(['Acc', '"Acc 2"'])} [${frac()}, ${frac()}]`);
  if (chance(0.2)) lines.push(`deaccelerator Dec [${frac()}, ${frac()}]`);
  const body = lines.join('\n  ') + '\n';
  if (chance(0.3)) {
    const config = [];
    if (chance(0.5)) config.push(`    nodeRadius: ${int(2, 14)}`);
    if (chance(0.5)) config.push(`    padding: ${int(0, 90)}`);
    if (chance(0.4)) config.push(`    nodeLabelOffset: ${int(0, 20)}`);
    if (chance(0.4)) config.push(`    showGrid: ${chance(0.5)}`);
    if (chance(0.3)) config.push(`    width: ${int(400, 1300)}\n    height: ${int(300, 900)}`);
    if (config.length > 0) return fm('  wardley-beta:\n' + config.join('\n'), body);
  }
  return body;
}
for (let i = 0; i < 45; i++) wardley.push(randomWardley());

// A random Cynefin diagram. The sizes are whole numbers: the engine has a
// known difference for a viewBox with other sizes.
function randomCynefin() {
  const lines = ['cynefin-beta'];
  if (chance(0.5)) lines.push(`title ${pick(words)} ${pick(words)}`);
  for (const domain of ['complex', 'complicated', 'clear', 'chaotic', 'confusion']) {
    if (chance(0.3)) continue;
    lines.push(domain);
    const n = int(0, domain === 'confusion' ? 6 : 4);
    for (let i = 0; i < n; i++) lines.push(`"${pick(words)}${chance(0.5) ? ' ' + pick(words) : ''}"`);
  }
  const domains = ['complex', 'complicated', 'clear', 'chaotic', 'confusion'];
  const transitions = int(0, 5);
  for (let i = 0; i < transitions; i++) {
    lines.push(`${pick(domains)} --> ${pick(domains)}${chance(0.6) ? ` : "${pick(words)}"` : ''}`);
  }
  const body = lines.join('\n  ') + '\n';
  const config = [];
  if (chance(0.6)) config.push(`    width: ${int(300, 1300)}\n    height: ${int(200, 1000)}`);
  if (chance(0.5)) config.push(`    padding: ${int(0, 90)}`);
  if (chance(0.6)) config.push(`    seed: ${pick([int(-1000, 1000), int(-2147483648, 2147483647), 0])}`);
  if (chance(0.5)) config.push(`    boundaryAmplitude: ${pick([int(0, 30), (rnd() * 30).toFixed(3)])}`);
  if (chance(0.3)) config.push(`    showDomainDescriptions: ${chance(0.5)}`);
  return config.length > 0 ? fm('  cynefin:\n' + config.join('\n'), body) : body;
}
for (let i = 0; i < 30; i++) cynefin.push(randomCynefin());

// A random tree with annotations and a random config.
function randomTree() {
  const lines = ['treeView-beta'];
  let depth = 0;
  const count = int(1, 14);
  for (let i = 0; i < count; i++) {
    depth = Math.max(0, Math.min(5, depth + pick([-2, -1, 0, 0, 1, 1])));
    const dir = chance(0.35);
    let line = '  '.repeat(depth + 1) + pick(words) + (dir ? '/' : pick(['.txt', '.md', '.ts', '', '.tar.gz']));
    if (chance(0.2)) line += ' :::' + pick(['highlight', 'other']);
    if (chance(0.2)) line += ` icon(${pick(['', 'none', 'file', 'folder', 'fa:bell', 'x'])})`;
    if (chance(0.25)) line += ` ## ${pick(words)} ${pick(words)}`;
    lines.push(line);
  }
  const body = lines.join('\n') + '\n';
  const config = [];
  if (chance(0.5)) config.push(`    showIcons: ${chance(0.7)}`);
  if (chance(0.4)) config.push(`    rowIndent: ${int(0, 30)}`);
  if (chance(0.4)) config.push(`    paddingX: ${int(0, 15)}`);
  if (chance(0.4)) config.push(`    paddingY: ${int(0, 15)}`);
  if (chance(0.3)) config.push(`    lineThickness: ${pick([0, 1, 2, 3, 0.5, 1.5])}`);
  if (chance(0.3)) config.push('    extensionIcons:\n      .ts: typescript\n      md: none\n      .txt: file');
  if (chance(0.2)) config.push('    defaultIconPack: fa');
  return config.length > 0 ? fm('  treeView:\n' + config.join('\n'), body) : body;
}
for (let i = 0; i < 30; i++) treeview.push(randomTree());

// Sizes that are not whole numbers: the text is measured after the viewBox
// is set, and the height of the svg comes from the ratio of the viewBox.
cynefin.push(
  fm('  cynefin:\n    width: 801.5\n    height: 600.25\n    padding: 33.3', cynefinFull),
  fm('  cynefin:\n    width: 777.7\n    height: 555.5\n    padding: 10.1', cynefinFull),
  fm('  cynefin:\n    padding: 12.34', cynefinFull),
  fm('  cynefin:\n    width: 333.3\n    height: 333.3\n    padding: 0', 'cynefin-beta\n  complex\n  "Square"\n  confusion\n  "one"\n'),
  fm('  cynefin:\n    width: 1618.034\n    height: 1000\n    padding: 0.5', cynefinFull)
);
wardley.push(
  fm('  wardley-beta:\n    width: 866.6\n    height: 666.6\n    padding: 33.3', map),
  fm('  wardley-beta:\n    width: 1000.7\n    height: 432.1', map),
  fm('  wardley-beta:\n    width: 777.7\n    height: 555.5\n    nodeRadius: 4.5', map)
);

export const EXTRA = { treeview, cynefin, wardley };
