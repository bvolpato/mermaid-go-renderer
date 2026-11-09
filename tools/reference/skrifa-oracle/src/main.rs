// The test oracle of internal/font/glyf_var.go.
//
// Skia in Chromium 156 on Linux reads and scales glyphs with the Fontations
// backend: the crates skrifa 0.46.2, read-fonts 0.43.2 and font-types
// 0.12.5. This program runs the same crates for a font file, a list of
// values of the wght axis and a list of font sizes, for every glyph, and
// writes what they return as JSON. internal/font/glyf_var_test.go replays
// the JSON against the Go port.
//
// The pen is a copy of VerbsPointsPen of Skia (revision aa789d9b,
// src/ports/fontations/src/verbs_points_pen.rs; BSD 3-clause license), so
// the verbs and the points are what Skia gets from skrifa. The hinting
// instance has the options of make_hinting_instance in
// src/ports/fontations/src/hinting.rs for the hinting mode "normal".
//
// Usage:
//
//   skrifa-oracle --font FILE --name NAME --out FILE.json --sizes 8,9.5,...
//       [--wght 300,400,...] [--full-wght 300,450] [--full-sizes 9.5,16]
//       [--full-count 20] [--detail] [--norm 250,1050,3.7]
//       [--enabled-range 1,300] [--enabled-sizes 8.4,8.5,...]
//       [--lsb-probe 6]
//
//   skrifa-oracle --variants 300 --font FILE --name NAME --out FILE.json
//       --sizes 9.5,16 --wght 560,300 [--synth-glyphs 100]
//
// The second form writes variants of the font and what skrifa returns for
// them: see variants.rs. The number is the count of the variants with
// changed bytes. The first value of wght gets the draws of the glyphs.
// --synth-glyphs is the number of glyphs that get variation data in the
// gvar tables that the program makes.
//
// gen.sh in this directory builds the program outside of the repository and
// writes the golden files of internal/font/testdata.
//
// The JSON has, for each location (a value of wght, or the default
// location of a font without axes):
//
//   coords            the normalized coordinates (F2Dot14 bits)
//   mvar              Mvar::metric_delta for some tags (Fixed bits)
//   metrics           for each size: ascent, descent, leading, cap_height,
//                     x_height, underline offset and thickness, strikeout
//                     offset and thickness of Metrics::new (f32 bits)
//   enabled           for each size: HintingInstance::is_enabled
//   unscaledAdvance   for each glyph: GlyphMetrics::advance_width in font
//                     units
//   linHash           a hash of GlyphMetrics::advance_width for each glyph
//                     and size (f32 bits)
//   unhinted, hinted  for each glyph one row: the hash of the row, and a tag
//                     of 16 bits for each size
//   hintedDetail      with --detail, for each glyph and size: the advance,
//                     the left side bearing, the number of points and the
//                     bounds of the points of the hinted path, in 26.6 units
//   paths             the full result for some glyphs and sizes
//
// And for the font: hasHinting (OutlineGlyph::has_hinting of each glyph),
// with --norm the normalized coordinate of many values of wght, with
// --enabled-range HintingInstance::is_enabled for each whole font size of a
// range at the default location, with --enabled-sizes the same for a list
// of font sizes, and with --lsb-probe the probe of the backward
// compatibility mode of the interpreter (see lsb_probe).
//
// The hash of one draw is FNV-1a (64 bits) of the bytes of draw_record. The
// tag of a draw is its hash folded to 16 bits. The hash of a row is FNV-1a
// of the hashes of its draws as big-endian bytes.

mod variants;

use std::fmt::Write as _;

use read_fonts::{tables::mvar::Mvar, types::Tag, FontRef, TableProvider};
use skrifa::{
    instance::{Location, Size},
    metrics::{GlyphMetrics, Metrics},
    outline::{
        DrawSettings, Engine, HintingInstance, HintingOptions, OutlineGlyph, OutlinePen,
        SmoothMode, Target,
    },
    GlyphId, MetadataProvider,
};

