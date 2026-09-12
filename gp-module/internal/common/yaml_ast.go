package common

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseYAMLNode parses a YAML byte slice into a root yaml.Node AST.
func ParseYAMLNode(data []byte) (*yaml.Node, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return &node, nil
}

// FindNode locates a child node within a yaml.Node tree using a dot-separated path (e.g. "spec.ports[0].containerPort").
func FindNode(root *yaml.Node, path string) *yaml.Node {
	if root == nil {
		return nil
	}

	curr := root
	if curr.Kind == yaml.DocumentNode && len(curr.Content) > 0 {
		curr = curr.Content[0]
	}

	parts := splitPath(path)
	for _, part := range parts {
		if curr == nil {
			return nil
		}

		if strings.HasSuffix(part, "]") {
			idxOpen := strings.Index(part, "[")
			key := part[:idxOpen]
			idxStr := part[idxOpen+1 : len(part)-1]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil
			}

			if key != "" {
				curr = findMapChild(curr, key)
			}
			if curr == nil || curr.Kind != yaml.SequenceNode || idx < 0 || idx >= len(curr.Content) {
				return nil
			}
			curr = curr.Content[idx]
		} else {
			curr = findMapChild(curr, part)
		}
	}

	return curr
}

// FindLineNumber returns the 1-based line number for a given path in the node, or 0 if not found.
func FindLineNumber(root *yaml.Node, path string) int {
	node := FindNode(root, path)
	if node != nil {
		return node.Line
	}
	return 0
}

func findMapChild(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			if i+1 < len(node.Content) {
				return node.Content[i+1]
			}
			return node.Content[i]
		}
	}
	return nil
}

// FindKeyNode returns the key node (useful to get line number of the key itself) rather than value node.
func FindKeyNode(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i]
		}
	}
	return nil
}

func splitPath(path string) []string {
	var parts []string
	var curr strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '.' {
			if curr.Len() > 0 {
				parts = append(parts, curr.String())
				curr.Reset()
			}
		} else {
			curr.WriteByte(c)
		}
	}
	if curr.Len() > 0 {
		parts = append(parts, curr.String())
	}
	return parts
}
