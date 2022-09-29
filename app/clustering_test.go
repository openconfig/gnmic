// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/lockers"
)

var testSetGetInstancesTagsMatches = map[string]struct {
	a      *App
	input  []string
	result map[string]int
}{
	"test1": {
		a: &App{
			apiServices: map[string]*lockers.Service{
				"gnmic1-api": {
					Tags: []string{
						"tag1",
						"tag2",
						"tag3",
					},
				},
				"gnmic2-api": {
					Tags: []string{
						"tag1",
						"tag2",
					},
				},
				"gnmic3-api": {},
			},
		},
		input: []string{
			"tag1",
			"tag2",
		},
		result: map[string]int{
			"gnmic1": 2,
			"gnmic2": 2,
			"gnmic3": 0,
		},
	},
	"test2": {
		a: &App{
			apiServices: map[string]*lockers.Service{
				"gnmic1-api": {
					Tags: []string{
						"tag1",
						"tag2",
						"tag3",
					},
				},
				"gnmic2-api": {
					Tags: []string{
						"tag1",
						"tag2",
					},
				},
				"gnmic3-api": {},
			},
		},
		input: []string{
			"tag1",
		},
		result: map[string]int{
			"gnmic1": 1,
			"gnmic2": 1,
			"gnmic3": 0,
		},
	},
	"test3": {
		a: &App{
			apiServices: map[string]*lockers.Service{
				"gnmic1-api": {
					Tags: []string{
						"tag1",
						"tag2",
						"tag3",
					},
				},
				"gnmic2-api": {
					Tags: []string{
						"tag1",
						"tag2",
					},
				},
				"gnmic3-api": {},
			},
		},
		input:  []string{},
		result: make(map[string]int),
	},
	"test4": {
		a: &App{
			apiServices: map[string]*lockers.Service{
				"gnmic1-api": {
					Tags: []string{
						"tag1",
						"tag2",
						"tag3",
					},
				},
				"gnmic2-api": {
					Tags: []string{
						"tag1",
						"tag2",
					},
				},
				"gnmic3-api": {},
			},
		},
		input: []string{
			"tag2",
		},
		result: map[string]int{
			"gnmic1": 0,
			"gnmic2": 0,
			"gnmic3": 0,
		},
	},
	"test5": {
		a: &App{
			apiServices: map[string]*lockers.Service{
				"gnmic1-api": {
					Tags: []string{
						"tag1",
						"tag2",
						"tag3",
					},
				},
				"gnmic2-api": {
					Tags: []string{
						"tag1",
						"tag2",
					},
				},
				"gnmic3-api": {
					Tags: []string{
						"tag1",
					},
				},
			},
		},
		input: []string{
			"tag1",
			"tag2",
			"tag3",
		},
		result: map[string]int{
			"gnmic1": 3,
			"gnmic2": 2,
			"gnmic3": 1,
		},
	},
}

var testSetGetHighestTagsMatches = map[string]struct {
	input  map[string]int
	result []string
}{
	"test1": {
		input: map[string]int{
			"gnmic1": 2,
			"gnmic2": 2,
			"gnmic3": 0,
		},
		result: []string{
			"gnmic1",
			"gnmic2",
		},
	},
	"test2": {
		input: map[string]int{
			"gnmic1": 0,
			"gnmic2": 0,
			"gnmic3": 0,
		},
		result: []string{
			"gnmic1",
			"gnmic2",
			"gnmic3",
		},
	},
	"test3": {
		input: map[string]int{
			"gnmic1": 1,
			"gnmic2": 1,
			"gnmic3": 1,
		},
		result: []string{
			"gnmic1",
			"gnmic2",
			"gnmic3",
		},
	},
	"test4": {
		input: map[string]int{
			"gnmic1": 0,
			"gnmic2": 0,
			"gnmic3": 0,
		},
		result: []string{
			"gnmic1",
			"gnmic2",
			"gnmic3",
		},
	},
	"test5": {
		input: map[string]int{
			"gnmic1": 3,
			"gnmic2": 2,
			"gnmic3": 1,
		},
		result: []string{
			"gnmic1",
		},
	},
}

func TestGetInstancesTagsMatches(t *testing.T) {
	for name, item := range testSetGetInstancesTagsMatches {
		t.Run(name, func(t *testing.T) {
			res := item.a.getInstancesTagsMatches(item.input)
			t.Logf("exp value: %+v", item.result)
			t.Logf("got value: %+v", res)
			if !cmp.Equal(item.result, res) {
				t.Fail()
			}
		})
	}
}

func TestGetHighestTagsMatches(t *testing.T) {
	a := &App{}
	for name, item := range testSetGetHighestTagsMatches {
		t.Run(name, func(t *testing.T) {
			res := a.getHighestTagsMatches(item.input)
			sort.Strings(res)
			t.Logf("exp value: %+v", item.result)
			t.Logf("got value: %+v", res)
			if !cmp.Equal(item.result, res) {
				t.Fail()
			}
		})
	}
}
