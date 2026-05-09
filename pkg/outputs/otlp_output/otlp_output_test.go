// © 2025-2026 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"context"
	"io"
	"log"
	"net"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

// newTestOutput creates an otlpOutput suitable for converter tests (no Init required).
func newTestOutput(cfg *config) *otlpOutput {
	cfg.resourceTagSet = make(map[string]bool, len(cfg.ResourceTagKeys))
	for _, k := range cfg.ResourceTagKeys {
		cfg.resourceTagSet[k] = true
	}
	cfg.counterRegexes = make([]*regexp.Regexp, 0, len(cfg.CounterPatterns))
	for _, p := range cfg.CounterPatterns {
		cfg.counterRegexes = append(cfg.counterRegexes, regexp.MustCompile(p))
	}
	o := &otlpOutput{}
	o.cfg = new(atomic.Pointer[config])
	o.cfg.Store(cfg)
	o.logger = log.New(io.Discard, "", 0)
	return o
}

// Test 1: OTLP Message Structure
func TestOTLP_MessageStructure(t *testing.T) {
	t.Skip("Implementation pending")

	// Test that gNMI metrics convert to proper OTLP structure
	// Given: gNMI metric update
	event := &formatters.EventMsg{
		Name:      "interfaces_interface_state_counters_in_octets",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"interface_name": "Ethernet1",
			"source":         "10.1.1.1:6030",
		},
		Values: map[string]interface{}{
			"value": int64(1234567890),
		},
	}

	// When: Converting to OTLP
	output := newTestOutput(&config{
		Endpoint: "localhost:4317",
	})
	otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

	// Then: Should have proper OTLP structure
	require.NotNil(t, otlpMetrics)
	require.Equal(t, 1, len(otlpMetrics.ResourceMetrics))
	require.Equal(t, 1, len(otlpMetrics.ResourceMetrics[0].ScopeMetrics))

	metric := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
	assert.Equal(t, "interfaces_interface_state_counters_in_octets", metric.Name)

	// Verify it's a Sum (monotonic counter)
	assert.NotNil(t, metric.GetSum())
	assert.True(t, metric.GetSum().IsMonotonic)
}

// Test 2: Resource Attributes
func TestOTLP_ResourceAttributes(t *testing.T) {
	t.Skip("Implementation pending")

	// Test that device metadata becomes OTLP resource attributes
	// Given: gNMI update with device metadata
	event := &formatters.EventMsg{
		Name:      "interfaces_interface_state_counters_in_octets",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"device": "switch1-jhb01",
			"vendor": "arista",
			"site":   "jhb01",
			"source": "10.1.1.1:6030",
		},
		Values: map[string]interface{}{
			"value": int64(100),
		},
	}

	// When: Converting to OTLP
	output := newTestOutput(&config{})
	otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

	// Then: Resource attributes should match metadata
	resource := otlpMetrics.ResourceMetrics[0].Resource
	assert.Equal(t, "switch1-jhb01", getAttributeValue(resource, "device"))
	assert.Equal(t, "arista", getAttributeValue(resource, "vendor"))
	assert.Equal(t, "jhb01", getAttributeValue(resource, "site"))
	assert.Equal(t, "10.1.1.1:6030", getAttributeValue(resource, "source"))
}

// Test 3: Metric Attributes from Path Keys
func TestOTLP_PathKeysAsAttributes(t *testing.T) {
	t.Skip("Implementation pending")

	// Test that gNMI path keys become OTLP metric attributes
	// Given: Event with path key as tag
	event := &formatters.EventMsg{
		Name:      "interfaces_interface_state_counters_in_octets",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"interface_name": "Ethernet1",
			"source":         "10.1.1.1:6030",
		},
		Values: map[string]interface{}{
			"value": int64(999),
		},
	}

	// When: Converting to OTLP
	output := newTestOutput(&config{})
	otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

	// Then: Path key becomes attribute
	dataPoint := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetSum().DataPoints[0]
	assert.Equal(t, "Ethernet1", getDataPointAttribute(dataPoint, "interface_name"))
}

