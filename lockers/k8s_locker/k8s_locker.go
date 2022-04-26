package k8s_locker

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/karimra/gnmic/lockers"
	"github.com/karimra/gnmic/utils"
	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/utils/pointer"
)

const (
	defaultLeaseDuration = 10 * time.Second
	loggingPrefix        = "[k8s_locker] "
	defaultNamespace     = "default"
	origKeyName          = "original-key"
)

func init() {
	lockers.Register("k8s", func() lockers.Locker {
		return &k8sLocker{
			Cfg:            &config{},
			m:              new(sync.RWMutex),
			acquiredlocks:  make(map[string]*lock),
			attemtinglocks: make(map[string]*lock),
			logger:         log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type k8sLocker struct {
	Cfg            *config
	clientset      *kubernetes.Clientset
	logger         *log.Logger
	m              *sync.RWMutex
	acquiredlocks  map[string]*lock
	attemtinglocks map[string]*lock

	identity string // hostname
}

type config struct {
	Namespace     string        `mapstructure:"namespace,omitempty" json:"namespace,omitempty"`
	LeaseDuration time.Duration `mapstructure:"lease-duration,omitempty" json:"lease-duration,omitempty"`
	RenewPeriod   time.Duration `mapstructure:"renew-period,omitempty" json:"renew-period,omitempty"`
	Debug         bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

type lock struct {
	lease    *coordinationv1.Lease
	doneChan chan struct{}
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
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	k.clientset, err = kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		return err
	}
	k.identity = k.getIdentity()
	return nil
}

func (k *k8sLocker) Lock(ctx context.Context, key string, val []byte) (bool, error) {
	nkey := strings.ReplaceAll(key, "/", "-")
	doneChan := make(chan struct{})
	l := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				origKeyName: key,
			},
			Name:      nkey,
			Namespace: k.Cfg.Namespace,
			Labels: map[string]string{
				"app": "gnmic",
				nkey:  string(val),
			},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       pointer.String(k.identity),
			LeaseDurationSeconds: pointer.Int32(int32(k.Cfg.LeaseDuration / time.Second)),
		},
	}
	k.m.Lock()
	k.attemtinglocks[nkey] = &lock{
		lease:    l,
		doneChan: doneChan,
	}
	k.m.Unlock()
	// cleanup when done
	defer func() {
		k.m.Lock()
		defer k.m.Unlock()
		delete(k.attemtinglocks, nkey)
	}()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-doneChan:
			return false, lockers.ErrCanceled
		default:
			now := metav1.NowMicro()
			var ol *coordinationv1.Lease
			var err error
			// get or create
			ol, err = k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Get(ctx, nkey, metav1.GetOptions{})
			if err != nil {
				if !errors.IsNotFound(err) {
					return false, err
				}
				// create lease
				k.logger.Printf("lease %q not found, creating it: %+v", nkey, l.String())
				l.Spec.AcquireTime = &now
				l.Spec.RenewTime = &now
				ol, err = k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Create(ctx, l, metav1.CreateOptions{})
				if err != nil {
					return false, err
				}
				k.m.Lock()
				k.acquiredlocks[nkey] = &lock{
					lease:    ol,
					doneChan: doneChan,
				}
				k.m.Unlock()
				return true, nil
			}
			// obtained, compare
			if ol != nil && ol.Spec.HolderIdentity != nil && *ol.Spec.HolderIdentity != "" {
				k.logger.Printf("%q held by other instance: %v", ol.Name, *ol.Spec.HolderIdentity != k.identity)
				k.logger.Printf("%q lease has renewTime: %v", ol.Name, ol.Spec.RenewTime != nil)
				if *ol.Spec.HolderIdentity != k.identity && ol.Spec.RenewTime != nil {
					expectedRenewTime := ol.Spec.RenewTime.Add(time.Duration(*ol.Spec.LeaseDurationSeconds) * time.Second)
					k.logger.Printf("%q existing lease renew time %v", ol.Name, ol.Spec.RenewTime)
					k.logger.Printf("%q expected lease renew time %v", ol.Name, expectedRenewTime)
					k.logger.Printf("%q renew time passed: %v", ol.Name, expectedRenewTime.Before(now.Time))
					if !expectedRenewTime.Before(now.Time) {
						k.logger.Printf("%q is currently held by %s", ol.Name, *ol.Spec.HolderIdentity)
						time.Sleep(k.Cfg.RenewPeriod)
						continue
					}
				}
			}
			k.logger.Printf("taking over lease %q", nkey)
			// update the lease
			now = metav1.NowMicro()
			l.Spec.AcquireTime = &now
			l.Spec.RenewTime = &now
			// set resource version to the latest value known
			l.SetResourceVersion(ol.GetResourceVersion())
			k.logger.Printf("%q updating with %+v", l.Name, l)
			ol, err = k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Update(ctx, l, metav1.UpdateOptions{})
			if err != nil {
				return false, err
			}
			k.m.Lock()
			if lc, ok := k.acquiredlocks[nkey]; ok {
				lc.lease = ol
			} else {
				k.acquiredlocks[nkey] = &lock{lease: ol, doneChan: doneChan}
			}
			k.m.Unlock()
			return true, nil
		}
	}
}

