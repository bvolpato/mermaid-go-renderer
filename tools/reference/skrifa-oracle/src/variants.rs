// Variants of a font, for the parts of internal/font/glyf_var.go that the
// web fonts do not use, and for data that is not valid.
//
// A variant is a font with other tables or with some bytes changed. The
// program makes the variants of a base font, runs skrifa on each, and writes
// the recipe of each variant with the results. The Go test makes the same
// fonts from the recipes and compares.
//
// The variants are:
//
//   - a font with an HVAR table (without a map of delta sets, and with a
//     map of format 0 and of format 1),
//   - a font whose prep program turns the backward compatibility mode of
//     the interpreter off (INSTCTRL with the selector 3), with and without
//     an hdmx table,
//   - a font with bit 3 of head.flags, which rounds the font size of the
//     hinted mode, and a font without units per em,
//   - fonts with other avar tables, for the special cases of the segment
//     maps, and a font whose axis has its default between its limits,
//   - fonts whose composite glyphs have no component, or every component,
//     with the flag USE_MY_METRICS, and fonts with composite glyphs in
//     composite glyphs,
//   - a font with another gvar table: random tuples with peaks in the
//     header or in the shared tuples, with intermediate regions, with deltas
//     for all points or for some points, with point numbers of the tuple or
//     of the glyph, and with deltas for the phantom points. The web fonts
//     have no deltas for the phantom points: their advances do not change
//     with the weight.
//   - fonts with a few bytes of one table changed: gvar, glyf, fvar, avar,
//     MVAR, and the HVAR, hdmx and gvar tables of the variants above.
//
// The tables HVAR, hdmx and gvar, and the changed bytes, come from a
// generator of pseudo-random numbers with a fixed seed.

use std::fmt::Write as _;
use std::panic::{catch_unwind, AssertUnwindSafe};

use read_fonts::{types::Tag, FontRef, TableProvider};
use skrifa::{
    instance::Size,
    metrics::GlyphMetrics,
    outline::{DrawSettings, HintingInstance},
    GlyphId, MetadataProvider,
};

use crate::{draw, draw_record, fnv1a64, fold16, join, opt_bits, skia_hinting_options, MVAR_TAGS};

type Tag4 = [u8; 4];

/// The tables of an sfnt file.
#[derive(Clone)]
struct Sfnt {
    version: Tag4,
    /// The tables, in the order of their tags.
    tables: Vec<(Tag4, Vec<u8>)>,
}

impl Sfnt {
    fn parse(data: &[u8]) -> Sfnt {
        let u16_at = |p: usize| u16::from_be_bytes([data[p], data[p + 1]]) as usize;
        let u32_at =
            |p: usize| u32::from_be_bytes([data[p], data[p + 1], data[p + 2], data[p + 3]]) as usize;
        let mut tables = vec![];
        for i in 0..u16_at(4) {
            let record = 12 + 16 * i;
            let tag: Tag4 = data[record..record + 4].try_into().unwrap();
            let (offset, length) = (u32_at(record + 8), u32_at(record + 12));
            tables.push((tag, data[offset..offset + length].to_vec()));
        }
        tables.sort_by_key(|t| t.0);
        Sfnt {
            version: data[0..4].try_into().unwrap(),
            tables,
        }
    }

    /// Writes the file: the header, the records of the tables in the order
    /// of their tags with the checksum 0, and the tables in the same order,
    /// each padded with zeros to a multiple of four bytes.
    fn build(&self) -> Vec<u8> {
        let n = self.tables.len();
        let entry_selector = (n as u32).ilog2();
        let search_range = 16 * (1u32 << entry_selector);
        let mut out = vec![];
        out.extend_from_slice(&self.version);
        out.extend_from_slice(&(n as u16).to_be_bytes());
        out.extend_from_slice(&(search_range as u16).to_be_bytes());
        out.extend_from_slice(&(entry_selector as u16).to_be_bytes());
        out.extend_from_slice(&((16 * n as u32 - search_range) as u16).to_be_bytes());
        let mut offset = 12 + 16 * n;
        for (tag, data) in &self.tables {
            out.extend_from_slice(tag);
            out.extend_from_slice(&0u32.to_be_bytes());
            out.extend_from_slice(&(offset as u32).to_be_bytes());
            out.extend_from_slice(&(data.len() as u32).to_be_bytes());
            offset += data.len().next_multiple_of(4);
        }
        for (_, data) in &self.tables {
            out.extend_from_slice(data);
            out.resize(out.len().next_multiple_of(4), 0);
        }
        out
    }

    fn table(&self, tag: &Tag4) -> Option<&Vec<u8>> {
        self.tables.iter().find(|t| &t.0 == tag).map(|t| &t.1)
    }

    fn set(&mut self, tag: Tag4, data: Vec<u8>) {
        match self.tables.iter_mut().find(|t| t.0 == tag) {
            Some(t) => t.1 = data,
            None => {
                self.tables.push((tag, data));
                self.tables.sort_by_key(|t| t.0);
            }
        }
    }
}

/// The generator splitmix64.
struct Rng(u64);

impl Rng {
    fn next(&mut self) -> u64 {
        self.0 = self.0.wrapping_add(0x9E3779B97F4A7C15);
        let mut z = self.0;
        z = (z ^ (z >> 30)).wrapping_mul(0xBF58476D1CE4E5B9);
        z = (z ^ (z >> 27)).wrapping_mul(0x94D049BB133111EB);
        z ^ (z >> 31)
    }

    fn below(&mut self, n: usize) -> usize {
        (self.next() % n as u64) as usize
    }

    /// A number from -range to range.
    fn delta(&mut self, range: i32) -> i32 {
        self.below(2 * range as usize + 1) as i32 - range
    }
}

/// A font made from the base font.
struct Variant {
    name: String,
    /// HintInstance::backward_compatibility of the hinting instance of the
    /// font. It is true unless the prep program turns the mode off.
    backward_compat: bool,
    /// Tables that replace a table of the base font, or that the base font
    /// does not have: the tag, and the index of the data in the list of the
    /// made tables.
    sets: Vec<(Tag4, usize)>,
    /// Bytes of a table that change: the tag, the position and the value.
    /// They apply after the tables of sets.
    edits: Vec<(Tag4, usize, u8)>,
}

