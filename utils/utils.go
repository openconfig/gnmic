// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"log"
	"net"
	"reflect"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
)

const (
	DefaultLoggingFlags = log.LstdFlags | log.Lmicroseconds | log.Lmsgprefix
)

func MergeMaps(dst, src map[string]interface{}) map[string]interface{} {
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			srcMap, srcMapOk := mapify(srcVal)
			dstMap, dstMapOk := mapify(dstVal)
			if srcMapOk && dstMapOk {
				srcVal = MergeMaps(dstMap, srcMap)
			}
		}
		dst[key] = srcVal
	}
	return dst
}

func mapify(i interface{}) (map[string]interface{}, bool) {
	value := reflect.ValueOf(i)
	if value.Kind() == reflect.Map {
		m := map[string]interface{}{}
		for _, k := range value.MapKeys() {
			m[k.String()] = value.MapIndex(k).Interface()
		}
		return m, true
	}
	return map[string]interface{}{}, false
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

func GetHost(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return h
}

func Convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		nm := map[string]interface{}{}
		for k, v := range x {
			nm[k.(string)] = Convert(v)
		}
		return nm
	case map[string]interface{}:
		for k, v := range x {
			x[k] = Convert(v)
		}
	case []interface{}:
		for k, v := range x {
			x[k] = Convert(v)
		}
	}
	return i
}
