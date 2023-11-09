// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

var collapseTestSet = map[string][]string{
	"1": {
		"",
		"/",
	},
	"2": {
		"/prefix1:elem1[key1=*]/prefix1:elem2/prefix2:elem3/prefix2:elem4",
		"/prefix1:elem1[key1=*]/elem2/prefix2:elem3/elem4",
	},
	"3": {
		"/prefix1:elem1[key1=*]/prefix1:elem2/prefix2:elem3/prefix2:elem4",
		"/prefix1:elem1[key1=*]/elem2/prefix2:elem3/elem4",
	},
	"4": {
		"/fake_prefix:",
		"/fake_prefix:",
	},
	"5": {
		"/:fake_prefix",
		"/:fake_prefix",
	},
	"6": {
		"/elem1/prefix1:elem2/prefix1:elem3",
		"/elem1/prefix1:elem2/elem3",
	},
}

func TestCollapsePrefixes(t *testing.T) {
	for name, item := range collapseTestSet {
		t.Run(name, func(t *testing.T) {
			r := collapsePrefixes(item[0])
			if r != item[1] {
				t.Logf("failed at item %q", name)
				t.Logf("expected: %q", item[1])
				t.Logf("	 got: %q", r)
				t.Fail()
			}
		})
	}
}
