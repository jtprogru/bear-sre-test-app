package server

import (
	"fmt"
	"os"
)

// checkSecretFile проверяет наличие и размер секретного файла.
func checkSecretFile(path string, minSize int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrSecretFileNotFound, path)
	}

	fi, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrSecretFileNotFound, path)
	}

	size := fi.Size()
	switch {
	case size == 0:
		return 0, fmt.Errorf("%w: %s", ErrSecretFileIsEmpty, path)
	case size < minSize:
		return 0, fmt.Errorf("%w: %d < %d bytes", ErrSecretFileIsTooShort, size, minSize)
	}
	return size, nil
}