// Test 4: Metric Type Detection
func TestOTLP_MetricTypeDetection(t *testing.T) {
	t.Skip("Implementation pending")

	tests := []struct {
		name              string
		metricName        string
		value             interface{}
		expectedType      string // "Sum" or "Gauge"
		expectedMonotonic bool
	}{
		{
			name:              "counter metric",
			metricName:        "interfaces_interface_state_counters_in_octets",
			value:             int64(1000),
			expectedType:      "Sum",
			expectedMonotonic: true,
		},
		{
			name:              "gauge metric - temperature",
			metricName:        "components_component_temperature_instant",
			value:             45.5,
			expectedType:      "Gauge",
			expectedMonotonic: false,
		},
		{
			name:              "gauge metric - status",
			metricName:        "interfaces_interface_state_oper_status",
			value:             "up",
			expectedType:      "Gauge",
			expectedMonotonic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &formatters.EventMsg{
				Name:      tt.metricName,
				Timestamp: time.Now().UnixNano(),
				Values: map[string]interface{}{
					"value": tt.value,
				},
			}

			output := newTestOutput(&config{})
			otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})
			metric := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]

			switch tt.expectedType {
			case "Sum":
				assert.NotNil(t, metric.GetSum())
				assert.Equal(t, tt.expectedMonotonic, metric.GetSum().IsMonotonic)
			case "Gauge":
				assert.NotNil(t, metric.GetGauge())
			}
		})
	}
}

// Test 5: gRPC Transport
func TestOTLP_GRPCTransport(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "5s",
		"batch-size": 1,
		"interval":   "100ms",
	}

	output := &otlpOutput{}

	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)
	defer output.Close()

	event := createTestEvent()
	output.WriteEvent(context.Background(), event)

	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, server.ReceivedMetricsCount(), 0)
}

// Test 6: Configuration Validation
func TestOTLP_ConfigValidation(t *testing.T) {
	t.Skip("Implementation pending")

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid gRPC config",
			config: map[string]interface{}{
				"endpoint": "localhost:4317",
				"protocol": "grpc",
			},
			expectError: false,
		},
		{
			name: "valid HTTP config",
			config: map[string]interface{}{
				"endpoint": "http://localhost:4318",
				"protocol": "http",
			},
			expectError: false,
		},
		{
			name: "missing endpoint",
			config: map[string]interface{}{
				"protocol": "grpc",
			},
			expectError: true,
			errorMsg:    "endpoint is required",
		},
		{
			name: "invalid protocol",
			config: map[string]interface{}{
				"endpoint": "localhost:4317",
				"protocol": "invalid",
			},
			expectError: true,
			errorMsg:    "unsupported protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &otlpOutput{}
			err := output.Init(context.Background(), "test-otlp", tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				output.Close()
			}
		})
	}
}

// Test 7: String Values as Attributes
func TestOTLP_StringValuesAsAttributes(t *testing.T) {
	t.Skip("Implementation pending")

	// Test strings-as-attributes conversion
	// Given: String value metric
	event := &formatters.EventMsg{
		Name:      "interfaces_interface_state_oper_status",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"interface_name": "Ethernet1",
		},
		Values: map[string]interface{}{
			"value": "up",
		},
	}

	// When: Converting with strings-as-attributes enabled
	output := newTestOutput(&config{StringsAsAttributes: true})
	otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

	// Then: Should create gauge with value=1 and status as attribute
	metric := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
	gauge := metric.GetGauge()
	require.NotNil(t, gauge)

	dataPoint := gauge.DataPoints[0]
	assert.Equal(t, float64(1), dataPoint.GetAsDouble())
	assert.Equal(t, "up", getDataPointAttribute(dataPoint, "value"))
}

// Test 8: Subscription Name Mapping
func TestOTLP_SubscriptionNameMapping(t *testing.T) {
	t.Skip("Implementation pending")

	// Test that subscription names become resource attributes
	// Given: Event with subscription name
	event := &formatters.EventMsg{
		Name:      "interfaces_interface_state_counters_in_octets",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"subscription_name": "arista",
			"source":            "10.1.1.1:6030",
		},
		Values: map[string]interface{}{
			"value": int64(100),
		},
	}

	// When: Converting with append-subscription-name enabled
	output := newTestOutput(&config{AppendSubscriptionName: true})
	otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

	// Then: subscription_name should be in attributes
	dataPoint := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0].GetSum().DataPoints[0]
	assert.Equal(t, "arista", getDataPointAttribute(dataPoint, "subscription_name"))
}

// TestBuildMetricName_StripLeadingUnderscore verifies the strip-leading-underscore config option.
// gNMI paths arrive with a leading "/" (see pkg/formatters/event.go updateToEvent), which the
// slash->underscore conversion turns into a leading "_". This test pins both the backward-compatible
// default (option off) and the new behavior (option on).
func TestBuildMetricName_StripLeadingUnderscore(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config
		event    *formatters.EventMsg
		valueKey string
		expected string
	}{
		{
			name:     "default preserves leading underscore (backward compat)",
			cfg:      &config{},
			event:    &formatters.EventMsg{Name: "sub1"},
			valueKey: "/interfaces/interface/state/counters/in-octets",
			expected: "_interfaces_interface_state_counters_in_octets",
		},
		{
			name:     "enabled removes leading underscore",
			cfg:      &config{StripLeadingUnderscore: true},
			event:    &formatters.EventMsg{Name: "sub1"},
			valueKey: "/interfaces/interface/state/counters/in-octets",
			expected: "interfaces_interface_state_counters_in_octets",
		},
		{
			name:     "disabled with metric-prefix yields double underscore (backward compat)",
			cfg:      &config{MetricPrefix: "gnmic"},
			event:    &formatters.EventMsg{Name: "sub1"},
			valueKey: "/interfaces/interface/state/counters/in-octets",
			expected: "gnmic__interfaces_interface_state_counters_in_octets",
		},
		{
			name:     "enabled with metric-prefix has single underscore separator",
			cfg:      &config{StripLeadingUnderscore: true, MetricPrefix: "gnmic"},
			event:    &formatters.EventMsg{Name: "sub1"},
			valueKey: "/interfaces/interface/state/counters/in-octets",
			expected: "gnmic_interfaces_interface_state_counters_in_octets",
		},
		{
			name:     "enabled with append-subscription-name has single underscore separator",
			cfg:      &config{StripLeadingUnderscore: true, AppendSubscriptionName: true},
			event:    &formatters.EventMsg{Name: "arista"},
			valueKey: "/interfaces/interface/state/counters/in-octets",
			expected: "arista_interfaces_interface_state_counters_in_octets",
		},
		{
			name:     "enabled does not touch non-leading underscores",
			cfg:      &config{StripLeadingUnderscore: true},
			event:    &formatters.EventMsg{Name: "sub1"},
			valueKey: "/a_b/c",
			expected: "a_b_c",
		},
		{
			name:     "enabled is a no-op when path has no leading slash",
			cfg:      &config{StripLeadingUnderscore: true},
			event:    &formatters.EventMsg{Name: "sub1"},
			valueKey: "interfaces/interface",
			expected: "interfaces_interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := newTestOutput(tt.cfg)
			got := output.buildMetricName(tt.cfg, tt.event, tt.valueKey)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// getResourceAttr returns the value of an attribute on a Resource, and whether it was present.
func getResourceAttr(r interface{ GetAttributes() []*commonpb.KeyValue }, key string) (string, bool) {
	for _, a := range r.GetAttributes() {
	    if a.Key == key {
	        return a.Value.GetStringValue(), true
	    }
	}
	return "", false
}

// getAttr returns the value of an attribute in a KeyValue slice, and whether it was present.
func getAttr(attrs []*commonpb.KeyValue, key string) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.GetStringValue(), true
		}
	}
	return "", false
}

// TestCreateResource_UniformTagsPromoted pins the backward-compatible path:
// when every event in the group agrees on a resource-tag-keys value, that tag
// is promoted to the Resource and not flagged as divergent.
func TestCreateResource_UniformTagsPromoted(t *testing.T) {
	cfg := &config{
		ResourceTagKeys: []string{"device", "model", "vendor"},
	}
	output := newTestOutput(cfg)

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"device": "bbr2", "model": "junos", "vendor": "juniper", "component_name": "RE0"}},
		{Tags: map[string]string{"device": "bbr2", "model": "junos", "vendor": "juniper", "component_name": "FPC0"}},
	}

	resource, divergent := output.createResource(cfg, events)

	device, ok := getResourceAttr(resource, "device")
	require.True(t, ok, "device should be on Resource")
	assert.Equal(t, "bbr2", device)

	model, ok := getResourceAttr(resource, "model")
	require.True(t, ok)
	assert.Equal(t, "junos", model)

	assert.Empty(t, divergent, "no divergent keys expected when all events agree")
}

