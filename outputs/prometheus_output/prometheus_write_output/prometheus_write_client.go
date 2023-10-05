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
	"fmt"
	"io"
	"net/http"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/openconfig/gnmic/utils"
	"github.com/prometheus/prometheus/prompb"
)

var backoff = 100 * time.Millisecond

func (p *promWriteOutput) createHTTPClient() error {
	c := &http.Client{
		Timeout: p.cfg.Timeout,
	}
	if p.cfg.TLS != nil {
		tlsCfg, err := utils.NewTLSConfig(
			p.cfg.TLS.CaFile,
			p.cfg.TLS.CertFile,
			p.cfg.TLS.KeyFile,
			"",
			p.cfg.TLS.SkipVerify,
			false,
		)
		if err != nil {
			return err
		}
		c.Transport = &http.Transport{
			TLSClientConfig: tlsCfg,
		}
	}
	p.httpClient = c
	return nil
}

func (p *promWriteOutput) writer(ctx context.Context) {
	p.logger.Printf("starting writer")
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.cfg.Debug {
				p.logger.Printf("write interval reached, writing to remote")
			}
			p.write(ctx)
		case <-p.buffDrainCh:
			if p.cfg.Debug {
				p.logger.Printf("buffer full, writing to remote")
			}
			p.write(ctx)
		}
	}
}

func (p *promWriteOutput) write(ctx context.Context) {
	buffSize := len(p.timeSeriesCh)
	if p.cfg.Debug {
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
		case ts := <-p.timeSeriesCh:
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
	chunk := make([]prompb.TimeSeries, 0, p.cfg.MaxTimeSeriesPerWrite)
	for i, pt := range pts {
		// append timeSeries to chunk
		chunk = append(chunk, pt)
		// if the chunk size reaches the configured max or
		// we reach the max number of time series gathered, send.
		chunkSize := len(chunk)
		if chunkSize == p.cfg.MaxTimeSeriesPerWrite || i+1 == numTS {
			if p.cfg.Debug {
				p.logger.Printf("writing a %d time series chunk", chunkSize)
			}
			start := time.Now()
			err := p.writeRequest(ctx, &prompb.WriteRequest{
				Timeseries: chunk,
			})
			if err != nil {
				prometheusWriteNumberOfFailSendMsgs.WithLabelValues(err.Error()).Inc()
				continue
			}
			prometheusWriteSendDuration.Set(float64(time.Since(start).Nanoseconds()))
			prometheusWriteNumberOfSentMsgs.Add(float64(chunkSize))
			// return if we are done with the gathered time series
			if i+1 == numTS {
				return
			}
			// reset chunk if we are not done yet
			chunk = make([]prompb.TimeSeries, 0, p.cfg.MaxTimeSeriesPerWrite)
		}
	}
}

// writeRequest marshals the supplied prompb.WriteRequest,
// creates an HTTP request with the proper configured options (Authentication, Headers,...),
// sends the request and checks the returned response status code.
// It returns an error if the status code is >=300.
func (p *promWriteOutput) writeRequest(ctx context.Context, wr *prompb.WriteRequest) error {
	httpReq, err := p.makeHTTPRequest(ctx, wr)
	if err != nil {
		return err
	}

	// send request with retries
	retries := 0
RETRY:
	rsp, err := p.httpClient.Do(httpReq)
	if err != nil {
		retries++
		err = fmt.Errorf("failed to write to remote: %w", err)
		p.logger.Print(err)
		if retries < p.cfg.MaxRetries {
			time.Sleep(backoff)
			goto RETRY
		}
		return err
	}
	defer rsp.Body.Close()

	if p.cfg.Debug {
		p.logger.Printf("got response from remote: status=%s", rsp.Status)
	}
	if rsp.StatusCode >= 300 {
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
	if p.cfg.Metadata == nil || !p.cfg.Metadata.Include {
		return
	}
	p.writeMetadata(ctx)
	ticker := time.NewTicker(p.cfg.Metadata.Interval)
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
	p.m.Lock()
	defer p.m.Unlock()

	if len(p.metadataCache) == 0 {
		return
	}

	mds := make([]prompb.MetricMetadata, 0, p.cfg.Metadata.MaxEntriesPerWrite)
	count := 0 // keep track of the number of entries in mds

	for _, md := range p.metadataCache {
		if count < p.cfg.Metadata.MaxEntriesPerWrite {
			count++
			mds = append(mds, md)
			continue
		}
		// max entries reached, write accumulated entries
		if p.cfg.Debug {
			p.logger.Printf("writing %d metadata points", len(mds))
		}
		start := time.Now()
		err := p.writeRequest(ctx, &prompb.WriteRequest{
			Metadata: mds,
		})
		if err != nil {
			prometheusWriteNumberOfFailSendMetadataMsgs.WithLabelValues(err.Error()).Inc()
			return
		}
		prometheusWriteMetadataSendDuration.Set(float64(time.Since(start).Nanoseconds()))
		prometheusWriteNumberOfSentMetadataMsgs.Add(float64(len(mds)))
		// reset counter and array then continue with the loop
		count = 0
		mds = make([]prompb.MetricMetadata, 0, p.cfg.Metadata.MaxEntriesPerWrite)
	}

	// no metadata entries to write, return
	if len(mds) == 0 {
		return
	}

	// loop done with some metadata entries left to write
	if p.cfg.Debug {
		p.logger.Printf("writing %d metadata points", len(mds))
	}
	start := time.Now()
	err := p.writeRequest(ctx, &prompb.WriteRequest{
		Metadata: mds,
	})
	if err != nil {
		prometheusWriteNumberOfFailSendMetadataMsgs.WithLabelValues(err.Error()).Inc()
		return
	}
	prometheusWriteMetadataSendDuration.Set(float64(time.Since(start).Nanoseconds()))
	prometheusWriteNumberOfSentMetadataMsgs.Add(float64(len(mds)))
}

func (p *promWriteOutput) makeHTTPRequest(ctx context.Context, wr *prompb.WriteRequest) (*http.Request, error) {
	b, err := gogoproto.Marshal(wr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto write request: %v", err)
	}
	compBytes := snappy.Encode(nil, b)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.URL, bytes.NewBuffer(compBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("Content-Type", "application/x-protobuf")

	if p.cfg.Authentication != nil {
		httpReq.SetBasicAuth(p.cfg.Authentication.Username, p.cfg.Authentication.Password)
	}

	if p.cfg.Authorization != nil && p.cfg.Authorization.Type != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("%s %s", p.cfg.Authorization.Type, p.cfg.Authorization.Credentials))
	}

	for k, v := range p.cfg.Headers {
		httpReq.Header.Add(k, v)
	}

	return httpReq, nil
}
