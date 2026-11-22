package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// TestLineReaderOneLinePerRead pins the contract that makes the shared
// reader safe for huh's per-prompt scanners: a Read never returns bytes
// belonging to a line after the first newline it reaches.
func TestLineReaderOneLinePerRead(t *testing.T) {
	lr := newLineReader(strings.NewReader("y\nsk-ant-key\nmodel\n"))
	buf := make([]byte, 64)
	want := []string{"y\n", "sk-ant-key\n", "model\n"}
	for i, w := range want {
		n, err := lr.Read(buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if got := string(buf[:n]); got != w {
			t.Errorf("read %d = %q, want %q", i, got, w)
		}
	}
	if _, err := lr.Read(buf); err == nil {
		t.Error("expected EOF after the final line")
	}
}

// TestLineReaderSurvivesMultipleScanners reproduces the accessible-mode bug
// from scripts/e2e/README.md: huh creates a fresh bufio.Scanner per prompt
// over one shared stdin. With a raw pipe, the first scanner buffers every
// available byte and the rest hit EOF. With lineReader each scanner gets its
// own line.
func TestLineReaderSurvivesMultipleScanners(t *testing.T) {
	piped := strings.NewReader("yes\nenv:OPENAI_API_KEY\ngpt-4o\n")
	lr := newLineReader(piped)

	var got []string
	for i := 0; i < 3; i++ {
		scanner := bufio.NewScanner(lr) // fresh scanner, like each huh prompt
		if !scanner.Scan() {
			t.Fatalf("scanner %d hit EOF before all fields were read", i)
		}
		got = append(got, scanner.Text())
	}
	want := []string{"yes", "env:OPENAI_API_KEY", "gpt-4o"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestLineReaderLongLineSplit covers the carry path: a line longer than the
// caller's buffer is resumed across reads and reassembled intact.
func TestLineReaderLongLineSplit(t *testing.T) {
	long := strings.Repeat("x", 100) + "\nshort\n"
	lr := newLineReader(strings.NewReader(long))

	var sb strings.Builder
	buf := make([]byte, 8)
	for {
		n, err := lr.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break // EOF ends the stream (not reachable before "short\n" is read)
		}
	}
	if got := sb.String(); got != long {
		t.Errorf("reassembled %q, want %q", got, long)
	}
}

// TestLineReaderEmptyLines ensures blank input lines (the huh default
// selection case) are delivered as empty tokens, not skipped.
func TestLineReaderEmptyLines(t *testing.T) {
	lr := newLineReader(bytes.NewReader([]byte("\n\nlast\n")))
	buf := make([]byte, 16)
	want := []string{"\n", "\n", "last\n"}
	for i, w := range want {
		n, err := lr.Read(buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if got := string(buf[:n]); got != w {
			t.Errorf("read %d = %q, want %q", i, got, w)
		}
	}
}

// TestLineReaderInstancePerFormLosesInput pins why the huh stdin reader is a
// process-wide singleton rather than one per form: each instance's internal
// bufio.Reader buffers whatever stdin delivers past the current line, so a
// fresh instance per form drops bytes the previous instance already slurped
// — the same over-read class of bug the wrapper exists to fix.
func TestLineReaderInstancePerFormLosesInput(t *testing.T) {
	piped := strings.NewReader("n\nopenai\n")

	// Form 1: own instance — reads one line, buffers the rest internally.
	first := newLineReader(piped)
	s1 := bufio.NewScanner(first)
	if !s1.Scan() {
		t.Fatal("form 1 scanner hit EOF immediately")
	}
	if s1.Text() != "n" {
		t.Fatalf("form 1 read %q, want %q", s1.Text(), "n")
	}

	// Form 2: a NEW instance over the same stdin — EOF, input lost.
	second := newLineReader(piped)
	s2 := bufio.NewScanner(second)
	if s2.Scan() {
		t.Fatalf("expected form 2 with a fresh instance to hit EOF (buffered input lost), got %q", s2.Text())
	}

	// Shared instance: the buffered line survives.
	shared := newLineReader(strings.NewReader("n\nopenai\n"))
	sa := bufio.NewScanner(shared)
	sb := bufio.NewScanner(shared)
	if !sa.Scan() || sa.Text() != "n" {
		t.Fatal("shared instance: form 1 scan failed")
	}
	if !sb.Scan() || sb.Text() != "openai" {
		t.Fatal("shared instance: form 2 scan failed — buffered input lost")
	}
}

// TestAccessibleModeDetectsTermDumb guards the mirrored huh detection the
// form wrapper relies on.
func TestAccessibleModeDetectsTermDumb(t *testing.T) {
	t.Setenv("TERM", "dumb")
	if !accessibleMode() {
		t.Fatal("TERM=dumb should enable accessible mode")
	}
	t.Setenv("TERM", "xterm-256color")
	if accessibleMode() {
		t.Fatal("normal TERM should not enable accessible mode")
	}
}
