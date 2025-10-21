// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package path

import (
	"errors"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
)

var errMalformedXPath = errors.New("malformed xpath")
var errMalformedXPathKey = errors.New("malformed xpath key")

var escapedBracketsReplacer = strings.NewReplacer(`\]`, `]`, `\[`, `[`)

// CreatePrefix //
func CreatePrefix(prefix, target string) (*gnmi.Path, error) {
	if len(prefix)+len(target) == 0 {
		return nil, nil
	}
	p, err := ParsePath(prefix)
	if err != nil {
		return nil, err
	}
	if target != "" {
		p.Target = target
	}
	return p, nil
}

// ParsePath creates a gnmi.Path out of a p string, check if the first element is prefixed by an origin,
// removes it from the xpath and adds it to the returned gnmiPath
func ParsePath(p string) (*gnmi.Path, error) {
	lp := len(p)
	if lp == 0 {
		return &gnmi.Path{}, nil
	}
	var origin string

	idx := strings.Index(p, ":")
	if idx >= 0 && p[0] != '/' && !strings.Contains(p[:idx], "/") &&
		// path == origin:/ || path == origin:
		((idx+1 < lp && p[idx+1] == '/') || (lp == idx+1)) {
		origin = p[:idx]
		p = p[idx+1:]
	}

	pes, err := toPathElems(p)
	if err != nil {
		return nil, err
	}
	return &gnmi.Path{
		Origin: origin,
		Elem:   pes,
	}, nil
}

// toPathElems parses a xpath and returns a list of path elements
func toPathElems(p string) ([]*gnmi.PathElem, error) {
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	buffer := make([]rune, 0)
	null := rune(0)
	prevC := rune(0)
	// track if the loop is traversing a key
	inKey := false
	for _, r := range p {
		switch r {
		case '[':
			if inKey && prevC != '\\' {
				return nil, errMalformedXPath
			}
			if prevC != '\\' {
				inKey = true
			}
		case ']':
			if !inKey && prevC != '\\' {
				return nil, errMalformedXPath
			}
			if prevC != '\\' {
				inKey = false
			}
		case '/':
			if !inKey {
				buffer = append(buffer, null)
				prevC = r
				continue
			}
		}
		buffer = append(buffer, r)
		prevC = r
	}
	if inKey {
		return nil, errMalformedXPath
	}
	stringElems := strings.Split(string(buffer), string(null))
	pElems := make([]*gnmi.PathElem, 0, len(stringElems))
	for _, s := range stringElems {
		if s == "" {
			continue
		}
		pe, err := toPathElem(s)
		if err != nil {
			return nil, err
		}
		pElems = append(pElems, pe)
	}
	return pElems, nil
}

// toPathElem take a xpath formatted path element such as "elem1[k=v]" and returns the corresponding gnmi.PathElem
func toPathElem(s string) (*gnmi.PathElem, error) {
	idx := -1
	prevC := rune(0)
	for i, r := range s {
		if r == '[' && prevC != '\\' {
			idx = i
			break
		}
		prevC = r
	}
	var kvs map[string]string
	if idx > 0 {
		var err error
		kvs, err = parseXPathKeys(s[idx:])
		if err != nil {
			return nil, err
		}
		s = s[:idx]
	}
	return &gnmi.PathElem{Name: s, Key: kvs}, nil
}

// parseXPathKeys takes keys definition from an xpath, e.g [k1=v1][k2=v2] and return the keys and values as a map[string]string
func parseXPathKeys(s string) (map[string]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	kvs := make(map[string]string)
	inKey := false
	start := 0
	prevRune := rune(0)
	for i, r := range s {
		switch r {
		case '[':
			if prevRune == '\\' {
				prevRune = r
				continue
			}
			if inKey {
				return nil, errMalformedXPathKey
			}
			inKey = true
			start = i + 1
		case ']':
			if prevRune == '\\' {
				prevRune = r
				continue
			}
			if !inKey {
				return nil, errMalformedXPathKey
			}
			eq := strings.Index(s[start:i], "=")
			if eq < 0 {
				return nil, errMalformedXPathKey
			}
			k, v := s[start:i][:eq], s[start:i][eq+1:]
			if len(k) == 0 || len(v) == 0 {
				return nil, errMalformedXPathKey
			}
			kvs[escapedBracketsReplacer.Replace(k)] = escapedBracketsReplacer.Replace(v)
			inKey = false

		default:
			if !inKey {
				return nil, errMalformedXPathKey
			}
		}
		prevRune = r
	}
	if inKey {
		return nil, errMalformedXPathKey
	}
	return kvs, nil
}

func PathElems(pf, p *gnmi.Path) []*gnmi.PathElem {
	r := make([]*gnmi.PathElem, 0, len(pf.GetElem())+len(p.GetElem()))
	r = append(r, pf.GetElem()...)
	return append(r, p.GetElem()...)
}

func GnmiPathToXPath(p *gnmi.Path, noKeys bool) string {
	if p == nil {
		return ""
	}
	sb := &strings.Builder{}
	if p.Origin != "" {
		sb.WriteString(p.Origin)
		sb.WriteString(":")
	}
	elems := p.GetElem()
	numElems := len(elems)
	sb.WriteString("/")

	for i, pe := range elems {
		sb.WriteString(pe.GetName())
		if !noKeys {
			numKeys := len(pe.GetKey())
			switch numKeys {
			case 0:
			case 1:
				for k := range pe.GetKey() {
					writeKey(sb, k, pe.GetKey()[k])
				}
			default:
				keys := make([]string, 0, numKeys)
				for k := range pe.GetKey() {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					writeKey(sb, k, pe.GetKey()[k])
				}
			}
		}
		if i+1 != numElems {
			sb.WriteString("/")
		}
	}
	return sb.String()
}

func writeKey(sb *strings.Builder, k, v string) {
	sb.WriteString("[")
	sb.WriteString(k)
	sb.WriteString("=")
	sb.WriteString(v)
	sb.WriteString("]")
}
