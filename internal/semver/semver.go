// Package semver parses, compares, sorts, and constraint-checks Semantic
// Versioning 2.0.0 versions, implementing the grammar and precedence rules
// from semver.org exactly. A single leading "v" is accepted on input
// (ubiquitous in git tags); everything else is strict: no partial versions,
// no leading zeros, no unknown characters.
package semver

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Version is a parsed SemVer 2.0.0 version. Prerelease and Build hold the
// dot-separated identifiers; String returns the input exactly as written,
// including any leading "v" and build metadata.
type Version struct {
	Major, Minor, Patch uint64
	Prerelease          []string
	Build               []string
	original            string
}

func (v Version) String() string { return v.original }

// Parse parses s per the SemVer 2.0.0 grammar. One leading "v" is stripped
// before parsing but preserved in String.
func Parse(s string) (Version, error) {
	if s == "" {
		return Version{}, fmt.Errorf("empty version")
	}
	v := Version{original: s}
	rest := strings.TrimPrefix(s, "v")
	rest, buildPart, hasBuild := strings.Cut(rest, "+")
	rest, prePart, hasPre := strings.Cut(rest, "-")

	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version %q: expected MAJOR.MINOR.PATCH, got %d numeric component(s)", s, len(parts))
	}
	var err error
	for i, name := range []string{"major", "minor", "patch"} {
		var n uint64
		if n, err = coreNumber(s, name, parts[i]); err != nil {
			return Version{}, err
		}
		switch i {
		case 0:
			v.Major = n
		case 1:
			v.Minor = n
		case 2:
			v.Patch = n
		}
	}
	if hasPre {
		if v.Prerelease, err = identifiers(s, "pre-release", prePart, true); err != nil {
			return Version{}, err
		}
	}
	if hasBuild {
		if v.Build, err = identifiers(s, "build", buildPart, false); err != nil {
			return Version{}, err
		}
	}
	return v, nil
}

func coreNumber(orig, name, s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid version %q: %s component is empty", orig, name)
	}
	if !numeric(s) {
		return 0, fmt.Errorf("invalid version %q: %s component %q must contain only digits", orig, name, s)
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("invalid version %q: %s component %q has a leading zero", orig, name, s)
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %s component %q is out of range", orig, name, s)
	}
	return n, nil
}

func identifiers(orig, kind, s string, noLeadingZeros bool) ([]string, error) {
	ids := strings.Split(s, ".")
	for _, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("invalid version %q: empty %s identifier", orig, kind)
		}
		for i := 0; i < len(id); i++ {
			c := id[i]
			if c != '-' && (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
				return nil, fmt.Errorf("invalid version %q: %s identifier %q may contain only [0-9A-Za-z-]", orig, kind, id)
			}
		}
		if noLeadingZeros && numeric(id) && len(id) > 1 && id[0] == '0' {
			return nil, fmt.Errorf("invalid version %q: numeric %s identifier %q has a leading zero", orig, kind, id)
		}
	}
	return ids, nil
}