impl Variant {
    fn apply(&self, base: &Sfnt, made: &[Vec<u8>]) -> Vec<u8> {
        let mut font = base.clone();
        for (tag, id) in &self.sets {
            font.set(*tag, made[*id].clone());
        }
        for (tag, pos, value) in &self.edits {
            let table = font.tables.iter_mut().find(|t| &t.0 == tag).unwrap();
            table.1[*pos] = *value;
        }
        font.build()
    }
}

fn be16(out: &mut Vec<u8>, v: u16) {
    out.extend_from_slice(&v.to_be_bytes());
}

fn be32(out: &mut Vec<u8>, v: u32) {
    out.extend_from_slice(&v.to_be_bytes());
}

/// An HVAR table for a font with one axis. The store has three regions and
/// two subtables: the first has a 16 bit and an 8 bit delta for each item,
/// and the second has a 32 bit and two 16 bit deltas. map selects the map of
/// the advances: 0 for no map, 1 for a map of format 0 with fewer entries
/// than the font has glyphs, and 2 for a map of format 1.
fn synth_hvar(rng: &mut Rng, glyph_count: usize, map: u8) -> Vec<u8> {
    // The ItemVariationStore.
    let mut store = vec![];
    be16(&mut store, 1); // format
    be32(&mut store, 16); // variationRegionListOffset
    be16(&mut store, 2); // itemVariationDataCount
    let region_list_len = 4 + 3 * 6;
    // The first subtable has fewer items than the font has glyphs: a glyph
    // after them has no delta.
    let item_count0 = glyph_count.saturating_sub(7).max(1);
    let data0_len = 6 + 2 * 2 + item_count0 * 3;
    be32(&mut store, 16 + region_list_len as u32);
    be32(&mut store, (16 + region_list_len + data0_len) as u32);
    // The VariationRegionList: one axis, three regions.
    be16(&mut store, 1);
    be16(&mut store, 3);
    for (start, peak, end) in [(0, 9924, 16384), (9924, 16384, 16384), (0, 16384, 16384)] {
        be16(&mut store, start);
        be16(&mut store, peak);
        be16(&mut store, end);
    }
    // ItemVariationData 0.
    be16(&mut store, item_count0 as u16);
    be16(&mut store, 1); // wordDeltaCount
    be16(&mut store, 2); // regionIndexCount
    be16(&mut store, 0);
    be16(&mut store, 1);
    for _ in 0..item_count0 {
        be16(&mut store, rng.delta(400) as i16 as u16);
        store.push(rng.delta(100) as i8 as u8);
    }
    // ItemVariationData 1, with long words.
    let item_count1 = 64;
    be16(&mut store, item_count1);
    be16(&mut store, 0x8000 | 1);
    be16(&mut store, 3);
    be16(&mut store, 2);
    be16(&mut store, 0);
    be16(&mut store, 1);
    for _ in 0..item_count1 {
        be32(&mut store, rng.delta(40000) as u32);
        be16(&mut store, rng.delta(300) as i16 as u16);
        be16(&mut store, rng.delta(300) as i16 as u16);
    }
    // The DeltaSetIndexMap of the advances.
    let mut index_map = vec![];
    match map {
        1 => {
            // Format 0: entries of two bytes, nine bits for the inner index.
            let map_count = glyph_count.saturating_sub(40).max(1);
            index_map.push(0);
            index_map.push((1 << 4) | 8);
            be16(&mut index_map, map_count as u16);
            for _ in 0..map_count {
                let (outer, inner) = if rng.below(4) == 0 {
                    (1, rng.below(70))
                } else {
                    (0, rng.below(glyph_count.min(500)))
                };
                be16(&mut index_map, ((outer << 9) | inner) as u16);
            }
        }
        2 => {
            // Format 1: entries of four bytes, sixteen bits for the inner
            // index.
            index_map.push(1);
            index_map.push((3 << 4) | 15);
            be32(&mut index_map, glyph_count as u32);
            for _ in 0..glyph_count {
                let (outer, inner) = match rng.below(8) {
                    0 => (1, rng.below(70)),
                    // An outer index without a subtable, and the index
                    // that says that the item has no variation.
                    1 => (2 + rng.below(3), rng.below(70)),
                    2 => (0xFFFF, 0xFFFF),
                    _ => (0, rng.below(glyph_count)),
                };
                be32(&mut index_map, ((outer << 16) | inner) as u32);
            }
        }
        _ => {}
    }
    let mut hvar = vec![];
    be16(&mut hvar, 1);
    be16(&mut hvar, 0);
    be32(&mut hvar, 20);
    be32(
        &mut hvar,
        if index_map.is_empty() {
            0
        } else {
            20 + store.len() as u32
        },
    );
    be32(&mut hvar, 0);
    be32(&mut hvar, 0);
    hvar.extend_from_slice(&store);
    hvar.extend_from_slice(&index_map);
    hvar
}

/// An hdmx table with a record for each of the font sizes.
fn synth_hdmx(rng: &mut Rng, glyph_count: usize, ppems: &[u8]) -> Vec<u8> {
    let record_size = (2 + glyph_count).next_multiple_of(4);
    let mut hdmx = vec![];
    be16(&mut hdmx, 0);
    be16(&mut hdmx, ppems.len() as u16);
    be32(&mut hdmx, record_size as u32);
    for ppem in ppems {
        let start = hdmx.len();
        hdmx.push(*ppem);
        hdmx.push(2 * *ppem);
        for _ in 0..glyph_count {
            hdmx.push(rng.below(2 * *ppem as usize) as u8);
        }
        hdmx.resize(start + record_size, 0);
    }
    hdmx
}

/// The packed point numbers of a tuple: the count, and runs of the
/// differences of the numbers. No numbers stand for all points.
fn pack_points(rng: &mut Rng, points: &[u16]) -> Vec<u8> {
    let n = points.len();
    let mut out = vec![];
    // A count of 128 or more has two bytes. A smaller count can have two
    // bytes also.
    if n < 128 && rng.below(8) != 0 {
        out.push(n as u8);
    } else {
        out.push((n >> 8) as u8 | 0x80);
        out.push(n as u8);
    }
    let (mut i, mut last) = (0, 0u16);
    while i < n {
        let run = (1 + rng.below(8)).min(n - i);
        let diffs: Vec<u16> = points[i..i + run]
            .iter()
            .map(|p| {
                let d = p - last;
                last = *p;
                d
            })
            .collect();
        let words = diffs.iter().any(|d| *d > 255) || rng.below(6) == 0;
        out.push((run - 1) as u8 | if words { 0x80 } else { 0 });
        for d in diffs {
            if words {
                be16(&mut out, d);
            } else {
                out.push(d as u8);
            }
        }
        i += run;
    }
    out
}

