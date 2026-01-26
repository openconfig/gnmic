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
	"io"
	"net/http"
	"sort"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	"github.com/openconfig/gnmic/pkg/api/utils"
)

var (
	ErrMarshal = errors.New("marshal error")
)

const backoff = 100 * time.Millisecond

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

func (p *promWriteOutput) writer(ctx context.Context) {
	defer p.wg.Done()
	defer p.logger.Printf("writer stopped")
	cfg := p.cfg.Load()
	p.logger.Printf("starting writer")
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		timeSeriesCh := *p.timeSeriesCh.Load()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if cfg.Debug {
				p.logger.Printf("write interval reached, writing to remote")
			}
			p.write(ctx, timeSeriesCh)
		case <-p.buffDrainCh:
			if cfg.Debug {
				p.logger.Printf("buffer full, writing to remote")
			}
			p.write(ctx, timeSeriesCh)
		}
	}
}

func (p *promWriteOutput) write(ctx context.Context, timeSeriesCh <-chan *prompb.TimeSeries) {
	cfg := p.cfg.Load()
	buffSize := len(timeSeriesCh)
	if cfg.Debug {
		p.logger.Printf("write triggered, buffer size: %d", buffSize)
	}
	if buffSize == 0 {
		return
	}
	pts := make([]prompb.TimeSeries, 0, buffSize)
	// read from buff channel for 1 second or
	// until we read a number of timeSeries equal to the buffer size
	for {
		select {
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
	chunk := make([]prompb.TimeSeries, 0, cfg.MaxTimeSeriesPerWrite)
	for i, pt := range pts {
		// append timeSeries to chunk
		chunk = append(chunk, pt)
		// if the chunk size reaches the configured max or
		// we reach the max number of time series gathered, send.
		chunkSize := len(chunk)
		if chunkSize == cfg.MaxTimeSeriesPerWrite || i+1 == numTS {
			if cfg.Debug {
				p.logger.Printf("writing a %d time series chunk", chunkSize)
			}
			start := time.Now()
			err := p.writeRequest(ctx, &prompb.WriteRequest{
				Timeseries: chunk,
			}, cfg)
			if err != nil {
				if cfg.Debug {
					p.logger.Print(err)
				}
				continue
			}
			prometheusWriteSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
			prometheusWriteNumberOfSentMsgs.WithLabelValues(cfg.Name).Add(float64(chunkSize))
			// return if we are done with the gathered time series
			if i+1 == numTS {
				return
			}
			// reset chunk if we are not done yet
			chunk = make([]prompb.TimeSeries, 0, cfg.MaxTimeSeriesPerWrite)
		}
	}
}

// writeRequest marshals the supplied prompb.WriteRequest,
// creates an HTTP request with the proper configured options (Authentication, Headers,...),
// sends the request and checks the returned response status code.
// It returns an error if the status code is >=300.
func (p *promWriteOutput) writeRequest(ctx context.Context, wr *prompb.WriteRequest, cfg *config) error {
	httpReq, err := p.makeHTTPRequest(ctx, wr)
	if err != nil {
		return err
	}

	// send request with retries
	retries := 0
RETRY:
	httpClient := p.httpClient.Load()
	//cfg := p.cfg.Load()
	rsp, err := httpClient.Do(httpReq)
	if err != nil {
		retries++
		err = fmt.Errorf("failed to write to remote: %w", err)
		p.logger.Print(err)
		if retries < cfg.MaxRetries {
			time.Sleep(backoff)
			goto RETRY
		}
		prometheusWriteNumberOfFailSendMsgs.WithLabelValues(cfg.Name, "client_failure").Inc()
		return err
	}
	defer rsp.Body.Close()

	if cfg.Debug {
		p.logger.Printf("got response from remote: status=%s", rsp.Status)
	}
	if rsp.StatusCode >= 300 {
		prometheusWriteNumberOfFailSendMsgs.WithLabelValues(cfg.Name, fmt.Sprintf("status_code=%d", rsp.StatusCode)).Inc()
		msg, err := io.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("write response failed, code=%d, body=%s", rsp.StatusCode, string(msg))
	}
	return nil
}

// metadataWriter writes the cached metadata entries to the remote address each `metadata.interval`
func (p *promWriteOutput) metadataWriter(ctx context.Context) {
	defer p.wg.Done()
	defer p.logger.Printf("metadata writer stopped")
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
		if cfg.Debug {
			p.logger.Printf("writing %d metadata points", len(mds))
		}
		start := time.Now()
		err := p.writeRequest(ctx, &prompb.WriteRequest{
			Metadata: mds,
		}, cfg)
		if err != nil {
			prometheusWriteNumberOfFailSendMetadataMsgs.WithLabelValues(cfg.Name).Add(1)
			if cfg.Debug {
				p.logger.Print(err)
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
	if cfg.Debug {
		p.logger.Printf("writing %d metadata points", len(mds))
	}
	start := time.Now()
	err := p.writeRequest(ctx, &prompb.WriteRequest{
		Metadata: mds,
	}, cfg)
	if err != nil {
		if cfg.Debug {
			p.logger.Print(err)
		}
		return
	}
	prometheusWriteMetadataSendDuration.WithLabelValues(cfg.Name).Set(float64(time.Since(start).Nanoseconds()))
	prometheusWriteNumberOfSentMetadataMsgs.WithLabelValues(cfg.Name).Add(float64(len(mds)))
}

func (p *promWriteOutput) makeHTTPRequest(ctx context.Context, wr *prompb.WriteRequest) (*http.Request, error) {
	cfg := p.cfg.Load()
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
