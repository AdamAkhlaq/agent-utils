package digest

import (
	"errors"
	"strings"
	"testing"
)

func TestSum(t *testing.T) {
	tests := []struct {
		name    string
		algo    string
		input   string
		want    string
		wantErr string
	}{
		{name: "sha256 empty", algo: "sha256", input: "", want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{name: "sha256 abc", algo: "sha256", input: "abc", want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{name: "sha1 abc", algo: "sha1", input: "abc", want: "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{name: "sha512 abc", algo: "sha512", input: "abc", want: "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
		{name: "md5 abc", algo: "md5", input: "abc", want: "900150983cd24fb0d6963f7d28e17f72"},
		{name: "md5 empty", algo: "md5", input: "", want: "d41d8cd98f00b204e9800998ecf8427e"},
		{name: "unknown algorithm", algo: "sha3", wantErr: `unknown algorithm "sha3" (valid: sha256, sha1, sha512, md5)`},
		{name: "empty algorithm", algo: "", wantErr: `unknown algorithm "" (valid: sha256, sha1, sha512, md5)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sum(tt.algo, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Sum() = %q, expected an error", got)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("Sum() error = %q, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Sum() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Sum() = %q, want %q", got, tt.want)
			}
		})
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestSumReadError(t *testing.T) {
	readFail := errors.New("disk on fire")
	_, err := Sum("sha256", errReader{err: readFail})
	if !errors.Is(err, readFail) {
		t.Fatalf("Sum() error = %v, want it to wrap %v", err, readFail)
	}
}
