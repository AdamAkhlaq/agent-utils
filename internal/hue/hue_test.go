package hue

import (
	"math/big"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    RGB
		wantErr string
	}{
		{name: "hex", input: "#ff8800", want: RGB{255, 136, 0}},
		{name: "hex without hash", input: "ff8800", want: RGB{255, 136, 0}},
		{name: "hex uppercase", input: "#FF8800", want: RGB{255, 136, 0}},
		{name: "short hex", input: "#f80", want: RGB{255, 136, 0}},
		{name: "short hex without hash", input: "F80", want: RGB{255, 136, 0}},
		{name: "rgb function", input: "rgb(255, 136, 0)", want: RGB{255, 136, 0}},
		{name: "rgb uppercase no spaces", input: "RGB(255,136,0)", want: RGB{255, 136, 0}},
		{name: "rgb extra spaces", input: "rgb( 255 , 136 , 0 )", want: RGB{255, 136, 0}},
		{name: "bare triple", input: "255,136,0", want: RGB{255, 136, 0}},
		{name: "bare triple with spaces", input: " 255, 136, 0 ", want: RGB{255, 136, 0}},
		{name: "hsl function", input: "hsl(32, 100%, 50%)", want: RGB{255, 136, 0}},
		{name: "hsl uppercase no spaces", input: "HSL(32,100%,50%)", want: RGB{255, 136, 0}},
		{name: "hsl hue 360 wraps to 0", input: "hsl(360, 100%, 50%)", want: RGB{255, 0, 0}},
		{name: "hsl white", input: "hsl(0, 0%, 100%)", want: RGB{255, 255, 255}},
		{name: "empty", input: "", wantErr: "empty color value"},
		{name: "whitespace only", input: "  \n", wantErr: "empty color value"},
		{name: "named color", input: "blue", wantErr: "unrecognized color"},
		{name: "hex 4 digits is alpha", input: "#ff88", wantErr: "alpha is not supported"},
		{name: "hex 8 digits is alpha", input: "#ff880080", wantErr: "alpha is not supported"},
		{name: "bare hex 8 digits is alpha", input: "ff880080", wantErr: "alpha is not supported"},
		{name: "rgba rejected", input: "rgba(255, 136, 0, 0.5)", wantErr: "alpha is not supported"},
		{name: "hsla rejected", input: "hsla(32, 100%, 50%, 0.5)", wantErr: "alpha is not supported"},
		{name: "rgb 4 components is alpha", input: "rgb(255, 136, 0, 128)", wantErr: "alpha is not supported"},
		{name: "hsl 4 components is alpha", input: "hsl(32, 100%, 50%, 1)", wantErr: "alpha is not supported"},
		{name: "hex 5 digits", input: "#ff880", wantErr: "must have 3 or 6 digits"},
		{name: "hex bad digit", input: "#gg8800", wantErr: "not a hex digit"},
		{name: "rgb out of range high", input: "rgb(300, 0, 0)", wantErr: "red component 300 out of range 0-255"},
		{name: "rgb out of range negative", input: "rgb(0, -1, 0)", wantErr: "green component -1 out of range 0-255"},
		{name: "rgb too few components", input: "rgb(1, 2)", wantErr: "exactly 3 components"},
		{name: "bare triple too few", input: "255,136", wantErr: "exactly 3 components"},
		{name: "rgb non-integer", input: "rgb(1.5, 0, 0)", wantErr: "must be an integer 0-255"},
		{name: "rgb percentages rejected", input: "rgb(100%, 0%, 0%)", wantErr: "must be an integer 0-255"},
		{name: "rgb missing paren", input: "rgb(255, 136, 0", wantErr: "missing closing parenthesis"},
		{name: "hsl missing percent", input: "hsl(30, 100, 50)", wantErr: "must be a percentage"},
		{name: "hsl hue out of range", input: "hsl(400, 100%, 50%)", wantErr: "hue 400 out of range 0-360"},
		{name: "hsl hue negative", input: "hsl(-10, 100%, 50%)", wantErr: "out of range 0-360"},
		{name: "hsl saturation out of range", input: "hsl(30, 101%, 50%)", wantErr: "saturation 101% out of range 0-100"},
		{name: "hsl lightness out of range", input: "hsl(30, 100%, 500%)", wantErr: "lightness 500% out of range 0-100"},
		{name: "hsl fractional hue", input: "hsl(30.5, 100%, 50%)", wantErr: "must be an integer"},
		{name: "hsl missing paren", input: "hsl(30, 100%, 50%", wantErr: "missing closing parenthesis"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, expected an error containing %q", tt.input, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse(%q) error = %q, want it to contain %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// referencePairs are RGB/HSL pairs verified by hand against the CSS
// conversion algorithm; they are exact in both directions.
var referencePairs = []struct {
	name string
	rgb  RGB
	hsl  HSL
}{
	{name: "orange #ff8800", rgb: RGB{255, 136, 0}, hsl: HSL{32, 100, 50}},
	{name: "red", rgb: RGB{255, 0, 0}, hsl: HSL{0, 100, 50}},
	{name: "green", rgb: RGB{0, 255, 0}, hsl: HSL{120, 100, 50}},
	{name: "blue", rgb: RGB{0, 0, 255}, hsl: HSL{240, 100, 50}},
	{name: "white", rgb: RGB{255, 255, 255}, hsl: HSL{0, 0, 100}},
	{name: "black", rgb: RGB{0, 0, 0}, hsl: HSL{0, 0, 0}},
	{name: "grey #808080", rgb: RGB{128, 128, 128}, hsl: HSL{0, 0, 50}},
	{name: "teal #008080", rgb: RGB{0, 128, 128}, hsl: HSL{180, 100, 25}},
	{name: "rebeccapurple #663399", rgb: RGB{102, 51, 153}, hsl: HSL{270, 50, 40}},
	{name: "pink #ff0088 negative hue branch", rgb: RGB{255, 0, 136}, hsl: HSL{328, 100, 50}},
	{name: "tan #bf8040 half rounding", rgb: RGB{191, 128, 64}, hsl: HSL{30, 50, 50}},
}

func TestToHSL(t *testing.T) {
	for _, tt := range referencePairs {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToHSL(tt.rgb); got != tt.hsl {
				t.Errorf("ToHSL(%v) = %v, want %v", tt.rgb, got, tt.hsl)
			}
		})
	}
}

