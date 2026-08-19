package img

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func jpegBytes(t *testing.T, m image.Image) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, m, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encoding test JPEG: %v", err)
	}
	return &buf
}

func TestResize(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		opts         ResizeOptions
		wantW, wantH int
	}{
		{name: "width alone preserves aspect ratio", srcW: 100, srcH: 50, opts: ResizeOptions{Width: 40, Quality: 75}, wantW: 40, wantH: 20},
		{name: "height alone preserves aspect ratio", srcW: 100, srcH: 50, opts: ResizeOptions{Height: 25, Quality: 75}, wantW: 50, wantH: 25},
		{name: "width and height force exact dimensions", srcW: 100, srcH: 50, opts: ResizeOptions{Width: 30, Height: 30, Quality: 75}, wantW: 30, wantH: 30},
		{name: "max scales the longest side down", srcW: 100, srcH: 50, opts: ResizeOptions{Max: 50, Quality: 75}, wantW: 50, wantH: 25},
		{name: "max on a portrait image", srcW: 50, srcH: 100, opts: ResizeOptions{Max: 50, Quality: 75}, wantW: 25, wantH: 50},
		{name: "max never upscales", srcW: 100, srcH: 50, opts: ResizeOptions{Max: 400, Quality: 75}, wantW: 100, wantH: 50},
		{name: "derived side rounds half up", srcW: 640, srcH: 427, opts: ResizeOptions{Width: 100, Quality: 75}, wantW: 100, wantH: 67},
		{name: "derived side never rounds to zero", srcW: 100, srcH: 1, opts: ResizeOptions{Width: 10, Quality: 75}, wantW: 10, wantH: 1},
		{name: "upscaling with explicit width works", srcW: 10, srcH: 5, opts: ResizeOptions{Width: 20, Quality: 75}, wantW: 20, wantH: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("png", func(t *testing.T) {
				src := solidNRGBA(tt.srcW, tt.srcH, color.NRGBA{R: 200, G: 50, B: 50, A: 255})
				var out bytes.Buffer
				if err := Resize(&out, pngBytes(t, src), tt.opts); err != nil {
					t.Fatalf("Resize() error = %v", err)
				}
				decoded, err := png.Decode(&out)
				if err != nil {
					t.Fatalf("output is not a valid PNG: %v", err)
				}
				if got, want := decoded.Bounds().Size(), image.Pt(tt.wantW, tt.wantH); got != want {
					t.Errorf("output size = %v, want %v", got, want)
				}
				assertCenterColor(t, decoded, 200, 50, 50, 2)
			})
			t.Run("jpeg", func(t *testing.T) {
				src := solidNRGBA(tt.srcW, tt.srcH, color.NRGBA{R: 200, G: 50, B: 50, A: 255})
				var out bytes.Buffer
				if err := Resize(&out, jpegBytes(t, src), tt.opts); err != nil {
					t.Fatalf("Resize() error = %v", err)
				}
				decoded, err := jpeg.Decode(&out)
				if err != nil {
					t.Fatalf("output is not a valid JPEG: %v", err)
				}
				if got, want := decoded.Bounds().Size(), image.Pt(tt.wantW, tt.wantH); got != want {
					t.Errorf("output size = %v, want %v", got, want)
				}
				assertCenterColor(t, decoded, 200, 50, 50, 20)
			})
		})
	}

	t.Run("transparency survives a PNG resize", func(t *testing.T) {
		src := solidNRGBA(8, 8, color.NRGBA{R: 255, A: 128})
		var out bytes.Buffer
		if err := Resize(&out, pngBytes(t, src), ResizeOptions{Width: 4, Quality: 75}); err != nil {
			t.Fatalf("Resize() error = %v", err)
		}
		decoded, err := png.Decode(&out)
		if err != nil {
			t.Fatalf("output is not a valid PNG: %v", err)
		}
		c := color.NRGBAModel.Convert(decoded.At(2, 2)).(color.NRGBA)
		if c.A < 120 || c.A > 136 {
			t.Errorf("center alpha = %d, want ~128", c.A)
		}
	})

	t.Run("quality changes JPEG output size", func(t *testing.T) {
		src := gradientNRGBA(64, 64)
		encode := func(quality int) int {
			var out bytes.Buffer
			if err := Resize(&out, jpegBytes(t, src), ResizeOptions{Width: 32, Quality: quality}); err != nil {
				t.Fatalf("Resize() error = %v", err)
			}
			return out.Len()
		}
		if low, high := encode(10), encode(95); low >= high {
			t.Errorf("quality 10 output (%d bytes) not smaller than quality 95 output (%d bytes)", low, high)
		}
	})

	t.Run("errors", func(t *testing.T) {
		var gifBuf bytes.Buffer
		if err := gif.Encode(&gifBuf, solidNRGBA(4, 4, color.NRGBA{A: 255}), nil); err != nil {
			t.Fatal(err)
		}
		tests := []struct {
			name    string
			input   string
			opts    ResizeOptions
			wantMsg string
		}{
			{name: "empty input", input: "", opts: ResizeOptions{Width: 10, Quality: 75}, wantMsg: "decoding image"},
			{name: "corrupt input", input: "not an image", opts: ResizeOptions{Width: 10, Quality: 75}, wantMsg: "decoding image"},
			// The gif import above registers the GIF decoder, mirroring the
			// real binary where internal/inspect registers it.
			{name: "decodable but unsupported format", input: gifBuf.String(), opts: ResizeOptions{Width: 10, Quality: 75}, wantMsg: `unsupported input format "gif"`},
			{name: "no sizing mode", input: "", opts: ResizeOptions{Quality: 75}, wantMsg: "sizing mode"},
			{name: "max combined with width", input: "", opts: ResizeOptions{Width: 10, Max: 10, Quality: 75}, wantMsg: "sizing mode"},
			{name: "quality out of range", input: "", opts: ResizeOptions{Width: 10, Quality: 0}, wantMsg: "quality"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := Resize(&bytes.Buffer{}, strings.NewReader(tt.input), tt.opts)
				if err == nil {
					t.Fatal("Resize() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantMsg) {
					t.Errorf("Resize() error = %q, want it to mention %q", err, tt.wantMsg)
				}
			})
		}
	})
}

func gradientNRGBA(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: 255})
		}
	}
	return m
}