// TestCreateResource_DivergentTagsInDivergenceSet is the spec-violation fix:
// when a resource-tag-keys value varies across events in the group, the tag
// is omitted from Resource and reported in the divergence set so that the
// per-datapoint attribute extraction can route it correctly.
func TestCreateResource_DivergentTagsInDivergenceSet(t *testing.T) {
	cfg := &config{
		ResourceTagKeys: []string{"device", "serial_no"},
	}
	output := newTestOutput(cfg)

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"device": "bbr2", "serial_no": "BUILTIN"}},
		{Tags: map[string]string{"device": "bbr2", "serial_no": "1W1CUPAA04012"}},
	}

	resource, divergent := output.createResource(cfg, events)

	device, ok := getResourceAttr(resource, "device")
	require.True(t, ok)
	assert.Equal(t, "bbr2", device, "uniform tag still promoted")

	_, ok = getResourceAttr(resource, "serial_no")
	assert.False(t, ok, "divergent tag must not appear on Resource")

	assert.True(t, divergent["serial_no"], "serial_no must be flagged as divergent")
	assert.False(t, divergent["device"], "device should not be flagged")
}

// TestCreateResource_MissingFromSomeEventsIsLenient pins the lenient semantic:
// a tag present in some events and absent in others, where the present events
// all agree, is still promoted to Resource. Absence does not count as divergence.
func TestCreateResource_MissingFromSomeEventsIsLenient(t *testing.T) {
	cfg := &config{
		ResourceTagKeys: []string{"model"},
	}
	output := newTestOutput(cfg)

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"model": "junos"}},
		{Tags: map[string]string{}}, // model absent
		{Tags: map[string]string{"model": "junos"}},
	}

	resource, divergent := output.createResource(cfg, events)

	model, ok := getResourceAttr(resource, "model")
	require.True(t, ok, "model should be promoted when present events agree")
	assert.Equal(t, "junos", model)
	assert.False(t, divergent["model"])
}

// TestCreateResource_MissingFromAllEventsSkipped verifies that a resource-tag-key
// absent from every event in the group is neither promoted nor flagged as divergent.
func TestCreateResource_MissingFromAllEventsSkipped(t *testing.T) {
	cfg := &config{
		ResourceTagKeys: []string{"vendor"},
	}
	output := newTestOutput(cfg)

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"device": "bbr2"}},
		{Tags: map[string]string{"device": "bbr2"}},
	}

	resource, divergent := output.createResource(cfg, events)

	_, ok := getResourceAttr(resource, "vendor")
	assert.False(t, ok, "absent tag not promoted")
	assert.False(t, divergent["vendor"], "absent tag not divergent either")
}

// TestCreateResource_ResourceAttributesStillAppended verifies that the static
// cfg.ResourceAttributes map continues to be merged into the Resource regardless
// of divergence handling for resource-tag-keys.
func TestCreateResource_ResourceAttributesStillAppended(t *testing.T) {
	cfg := &config{
		ResourceTagKeys:    []string{"device"},
		ResourceAttributes: map[string]string{"telemetry.source": "gnmi"},
	}
	output := newTestOutput(cfg)

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"device": "bbr2"}},
	}

	resource, _ := output.createResource(cfg, events)

	v, ok := getResourceAttr(resource, "telemetry.source")
	require.True(t, ok)
	assert.Equal(t, "gnmi", v)
}

// TestExtractAttributesForMetric_DivergentKeysIncluded verifies that when a
// resource-tag-key is flagged as divergent for the current group, the event's
// own value for that key flows through to datapoint attributes with its correct
// per-event value instead of being excluded as a resource-only tag.
func TestExtractAttributesForMetric_DivergentKeysIncluded(t *testing.T) {
	cfg := &config{
		ResourceTagKeys: []string{"device", "serial_no"},
	}
	output := newTestOutput(cfg)

	event := &formatters.EventMsg{Tags: map[string]string{
		"device":         "bbr2",
		"serial_no":      "BUILTIN",
		"component_name": "RE0",
	}}
	divergent := map[string]bool{"serial_no": true}

	attrs := output.extractAttributesForMetric(cfg, divergent, event)

	v, ok := getAttr(attrs, "serial_no")
	require.True(t, ok, "divergent resource-tag-key must appear on datapoint attrs")
	assert.Equal(t, "BUILTIN", v, "with this event's correct value")

	_, ok = getAttr(attrs, "device")
	assert.False(t, ok, "non-divergent resource-tag-key still excluded from datapoint attrs")

	v, ok = getAttr(attrs, "component_name")
	require.True(t, ok, "non-resource tag still on datapoint attrs")
	assert.Equal(t, "RE0", v)
}

