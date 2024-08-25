package chat

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up                       key.Binding // ctrl+up
	Down                     key.Binding // ctrl+down
	HalfPageUp               key.Binding // ctrl+u
	HalfPageDown             key.Binding // ctrl+d
	HighlightPreviousMessage key.Binding // ctrl+p
	HighlightNextMessage     key.Binding // ctrl+n
	CopyHighlightedMessage   key.Binding // alt+c
	CopyLastResponse         key.Binding // ctrl+shift+c
	ToggleImagePicker        key.Binding // ctrl+o
	RemoveAttachment         key.Binding // ctrl+x
	ToggleHelp               key.Binding // ctrl+h
	Quit                     key.Binding // ctrl+c
	FullHelpKeys             [][]key.Binding
}

var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("ctrl+up", "k"),
		key.WithHelp("ctrl+↑", "Move view up"),
	),
	Down: key.NewBinding(
		key.WithKeys("ctrl+down", "j"),
		key.WithHelp("ctrl+↓", "Move view down"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "Half page up"),
	),
	HalfPageDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "Half page down"),
	),
	HighlightPreviousMessage: key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "Previous message"),
	),
	HighlightNextMessage: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "Next message"),
	),
	CopyLastResponse: key.NewBinding(
		key.WithKeys("ctrl+y"),
		key.WithHelp("ctrl+y", "Copy last response"),
	),
	CopyHighlightedMessage: key.NewBinding(
		key.WithKeys("alt+y"),
		key.WithHelp("alt+y", "Copy highlighted message"),
	),
	ToggleImagePicker: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "Toggle image picker"),
	),
	RemoveAttachment: key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "Remove attachment"),
	),
	ToggleHelp: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("ctrl+h", "Toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "Exit chat"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit}
}

func (k *KeyMap) SetFullHelpKeys(keys [][]key.Binding) {
	k.FullHelpKeys = keys
}

func (k *KeyMap) DefaultFullHelpKeys() [][]key.Binding {
	return [][]key.Binding{
		{
			k.Up,
			k.Down,
			k.HalfPageUp,
			k.HalfPageDown,
			k.CopyLastResponse,
			k.ToggleHelp,
		},
		{
			k.HighlightPreviousMessage,
			k.HighlightNextMessage,
			k.CopyHighlightedMessage,
			k.ToggleImagePicker,
			k.RemoveAttachment,
			k.Quit,
		},
	}
}

func (k *KeyMap) DefaultFullHelpKeysNonMultiModal() [][]key.Binding {
	return [][]key.Binding{
		{
			k.Up,
			k.Down,
			k.HalfPageUp,
			k.HalfPageDown,
			k.CopyLastResponse,
			k.ToggleHelp,
		},
		{
			k.HighlightPreviousMessage,
			k.HighlightNextMessage,
			k.CopyHighlightedMessage,
			k.RemoveAttachment,
			k.Quit,
		},
	}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k KeyMap) FullHelp() [][]key.Binding {
	return k.FullHelpKeys
}
