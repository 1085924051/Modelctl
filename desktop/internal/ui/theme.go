package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type modelTheme struct{ fyne.Theme }

func (t modelTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 35, G: 112, B: 82, A: 255}
	case theme.ColorNameBackground:
		return color.NRGBA{R: 247, G: 249, B: 250, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 230, G: 243, B: 237, A: 255}
	}
	return t.Theme.Color(name, variant)
}

func (t modelTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 6
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputBorder:
		return 1
	}
	return t.Theme.Size(name)
}

func newTheme() fyne.Theme { return modelTheme{Theme: theme.DefaultTheme()} }
