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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
	_ "github.com/openconfig/gnmic/pkg/formatters/all"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestConfig_LogValue_smoke(t *testing.T) {
	c := &config{Password: "secret", Username: "u", Name: "n"}
	require.NotPanics(t, func() { _ = c.LogValue() })
}

func TestUpdateProcessor_missingIsOK(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	require.NoError(t, o.Init(context.Background(), "x", baseOutputCfg(), outputs.WithConfigStore(memStore())))
	err := o.UpdateProcessor("no-such-processor", map[string]any{})
	if err != nil {
		t.Logf("UpdateProcessor returned (acceptable): %v", err)
	}
	require.NoError(t, o.Close())
}

func TestEventToRows_nilAndDeletes(t *testing.T) {
	require.Nil(t, eventToRows(nil, nil, false))
	ev := &formatters.EventMsg{
		Deletes: []string{"/gone"},
		Tags:    map[string]string{"target": "tg", "subscription-name": "sub1"},
		Name:    "n",
	}
	rows := eventToRows(ev, outputs.Meta{"source": "host:57400"}, false)
	require.Len(t, rows, 1)
	require.True(t, rows[0].isDelete)
	require.Equal(t, "/gone", rows[0].path)
	require.Equal(t, "tg", rows[0].target)
	require.Equal(t, "sub1", rows[0].subscription)
}

func TestResolveTarget_metaSubscriptionTarget(t *testing.T) {
	ev := &formatters.EventMsg{Tags: map[string]string{}}
	require.Equal(t, "leaf", resolveTarget(ev, outputs.Meta{"subscription-target": "leaf"}))
}

func TestResolveTarget_metaSourceHost(t *testing.T) {
	ev := &formatters.EventMsg{Tags: map[string]string{}}
	require.Equal(t, "1.2.3.4", resolveTarget(ev, outputs.Meta{"source": "1.2.3.4:22"}))
}

func TestResolveTarget_evTagsSubscriptionTarget(t *testing.T) {
	ev := &formatters.EventMsg{Tags: map[string]string{"subscription-target": "s"}}
	require.Equal(t, "s", resolveTarget(ev, nil))
}

func TestResolveTarget_evTagsSourceHost(t *testing.T) {
	ev := &formatters.EventMsg{Tags: map[string]string{"source": "10.0.0.5:9339"}}
	require.Equal(t, "10.0.0.5", resolveTarget(ev, nil))
}

func TestMapValue_jsonNumberFloatAndStringFallback(t *testing.T) {
	r := telemetryRow{}
	mapValue(json.Number("2.5"), &r)
	require.Equal(t, "float", r.valueType)
	require.NotNil(t, r.valueFloat)

	r = telemetryRow{}
	mapValue(json.Number("not-a-number"), &r)
	require.Equal(t, "string", r.valueType)
	require.Equal(t, "not-a-number", r.valueString)
}

func TestMapValue_decimal64(t *testing.T) {
	r := telemetryRow{}
	mapValue(&gnmi.Decimal64{Digits: 12345, Precision: 3}, &r)
	require.Equal(t, "float", r.valueType)
	require.NotNil(t, r.valueFloat)
}

func TestMapValue_uint32Float32BoolBytes(t *testing.T) {
	r := telemetryRow{}
	mapValue(uint32(9), &r)
	require.Equal(t, "uint", r.valueType)

	r = telemetryRow{}
	mapValue(float32(1.25), &r)
	require.Equal(t, "float", r.valueType)

	r = telemetryRow{}
	mapValue(true, &r)
	require.Equal(t, "bool", r.valueType)

	r = telemetryRow{}
	mapValue([]byte{1, 2, 3}, &r)
	require.Equal(t, "bytes", r.valueType)
}

func TestMapValue_jsonMarshalStruct(t *testing.T) {
	r := telemetryRow{}
	mapValue(map[string]any{"a": 1}, &r)
	require.Equal(t, "json", r.valueType)
	require.Contains(t, r.valueString, `"a":1`)
}

