// Command brandgen exports the mascot, browser icons, and social preview.
// Run from the repository root: go run ./tools/brandgen
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "brandgen:", err)
		os.Exit(1)
	}
}

func run() error {
	logo, err := os.ReadFile("assets/logo.svg")
	if err != nil {
		return err
	}
	var source struct {
		Inner string `xml:",innerxml"`
	}
	if err := xml.Unmarshal(logo, &source); err != nil {
		return err
	}
	// Keep the same silhouette at small sizes, with fewer interior details.
	icon := strings.Replace(string(logo), "<defs>", "<style>.detail{display:none}.outline{stroke-width:16}</style>\n  <defs>", 1)
	if err := write("icon.svg", []byte(icon)); err != nil {
		return err
	}
	for _, asset := range []struct {
		name       string
		svg        string
		size       int
		background string
	}{
		{"logo.png", string(logo), 1024, "transparent"},
		{"favicon-32.png", icon, 32, "transparent"},
		{"apple-touch-icon.png", icon, 180, "#0b2a3a"},
		{"icon-192.png", icon, 192, "#0b2a3a"},
		{"icon-512.png", icon, 512, "#0b2a3a"},
	} {
		png, err := mermaid.SVGToPNG(asset.svg, mermaid.WithScale(float64(asset.size)/512), mermaid.WithBackgroundColor(asset.background))
		if err != nil {
			return fmt.Errorf("%s: %w", asset.name, err)
		}
		if err := write(asset.name, png); err != nil {
			return err
		}
	}
	// ICO supports PNG payloads; include the native sizes used by browsers.
	var ico bytes.Buffer
	sizes := []int{16, 32, 48}
	for _, value := range []uint16{0, 1, uint16(len(sizes))} {
		_ = binary.Write(&ico, binary.LittleEndian, value)
	}
	var images [][]byte
	offset := 6 + 16*len(sizes)
	for _, size := range sizes {
		png, err := mermaid.SVGToPNG(icon, mermaid.WithScale(float64(size)/512), mermaid.WithBackgroundColor("transparent"))
		if err != nil {
			return err
		}
		ico.Write([]byte{byte(size), byte(size), 0, 0})
		for _, value := range []uint16{1, 32} {
			_ = binary.Write(&ico, binary.LittleEndian, value)
		}
		for _, value := range []uint32{uint32(len(png)), uint32(offset)} {
			_ = binary.Write(&ico, binary.LittleEndian, value)
		}
		images = append(images, png)
		offset += len(png)
	}
	for _, png := range images {
		ico.Write(png)
	}
	if err := write("favicon.ico", ico.Bytes()); err != nil {
		return err
	}
	social := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="#0b2a3a"/>
  <svg x="50" y="65" width="470" height="470" viewBox="0 0 512 512">%s</svg>
  <g font-family="Arial, sans-serif">
    <text x="565" y="270" fill="#f1fbfd" font-size="112" font-weight="700" letter-spacing="-5">mmdg</text>
    <text x="568" y="344" fill="#5eead4" font-size="35" font-weight="700">Mermaid diagrams in pure Go.</text>
    <text x="568" y="402" fill="#bcd6de" font-size="28">No browser. Just Go.</text>
    <text x="568" y="480" fill="#86a7b3" font-size="23">SVG · PNG · PDF</text>
  </g>
</svg>`, source.Inner)
	png, err := mermaid.SVGToPNG(social)
	if err != nil {
		return err
	}
	return write("social-card.png", png)
}

func write(name string, data []byte) error {
	return os.WriteFile(filepath.Join("assets", name), data, 0o644)
}
