package format

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

// What aliases and merge keys may add to a document. A few hundred bytes of
// nested aliases can stand for gigabytes of JSON (the "billion laughs"
// pattern), so expansion stops as soon as either limit is passed. A node is a
// value, a mapping key, or a mapping merged by a merge key, counted each time
// an alias brings it into the output; bytes are bytes of the pretty-printed
// output. The most alias-heavy real config measured, GitLab's
// rules.gitlab-ci.yml, adds 8,873 nodes and 389 KiB.
const (
	maxAliasNodes = 250_000
	maxAliasBytes = 16 << 20
)

// maxYAMLDepth is go-yaml's own decode depth limit, applied here to nested
// mappings, sequences, and merge keys.
const maxYAMLDepth = 10_000

// yamlLimits caps one conversion. YAMLToJSON uses the constants above and
// the output floor IndentJSON uses; tests use smaller limits to reach the
// edges without megabytes of output.
type yamlLimits struct {
	aliasNodes, aliasBytes, depth, outputFloor int
}

func (l yamlLimits) nodesError() error {
	return fmt.Errorf("alias expansion exceeds the size limit: aliases and merge keys add more than %d nodes (keys, values, and merged mappings); %s", l.aliasNodes, l.capped())
}

func (l yamlLimits) bytesError() error {
	return fmt.Errorf("alias expansion exceeds the size limit: aliases and merge keys add more than %s of JSON output; %s", byteCount(l.aliasBytes), l.capped())
}

func (l yamlLimits) capped() string {
	return fmt.Sprintf("expansion is capped at %d nodes and %s of JSON output", l.aliasNodes, byteCount(l.aliasBytes))
}

func (l yamlLimits) depthError() error {
	return fmt.Errorf("invalid YAML: nesting exceeds the depth limit of %d levels", l.depth)
}

func byteCount(n int) string {
	if n%(1<<20) == 0 {
		return fmt.Sprintf("%d MiB", n>>20)
	}
	return fmt.Sprintf("%d bytes", n)
}

// YAMLToJSON converts the single YAML document in r to pretty-printed JSON in
// w. It writes JSON straight from go-yaml's syntax tree: go-yaml decodes each
// scalar, while mappings, sequences, anchors, aliases, and merge keys are
// handled here, so key order survives, aliases and merge keys follow the
// YAML spec, and alias expansion is bounded by maxAliasNodes and
// maxAliasBytes. Its output is limited as IndentJSON limits it, checked as
// it is written. Nothing is written to w unless the whole conversion
// succeeds.
func YAMLToJSON(w io.Writer, r io.Reader) error {
	return yamlToJSON(w, r, yamlLimits{aliasNodes: maxAliasNodes, aliasBytes: maxAliasBytes, depth: maxYAMLDepth, outputFloor: outputFloor})
}