func TestFromHSL(t *testing.T) {
	for _, tt := range referencePairs {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromHSL(tt.hsl); got != tt.rgb {
				t.Errorf("FromHSL(%v) = %v, want %v", tt.hsl, got, tt.rgb)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		to      string
		want    string
		wantErr string
	}{
		{name: "hex to rgb", input: "#ff8800", to: "rgb", want: "rgb(255, 136, 0)"},
		{name: "hex to hsl", input: "#ff8800", to: "hsl", want: "hsl(32, 100%, 50%)"},
		{name: "hex to hex normalizes", input: "#F80", to: "hex", want: "#ff8800"},
		{name: "rgb to hex", input: "rgb(255, 136, 0)", to: "hex", want: "#ff8800"},
		{name: "bare triple to hsl", input: "255,136,0", to: "hsl", want: "hsl(32, 100%, 50%)"},
		{name: "hsl to hex", input: "hsl(32, 100%, 50%)", to: "hex", want: "#ff8800"},
		{name: "hsl to rgb", input: "hsl(32, 100%, 50%)", to: "rgb", want: "rgb(255, 136, 0)"},
		{name: "hsl grey to hex", input: "hsl(0, 0%, 50%)", to: "hex", want: "#808080"},
		{name: "unknown form", input: "#ff8800", to: "cmyk", wantErr: "unknown output form"},
		{name: "parse error propagates", input: "rgb(300, 0, 0)", to: "hex", wantErr: "out of range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.input, tt.to)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Convert(%q, %q) error = %v, want it to contain %q", tt.input, tt.to, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert(%q, %q) error = %v", tt.input, tt.to, err)
			}
			if got != tt.want {
				t.Errorf("Convert(%q, %q) = %q, want %q", tt.input, tt.to, got, tt.want)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	t.Run("all forms of one color", func(t *testing.T) {
		got, err := JSON("hsl(32, 100%, 50%)")
		if err != nil {
			t.Fatalf("JSON() error = %v", err)
		}
		want := `{
  "hex": "#ff8800",
  "rgb": {
    "r": 255,
    "g": 136,
    "b": 0,
    "css": "rgb(255, 136, 0)"
  },
  "hsl": {
    "h": 32,
    "s": 100,
    "l": 50,
    "css": "hsl(32, 100%, 50%)"
  }
}`
		if got != want {
			t.Errorf("JSON() = %s, want %s", got, want)
		}
	})

	t.Run("parse error propagates", func(t *testing.T) {
		if _, err := JSON("nope"); err == nil || !strings.Contains(err.Error(), "unrecognized color") {
			t.Fatalf("JSON(\"nope\") error = %v, want unrecognized color", err)
		}
	})
}

func TestRoundTrip(t *testing.T) {
	t.Run("hex to hsl to hex stable", func(t *testing.T) {
		for _, hex := range []string{
			"#ff8800", "#ff0000", "#00ff00", "#0000ff", "#ffffff",
			"#000000", "#808080", "#008080", "#663399", "#bf8040", "#ff0088",
		} {
			hsl, err := Convert(hex, "hsl")
			if err != nil {
				t.Fatalf("Convert(%q, hsl) error = %v", hex, err)
			}
			back, err := Convert(hsl, "hex")
			if err != nil {
				t.Fatalf("Convert(%q, hex) error = %v", hsl, err)
			}
			if back != hex {
				t.Errorf("round trip %s -> %s -> %s, want %s back", hex, hsl, back, hex)
			}
		}
	})

	// Rounding makes hex -> hsl lossy in general: 16.7M RGB values map onto
	// roughly 2.4M rounded HSL triples, so near neighbors collapse. These
	// pin the documented examples.
	t.Run("hex to hsl to hex lossy neighbors", func(t *testing.T) {
		for _, tt := range []struct{ in, out string }{
			{in: "#ff8801", out: "#ff8800"},
			{in: "#010000", out: "#000000"},
		} {
			hsl, err := Convert(tt.in, "hsl")
			if err != nil {
				t.Fatalf("Convert(%q, hsl) error = %v", tt.in, err)
			}
			back, err := Convert(hsl, "hex")
			if err != nil {
				t.Fatalf("Convert(%q, hex) error = %v", hsl, err)
			}
			if back != tt.out {
				t.Errorf("round trip %s -> %s -> %s, want %s", tt.in, hsl, back, tt.out)
			}
		}
	})

	t.Run("conversion is deterministic", func(t *testing.T) {
		first, err := Convert("#1a2b3c", "hsl")
		if err != nil {
			t.Fatalf("Convert() error = %v", err)
		}
		for i := 0; i < 3; i++ {
			again, err := Convert("#1a2b3c", "hsl")
			if err != nil {
				t.Fatalf("Convert() error = %v", err)
			}
			if again != first {
				t.Fatalf("Convert() = %q, previously %q", again, first)
			}
		}
	})
}

// Exact halves must round up. The float64 conversions these replaced
// rounded every case here down, except the hue wrap, which pins 359.5 to 0.
func TestConversionsRoundExactHalvesUp(t *testing.T) {
	toHSL := []struct {
		name string
		rgb  RGB
		want HSL
	}{
		{name: "hue 122.5", rgb: RGB{0x00, 0x18, 0x01}, want: HSL{123, 100, 5}},
		{name: "saturation 87.5", rgb: RGB{0x07, 0x07, 0x69}, want: HSL{240, 88, 22}},
		{name: "hue 359.5 wraps to 0", rgb: RGB{0x7e, 0x06, 0x07}, want: HSL{0, 91, 26}},
	}
	for _, tt := range toHSL {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToHSL(tt.rgb); got != tt.want {
				t.Errorf("ToHSL(%v) = %v, want %v", tt.rgb, got, tt.want)
			}
		})
	}
	fromHSL := []struct {
		name string
		hsl  HSL
		want RGB
	}{
		{name: "red 25.5", hsl: HSL{0, 100, 5}, want: RGB{26, 0, 0}},
		{name: "green and blue 25.5", hsl: HSL{0, 80, 50}, want: RGB{230, 26, 26}},
		{name: "red 229.5", hsl: HSL{0, 60, 75}, want: RGB{230, 153, 153}},
	}
	for _, tt := range fromHSL {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromHSL(tt.hsl); got != tt.want {
				t.Errorf("FromHSL(%v) = %v, want %v", tt.hsl, got, tt.want)
			}
		})
	}
}

