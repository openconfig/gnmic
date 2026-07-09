// © 2025-2026 NVIDIA Corporation
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of NVIDIA's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net"
	"net/http"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
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
	o.state = new(atomic.Pointer[outputState])
	o.state.Store(&outputState{cfg: cfg, transport: &transportState{}})
	o.logger = logging.DiscardLogger()
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
	otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
	otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
	otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
			otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})
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

// Renamed from TestOTLP_InitCompilesCounterPatterns: upstream carries two
// tests with that name (one here, one near the end of the file) from the
// #886/#887 merge; this one keeps the end-to-end conversion angle under a
// distinct name.
func TestOTLP_InitCounterPatternsClassifyAsSum(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":         endpoint,
		"protocol":         "grpc",
		"timeout":          "5s",
		"batch-size":       1,
		"interval":         "100ms",
		"counter-patterns": []string{"counters", "octets"},
	}

	output := &otlpOutput{}
	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)
	defer output.Close()

	stored := output.loadCfg()
	require.Len(t, stored.counterRegexes, 2)

	event := &formatters.EventMsg{
		Name:      "interfaces",
		Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{
			"source": "10.0.0.1:57400",
		},
		Values: map[string]interface{}{
			"/interfaces/interface/state/counters/out-octets": int64(1000),
		},
	}
	otlpMetrics := output.convertToOTLP(stored, []*formatters.EventMsg{event})
	require.NotNil(t, otlpMetrics)
	require.Len(t, otlpMetrics.ResourceMetrics, 1)
	metric := otlpMetrics.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
	require.NotNil(t, metric.GetSum(), "counter-patterns should classify matching values as Sum after Init")
	assert.True(t, metric.GetSum().IsMonotonic)
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
	otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
	otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
			otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
			otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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
	otlpMetrics := output.convertToOTLP(output.state.Load().cfg, []*formatters.EventMsg{event})

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

	state := output.state.Load()
	require.NotNil(t, state, "outputState should be created")
	gs := state.transport.grpcState
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

// TestOTLP_InitCompilesCounterPatterns verifies that Init (the subscribe path)
// compiles counter-patterns into counterRegexes so that isCounter honors them at
// runtime. Before the fix, Init only called setDefaultsFor and stored the config
// without validateConfig, leaving counterRegexes nil so every metric exported as a
// Gauge in subscribe mode.
func TestOTLP_InitCompilesCounterPatterns(t *testing.T) {
	cfg := map[string]interface{}{
		"endpoint":         "unreachable-host:4317",
		"protocol":         "grpc",
		"timeout":          "1s",
		"counter-patterns": []string{"counters", "octets|packets"},
	}

	output := &otlpOutput{}
	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err, "Init should succeed with valid counter-patterns")
	defer output.Close()

	stored := output.loadCfg()
	require.NotNil(t, stored)
	require.NotNil(t, stored.counterRegexes, "counterRegexes should be compiled by Init")
	assert.Len(t, stored.counterRegexes, 2)

	assert.True(t, output.isCounter(stored, "out-octets"), "matching key should be treated as a counter")
	assert.False(t, output.isCounter(stored, "oper-status"), "non-matching key should not be a counter")
}

// TestOTLP_InitEmptyCounterPatterns verifies the default behavior is preserved:
// with no counter-patterns, counterRegexes is non-nil but empty and every key is a
// Gauge.
func TestOTLP_InitEmptyCounterPatterns(t *testing.T) {
	cfg := map[string]interface{}{
		"endpoint": "unreachable-host:4317",
		"protocol": "grpc",
		"timeout":  "1s",
	}

	output := &otlpOutput{}
	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err)
	defer output.Close()

	stored := output.loadCfg()
	require.NotNil(t, stored)
	require.NotNil(t, stored.counterRegexes)
	assert.Len(t, stored.counterRegexes, 0)
	assert.False(t, output.isCounter(stored, "out-octets"))
}