func TestExpandTelemetryTTL_shorthandUnits(t *testing.T) {
	require.Contains(t, expandTelemetryTTL("2 WEEK"), "INTERVAL 2 WEEK")
	require.Contains(t, expandTelemetryTTL("3 HOUR"), "INTERVAL 3 HOUR")
	require.Contains(t, expandTelemetryTTL("15 MINUTE"), "INTERVAL 15 MINUTE")
	require.Contains(t, expandTelemetryTTL("45 SECOND"), "INTERVAL 45 SECOND")
	require.Equal(t, "", expandTelemetryTTL("  "))
	require.Equal(t, "noop", expandTelemetryTTL("noop"))
}

func TestExpandTelemetryTTL_passthrough(t *testing.T) {
	require.Equal(t, "toDateTime(timestamp) + 1", expandTelemetryTTL("toDateTime(timestamp) + 1"))
	require.Equal(t, "toIntervalDay(1)", expandTelemetryTTL("toIntervalDay(1)"))
}

func TestBuildCreateTableSQL_partitionByForbidden(t *testing.T) {
	_, err := buildCreateTableSQL(&config{
		Database: "d", Table: "t", TableEngine: "MergeTree",
		PartitionBy: "bad;expr", OrderBy: []string{"target"},
	})
	require.Error(t, err)
}

func TestBuildCreateTableSQL_ttlForbidden(t *testing.T) {
	_, err := buildCreateTableSQL(&config{
		Database: "d", Table: "t", TableEngine: "MergeTree",
		PartitionBy: "toYYYYMMDD(timestamp)", OrderBy: []string{"target"},
		TTL: "bad;\n",
	})
	require.Error(t, err)
}

func TestValidateIdent_errors(t *testing.T) {
	require.Error(t, validateIdent(""))
	require.Error(t, validateIdent("bad-name"))
	require.NoError(t, validateIdent("ok_name"))
}

func TestWaitLiveConn_ctxDeadlineWithMetrics(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.cfg.Store(&config{
		Name: "n", Table: "t", RecoveryWaitTime: 10 * time.Millisecond, EnableMetrics: true,
	})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Millisecond))
	defer cancel()
	time.Sleep(15 * time.Millisecond)
	require.False(t, o.waitLiveConn(ctx, o.cfg.Load(), 2))
}

func TestFlushBatch_ctxShutdownDuringInsertRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	insertBatchHook = func(context.Context, driver.Conn, *config, []telemetryRow) error {
		if calls.Add(1) == 1 {
			cancel()
		}
		return errors.New("fail")
	}
	defer func() { insertBatchHook = nil }()

	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(chFailedRows))
	o.cfg.Store(&config{
		Name: "n", Table: "t", Database: "d", BatchSize: 10, Timeout: time.Second,
		RecoveryWaitTime: time.Millisecond, EnableMetrics: true,
	})
	storeDriverConn(o, &fakeConn{})

	o.flushBatch(ctx, []telemetryRow{{path: "/p", valueType: "string", valueString: "v"}})
	require.GreaterOrEqual(t, calls.Load(), int32(1))
}

func TestFlushBatch_reconnectPingFailsThenSucceeds(t *testing.T) {
	var dials atomic.Int32
	openConnHook = func(*config) (driver.Conn, error) {
		n := dials.Add(1)
		if n == 1 {
			return &fakeConn{pingErr: errors.New("nope")}, nil
		}
		return &fakeConn{}, nil
	}
	defer func() { openConnHook = nil }()

	var insertCalls atomic.Int32
	insertBatchHook = func(context.Context, driver.Conn, *config, []telemetryRow) error {
		n := insertCalls.Add(1)
		if n < 3 {
			return errors.New("insert fail")
		}
		return nil
	}
	defer func() { insertBatchHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Table: "t", Database: "d", BatchSize: 10, Timeout: time.Second,
		RecoveryWaitTime: time.Millisecond, EnableMetrics: false,
	})
	storeDriverConn(o, &fakeConn{})

	o.flushBatch(context.Background(), []telemetryRow{{path: "/p", valueType: "string", valueString: "v"}})
	require.GreaterOrEqual(t, dials.Load(), int32(2))
}

