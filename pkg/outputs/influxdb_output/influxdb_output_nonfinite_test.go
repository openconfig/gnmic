// SPDX-License-Identifier: Apache-2.0

package influxdb_output

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
)

// nonFiniteHarness starts an influxdb output against an httptest server that
// records every line-protocol body it receives.
type nonFiniteHarness struct {
	out    *influxDBOutput
	srv    *httptest.Server
	writes *atomic.Int64

	mu     sync.Mutex
	bodies []string
}

func (h *nonFiniteHarness) lines() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.bodies...)
}

func newNonFiniteHarness(t *testing.T) *nonFiniteHarness {
	t.Helper()
	h := &nonFiniteHarness{writes: new(atomic.Int64)}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		h.mu.Lock()
		h.bodies = append(h.bodies, string(buf[:n]))
		h.mu.Unlock()
		h.writes.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(h.srv.Close)

	h.out = &influxDBOutput{}
	cfg := map[string]any{
		"url": h.srv.URL, "org": "o", "bucket": "b", "token": "t",
		// health-check-period defaults to 0, so there is no health-check
		// goroutine to fire the reset signal. Nothing rescues a wedged worker.
		"health-check-period": "0",
		"batch-size":          1,
		"flush-timer":         "50ms",
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := h.out.Init(ctx, "nonfinite", cfg, outputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		// Close() waits on the worker WaitGroup; a wedged worker would hang the
		// test binary rather than fail it, so do not block the test on Close.
		done := make(chan struct{})
		go func() { h.out.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})
	return h
}

// send pushes one event into the output and reports whether the pipeline
// accepted it within the timeout. A false return means the worker is not
// consuming eventChan, i.e. the output is wedged.
func (h *nonFiniteHarness) send(name string, values map[string]any) bool {
	ev := &formatters.EventMsg{
		Name:      name,
		Timestamp: time.Now().UnixNano(),
		Tags:      map[string]string{"target": "router1"},
		Values:    values,
	}
	done := make(chan struct{})
	go func() { h.out.eventChan <- ev; close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(3 * time.Second):
		return false
	}
}

// TestInfluxDBOutput_SurvivesNonFiniteFloatBurst covers a burst of unencodable
// points, which is what it takes to trigger the deadlock.
//
// A dark optical lane reports -Inf repeatedly (every sample interval, for every
// dark lane). Before the fix the output wedges permanently partway through the
// burst: the worker blocks inside WritePoint on the influx client's
// single-slot error channel, which only the worker itself drains.
//
// If the bug were present this test fails at the first send() that times out.
func TestInfluxDBOutput_SurvivesNonFiniteFloatBurst(t *testing.T) {
	h := newNonFiniteHarness(t)

	if !h.send("optics", map[string]any{"laser_output_power_dbm": -1.5}) {
		t.Fatal("pipeline wedged on the very first good point (harness broken)")
	}

	// A single optics sample can carry many dark lanes, so failures arrive in a
	// burst. 40 is well past where the unpatched code wedges (3rd to 8th,
	// depending on scheduling).
	for n := 1; n <= 40; n++ {
		if !h.send("optics", map[string]any{"lane_laser_receiver_power_dbm": math.Inf(-1)}) {
			t.Fatalf("output wedged permanently on -Inf point #%d: the worker is "+
				"blocked in WritePoint on the influx client error channel and will "+
				"never consume eventChan again", n)
		}
	}

	// Also exercise the other non-finite forms that hit the same library path.
	for _, v := range []any{math.Inf(1), math.NaN(), float32(math.Inf(-1))} {
		if !h.send("optics", map[string]any{"lane_laser_receiver_power_dbm": v}) {
			t.Fatalf("output wedged on non-finite value %v", v)
		}
	}

	// The decisive assertion: after the burst, good data must still reach influx.
	before := h.writes.Load()
	for n := 0; n < 5; n++ {
		if !h.send("optics", map[string]any{"laser_output_power_dbm": float64(-2 - n)}) {
			t.Fatalf("output wedged on good point #%d after the -Inf burst", n)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && h.writes.Load() < before+5 {
		time.Sleep(20 * time.Millisecond)
	}
	if got := h.writes.Load(); got < before+5 {
		t.Fatalf("writes did not resume after -Inf burst: got %d, want >= %d", got, before+5)
	}
}

// TestInfluxDBOutput_NonFiniteFieldDroppedPointKept asserts the chosen
// behaviour: a non-finite field is dropped, but the finite fields sharing that
// point still reach influx. -Inf must never be coerced to a number (-40 dBm is
// a real, distinct reading), and one dark lane must not discard the whole
// sample.
func TestInfluxDBOutput_NonFiniteFieldDroppedPointKept(t *testing.T) {
	h := newNonFiniteHarness(t)

	if !h.send("optics", map[string]any{
		"lane_laser_receiver_power_dbm": math.Inf(-1), // dark lane
		"laser_output_power_dbm":        -1.234,       // valid, same point
		"module_temperature":            41.0,         // valid, same point
	}) {
		t.Fatal("output wedged on mixed finite/non-finite point")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && h.writes.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}

	body := strings.Join(h.lines(), "\n")
	if body == "" {
		t.Fatal("point with a non-finite field was dropped entirely; the finite fields should have been written")
	}
	if !strings.Contains(body, "laser_output_power_dbm") || !strings.Contains(body, "module_temperature") {
		t.Fatalf("finite fields missing from written line protocol: %q", body)
	}
	if strings.Contains(body, "lane_laser_receiver_power_dbm") {
		t.Fatalf("non-finite field should have been dropped, got: %q", body)
	}
	if strings.Contains(body, "Inf") || strings.Contains(body, "inf") {
		t.Fatalf("non-finite value leaked into line protocol: %q", body)
	}
}

// TestInfluxDBOutput_DarkLaneViaWrite drives the whole path -- a gNMI
// SubscribeResponse through Write() and ResponseToEventMsgs, rather than a
// hand-built EventMsg.
//
// A dark optical lane reports 10*log10(0) = -Inf for received power, arriving as
// a float32 float_val. Under proto encoding the value reaches the output; under
// json it is dropped earlier, because encoding/json cannot represent a
// non-finite float.
func TestInfluxDBOutput_DarkLaneViaWrite(t *testing.T) {
	h := newNonFiniteHarness(t)

	darkLaneUpdate := func(port string) *gnmi.SubscribeResponse {
		return &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: &gnmi.Notification{
					Timestamp: time.Now().UnixNano(),
					Prefix: &gnmi.Path{Elem: []*gnmi.PathElem{
						{Name: "components"}, {Name: "optics"},
						{Name: "port", Key: map[string]string{"name": port}},
					}},
					Update: []*gnmi.Update{
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "lane_laser_receiver_power_dbm"}}},
							// Exactly what the router emits for a dark lane:
							// a float_val of -inf.
							Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_FloatVal{FloatVal: float32(math.Inf(-1))}},
						},
						{
							Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "laser_output_power_dbm"}}},
							Val:  &gnmi.TypedValue{Value: &gnmi.TypedValue_FloatVal{FloatVal: 1.6820288}},
						},
					},
				},
			},
		}
	}

	// Two dark lanes, sampled repeatedly as a 60s interval would.
	meta := outputs.Meta{"subscription-name": "optics", "source": "router1"}
	for round := 0; round < 15; round++ {
		for _, port := range []string{"port1", "port2"} {
			done := make(chan struct{})
			go func() {
				h.out.Write(context.Background(), darkLaneUpdate(port), meta)
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatalf("Write() blocked on dark-lane sample (round %d, port %s): "+
					"the output worker has deadlocked, which stalls every "+
					"subscription on every target", round, port)
			}
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && h.writes.Load() == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	body := strings.Join(h.lines(), "\n")
	if !strings.Contains(body, "laser_output_power_dbm") {
		t.Fatalf("the good field of the dark-lane sample never reached influx: %q", body)
	}
	if strings.Contains(body, "lane_laser_receiver_power_dbm") {
		t.Fatalf("non-finite dark-lane field leaked into line protocol: %q", body)
	}
}

// TestInfluxDBOutput_SurvivesAnyEncodeError guards the structural fix rather
// than the -Inf symptom.
//
// -Inf was only the trigger we happened to hit. Every line-protocol encode
// failure reaches the same unconditional send on the client's single-slot error
// channel, so ANY repeatable encode error wedges the output if gnmic drains that
// channel from the goroutine that calls WritePoint. Empty measurement names and
// empty field keys were both verified to wedge before the fix.
//
// This test is the reason the fix drains errors on a separate goroutine instead
// of only sanitising non-finite floats.
func TestInfluxDBOutput_SurvivesAnyEncodeError(t *testing.T) {
	cases := []struct {
		name        string
		measurement string
		values      map[string]any
	}{
		{"empty measurement name", "", map[string]any{"v": 1.0}},
		{"empty field key", "m", map[string]any{"": 1.0}},
		{"unsupported value type", "m", map[string]any{"v": []any{1, 2}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newNonFiniteHarness(t)
			for n := 1; n <= 30; n++ {
				vals := make(map[string]any, len(c.values))
				for k, v := range c.values {
					vals[k] = v
				}
				if !h.send(c.measurement, vals) {
					t.Fatalf("output wedged on %s at repetition #%d: the worker is "+
						"blocked in WritePoint on the influx client error channel", c.name, n)
				}
			}
			// Good data must still get through afterwards.
			if !h.send("optics", map[string]any{"laser_output_power_dbm": -1.5}) {
				t.Fatalf("output wedged on a good point after repeated %s", c.name)
			}
		})
	}
}

// TestInfluxDBOutput_NonFiniteLoggingIsRateLimited pins the log volume.
//
// A dark lane reports -Inf on EVERY sample, so this is a steady-state condition,
// not a one-off event. One log line per dropped field would be roughly 2.9k
// lines/day for one device with two dark lanes, and ~90k/day on a card with many
// unlit lanes -- enough to bury real errors. The metric carries the exact counts;
// the log says it once, then summarizes.
func TestInfluxDBOutput_NonFiniteLoggingIsRateLimited(t *testing.T) {
	var buf lockedBuffer
	out := &influxDBOutput{}
	out.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// A device with two dark lanes sampled every 60s reaches this in a few months.
	for n := 0; n < 500; n++ {
		out.logNonFinite(0, "optics", []string{"lane_laser_receiver_power_dbm"})
	}

	lines := buf.lines()
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 log line for 500 drops in one interval, got %d:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
	// The one line must be actionable: name the field and point at the metric.
	if !strings.Contains(lines[0], "lane_laser_receiver_power_dbm") {
		t.Errorf("first log line should name the dropped field: %s", lines[0])
	}
	if !strings.Contains(lines[0], "non_finite_values_dropped_total") {
		t.Errorf("first log line should point at the metric: %s", lines[0])
	}

	// After the interval elapses, exactly one summary line appears, carrying the
	// suppressed count rather than one line per drop.
	out.nonFiniteLog.mu.Lock()
	out.nonFiniteLog.nextLog = time.Now().Add(-time.Second)
	out.nonFiniteLog.mu.Unlock()

	for n := 0; n < 300; n++ {
		out.logNonFinite(0, "optics", []string{"lane_laser_receiver_power_dbm"})
	}
	lines = buf.lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines total after the interval elapsed, got %d:\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
	// 500 unlogged from round 1 (the first line reset the counter to 0 after
	// consuming 1) plus 300 more; the point is that it reports a count, not 300 lines.
	if !strings.Contains(lines[1], "count=") {
		t.Errorf("summary line should carry a suppressed count: %s", lines[1])
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, ln := range strings.Split(l.b.String(), "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// TestInfluxDBOutput_NonFiniteLogIntervalIsReal checks the suppression window
// without faking it. The rate-limit test above rewrites nextLog to simulate
// expiry, so on its own it would still pass if the interval were 5ms.
func TestInfluxDBOutput_NonFiniteLogIntervalIsReal(t *testing.T) {
	if nonFiniteLogInterval != 5*time.Minute {
		t.Fatalf("nonFiniteLogInterval = %v, want 5m", nonFiniteLogInterval)
	}
	var buf lockedBuffer
	out := &influxDBOutput{}
	out.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	out.logNonFinite(0, "optics", []string{"rx_power"})
	out.logNonFinite(0, "optics", []string{"rx_power"})
	if n := len(buf.lines()); n != 1 {
		t.Fatalf("second call inside the window logged: got %d lines, want 1", n)
	}
	out.nonFiniteLog.mu.Lock()
	remaining := time.Until(out.nonFiniteLog.nextLog)
	out.nonFiniteLog.mu.Unlock()
	if remaining < 4*time.Minute+50*time.Second || remaining > 5*time.Minute {
		t.Fatalf("next log window is %v away, want ~5m", remaining)
	}
}

// TestInfluxDBOutput_NonFiniteSummaryCountIsAccurate pins the reported count,
// not merely the presence of a count field. An under-reporting summary would
// understate how much data is being dropped.
func TestInfluxDBOutput_NonFiniteSummaryCountIsAccurate(t *testing.T) {
	var buf lockedBuffer
	out := &influxDBOutput{}
	out.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	out.logNonFinite(0, "optics", []string{"rx_power"}) // logs, consumes 1
	for n := 0; n < 9; n++ {                            // 9 suppressed
		out.logNonFinite(0, "optics", []string{"rx_power"})
	}
	out.nonFiniteLog.mu.Lock()
	out.nonFiniteLog.nextLog = time.Now().Add(-time.Second)
	out.nonFiniteLog.mu.Unlock()
	out.logNonFinite(0, "optics", []string{"rx_power"}) // 9 + this one = 10

	lines := buf.lines()
	last := lines[len(lines)-1]
	if !strings.Contains(last, "count=10") {
		t.Fatalf("summary should report count=10 (9 suppressed + the triggering drop), got: %s", last)
	}
}

// TestInfluxDBOutput_NonFiniteSummarySpansMeasurements guards against a summary
// that misattributes fields.
//
// The suppression window spans every subscription the output serves. Reporting a
// bare field name alongside whichever measurement happened to trigger the flush
// claims a pairing that never existed, so names are qualified "measurement/field".
func TestInfluxDBOutput_NonFiniteSummarySpansMeasurements(t *testing.T) {
	var buf lockedBuffer
	out := &influxDBOutput{}
	out.logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	out.logNonFinite(0, "optics", []string{"rx_power"}) // first line
	out.logNonFinite(0, "interfaces", []string{"if_rate"})
	out.nonFiniteLog.mu.Lock()
	out.nonFiniteLog.nextLog = time.Now().Add(-time.Second)
	out.nonFiniteLog.mu.Unlock()
	out.logNonFinite(0, "temperature", []string{"degrees"})

	lines := buf.lines()
	summary := lines[len(lines)-1]
	for _, want := range []string{
		"interfaces/if_rate",
		"temperature/degrees",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing qualified field %q: %s", want, summary)
		}
	}
	// The summary covers several measurements, so it must not claim one.
	if strings.Contains(summary, "measurement=") {
		t.Errorf("summary should not attribute all fields to a single measurement: %s", summary)
	}
}

// TestInfluxDBOutput_DrainerNeverAbandonsPendingError guards the shutdown case.
//
// A queued error means a producer is blocked delivering it. If the drainer
// returned on shutdown while an error sat in the single-slot channel, the worker
// would stay parked in WritePoint forever and Close()'s WaitGroup wait would
// hang -- reintroducing the original deadlock during shutdown or a config
// update. So a pending error must always win over the stop signal.
//
// With stop and errCh both ready, a plain two-case select picks pseudo-randomly:
// the pre-fix version abandoned the error in ~half of 200 runs.
func TestInfluxDBOutput_DrainerNeverAbandonsPendingError(t *testing.T) {
	const runs = 200
	abandoned := 0
	for n := 0; n < runs; n++ {
		out := &influxDBOutput{logger: logging.DiscardLogger()}
		errCh := make(chan error, 1)
		errCh <- errors.New("is Inf")

		stop := make(chan struct{})
		close(stop) // stop already signalled: both select cases are ready
		done := make(chan struct{})
		go out.drainWriteErrors(stop, errCh, 0, done)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("drainer did not exit")
		}
		if len(errCh) > 0 {
			abandoned++
		}
	}
	if abandoned > 0 {
		t.Fatalf("drainer abandoned a pending error in %d/%d runs; a producer blocked "+
			"on that channel would never be released", abandoned, runs)
	}
}

// TestClientNeedsRebuild_OrgBucket: each org/bucket pair gets its own WriteAPI
// with its own error channel, and a worker binds one for its lifetime. Changing
// either must restart the workers, or the new API's errors go undrained and the
// output can wedge exactly as it did originally.
func TestClientNeedsRebuild_OrgBucket(t *testing.T) {
	base := &Config{URL: "http://x", Org: "o", Bucket: "b", Token: "t"}
	for _, tc := range []struct {
		name string
		mod  func(*Config)
	}{
		{"org changed", func(c *Config) { c.Org = "o2" }},
		{"bucket changed", func(c *Config) { c.Bucket = "b2" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := *base
			tc.mod(&next)
			if !clientNeedsRebuild(base, &next) {
				t.Fatalf("%s must trigger a client/worker rebuild", tc.name)
			}
		})
	}
	same := *base
	if clientNeedsRebuild(base, &same) {
		t.Fatal("identical config should not trigger a rebuild")
	}
}

// TestInfluxDBOutput_MetricsEnabledViaUpdate: enable-metrics can be switched on
// by a live config reload, not just at startup. Init is the only place that
// registers the collector, so without the same calls in Update the counter
// increments in memory but never appears on /metrics -- a silent monitoring gap
// for the metric added to make these drops visible in the first place.
func TestInfluxDBOutput_MetricsEnabledViaUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	cfg := func(metrics bool) map[string]any {
		return map[string]any{
			"url": srv.URL, "org": "o", "bucket": "b", "token": "t",
			"health-check-period": "0",
			"batch-size":          1,
			"flush-timer":         "50ms",
			"enable-metrics":      metrics,
		}
	}

	// registerMetricsOnce is a package global (the pattern every output in this
	// repo uses), so it is already spent under -count=2 or after another test
	// registered. Reset it so this test exercises registration rather than
	// inheriting someone else's.
	registerMetricsOnce = sync.Once{}
	t.Cleanup(func() { registerMetricsOnce = sync.Once{} })

	reg := prometheus.NewRegistry()
	out := &influxDBOutput{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := out.Init(ctx, "upd", cfg(false),
		outputs.WithConfigStore(memStore()), outputs.WithRegistry(reg)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() { out.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	})

	// Repeated updates must also not fail on duplicate registration.
	for n := 0; n < 3; n++ {
		if err := out.Update(ctx, cfg(true)); err != nil {
			t.Fatalf("Update %d: %v", n, err)
		}
	}

	out.eventChan <- &formatters.EventMsg{
		Name:      "optics",
		Timestamp: time.Now().UnixNano(),
		Tags:      map[string]string{"target": "router1"},
		Values:    map[string]any{"lane_laser_receiver_power_dbm": math.Inf(-1)},
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		families, err := reg.Gather()
		if err != nil {
			t.Fatalf("Gather: %v", err)
		}
		for _, f := range families {
			if strings.Contains(f.GetName(), "non_finite_values_dropped_total") {
				return // exposed, as it should be
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("non_finite_values_dropped_total never appeared on the registry after " +
		"enable-metrics was switched on by Update()")
}