/// Packed deltas: runs of zeros, and runs of numbers of 8, 16 or 32 bits.
fn pack_deltas(rng: &mut Rng, deltas: &[i32]) -> Vec<u8> {
    let mut out = vec![];
    let mut i = 0;
    while i < deltas.len() {
        let max_run = (1 + rng.below(12)).min(deltas.len() - i);
        if deltas[i] == 0 {
            let mut run = 0;
            while run < max_run && deltas[i + run] == 0 {
                run += 1;
            }
            out.push(0x80 | (run - 1) as u8);
            i += run;
            continue;
        }
        let run = &deltas[i..i + max_run];
        // The smallest size that has all numbers of the run, and sometimes
        // a larger size.
        let size = if run.iter().all(|d| (-128..=127).contains(d)) && rng.below(8) != 0 {
            1
        } else if run.iter().all(|d| (-32768..=32767).contains(d)) && rng.below(8) != 0 {
            2
        } else {
            4
        };
        out.push((max_run - 1) as u8 | [0, 0, 0x40, 0, 0xC0][size]);
        for d in run {
            match size {
                1 => out.push(*d as i8 as u8),
                2 => be16(&mut out, *d as i16 as u16),
                _ => be32(&mut out, *d as u32),
            }
        }
        i += max_run;
    }
    out
}

/// The numbers of the points that a tuple has deltas for, of the n points of
/// a glyph (with its phantom points). No numbers stand for all points.
fn point_set(rng: &mut Rng, n: usize) -> Vec<u16> {
    let mut points: Vec<u16> = match rng.below(6) {
        0 => vec![],
        // One point: skrifa moves the contour of the point.
        1 => vec![rng.below(n) as u16],
        // A few points, and the phantom points.
        2 => (0..n)
            .filter(|i| *i + 4 >= n || rng.below(5) == 0)
            .map(|i| i as u16)
            .collect(),
        // A point after the points of the glyph.
        3 => {
            let mut p: Vec<u16> = (0..n).filter(|_| rng.below(3) == 0).map(|i| i as u16).collect();
            p.push((n + rng.below(5)) as u16);
            p
        }
        _ => (0..n).filter(|_| rng.below(3) == 0).map(|i| i as u16).collect(),
    };
    // A number can come two times.
    if !points.is_empty() && rng.below(12) == 0 {
        let k = rng.below(points.len());
        points.insert(k, points[k]);
    }
    points
}

/// A gvar table with random tuples for each glyph. counts has the number of
/// points of each simple glyph and the number of components of each
/// composite glyph. The table has shared tuples for axis_count axes. The
/// font has one axis: with two axes here, the second coordinate is 0. Only
/// the first limit glyphs get variation data.
fn synth_gvar(
    rng: &mut Rng,
    counts: &[usize],
    axis_count: usize,
    long_offsets: bool,
    limit: usize,
) -> Vec<u8> {
    // The shared tuples. The last one has the peak 0 on all axes: it
    // applies at every location.
    let shared_peaks: [i16; 5] = [9924, 16384, 5000, -9000, 0];
    let peaks: [i16; 8] = [16384, 9924, 12000, 3000, 6442, -8000, 16000, 0];
    let mut glyph_data: Vec<Vec<u8>> = vec![];
    for (gid, count) in counts.iter().enumerate() {
        let n = count + 4;
        // Some glyphs have no variation data.
        let tuple_count = if gid >= limit || rng.below(10) == 0 {
            0
        } else {
            1 + rng.below(3)
        };
        if tuple_count == 0 {
            glyph_data.push(vec![]);
            continue;
        }
        let use_shared_points = rng.below(3) == 0;
        let shared_points = point_set(rng, n);
        let mut headers = vec![];
        let mut serialized = vec![];
        if use_shared_points {
            serialized.extend_from_slice(&pack_points(rng, &shared_points));
        }
        for _ in 0..tuple_count {
            let mut tuple_index: u16;
            let mut tuples = vec![];
            let peak0: i16;
            if rng.below(2) == 0 {
                tuple_index = rng.below(shared_peaks.len()) as u16;
                peak0 = shared_peaks[tuple_index as usize];
            } else {
                // The peak is in the header. The low bits are not used
                // then.
                tuple_index = 0x8000 | rng.below(0x1000) as u16;
                peak0 = peaks[rng.below(peaks.len())];
                be16(&mut tuples, peak0 as u16);
                for _ in 1..axis_count {
                    be16(&mut tuples, if rng.below(6) == 0 { 16384 } else { 0 });
                }
            }
            if rng.below(3) == 0 {
                // An intermediate region around the peak, or sometimes a
                // region that does not have the peak.
                tuple_index |= 0x4000;
                // 6442 is the coordinate of the value 560 of wght, where
                // the oracle draws the variants: a region can start or end
                // at the coordinate.
                let p = peak0 as i32;
                let start = match rng.below(5) {
                    0 => 0,
                    1 => p / 2,
                    2 => p - 1000,
                    3 => 6442,
                    _ => p + 200,
                };
                let end = match rng.below(5) {
                    0 => 16384,
                    1 => p + (16384 - p) / 2,
                    2 => p + 500,
                    3 => 6442,
                    _ => 16384.min(p + 3000),
                };
                be16(&mut tuples, start as i16 as u16);
                for _ in 1..axis_count {
                    be16(&mut tuples, 0);
                }
                be16(&mut tuples, end as i16 as u16);
                for _ in 1..axis_count {
                    be16(&mut tuples, 16384);
                }
            }
            // The point numbers of the tuple, the point numbers of the
            // glyph, or sometimes no point numbers at all.
            let private = if use_shared_points {
                rng.below(2) == 0
            } else {
                rng.below(12) != 0
            };
            let mut data = vec![];
            let points = if private {
                tuple_index |= 0x2000;
                let points = point_set(rng, n);
                data.extend_from_slice(&pack_points(rng, &points));
                points
            } else if use_shared_points {
                shared_points.clone()
            } else {
                vec![]
            };
            // One delta of x and of y for each point of the tuple. The
            // phantom points get deltas that are not 0.
            let targets: Vec<usize> = if points.is_empty() {
                (0..n).collect()
            } else {
                points.iter().map(|p| *p as usize).collect()
            };
            let mut xs = vec![];
            let mut ys = vec![];
            for target in &targets {
                for coords in [&mut xs, &mut ys] {
                    coords.push(if *target >= *count {
                        rng.delta(150)
                    } else {
                        match rng.below(16) {
                            0..=3 => 0,
                            4 => rng.delta(3000),
                            5 => rng.delta(100000),
                            _ => rng.delta(80),
                        }
                    });
                }
            }
            // Sometimes fewer deltas than points.
            if rng.below(15) == 0 {
                ys.truncate(ys.len() / 2);
            }
            // Sometimes a run of zeros that goes past the last point.
            if rng.below(12) == 0 {
                let coords = if rng.below(2) == 0 { &mut xs } else { &mut ys };
                if let Some(last) = coords.last_mut() {
                    *last = 0;
                }
                coords.extend(std::iter::repeat(0).take(1 + rng.below(3)));
            }
            if rng.below(5) == 0 {
                // One stream: a run can have deltas of x and of y.
                xs.extend_from_slice(&ys);
                data.extend_from_slice(&pack_deltas(rng, &xs));
            } else {
                data.extend_from_slice(&pack_deltas(rng, &xs));
                data.extend_from_slice(&pack_deltas(rng, &ys));
            }
            be16(&mut headers, data.len() as u16);
            be16(&mut headers, tuple_index);
            headers.extend_from_slice(&tuples);
            serialized.extend_from_slice(&data);
        }
        let mut data = vec![];
        be16(
            &mut data,
            tuple_count as u16 | if use_shared_points { 0x8000 } else { 0 },
        );
        be16(&mut data, 4 + headers.len() as u16);
        data.extend_from_slice(&headers);
        data.extend_from_slice(&serialized);
        if data.len() % 2 == 1 {
            data.push(0);
        }
        glyph_data.push(data);
    }
    let offset_size = if long_offsets { 4 } else { 2 };
    let shared_offset = 20 + (counts.len() + 1) * offset_size;
    let data_offset = shared_offset + shared_peaks.len() * axis_count * 2;
    let mut gvar = vec![];
    be16(&mut gvar, 1);
    be16(&mut gvar, 0);
    be16(&mut gvar, axis_count as u16);
    be16(&mut gvar, shared_peaks.len() as u16);
    be32(&mut gvar, shared_offset as u32);
    be16(&mut gvar, counts.len() as u16);
    be16(&mut gvar, long_offsets as u16);
    be32(&mut gvar, data_offset as u32);
    let mut offset = 0;
    for data in glyph_data.iter().map(Some).chain([None]) {
        if long_offsets {
            be32(&mut gvar, offset as u32);
        } else {
            be16(&mut gvar, (offset / 2) as u16);
        }
        offset += data.map(|d| d.len()).unwrap_or(0);
    }
    for peak in shared_peaks {
        be16(&mut gvar, peak as u16);
        for _ in 1..axis_count {
            be16(&mut gvar, 0);
        }
    }
    for data in &glyph_data {
        gvar.extend_from_slice(data);
    }
    gvar
}

