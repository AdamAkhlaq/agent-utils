package arith

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

func rat(t *testing.T, s string) *big.Rat {
	t.Helper()
	x, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("bad test value %q", s)
	}
	return x
}

func TestEval(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string // as a big.Rat string
	}{
		{name: "integer", expr: "42", want: "42"},
		{name: "addition", expr: "1+2", want: "3"},
		{name: "decimal sum is exact", expr: "0.1 + 0.2", want: "3/10"},
		{name: "subtraction below zero", expr: "1 - 3", want: "-2"},
		{name: "precedence of * over +", expr: "1 + 2 * 3", want: "7"},
		{name: "parentheses", expr: "(1 + 2) * 3", want: "9"},
		{name: "left-associative -", expr: "10 - 4 - 3", want: "3"},
		{name: "left-associative /", expr: "8 / 4 / 2", want: "1"},
		{name: "division is exact", expr: "1 / 3", want: "1/3"},
		{name: "fraction arithmetic", expr: "1/3 + 1/6", want: "1/2"},
		{name: "unary minus", expr: "-5", want: "-5"},
		{name: "unary plus", expr: "+5", want: "5"},
		{name: "double negation", expr: "--5", want: "5"},
		{name: "signed operand after operator", expr: "2 * -3", want: "-6"},
		{name: "minus a negative", expr: "2 - -3", want: "5"},
		{name: "power", expr: "2^10", want: "1024"},
		{name: "power is right-associative", expr: "2^3^2", want: "512"},
		{name: "power binds tighter than leading minus", expr: "-2^2", want: "-4"},
		{name: "parenthesized negative base", expr: "(-2)^2", want: "4"},
		{name: "odd power of a negative base", expr: "(-2)^3", want: "-8"},
		{name: "negative exponent", expr: "2^-2", want: "1/4"},
		{name: "signed exponent chain", expr: "2^-3^2", want: "1/512"},
		{name: "power binds tighter than *", expr: "3 * 2^2", want: "12"},
		{name: "power of a fraction", expr: "(2/3)^-2", want: "9/4"},
		{name: "power of a decimal", expr: "1.1^2", want: "121/100"},
		{name: "integer-valued exponent expression", expr: "2^(6/3)", want: "4"},
		{name: "zero to the zero", expr: "0^0", want: "1"},
		{name: "zero to a positive power", expr: "0^5", want: "0"},
		{name: "one to a huge power", expr: "1^99999999999999999999", want: "1"},
		{name: "minus one to a huge odd power", expr: "(-1)^99999999999999999999", want: "-1"},
		{name: "minus one to a huge even power", expr: "(-1)^-99999999999999999998", want: "1"},
		{name: "power at the size limit", expr: "2^65535 / 2^65534", want: "2"},
		{name: "modulo", expr: "7 % 3", want: "1"},
		{name: "modulo sign follows divisor", expr: "-7 % 3", want: "2"},
		{name: "modulo negative divisor", expr: "7 % -3", want: "-2"},
		{name: "modulo both negative", expr: "-7 % -3", want: "-1"},
		{name: "modulo of a decimal", expr: "7.5 % 2", want: "3/2"},
		{name: "modulo negative decimal", expr: "-7.5 % 2", want: "1/2"},
		{name: "modulo of fractions", expr: "(1/2) % (1/3)", want: "1/6"},
		{name: "modulo exact multiple", expr: "6 % 3", want: "0"},
		{name: "modulo shares * precedence", expr: "2 * 7 % 4", want: "2"},
		{name: "scientific notation", expr: "1.5e3", want: "1500"},
		{name: "uppercase exponent", expr: "1.5E3", want: "1500"},
		{name: "explicit positive exponent", expr: "2e+2", want: "200"},
		{name: "negative exponent literal", expr: "25e-3", want: "1/40"},
		{name: "leading-dot decimal", expr: ".5", want: "1/2"},
		{name: "leading zeros", expr: "007.50", want: "15/2"},
		{name: "zero with a huge exponent", expr: "0e99999999999999999999999", want: "0"},
		{name: "long exact decimal", expr: "0.1234567890123456789012345", want: "2469135780246913578024690/20000000000000000000000000"},
		{name: "big integer arithmetic", expr: "99999999999999999999 + 1", want: "100000000000000000000"},
		{name: "whitespace everywhere", expr: " \t( 1 +\n2 ) * 3 ", want: "9"},
		{name: "nesting at the limit", expr: strings.Repeat("(", maxDepth) + "1" + strings.Repeat(")", maxDepth), want: "1"},
		{name: "signs at the limit", expr: strings.Repeat("-", maxDepth) + "1", want: "1"},
		{name: "power chain at the limit", expr: strings.Repeat("1^", maxDepth) + "1", want: "1"},
		{name: "no whitespace", expr: "(1+2)*3", want: "9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr)
			if err != nil {
				t.Fatalf("Eval(%q) error = %v", tt.expr, err)
			}
			if want := rat(t, tt.want); got.Cmp(want) != 0 {
				t.Errorf("Eval(%q) = %s, want %s", tt.expr, got.RatString(), want.RatString())
			}
		})
	}
}

