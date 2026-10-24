package text

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func FuzzMarkdownTable(f *testing.F) {
	for _, s := range []string{
		`[{"a":1}]`,
		`[["a","b"],["1"]]`,
		`[{"a":"x|y"},{"b":"\n"}]`,
		`[["","",""],[],[]]`,
		`[{"1":0},{"2":0},{"3":0}]`,
		`[["` + strings.Repeat("x", 81) + `"],["é"]]`,
		`[{"a":null,"b":true,"c":1.10}]`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		var out bytes.Buffer
		if err := MarkdownTable(&out, bytes.NewReader(in)); err != nil {
			if out.Len() != 0 {
				t.Fatalf("wrote %d bytes before failing: %v", out.Len(), err)
			}
			return
		}
		tbl, err := parseJSONTable(bytes.TrimPrefix(in, []byte("\xef\xbb\xbf")), len(in))
		if err != nil {
			t.Fatalf("MarkdownTable() succeeded but parseJSONTable() failed: %v", err)
		}
		checkTable(t, tbl, out.Bytes())
	})
}

func FuzzMarkdownTableCSV(f *testing.F) {
	for _, s := range []string{
		"a,b\n1,2\n",
		"a\n\"x\r\ny\"\n",
		"a|b,c\n|,\n",
		"a\r\nb\r\n",
		"h,i\n" + strings.Repeat("x", 81) + ",\n,\n",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		var out bytes.Buffer
		if err := MarkdownTableCSV(&out, bytes.NewReader(in)); err != nil {
			if out.Len() != 0 {
				t.Fatalf("wrote %d bytes before failing: %v", out.Len(), err)
			}
			return
		}
		tbl, err := parseCSVTable(bytes.NewReader(in))
		if err != nil {
			t.Fatalf("MarkdownTableCSV() succeeded but parseCSVTable() failed: %v", err)
		}
		checkTable(t, tbl, out.Bytes())
	})
}

// checkTable verifies a rendered table: its size is the one the output limit
// was checked against, it is within that limit, and every line has as many
// unescaped pipes as the header.
func checkTable(t *testing.T, tbl *table, out []byte) {
	t.Helper()
	pad := tbl.size(true, tbl.limit) <= tbl.limit
	if got := tbl.size(pad, math.MaxInt64); got != int64(len(out)) {
		t.Fatalf("size(pad=%t) = %d, but %d bytes were written", pad, got, len(out))
	}
	if int64(len(out)) > tbl.limit {
		t.Fatalf("wrote %d bytes, over the limit of %d", len(out), tbl.limit)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	want := -1
	for i, line := range lines {
		pipes := 0
		for j := 0; j < len(line); j++ {
			if line[j] == '\\' && j+1 < len(line) && line[j+1] == '|' {
				j++
				continue
			}
			if line[j] == '|' {
				pipes++
			}
		}
		if want < 0 {
			want = pipes
		} else if pipes != want {
			t.Fatalf("line %d has %d pipes, the header has %d: %q", i+1, pipes, want, line)
		}
	}
}
