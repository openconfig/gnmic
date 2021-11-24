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
