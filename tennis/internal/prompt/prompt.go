package prompt

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// IsInteractive reports whether stdin is an interactive terminal (character device).
func IsInteractive(stdin *os.File) bool {
	fi, err := stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ReadLine writes label to w, reads one line from r, and returns it with surrounding space trimmed.
func ReadLine(w io.Writer, r io.Reader, label string) (string, error) {
	return ReadLineFrom(w, bufio.NewReader(r), label)
}

// ReadLineFrom reads one line using br. Reuse the same bufio.Reader when prompting multiple times on one stream.
func ReadLineFrom(w io.Writer, br *bufio.Reader, label string) (string, error) {
	if _, err := io.WriteString(w, label); err != nil {
		return "", err
	}
	line, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
