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
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
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

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

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
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
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
		v, err := toFloat(val)
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
		labelName := MetricNameRegex.ReplaceAllString(path.Base(k), "_")
		if _, ok := addedLabels[labelName]; ok {
			continue
		}
		labels = append(labels, prompb.Label{Name: labelName, Value: v})
		addedLabels[labelName] = struct{}{}
	}
	if !m.StringsAsLabels {
		return labels
	}
	labelsFromValues := buildUniqueLabelsFromValues(ev.Values, addedLabels)
	labels = append(labels, labelsFromValues...)
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
	case bool:
		if i {
			return 1, nil
		}
		return 0, nil
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
		return math.NaN(), errors.New("toFloat: unknown value is of incompatible type")
	}
}

// MetricName generates the prometheus metric name based on the output plugin,
// the measurement name and the value name.
// it makes sure the name matches the regex "[^a-zA-Z0-9_]+"
func (m *MetricBuilder) MetricName(measName, valueName string) string {
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
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

		// The prometheus spec requires label names to be sorted
		// https://prometheus.io/docs/concepts/remote_write_spec/
		slices.SortFunc(tsLabelsWithName, func(a prompb.Label, b prompb.Label) int {
			return cmp.Compare(a.Name, b.Name)
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

type tempLabel struct {
	path        string // xpath
	name        string // label name
	value       string // label value
	suffixCount int    // suffix count to handle duplicates
}

func labelNameFromPath(path string, numElems int) string {
	elems := strings.Split(path, "/")
	nonEmpty := make([]string, 0, len(elems))
	for _, e := range elems {
		if e != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}
	if numElems > len(nonEmpty) {
		numElems = len(nonEmpty)
	}
	selected := nonEmpty[len(nonEmpty)-numElems:]
	return MetricNameRegex.ReplaceAllString(strings.Join(selected, "_"), "_")
}

func buildUniqueLabelsFromValues(values map[string]any, addedLabels map[string]struct{}) []prompb.Label {
	tempLabels := make([]tempLabel, 0, len(values))
	var err error
	// gather strings and booleans as labels
	for k, v := range values {
		_, err = toFloat(v)
		if err == nil {
			continue
		}
		val := ""
		switch v := v.(type) {
		case string:
			val = v
		case bool:
			val = strconv.FormatBool(v)
		}
		labelName := MetricNameRegex.ReplaceAllString(filepath.Base(k), "_")
		tempLabels = append(tempLabels, tempLabel{
			path:        k,
			name:        labelName,
			value:       val,
			suffixCount: 1,
		})
	}

	// resolve duplicate label names by including more xpath elements
	// from the end of the path until all names are unique and don't
	// collide with already added label tags.
	for {
		groups := make(map[string][]int, len(tempLabels))
		for idx, l := range tempLabels {
			groups[l.name] = append(groups[l.name], idx)
		}

		changed := false
		for name, indices := range groups {
			_, alreadyAdded := addedLabels[name]
			if len(indices) <= 1 && !alreadyAdded {
				continue
			}
			for _, idx := range indices {
				tempLabels[idx].suffixCount++
				newName := labelNameFromPath(tempLabels[idx].path, tempLabels[idx].suffixCount)
				if newName != tempLabels[idx].name {
					tempLabels[idx].name = newName
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}

	// drop any labels that still collide after exhausting path elements.
	taken := make(map[string]struct{}, len(addedLabels)+len(tempLabels))
	for k := range addedLabels {
		taken[k] = struct{}{}
	}
	result := make([]prompb.Label, 0, len(tempLabels))
	for _, l := range tempLabels {
		if _, exists := taken[l.name]; exists {
			continue
		}
		taken[l.name] = struct{}{}
		result = append(result, prompb.Label{Name: l.name, Value: l.value})
	}
	return result
}
