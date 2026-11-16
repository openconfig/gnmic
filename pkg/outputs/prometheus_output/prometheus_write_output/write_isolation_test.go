// © 2026 Supranett
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

type writeTestTransport func(*http.Request) (*http.Response, error)

func (transport writeTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func isolationWriter(t *testing.T, handler http.HandlerFunc) (*promWriteOutput, *config) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), URL: server.URL, MaxTimeSeriesPerWrite: 2, MaxRetries: 3}
	p.cfg.Store(cfg)
	p.httpClient.Store(server.Client())
	return p, cfg
}

func decodeWriteRequest(t *testing.T, r *http.Request) *prompb.WriteRequest {
	t.Helper()
	encoded, err := io.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
		return nil
	}
	decoded, err := snappy.Decode(nil, encoded)
	if err != nil {
		t.Error(err)
		return nil
	}
	request := &prompb.WriteRequest{}
	if err := request.Unmarshal(decoded); err != nil {
		t.Error(err)
		return nil
	}
	return request
}

func isolationSeries(index int) prompb.TimeSeries {
	return prompb.TimeSeries{
		Labels:  []prompb.Label{{Name: "__name__", Value: fmt.Sprintf("sample_%d", index)}},
		Samples: []prompb.Sample{{Value: float64(index), Timestamp: int64(index + 1)}},
	}
}

func TestWriteIsolatesRejectedChunks(t *testing.T) {
	for rejected := range 3 {
		t.Run(fmt.Sprint(rejected), func(t *testing.T) {
			var mu sync.Mutex
			var sizes []int
			accepted := 0
			p, _ := isolationWriter(t, func(w http.ResponseWriter, r *http.Request) {
				request := decodeWriteRequest(t, r)
				if request == nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				sizes = append(sizes, len(request.Timeseries))
				for _, series := range request.Timeseries {
					if series.Samples[0].Value == float64(rejected*2) {
						http.Error(w, "invalid sample", http.StatusBadRequest)
						return
					}
				}
				accepted += len(request.Timeseries)
				w.WriteHeader(http.StatusNoContent)
			})
			queue := make(chan *prompb.TimeSeries, 6)
			for index := range 6 {
				series := isolationSeries(index)
				queue <- &series
			}
			p.write(t.Context(), queue)
			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(sizes, []int{2, 2, 2}) || accepted != 4 {
				t.Fatalf("request sizes=%v accepted=%d, want [2 2 2] and 4", sizes, accepted)
			}
		})
	}
}

func TestWriteRequestRetriesRecoverableFailures(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			attempts := 0
			var mu sync.Mutex
			p, cfg := isolationWriter(t, func(w http.ResponseWriter, r *http.Request) {
				request := decodeWriteRequest(t, r)
				if request == nil || len(request.Timeseries) != 1 || request.Timeseries[0].Samples[0].Value != 1 {
					t.Error("retry request body changed")
				}
				mu.Lock()
				defer mu.Unlock()
				attempts++
				if attempts == 1 {
					w.WriteHeader(status)
				} else {
					w.WriteHeader(http.StatusNoContent)
				}
			})
			err := p.writeRequest(t.Context(), &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{isolationSeries(1)}}, cfg)
			mu.Lock()
			defer mu.Unlock()
			if status == http.StatusBadRequest {
				if err == nil || attempts != 1 {
					t.Fatalf("permanent failure: attempts=%d error=%v", attempts, err)
				}
			} else if err != nil || attempts != 2 {
				t.Fatalf("recoverable failure: attempts=%d error=%v", attempts, err)
			}
		})
	}
}

func TestWriteRequestRetryBudgetAndCancellation(t *testing.T) {
	var attempts atomic.Int32
	p, cfg := isolationWriter(t, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	request := &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{isolationSeries(1)}}
	if err := p.writeRequest(t.Context(), request, cfg); err == nil || attempts.Load() != int32(cfg.MaxRetries) {
		t.Fatalf("retry budget: attempts=%d error=%v", attempts.Load(), err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := p.writeRequest(ctx, request, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error=%v", err)
	}
}

func TestWriteRequestRebuildsBodyAfterTransportError(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), URL: "http://receiver.test/write", MaxRetries: 3}
	p.cfg.Store(cfg)
	var bodies [][]byte
	p.httpClient.Store(&http.Client{Transport: writeTestTransport(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		_ = request.Body.Close()
		if err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			return nil, errors.New("transport interrupted")
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})})
	if err := p.writeRequest(t.Context(), &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{isolationSeries(1)}}, cfg); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || len(bodies[0]) == 0 || !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("retry bodies = %v", bodies)
	}
}

func TestWriteRequestCancelsPendingRetry(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), URL: "http://receiver.test/write", MaxRetries: 100}
	p.cfg.Store(cfg)
	seen := make(chan struct{})
	var attempts atomic.Int32
	p.httpClient.Store(&http.Client{Transport: writeTestTransport(func(request *http.Request) (*http.Response, error) {
		_ = request.Body.Close()
		if attempts.Add(1) == 1 {
			close(seen)
		}
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- p.writeRequest(ctx, &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{isolationSeries(1)}}, cfg)
	}()
	<-seen
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) || attempts.Load() != 1 {
		t.Fatalf("cancellation: attempts=%d error=%v", attempts.Load(), err)
	}
}

func TestWriteRequestZeroAndOneBudgetsMakeOneAttempt(t *testing.T) {
	for _, budget := range []int{0, 1} {
		t.Run(fmt.Sprint(budget), func(t *testing.T) {
			var attempts atomic.Int32
			p, cfg := isolationWriter(t, func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			})
			cfg.MaxRetries = budget
			err := p.writeRequest(t.Context(), &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{isolationSeries(1)}}, cfg)
			if err == nil || attempts.Load() != 1 {
				t.Fatalf("budget=%d attempts=%d error=%v, want one failed attempt", budget, attempts.Load(), err)
			}
		})
	}
}

type trackedWriteBody struct {
	io.Reader
	closes *int
}

func (b *trackedWriteBody) Close() error {
	*b.closes++
	return nil
}

func TestWriteRequestUsesSnapshotAndClosesResponses(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	cfg := &config{Name: t.Name(), URL: "http://receiver.test/write", MaxRetries: 3, Headers: map[string]string{"X-Test": "original"}}
	p.cfg.Store(&config{URL: "http://other.test/write", Headers: map[string]string{"X-Test": "changed"}})
	attempts, closes := 0, 0
	p.httpClient.Store(&http.Client{Transport: writeTestTransport(func(request *http.Request) (*http.Response, error) {
		_ = request.Body.Close()
		if request.URL.String() != cfg.URL || request.Header.Get("X-Test") != "original" {
			t.Errorf("request used a different configuration: %v %v", request.URL, request.Header)
		}
		attempts++
		status := http.StatusNoContent
		if attempts == 1 {
			status = http.StatusServiceUnavailable
		}
		return &http.Response{StatusCode: status, Body: &trackedWriteBody{Reader: bytes.NewReader(nil), closes: &closes}}, nil
	})})
	if err := p.writeRequest(t.Context(), &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{isolationSeries(1)}}, cfg); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || closes != 2 {
		t.Fatalf("attempts=%d closed bodies=%d, want 2 each", attempts, closes)
	}
}