func numeric(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Compare orders a and b by SemVer 2.0.0 precedence: -1, 0, or 1. Build
// metadata is ignored, so versions differing only in build metadata are equal.
func Compare(a, b Version) int {
	if c := cmp.Compare(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Patch, b.Patch); c != 0 {
		return c
	}
	switch {
	case len(a.Prerelease) == 0 && len(b.Prerelease) == 0:
		return 0
	case len(a.Prerelease) == 0:
		return 1
	case len(b.Prerelease) == 0:
		return -1
	}
	for i := 0; i < len(a.Prerelease) && i < len(b.Prerelease); i++ {
		if c := compareIdentifier(a.Prerelease[i], b.Prerelease[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.Prerelease), len(b.Prerelease))
}

// compareIdentifier orders one pre-release identifier pair: numeric ones
// compare as numbers and rank below alphanumeric ones, which compare in ASCII
// order. Numeric identifiers have no leading zeros, so a longer one is a
// larger number and equal lengths compare digit by digit; this never
// overflows, whatever the magnitude.
func compareIdentifier(x, y string) int {
	xNum, yNum := numeric(x), numeric(y)
	switch {
	case xNum && yNum:
		if c := cmp.Compare(len(x), len(y)); c != 0 {
			return c
		}
		return strings.Compare(x, y)
	case xNum:
		return -1
	case yNum:
		return 1
	default:
		return strings.Compare(x, y)
	}
}

// CompareStrings parses a and b and compares them by precedence. Build
// metadata is ignored, so "1.0.0+linux" and "1.0.0+mac" compare equal.
func CompareStrings(a, b string) (int, error) {
	va, err := Parse(a)
	if err != nil {
		return 0, fmt.Errorf("first version: %w", err)
	}
	vb, err := Parse(b)
	if err != nil {
		return 0, fmt.Errorf("second version: %w", err)
	}
	return Compare(va, vb), nil
}

// Sort reads versions one per line from r, sorts them ascending by SemVer
// precedence, and writes them to w exactly as written. Blank lines are
// skipped; any invalid line fails with its line number and value. Versions
// with equal precedence (differing only in build metadata or a "v" prefix)
// are ordered by their original text, byte-wise ascending, so the output is
// deterministic and independent of input order; descending reverses the
// entire order, tiebreak included.
func Sort(w io.Writer, r io.Reader, descending bool) error {
	scanner := bufio.NewScanner(r)
	var versions []Version
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		v, err := Parse(text)
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		versions = append(versions, v)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	slices.SortFunc(versions, func(a, b Version) int {
		c := Compare(a, b)
		if c == 0 {
			c = strings.Compare(a.original, b.original)
		}
		if descending {
			return -c
		}
		return c
	})
	for _, v := range versions {
		if _, err := fmt.Fprintln(w, v); err != nil {
			return err
		}
	}
	return nil
}

// constraintOps are tried in order; two-character operators come before their
// one-character prefixes so ">=" isn't read as ">" of "=1.2.3".
var constraintOps = []string{">=", "<=", ">", "<", "="}

type comparator struct {
	op string
	v  Version
}

// Constraint is a conjunction of comparators: a version satisfies it only if
// every comparator holds.
type Constraint struct {
	comparators []comparator
}

// ParseConstraint parses space-separated comparators, each an operator (=, >,
// >=, <, <=) directly followed by a version, all ANDed. Ecosystem-specific
// range syntax (npm's ^, ~, ||) is rejected.
func ParseConstraint(s string) (Constraint, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return Constraint{}, fmt.Errorf(`empty constraint (expected space-separated comparators like ">=1.2.0 <2.0.0")`)
	}
	var c Constraint
	for _, f := range fields {
		op := ""
		for _, candidate := range constraintOps {
			if strings.HasPrefix(f, candidate) {
				op = candidate
				break
			}
		}
		if op == "" {
			if strings.HasPrefix(f, "^") || strings.HasPrefix(f, "~") || strings.Contains(f, "||") {
				return Constraint{}, fmt.Errorf("unsupported operator in %q: only =, >, >=, <, <= are supported (npm-style ^, ~, and || ranges are not)", f)
			}
			return Constraint{}, fmt.Errorf("comparator %q is missing an operator (use =%s for an exact match)", f, f)
		}
		v, err := Parse(f[len(op):])
		if err != nil {
			return Constraint{}, fmt.Errorf("comparator %q: %w", f, err)
		}
		c.comparators = append(c.comparators, comparator{op: op, v: v})
	}
	return c, nil
}

// Match reports whether v satisfies every comparator. Matching is pure SemVer
// precedence: a pre-release orders below its release, so 2.0.0-rc.1 does
// satisfy "<2.0.0", and build metadata never affects the result.
func (c Constraint) Match(v Version) bool {
	for _, comp := range c.comparators {
		r := Compare(v, comp.v)
		ok := false
		switch comp.op {
		case "=":
			ok = r == 0
		case ">":
			ok = r > 0
		case ">=":
			ok = r >= 0
		case "<":
			ok = r < 0
		case "<=":
			ok = r <= 0
		}
		if !ok {
			return false
		}
	}
	return true
}

// MatchString parses version and reports whether it satisfies the constraint.
func (c Constraint) MatchString(version string) (bool, error) {
	v, err := Parse(version)
	if err != nil {
		return false, err
	}
	return c.Match(v), nil
}
