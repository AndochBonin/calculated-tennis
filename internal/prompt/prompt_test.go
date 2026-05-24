package prompt

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestReadLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		label string
		in    string
		want  string
	}{
		{name: "trims_space", label: "Price: ", in: "  0.50 \n", want: "0.50"},
		{name: "plain_line", label: "Token ID: ", in: "abc123\n", want: "abc123"},
		{name: "empty_line", label: "Player name: ", in: "\n", want: ""},
		{name: "whitespace_only", label: "Player name: ", in: "   \n", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			got, err := ReadLine(&out, strings.NewReader(tt.in), tt.label)
			if err != nil {
				t.Fatalf("ReadLine: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
			if out.String() != tt.label {
				t.Fatalf("wrote %q want %q", out.String(), tt.label)
			}
		})
	}
}

func TestReadLineFrom_reuseReader(t *testing.T) {
	t.Parallel()
	br := bufio.NewReader(strings.NewReader("0.42\nabc-token\n"))
	var out bytes.Buffer
	price, err := ReadLineFrom(&out, br, "Limit price (decimal): ")
	if err != nil {
		t.Fatalf("ReadLineFrom price: %v", err)
	}
	token, err := ReadLineFrom(&out, br, "Token ID: ")
	if err != nil {
		t.Fatalf("ReadLineFrom token: %v", err)
	}
	if price != "0.42" || token != "abc-token" {
		t.Fatalf("got price=%q token=%q", price, token)
	}
}

func TestReadLine_eof(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	got, err := ReadLine(&out, strings.NewReader(""), "label: ")
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestIsInteractive_pipe(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsInteractive(r) {
		t.Fatal("pipe reader should not be interactive")
	}
}
