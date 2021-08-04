package loaders

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/types"
)

var testSet = map[string]struct {
	m1, m2 map[string]*types.TargetConfig
	output *TargetOperation
}{
	"t1": {
		m1: nil,
		m2: nil,
		output: &TargetOperation{
			Add: make([]*types.TargetConfig, 0),
			Del: make([]string, 0),
		},
	},
	"t2": {
		m1: nil,
		m2: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		output: &TargetOperation{
			Add: []*types.TargetConfig{
				{
					Name: "target1",
				},
			},
			Del: make([]string, 0),
		},
	},
	"t3": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		m2: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		output: &TargetOperation{
			Add: make([]*types.TargetConfig, 0),
			Del: make([]string, 0),
		},
	},
	"t4": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
			"target2": {Name: "target2"},
		},
		m2: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
			"target2": {Name: "target2"},
		},
		output: &TargetOperation{
			Add: make([]*types.TargetConfig, 0),
			Del: make([]string, 0),
		},
	},
	"t5": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		m2: nil,
		output: &TargetOperation{
			Add: make([]*types.TargetConfig, 0),
			Del: []string{"target1"},
		},
	},
	"t6": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		m2: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
			"target2": {Name: "target2"},
		},
		output: &TargetOperation{
			Add: []*types.TargetConfig{
				{
					Name: "target2",
				},
			},
			Del: make([]string, 0),
		},
	},
	"t7": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		m2: map[string]*types.TargetConfig{
			"target2": {Name: "target2"},
		},
		output: &TargetOperation{
			Add: []*types.TargetConfig{
				{
					Name: "target2",
				},
			},
			Del: []string{"target1"},
		},
	},
	"t8": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
		},
		m2: map[string]*types.TargetConfig{
			"target2": {Name: "target2"},
			"target3": {Name: "target3"},
		},
		output: &TargetOperation{
			Add: []*types.TargetConfig{
				{
					Name: "target2",
				},
				{
					Name: "target3",
				},
			},
			Del: []string{"target1"},
		},
	},
	"t9": {
		m1: map[string]*types.TargetConfig{
			"target1": {Name: "target1"},
			"target2": {Name: "target2"},
		},
		m2: map[string]*types.TargetConfig{
			"target2": {Name: "target2"},
			"target3": {Name: "target3"},
		},
		output: &TargetOperation{
			Add: []*types.TargetConfig{
				{
					Name: "target3",
				},
			},
			Del: []string{"target1"},
		},
	},
}

func TestGetInstancesTagsMatches(t *testing.T) {
	for name, item := range testSet {
		t.Run(name, func(t *testing.T) {
			res := Diff(item.m1, item.m2)
			sort.Slice(res.Add, func(i, j int) bool {
				return res.Add[i].Name < res.Add[j].Name
			})
			sort.Slice(item.output.Add, func(i, j int) bool {
				return item.output.Add[i].Name < item.output.Add[j].Name
			})
			t.Logf("exp value: %+v", item.output)
			t.Logf("got value: %+v", res)
			if !cmp.Equal(item.output, res) {
				t.Fail()
			}
		})
	}
}
