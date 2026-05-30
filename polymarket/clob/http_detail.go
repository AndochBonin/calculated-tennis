package clob

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func errUnexpectedHTTP(op string, resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return fmt.Errorf("%s: unexpected status %d", op, resp.StatusCode)
	}
	msg := strings.TrimSpace(string(body))
	if msg != "" {
		return fmt.Errorf("%s: unexpected status %d: %s", op, resp.StatusCode, msg)
	}
	return fmt.Errorf("%s: unexpected status %d", op, resp.StatusCode)
}