func yamlToJSON(w io.Writer, r io.Reader, limits yamlLimits) error {
	data, err := ReadText(r)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("empty input")
	}
	c, err := convertYAML(data, limits)
	if err != nil {
		return err
	}
	if _, err := w.Write(c.out.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// convertYAML converts data into the returned converter's buffer. The
// converter is returned even on failure, so tests can see how much it wrote
// and how much work it did.
func convertYAML(data []byte, limits yamlLimits) (*yamlConverter, error) {
	c := &yamlConverter{
		limits:  limits,
		output:  outputLimit(limits.outputFloor, len(data)),
		anchors: map[anchorName]anchorRef{},
		open:    map[*ast.AnchorNode]bool{},
		targets: map[*ast.AliasNode]ast.Node{},
		merges:  map[*ast.MappingValueNode]bool{},
		scalars: map[ast.Node]*scalarForm{},
		keyIDs:  map[string]int{},
	}
	tokens := tokenize(data)
	if err := checkNesting(tokens, limits); err != nil {
		return c, err
	}
	if n := countDocuments(tokens); n > 1 {
		return c, fmt.Errorf("input has %d YAML documents; expected one", n)
	}
	file, err := parser.Parse(tokens, 0)
	if err != nil {
		return c, fmt.Errorf("invalid YAML: %w", err)
	}
	var body ast.Node
	docs := 0
	for _, doc := range file.Docs {
		if d, ok := doc.Body.(*ast.DirectiveNode); ok {
			if err := checkDirective(d); err != nil {
				return c, err
			}
			continue
		}
		docs++
		body = doc.Body
	}
	if docs > 1 {
		return c, fmt.Errorf("input has %d YAML documents; expected one", docs)
	}
	if err := c.resolve(body, 0); err != nil {
		return c, err
	}
	if c.lines > c.output {
		return c, outputLimitError(c.limits.outputFloor, c.output, c.nesting, 2)
	}
	if err := c.value(body, 0); err != nil {
		return c, err
	}
	// newline keeps the output near its limit as it grows; this makes the
	// limit exact, counting what follows the last line break.
	if int64(c.out.Len()) > c.output {
		return c, outputLimitError(c.limits.outputFloor, c.output, c.nesting, 2)
	}
	return c, nil
}

// tokenize splits data into go-yaml's tokens. go-yaml's lexer drops a tag
// that ends the input with no line break after it, so a: !!str loses its
// tag; such input is read with the line break added. That changes no
// scalar: a line break at the end matters only to a block scalar that ends
// there, and here the input ends with a tag instead.
func tokenize(data []byte) token.Tokens {
	tokens := lexer.Tokenize(string(data))
	if len(data) == 0 || data[len(data)-1] == '\n' || !bytes.Contains(data[bytes.LastIndexByte(data, '\n')+1:], []byte("!")) {
		return tokens
	}
	withBreak := lexer.Tokenize(string(data) + "\n")
	if countTags(withBreak) > countTags(tokens) {
		return withBreak
	}
	return tokens
}

func countTags(tokens token.Tokens) int {
	n := 0
	for _, tk := range tokens {
		if tk.Type == token.TagType {
			n++
		}
	}
	return n
}

// checkDirective accepts a %YAML 1.1 or 1.2 directive: go-yaml reads the
// document after it the same way whatever the version. A %TAG directive
// changes what the document's tags mean, which go-yaml's parser gets wrong
// (it keeps only the next token as the tagged value), so it is rejected.
func checkDirective(d *ast.DirectiveNode) error {
	name := ""
	if d.Name != nil {
		name = d.Name.GetToken().Value
	}
	switch name {
	case "YAML":
		version := ""
		if len(d.Values) == 1 {
			version = d.Values[0].GetToken().Value
		}
		if version == "1.1" || version == "1.2" {
			return nil
		}
		return fmt.Errorf("unsupported directive %%YAML %s at %s: yaml2json reads YAML 1.1 and 1.2", version, position(d))
	case "TAG":
		return fmt.Errorf("unsupported directive %%TAG at %s: yaml2json cannot resolve tag handles that %%TAG defines", position(d))
	}
	return fmt.Errorf("unsupported directive %%%s at %s: yaml2json reads only %%YAML directives", name, position(d))
}

// checkNesting rejects nesting past the depth limit before go-yaml parses it,
// because go-yaml's parser has no depth limit and uses memory quadratic in
// the depth: 100,000 nested brackets (200 KB) need about 15 GB. It counts the
// nesting the tokens prove: open flow collections ([ and {), plus block
// sequence entries and explicit keys written on one line (- - - x), which
// each open a level inside the one before. That count is never more than the
// real depth, so it rejects nothing the exact checks after parsing would
// accept. Nesting by indentation alone is not counted: it takes input
// quadratic in the depth, so the parser's memory stays in proportion to the
// input.
func checkNesting(tokens token.Tokens, limits yamlLimits) error {
	flow, compact, line := 0, 0, 0
	for _, tk := range tokens {
		if flow == 0 && tk.Position.Line != line {
			compact, line = 0, tk.Position.Line
		}
		switch tk.Type {
		case token.SequenceStartType, token.MappingStartType:
			flow++
		case token.SequenceEndType, token.MappingEndType:
			if flow > 0 {
				flow--
			}
		case token.SequenceEntryType, token.MappingKeyType:
			if flow == 0 {
				compact++
			}
		}
		if flow+compact > limits.depth {
			return limits.depthError()
		}
	}
	return nil
}

// countDocuments counts the documents in a token stream: one for each
// document header (---), and one for content that comes before any header
// or after a document end (...). go-yaml's parser gets this wrong for an
// empty document followed by another, as in "---\n---\na: 1", which it
// reads as one empty document, silently dropping the rest.
func countDocuments(tokens token.Tokens) int {
	docs, open, directiveLine := 0, false, 0
	for _, tk := range tokens {
		switch {
		case tk.Type == token.DirectiveType:
			directiveLine = tk.Position.Line
		case tk.Position.Line == directiveLine, tk.Type == token.CommentType:
		case tk.Type == token.DocumentHeaderType:
			docs, open = docs+1, true
		case tk.Type == token.DocumentEndType:
			open = false
		case !open:
			docs, open = docs+1, true
		}
	}
	return docs
}

// anchorName identifies an anchor as written. go-yaml reads a quoted anchor
// name as a quoted scalar, so the quoting style is kept apart from the
// unquoted text: &"x" and *x are different names, as in the YAML spec.
type anchorName struct {
	style token.Type
	text  string
}

func nameOf(n ast.Node) anchorName {
	if n == nil || n.GetToken() == nil {
		return anchorName{}
	}
	tk := n.GetToken()
	style := token.StringType
	if tk.Type == token.DoubleQuoteType || tk.Type == token.SingleQuoteType {
		style = tk.Type
	}
	return anchorName{style, tk.Value}
}

// anchorRef is an anchor and the node it marks. A tag and an anchor are both
// properties of one node, in either order, so the node includes a tag
// written before the anchor: in !!str &x 5, *x stands for "5". go-yaml puts
// that tag outside the anchor, and its decoder drops it from the alias.
type anchorRef struct {
	anchor *ast.AnchorNode
	node   ast.Node
}

// yamlConverter writes the JSON for a syntax tree and keeps the alias
// budget: everything written while an alias is being expanded counts toward
// it, and so does every node visited there, even one that writes nothing
// (merging an empty mapping). Each node is unwrapped in a bounded number of
// steps, and a scalar reached through an alias is decoded once, so an alias
// use costs time in proportion to the nodes it visits and the bytes it
// writes. Both are checked before every node and before every !!binary
// array, the one value whose output grows with its depth, so aliases add at
// most time and memory in proportion to the limit. The rest is the
// document's own pretty-printed size, which grows with nesting depth as
// well as length.
type yamlConverter struct {
	limits  yamlLimits
	out     bytes.Buffer
	output  int64 // the most out may hold
	nesting int   // the deepest mapping or sequence seen, counting from 1
	lines   int64 // the least output resolve has found the document must write
	merging int   // merge-key values being resolved, whose pairs are not counted in lines

	// Set by resolve, before anything is written.
	anchors map[anchorName]anchorRef
	open    map[*ast.AnchorNode]bool // anchors whose node is being resolved
	targets map[*ast.AliasNode]ast.Node
	merges  map[*ast.MappingValueNode]bool // pairs whose key is the merge key

	scalars map[ast.Node]*scalarForm // scalars decoded through an alias
	keyIDs  map[string]int           // every key string, numbered from 1
	work    int                      // bytes of scalar text decoded and keys formatted; measured for tests, not limited

	aliasDepth int // aliases being expanded, nested
	aliasStart int // out.Len() when the outermost of them began
	aliasBytes int // bytes written by aliases already expanded
	aliasNodes int
}

// scalarForm is a decoded scalar and, once it has been used as a mapping
// key, its key.
type scalarForm struct {
	value any
	key   objectKey
}

// objectKey is a JSON object key and its number in keyIDs, from 1.
type objectKey struct {
	text string
	id   int
}

// resolve binds every alias to the most recent anchor of that name earlier
// in the document and records which pairs are merge keys, before anything is
// written. go-yaml's decoder instead resolves an alias inside a merged
// mapping against the anchors in effect where it is merged, which lets a
// short document rebind anchors into an exponential expansion.
func (c *yamlConverter) resolve(n ast.Node, level int) error {
	var tag *ast.TagNode
	var anchor *ast.AnchorNode
	start := n
unwrap:
	for {
		switch t := n.(type) {
		case *ast.TagNode:
			// YAML gives a node at most one tag. go-yaml accepts more, and
			// after three it silently drops the rest of the mapping.
			if tag != nil {
				return fmt.Errorf("invalid YAML: node at %s has more than one tag (%s and %s)", position(t), tag.Start.Value, t.Start.Value)
			}
			tag = t
			n = t.Value
		case *ast.MappingKeyNode:
			n = t.Value
		case *ast.AnchorNode:
			if anchor != nil {
				return fmt.Errorf("invalid YAML: node at %s has more than one anchor", position(t))
			}
			anchor = t
			c.anchors[nameOf(t.Name)] = anchorRef{t, start}
			c.open[t] = true
			defer delete(c.open, t)
			n = t.Value
		default:
			break unwrap
		}
	}
	switch t := n.(type) {
	case *ast.AliasNode:
		// The YAML spec gives an alias no tag of its own; go-yaml accepts
		// one, but rejecting it keeps every tag on the node it was written on.
		if tag != nil {
			return fmt.Errorf("invalid YAML: alias %s at %s has a tag (%s), which YAML does not allow on an alias", aliasText(t), position(t), tag.Start.Value)
		}
		ref, ok := c.anchors[nameOf(t.Value)]
		if !ok {
			return fmt.Errorf("invalid YAML: alias %s at %s does not refer to an earlier anchor", aliasText(t), position(t))
		}
		if c.open[ref.anchor] {
			return fmt.Errorf("invalid YAML: alias %s at %s refers to an anchor that contains it", aliasText(t), position(t))
		}
		c.targets[t] = ref.node
	case *ast.MappingNode:
		return c.resolvePairs(t.Values, level)
	case *ast.MappingValueNode:
		return c.resolvePairs([]*ast.MappingValueNode{t}, level)
	case *ast.SequenceNode:
		if level >= c.limits.depth {
			return c.limits.depthError()
		}
		c.nesting = max(c.nesting, level+1)
		if len(t.Values) > 0 {
			c.countLine(level)
		}
		for _, v := range t.Values {
			c.countLine(level + 1)
			if err := c.resolve(v, level+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *yamlConverter) resolvePairs(pairs []*ast.MappingValueNode, level int) error {
	if level >= c.limits.depth {
		return c.limits.depthError()
	}
	c.nesting = max(c.nesting, level+1)
	explicit := false
	for _, p := range pairs {
		merge := isMergeKey(p.Key)
		if merge {
			c.merges[p] = true
		} else {
			c.countLine(level + 1)
			explicit = true
		}
		if err := checkValueIndent(p); err != nil {
			return err
		}
		if err := c.resolve(p.Key, level+1); err != nil {
			return err
		}
		if merge {
			c.merging++
		}
		err := c.resolve(p.Value, level+1)
		if merge {
			c.merging--
		}
		if err != nil {
			return err
		}
	}
	if explicit {
		c.countLine(level)
	}
	return nil
}

// countLine adds a line at level to the output the document must write: a
// sequence item or an explicit mapping key, each written once on its own
// line wherever it appears, or the closing line of a sequence or mapping
// with one. Pairs inside a merge-key value are skipped, since a key there
// may lose to one written explicitly.
func (c *yamlConverter) countLine(level int) {
	if c.merging == 0 {
		c.lines += 1 + 2*int64(level)
	}
}

// checkValueIndent rejects a block mapping that go-yaml placed under a key
// although it is not indented past the key. go-yaml does this when a tag
// has an empty value and another key follows (a: !!str &x, then b: 1
// on the next line), turning the sibling key into the tagged value.
func checkValueIndent(p *ast.MappingValueNode) error {
	var first *ast.MappingValueNode
	switch t := unwrapValue(p.Value).(type) {
	case *ast.MappingNode:
		if t.IsFlowStyle || len(t.Values) == 0 {
			return nil
		}
		first = t.Values[0]
	case *ast.MappingValueNode:
		first = t
	default:
		if tag, ok := t.(*ast.TagNode); ok {
			if m, ok := unwrapValue(tag.Value).(*ast.MappingNode); ok && !m.IsFlowStyle && len(m.Values) > 0 {
				first = m.Values[0]
			}
		}
	}
	if first == nil || p.Key == nil || first.Key == nil {
		return nil
	}
	key, inner := p.Key.GetToken().Position, first.Key.GetToken().Position
	if inner.Line > key.Line && inner.Column <= key.Column {
		return fmt.Errorf("unsupported YAML: go-yaml's parser misreads the empty value at %s, which has a tag and is followed by another key; write the value explicitly (\"\" or null)", position(p.Value))
	}
	return nil
}

func (c *yamlConverter) enterAlias() {
	if c.aliasDepth == 0 {
		c.aliasStart = c.out.Len()
	}
	c.aliasDepth++
}

// exitAlias ends an expansion begun by enterAlias and returns err, or the
// budget error if the expansion's last write passed the limit.
func (c *yamlConverter) exitAlias(err error) error {
	c.aliasDepth--
	if c.aliasDepth == 0 {
		c.aliasBytes += c.out.Len() - c.aliasStart
	}
	if err != nil {
		return err
	}
	return c.reserve(0)
}

// reserve checks that n more bytes of output still fit the budget, if an
// alias is being expanded, together with what aliases have written so far.
func (c *yamlConverter) reserve(n int) error {
	added := c.aliasBytes
	if c.aliasDepth > 0 {
		added += c.out.Len() - c.aliasStart + n
	}
	if added > c.limits.aliasBytes {
		return c.limits.bytesError()
	}
	return nil
}

// visit counts n nodes against the budget if an alias is being expanded.
func (c *yamlConverter) visit(n int) error {
	if c.aliasDepth == 0 {
		return nil
	}
	return c.hop(n)
}

// hop counts n nodes against the budget unconditionally, for following an
// alias to its anchor.
func (c *yamlConverter) hop(n int) error {
	c.aliasNodes += n
	if c.aliasNodes > c.limits.aliasNodes {
		return c.limits.nodesError()
	}
	return c.reserve(0)
}

func (c *yamlConverter) value(n ast.Node, level int) error {
	if err := c.visit(1); err != nil {
		return err
	}
	switch t := unwrapValue(n).(type) {
	case *ast.AliasNode:
		c.enterAlias()
		return c.exitAlias(c.value(c.targets[t], level))
	case *ast.MappingNode:
		return c.mapping(t.Values, level)
	case *ast.MappingValueNode:
		return c.mapping([]*ast.MappingValueNode{t}, level)
	case *ast.SequenceNode:
		return c.sequence(t.Values, level)
	default:
		v, _, err := c.scalar(t, false)
		if err != nil {
			return err
		}
		return c.writeScalar(v, t, level)
	}
}

// unwrapValue skips anchors and the tags that leave a value as it is (!!map,
// !!seq, and custom tags, as go-yaml's decoder does), stopping at a tag that
// converts a scalar. resolve allows one anchor and one tag per node, so this
// takes at most a few steps.
func unwrapValue(n ast.Node) ast.Node {
	for {
		switch t := n.(type) {
		case *ast.AnchorNode:
			n = t.Value
		case *ast.MappingKeyNode:
			n = t.Value
		case *ast.TagNode:
			if scalarTag(t) {
				return t
			}
			n = t.Value
		default:
			return n
		}
	}
}

func scalarTag(t *ast.TagNode) bool {
	switch token.ReservedTagKeyword(t.Start.Value) {
	case token.StringTag, token.IntegerTag, token.FloatTag, token.BooleanTag,
		token.NullTag, token.BinaryTag, token.TimestampTag:
		return true
	}
	return false
}

func (c *yamlConverter) sequence(values []ast.Node, level int) error {
	if level >= c.limits.depth {
		return c.limits.depthError()
	}
	c.nesting = max(c.nesting, level+1)
	if len(values) == 0 {
		c.out.WriteString("[]")
		return nil
	}
	c.out.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			c.out.WriteByte(',')
		}
		if err := c.newline(level + 1); err != nil {
			return err
		}
		if err := c.value(v, level+1); err != nil {
			return err
		}
	}
	if err := c.newline(level); err != nil {
		return err
	}
	c.out.WriteByte(']')
	return nil
}

// smallMapping is the number of keys up to which a mapping without merge
// keys finds duplicate keys by scanning, rather than with a map.
const smallMapping = 8

func (c *yamlConverter) mapping(pairs []*ast.MappingValueNode, level int) error {
	if level >= c.limits.depth {
		return c.limits.depthError()
	}
	c.nesting = max(c.nesting, level+1)
	for _, p := range pairs {
		if c.merges[p] {
			return c.mergedMapping(pairs, level)
		}
	}
	if len(pairs) == 0 {
		c.out.WriteString("{}")
		return nil
	}
	var small [smallMapping]int
	ids := small[:0]
	var seen map[int]bool
	if len(pairs) > smallMapping {
		seen = make(map[int]bool, len(pairs))
	}
	for i, p := range pairs {
		if err := c.visit(1); err != nil {
			return err
		}
		k, aliased, err := c.key(p.Key)
		if err != nil {
			return err
		}
		dup := false
		if seen != nil {
			dup = seen[k.id]
			seen[k.id] = true
		} else {
			for _, id := range ids {
				dup = dup || id == k.id
			}
			ids = append(ids, k.id)
		}
		if dup {
			return duplicateKeyError(k.text, p.Key)
		}
		if err := c.writeKey(k.text, aliased, level, i == 0); err != nil {
			return err
		}
		if err := c.value(p.Value, level+1); err != nil {
			return err
		}
	}
	if err := c.newline(level); err != nil {
		return err
	}
	c.out.WriteByte('}')
	return nil
}

func duplicateKeyError(k string, at ast.Node) error {
	return fmt.Errorf("invalid YAML: duplicate mapping key %s at %s", strconv.Quote(k), position(at))
}

// jsonObject is a mapping with merge keys being written. A key appears in
// it once: written holds the keys already written, and blocked counts, for
// each key, the mappings on the current merge path that define it
// explicitly. Keys are their numbers in keyIDs.
type jsonObject struct {
	level   int
	written map[int]bool
	blocked map[int]int
}

func (c *yamlConverter) mergedMapping(pairs []*ast.MappingValueNode, level int) error {
	o := &jsonObject{level: level, written: map[int]bool{}, blocked: map[int]int{}}
	if err := c.members(o, pairs, 0); err != nil {
		return err
	}
	if len(o.written) == 0 {
		c.out.WriteString("{}")
		return nil
	}
	if err := c.newline(level); err != nil {
		return err
	}
	c.out.WriteByte('}')
	return nil
}

// members writes the pairs of one mapping into o, following the YAML
// merge-key spec: a key the mapping defines explicitly wins over the same
// key from a merge (wherever the merge key appears), and in a list of merged
// mappings an earlier one wins over a later one. Explicit keys are written
// where they appear and merged keys where the merge key appears, so the
// result is go-yaml's output with the losing duplicates dropped.
func (c *yamlConverter) members(o *jsonObject, pairs []*ast.MappingValueNode, mergeDepth int) error {
	type explicitKey struct {
		objectKey
		aliased bool
	}
	keys := make([]explicitKey, len(pairs))
	own := make(map[int]bool, len(pairs))
	defer func() {
		for id := range own {
			o.blocked[id]--
		}
	}()
	for i, p := range pairs {
		if c.merges[p] {
			continue
		}
		if err := c.visit(1); err != nil {
			return err
		}
		k, aliased, err := c.key(p.Key)
		if err != nil {
			return err
		}
		if own[k.id] {
			return duplicateKeyError(k.text, p.Key)
		}
		own[k.id] = true
		o.blocked[k.id]++
		keys[i] = explicitKey{k, aliased}
	}
	for i, p := range pairs {
		if c.merges[p] {
			if mergeDepth >= c.limits.depth {
				return c.limits.depthError()
			}
			sources, err := c.mergeSources(p.Value)
			if err != nil {
				return err
			}
			for _, src := range sources {
				if err := c.merge(o, src, mergeDepth); err != nil {
					return err
				}
			}
			continue
		}
		k := keys[i]
		// This mapping's own definition accounts for one block; any other
		// is a mapping further up the merge path, which wins.
		if o.written[k.id] || o.blocked[k.id] > 1 {
			continue
		}
		first := len(o.written) == 0
		o.written[k.id] = true
		if err := c.writeKey(k.text, k.aliased, o.level, first); err != nil {
			return err
		}
		if err := c.value(p.Value, o.level+1); err != nil {
			return err
		}
	}
	return nil
}

// writeKey starts a member of the object at level and writes its key,
// counted as alias output if the key came through an alias.
func (c *yamlConverter) writeKey(k string, aliased bool, level int, first bool) error {
	if aliased {
		c.enterAlias()
	}
	if first {
		c.out.WriteByte('{')
	} else {
		c.out.WriteByte(',')
	}
	if err := c.newline(level + 1); err != nil {
		return err
	}
	c.writeString(k)
	c.out.WriteString(": ")
	if aliased {
		return c.exitAlias(nil)
	}
	return nil
}

type mergeSource struct {
	pairs   []*ast.MappingValueNode
	aliased bool
}

func (c *yamlConverter) merge(o *jsonObject, src mergeSource, mergeDepth int) error {
	if src.aliased {
		c.enterAlias()
	}
	err := c.visit(1)
	if err == nil {
		err = c.members(o, src.pairs, mergeDepth+1)
	}
	if src.aliased {
		err = c.exitAlias(err)
	}
	return err
}

// isMergeKey reports whether k is the merge key: a plain << scalar, or one
// tagged !!merge. go-yaml's parser marks << as a merge key only when a
// value follows it and it is not an explicit (?) key, but the spec depends
// on neither.
func isMergeKey(k ast.Node) bool {
	for {
		switch t := k.(type) {
		case *ast.MappingKeyNode:
			k = t.Value
		case *ast.TagNode:
			if token.ReservedTagKeyword(t.Start.Value) != token.MergeTag {
				return false
			}
			k = t.Value
		case *ast.MergeKeyNode:
			return true
		case *ast.StringNode:
			// The value alone is not enough: go-yaml reads "<\t<" as "<<".
			return t.Token.Type == token.StringType && strings.TrimSpace(t.Token.Origin) == "<<"
		default:
			return false
		}
	}
}

// mergeSources returns the mappings a merge key's value names: one mapping,
// or a list of them, each possibly through an alias.
func (c *yamlConverter) mergeSources(v ast.Node) ([]mergeSource, error) {
	n, aliased, err := c.unwrapMerge(v)
	if err != nil {
		return nil, err
	}
	if seq, ok := n.(*ast.SequenceNode); ok {
		sources := make([]mergeSource, 0, len(seq.Values))
		for _, e := range seq.Values {
			m, elemAliased, err := c.unwrapMerge(e)
			if err != nil {
				return nil, err
			}
			pairs, ok := mappingPairs(m)
			if !ok {
				return nil, fmt.Errorf("invalid YAML: merge list entry at %s is not a mapping", position(e))
			}
			sources = append(sources, mergeSource{pairs, aliased || elemAliased})
		}
		return sources, nil
	}
	pairs, ok := mappingPairs(n)
	if !ok {
		return nil, mergeValueError(v)
	}
	return []mergeSource{{pairs, aliased}}, nil
}

func (c *yamlConverter) unwrapMerge(n ast.Node) (ast.Node, bool, error) {
	n = unwrapValue(n)
	a, ok := n.(*ast.AliasNode)
	if !ok {
		return n, false, nil
	}
	if err := c.hop(1); err != nil {
		return nil, false, err
	}
	return unwrapValue(c.targets[a]), true, nil
}

func mappingPairs(n ast.Node) ([]*ast.MappingValueNode, bool) {
	switch t := n.(type) {
	case *ast.MappingNode:
		return t.Values, true
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{t}, true
	}
	return nil, false
}

func mergeValueError(v ast.Node) error {
	return fmt.Errorf("invalid YAML: merge key value at %s is not a mapping or a list of mappings", position(v))
}

// key returns the JSON object key for a mapping key node, formatted the way
// go-yaml formats a decoded key (null as "null", numbers and booleans with
// fmt.Sprint), except that a timestamp is written in RFC 3339, as it is as a
// value, and !!binary data as base64. It reports whether the key came
// through an alias.
func (c *yamlConverter) key(n ast.Node) (objectKey, bool, error) {
	node := unwrapValue(n)
	aliased := false
	if a, ok := node.(*ast.AliasNode); ok {
		if err := c.hop(1); err != nil {
			return objectKey{}, false, err
		}
		aliased = true
		node = unwrapValue(c.targets[a])
	}
	switch node.(type) {
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		return objectKey{}, false, fmt.Errorf("invalid YAML: mapping key at %s is a %s; JSON object keys must be scalars", position(n), kindOf(node))
	}
	v, f, err := c.scalar(node, aliased)
	if err != nil {
		return objectKey{}, false, err
	}
	if f != nil && f.key.id != 0 {
		return f.key, aliased, nil
	}
	var k objectKey
	switch x := v.(type) {
	case nil:
		k.text = "null"
	case string:
		k.text = x
	case time.Time:
		k.text = x.Format(time.RFC3339Nano)
	case []byte:
		k.text = base64.StdEncoding.EncodeToString(x)
	default:
		k.text = fmt.Sprint(x)
	}
	c.work += len(k.text)
	k.id = c.keyIDs[k.text]
	if k.id == 0 {
		k.id = len(c.keyIDs) + 1
		c.keyIDs[k.text] = k.id
	}
	if f != nil {
		f.key = k
	}
	return k, aliased, nil
}

func kindOf(n ast.Node) string {
	if _, ok := n.(*ast.SequenceNode); ok {
		return "sequence"
	}
	return "mapping"
}

// scalar returns the decoded scalar n. When n is reached through an alias,
// or inside an alias's expansion, it may be reached again, so it is decoded
// once and kept in the cache, which scalar also returns.
func (c *yamlConverter) scalar(n ast.Node, aliased bool) (any, *scalarForm, error) {
	if f := c.scalars[n]; f != nil {
		return f.value, f, nil
	}
	v, err := c.decodeScalar(n)
	if err != nil {
		return nil, nil, err
	}
	if !aliased && c.aliasDepth == 0 {
		return v, nil, nil
	}
	f := &scalarForm{value: v}
	c.scalars[n] = f
	return v, f, nil
}

// decodeScalar decodes a scalar node and its tag to the Go value go-yaml's
// decoder produces, with two exceptions: !!str and !!binary read the scalar
// as written (go-yaml turns !!str 0x1F into "31" and !!binary 1234 into an
// error), and a tag go-yaml would silently turn into a zero value is an
// error.
func (c *yamlConverter) decodeScalar(n ast.Node) (any, error) {
	var tag *ast.TagNode
	node := n
unwrap:
	for {
		switch t := node.(type) {
		case *ast.AnchorNode:
			node = t.Value
		case *ast.MappingKeyNode:
			node = t.Value
		case *ast.TagNode:
			if scalarTag(t) {
				tag = t
			}
			node = t.Value
		default:
			break unwrap
		}
	}
	var v any
	var text string // the scalar as written
	switch t := node.(type) {
	case nil:
	case *ast.LiteralNode:
		v, text = t.Value.Value, t.Value.Value
	case *ast.StringNode:
		v, text = t.Value, t.Value
	case *ast.NullNode:
		if t.Token.Type != token.ImplicitNullType {
			text = t.Token.Value
		}
	case *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode, *ast.InfinityNode, *ast.NanNode:
		v, text = t.(ast.ScalarNode).GetValue(), t.GetToken().Value
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		if tag != nil {
			return nil, fmt.Errorf("invalid YAML: %s at %s applies to a %s, not a scalar", tag.Start.Value, position(tag), kindOf(t))
		}
		return nil, fmt.Errorf("invalid YAML: unexpected %s at %s", kindOf(t), position(t))
	default:
		return nil, fmt.Errorf("invalid YAML: unsupported %s node at %s", t.Type(), position(t))
	}
	if tag == nil {
		return v, nil
	}
	switch token.ReservedTagKeyword(tag.Start.Value) {
	case token.StringTag:
		return text, nil
	case token.NullTag:
		// go-yaml decodes !!null as null without reading its value.
		switch text {
		case "", "~", "null", "Null", "NULL":
			return nil, nil
		}
		return nil, cannotConvert(text, tag)
	case token.BinaryTag:
		v = text
	}
	c.work += len(text)
	return convertTag(tag, v, text)
}

// timestampFormats are the layouts go-yaml's decoder accepts for !!timestamp.
var timestampFormats = []string{
	"2006-1-2T15:4:5.999999999Z07:00",
	"2006-1-2t15:4:5.999999999Z07:00",
	"2006-1-2 15:4:5.999999999",
	"2006-1-2",
}

func cannotConvert(text string, t *ast.TagNode) error {
	return fmt.Errorf("invalid YAML: cannot convert %s to %s at %s", strconv.Quote(text), t.Start.Value, position(t))
}

// convertTag applies a scalar tag to a decoded value the way go-yaml's
// decoder does, except that an integer keeps the whole unsigned 64-bit
// range. text is the scalar as written, for errors.
func convertTag(t *ast.TagNode, v any, text string) (any, error) {
	tag := token.ReservedTagKeyword(t.Start.Value)
	fail := func() (any, error) {
		return nil, cannotConvert(text, t)
	}
	switch tag {
	case token.IntegerTag:
		switch x := v.(type) {
		case int, int64, uint64:
			return x, nil
		case string:
			if i, err := strconv.ParseInt(x, 10, 64); err == nil {
				return i, nil
			}
			if u, err := strconv.ParseUint(x, 10, 64); err == nil {
				return u, nil
			}
		}
		return fail()
	case token.FloatTag:
		switch x := v.(type) {
		case int:
			return float64(x), nil
		case int64:
			return float64(x), nil
		case uint64:
			return float64(x), nil
		case float64:
			return x, nil
		case string:
			if f, err := strconv.ParseFloat(x, 64); err == nil {
				return f, nil
			}
		}
		return fail()
	case token.BooleanTag:
		s := strings.ToLower(fmt.Sprint(v))
		if b, err := strconv.ParseBool(s); err == nil {
			return b, nil
		}
		switch s {
		case "yes":
			return true, nil
		case "no":
			return false, nil
		}
		return fail()
	case token.BinaryTag:
		s, ok := v.(string)
		if !ok {
			return fail()
		}
		// The YAML binary type ignores whitespace in the base64 text, which
		// a folded or multi-line plain scalar turns into spaces.
		b, err := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				return -1
			}
			return r
		}, s))
		if err != nil {
			return fail()
		}
		return b, nil
	case token.TimestampTag:
		switch x := v.(type) {
		case time.Time:
			return x, nil
		case string:
			for _, layout := range timestampFormats {
				if ts, err := time.Parse(layout, x); err == nil {
					return ts, nil
				}
			}
		}
		return fail()
	}
	return v, nil
}