func (k *k8sLocker) KeepLock(ctx context.Context, key string) (chan struct{}, chan error) {
	doneChan := make(chan struct{})
	errChan := make(chan error)
	nkey := strings.ReplaceAll(key, "/", "-")

	go func() {
		defer close(doneChan)
		ticker := time.NewTicker(k.Cfg.RenewPeriod)
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case <-doneChan:
				return
			case <-ticker.C:
				k.m.RLock()
				lock, ok := k.acquiredlocks[nkey]
				k.m.RUnlock()
				if !ok {
					errChan <- fmt.Errorf("unable to maintain lock %q: not found in acquiredlocks", nkey)
					return
				}
				ol, err := k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Get(ctx, nkey, metav1.GetOptions{})
				if err != nil {
					errChan <- fmt.Errorf("unable to maintain lock %q: %v", nkey, err)
					return
				}
				lock.lease.SetResourceVersion(ol.GetResourceVersion())
				switch k.compareLeases(lock.lease, ol) {
				case 0, 1:
					now := metav1.NowMicro()
					lock.lease.Spec.AcquireTime = &now
					lock.lease.Spec.RenewTime = &now
					ol, err = k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Update(ctx, lock.lease, metav1.UpdateOptions{})
					if err != nil {
						errChan <- fmt.Errorf("unable to update lock %q: %v", nkey, err)
						return
					}

					k.m.Lock()
					if lock, ok := k.acquiredlocks[nkey]; ok {
						lock.lease = ol
					}
					k.m.Unlock()
				case -1:
					errChan <- fmt.Errorf("%q failed to keep lease", nkey)
					return
				}
			}
		}
	}()
	return doneChan, errChan
}

func (k *k8sLocker) Unlock(ctx context.Context, key string) error {
	nkey := strings.ReplaceAll(key, "/", "-")
	k.m.Lock()
	defer k.m.Unlock()
	k.unlock(ctx, nkey)
	return nil
}

// assumes the mutex is locked
func (k *k8sLocker) unlock(ctx context.Context, key string) error {
	if lock, ok := k.acquiredlocks[key]; ok {
		delete(k.acquiredlocks, key)
		return k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Delete(ctx, lock.lease.Name, metav1.DeleteOptions{})
	}
	if lock, ok := k.attemtinglocks[key]; ok {
		delete(k.attemtinglocks, key)
		close(lock.doneChan)
		return k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Delete(ctx, lock.lease.Name, metav1.DeleteOptions{})
	}
	return nil
}

func (k *k8sLocker) Stop() error {
	k.m.Lock()
	defer k.m.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for key := range k.acquiredlocks {
		k.unlock(ctx, key)
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
	if k.Cfg.Namespace == "" {
		k.Cfg.Namespace = defaultNamespace
	}
	if k.Cfg.LeaseDuration <= 0 {
		k.Cfg.LeaseDuration = defaultLeaseDuration
	}
	if k.Cfg.RenewPeriod <= 0 || k.Cfg.RenewPeriod >= k.Cfg.LeaseDuration {
		k.Cfg.RenewPeriod = k.Cfg.LeaseDuration / 2
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

// compares 2 Leases, assume l1 is not nil and has a valid holderIdentity value.
// returns 0 if l1 and l2 have the same holder identity
// return 1 if l2 is nil, has no holder or has an expired renewTime
// returns -1 if l2 has another holder identity and has a valid renewTime
func (l *k8sLocker) compareLeases(l1, l2 *coordinationv1.Lease) int {
	if l2 == nil {
		return 1
	}
	if l2.Spec.HolderIdentity == nil {
		return 1
	}
	now := time.Now()
	if *l2.Spec.HolderIdentity == "" {
		return 1
	}
	if *l1.Spec.HolderIdentity != *l2.Spec.HolderIdentity {
		if l2.Spec.RenewTime == nil {
			return 1
		}
		expectedRenewTime := l2.Spec.RenewTime.Add(time.Duration(*l2.Spec.LeaseDurationSeconds) * time.Second)
		if expectedRenewTime.Before(now) {
			return 1
		} else {
			return -1
		}
	}
	return 0
}

func (l *k8sLocker) getIdentity() string {
	name, err := os.Hostname()
	if err != nil {
		return uuid.NewString()
	}
	return name
}