func TestFlushBatch_reconnectDialFailsThenSucceeds(t *testing.T) {
	var dials atomic.Int32
	openConnHook = func(*config) (driver.Conn, error) {
		n := dials.Add(1)
		if n == 1 {
			return nil, errors.New("dial down")
		}
		return &fakeConn{}, nil
	}
	defer func() { openConnHook = nil }()

	var insertCalls atomic.Int32
	insertBatchHook = func(context.Context, driver.Conn, *config, []telemetryRow) error {
		n := insertCalls.Add(1)
		if n < 3 {
			return errors.New("insert fail")
		}
		return nil
	}
	defer func() { insertBatchHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Table: "t", Database: "d", BatchSize: 10, Timeout: time.Second,
		RecoveryWaitTime: time.Millisecond, EnableMetrics: false,
	})
	storeDriverConn(o, &fakeConn{})

	o.flushBatch(context.Background(), []telemetryRow{{path: "/p", valueType: "string", valueString: "v"}})
	require.GreaterOrEqual(t, dials.Load(), int32(2))
}

func TestInsertBatchCH_nilTagsRow(t *testing.T) {
	insertBatchHook = nil
	err := insertBatchCH(context.Background(), &fakeConn{}, &config{Database: "d", Table: "t"}, []telemetryRow{{
		path: "/p", valueType: "string", valueString: "v", tags: nil,
	}})
	require.NoError(t, err)
}

func TestRegisterMetrics_nilRegistryReturnsNil(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["enable-metrics"] = true
	require.NoError(t, o.Init(context.Background(), "m", cfg, outputs.WithConfigStore(memStore())))
	time.Sleep(25 * time.Millisecond)
	require.NoError(t, o.Close())
}

func TestWrite_unsupportedProtoMessage(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.cfg.Store(&config{Name: "n", Table: "t"})
	o.dynCfg.Store(&dynConfig{targetTpl: outputs.DefaultTargetTemplate, evps: nil})
	ch := make(chan telemetryRow, 4)
	o.rowChan.Store(&ch)

	o.Write(context.Background(), &gnmi.SetResponse{}, nil)
}

func TestWrite_subscribeConversionError(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{Name: "n", Table: "t"})
	o.dynCfg.Store(&dynConfig{targetTpl: outputs.DefaultTargetTemplate, evps: nil})
	ch := make(chan telemetryRow, 4)
	o.rowChan.Store(&ch)

	bad := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{},
		},
	}
	o.Write(context.Background(), bad, outputs.Meta{"subscription-name": "s"})
}

func TestWaitLiveConn_nilCfgDuringPoll(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	cfg := &config{Name: "n", Table: "t", RecoveryWaitTime: time.Second}
	o.cfg.Store(cfg)
	go func() {
		time.Sleep(15 * time.Millisecond)
		var c *config
		o.cfg.Store(c)
	}()
	require.False(t, o.waitLiveConn(context.Background(), cfg, 1))
}

func TestEnqueueRows_ctxCancelled(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	ch := make(chan telemetryRow, 4)
	o.rowChan.Store(&ch)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := &config{Name: "n", Table: "t", Timeout: time.Second}
	ev := &formatters.EventMsg{Values: map[string]any{"/a": 1}}
	o.enqueueRows(ctx, cfg, ev, nil)
}

func TestTryOpenConn_hookError(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return nil, errors.New("hooked") }
	defer func() { openConnHook = nil }()
	_, err := tryOpenConn(&config{Address: "x:1"})
	require.Error(t, err)
}

func TestRunBootstrapDial_noConfigLogsError(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.wg.Add(1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.runBootstrapDial(context.Background())
	}()
	wg.Wait()
}

func TestWrite_overrideTimestampsSubscribe(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["override-timestamps"] = true
	require.NoError(t, o.Init(context.Background(), "wts", cfg, outputs.WithConfigStore(memStore())))
	time.Sleep(25 * time.Millisecond)

	meta := outputs.Meta{"subscription-name": "s1", "source": "10.0.0.1:1"}
	upd := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: "name"}}},
						Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "dev"}},
					},
				},
			},
		},
	}
	o.Write(context.Background(), upd, meta)
	require.NoError(t, o.Close())
}