/// The number of points of each simple glyph of a font, and the number of
/// components of each composite glyph.
fn point_counts(base_data: &[u8]) -> Vec<usize> {
    use read_fonts::tables::glyf::Glyph;
    let font = FontRef::new(base_data).expect("font");
    let glyph_count = font.maxp().expect("maxp").num_glyphs() as u32;
    let (loca, glyf) = (font.loca(None).expect("loca"), font.glyf().expect("glyf"));
    (0..glyph_count)
        .map(|gid| match loca.get_glyf(GlyphId::new(gid), &glyf) {
            Ok(Some(Glyph::Simple(simple))) => simple.num_points(),
            Ok(Some(Glyph::Composite(composite))) => composite.components().count(),
            _ => 0,
        })
        .collect()
}

/// An avar table of version 1 for one axis, with the pairs of a segment
/// map.
fn synth_avar(pairs: &[(i16, i16)]) -> Vec<u8> {
    let mut avar = vec![];
    be16(&mut avar, 1);
    be16(&mut avar, 0);
    be16(&mut avar, 0);
    be16(&mut avar, 1);
    be16(&mut avar, pairs.len() as u16);
    for (from, to) in pairs {
        be16(&mut avar, *from as u16);
        be16(&mut avar, *to as u16);
    }
    avar
}

/// The segment maps of the avar variants. The coordinates 0, 4096, 8192,
/// 12288 and 16384 are the normalized values 300, 475, 650, 825 and 1000 of
/// the axis wght, so that a value can be equal to a from coordinate.
const AVAR_MAPS: [&[(i16, i16)]; 9] = [
    // One pair, and no pair.
    &[(0, 3000)],
    &[],
    // Two pairs for -1 and for +1.
    &[(-16384, -16384), (-16384, -16384), (0, 0), (8192, 4000), (16384, 16384), (16384, 16384)],
    // Two, three and four pairs with the same from coordinate.
    &[(-16384, -16384), (0, 0), (8192, 3000), (8192, 6000), (16384, 16384)],
    &[(-16384, -16384), (0, 0), (8192, 2000), (8192, 5000), (8192, 9000), (16384, 16384)],
    &[(-16384, -16384), (0, -500), (0, 100), (0, 700), (0, -50), (16384, 16384)],
    &[(-16384, -16384), (-8192, -3000), (-8192, -6000), (0, 0), (0, 0), (16384, 16384)],
    // A map that starts after the first value and ends before the last.
    &[(4096, 1000), (12288, 15000)],
    // Coordinates that are not in order.
    &[(-16384, -16384), (0, 0), (12288, 6000), (4096, 9000), (16384, 16384)],
];

/// A component of a composite glyph: the position of its flags and of its
/// glyph in the glyf table, and the flags.
struct ComponentRef {
    flags_pos: usize,
    flags: u16,
    glyph_pos: usize,
}