// writeScalar writes a decoded scalar the way go-yaml's JSON encoder does,
// except that strings are escaped as valid JSON and a value JSON cannot
// represent is an error.
func (c *yamlConverter) writeScalar(v any, at ast.Node, level int) error {
	switch x := v.(type) {
	case nil:
		c.out.WriteString("null")
	case string:
		c.writeString(x)
	case bool:
		c.out.WriteString(strconv.FormatBool(x))
	case int:
		c.out.WriteString(strconv.Itoa(x))
	case int64:
		c.out.WriteString(strconv.FormatInt(x, 10))
	case uint64:
		c.out.WriteString(strconv.FormatUint(x, 10))
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return fmt.Errorf("value at %s is %v, which JSON cannot represent", position(at), x)
		}
		s := strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(s, ".e") {
			s += ".0"
		}
		c.out.WriteString(s)
	case time.Time:
		c.writeString(x.Format(time.RFC3339Nano))
	case []byte:
		// go-yaml's encoder writes !!binary data as an array of byte values,
		// one line each, so its size is charged in full before writing.
		if err := c.visit(len(x)); err != nil {
			return err
		}
		if len(x) == 0 {
			c.out.WriteString("[]")
			return nil
		}
		if err := c.reserve(byteArraySize(x, level)); err != nil {
			return err
		}
		c.out.WriteByte('[')
		for i, b := range x {
			if i > 0 {
				c.out.WriteByte(',')
			}
			if err := c.newline(level + 1); err != nil {
				return err
			}
			c.out.WriteString(strconv.Itoa(int(b)))
		}
		if err := c.newline(level); err != nil {
			return err
		}
		c.out.WriteByte(']')
	default:
		return fmt.Errorf("value at %s has unsupported type %T", position(at), v)
	}
	return nil
}

