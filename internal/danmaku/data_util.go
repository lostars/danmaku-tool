package danmaku

import (
	"errors"
	"fmt"
	"os"
)

func checkPersistPath(fullPath, filename string) error {
	if fullPath == "" || filename == "" {
		return errors.New("empty save path or filename")
	}

	// check path
	_, fileStatError := os.Stat(fullPath)
	if fileStatError != nil {
		if os.IsNotExist(fileStatError) {
			mkdirError := os.MkdirAll(fullPath, os.ModePerm)
			if mkdirError != nil {
				return errors.New(fmt.Sprintf("create path %s error: %s", fullPath, mkdirError.Error()))
			}
		} else {
			return errors.New(fmt.Sprintf("create path %s error: %s", fullPath, fileStatError.Error()))
		}
	}
	return nil
}
