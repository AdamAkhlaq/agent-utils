// Package chars inspects and normalizes text at the codepoint level, for
// diagnosing "these strings look identical but don't match" bugs: invisible
// characters, bidi controls, and normalization mismatches. It cannot be named
// "unicode" because it imports the stdlib package of that name.
package chars

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/unicode/runenames"
)

// RuneInfo describes one codepoint of the input. Char is the character
// itself, except for control (Cc) and format (Cf) codepoints, where it is
// empty: those are invisible or terminal-mangling, and Codepoint and Name
// already identify them. UTF8 is the codepoint's UTF-8 encoding as lowercase
// hex. Name is empty and Category is "Cn" for unassigned codepoints.
type RuneInfo struct {
	Char      string `json:"char"`
	Codepoint string `json:"codepoint"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	UTF8      string `json:"utf8"`
}

// Finding is one suspicious codepoint found by Check. Index is the rune
// index and Offset the byte offset of the codepoint in the input.
type Finding struct {
	Index     int    `json:"index"`
	Offset    int    `json:"offset"`
	Codepoint string `json:"codepoint"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Reason    string `json:"reason"`
}

// Report is Check's result. NFC is true when the input is already
// NFC-normalized; false signals a normalization mismatch risk even when
// Findings is empty.
type Report struct {
	NFC      bool      `json:"nfc"`
	Findings []Finding `json:"findings"`
}

// validate rejects data that is not valid UTF-8, reporting the byte offset of
// the first invalid sequence. A decoded size of 1 for RuneError distinguishes
// truly invalid bytes from a literal U+FFFD in the input, which is valid.
func validate(data []byte) error {
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("invalid UTF-8 at byte offset %d (0x%02x)", i, data[i])
		}
		i += size
	}
	return nil
}

// category returns r's Unicode general category, e.g. "Lu", "Zs", "Cf".
// unicode.Categories also holds one-letter groupings and the composite "LC"
// cased-letter class; only the real two-letter categories apply, and they are
// disjoint, so at most one matches. Unassigned codepoints report "Cn".
func category(r rune) string {
	for name, table := range unicode.Categories {
		if len(name) == 2 && name != "LC" && unicode.Is(table, r) {
			return name
		}
	}
	return "Cn"
}

// Inspect decodes data into one RuneInfo per codepoint, exactly as received:
// nothing is trimmed or normalized first. Invalid UTF-8 is an error, never
// silently replaced.
func Inspect(data []byte) ([]RuneInfo, error) {
	if err := validate(data); err != nil {
		return nil, err
	}
	infos := []RuneInfo{}
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		cat := category(r)
		char := string(r)
		if cat == "Cc" || cat == "Cf" {
			char = ""
		}
		infos = append(infos, RuneInfo{
			Char:      char,
			Codepoint: fmt.Sprintf("U+%04X", r),
			Name:      runenames.Name(r),
			Category:  cat,
			UTF8:      hex.EncodeToString(data[i : i+size]),
		})
		i += size
	}
	return infos, nil
}

// Check scans data for codepoints that commonly cause invisible-difference
// bugs: zero-width characters, bidi controls, the BOM, non-breaking spaces,
// and any other format (Cf) or control (Cc, except tab, LF, and CR)
// codepoints. It also reports whether the input is NFC-normalized. Finding
// nothing is a valid result, not an error.
func Check(data []byte) (Report, error) {
	if err := validate(data); err != nil {
		return Report{}, err
	}
	report := Report{
		NFC:      norm.NFC.IsNormal(data),
		Findings: []Finding{},
	}
	index := 0
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if reason := suspicion(r); reason != "" {
			report.Findings = append(report.Findings, Finding{
				Index:     index,
				Offset:    i,
				Codepoint: fmt.Sprintf("U+%04X", r),
				Name:      runenames.Name(r),
				Category:  category(r),
				Reason:    reason,
			})
		}
		i += size
		index++
	}
	return report, nil
}

// suspicion classifies r for Check, returning "" for unsuspicious codepoints.
func suspicion(r rune) string {
	switch r {
	case '\uFEFF':
		return "byte order mark"
	case '\u200B', '\u200C', '\u200D', '\u2060':
		return "zero-width"
	case '\u061C', '\u200E', '\u200F',
		'\u202A', '\u202B', '\u202C', '\u202D', '\u202E',
		'\u2066', '\u2067', '\u2068', '\u2069':
		return "bidi control"
	case '\u00A0', '\u202F':
		return "non-breaking space"
	case '\t', '\n', '\r':
		return ""
	}
	switch category(r) {
	case "Cf":
		return "invisible format character"
	case "Cc":
		return "control character"
	}
	return ""
}

// NFC returns data normalized to Unicode NFC (composed) form. Invalid UTF-8
// is an error: x/text's normalizer would pass the invalid bytes through
// silently (verified empirically), which would hide corruption.
func NFC(data []byte) ([]byte, error) {
	if err := validate(data); err != nil {
		return nil, err
	}
	return norm.NFC.Bytes(data), nil
}

// NFD returns data normalized to Unicode NFD (decomposed) form. Invalid
// UTF-8 is an error, as for NFC.
func NFD(data []byte) ([]byte, error) {
	if err := validate(data); err != nil {
		return nil, err
	}
	return norm.NFD.Bytes(data), nil
}

// InspectJSON renders Inspect's result as an indented JSON array without a
// trailing newline. HTML escaping is disabled so characters like "<" appear
// literally in the char field.
func InspectJSON(data []byte) (string, error) {
	infos, err := Inspect(data)
	if err != nil {
		return "", err
	}
	return marshal(infos)
}

// CheckJSON renders Check's result as an indented JSON object without a
// trailing newline.
func CheckJSON(data []byte) (string, error) {
	report, err := Check(data)
	if err != nil {
		return "", err
	}
	return marshal(report)
}

func marshal(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", fmt.Errorf("encoding JSON: %w", err)
	}
	return string(bytes.TrimRight(buf.Bytes(), "\n")), nil
}
