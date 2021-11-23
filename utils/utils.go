package utils

import (
	"log"
	"net"
	"reflect"
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
	sb := strings.Builder{}
	if p.Origin != "" {
		sb.WriteString(p.Origin)
		sb.WriteString(":")
	}
	elems := p.GetElem()
	numElems := len(elems)
	for i, pe := range elems {
		sb.WriteString(pe.GetName())
		if !noKeys {
			for k, v := range pe.GetKey() {
				sb.WriteString("[")
				sb.WriteString(k)
				sb.WriteString("=")
				sb.WriteString(v)
				sb.WriteString("]")
			}
		}
		if i+1 != numElems {
			sb.WriteString("/")
		}
	}
	return sb.String()
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
