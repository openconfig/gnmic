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

var pathsTable = map[string]struct {
	strPath  string
	gnmiPath *gnmi.Path
}{
	"test1": {
		strPath:  "/",
		gnmiPath: &gnmi.Path{},
	},
	"test2": {
		strPath: "origin:e1/e2",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2"},
			},
		},
	},
	"test3": {
		strPath: "origin:",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
		},
	},
	"test4": {
		strPath: "e",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e"},
			},
		},
	},
	"test5": {
		strPath: "/e1/e2",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2"},
			},
		},
	},
	"test6": {
		strPath: "/e1/e2[k=v]",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": "v",
					}},
			},
		},
	},
	"test7": {
		strPath: "origin:/e1/e2[k=v]",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": "v",
					}},
			},
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
	for name, tc := range pathsTable {
		t.Run(name, func(t *testing.T) {
			p, err := ParsePath(tc.strPath)
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(p, tc.gnmiPath) {
				t.Errorf("ParsePath failed with path: %s", tc.strPath)
			}
		})
	}
}
