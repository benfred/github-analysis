package githubarchive

import (
	"bufio"
	"compress/gzip"
	"io"

	"os"
)

// Scanner scans over the contents of a githubarchive File
type Scanner struct {
	f       *os.File
	gr      *gzip.Reader
	buf     *bufio.Reader
	bytes   []byte
	lastErr error
}

// Bytes returns the current line as a byte array
func (it *Scanner) Bytes() []byte {
	return it.bytes
}

// Event returns the current Event, parsing the line from JSON
func (it *Scanner) Event() *Event {
	return ParseEvent(it.bytes)
}

// Scan for a valid entry. Returns if it found an entry (which can
// be accesssed through Event for the parsed version or Bytes
// the the raw version. On error this function returns false, and the
// error object is available on the Err member
func (it *Scanner) Scan() bool {
	bytes, err := it.buf.ReadBytes('\n')

	if err != nil {
		if err != io.EOF {
			it.lastErr = err
		}
		return false
	}
	it.bytes = bytes
	return true
}

// Err returns the last error seen in scan
func (it *Scanner) Err() error {
	return it.lastErr
}

// Close the scanner
func (it *Scanner) Close() {
	it.f.Close()
	it.gr.Close()
}

// NewScanner open filename and creates a new scanner from its contents
func NewScanner(filename string) (*Scanner, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}

	buf := bufio.NewReaderSize(gr, 20*1024*1024)
	return &Scanner{f: f, gr: gr, buf: buf}, nil
}
