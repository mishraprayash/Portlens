package tui

import (
	"bytes"
	"testing"
)

func TestReadKeyEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantType KeyType
		wantRune rune
	}{
		{"Ctrl+C", []byte{3}, KeyCtrlC, 0},
		{"Tab", []byte{9}, KeyTab, 0},
		{"LF", []byte{10}, KeyEnter, 0},
		{"CR", []byte{13}, KeyEnter, 0},
		{"Esc", []byte{27}, KeyEsc, 0},
		{"Backspace 127", []byte{127}, KeyBackspace, 0},
		{"Backspace 8", []byte{8}, KeyBackspace, 0},
		{"Up Arrow CSI", []byte{27, '[', 'A'}, KeyUp, 0},
		{"Down Arrow CSI", []byte{27, '[', 'B'}, KeyDown, 0},
		{"Right Arrow CSI", []byte{27, '[', 'C'}, KeyRight, 0},
		{"Left Arrow CSI", []byte{27, '[', 'D'}, KeyLeft, 0},
		{"Home CSI", []byte{27, '[', 'H'}, KeyHome, 0},
		{"End CSI", []byte{27, '[', 'F'}, KeyEnd, 0},
		{"Page Up CSI", []byte{27, '[', '5', '~'}, KeyPgUp, 0},
		{"Page Down CSI", []byte{27, '[', '6', '~'}, KeyPgDown, 0},
		{"Delete CSI", []byte{27, '[', '3', '~'}, KeyDelete, 0},
		{"Up Arrow SS3", []byte{27, 'O', 'A'}, KeyUp, 0},
		{"Down Arrow SS3", []byte{27, 'O', 'B'}, KeyDown, 0},
		{"Rune 'a'", []byte{'a'}, KeyRune, 'a'},
		{"Rune 'Z'", []byte{'Z'}, KeyRune, 'Z'},
		{"Rune '?'", []byte{'?'}, KeyRune, '?'},
		{"Rune '/'", []byte{'/'}, KeyRune, '/'},
		{"UTF-8 Rune '⌘'", []byte("⌘"), KeyRune, '⌘'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			k, err := readKeyEvent(r)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k.Type != tt.wantType {
				t.Errorf("got type %v, want %v", k.Type, tt.wantType)
			}
			if k.Rune != tt.wantRune {
				t.Errorf("got rune %c, want %c", k.Rune, tt.wantRune)
			}
		})
	}
}
