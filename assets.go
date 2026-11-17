package main

import (
	"bytes"
	"embed"
	_ "embed"
	"image"
	"image/png"
	"log"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

//go:embed icons/*.svg
var iconFiles embed.FS

//go:embed resources/icon.png
var appIconBytes []byte

var (
	icons         = map[string]*ui.SVG{}
	appIconSource ui.ImageSource
	// trayIconBytes is the app icon at the size Windows shows in the
	// notification area (32 px, crisp at 150% scale).
	trayIconBytes []byte
)

func init() {
	if len(appIconBytes) > 0 {
		src, err := ui.DecodeImage(appIconBytes)
		if err != nil {
			log.Printf("failed to decode app icon: %v", err)
		} else {
			appIconSource = src
		}
		trayIconBytes = resizePNG(appIconBytes, 32)
	}
}

// resizePNG scales a PNG to a size×size square, keeping its alpha.
func resizePNG(data []byte, size int) []byte {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("failed to decode tray icon: %v", err)
		return nil
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		log.Printf("failed to encode tray icon: %v", err)
		return nil
	}
	return buf.Bytes()
}

func icon(name string) *ui.SVG {
	if s, ok := icons[name]; ok {
		return s
	}
	data, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		log.Printf("icon %q not found: %v", name, err)
		return nil
	}
	s := ui.MustParseSVG(data)
	icons[name] = s
	return s
}
