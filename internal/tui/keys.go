package tui

import (
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

// KeyReader reads and decodes keyboard events from an underlying io.Reader,
// buffering leftover bytes across reads so fast typing, escape sequences,
// and multi-byte inputs are never lost.
type KeyReader struct {
	r   io.Reader
	buf []byte
}

// NewKeyReader initializes a new KeyReader.
func NewKeyReader(r io.Reader) *KeyReader {
	return &KeyReader{r: r}
}

// ReadKey reads and decodes the next keyboard event.
func (kr *KeyReader) ReadKey() (Key, error) {
	for {
		if len(kr.buf) == 0 {
			var tmp [64]byte
			n, err := kr.r.Read(tmp[:])
			if err != nil {
				return Key{}, err
			}
			if n == 0 {
				continue
			}
			kr.buf = append(kr.buf, tmp[:n]...)
		}

		b := kr.buf

		// Escape sequences
		if b[0] == 27 {
			if len(b) == 1 {
				kr.buf = nil
				return Key{Type: KeyEsc}, nil
			}
			if b[1] == '[' {
				if len(b) >= 3 {
					kType := KeyUnknown
					consumed := 3
					switch b[2] {
					case 'A':
						kType = KeyUp
					case 'B':
						kType = KeyDown
					case 'C':
						kType = KeyRight
					case 'D':
						kType = KeyLeft
					case 'H':
						kType = KeyHome
					case 'F':
						kType = KeyEnd
					case '1', '7':
						if len(b) >= 4 && b[3] == '~' {
							kType = KeyHome
							consumed = 4
						}
					case '4', '8':
						if len(b) >= 4 && b[3] == '~' {
							kType = KeyEnd
							consumed = 4
						}
					case '3':
						if len(b) >= 4 && b[3] == '~' {
							kType = KeyDelete
							consumed = 4
						}
					case '5':
						if len(b) >= 4 && b[3] == '~' {
							kType = KeyPgUp
							consumed = 4
						}
					case '6':
						if len(b) >= 4 && b[3] == '~' {
							kType = KeyPgDown
							consumed = 4
						}
					}
					if kType != KeyUnknown {
						kr.buf = kr.buf[consumed:]
						return Key{Type: kType}, nil
					}
				}
				kr.buf = kr.buf[2:]
				return Key{Type: KeyEsc}, nil
			} else if b[1] == 'O' { // SS3 mode
				if len(b) >= 3 {
					kType := KeyUnknown
					switch b[2] {
					case 'A':
						kType = KeyUp
					case 'B':
						kType = KeyDown
					case 'C':
						kType = KeyRight
					case 'D':
						kType = KeyLeft
					case 'H':
						kType = KeyHome
					case 'F':
						kType = KeyEnd
					}
					if kType != KeyUnknown {
						kr.buf = kr.buf[3:]
						return Key{Type: kType}, nil
					}
				}
				kr.buf = kr.buf[2:]
				return Key{Type: KeyEsc}, nil
			}
			kr.buf = kr.buf[1:]
			return Key{Type: KeyEsc}, nil
		}

		// Single control characters
		switch b[0] {
		case 3: // Ctrl+C
			kr.buf = kr.buf[1:]
			return Key{Type: KeyCtrlC}, nil
		case 9: // Tab
			kr.buf = kr.buf[1:]
			return Key{Type: KeyTab}, nil
		case 10, 13: // LF or CR
			kr.buf = kr.buf[1:]
			return Key{Type: KeyEnter}, nil
		case 127, 8: // Backspace
			kr.buf = kr.buf[1:]
			return Key{Type: KeyBackspace}, nil
		}

		// Normal UTF-8 rune decoding
		rChar, size := utf8.DecodeRune(b)
		if rChar == utf8.RuneError {
			kr.buf = kr.buf[1:]
			return Key{Type: KeyUnknown}, nil
		}
		kr.buf = kr.buf[size:]
		return Key{Type: KeyRune, Rune: rChar}, nil
	}
}

// readKeyEvent reads bytes from r and decodes the next keyboard event.
func readKeyEvent(r io.Reader) (Key, error) {
	return NewKeyReader(r).ReadKey()
}