#[derive(Copy, Clone, Default, PartialEq)]
struct FfiPoint {
    x: f32,
    y: f32,
}

impl FfiPoint {
    fn new(x: f32, y: f32) -> Self {
        Self { x, y }
    }
}

// Values need to match SkPathVerb.
#[repr(u8)]
enum PathVerb {
    MoveTo = 0,
    LineTo = 1,
    QuadTo = 2,
    CubicTo = 4,
    Close = 5,
}

/// VerbsPointsPen of Skia.
struct VerbsPointsPen<'a> {
    verbs: &'a mut Vec<u8>,
    points: &'a mut Vec<FfiPoint>,
    started: bool,
    current: FfiPoint,
}

impl<'a> VerbsPointsPen<'a> {
    fn new(verbs: &'a mut Vec<u8>, points: &'a mut Vec<FfiPoint>) -> Self {
        verbs.clear();
        points.clear();
        Self {
            verbs,
            points,
            started: false,
            current: FfiPoint::default(),
        }
    }

    fn going_to(&mut self, point: &FfiPoint) {
        if !self.started {
            self.started = true;
            self.verbs.push(PathVerb::MoveTo as u8);
            self.points.push(self.current);
        }
        self.current = *point;
    }

    fn current_is_not(&self, point: &FfiPoint) -> bool {
        self.current != *point
    }
}

impl<'a> OutlinePen for VerbsPointsPen<'a> {
    fn move_to(&mut self, x: f32, y: f32) {
        let pt0 = FfiPoint::new(x, -y);
        if self.started {
            self.close();
            self.started = false;
        }
        self.current = pt0;
    }

    fn line_to(&mut self, x: f32, y: f32) {
        let pt0 = FfiPoint::new(x, -y);
        if self.current_is_not(&pt0) {
            self.going_to(&pt0);
            self.verbs.push(PathVerb::LineTo as u8);
            self.points.push(pt0);
        }
    }

    fn quad_to(&mut self, cx0: f32, cy0: f32, x: f32, y: f32) {
        let pt0 = FfiPoint::new(cx0, -cy0);
        let pt1 = FfiPoint::new(x, -y);
        if self.current_is_not(&pt0) || self.current_is_not(&pt1) {
            self.going_to(&pt1);
            self.verbs.push(PathVerb::QuadTo as u8);
            self.points.push(pt0);
            self.points.push(pt1);
        }
    }

    fn curve_to(&mut self, cx0: f32, cy0: f32, cx1: f32, cy1: f32, x: f32, y: f32) {
        let pt0 = FfiPoint::new(cx0, -cy0);
        let pt1 = FfiPoint::new(cx1, -cy1);
        let pt2 = FfiPoint::new(x, -y);
        if self.current_is_not(&pt0) || self.current_is_not(&pt1) || self.current_is_not(&pt2) {
            self.going_to(&pt2);
            self.verbs.push(PathVerb::CubicTo as u8);
            self.points.push(pt0);
            self.points.push(pt1);
            self.points.push(pt2);
        }
    }

    fn close(&mut self) {
        if let Some(verb) = self.verbs.last().cloned() {
            if verb == PathVerb::QuadTo as u8
                || verb == PathVerb::CubicTo as u8
                || verb == PathVerb::LineTo as u8
                || verb == PathVerb::MoveTo as u8
            {
                self.verbs.push(PathVerb::Close as u8);
            }
        }
    }
}

/// The result of one draw of a glyph, as get_path_verbs_points of Skia
/// returns it.
#[derive(Default, Clone)]
struct Draw {
    /// False when skrifa returns an error: Skia then has no path.
    ok: bool,
    advance: Option<f32>,
    lsb: Option<f32>,
    verbs: Vec<u8>,
    points: Vec<FfiPoint>,
}

