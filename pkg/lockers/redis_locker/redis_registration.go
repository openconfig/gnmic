package redis_locker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	goredis "github.com/redis/go-redis/v9"

	"github.com/openconfig/gnmic/pkg/lockers"
)

// defaultWatchTimeout
const defaultWatchTimeout = 10 * time.Second

// redisRegistration represents a gnmic endpoint in redis.
// It's serialised in the redis value to allow recovering
// it during service discovery.
type redisRegistration struct {
	ID      string
	Address string
	Port    int
	Tags    []string
	Rand    string
}

func (k *redisLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	ctx, cancel := context.WithCancel(ctx)
	k.m.Lock()
	k.registerLock[s.ID] = cancel
	k.m.Unlock()
	if k.Cfg.Debug {
		k.logger.Printf("locking service=%s", s.ID)
	}
	mutex := k.redisLocker.NewMutex(
		fmt.Sprintf("%s-%s", s.Name, s.ID),
		redsync.WithGenValueFunc(func() (string, error) {
			rand, err := k.genRandValue()
			if err != nil {
				return "", err
			}
			reg := &redisRegistration{
				ID:      s.ID,
				Address: s.Address,
				Port:    s.Port,
				Tags:    s.Tags,
				Rand:    rand,
			}
			val, err := json.Marshal(reg)
			if err != nil {
				return "", err
			}
			return string(val), nil
		}),
		redsync.WithExpiry(s.TTL),
	)

	err := mutex.LockContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to lock service=%s, %w", s.ID, err)
	}

	ticker := time.NewTicker(s.TTL / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ok, err := mutex.ExtendContext(ctx)
			if err != nil {
				return fmt.Errorf("failed to extend lock for service=%s: %w", s.ID, err)
			}
			if !ok {
				return fmt.Errorf("could not extend lock for service=%s", s.ID)
			}
		case <-ctx.Done():
			mutex.Unlock()
			return nil
		}
	}
}

func (k *redisLocker) Deregister(s string) error {
	k.m.Lock()
	defer k.m.Unlock()
	for sid, lockCancel := range k.registerLock {
		if k.Cfg.Debug {
			k.logger.Printf("unlocking service=%s", sid)
		}
		lockCancel()
		delete(k.registerLock, sid)
	}
	return nil
}

func (k *redisLocker) WatchServices(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration) error {
	if watchTimeout <= 0 {
		watchTimeout = defaultWatchTimeout
	}
	var err error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if k.Cfg.Debug {
				k.logger.Printf("(re)starting watch service=%q", serviceName)
			}
			err = k.watch(ctx, serviceName, tags, sChan, watchTimeout)
			if err != nil {
				k.logger.Printf("watch ended with error: %s", err)
				time.Sleep(k.Cfg.RetryTimer)
				continue
			}

			time.Sleep(k.Cfg.PollTimer)
		}
	}
}

func (k *redisLocker) watch(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration) error {
	// timeoutSeconds := int64(watchTimeout.Seconds())
	// TODO: implement watch
	services, err := k.GetServices(ctx, serviceName, tags)
	if err != nil {
		return err
	}

	sChan <- services
	return nil
}

func (k *redisLocker) getBatchOfKeys(ctx context.Context, key string, batchSize int64, cursor uint64) (uint64, map[string]*goredis.StringCmd, error) {
	keys, cursor, err := k.client.Scan(
		ctx,
		cursor,
		key,
		batchSize,
	).Result()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	results := map[string]*goredis.StringCmd{}
	_, err = k.client.Pipelined(ctx, func(p goredis.Pipeliner) error {
		for _, k := range keys {
			results[k] = p.Get(ctx, k)
		}
		return nil
	})

	if err != nil {
		return cursor, nil, fmt.Errorf("error getting contents of keys")
	}

	return cursor, results, nil
}

func (k *redisLocker) GetServices(ctx context.Context, serviceName string, tags []string) ([]*lockers.Service, error) {
	var pageSize int64 = 50
	var cursor uint64
	var err error
	var cmds map[string]*goredis.StringCmd
	discoveredServiceRegistrations := []*redisRegistration{}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// to select all gnmic instances, matching the given prefix
			cursor, cmds, err = k.getBatchOfKeys(
				ctx,
				fmt.Sprintf("%s-*", serviceName),
				pageSize,
				cursor,
			)

			if err != nil {
				return nil, err
			}
			for _, cmd := range cmds {
				bytesVal, err := cmd.Bytes()
				if err != nil {
					// key removed from redis
					// could be that it has expired
					// doesn't make a difference, we skip it
					continue
				}
				serviceRegistration := &redisRegistration{}
				if err := json.Unmarshal(bytesVal, serviceRegistration); err != nil {
					// we don't have the data we expect
					// skip it
					continue
				}

				discoveredServiceRegistrations = append(
					discoveredServiceRegistrations,
					serviceRegistration,
				)
			}
			// termination condition for redis scan
			if cursor == 0 {
				if k.Cfg.Debug {
					k.logger.Printf("got %d services from redis", len(discoveredServiceRegistrations))
				}
				// convert discovered servicesRegistrations to services
				discoveredServices := make([]*lockers.Service, len(discoveredServiceRegistrations))
				for i, registration := range discoveredServiceRegistrations {
					// match the required tags
					if !matchTags(registration.Tags, tags) {
						continue
					}
					discoveredServices[i] = &lockers.Service{
						ID:   registration.ID,
						Tags: registration.Tags,
						Address: fmt.Sprintf(
							"%s:%d",
							registration.Address,
							registration.Port,
						),
					}
				}
				return discoveredServices, nil
			}
		}
	}
}

func (k *redisLocker) IsLocked(ctx context.Context, key string) (bool, error) {
	count, err := k.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("error during redis query: %w", err)
	}

	if count > 0 {
		return true, nil
	}
	return false, nil
}

func (k *redisLocker) List(ctx context.Context, prefix string) (map[string]string, error) {
	var cursor uint64
	var err error
	var cmds map[string]*goredis.StringCmd
	data := map[string]string{}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		cursor, cmds, err = k.getBatchOfKeys(
			ctx,
			fmt.Sprintf("%s*", prefix),
			100,
			cursor,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch from redis: %w", err)
		}
		if k.Cfg.Debug {
			k.logger.Printf(
				"got %d keys from redis for prefix=%s",
				len(cmds),
				prefix,
			)
		}
		for key, cmd := range cmds {
			bytesVal, err := cmd.Bytes()
			if err != nil {
				// key removed from redis
				// could be that it has expired
				// doesn't make a difference, we skip it
				continue
			}
			// we add a random string at the end of the value for redis
			// redlock algorithm, so we need to remove it here
			lastIndex := bytes.LastIndex(bytesVal, []byte("-"))
			// if it's not there, we skip the key
			if lastIndex < 0 {
				continue
			}
			data[key] = string(bytesVal[:lastIndex])
		}

		if cursor == 0 {
			return data, nil
		}
	}
}

func matchTags(tags, wantedTags []string) bool {
	if wantedTags == nil {
		return true
	}
	tagsMap := map[string]struct{}{}

	for _, t := range tags {
		tagsMap[t] = struct{}{}
	}

	for _, wt := range wantedTags {
		if _, ok := tagsMap[wt]; !ok {
			return false
		}
	}
	return true
}
