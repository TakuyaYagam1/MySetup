package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func xkbLayoutOptions(current []string) []huh.Option[string] {
	layouts := config.AvailableXKBLayouts()
	byCode := make(map[string]config.XKBLayout, len(layouts))
	for _, layout := range layouts {
		byCode[layout.Code] = layout
	}

	ordered := make([]config.XKBLayout, 0, len(current)+len(layouts))
	seen := map[string]struct{}{}
	for _, code := range current {
		if !config.ValidXKBLayoutCode(code) {
			continue
		}
		layout, ok := byCode[code]
		if !ok {
			layout = config.XKBLayout{Code: code, Description: "unknown current layout"}
		}
		ordered = append(ordered, layout)
		seen[code] = struct{}{}
	}
	for _, layout := range layouts {
		if _, ok := seen[layout.Code]; ok {
			continue
		}
		ordered = append(ordered, layout)
	}

	options := make([]huh.Option[string], 0, len(ordered))
	for _, layout := range ordered {
		options = append(options, huh.NewOption(config.XKBLayoutLabel(layout), layout.Code))
	}
	return options
}
