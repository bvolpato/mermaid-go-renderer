package mermaid

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// chromePath returns the path to a Chrome/Chromium executable, or "" if none is found.
// It checks CHROME_PATH env var first, then well-known locations per platform.
func chromePath() string {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidates := chromeSearchPaths()
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func chromeSearchPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"google-chrome",
			"chromium",
		}
	case "linux":
		return []string{
			"google-chrome-stable",
			"google-chrome",
			"chromium-browser",
			"chromium",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/snap/bin/chromium",
		}
	case "windows":
		localApp := os.Getenv("LOCALAPPDATA")
		progFiles := os.Getenv("PROGRAMFILES")
		progFiles86 := os.Getenv("PROGRAMFILES(X86)")
		paths := []string{}
		for _, base := range []string{localApp, progFiles, progFiles86} {
			if base == "" {
				continue
			}
			paths = append(paths,
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(base, "Chromium", "Application", "chrome.exe"),
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
		return paths
	default:
		return []string{"google-chrome", "chromium"}
	}
}

// renderPNGWithBrowser renders a Mermaid diagram to PNG using Chrome headless.
// It writes an HTML page that uses Mermaid.js CDN, screenshots it with Chrome,
// then crops the image to the SVG bounding box.
func renderPNGWithBrowser(mermaidCode string, outputPath string, width int, height int) error {
	chrome := chromePath()
	if chrome == "" {
		return fmt.Errorf("no Chrome/Chromium browser found")
	}

	tmpDir, err := os.MkdirTemp("", "mmdg-browser-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	htmlPath := filepath.Join(tmpDir, "render.html")
	pngPath := filepath.Join(tmpDir, "screenshot.png")

	if width <= 0 {
		width = 2400
	}
	if height <= 0 {
		height = 1600
	}

	escapedCode := html.EscapeString(mermaidCode)

	htmlContent := `<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>
<style>
* { margin: 0; padding: 0; }
body { background: white; display: inline-block; }
#container { display: inline-block; padding: 20px; }
</style>
</head>
<body>
<div id="container">
<pre class="mermaid">` + escapedCode + `</pre>
</div>
<script>
mermaid.initialize({
  startOnLoad: true,
  theme: 'default',
  flowchart: { useMaxWidth: false },
  sequence: { useMaxWidth: false }
});
mermaid.run().then(function() {
  var svg = document.querySelector('#container svg');
  if (svg) {
    var bbox = svg.getBoundingClientRect();
    document.title = 'MMDG_READY_' + Math.ceil(bbox.width + 40) + 'x' + Math.ceil(bbox.height + 40);
  } else {
    document.title = 'MMDG_READY_` + fmt.Sprintf("%d", width) + `x` + fmt.Sprintf("%d", height) + `';
  }
});
</script>
</body></html>`

	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return fmt.Errorf("write html: %w", err)
	}

	windowSize := fmt.Sprintf("%d,%d", width, height)

	args := []string{
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-software-rasterizer",
		"--disable-dev-shm-usage",
		"--hide-scrollbars",
		"--screenshot=" + pngPath,
		"--window-size=" + windowSize,
		"file://" + htmlPath,
	}

	ctx, cancel := withTimeout(30 * time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, chrome, args...)
	cmd.Env = append(os.Environ(), "DISPLAY=:99")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chrome headless failed: %w (output: %s)", err, string(output))
	}

	if _, statErr := os.Stat(pngPath); statErr != nil {
		return fmt.Errorf("chrome did not produce screenshot: %w", statErr)
	}

	screenshotData, err := os.ReadFile(pngPath)
	if err != nil {
		return fmt.Errorf("read screenshot: %w", err)
	}

	croppedData, cropErr := cropWhitespace(screenshotData)
	if cropErr == nil && len(croppedData) > 0 {
		screenshotData = croppedData
	}

	if outputPath == "" {
		_, err = os.Stdout.Write(screenshotData)
		return err
	}
	return os.WriteFile(outputPath, screenshotData, 0o644)
}
