// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/stretchr/testify/require"
)

func TestSetDefaults(t *testing.T) {
	c := &config{}
	require.NoError(t, setDefaults(c))
	require.Equal(t, defaultAddress, c.Address)
	require.Equal(t, defaultDatabase, c.Database)
	require.Equal(t, defaultTable, c.Table)
	require.Equal(t, defaultUsername, c.Username)
	require.Equal(t, defaultBatchSize, c.BatchSize)
	require.Equal(t, defaultFlushTimer, c.FlushTimer)
	require.Equal(t, defaultNumWorkers, c.NumWorkers)
	require.Equal(t, defaultBufferSize, c.BufferSize)
	require.Equal(t, defaultTableEngine, c.TableEngine)
	require.Equal(t, defaultPartitionBy, c.PartitionBy)
	require.Equal(t, []string{"target", "path", "timestamp"}, c.OrderBy)
}

func TestApplyTTLDefault(t *testing.T) {
	t.Run("default when key absent", func(t *testing.T) {
		c := &config{}
		applyTTLDefault(map[string]any{"address": "x"}, c)
		require.Equal(t, "30 DAY", c.TTL)
	})
	t.Run("explicit empty disables", func(t *testing.T) {
		c := &config{}
		applyTTLDefault(map[string]any{"ttl": ""}, c)
		require.Equal(t, "", c.TTL)
	})
	t.Run("explicit value preserved", func(t *testing.T) {
		c := &config{TTL: "60 DAY"}
		applyTTLDefault(map[string]any{"ttl": "60 DAY"}, c)
		require.Equal(t, "60 DAY", c.TTL)
	})
}

func TestApplyCreateTableDefault(t *testing.T) {
	t.Run("default true when absent", func(t *testing.T) {
		c := &config{}
		applyCreateTableDefault(map[string]any{}, c)
		require.True(t, c.CreateTable)
	})
	t.Run("explicit false", func(t *testing.T) {
		c := &config{}
		applyCreateTableDefault(map[string]any{"create-table": false}, c)
		require.False(t, c.CreateTable)
	})
}

func TestExpandTelemetryTTL(t *testing.T) {
	require.Equal(t, "toDateTime(timestamp) + INTERVAL 30 DAY", expandTelemetryTTL("30 DAY"))
	require.Equal(t, "toDateTime(timestamp) + INTERVAL 1 WEEK", expandTelemetryTTL("1 WEEK"))
	require.Equal(t, "toDateTime(timestamp) + INTERVAL 30 DAY", expandTelemetryTTL("toDateTime(timestamp) + INTERVAL 30 DAY"))
	require.Equal(t, "", expandTelemetryTTL(""))
}

func TestBuildCreateTableSQLTTL(t *testing.T) {
	cfg := &config{
		Database:    "telemetry",
		Table:       "gnmic_telemetry",
		TableEngine: "MergeTree",
		PartitionBy: "toYYYYMMDD(timestamp)",
		OrderBy:     []string{"target", "path", "timestamp"},
		TTL:         "",
	}
	sql, err := buildCreateTableSQL(cfg)
	require.NoError(t, err)
	require.NotContains(t, sql, "TTL")

	cfg.TTL = "30 DAY"
	sql, err = buildCreateTableSQL(cfg)
	require.NoError(t, err)
	require.Contains(t, sql, "TTL")
	require.Contains(t, sql, "toDateTime(timestamp) + INTERVAL 30 DAY")
}

func TestBuildCreateTableSQLRejectInjection(t *testing.T) {
	cfg := &config{
		Database:    "telemetry",
		Table:       "gnmic_telemetry",
		TableEngine: "MergeTree",
		PartitionBy: "toYYYYMMDD(timestamp); DROP TABLE x",
		OrderBy:     []string{"target"},
	}
	_, err := buildCreateTableSQL(cfg)
	require.Error(t, err)
}

