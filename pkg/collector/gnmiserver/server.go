package gnmiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	gnmiserver "github.com/openconfig/gnmic/pkg/api/server"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/cache"
	inputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/inputs"
	outputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/outputs"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zestor-dev/zestor/store"
)

type Server struct {
	configStore    store.Store[any]
	targetsManager *targets_manager.TargetsManager
	outputsManager *outputs_manager.OutputsManager
	inputsManager  *inputs_manager.InputsManager
	cache          cache.Cache

	logger *slog.Logger
	reg    *prometheus.Registry
}

func NewServer(
	configStore store.Store[any],
	targetsManager *targets_manager.TargetsManager,
	outputsManager *outputs_manager.OutputsManager,
	inputsManager *inputs_manager.InputsManager,
	reg *prometheus.Registry,
) *Server {
	return &Server{
		configStore:    configStore,
		targetsManager: targetsManager,
		outputsManager: outputsManager,
		inputsManager:  inputsManager,
		reg:            reg,
	}
}

func (s *Server) Start(ctx context.Context, cache cache.Cache, wg *sync.WaitGroup) error {
	s.logger = logging.NewLogger(s.configStore, "component", "gnmi-server")
	cfg, ok, err := s.configStore.Get("gnmi-server", "gnmi-server")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if cfg == nil {
		return nil
	}
	switch cfg := cfg.(type) {
	case *config.GNMIServer:
		if cfg == nil {
			return nil
		}
		s.cache = cache
		var grpcKeepAlive *keepalive.ServerParameters
		if cfg.GRPCKeepalive != nil {
			grpcKeepAlive = cfg.GRPCKeepalive.Convert()
		}
		srv, err := gnmiserver.New(gnmiserver.Config{
			Address:              cfg.Address,
			MaxUnaryRPC:          cfg.MaxUnaryRPC,
			MaxStreamingRPC:      cfg.MaxSubscriptions,
			MaxRecvMsgSize:       cfg.MaxRecvMsgSize,
			MaxSendMsgSize:       cfg.MaxSendMsgSize,
			MaxConcurrentStreams: cfg.MaxConcurrentStreams,
			TCPKeepalive:         cfg.TCPKeepalive,
			Keepalive:            grpcKeepAlive,
			RateLimit:            cfg.RateLimit,
			Timeout:              cfg.Timeout,
			TLS:                  cfg.TLS,
		},
			gnmiserver.WithLogger(log.New(os.Stderr, "", log.LstdFlags)), // TODO: use s.logger
			gnmiserver.WithGetHandler(s.getHandler),
			gnmiserver.WithSetHandler(s.setHandler),
			gnmiserver.WithSubscribeHandler(s.subscribeHandler),
			gnmiserver.WithRegistry(s.reg),
		)
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := srv.Start(ctx)
			if err != nil {
				s.logger.Error("failed to start gNMI server", "error", err)
			}
		}()
		return nil
	default:
		return errors.New("invalid gnmi-server config")
	}
}

func (s *Server) Stop() error {
	return nil
}

func (s *Server) getHandler(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	s.logger.Info("received Get request", "request", req)
	prefix := req.GetPrefix()
	paths := req.GetPath()
	response := &gnmi.GetResponse{
		Notification: make([]*gnmi.Notification, 0),
	}
	if prefix == nil && len(paths) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing path")
	}
	origin, err := getOrigin(prefix, req.GetPath())
	if err != nil {
		return nil, err
	}
	if origin == "" {
		origin = "gnmic"
	}
	if origin != "gnmic" {
		return nil, status.Errorf(codes.InvalidArgument, "origin mismatch: %v", origin)
	}
	for _, path := range paths {
		firstElem, err := getFirstElem(prefix, path)
		if err != nil {
			return nil, err
		}
		switch firstElem.GetName() {
		case "targets":
			notifications, err := s.getTargetsHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		case "subscriptions":
			notifications, err := s.getSubscriptionsHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		case "outputs":
			notifications, err := s.getOutputsHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		case "inputs":
			notifications, err := s.getInputsHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		case "processors":
			notifications, err := s.getProcessorsHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		case "clustering":
			notifications, err := s.getClusteringHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		case "gnmi-server":
			notifications, err := s.getGNMIServerHandler(ctx, req)
			if err != nil {
				return nil, err
			}
			response.Notification = append(response.Notification, notifications...)
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown path element %q", firstElem.GetName())
		}
	}
	return response, nil
}

