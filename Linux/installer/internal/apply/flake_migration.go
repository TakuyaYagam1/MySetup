package apply

import (
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

func migrateGeneratedThinFlake(text string, state config.State) (string, bool, error) {
	lockMode := LockModeIndependent
	if strings.Contains(text, "# lock mode: managed") {
		lockMode = LockModeManaged
	}

	generated, err := FlakeNix(state, lockMode)
	if err != nil {
		return "", false, err
	}
	return generated, generated != text, nil
}