/// The composite glyphs of a font, with their components.
fn composites(base: &Sfnt) -> Vec<(usize, Vec<ComponentRef>)> {
    let head = base.table(b"head").expect("head");
    let loca = base.table(b"loca").expect("loca");
    let glyf = base.table(b"glyf").expect("glyf");
    let long = head[51] == 1;
    let offset_at = |g: usize| {
        if long {
            u32::from_be_bytes(loca[4 * g..4 * g + 4].try_into().unwrap()) as usize
        } else {
            2 * u16::from_be_bytes([loca[2 * g], loca[2 * g + 1]]) as usize
        }
    };
    let glyph_count = loca.len() / if long { 4 } else { 2 } - 1;
    let mut out = vec![];
    for gid in 0..glyph_count {
        let (start, end) = (offset_at(gid), offset_at(gid + 1));
        if end <= start || i16::from_be_bytes([glyf[start], glyf[start + 1]]) >= 0 {
            continue;
        }
        let mut components = vec![];
        let mut p = start + 10;
        loop {
            let flags = u16::from_be_bytes([glyf[p], glyf[p + 1]]);
            components.push(ComponentRef {
                flags_pos: p,
                flags,
                glyph_pos: p + 2,
            });
            p += 4 + if flags & 0x0001 != 0 { 4 } else { 2 };
            p += match flags {
                f if f & 0x0008 != 0 => 2,
                f if f & 0x0040 != 0 => 4,
                f if f & 0x0080 != 0 => 8,
                _ => 0,
            };
            if flags & 0x0020 == 0 {
                break;
            }
        }
        out.push((gid, components));
    }
    out
}

