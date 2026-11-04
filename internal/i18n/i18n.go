// Package i18n puts the app's words in the user's language. The English
// text is the key: translations/<language>.json maps it to the language's
// words, and text without a translation shows in English.
package i18n

import (
	"embed"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
)

//go:embed translations
var translations embed.FS

func init() {
	if err := lang.AddTranslationsFS(translations, "translations"); err != nil {
		fyne.LogError("loading the translations", err)
	}
}

// T is s in the user's language; with args, s is a format for them, as
// fmt.Sprintf's, and so is its translation.
func T(s string, args ...any) string {
	t := lang.L(s)
	if len(args) == 0 {
		return t
	}
	return fmt.Sprintf(t, args...)
}

// N is one when n is 1 and other otherwise, in the user's language, as
// formats for args.
func N(one, other string, n int, args ...any) string {
	if n == 1 {
		return T(one, args...)
	}
	return T(other, args...)
}

// Mark is s, untranslated: it marks text translated later with T, as the
// names in a table, so that the translations have it.
func Mark(s string) string { return s }
