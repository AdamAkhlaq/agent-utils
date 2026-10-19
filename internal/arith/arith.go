// Package arith evaluates arithmetic expressions exactly, as rational numbers
// (math/big.Rat), for the calc command. Nothing is ever converted to floating
// point: decimal input is read exactly and the only rounding is the final
// rendering of a non-terminating decimal.
package arith

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

const (
	// maxBits caps the numerator and denominator of every literal and
	// intermediate result, so an expression like 10^10^10 fails at once
	// instead of exhausting memory.
	maxBits = 1 << 16
	// maxDigits is the number of decimal digits in a maxBits-bit number.
	maxDigits = 19729
	// maxLiteralDigits bounds the significant digits of a literal and the
	// places after its decimal point. No value within maxBits needs more in
	// its exact decimal form (2^-65536 has 65536 places), so anything calc
	// prints can be read back.
	maxLiteralDigits = maxBits
	// maxDepth caps nesting (parentheses, signs, chained powers), which the
	// parser follows recursively.
	maxDepth = 1000
	// defaultPlaces is how far a non-terminating decimal is rounded when no
	// scale is given.
	defaultPlaces = 20
)

var errEmpty = errors.New("empty expression")

// Eval parses expr and evaluates it exactly. The grammar, loosest binding
// first:
//
//	expr    = term { ("+" | "-") term }
//	term    = unary { ("*" | "/" | "%") unary }
//	unary   = ("+" | "-") unary | power
//	power   = primary [ "^" unary ]
//	primary = number | "(" expr ")"
//
// so ^ is right-associative and binds tighter than a sign on its left
// (-2^2 is -4), while its exponent may carry a sign (2^-1 is 1/2). Numbers
// are decimal, with an optional fraction and exponent (12, 0.5, .5, 1.5e3).
// % is the floored remainder x - y*floor(x/y), defined for any rationals,
// whose sign follows the divisor. ^ takes integer exponents only, and 0^0 is
// 1.
//
// A syntax error is reported before any evaluation error, and every error
// except an empty expression names the 1-based column (in characters) where
// it occurred.
func Eval(expr string) (*big.Rat, error) {
	p := &parser{src: []rune(expr)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind == tokEOF {
		return nil, errEmpty
	}
	x, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokEOF {
		return nil, p.unexpected()
	}
	if p.evalErr != nil {
		return nil, p.evalErr
	}
	return x, nil
}

// Format renders x in decimal notation, never with an exponent, and reports
// whether the rendering is exactly x. A negative scale prints a terminating
// decimal exactly (integers without a decimal point, no trailing zeros) and
// rounds any other value to 20 places; scale >= 0 rounds to exactly that many
// places, padding with zeros. Rounding is to nearest, halves away from zero,
// and a value that rounds to zero prints without a sign.
func Format(x *big.Rat, scale int) (string, bool) {
	places, terminates := decimalPlaces(x)
	if scale < 0 {
		if terminates {
			return x.FloatString(places), true
		}
		scale = defaultPlaces
	}
	s := x.FloatString(scale)
	if strings.Trim(s, "-0.") == "" {
		s = strings.TrimPrefix(s, "-")
	}
	return s, terminates && places <= scale
}

// JSON renders x as a pretty-printed object: the decimal result as Format
// renders it at scale, the exact value as a fraction in lowest terms (always
// "p/q", so integers are "n/1"), and whether the result is exact. Numbers are
// strings so that no JSON consumer can lose precision.
func JSON(x *big.Rat, scale int) (string, error) {
	result, exact := Format(x, scale)
	out, err := json.MarshalIndent(struct {
		Result   string `json:"result"`
		Fraction string `json:"fraction"`
		Exact    bool   `json:"exact"`
	}{result, x.String(), exact}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding result: %w", err)
	}
	return string(out), nil
}

// decimalPlaces reports whether x has a terminating decimal expansion, which
// is when its reduced denominator is 2^a * 5^b, and if so how many places it
// needs: max(a, b).
func decimalPlaces(x *big.Rat) (int, bool) {
	d := new(big.Int).Set(x.Denom())
	twos := int(d.TrailingZeroBits())
	d.Rsh(d, uint(twos))
	fives := 0
	one, five := big.NewInt(1), big.NewInt(5)
	q, r := new(big.Int), new(big.Int)
	for d.Cmp(one) != 0 {
		q.QuoRem(d, five, r)
		if r.Sign() != 0 {
			return 0, false
		}
		d, q = q, d
		fives++
	}
	return max(twos, fives), true
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokOp // one of + - * / % ^ ( )
)

type token struct {
	kind tokenKind
	text string
	col  int
}

// parser is a recursive-descent parser that evaluates as it parses. An
// evaluation error (division by zero, an oversized result) is recorded in
// evalErr and the rest of the expression is still parsed, with nil standing
// in for unknown values, so that a later syntax error takes precedence.
type parser struct {
	src     []rune
	pos     int
	tok     token
	depth   int
	open    int // parentheses open at the current token
	evalErr error
}

func (p *parser) expr() (*big.Rat, error) {
	x, err := p.term()
	if err != nil {
		return nil, err
	}
	for p.isOp("+", "-") {
		op := p.tok
		if err := p.advance(); err != nil {
			return nil, err
		}
		y, err := p.term()
		if err != nil {
			return nil, err
		}
		x = p.binary(op, x, y)
	}
	return x, nil
}

func (p *parser) term() (*big.Rat, error) {
	x, err := p.unary()
	if err != nil {
		return nil, err
	}
	for p.isOp("*", "/", "%") {
		op := p.tok
		if err := p.advance(); err != nil {
			return nil, err
		}
		y, err := p.unary()
		if err != nil {
			return nil, err
		}
		x = p.binary(op, x, y)
	}
	return x, nil
}

// unary is the one production every recursive path passes through, so it
// enforces the nesting limit. The outermost operand is not nested, hence
// the +1.
func (p *parser) unary() (*big.Rat, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth+1 {
		return nil, fmt.Errorf("syntax error at column %d: expression nests deeper than %d levels", p.tok.col, maxDepth)
	}
	if !p.isOp("+", "-") {
		return p.power()
	}
	op := p.tok
	if err := p.advance(); err != nil {
		return nil, err
	}
	x, err := p.unary()
	if err != nil || x == nil || op.text == "+" {
		return x, err
	}
	return x.Neg(x), nil
}

func (p *parser) power() (*big.Rat, error) {
	base, err := p.primary()
	if err != nil || !p.isOp("^") {
		return base, err
	}
	op := p.tok
	if err := p.advance(); err != nil {
		return nil, err
	}
	exp, err := p.unary()
	if err != nil {
		return nil, err
	}
	return p.pow(op, base, exp), nil
}

func (p *parser) primary() (*big.Rat, error) {
	switch {
	case p.tok.kind == tokNumber:
		x := p.number(p.tok)
		return x, p.advance()
	case p.isOp("("):
		open := p.tok
		p.open++
		if err := p.advance(); err != nil {
			return nil, err
		}
		x, err := p.expr()
		if err != nil {
			return nil, err
		}
		if !p.isOp(")") {
			if p.tok.kind == tokEOF {
				return nil, fmt.Errorf("syntax error at column %d: missing \")\" to close the \"(\" at column %d", p.tok.col, open.col)
			}
			return nil, p.unexpected()
		}
		p.open--
		return x, p.advance()
	}
	return nil, p.unexpected()
}

func (p *parser) isOp(ops ...string) bool {
	if p.tok.kind != tokOp {
		return false
	}
	for _, op := range ops {
		if p.tok.text == op {
			return true
		}
	}
	return false
}

func (p *parser) unexpected() error {
	t := p.tok
	switch {
	case t.kind == tokEOF:
		return fmt.Errorf("syntax error at column %d: unexpected end of expression", t.col)
	case t.kind == tokNumber:
		return fmt.Errorf("syntax error at column %d: unexpected number %q (calc has no implicit multiplication; write the operator)", t.col, t.text)
	case t.text == "*" && t.col >= 2 && p.src[t.col-2] == '*':
		return fmt.Errorf("syntax error at column %d: unexpected \"*\" (write powers with ^, not **)", t.col)
	case t.text == ")" && p.open > 0:
		return fmt.Errorf("syntax error at column %d: unexpected \")\" where a number was expected", t.col)
	case t.text == ")":
		return fmt.Errorf("syntax error at column %d: unexpected \")\" with no \"(\" to close", t.col)
	case t.text == "(":
		return fmt.Errorf("syntax error at column %d: unexpected \"(\" (calc has no implicit multiplication; write the operator)", t.col)
	}
	return fmt.Errorf("syntax error at column %d: unexpected %q", t.col, t.text)
}

// fail records the first evaluation error; later ones are consequences of
// the expression already being unevaluable, or simply less relevant.
func (p *parser) fail(format string, args ...any) {
	if p.evalErr == nil {
		p.evalErr = fmt.Errorf(format, args...)
	}
}

// checkSize passes x through if it is within the size limit and otherwise
// records an error naming what produced it.
func (p *parser) checkSize(x *big.Rat, what string, col int) *big.Rat {
	if x.Num().BitLen() > maxBits || x.Denom().BitLen() > maxBits {
		return p.tooLarge(what, col)
	}
	return x
}

func (p *parser) tooLarge(what string, col int) *big.Rat {
	p.fail("%s at column %d exceeds the size limit: numerators and denominators are capped at %d bits (about %d decimal digits)", what, col, maxBits, maxDigits)
	return nil
}

func (p *parser) binary(op token, x, y *big.Rat) *big.Rat {
	if x == nil || y == nil {
		return nil
	}
	z := new(big.Rat)
	switch op.text {
	case "+":
		z.Add(x, y)
	case "-":
		z.Sub(x, y)
	case "*":
		z.Mul(x, y)
	case "/":
		if y.Sign() == 0 {
			p.fail("division by zero at column %d", op.col)
			return nil
		}
		z.Quo(x, y)
	case "%":
		if y.Sign() == 0 {
			p.fail("modulo by zero at column %d", op.col)
			return nil
		}
		// Rat denominators are positive, so Int.Div (Euclidean) floors.
		q := z.Quo(x, y)
		floor := new(big.Int).Div(q.Num(), q.Denom())
		z.Sub(x, new(big.Rat).Mul(y, new(big.Rat).SetInt(floor)))
	}
	return p.checkSize(z, fmt.Sprintf("result of %q", op.text), op.col)
}

func (p *parser) pow(op token, base, exp *big.Rat) *big.Rat {
	if base == nil || exp == nil {
		return nil
	}
	if !exp.IsInt() {
		p.fail("exponent %s at column %d is not an integer: calc only raises to integer powers, because fractional powers (roots) are irrational in general", exp.RatString(), op.col)
		return nil
	}
	e := exp.Num()
	num, den := base.Num(), base.Denom()
	switch {
	case base.Sign() == 0 && e.Sign() < 0:
		p.fail("division by zero at column %d: 0 raised to a negative power", op.col)
		return nil
	case base.Sign() == 0 && e.Sign() == 0:
		return big.NewRat(1, 1)
	case base.Sign() == 0:
		return new(big.Rat)
	case num.CmpAbs(den) == 0: // base is 1 or -1
		if num.Sign() < 0 && e.Bit(0) == 1 {
			return big.NewRat(-1, 1)
		}
		return big.NewRat(1, 1)
	}
	// |base| is not 0 or 1, so its numerator or denominator is at least 2
	// and grows by at least one bit per unit of exponent: bound the work
	// before doing it. The exact size is checked afterwards.
	n := new(big.Int).Abs(e)
	what := fmt.Sprintf("result of %q", op.text)
	if n.Cmp(big.NewInt(maxBits)) > 0 ||
		int64(num.BitLen()-1)*n.Int64() > maxBits || int64(den.BitLen()-1)*n.Int64() > maxBits {
		return p.tooLarge(what, op.col)
	}
	pn := new(big.Int).Exp(num, n, nil)
	pd := new(big.Int).Exp(den, n, nil)
	if e.Sign() < 0 {
		pn, pd = pd, pn
	}
	return p.checkSize(new(big.Rat).SetFrac(pn, pd), what, op.col)
}

// number converts a numeric literal exactly. Its digits are normalized to
// an integer m times 10^e before anything is computed, so a literal like
// 1e999999999 is rejected without being built.
func (p *parser) number(t token) *big.Rat {
	mantissa, expPart, hasExp := strings.Cut(strings.ToLower(t.text), "e")
	intPart, fracPart, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(intPart+fracPart, "0")
	if digits == "" {
		return new(big.Rat)
	}
	trimmed := strings.TrimRight(digits, "0")
	exp := int64(len(digits)-len(trimmed)) - int64(len(fracPart))
	digits = trimmed
	if hasExp {
		e, err := strconv.ParseInt(expPart, 10, 64)
		// Past ±2^62 no input length can bring exp back into range, and
		// the sum below cannot overflow.
		if err != nil || e > 1<<62 || e < -(1<<62) {
			return p.tooLarge("number", t.col)
		}
		exp += e
	}
	if len(digits) > maxLiteralDigits || exp > maxDigits || -exp > maxLiteralDigits {
		return p.tooLarge("number", t.col)
	}
	m, _ := new(big.Int).SetString(digits, 10)
	pow10 := new(big.Int).Exp(big.NewInt(10), big.NewInt(max(exp, -exp)), nil)
	x := new(big.Rat)
	if exp >= 0 {
		x.SetInt(m.Mul(m, pow10))
	} else {
		x.SetFrac(m, pow10)
	}
	return p.checkSize(x, "number", t.col)
}

// advance scans the next token into p.tok. Lexical errors are syntax errors
// and end parsing.
func (p *parser) advance() error {
	for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}
	start := p.pos
	col := start + 1
	if p.pos == len(p.src) {
		p.tok = token{kind: tokEOF, col: col}
		return nil
	}
	c := p.src[p.pos]
	switch {
	case isDigit(c) || c == '.':
		if err := p.scanNumber(); err != nil {
			return err
		}
		p.tok = token{kind: tokNumber, text: string(p.src[start:p.pos]), col: col}
		return nil
	case strings.ContainsRune("+-*/%^()", c):
		p.pos++
		p.tok = token{kind: tokOp, text: string(c), col: col}
		return nil
	case unicode.IsLetter(c) || c == '_':
		for p.pos < len(p.src) && (unicode.IsLetter(p.src[p.pos]) || isDigit(p.src[p.pos]) || p.src[p.pos] == '_') {
			p.pos++
		}
		return fmt.Errorf("unknown name %q at column %d: calc does exact rational arithmetic only (numbers, + - * / %% ^, and parentheses), with no functions, constants, or variables", string(p.src[start:p.pos]), col)
	}
	if ascii, ok := lookalikes[c]; ok {
		return fmt.Errorf("syntax error at column %d: unexpected character %q (U+%04X); use the ASCII %q", col, string(c), c, ascii)
	}
	if c == ',' {
		return fmt.Errorf("syntax error at column %d: unexpected \",\" (write numbers without digit grouping)", col)
	}
	return fmt.Errorf("syntax error at column %d: unexpected character %q (U+%04X)", col, string(c), c)
}

