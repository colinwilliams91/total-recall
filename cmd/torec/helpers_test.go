package main

import (
	"bytes"
	"io"
	"os"
)

// captureStderr redirects os.Stderr to an internal buffer and returns the buffer
// along with a restore function. Read buf only after calling restore.
func captureStderr(buf *bytes.Buffer) func() {
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	return func() {
		w.Close()
		io.Copy(buf, r)
		os.Stderr = orig
	}
}

// captureStdout redirects os.Stdout to an internal buffer and returns the buffer
// along with a restore function. Read buf only after calling restore.
func captureStdout(buf *bytes.Buffer) func() {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	return func() {
		w.Close()
		io.Copy(buf, r)
		os.Stdout = orig
	}
}
