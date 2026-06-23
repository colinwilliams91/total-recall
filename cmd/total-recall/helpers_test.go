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
