#!/bin/sh
# Writes the golden files internal/font/testdata/skrifa-*.json.gz, which
# internal/font/glyf_var_test.go replays.
#
# The script builds the oracle with cargo in a temporary directory, so that
# no build output lands in the repository. It reads the web fonts from
# internal/font/webfontdata: each .sfnt.gz file there is the sfnt data that
# webfontdata.Font returns.
#
# Run it from any directory:
#
#   sh tools/reference/skrifa-oracle/gen.sh
set -eu

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
fonts="$root/internal/font/webfontdata/files"
out="$root/internal/font/testdata"
work=$(mktemp -d)

CARGO_TARGET_DIR="$work/target" cargo build --release --locked \
	--manifest-path "$here/Cargo.toml" </dev/null
oracle="$work/target/release/skrifa-oracle"

sizes=8,9.5,10.3,12,13.5,14,16,17.33,20,24,30,40,64
# Font sizes around the size at which the prep program of Open Sans turns
# hinting on.
enabled_sizes=7.5,8.25,8.4,8.49,8.5,8.51,8.6,8.75,8.99,9.25,9.49,9.5,10.5,255.4,255.5,300.5

# run NAME FILE OPTIONS...
run() {
	name=$1
	file=$2
	shift 2
	gzip -dc "$fonts/$file.sfnt.gz" >"$work/$name.ttf"
	"$oracle" --font "$work/$name.ttf" --name "$name" --out "$work/$name.json" \
		--sizes "$sizes" --full-sizes 9.5,16 --full-count 20 --lsb-probe 6 \
		--enabled-range 1,300 --enabled-sizes "$enabled_sizes" "$@" </dev/null
	gzip -9 -n -c "$work/$name.json" >"$out/skrifa-$name.json.gz"
}

for subset in latin latin-ext vietnamese cyrillic-ext; do
	run "recursive-$subset" "recursive/files/recursive-$subset-wght-normal" \
		--wght 300,400,450,500,700,850,1000 --full-wght 300,450,1000 \
		--norm 250,1050,3.7
done
for style in normal italic; do
	run "open-sans-latin-400-$style" "open-sans/files/open-sans-latin-400-$style" \
		--detail
done

# Variants of two of the fonts: other tables, and changed bytes. See
# src/variants.rs.
variants() {
	name=$1
	count=$2
	shift 2
	"$oracle" --variants "$count" --font "$work/$name.ttf" --name "$name" \
		--out "$work/variants-$name.json" --sizes 9.5,16 --wght 560,300 "$@" </dev/null
	gzip -9 -n -c "$work/variants-$name.json" >"$out/skrifa-variants-$name.json.gz"
}
variants recursive-latin 120 --synth-glyphs 100
variants recursive-vietnamese 300

ls -l "$out"/skrifa-*.json.gz
echo "The build directory is $work. Remove it when you do not need it."
