package img

import (
	"bytes"
	"image"
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

func TestQRDecodeErrors(t *testing.T) {
	t.Run("not an image", func(t *testing.T) {
		if _, err := QRDecode(strings.NewReader("not an image")); err == nil {
			t.Error("QRDecode() expected an error for non-image input")
		}
	})
	t.Run("image without QR code", func(t *testing.T) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64))); err != nil {
			t.Fatal(err)
		}
		if _, err := QRDecode(&buf); err == nil {
			t.Error("QRDecode() expected an error for an image with no QR code")
		}
	})
}
