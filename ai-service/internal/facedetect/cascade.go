package facedetect

import "os"

func readCascade(path string) ([]byte, error) {
	return os.ReadFile(path)
}