/// The variants of a base font: the fixed variants, and fuzz variants with
/// changed bytes. made gets the tables that the variants add or replace.
fn recipes(
    base: &Sfnt,
    base_data: &[u8],
    fuzz: usize,
    synth_glyphs: usize,
    seed: u64,
    made: &mut Vec<Vec<u8>>,
) -> Vec<Variant> {
    let mut rng = Rng(seed);
    let counts = point_counts(base_data);
    let glyph_count = counts.len();
    // The made tables, by their index in made.
    const HVAR: usize = 0;
    const HVAR_MAP0: usize = 1;
    const HVAR_MAP1: usize = 2;
    const HDMX: usize = 3;
    const PREP: usize = 4;
    const GVAR_A: usize = 5;
    const GVAR_B: usize = 6;
    const GVAR_C: usize = 7;
    made.push(synth_hvar(&mut rng, glyph_count, 0));
    made.push(synth_hvar(&mut rng, glyph_count, 1));
    made.push(synth_hvar(&mut rng, glyph_count, 2));
    made.push(synth_hdmx(&mut rng, glyph_count, &[8, 9, 12, 14, 16, 20, 24, 30, 40, 64]));
    // PUSHB[2] 4 3, INSTCTRL: the selector 3 with the value 4 turns the
    // backward compatibility mode off.
    let mut prep = vec![0xB1, 4, 3, 0x8E];
    prep.extend_from_slice(base.table(b"prep").map(|t| t.as_slice()).unwrap_or(&[]));
    made.push(prep);
    made.push(synth_gvar(&mut rng, &counts, 1, false, synth_glyphs));
    made.push(synth_gvar(&mut rng, &counts, 1, true, synth_glyphs));
    made.push(synth_gvar(&mut rng, &counts, 2, false, synth_glyphs));
    let head_flags = base.table(b"head").expect("head")[17];
    let int_ppem: (Tag4, usize, u8) = (*b"head", 17, head_flags | 8);
    // A font without units per em: skrifa does not scale its outlines.
    let upem0: Vec<(Tag4, usize, u8)> = vec![(*b"head", 18, 0), (*b"head", 19, 0)];
    // The flag USE_MY_METRICS of every component goes away: a composite
    // glyph then keeps its own phantom points, with their deltas.
    let composite_glyphs = composites(base);
    let mut no_use_my_metrics = vec![];
    for (_, components) in &composite_glyphs {
        for c in components {
            if c.flags & 0x0200 != 0 {
                no_use_my_metrics.push((*b"glyf", c.flags_pos, (c.flags >> 8) as u8 & !0x02));
            }
        }
    }
    // Every fourth composite glyph gets the next two composite glyphs as
    // its first two components: composite glyphs in composite glyphs.
    let mut nested = vec![];
    for k in (0..composite_glyphs.len().saturating_sub(2)).step_by(4) {
        let components = &composite_glyphs[k].1;
        if components.len() < 2 {
            continue;
        }
        for (c, target) in components.iter().zip([k + 1, k + 2]) {
            let gid = composite_glyphs[target].0 as u16;
            nested.push((*b"glyf", c.glyph_pos, (gid >> 8) as u8));
            nested.push((*b"glyf", c.glyph_pos + 1, gid as u8));
        }
    }
    let nested_no_use_my_metrics: Vec<_> =
        nested.iter().chain(&no_use_my_metrics).copied().collect();
    // Every component gets the flag USE_MY_METRICS: the last one counts.
    let mut all_use_my_metrics = vec![];
    for (_, components) in &composite_glyphs {
        for c in components {
            all_use_my_metrics.push((*b"glyf", c.flags_pos, (c.flags >> 8) as u8 | 0x02));
        }
    }
    // The default of the axis becomes 500, so that a value can be less
    // than the default.
    let fvar = base.table(b"fvar").expect("fvar");
    let axis = u16::from_be_bytes([fvar[4], fvar[5]]) as usize;
    let fvar_default: Vec<(Tag4, usize, u8)> = [0x01u8, 0xF4, 0, 0]
        .iter()
        .enumerate()
        .map(|(i, b)| (*b"fvar", axis + 8 + i, *b))
        .collect();
    // The offset of the first subtable of the store of MVAR becomes null:
    // an item of it has no delta.
    let mvar = base.table(b"MVAR").expect("MVAR");
    let store = u16::from_be_bytes([mvar[10], mvar[11]]) as usize;
    let mvar_null: Vec<(Tag4, usize, u8)> =
        (0..4).map(|i| (*b"MVAR", store + 8 + i, 0)).collect();
    let avar_base = made.len();
    for pairs in AVAR_MAPS {
        made.push(synth_avar(pairs));
    }

    let variant = |name: &str,
                   backward_compat: bool,
                   sets: &[(&Tag4, usize)],
                   edits: Vec<(Tag4, usize, u8)>| Variant {
        name: name.to_string(),
        backward_compat,
        sets: sets.iter().map(|(tag, id)| (**tag, *id)).collect(),
        edits,
    };
    let mut variants = vec![
        variant("base", true, &[], vec![]),
        variant("int-ppem", true, &[], vec![int_ppem]),
        variant("hvar", true, &[(b"HVAR", HVAR)], vec![]),
        variant("hvar-map0", true, &[(b"HVAR", HVAR_MAP0)], vec![]),
        variant("hvar-map1", true, &[(b"HVAR", HVAR_MAP1)], vec![]),
        variant("no-bc", false, &[(b"prep", PREP)], vec![]),
        variant("no-bc-hdmx", false, &[(b"prep", PREP), (b"hdmx", HDMX)], vec![]),
        variant("hdmx", true, &[(b"hdmx", HDMX)], vec![]),
        variant("hvar-no-bc", false, &[(b"HVAR", HVAR), (b"prep", PREP)], vec![]),
        variant(
            "int-ppem-no-bc-hdmx",
            false,
            &[(b"prep", PREP), (b"hdmx", HDMX)],
            vec![int_ppem],
        ),
        variant("gvar-a", true, &[(b"gvar", GVAR_A)], vec![]),
        variant("gvar-b", true, &[(b"gvar", GVAR_B)], vec![]),
        variant("gvar-c", true, &[(b"gvar", GVAR_C)], vec![]),
        variant("gvar-a-hvar", true, &[(b"gvar", GVAR_A), (b"HVAR", HVAR)], vec![]),
        variant(
            "gvar-b-hvar-map1",
            true,
            &[(b"gvar", GVAR_B), (b"HVAR", HVAR_MAP1)],
            vec![],
        ),
        variant("gvar-a-no-bc", false, &[(b"gvar", GVAR_A), (b"prep", PREP)], vec![]),
        variant(
            "gvar-b-no-bc-hdmx",
            false,
            &[(b"gvar", GVAR_B), (b"prep", PREP), (b"hdmx", HDMX)],
            vec![],
        ),
        variant(
            "gvar-a-hvar-no-bc-hdmx-int-ppem",
            false,
            &[(b"gvar", GVAR_A), (b"HVAR", HVAR), (b"prep", PREP), (b"hdmx", HDMX)],
            vec![int_ppem],
        ),
        variant("upem0", true, &[], upem0.clone()),
        variant("gvar-a-upem0", true, &[(b"gvar", GVAR_A)], upem0.clone()),
        variant(
            "gvar-b-hvar-upem0-no-bc",
            false,
            &[(b"gvar", GVAR_B), (b"HVAR", HVAR), (b"prep", PREP)],
            upem0.clone(),
        ),
        variant(
            "upem0-no-bc-hdmx",
            false,
            &[(b"prep", PREP), (b"hdmx", HDMX)],
            upem0.clone(),
        ),
        variant("fvar-default", true, &[], fvar_default.clone()),
        variant("gvar-a-fvar-default", true, &[(b"gvar", GVAR_A)], fvar_default.clone()),
        variant("mvar-null", true, &[], mvar_null.clone()),
        variant("all-umm", true, &[], all_use_my_metrics.clone()),
        variant("gvar-a-all-umm", true, &[(b"gvar", GVAR_A)], all_use_my_metrics.clone()),
        variant(
            "gvar-b-hvar-all-umm",
            true,
            &[(b"gvar", GVAR_B), (b"HVAR", HVAR)],
            all_use_my_metrics.clone(),
        ),
        variant("no-umm", true, &[], no_use_my_metrics.clone()),
        variant("nested", true, &[], nested.clone()),
        variant("gvar-a-no-umm", true, &[(b"gvar", GVAR_A)], no_use_my_metrics.clone()),
        variant("gvar-b-no-umm-no-bc", false, &[(b"gvar", GVAR_B), (b"prep", PREP)], no_use_my_metrics.clone()),
        variant("gvar-a-nested", true, &[(b"gvar", GVAR_A)], nested.clone()),
        variant("gvar-b-nested", true, &[(b"gvar", GVAR_B)], nested.clone()),
        variant("gvar-c-nested-no-umm", true, &[(b"gvar", GVAR_C)], nested_no_use_my_metrics.clone()),
        variant(
            "gvar-b-hvar-nested-no-umm",
            true,
            &[(b"gvar", GVAR_B), (b"HVAR", HVAR)],
            nested_no_use_my_metrics.clone(),
        ),
    ];

    for i in 0..AVAR_MAPS.len() {
        variants.push(variant(&format!("avar-{i}"), true, &[(b"avar", avar_base + i)], vec![]));
        variants.push(variant(
            &format!("avar-{i}-fvar-default"),
            true,
            &[(b"avar", avar_base + i)],
            fvar_default.clone(),
        ));
    }

    // The fuzz variants. Each kind changes bytes of one table.
    let kinds = [
        "gvar", "gvar-data", "gvar-headers", "gvar-a", "gvar-headers", "gvar-b", "glyf", "gvar-a",
        "fvar", "avar", "MVAR", "hvar", "hvar-map0", "hvar-map1", "hdmx", "gvar-c", "glyf",
        "gvar-a-hvar", "gvar-b", "gvar-a-nested", "gvar-b-nested",
    ];
    let gvar = base.table(b"gvar").cloned().unwrap_or_default();
    for k in 0..fuzz {
        let kind = kinds[k % kinds.len()];
        let mut sets: Vec<(&Tag4, usize)> = vec![];
        let mut backward_compat = true;
        // The table whose bytes change, and the range of the bytes.
        let (tag, table, range): (Tag4, Vec<u8>, std::ops::Range<usize>) = match kind {
            "gvar" => (*b"gvar", gvar.clone(), 0..gvar.len()),
            "gvar-data" => {
                // The variation data of the glyphs.
                let start = u32::from_be_bytes([gvar[16], gvar[17], gvar[18], gvar[19]]) as usize;
                (*b"gvar", gvar.clone(), start.min(gvar.len() - 1)..gvar.len())
            }
            "gvar-headers" => {
                // The first bytes of the variation data of one glyph: the
                // count of the tuples, the offset of the data, and the
                // headers of the first tuples. The offsets of the data have
                // 16 bits here.
                let start = u32::from_be_bytes([gvar[16], gvar[17], gvar[18], gvar[19]]) as usize;
                let gid = rng.below(glyph_count);
                let offset_at =
                    |g: usize| 2 * u16::from_be_bytes([gvar[20 + 2 * g], gvar[21 + 2 * g]]) as usize;
                let (lo, hi) = (start + offset_at(gid), start + offset_at(gid + 1));
                if hi > lo {
                    (*b"gvar", gvar.clone(), lo..hi.min(lo + 20))
                } else {
                    (*b"gvar", gvar.clone(), 0..gvar.len())
                }
            }
            "gvar-a" | "gvar-b" | "gvar-c" | "gvar-a-hvar" | "gvar-a-nested" | "gvar-b-nested" => {
                let id = match kind {
                    "gvar-b" | "gvar-b-nested" => GVAR_B,
                    "gvar-c" => GVAR_C,
                    _ => GVAR_A,
                };
                sets.push((b"gvar", id));
                if kind == "gvar-a-hvar" {
                    sets.push((b"HVAR", HVAR));
                }
                let table = made[id].clone();
                // The header with the offsets and the shared tuples, or
                // the whole table.
                let range = match rng.below(4) {
                    0 => 0..(20 + 2 * glyph_count + 30).min(table.len()),
                    _ => 0..table.len(),
                };
                (*b"gvar", table, range)
            }
            "hvar" | "hvar-map0" | "hvar-map1" => {
                let id = match kind {
                    "hvar" => HVAR,
                    "hvar-map0" => HVAR_MAP0,
                    _ => HVAR_MAP1,
                };
                sets.push((b"HVAR", id));
                let table = made[id].clone();
                // The header and the start of the store, or the whole
                // table.
                let range = match rng.below(3) {
                    0 => 0..60,
                    _ => 0..table.len(),
                };
                (*b"HVAR", table, range)
            }
            "hdmx" => {
                sets.push((b"prep", PREP));
                sets.push((b"hdmx", HDMX));
                backward_compat = false;
                let table = made[HDMX].clone();
                let range = match rng.below(2) {
                    0 => 0..8,
                    _ => 0..table.len(),
                };
                (*b"hdmx", table, range)
            }
            other => {
                let tag: Tag4 = other.as_bytes().try_into().unwrap();
                let table = base.table(&tag).cloned().unwrap_or_default();
                let len = table.len();
                (tag, table, 0..len)
            }
        };
        if range.is_empty() {
            continue;
        }
        // The variants with composite glyphs in composite glyphs change the
        // glyf table also.
        let mut edits = if kind.ends_with("-nested") {
            nested_no_use_my_metrics.clone()
        } else {
            vec![]
        };
        for _ in 0..1 + rng.below(4) {
            let pos = range.start + rng.below(range.len());
            // A new value, or one bit of the old value changed.
            let value = match rng.below(3) {
                0 => table[pos] ^ (1 << rng.below(8)),
                _ => rng.below(256) as u8,
            };
            edits.push((tag, pos, value));
        }
        variants.push(variant(&format!("fuzz-{k}-{kind}"), backward_compat, &sets, edits));
    }
    variants
}