// TestConversionsAreExact checks every 11th integer hsl input and every 41st
// RGB color against the CSS formulas evaluated in exact rationals. The float64
// conversions this replaced fail it about 1,360 times on RGB and 250 times on
// hsl. The full spaces (3.7M and 16.7M inputs) take half a minute, so they are
// sampled.
func TestConversionsAreExact(t *testing.T) {
	for i := 0; i < 360*101*101; i += 11 {
		v := HSL{H: i / (101 * 101), S: i / 101 % 101, L: i % 101}
		if got, want := FromHSL(v), ratFromHSL(v); got != want {
			t.Fatalf("FromHSL(%v) = %v, want %v", v, got, want)
		}
	}
	for i := 0; i < 1<<24; i += 41 {
		c := RGB{uint8(i >> 16), uint8(i >> 8), uint8(i)}
		if got, want := ToHSL(c), ratToHSL(c); got != want {
			t.Fatalf("ToHSL(%v) = %v, want %v", c, got, want)
		}
	}
}

func rat(n, d int64) *big.Rat { return big.NewRat(n, d) }

// roundRat rounds a non-negative rational to the nearest integer, halves up.
func roundRat(x *big.Rat) int {
	n := new(big.Int).Add(new(big.Int).Mul(x.Num(), big.NewInt(2)), x.Denom())
	return int(n.Quo(n, new(big.Int).Mul(x.Denom(), big.NewInt(2))).Int64())
}

