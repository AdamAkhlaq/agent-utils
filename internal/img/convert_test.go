package img

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, m image.Image) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return &buf
}

func solidNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		switch i % 4 {
		case 0:
			m.Pix[i] = c.R
		case 1:
			m.Pix[i] = c.G
		case 2:
			m.Pix[i] = c.B
		case 3:
			m.Pix[i] = c.A
		}
	}
	return m
}

// JPEG is lossy, so pixel checks compare against a tolerance, not equality.
func assertCenterColor(t *testing.T, m image.Image, wantR, wantG, wantB uint8, tolerance int) {
	t.Helper()
	b := m.Bounds()
	c := color.NRGBAModel.Convert(m.At((b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2)).(color.NRGBA)
	for _, ch := range []struct {
		name      string
		got, want uint8
	}{{"R", c.R, wantR}, {"G", c.G, wantG}, {"B", c.B, wantB}} {
		diff := int(ch.got) - int(ch.want)
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Errorf("center pixel %s = %d, want %d within tolerance %d", ch.name, ch.got, ch.want, tolerance)
		}
	}
}

func TestPNGToJPEG(t *testing.T) {
	tests := []struct {
		name                string
		src                 color.NRGBA
		wantR, wantG, wantB uint8
	}{
		{name: "opaque color survives", src: color.NRGBA{R: 200, G: 50, B: 50, A: 255}, wantR: 200, wantG: 50, wantB: 50},
		{name: "fully transparent becomes white", src: color.NRGBA{}, wantR: 255, wantG: 255, wantB: 255},
		{name: "half transparent red blends with white", src: color.NRGBA{R: 255, A: 128}, wantR: 255, wantG: 127, wantB: 127},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := PNGToJPEG(&out, pngBytes(t, solidNRGBA(8, 8, tt.src)), 100); err != nil {
				t.Fatalf("PNGToJPEG() error = %v", err)
			}
			decoded, err := jpeg.Decode(&out)
			if err != nil {
				t.Fatalf("output is not a valid JPEG: %v", err)
			}
			if got, want := decoded.Bounds().Size(), image.Pt(8, 8); got != want {
				t.Errorf("output size = %v, want %v", got, want)
			}
			assertCenterColor(t, decoded, tt.wantR, tt.wantG, tt.wantB, 20)
		})
	}

	t.Run("empty input", func(t *testing.T) {
		if err := PNGToJPEG(&bytes.Buffer{}, strings.NewReader(""), 85); err == nil {
			t.Fatal("PNGToJPEG() expected an error, got nil")
		}
	})

	t.Run("input is not a PNG", func(t *testing.T) {
		var jpg bytes.Buffer
		if err := jpeg.Encode(&jpg, solidNRGBA(8, 8, color.NRGBA{A: 255}), nil); err != nil {
			t.Fatal(err)
		}
		err := PNGToJPEG(&bytes.Buffer{}, &jpg, 85)
		if err == nil {
			t.Fatal("PNGToJPEG() expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "decoding PNG") {
			t.Errorf("PNGToJPEG() error = %q, want it to mention decoding PNG", err)
		}
	})
}

func TestJPEGToPNG(t *testing.T) {
	t.Run("converts and preserves size and color", func(t *testing.T) {
		var jpg bytes.Buffer
		src := solidNRGBA(8, 8, color.NRGBA{R: 30, G: 180, B: 90, A: 255})
		if err := jpeg.Encode(&jpg, src, &jpeg.Options{Quality: 100}); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := JPEGToPNG(&out, &jpg); err != nil {
			t.Fatalf("JPEGToPNG() error = %v", err)
		}
		// IHDR bit depth (offset 24: signature, chunk header, width, height)
		// must stay 8; png.Encode widens YCbCr input to a 16-bit PNG.
		if got := out.Bytes()[24]; got != 8 {
			t.Errorf("PNG bit depth = %d, want 8", got)
		}
		decoded, err := png.Decode(&out)
		if err != nil {
			t.Fatalf("output is not a valid PNG: %v", err)
		}
		if got, want := decoded.Bounds().Size(), image.Pt(8, 8); got != want {
			t.Errorf("output size = %v, want %v", got, want)
		}
		assertCenterColor(t, decoded, 30, 180, 90, 20)
	})

	t.Run("empty input", func(t *testing.T) {
		if err := JPEGToPNG(&bytes.Buffer{}, strings.NewReader("")); err == nil {
			t.Fatal("JPEGToPNG() expected an error, got nil")
		}
	})

	t.Run("input is not a JPEG", func(t *testing.T) {
		err := JPEGToPNG(&bytes.Buffer{}, pngBytes(t, solidNRGBA(2, 2, color.NRGBA{A: 255})))
		if err == nil {
			t.Fatal("JPEGToPNG() expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "decoding JPEG") {
			t.Errorf("JPEGToPNG() error = %q, want it to mention decoding JPEG", err)
		}
	})
}
