package prometheus_write_output

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/prometheus/prompb"
)

func (p *promWriteOutput) createHTTPClient() error {
	c := &http.Client{
		Timeout: p.Cfg.Timeout,
	}
	if p.Cfg.TLS != nil {
		tlsCfg, err := utils.NewTLSConfig(
			p.Cfg.TLS.CAFile,
			p.Cfg.TLS.CertFile,
			p.Cfg.TLS.KeyFile,
			p.Cfg.TLS.SkipVerify,
			false)
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
	ticker := time.NewTicker(p.Cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.Cfg.Debug {
				p.logger.Printf("write interval reached, writing to remote")
			}
			p.write(ctx)
		case <-p.buffCh:
			if p.Cfg.Debug {
				p.logger.Printf("buffer full, writing to remote")
			}
			p.write(ctx)
		}
	}
}

func (p *promWriteOutput) write(ctx context.Context) {
	buffSize := len(p.timeSeriesCh)
	if p.Cfg.Debug {
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
				goto BUFF
			}
		case <-time.After(time.Second):
			goto BUFF
		}
	}
BUFF:
	if len(pts) == 0 {
		return
	}
	wr := &prompb.WriteRequest{
		Timeseries: pts,
		Metadata: []prompb.MetricMetadata{
			{
				Type: prompb.MetricMetadata_COUNTER,
				Help: defaultMetricHelp,
			},
		},
	}
	if p.Cfg.IncludeMetadata {
		wr.Metadata = []prompb.MetricMetadata{
			{
				Type: prompb.MetricMetadata_COUNTER,
				Help: defaultMetricHelp,
			},
		}
	}
	err := p.writeReq(ctx, wr)
	if err != nil {
		p.logger.Print(err)
	}
}

func (p *promWriteOutput) writeReq(ctx context.Context, wr *prompb.WriteRequest) error {
	b, err := gogoproto.Marshal(wr)
	if err != nil {
		return fmt.Errorf("failed to marshal proto write request: %v", err)
	}
	compBytes := snappy.Encode(nil, b)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Cfg.URL, bytes.NewBuffer(compBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("Content-Type", "application/x-protobuf")

	if p.Cfg.Authentication != nil {
		httpReq.SetBasicAuth(p.Cfg.Authentication.Username, p.Cfg.Authentication.Password)
	}

	if p.Cfg.Authorization != nil && strings.ToLower(p.Cfg.Authorization.Type) == "bearer" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.Cfg.Authorization.Credentials))
	}

	for k, v := range p.Cfg.Headers {
		httpReq.Header.Add(k, v)
	}

	rsp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to write to remote: %v", err)
	}
	defer rsp.Body.Close()
	if p.Cfg.Debug {
		p.logger.Printf("got response from remote: status=%s", rsp.Status)
	}
	if rsp.StatusCode >= 300 {
		msg, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("write response, code=%d, body=%s", rsp.StatusCode, string(msg))
	}
	return nil
}
