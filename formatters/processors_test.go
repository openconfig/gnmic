package formatters

import (
	"testing"
	"time"

	"github.com/itchyny/gojq"
)

var testset = map[string]struct {
	condition string
	input     []*EventMsg
	result    bool
}{
	"always_true": {
		condition: "any([true])",
		input: []*EventMsg{
			{
				Name:      "dummy1",
				Timestamp: time.Now().Unix(),
				Tags:      map[string]string{"t1": "t1v"},
				Values: map[string]interface{}{
					"path/dummy": 1,
				},
			},
			{
				Name:      "dummy2",
				Timestamp: time.Now().Unix(),
				Tags:      map[string]string{"t1": "t1v"},
				Values: map[string]interface{}{
					"path/dummy": 1,
				},
			},
		},
		result: true,
	},
}

func TestCheckCondition(t *testing.T) {
	for name, item := range testset {
		t.Run(name, func(t *testing.T) {
			t.Logf("running test item %s", name)
			q, err := gojq.Parse(item.condition)
			if err != nil {
				t.Logf("condition parse failed :%v", err)
				t.Fail()
			}
			code, err := gojq.Compile(q)
			if err != nil {
				t.Logf("query compile failed :%v", err)
				t.Fail()
			}
			for _, in := range item.input {
				ok, err := CheckCondition(code, in)
				if err != nil {
					t.Logf("check condition failed :%v", err)
					t.Fail()
				}
				if ok != item.result {
					t.Logf("failed at %q", name)
					t.Logf("expected: (%T)%+v", item.result, item.result)
					t.Logf("     got: (%T)%+v", ok, ok)
					t.Fail()
				}
			}
		})
	}
}
