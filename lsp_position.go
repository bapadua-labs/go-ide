package main

import (
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"
)

func byteOffsetToPosition(text string, row, byteCol int) protocol.Position {
	lines := strings.Split(text, "\n")
	line := ""
	if row >= 0 && row < len(lines) {
		line = lines[row]
	}
	return protocol.Position{
		Line:      uint32(row),
		Character: uint32(utf16Column(line, byteCol)),
	}
}

func positionToByteOffset(text string, pos protocol.Position) int {
	lines := strings.Split(text, "\n")
	row := int(pos.Line)
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	byteCol := utf16ColumnToByte(lines[row], int(pos.Character))
	offset := 0
	for i := 0; i < row; i++ {
		offset += len(lines[i]) + 1
	}
	return offset + byteCol
}

func utf16Column(line string, byteCol int) int {
	if byteCol < 0 {
		return 0
	}
	if byteCol > len(line) {
		byteCol = len(line)
	}
	col := 0
	for i := 0; i < byteCol; {
		r, size := utf8.DecodeRuneInString(line[i:])
		if r == 0 {
			break
		}
		if r > 0xFFFF {
			col += 2
		} else {
			col++
		}
		i += size
	}
	return col
}

func utf16ColumnToByte(line string, utf16Col int) int {
	if utf16Col <= 0 {
		return 0
	}
	col := 0
	for i := 0; i < len(line); {
		if col >= utf16Col {
			return i
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		if r > 0xFFFF {
			col += 2
		} else {
			col++
		}
		i += size
	}
	return len(line)
}
