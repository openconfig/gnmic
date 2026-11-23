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
	defaultNamespace     = "default"
	defaultLeaseDuration = 15 * time.Second
	defaultRetryPeriod   = 2 * time.Second
	defaultQPS           = 100
	defaultBurst         = 200
	origKeyName          = "original-key"
	origValueName        = "original-value"
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
	Namespace     string        `mapstructure:"namespace,omitempty" json:"namespace,omitempty"`
	LeaseDuration time.Duration `mapstructure:"lease-duration,omitempty" json:"lease-duration,omitempty"`
	RenewDeadline time.Duration `mapstructure:"renew-deadline,omitempty" json:"renew-deadline,omitempty"`
	RetryPeriod   time.Duration `mapstructure:"retry-period,omitempty" json:"retry-period,omitempty"`
	// Deprecated aliases of RenewDeadline and RetryPeriod.
	RenewPeriod time.Duration `mapstructure:"renew-period,omitempty" json:"-"`
	RetryTimer  time.Duration `mapstructure:"retry-timer,omitempty" json:"-"`
	Debug       bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
	QPS         float32       `mapstructure:"qps,omitempty" json:"qps,omitempty"`
	Burst       int           `mapstructure:"burst,omitempty" json:"burst,omitempty"`
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
	cfgREST.QPS, cfgREST.Burst = k.Cfg.clientLimits()
	k.clientset, err = kubernetes.NewForConfig(cfgREST)
	if err != nil {
		return err
	}
	return k.startLeaseCache(ctx)
}

// setDefaults fills in defaults and validates the Lease timings. A fractional
// lease-duration is rounded down, and deprecated renew-period or retry-timer values
// that violate the timing constraints fall back to the defaults, with a warning.
func (k *k8sLocker) setDefaults() error {
	c := k.Cfg
	if c.Namespace == "" {
		c.Namespace = defaultNamespace
	}
	if c.LeaseDuration <= 0 {
		c.LeaseDuration = defaultLeaseDuration
	}
	if whole := c.LeaseDuration.Truncate(time.Second); whole != c.LeaseDuration {
		if whole == 0 {
			return fmt.Errorf("lease-duration (%s) must be at least 1s", c.LeaseDuration)
		}
		k.logger.Warn("lease-duration rounded down to whole seconds", "configured", c.LeaseDuration, "lease-duration", whole)
		c.LeaseDuration = whole
	}
	renewAlias, err := applyAlias(&c.RenewDeadline, &c.RenewPeriod, "renew-deadline", "renew-period")
	if err != nil {
		return err
	}
	retryAlias, err := applyAlias(&c.RetryPeriod, &c.RetryTimer, "retry-period", "retry-timer")
	if err != nil {
		return err
	}
	c.defaultTimings()
	err = c.validateTimings()
	if err == nil || !(renewAlias || retryAlias) {
		return err
	}
	if renewAlias {
		c.RenewDeadline = 0
	}
	if retryAlias {
		c.RetryPeriod = 0
	}
	c.defaultTimings()
	if fallbackErr := c.validateTimings(); fallbackErr != nil {
		return fallbackErr
	}
	k.logger.Warn("deprecated renew-period or retry-timer value replaced by its default",
		"reason", err, "renew-deadline", c.RenewDeadline, "retry-period", c.RetryPeriod)
	return nil
}

// applyAlias moves a deprecated setting to its unset replacement and reports whether it did.
func applyAlias(value, alias *time.Duration, name, aliasName string) (bool, error) {
	deprecated := *alias
	*alias = 0
	if deprecated == 0 || *value == deprecated {
		return false, nil
	}
	if *value != 0 {
		return false, fmt.Errorf("%s and %s must match when both are set", aliasName, name)
	}
	*value = deprecated
	return true, nil
}

func (c *config) defaultTimings() {
	if c.RenewDeadline == 0 {
		c.RenewDeadline = c.LeaseDuration * 2 / 3
	}
	if c.RetryPeriod == 0 {
		c.RetryPeriod = min(defaultRetryPeriod, c.LeaseDuration/4)
	}
}

// validateTimings applies the client-go leader election constraints, and requires
// renewal to give up before other candidates can consider the Lease expired.
func (c *config) validateTimings() error {
	if c.RenewDeadline <= 0 || c.RetryPeriod <= 0 || c.RenewDeadline+c.RetryPeriod >= c.LeaseDuration {
		return fmt.Errorf("positive renew-deadline (%s) and retry-period (%s) must sum to less than lease-duration (%s)",
			c.RenewDeadline, c.RetryPeriod, c.LeaseDuration)
	}
	if c.RenewDeadline <= time.Duration(leaderelection.JitterFactor*float64(c.RetryPeriod)) {
		return fmt.Errorf("renew-deadline (%s) must be greater than retry-period (%s) * %g",
			c.RenewDeadline, c.RetryPeriod, leaderelection.JitterFactor)
	}
	return nil
}

// clientLimits returns the client-go request budget. Each owned Lease is renewed
// once per retry period, so the defaults exceed the client-go defaults of 5 and 10.
func (c *config) clientLimits() (float32, int) {
	qps, burst := float32(defaultQPS), defaultBurst
	if c.QPS > 0 {
		qps = c.QPS
	}
	if c.Burst > 0 {
		burst = c.Burst
	}
	return qps, burst
}

func (k *k8sLocker) SetLogger(logger *slog.Logger) {
	k.logger = lockers.BindLogger(logger, "k8s")
}

func (k *k8sLocker) String() string {
	b, _ := json.Marshal(k.Cfg)
	return string(b)
}
