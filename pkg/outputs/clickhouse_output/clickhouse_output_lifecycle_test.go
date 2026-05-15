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
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
	"google.golang.org/protobuf/proto"
)

func memStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func TestClickhouseOutput_String(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	require.Equal(t, "", o.String())
	cfg := &config{Name: "x", Address: "h:9"}
	o.cfg.Store(cfg)
	require.Contains(t, o.String(), "x")
}

func TestTlsEqual(t *testing.T) {
	require.True(t, tlsEqual(nil, nil))
	require.False(t, tlsEqual(nil, &types.TLSConfig{}))
	require.False(t, tlsEqual(&types.TLSConfig{}, nil))
	a := &types.TLSConfig{CaFile: "a", CertFile: "b", KeyFile: "c", SkipVerify: true}
	b := &types.TLSConfig{CaFile: "a", CertFile: "b", KeyFile: "c", SkipVerify: true}
	require.True(t, tlsEqual(a, b))
	require.False(t, tlsEqual(a, &types.TLSConfig{CaFile: "z"}))
}

func TestConnPollInterval(t *testing.T) {
	require.Equal(t, 100*time.Millisecond, connPollInterval(&config{RecoveryWaitTime: 0}))
	require.Equal(t, time.Second, connPollInterval(&config{RecoveryWaitTime: 200 * time.Second}))
	require.Equal(t, 500*time.Millisecond, connPollInterval(&config{RecoveryWaitTime: 5 * time.Second}))
}

func TestWait_zeroUsesOneSecond(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	require.NoError(t, wait(ctx, 0))
	require.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond)
}

func TestWaitLiveConn_cancel(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.cfg.Store(&config{Name: "n", Table: "t", RecoveryWaitTime: 15 * time.Millisecond, EnableMetrics: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.False(t, o.waitLiveConn(ctx, o.cfg.Load(), 2))
}

func storeDriverConn(o *clickhouseOutput, c driver.Conn) {
	o.conn.Store(&c)
}

func TestWaitLiveConn_ready(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	cfg := &config{Name: "n", Table: "t", RecoveryWaitTime: 10 * time.Millisecond, EnableMetrics: false}
	o.cfg.Store(cfg)
	storeDriverConn(o, &fakeConn{})
	require.True(t, o.waitLiveConn(context.Background(), cfg, 1))
}

func TestRunBootstrapDial_cancelledBeforeDial(t *testing.T) {
	var n atomic.Int32
	openConnHook = func(*config) (driver.Conn, error) {
		n.Add(1)
		return &fakeConn{}, nil
	}
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Address: "x:1", RecoveryWaitTime: time.Millisecond, Timeout: 50 * time.Millisecond,
		CreateTable: false,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	o.wg.Add(1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.runBootstrapDial(ctx)
	}()
	wg.Wait()
	require.Equal(t, int32(0), n.Load())
}

func TestRunBootstrapDial_successStoresConn(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) {
		return &fakeConn{}, nil
	}
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Address: "x:1", RecoveryWaitTime: time.Millisecond, Timeout: 50 * time.Millisecond,
		CreateTable: false,
	})

	o.wg.Add(1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.runBootstrapDial(context.Background())
	}()
	wg.Wait()

	p := o.conn.Load()
	require.NotNil(t, p)
	require.NotNil(t, *p)
}

func TestDialUntilReady_pingRetryThenSuccess(t *testing.T) {
	var n atomic.Int32
	openConnHook = func(*config) (driver.Conn, error) {
		if n.Add(1) < 3 {
			return &fakeConn{pingErr: errors.New("down")}, nil
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
	require.GreaterOrEqual(t, n.Load(), int32(3))
}

func TestDialUntilReady_execRetryThenSuccess(t *testing.T) {
	var n atomic.Int32
	openConnHook = func(*config) (driver.Conn, error) {
		if n.Add(1) == 1 {
			return &fakeConn{execErr: errors.New("ddl")}, nil
		}
		return &fakeConn{}, nil
	}
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Address: "x:1", Database: "default", Table: "t", TableEngine: "MergeTree",
		PartitionBy: "toYYYYMMDD(timestamp)", OrderBy: []string{"target"},
		RecoveryWaitTime: time.Millisecond, Timeout: 100 * time.Millisecond,
		CreateTable: true,
	})

	conn, err := o.dialUntilReady(context.Background())
	require.NoError(t, err)
	require.NotNil(t, conn)
	_ = conn.Close()
	require.GreaterOrEqual(t, n.Load(), int32(2))
}

func TestDialUntilReady_noConfig(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	_, err := o.dialUntilReady(context.Background())
	require.Error(t, err)
}

