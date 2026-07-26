package app

import "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"

type Options struct {
	paths.Options
	DryRun   bool
	Yes      bool
	Layout   string
	LockMode string
}
