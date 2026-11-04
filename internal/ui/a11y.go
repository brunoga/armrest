package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
)

// iconButton is a button showing only an icon, which screen readers call
// by its label instead of the icon's file name.
type iconButton struct {
	widget.Button
	label string
}

// newIconButton is a button of icon alone, called label (in the user's
// language) by screen readers.
func newIconButton(label string, icon fyne.Resource, tapped func()) *iconButton {
	b := &iconButton{label: label}
	b.Icon = icon
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *iconButton) AccessibilityLabel() string { return b.label }

// AccessibilityLabel says what the handle does.
func (h *dragHandle) AccessibilityLabel() string { return i18n.T("Drag to move") }

// AccessibilityRole is a button's: it is a control, not text.
func (h *dragHandle) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }
