package main

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go.lsp.dev/protocol"
)

const maxPopupWidth = 480

type lspTextPopup struct {
	layer      *fyne.Container
	background *canvas.Rectangle
	label      *widget.Label
}

func newLSPTextPopup(layer *fyne.Container) *lspTextPopup {
	th := fyne.CurrentApp().Settings().Theme()
	bg := canvas.NewRectangle(th.Color(theme.ColorNameOverlayBackground, theme.VariantDark))
	bg.CornerRadius = th.Size(theme.SizeNamePopupRadius)
	lbl := widget.NewLabel("")
	lbl.Wrapping = fyne.TextWrapWord
	lbl.TextStyle = fyne.TextStyle{Monospace: true}
	pop := &lspTextPopup{
		layer:      layer,
		background: bg,
		label:      lbl,
	}
	layer.Objects = []fyne.CanvasObject{bg, lbl}
	layer.Hide()
	return pop
}

func (p *lspTextPopup) ShowAt(pos fyne.Position, content string) {
	content = plainTextFromMarkdown(strings.TrimSpace(content))
	if content == "" {
		p.Hide()
		return
	}
	content = wrapPlainText(content, 64)
	p.label.SetText(content)

	const padX, padY float32 = 8, 6
	textWidth := float32(maxPopupWidth - 16)
	p.label.Resize(fyne.NewSize(textWidth, 1))
	labelSize := p.label.MinSize()
	popSize := fyne.NewSize(labelSize.Width+2*padX, labelSize.Height+2*padY)

	p.background.Resize(popSize)
	p.background.Move(pos)
	p.label.Resize(labelSize)
	p.label.Move(pos.Add(fyne.NewPos(padX, padY)))
	p.layer.Show()
	p.layer.Refresh()
}

func (p *lspTextPopup) Hide() {
	if p.layer != nil {
		p.layer.Hide()
	}
}

func diagnosticStyle(severity protocol.DiagnosticSeverity, base color.Color) *widget.CustomTextGridStyle {
	style := &widget.CustomTextGridStyle{FGColor: base}
	underlineColor := color.NRGBA{R: 0xe0, G: 0x6c, B: 0x75, A: 0xff}
	switch severity {
	case protocol.DiagnosticSeverityWarning, protocol.DiagnosticSeverityInformation:
		underlineColor = color.NRGBA{R: 0xe5, G: 0xc0, B: 0x7b, A: 0xff}
	case protocol.DiagnosticSeverityHint:
		underlineColor = color.NRGBA{R: 0x56, G: 0xb6, B: 0xc2, A: 0xff}
	}
	style.TextStyle = fyne.TextStyle{Underline: true}
	style.FGColor = underlineColor
	return style
}

func (ed *codeEditor) diagnosticAt(row, byteCol int) *fileDiagnostic {
	for i := range ed.diagnostics {
		d := &ed.diagnostics[i]
		if diagnosticInRange(*d, row, byteCol, ed.text) {
			return d
		}
	}
	return nil
}

func diagnosticForBytePos(text string, diags []fileDiagnostic, bytePos int) *fileDiagnostic {
	for i := range diags {
		start, end := diagnosticByteRange(text, diags[i])
		if bytePos >= start && bytePos < end {
			return &diags[i]
		}
	}
	return nil
}
