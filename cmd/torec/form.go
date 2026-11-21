package main

import (
	"bufio"
	"io"
	"os"
	"sync"

	"github.com/charmbracelet/huh"
)

// huh forms need hardening in accessible mode: huh's accessibility prompts
// each create a fresh bufio.Scanner over the same stdin, and a Scanner reads
// greedily into its 64KB buffer. With piped input the first prompt's scanner
// slurps every buffered byte, so every later field hits EOF and silently
// falls back to its default (the bug documented in scripts/e2e/README.md).
// A real terminal is unaffected — the OS delivers one line per Enter.

// accessibleMode reports whether huh will run its forms in accessible mode.
// Mirrors huh's own detection (TERM=dumb switches NewForm to accessible);
// kept as a var so tests can stub it.
var accessibleMode = func() bool {
	return os.Getenv("TERM") == "dumb"
}

// lineReader releases exactly one input line per Read call. Lines longer
// than the caller's buffer are resumed across reads, but a read never
// returns bytes belonging to a line after the first newline it has reached —
// which is the guarantee that makes one shared reader safe for huh's
// per-prompt scanners.
type lineReader struct {
	src   *bufio.Reader
	carry []byte // unread remainder of an over-long line
}

func newLineReader(r io.Reader) *lineReader {
	return &lineReader{src: bufio.NewReader(r)}
}

func (l *lineReader) Read(p []byte) (int, error) {
	if len(l.carry) > 0 {
		n := copy(p, l.carry)
		l.carry = l.carry[n:]
		return n, nil
	}

	var line []byte
	for {
		b, err := l.src.ReadByte()
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				break // final line without a trailing newline
			}
			return 0, err
		}
		line = append(line, b)
		if b == '\n' {
			break
		}
	}

	n := copy(p, line)
	if n < len(line) {
		l.carry = append(l.carry, line[n:]...)
	}
	return n, nil
}

// The lineReader must be a process-wide singleton. Its internal bufio.Reader
// buffers whatever stdin delivers past the current line, and huh runs init
// as several separate forms; a fresh lineReader per form would drop anything
// buffered by the previous form's reader (the same over-read class of bug
// this wrapper exists to fix).
var (
	huhStdinOnce sync.Once
	huhStdin     *lineReader
)

// newHuhForm wraps huh.NewForm to apply the accessible-mode stdin hardening.
// Use it for every interactive form so scripted/piped input survives the
// multi-field init and repo prompts (see scripts/e2e/README.md).
func newHuhForm(groups ...*huh.Group) *huh.Form {
	f := huh.NewForm(groups...)
	if accessibleMode() {
		huhStdinOnce.Do(func() {
			huhStdin = newLineReader(os.Stdin)
		})
		f.WithInput(huhStdin)
	}
	return f
}