// TestOTLP_InitInvalidCounterPattern verifies that an invalid regex in
// counter-patterns causes Init to fail cleanly, matching the Update/Validate paths.
func TestOTLP_InitInvalidCounterPattern(t *testing.T) {
	cfg := map[string]interface{}{
		"endpoint":         "unreachable-host:4317",
		"protocol":         "grpc",
		"timeout":          "1s",
		"counter-patterns": []string{"["},
	}

	output := &otlpOutput{}
	err := output.Init(context.Background(), "test-otlp", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	assert.Error(t, err, "Init should fail on an invalid counter-pattern")
}

// Decision-path: PartialSuccess on gRPC must NOT be retried (parity with HTTP).
// Drives the public sendBatch path through the gRPC mock server.
func TestSendBatch_GRPCPartialSuccessNotRetried(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	mock := &mockPartialSuccessServer{rejected: 5}
	metricsv1.RegisterMetricsServiceServer(server, mock)
	go server.Serve(listener)
	defer server.Stop()

	o := &otlpOutput{}
	cfgMap := map[string]interface{}{
		"type":        "otlp",
		"endpoint":    listener.Addr().String(),
		"protocol":    "grpc",
		"max-retries": 5,
		"timeout":     "2s",
		"batch-size":  1,
		"interval":    "50ms",
	}
	require.NoError(t, o.Init(context.Background(), "test-otlp", cfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	event := &formatters.EventMsg{
		Name:      "test",
		Timestamp: time.Now().UnixNano(),
		Tags:      map[string]string{"source": "10.1.1.1:6030"},
		Values:    map[string]interface{}{"value": int64(1)},
	}
	o.WriteEvent(context.Background(), event)

	// Poll up to 1s for at least one Export call.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if mock.callCount() > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	// Give a generous additional 200ms to detect any retry.
	time.Sleep(200 * time.Millisecond)

	require.Equal(t, int32(1), mock.callCount(), "gRPC PartialSuccess must not retry")
}

type mockPartialSuccessServer struct {
	metricsv1.UnimplementedMetricsServiceServer
	rejected int64
	calls    atomic.Int32
}

func (m *mockPartialSuccessServer) Export(ctx context.Context, req *metricsv1.ExportMetricsServiceRequest) (*metricsv1.ExportMetricsServiceResponse, error) {
	m.calls.Add(1)
	return &metricsv1.ExportMetricsServiceResponse{
		PartialSuccess: &metricsv1.ExportMetricsPartialSuccess{
			RejectedDataPoints: m.rejected,
			ErrorMessage:       "schema validation failed",
		},
	}, nil
}

func (m *mockPartialSuccessServer) callCount() int32 {
	return m.calls.Load()
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

// ---------------------------------------------------------------------------
// Public Update() reload-path tests (Tasks 16c)
//
// These tests drive the real Update(ctx, cfg) API end-to-end. The existing
// reload tests use withCfg / direct state.Swap shortcuts that bypass Update.
// ---------------------------------------------------------------------------

// TestUpdate_EndpointChange_RebuildsTransport verifies the rebuild path
// (needsTransportRebuild == true). The endpoint change forces a new transport
// to be allocated; the old transport pointer must differ from the new one, and
// data sent after the Update reaches server2, not server1.
func TestUpdate_EndpointChange_RebuildsTransport(t *testing.T) {
	server1, endpoint1 := startMockOTLPServer(t)
	defer server1.Stop()
	server2, endpoint2 := startMockOTLPServer(t)
	defer server2.Stop()

	cfg1 := map[string]interface{}{
		"endpoint":   endpoint1,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}
	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-rebuild", cfg1,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	s1 := o.state.Load()
	require.NotNil(t, s1)
	transport1 := s1.transport

	cfg2 := map[string]interface{}{
		"endpoint":   endpoint2,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}
	require.NoError(t, o.Update(context.Background(), cfg2))

	s2 := o.state.Load()
	require.NotNil(t, s2)
	require.NotSame(t, s1, s2, "Update must publish a new outputState")
	require.NotSame(t, transport1, s2.transport, "endpoint change must allocate a new transportState")

	// A batch sent after the Update must reach server2.
	o.WriteEvent(context.Background(), &formatters.EventMsg{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{"source": "x"}, Values: map[string]interface{}{"value": int64(1)},
	})
	require.Eventually(t, func() bool {
		return server2.ReceivedMetricsCount() > 0
	}, 2*time.Second, 20*time.Millisecond, "data must reach server2 after endpoint Update")

	// server1 must not have received the post-reload batch.
	require.Equal(t, 0, server1.ReceivedMetricsCount(), "server1 must not receive data after endpoint Update")
}

// TestUpdate_TwoStepReload_RealUpdateRespectsInFlightAcrossConfigOnly is the
// regression test for the WaitGroup-on-fresh-state bug (Tasks 15d/16a), driven
// via the public Update() API rather than direct state.Swap shortcuts. This
// ensures Update's branch logic, its Swap block, and its async cleanup goroutine
// are all exercised together.
//
// Step 1: cfg-only reload (no transport rebuild) → new outputState, same transport.
// Step 2: endpoint change (rebuild) → new transport. The cleanup goroutine for
// the original transport must block on inFlight until we release it.
func TestUpdate_TwoStepReload_RealUpdateRespectsInFlightAcrossConfigOnly(t *testing.T) {
	server1, endpoint1 := startMockOTLPServer(t)
	defer server1.Stop()
	server2, endpoint2 := startMockOTLPServer(t)
	defer server2.Stop()

	cfg1 := map[string]interface{}{
		"endpoint":   endpoint1,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}
	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-two-step", cfg1,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	s1 := o.state.Load()
	T1 := s1.transport

	// Simulate an in-flight batch holding T1's WaitGroup counter.
	o.stateMu.Lock()
	T1.inFlight.Add(1)
	o.stateMu.Unlock()

	// Step 1: cfg-only reload (header change only → needsTransportRebuild == false).
	cfgOnly := map[string]interface{}{
		"endpoint":   endpoint1,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
		"headers":    map[string]interface{}{"X-Scope-OrgID": "step1"},
	}
	require.NoError(t, o.Update(context.Background(), cfgOnly))

	s2 := o.state.Load()
	require.NotSame(t, s1, s2, "cfg-only reload must publish a new outputState")
	require.Same(t, T1, s2.transport, "cfg-only reload must share the original transport")

	// Step 2: rebuild reload (endpoint changes → needsTransportRebuild == true).
	cfg2 := map[string]interface{}{
		"endpoint":   endpoint2,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}
	require.NoError(t, o.Update(context.Background(), cfg2))

	s3 := o.state.Load()
	require.NotSame(t, T1, s3.transport, "rebuild reload must allocate a new transport")

	// T1 is being cleaned up asynchronously, but inFlight is still 1.
	// Give the cleanup goroutine a brief window to start, then assert the
	// connection is NOT yet closed (blocked on inFlight.Wait).
	time.Sleep(100 * time.Millisecond)
	connState := T1.grpcState.conn.GetState()
	require.NotEqual(t, connectivity.Shutdown, connState,
		"T1 conn must not be closed while inFlight > 0")

	// Release the in-flight hold — cleanup goroutine should now drain.
	T1.inFlight.Done()

	// Poll for Shutdown with a generous timeout.
	require.Eventually(t, func() bool {
		return T1.grpcState.conn.GetState() == connectivity.Shutdown
	}, 2*time.Second, 25*time.Millisecond, "T1 conn must transition to Shutdown after inFlight drains")
}

// TestUpdate_BufferSizeChange_SwapsEventChannel verifies that
// channelNeedsSwap == true when buffer-size changes: the event channel must be
// replaced with one of the new capacity.
func TestUpdate_BufferSizeChange_SwapsEventChannel(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg1 := map[string]interface{}{
		"endpoint":    endpoint,
		"protocol":    "grpc",
		"timeout":     "2s",
		"batch-size":  1,
		"interval":    "50ms",
		"buffer-size": 10,
	}
	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-buf-swap", cfg1,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	oldCh := *o.eventCh.Load()
	require.Equal(t, 10, cap(oldCh), "initial channel capacity must match buffer-size")

	cfg2 := map[string]interface{}{
		"endpoint":    endpoint,
		"protocol":    "grpc",
		"timeout":     "2s",
		"batch-size":  1,
		"interval":    "50ms",
		"buffer-size": 32,
	}
	require.NoError(t, o.Update(context.Background(), cfg2))

	newCh := *o.eventCh.Load()
	require.Equal(t, 32, cap(newCh), "channel capacity must update to new buffer-size")
	// Capacity change is the load-bearing assertion; different cap guarantees
	// a new channel was allocated.
	require.NotEqual(t, cap(oldCh), cap(newCh), "Update must allocate a new channel when buffer-size changes")
}

// TestUpdate_NumWorkersChange_RestartsWorkers verifies needsWorkerRestart == true
// when num-workers changes. After the Update the pipeline must still be
// functional (data flows end-to-end).
func TestUpdate_NumWorkersChange_RestartsWorkers(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg1 := map[string]interface{}{
		"endpoint":    endpoint,
		"protocol":    "grpc",
		"timeout":     "2s",
		"batch-size":  1,
		"interval":    "50ms",
		"num-workers": 2,
	}
	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-worker-restart", cfg1,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	cfg2 := map[string]interface{}{
		"endpoint":    endpoint,
		"protocol":    "grpc",
		"timeout":     "2s",
		"batch-size":  1,
		"interval":    "50ms",
		"num-workers": 4,
	}
	require.NoError(t, o.Update(context.Background(), cfg2))

	// cancelFn must not be nil after worker restart.
	require.NotNil(t, o.cancelFn, "cancelFn must not be nil after worker restart")

	// Pipeline must still work after the restart — data flows end-to-end.
	countBefore := server.ReceivedMetricsCount()
	o.WriteEvent(context.Background(), &formatters.EventMsg{
		Name: "test", Timestamp: time.Now().UnixNano(),
		Tags: map[string]string{"source": "x"}, Values: map[string]interface{}{"value": int64(1)},
	})
	require.Eventually(t, func() bool {
		return server.ReceivedMetricsCount() > countBefore
	}, 2*time.Second, 20*time.Millisecond, "data must flow end-to-end after worker-count Update")
}

// TestOTLP_HTTPTransport_EndToEnd verifies that configuring protocol: http
// results in a working mTLS-authenticated POST to /v1/metrics carrying a
// valid ExportMetricsServiceRequest built from a real gnmi Event.
//
// Default-on gzip is exercised: the body arrives Content-Encoding: gzip and
// the test decompresses before unmarshal — same path real receivers use.
func TestOTLP_HTTPTransport_EndToEnd(t *testing.T) {
	var (
		mu          sync.Mutex
		gotBody     []byte
		gotEncoding string
	)
	srv := newMTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/metrics", r.URL.Path)
		require.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		mu.Lock()
		defer mu.Unlock()
		gotEncoding = r.Header.Get("Content-Encoding")
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = b
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	o := &otlpOutput{}
	cfgMap := map[string]interface{}{
		"type":       "otlp",
		"endpoint":   srv.URL,
		"protocol":   "http",
		"batch-size": 1,
		"interval":   "50ms",
		"timeout":    "2s",
		"tls": map[string]interface{}{
			"ca-file":   srv.CAPath(),
			"cert-file": srv.ClientCertPath(),
			"key-file":  srv.ClientKeyPath(),
		},
	}
	require.NoError(t, o.Init(context.Background(), "test-otlp", cfgMap,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	event := &formatters.EventMsg{
		Name:      "interfaces_interface_state_counters_in_octets",
		Timestamp: time.Now().UnixNano(),
		Tags:      map[string]string{"interface_name": "Ethernet1", "source": "10.1.1.1:6030"},
		Values:    map[string]interface{}{"value": int64(1234567890)},
	}
	// WriteEvent (not Write) — Write takes proto.Message and handles SubscribeResponse;
	// EventMsg is the input form for the batching pipeline.
	o.WriteEvent(context.Background(), event)

	// Poll up to 2 seconds for the body to arrive.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := gotBody != nil
		mu.Unlock()
		if got {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	mu.Lock()
	body := gotBody
	encoding := gotEncoding
	mu.Unlock()
	require.NotNil(t, body, "server did not receive a request within deadline")

	// Default compression for protocol:http is gzip — decompress before unmarshal.
	if encoding == "gzip" {
		gr, err := gzip.NewReader(bytes.NewReader(body))
		require.NoError(t, err)
		body, err = io.ReadAll(gr)
		require.NoError(t, err)
	}

	var req metricsv1.ExportMetricsServiceRequest
	require.NoError(t, proto.Unmarshal(body, &req))
	require.NotEmpty(t, req.ResourceMetrics, "ResourceMetrics must be populated")
}

// TestInit_FailureAfterRegisterMetrics_UnregistersCleanly verifies that when
// Init fails after registerMetrics succeeds, the collectors are unregistered so
// that a subsequent Init retry on the same *otlpOutput and same registry does
// not collide.
func TestInit_FailureAfterRegisterMetrics_UnregistersCleanly(t *testing.T) {
	reg := prometheus.NewRegistry()

	// A bad TLS path causes buildOutputState→initHTTPFor to fail after
	// registerMetrics has already registered the collectors.
	badCfg := map[string]interface{}{
		"endpoint": "http://127.0.0.1:9999",
		"protocol": "http",
		"tls": map[string]interface{}{
			"ca-file": "/nonexistent/ca.pem", // guaranteed to fail
		},
		"enable-metrics": true,
	}

	o := &otlpOutput{}

	// First Init: must fail (bad TLS).
	err := o.Init(context.Background(), "test-cleanup", badCfg,
		outputs.WithRegistry(reg),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.Error(t, err, "first Init must fail (bad TLS path)")

	// After failure the collectors must NOT still be registered.
	// If they are, reg.Register would return an AlreadyRegisteredError.
	require.NoError(t, reg.Register(otlpNumberOfSentEvents),
		"otlpNumberOfSentEvents must be unregistered after Init failure")
	// Clean up so the package-level variable is not left registered in this registry.
	reg.Unregister(otlpNumberOfSentEvents)

	// Second Init with a valid config on the same *otlpOutput must succeed.
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	goodCfg := map[string]interface{}{
		"endpoint":       endpoint,
		"protocol":       "grpc",
		"enable-metrics": true,
	}
	reg2 := prometheus.NewRegistry()
	err = o.Init(context.Background(), "test-cleanup", goodCfg,
		outputs.WithRegistry(reg2),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	)
	require.NoError(t, err, "second Init retry must succeed without duplicate-registration error")
	defer o.Close()
}

// TestRegisterMetrics_PartialFailure_DoesNotRemoveExternallyOwnedCollectors
// verifies that the per-instance ownership tracking means rollback only removes
// collectors this instance registered — not the externally pre-registered one
// that caused the failure.
func TestRegisterMetrics_PartialFailure_DoesNotRemoveExternallyOwnedCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()

	// Pre-register otlpSendDuration (the third collector in registerMetrics order)
	// to simulate another owner having registered it before us.
	require.NoError(t, reg.Register(otlpSendDuration))

	o := &otlpOutput{}
	o.reg = reg

	err := o.registerMetrics(&config{EnableMetrics: true, Name: "x"})
	require.Error(t, err, "registerMetrics must fail because otlpSendDuration is pre-registered")

	// 1. The first two collectors (which this instance registered) must have been
	//    rolled back — re-registering them must succeed.
	require.NoError(t, reg.Register(otlpNumberOfSentEvents),
		"otlpNumberOfSentEvents must be unregistered after partial failure (owned by this instance)")
	require.NoError(t, reg.Register(otlpNumberOfFailedEvents),
		"otlpNumberOfFailedEvents must be unregistered after partial failure (owned by this instance)")

	// 2. otlpSendDuration was NOT registered by this instance, so the rollback
	//    must NOT have removed it — re-registering it must return AlreadyRegisteredError.
	err = reg.Register(otlpSendDuration)
	require.Error(t, err, "otlpSendDuration must still be registered (externally owned, rollback must not touch it)")
	var alreadyErr prometheus.AlreadyRegisteredError
	require.ErrorAs(t, err, &alreadyErr,
		"error must be AlreadyRegisteredError — otlpSendDuration survived the rollback")

	// 3. registeredMetrics must be nil after rollback (drained).
	require.Nil(t, o.registeredMetrics, "registeredMetrics must be nil after rollback")

	// Cleanup.
	reg.Unregister(otlpNumberOfSentEvents)
	reg.Unregister(otlpNumberOfFailedEvents)
	reg.Unregister(otlpSendDuration)
}

// TestClose_WaitsForInFlightBatchBeforeCleanup verifies that Close blocks on
// transport.inFlight.Wait() before calling cleanup(). This ensures a future
// sendBatch caller outside the worker path cannot observe a torn-down transport.
func TestClose_WaitsForInFlightBatchBeforeCleanup(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}

	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-close-inflight", cfg,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))

	// Simulate an in-flight batch by manually incrementing inFlight under stateMu
	// (the same protocol sendBatch uses).
	o.stateMu.Lock()
	state := o.state.Load()
	state.transport.inFlight.Add(1)
	o.stateMu.Unlock()

	// Call Close in a goroutine; it should block on inFlight.Wait().
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		o.Close()
	}()

	// Assert Close has NOT returned within 50 ms (it must be blocked on Wait).
	select {
	case <-closeDone:
		t.Fatal("Close returned before inFlight was drained — Wait is not load-bearing")
	case <-time.After(50 * time.Millisecond):
		// expected: Close is still blocked
	}

	// Release the in-flight hold — Close must now proceed.
	state.transport.inFlight.Done()

	// Assert Close returns within 2 s.
	select {
	case <-closeDone:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return within 2s after inFlight was drained")
	}
}

// TestClose_UnregistersMetrics verifies that after a successful Init+Close cycle
// the collectors are unregistered, so a subsequent Init does not hit a
// duplicate-registration error.
func TestClose_UnregistersMetrics(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	reg := prometheus.NewRegistry()

	cfg := map[string]interface{}{
		"endpoint":       endpoint,
		"protocol":       "grpc",
		"enable-metrics": true,
	}

	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-close-metrics", cfg,
		outputs.WithRegistry(reg),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))

	require.NoError(t, o.Close())

	// After Close, all collectors must be unregistered.
	require.NoError(t, reg.Register(otlpNumberOfSentEvents),
		"otlpNumberOfSentEvents must be unregistered after Close")
	require.NoError(t, reg.Register(otlpNumberOfFailedEvents),
		"otlpNumberOfFailedEvents must be unregistered after Close")
	require.NoError(t, reg.Register(otlpSendDuration),
		"otlpSendDuration must be unregistered after Close")
	require.NoError(t, reg.Register(otlpRejectedDataPoints),
		"otlpRejectedDataPoints must be unregistered after Close")
	require.NoError(t, reg.Register(otlpMalformedResponses),
		"otlpMalformedResponses must be unregistered after Close")
	// Cleanup.
	reg.Unregister(otlpNumberOfSentEvents)
	reg.Unregister(otlpNumberOfFailedEvents)
	reg.Unregister(otlpSendDuration)
	reg.Unregister(otlpRejectedDataPoints)
	reg.Unregister(otlpMalformedResponses)
}

// TestUpdate_TransportRebuildFailure_DoesNotApplyProcessors verifies that when
// Update's buildOutputState fails, o.dynCfg is NOT updated. In the buggy code,
// o.dynCfg.Store(dc) ran before buildOutputState, so a new dc pointer was always
// published regardless of whether the transport build succeeded.
//
// We test with a protocol change (grpc → http) and an ftp:// endpoint that passes
// validateConfig but is rejected by initHTTPFor's resolveMetricsURL (unsupported
// scheme). Event-processors remain unchanged so rebuildProcessors is false — the
// test focuses on the transport-failure branch.
func TestUpdate_TransportRebuildFailure_DoesNotApplyProcessors(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg1 := map[string]interface{}{
		"endpoint":   endpoint,
		"protocol":   "grpc",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}

	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "test-dynCfg-no-apply", cfg1,
		outputs.WithLogger(logging.DiscardLogger()),
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))
	defer o.Close()

	// Capture baseline state pointer and dynCfg pointer.
	s1 := o.state.Load()
	prevDC := o.dynCfg.Load()
	require.NotNil(t, s1)
	require.NotNil(t, prevDC)

	// Attempt Update: switch to http with an ftp:// endpoint.
	// This passes validateConfig (protocol "http" is valid) but fails in
	// initHTTPFor → resolveMetricsURL with "unsupported endpoint scheme".
	badCfg := map[string]interface{}{
		"endpoint":   "ftp://127.0.0.1:4318",
		"protocol":   "http",
		"timeout":    "2s",
		"batch-size": 1,
		"interval":   "50ms",
	}
	err := o.Update(context.Background(), badCfg)

	// 1. Update must return an error.
	require.Error(t, err, "Update must fail on ftp:// endpoint scheme")
	require.Contains(t, err.Error(), "unsupported endpoint scheme")

	// 2. dynCfg must NOT have been updated — pointer identity must be preserved.
	require.Same(t, prevDC, o.dynCfg.Load(),
		"dynCfg must not be updated when transport rebuild fails")

	// 3. state must NOT have been updated — original state pointer must be preserved.
	require.Same(t, s1, o.state.Load(),
		"state must not be updated when transport rebuild fails")
}