// lookalikes maps typographic operators, common in text an expression is
// copied from, to the ASCII operator calc accepts.
var lookalikes = map[rune]string{
	'\u2212': "-", // minus sign
	'\u2013': "-", // en dash
	'\u00d7': "*", // multiplication sign
	'\u00f7': "/", // division sign
}

// scanNumber consumes digits [ "." digits ] [ ("e" | "E") [sign] digits ],
// where the integer digits may be omitted when a fraction is present (.5).
func (p *parser) scanNumber() error {
	start := p.pos
	p.scanDigits()
	if p.pos < len(p.src) && p.src[p.pos] == '.' {
		p.pos++
		if p.scanDigits() == 0 {
			return fmt.Errorf("syntax error at column %d: expected a digit after the decimal point", p.pos+1)
		}
		if p.pos < len(p.src) && p.src[p.pos] == '.' {
			return fmt.Errorf("syntax error at column %d: a number has at most one decimal point", p.pos+1)
		}
	}
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
			p.pos++
		}
		if p.scanDigits() == 0 {
			return fmt.Errorf("syntax error at column %d: expected exponent digits after %q", p.pos+1, string(p.src[start:p.pos]))
		}
	}
	return nil
}

func (p *parser) scanDigits() int {
	start := p.pos
	for p.pos < len(p.src) && isDigit(p.src[p.pos]) {
		p.pos++
	}
	return p.pos - start
}

func isDigit(c rune) bool { return '0' <= c && c <= '9' }
