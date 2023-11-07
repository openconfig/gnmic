// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"testing"

	"github.com/openconfig/gnmic/formatters"
	"github.com/prometheus/prometheus/model/labels"
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