fn draw(glyph: Option<&OutlineGlyph>, settings: DrawSettings) -> Draw {
    let mut out = Draw::default();
    let Some(glyph) = glyph else {
        return out;
    };
    let mut pen = VerbsPointsPen::new(&mut out.verbs, &mut out.points);
    match glyph.draw(settings, &mut pen) {
        Err(_) => {
            out.verbs.clear();
            out.points.clear();
        }
        Ok(metrics) => {
            out.ok = true;
            out.advance = metrics.advance_width;
            out.lsb = metrics.lsb;
        }
    }
    out
}

fn opt_bits(v: Option<f32>) -> [u8; 5] {
    match v {
        Some(v) => {
            let b = v.to_bits().to_be_bytes();
            [1, b[0], b[1], b[2], b[3]]
        }
        None => [0; 5],
    }
}

/// The bytes that the hash of a draw covers: a status byte, and for a draw
/// without an error the advance and the left side bearing (a byte 1 and the
/// f32 bits, or five zero bytes for None), the number of verbs, the verbs,
/// the number of points, and the f32 bits of x and y of each point. All
/// numbers are big-endian.
fn draw_record(d: &Draw) -> Vec<u8> {
    if !d.ok {
        return vec![0xEE];
    }
    let mut r = vec![0x01];
    r.extend_from_slice(&opt_bits(d.advance));
    r.extend_from_slice(&opt_bits(d.lsb));
    r.extend_from_slice(&(d.verbs.len() as u32).to_be_bytes());
    r.extend_from_slice(&d.verbs);
    r.extend_from_slice(&(d.points.len() as u32).to_be_bytes());
    for p in &d.points {
        r.extend_from_slice(&p.x.to_bits().to_be_bytes());
        r.extend_from_slice(&p.y.to_bits().to_be_bytes());
    }
    r
}

fn fnv1a64(bytes: &[u8]) -> u64 {
    let mut h: u64 = 0xcbf29ce484222325;
    for b in bytes {
        h ^= *b as u64;
        h = h.wrapping_mul(0x100000001b3);
    }
    h
}

fn fold16(h: u64) -> u16 {
    (h ^ (h >> 16) ^ (h >> 32) ^ (h >> 48)) as u16
}

/// A row of the golden file: the hash of the row and the tag of each draw.
fn row(hashes: &[u64]) -> String {
    let mut bytes = Vec::with_capacity(hashes.len() * 8);
    for h in hashes {
        bytes.extend_from_slice(&h.to_be_bytes());
    }
    let mut s = format!("{:016x}", fnv1a64(&bytes));
    for h in hashes {
        write!(s, "{:04x}", fold16(*h)).unwrap();
    }
    s
}

/// A pixel value as a whole number of 26.6 units. Every value that skrifa
/// gives to a pen for a TrueType outline is such a number.
fn to_26dot6(v: f32) -> i32 {
    let scaled = v * 64.0;
    assert!(
        scaled.fract() == 0.0 && scaled.abs() < 16777216.0,
        "{v} is not a 26.6 number"
    );
    scaled as i32
}

fn json_opt_bits(v: Option<f32>) -> String {
    match v {
        Some(v) => v.to_bits().to_string(),
        None => "null".to_string(),
    }
}

fn json_draw(d: &Draw) -> String {
    if !d.ok {
        return "{\"err\":true}".to_string();
    }
    let verbs: String = d.verbs.iter().map(|v| (b'0' + v) as char).collect();
    let mut s = format!(
        "{{\"adv\":{},\"lsb\":{},\"v\":\"{}\",\"p\":[",
        json_opt_bits(d.advance),
        json_opt_bits(d.lsb),
        verbs
    );
    for (i, p) in d.points.iter().enumerate() {
        if i > 0 {
            s.push(',');
        }
        write!(s, "{},{}", to_26dot6(p.x), to_26dot6(p.y)).unwrap();
    }
    s.push_str("]}");
    s
}

