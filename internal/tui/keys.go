package tui

import (
	"errors"
	"io"
	"unicode/utf8"
)

// KeyType identifies a terminal key press event.
type KeyType int

const (
	KeyUnknown KeyType = iota
	KeyRune
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPgUp
	KeyPgDown
	KeyHome
	KeyEnd
	KeyEnter
	KeyEsc
	KeyBackspace
	KeyDelete
	KeyTab
	KeyCtrlC
)

// Key represents a decoded keyboard event.
type Key struct {
	Type KeyType
	Rune rune
}

// ReadKeyEvent reads bytes from r and decodes the next keyboard event.
func ReadKeyEvent(r io.Reader) (Key, error) {
	var buf [64]byte
	n, err := r.Read(buf[:])
	if err != nil {
		return Key{}, err
	}
	if n == 0 {
		return Key{}, errors.New("empty read")
	}

	b := buf[:n]

	// Handle Control characters
	if len(b) == 1 {
		switch b[0] {
		case 3: // Ctrl+C
			return Key{Type: KeyCtrlC}, nil
		case 9: // Tab
			return Key{Type: KeyTab}, nil
		case 10, 13: // LF or CR
			return Key{Type: KeyEnter}, nil
		case 27: // Escape
			return Key{Type: KeyEsc}, nil
		case 127, 8: // Backspace
			return Key{Type: KeyBackspace}, nil
		}
	}

	// Escape sequences
	if b[0] == 27 {
		if len(b) == 1 {
			return Key{Type: KeyEsc}, nil
		}
		if b[1] == '[' {
			if len(b) >= 3 {
				switch b[2] {
				case 'A':
					return Key{Type: KeyUp}, nil
				case 'B':
					return Key{Type: KeyDown}, nil
				case 'C':
					return Key{Type: KeyRight}, nil
				case 'D':
					return Key{Type: KeyLeft}, nil
				case 'H':
					return Key{Type: KeyHome}, nil
				case 'F':
					return Key{Type: KeyEnd}, nil
				case '1', '7':
					if len(b) >= 4 && b[3] == '~' {
						return Key{Type: KeyHome}, nil
					}
				case '4', '8':
					if len(b) >= 4 && b[3] == '~' {
						return Key{Type: KeyEnd}, nil
					}
				case '3':
					if len(b) >= 4 && b[3] == '~' {
						return Key{Type: KeyDelete}, nil
					}
				case '5':
					if len(b) >= 4 && b[3] == '~' {
						return Key{Type: KeyPgUp}, nil
					}
				case '6':
					if len(b) >= 4 && b[3] == '~' {
						return Key{Type: KeyPgDown}, nil
					}
				}
			}
			return Key{Type: KeyUnknown}, nil
		} else if b[1] == 'O' { // SS3 application cursor mode
			if len(b) >= 3 {
				switch b[2] {
				case 'A':
					return Key{Type: KeyUp}, nil
				case 'B':
					return Key{Type: KeyDown}, nil
				case 'C':
					return Key{Type: KeyRight}, nil
				case 'D':
					return Key{Type: KeyLeft}, nil
				case 'H':
					return Key{Type: KeyHome}, nil
				case 'F':
					return Key{Type: KeyEnd}, nil
				}
			}
			return Key{Type: KeyUnknown}, nil
		}
		// If unrecognized escape sequence, treat as escape
		return Key{Type: KeyEsc}, nil
	}

	// Normal UTF-8 rune decoding
	rChar, _ := utf8.DecodeRune(b)
	if rChar == utf8.RuneError {
		return Key{Type: KeyUnknown}, nil
	}
	return Key{Type: KeyRune, Rune: rChar}, nil
}
