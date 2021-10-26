package http_loader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/karimra/gnmic/loaders"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	loggingPrefix   = "[http_loader] "
	loaderType      = "http"
	defaultInterval = 1 * time.Minute
	defaultTimeout  = 50 * time.Second
)

func init() {
	loaders.Register(loaderType, func() loaders.TargetLoader {
		return &httpLoader{
			cfg:         &cfg{},
			lastTargets: make(map[string]*types.TargetConfig),
			logger:      log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type httpLoader struct {
	cfg         *cfg
	lastTargets map[string]*types.TargetConfig
	logger      *log.Logger
}

type cfg struct {
	// the server URL, must include http or https as a prefix
	URL string `json:"url,omitempty" mapstructure:"url,omitempty"`
	// server query interval
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	// query timeout
	Timeout time.Duration `json:"timeout,omitempty" mapstructure:"timeout,omitempty"`
	// TLS config
	SkipVerify bool   `json:"skip-verify,omitempty" mapstructure:"skip-verify,omitempty"`
	CAFile     string `json:"ca-file,omitempty" mapstructure:"ca-file,omitempty"`
	CertFile   string `json:"cert-file,omitempty" mapstructure:"cert-file,omitempty"`
	KeyFile    string `json:"key-file,omitempty" mapstructure:"key-file,omitempty"`
	// HTTP basicAuth
	Username string `json:"username,omitempty" mapstructure:"username,omitempty"`
	Password string `json:"password,omitempty" mapstructure:"password,omitempty"`
	// Oauth2
	Token string `json:"token,omitempty" mapstructure:"token,omitempty"`
	// if true, registers httpLoader prometheus metrics with the provided
	// prometheus registry
	EnableMetrics bool `json:"enable-metrics,omitempty" mapstructure:"enable-metrics,omitempty"`
}

func (h *httpLoader) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger, opts ...loaders.Option) error {
	err := loaders.DecodeConfig(cfg, h.cfg)
	if err != nil {
		return err
	}
	err = h.setDefaults()
	if err != nil {
		return err
	}
	if logger != nil {
		h.logger.SetOutput(logger.Writer())
		h.logger.SetFlags(logger.Flags())
	}
	return nil
}

func (h *httpLoader) Start(ctx context.Context) chan *loaders.TargetOperation {
	opChan := make(chan *loaders.TargetOperation)
	ticker := time.NewTicker(h.cfg.Interval)
	go func() {
		defer close(opChan)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				readTargets, err := h.query()
				if err != nil {
					h.logger.Printf("failed to read targets from HTTP remote: %v", err)
					continue
				}
				select {
				case <-ctx.Done():
					ticker.Stop()
					return
				case opChan <- h.diff(readTargets):
				}
			}
		}
	}()
	return opChan
}

func (h *httpLoader) RegisterMetrics(reg *prometheus.Registry) {
	if !h.cfg.EnableMetrics && reg != nil {
		return
	}
	if err := registerMetrics(reg); err != nil {
		h.logger.Printf("failed to register metrics: %v", err)
	}
}

func (h *httpLoader) setDefaults() error {
	if h.cfg.URL == "" {
		return errors.New("missing URL")
	}
	if h.cfg.Interval <= 0 {
		h.cfg.Interval = defaultInterval
	}
	if h.cfg.Timeout <= 0 {
		h.cfg.Timeout = defaultTimeout
	}
	return nil
}

func (h *httpLoader) query() (map[string]*types.TargetConfig, error) {
	c := resty.New()
	tlsCfg, err := utils.NewTLSConfig(h.cfg.CAFile, h.cfg.CertFile, h.cfg.KeyFile, h.cfg.SkipVerify, false)
	if err != nil {
		httpLoaderFailedGetRequests.WithLabelValues(loaderType, fmt.Sprintf("%v", err))
		return nil, err
	}
	if tlsCfg != nil {
		c = c.SetTLSClientConfig(tlsCfg)
	}
	c.SetTimeout(h.cfg.Timeout)
	if h.cfg.Username != "" && h.cfg.Password != "" {
		c.SetBasicAuth(h.cfg.Username, h.cfg.Password)
	}
	if h.cfg.Token != "" {
		c.SetAuthToken(h.cfg.Token)
	}
	result := make(map[string]*types.TargetConfig)
	start := time.Now()
	httpLoaderGetRequestsTotal.WithLabelValues(loaderType).Add(1)
	rsp, err := c.R().SetResult(result).Get(h.cfg.URL)
	if err != nil {
		return nil, err
	}
	httpLoaderGetRequestDuration.WithLabelValues(loaderType).Set(float64(time.Since(start).Nanoseconds()))
	if rsp.StatusCode() != 200 {
		httpLoaderFailedGetRequests.WithLabelValues(loaderType, rsp.Status())
		return nil, fmt.Errorf("failed request, code=%d", rsp.StatusCode())
	}
	return rsp.Result().(map[string]*types.TargetConfig), nil
}

func (h *httpLoader) diff(m map[string]*types.TargetConfig) *loaders.TargetOperation {
	result := loaders.Diff(h.lastTargets, m)
	for _, t := range result.Add {
		if _, ok := h.lastTargets[t.Name]; !ok {
			h.lastTargets[t.Name] = t
		}
	}
	for _, n := range result.Del {
		delete(h.lastTargets, n)
	}
	httpLoaderLoadedTargets.WithLabelValues(loaderType).Set(float64(len(result.Add)))
	httpLoaderDeletedTargets.WithLabelValues(loaderType).Set(float64(len(result.Del)))
	return result
}
