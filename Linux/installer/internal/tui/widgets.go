package tui

import (
	"regexp"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type builder struct {
	s string
}

func (b *builder) line(s string) {
	b.s += s + "\n"
}

func stringsBuilder(fn func(*builder)) string {
	var b builder
	fn(&b)
	return b.s
}

var (
	tuiMonitorModeRe     = regexp.MustCompile(`^([0-9]+x[0-9]+@[0-9]+(\.[0-9]+)?|preferred|auto|highres|highrr)$`)
	tuiMonitorNameRe     = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	tuiMonitorPositionRe = regexp.MustCompile(`^(-?[0-9]+x-?[0-9]+|auto)$`)
	tuiMonitorScaleRe    = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
)

var (
	macchiatoBase     = lipgloss.Color("#24273a")
	macchiatoSurface0 = lipgloss.Color("#363a4f")
	macchiatoMantle   = lipgloss.Color("#1e2030")
	macchiatoText     = lipgloss.Color("#cad3f5")
	macchiatoSubtext  = lipgloss.Color("#a5adcb")
	macchiatoMauve    = lipgloss.Color("#c6a0f6")
	macchiatoLavender = lipgloss.Color("#b7bdf8")
	macchiatoRed      = lipgloss.Color("#ed8796")

	screenStyle             = lipgloss.NewStyle().Background(macchiatoBase).Foreground(macchiatoText)
	spacerStyle             = lipgloss.NewStyle().Background(macchiatoBase)
	sidebarStyle            = lipgloss.NewStyle().Background(macchiatoMantle)
	panelStyle              = lipgloss.NewStyle().Background(macchiatoSurface0)
	footerStyle             = lipgloss.NewStyle().Background(macchiatoBase).Foreground(macchiatoSubtext)
	titleStyle              = lipgloss.NewStyle().Bold(true).Foreground(macchiatoMauve)
	headerStyle             = lipgloss.NewStyle().Bold(true).Foreground(macchiatoLavender)
	mutedStyle              = lipgloss.NewStyle().Foreground(macchiatoSubtext)
	previewHeadingStyle     = lipgloss.NewStyle().Bold(true).Foreground(macchiatoMauve)
	previewDescriptionStyle = lipgloss.NewStyle().Foreground(macchiatoSubtext)
	previewValueStyle       = lipgloss.NewStyle().Bold(true).Foreground(macchiatoLavender)
	formErrorStyle          = lipgloss.NewStyle().Foreground(macchiatoRed)
	itemStyle               = lipgloss.NewStyle().Foreground(macchiatoText)
	selectedStyle           = lipgloss.NewStyle().Foreground(macchiatoBase).Background(macchiatoMauve).Bold(true)
)

func theme() *huh.Theme {
	t := huh.ThemeCatppuccin()
	return t
}
