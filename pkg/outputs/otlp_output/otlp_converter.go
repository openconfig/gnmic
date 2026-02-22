// © 2025 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/version"
)

var stringsBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// convertToOTLP converts gNMI EventMsg slice to OTLP ExportMetricsServiceRequest
func (o *otlpOutput) convertToOTLP(events []*formatters.EventMsg) *metricsv1.ExportMetricsServiceRequest {
	cfg := o.cfg.Load()

	if cfg.Debug {
		o.logger.Printf("DEBUG: convertToOTLP called with %d events", len(events))
	}

	// Group events by resource (source)
	resourceGroups := o.groupByResource(events)

	if cfg.Debug {
		o.logger.Printf("DEBUG: Grouped into %d resource groups", len(resourceGroups))
	}

	req := &metricsv1.ExportMetricsServiceRequest{
		ResourceMetrics: make([]*metricspb.ResourceMetrics, 0, len(resourceGroups)),
	}

	totalMetrics := 0
	skippedEvents := 0

	for _, groupedEvents := range resourceGroups {
		rm := &metricspb.ResourceMetrics{
			Resource: o.createResource(cfg, groupedEvents[0]),
			ScopeMetrics: []*metricspb.ScopeMetrics{
				{
					Scope: &commonpb.InstrumentationScope{
						Name:    "gNMIc",
						Version: version.Version,
					},
					Metrics: make([]*metricspb.Metric, 0, len(groupedEvents)),
				},
			},
		}

		// Convert each event to OTLP metric
		for _, event := range groupedEvents {
			metrics, err := o.convertEventToMetrics(cfg, event)
			if err != nil {
				if cfg.Debug {
					o.logger.Printf("DEBUG: failed to convert event %s: %v", event.Name, err)
				}
				skippedEvents++
				continue
			}
			if len(metrics) == 0 {
				if cfg.Debug {
					o.logger.Printf("DEBUG: convertEvent returned nil for event: name=%s, values=%v", event.Name, event.Values)
				}
				skippedEvents++
				continue
			}
			rm.ScopeMetrics[0].Metrics = append(rm.ScopeMetrics[0].Metrics, metrics...)
			totalMetrics += len(metrics)
		}

		if len(rm.ScopeMetrics[0].Metrics) > 0 {
			req.ResourceMetrics = append(req.ResourceMetrics, rm)
		}
	}

	if cfg.Debug {
		o.logger.Printf("DEBUG: Converted %d metrics, skipped %d events, %d ResourceMetrics",
			totalMetrics, skippedEvents, len(req.ResourceMetrics))
	}

	return req
}

// groupByResource groups events by their source (device) for resource attribution
func (o *otlpOutput) groupByResource(events []*formatters.EventMsg) map[string][]*formatters.EventMsg {
	groups := make(map[string][]*formatters.EventMsg)

	for _, event := range events {
		// Use source as resource key
		source := event.Tags["source"]
		if source == "" {
			source = "unknown"
		}
		groups[source] = append(groups[source], event)
	}

	return groups
}

// createResource creates OTLP Resource from event metadata.
// Tags listed in cfg.ResourceTagKeys are placed as resource attributes.
func (o *otlpOutput) createResource(cfg *config, event *formatters.EventMsg) *resourcepb.Resource {
	attrs := make([]*commonpb.KeyValue, 0, len(cfg.ResourceTagKeys)+len(cfg.ResourceAttributes))

	for _, key := range cfg.ResourceTagKeys {
		if val, ok := event.Tags[key]; ok {
			attrs = append(attrs, &commonpb.KeyValue{
				Key:   key,
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
			})
		}
	}

	for key, val := range cfg.ResourceAttributes {
		attrs = append(attrs, &commonpb.KeyValue{
			Key:   key,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
		})
	}

	return &resourcepb.Resource{
		Attributes: attrs,
	}
}

// convertEventToMetrics converts a single gNMI event to OTLP metrics.
// Returns nil if the event has no valid values to convert.
func (o *otlpOutput) convertEventToMetrics(cfg *config, event *formatters.EventMsg) ([]*metricspb.Metric, error) {
	if len(event.Values) == 0 {
		if cfg.Debug {
			o.logger.Printf("DEBUG: event has no values (event: %s)", event.Name)
		}
		return nil, nil
	}

	attributes := o.extractAttributesForMetric(cfg, event)

	result := make([]*metricspb.Metric, 0, len(event.Values))
	for k, v := range event.Values {
		metricName := o.buildMetricName(cfg, event, k)

		metric := &metricspb.Metric{
			Name: metricName,
		}

		// Handle string values
		switch v := v.(type) {
		case string:
			if !cfg.StringsAsAttributes {
				if cfg.Debug {
					o.logger.Printf("DEBUG: skipping string value (strings-as-attributes=false): %s", event.Name)
				}
				continue
			}
			metric.Data = &metricspb.Metric_Gauge{
				Gauge: o.createGaugeWithString(event, attributes, v),
			}
			result = append(result, metric)
			continue
		}

		dataPoint := o.createNumberDataPointWithValue(cfg, event, attributes, v)
		if dataPoint == nil {
			if cfg.Debug {
				o.logger.Printf("DEBUG: failed to create data point for value type %T (event: %s)", v, event.Name)
			}
			continue
		}

		if o.isCounter(cfg, k) {
			metric.Data = &metricspb.Metric_Sum{
				Sum: &metricspb.Sum{
					AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
					IsMonotonic:            true,
					DataPoints:             []*metricspb.NumberDataPoint{dataPoint},
				},
			}
		} else {
			metric.Data = &metricspb.Metric_Gauge{
				Gauge: &metricspb.Gauge{
					DataPoints: []*metricspb.NumberDataPoint{dataPoint},
				},
			}
		}
		result = append(result, metric)
	}

	return result, nil
}