/// What skrifa returns for a variant at one location.
#[derive(Default)]
struct Outcome {
    coords: Vec<i16>,
    mvar: Vec<Option<i32>>,
    adv_hash: u64,
    instance: Vec<bool>,
    enabled: Vec<bool>,
    /// The hashes of the draws without hinting and of the hinted draws, for
    /// each glyph and size.
    draws: [Vec<u64>; 2],
}

impl Outcome {
    fn draw_hash(&self, mode: usize) -> u64 {
        let mut bytes = Vec::with_capacity(self.draws[mode].len() * 8);
        for h in &self.draws[mode] {
            bytes.extend_from_slice(&h.to_be_bytes());
        }
        fnv1a64(&bytes)
    }
}

/// What skrifa returns for a variant: the number of glyphs,
/// OutlineGlyph::has_hinting of each glyph, and the result at each
/// location.
struct FontOutcome {
    glyph_count: u32,
    has_hinting: String,
    /// The normalized coordinates of the values of NORM_WGHT.
    norm: Vec<Vec<i16>>,
    locations: Vec<Outcome>,
}

/// The values of wght whose normalized coordinates the file has for each
/// variant: 250, 275 and so on to 1050.
fn norm_wght() -> Vec<f32> {
    (0..33).map(|k| 250.0 + 25.0 * k as f32).collect()
}

/// Runs skrifa on a font, at each value of wght. The first value gets the
/// draws of the glyphs. The other values get the coordinates, the deltas of
/// MVAR and the advances only. The result is None for data that FontRef does
/// not accept.
fn evaluate(data: &[u8], wghts: &[f32], sizes: &[f32]) -> Option<FontOutcome> {
    let font = FontRef::new(data).ok()?;
    let glyph_count = font.maxp().map(|m| m.num_glyphs() as u32).unwrap_or(0);
    let outlines = font.outline_glyphs();
    let has_hinting: Vec<Option<bool>> = (0..glyph_count)
        .map(|gid| outlines.get(GlyphId::new(gid)).and_then(|g| g.has_hinting()))
        .collect();
    let mvar = font.mvar().ok();
    let mut locations = vec![];
    for (li, wght) in wghts.iter().enumerate() {
        let mut out = Outcome::default();
        let location = font.axes().location([("wght", *wght)]);
        out.coords = location.coords().iter().map(|c| c.to_bits()).collect();
        for tag in MVAR_TAGS {
            out.mvar.push(
                mvar.as_ref()
                    .and_then(|mvar| mvar.metric_delta(Tag::new(tag), location.coords()).ok())
                    .map(|d| d.to_bits()),
            );
        }
        let unscaled = GlyphMetrics::new(&font, Size::unscaled(), &location);
        let instances: Vec<Option<HintingInstance>> = sizes
            .iter()
            .map(|size| {
                HintingInstance::new(&outlines, Size::new(*size), &location, skia_hinting_options())
                    .ok()
            })
            .collect();
        out.instance = instances.iter().map(|i| i.is_some()).collect();
        out.enabled = instances
            .iter()
            .map(|i| i.as_ref().map(|i| i.is_enabled()).unwrap_or(false))
            .collect();
        let mut adv_bytes = vec![];
        for gid in 0..glyph_count {
            let glyph_id = GlyphId::new(gid);
            let glyph = outlines.get(glyph_id);
            adv_bytes.extend_from_slice(&opt_bits(unscaled.advance_width(glyph_id)));
            if li != 0 {
                continue;
            }
            for (si, size) in sizes.iter().enumerate() {
                let unhinted = draw(
                    glyph.as_ref(),
                    DrawSettings::unhinted(Size::new(*size), &location),
                );
                out.draws[0].push(fnv1a64(&draw_record(&unhinted)));
                // The hinted draw of a glyph with instructions needs the
                // interpreter. The Go port does not have it, so the hash
                // has a marker for such a glyph.
                let hinted = if has_hinting[gid as usize] == Some(true) {
                    vec![0xDD]
                } else {
                    draw_record(&match &instances[si] {
                        Some(instance) => {
                            draw(glyph.as_ref(), DrawSettings::hinted(instance, false))
                        }
                        None => unhinted.clone(),
                    })
                };
                out.draws[1].push(fnv1a64(&hinted));
            }
        }
        out.adv_hash = fnv1a64(&adv_bytes);
        locations.push(out);
    }
    Some(FontOutcome {
        norm: norm_wght()
            .iter()
            .map(|w| {
                let location = font.axes().location([("wght", *w)]);
                location.coords().iter().map(|c| c.to_bits()).collect()
            })
            .collect(),
        glyph_count,
        has_hinting: has_hinting
            .iter()
            .map(|h| match h {
                Some(true) => '1',
                Some(false) => '0',
                None => '2',
            })
            .collect(),
        locations,
    })
}

