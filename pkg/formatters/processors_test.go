// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"fmt"
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
	"event_fields": {
		condition: `.name == "port-counters" and .timestamp == 42 and .tags.resource_id == "device-1" and .values["/COUNTERS/oid:1/packets"] == 7`,
		input: []*EventMsg{
			{
				Name:      "port-counters",
				Timestamp: 42,
				Tags:      map[string]string{"resource_id": "device-1"},
				Values: map[string]interface{}{
					"/COUNTERS/oid:1/packets": 7,
				},
			},
		},
		result: true,
	},
	"event_fields_no_match": {
		condition: `.name == "port-counters" and .tags.resource_id == "device-2"`,
		input: []*EventMsg{
			{
				Name: "port-counters",
				Tags: map[string]string{"resource_id": "device-1"},
			},
		},
		result: false,
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

func BenchmarkCheckConditionLargeEvent(b *testing.B) {
	query, err := gojq.Parse(`.name == "port-counters"`)
	if err != nil {
		b.Fatal(err)
	}
	condition, err := gojq.Compile(query)
	if err != nil {
		b.Fatal(err)
	}
	values := make(map[string]interface{}, 34_014)
	for index := range 34_014 {
		values[fmt.Sprintf("/COUNTERS/oid:0x%016x/SAI_PORT_STAT_IF_IN_OCTETS", index)] = index
	}
	event := &EventMsg{
		Name:      "port-counters",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"resource_id":       "network-device-001",
			"subscription-name": "port-counters",
		},
		Values: values,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		matched, err := CheckCondition(condition, event)
		if err != nil {
			b.Fatal(err)
		}
		if !matched {
			b.Fatal("condition did not match")
		}
	}
}
