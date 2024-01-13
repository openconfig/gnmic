// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jhump/protoreflect/dynamic"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/types"
)

// Subscribe sends a gnmi.SubscribeRequest to the target *t, responses and error are sent to the target channels
func (t *Target) Subscribe(ctx context.Context, req *gnmi.SubscribeRequest, subscriptionName string) {
	var subscribeClient gnmi.GNMI_SubscribeClient
	var nctx context.Context
	var cancel context.CancelFunc
	var err error
SUBSC:
	select {
	case <-ctx.Done():
		return
	default:
		nctx, cancel = context.WithCancel(ctx)
		defer cancel()
		nctx = t.appendRequestMetadata(nctx)
		subscribeClient, err = t.Client.Subscribe(nctx, t.callOpts()...)
		if err != nil {
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              fmt.Errorf("failed to create a subscribe client, target='%s', retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
			}
			cancel()
			time.Sleep(t.Config.RetryTimer)
			goto SUBSC
		}
	}
	t.m.Lock()
	if cfn, ok := t.subscribeCancelFn[subscriptionName]; ok {
		cfn()
	}
	t.SubscribeClients[subscriptionName] = subscribeClient
	t.subscribeCancelFn[subscriptionName] = cancel
	subConfig := t.Subscriptions[subscriptionName]
	t.m.Unlock()

	err = subscribeClient.Send(req)
	if err != nil {
		t.errors <- &TargetError{
			SubscriptionName: subscriptionName,
			Err:              fmt.Errorf("target '%s' send error, retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
		}
		cancel()
		time.Sleep(t.Config.RetryTimer)
		goto SUBSC
	}

	switch req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_STREAM:
		err = t.handleStreamSubscriptionRcv(nctx, subscribeClient, subscriptionName, subConfig)
		if err != nil {
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              err,
			}
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              fmt.Errorf("retrying in %s", t.Config.RetryTimer),
			}
			cancel()
			time.Sleep(t.Config.RetryTimer)
			goto SUBSC
		}
	case gnmi.SubscriptionList_ONCE:
		err = t.handleONCESubscriptionRcv(nctx, subscribeClient, subscriptionName, subConfig)
		if err != nil {
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              err,
			}
			if errors.Is(err, io.EOF) {
				return
			}
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              fmt.Errorf("retrying in %d", t.Config.RetryTimer),
			}
			cancel()
			time.Sleep(t.Config.RetryTimer)
			goto SUBSC
		}
		return
	case gnmi.SubscriptionList_POLL:
		go t.listenPolls(nctx)
		err = t.handlePollSubscriptionRcv(nctx, subscribeClient, subscriptionName, subConfig)
		if err != nil {
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              err,
			}
			cancel()
			time.Sleep(t.Config.RetryTimer)
			goto SUBSC
		}
	}
}

func (t *Target) SubscribeOnceChan(ctx context.Context, req *gnmi.SubscribeRequest) (chan *gnmi.SubscribeResponse, chan error) {
	responseCh := make(chan *gnmi.SubscribeResponse)
	errCh := make(chan error)
	go func() {
		nctx, cancel := context.WithCancel(ctx)
		defer cancel()

		nctx = t.appendRequestMetadata(nctx)
		subscribeClient, err := t.Client.Subscribe(nctx, t.callOpts()...)
		if err != nil {
			errCh <- err
			return
		}
		err = subscribeClient.Send(req)
		if err != nil {
			errCh <- err
			return
		}
		for {
			response, err := subscribeClient.Recv()
			if err != nil {
				errCh <- err
				return
			}
			responseCh <- response
		}
	}()

	return responseCh, errCh
}