fn hex(data: &[u8]) -> String {
    let mut s = String::with_capacity(2 * data.len());
    for b in data {
        write!(s, "{b:02x}").unwrap();
    }
    s
}

fn tag_str(tag: &Tag4) -> &str {
    std::str::from_utf8(tag).unwrap()
}

/// Writes the variants of a base font and their results.
pub fn run(
    base_data: &[u8],
    name: &str,
    wghts: &[f32],
    sizes: &[f32],
    size_names: &[String],
    fuzz: usize,
    synth_glyphs: usize,
    out_path: &str,
) {
    // A panic of skrifa is a result here, not a message.
    std::panic::set_hook(Box::new(|_| {}));
    let base = Sfnt::parse(base_data);
    let mut made = vec![];
    let variants = recipes(&base, base_data, fuzz, synth_glyphs, 0x5EED, &mut made);
    let mut out = String::new();
    write!(
        out,
        "{{\"base\":\"{name}\",\"wght\":[{}],\"normWght\":[{}],\"sizes\":[{}],\"sizeBits\":[{}],\"tables\":[{}],\"variants\":[",
        join(wghts.iter()),
        join(norm_wght().iter()),
        size_names
            .iter()
            .map(|s| format!("\"{s}\""))
            .collect::<Vec<_>>()
            .join(","),
        join(sizes.iter().map(|s| s.to_bits())),
        made.iter()
            .map(|data| format!("\n\"{}\"", hex(data)))
            .collect::<Vec<_>>()
            .join(","),
    )
    .unwrap();
    let mut base_outcome: Option<FontOutcome> = None;
    let (mut panics, mut rejected, mut changed_variants) = (0, 0, 0);
    for (vi, variant) in variants.iter().enumerate() {
        let data = variant.apply(&base, &made);
        if vi > 0 {
            out.push(',');
        }
        write!(
            out,
            "\n{{\"name\":\"{}\",\"backwardCompat\":{},\"set\":{{{}}},\"edits\":[{}],\"fontHash\":\"{:016x}\",",
            variant.name,
            variant.backward_compat,
            variant
                .sets
                .iter()
                .map(|(tag, id)| format!("\"{}\":{id}", tag_str(tag)))
                .collect::<Vec<_>>()
                .join(","),
            variant
                .edits
                .iter()
                .map(|(tag, pos, value)| format!("[\"{}\",{pos},{value}]", tag_str(tag)))
                .collect::<Vec<_>>()
                .join(","),
            fnv1a64(&data),
        )
        .unwrap();
        let outcome = match catch_unwind(AssertUnwindSafe(|| evaluate(&data, wghts, sizes))) {
            Err(_) => {
                panics += 1;
                out.push_str("\"panic\":true}");
                continue;
            }
            Ok(None) => {
                rejected += 1;
                out.push_str("\"font\":false}");
                continue;
            }
            Ok(Some(outcome)) => outcome,
        };
        write!(
            out,
            "\"glyphCount\":{},\"hasHintingHash\":\"{:016x}\",\"norm\":[{}],\"locations\":[",
            outcome.glyph_count,
            fnv1a64(outcome.has_hinting.as_bytes()),
            outcome
                .norm
                .iter()
                .map(|coords| format!("[{}]", join(coords.iter())))
                .collect::<Vec<_>>()
                .join(","),
        )
        .unwrap();
        let mut any_changed = false;
        for (li, loc) in outcome.locations.iter().enumerate() {
            if li > 0 {
                out.push(',');
            }
            write!(
                out,
                "{{\"coords\":[{}],\"mvar\":[{}],\"advHash\":\"{:016x}\",",
                join(loc.coords.iter()),
                loc.mvar
                    .iter()
                    .map(|d| d.map(|d| d.to_string()).unwrap_or("null".to_string()))
                    .collect::<Vec<_>>()
                    .join(","),
                loc.adv_hash,
            )
            .unwrap();
            if li == 0 {
                write!(
                    out,
                    "\"instance\":[{}],\"enabled\":[{}],\"unhinted\":\"{:016x}\",\"hinted\":\"{:016x}\",",
                    join(loc.instance.iter()),
                    join(loc.enabled.iter()),
                    loc.draw_hash(0),
                    loc.draw_hash(1),
                )
                .unwrap();
            }
            // The draws that differ from the draws of the base font: the
            // glyph, the index of the size, the mode (0 without hinting, 1
            // hinted) and the tag of the draw. The list has at most 24
            // draws.
            let mut changed = vec![];
            let mut changed_count = 0;
            let mut adv_changed = false;
            if let Some(base_loc) = base_outcome.as_ref().map(|b| &b.locations[li]) {
                adv_changed = base_loc.adv_hash != loc.adv_hash;
                for mode in 0..2 {
                    for (i, h) in loc.draws[mode].iter().enumerate() {
                        if base_loc.draws[mode].get(i) != Some(h) {
                            changed_count += 1;
                            if changed.len() < 24 {
                                changed.push(format!(
                                    "[{},{},{mode},{}]",
                                    i / sizes.len(),
                                    i % sizes.len(),
                                    fold16(*h)
                                ));
                            }
                        }
                    }
                }
            }
            any_changed |= changed_count != 0 || adv_changed;
            write!(
                out,
                "\"advChanged\":{adv_changed},\"changedCount\":{changed_count},\"changed\":[{}]}}",
                changed.join(",")
            )
            .unwrap();
        }
        out.push_str("]}");
        if any_changed {
            changed_variants += 1;
        }
        if vi == 0 {
            base_outcome = Some(outcome);
        }
    }
    out.push_str("]}\n");
    std::fs::write(out_path, out).expect("output file");
    eprintln!(
        "{name}: {} variants, {changed_variants} with results that differ from the base font, {panics} panics of skrifa, {rejected} fonts that FontRef does not accept",
        variants.len()
    );
}