// buildMetricName creates metric name from event and value key
// event.Name contains the subscription name (e.g., "nvos", "arista")
// valueKey contains the metric path (e.g., "interfaces/interface/state/counters/in-octets")
func (o *otlpOutput) buildMetricName(cfg *config, event *formatters.EventMsg, valueKey string) string {
	sb := stringsBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringsBuilderPool.Put(sb)
	}()

	// Add global prefix if configured
	if cfg.MetricPrefix != "" {
		sb.WriteString(cfg.MetricPrefix)
		sb.WriteString("_")
	}

	// Append subscription name if configured (for vendor-specific prefixes)
	if cfg.AppendSubscriptionName {
		sb.WriteString(event.Name) // subscription name (nvos, arista, etc.)
		sb.WriteString("_")
	}

	// Append the value key (metric path), converting slashes to underscores
	// e.g., "interfaces/interface/state/counters/in-octets" -> "interfaces_interface_state_counters_in_octets"
	metricPath := strings.ReplaceAll(valueKey, "/", "_")
	sb.WriteString(metricPath)

	name := sb.String()

	// Replace remaining hyphens with underscores (Prometheus convention)
	name = strings.ReplaceAll(name, "-", "_")

	return name
}

// extractAttributesForMetric extracts data point attributes from event tags.
// Tags listed in cfg.ResourceTagKeys are excluded (they live on the Resource).
func (o *otlpOutput) extractAttributesForMetric(cfg *config, event *formatters.EventMsg) []*commonpb.KeyValue {
	attrs := make([]*commonpb.KeyValue, 0, len(event.Tags))

	for key, val := range event.Tags {
		if cfg.resourceTagSet[key] {
			continue
		}
		attrs = append(attrs, &commonpb.KeyValue{
			Key:   key,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
		})
	}

	return attrs
}

// isCounter returns true if any of the configured counter-patterns match the value key.
func (o *otlpOutput) isCounter(cfg *config, valueName string) bool {
	for _, re := range cfg.counterRegexes {
		if re.MatchString(valueName) {
			return true
		}
	}
	return false
}

// createNumberDataPointWithValue creates OTLP data point from event with a specific value
func (o *otlpOutput) createNumberDataPointWithValue(cfg *config, event *formatters.EventMsg, attrs []*commonpb.KeyValue, value interface{}) *metricspb.NumberDataPoint {
	dp := &metricspb.NumberDataPoint{
		Attributes:   attrs,
		TimeUnixNano: uint64(event.Timestamp),
	}

	// Handle value conversion
	switch v := value.(type) {
	case int:
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: int64(v)}
	case int32:
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: int64(v)}
	case int64:
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: v}
	case uint:
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: int64(v)}
	case uint32:
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: int64(v)}
	case uint64:
		// Handle potential overflow
		maxInt64 := uint64(9223372036854775807) // math.MaxInt64
		if v > maxInt64 {
			// Convert to double if too large for int64
			dp.Value = &metricspb.NumberDataPoint_AsDouble{AsDouble: float64(v)}
		} else {
			dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: int64(v)}
		}
	case float32:
		dp.Value = &metricspb.NumberDataPoint_AsDouble{AsDouble: float64(v)}
	case float64:
		dp.Value = &metricspb.NumberDataPoint_AsDouble{AsDouble: v}
	case string:
		// Try to parse as number
		if fVal, err := strconv.ParseFloat(v, 64); err == nil {
			dp.Value = &metricspb.NumberDataPoint_AsDouble{AsDouble: fVal}
		} else {
			return nil
		}
	default:
		if cfg.Debug {
			o.logger.Printf("unsupported value type %T for metric %s", v, event.Name)
		}
		return nil
	}

	return dp
}

// createGaugeWithString creates OTLP Gauge with string value as attribute.
// It copies attrs to avoid mutating the caller's shared slice.
func (o *otlpOutput) createGaugeWithString(event *formatters.EventMsg, attrs []*commonpb.KeyValue, strVal string) *metricspb.Gauge {
	dpAttrs := make([]*commonpb.KeyValue, len(attrs), len(attrs)+1)
	copy(dpAttrs, attrs)
	dpAttrs = append(dpAttrs, &commonpb.KeyValue{
		Key:   "value",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: strVal}},
	})

	dp := &metricspb.NumberDataPoint{
		Attributes:   dpAttrs,
		TimeUnixNano: uint64(event.Timestamp),
		Value:        &metricspb.NumberDataPoint_AsDouble{AsDouble: 1.0},
	}

	return &metricspb.Gauge{
		DataPoints: []*metricspb.NumberDataPoint{dp},
	}
}

