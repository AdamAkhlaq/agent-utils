package text

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const (
	escByte = 0x1b
	belByte = 0x07
)

// ansiState tracks where the scanner is inside the ECMA-48 escape grammar.
type ansiState int

const (
	ansiPlain     ansiState = iota
	ansiEsc                 // after ESC, kind of sequence not yet known
	ansiEscInter            // inside an ESC sequence's intermediate bytes
	ansiCSI                 // inside ESC [ ... , waiting for the final byte
	ansiString              // inside an OSC/DCS/SOS/PM/APC string payload
	ansiStringEsc           // saw ESC inside a string; a following \ is ST
)

// StripANSI copies r to w with ANSI terminal escape sequences removed: CSI
// sequences (colors including 256-color and truecolor, cursor movement,
// erase, private ?-prefixed modes), OSC strings terminated by BEL or ST
// (window titles, OSC 8 hyperlinks), the other ECMA-48 strings (DCS, SOS,
// PM, APC), and short ESC sequences such as ESC ( B or ESC =. Everything
// else, including \r, \n, \t and multi-byte UTF-8, passes through untouched,
// so input containing no escapes comes out byte-identical. All escape bytes
// are ASCII, so the byte-level scan never splits a UTF-8 rune, and the input
// is streamed in constant memory. A sequence truncated by EOF is dropped
// silently: for a filter that cleans logs, emitting a partial escape
// sequence would be worse than omitting it.
func StripANSI(w io.Writer, r io.Reader) error {
	br := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	st := ansiPlain
	for {
		b, err := br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("reading input: %w", err)
		}
		// A byte can end one state and still need handling in the next: the
		// byte that aborts a malformed sequence may be plain text or the
		// start of a new escape, hence the reprocessing loop.
		for handled := false; !handled; {
			handled = true
			switch st {
			case ansiPlain:
				if b == escByte {
					st = ansiEsc
				} else if err := bw.WriteByte(b); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
			case ansiEsc:
				switch {
				case b == '[':
					st = ansiCSI
				case b == ']' || b == 'P' || b == 'X' || b == '^' || b == '_':
					st = ansiString
				case b >= 0x20 && b <= 0x2f:
					st = ansiEscInter
				case b >= 0x30 && b <= 0x7e:
					st = ansiPlain // complete two-byte sequence, e.g. ESC =
				default:
					// Not a byte that can continue a sequence: drop the lone
					// ESC and reprocess the byte as ordinary input.
					st = ansiPlain
					handled = false
				}
			case ansiEscInter:
				switch {
				case b >= 0x20 && b <= 0x2f:
					// further intermediate bytes
				case b >= 0x30 && b <= 0x7e:
					st = ansiPlain // final byte, e.g. the B in ESC ( B
				default:
					st = ansiPlain
					handled = false
				}
			case ansiCSI:
				switch {
				case b >= 0x20 && b <= 0x3f:
					// parameter and intermediate bytes: digits ; : ? and co.
				case b >= 0x40 && b <= 0x7e:
					st = ansiPlain // final byte, e.g. m K H
				default:
					st = ansiPlain
					handled = false
				}
			case ansiString:
				switch b {
				case belByte:
					st = ansiPlain
				case escByte:
					st = ansiStringEsc
				}
			case ansiStringEsc:
				if b == '\\' {
					st = ansiPlain // ST (ESC \) terminates the string
				} else {
					// Any ESC ends the string; it also starts a new escape
					// sequence, so reprocess this byte after it.
					st = ansiEsc
					handled = false
				}
			}
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
