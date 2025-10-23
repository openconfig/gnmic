// © 2025 Drew Elliott
//
// This code is a Contribution to the gNMIc project ("Work") made under the Apache License 2.0.
//
// SPDX-License-Identifier: Apache-2.0

package otlp_output

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"

	"github.com/openconfig/gnmic/pkg/formatters"
)

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
	cfg := &config{
		Endpoint: "localhost:4317",
	}
	output := &otlpOutput{cfg: cfg}
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
	output := &otlpOutput{cfg: &config{}}
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
	output := &otlpOutput{cfg: &config{}}
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

			output := &otlpOutput{cfg: &config{}}
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
	t.Skip("Implementation pending")

	// Test OTLP gRPC transport
	// Given: Mock OTLP collector gRPC server
	server, endpoint := startMockOTLPServer(t)
	defer server.Stop()

	// When: Sending metrics via gRPC
	cfg := map[string]interface{}{
		"endpoint": endpoint,
		"protocol": "grpc",
		"timeout":  "5s",
	}
	output := &otlpOutput{}
	err := output.Init(context.Background(), "test-otlp", cfg)
	require.NoError(t, err)
	defer output.Close()

	// Send test event
	event := createTestEvent()
	output.WriteEvent(context.Background(), event)

	// Then: Metrics received by server
	time.Sleep(100 * time.Millisecond)
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
	cfg := &config{StringsAsAttributes: true}
	output := &otlpOutput{cfg: cfg}
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
	cfg := &config{AppendSubscriptionName: true}
	output := &otlpOutput{cfg: cfg}
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
	grpcServer   *grpc.Server
	listener     net.Listener
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

func (m *mockOTLPServer) Export(ctx context.Context, req *metricsv1.ExportMetricsServiceRequest) (*metricsv1.ExportMetricsServiceResponse, error) {
	m.receivedReqs = append(m.receivedReqs, req)
	m.metricsCount += len(req.ResourceMetrics)
	return &metricsv1.ExportMetricsServiceResponse{}, nil
}

func (m *mockOTLPServer) ReceivedMetricsCount() int {
	return m.metricsCount
}

func (m *mockOTLPServer) Stop() {
	m.grpcServer.Stop()
	m.listener.Close()
}
