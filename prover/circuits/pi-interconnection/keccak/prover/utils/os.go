package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func ReadAllJsonFiles(dirPath string) (map[string][]byte, error) {
	res := make(map[string][]byte)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		title, isJson := strings.CutSuffix(name, ".json")
		if entry.IsDir() || !isJson {
			continue
		}
		res[title], err = os.ReadFile(filepath.Join(dirPath, name))
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}
