package main

import (
	"image/color"
)

var bracketOpen = map[byte]bool{'(': true, '[': true, '{': true}

var bracketClose = map[byte]byte{
	')': '(',
	']': '[',
	'}': '{',
}

// bracketColorAt retorna a cor do bracket na posição byte, ou nil se não for bracket colorido.
func bracketColorAt(text string, colors map[int]int, pos int) color.Color {
	idx, ok := colors[pos]
	if !ok {
		return nil
	}
	if idx < 0 {
		return bracketMismatchColor
	}
	return bracketPalette[idx%len(bracketPalette)]
}

// bracketColors calcula o índice de cor para cada bracket no texto.
// Retorna -1 para brackets sem par correspondente.
func bracketColors(text string) map[int]int {
	colors := make(map[int]int)
	depth := 0
	stack := make([]int, 0, 32)

	i := 0
	for i < len(text) {
		switch {
		case text[i] == '/' && i+1 < len(text) && text[i+1] == '/':
			i += 2
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case text[i] == '/' && i+1 < len(text) && text[i+1] == '*':
			i += 2
			for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
				i++
			}
			if i+1 < len(text) {
				i += 2
			}
		case text[i] == '"':
			i = skipGoString(text, i, '"')
		case text[i] == '\'':
			i = skipGoString(text, i, '\'')
		case text[i] == '`':
			i++
			for i < len(text) && text[i] != '`' {
				i++
			}
			if i < len(text) {
				i++
			}
		case bracketOpen[text[i]]:
			colors[i] = depth % len(bracketPalette)
			stack = append(stack, i)
			depth++
			i++
		case bracketClose[text[i]] != 0:
			open := bracketClose[text[i]]
			if len(stack) > 0 && text[stack[len(stack)-1]] == open {
				stack = stack[:len(stack)-1]
				depth--
				colors[i] = depth % len(bracketPalette)
			} else {
				colors[i] = -1
			}
			i++
		default:
			i++
		}
	}

	for _, pos := range stack {
		colors[pos] = -1
	}
	return colors
}

func skipGoString(text string, i int, quote byte) int {
	i++
	for i < len(text) {
		if text[i] == '\\' && i+1 < len(text) {
			i += 2
			continue
		}
		if text[i] == quote {
			return i + 1
		}
		i++
	}
	return i
}
