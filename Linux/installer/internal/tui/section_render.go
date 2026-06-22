package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m sectionModel) View() string {
	layout := layoutFor(m.width, m.height)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderSidebar(m, layout.sidebarWidth, layout.bodyHeight),
		spacerStyle.Width(layout.gapWidth).Height(layout.bodyHeight).Render(""),
		renderContent(m, layout.contentWidth, layout.bodyHeight),
	)
	footer := renderFooter(layout.width)
	return screenStyle.
		Width(layout.width).
		Height(layout.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, body, footer))
}

func renderSidebar(m sectionModel, width, height int) string {
	sidebar := stringsBuilder(func(b *builder) {
		b.line(sidebarText(width, titleStyle.Render(" MySetup")))
		b.line(sidebarText(width, mutedStyle.Render("Catppuccin Macchiato installer")))
		b.line("")
		for i, section := range sections {
			item := sidebarText(width, section)
			if i == m.cursor {
				item = selectedStyle.Width(maxInt(1, width-4)).Render(" " + section)
			} else {
				item = itemStyle.Render(item)
			}
			b.line(item)
		}
	})
	return sidebarStyle.Width(width).Height(height).Render(sidebar)
}

func renderContent(m sectionModel, width, height int) string {
	content := stringsBuilder(func(b *builder) {
		b.line(contentText(width, headerStyle.Render(selectedSection(m))))
		b.line("")
		for _, line := range sectionPreview(m) {
			b.line(renderPreviewLine(width, line))
		}
		b.line("")
		b.line(contentText(width, mutedStyle.Render("Select a section with ↑/↓ or k/j, then press Enter.")))
	})
	return panelStyle.Width(width).Height(height).Render(content)
}

func selectedSection(m sectionModel) string {
	if m.cursor < 0 || m.cursor >= len(sections) {
		return "MySetup"
	}
	return sections[m.cursor]
}

const previewHeadingPrefix = "## "

func previewValuePrefix(label string) string {
	return label + ": "
}

func isPreviewValueLine(line string) bool {
	for _, label := range []string{"current", "options", "status", "action", "target", "order", "mode"} {
		if strings.HasPrefix(line, previewValuePrefix(label)) {
			return true
		}
	}
	return false
}

func renderPreviewLine(width int, line string) string {
	switch {
	case strings.HasPrefix(line, previewHeadingPrefix):
		return contentText(width, previewHeadingStyle.Render(line))
	case isPreviewValueLine(line):
		return contentText(width, previewValueStyle.Render(line))
	case line == "":
		return contentText(width, "")
	default:
		return contentText(width, previewDescriptionStyle.Render(line))
	}
}

func renderFooter(width int) string {
	return footerStyle.Width(width).Render("  ↑/↓ or j/k: move    Enter: open    Esc/Ctrl+C: quit")
}

func sidebarText(width int, text string) string {
	return lipgloss.NewStyle().PaddingLeft(2).Width(maxInt(1, width-2)).Render(text)
}

func contentText(width int, text string) string {
	return lipgloss.NewStyle().PaddingLeft(3).Width(maxInt(1, width-3)).Render(text)
}
