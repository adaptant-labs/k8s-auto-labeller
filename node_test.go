package main

import (
	"testing"
)

const (
	nodeName = "node"
	labelName = "label"
)

func TestNodeLabelMap_ResetLabels(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()

	nodeLabelMap.Add(nodeName)
	nodeLabelMap.AddLabelToNode(nodeName, labelName)
	nodeLabelMap.ResetLabels(nodeName)

	if nodeLabelMap.m[nodeName][labelName] != false {
		t.Error("unexpected label state in node label map, expected false")
	}
}

func TestNodeLabelMap_Add(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()

	nodeLabelMap.Add(nodeName)
	if _, exists := nodeLabelMap.m[nodeName]; !exists {
		t.Error("node not found in node label map")
	}
}

func TestNodeLabelMap_Remove(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()

	nodeLabelMap.Add(nodeName)
	nodeLabelMap.Remove(nodeName)

	if _, exists := nodeLabelMap.m[nodeName]; exists {
		t.Error("node unexpectedly found in node label map")
	}
}

func TestNodeLabelMap_AddLabelToNode(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()

	nodeLabelMap.Add(nodeName)
	nodeLabelMap.AddLabelToNode(nodeName, labelName)

	if nodeLabelMap.m[nodeName][labelName] != true {
		t.Error("unexpected label state in node label map, expected true")
	}
}

func TestNodeLabelMap_RemoveLabelFromNode(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()

	nodeLabelMap.Add(nodeName)
	nodeLabelMap.AddLabelToNode(nodeName, labelName)
	nodeLabelMap.RemoveLabelFromNode(nodeName, labelName)

	if _, exists := nodeLabelMap.m[nodeName][labelName]; exists {
		t.Error("node label unexpectedly found in node label map")
	}
}

func TestNodeLabelMap_GetLabels(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()
	nodeLabelMap.Add(nodeName)
	nodeLabelMap.AddLabelToNode(nodeName, labelName)

	matched := false
	labels := nodeLabelMap.GetLabels(nodeName)

	if len(labels) != 1 {
		t.Error("unexpected node label count")
	}

	for label := range labels {
		if label == labelName {
			matched = true
			break
		}
	}

	if !matched {
		t.Error("unexpected node label result")
	}
}

func TestNodeLabelMap_SetPossible(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()
	nodeLabelMap.Add(nodeName)

	possibleLabelMap = make(map[string][]string, 0)
	possibleLabelMap[labelName] = []string{labelName}

	nodeLabels := make(map[string]string, 0)
	nodeLabels[labelName] = "true"
	nodeLabelMap.SetPossible(nodeName, nodeLabels)
	labels := nodeLabelMap.GetLabels(nodeName)

	if len(labels) != 1 {
		t.Error("unexpected node label count")
	}

	matched := false
	for label := range labels {
		if label == labelName {
			matched = true
			break
		}
	}

	if !matched {
		t.Error("unexpected node label result")
	}
}

func TestNodeLabelMap_Valid(t *testing.T) {
	nodeLabelMap := NewNodeLabelMap()
	if nodeLabelMap.Valid(nodeName) != false {
		t.Error("unexpected node validity state")
	}
}
