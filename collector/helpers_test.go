package collector

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
)

var prefixSet = map[string]*gnmi.Path{
	"": nil,
	"target%%%origin:/e1/e2": {
		Origin: "origin",
		Target: "target",
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2"},
		},
	},
	"/e1": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
		},
	},
	"/e1/e2[k=v]": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2",
				Key: map[string]string{
					"k": "v",
				}},
		},
	},
}

var pathsSet = map[string]*gnmi.Path{
	"/": {},
	"origin:e1/e2": {
		Origin: "origin",
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2"},
		},
	},
	"origin:/e1/e2": {
		Origin: "origin",
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2"},
		},
	},
	"origin:": {Origin: "origin"},
	"e": {
		Elem: []*gnmi.PathElem{
			{Name: "e"},
		},
	},
	"/e1/e2": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2"},
		},
	},
	"/e1/e2[k=v]": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2",
				Key: map[string]string{
					"k": "v",
				}},
		},
	},
	"origin:/e1/e2[k=v]": {
		Origin: "origin",
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2",
				Key: map[string]string{
					"k": "v",
				}},
		},
	},
}

func TestCreatePrefix(t *testing.T) {
	var target, prefix string
	for e, p := range prefixSet {
		val := strings.Split(e, "%%%")
		fmt.Printf("%d: %v\n", len(val), val)
		if len(val) == 2 {
			target, prefix = val[0], val[1]
		} else if len(val) == 1 {
			target, prefix = "", val[0]
		}
		fmt.Println(target, prefix)
		gp, err := CreatePrefix(prefix, target)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(p, gp) {
			t.Errorf("failed at elem: %s: expecting %v, got %v", e, p, gp)
		}

	}
}

func TestParsePath(t *testing.T) {
	for p, g := range pathsSet {
		pg, err := ParsePath(p)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(g, pg) {
			t.Errorf("ParsePath failed with path: %s", p)
		}
	}
}
