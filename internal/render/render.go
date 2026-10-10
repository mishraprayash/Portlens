// Package render converts model structures into human-readable terminal output
// and machine-readable JSON. It handles styling, alignment, and the graceful
// fallback to plain text when output is not a terminal.
package render

import (
	"io"
	"os"

	"golang.org/x/term"
)

// DefaultWidth is used when the terminal width cannot be determined.
const DefaultWidth = 100

// Renderer writes styled output to an io.Writer.
type Renderer struct {
	W     io.Writer
	Color bool
	Width int
}

// Option configures a Renderer.
type Option func(*Renderer)

// WithColor enables or disables ANSI color escape codes.
func WithColor(color bool) Option {
	return func(r *Renderer) {
		r.Color = color
	}
}

// WithWidth sets the line width for horizontal rules and tables.
func WithWidth(width int) Option {
	return func(r *Renderer) {
		if width > 0 {
			r.Width = width
		}
	}
}

// NewRenderer builds a Renderer using functional options.
func NewRenderer(w io.Writer, opts ...Option) *Renderer {
	r := &Renderer{W: w, Width: DefaultWidth}
	isTerm := false
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		isTerm = true
		r.Color = true
		if width, _, err := term.GetSize(int(f.Fd())); err == nil && width > 0 {
			r.Width = width
		}
	}
	for _, opt := range opts {
		opt(r)
	}
	// Piped or non-terminal output never carries color escapes unless w is a terminal.
	if !isTerm {
		r.Color = false
	}
	return r
}

// New builds a Renderer with color flag (retained for backward compatibility).
func New(w io.Writer, color bool) *Renderer {
	return NewRenderer(w, WithColor(color))
}

// IsInteractive reports whether the output writer is a terminal.
func (r *Renderer) IsInteractive() bool {
	f, ok := r.W.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func (r *Renderer) write(s string) {
	_, _ = io.WriteString(r.W, s)
}

func (r *Renderer) writeln(s string) {
	r.write(s + "\n")
}

// Styles return ANSI-wrapped strings when color is enabled.
func (r *Renderer) bold(s string) string    { return r.wrap("1", s) }
func (r *Renderer) dim(s string) string     { return r.wrap("2", s) }
func (r *Renderer) red(s string) string     { return r.wrap("31", s) }
func (r *Renderer) green(s string) string   { return r.wrap("32", s) }
func (r *Renderer) yellow(s string) string  { return r.wrap("33", s) }
func (r *Renderer) blue(s string) string    { return r.wrap("34", s) }
func (r *Renderer) magenta(s string) string { return r.wrap("35", s) }
func (r *Renderer) cyan(s string) string    { return r.wrap("36", s) }

func (r *Renderer) wrap(code, s string) string {
	if !r.Color || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// hr renders a horizontal rule across the renderer width.
func (r *Renderer) hr() string {
	w := r.Width
	if w > 120 {
		w = 120
	}
	line := make([]rune, w)
	for i := range line {
		line[i] = '─'
	}
	return r.dim(string(line))
}

// section prints a section header followed by the given body lines.
func (r *Renderer) section(title string, body func()) {
	r.writeln(r.bold(title))
	r.writeln(r.hr())
	body()
	r.writeln("")
}

// kv renders aligned key/value rows. Labels are padded to the longest label.
func (r *Renderer) kv(rows [][2]string) string {
	pad := 0
	for _, row := range rows {
		if len(row[0]) > pad {
			pad = len(row[0])
		}
	}
	pad += 3
	out := ""
	for _, row := range rows {
		out += r.dim(row[0]) + spaces(pad-len(row[0])) + row[1] + "\n"
	}
	return out
}

func spaces(n int) string {
	if n <= 0 {
		return " "
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
