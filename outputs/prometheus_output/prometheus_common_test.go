package prometheus_output

import (
	"testing"
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
