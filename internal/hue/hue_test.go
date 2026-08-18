package hue

import (
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
	// roughly 3.7M rounded HSL triples, so near neighbors collapse. These
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
