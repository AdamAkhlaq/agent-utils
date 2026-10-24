// Package hue parses and converts colors between hex, rgb, and hsl for the
// color command ("color" would collide with the stdlib image/color package).
// The canonical space is 8-bit RGB: every input parses to an RGB triple and
// every output renders from one, so conversions are deterministic.
package hue

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// RGB is an 8-bit-per-channel color, the package's canonical representation.
type RGB struct {
	R, G, B uint8
}

// HSL holds rounded integer hue (degrees, 0-359) and saturation and lightness
// (percent, 0-100).
type HSL struct {
	H, S, L int
}

const acceptedForms = "#rgb, #rrggbb (leading # optional), rgb(r, g, b), a bare r,g,b triple, or hsl(h, s%, l%)"

// Parse turns s into an RGB color. Accepted forms, case-insensitive: hex as
// #rgb or #rrggbb (leading # optional), rgb(r, g, b) or a bare r,g,b triple
// with integer components 0-255, and hsl(h, s%, l%) with integer components
// (hue 0-360, where 360 means 0). Alpha forms (#rrggbbaa, rgba(), hsla()) are
// rejected, not silently stripped.
func Parse(s string) (RGB, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return RGB{}, fmt.Errorf("empty color value (accepted: %s)", acceptedForms)
	}
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "rgba("):
		return RGB{}, fmt.Errorf("alpha is not supported: %q is an rgba() color, pass rgb(r, g, b) without the alpha component", s)
	case strings.HasPrefix(lower, "hsla("):
		return RGB{}, fmt.Errorf("alpha is not supported: %q is an hsla() color, pass hsl(h, s%%, l%%) without the alpha component", s)
	case strings.HasPrefix(lower, "rgb("):
		return parseRGBFunc(lower)
	case strings.HasPrefix(lower, "hsl("):
		return parseHSLFunc(lower)
	case strings.HasPrefix(lower, "#") || isHexDigits(lower):
		return parseHex(lower)
	case strings.Contains(lower, ","):
		return parseComponents(strings.Split(lower, ","))
	}
	return RGB{}, fmt.Errorf("unrecognized color %q (accepted: %s)", s, acceptedForms)
}

func isHexDigits(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return len(s) > 0
}

func parseHex(s string) (RGB, error) {
	digits := strings.TrimPrefix(s, "#")
	switch len(digits) {
	case 4, 8:
		return RGB{}, fmt.Errorf("alpha is not supported: %q has an alpha digit pair, pass %d hex digits without it", s, len(digits)-len(digits)/4)
	case 3:
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	case 6:
	default:
		return RGB{}, fmt.Errorf("hex color %q must have 3 or 6 digits, got %d", s, len(digits))
	}
	var c [3]uint8
	for i := range c {
		n, err := strconv.ParseUint(digits[2*i:2*i+2], 16, 8)
		if err != nil {
			return RGB{}, fmt.Errorf("invalid hex color %q: %q is not a hex digit pair", s, digits[2*i:2*i+2])
		}
		c[i] = uint8(n)
	}
	return RGB{c[0], c[1], c[2]}, nil
}

func parseRGBFunc(s string) (RGB, error) {
	body, ok := strings.CutSuffix(strings.TrimPrefix(s, "rgb("), ")")
	if !ok {
		return RGB{}, fmt.Errorf("invalid rgb color %q: missing closing parenthesis", s)
	}
	return parseComponents(strings.Split(body, ","))
}

func parseComponents(parts []string) (RGB, error) {
	if len(parts) == 4 {
		return RGB{}, fmt.Errorf("alpha is not supported: got 4 rgb components, pass 3 without the alpha component")
	}
	if len(parts) != 3 {
		return RGB{}, fmt.Errorf("rgb takes exactly 3 components, got %d", len(parts))
	}
	names := [3]string{"red", "green", "blue"}
	var c [3]uint8
	for i, part := range parts {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil {
			return RGB{}, fmt.Errorf("invalid rgb %s component %q (must be an integer 0-255)", names[i], part)
		}
		if n < 0 || n > 255 {
			return RGB{}, fmt.Errorf("rgb %s component %d out of range 0-255", names[i], n)
		}
		c[i] = uint8(n)
	}
	return RGB{c[0], c[1], c[2]}, nil
}

func parseHSLFunc(s string) (RGB, error) {
	body, ok := strings.CutSuffix(strings.TrimPrefix(s, "hsl("), ")")
	if !ok {
		return RGB{}, fmt.Errorf("invalid hsl color %q: missing closing parenthesis", s)
	}
	parts := strings.Split(body, ",")
	if len(parts) == 4 {
		return RGB{}, fmt.Errorf("alpha is not supported: got 4 hsl components, pass 3 without the alpha component")
	}
	if len(parts) != 3 {
		return RGB{}, fmt.Errorf("hsl takes exactly 3 components, got %d", len(parts))
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return RGB{}, fmt.Errorf("invalid hsl hue %q (must be an integer number of degrees)", strings.TrimSpace(parts[0]))
	}
	if h < 0 || h > 360 {
		return RGB{}, fmt.Errorf("hsl hue %d out of range 0-360", h)
	}
	names := [2]string{"saturation", "lightness"}
	var sl [2]int
	for i, part := range parts[1:] {
		part = strings.TrimSpace(part)
		digits, ok := strings.CutSuffix(part, "%")
		if !ok {
			return RGB{}, fmt.Errorf("hsl %s must be a percentage like 50%%, got %q", names[i], part)
		}
		n, err := strconv.Atoi(digits)
		if err != nil {
			return RGB{}, fmt.Errorf("invalid hsl %s %q (must be an integer percentage)", names[i], part)
		}
		if n < 0 || n > 100 {
			return RGB{}, fmt.Errorf("hsl %s %d%% out of range 0-100", names[i], n)
		}
		sl[i] = n
	}
	return FromHSL(HSL{H: h % 360, S: sl[0], L: sl[1]}), nil
}

