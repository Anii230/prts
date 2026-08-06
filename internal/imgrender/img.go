// Package imgrender renders images as ANSI truecolor half-blocks for the
// terminal. Each output cell is 2 vertical pixels, giving posters decent
// resolution without external tools.
package imgrender

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"golang.org/x/image/webp"

	"prts/internal/httpget"
)

// Render downloads url, scales it to fit cols x rows cells while preserving
// aspect ratio (letterboxing with the background color), and returns an ANSI
// string composed of half-block characters.
func Render(ctx context.Context, url string, cols, rows int, bgColor string) (string, error) {
	if cols <= 0 || rows <= 0 {
		return "", fmt.Errorf("invalid size %dx%d", cols, rows)
	}

	data, err := httpget.Get(ctx, url)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		img, err = webp.Decode(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("decode image: %w", err)
		}
	}

	src := img.Bounds()
	sw, sh := src.Dx(), src.Dy()
	targetW, targetH := cols, rows*2 // targetW x targetH pixels

	// Fit while preserving aspect ratio.
	scale := float64(targetW) / float64(sw)
	if float64(sh)*scale > float64(targetH) {
		scale = float64(targetH) / float64(sh)
	}
	w := int(float64(sw) * scale)
	h := int(float64(sh) * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	ox := (targetW - w) / 2
	oy := (targetH - h) / 2

	var b strings.Builder
	for cellY := 0; cellY < rows; cellY++ {
		for cellX := 0; cellX < cols; cellX++ {
			top := sample(img, src, ((cellX-ox)*sw)/w, ((cellY*2-oy)*sh)/h)
			bottom := sample(img, src, ((cellX-ox)*sw)/w, ((cellY*2+1-oy)*sh)/h)

			if top.A == 0 && bottom.A == 0 {
				b.WriteString(blank(bgColor))
				continue
			}
			b.WriteString("\x1b[38;2;")
			b.WriteString(rgb(top))
			b.WriteString("m\x1b[48;2;")
			b.WriteString(rgb(bottom))
			b.WriteString("m▀")
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String(), nil
}

type pixel struct{ R, G, B, A uint32 }

func sample(img image.Image, bounds image.Rectangle, x, y int) pixel {
	// Nearest-neighbor with clamping (letterbox areas map outside the image).
	if x < 0 || y < 0 || x >= bounds.Dx() || y >= bounds.Dy() {
		return pixel{}
	}
	r, g, bb, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
	return pixel{r >> 8, g >> 8, bb >> 8, a >> 8}
}

func rgb(p pixel) string {
	return fmt.Sprintf("%d;%d;%d", p.R, p.G, p.B)
}

// blank returns a colored space for empty cells (used for letterboxing).
func blank(bg string) string {
	if bg == "" {
		return " "
	}
	return "\x1b[38;2;" + bg + "m\x1b[48;2;" + bg + "m " + "\x1b[0m"
}
