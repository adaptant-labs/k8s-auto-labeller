package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func readLabelsFromFile(path string) []string {
	fileBytes, err := fsutil.ReadFile(path)
	if err != nil {
		return []string{""}
	}

	return strings.Split(string(fileBytes), "\n")
}

func pathToLabel(path string) string {
	// Strip leading label directory (defaults to 'labels/') from the file path
	return strings.Replace(path, labelDir+"/", "", 1)
}

// Construct a label map in the form of label: [dependent labels...] gleaned
// from the filesystem
func buildPossibleLabelMap() (map[string][]string, error) {
	labelMap := make(map[string][]string)

	err := filepath.Walk(labelDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			labelMap[pathToLabel(path)] = readLabelsFromFile(path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(labelMap) == 0 {
		return nil, errors.New("no labels found")
	}

	return labelMap, nil
}
