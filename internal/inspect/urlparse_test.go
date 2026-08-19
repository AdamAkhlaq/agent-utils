package inspect

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    URLInfo
		wantErr string
	}{
		{
			name: "full url with user port multi-value query and fragment",
			raw:  "https://alice:s3cr3t@example.com:8443/a/b?tag=x&tag=y&q=1#frag",
			want: URLInfo{
				Scheme:      "https",
				User:        "alice",
				HasPassword: true,
				Host:        "example.com",
				Port:        "8443",
				Path:        "/a/b",
				RawQuery:    "tag=x&tag=y&q=1",
				Query:       url.Values{"tag": {"x", "y"}, "q": {"1"}},
				Fragment:    "frag",
			},
		},
		{
			name: "minimal url",
			raw:  "https://example.com",
			want: URLInfo{
				Scheme: "https",
				Host:   "example.com",
				Query:  url.Values{},
			},
		},
		{
			name: "encoded characters are decoded",
			raw:  "https://example.com/a%20b?q=x%26y",
			want: URLInfo{
				Scheme:   "https",
				Host:     "example.com",
				Path:     "/a b",
				RawQuery: "q=x%26y",
				Query:    url.Values{"q": {"x&y"}},
			},
		},
		{
			name: "repeated query keys preserve every value",
			raw:  "https://example.com/?a=1&a=2&a=3",
			want: URLInfo{
				Scheme:   "https",
				Host:     "example.com",
				Path:     "/",
				RawQuery: "a=1&a=2&a=3",
				Query:    url.Values{"a": {"1", "2", "3"}},
			},
		},
		{
			name: "username without password",
			raw:  "ftp://bob@example.com/",
			want: URLInfo{
				Scheme: "ftp",
				User:   "bob",
				Host:   "example.com",
				Path:   "/",
				Query:  url.Values{},
			},
		},
		{
			name: "ipv6 host loses brackets and keeps port separate",
			raw:  "http://[::1]:8080/path",
			want: URLInfo{
				Scheme: "http",
				Host:   "::1",
				Port:   "8080",
				Path:   "/path",
				Query:  url.Values{},
			},
		},
		{
			name: "scheme-less input parses as a bare path",
			raw:  "example.com/x?a=1",
			want: URLInfo{
				Path:     "example.com/x",
				RawQuery: "a=1",
				Query:    url.Values{"a": {"1"}},
			},
		},
		{
			name: "opaque url keeps the scheme-specific part",
			raw:  "mailto:someone@example.com",
			want: URLInfo{
				Scheme: "mailto",
				Opaque: "someone@example.com",
				Query:  url.Values{},
			},
		},
		{
			name:    "invalid url",
			raw:     "http://exa mple.com/",
			wantErr: "parsing URL",
		},
		{
			name:    "missing protocol scheme",
			raw:     "://bad",
			wantErr: "parsing URL",
		},
		{
			name:    "malformed query escape",
			raw:     "https://example.com/?bad=%zz",
			wantErr: "parsing query string",
		},
		{
			name:    "empty input",
			raw:     "",
			wantErr: "empty input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseURL() expected an error, got %+v", got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ParseURL() error = %q, want it to mention %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseURL() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseURL() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestURLJSON(t *testing.T) {
	t.Run("full info renders every field and redacts the password", func(t *testing.T) {
		info, err := ParseURL("https://alice:s3cr3t@example.com:8443/a/b?tag=x&tag=y&q=1#frag")
		if err != nil {
			t.Fatalf("ParseURL() error = %v", err)
		}
		got, err := URLJSON(info)
		if err != nil {
			t.Fatalf("URLJSON() error = %v", err)
		}
		want := `{
  "scheme": "https",
  "opaque": "",
  "user": "alice",
  "has_password": true,
  "host": "example.com",
  "port": "8443",
  "path": "/a/b",
  "raw_query": "tag=x&tag=y&q=1",
  "query": {
    "q": [
      "1"
    ],
    "tag": [
      "x",
      "y"
    ]
  },
  "fragment": "frag"
}`
		if got != want {
			t.Errorf("URLJSON() = %s, want %s", got, want)
		}
		if strings.Contains(got, "s3cr3t") {
			t.Errorf("URLJSON() leaked the password: %s", got)
		}
	})

	t.Run("minimal info keeps the fixed schema", func(t *testing.T) {
		info, err := ParseURL("https://example.com")
		if err != nil {
			t.Fatalf("ParseURL() error = %v", err)
		}
		got, err := URLJSON(info)
		if err != nil {
			t.Fatalf("URLJSON() error = %v", err)
		}
		want := `{
  "scheme": "https",
  "opaque": "",
  "user": "",
  "has_password": false,
  "host": "example.com",
  "port": "",
  "path": "",
  "raw_query": "",
  "query": {},
  "fragment": ""
}`
		if got != want {
			t.Errorf("URLJSON() = %s, want %s", got, want)
		}
	})

	t.Run("nil query renders as an empty object", func(t *testing.T) {
		got, err := URLJSON(URLInfo{Scheme: "https", Host: "example.com"})
		if err != nil {
			t.Fatalf("URLJSON() error = %v", err)
		}
		if !strings.Contains(got, `"query": {}`) {
			t.Errorf("URLJSON() query = %s, want an empty object", got)
		}
		if !json.Valid([]byte(got)) {
			t.Fatalf("URLJSON() output is not valid JSON: %q", got)
		}
	})
}
