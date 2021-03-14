package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testItem struct {
	in  string
	out []string
}

var promptArgsTestSet = map[string]testItem{
	"no_args": {
		in:  ``,
		out: []string{""},
	},
	"one_arg": {
		in:  `arg`,
		out: []string{"arg"},
	},
	"multiple_args": {
		in:  `arg1 arg2 --flag1 val1`,
		out: []string{"arg1", "arg2", "--flag1", "val1"},
	},
	"single_quoted_args": {
		in:  `arg1 arg2 --flag1 'val 1'`,
		out: []string{"arg1", "arg2", "--flag1", "val 1"},
	},
	"double_quoted_args": {
		in:  `arg1 arg2 --flag1 "val 1"`,
		out: []string{"arg1", "arg2", "--flag1", "val 1"},
	},
	"quoted_args_with_multiple_spaces": {
		in:  `arg1 arg2 --flag1 "val 1" --flag2 "val  \t2"`,
		out: []string{"arg1", "arg2", "--flag1", "val 1", "--flag2", `val  \t2`},
	},
	"quoted_args_with_spaces_between_items": {
		in:  `      arg1 arg2       --flag1 'val 1'      --flag2 "val 2"             `,
		out: []string{"arg1", "arg2", "--flag1", "val 1", "--flag2", `val 2`},
	},
}

func TestGetInstancesTagsMatches(t *testing.T) {
	for name, item := range promptArgsTestSet {
		t.Run(name, func(t *testing.T) {
			res, err := parsePromptArgs(item.in)
			if err != nil {
				t.Logf("failed: %v", err)
				t.Fail()
			}
			t.Logf("exp value: %#v", item.out)
			t.Logf("got value: %#v", res)
			if !cmp.Equal(item.out, res) {
				t.Fail()
			}
		})
	}
}
