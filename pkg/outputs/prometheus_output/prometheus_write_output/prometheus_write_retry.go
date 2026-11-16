// © 2026 Supranett
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

const (
	backoff            = 100 * time.Millisecond
	maxWriteErrorBytes = 64 << 10
)

func (p *promWriteOutput) writeRequest(ctx context.Context, wr *prompb.WriteRequest, cfg *config) error {
	var lastErr error
	reason := "client_failure"
	for attempt := 0; attempt < max(1, cfg.MaxRetries); attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempt > 0 {
			timer := time.NewTimer(backoff * time.Duration(1<<min(attempt-1, 6)))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		request, err := p.makeHTTPRequest(ctx, wr, cfg)
		if err != nil {
			return err
		}
		response, err := p.httpClient.Load().Do(request)
		if err != nil {
			lastErr = fmt.Errorf("request remote write: %w", err)
			reason = "client_failure"
			continue
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxWriteErrorBytes))
			_ = response.Body.Close()
			return nil
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxWriteErrorBytes))
		_ = response.Body.Close()
		lastErr = errors.Join(fmt.Errorf("remote write returned HTTP %d: %s", response.StatusCode, body), readErr)
		reason = fmt.Sprintf("status_code=%d", response.StatusCode)
		if response.StatusCode != http.StatusTooManyRequests && (response.StatusCode < 500 || response.StatusCode >= 600) {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	prometheusWriteNumberOfFailSendMsgs.WithLabelValues(cfg.Name, reason).Inc()
	p.logger.Error("remote write failed", "reason", reason)
	return lastErr
}
