// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
)

const (
	metricNameRegex   = "[^a-zA-Z0-9_]+"
	defaultMetricHelp = "gNMIc generated metric"
)

var (
	MetricNameRegex = regexp.MustCompile(metricNameRegex)
)

type PromMetric struct {
	Name string
	Time *time.Time
	// AddedAt is used to expire metrics if the time field is not initialized
	// this happens when ExportTimestamp == false
	AddedAt time.Time

	labels []prompb.Label
	value  float64
}

// Metric
func (p *PromMetric) CalculateKey() uint64 {
	h := fnv.New64a()
	h.Write([]byte(p.Name))
	if len(p.labels) > 0 {
		h.Write([]byte(":"))
		sort.Slice(p.labels, func(i, j int) bool {
			return p.labels[i].Name < p.labels[j].Name
		})
		for _, label := range p.labels {
			h.Write([]byte(label.Name))
			h.Write([]byte(":"))
			h.Write([]byte(label.Value))
			h.Write([]byte(":"))
		}
	}
	return h.Sum64()
}

func (p *PromMetric) String() string {
	if p == nil {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString("name=")
	sb.WriteString(p.Name)
	sb.WriteString(",")
	numLabels := len(p.labels)
	if numLabels > 0 {
		sb.WriteString("labels=[")
		for i, lb := range p.labels {
			sb.WriteString(lb.Name)
			sb.WriteString("=")
			sb.WriteString(lb.Value)
			if i < numLabels-1 {
				sb.WriteString(",")
			}
		}
		sb.WriteString("],")
	}
	sb.WriteString(fmt.Sprintf("value=%f,", p.value))
	sb.WriteString("time=")
	if p.Time != nil {
		sb.WriteString(p.Time.String())
	} else {
		sb.WriteString("nil")
	}
	sb.WriteString(",addedAt=")
	sb.WriteString(p.AddedAt.String())
	return sb.String()
}

// Desc implements prometheus.Metric
func (p *PromMetric) Desc() *prometheus.Desc {
	labelNames := make([]string, 0, len(p.labels))
	for _, label := range p.labels {
		labelNames = append(labelNames, label.Name)
	}

	return prometheus.NewDesc(p.Name, defaultMetricHelp, labelNames, nil)
}

// Write implements prometheus.Metric
func (p *PromMetric) Write(out *dto.Metric) error {
	out.Untyped = &dto.Untyped{
		Value: &p.value,
	}
	out.Label = make([]*dto.LabelPair, 0, len(p.labels))
	for i := range p.labels {
		out.Label = append(out.Label, &dto.LabelPair{Name: &p.labels[i].Name, Value: &p.labels[i].Value})
	}
	if p.Time == nil {
		return nil
	}
	timestamp := p.Time.UnixNano() / 1000000
	out.TimestampMs = &timestamp
	return nil
}

func (mb *MetricBuilder) MetricsFromEvent(ev *formatters.EventMsg, now time.Time) []*PromMetric {
	pms := make([]*PromMetric, 0, len(ev.Values))
	labels := mb.GetLabels(ev)
	for vName, val := range ev.Values {
		v, err := getFloat(val)
		if err != nil {
			if !mb.StringsAsLabels {
				continue
			}
			v = 1.0
		}
		pm := &PromMetric{
			Name:    mb.MetricName(ev.Name, vName),
			labels:  labels,
			value:   v,
			AddedAt: now,
		}
		if mb.OverrideTimestamps && mb.ExportTimestamps {
			ev.Timestamp = now.UnixNano()
		}
		if mb.ExportTimestamps {
			tm := time.Unix(0, ev.Timestamp)
			pm.Time = &tm
		}
		pms = append(pms, pm)
	}
	return pms
}

type MetricBuilder struct {
	Prefix                 string
	AppendSubscriptionName bool
	StringsAsLabels        bool
	OverrideTimestamps     bool
	ExportTimestamps       bool
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
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
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
	timestamp := ev.Timestamp / int64(time.Millisecond)
	for k, v := range ev.Values {
		fv, err := toFloat(v)
		if err != nil {
			if !m.StringsAsLabels {
				continue
			}
			fv = 1.0
		}
		tsName := m.MetricName(ev.Name, k)
		tsLabelsWithName := make([]prompb.Label, 0, len(tsLabels)+1)
		tsLabelsWithName = append(tsLabelsWithName, tsLabels...)
		tsLabelsWithName = append(tsLabelsWithName,
			prompb.Label{
				Name:  labels.MetricName,
				Value: tsName,
			})
		nts := &NamedTimeSeries{
			Name: tsName,
			TS: &prompb.TimeSeries{
				Labels: tsLabelsWithName,
				Samples: []prompb.Sample{
					{
						Value:     fv,
						Timestamp: timestamp,
					},
				},
			},
		}
		promTS = append(promTS, nts)
	}
	return promTS
}

func getFloat(v interface{}) (float64, error) {
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
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
	case *gnmi.Decimal64:
		return float64(i.Digits) / math.Pow10(int(i.Precision)), nil
	default:
		return math.NaN(), errors.New("getFloat: unknown value is of incompatible type")
	}
}