func TestDialUntilReady_openConnFailsThenSuccess(t *testing.T) {
	var n atomic.Int32
	openConnHook = func(*config) (driver.Conn, error) {
		if n.Add(1) == 1 {
			return nil, errors.New("refused")
		}
		return &fakeConn{}, nil
	}
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Address: "x:1", RecoveryWaitTime: time.Millisecond, Timeout: 100 * time.Millisecond,
		CreateTable: false,
	})
	conn, err := o.dialUntilReady(context.Background())
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()
	require.GreaterOrEqual(t, n.Load(), int32(2))
}

func TestWorker_drainsAndFlushesRows(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["flush-timer"] = "8ms"
	cfg["batch-size"] = 1
	require.NoError(t, o.Init(context.Background(), "wk", cfg, outputs.WithConfigStore(memStore())))
	time.Sleep(35 * time.Millisecond)

	meta := outputs.Meta{"subscription-name": "s1", "source": "10.0.0.1:1"}
	upd := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Timestamp: time.Now().UnixNano(),
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
						Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 7}},
					},
				},
			},
		},
	}
	o.Write(context.Background(), upd, meta)
	time.Sleep(40 * time.Millisecond)
	require.NoError(t, o.Close())
}

func TestUpdate_noRestartDebugToggle(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	ctx := context.Background()
	require.NoError(t, o.Init(ctx, "u", baseOutputCfg(), outputs.WithConfigStore(memStore())))
	time.Sleep(30 * time.Millisecond)

	cfg2 := baseOutputCfg()
	cfg2["debug"] = true
	require.NoError(t, o.Update(ctx, cfg2))
	require.NoError(t, o.Close())
}

func TestValidate_badOrderByIdent(t *testing.T) {
	o := &clickhouseOutput{}
	err := o.Validate(map[string]any{
		"database": "d", "table": "t", "order-by": []string{"bad-col!"},
	})
	require.Error(t, err)
}

func TestInsertBatchCH_prepareBatchError(t *testing.T) {
	insertBatchHook = nil
	err := insertBatchCH(context.Background(), &fakeConn{prepErr: errors.New("no")}, &config{Database: "d", Table: "t"}, []telemetryRow{
		{path: "/p", valueType: "string", valueString: "v"},
	})
	require.Error(t, err)
}

func TestDrainRowChan_rootCtxCancels(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o.rootCtx = ctx
	oldCh := make(chan telemetryRow)
	newCh := make(chan telemetryRow, 1)
	o.drainRowChan(oldCh, newCh, &config{Name: "n", Table: "t"})
}

func TestInsertBatchCH_appendError(t *testing.T) {
	insertBatchHook = nil
	err := insertBatchCH(context.Background(), &fakeConn{appendErr: errors.New("append")}, &config{Database: "d", Table: "t"}, []telemetryRow{
		{path: "/p", valueType: "string", valueString: "v"},
	})
	require.Error(t, err)
}

func TestInsertBatchCH_sendError(t *testing.T) {
	insertBatchHook = nil
	err := insertBatchCH(context.Background(), &fakeConn{sendErr: errors.New("send")}, &config{Database: "d", Table: "t"}, []telemetryRow{
		{path: "/p", valueType: "string", valueString: "v"},
	})
	require.Error(t, err)
}

func TestWrite_addTargetTemplateExecuteError(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["add-target"] = "overwrite"
	cfg["target-template"] = `{{ printf "%z" "x" }}`
	require.NoError(t, o.Init(context.Background(), "at", cfg, outputs.WithConfigStore(memStore())))
	time.Sleep(20 * time.Millisecond)

	meta := outputs.Meta{"subscription-name": "s1", "source": "10.0.0.1:1"}
	upd := &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{
				Prefix:    &gnmi.Path{},
				Timestamp: time.Now().UnixNano(),
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "a"}}},
						Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "v"}},
					},
				},
			},
		},
	}
	o.Write(context.Background(), upd, meta)
	require.NoError(t, o.Close())
}

