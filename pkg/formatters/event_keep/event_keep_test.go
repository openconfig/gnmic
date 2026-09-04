// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_keep

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/openconfig/gnmic/pkg/formatters"
)

func TestKeepApply(t *testing.T) {
	tests := map[string]struct {
		config map[string]any
		event  *formatters.EventMsg
		want   *formatters.EventMsg
	}{
		"no selectors is a no-op": {
			event: &formatters.EventMsg{
				Values: map[string]any{"a": 1},
				Tags:   map[string]string{"source": "leaf-1"},
			},
			want: &formatters.EventMsg{
				Values: map[string]any{"a": 1},
				Tags:   map[string]string{"source": "leaf-1"},
			},
		},
		"value names retain matching fields": {
			config: map[string]any{"value-names": []string{"^/interfaces/", "^/system/uptime$"}},
			event: &formatters.EventMsg{
				Values: map[string]any{
					"/interfaces/ethernet-1/in-octets": 42,
					"/system/uptime":                   10,
					"/vendor/debug":                    "ignored",
				},
				Tags: map[string]string{"source": "leaf-1"},
			},
			want: &formatters.EventMsg{
				Values: map[string]any{
					"/interfaces/ethernet-1/in-octets": 42,
					"/system/uptime":                   10,
				},
				Tags: map[string]string{"source": "leaf-1"},
			},
		},
		"name and value selectors use OR semantics": {
			config: map[string]any{
				"value-names": []string{"^state$"},
				"values":      []string{"^UP$"},
			},
			event: &formatters.EventMsg{Values: map[string]any{
				"state":  1,
				"status": "UP",
				"count":  2,
			}},
			want: &formatters.EventMsg{Values: map[string]any{
				"state":  1,
				"status": "UP",
			}},
		},
		"value selectors match strings only": {
			config: map[string]any{"values": []string{"^2$"}},
			event: &formatters.EventMsg{Values: map[string]any{
				"string": "2",
				"number": 2,
			}},
			want: &formatters.EventMsg{Values: map[string]any{
				"string": "2",
			}},
		},
		"tag selectors do not filter values": {
			config: map[string]any{
				"tag-names": []string{"^resource_id$"},
				"tags":      []string{"^leaf-"},
			},
			event: &formatters.EventMsg{
				Values: map[string]any{"state": 1},
				Tags: map[string]string{
					"resource_id": "resource-1",
					"source":      "leaf-1",
					"internal":    "drop",
				},
			},
			want: &formatters.EventMsg{
				Values: map[string]any{"state": 1},
				Tags: map[string]string{
					"resource_id": "resource-1",
					"source":      "leaf-1",
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			processor := &keep{}
			if err := processor.Init(test.config); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			got := processor.Apply(test.event)
			if len(got) != 1 || !reflect.DeepEqual(got[0], test.want) {
				t.Fatalf("Apply() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestKeepApplyNil(t *testing.T) {
	processor := &keep{}
	if err := processor.Init(map[string]any{"value-names": []string{".*"}}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if got := processor.Apply(nil); len(got) != 1 || got[0] != nil {
		t.Fatalf("Apply(nil) = %#v, want [nil]", got)
	}
}

func TestKeepApplyFiltersEmptyEvents(t *testing.T) {
	processor := &keep{}
	if err := processor.Init(map[string]any{
		"value-names": []string{"^keep$"},
		"tag-names":   []string{"^keep$"},
	}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	empty := &formatters.EventMsg{
		Values: map[string]any{"drop": 1},
		Tags:   map[string]string{"drop": "tag"},
	}
	deleted := &formatters.EventMsg{
		Tags:    map[string]string{"drop": "tag"},
		Deletes: []string{"/interfaces/interface[name=ethernet-1]"},
	}
	retained := &formatters.EventMsg{
		Values: map[string]any{"keep": 1},
		Tags:   map[string]string{"drop": "tag"},
	}

	got := processor.Apply(empty, deleted, retained)
	want := []*formatters.EventMsg{deleted, retained}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Apply() = %#v, want %#v", got, want)
	}
}

func TestKeepInitRejectsInvalidPattern(t *testing.T) {
	processor := &keep{}
	err := processor.Init(map[string]any{"value-names": []string{"["}})
	if err == nil {
		t.Fatal("Init() error = nil, want invalid regular expression")
	}
}

func BenchmarkKeepWideEventValueNames(b *testing.B) {
	processor := &keep{}
	if err := processor.Init(map[string]any{
		"value-names": []string{`^/COUNTERS/[^/]+/(SAI_PORT_STAT_IF_IN_OCTETS|SAI_PORT_STAT_IF_OUT_OCTETS)$`},
	}); err != nil {
		b.Fatal(err)
	}
	template := make(map[string]any, 34_014)
	for i := range 34_012 {
		template["/COUNTERS/oid:0x1/VENDOR_FIELD_"+strconv.Itoa(i)] = i
	}
	template["/COUNTERS/oid:0x1/SAI_PORT_STAT_IF_IN_OCTETS"] = 1
	template["/COUNTERS/oid:0x1/SAI_PORT_STAT_IF_OUT_OCTETS"] = 2

	b.ReportAllocs()
	for b.Loop() {
		values := make(map[string]any, len(template))
		for key, value := range template {
			values[key] = value
		}
		processor.Apply(&formatters.EventMsg{Values: values})
	}
}
