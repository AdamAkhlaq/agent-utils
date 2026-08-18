package semver

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Version
	}{
		{name: "plain", input: "1.2.3", want: Version{Major: 1, Minor: 2, Patch: 3}},
		{name: "zeros", input: "0.0.0", want: Version{}},
		{name: "leading v", input: "v1.2.3", want: Version{Major: 1, Minor: 2, Patch: 3}},
		{name: "multi-digit", input: "10.20.30", want: Version{Major: 10, Minor: 20, Patch: 30}},
		{name: "single zero components", input: "1.0.0", want: Version{Major: 1}},
		{name: "pre-release", input: "1.0.0-alpha", want: Version{Major: 1, Prerelease: []string{"alpha"}}},
		{name: "numeric pre-release", input: "1.0.0-0.3.7", want: Version{Major: 1, Prerelease: []string{"0", "3", "7"}}},
		{name: "hyphenated pre-release", input: "1.0.0-x-y-z.--", want: Version{Major: 1, Prerelease: []string{"x-y-z", "--"}}},
		{name: "build metadata", input: "1.0.0+20130313144700", want: Version{Major: 1, Build: []string{"20130313144700"}}},
		{name: "build with leading zero", input: "1.0.0+001", want: Version{Major: 1, Build: []string{"001"}}},
		{name: "pre-release and build", input: "1.0.0-beta+exp.sha.5114f85", want: Version{Major: 1, Prerelease: []string{"beta"}, Build: []string{"exp", "sha", "5114f85"}}},
		{name: "hyphen inside build", input: "1.0.0+21AF26D3---117B344092BD", want: Version{Major: 1, Build: []string{"21AF26D3---117B344092BD"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.input, err)
			}
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
				t.Errorf("Parse(%q) core = %d.%d.%d, want %d.%d.%d",
					tt.input, got.Major, got.Minor, got.Patch, tt.want.Major, tt.want.Minor, tt.want.Patch)
			}
			if !slices.Equal(got.Prerelease, tt.want.Prerelease) {
				t.Errorf("Parse(%q) prerelease = %v, want %v", tt.input, got.Prerelease, tt.want.Prerelease)
			}
			if !slices.Equal(got.Build, tt.want.Build) {
				t.Errorf("Parse(%q) build = %v, want %v", tt.input, got.Build, tt.want.Build)
			}
			if got.String() != tt.input {
				t.Errorf("Parse(%q).String() = %q, want the input back", tt.input, got.String())
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "empty version"},
		{name: "major only", input: "1", wantErr: "expected MAJOR.MINOR.PATCH"},
		{name: "partial", input: "1.2", wantErr: "expected MAJOR.MINOR.PATCH"},
		{name: "four components", input: "1.2.3.4", wantErr: "expected MAJOR.MINOR.PATCH"},
		{name: "leading zero major", input: "01.2.3", wantErr: `major component "01" has a leading zero`},
		{name: "leading zero minor", input: "1.02.3", wantErr: `minor component "02" has a leading zero`},
		{name: "leading zero patch", input: "1.2.03", wantErr: `patch component "03" has a leading zero`},
		{name: "non-numeric core", input: "1.a.3", wantErr: "must contain only digits"},
		{name: "empty component", input: "1..3", wantErr: "minor component is empty"},
		{name: "out of range", input: "99999999999999999999999.0.0", wantErr: "out of range"},
		{name: "bare v", input: "v", wantErr: "expected MAJOR.MINOR.PATCH"},
		{name: "double v", input: "vv1.2.3", wantErr: "must contain only digits"},
		{name: "uppercase V", input: "V1.2.3", wantErr: "must contain only digits"},
		{name: "surrounding space", input: " 1.2.3", wantErr: "must contain only digits"},
		{name: "empty pre-release", input: "1.2.3-", wantErr: "empty pre-release identifier"},
		{name: "empty pre-release identifier", input: "1.2.3-alpha..1", wantErr: "empty pre-release identifier"},
		{name: "numeric pre-release leading zero", input: "1.2.3-0123", wantErr: `numeric pre-release identifier "0123" has a leading zero`},
		{name: "pre-release invalid char", input: "1.2.3-alpha_beta", wantErr: "may contain only [0-9A-Za-z-]"},
		{name: "pre-release unicode", input: "1.2.3-alphα", wantErr: "may contain only [0-9A-Za-z-]"},
		{name: "empty build", input: "1.2.3+", wantErr: "empty build identifier"},
		{name: "empty build identifier", input: "1.2.3+exp..1", wantErr: "empty build identifier"},
		{name: "build invalid char", input: "1.2.3+exp_1", wantErr: "may contain only [0-9A-Za-z-]"},
		{name: "build only", input: "+build", wantErr: "expected MAJOR.MINOR.PATCH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Fatalf("Parse(%q) expected an error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Parse(%q) error = %q, want it to contain %q", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestComparePrecedenceChains verifies every adjacent and non-adjacent pair of
// the spec's own precedence examples, both directions plus self-equality.
func TestComparePrecedenceChains(t *testing.T) {
	chains := map[string][]string{
		"spec pre-release chain": {
			"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
			"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
		},
		"spec core chain":            {"1.0.0", "2.0.0", "2.1.0", "2.1.1"},
		"numeric below alphanumeric": {"1.0.0-1", "1.0.0-2", "1.0.0-11", "1.0.0-a", "1.0.0-a.1"},
		"huge numeric identifiers":   {"1.0.0-18446744073709551616", "1.0.0-18446744073709551617"},
	}
	for name, chain := range chains {
		t.Run(name, func(t *testing.T) {
			versions := make([]Version, len(chain))
			for i, s := range chain {
				v, err := Parse(s)
				if err != nil {
					t.Fatalf("Parse(%q) error = %v", s, err)
				}
				versions[i] = v
			}
			for i := range versions {
				for j := range versions {
					want := 0
					if i < j {
						want = -1
					} else if i > j {
						want = 1
					}
					if got := Compare(versions[i], versions[j]); got != want {
						t.Errorf("Compare(%s, %s) = %d, want %d", chain[i], chain[j], got, want)
					}
				}
			}
		})
	}
}

func TestCompareBuildMetadataIgnored(t *testing.T) {
	pairs := [][2]string{
		{"1.0.0+linux", "1.0.0+mac"},
		{"1.0.0", "1.0.0+20130313144700"},
		{"1.0.0-alpha+001", "1.0.0-alpha"},
		{"v1.2.3", "1.2.3"},
	}
	for _, p := range pairs {
		got, err := CompareStrings(p[0], p[1])
		if err != nil {
			t.Fatalf("CompareStrings(%q, %q) error = %v", p[0], p[1], err)
		}
		if got != 0 {
			t.Errorf("CompareStrings(%q, %q) = %d, want 0", p[0], p[1], got)
		}
	}
}

func TestCompareStringsErrors(t *testing.T) {
	if _, err := CompareStrings("nope", "1.0.0"); err == nil || !strings.Contains(err.Error(), "first version") {
		t.Errorf("CompareStrings error = %v, want it to name the first version", err)
	}
	if _, err := CompareStrings("1.0.0", "1.2"); err == nil || !strings.Contains(err.Error(), "second version") {
		t.Errorf("CompareStrings error = %v, want it to name the second version", err)
	}
}

func TestSort(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		descending bool
		want       string
		wantErr    string
	}{
		{
			name:  "precedence order",
			input: "1.0.0\n1.0.0-rc.1\n2.0.0\n1.0.0-alpha\n1.0.0-alpha.1\n",
			want:  "1.0.0-alpha\n1.0.0-alpha.1\n1.0.0-rc.1\n1.0.0\n2.0.0\n",
		},
		{
			name:       "descending",
			input:      "1.0.0\n2.0.0\n1.5.0\n",
			descending: true,
			want:       "2.0.0\n1.5.0\n1.0.0\n",
		},
		{
			name:  "numeric not lexicographic",
			input: "1.0.0-beta.11\n1.0.0-beta.2\n",
			want:  "1.0.0-beta.2\n1.0.0-beta.11\n",
		},
		{
			name:  "blank lines and whitespace skipped",
			input: "\n2.0.0\n\n  1.0.0  \n\n",
			want:  "1.0.0\n2.0.0\n",
		},
		{
			name:  "v prefix preserved in output",
			input: "v2.0.0\n1.0.0\n",
			want:  "1.0.0\nv2.0.0\n",
		},
		{
			name:  "build metadata tie broken by original text",
			input: "1.0.0+bbb\n1.0.0+aaa\n1.0.0\n",
			want:  "1.0.0\n1.0.0+aaa\n1.0.0+bbb\n",
		},
		{
			name:  "tiebreak independent of input order",
			input: "1.0.0\n1.0.0+aaa\n1.0.0+bbb\n",
			want:  "1.0.0\n1.0.0+aaa\n1.0.0+bbb\n",
		},
		{
			name:       "descending reverses tiebreak too",
			input:      "1.0.0+aaa\n1.0.0+bbb\n",
			descending: true,
			want:       "1.0.0+bbb\n1.0.0+aaa\n",
		},
		{name: "empty input", input: "", want: ""},
		{name: "only blank lines", input: "\n\n", want: ""},
		{
			name:    "invalid line names line and value",
			input:   "1.0.0\n\n1.2\n",
			wantErr: `line 3: invalid version "1.2"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Sort(&out, strings.NewReader(tt.input), tt.descending)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Sort() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Sort() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Sort() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("Sort() output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseConstraintErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "empty constraint"},
		{name: "whitespace only", input: "   ", wantErr: "empty constraint"},
		{name: "caret", input: "^1.2.3", wantErr: "npm-style ^, ~, and || ranges are not"},
		{name: "tilde", input: "~1.2.3", wantErr: "npm-style ^, ~, and || ranges are not"},
		{name: "or range", input: ">=1.0.0 || <0.5.0", wantErr: "npm-style ^, ~, and || ranges are not"},
		{name: "bare version", input: "1.2.3", wantErr: `comparator "1.2.3" is missing an operator (use =1.2.3`},
		{name: "bad version in comparator", input: ">=1.2", wantErr: `comparator ">=1.2": invalid version "1.2"`},
		{name: "operator without version", input: ">=", wantErr: "empty version"},
		{name: "double operator", input: ">>1.0.0", wantErr: "invalid version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConstraint(tt.input)
			if err == nil {
				t.Fatalf("ParseConstraint(%q) expected an error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseConstraint(%q) error = %q, want it to contain %q", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestConstraintMatch(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		version    string
		want       bool
	}{
		{name: "equal matches", constraint: "=1.2.3", version: "1.2.3", want: true},
		{name: "equal rejects", constraint: "=1.2.3", version: "1.2.4", want: false},
		{name: "equal ignores build metadata", constraint: "=1.0.0", version: "1.0.0+build.7", want: true},
		{name: "equal ignores v prefix", constraint: "=1.2.3", version: "v1.2.3", want: true},
		{name: "greater matches", constraint: ">1.0.0", version: "1.0.1", want: true},
		{name: "greater rejects equal", constraint: ">1.0.0", version: "1.0.0", want: false},
		{name: "greater-equal matches equal", constraint: ">=1.0.0", version: "1.0.0", want: true},
		{name: "greater-equal rejects lower", constraint: ">=1.0.0", version: "0.9.9", want: false},
		{name: "less matches", constraint: "<2.0.0", version: "1.9.9", want: true},
		{name: "less rejects equal", constraint: "<2.0.0", version: "2.0.0", want: false},
		{name: "less-equal matches equal", constraint: "<=2.0.0", version: "2.0.0", want: true},
		{name: "less-equal rejects higher", constraint: "<=2.0.0", version: "2.0.1", want: false},
		{name: "range matches lower bound", constraint: ">=1.2.0 <2.0.0", version: "1.2.0", want: true},
		{name: "range matches middle", constraint: ">=1.2.0 <2.0.0", version: "1.5.3", want: true},
		{name: "range rejects below", constraint: ">=1.2.0 <2.0.0", version: "1.1.9", want: false},
		{name: "range rejects upper bound", constraint: ">=1.2.0 <2.0.0", version: "2.0.0", want: false},
		// Pure SemVer precedence: a pre-release sorts below its release, so
		// 2.0.0-rc.1 < 2.0.0 satisfies the upper bound, and 1.2.0-rc.1 fails
		// a >=1.2.0 lower bound.
		{name: "pre-release satisfies upper bound", constraint: "<2.0.0", version: "2.0.0-rc.1", want: true},
		{name: "pre-release fails lower bound of its own release", constraint: ">=1.2.0", version: "1.2.0-rc.1", want: false},
		{name: "pre-release within range", constraint: ">=1.2.0 <2.0.0", version: "2.0.0-rc.1", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseConstraint(tt.constraint)
			if err != nil {
				t.Fatalf("ParseConstraint(%q) error = %v", tt.constraint, err)
			}
			got, err := c.MatchString(tt.version)
			if err != nil {
				t.Fatalf("MatchString(%q) error = %v", tt.version, err)
			}
			if got != tt.want {
				t.Errorf("%q against %q = %v, want %v", tt.version, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestMatchStringInvalidVersion(t *testing.T) {
	c, err := ParseConstraint(">=1.0.0")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	if _, err := c.MatchString("1.2"); err == nil || !strings.Contains(err.Error(), `invalid version "1.2"`) {
		t.Errorf("MatchString(%q) error = %v, want an invalid version error", "1.2", err)
	}
}
