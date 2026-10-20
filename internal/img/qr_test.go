package img

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"strings"
	"testing"
)

func TestQRRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		text string
		size int
	}{
		{name: "url", text: "https://example.com", size: 256},
		{name: "plain text", text: "hello, world", size: 256},
		// Dense codes need more pixels: around 500 characters at 256px sits
		// right at the decoder's edge, where success depends on the exact
		// content, so the long case uses 512px for reliable margin.
		{name: "long text", text: strings.Repeat("agent-utils ", 50), size: 512},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := QR(&buf, tt.text, tt.size); err != nil {
				t.Fatalf("QR() error = %v", err)
			}
			got, err := QRDecode(&buf)
			if err != nil {
				t.Fatalf("QRDecode() error = %v", err)
			}
			if got != tt.text {
				t.Errorf("round trip = %q, want %q", got, tt.text)
			}
		})
	}
}

func TestQREmptyText(t *testing.T) {
	var buf bytes.Buffer
	if err := QR(&buf, "", 256); err == nil {
		t.Error("QR() expected an error for empty text")
	}
}

func TestQRDecodeInverted(t *testing.T) {
	var code bytes.Buffer
	if err := QR(&code, "https://example.com", 256); err != nil {
		t.Fatal(err)
	}
	src, err := png.Decode(&code)
	if err != nil {
		t.Fatal(err)
	}
	inverted := image.NewGray(src.Bounds())
	draw.Draw(inverted, inverted.Bounds(), src, image.Point{}, draw.Src)
	for i := range inverted.Pix {
		inverted.Pix[i] = 255 - inverted.Pix[i]
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, inverted); err != nil {
		t.Fatal(err)
	}
	got, err := QRDecode(&buf)
	if err != nil || got != "https://example.com" {
		t.Errorf("QRDecode() = %q, %v, want the text of the light-on-dark code", got, err)
	}
}

func TestQRDecodeErrors(t *testing.T) {
	encodePNG := func(m image.Image) []byte {
		var buf bytes.Buffer
		if err := png.Encode(&buf, m); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	var code bytes.Buffer
	if err := QR(&code, "https://example.com", 256); err != nil {
		t.Fatal(err)
	}
	src, err := png.Decode(&code)
	if err != nil {
		t.Fatal(err)
	}
	noise := image.NewGray(image.Rect(0, 0, 256, 256))
	for i, x := range noise.Pix {
		x = uint8(uint32(i) * 2654435761 >> 24)
		noise.Pix[i] = 255 * (x & 1)
	}
	tests := []struct {
		name    string
		input   []byte
		wantErr error
		wantMsg string
	}{
		{name: "not an image", input: []byte("not an image"), wantMsg: "decoding QR image: image: unknown format"},
		{name: "empty input", input: nil, wantMsg: "decoding QR image:"},
		{name: "image without a code", input: encodePNG(image.NewGray(image.Rect(0, 0, 64, 64))), wantErr: ErrNoQRCode},
		{name: "random noise", input: encodePNG(noise), wantErr: ErrNoQRCode},
		{name: "code cut in half", input: encodePNG(src.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(image.Rect(0, 0, 128, 256))), wantErr: ErrNoQRCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := QRDecode(bytes.NewReader(tt.input))
			if err == nil {
				t.Fatal("QRDecode() succeeded, want an error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("QRDecode() error = %v, want %v", err, tt.wantErr)
			}
			if !strings.HasPrefix(err.Error(), tt.wantMsg) {
				t.Errorf("QRDecode() error = %q, want it to start with %q", err, tt.wantMsg)
			}
			if strings.Contains(err.Error(), "Exception") {
				t.Errorf("QRDecode() error = %q leaks the decoder's internals", err)
			}
		})
	}
}
