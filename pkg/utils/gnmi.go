package utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/types"
)

const (
	SubscriptionMode_STREAM               = "STREAM"
	SubscriptionMode_ONCE                 = "ONCE"
	SubscriptionMode_POLL                 = "POLL"
	SubscriptionStreamMode_TARGET_DEFINED = "TARGET_DEFINED"
	SubscriptionStreamMode_ON_CHANGE      = "ON_CHANGE"
	SubscriptionStreamMode_SAMPLE         = "SAMPLE"
)

const (
	subscriptionDefaultMode       = SubscriptionMode_STREAM
	subscriptionDefaultStreamMode = SubscriptionStreamMode_TARGET_DEFINED
	subscriptionDefaultEncoding   = "JSON"
)

var ErrConfig = errors.New("config error")

func CreateSubscribeRequest(cfg *types.SubscriptionConfig, tc *types.TargetConfig, defaultEncoding string) (*gnmi.SubscribeRequest, error) {
	if err := validateAndSetDefaults(cfg); err != nil {
		return nil, err
	}
	gnmiOpts, err := SubscriptionOpts(cfg, tc, defaultEncoding)
	if err != nil {
		return nil, err
	}
	return api.NewSubscribeRequest(gnmiOpts...)
}

func validateAndSetDefaults(sc *types.SubscriptionConfig) error {
	numPaths := len(sc.Paths)
	numStreamSubs := len(sc.StreamSubscriptions)
	if sc.Prefix == "" && numPaths == 0 && numStreamSubs == 0 {
		return fmt.Errorf("%w: missing path(s) in subscription %q", ErrConfig, sc.Name)
	}

	if numPaths > 0 && numStreamSubs > 0 {
		return fmt.Errorf("%w: subscription %q: cannot set 'paths' and 'stream-subscriptions' at the same time", ErrConfig, sc.Name)
	}

	// validate subscription Mode
	switch strings.ToUpper(sc.Mode) {
	case "":
		sc.Mode = subscriptionDefaultMode
	case "ONCE", "POLL":
		if numStreamSubs > 0 {
			return fmt.Errorf("%w: subscription %q: cannot set 'stream-subscriptions' and 'mode'", ErrConfig, sc.Name)
		}
	case "STREAM":
	default:
		return fmt.Errorf("%w: subscription %s: unknown subscription mode %q", ErrConfig, sc.Name, sc.Mode)
	}
	// validate encoding
	if sc.Encoding != nil {
		switch strings.ToUpper(strings.ReplaceAll(*sc.Encoding, "-", "_")) {
		case "":
			sc.Encoding = pointer.ToString(subscriptionDefaultEncoding)
		case "JSON":
		case "BYTES":
		case "PROTO":
		case "ASCII":
		case "JSON_IETF":
		default:
			// allow integer encoding values
			_, err := strconv.Atoi(*sc.Encoding)
			if err != nil {
				return fmt.Errorf("%w: subscription %s: unknown encoding type %q", ErrConfig, sc.Name, *sc.Encoding)
			}
		}
	}

	// validate subscription stream mode
	if strings.ToUpper(sc.Mode) == "STREAM" {
		if len(sc.StreamSubscriptions) == 0 {
			switch strings.ToUpper(strings.ReplaceAll(sc.StreamMode, "-", "_")) {
			case "":
				sc.StreamMode = subscriptionDefaultStreamMode
			case "TARGET_DEFINED":
			case "SAMPLE":
			case "ON_CHANGE":
			default:
				return fmt.Errorf("%w: subscription %s: unknown stream-mode type %q", ErrConfig, sc.Name, sc.StreamMode)
			}
			return nil
		}

		// stream subscriptions
		for i, scs := range sc.StreamSubscriptions {
			if scs.Mode != "" {
				return fmt.Errorf("%w: subscription %s/%d: 'mode' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Prefix != "" {
				return fmt.Errorf("%w: subscription %s/%d: 'prefix' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Target != "" {
				return fmt.Errorf("%w: subscription %s/%d: 'target' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.SetTarget {
				return fmt.Errorf("%w: subscription %s/%d: 'set-target' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Encoding != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'encoding' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.History != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'history' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Models != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'models' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.UpdatesOnly {
				return fmt.Errorf("%w: subscription %s/%d: 'updates-only' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.StreamSubscriptions != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'subscriptions' attribute cannot be set", ErrConfig, sc.Name, i)
			}
			if scs.Qos != nil {
				return fmt.Errorf("%w: subscription %s/%d: 'qos' attribute cannot be set", ErrConfig, sc.Name, i)
			}

			switch strings.ReplaceAll(strings.ToUpper(scs.StreamMode), "-", "_") {
			case "":
				scs.StreamMode = subscriptionDefaultStreamMode
			case "TARGET_DEFINED":
			case "SAMPLE":
			case "ON_CHANGE":
			default:
				return fmt.Errorf("%w: subscription %s/%d: unknown subscription stream mode %q", ErrConfig, sc.Name, i, scs.StreamMode)
			}
		}
	}
	return nil
}

func SubscriptionOpts(sc *types.SubscriptionConfig, tc *types.TargetConfig, defaultEncoding string) ([]api.GNMIOption, error) {
	gnmiOpts := make([]api.GNMIOption, 0, 4)

	gnmiOpts = append(gnmiOpts,
		api.Prefix(sc.Prefix),
		api.SubscriptionListMode(sc.Mode),
		api.UpdatesOnly(sc.UpdatesOnly),
	)
	// encoding
	switch {
	case sc.Encoding != nil:
		gnmiOpts = append(gnmiOpts, api.Encoding(*sc.Encoding))
	case tc != nil && tc.Encoding != nil:
		gnmiOpts = append(gnmiOpts, api.Encoding(*tc.Encoding))
	default:
		gnmiOpts = append(gnmiOpts, api.Encoding(defaultEncoding))
	}

	// history extension
	if sc.History != nil {
		if !sc.History.Snapshot.IsZero() {
			gnmiOpts = append(gnmiOpts, api.Extension_HistorySnapshotTime(sc.History.Snapshot))
		}
		if !sc.History.Start.IsZero() && !sc.History.End.IsZero() {
			gnmiOpts = append(gnmiOpts, api.Extension_HistoryRange(sc.History.Start, sc.History.End))
		}
	}
	// QoS
	if sc.Qos != nil {
		gnmiOpts = append(gnmiOpts, api.Qos(*sc.Qos))
	}

	// add models
	for _, m := range sc.Models {
		gnmiOpts = append(gnmiOpts, api.UseModel(m, "", ""))
	}

	// add target opt
	if sc.Target != "" {
		gnmiOpts = append(gnmiOpts, api.Target(sc.Target))
	} else if sc.SetTarget {
		gnmiOpts = append(gnmiOpts, api.Target(tc.Name))
	}
	// add gNMI subscriptions
	// multiple stream subscriptions
	if len(sc.StreamSubscriptions) > 0 {
		for _, ssc := range sc.StreamSubscriptions {
			streamGNMIOpts, err := streamSubscriptionOpts(ssc)
			if err != nil {
				return nil, err
			}
			gnmiOpts = append(gnmiOpts, streamGNMIOpts...)
		}
	}

	for _, p := range sc.Paths {
		subGnmiOpts := make([]api.GNMIOption, 0, 2)
		switch gnmi.SubscriptionList_Mode(gnmi.SubscriptionList_Mode_value[strings.ToUpper(sc.Mode)]) {
		case gnmi.SubscriptionList_STREAM:
			switch gnmi.SubscriptionMode(gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(sc.StreamMode), "-", "_", -1)]) {
			case gnmi.SubscriptionMode_ON_CHANGE:
				if sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
			case gnmi.SubscriptionMode_TARGET_DEFINED:
				if sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
				if sc.SampleInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.SampleInterval(*sc.SampleInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SuppressRedundant(sc.SuppressRedundant))
				if sc.SuppressRedundant && sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
			case gnmi.SubscriptionMode_SAMPLE:
				if sc.SampleInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.SampleInterval(*sc.SampleInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SuppressRedundant(sc.SuppressRedundant))
				if sc.SuppressRedundant && sc.HeartbeatInterval != nil {
					subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
				}
				subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
			default:
				return nil, fmt.Errorf("%w: subscription %s unknown stream subscription mode %s", ErrConfig, sc.Name, sc.StreamMode)
			}
		default:
			// poll and once subscription modes
		}
		//
		subGnmiOpts = append(subGnmiOpts, api.Path(p))
		gnmiOpts = append(gnmiOpts,
			api.Subscription(subGnmiOpts...),
		)
	}

	// Depth extension
	if sc.Depth > 0 {
		gnmiOpts = append(gnmiOpts, api.Extension_Depth(sc.Depth))
	}
	return gnmiOpts, nil
}

func streamSubscriptionOpts(sc *types.SubscriptionConfig) ([]api.GNMIOption, error) {
	gnmiOpts := make([]api.GNMIOption, 0)
	for _, p := range sc.Paths {
		subGnmiOpts := make([]api.GNMIOption, 0, 2)
		switch gnmi.SubscriptionMode(gnmi.SubscriptionMode_value[strings.Replace(strings.ToUpper(sc.StreamMode), "-", "_", -1)]) {
		case gnmi.SubscriptionMode_ON_CHANGE:
			if sc.HeartbeatInterval != nil {
				subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
			}
			subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
		case gnmi.SubscriptionMode_SAMPLE, gnmi.SubscriptionMode_TARGET_DEFINED:
			if sc.SampleInterval != nil {
				subGnmiOpts = append(subGnmiOpts, api.SampleInterval(*sc.SampleInterval))
			}
			subGnmiOpts = append(subGnmiOpts, api.SuppressRedundant(sc.SuppressRedundant))
			if sc.SuppressRedundant && sc.HeartbeatInterval != nil {
				subGnmiOpts = append(subGnmiOpts, api.HeartbeatInterval(*sc.HeartbeatInterval))
			}
			subGnmiOpts = append(subGnmiOpts, api.SubscriptionMode(sc.StreamMode))
		default:
			return nil, fmt.Errorf("%w: subscription %s unknown stream subscription mode %s", ErrConfig, sc.Name, sc.StreamMode)
		}

		subGnmiOpts = append(subGnmiOpts, api.Path(p))
		gnmiOpts = append(gnmiOpts,
			api.Subscription(subGnmiOpts...),
		)
	}
	return gnmiOpts, nil
}