// TestInit_GRPCBareEndpointWithoutPortRejected verifies that gRPC's bare-endpoint
// form requires an explicit port, mirroring the HTTP path's resolveMetricsURL
// discipline. A bare host like "panoptes" with no scheme and no port is rejected
// at Init rather than passed through to grpc.NewClient where it would silently
// resolve via the default resolver and dial the wrong port.
func TestInit_GRPCBareEndpointWithoutPortRejected(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{"no_port", "panoptes"},
		{"trailing_colon", "panoptes:"},
		{"port_only", ":4317"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &otlpOutput{}
			err := o.Init(context.Background(), "test-grpc-bare", map[string]any{
				"endpoint": tc.endpoint,
				"protocol": "grpc",
			},
				outputs.WithLogger(logging.DiscardLogger()),
				outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), "must be host:port")
		})
	}
}

// Security: String() feeds the reload log (Update logs the marshaled config
// at Info level), and Headers carries credentials such as Authorization
// bearer tokens. Values must be redacted — for every header, not just a
// blocklist — while key names stay visible for debugging. The live config
// must not be mutated by the redaction.
func TestString_RedactsHeaderValues(t *testing.T) {
	o := &otlpOutput{}
	o.initFields()
	headers := map[string]string{
		"Authorization": "Bearer super-secret-token",
		"X-Api-Key":     "another-secret-value",
		"X-Scope-OrgID": "tenant-1",
	}
	cfg := &config{Name: "redaction-test", Endpoint: "collector:4318", Headers: headers}
	o.state.Store(&outputState{cfg: cfg})

	s := o.String()
	require.NotEmpty(t, s)
	for k := range headers {
		require.Contains(t, s, k, "header names must stay visible")
	}
	for _, v := range headers {
		require.NotContains(t, s, v, "header values must never appear in String()")
	}
	require.Contains(t, s, "***")

	// The redaction must operate on a copy.
	require.Equal(t, "Bearer super-secret-token", cfg.Headers["Authorization"],
		"live config must not be mutated")
}

