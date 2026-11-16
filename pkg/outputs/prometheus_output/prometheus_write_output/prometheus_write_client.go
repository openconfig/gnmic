// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	"github.com/openconfig/gnmic/pkg/api/utils"
)

var (
	ErrMarshal = errors.New("marshal error")
)

func (p *promWriteOutput) createHTTPClientFor(c *config) (*http.Client, error) {
	cl := &http.Client{
		Timeout: c.Timeout,
	}
	if c.TLS != nil {
		tlsCfg, err := utils.NewTLSConfig(
			c.TLS.CaFile,
			c.TLS.CertFile,
			c.TLS.KeyFile,
			"",
			c.TLS.SkipVerify,
			false,
			false,
		)
		if err != nil {
			return nil, err
		}
		cl.Transport = &http.Transport{
			TLSClientConfig: tlsCfg,
		}
	}
	return cl, nil
}

func (p *promWriteOutput) writer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer p.logger.Info("writer stopped")
	cfg := p.cfg.Load()
	p.logger.Info("starting writer")
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		timeSeriesCh := *p.timeSeriesCh.Load()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.logger.Debug("write interval reached, writing to remote")
			p.write(ctx, timeSeriesCh)
		case <-p.buffDrainCh:
			p.logger.Debug("buffer full, writing to remote")
			p.write(ctx, timeSeriesCh)
		}
	}
}

func (p *promWriteOutput) write(ctx context.Context, timeSeriesCh <-chan *prompb.TimeSeries) {
	cfg := p.cfg.Load()
	buffSize := len(timeSeriesCh)
	p.logger.Debug("write triggered", "buffer-size", buffSize)
	if buffSize == 0 {
		return
	}
	pts := make([]prompb.TimeSeries, 0, buffSize)
	// read from buff channel for 1 second or
	// until we read a number of timeSeries equal to the buffer size
	for {
		select {
		case <-ctx.Done():
			return
		case ts := <-timeSeriesCh:
			pts = append(pts, *ts)
			if len(pts) == buffSize {
				goto WRITE
			}
		case <-time.After(time.Second):
			goto WRITE
		}
	}
WRITE:
	numTS := len(pts)
	if numTS == 0 {
		return
	}
	// sort timeSeries by timestamp
	sort.Slice(pts, func(i, j int) bool {
		return pts[i].Samples[0].Timestamp < pts[j].Samples[0].Timestamp
	})
	for offset := 0; offset < numTS; offset += cfg.MaxTimeSeriesPerWrite {
		if ctx.Err() != nil {
			return
		}
		chunk := pts[offset:min(offset+cfg.MaxTimeSeriesPerWrite, numTS)]
		start := time.Now()
		if err := p.writeRequest(ctx, &prompb.WriteRequest{Timeseries: chunk}, cfg); err != nil {
			if cfg.Debug {
				p.logger.Warn("remote write chunk failed", "err", err, "series", len(chunk))
			}
			continue
		}
		prometheusWriteSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
		prometheusWriteNumberOfSentMsgs.WithLabelValues(cfg.Name).Add(float64(len(chunk)))
	}
}

// metadataWriter writes the cached metadata entries to the remote address each `metadata.interval`
func (p *promWriteOutput) metadataWriter(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer p.logger.Info("metadata writer stopped")
	cfg := p.cfg.Load()
	if cfg.Metadata == nil || !cfg.Metadata.Include {
		return
	}
	p.writeMetadata(ctx)
	ticker := time.NewTicker(cfg.Metadata.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.writeMetadata(ctx)
		}
	}
}

// writeMetadata writes the currently cached metadata entries to the remote address,
// it will multiple prompb.WriteRequest with at most `metadata.max-entries` each until all entries are sent.
func (p *promWriteOutput) writeMetadata(ctx context.Context) {
	cfg := p.cfg.Load()
	p.m.Lock()
	defer p.m.Unlock()

	if len(p.metadataCache) == 0 {
		return
	}

	mds := make([]prompb.MetricMetadata, 0, cfg.Metadata.MaxEntriesPerWrite)
	count := 0 // keep track of the number of entries in mds

	for _, md := range p.metadataCache {
		if count < cfg.Metadata.MaxEntriesPerWrite {
			count++
			mds = append(mds, md)
			continue
		}
		// max entries reached, write accumulated entries
		p.logger.Debug("writing metadata points", "count", len(mds))
		start := time.Now()
		err := p.writeRequest(ctx, &prompb.WriteRequest{
			Metadata: mds,
		}, cfg)
		if err != nil {
			prometheusWriteNumberOfFailSendMetadataMsgs.WithLabelValues(cfg.Name).Add(1)
			if cfg.Debug {
				p.logger.Warn("write error", "err", err)
			}
			return
		}
		prometheusWriteMetadataSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
		prometheusWriteNumberOfSentMetadataMsgs.WithLabelValues(cfg.Name).Add(float64(len(mds)))
		// reset counter and array then continue with the loop
		count = 0
		mds = make([]prompb.MetricMetadata, 0, cfg.Metadata.MaxEntriesPerWrite)
	}

	// no metadata entries to write, return
	if len(mds) == 0 {
		return
	}

	// loop done with some metadata entries left to write
	p.logger.Debug("writing metadata points", "count", len(mds))
	start := time.Now()
	err := p.writeRequest(ctx, &prompb.WriteRequest{
		Metadata: mds,
	}, cfg)
	if err != nil {
		if cfg.Debug {
			p.logger.Warn("write error", "err", err)
		}
		return
	}
	prometheusWriteMetadataSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
	prometheusWriteNumberOfSentMetadataMsgs.WithLabelValues(cfg.Name).Add(float64(len(mds)))
}

func (p *promWriteOutput) makeHTTPRequest(ctx context.Context, wr *prompb.WriteRequest, cfg *config) (*http.Request, error) {
	b, err := gogoproto.Marshal(wr)
	if err != nil {
		prometheusWriteNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "marshal_error").Inc()
		return nil, fmt.Errorf("marshal error: %w", err)
	}
	compBytes := snappy.Encode(nil, b)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewBuffer(compBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("Content-Type", "application/x-protobuf")

	if cfg.Authentication != nil {
		httpReq.SetBasicAuth(cfg.Authentication.Username, cfg.Authentication.Password)
	}

	if cfg.Authorization != nil && cfg.Authorization.Type != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("%s %s", cfg.Authorization.Type, cfg.Authorization.Credentials))
	}

	for k, v := range cfg.Headers {
		httpReq.Header.Add(k, v)
	}

	return httpReq, nil
}
