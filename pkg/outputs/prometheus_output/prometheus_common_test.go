// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"cmp"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
)

var metricNameSet = map[string]struct {
	p         *MetricBuilder
	measName  string // aka subscription name
	valueName string
	want      string
}{
	"with_prefix_with_subscription_with_value_no-append-subsc": {
		p: &MetricBuilder{
			Prefix: "gnmic",
		},
		measName:  "sub",
		valueName: "value",
		want:      "gnmic_value",
	},
	"with_prefix_with_subscription_with_value_with_append-subsc": {
		p: &MetricBuilder{
			Prefix:                 "gnmic",
			AppendSubscriptionName: true,
		},
		measName:  "sub",
		valueName: "value",
		want:      "gnmic_sub_value",
	},
	"with_prefix-bad-chars_with_subscription_with_value_with_append-subsc": {
		p: &MetricBuilder{
			Prefix:                 "gnmic-prefix",
			AppendSubscriptionName: true,
		},

		measName:  "sub",
		valueName: "value",
		want:      "gnmic_prefix_sub_value",
	},
	"without_prefix_with_subscription_with_value_no-append-subsc": {
		p:         &MetricBuilder{},
		measName:  "sub",
		valueName: "value",
		want:      "value",
	},
	"without_prefix_with_subscription_with_value_with_append-subsc": {
		p: &MetricBuilder{
			AppendSubscriptionName: true,
		},
		measName:  "sub",
		valueName: "value",
		want:      "sub_value",
	},
	"without_prefix_with_subscription-bad-chars_with_value-bad-chars_with_append-subsc": {
		p: &MetricBuilder{
			AppendSubscriptionName: true,
		},
		measName:  "sub-name",
		valueName: "value-name2",
		want:      "sub_name_value_name2",
	},
}

func TestTimeSeriesFromEvent(t *testing.T) {
	metricBuilder := &MetricBuilder{StringsAsLabels: true}
	event := &formatters.EventMsg{
		Name:      "eventName",
		Timestamp: 12345,
		Tags: map[string]string{
			"tagName": "tagVal",
		},
		Values: map[string]interface{}{
			"strName1": "strVal1",
			"strName2": "strVal2",
			"intName1": 1,
			"intName2": 2,
		},
		Deletes: []string{},
	}
	for _, nts := range metricBuilder.TimeSeriesFromEvent(event) {
		for _, label := range nts.TS.Labels {
			if label.Name == labels.MetricName && label.Value != nts.Name {
				t.Errorf("__name__ label wrong, expected '%s', got '%s'", nts.Name, label.Value)
			}
		}
	}
}

func TestTimeSeriesLabelsSorted(t *testing.T) {
	metricBuilder := &MetricBuilder{StringsAsLabels: true}
	event := &formatters.EventMsg{
		Name:      "eventName",
		Timestamp: 12345,
		Tags: map[string]string{
			"tagName": "tagVal",
		},
		Values: map[string]interface{}{
			"strName1": "strVal1",
			"strName2": "strVal2",
			"intName1": 1,
			"intName2": 2,
		},
		Deletes: []string{},
	}
	for _, nts := range metricBuilder.TimeSeriesFromEvent(event) {
		areLabelsSorted := slices.IsSortedFunc(nts.TS.Labels, func(a prompb.Label, b prompb.Label) int {
			return cmp.Compare(a.Name, b.Name)
		})
		if !areLabelsSorted {
			t.Errorf("labels names are not sorted, got '%v'", nts.TS.Labels)
		}
	}
}

func TestMetricName(t *testing.T) {
	for name, tc := range metricNameSet {
		t.Run(name, func(t *testing.T) {
			got := tc.p.MetricName(tc.measName, tc.valueName)
			if got != tc.want {
				t.Errorf("failed at '%s', expected %v, got %+v", name, tc.want, got)
			}
		})
	}
}

func BenchmarkMetricName(b *testing.B) {
	for name, tc := range metricNameSet {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.p.MetricName(tc.measName, tc.valueName)
			}
		})
	}
}

