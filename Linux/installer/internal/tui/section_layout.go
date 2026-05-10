package tui

type layoutSize struct {
	width        int
	height       int
	sidebarWidth int
	contentWidth int
	bodyHeight   int
	gapWidth     int
}

func layoutFor(width, height int) layoutSize {
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 30
	}
	gapWidth := 1
	sidebarWidth := minInt(32, maxInt(20, width/4))
	if width < 80 {
		sidebarWidth = minInt(24, maxInt(18, width/3))
	}
	if width < 44 {
		sidebarWidth = maxInt(14, width/3)
	}
	contentWidth := maxInt(12, width-sidebarWidth-gapWidth)
	bodyHeight := maxInt(8, height-1)
	return layoutSize{
		width:        width,
		height:       height,
		sidebarWidth: sidebarWidth,
		contentWidth: contentWidth,
		bodyHeight:   bodyHeight,
		gapWidth:     gapWidth,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
