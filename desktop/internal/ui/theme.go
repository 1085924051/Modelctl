package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type modelTheme struct{ fyne.Theme }

func (t modelTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 38, G: 96, B: 72, A: 255}
	}
	return t.Theme.Color(name, variant)
}

func newTheme() fyne.Theme { return modelTheme{Theme: theme.DefaultTheme()} }
