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
