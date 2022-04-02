package k8s_locker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/karimra/gnmic/lockers"
	"github.com/karimra/gnmic/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/utils/pointer"
)

const (
	defaultSessionTTL = 10 * time.Second
	defaultRetryTimer = 2 * time.Second
	defaultDelay      = 5 * time.Second
	loggingPrefix     = "[k8s_locker] "
)

func init() {
	lockers.Register("k8s", func() lockers.Locker {
		return &k8sLocker{
			Cfg:            &config{},
			m:              new(sync.Mutex),
			acquiredlocks:  make(map[string]*locks),
			attemtinglocks: make(map[string]*locks),
			logger:         log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			services:       make(map[string]context.CancelFunc),
		}
	})
}

type k8sLocker struct {
	Cfg            *config
	clientset      *kubernetes.Clientset
	logger         *log.Logger
	m              *sync.Mutex
	acquiredlocks  map[string]*locks
	attemtinglocks map[string]*locks
	services       map[string]context.CancelFunc
	//
	identity string
}

type config struct {
	Address     string `mapstructure:"address,omitempty" json:"address,omitempty"`
	Namespace   string
	SessionTTL  time.Duration `mapstructure:"session-ttl,omitempty" json:"session-ttl,omitempty"`
	Delay       time.Duration `mapstructure:"delay,omitempty" json:"delay,omitempty"`
	RetryTimer  time.Duration `mapstructure:"retry-timer,omitempty" json:"retry-timer,omitempty"`
	RenewPeriod time.Duration `mapstructure:"renew-period,omitempty" json:"renew-period,omitempty"`
	Debug       bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

type locks struct {
	sessionID string
	doneChan  chan struct{}
}

func (k *k8sLocker) Init(ctx context.Context, cfg map[string]interface{}, opts ...lockers.Option) error {
	err := lockers.DecodeConfig(cfg, k.Cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(k)
	}
	err = k.setDefaults()
	if err != nil {
		return err
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		return err
	}
	k.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func (k *k8sLocker) Lock(ctx context.Context, key string, val []byte) (bool, error) {
	// k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Watch(ctx context.Context, opts metav1.ListOptions)
	ll := resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name: key,
		},
		Client: k.clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: k.identity,
		},
	}

	err := ll.Create(ctx, resourcelock.LeaderElectionRecord{
		HolderIdentity: k.identity,
	})
	if err != nil {
		return false, err
	}
	cfgMap := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: key, Namespace: k.Cfg.Namespace},
		Immutable:  pointer.Bool(true),
		// Data:       map[string]string{},
		BinaryData: map[string][]byte{
			"value": val,
		},
	}
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			_, err := k.clientset.CoreV1().ConfigMaps(k.Cfg.Namespace).Create(ctx, cfgMap, metav1.CreateOptions{})
			fmt.Println("create config map:", err)
			if err == nil {
				return true, nil
			}
			time.Sleep(k.Cfg.RetryTimer)
		}
	}
	return false, nil
}

func (k *k8sLocker) KeepLock(ctx context.Context, key string) (chan struct{}, chan error) {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	return doneChan, errChan
}

func (k *k8sLocker) Unlock(ctx context.Context, key string) error {
	return nil
}

func (k *k8sLocker) Stop() error {
	k.m.Lock()
	defer k.m.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for key := range k.acquiredlocks {
		k.Unlock(ctx, key)
	}
	return nil
}

func (k *k8sLocker) SetLogger(logger *log.Logger) {
	if logger != nil && k.logger != nil {
		k.logger.SetOutput(logger.Writer())
		k.logger.SetFlags(logger.Flags())
	}
}

// helpers

func (k *k8sLocker) setDefaults() error {
	if k.Cfg.SessionTTL <= 0 {
		k.Cfg.SessionTTL = defaultSessionTTL
	}
	if k.Cfg.RetryTimer <= 0 {
		k.Cfg.RetryTimer = defaultRetryTimer
	}
	if k.Cfg.RenewPeriod <= 0 || k.Cfg.RenewPeriod >= k.Cfg.SessionTTL {
		k.Cfg.RenewPeriod = k.Cfg.SessionTTL / 2
	}
	if k.Cfg.Delay < 0 {
		k.Cfg.Delay = defaultDelay
	}
	if k.Cfg.Delay > 60*time.Second {
		k.Cfg.Delay = 60 * time.Second
	}
	return nil
}

func (k *k8sLocker) String() string {
	b, err := json.Marshal(k.Cfg)
	if err != nil {
		return ""
	}
	return string(b)
}