func TestEventToRowsValueTypes(t *testing.T) {
	meta := outputs.Meta{
		"source":              "192.0.2.1:57400",
		"subscription-name":   "subA",
		"subscription-target": "router1",
	}
	ev := &formatters.EventMsg{
		Name:      "subA",
		Timestamp: 1_700_000_000_000_000_000,
		Tags:      map[string]string{"k": "v"},
		Values: map[string]any{
			"/p/int":    int64(42),
			"/p/uint":   uint64(99),
			"/p/float":  3.14,
			"/p/bool":   true,
			"/p/string": "hello",
			"/p/bytes":  []byte{1, 2, 3},
			"/p/json":   map[string]any{"a": 1},
		},
	}
	rows := eventToRows(ev, meta, false)
	require.Len(t, rows, 7)

	byPath := map[string]telemetryRow{}
	for _, r := range rows {
		byPath[r.path] = r
	}
	require.Equal(t, "int", byPath["/p/int"].valueType)
	require.NotNil(t, byPath["/p/int"].valueInt)
	require.Equal(t, int64(42), *byPath["/p/int"].valueInt)

	require.Equal(t, "uint", byPath["/p/uint"].valueType)
	require.Equal(t, uint64(99), *byPath["/p/uint"].valueUint)

	require.Equal(t, "float", byPath["/p/float"].valueType)
	require.InDelta(t, 3.14, *byPath["/p/float"].valueFloat, 1e-9)

	require.Equal(t, "bool", byPath["/p/bool"].valueType)
	require.True(t, *byPath["/p/bool"].valueBool)

	require.Equal(t, "string", byPath["/p/string"].valueType)
	require.Equal(t, "hello", byPath["/p/string"].valueString)

	require.Equal(t, "bytes", byPath["/p/bytes"].valueType)
	require.NotEmpty(t, byPath["/p/bytes"].valueString)

	require.Equal(t, "json", byPath["/p/json"].valueType)
	require.Contains(t, byPath["/p/json"].valueString, `"a"`)
}

func TestEventToRowsDeletes(t *testing.T) {
	ev := &formatters.EventMsg{
		Name:    "sub",
		Deletes: []string{"/interfaces/interface[name=eth0]/state"},
	}
	rows := eventToRows(ev, nil, false)
	require.Len(t, rows, 1)
	require.True(t, rows[0].isDelete)
	require.Equal(t, "/interfaces/interface[name=eth0]/state", rows[0].path)
	require.Equal(t, "", rows[0].valueType)
}

func TestEventToRowsTimestampOverride(t *testing.T) {
	ev := &formatters.EventMsg{
		Timestamp: 0,
		Values:    map[string]any{"/x": 1},
	}
	r := eventToRows(ev, nil, false)
	require.Len(t, r, 1)
	require.False(t, r[0].timestamp.IsZero())

	ev2 := &formatters.EventMsg{
		Timestamp: 1_700_000_000_000_000_000,
		Values:    map[string]any{"/x": 1},
	}
	r2 := eventToRows(ev2, nil, true)
	require.Len(t, r2, 1)
	require.WithinDuration(t, time.Now().UTC(), r2[0].timestamp, 2*time.Second)
}

func TestResolveTargetOrder(t *testing.T) {
	ev := &formatters.EventMsg{
		Tags: map[string]string{"target": "from-path"},
		Values: map[string]any{
			"/m": 1,
		},
	}
	meta := outputs.Meta{
		"subscription-target": "from-meta",
		"source":              "198.51.100.2:1234",
	}
	rows := eventToRows(ev, meta, false)
	require.Equal(t, "from-path", rows[0].target)

	ev2 := &formatters.EventMsg{
		Tags: map[string]string{},
		Values: map[string]any{
			"/m": 1,
		},
	}
	rows2 := eventToRows(ev2, meta, false)
	require.Equal(t, "from-meta", rows2[0].target)

	meta3 := outputs.Meta{"source": "198.51.100.2:1234"}
	rows3 := eventToRows(ev2, meta3, false)
	require.Equal(t, "198.51.100.2", rows3[0].target)
}

func TestInsertBatchHook(t *testing.T) {
	var calls int
	insertBatchHook = func(context.Context, driver.Conn, *config, []telemetryRow) error {
		calls++
		return errors.New("boom")
	}
	defer func() { insertBatchHook = nil }()

	cfg := &config{Name: "t", Database: "d", Table: "t", BatchSize: 10}
	rows := []telemetryRow{{path: "/x", valueType: "int", valueInt: ptrInt64(1)}}

	err := insertBatchCH(context.Background(), nil, cfg, rows)
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func ptrInt64(v int64) *int64 { return &v }