func baseOutputCfg() map[string]any {
	return map[string]any{
		"create-table":       false,
		"recovery-wait-time": "1ms",
		"flush-timer":        "20ms",
		"batch-size":         2,
		"num-workers":        1,
		"buffer-size":        16,
		"timeout":            "100ms",
		"address":            "127.0.0.1:9",
	}
}

func TestInitUpdateClose_withOpenConnHook(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	ctx := context.Background()
	require.NoError(t, o.Init(ctx, "out1", baseOutputCfg(), outputs.WithConfigStore(memStore())))

	time.Sleep(40 * time.Millisecond)
	p := o.conn.Load()
	require.NotNil(t, p)
	require.NotNil(t, *p)

	require.NoError(t, o.Update(ctx, baseOutputCfg()))

	cfg2 := baseOutputCfg()
	cfg2["buffer-size"] = 32
	require.NoError(t, o.Update(ctx, cfg2))

	require.NoError(t, o.Close())
}

func TestInit_decodeError(t *testing.T) {
	o := &clickhouseOutput{}
	err := o.Init(context.Background(), "x", map[string]any{"batch-size": "nope"}, outputs.WithConfigStore(memStore()))
	require.Error(t, err)
}

func TestInit_optionError(t *testing.T) {
	o := &clickhouseOutput{}
	bad := func(*outputs.OutputOptions) error { return errors.New("opt") }
	err := o.Init(context.Background(), "x", baseOutputCfg(), bad, outputs.WithConfigStore(memStore()))
	require.Error(t, err)
}

func TestInit_badTargetTemplate(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["add-target"] = "overwrite"
	cfg["target-template"] = "{{"
	err := o.Init(context.Background(), "x", cfg, outputs.WithConfigStore(memStore()))
	require.Error(t, err)
}

func TestUpdate_beforeInit(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.cfg.Store(&config{Name: "n", Address: "a:1", BufferSize: 8, NumWorkers: 1})
	err := o.Update(context.Background(), map[string]any{
		"address": "b:1", "buffer-size": 16, "num-workers": 1,
		"recovery-wait-time": "1ms", "flush-timer": "10ms", "batch-size": 2, "timeout": "50ms",
	})
	require.Error(t, err)
}

func TestFlushBatch_success(t *testing.T) {
	insertBatchHook = nil
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Table: "t", Database: "d", BatchSize: 10, Timeout: time.Second,
		RecoveryWaitTime: time.Millisecond, EnableMetrics: false,
	})
	storeDriverConn(o, &fakeConn{})

	o.flushBatch(context.Background(), []telemetryRow{
		{path: "/p", valueType: "int", valueInt: ptrInt64(7)},
	})
}

func TestFlushBatch_insertRetryThenSuccess(t *testing.T) {
	var calls atomic.Int32
	insertBatchHook = func(context.Context, driver.Conn, *config, []telemetryRow) error {
		if calls.Add(1) == 1 {
			return errors.New("insert fail")
		}
		return nil
	}
	defer func() { insertBatchHook = nil }()

	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	o.init()
	o.logger = slog.Default()
	o.cfg.Store(&config{
		Name: "n", Table: "t", Database: "d", BatchSize: 10, Timeout: time.Second,
		RecoveryWaitTime: time.Millisecond, EnableMetrics: true,
	})
	storeDriverConn(o, &fakeConn{})

	o.flushBatch(context.Background(), []telemetryRow{{path: "/p", valueType: "string", valueString: "v"}})
	require.GreaterOrEqual(t, calls.Load(), int32(2))
}

func TestInsertBatchCH_emptyRows(t *testing.T) {
	require.NoError(t, insertBatchCH(context.Background(), nil, &config{}, nil))
}

func TestEnqueueRows_fullBufferTimeout(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	ch := make(chan telemetryRow, 1)
	o.rowChan.Store(&ch)
	cfg := &config{Name: "n", Table: "t", Timeout: time.Nanosecond, Debug: true, EnableMetrics: true}
	ev := &formatters.EventMsg{Values: map[string]any{"/a": 1, "/b": 2}}
	ctx := context.Background()
	o.enqueueRows(ctx, cfg, ev, nil)
}

func TestWrite_nilAndUnknownProto(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.cfg.Store(&config{Name: "n", Table: "t"})
	o.dynCfg.Store(&dynConfig{targetTpl: outputs.DefaultTargetTemplate, evps: nil})
	tmp := make(chan telemetryRow, 4)
	o.rowChan.Store(&tmp)

	o.Write(context.Background(), nil, nil)
	o.Write(context.Background(), proto.Message(nil), nil)
}

