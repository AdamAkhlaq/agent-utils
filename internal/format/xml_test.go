package format

import (
	"bytes"
	"strings"
	"testing"
)

func TestXMLToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "text-only element becomes a string",
			input: "<name>Ada</name>",
			want:  "{\n  \"name\": \"Ada\"\n}",
		},
		{
			name:  "attributes become @-prefixed keys",
			input: `<user id="7" role="admin"/>`,
			want:  "{\n  \"user\": {\n    \"@id\": \"7\",\n    \"@role\": \"admin\"\n  }\n}",
		},
		{
			name:  "text next to attributes goes under #text",
			input: `<user id="7">Ada</user>`,
			want:  "{\n  \"user\": {\n    \"#text\": \"Ada\",\n    \"@id\": \"7\"\n  }\n}",
		},
		{
			name:  "nested structure with sorted keys",
			input: "<service><port>80</port><name>api</name></service>",
			want:  "{\n  \"service\": {\n    \"name\": \"api\",\n    \"port\": \"80\"\n  }\n}",
		},
		{
			name:  "repeated siblings become an array",
			input: "<list><item>a</item><item>b</item><item>c</item></list>",
			want:  "{\n  \"list\": {\n    \"item\": [\n      \"a\",\n      \"b\",\n      \"c\"\n    ]\n  }\n}",
		},
		{
			name:  "single sibling stays a scalar, not a one-element array",
			input: "<list><item>a</item></list>",
			want:  "{\n  \"list\": {\n    \"item\": \"a\"\n  }\n}",
		},
		{
			name:  "mixed text and children",
			input: "<p>hello <b>bold</b> world</p>",
			want:  "{\n  \"p\": {\n    \"#text\": \"hello  world\",\n    \"b\": \"bold\"\n  }\n}",
		},
		{
			name:  "whitespace-only text between elements is dropped",
			input: "<a>\n  <b>1</b>\n  <c>2</c>\n</a>",
			want:  "{\n  \"a\": {\n    \"b\": \"1\",\n    \"c\": \"2\"\n  }\n}",
		},
		{
			name:  "meaningful text is trimmed",
			input: "<a>  spaced out  </a>",
			want:  "{\n  \"a\": \"spaced out\"\n}",
		},
		{
			name:  "empty element becomes an empty string",
			input: "<a/>",
			want:  "{\n  \"a\": \"\"\n}",
		},
		{
			name:  "CDATA is text and html characters stay literal",
			input: `<code><![CDATA[if (a < b && c > d) { return "x"; }]]></code>`,
			want:  "{\n  \"code\": \"if (a < b && c > d) { return \\\"x\\\"; }\"\n}",
		},
		{
			name:  "entities are decoded",
			input: "<a>fish &amp; chips</a>",
			want:  "{\n  \"a\": \"fish & chips\"\n}",
		},
		{
			name:  "comments are dropped",
			input: "<a><!-- note -->1<!-- more --></a>",
			want:  "{\n  \"a\": \"1\"\n}",
		},
		{
			name:  "declaration doctype and processing instructions are dropped",
			input: "<?xml version=\"1.0\"?>\n<!DOCTYPE a>\n<?pi data?>\n<a>1</a>",
			want:  "{\n  \"a\": \"1\"\n}",
		},
		{
			name:  "declared namespace prefix kept as written",
			input: `<x:a xmlns:x="urn:u"><x:b>1</x:b></x:a>`,
			want:  "{\n  \"x:a\": {\n    \"@xmlns:x\": \"urn:u\",\n    \"x:b\": \"1\"\n  }\n}",
		},
		{
			name:  "default namespace leaves names unprefixed",
			input: `<a xmlns="urn:d"><b>1</b></a>`,
			want:  "{\n  \"a\": {\n    \"@xmlns\": \"urn:d\",\n    \"b\": \"1\"\n  }\n}",
		},
		{
			name:  "undeclared namespace prefix kept as written",
			input: "<x:a><x:b>1</x:b></x:a>",
			want:  "{\n  \"x:a\": {\n    \"x:b\": \"1\"\n  }\n}",
		},
		{
			name:  "prefixed attribute kept as written",
			input: `<a xmlns:x="urn:u" x:id="1" plain="2"/>`,
			want:  "{\n  \"a\": {\n    \"@plain\": \"2\",\n    \"@x:id\": \"1\",\n    \"@xmlns:x\": \"urn:u\"\n  }\n}",
		},
		{
			name: "realistic rss snippet",
			input: `<rss version="2.0">
  <channel>
    <title>Example</title>
    <item><title>First</title><link>https://e.com/1</link></item>
    <item><title>Second</title><link>https://e.com/2</link></item>
  </channel>
</rss>`,
			want: "{\n  \"rss\": {\n    \"@version\": \"2.0\",\n    \"channel\": {\n      \"item\": [\n        {\n          \"link\": \"https://e.com/1\",\n          \"title\": \"First\"\n        },\n        {\n          \"link\": \"https://e.com/2\",\n          \"title\": \"Second\"\n        }\n      ],\n      \"title\": \"Example\"\n    }\n  }\n}",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "unclosed tag", input: "<a><b>", wantErr: "invalid XML"},
		{name: "mismatched close tag", input: "<a><b>1</a>", wantErr: "invalid XML"},
		{name: "garbage input", input: "not xml at all", wantErr: "text outside the root element at line 1"},
		{name: "text after the root", input: "<a>1</a>junk", wantErr: "text outside the root element"},
		{name: "second root element", input: "<a>1</a>\n<b>2</b>", wantErr: "second root element <b> at line 2"},
		{name: "comment-only document has no root", input: "<!-- just a comment -->", wantErr: "no root element"},
		{name: "unknown entity", input: "<a>&nbsp;</a>", wantErr: "invalid character entity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := XMLToJSON(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("XMLToJSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("XMLToJSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("XMLToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("XMLToJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestXMLToJSONOutputIsValidJSON(t *testing.T) {
	input := `<project><groupId>com.example</groupId><dependencies><dependency><artifactId>junit</artifactId></dependency></dependencies></project>`
	var out bytes.Buffer
	if err := XMLToJSON(&out, strings.NewReader(input)); err != nil {
		t.Fatalf("XMLToJSON() error = %v", err)
	}
	if err := JSONValid(&out); err != nil {
		t.Errorf("XMLToJSON() output is not valid JSON: %v", err)
	}
}
