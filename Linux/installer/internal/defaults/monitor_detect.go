package defaults

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

type hyprctlMonitor struct {
	Name        string  `json:"name"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	RefreshRate float64 `json:"refreshRate"`
	X           int     `json:"x"`
	Y           int     `json:"y"`
	Scale       float64 `json:"scale"`
	Focused     bool    `json:"focused"`
	Disabled    bool    `json:"disabled"`
}

func DetectMonitors(ctx context.Context, runner run.CommandRunner) ([]config.Monitor, error) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return nil, nil //nolint:nilerr // documented "unavailable" signal
	}

	output, err := runner.Output(ctx, "hyprctl", "monitors", "-j")
	if err != nil {
		return nil, nil //nolint:nilerr // documented "unavailable" signal
	}
	if output == "" {
		return nil, nil
	}

	var raw []hyprctlMonitor
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil, fmt.Errorf("parse hyprctl monitors output: %w", err)
	}

	enabled := raw[:0]
	for _, monitor := range raw {
		if monitor.Disabled {
			continue
		}
		if monitor.Name == "" {
			continue
		}
		enabled = append(enabled, monitor)
	}
	if len(enabled) == 0 {
		return nil, nil
	}

	sort.SliceStable(enabled, func(i, j int) bool {
		if enabled[i].Focused != enabled[j].Focused {
			return enabled[i].Focused
		}
		if enabled[i].Y != enabled[j].Y {
			return enabled[i].Y < enabled[j].Y
		}
		return enabled[i].X < enabled[j].X
	})

	monitors := make([]config.Monitor, 0, len(enabled))
	for _, monitor := range enabled {
		monitors = append(monitors, config.Monitor{
			Name:     monitor.Name,
			Mode:     "preferred",
			Position: fmt.Sprintf("%dx%d", monitor.X, monitor.Y),
			Scale:    formatScale(monitor.Scale),
		})
	}
	return monitors, nil
}

func formatScale(scale float64) string {
	if scale <= 0 {
		return "1"
	}
	return strconv.FormatFloat(scale, 'f', -1, 64)
}
