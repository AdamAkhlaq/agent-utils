// Package inspect identifies what a file is from its content, so a caller
// (typically an agent) can learn about a file without reading it themselves.
package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Info describes a file's content. Width and Height are zero for non-images.
type Info struct {
	MIME   string
	Bytes  int64
	Width  int
	Height int
}

// Detect sniffs r's MIME type from its magic bytes and, for images, decodes
// the dimensions from the header.
func Detect(r io.Reader) (Info, error) {
	// The whole input is buffered in memory: Bytes needs the full length
	// anyway, and image.DecodeConfig can require bytes deep into the file
	// (a progressive JPEG's dimensions may sit well past the first 512).
	data, err := io.ReadAll(r)
	if err != nil {
		return Info{}, fmt.Errorf("reading input: %w", err)
	}
	if len(data) == 0 {
		return Info{}, fmt.Errorf("empty input")
	}
	info := Info{
		MIME:  http.DetectContentType(data),
		Bytes: int64(len(data)),
	}
	if strings.HasPrefix(info.MIME, "image/") {
		// Best effort: the MIME sniff already succeeded, so an image that
		// fails to decode keeps zero dimensions instead of failing outright.
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			info.Width, info.Height = cfg.Width, cfg.Height
		}
	}
	return info, nil
}

// JSON renders info as an indented JSON object; width and height are omitted
// when zero, so non-images carry no misleading dimension fields.
func JSON(info Info) (string, error) {
	repr := struct {
		MIME   string `json:"mime"`
		Bytes  int64  `json:"bytes"`
		Width  int    `json:"width,omitempty"`
		Height int    `json:"height,omitempty"`
	}{info.MIME, info.Bytes, info.Width, info.Height}
	out, err := json.MarshalIndent(repr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding file info: %w", err)
	}
	return string(out), nil
}