/// The detail of a hinted draw: the advance and the left side bearing in
/// 26.6 units, the number of points, and the bounds of the points.
fn json_detail(d: &Draw) -> String {
    if !d.ok {
        return "null".to_string();
    }
    let adv = to_26dot6(d.advance.expect("advance"));
    let lsb = to_26dot6(d.lsb.expect("lsb"));
    if d.points.is_empty() {
        return format!("[{adv},{lsb},0]");
    }
    let (mut x0, mut y0, mut x1, mut y1) = (i32::MAX, i32::MAX, i32::MIN, i32::MIN);
    for p in &d.points {
        let (x, y) = (to_26dot6(p.x), to_26dot6(p.y));
        x0 = x0.min(x);
        y0 = y0.min(y);
        x1 = x1.max(x);
        y1 = y1.max(y);
    }
    format!("[{adv},{lsb},{},{x0},{y0},{x1},{y1}]", d.points.len())
}

fn parse_f32_list(s: &str) -> Vec<f32> {
    s.split(',')
        .filter(|p| !p.is_empty())
        .map(|p| p.parse::<f32>().expect("number"))
        .collect()
}

fn join<T: ToString>(items: impl IntoIterator<Item = T>) -> String {
    items
        .into_iter()
        .map(|v| v.to_string())
        .collect::<Vec<_>>()
        .join(",")
}

/// The tags of the MVAR table that the file records: the tags that
/// Metrics::new of skrifa reads, and two more. The records of the two in the
/// MVAR table of Recursive have the index that says that the metric does
/// not change.
const MVAR_TAGS: [&[u8; 4]; 11] = [
    b"hasc", b"hdsc", b"hlgp", b"cpht", b"xhgt", b"undo", b"unds", b"stro", b"strs", b"hcrs",
    b"sbxo",
];

/// The change of the left side bearing of the glyphs of lsb_probe, in font
/// units.
const LSB_PROBE_DELTA: i16 = -7;

/// The probe of the backward compatibility mode of the interpreter.
///
/// skrifa does not say whether a hinting instance has the mode on
/// (HintInstance::backward_compatibility). The hinted draw shows it: with the
/// mode off, the scaler rounds the phantom points of a glyph, and the left
/// side bearing of AdjustedMetrics is a whole number of pixels. In most
/// fonts the first phantom point is at x 0, where the rounding changes
/// nothing. So the probe makes a copy of the font in which some glyphs have
/// a left side bearing in hmtx that is 7 units less: their first phantom
/// point is then between two pixels.
///
/// The glyphs are the first count glyphs with instructions and the first
/// count glyphs without instructions that have a path. The result has, for
/// each of these glyphs and each font size, at the default location: the
/// glyph, the index of the size, the left side bearing of the draw without
/// hinting, the left side bearing and the advance of the hinted draw (f32
/// bits), and the tag of the hinted draw.
fn lsb_probe(data: &[u8], sizes: &[f32], count: usize) -> String {
    let font = FontRef::new(data).expect("font");
    let outlines = font.outline_glyphs();
    let glyph_count = font.maxp().expect("maxp").num_glyphs() as u32;
    let location = font.axes().location::<[(&str, f32); 0]>([]);
    let mut glyphs: Vec<u32> = vec![];
    for with_instructions in [true, false] {
        let mut found = 0;
        for gid in 0..glyph_count {
            let glyph = outlines.get(GlyphId::new(gid));
            let has_path = !draw(
                glyph.as_ref(),
                DrawSettings::unhinted(Size::new(16.0), &location),
            )
            .points
            .is_empty();
            if has_path
                && glyph.and_then(|g| g.has_hinting()) == Some(with_instructions)
                && found < count
            {
                glyphs.push(gid);
                found += 1;
            }
        }
    }
    // The copy of the font with the other left side bearings. The bearing
    // of a glyph is in its record of hmtx, or for a glyph after the records
    // in the array of bearings.
    let mut patched = data.to_vec();
    let table = |tag: &[u8; 4]| {
        let count = u16::from_be_bytes([data[4], data[5]]) as usize;
        (0..count)
            .map(|i| &data[12 + 16 * i..28 + 16 * i])
            .find(|record| &record[..4] == tag)
            .map(|record| u32::from_be_bytes(record[8..12].try_into().unwrap()) as usize)
            .expect("table")
    };
    let (hhea, hmtx) = (table(b"hhea"), table(b"hmtx"));
    let metrics = u16::from_be_bytes([data[hhea + 34], data[hhea + 35]]) as u32;
    for gid in &glyphs {
        let pos = if *gid < metrics {
            hmtx + 4 * *gid as usize + 2
        } else {
            hmtx + 4 * metrics as usize + 2 * (*gid - metrics) as usize
        };
        let lsb = i16::from_be_bytes([data[pos], data[pos + 1]]).wrapping_add(LSB_PROBE_DELTA);
        patched[pos..pos + 2].copy_from_slice(&lsb.to_be_bytes());
    }
    let font = FontRef::new(&patched).expect("font");
    let outlines = font.outline_glyphs();
    let mut rows = vec![];
    for (si, size) in sizes.iter().enumerate() {
        let instance =
            HintingInstance::new(&outlines, Size::new(*size), &location, skia_hinting_options())
                .ok();
        for gid in &glyphs {
            let glyph = outlines.get(GlyphId::new(*gid));
            let unhinted = draw(
                glyph.as_ref(),
                DrawSettings::unhinted(Size::new(*size), &location),
            );
            let hinted = match &instance {
                Some(instance) => draw(glyph.as_ref(), DrawSettings::hinted(instance, false)),
                None => unhinted.clone(),
            };
            rows.push(format!(
                "[{gid},{si},{},{},{},{}]",
                unhinted.lsb.expect("lsb").to_bits(),
                hinted.lsb.expect("lsb").to_bits(),
                hinted.advance.expect("advance").to_bits(),
                fold16(fnv1a64(&draw_record(&hinted))),
            ));
        }
    }
    format!(
        "\"lsbProbe\":{{\"delta\":{LSB_PROBE_DELTA},\"glyphs\":[{}],\"rows\":[{}]}},",
        join(glyphs.iter()),
        rows.join(",")
    )
}

