package main

import (
	"fmt"
	"github.com/spf13/afero"
	"testing"
)

const (
	testLabelFile  = "testLabel"
	dependentLabel = "dependentLabel"
)

var (
	testLabelPath = fmt.Sprintf("%s/%s", labelDir, testLabelFile)
)

// Create a mocked label in a bare memory-mapped filesystem in order to validate label extraction and matching
func init() {
	appfs = afero.NewMemMapFs()
	fsutil = &afero.Afero{Fs: appfs}

	appfs.MkdirAll(labelDir, 0755)
	afero.WriteFile(appfs, testLabelPath, []byte(dependentLabel+"\n"), 0644)
}

func TestReadLabelsFromFile(t *testing.T) {
	labels := readLabelsFromFile(testLabelPath)
	matched := false
	for _, label := range labels {
		if label == dependentLabel {
			matched = true
			break
		}
	}

	if matched == false {
		t.Error("unexpected error matching test label")
	}
}

func TestBuildLabelMap(t *testing.T) {
	labelMap, err := buildPossibleLabelMap()
	if err != nil {
		t.Error("unexpected error:", err.Error())
		return
	}

	if len(labelMap) == 0 {
		t.Error("received an empty label map")
		return
	}
}

func TestPathToLabel(t *testing.T) {
	label := pathToLabel(testLabelPath)
	if label != testLabelFile {
		t.Errorf("unexpected label: %s\n", label)
	}
}
