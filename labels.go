package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func readLabelsFromFile(path string) []string {
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return []string{""}
	}

	return strings.Split(string(fileBytes), "\n")
}

func pathToLabel(path string) string {
	// Strip leading 'labels/' from file path
	return strings.Replace(path, labelDir+"/", "", 1)
}

func buildLabelMap() (map[string][]string, error) {
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

	return labelMap, nil
}
