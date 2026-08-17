package inspect

import (
	"bytes"
	"encoding/json"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func solidNRGBA(w, h int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		m.Pix[i] = 255
	}
	return m
}

func encodeImage(t *testing.T, format string, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, m)
	case "jpeg":
		err = jpeg.Encode(&buf, m, nil)
	case "gif":
		err = gif.Encode(&buf, m, nil)
	}
	if err != nil {
		t.Fatalf("encoding test %s: %v", format, err)
	}
	return buf.Bytes()
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name                  string
		input                 func(t *testing.T) []byte
		wantMIME              string
		wantWidth, wantHeight int
	}{
		{
			name:     "png with dimensions",
			input:    func(t *testing.T) []byte { return encodeImage(t, "png", solidNRGBA(8, 8)) },
			wantMIME: "image/png", wantWidth: 8, wantHeight: 8,
		},
		{
			name:     "jpeg with dimensions",
			input:    func(t *testing.T) []byte { return encodeImage(t, "jpeg", solidNRGBA(5, 3)) },
			wantMIME: "image/jpeg", wantWidth: 5, wantHeight: 3,
		},
		{
			name:     "gif with dimensions",
			input:    func(t *testing.T) []byte { return encodeImage(t, "gif", solidNRGBA(4, 6)) },
			wantMIME: "image/gif", wantWidth: 4, wantHeight: 6,
		},
		{
			name:     "plain text",
			input:    func(t *testing.T) []byte { return []byte("hello world") },
			wantMIME: "text/plain; charset=utf-8",
		},
		{
			name:     "json text sniffs as plain text",
			input:    func(t *testing.T) []byte { return []byte(`{"a": 1}`) },
			wantMIME: "text/plain; charset=utf-8",
		},
		{
			name:     "binary garbage",
			input:    func(t *testing.T) []byte { return []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe} },
			wantMIME: "application/octet-stream",
		},
		{
			name: "image mime with undecodable body keeps zero dimensions",
			input: func(t *testing.T) []byte {
				return append([]byte("\x89PNG\r\n\x1a\n"), []byte("not a real png body")...)
			},
			wantMIME: "image/png",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.input(t)
			info, err := Detect(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if info.MIME != tt.wantMIME {
				t.Errorf("Detect() MIME = %q, want %q", info.MIME, tt.wantMIME)
			}
			if info.Bytes != int64(len(data)) {
				t.Errorf("Detect() Bytes = %d, want %d", info.Bytes, len(data))
			}
			if info.Width != tt.wantWidth || info.Height != tt.wantHeight {
				t.Errorf("Detect() dimensions = %dx%d, want %dx%d",
					info.Width, info.Height, tt.wantWidth, tt.wantHeight)
			}
		})
	}

	t.Run("empty input", func(t *testing.T) {
		_, err := Detect(strings.NewReader(""))
		if err == nil {
			t.Fatal("Detect() expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "empty input") {
			t.Errorf("Detect() error = %q, want it to mention empty input", err)
		}
	})
}

func TestJSON(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "image includes dimensions",
			info: Info{MIME: "image/png", Bytes: 90, Width: 8, Height: 8},
			want: "{\n  \"mime\": \"image/png\",\n  \"bytes\": 90,\n  \"width\": 8,\n  \"height\": 8\n}",
		},
		{
			name: "non-image omits dimensions",
			info: Info{MIME: "text/plain; charset=utf-8", Bytes: 11},
			want: "{\n  \"mime\": \"text/plain; charset=utf-8\",\n  \"bytes\": 11\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JSON(tt.info)
			if err != nil {
				t.Fatalf("JSON() error = %v", err)
			}
			if !json.Valid([]byte(got)) {
				t.Fatalf("JSON() output is not valid JSON: %q", got)
			}
			if got != tt.want {
				t.Errorf("JSON() = %q, want %q", got, tt.want)
			}
		})
	}
}