func Test_buildUniqueLabelsFromValues(t *testing.T) {
	tests := []struct {
		name        string
		values      map[string]any
		addedLabels map[string]struct{}
		want        []prompb.Label
	}{
		{
			name: "no_duplicates",
			values: map[string]any{
				"a/b/c": "a",
				"a/b/d": "b",
				"a/b/e": "c",
			},
			want: []prompb.Label{
				{Name: "c", Value: "a"},
				{Name: "d", Value: "b"},
				{Name: "e", Value: "c"},
			},
		},
		{
			name: "with_duplicates",
			values: map[string]any{
				"a/a/name": "a",
				"a/b/name": "b",
				"a/c/name": "c",
			},
			want: []prompb.Label{
				{Name: "a_name", Value: "a"},
				{Name: "b_name", Value: "b"},
				{Name: "c_name", Value: "c"},
			},
		},
		{
			name: "with_duplicates_3_elements",
			values: map[string]any{
				"a/a/name": "a",
				"b/a/name": "b",
				"c/a/name": "c",
			},
			want: []prompb.Label{
				{Name: "a_a_name", Value: "a"},
				{Name: "b_a_name", Value: "b"},
				{Name: "c_a_name", Value: "c"},
			},
		},
		{
			name: "with_duplicates_and_floats",
			values: map[string]any{
				"a/a/name": "a",
				"a/b/name": "b",
				"a/c/name": "1",
			},
			want: []prompb.Label{
				{Name: "a_name", Value: "a"},
				{Name: "b_name", Value: "b"},
			},
		},
		{
			name: "collision_with_added_labels",
			values: map[string]any{
				"a/b/name": "val",
			},
			addedLabels: map[string]struct{}{
				"name": {},
			},
			want: []prompb.Label{
				{Name: "b_name", Value: "val"},
			},
		},
		{
			name: "collision_with_added_labels_and_duplicates",
			values: map[string]any{
				"a/b/name": "v1",
				"a/c/name": "v2",
			},
			addedLabels: map[string]struct{}{
				"name": {},
			},
			want: []prompb.Label{
				{Name: "b_name", Value: "v1"},
				{Name: "c_name", Value: "v2"},
			},
		},
		{
			name: "collision_with_added_labels_and_duplicates_2",
			values: map[string]any{
				"a/b/name": "v1",
				"a/c/name": "v2",
			},
			addedLabels: map[string]struct{}{
				"b_name": {},
			},
			want: []prompb.Label{
				{Name: "a_b_name", Value: "v1"},
				{Name: "c_name", Value: "v2"},
			},
		},
		{
			name: "collision_with_added_labels_full_path_exhausted",
			values: map[string]any{
				"a/b/name": "val",
			},
			addedLabels: map[string]struct{}{
				"name":     {},
				"b_name":   {},
				"a_b_name": {},
			},
			want: []prompb.Label{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addedLabels := tt.addedLabels
			if addedLabels == nil {
				addedLabels = make(map[string]struct{})
			}
			got := buildUniqueLabelsFromValues(tt.values, addedLabels)
			if len(got) != len(tt.want) {
				t.Errorf("buildUniqueLabelsFromValues() length = %d, want %d", len(got), len(tt.want))
				return
			}
			sort.Slice(got, func(i, j int) bool {
				return got[i].Name < got[j].Name
			})
			sort.Slice(tt.want, func(i, j int) bool {
				return tt.want[i].Name < tt.want[j].Name
			})
			for i, label := range got {
				expected := tt.want[i]
				if label.Name != expected.Name || label.Value != expected.Value {
					t.Errorf("Label mismatch at index %d: got %+v, want %+v", i, label, expected)
				}
			}
		})
	}
}

func TestMetricBuilder_MetricsFromEvent(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		ev   *formatters.EventMsg
		now  time.Time
		want []*PromMetric
	}{
		{
			name: "no_duplicates",
			ev: &formatters.EventMsg{
				Name:      "eventName",
				Timestamp: 42,
				Tags: map[string]string{
					"t1": "v1",
					"t2": "v2",
				},
				Values: map[string]any{
					"a/b/c": "1",
				},
			},
			now: time.Unix(0, 42),
			want: []*PromMetric{
				{
					Name:  "a_b_c",
					value: 1,
					labels: []prompb.Label{
						{Name: "t1", Value: "v1"},
						{Name: "t2", Value: "v2"},
					},
				},
			},
		},
		{
			name: "no_duplicates_strings_as_labels",
			ev: &formatters.EventMsg{
				Name:      "eventName",
				Timestamp: 42,
				Tags: map[string]string{
					"t1": "v1",
					"t2": "v2",
				},
				Values: map[string]any{
					"a/b/c": "a",
				},
			},
			now: time.Unix(0, 42),
			want: []*PromMetric{
				{
					Name:  "a_b_c",
					value: 1,
					labels: []prompb.Label{
						{Name: "t1", Value: "v1"},
						{Name: "t2", Value: "v2"},
						{Name: "c", Value: "a"},
					},
				},
			},
		},
		{
			name: "duplicates_strings_as_labels",
			ev: &formatters.EventMsg{
				Name:      "eventName",
				Timestamp: 42,
				Tags: map[string]string{
					"t1": "v1",
					"t2": "v2",
				},
				Values: map[string]any{
					"a/a/c": "a",
					"a/b/c": "b",
				},
			},
			now: time.Unix(0, 42),
			want: []*PromMetric{
				{
					Name:  "a_a_c",
					value: 1,
					labels: []prompb.Label{
						{Name: "t1", Value: "v1"},
						{Name: "t2", Value: "v2"},
						{Name: "a_c", Value: "a"},
						{Name: "b_c", Value: "b"},
					},
				},
				{
					Name:  "a_b_c",
					value: 1,
					labels: []prompb.Label{
						{Name: "t1", Value: "v1"},
						{Name: "t2", Value: "v2"},
						{Name: "a_c", Value: "a"},
						{Name: "b_c", Value: "b"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := &MetricBuilder{
				StringsAsLabels: true,
			}
			got := mb.MetricsFromEvent(tt.ev, tt.now)
			if len(got) != len(tt.want) {
				t.Errorf("MetricsFromEvent() = %v, want %v", got, tt.want)
			}
			sort.Slice(got, func(i, j int) bool {
				return got[i].Name < got[j].Name
			})
			sort.Slice(tt.want, func(i, j int) bool {
				return tt.want[i].Name < tt.want[j].Name
			})
			for i, pm := range got {
				expected := tt.want[i]
				if pm.Name != expected.Name || pm.value != expected.value {
					t.Errorf("Metric mismatch at index %d: got %+v, want %+v", i, pm, expected)
				}
				if len(pm.labels) != len(expected.labels) {
					t.Errorf("Metric labels mismatch at index %d: got %+v, want %+v", i, pm.labels, expected.labels)
				}
				sort.Slice(pm.labels, func(i, j int) bool {
					return pm.labels[i].Name < pm.labels[j].Name
				})
				sort.Slice(expected.labels, func(i, j int) bool {
					return expected.labels[i].Name < expected.labels[j].Name
				})
				for j, label := range pm.labels {
					expectedLabel := expected.labels[j]
					if label.Name != expectedLabel.Name || label.Value != expectedLabel.Value {
						t.Errorf("Metric label mismatch at index %d: got %+v, want %+v", j, label, expectedLabel)
					}
				}
			}
		})
	}
}