// TestExtractAttributesForMetric_NoDivergenceMatchesOriginalBehavior is a
// regression pin: when no keys are divergent, datapoint attrs exclude every
// resource-tag-key — matching behavior prior to this change.
func TestExtractAttributesForMetric_NoDivergenceMatchesOriginalBehavior(t *testing.T) {
	cfg := &config{
		ResourceTagKeys: []string{"device", "model"},
	}
	output := newTestOutput(cfg)

	event := &formatters.EventMsg{Tags: map[string]string{
		"device":         "bbr2",
		"model":          "junos",
		"component_name": "RE0",
	}}

	attrs := output.extractAttributesForMetric(cfg, map[string]bool{}, event)

	_, ok := getAttr(attrs, "device")
	assert.False(t, ok)
	_, ok = getAttr(attrs, "model")
	assert.False(t, ok)
	v, ok := getAttr(attrs, "component_name")
	require.True(t, ok)
	assert.Equal(t, "RE0", v)
}

// TestGroupByResource_PrefersDeviceOverTargetAndSource pins the preference
// order: when device is present, it is used as the group key regardless of
// whether target or source is also present. device is the most stable and
// human-readable identifier of the producing entity, so it leads the chain.
func TestGroupByResource_PrefersDeviceOverTargetAndSource(t *testing.T) {
	output := newTestOutput(&config{})

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"device": "bbr2", "target": "bbr2-t", "source": "10.1.1.1:6030"}},
		{Tags: map[string]string{"device": "bbr2", "target": "bbr2-t", "source": "10.1.1.1:6030"}},
		{Tags: map[string]string{"device": "fnx5", "target": "fnx5-t", "source": "10.2.2.2:6030"}},
	}

	groups := output.groupByResource(events)

	assert.Len(t, groups["bbr2"], 2, "device leads the preference chain")
	assert.Len(t, groups["fnx5"], 1)
	_, ok := groups["10.1.1.1:6030"]
	assert.False(t, ok, "source must not be used when device is present")
}

// TestGroupByResource_UsesSourceWhenDeviceAndTargetAbsent verifies that when
// only source is present, it is still used as the group key — matching
// behavior prior to this change for deployments that have not set device or
// target tags.
func TestGroupByResource_UsesSourceWhenDeviceAndTargetAbsent(t *testing.T) {
	output := newTestOutput(&config{})

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"source": "10.1.1.1:6030"}},
		{Tags: map[string]string{"source": "10.2.2.2:6030"}},
	}

	groups := output.groupByResource(events)

	assert.Len(t, groups["10.1.1.1:6030"], 1)
	assert.Len(t, groups["10.2.2.2:6030"], 1)
}

// TestGroupByResource_FallsBackToDeviceWhenSourceAbsent verifies that when
// events have no source tag (e.g., stripped by a processor because it
// duplicates device), grouping falls back to device so per-target Resource
// isolation is preserved.
func TestGroupByResource_FallsBackToDeviceWhenSourceAbsent(t *testing.T) {
	output := newTestOutput(&config{})

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"device": "bbr2"}},
		{Tags: map[string]string{"device": "bbr2"}},
		{Tags: map[string]string{"device": "fnx5"}},
	}

	groups := output.groupByResource(events)

	assert.Len(t, groups["bbr2"], 2)
	assert.Len(t, groups["fnx5"], 1)
	_, ok := groups["unknown"]
	assert.False(t, ok, "unknown bucket must not be populated when device is present")
}

// TestGroupByResource_FallsBackToTargetWhenSourceAndDeviceAbsent verifies the
// third step of the fallback chain: when both source and device are absent,
// fall back to target.
func TestGroupByResource_FallsBackToTargetWhenSourceAndDeviceAbsent(t *testing.T) {
	output := newTestOutput(&config{})

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"target": "bbr2-logical"}},
		{Tags: map[string]string{"target": "bbr2-logical"}},
	}

	groups := output.groupByResource(events)

	assert.Len(t, groups["bbr2-logical"], 2)
	_, ok := groups["unknown"]
	assert.False(t, ok)
}

// TestGroupByResource_FallsBackToUnknownAsLastResort documents that when the
// whole fallback chain is empty, events collapse into a literal "unknown"
// bucket. This is the same landmine behavior as before the fallback chain —
// preserved as a last resort.
func TestGroupByResource_FallsBackToUnknownAsLastResort(t *testing.T) {
	output := newTestOutput(&config{})

	events := []*formatters.EventMsg{
		{Tags: map[string]string{"unrelated": "x"}},
		{Tags: map[string]string{"unrelated": "y"}},
	}

	groups := output.groupByResource(events)

	assert.Len(t, groups["unknown"], 2)
}

