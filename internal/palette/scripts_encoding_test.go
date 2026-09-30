package palette

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func powerShellUTF16(text string, bigEndian bool) []byte {
	var order binary.ByteOrder = binary.LittleEndian
	bom := []byte{0xff, 0xfe}
	if bigEndian {
		order, bom = binary.BigEndian, []byte{0xfe, 0xff}
	}
	for _, code := range utf16.Encode([]rune(text)) {
		var encoded [2]byte
		order.PutUint16(encoded[:], code)
		bom = append(bom, encoded[:]...)
	}
	return bom
}

func TestPowerShellMetadataKeepsItsModeAcrossEncodings(t *testing.T) {
	header := "# @palette.title Open Tool 工具\r\n# @palette.mode pane\r\nWrite-Output 'hello'\r\n"
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"UTF-8", []byte(header)},
		{"UTF-8 BOM", append([]byte{0xef, 0xbb, 0xbf}, []byte(header)...)},
		{"UTF-16LE", powerShellUTF16(header, false)},
		{"UTF-16BE", powerShellUTF16(header, true)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tool.ps1")
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			meta, ok, err := readScript(path, "windows")
			if err != nil || !ok || meta.title != "Open Tool 工具" || meta.mode != "pane" {
				t.Fatalf("metadata = %+v, %v, %v; want the authored title and pane mode", meta, ok, err)
			}
		})
	}
}

func TestPowerShellHeaderLimitCountsEncodedBytes(t *testing.T) {
	for _, bigEndian := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "tool.ps1")
		// Short decoded text can still exceed the raw-byte read limit.
		longHeader := "# " + strings.Repeat("x", 2200) + "\n# @palette.mode pane\n"
		if err := os.WriteFile(path, powerShellUTF16(longHeader, bigEndian), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readScript(path, "windows"); err == nil {
			t.Error("a truncated UTF-16 header silently selected the default mode")
		}
		longBody := "# @palette.mode pane\nWrite-Output '" + strings.Repeat("x", 5000) + "'\n"
		if err := os.WriteFile(path, powerShellUTF16(longBody, bigEndian), 0o600); err != nil {
			t.Fatal(err)
		}
		if meta, _, err := readScript(path, "windows"); err != nil || meta.mode != "pane" {
			t.Errorf("long body changed metadata: %+v, %v", meta, err)
		}
	}
}

func TestPowerShellRejectsUnreadableHeaderEncodings(t *testing.T) {
	for _, data := range [][]byte{
		{0xff, 0xfe, '#'},
		{0xfe, 0xff, '#'},
		{0xff, 0xfe, 0, 0, '#', 0, 0, 0},
		{0, 0, 0xfe, 0xff, 0, 0, 0, '#'},
	} {
		path := filepath.Join(t.TempDir(), "tool.ps1")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readScript(path, "windows"); err == nil {
			t.Errorf("header %x silently selected the default mode", data)
		}
	}
}
