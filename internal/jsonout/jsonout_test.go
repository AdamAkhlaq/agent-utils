package jsonout

import (
	"testing"
)

func TestMarshal(t *testing.T) {
	tests := []struct {
		name    string
		v       any
		want    string
		wantErr bool
	}{
		{name: "html characters as is", v: map[string]string{"<k>": "AT&T <b>"}, want: "{\n  \"<k>\": \"AT&T <b>\"\n}"},
		{name: "line and paragraph separators escaped", v: "a\u2028b\u2029c", want: `"a\u2028b\u2029c"`},
		{name: "control characters escaped", v: "\x01\t\n", want: `"\u0001\t\n"`},
		{name: "nested indentation", v: struct {
			A []int `json:"a"`
			B struct {
				C bool `json:"c"`
			} `json:"b"`
		}{A: []int{1, 2}}, want: "{\n  \"a\": [\n    1,\n    2\n  ],\n  \"b\": {\n    \"c\": false\n  }\n}"},
		{name: "empty array", v: []string{}, want: "[]"},
		{name: "null", v: nil, want: "null"},
		{name: "unsupported type", v: make(chan int), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Marshal() = %q, want %q", got, tt.want)
			}
		})
	}
}