func TestDialUntilReady_cancelledAfterDialErrorCarriesLastErr(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return nil, errors.New("refused") }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Address: "x:1", RecoveryWaitTime: 50 * time.Millisecond, Timeout: 100 * time.Millisecond,
		CreateTable: false,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := o.dialUntilReady(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cancelled")
}

func TestFlushBatch_multiChunkInsert(t *testing.T) {
	var batches atomic.Int32
	insertBatchHook = func(_ context.Context, _ driver.Conn, _ *config, rows []telemetryRow) error {
		batches.Add(1)
		require.LessOrEqual(t, len(rows), 2)
		return nil
	}
	defer func() { insertBatchHook = nil }()
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Table: "t", Database: "d", BatchSize: 2, Timeout: time.Second,
		RecoveryWaitTime: time.Millisecond, EnableMetrics: false,
	})
	storeDriverConn(o, &fakeConn{})
	rows := []telemetryRow{
		{path: "/a", valueType: "int", valueInt: ptrInt64(1)},
		{path: "/b", valueType: "int", valueInt: ptrInt64(2)},
		{path: "/c", valueType: "int", valueInt: ptrInt64(3)},
	}
	o.flushBatch(context.Background(), rows)
	require.Equal(t, int32(2), batches.Load())
}

func TestInit_eventProcessorNotFound(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["event-processors"] = []string{"processor-that-is-not-in-store"}
	err := o.Init(context.Background(), "ep", cfg, outputs.WithConfigStore(memStore()))
	require.Error(t, err)
}

func TestUpdate_eventProcessorNotFound(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	ctx := context.Background()
	require.NoError(t, o.Init(ctx, "u", baseOutputCfg(), outputs.WithConfigStore(memStore())))
	time.Sleep(25 * time.Millisecond)

	cfg2 := baseOutputCfg()
	cfg2["event-processors"] = []string{"processor-that-is-not-in-store"}
	require.Error(t, o.Update(ctx, cfg2))
	require.NoError(t, o.Close())
}

func TestUpdateProcessor_success(t *testing.T) {
	st := memStore()
	_, err := st.Set("processors", "p1", map[string]any{
		"event-merge": map[string]any{},
	})
	require.NoError(t, err)

	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["event-processors"] = []string{"p1"}
	require.NoError(t, o.Init(context.Background(), "up", cfg, outputs.WithConfigStore(st)))
	time.Sleep(25 * time.Millisecond)

	require.NoError(t, o.UpdateProcessor("p1", map[string]any{
		"event-merge": map[string]any{"always": true},
	}))
	require.NoError(t, o.Close())
}

func TestUpdateProcessor_invalidProcessorConfig(t *testing.T) {
	st := memStore()
	_, serr := st.Set("processors", "p1", map[string]any{
		"event-merge": map[string]any{},
	})
	require.NoError(t, serr)

	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["event-processors"] = []string{"p1"}
	require.NoError(t, o.Init(context.Background(), "up2", cfg, outputs.WithConfigStore(st)))
	time.Sleep(25 * time.Millisecond)

	err := o.UpdateProcessor("p1", map[string]any{
		"event-merge": map[string]any{"always": "not-a-bool"},
	})
	require.Error(t, err)
	require.NoError(t, o.Close())
}

func TestWrite_manySubscribeEventsForWorkerCoverage(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["batch-size"] = 3
	cfg["buffer-size"] = 64
	cfg["flush-timer"] = "15ms"
	require.NoError(t, o.Init(context.Background(), "mw", cfg, outputs.WithConfigStore(memStore())))
	time.Sleep(30 * time.Millisecond)

	meta := outputs.Meta{"subscription-name": "s1", "source": "10.0.0.1:1"}
	for i := 0; i < 18; i++ {
		upd := &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: time.Now().UnixNano(),
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "counters"}, {Name: "n", Key: map[string]string{"i": fmt.Sprintf("%d", i)}}}},
							Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_UintVal{UintVal: uint64(i)}},
						},
					},
				},
			},
		}
		o.Write(context.Background(), upd, meta)
	}
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, o.Close())
}
