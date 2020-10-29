package main

import "sync"

type LabelState map[string]bool
type NodeLabelMapType map[string]LabelState

type NodeLabelMap struct {
	m NodeLabelMapType
	sync.RWMutex
}

func NewNodeLabelMap() *NodeLabelMap {
	return &NodeLabelMap{
		m: make(NodeLabelMapType),
	}
}

// Disable any previously set labels, the reconciler will use this to clear stale node labels
func (n *NodeLabelMap) ResetLabels(nodeName string) {
	for label := range n.m[nodeName] {
		n.m[nodeName][label] = false
	}
}

// Add a node to the node map
func (n *NodeLabelMap) Add(nodeName string) {
	if _, exists := n.m[nodeName]; !exists {
		n.m[nodeName] = make(LabelState)
	}
}

// Remove a node from the node map
func (n *NodeLabelMap) Remove(nodeName string) {
	delete(n.m, nodeName)
}

// Identify labels to set. The map of possible labels must be iterated over with the read lock held,
// as it may changed by the filesystem watchers.
func (n *NodeLabelMap) SetPossible(nodeName string, nodeLabels map[string]string) {
	possibleLabelLock.RLock()
	for label := range nodeLabels {
		for labelKey, fileLabels := range possibleLabelMap {
			for _, fileLabel := range fileLabels {
				if fileLabel == label {
					n.m[nodeName][labelKey] = true
					break
				}
			}
		}
	}
	possibleLabelLock.RUnlock()
}

func (n *NodeLabelMap) AddLabelToNode(nodeName string, label string) {
	n.m[nodeName][label] = true
}

func (n *NodeLabelMap) RemoveLabelFromNode(nodeName string, label string) {
	delete(n.m[nodeName], label)
}

func (n *NodeLabelMap) GetLabels(nodeName string) LabelState {
	return n.m[nodeName]
}

// Determine if there are valid labels set for a given node
func (n *NodeLabelMap) Valid(nodeName string) bool {
	if _, exists := n.m[nodeName]; exists {
		return len(n.m[nodeName]) > 0
	}

	return false
}
