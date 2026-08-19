package chars

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestInspect(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []RuneInfo
		wantErr string
	}{
		{
			name:  "ascii upper and lower",
			input: "Ab",
			want: []RuneInfo{
				// 'A' must be "Lu", not the composite "LC" class that
				// unicode.Categories also exposes under a two-letter key.
				{Char: "A", Codepoint: "U+0041", Name: "LATIN CAPITAL LETTER A", Category: "Lu", UTF8: "41"},
				{Char: "b", Codepoint: "U+0062", Name: "LATIN SMALL LETTER B", Category: "Ll", UTF8: "62"},
			},
		},
		{
			name:  "emoji zwj sequence shown as constituent runes",
			input: "\U0001F468\u200D\U0001F469",
			want: []RuneInfo{
				{Char: "\U0001F468", Codepoint: "U+1F468", Name: "MAN", Category: "So", UTF8: "f09f91a8"},
				{Char: "", Codepoint: "U+200D", Name: "ZERO WIDTH JOINER", Category: "Cf", UTF8: "e2808d"},
				{Char: "\U0001F469", Codepoint: "U+1F469", Name: "WOMAN", Category: "So", UTF8: "f09f91a9"},
			},
		},
		{
			name:  "combining accent stays decomposed",
			input: "e\u0301",
			want: []RuneInfo{
				{Char: "e", Codepoint: "U+0065", Name: "LATIN SMALL LETTER E", Category: "Ll", UTF8: "65"},
				{Char: "\u0301", Codepoint: "U+0301", Name: "COMBINING ACUTE ACCENT", Category: "Mn", UTF8: "cc81"},
			},
		},
		{
			name:  "control char renders empty",
			input: "\t",
			want: []RuneInfo{
				{Char: "", Codepoint: "U+0009", Name: "<control>", Category: "Cc", UTF8: "09"},
			},
		},
		{
			name:  "newline is inspected not stripped",
			input: "a\n",
			want: []RuneInfo{
				{Char: "a", Codepoint: "U+0061", Name: "LATIN SMALL LETTER A", Category: "Ll", UTF8: "61"},
				{Char: "", Codepoint: "U+000A", Name: "<control>", Category: "Cc", UTF8: "0a"},
			},
		},
		{
			name:  "unassigned codepoint",
			input: "\u0378",
			want: []RuneInfo{
				{Char: "\u0378", Codepoint: "U+0378", Name: "", Category: "Cn", UTF8: "cdb8"},
			},
		},
		{
			name:  "literal replacement char is valid input",
			input: "�",
			want: []RuneInfo{
				{Char: "�", Codepoint: "U+FFFD", Name: "REPLACEMENT CHARACTER", Category: "So", UTF8: "efbfbd"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  []RuneInfo{},
		},
		{
			name:    "invalid utf-8 at start",
			input:   "\xff",
			wantErr: "invalid UTF-8 at byte offset 0",
		},
		{
			name:    "invalid utf-8 mid input reports offset",
			input:   "hi\xc3(",
			wantErr: "invalid UTF-8 at byte offset 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Inspect([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Inspect() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Inspect() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Inspect() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNFC bool
		want    []Finding
		wantErr string
	}{
		{
			name:    "clean ascii with allowed whitespace",
			input:   "hello\tworld\r\n",
			wantNFC: true,
			want:    []Finding{},
		},
		{
			name:    "zero-width space with offsets",
			input:   "ab\u200Bc",
			wantNFC: true,
			want: []Finding{
				{Index: 2, Offset: 2, Codepoint: "U+200B", Name: "ZERO WIDTH SPACE", Category: "Cf", Reason: "zero-width"},
			},
		},
		{
			name:    "byte offset diverges from rune index after a multibyte rune",
			input:   "é\u200D",
			wantNFC: true,
			want: []Finding{
				{Index: 1, Offset: 2, Codepoint: "U+200D", Name: "ZERO WIDTH JOINER", Category: "Cf", Reason: "zero-width"},
			},
		},
		{
			name:    "bidi override",
			input:   "a\u202Eb",
			wantNFC: true,
			want: []Finding{
				{Index: 1, Offset: 1, Codepoint: "U+202E", Name: "RIGHT-TO-LEFT OVERRIDE", Category: "Cf", Reason: "bidi control"},
			},
		},
		{
			name:    "leading bom",
			input:   "\uFEFFhi",
			wantNFC: true,
			want: []Finding{
				{Index: 0, Offset: 0, Codepoint: "U+FEFF", Name: "ZERO WIDTH NO-BREAK SPACE", Category: "Cf", Reason: "byte order mark"},
			},
		},
		{
			name:    "non-breaking spaces",
			input:   "a\u00A0b\u202Fc",
			wantNFC: true,
			want: []Finding{
				{Index: 1, Offset: 1, Codepoint: "U+00A0", Name: "NO-BREAK SPACE", Category: "Zs", Reason: "non-breaking space"},
				{Index: 3, Offset: 4, Codepoint: "U+202F", Name: "NARROW NO-BREAK SPACE", Category: "Zs", Reason: "non-breaking space"},
			},
		},
		{
			name:    "soft hyphen is invisible format",
			input:   "co\u00ADop",
			wantNFC: true,
			want: []Finding{
				{Index: 2, Offset: 2, Codepoint: "U+00AD", Name: "SOFT HYPHEN", Category: "Cf", Reason: "invisible format character"},
			},
		},
		{
			name:    "escape is a control character",
			input:   "\x1b[31m",
			wantNFC: true,
			want: []Finding{
				{Index: 0, Offset: 0, Codepoint: "U+001B", Name: "<control>", Category: "Cc", Reason: "control character"},
			},
		},
		{
			name:    "decomposed accent flips nfc only",
			input:   "e\u0301",
			wantNFC: false,
			want:    []Finding{},
		},
		{
			name:    "empty input is nfc with no findings",
			input:   "",
			wantNFC: true,
			want:    []Finding{},
		},
		{
			name:    "invalid utf-8",
			input:   "a\x80",
			wantErr: "invalid UTF-8 at byte offset 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Check([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Check() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if got.NFC != tt.wantNFC {
				t.Errorf("Check() NFC = %v, want %v", got.NFC, tt.wantNFC)
			}
			if !reflect.DeepEqual(got.Findings, tt.want) {
				t.Errorf("Check() findings = %+v, want %+v", got.Findings, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		normalize func([]byte) ([]byte, error)
		input     string
		want      string
		wantErr   string
	}{
		{name: "nfc composes accent", normalize: NFC, input: "e\u0301", want: "é"},
		{name: "nfd decomposes accent", normalize: NFD, input: "é", want: "e\u0301"},
		{name: "nfc idempotent on composed", normalize: NFC, input: "é", want: "é"},
		{name: "nfd idempotent on decomposed", normalize: NFD, input: "e\u0301", want: "e\u0301"},
		{name: "nfd decomposes hangul", normalize: NFD, input: "가", want: "\u1100\u1161"},
		{name: "ascii untouched by nfc", normalize: NFC, input: "plain text\n", want: "plain text\n"},
		{name: "ascii untouched by nfd", normalize: NFD, input: "plain text\n", want: "plain text\n"},
		{name: "empty input", normalize: NFC, input: "", want: ""},
		{name: "nfc rejects invalid utf-8", normalize: NFC, input: "\xff", wantErr: "invalid UTF-8 at byte offset 0"},
		{name: "nfd rejects invalid utf-8", normalize: NFD, input: "\xff", wantErr: "invalid UTF-8 at byte offset 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.normalize([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("normalize error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("normalize = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("nfc of nfd round trips", func(t *testing.T) {
		original := "café naïve 가"
		decomposed, err := NFD([]byte(original))
		if err != nil {
			t.Fatalf("NFD() error = %v", err)
		}
		recomposed, err := NFC(decomposed)
		if err != nil {
			t.Fatalf("NFC() error = %v", err)
		}
		if string(recomposed) != original {
			t.Errorf("NFC(NFD(%q)) = %q, want the original", original, recomposed)
		}
	})
}

func TestInspectJSON(t *testing.T) {
	t.Run("emits a parseable array", func(t *testing.T) {
		out, err := InspectJSON([]byte("e\u0301"))
		if err != nil {
			t.Fatalf("InspectJSON() error = %v", err)
		}
		var infos []RuneInfo
		if err := json.Unmarshal([]byte(out), &infos); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, out)
		}
		if len(infos) != 2 || infos[1].Codepoint != "U+0301" {
			t.Errorf("InspectJSON() parsed = %+v", infos)
		}
		if strings.HasSuffix(out, "\n") {
			t.Error("InspectJSON() output ends in a newline; the CLI adds it")
		}
	})
	t.Run("empty input is an empty array", func(t *testing.T) {
		out, err := InspectJSON(nil)
		if err != nil {
			t.Fatalf("InspectJSON() error = %v", err)
		}
		if out != "[]" {
			t.Errorf("InspectJSON(nil) = %q, want %q", out, "[]")
		}
	})
	t.Run("no html escaping", func(t *testing.T) {
		out, err := InspectJSON([]byte("<"))
		if err != nil {
			t.Fatalf("InspectJSON() error = %v", err)
		}
		if !strings.Contains(out, `"char": "<"`) {
			t.Errorf("InspectJSON(\"<\") escaped the char field:\n%s", out)
		}
	})
	t.Run("propagates invalid utf-8", func(t *testing.T) {
		if _, err := InspectJSON([]byte{0xff}); err == nil {
			t.Fatal("InspectJSON() expected an error, got nil")
		}
	})
}

func TestCheckJSON(t *testing.T) {
	t.Run("emits a parseable report", func(t *testing.T) {
		out, err := CheckJSON([]byte("a\u200Bb"))
		if err != nil {
			t.Fatalf("CheckJSON() error = %v", err)
		}
		var report Report
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, out)
		}
		if !report.NFC || len(report.Findings) != 1 {
			t.Errorf("CheckJSON() parsed = %+v", report)
		}
	})
	t.Run("empty findings serialize as an array", func(t *testing.T) {
		out, err := CheckJSON([]byte("ok"))
		if err != nil {
			t.Fatalf("CheckJSON() error = %v", err)
		}
		if !strings.Contains(out, `"findings": []`) {
			t.Errorf("CheckJSON() findings not an empty array:\n%s", out)
		}
	})
	t.Run("propagates invalid utf-8", func(t *testing.T) {
		if _, err := CheckJSON([]byte{0xff}); err == nil {
			t.Fatal("CheckJSON() expected an error, got nil")
		}
	})
}