func TestEvalErrors(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr string
	}{
		{name: "empty", expr: "", wantErr: "empty expression"},
		{name: "whitespace only", expr: " \t\n", wantErr: "empty expression"},
		{name: "dangling operator", expr: "1 +", wantErr: "syntax error at column 4: unexpected end of expression"},
		{name: "leading binary operator", expr: "* 2", wantErr: `syntax error at column 1: unexpected "*"`},
		{name: "python power operator", expr: "2**3", wantErr: `column 3: unexpected "*" (write powers with ^, not **)`},
		{name: "unclosed parenthesis", expr: "(1 + 2", wantErr: `syntax error at column 7: missing ")" to close the "(" at column 1`},
		{name: "unmatched close", expr: "1 + 2)", wantErr: `syntax error at column 6: unexpected ")" with no "(" to close`},
		{name: "empty parentheses", expr: "()", wantErr: `syntax error at column 2: unexpected ")" where a number was expected`},
		{name: "missing operand inside parentheses", expr: "(1 +)", wantErr: `syntax error at column 5: unexpected ")" where a number was expected`},
		{name: "unmatched close after parentheses", expr: "(1) + 2)", wantErr: `syntax error at column 8: unexpected ")" with no "(" to close`},
		{name: "missing operand at top level", expr: "1 * )", wantErr: `syntax error at column 5: unexpected ")" with no "(" to close`},
		{name: "implicit multiplication", expr: "2(3)", wantErr: `column 2: unexpected "(" (calc has no implicit multiplication`},
		{name: "two numbers", expr: "2 3", wantErr: `column 3: unexpected number "3"`},
		{name: "trailing decimal point", expr: "5.", wantErr: "column 3: expected a digit after the decimal point"},
		{name: "bare decimal point", expr: ".", wantErr: "column 2: expected a digit after the decimal point"},
		{name: "two decimal points", expr: "1.5.3", wantErr: "column 4: a number has at most one decimal point"},
		{name: "exponent without digits", expr: "1e", wantErr: `column 3: expected exponent digits after "1e"`},
		{name: "exponent sign without digits", expr: "1e-", wantErr: `column 4: expected exponent digits after "1e-"`},
		{name: "function", expr: "sqrt(2)", wantErr: `unknown name "sqrt" at column 1: calc does exact rational arithmetic only`},
		{name: "constant", expr: "2 * pi", wantErr: `unknown name "pi" at column 5`},
		{name: "unicode name", expr: "π", wantErr: `unknown name "π" at column 1`},
		{name: "hex literal", expr: "0x1f", wantErr: `unknown name "x1f" at column 2`},
		{name: "digit grouping", expr: "1,000", wantErr: `column 2: unexpected "," (write numbers without digit grouping)`},
		{name: "unicode minus", expr: "3 − 1", wantErr: `column 3: unexpected character "−" (U+2212); use the ASCII "-"`},
		{name: "multiplication sign", expr: "3 × 2", wantErr: `use the ASCII "*"`},
		{name: "division sign", expr: "3 ÷ 2", wantErr: `use the ASCII "/"`},
		{name: "unknown character", expr: "1 & 2", wantErr: `column 3: unexpected character "&" (U+0026)`},
		{name: "division by zero", expr: "1 / 0", wantErr: "division by zero at column 3"},
		{name: "division by zero expression", expr: "1 / (2 - 2)", wantErr: "division by zero at column 3"},
		{name: "modulo by zero", expr: "5 % 0", wantErr: "modulo by zero at column 3"},
		{name: "zero to a negative power", expr: "0^-1", wantErr: "division by zero at column 2: 0 raised to a negative power"},
		{name: "fractional exponent", expr: "2^0.5", wantErr: "exponent 1/2 at column 2 is not an integer"},
		{name: "power tower", expr: "10^10^10", wantErr: `result of "^" at column 3 exceeds the size limit`},
		{name: "power past the limit", expr: "2^65536", wantErr: `result of "^" at column 2 exceeds the size limit`},
		{name: "negative power past the limit", expr: "2^-65536", wantErr: `result of "^" at column 2 exceeds the size limit`},
		{name: "product past the limit", expr: "2^65535 * 2", wantErr: `result of "*" at column 9 exceeds the size limit`},
		{name: "huge literal exponent", expr: "1e999999999", wantErr: "number at column 1 exceeds the size limit"},
		{name: "literal exponent past int64", expr: "1e99999999999999999999", wantErr: "number at column 1 exceeds the size limit"},
		{name: "tiny literal", expr: "1e-99999", wantErr: "number at column 1 exceeds the size limit"},
		{name: "too many digits", expr: "1" + strings.Repeat("7", maxLiteralDigits), wantErr: "number at column 1 exceeds the size limit"},
		{name: "syntax error wins over evaluation error", expr: "1/0 +", wantErr: "syntax error at column 6"},
		{name: "first evaluation error wins", expr: "1/0 + 10^10^10", wantErr: "division by zero at column 2"},
		{name: "nesting limit", expr: strings.Repeat("(", maxDepth+1) + "1" + strings.Repeat(")", maxDepth+1), wantErr: "column 1002: expression nests deeper than 1000 levels"},
		{name: "sign chain limit", expr: strings.Repeat("-", maxDepth+1) + "1", wantErr: "nests deeper than 1000 levels"},
		{name: "power chain limit", expr: strings.Repeat("1^", maxDepth+1) + "1", wantErr: "nests deeper than 1000 levels"},
		{name: "column counts characters, not bytes", expr: "ééé", wantErr: `unknown name "ééé" at column 1`},
		{name: "column after multibyte whitespace", expr: "1 + )", wantErr: "syntax error at column 5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr)
			if err == nil {
				t.Fatalf("Eval(%q) = %v, expected an error containing %q", tt.expr, got, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Eval(%q) error = %q, want it to contain %q", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestEvalLimitsAreCheap(t *testing.T) {
	inputs := map[string]string{
		"power tower":           "10^10^10^10",
		"many large products":   strings.Repeat("2^65000 * 2^-65000 + ", 200) + "1",
		"long sum":              strings.Repeat("1+", 100000) + "1",
		"limit-sized literals":  strings.Repeat("9", maxLiteralDigits) + "e-" + "65536",
		"huge exponent literal": "7e-9223372036854775808",
	}
	for name, expr := range inputs {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			_, _ = Eval(expr)
			if d := time.Since(start); d > 2*time.Second {
				t.Errorf("Eval took %v", d)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		scale     int
		want      string
		wantExact bool
	}{
		{name: "integer", value: "42", scale: -1, want: "42", wantExact: true},
		{name: "negative integer", value: "-42", scale: -1, want: "-42", wantExact: true},
		{name: "zero", value: "0", scale: -1, want: "0", wantExact: true},
		{name: "terminating decimal", value: "3/10", scale: -1, want: "0.3", wantExact: true},
		{name: "negative terminating decimal", value: "-1/8", scale: -1, want: "-0.125", wantExact: true},
		{name: "terminating beyond 20 places stays exact", value: "1/1073741824", scale: -1, want: "0.000000000931322574615478515625", wantExact: true},
		{name: "big integer without exponent", value: "1e30", scale: -1, want: "1000000000000000000000000000000", wantExact: true},
		{name: "repeating decimal rounds to 20 places", value: "1/3", scale: -1, want: "0.33333333333333333333", wantExact: false},
		{name: "repeating decimal rounds up", value: "2/3", scale: -1, want: "0.66666666666666666667", wantExact: false},
		{name: "negative repeating decimal", value: "-2/3", scale: -1, want: "-0.66666666666666666667", wantExact: false},
		{name: "repeating decimal keeps 20 places when it rounds to zeros", value: "1/30000000000000000000000000", scale: -1, want: "0.00000000000000000000", wantExact: false},
		{name: "tiny negative rounds to unsigned zero", value: "-1/30000000000000000000000000", scale: -1, want: "0.00000000000000000000", wantExact: false},
		{name: "scale pads an integer", value: "5", scale: 2, want: "5.00", wantExact: true},
		{name: "scale pads a decimal", value: "1/2", scale: 3, want: "0.500", wantExact: true},
		{name: "scale zero rounds to an integer", value: "5/2", scale: 0, want: "3", wantExact: false},
		{name: "half rounds away from zero", value: "0.125", scale: 2, want: "0.13", wantExact: false},
		{name: "negative half rounds away from zero", value: "-0.125", scale: 2, want: "-0.13", wantExact: false},
		{name: "below half rounds down", value: "0.0049999", scale: 2, want: "0.00", wantExact: false},
		{name: "negative half to integer", value: "-2.5", scale: 0, want: "-3", wantExact: false},
		{name: "scale rounds a repeating decimal", value: "1/3", scale: 4, want: "0.3333", wantExact: false},
		{name: "scale wider than the default", value: "1/7", scale: 30, want: "0.142857142857142857142857142857", wantExact: false},
		{name: "scale exactly fits", value: "0.125", scale: 3, want: "0.125", wantExact: true},
		{name: "negative rounding to zero drops the sign", value: "-0.001", scale: 2, want: "0.00", wantExact: false},
		{name: "negative rounding to zero at scale 0", value: "-0.4", scale: 0, want: "0", wantExact: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, exact := Format(rat(t, tt.value), tt.scale)
			if got != tt.want || exact != tt.wantExact {
				t.Errorf("Format(%s, %d) = %q, %v; want %q, %v", tt.value, tt.scale, got, exact, tt.want, tt.wantExact)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	tests := []struct {
		name  string
		value string
		scale int
		want  string
	}{
		{name: "repeating", value: "1/3", scale: -1, want: `{
  "result": "0.33333333333333333333",
  "fraction": "1/3",
  "exact": false
}`},
		{name: "integer keeps the fraction shape", value: "5", scale: -1, want: `{
  "result": "5",
  "fraction": "5/1",
  "exact": true
}`},
		{name: "scale applies to the result only", value: "-1/8", scale: 2, want: `{
  "result": "-0.13",
  "fraction": "-1/8",
  "exact": false
}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := JSON(rat(t, tt.value), tt.scale)
			if err != nil {
				t.Fatalf("JSON() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("JSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

// Everything calc prints exactly must read back as the same value, including
// values at the size limit, whose decimals are the longest there are.
func TestFormatReadsBack(t *testing.T) {
	for _, expr := range []string{"1/8", "-123.456", "2^65535", "2^-65535", "5^-28224", "-(2^-65535) * (2^65535 - 1)"} {
		t.Run(expr, func(t *testing.T) {
			x, err := Eval(expr)
			if err != nil {
				t.Fatalf("Eval(%q) error = %v", expr, err)
			}
			s, exact := Format(x, -1)
			if !exact {
				t.Fatalf("Format(%s) is not exact", expr)
			}
			back, err := Eval(s)
			if err != nil {
				t.Fatalf("Eval(Format(%s)) error = %v", expr, err)
			}
			if back.Cmp(x) != 0 {
				t.Errorf("Eval(Format(%s)) = %s, want %s", expr, back.RatString(), x.RatString())
			}
		})
	}
}

func FuzzEval(f *testing.F) {
	for _, seed := range []string{
		"1 + 2 * 3", "-2^2", "2^-3^2", "(1/3) % (1/7)", "1.5e3 - .25", "0^0",
		"10^10^10", "1e-99999", "((1)", "2**3", "sqrt(2)", "1/0 +", "3 − 1", "",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		x, err := Eval(expr)
		if err != nil {
			return
		}
		if x == nil {
			t.Fatalf("Eval(%q) returned nil without an error", expr)
		}
		s, exact := Format(x, -1)
		if exact {
			back, err := Eval(s)
			if err != nil || back.Cmp(x) != 0 {
				t.Fatalf("Eval(%q) = %s prints as %q, which reads back as %v (err %v)", expr, x.RatString(), s, back, err)
			}
		}
		if s, _ := Format(x, 3); !strings.Contains(s, ".") || len(s)-strings.Index(s, ".") != 4 {
			t.Fatalf("Format(%s, 3) = %q, want exactly 3 places", x.RatString(), s)
		}
	})
}