// Helper functions

func createTestEvent() *formatters.EventMsg {
	return &formatters.EventMsg{
		Name:      "test_metric",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"source": "test:1234",
		},
		Values: map[string]interface{}{
			"value": int64(42),
		},
	}
}

func getAttributeValue(resource interface{}, key string) string {
	// Helper to extract attribute value from resource
	// Will implement when we have the actual OTLP structures
	return ""
}

func getDataPointAttribute(dataPoint interface{}, key string) string {
	// Helper to extract attribute value from data point
	// Will implement when we have the actual OTLP structures
	return ""
}

// Mock OTLP server for testing
type mockOTLPServer struct {
	metricsv1.UnimplementedMetricsServiceServer
	grpcServer *grpc.Server
	listener   net.Listener

	m            sync.Mutex
	metricsCount int
	receivedReqs []*metricsv1.ExportMetricsServiceRequest
}

func startMockOTLPServer(t *testing.T) (*mockOTLPServer, string) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	mock := &mockOTLPServer{
		grpcServer: server,
		listener:   listener,
	}

	metricsv1.RegisterMetricsServiceServer(server, mock)

	go server.Serve(listener)

	return mock, listener.Addr().String()
}

func startMockOTLPServerOnAddress(t *testing.T, addr string) (*mockOTLPServer, string) {
	listener, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	server := grpc.NewServer()
	mock := &mockOTLPServer{
		grpcServer: server,
		listener:   listener,
	}

	metricsv1.RegisterMetricsServiceServer(server, mock)
	go server.Serve(listener)

	return mock, listener.Addr().String()
}

func (m *mockOTLPServer) Export(ctx context.Context, req *metricsv1.ExportMetricsServiceRequest) (*metricsv1.ExportMetricsServiceResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.m.Lock()
	defer m.m.Unlock()
	m.receivedReqs = append(m.receivedReqs, req)
	m.metricsCount += len(req.ResourceMetrics)
	return &metricsv1.ExportMetricsServiceResponse{}, nil
}

func (m *mockOTLPServer) ReceivedMetricsCount() int {
	m.m.Lock()
	defer m.m.Unlock()
	return m.metricsCount
}

func (m *mockOTLPServer) Stop() {
	m.grpcServer.Stop()
	m.listener.Close()
}

// Test 9: Resource Tag Keys control data point vs resource attribute placement
func TestOTLP_ResourceTagKeys(t *testing.T) {
	tests := []struct {
		name                   string
		resourceTagKeys        []string
		eventTags              map[string]string
		expectedInDataPoint    []string
		expectedNotInDataPoint []string
	}{
		{
			name:            "empty resource-tag-keys: all tags become data point attributes",
			resourceTagKeys: []string{},
			eventTags: map[string]string{
				"device":            "nvswitch1-nvl9-gp1-jhb01",
				"vendor":            "nvidia",
				"model":             "nvos",
				"interface_name":    "Ethernet1",
				"subscription_name": "nvos",
			},
			expectedInDataPoint:    []string{"device", "vendor", "model", "interface_name", "subscription_name"},
			expectedNotInDataPoint: []string{},
		},
		{
			name:            "default resource-tag-keys: device/vendor/model/site/source excluded from data point",
			resourceTagKeys: []string{"device", "vendor", "model", "site", "source"},
			eventTags: map[string]string{
				"device":         "nvswitch1-nvl9-gp1-jhb01",
				"vendor":         "nvidia",
				"model":          "nvos",
				"interface_name": "Ethernet1",
			},
			expectedInDataPoint:    []string{"interface_name"},
			expectedNotInDataPoint: []string{"device", "vendor", "model"},
		},
		{
			name:            "custom resource-tag-keys: only specified keys excluded",
			resourceTagKeys: []string{"source"},
			eventTags: map[string]string{
				"device":         "nvswitch1",
				"vendor":         "nvidia",
				"source":         "10.0.0.1",
				"interface_name": "Ethernet1",
			},
			expectedInDataPoint:    []string{"device", "vendor", "interface_name"},
			expectedNotInDataPoint: []string{"source"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &formatters.EventMsg{
				Name:      "test_metric",
				Timestamp: time.Now().UnixNano(),
				Tags:      tt.eventTags,
				Values: map[string]interface{}{
					"value": int64(100),
				},
			}

			output := newTestOutput(&config{
				ResourceTagKeys: tt.resourceTagKeys,
			})
			otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

			require.NotNil(t, otlpMetrics)
			require.Len(t, otlpMetrics.ResourceMetrics, 1)
			require.Len(t, otlpMetrics.ResourceMetrics[0].ScopeMetrics, 1)
			require.Len(t, otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics, 1)

			metric := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
			var dataPointAttrs map[string]string

			if metric.GetGauge() != nil {
				dataPointAttrs = extractAttributesMap(metric.GetGauge().DataPoints[0].Attributes)
			} else if metric.GetSum() != nil {
				dataPointAttrs = extractAttributesMap(metric.GetSum().DataPoints[0].Attributes)
			} else {
				t.Fatal("Metric has neither Gauge nor Sum data")
			}

			for _, key := range tt.expectedInDataPoint {
				assert.Contains(t, dataPointAttrs, key, "Expected tag '%s' to be in data point attributes", key)
				assert.Equal(t, tt.eventTags[key], dataPointAttrs[key], "Tag '%s' value mismatch", key)
			}

			for _, key := range tt.expectedNotInDataPoint {
				assert.NotContains(t, dataPointAttrs, key, "Tag '%s' should NOT be in data point attributes", key)
			}
		})
	}
}

