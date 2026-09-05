package main

import (
	"image/color"
	"unicode"
	"unicode/utf8"
)

type syntaxKind int

const (
	syntaxDefault syntaxKind = iota
	syntaxKeyword
	syntaxString
	syntaxComment
	syntaxNumber
	syntaxFunction
	syntaxType
	syntaxPackage
)

var syntaxColors = map[syntaxKind]color.Color{
	syntaxKeyword:  color.NRGBA{R: 0xc6, G: 0x78, B: 0xdd, A: 0xff},
	syntaxString:   color.NRGBA{R: 0x98, G: 0xc3, B: 0x79, A: 0xff},
	syntaxComment:  color.NRGBA{R: 0x5c, G: 0x63, B: 0x70, A: 0xff},
	syntaxNumber:   color.NRGBA{R: 0xd1, G: 0x9a, B: 0x66, A: 0xff},
	syntaxFunction: color.NRGBA{R: 0x61, G: 0xaf, B: 0xef, A: 0xff},
	syntaxType:     color.NRGBA{R: 0xe5, G: 0xc0, B: 0x7b, A: 0xff},
	syntaxPackage:  color.NRGBA{R: 0x56, G: 0xb6, B: 0xc2, A: 0xff},
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}

func syntaxColorAt(text string, kinds map[int]syntaxKind, pos int) color.Color {
	k, ok := kinds[pos]
	if !ok || k == syntaxDefault {
		return nil
	}
	return syntaxColors[k]
}

func markRange(kinds map[int]syntaxKind, start, end int, kind syntaxKind) {
	for i := start; i < end; i++ {
		kinds[i] = kind
	}
}

func goSyntaxHighlight(text string) map[int]syntaxKind {
	kinds := make(map[int]syntaxKind)
	knownTypes := make(map[string]bool)

	i := 0
	for i < len(text) {
		switch {
		case text[i] == '/' && i+1 < len(text) && text[i+1] == '/':
			start := i
			i += 2
			for i < len(text) && text[i] != '\n' {
				i++
			}
			markRange(kinds, start, i, syntaxComment)

		case text[i] == '/' && i+1 < len(text) && text[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
				i++
			}
			if i+1 < len(text) {
				i += 2
			}
			markRange(kinds, start, i, syntaxComment)

		case text[i] == '"':
			start := i
			end := skipGoString(text, i, '"')
			if kinds[start] != syntaxPackage {
				markRange(kinds, start, end, syntaxString)
			}
			i = end

		case text[i] == '\'':
			start := i
			i = skipGoString(text, i, '\'')
			markRange(kinds, start, i, syntaxString)

		case text[i] == '`':
			start := i
			i++
			for i < len(text) && text[i] != '`' {
				i++
			}
			if i < len(text) {
				i++
			}
			markRange(kinds, start, i, syntaxString)

		case isDigit(text[i]) || (text[i] == '.' && i+1 < len(text) && isDigit(text[i+1])):
			start := i
			i = skipNumber(text, i)
			markRange(kinds, start, i, syntaxNumber)

		case text[i] == '.':
			if ident, end := readIdent(text, i+1); ident != "" {
				markRange(kinds, i+1, end, syntaxFunction)
				i = end
			} else {
				i++
			}

		case isIdentStart(text[i]):
			start := i
			ident, end := readIdent(text, i)
			i = end

			switch {
			case goKeywords[ident]:
				markRange(kinds, start, end, syntaxKeyword)
				switch ident {
				case "func":
					if fnStart, fnEnd := readFuncName(text, end); fnStart >= 0 {
						markRange(kinds, fnStart, fnEnd, syntaxFunction)
					}
				case "type":
					if typeStart, typeEnd := readTypeName(text, end); typeStart >= 0 {
						name := text[typeStart:typeEnd]
						markRange(kinds, typeStart, typeEnd, syntaxType)
						knownTypes[name] = true
					}
				case "package":
					if pkgStart, pkgEnd := readNextIdent(text, end); pkgStart >= 0 {
						markRange(kinds, pkgStart, pkgEnd, syntaxPackage)
					}
				case "import":
					highlightImportLine(text, end, kinds)
				}

			case knownTypes[ident]:
				markRange(kinds, start, end, syntaxType)

			default:
				if pkgStart, pkgEnd := readImportAlias(text, start); pkgStart >= 0 {
					markRange(kinds, pkgStart, pkgEnd, syntaxPackage)
				}
			}

		default:
			i++
		}
	}

	return kinds
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func readIdent(text string, i int) (ident string, end int) {
	if i >= len(text) || !isIdentStart(text[i]) {
		return "", i
	}
	start := i
	i++
	for i < len(text) && isIdentPart(text[i]) {
		i++
	}
	return text[start:i], i
}

func readNextIdent(text string, i int) (start, end int) {
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '\n' {
			return -1, -1
		}
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		if !isIdentStart(text[i]) {
			return -1, -1
		}
		_, end := readIdent(text, i)
		return i, end
	}
	return -1, -1
}

