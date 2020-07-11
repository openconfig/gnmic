package collector

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
)

var prefixSet = map[string]*gnmi.Path{
	"": nil,
	"target:::/e1/e2": {
		Target: "target",
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2"},
		},
	},
	":::/e1": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
		},
	},
	":::/e1/e2[k=v]": {
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
		val := strings.Split(e, ":::")
		if len(val) == 2 {
			target, prefix = val[0], val[1]
		}
		gp, err := CreatePrefix(prefix, target)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(p, gp) {
			t.Errorf("failed at elem: %s", e)
		}

	}
}
