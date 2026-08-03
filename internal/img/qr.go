package img

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"

	"github.com/makiuchi-d/gozxing"
	zxqrcode "github.com/makiuchi-d/gozxing/qrcode"
	qrcode "github.com/skip2/go-qrcode"
)

// QR writes a size x size pixel PNG containing text as a QR code to w.
func QR(w io.Writer, text string, size int) error {
	png, err := qrcode.Encode(text, qrcode.Medium, size)
	if err != nil {
		return fmt.Errorf("generating QR code: %w", err)
	}
	if _, err := w.Write(png); err != nil {
		return fmt.Errorf("generating QR code: %w", err)
	}
	return nil
}

// QRDecode reads a PNG or JPEG image from r and returns the text embedded in
// its QR code. The blank imports above register those formats with
// image.Decode.
func QRDecode(r io.Reader) (string, error) {
	m, _, err := image.Decode(r)
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