func skipSpaces(text string, i int) int {
	for i < len(text) {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsSpace(r) {
			return i
		}
		i += size
	}
	return i
}

func readFuncName(text string, i int) (start, end int) {
	i = skipSpaces(text, i)
	if i >= len(text) {
		return -1, -1
	}
	if text[i] == '(' {
		depth := 0
		for i < len(text) {
			switch text[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					i++
					return readNextIdent(text, i)
				}
			}
			i++
		}
		return -1, -1
	}
	return readNextIdent(text, i)
}

func readTypeName(text string, i int) (start, end int) {
	return readNextIdent(text, i)
}

func highlightImportLine(text string, afterImport int, kinds map[int]syntaxKind) {
	i := skipSpaces(text, afterImport)
	if i >= len(text) || text[i] == '\n' {
		return
	}

	if text[i] == '(' {
		depth := 1
		i++
		for i < len(text) && depth > 0 {
			i = skipSpaces(text, i)
			if i >= len(text) {
				return
			}
			if text[i] == ')' {
				depth--
				i++
				continue
			}
			if text[i] == '(' {
				depth++
				i++
				continue
			}
			i = highlightImportSpec(text, i, kinds)
		}
		return
	}

	highlightImportSpec(text, i, kinds)
}

func highlightImportSpec(text string, i int, kinds map[int]syntaxKind) int {
	i = skipSpaces(text, i)
	if i >= len(text) {
		return i
	}

	if isIdentStart(text[i]) {
		aliasStart := i
		ident, identEnd := readIdent(text, i)
		i = skipSpaces(text, identEnd)
		if i < len(text) && text[i] == '"' {
			markRange(kinds, aliasStart, identEnd, syntaxPackage)
			start := i
			i = skipGoString(text, i, '"')
			markRange(kinds, start, i, syntaxPackage)
			return i
		}
		if ident != "" && i < len(text) && text[i] != '"' {
			return identEnd
		}
	}

	if i < len(text) && text[i] == '"' {
		start := i
		i = skipGoString(text, i, '"')
		markRange(kinds, start, i, syntaxPackage)
	}
	return i
}

func readImportAlias(text string, pos int) (start, end int) {
	lineStart := pos
	for lineStart > 0 && text[lineStart-1] != '\n' {
		lineStart--
	}
	segment := text[lineStart:pos]
	if !containsImportKeyword(segment) {
		return -1, -1
	}
	_, identEnd := readIdent(text, pos)
	i := skipSpaces(text, identEnd)
	if i >= len(text) || text[i] != '"' {
		return -1, -1
	}
	return pos, identEnd
}

func containsImportKeyword(segment string) bool {
	for i := 0; i < len(segment); {
		ident, next := readIdent(segment, i)
		if ident == "import" {
			return true
		}
		if ident == "" {
			i++
			continue
		}
		i = next
	}
	return false
}

func skipNumber(text string, i int) int {
	if i < len(text) && text[i] == '0' && i+1 < len(text) && (text[i+1] == 'x' || text[i+1] == 'X') {
		i += 2
		for i < len(text) && isHexDigit(text[i]) {
			i++
		}
		return i
	}
	for i < len(text) && (isDigit(text[i]) || text[i] == '_') {
		i++
	}
	if i < len(text) && text[i] == '.' {
		i++
		for i < len(text) && (isDigit(text[i]) || text[i] == '_') {
			i++
		}
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		i++
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			i++
		}
		for i < len(text) && (isDigit(text[i]) || text[i] == '_') {
			i++
		}
	}
	return i
}

func isHexDigit(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
