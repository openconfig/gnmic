// SPDX-License-Identifier: Apache-2.0

// Package gnmiserver implements the gNMI RPC handlers shared by the
// `subscribe` and `collect` commands' northbound gNMI servers:
//
//   - Subscribe RPCs are served from a cache kept in sync with the
//     configured subscriptions.
//   - Get and Set RPCs are relayed to the target(s) selected via the request
//     Prefix.Target field. Target resolution is command specific and is
//     provided by the caller as a TargetSelectFn.
//   - Get requests with paths under the `gnmic` origin are served by the
//     caller provided InternalGetFn, typically from the command's own
//     configuration.
//
// The handlers are meant to be registered on a pkg/api/server gRPC server.
package gnmiserver

import (
	"context"
	"log/slog"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/cache"
)

// Config holds the handlers' behavioral knobs,
// taken from the `gnmi-server` configuration section.
type Config struct {
	// DefaultSampleInterval is used when a stream/sample
	// subscription requests a zero sample interval.
	DefaultSampleInterval time.Duration
	// MinSampleInterval is used when a stream/sample subscription
	// requests a non-zero sample interval lower than this value.
	MinSampleInterval time.Duration
	// MinHeartbeatInterval is used when a subscription requests a
	// non-zero heartbeat interval lower than this value.
	MinHeartbeatInterval time.Duration
	// Debug enables additional debug logs.
	Debug bool
}

// TargetSelectFn resolves the request Prefix.Target value into a set of gNMI
// targets. The returned cleanup function is called once the RPC is done; it
// is used to release connections dialed on demand.
type TargetSelectFn func(ctx context.Context, target string) (map[string]*target.Target, func(), error)

// InternalGetFn serves Get requests with paths under the `gnmic` origin.
type InternalGetFn func(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error)

// Handlers implements the gNMI Get, Set and Subscribe RPC handlers.
type Handlers struct {
	cfg           Config
	cache         cache.Cache
	logger        *slog.Logger
	selectTargets TargetSelectFn
	internalGet   InternalGetFn
}

// New creates gNMI RPC handlers.
// c may be nil, in which case Subscribe RPCs are rejected with an
// `Unimplemented` status code.
func New(cfg Config, c cache.Cache, logger *slog.Logger, selectTargets TargetSelectFn, internalGet InternalGetFn) *Handlers {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Handlers{
		cfg:           cfg,
		cache:         c,
		logger:        logger,
		selectTargets: selectTargets,
		internalGet:   internalGet,
	}
}

// Capabilities handles a gNMI Capabilities RPC, advertising the gNMI version
// and the encodings the server can carry (relayed unary RPCs and cache-served
// subscriptions pass values through in the encoding produced by the targets).
func (h *Handlers) Capabilities(ctx context.Context, req *gnmi.CapabilityRequest) (*gnmi.CapabilityResponse, error) {
	return &gnmi.CapabilityResponse{
		GNMIVersion: "0.10.0",
		SupportedEncodings: []gnmi.Encoding{
			gnmi.Encoding_JSON,
			gnmi.Encoding_BYTES,
			gnmi.Encoding_PROTO,
			gnmi.Encoding_ASCII,
			gnmi.Encoding_JSON_IETF,
		},
	}, nil
}
