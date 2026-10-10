package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"

	"github.com/portlens/portlens/internal/actions"
)

func isTerminalFd(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

func isTerminalReader(r io.Reader) bool {
	f, ok := r.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(f.Fd()))
}

// newConfirm returns a confirmation function. When the input is not a terminal,
// confirmations are refused unless explicitly bypassed with yes, in which case
// they always succeed. This ensures destructive actions never silently proceed
// in scripts.
func newConfirm(stdout io.Writer, stdin io.Reader, yes bool) actions.ConfirmFunc {
	return func(prompt string) (bool, error) {
		if yes {
			return true, nil
		}
		reader, ok := stdin.(interface {
			Fd() uintptr
		})
		if !ok || !term.IsTerminal(int(reader.Fd())) {
			return false, fmt.Errorf("confirmation required but input is not a terminal (use --yes or --force)")
		}
		fmt.Fprint(stdout, prompt)
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, nil
		default:
			return false, nil
		}
	}
}
