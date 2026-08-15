package imageutil_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/qutaq/GophProfile/internal/pkg/imageutil"
)

func TestResizeJPEG(t *testing.T) {
	src := solidPNG(t, 200, 150)

	out, err := imageutil.ResizeJPEG(src, 100, 100)
	if err != nil {
		t.Fatalf("ResizeJPEG: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}

	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q", format)
	}
	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 100 {
		t.Fatalf("size = %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func solidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 144, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
