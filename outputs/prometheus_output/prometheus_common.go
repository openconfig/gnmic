package prometheus_output

import (
	"errors"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
)

const (
	metricNameRegex = "[^a-zA-Z0-9_]+"
)

var (
	MetricNameRegex = regexp.MustCompile(metricNameRegex)
)

type MetricBuilder struct {
	Prefix                 string
	AppendSubscriptionName bool
	StringsAsLabels        bool
}

func (m *MetricBuilder) GetLabels(ev *formatters.EventMsg) []prompb.Label {
	labels := make([]prompb.Label, 0, len(ev.Tags))
	addedLabels := make(map[string]struct{})
	for k, v := range ev.Tags {
		labelName := MetricNameRegex.ReplaceAllString(filepath.Base(k), "_")
		if _, ok := addedLabels[labelName]; ok {
			continue
		}
		labels = append(labels, prompb.Label{Name: labelName, Value: v})
		addedLabels[labelName] = struct{}{}
	}
	if !m.StringsAsLabels {
		return labels
	}

	var err error
	for k, v := range ev.Values {
		_, err = toFloat(v)
		if err == nil {
			continue
		}
		if vs, ok := v.(string); ok {
			labelName := MetricNameRegex.ReplaceAllString(filepath.Base(k), "_")
			if _, ok := addedLabels[labelName]; ok {
				continue
			}
			labels = append(labels, prompb.Label{Name: labelName, Value: vs})
		}
	}
	return labels
}

func toFloat(v interface{}) (float64, error) {
	switch i := v.(type) {
	case float64:
		return float64(i), nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int16:
		return float64(i), nil
	case int8:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint16:
		return float64(i), nil
	case uint8:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return math.NaN(), err
		}
		return f, err
	case *gnmi.Decimal64:
		return float64(i.Digits) / math.Pow10(int(i.Precision)), nil
	default:
		return math.NaN(), errors.New("getFloat: unknown value is of incompatible type")
	}
}

// MetricName generates the prometheus metric name based on the output plugin,
// the measurement name and the value name.
// it makes sure the name matches the regex "[^a-zA-Z0-9_]+"
func (m *MetricBuilder) MetricName(measName, valueName string) string {
	sb := strings.Builder{}
	if m.Prefix != "" {
		sb.WriteString(MetricNameRegex.ReplaceAllString(m.Prefix, "_"))
		sb.WriteString("_")
	}
	if m.AppendSubscriptionName {
		sb.WriteString(strings.TrimRight(MetricNameRegex.ReplaceAllString(measName, "_"), "_"))
		sb.WriteString("_")
	}
	sb.WriteString(strings.TrimLeft(MetricNameRegex.ReplaceAllString(valueName, "_"), "_"))
	return sb.String()
}

type NamedTimeSeries struct {
	Name string
	TS   *prompb.TimeSeries
}

func (m *MetricBuilder) TimeSeriesFromEvent(ev *formatters.EventMsg) []*NamedTimeSeries {
	promTS := make([]*NamedTimeSeries, 0, len(ev.Values))
	tsLabels := m.GetLabels(ev)
	for k, v := range ev.Values {
		fv, err := toFloat(v)
		if err != nil {
			continue
		}
		tsName := m.MetricName(ev.Name, k)
		nts := &NamedTimeSeries{
			Name: tsName,
			TS: &prompb.TimeSeries{
				Labels: append(tsLabels,
					prompb.Label{
						Name:  labels.MetricName,
						Value: m.MetricName(ev.Name, k),
					}),
				Samples: []prompb.Sample{
					{
						Value:     fv,
						Timestamp: ev.Timestamp / int64(time.Millisecond),
					},
				},
			},
		}
		promTS = append(promTS, nts)
	}
	return promTS
}
