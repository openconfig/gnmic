// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	coordinationlisters "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"

	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/logging"
)

const (
	defaultNamespace = "default"
	origKeyName      = "original-key"
	origValueName    = "original-value"
)

func init() {
	lockers.Register("k8s", func() lockers.Locker {
		return &k8sLocker{Cfg: &config{}, locks: make(map[string]*leaseSession), logger: logging.DiscardLogger()}
	})
}

type k8sLocker struct {
	Cfg        *config
	clientset  kubernetes.Interface
	logger     *slog.Logger
	leases     coordinationlisters.LeaseNamespaceLister
	leaseIndex cache.Indexer
	stopCache  context.CancelFunc
	cacheDone  chan struct{}
	mu         sync.Mutex
	locks      map[string]*leaseSession
	stopped    bool
}

type config struct {
	Namespace     string        `mapstructure:"namespace" json:"namespace"`
	LeaseDuration time.Duration `mapstructure:"lease-duration" json:"lease-duration"`
	RenewDeadline time.Duration `mapstructure:"renew-deadline" json:"renew-deadline"`
	RetryPeriod   time.Duration `mapstructure:"retry-period" json:"retry-period"`
	RenewPeriod   time.Duration `mapstructure:"renew-period" json:"-"`
	RetryTimer    time.Duration `mapstructure:"retry-timer" json:"-"`
	QPS           float32       `mapstructure:"qps" json:"qps"`
	Burst         int           `mapstructure:"burst" json:"burst"`
	Debug         bool          `mapstructure:"debug" json:"debug,omitempty"`
}

func (k *k8sLocker) Init(ctx context.Context, cfg map[string]interface{}, opts ...lockers.Option) error {
	if err := lockers.DecodeConfig(cfg, k.Cfg); err != nil {
		return err
	}
	for _, opt := range opts {
		opt(k)
	}
	if err := k.setDefaults(); err != nil {
		return err
	}
	cfgREST, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	cfgREST.QPS, cfgREST.Burst = k.Cfg.QPS, k.Cfg.Burst
	k.clientset, err = kubernetes.NewForConfig(cfgREST)
	if err != nil {
		return err
	}
	return k.startLeaseCache(ctx)
}

func (k *k8sLocker) setDefaults() error {
	if k.Cfg.RenewPeriod != 0 {
		if k.Cfg.RenewDeadline != 0 && k.Cfg.RenewDeadline != k.Cfg.RenewPeriod {
			return fmt.Errorf("renew-period and renew-deadline must match when both are set")
		}
		k.Cfg.RenewDeadline = k.Cfg.RenewPeriod
	}
	if k.Cfg.RetryTimer != 0 {
		if k.Cfg.RetryPeriod != 0 && k.Cfg.RetryPeriod != k.Cfg.RetryTimer {
			return fmt.Errorf("retry-timer and retry-period must match when both are set")
		}
		k.Cfg.RetryPeriod = k.Cfg.RetryTimer
	}
	if k.Cfg.Namespace == "" {
		k.Cfg.Namespace = defaultNamespace
	}
	if k.Cfg.LeaseDuration == 0 {
		k.Cfg.LeaseDuration = 15 * time.Second
	}
	if k.Cfg.RenewDeadline == 0 {
		k.Cfg.RenewDeadline = k.Cfg.LeaseDuration * 2 / 3
	}
	if k.Cfg.RetryPeriod == 0 {
		k.Cfg.RetryPeriod = 2 * time.Second
	}
	if k.Cfg.QPS == 0 {
		k.Cfg.QPS = 100
	}
	if k.Cfg.Burst == 0 {
		k.Cfg.Burst = 200
	}
	if k.Cfg.LeaseDuration < time.Second || k.Cfg.LeaseDuration%time.Second != 0 {
		return fmt.Errorf("lease-duration must be a positive whole number of seconds")
	}
	if k.Cfg.RetryPeriod <= 0 || k.Cfg.RenewDeadline <= 0 || k.Cfg.RenewDeadline+k.Cfg.RetryPeriod >= k.Cfg.LeaseDuration {
		return fmt.Errorf("positive renew-deadline and retry-period must sum to less than lease-duration")
	}
	if k.Cfg.RenewDeadline <= time.Duration(leaderelection.JitterFactor*float64(k.Cfg.RetryPeriod)) {
		return fmt.Errorf("renew-deadline must be greater than retry-period * %g", leaderelection.JitterFactor)
	}
	if k.Cfg.QPS < 0 || k.Cfg.Burst < 1 {
		return fmt.Errorf("qps and burst must be positive")
	}
	return nil
}

func (k *k8sLocker) SetLogger(logger *slog.Logger) {
	k.logger = lockers.BindLogger(logger, "k8s")
}

func (k *k8sLocker) String() string {
	b, _ := json.Marshal(k.Cfg)
	return string(b)
}
