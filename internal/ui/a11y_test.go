package ui

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// luminance is c's relative luminance, as WCAG defines it.
func luminance(c color.Color) float64 {
	r, g, b, _ := color.NRGBAModel.Convert(c).RGBA()
	lin := func(v uint32) float64 {
		x := float64(v) / 0xFFFF
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func contrast(a, b color.Color) float64 {
	x, y := luminance(a), luminance(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// TestContrast checks text is readable on what it is shown on: WCAG AA,
// 4.5 to 1, in both variants.
func TestContrast(t *testing.T) {
	pairs := []struct{ text, on fyne.ThemeColorName }{
		{theme.ColorNameForeground, theme.ColorNameBackground},
		{theme.ColorNameForeground, colorPage},
		{theme.ColorNameForeground, theme.ColorNameButton},
		{theme.ColorNameForeground, theme.ColorNameInputBackground},
		{theme.ColorNamePlaceHolder, theme.ColorNameBackground},
		{theme.ColorNamePlaceHolder, theme.ColorNameInputBackground},
		{theme.ColorNamePrimary, theme.ColorNameBackground},
		{theme.ColorNamePrimary, colorPage},
		{theme.ColorNameForegroundOnPrimary, theme.ColorNamePrimary},
		{theme.ColorNameError, theme.ColorNameBackground},
		{theme.ColorNameError, colorPage},
		{theme.ColorNameForegroundOnError, theme.ColorNameError},
		{theme.ColorNameForegroundOnWarning, theme.ColorNameWarning},
		{theme.ColorNameHyperlink, theme.ColorNameBackground},
	}
	l := look{}
	for _, v := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, p := range pairs {
			if r := contrast(l.Color(p.text, v), l.Color(p.on, v)); r < 4.5 {
				t.Errorf("variant %d: %s on %s is %.2f to 1, under 4.5", v, p.text, p.on, r)
			}
		}
		if r := contrast(cream, l.Color(colorBar, v)); r < 4.5 {
			t.Errorf("variant %d: the bar's text is %.2f to 1, under 4.5", v, r)
		}
	}
}
