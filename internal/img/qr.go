package img

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/makiuchi-d/gozxing"
	zxqrcode "github.com/makiuchi-d/gozxing/qrcode"
	qrcode "github.com/skip2/go-qrcode"
)

// QR writes a size x size pixel PNG containing text as a QR code to w. The
// size may not exceed MaxQRSize.
func QR(w io.Writer, text string, size int) error {
	// The encoder reads a negative size as a multiple of the code's module
	// count, an image size the caller never asked for.
	if size <= 0 {
		return fmt.Errorf("generating QR code: size must be positive, got %d", size)
	}
	if err := checkPixels("QR code image", size, size); err != nil {
		return fmt.Errorf("generating QR code: %w", err)
	}
	png, err := qrcode.Encode(text, qrcode.Medium, size)
	if err != nil {
		return fmt.Errorf("generating QR code: %w", err)
	}
	if _, err := w.Write(png); err != nil {
		return fmt.Errorf("generating QR code: %w", err)
	}
	return nil
}

// QRDecode reads a PNG, JPEG, GIF, BMP, TIFF, or WebP image from r and
// returns the text embedded in its QR code. Importing a format's decoder
// registers it with image.Decode; this package imports all six.
func QRDecode(r io.Reader) (string, error) {
	m, _, err := decodeAnyLimited(r)
	if err != nil {
		return "", fmt.Errorf("decoding QR image: %w", err)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(m)
	if err != nil {
		return "", fmt.Errorf("decoding QR image: %w", err)
	}
	result, err := zxqrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		return "", fmt.Errorf("decoding QR image: %w", err)
	}
	return result.GetText(), nil
}