func (s *Server) setHandler(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	return nil, nil
}

func (s *Server) subscribeHandler(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer) error {
	return nil
}

func getOrigin(pr *gnmi.Path, p []*gnmi.Path) (string, error) {
	var origin string
	if pr != nil && pr.GetOrigin() != "" {
		origin = pr.GetOrigin()
	} else {
		// Find the first non-empty origin among the paths, if any.
		for _, path := range p {
			if path != nil && path.GetOrigin() != "" {
				origin = path.GetOrigin()
				break
			}
		}
	}

	if origin != "" {
		// Verify that all origins (if set) match the found origin.
		if pr != nil && pr.GetOrigin() != "" && pr.GetOrigin() != origin {
			return "", errors.New("origin mismatch")
		}
		for _, path := range p {
			if path != nil && path.GetOrigin() != "" && path.GetOrigin() != origin {
				return "", errors.New("origin mismatch")
			}
		}
	}

	return origin, nil
}

func getFirstElem(pr, p *gnmi.Path) (*gnmi.PathElem, error) {
	if pr != nil && pr.GetElem() != nil && len(pr.GetElem()) > 0 {
		return pr.GetElem()[0], nil
	}
	if p != nil && p.GetElem() != nil && len(p.GetElem()) > 0 {
		return p.GetElem()[0], nil
	}
	return nil, status.Errorf(codes.InvalidArgument, "missing path")
}

func (s *Server) getTargetsHandler(ctx context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for targets", "request", req)
	notifications := make([]*gnmi.Notification, 0)
	targets, err := s.configStore.List("targets")
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if tg, ok := target.(*types.TargetConfig); ok {
				notification, err := targetConfigToNotification(tg, req.GetEncoding())
				if err != nil {
					return nil, err
				}
				notifications = append(notifications, notification)
			} else {
				return nil, status.Errorf(codes.Internal, "invalid target config: %v", target)
			}
		}
	}
	return notifications, nil
}

func (s *Server) getSubscriptionsHandler(ctx context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for subscriptions", "request", req)
	notifications := make([]*gnmi.Notification, 0)
	subscriptions, err := s.configStore.List("subscriptions")
	if err != nil {
		return nil, err
	}
	for _, subscription := range subscriptions {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if sub, ok := subscription.(*types.SubscriptionConfig); ok {
				notification, err := subscriptionConfigToNotification(sub, req.GetEncoding())
				if err != nil {
					return nil, err
				}
				notifications = append(notifications, notification)
			} else {
				return nil, status.Errorf(codes.Internal, "invalid subscription config: %v", subscription)
			}
		}
	}
	return notifications, nil
}

func (s *Server) getOutputsHandler(ctx context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for outputs", "request", req)
	notifications := make([]*gnmi.Notification, 0)
	outputs, err := s.configStore.List("outputs")
	if err != nil {
		return nil, err
	}
	for _, output := range outputs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if outputCfg, ok := output.(map[string]any); ok {
				notification, err := outputConfigToNotification(outputCfg, req.GetEncoding())
				if err != nil {
					return nil, err
				}
				notifications = append(notifications, notification)
			} else {
				return nil, status.Errorf(codes.Internal, "invalid output config: %v", output)
			}
		}
	}
	return notifications, nil
}

func (s *Server) getInputsHandler(ctx context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for inputs", "request", req)
	notifications := make([]*gnmi.Notification, 0)
	inputs, err := s.configStore.List("inputs")
	if err != nil {
		return nil, err
	}
	for _, input := range inputs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if inputCfg, ok := input.(map[string]any); ok {
				notification, err := inputConfigToNotification(inputCfg, req.GetEncoding())
				if err != nil {
					return nil, err
				}
				notifications = append(notifications, notification)
			} else {
				return nil, status.Errorf(codes.Internal, "invalid input config: %v", input)
			}
		}
	}
	return notifications, nil
}