// Test 10: Resource Attributes Behavior with ResourceTagKeys
func TestOTLP_ResourceAttributesBehavior(t *testing.T) {
	tests := []struct {
		name                string
		resourceTagKeys     []string
		eventTags           map[string]string
		expectInResource    []string
		expectNotInResource []string
	}{
		{
			name:            "empty resource-tag-keys: no tags in resource",
			resourceTagKeys: []string{},
			eventTags: map[string]string{
				"device": "nvswitch1",
				"vendor": "nvidia",
			},
			expectInResource:    []string{},
			expectNotInResource: []string{"device", "vendor"},
		},
		{
			name:            "default resource-tag-keys: device/vendor in resource",
			resourceTagKeys: []string{"device", "vendor", "model", "site", "source"},
			eventTags: map[string]string{
				"device": "nvswitch1",
				"vendor": "nvidia",
			},
			expectInResource:    []string{"device", "vendor"},
			expectNotInResource: []string{},
		},
		{
			name:            "custom resource-tag-keys: only source in resource",
			resourceTagKeys: []string{"source"},
			eventTags: map[string]string{
				"device": "nvswitch1",
				"vendor": "nvidia",
				"source": "10.0.0.1",
			},
			expectInResource:    []string{"source"},
			expectNotInResource: []string{"device", "vendor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &formatters.EventMsg{
				Name:      "test_metric",
				Timestamp: time.Now().UnixNano(),
				Tags:      tt.eventTags,
				Values: map[string]interface{}{
					"value": int64(100),
				},
			}

			output := newTestOutput(&config{
				ResourceTagKeys: tt.resourceTagKeys,
			})
			otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

			require.NotNil(t, otlpMetrics)
			resource := otlpMetrics.ResourceMetrics[0].Resource
			resourceAttrs := extractAttributesMap(resource.Attributes)

			for _, key := range tt.expectInResource {
				assert.Contains(t, resourceAttrs, key, "Expected tag '%s' in resource attributes", key)
			}

			for _, key := range tt.expectNotInResource {
				assert.NotContains(t, resourceAttrs, key, "Tag '%s' should NOT be in resource attributes", key)
			}
		})
	}
}

// Test 11: Configured Resource Attributes Always Included
func TestOTLP_ConfiguredResourceAttributesAlwaysIncluded(t *testing.T) {
	event := &formatters.EventMsg{
		Name:      "test_metric",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"device": "nvswitch1",
		},
		Values: map[string]interface{}{
			"value": int64(100),
		},
	}

	output := newTestOutput(&config{
		ResourceTagKeys: []string{},
		ResourceAttributes: map[string]string{
			"service.name":    "gnmic-collector",
			"service.version": "0.42.0",
		},
	})
	otlpMetrics := output.convertToOTLP([]*formatters.EventMsg{event})

	resource := otlpMetrics.ResourceMetrics[0].Resource
	resourceAttrs := extractAttributesMap(resource.Attributes)

	// Configured resource attributes should always be present
	assert.Equal(t, "gnmic-collector", resourceAttrs["service.name"])
	assert.Equal(t, "0.42.0", resourceAttrs["service.version"])
}

