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

	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/openconfig/gnmic/pkg/formatters"
)

// convertToOTLP converts gNMI EventMsg slice to OTLP ExportMetricsServiceRequest
func (o *otlpOutput) convertToOTLP(events []*formatters.EventMsg) *metricsv1.ExportMetricsServiceRequest {
	// Always log conversion start (not just debug mode)
	o.logger.Printf("convertToOTLP called with %d events, debug=%v", len(events), o.cfg.Debug)

	if o.cfg.Debug {
		o.logger.Printf("DEBUG: convertToOTLP called with %d events", len(events))
	}

	// Group events by resource (source)
	resourceGroups := o.groupByResource(events)

	if o.cfg.Debug {
		o.logger.Printf("DEBUG: Grouped into %d resource groups", len(resourceGroups))
	}

	req := &metricsv1.ExportMetricsServiceRequest{
		ResourceMetrics: make([]*metricspb.ResourceMetrics, 0, len(resourceGroups)),
	}

	totalMetrics := 0
	skippedEvents := 0

	for _, groupedEvents := range resourceGroups {
		rm := &metricspb.ResourceMetrics{
			Resource: o.createResource(groupedEvents[0]),
			ScopeMetrics: []*metricspb.ScopeMetrics{
				{
					Scope: &commonpb.InstrumentationScope{
						Name:    "gnmic",
						Version: "0.42.0", // TODO: Get from build info
					},
					Metrics: make([]*metricspb.Metric, 0, len(groupedEvents)),
				},
			},
		}

		// Convert each event to OTLP metric
		for _, event := range groupedEvents {
			metric, err := o.convertEvent(event)
			if err != nil {
				if o.cfg.Debug {
					o.logger.Printf("DEBUG: failed to convert event %s: %v", event.Name, err)
				}
				skippedEvents++
				continue
			}
			if metric == nil {
				if o.cfg.Debug {
					o.logger.Printf("DEBUG: convertEvent returned nil for event: name=%s, values=%v", event.Name, event.Values)
				}
				skippedEvents++
				continue
			}
			rm.ScopeMetrics[0].Metrics = append(rm.ScopeMetrics[0].Metrics, metric)
			totalMetrics++
		}

		if len(rm.ScopeMetrics[0].Metrics) > 0 {
			req.ResourceMetrics = append(req.ResourceMetrics, rm)
		}
	}

	// Always log conversion results (not just debug mode)
	o.logger.Printf("Converted %d metrics, skipped %d events, %d ResourceMetrics",
		totalMetrics, skippedEvents, len(req.ResourceMetrics))

	if o.cfg.Debug {
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

// createResource creates OTLP Resource from event metadata
func (o *otlpOutput) createResource(event *formatters.EventMsg) *resourcepb.Resource {
	attrs := make([]*commonpb.KeyValue, 0)

	// When add-event-tags-as-attributes is enabled, don't duplicate tags in resource attributes
	// Resource attributes don't become Prometheus labels, so we only add them for OTLP semantics
	if !o.cfg.AddEventTagsAsAttributes {
		// Add device metadata as resource attributes (legacy behavior)
		resourceKeys := []string{"device", "vendor", "model", "site", "source", "subscription_name"}
		for _, key := range resourceKeys {
			if val, ok := event.Tags[key]; ok {
				attrs = append(attrs, &commonpb.KeyValue{
					Key:   key,
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
				})
			}
		}
	}

	// Always add configured resource attributes
	for key, val := range o.cfg.ResourceAttributes {
		attrs = append(attrs, &commonpb.KeyValue{
			Key:   key,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
		})
	}

	return &resourcepb.Resource{
		Attributes: attrs,
	}
}

// convertEvent converts a single gNMI event to OTLP metrics
// Returns nil if the event has no valid values to convert
func (o *otlpOutput) convertEvent(event *formatters.EventMsg) (*metricspb.Metric, error) {
	if len(event.Values) == 0 {
		if o.cfg.Debug {
			o.logger.Printf("DEBUG: event has no values (event: %s)", event.Name)
		}
		return nil, nil
	}

	// gNMI events typically have only ONE value per event
	// If there are multiple values, we'll only process the first one
	// and log a warning (this is unexpected but we'll handle it gracefully)
	if len(event.Values) > 1 && o.cfg.Debug {
		o.logger.Printf("DEBUG: event has multiple values (%d), using first (event: %s)", len(event.Values), event.Name)
	}

	// Get the first (and typically only) value from the event
	var value interface{}
	var valueKey string
	for k, v := range event.Values {
		value = v
		valueKey = k
		break // Take first value
	}

	// Build metric name from subscription name + value key
	metricName := o.buildMetricName(event, valueKey)

	// Extract path keys as attributes
	attributes := o.extractAttributes(event)

	// Determine metric type and create appropriate OTLP metric
	metric := &metricspb.Metric{
		Name:        metricName,
		Description: "", // Could extract from event metadata if available
		Unit:        "", // Could extract from event metadata if available
	}

	// Handle string values
	if strVal, isString := value.(string); isString {
		if !o.cfg.StringsAsAttributes {
			// Skip string values if not configured to handle them
			if o.cfg.Debug {
				o.logger.Printf("DEBUG: skipping string value (strings-as-attributes=false): %s", event.Name)
			}
			return nil, nil
		}
		// Convert string to gauge with value=1 and string as attribute
		metric.Data = &metricspb.Metric_Gauge{
			Gauge: o.createGaugeWithString(event, attributes, strVal),
		}
		return metric, nil
	}

	// Create data point with the value
	dataPoint := o.createNumberDataPointWithValue(event, attributes, value)
	if dataPoint == nil {
		if o.cfg.Debug {
			o.logger.Printf("DEBUG: failed to create data point for value type %T (event: %s)", value, event.Name)
		}
		return nil, nil
	}

	// Detect metric type from path and value
	if o.isCounter(event) {
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

	return metric, nil
}

// buildMetricName creates metric name from event and value key
// event.Name contains the subscription name (e.g., "nvos", "arista")
// valueKey contains the metric path (e.g., "interfaces/interface/state/counters/in-octets")
func (o *otlpOutput) buildMetricName(event *formatters.EventMsg, valueKey string) string {
	var sb strings.Builder

	// Add global prefix if configured
	if o.cfg.MetricPrefix != "" {
		sb.WriteString(o.cfg.MetricPrefix)
		sb.WriteString("_")
	}

	// Append subscription name if configured (for vendor-specific prefixes)
	if o.cfg.AppendSubscriptionName {
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

// extractAttributes extracts attributes from event tags
func (o *otlpOutput) extractAttributes(event *formatters.EventMsg) []*commonpb.KeyValue {
	attrs := make([]*commonpb.KeyValue, 0)

	// When add-event-tags-as-attributes is enabled, include all event-tags as data point attributes
	// This makes them appear as Prometheus labels
	if o.cfg.AddEventTagsAsAttributes {
		for key, val := range event.Tags {
			attrs = append(attrs, &commonpb.KeyValue{
				Key:   key,
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
			})
		}
		return attrs
	}

	// Legacy behavior: exclude resource-level tags from metric attributes
	resourceTags := map[string]bool{
		"device":            true,
		"vendor":            true,
		"model":             true,
		"site":              true,
		"source":            true,
		"subscription_name": !o.cfg.AppendSubscriptionName, // Only skip if not appending
	}

	for key, val := range event.Tags {
		if resourceTags[key] {
			continue
		}

		attrs = append(attrs, &commonpb.KeyValue{
			Key:   key,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
		})
	}

	// Add subscription name as attribute if configured
	if o.cfg.AppendSubscriptionName {
		if sub, ok := event.Tags["subscription_name"]; ok {
			attrs = append(attrs, &commonpb.KeyValue{
				Key:   "subscription_name",
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: sub}},
			})
		}
	}

	return attrs
}

// isCounter detects if metric is a monotonic counter based on name
func (o *otlpOutput) isCounter(event *formatters.EventMsg) bool {
	// Heuristics: path contains counter-related keywords
	counterKeywords := []string{"counter", "octets", "packets", "bytes", "errors", "discards", "drops"}

	path := strings.ToLower(event.Name)
	for _, keyword := range counterKeywords {
		if strings.Contains(path, keyword) {
			return true
		}
	}

	return false
}

// createNumberDataPointWithValue creates OTLP data point from event with a specific value
func (o *otlpOutput) createNumberDataPointWithValue(event *formatters.EventMsg, attrs []*commonpb.KeyValue, value interface{}) *metricspb.NumberDataPoint {
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
		// Unknown type
		if o.cfg.Debug {
			o.logger.Printf("unsupported value type %T for metric %s", v, event.Name)
		}
		return nil
	}

	return dp
}

// createGaugeWithString creates OTLP Gauge with string value as attribute
func (o *otlpOutput) createGaugeWithString(event *formatters.EventMsg, attrs []*commonpb.KeyValue, strVal string) *metricspb.Gauge {
	// Add string value as attribute
	attrs = append(attrs, &commonpb.KeyValue{
		Key:   "value",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: strVal}},
	})

	dp := &metricspb.NumberDataPoint{
		Attributes:   attrs,
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
func (o *otlpOutput) sendGRPC(req *metricsv1.ExportMetricsServiceRequest) error {
	if o.grpcClient == nil {
		return fmt.Errorf("gRPC client not initialized")
	}

	if err := o.validateRequest(req); err != nil {
		o.logger.Printf("VALIDATION ERROR: %v", err)
		return fmt.Errorf("request validation failed: %w", err)
	}

	ctx := o.ctx
	if o.cfg.Timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(o.ctx, o.cfg.Timeout)
		defer cancel()
	}

	if o.cfg.Debug {
		o.logger.Printf("DEBUG: Sending OTLP request with %d ResourceMetrics", len(req.ResourceMetrics))
		if len(req.ResourceMetrics) > 0 && len(req.ResourceMetrics[0].ScopeMetrics) > 0 {
			o.logger.Printf("DEBUG: First ScopeMetric has %d Metrics", len(req.ResourceMetrics[0].ScopeMetrics[0].Metrics))
		}
	}

	response, err := o.grpcClient.Export(ctx, req)
	if err != nil {
		if o.cfg.Debug {
			o.logger.Printf("DEBUG: gRPC Export returned error: %v", err)
		}
		return fmt.Errorf("grpc export failed: %w", err)
	}

	if o.cfg.Debug {
		o.logger.Printf("DEBUG: gRPC Export succeeded")
	}

	if response.PartialSuccess != nil && response.PartialSuccess.RejectedDataPoints > 0 {
		errMsg := fmt.Sprintf("OTEL rejected %d data points: %s",
			response.PartialSuccess.RejectedDataPoints,
			response.PartialSuccess.ErrorMessage)
		o.logger.Printf("ERROR: %s", errMsg)
		if o.cfg.EnableMetrics {
			otlpRejectedDataPoints.WithLabelValues(o.cfg.Name).Add(float64(response.PartialSuccess.RejectedDataPoints))
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}
