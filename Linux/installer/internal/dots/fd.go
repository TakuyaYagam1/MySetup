package dots

import (
	"fmt"
	"os"
)

func newFileFromUnixFD(fd int, name string) (*os.File, error) {
	if fd < 0 {
		return nil, fmt.Errorf("invalid file descriptor for %s: %d", name, fd)
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		return nil, fmt.Errorf("wrap file descriptor for %s", name)
	}
	return file, nil
}