// Test 12: Init succeeds even when endpoint is unreachable
func TestOTLP_InitSucceedsWithUnreachableEndpoint(t *testing.T) {
	cfg := map[string]interface{}{
		"endpoint": "unreachable-host:4317",
		"protocol": "grpc",
		"timeout":  "1s",
	}

	output := &otlpOutput{}

	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	assert.NoError(t, err, "Init should succeed even with unreachable endpoint")

	gs := output.grpcState.Load()
	assert.NotNil(t, gs, "gRPC state should be created")
	assert.NotNil(t, gs.conn, "gRPC connection should be created")
	assert.NotNil(t, gs.client, "gRPC client should be created")

	output.Close()
}

// Test 13: Connection happens lazily on first RPC
func TestOTLP_ConnectionOnFirstRPC(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "5s",
		"batch-size": 1,
		"interval":   "100ms",
	}

	output := &otlpOutput{}

	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)
	defer output.Close()

	assert.Equal(t, 0, server.ReceivedMetricsCount(), "No metrics should be sent yet")

	event := createTestEvent()
	output.WriteEvent(context.Background(), event)

	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, server.ReceivedMetricsCount(), 0, "Metrics should be sent on first RPC")
}

// Test 14: Retry behavior with delayed endpoint availability
func TestOTLP_ReconnectWhenEndpointBecomesAvailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	endpoint := listener.Addr().String()
	listener.Close()

	cfg := map[string]interface{}{
		"endpoint":    endpoint,
		"protocol":    "grpc",
		"timeout":     "2s",
		"batch-size":  1,
		"interval":    "200ms",
		"max-retries": 10,
	}

	output := &otlpOutput{}

	err = output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)
	defer output.Close()

	server, newEndpoint := startMockOTLPServerOnAddress(t, endpoint)
	defer server.Stop()
	assert.Equal(t, endpoint, newEndpoint)

	event := createTestEvent()
	output.WriteEvent(context.Background(), event)

	time.Sleep(500 * time.Millisecond)

	assert.Greater(t, server.ReceivedMetricsCount(), 0, "Should successfully send after endpoint becomes available")
}

// Test 15: Graceful shutdown flushes remaining batch
func TestOTLP_GracefulShutdownFlushes(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "5s",
		"batch-size": 100,
		"interval":   "10s",
	}

	output := &otlpOutput{}

	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)

	event := createTestEvent()
	output.WriteEvent(context.Background(), event)
	output.WriteEvent(context.Background(), event)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, server.ReceivedMetricsCount(), "Batch should not be sent yet (batch size not reached)")

	output.Close()

	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, server.ReceivedMetricsCount(), 0, "Remaining batch should be flushed on shutdown")
}

// Test 16: Context cancellation sends final batch with fresh context
func TestOTLP_ContextCancellationFlushes(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "5s",
		"batch-size": 100,
		"interval":   "10s",
	}

	ctx, cancel := context.WithCancel(context.Background())

	output := &otlpOutput{}

	err := output.Init(ctx, "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)

	event := createTestEvent()
	output.WriteEvent(context.Background(), event)
	output.WriteEvent(context.Background(), event)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, server.ReceivedMetricsCount(), "Batch should not be sent yet")

	cancel()

	time.Sleep(200 * time.Millisecond)
	output.Close()

	assert.Greater(t, server.ReceivedMetricsCount(), 0, "Batch should be flushed even after context cancellation")
}

// Test 17: Channel close flushes remaining batch
func TestOTLP_ChannelCloseFlushes(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "5s",
		"batch-size": 100,
		"interval":   "10s",
	}

	output := &otlpOutput{}

	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)

	event := createTestEvent()
	output.WriteEvent(context.Background(), event)
	output.WriteEvent(context.Background(), event)
	output.WriteEvent(context.Background(), event)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, server.ReceivedMetricsCount(), "Batch should not be sent yet")

	close(*output.eventCh.Load())

	time.Sleep(200 * time.Millisecond)

	assert.Greater(t, server.ReceivedMetricsCount(), 0, "Remaining batch should be flushed when channel closes")
}

// Helper to extract attributes map from KeyValue slice
func extractAttributesMap(attrs []*commonpb.KeyValue) map[string]string {
	result := make(map[string]string)
	for _, attr := range attrs {
		if strVal := attr.Value.GetStringValue(); strVal != "" {
			result[attr.Key] = strVal
		}
	}
	return result
}