// Config guard: a negative numeric value would otherwise panic at runtime —
// buffer-size in make(chan) and num-workers in wg.Add at Init; batch-size in
// make([]) and interval in time.NewTicker inside the worker goroutine, where
// an unrecovered panic kills the process. Negative timeout disables deadlines
// and negative max-retries silently drops every batch. All must be rejected
// at validation time. Zero is not covered here: setDefaultsFor replaces zero
// with the default before validateConfig runs.
func TestValidateConfig_RejectsInvalidNumerics(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config)
		want   string
	}{
		{"negative batch-size", func(c *config) { c.BatchSize = -1 }, "batch-size"},
		{"negative buffer-size", func(c *config) { c.BufferSize = -1 }, "buffer-size"},
		{"negative num-workers", func(c *config) { c.NumWorkers = -1 }, "num-workers"},
		{"negative interval", func(c *config) { c.Interval = -time.Second }, "interval"},
		{"negative timeout", func(c *config) { c.Timeout = -time.Second }, "timeout"},
		{"negative max-retries", func(c *config) { c.MaxRetries = -1 }, "max-retries"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &otlpOutput{}
			c := &config{Endpoint: "collector:4317"}
			tc.mutate(c)
			o.setDefaultsFor(c)
			err := o.validateConfig(c)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}

	// Sanity: defaults alone validate cleanly.
	o := &otlpOutput{}
	c := &config{Endpoint: "collector:4317"}
	o.setDefaultsFor(c)
	require.NoError(t, o.validateConfig(c))
}

// Race regression: the outputs manager dispatches Write/WriteEvent goroutines
// without synchronizing against Close (go mgr.write(e) in outputs_manager.go),
// so producers can still be sending while — and after — Close runs. Close
// must not close the event channel (send on closed channel panics the whole
// process even inside a select) and Write/WriteEvent must tolerate the
// post-Close nil state. Run under -race, this also proves the paths are
// data-race free.
func TestClose_ConcurrentWriteEventDoesNotPanic(t *testing.T) {
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	cfg := map[string]interface{}{
		"endpoint": endpoint,
		"protocol": "grpc",
		"interval": "50ms",
	}
	o := &otlpOutput{}
	require.NoError(t, o.Init(context.Background(), "close-race", cfg,
		outputs.WithConfigStore(gomap.NewMemStore(store.StoreOptions[any]{})),
	))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ev := createTestEvent()
			for {
				select {
				case <-stop:
					return
				default:
					o.WriteEvent(context.Background(), ev)
				}
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, o.Close())
	// Producers keep hammering after Close — must neither panic nor race.
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()

	require.NoError(t, o.Close(), "Close must stay idempotent")
}