func TestWrite_subscribeAndGet(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	require.NoError(t, o.Init(context.Background(), "w", baseOutputCfg(), outputs.WithConfigStore(memStore())))
	time.Sleep(30 * time.Millisecond)

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

	get := &gnmi.GetResponse{
		Notification: []*gnmi.Notification{
			{
				Timestamp: time.Now().UnixNano(),
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "system"}, {Name: "contact"}}},
						Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_StringVal{StringVal: "x"}},
					},
				},
			},
		},
	}
	o.Write(context.Background(), get, meta)
}

func TestWriteEvent_nil(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.WriteEvent(context.Background(), nil)
}

func TestWriteEvent_appliesProcessors(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	require.NoError(t, o.Init(context.Background(), "we", baseOutputCfg(), outputs.WithConfigStore(memStore())))
	time.Sleep(30 * time.Millisecond)

	o.WriteEvent(context.Background(), &formatters.EventMsg{
		Name:   "n",
		Values: map[string]any{"/k": int64(1)},
	})
}

func TestDrainRowChan_overflowMetric(t *testing.T) {
	o := &clickhouseOutput{}
	o.init()
	o.rootCtx = context.Background()
	cfg := &config{Name: "n", Table: "t", EnableMetrics: true}
	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(chDroppedRows))
	oldCh := make(chan telemetryRow, 2)
	newCh := make(chan telemetryRow)
	oldCh <- telemetryRow{path: "/a"}
	oldCh <- telemetryRow{path: "/b"}
	close(oldCh)
	o.drainRowChan(oldCh, newCh, cfg)
}

func TestValidate_errors(t *testing.T) {
	o := &clickhouseOutput{}
	require.Error(t, o.Validate(map[string]any{"batch-size": "x"}))
	require.Error(t, o.Validate(map[string]any{
		"database": "d", "table": "t", "partition-by": "x;y", "order-by": []string{"target"},
	}))
}

func TestValidate_badTargetTemplate(t *testing.T) {
	o := &clickhouseOutput{}
	err := o.Validate(map[string]any{"target-template": "{{invalid"})
	require.Error(t, err)
}

func TestRegisterMetrics_withRegistry(t *testing.T) {
	openConnHook = func(*config) (driver.Conn, error) { return &fakeConn{}, nil }
	defer func() { openConnHook = nil }()

	o := &clickhouseOutput{}
	cfg := baseOutputCfg()
	cfg["enable-metrics"] = true
	reg := prometheus.NewRegistry()
	require.NoError(t, o.Init(context.Background(), "m", cfg,
		outputs.WithConfigStore(memStore()),
		outputs.WithRegistry(reg),
	))
	time.Sleep(30 * time.Millisecond)
	require.NoError(t, o.Close())
}

func TestOpenConn_tlsSkipVerifyWithoutCAFiles(t *testing.T) {
	openConnHook = nil
	c := &config{
		Address: "127.0.0.1:1", Timeout: 10 * time.Millisecond, Username: "d", Database: "db",
		TLS: &types.TLSConfig{SkipVerify: true},
	}
	conn, err := openConn(c)
	require.NoError(t, err)
	require.NotNil(t, conn)
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	require.Error(t, conn.Ping(ctx))
	_ = conn.Close()
}

func TestOpenConn_tlsBadFiles(t *testing.T) {
	openConnHook = nil
	c := &config{
		Address: "127.0.0.1:1", Timeout: 10 * time.Millisecond,
		TLS: &types.TLSConfig{CaFile: "/no/such/ca.pem"},
	}
	_, err := openConn(c)
	require.Error(t, err)
}

func TestExpandTelemetryTTL_monthAndPassthrough(t *testing.T) {
	require.Contains(t, expandTelemetryTTL("6 MONTH"), "INTERVAL 6 MONTH")
	require.Equal(t, "custom + 1", expandTelemetryTTL("custom + 1"))
}

func TestBuildCreateTableSQL_badDatabase(t *testing.T) {
	_, err := buildCreateTableSQL(&config{
		Database:    "bad-db",
		Table:       "t",
		TableEngine: "MergeTree",
		PartitionBy: "toYYYYMMDD(timestamp)",
		OrderBy:     []string{"target"},
	})
	require.Error(t, err)
}

func TestMapValue_jsonMarshalFallback(t *testing.T) {
	r := telemetryRow{}
	ch := make(chan int)
	mapValue(ch, &r)
	require.Equal(t, "json", r.valueType)
}

func TestMapValue_intWidths(t *testing.T) {
	r := telemetryRow{}
	mapValue(int8(3), &r)
	require.Equal(t, "int", r.valueType)
	require.Equal(t, int64(3), *r.valueInt)

	r = telemetryRow{}
	mapValue(uint16(5), &r)
	require.Equal(t, "uint", r.valueType)
}