func ratAbs(x *big.Rat) *big.Rat { return new(big.Rat).Abs(x) }

func ratToHSL(c RGB) HSL {
	r, g, b := rat(int64(c.R), 255), rat(int64(c.G), 255), rat(int64(c.B), 255)
	hi, lo := r, r
	for _, x := range []*big.Rat{g, b} {
		if x.Cmp(hi) > 0 {
			hi = x
		}
		if x.Cmp(lo) < 0 {
			lo = x
		}
	}
	l := new(big.Rat).Quo(new(big.Rat).Add(hi, lo), rat(2, 1))
	if hi.Cmp(lo) == 0 {
		return HSL{L: roundRat(new(big.Rat).Mul(l, rat(100, 1)))}
	}
	d := new(big.Rat).Sub(hi, lo)
	twoLMinus1 := new(big.Rat).Sub(new(big.Rat).Mul(l, rat(2, 1)), rat(1, 1))
	s := new(big.Rat).Quo(d, new(big.Rat).Sub(rat(1, 1), ratAbs(twoLMinus1)))
	var sector *big.Rat
	switch hi {
	case r:
		sector = new(big.Rat).Quo(new(big.Rat).Sub(g, b), d)
		if sector.Sign() < 0 {
			sector.Add(sector, rat(6, 1))
		}
	case g:
		sector = new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Sub(b, r), d), rat(2, 1))
	default:
		sector = new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Sub(r, g), d), rat(4, 1))
	}
	return HSL{
		H: roundRat(new(big.Rat).Mul(sector, rat(60, 1))) % 360,
		S: roundRat(new(big.Rat).Mul(s, rat(100, 1))),
		L: roundRat(new(big.Rat).Mul(l, rat(100, 1))),
	}
}

func ratFromHSL(v HSL) RGB {
	s, l := rat(int64(v.S), 100), rat(int64(v.L), 100)
	twoLMinus1 := new(big.Rat).Sub(new(big.Rat).Mul(l, rat(2, 1)), rat(1, 1))
	c := new(big.Rat).Mul(new(big.Rat).Sub(rat(1, 1), ratAbs(twoLMinus1)), s)
	// h/60 mod 2, for an integer h in 0-359
	hMod2 := rat(int64(v.H%120), 60)
	x := new(big.Rat).Mul(c, new(big.Rat).Sub(rat(1, 1), ratAbs(new(big.Rat).Sub(hMod2, rat(1, 1)))))
	m := new(big.Rat).Sub(l, new(big.Rat).Quo(c, rat(2, 1)))
	zero := new(big.Rat)
	var ch [3]*big.Rat
	switch {
	case v.H < 60:
		ch = [3]*big.Rat{c, x, zero}
	case v.H < 120:
		ch = [3]*big.Rat{x, c, zero}
	case v.H < 180:
		ch = [3]*big.Rat{zero, c, x}
	case v.H < 240:
		ch = [3]*big.Rat{zero, x, c}
	case v.H < 300:
		ch = [3]*big.Rat{x, zero, c}
	default:
		ch = [3]*big.Rat{c, zero, x}
	}
	var out [3]uint8
	for i, x := range ch {
		out[i] = uint8(roundRat(new(big.Rat).Mul(new(big.Rat).Add(x, m), rat(255, 1))))
	}
	return RGB{out[0], out[1], out[2]}
}