// ToHSL converts c using the standard HSL derivation (CSS Color Module,
// hue from the dominant channel, lightness as the mid-range). Each component
// is rounded to the nearest integer, halves away from zero; a rounded hue of
// 360 wraps to 0 so hue stays in 0-359. The arithmetic is exact, in integers:
// float64 puts many exact halves on the wrong side (the hue of #001801 is
// 122.5 exactly, and comes out of float64 as 122.49999999999999).
func ToHSL(c RGB) HSL {
	r, g, b := int(c.R), int(c.G), int(c.B)
	hi, lo := max(r, g, b), min(r, g, b)
	d := hi - lo
	// l = (hi+lo)/510, s = d/(255-|hi+lo-255|), and h = 60*sector/d with
	// sector in [0, 6d), all in 0-255 channel units.
	l := roundDiv(10*(hi+lo), 51)
	if d == 0 {
		return HSL{L: l}
	}
	s := roundDiv(100*d, 255-abs(hi+lo-255))
	var sector int
	switch hi {
	case r:
		sector = (g - b + 6*d) % (6 * d)
	case g:
		sector = b - r + 2*d
	default:
		sector = r - g + 4*d
	}
	return HSL{H: roundDiv(60*sector, d) % 360, S: s, L: l}
}

// FromHSL converts v to RGB using the standard chroma/intermediate algorithm
// (CSS Color Module: c = (1-|2l-1|)*s, x = c*(1-|(h/60 mod 2)-1|), m = l-c/2).
// Each channel is rounded to the nearest integer, halves away from zero, with
// exact integer arithmetic for the same reason as ToHSL: hsl(0, 100%, 5%) has
// a red channel of 25.5 exactly, which float64 rounds to 25.
func FromHSL(v HSL) RGB {
	// Every quantity is scaled by 100 for s, 100 for l, and 60 for h, which
	// keeps c, x, and m integers: c is even and carries the factor 60.
	const scale = 100 * 100 * 60
	c := (100 - abs(2*v.L-100)) * v.S * 60
	x := c * (60 - abs(v.H%120-60)) / 60
	m := v.L*100*60 - c/2
	var r, g, b int
	switch {
	case v.H < 60:
		r, g, b = c, x, 0
	case v.H < 120:
		r, g, b = x, c, 0
	case v.H < 180:
		r, g, b = 0, c, x
	case v.H < 240:
		r, g, b = 0, x, c
	case v.H < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	channel := func(v int) uint8 { return uint8(roundDiv(255*(v+m), scale)) }
	return RGB{channel(r), channel(g), channel(b)}
}

// roundDiv returns n/d rounded to the nearest integer, halves up, for n >= 0
// and d > 0.
func roundDiv(n, d int) int {
	return (2*n + d) / (2 * d)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Hex renders c as lowercase #rrggbb.
func Hex(c RGB) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// CSSRGB renders c as rgb(r, g, b).
func CSSRGB(c RGB) string {
	return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
}

// CSSHSL renders v as hsl(h, s%, l%).
func CSSHSL(v HSL) string {
	return fmt.Sprintf("hsl(%d, %d%%, %d%%)", v.H, v.S, v.L)
}

// Convert parses input and renders it in the target form: hex, rgb, or hsl.
func Convert(input, to string) (string, error) {
	c, err := Parse(input)
	if err != nil {
		return "", err
	}
	switch to {
	case "hex":
		return Hex(c), nil
	case "rgb":
		return CSSRGB(c), nil
	case "hsl":
		return CSSHSL(ToHSL(c)), nil
	default:
		return "", fmt.Errorf("unknown output form %q (valid: hex, rgb, hsl)", to)
	}
}

// JSON parses input and renders all three forms at once, so one call gives an
// agent whatever form it turns out to need.
func JSON(input string) (string, error) {
	c, err := Parse(input)
	if err != nil {
		return "", err
	}
	h := ToHSL(c)
	repr := struct {
		Hex string  `json:"hex"`
		RGB jsonRGB `json:"rgb"`
		HSL jsonHSL `json:"hsl"`
	}{
		Hex: Hex(c),
		RGB: jsonRGB{R: c.R, G: c.G, B: c.B, CSS: CSSRGB(c)},
		HSL: jsonHSL{H: h.H, S: h.S, L: h.L, CSS: CSSHSL(h)},
	}
	out, err := json.MarshalIndent(repr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding representations: %w", err)
	}
	return string(out), nil
}

type jsonRGB struct {
	R   uint8  `json:"r"`
	G   uint8  `json:"g"`
	B   uint8  `json:"b"`
	CSS string `json:"css"`
}

type jsonHSL struct {
	H   int    `json:"h"`
	S   int    `json:"s"`
	L   int    `json:"l"`
	CSS string `json:"css"`
}