func (s *Server) getProcessorsHandler(ctx context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for processors", "request", req)
	notifications := make([]*gnmi.Notification, 0)
	processors, err := s.configStore.List("processors")
	if err != nil {
		return nil, err
	}
	for _, processor := range processors {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if processorCfg, ok := processor.(map[string]any); ok {
				notification, err := processorConfigToNotification(processorCfg, req.GetEncoding())
				if err != nil {
					return nil, err
				}
				notifications = append(notifications, notification)
			} else {
				return nil, status.Errorf(codes.Internal, "invalid processor config: %v", processor)
			}
		}
	}
	return notifications, nil
}

func (s *Server) getClusteringHandler(_ context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for clustering", "request", req)
	clusteringCfg, ok, err := s.configStore.Get("clustering", "clustering")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if clusteringCfg == nil {
		return nil, nil
	}
	if clustering, ok := clusteringCfg.(*config.Clustering); ok {
		b, _ := json.Marshal(clustering)
		return []*gnmi.Notification{
			{
				Timestamp: time.Now().UnixNano(),
				Update: []*gnmi.Update{
					{
						Path: &gnmi.Path{
							Origin: "gnmic",
							Elem: []*gnmi.PathElem{
								{
									Name: "clustering",
								},
							},
						},
						Val: &gnmi.TypedValue{
							Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
						},
					},
				},
			},
		}, nil
	}
	return nil, status.Errorf(codes.Internal, "invalid clustering config: %v", clusteringCfg)
}

func (s *Server) getGNMIServerHandler(ctx context.Context, req *gnmi.GetRequest) ([]*gnmi.Notification, error) {
	s.logger.Info("received Get request for gnmi-server", "request", req)
	cfg, ok, err := s.configStore.Get("gnmi-server", "gnmi-server")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Errorf(codes.NotFound, "gnmi-server config not found")
	}
	_ = cfg
	return nil, nil
	// return []*gnmi.Notification{gnmiServerConfigToNotification(cfg)}, nil
}

func targetConfigToNotification(target *types.TargetConfig, encoding gnmi.Encoding) (*gnmi.Notification, error) {
	switch encoding {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(target)
		return &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "target",
								Key:  map[string]string{"name": target.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}, nil
	case gnmi.Encoding_BYTES:
		return &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "target",
								Key:  map[string]string{"name": target.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(target.Address)},
					},
				},
			},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown encoding: %v", encoding)
	}
}

func subscriptionConfigToNotification(subscription *types.SubscriptionConfig, encoding gnmi.Encoding) (*gnmi.Notification, error) {
	switch encoding {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(subscription)
		return &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "subscription",
								Key:  map[string]string{"name": subscription.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown encoding: %v", encoding)
	}
}

func outputConfigToNotification(output map[string]any, encoding gnmi.Encoding) (*gnmi.Notification, error) {
	outputName, ok := output["name"].(string)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "output name is required")
	}
	if outputName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "output name is required")
	}
	switch encoding {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(output)
		return &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "output",
								Key:  map[string]string{"name": outputName},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown encoding: %v", encoding)
	}
}

func inputConfigToNotification(input map[string]any, encoding gnmi.Encoding) (*gnmi.Notification, error) {
	switch encoding {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		inputName, ok := input["name"].(string)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "input name is required")
		}
		if inputName == "" {
			return nil, status.Errorf(codes.InvalidArgument, "input name is required")
		}
		b, _ := json.Marshal(input)
		return &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "input",
								Key:  map[string]string{"name": inputName},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown encoding: %v", encoding)
	}
}

func processorConfigToNotification(processor map[string]any, encoding gnmi.Encoding) (*gnmi.Notification, error) {
	switch encoding {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		processorName, ok := processor["name"].(string)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "processor name is required")
		}
		if processorName == "" {
			return nil, status.Errorf(codes.InvalidArgument, "processor name is required")
		}
		b, _ := json.Marshal(processor)
		return &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "processor",
								Key:  map[string]string{"name": processorName},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown encoding: %v", encoding)
	}
}