// validateRequest validates OTLP request structure
func (o *otlpOutput) validateRequest(req *metricsv1.ExportMetricsServiceRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if len(req.ResourceMetrics) == 0 {
		return fmt.Errorf("ResourceMetrics is empty")
	}

	for i, rm := range req.ResourceMetrics {
		if rm.Resource == nil {
			return fmt.Errorf("ResourceMetrics[%d].Resource is nil", i)
		}

		if len(rm.ScopeMetrics) == 0 {
			return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics is empty", i)
		}

		for j, sm := range rm.ScopeMetrics {
			if sm.Scope == nil {
				return fmt.Errorf("ScopeMetrics[%d].Scope is nil", j)
			}

			if len(sm.Metrics) == 0 {
				return fmt.Errorf("ScopeMetrics[%d].Metrics is empty", j)
			}

			for k, m := range sm.Metrics {
				if m.Name == "" {
					return fmt.Errorf("Metric[%d].Name is empty", k)
				}

				if m.Data == nil {
					return fmt.Errorf("Metric[%d].Data is nil", k)
				}

				if err := o.validateMetricData(i, j, k, m); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateMetricData validates metric data points
func (o *otlpOutput) validateMetricData(rmIdx, smIdx, mIdx int, m *metricspb.Metric) error {
	var dataPoints []*metricspb.NumberDataPoint

	switch data := m.Data.(type) {
	case *metricspb.Metric_Gauge:
		if data.Gauge == nil {
			return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d].Gauge is nil", rmIdx, smIdx, mIdx)
		}
		dataPoints = data.Gauge.DataPoints
	case *metricspb.Metric_Sum:
		if data.Sum == nil {
			return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d].Sum is nil", rmIdx, smIdx, mIdx)
		}
		dataPoints = data.Sum.DataPoints
	case *metricspb.Metric_Histogram:
		return nil
	case *metricspb.Metric_ExponentialHistogram:
		return nil
	case *metricspb.Metric_Summary:
		return nil
	default:
		return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d] has unknown data type: %T", rmIdx, smIdx, mIdx, m.Data)
	}

	if len(dataPoints) == 0 {
		return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d] has no data points", rmIdx, smIdx, mIdx)
	}

	for dpIdx, dp := range dataPoints {
		if dp.TimeUnixNano == 0 {
			return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d].DataPoint[%d] has zero timestamp", rmIdx, smIdx, mIdx, dpIdx)
		}

		if dp.Value == nil {
			return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d].DataPoint[%d] has nil value", rmIdx, smIdx, mIdx, dpIdx)
		}

		switch v := dp.Value.(type) {
		case *metricspb.NumberDataPoint_AsInt:
			// Valid
		case *metricspb.NumberDataPoint_AsDouble:
			// Valid
		default:
			return fmt.Errorf("ResourceMetrics[%d].ScopeMetrics[%d].Metric[%d].DataPoint[%d] has invalid value type: %T", rmIdx, smIdx, mIdx, dpIdx, v)
		}
	}

	return nil
}

// sendGRPC sends the OTLP metrics via gRPC
func (o *otlpOutput) sendGRPC(ctx context.Context, req *metricsv1.ExportMetricsServiceRequest) error {
	cfg := o.cfg.Load()
	gs := o.grpcState.Load()

	if gs == nil || gs.client == nil {
		return fmt.Errorf("gRPC client not initialized")
	}

	if err := o.validateRequest(req); err != nil {
		o.logger.Printf("VALIDATION ERROR: %v", err)
		return fmt.Errorf("request validation failed: %w", err)
	}

	if cfg.Timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	if cfg.Debug {
		o.logger.Printf("DEBUG: Sending OTLP request with %d ResourceMetrics", len(req.ResourceMetrics))
		if len(req.ResourceMetrics) > 0 && len(req.ResourceMetrics[0].ScopeMetrics) > 0 {
			o.logger.Printf("DEBUG: First ScopeMetric has %d Metrics", len(req.ResourceMetrics[0].ScopeMetrics[0].Metrics))
		}
	}

	response, err := gs.client.Export(ctx, req)
	if err != nil {
		if cfg.Debug {
			o.logger.Printf("DEBUG: gRPC Export returned error: %v", err)
		}
		return fmt.Errorf("grpc export failed: %w", err)
	}

	if cfg.Debug {
		o.logger.Printf("DEBUG: gRPC Export succeeded")
	}

	if response.PartialSuccess != nil && response.PartialSuccess.RejectedDataPoints > 0 {
		errMsg := fmt.Sprintf("OTEL rejected %d data points: %s",
			response.PartialSuccess.RejectedDataPoints,
			response.PartialSuccess.ErrorMessage)
		o.logger.Printf("ERROR: %s", errMsg)
		if cfg.EnableMetrics {
			otlpRejectedDataPoints.WithLabelValues(cfg.Name).Add(float64(response.PartialSuccess.RejectedDataPoints))
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}
