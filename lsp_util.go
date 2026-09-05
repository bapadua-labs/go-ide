package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

type sourceLocation struct {
	Path string
	Row  int
	Col  int
}

type fileDiagnostic struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
	Message  string
	Severity protocol.DiagnosticSeverity
}

func fileDiagnosticsFromLSP(items []protocol.Diagnostic) []fileDiagnostic {
	out := make([]fileDiagnostic, 0, len(items))
	for _, d := range items {
		out = append(out, fileDiagnostic{
			StartRow: int(d.Range.Start.Line),
			StartCol: int(d.Range.Start.Character),
			EndRow:   int(d.Range.End.Line),
			EndCol:   int(d.Range.End.Character),
			Message:  diagnosticMessageText(d.Message),
			Severity: d.Severity,
		})
	}
	return out
}

func diagnosticMessageText(msg protocol.InlayHintTooltip) string {
	if msg == nil {
		return ""
	}
	switch v := msg.(type) {
	case protocol.String:
		return string(v)
	case *protocol.MarkupContent:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(v.Value)
	default:
		return ""
	}
}

func uriPath(u uri.URI) string {
	return normalizePath(u.FsPath())
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	return normalizePath(a) == normalizePath(b)
}

func snapToIdentifier(text string, row, col int) (int, int, string) {
	_, _, word := identifierAt(text, row, col)
	if word != "" {
		return row, col, word
	}
	if col > 0 {
		_, _, word = identifierAt(text, row, col-1)
		if word != "" {
			return row, col - 1, word
		}
	}
	return row, col, ""
}

func locationsFromDefinition(result protocol.DefinitionResult) []sourceLocation {
	if result == nil {
		return nil
	}
	switch v := result.(type) {
	case *protocol.Location:
		if v == nil {
			return nil
		}
		return []sourceLocation{locationFromLSP(*v)}
	case protocol.LocationSlice:
		return locationsFromSlice(v)
	case protocol.DefinitionLinkSlice:
		out := make([]sourceLocation, 0, len(v))
		for _, link := range v {
			out = append(out, sourceLocation{
				Path: uriPath(link.TargetURI),
				Row:  int(link.TargetRange.Start.Line),
				Col:  columnFromPosition(uriPath(link.TargetURI), link.TargetRange.Start),
			})
		}
		return out
	default:
		return nil
	}
}

func locationsFromReferences(items []protocol.Location) []sourceLocation {
	return locationsFromSlice(items)
}

func locationsFromSlice(items []protocol.Location) []sourceLocation {
	out := make([]sourceLocation, 0, len(items))
	for _, loc := range items {
		out = append(out, locationFromLSP(loc))
	}
	return out
}

func locationFromLSP(loc protocol.Location) sourceLocation {
	path := uriPath(loc.URI)
	return sourceLocation{
		Path: path,
		Row:  int(loc.Range.Start.Line),
		Col:  columnFromPosition(path, loc.Range.Start),
	}
}

func columnFromPosition(path string, pos protocol.Position) int {
	text := readFileOrEmpty(path)
	lines := strings.Split(text, "\n")
	line := ""
	if int(pos.Line) >= 0 && int(pos.Line) < len(lines) {
		line = lines[int(pos.Line)]
	}
	return utf16ColumnToByte(line, int(pos.Character))
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func hoverContentsText(contents protocol.HoverContents) string {
	if contents == nil {
		return ""
	}
	var raw string
	switch v := contents.(type) {
	case *protocol.MarkupContent:
		if v == nil {
			return ""
		}
		raw = v.Value
	case protocol.String:
		raw = string(v)
	case *protocol.MarkedStringWithLanguage:
		if v == nil {
			return ""
		}
		raw = v.Value
	case protocol.MarkedStringSlice:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			switch s := item.(type) {
			case protocol.String:
				parts = append(parts, string(s))
			case *protocol.MarkedStringWithLanguage:
				if s != nil {
					parts = append(parts, s.Value)
				}
			}
		}
		raw = strings.Join(parts, "\n\n")
	default:
		return ""
	}
	return plainTextFromMarkdown(raw)
}

func signatureHelpText(help *protocol.SignatureHelp) string {
	if help == nil || len(help.Signatures) == 0 {
		return ""
	}
	idx := 0
	if help.ActiveSignature != nil {
		idx = int(*help.ActiveSignature)
	}
	if idx < 0 || idx >= len(help.Signatures) {
		idx = 0
	}
	sig := help.Signatures[idx]
	label := sig.Label
	if len(sig.Parameters) > 0 {
		active := 0
		if ap, ok := help.ActiveParameter.Get(); ok {
			active = int(ap)
		}
		if active >= 0 && active < len(sig.Parameters) {
			label += "\n\n> " + parameterLabel(sig.Parameters[active])
		}
	}
	if sig.Documentation != nil {
		if doc, ok := sig.Documentation.(*protocol.MarkupContent); ok && doc != nil {
			label += "\n\n" + plainTextFromMarkdown(doc.Value)
		}
	}
	return strings.TrimSpace(label)
}

var (
	inlineCodeRE     = regexp.MustCompile("`([^`]*)`")
	inlineBoldRE     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	inlineItalicRE   = regexp.MustCompile(`\*([^*]+)\*`)
	markdownLinkRE   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	snippetPlaceholderRE = regexp.MustCompile(`\$\{\d+:([^}]*)\}`)
	snippetIndexRE       = regexp.MustCompile(`\$\{\d+\}`)
)

func plainTextFromMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	inCode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			line = strings.TrimLeft(trimmed, "# ")
		}
		if inCode {
			out = append(out, line)
			continue
		}
		out = append(out, stripInlineMarkdown(line))
	}
	return collapseBlankLines(strings.TrimSpace(strings.Join(out, "\n")))
}

func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, line)
		prevBlank = blank
	}
	return strings.Join(out, "\n")
}

func stripInlineMarkdown(line string) string {
	line = markdownLinkRE.ReplaceAllString(line, "$1")
	line = inlineCodeRE.ReplaceAllString(line, "$1")
	line = inlineBoldRE.ReplaceAllString(line, "$1")
	line = inlineItalicRE.ReplaceAllString(line, "$1")
	return line
}

func wrapPlainText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return text
	}
	var b strings.Builder
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		lineLen := 0
		for _, w := range words {
			wl := utf8.RuneCountInString(w)
			if lineLen > 0 && lineLen+1+wl > maxRunes {
				b.WriteByte('\n')
				lineLen = 0
			}
			if lineLen > 0 {
				b.WriteByte(' ')
				lineLen++
			}
			b.WriteString(w)
			lineLen += wl
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func expandSnippet(text string) string {
	text = snippetPlaceholderRE.ReplaceAllString(text, "$1")
	text = snippetIndexRE.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "$0", "")
	return text
}

func parameterLabel(p protocol.ParameterInformation) string {
	switch v := p.Label.(type) {
	case protocol.String:
		return string(v)
	default:
		return fmt.Sprintf("%v", p.Label)
	}
}

func applyWorkspaceEdit(edit *protocol.WorkspaceEdit) ([]string, error) {
	if edit == nil {
		return nil, nil
	}

	byPath := map[string][]protocol.TextEdit{}

	for u, edits := range edit.Changes {
		byPath[uriPath(u)] = append(byPath[uriPath(u)], edits...)
	}
	for _, change := range edit.DocumentChanges {
		switch v := change.(type) {
		case *protocol.TextDocumentEdit:
			if v == nil {
				continue
			}
			path := uriPath(v.TextDocument.URI)
			for _, el := range v.Edits {
				switch e := el.(type) {
				case *protocol.TextEdit:
					if e != nil {
						byPath[path] = append(byPath[path], *e)
					}
				case *protocol.AnnotatedTextEdit:
					if e != nil {
						byPath[path] = append(byPath[path], e.TextEdit)
					}
				}
			}
		}
	}

	changed := make([]string, 0, len(byPath))
	for path, edits := range byPath {
		if err := applyTextEdits(path, edits); err != nil {
			return changed, err
		}
		changed = append(changed, path)
	}
	sort.Strings(changed)
	return changed, nil
}

func applyTextEdits(path string, edits []protocol.TextEdit) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)

	sort.Slice(edits, func(i, j int) bool {
		a := positionToByteOffset(text, edits[i].Range.Start)
		b := positionToByteOffset(text, edits[j].Range.Start)
		return a > b
	})

	for _, edit := range edits {
		start := positionToByteOffset(text, edit.Range.Start)
		end := positionToByteOffset(text, edit.Range.End)
		if start < 0 {
			start = 0
		}
		if end > len(text) {
			end = len(text)
		}
		if start > end {
			start, end = end, start
		}
		text = text[:start] + edit.NewText + text[end:]
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func identifierAt(text string, row, byteCol int) (start, end int, word string) {
	lines := strings.Split(text, "\n")
	if row < 0 || row >= len(lines) {
		return 0, 0, ""
	}
	line := lines[row]
	if byteCol > len(line) {
		byteCol = len(line)
	}

	start = byteCol
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:start])
		if !isIdentRuneRune(r) {
			break
		}
		start -= size
	}
	end = byteCol
	for end < len(line) {
		r, size := utf8.DecodeRuneInString(line[end:])
		if !isIdentRuneRune(r) {
			break
		}
		end += size
	}
	if start >= end {
		return 0, 0, ""
	}
	return start, end, line[start:end]
}

func isIdentRuneRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func diagnosticByteRange(text string, d fileDiagnostic) (start, end int) {
	lines := strings.Split(text, "\n")
	offset := 0
	for row, line := range lines {
		lineLen := len(line)
		if row < d.StartRow {
			offset += lineLen + 1
			continue
		}
		if row > d.EndRow {
			break
		}
		lineStart := offset
		lineEnd := offset + lineLen
		if row == d.StartRow && row == d.EndRow {
			s := lineStart + utf16ColumnToByte(line, d.StartCol)
			e := lineStart + utf16ColumnToByte(line, d.EndCol)
			return s, e
		}
		if row == d.StartRow {
			return lineStart + utf16ColumnToByte(line, d.StartCol), lineEnd
		}
		if row == d.EndRow {
			return lineStart, lineStart + utf16ColumnToByte(line, d.EndCol)
		}
		return lineStart, lineEnd
	}
	return 0, 0
}

func diagnosticInRange(d fileDiagnostic, row, byteCol int, text string) bool {
	if row < d.StartRow || row > d.EndRow {
		return false
	}
	lines := strings.Split(text, "\n")
	line := ""
	if row >= 0 && row < len(lines) {
		line = lines[row]
	}
	start := utf16ColumnToByte(line, d.StartCol)
	end := utf16ColumnToByte(line, d.EndCol)
	if row != d.StartRow {
		start = 0
	}
	if row != d.EndRow {
		end = len(line)
	}
	return byteCol >= start && byteCol < end
}

func locationLabel(loc sourceLocation) string {
	return fmt.Sprintf("%s:%d:%d", filepath.Base(loc.Path), loc.Row+1, loc.Col+1)
}
