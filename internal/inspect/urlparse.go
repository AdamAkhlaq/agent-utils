package inspect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// URLInfo describes a URL's components. Host is the hostname alone (no port,
// no IPv6 brackets); Port is a string because it is a URL component, not a
// number the caller should do arithmetic on. HasPassword reports whether the
// userinfo carried a password without ever exposing it. Opaque holds the
// scheme-specific part of non-hierarchical URLs like mailto:a@b. Query maps
// each key to its decoded values in order of appearance, never nil.
type URLInfo struct {
	Scheme      string
	Opaque      string
	User        string
	HasPassword bool
	Host        string
	Port        string
	Path        string
	RawQuery    string
	Query       url.Values
	Fragment    string
}

// ParseURL parses raw into its components. The query string is decoded
// strictly: a malformed escape is an error, not a silently dropped pair.
// Inputs without a scheme are parsed as net/url sees them, so "example.com/x"
// is a bare path with an empty host.
func ParseURL(raw string) (URLInfo, error) {
	if raw == "" {
		return URLInfo{}, fmt.Errorf("empty input")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return URLInfo{}, fmt.Errorf("parsing URL: %w", err)
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return URLInfo{}, fmt.Errorf("parsing query string: %w", err)
	}
	_, hasPassword := u.User.Password()
	return URLInfo{
		Scheme:      u.Scheme,
		Opaque:      u.Opaque,
		User:        u.User.Username(),
		HasPassword: hasPassword,
		Host:        u.Hostname(),
		Port:        u.Port(),
		Path:        u.Path,
		RawQuery:    u.RawQuery,
		Query:       query,
		Fragment:    u.Fragment,
	}, nil
}

// URLJSON renders info as an indented JSON object. Every field is always
// present (absent components are "" or {}), so consumers get a fixed schema;
// query keys are sorted by json.Marshal, keeping the output deterministic.
func URLJSON(info URLInfo) (string, error) {
	query := info.Query
	if query == nil {
		query = url.Values{}
	}
	repr := struct {
		Scheme      string     `json:"scheme"`
		Opaque      string     `json:"opaque"`
		User        string     `json:"user"`
		HasPassword bool       `json:"has_password"`
		Host        string     `json:"host"`
		Port        string     `json:"port"`
		Path        string     `json:"path"`
		RawQuery    string     `json:"raw_query"`
		Query       url.Values `json:"query"`
		Fragment    string     `json:"fragment"`
	}{
		info.Scheme, info.Opaque, info.User, info.HasPassword, info.Host,
		info.Port, info.Path, info.RawQuery, query, info.Fragment,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	// json.Marshal escapes &, <, and > to \u002x for HTML embedding; URLs
	// are full of &, and this output is not destined for HTML.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(repr); err != nil {
		return "", fmt.Errorf("encoding URL info: %w", err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
