package cmd

import (
	"fmt"
	"io"
)

// fail writes a formatted message to w and returns code. Error paths stay a
// single expression so the message and its exit code cannot drift apart:
//
//	return fail(stderr, mapError(err), "portlens: %v\n", err)
func fail(w io.Writer, code int, format string, a ...any) int {
	fmt.Fprintf(w, format, a...)
	return code
}
