package event_rate_limit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/openconfig/gnmic/formatters"
	"github.com/openconfig/gnmic/types"
	"github.com/openconfig/gnmic/utils"
)

const (
	processorType          = "event-rate-limit"
	loggingPrefix          = "[" + processorType + "] "
	defaultCacheSize       = 1000
	oneSecond        int64 = int64(time.Second)
)

var (
	eqChar = []byte("=")
	lfChar = []byte("\n")
)

// RateLimit rate-limits the message to the given rate.
type RateLimit struct {
	// formatters.EventProcessor

	PerSecondLimit float64 `mapstructure:"per-second,omitempty" json:"per-second,omitempty"`
	CacheSize      int     `mapstructure:"cache-size,omitempty" json:"cache-size,omitempty"`
	Debug          bool    `mapstructure:"debug,omitempty" json:"debug,omitempty"`

	// eventIndex is an lru cache used to compare the events hash with known value.
	// LRU cache seems like a good choice because we expect the rate-limiter to be
	// most useful in burst scenarios.
	// We need some form of control over the size of the cache to contain RAM usage
	// so LRU is good in that respect also.
	eventIndex *lru.Cache[string, int64]
	logger     *log.Logger
}

func init() {
	formatters.Register(processorType, func() formatters.EventProcessor {
		return &RateLimit{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

func (o *RateLimit) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, o)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.CacheSize <= 0 {
		o.logger.Printf("using default value for lru size %d", defaultCacheSize)
		o.CacheSize = defaultCacheSize

	}
	if o.PerSecondLimit <= 0 {
		return fmt.Errorf("provided limit is %f, must be greater than 0", o.PerSecondLimit)
	}
	if o.logger.Writer() != io.Discard {
		b, err := json.Marshal(o)
		if err != nil {
			o.logger.Printf("initialized processor '%s': %+v", processorType, o)
			return nil
		}
		o.logger.Printf("initialized processor '%s': %s", processorType, string(b))
	}

	o.eventIndex, err = lru.New[string, int64](o.CacheSize)
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	return nil
}

func (o *RateLimit) Apply(es ...*formatters.EventMsg) []*formatters.EventMsg {
	validEs := make([]*formatters.EventMsg, 0, len(es))

	for _, e := range es {
		if e == nil {
			continue
		}
		h := hashEvent(e)
		ts, has := o.eventIndex.Get(h)
		// we check that we have the event hash in the map, if not, it's the first time we see the event
		if val := float64(e.Timestamp-ts) * o.PerSecondLimit; has && e.Timestamp != ts && int64(val) < oneSecond {
			// reject event
			o.logger.Printf("dropping event val %.2f lower than configured rate", val)
			continue
		}
		// retain the last event that passed through
		o.eventIndex.Add(h, e.Timestamp)
		validEs = append(validEs, e)
	}

	return validEs
}

func hashEvent(e *formatters.EventMsg) string {
	h := sha256.New()
	tagKeys := make([]string, len(e.Tags))
	i := 0
	for tagKey := range e.Tags {
		tagKeys[i] = tagKey
		i++
	}
	sort.Strings(tagKeys)

	for _, tagKey := range tagKeys {
		h.Write([]byte(tagKey))
		h.Write(eqChar)
		h.Write([]byte(e.Tags[tagKey]))
		h.Write(lfChar)
	}

	return hex.EncodeToString(h.Sum(nil))
}

func (o *RateLimit) WithLogger(l *log.Logger) {
	if o.Debug && l != nil {
		o.logger = log.New(l.Writer(), loggingPrefix, l.Flags())
	} else if o.Debug {
		o.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

func (o *RateLimit) WithTargets(tcs map[string]*types.TargetConfig) {}

func (o *RateLimit) WithActions(act map[string]map[string]interface{}) {}

func (o *RateLimit) WithProcessors(procs map[string]map[string]any) {}
