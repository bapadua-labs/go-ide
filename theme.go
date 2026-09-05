package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Paleta de cores para rainbow brackets (estilo VS Code).
var bracketPalette = []color.Color{
	color.NRGBA{R: 0xff, G: 0xd7, B: 0x00, A: 0xff}, // dourado
	color.NRGBA{R: 0xda, G: 0x70, B: 0xd6, A: 0xff}, // orquídea
	color.NRGBA{R: 0x4e, G: 0xc9, B: 0xb0, A: 0xff}, // ciano
	color.NRGBA{R: 0x56, G: 0x9c, B: 0xd6, A: 0xff}, // azul
	color.NRGBA{R: 0xce, G: 0x91, B: 0x78, A: 0xff}, // laranja
	color.NRGBA{R: 0xb5, G: 0xce, B: 0xa8, A: 0xff}, // verde
}

var bracketMismatchColor = color.NRGBA{R: 0xf4, G: 0x47, B: 0x47, A: 0xff}

// rainbowTheme é um tema escuro inspirado em editores com rainbow brackets.
type rainbowTheme struct {
	fallback fyne.Theme
}

func newRainbowTheme() fyne.Theme {
	return &rainbowTheme{fallback: theme.DefaultTheme()}
}

func (t *rainbowTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	_ = variant
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x28, G: 0x2c, B: 0x34, A: 0xff}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x3e, G: 0x44, B: 0x51, A: 0xff}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x5c, G: 0x63, B: 0x6e, A: 0xff}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x35, G: 0x3b, B: 0x45, A: 0xff}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xe0, G: 0x6c, B: 0x75, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xab, G: 0xb2, B: 0xbf, A: 0xff}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x3e, G: 0x44, B: 0x51, A: 0xff}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0x21, G: 0x25, B: 0x2b, A: 0xff}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x21, G: 0x25, B: 0x2b, A: 0xff}
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 0x18, G: 0x1a, B: 0x1f, A: 0xff}
	case theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x21, G: 0x25, B: 0x2b, A: 0xff}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x28, G: 0x2c, B: 0x34, A: 0xf0}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x5c, G: 0x63, B: 0x6e, A: 0xff}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x61, G: 0xaf, B: 0xef, A: 0xff}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x4b, G: 0x52, B: 0x60, A: 0xff}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0x21, G: 0x25, B: 0x2b, A: 0xff}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x3e, G: 0x44, B: 0x51, A: 0xff}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x18, G: 0x1a, B: 0x1f, A: 0xff}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x56, G: 0x9c, B: 0xd6, A: 0xff}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x61, G: 0xaf, B: 0xef, A: 0x66}
	}
	return t.fallback.Color(name, variant)
}

func (t *rainbowTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.fallback.Font(style)
}

func (t *rainbowTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.fallback.Icon(name)
}

func (t *rainbowTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.fallback.Size(name)
}
