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
	"strings"
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
		"structured value paths retain exact segments": {
			config: map[string]any{"value-name-paths": []string{
				"/interfaces/*/in-octets",
				"/system/uptime",
			}},
			event: &formatters.EventMsg{Values: map[string]any{
				"/interfaces/ethernet-1/in-octets":       42,
				"/interfaces/ethernet-1/state/in-octets": 24,
				"/interfaces/in-octets":                  12,
				"/system/uptime":                         10,
				"/vendor/debug":                          "ignored",
			}},
			want: &formatters.EventMsg{Values: map[string]any{
				"/interfaces/ethernet-1/in-octets": 42,
				"/system/uptime":                   10,
			}},
		},
		"path and regular expression selectors use OR semantics": {
			config: map[string]any{
				"value-name-paths": []string{"/interfaces/*/in-octets"},
				"value-names":      []string{"^state$"},
			},
			event: &formatters.EventMsg{Values: map[string]any{
				"/interfaces/ethernet-1/in-octets": 42,
				"state":                            1,
				"drop":                             2,
			}},
			want: &formatters.EventMsg{Values: map[string]any{
				"/interfaces/ethernet-1/in-octets": 42,
				"state":                            1,
			}},
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

func TestKeepInitRejectsInvalidValueNamePath(t *testing.T) {
	for _, selector := range []string{"", "relative/path", "/", "/interfaces//state", "/interfaces/state/"} {
		t.Run(selector, func(t *testing.T) {
			processor := &keep{}
			err := processor.Init(map[string]any{"value-name-paths": []string{selector}})
			if err == nil {
				t.Fatalf("Init() error = nil, want invalid selector %q", selector)
			}
		})
	}
}

func BenchmarkKeepWideEventValueNames(b *testing.B) {
	benchmarkKeepWideEvent(b, map[string]any{
		"value-names": []string{`^/COUNTERS/[^/]+/(SAI_PORT_STAT_IF_IN_OCTETS|SAI_PORT_STAT_IF_OUT_OCTETS)$`},
	}, []string{"SAI_PORT_STAT_IF_IN_OCTETS", "SAI_PORT_STAT_IF_OUT_OCTETS"})
}

func BenchmarkKeepWideEventValueNamePaths(b *testing.B) {
	benchmarkKeepWideEvent(b, map[string]any{
		"value-name-paths": []string{
			"/COUNTERS/*/SAI_PORT_STAT_IF_IN_OCTETS",
			"/COUNTERS/*/SAI_PORT_STAT_IF_OUT_OCTETS",
		},
	}, []string{"SAI_PORT_STAT_IF_IN_OCTETS", "SAI_PORT_STAT_IF_OUT_OCTETS"})
}

func BenchmarkKeepWideEventManyFields(b *testing.B) {
	fields := make([]string, 64)
	paths := make([]string, 64)
	for i := range fields {
		fields[i] = "SAI_PORT_STAT_FIELD_" + strconv.Itoa(i)
		paths[i] = "/COUNTERS/*/" + fields[i]
	}
	b.Run("regular expressions", func(b *testing.B) {
		benchmarkKeepWideEvent(b, map[string]any{
			"value-names": []string{`^/COUNTERS/[^/]+/(` + strings.Join(fields, "|") + `)$`},
		}, fields)
	})
	b.Run("structured paths", func(b *testing.B) {
		benchmarkKeepWideEvent(b, map[string]any{"value-name-paths": paths}, fields)
	})
}

func benchmarkKeepWideEvent(b *testing.B, config map[string]any, retained []string) {
	processor := &keep{}
	if err := processor.Init(config); err != nil {
		b.Fatal(err)
	}
	template := make(map[string]any, 34_000+len(retained))
	for i := range 34_000 {
		template["/COUNTERS/oid:0x1/VENDOR_FIELD_"+strconv.Itoa(i)] = i
	}
	for i, field := range retained {
		template["/COUNTERS/oid:0x1/"+field] = i
	}

	b.ReportAllocs()
	for b.Loop() {
		values := make(map[string]any, len(template))
		for key, value := range template {
			values[key] = value
		}
		processor.Apply(&formatters.EventMsg{Values: values})
	}
}
