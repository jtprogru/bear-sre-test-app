package server

import (
	"fmt"
	"os"
)

// checkSecretFile проверяет наличие и размер секретного файла.
//
// Используется os.Stat, а не os.Open: открытый дескриптор здесь не нужен, а
// прежняя реализация его не закрывала — каждый запрос к /secret навсегда
// съедал один fd, и сервис умирал об ulimit -n через несколько тысяч запросов.
func checkSecretFile(path string, minSize int64) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrSecretFileNotFound, path)
	}
	if fi.IsDir() {
		return 0, fmt.Errorf("%w: %s is a directory", ErrSecretFileNotFound, path)
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