/// The options of make_hinting_instance of Skia for the hinting mode
/// "normal": no light hinting, no LCD antialiasing, and no forced automatic
/// hinter.
fn skia_hinting_options() -> HintingOptions {
    HintingOptions {
        engine: Engine::AutoFallback,
        target: Target::Smooth {
            mode: SmoothMode::Normal,
            symmetric_rendering: true,
            preserve_linear_metrics: false,
        },
    }
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let mut font_path = String::new();
    let mut name = String::new();
    let mut out_path = String::new();
    let mut sizes: Vec<f32> = vec![];
    let mut size_names: Vec<String> = vec![];
    let mut wghts: Vec<f32> = vec![];
    let mut full_wghts: Vec<f32> = vec![];
    let mut full_sizes: Vec<f32> = vec![];
    let mut full_count = 20usize;
    let mut detail = false;
    let mut norm: Vec<f32> = vec![];
    let mut enabled_range: Vec<f32> = vec![];
    let mut enabled_sizes: Vec<f32> = vec![];
    let mut probe_count = 0usize;
    let mut fuzz: Option<usize> = None;
    let mut synth_glyphs = usize::MAX;
    let mut i = 1;
    while i < args.len() {
        let value = || args.get(i + 1).expect("value of the option").clone();
        match args[i].as_str() {
            "--font" => font_path = value(),
            "--name" => name = value(),
            "--out" => out_path = value(),
            "--sizes" => {
                sizes = parse_f32_list(&value());
                size_names = value().split(',').map(|s| s.to_string()).collect();
            }
            "--wght" => wghts = parse_f32_list(&value()),
            "--full-wght" => full_wghts = parse_f32_list(&value()),
            "--full-sizes" => full_sizes = parse_f32_list(&value()),
            "--full-count" => full_count = value().parse().expect("count"),
            "--norm" => norm = parse_f32_list(&value()),
            "--enabled-range" => enabled_range = parse_f32_list(&value()),
            "--enabled-sizes" => enabled_sizes = parse_f32_list(&value()),
            "--lsb-probe" => probe_count = value().parse().expect("count"),
            "--variants" => fuzz = Some(value().parse().expect("count")),
            "--synth-glyphs" => synth_glyphs = value().parse().expect("count"),
            "--detail" => {
                detail = true;
                i += 1;
                continue;
            }
            other => panic!("unknown option {other}"),
        }
        i += 2;
    }
    let data = std::fs::read(&font_path).expect("font file");
    if let Some(fuzz) = fuzz {
        variants::run(
            &data,
            &name,
            &wghts,
            &sizes,
            &size_names,
            fuzz,
            synth_glyphs,
            &out_path,
        );
        return;
    }
    let font = FontRef::new(&data).expect("font");
    let glyph_count = font.maxp().expect("maxp").num_glyphs() as u32;
    let outlines = font.outline_glyphs();
    let mvar: Option<Mvar> = font.mvar().ok();

    // The glyphs with a full record: about full_count glyphs, spread over
    // the font.
    let step = (glyph_count as usize).div_ceil(full_count.max(1)).max(1) as u32;
    let full_glyphs: Vec<u32> = (0..glyph_count).filter(|g| g % step == 0).collect();
    let full_size_ix: Vec<usize> = sizes
        .iter()
        .enumerate()
        .filter(|(_, s)| full_sizes.contains(s))
        .map(|(i, _)| i)
        .collect();

    let mut out = String::new();
    write!(
        out,
        "{{\"font\":\"{name}\",\"skrifa\":\"0.46.2\",\"glyphCount\":{glyph_count},\"sizes\":[{}],\"sizeBits\":[{}],\"fullGlyphs\":[{}],\"fullSizes\":[{}],",
        size_names
            .iter()
            .map(|s| format!("\"{s}\""))
            .collect::<Vec<_>>()
            .join(","),
        join(sizes.iter().map(|s| s.to_bits())),
        join(full_glyphs.iter()),
        join(full_size_ix.iter()),
    )
    .unwrap();
    // OutlineGlyph::has_hinting of each glyph: the glyph or a component of
    // it has instructions.
    let has_hinting: Vec<&str> = (0..glyph_count)
        .map(|gid| {
            match outlines
                .get(GlyphId::new(gid))
                .and_then(|g| g.has_hinting())
            {
                Some(true) => "1",
                Some(false) => "0",
                None => "2",
            }
        })
        .collect();
    write!(out, "\"hasHinting\":\"{}\",", has_hinting.concat()).unwrap();

    // The normalized coordinate of the values lo, lo + step, ... of wght:
    // pairs of the f32 bits of the value and the F2Dot14 bits.
    if let [lo, hi, step] = norm[..] {
        let mut pairs = vec![];
        let mut k = 0;
        loop {
            let value = lo + k as f32 * step;
            if value > hi {
                break;
            }
            let location = font.axes().location([("wght", value)]);
            pairs.push(format!(
                "[{},{}]",
                value.to_bits(),
                join(location.coords().iter().map(|c| c.to_bits()))
            ));
            k += 1;
        }
        write!(out, "\"normalize\":[{}],", pairs.join(",")).unwrap();
    }

    // HintingInstance::is_enabled for each whole font size of a range, at
    // the default location: 1 for enabled, 0 for not enabled, 2 for a font
    // size without a hinting instance.
    if let [lo, hi] = enabled_range[..] {
        let location = font.axes().location::<[(&str, f32); 0]>([]);
        let values: Vec<&str> = (lo as u32..=hi as u32)
            .map(|ppem| {
                match HintingInstance::new(
                    &outlines,
                    Size::new(ppem as f32),
                    &location,
                    skia_hinting_options(),
                ) {
                    Ok(instance) if instance.is_enabled() => "1",
                    Ok(_) => "0",
                    Err(_) => "2",
                }
            })
            .collect();
        write!(
            out,
            "\"enabledByPpem\":{{\"from\":{},\"values\":\"{}\"}},",
            lo as u32,
            values.concat()
        )
        .unwrap();
    }
    // HintingInstance::is_enabled for font sizes that are not whole
    // numbers: pairs of the f32 bits of the size and 1, 0 or 2 as above.
    if !enabled_sizes.is_empty() {
        let location = font.axes().location::<[(&str, f32); 0]>([]);
        let pairs: Vec<String> = enabled_sizes
            .iter()
            .map(|size| {
                let state = match HintingInstance::new(
                    &outlines,
                    Size::new(*size),
                    &location,
                    skia_hinting_options(),
                ) {
                    Ok(instance) if instance.is_enabled() => 1,
                    Ok(_) => 0,
                    Err(_) => 2,
                };
                format!("[{},{state}]", size.to_bits())
            })
            .collect();
        write!(out, "\"enabledBySize\":[{}],", pairs.join(",")).unwrap();
    }
    if probe_count > 0 {
        out.push_str(&lsb_probe(&data, &sizes, probe_count));
    }
    out.push_str("\"locations\":[");

    // A font without the option --wght has one location: no settings.
    let locations: Vec<Option<f32>> = if wghts.is_empty() {
        vec![None]
    } else {
        wghts.iter().map(|w| Some(*w)).collect()
    };
    let (mut draws, mut errors) = (0usize, 0usize);
    for (li, wght) in locations.iter().enumerate() {
        let location: Location = match wght {
            Some(w) => font.axes().location([("wght", *w)]),
            None => font.axes().location::<[(&str, f32); 0]>([]),
        };
        let is_full_location = match wght {
            Some(w) => full_wghts.contains(w),
            None => true,
        };
        if li > 0 {
            out.push(',');
        }
        write!(
            out,
            "\n{{\"wght\":{},\"coords\":[{}],",
            wght.map(|w| w.to_string()).unwrap_or("null".to_string()),
            join(location.coords().iter().map(|c| c.to_bits())),
        )
        .unwrap();

        // Mvar::metric_delta at the coordinates: the bits of the Fixed, or
        // null for an error.
        out.push_str("\"mvar\":{");
        for (ti, tag) in MVAR_TAGS.iter().enumerate() {
            if ti > 0 {
                out.push(',');
            }
            let delta = mvar
                .as_ref()
                .and_then(|mvar| mvar.metric_delta(Tag::new(tag), location.coords()).ok());
            write!(
                out,
                "\"{}\":{}",
                std::str::from_utf8(*tag).unwrap(),
                delta
                    .map(|d| d.to_bits().to_string())
                    .unwrap_or("null".to_string())
            )
            .unwrap();
        }
        out.push_str("},\"metrics\":[");
        for (si, size) in sizes.iter().enumerate() {
            let m = Metrics::new(&font, Size::new(*size), &location);
            if si > 0 {
                out.push(',');
            }
            write!(
                out,
                "[{},{},{},{},{},{},{},{},{}]",
                m.ascent.to_bits(),
                m.descent.to_bits(),
                m.leading.to_bits(),
                json_opt_bits(m.cap_height),
                json_opt_bits(m.x_height),
                json_opt_bits(m.underline.map(|d| d.offset)),
                json_opt_bits(m.underline.map(|d| d.thickness)),
                json_opt_bits(m.strikeout.map(|d| d.offset)),
                json_opt_bits(m.strikeout.map(|d| d.thickness)),
            )
            .unwrap();
        }
        out.push_str("],");

        // The advance of each glyph in font units.
        let unscaled = GlyphMetrics::new(&font, Size::unscaled(), &location);
        let unscaled_advances: Vec<String> = (0..glyph_count)
            .map(|gid| match unscaled.advance_width(GlyphId::new(gid)) {
                Some(a) => {
                    assert!(a.fract() == 0.0);
                    (a as i32).to_string()
                }
                None => "null".to_string(),
            })
            .collect();
        write!(out, "\"unscaledAdvance\":[{}],", unscaled_advances.join(",")).unwrap();

        // The hinting instance of Skia for each size.
        let instances: Vec<Option<HintingInstance>> = sizes
            .iter()
            .map(|size| {
                HintingInstance::new(
                    &outlines,
                    Size::new(*size),
                    &location,
                    skia_hinting_options(),
                )
                .ok()
            })
            .collect();
        write!(
            out,
            "\"instance\":[{}],\"enabled\":[{}],",
            join(instances.iter().map(|i| i.is_some())),
            join(
                instances
                    .iter()
                    .map(|i| i.as_ref().map(|i| i.is_enabled()).unwrap_or(false))
            ),
        )
        .unwrap();
        let glyph_metrics: Vec<GlyphMetrics> = sizes
            .iter()
            .map(|size| GlyphMetrics::new(&font, Size::new(*size), &location))
            .collect();

        let mut lin_bytes: Vec<u8> = vec![];
        let mut unhinted_rows: Vec<String> = vec![];
        let mut hinted_rows: Vec<String> = vec![];
        let mut details: Vec<String> = vec![];
        let mut paths: Vec<String> = vec![];
        for gid in 0..glyph_count {
            let glyph_id = GlyphId::new(gid);
            let glyph = outlines.get(glyph_id);
            let mut unhinted_hashes = vec![];
            let mut hinted_hashes = vec![];
            let mut glyph_details = vec![];
            for (si, size) in sizes.iter().enumerate() {
                let lin = glyph_metrics[si].advance_width(glyph_id);
                lin_bytes.extend_from_slice(&opt_bits(lin));
                let unhinted = draw(
                    glyph.as_ref(),
                    DrawSettings::unhinted(Size::new(*size), &location),
                );
                // get_path_verbs_points: without a hinting instance Skia
                // draws without hinting.
                let hinted = match &instances[si] {
                    Some(instance) => draw(glyph.as_ref(), DrawSettings::hinted(instance, false)),
                    None => unhinted.clone(),
                };
                draws += 2;
                errors += usize::from(!unhinted.ok) + usize::from(!hinted.ok);
                unhinted_hashes.push(fnv1a64(&draw_record(&unhinted)));
                hinted_hashes.push(fnv1a64(&draw_record(&hinted)));
                if detail {
                    glyph_details.push(json_detail(&hinted));
                }
                if is_full_location && full_glyphs.contains(&gid) && full_size_ix.contains(&si) {
                    paths.push(format!(
                        "{{\"g\":{gid},\"s\":{si},\"lin\":{},\"u\":{},\"h\":{}}}",
                        json_opt_bits(lin),
                        json_draw(&unhinted),
                        json_draw(&hinted)
                    ));
                }
            }
            unhinted_rows.push(format!("\"{}\"", row(&unhinted_hashes)));
            hinted_rows.push(format!("\"{}\"", row(&hinted_hashes)));
            if detail {
                details.push(format!("[{}]", glyph_details.join(",")));
            }
        }
        write!(
            out,
            "\"linHash\":\"{:016x}\",\n\"unhinted\":[{}],\n\"hinted\":[{}],",
            fnv1a64(&lin_bytes),
            unhinted_rows.join(","),
            hinted_rows.join(","),
        )
        .unwrap();
        if detail {
            write!(out, "\n\"hintedDetail\":[{}],", details.join(",")).unwrap();
        }
        write!(out, "\n\"paths\":[{}]}}", paths.join(",\n")).unwrap();
    }
    out.push_str("]}\n");
    std::fs::write(&out_path, out).expect("output file");
    eprintln!(
        "{name}: {glyph_count} glyphs, {} locations, {} sizes, {draws} draws, {errors} draw errors",
        locations.len(),
        sizes.len()
    );
}
