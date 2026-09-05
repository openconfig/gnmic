// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_keep

import (
	"fmt"
	"strings"
)

type pathMatcher struct {
	root pathNode
}

type pathNode struct {
	literals map[string]*pathNode
	wildcard *pathNode
	terminal bool
}

func compilePathMatcher(selectors []string) (*pathMatcher, error) {
	if len(selectors) == 0 {
		return nil, nil
	}
	m := &pathMatcher{root: pathNode{literals: make(map[string]*pathNode)}}
	for _, selector := range selectors {
		if selector == "" || selector[0] != '/' {
			return nil, fmt.Errorf("invalid value-name-paths selector %q: must be absolute", selector)
		}
		segments := strings.Split(selector[1:], "/")
		node := &m.root
		for _, segment := range segments {
			if segment == "" {
				return nil, fmt.Errorf("invalid value-name-paths selector %q: empty path segment", selector)
			}
			if segment == "*" {
				if node.wildcard == nil {
					node.wildcard = newPathNode()
				}
				node = node.wildcard
				continue
			}
			child := node.literals[segment]
			if child == nil {
				child = newPathNode()
				node.literals[segment] = child
			}
			node = child
		}
		node.terminal = true
	}
	return m, nil
}

func newPathNode() *pathNode {
	return &pathNode{literals: make(map[string]*pathNode)}
}

func (m *pathMatcher) Match(value string) bool {
	if m == nil || len(value) < 2 || value[0] != '/' {
		return false
	}
	return m.root.match(value[1:])
}

func (n *pathNode) match(value string) bool {
	segment, remaining, more := strings.Cut(value, "/")
	if segment == "" {
		return false
	}
	if child := n.literals[segment]; child != nil && child.matchesRemaining(remaining, more) {
		return true
	}
	return n.wildcard != nil && n.wildcard.matchesRemaining(remaining, more)
}

func (n *pathNode) matchesRemaining(remaining string, more bool) bool {
	if !more {
		return n.terminal
	}
	return n.match(remaining)
}
