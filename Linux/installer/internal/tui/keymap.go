package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

func installerFormKeyMap() *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	keymap.Input.Prev = key.NewBinding(key.WithKeys("shift+tab", "shift+enter"), key.WithHelp("shift+tab", "back field"))
	keymap.Input.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Input.Submit = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save section"))
	keymap.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab", "shift+enter"), key.WithHelp("shift+tab", "back field"))
	keymap.Confirm.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Confirm.Submit = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save section"))
	keymap.Confirm.Toggle = key.NewBinding(key.WithKeys("left", "right", "up", "down"), key.WithHelp("arrows", "choose"))
	keymap.Confirm.Accept = key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "yes"), key.WithDisabled())
	keymap.Confirm.Reject = key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n", "no"), key.WithDisabled())
	keymap.Select.Prev = key.NewBinding(key.WithKeys("shift+tab", "shift+enter"), key.WithHelp("shift+tab", "back field"))
	keymap.Select.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Select.Submit = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save section"))
	keymap.Select.Filter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"), key.WithDisabled())
	keymap.Select.SetFilter = key.NewBinding(key.WithKeys("enter", "esc"), key.WithHelp("enter/esc", "set filter"), key.WithDisabled())
	keymap.Select.ClearFilter = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter"), key.WithDisabled())
	keymap.MultiSelect.Prev = key.NewBinding(key.WithKeys("shift+tab", "shift+enter"), key.WithHelp("shift+tab", "back field"))
	keymap.MultiSelect.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.MultiSelect.Submit = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save section"))
	keymap.MultiSelect.Toggle = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle"))
	keymap.MultiSelect.SelectAll = key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "select all"), key.WithDisabled())
	keymap.MultiSelect.SelectNone = key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "select none"), key.WithDisabled())
	keymap.Note.Prev = key.NewBinding(key.WithKeys("shift+tab", "shift+enter"), key.WithHelp("shift+tab", "back field"))
	keymap.Note.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Note.Submit = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save section"))
	return keymap
}
