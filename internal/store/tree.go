package store

import "strings"

type node struct {
	name     string // the last part of the dotted name
	fullName string // full dotted name from root
	children []*node
}

func (n *node) add(name string) {
	parts := strings.Split(name, ".")
	nn := &node{fullName: name, name: parts[len(parts)-1]}

	insert(parts, n, nn)
}

func insert(parts []string, existing *node, inserted *node) {
	if len(parts) == 1 {
		existing.children = append(existing.children, inserted)
		return
	}

	for _, child := range existing.children {
		if parts[0] == child.name {
			insert(parts[1:], child, inserted)
			return
		}
	}

	// add child
	newChild := &node{name: parts[0]}
	existing.children = append(existing.children, newChild)
	insert(parts[1:], newChild, inserted)
}

// buildHierarchy constructs a map of maps with values.
func (s *Store) buildHierarchy() map[string]any {
	h := make(map[string]any)
	buildMap(s.tree, h, s.kv)
	return h
}

func buildMap(n *node, m map[string]any, kv map[string]any) {
	for _, child := range n.children {
		if len(child.children) == 0 {
			if v, ok := kv[child.fullName]; ok {
				m[child.name] = v // leaf node, set value
			}
			continue
		}

		mm := make(map[string]any)
		m[child.name] = mm

		buildMap(child, mm, kv)
	}
}