func (t *Target) SubscribeOnce(ctx context.Context, req *gnmi.SubscribeRequest) ([]*gnmi.SubscribeResponse, error) {
	responses := make([]*gnmi.SubscribeResponse, 0)
	rspChan, errChan := t.SubscribeOnceChan(ctx, req)
LOOP:
	for {
		select {
		case r := <-rspChan:
			switch r.Response.(type) {
			case *gnmi.SubscribeResponse_Update:
				responses = append(responses, r)
			case *gnmi.SubscribeResponse_SyncResponse:
				break LOOP
			}
		case err := <-errChan: // only non nil errors
			if err == io.EOF {
				break LOOP
			}
			return nil, err
		}
	}
	return responses, nil
}

func (t *Target) SubscribePoll(ctx context.Context, subName string) error {
	t.m.Lock()
	stream, ok := t.SubscribeClients[subName]
	t.m.Unlock()
	if !ok {
		return fmt.Errorf("unknown subscription name %q", subName)
	}
	return stream.Send(&gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Poll{
			Poll: new(gnmi.Poll),
		},
	})
}

func (t *Target) ReadSubscriptions() (chan *SubscribeResponse, chan *TargetError) {
	return t.subscribeResponses, t.errors
}

func (t *Target) NumberOfOnceSubscriptions() int {
	num := 0
	for _, sub := range t.Subscriptions {
		if strings.ToUpper(sub.Mode) == "ONCE" {
			num++
		}
	}
	return num
}

func (t *Target) DecodeProtoBytes(resp *gnmi.SubscribeResponse) error {
	if t.RootDesc == nil {
		return nil
	}
	switch resp := resp.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		for _, update := range resp.Update.Update {
			switch update.Val.Value.(type) {
			case *gnmi.TypedValue_ProtoBytes:
				m := dynamic.NewMessage(t.RootDesc.GetFile().FindMessage("Nokia.SROS.root"))
				err := m.Unmarshal(update.Val.GetProtoBytes())
				if err != nil {
					return err
				}
				jsondata, err := m.MarshalJSON()
				if err != nil {
					return err
				}
				update.Val.Value = &gnmi.TypedValue_JsonVal{JsonVal: jsondata}
			}
		}
	}
	return nil
}

func (t *Target) DeleteSubscription(name string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.subscribeCancelFn[name]()
	delete(t.subscribeCancelFn, name)
	delete(t.SubscribeClients, name)
	delete(t.Subscriptions, name)
}

func (t *Target) StopSubscription(name string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.subscribeCancelFn[name]()
	delete(t.subscribeCancelFn, name)
	delete(t.SubscribeClients, name)
}

func (t *Target) listenPolls(ctx context.Context) {
	for {
		select {
		case subName := <-t.pollChan:
			err := t.SubscribePoll(ctx, subName)
			if err != nil {
				t.errors <- &TargetError{
					SubscriptionName: subName,
					Err:              fmt.Errorf("failed to send PollRequest to subscription %s: %v", subName, err),
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (t *Target) handleStreamSubscriptionRcv(ctx context.Context, stream gnmi.GNMI_SubscribeClient, subscriptionName string, subConfig *types.SubscriptionConfig) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		t.subscribeResponses <- &SubscribeResponse{
			SubscriptionName:   subscriptionName,
			SubscriptionConfig: subConfig,
			Response:           response,
		}
	}
}

func (t *Target) handleONCESubscriptionRcv(ctx context.Context, stream gnmi.GNMI_SubscribeClient, subscriptionName string, subConfig *types.SubscriptionConfig) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		t.subscribeResponses <- &SubscribeResponse{
			SubscriptionName:   subscriptionName,
			SubscriptionConfig: subConfig,
			Response:           response,
		}
		switch response.Response.(type) {
		case *gnmi.SubscribeResponse_SyncResponse:
			return nil
		}
	}
}

func (t *Target) handlePollSubscriptionRcv(ctx context.Context, stream gnmi.GNMI_SubscribeClient, subscriptionName string, subConfig *types.SubscriptionConfig) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			response, err := stream.Recv()
			if err != nil {
				return err
			}
			t.subscribeResponses <- &SubscribeResponse{
				SubscriptionName:   subscriptionName,
				SubscriptionConfig: subConfig,
				Response:           response,
			}
		}
	}
}