// byteArraySize is the number of bytes writeScalar writes for a non-empty
// !!binary value at level.
func byteArraySize(x []byte, level int) int {
	size := len("[") + len(x)*(len(",\n")+2*(level+1)) - len(",") + len("\n") + 2*level + len("]")
	for _, b := range x {
		switch {
		case b < 10:
			size++
		case b < 100:
			size += 2
		default:
			size += 3
		}
	}
	return size
}

const indentSpaces = "                                                                "

// newline starts a line indented for level, as json.Indent with a
// two-space indent would, unless the line would take the output past its
// limit.
func (c *yamlConverter) newline(level int) error {
	if int64(c.out.Len())+1+2*int64(level) > c.output {
		return outputLimitError(c.limits.outputFloor, c.output, c.nesting, 2)
	}
	c.out.WriteByte('\n')
	for n := 2 * level; n > 0; {
		chunk := min(n, len(indentSpaces))
		c.out.WriteString(indentSpaces[:chunk])
		n -= chunk
	}
	return nil
}

// writeString writes s as a JSON string with the escapes Go's strconv.Quote
// uses, which is what go-yaml's encoder writes, except where strconv.Quote
// writes an escape JSON does not have (\a, \v, \x, \U): those become \u
// escapes, and invalid UTF-8 becomes U+FFFD.
func (c *yamlConverter) writeString(s string) {
	const hex = "0123456789abcdef"
	b := &c.out
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && width == 1:
			b.WriteString(`�`)
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteByte(byte(r))
		case strconv.IsPrint(r):
			b.WriteString(s[i : i+width])
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		default:
			units := []rune{r}
			if r >= 0x10000 {
				r1, r2 := utf16.EncodeRune(r)
				units = []rune{r1, r2}
			}
			for _, u := range units {
				b.WriteString(`\u`)
				b.WriteByte(hex[u>>12&0xf])
				b.WriteByte(hex[u>>8&0xf])
				b.WriteByte(hex[u>>4&0xf])
				b.WriteByte(hex[u&0xf])
			}
		}
		i += width
	}
	b.WriteByte('"')
}

func position(n ast.Node) string {
	if n == nil || n.GetToken() == nil || n.GetToken().Position == nil {
		return "an unknown position"
	}
	p := n.GetToken().Position
	return fmt.Sprintf("line %d, column %d", p.Line, p.Column)
}

func aliasText(a *ast.AliasNode) string {
	if a.Value == nil || a.Value.GetToken() == nil {
		return "*"
	}
	return "*" + a.Value.GetToken().Value
}
