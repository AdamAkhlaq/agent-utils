package format

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XMLToJSON converts the XML document in r to pretty-printed JSON in w using
// a fixed convention: an element becomes an object, attributes become
// "@"-prefixed keys, text alongside attributes or children goes under
// "#text", an element with only text becomes a plain string, and repeated
// sibling elements collapse into an array. Whitespace-only text between
// elements is dropped; meaningful text is trimmed of leading and trailing
// whitespace. CDATA is text; comments, processing instructions, the XML
// declaration, and DOCTYPE are dropped. Namespace prefixes stay part of the
// key name as written. Object keys are emitted in sorted order, which makes
// the output deterministic.
func XMLToJSON(w io.Writer, r io.Reader) error {
	data, err := ReadText(r)
	if err != nil {
		return err
	}
	doc, err := xmlDocument(data, maxXMLDepth)
	if err != nil {
		return err
	}
	return writeJSON(w, doc, len(data))
}

// maxXMLDepth caps element nesting so that every tree within it encodes:
// encoding/json refuses nesting past 10,000 levels (from Go 1.27), and the
// limit keeps output within that whatever the toolchain. In the worst case
// each element level costs two JSON levels (an object inside the array its
// repeated siblings collapse into), plus the object the root's name adds and
// the object of an innermost element with attributes.
const maxXMLDepth = 5_000

// xmlDocument parses data into the JSON value XMLToJSON writes, failing on
// elements nested deeper than maxDepth.
func xmlDocument(data []byte, maxDepth int) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	d := xmlDecoder{dec: dec, maxDepth: maxDepth}
	var rootKey string
	var rootVal any
	seenRoot := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// The decoder happily tokenizes a second top-level element
			// (verified empirically), so a single root is enforced here.
			if seenRoot {
				line, _ := dec.InputPos()
				return nil, fmt.Errorf("invalid XML: second root element <%s> at line %d; an XML document has exactly one root", d.ns.name(t.Name), line)
			}
			seenRoot = true
			rootKey, rootVal, err = d.element(t, 1)
			if err != nil {
				return nil, err
			}
		case xml.CharData:
			// Same story: text outside the root comes through as a plain
			// token, not a decoder error.
			if len(bytes.TrimSpace(t)) > 0 {
				line, _ := dec.InputPos()
				return nil, fmt.Errorf("invalid XML: text outside the root element at line %d", line)
			}
		}
	}
	if !seenRoot {
		return nil, fmt.Errorf("invalid XML: no root element found")
	}
	return map[string]any{rootKey: rootVal}, nil
}

type xmlDecoder struct {
	dec      *xml.Decoder
	ns       nsStack
	maxDepth int
}

// element consumes tokens from start, the element at the given depth (the
// root is 1), through its matching end tag and returns the element's key and
// JSON value.
func (d *xmlDecoder) element(start xml.StartElement, depth int) (string, any, error) {
	if depth > d.maxDepth {
		line, _ := d.dec.InputPos()
		return "", nil, fmt.Errorf("invalid XML: nesting exceeds the depth limit of %d levels at line %d", d.maxDepth, line)
	}
	d.ns.push(start.Attr)
	defer d.ns.pop()
	obj := make(map[string]any)
	for _, a := range start.Attr {
		obj["@"+d.ns.name(a.Name)] = a.Value
	}
	var text strings.Builder
	for {
		tok, err := d.dec.Token()
		if err != nil {
			return "", nil, fmt.Errorf("invalid XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			key, val, err := d.element(t, depth+1)
			if err != nil {
				return "", nil, err
			}
			addChild(obj, key, val)
		case xml.EndElement:
			name := d.ns.name(start.Name)
			content := strings.TrimSpace(text.String())
			if len(obj) == 0 {
				return name, content, nil
			}
			if content != "" {
				obj["#text"] = content
			}
			return name, obj, nil
		case xml.CharData:
			text.Write(t)
		}
	}
}

// addChild inserts a child element's value, collapsing repeated siblings with
// the same name into an array. No key collisions are possible: attribute keys
// carry an "@" prefix and "#text" is not a legal XML element name.
func addChild(obj map[string]any, key string, val any) {
	existing, ok := obj[key]
	if !ok {
		obj[key] = val
		return
	}
	if arr, ok := existing.([]any); ok {
		obj[key] = append(arr, val)
		return
	}
	obj[key] = []any{existing, val}
}

// nsStack reconstructs names as written. encoding/xml resolves a declared
// prefix to its namespace URI in Name.Space (an undeclared prefix stays as
// the literal prefix), so each element's xmlns declarations are tracked as a
// URI-to-prefix scope and looked up innermost-first.
type nsStack []map[string]string

func (s *nsStack) push(attrs []xml.Attr) {
	scope := make(map[string]string)
	for _, a := range attrs {
		switch {
		case a.Name.Space == "xmlns":
			scope[a.Value] = a.Name.Local
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			scope[a.Value] = ""
		}
	}
	*s = append(*s, scope)
}

func (s *nsStack) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s nsStack) name(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	if n.Space == "xmlns" {
		return "xmlns:" + n.Local
	}
	for i := len(s) - 1; i >= 0; i-- {
		if prefix, ok := s[i][n.Space]; ok {
			if prefix == "" {
				return n.Local
			}
			return prefix + ":" + n.Local
		}
	}
	return n.Space + ":" + n.Local
}
