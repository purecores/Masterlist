package main

import (
	_ "embed"
	"embed"
	"log"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var iconFiles embed.FS

//go:embed resources/icon.png
var appIconBytes []byte

var (
	icons         = map[string]*ui.SVG{}
	appIconSource ui.ImageSource
)

func init() {
	if len(appIconBytes) > 0 {
		src, err := ui.DecodeImage(appIconBytes)
		if err != nil {
			log.Printf("failed to decode app icon: %v", err)
		} else {
			appIconSource = src
		}
	}
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
