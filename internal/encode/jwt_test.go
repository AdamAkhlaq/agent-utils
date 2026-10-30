package encode

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// makeJWT assembles an unsigned test token from literal JSON segments, so
// each case shows exactly what it decodes.
func makeJWT(header, payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(header)) + "." + enc([]byte(payload)) + ".signature"
}

// the sample token from jwt.io, HS256-signed
const jwtIOToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
	"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ." +
	"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

const jwtIOWant = `{
  "header": {
    "alg": "HS256",
    "typ": "JWT"
  },
  "payload": {
    "sub": "1234567890",
    "name": "John Doe",
    "iat": 1516239022
  }
}`

func TestJWTDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "real token preserves claim order",
			input: jwtIOToken,
			want:  jwtIOWant,
		},
		{
			name:  "bearer prefix and surrounding whitespace ignored",
			input: "Bearer " + jwtIOToken + "\n",
			want:  jwtIOWant,
		},
		{
			name:  "unsecured token with empty signature",
			input: strings.TrimSuffix(makeJWT(`{"alg":"none"}`, `{}`), "signature"),
			want:  "{\n  \"header\": {\n    \"alg\": \"none\"\n  },\n  \"payload\": {}\n}",
		},
		{
			name:  "big number claim survives exactly",
			input: makeJWT(`{"alg":"none"}`, `{"id":12345678901234567890}`),
			want:  "{\n  \"header\": {\n    \"alg\": \"none\"\n  },\n  \"payload\": {\n    \"id\": 12345678901234567890\n  }\n}",
		},
		{
			name:  "padded base64 tolerated",
			input: base64.URLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + base64.URLEncoding.EncodeToString([]byte(`{}`)) + ".",
			want:  "{\n  \"header\": {\n    \"alg\": \"none\"\n  },\n  \"payload\": {}\n}",
		},
		{name: "empty input", input: "", wantErr: "expected 3 dot-separated parts, got 1"},
		{name: "not a token", input: "garbage", wantErr: "expected 3 dot-separated parts, got 1"},
		{name: "two parts", input: "a.b", wantErr: "expected 3 dot-separated parts, got 2"},
		{name: "jwe token", input: "a.b.c.d.e", wantErr: "JWE"},
		{name: "header not base64url", input: "!!!.e30.sig", wantErr: "decoding header"},
		{name: "header not JSON", input: makeJWT("hi", `{}`), wantErr: "header is not valid JSON"},
		{name: "payload not base64url", input: "e30.!!!.sig", wantErr: "decoding payload"},
		{name: "payload not JSON", input: makeJWT(`{}`, "hi"), wantErr: "payload is not valid JSON"},
		{name: "payload with trailing data", input: makeJWT(`{}`, `{}x`), wantErr: "payload is not valid JSON"},
		// 27 KB nested 9,999 deep would be 200 MB of output.
		{name: "payload nested too deep to pretty-print", input: makeJWT(`{"alg":"none"}`, `{"a":`+strings.Repeat("[", 9999)+strings.Repeat("]", 9999)+`}`),
			wantErr: "output size limit: pretty-printed output would exceed 64 MiB (the larger of 64 MiB and 64 times the input size): the JSON nests 10001 levels deep, with 2 spaces of indentation per level"},
		{name: "header nested too deep to pretty-print", input: makeJWT(strings.Repeat("[", 9999)+strings.Repeat("]", 9999), `{}`), wantErr: "output size limit"},
		{name: "header invalid UTF-8", input: makeJWT("{\"alg\":\"x\xff\"}", `{}`), wantErr: "header is not valid JSON: invalid UTF-8 at byte offset 9 (0xff)"},
		{name: "payload invalid UTF-8", input: makeJWT(`{}`, "{\"sub\":\"x\xffy\"}"), wantErr: "payload is not valid JSON: invalid UTF-8 at byte offset 9 (0xff)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := JWTDecode(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("JWTDecode() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("JWTDecode() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("JWTDecode() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("JWTDecode() = %q, want %q", got, tt.want)
			}
		})
	}
}
