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
	"github.com/openconfig/gnmic/pkg/api/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Subscribe sends a gnmi.SubscribeRequest to the target *t, responses and error are sent to the target channels
func (t *Target) Subscribe(ctx context.Context, req *gnmi.SubscribeRequest, subscriptionName string) {
	var subscribeClient gnmi.GNMI_SubscribeClient
	var nctx context.Context
	var cancel context.CancelFunc
	var err error
	goto SUBSC_NODELAY
SUBSC:
	{
		retry := time.NewTimer(t.Config.RetryTimer)
		select {
		case <-ctx.Done():
			retry.Stop()
			return
		case <-retry.C:
		}
	}
SUBSC_NODELAY:
	select {
	case <-ctx.Done():
		return
	default:
		nctx, cancel = context.WithCancel(ctx)
		nctx = t.appendRequestMetadata(nctx)
		subscribeClient, err = t.Client.Subscribe(nctx, t.callOpts()...)
		if err != nil {
			t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              fmt.Errorf("failed to create a subscribe client, target='%s', retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
			}
			cancel()
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
		select {
		case t.errors <- &TargetError{
			SubscriptionName: subscriptionName,
			Err:              fmt.Errorf("target '%s' send error, retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
		}:
		case <-ctx.Done():
			cancel()
			return
		}
		cancel()
		goto SUBSC
	}

	switch req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_STREAM:
		err = t.handleStreamSubscriptionRcv(nctx, subscribeClient, subscriptionName, subConfig, t.subscribeResponses)
		if err != nil {
			select {
			case t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              err,
			}:
			case <-ctx.Done():
				cancel()
				return
			}
			select {
			case t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              fmt.Errorf("retrying in %s", t.Config.RetryTimer),
			}:
			case <-ctx.Done():
				cancel()
				return
			}
			cancel()
			goto SUBSC
		}
	case gnmi.SubscriptionList_ONCE:
		err = t.handleONCESubscriptionRcv(nctx, subscribeClient, subscriptionName, subConfig, t.subscribeResponses)
		if err != nil {
			select {
			case t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              err,
			}:
			case <-ctx.Done():
				cancel()
				return
			}
			if errors.Is(err, io.EOF) {
				cancel()
				return
			}
			select {
			case t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              fmt.Errorf("retrying in %d", t.Config.RetryTimer),
			}:
			case <-ctx.Done():
				cancel()
				return
			}
			cancel()
			goto SUBSC
		}
		cancel()
		return
	case gnmi.SubscriptionList_POLL:
		go t.listenPolls(nctx)
		err = t.handlePollSubscriptionRcv(nctx, subscribeClient, subscriptionName, subConfig, t.subscribeResponses)
		if err != nil {
			select {
			case t.errors <- &TargetError{
				SubscriptionName: subscriptionName,
				Err:              err,
			}:
			case <-ctx.Done():
				cancel()
				return
			}
			cancel()
			goto SUBSC
		}
	}
	cancel()
}

func (t *Target) SubscribeChan(ctx context.Context, req *gnmi.SubscribeRequest, subscriptionName string) (chan *SubscribeResponse, chan *TargetError) {
	responseCh := make(chan *SubscribeResponse, 1)
	errCh := make(chan *TargetError, 1)

	go func() {
		defer close(responseCh)
		defer close(errCh)

		firstAttempt := true
		for {
			// retry delay, skipped the first attempt
			if !firstAttempt {
				timer := time.NewTimer(t.Config.RetryTimer)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
			firstAttempt = false

			// check if parent context is done
			if ctx.Err() != nil {
				return
			}

			// attempt subscription
			// return true if retry is needed
			shouldRetry := t.attemptSubscription(ctx, req, subscriptionName, responseCh, errCh)
			if !shouldRetry {
				return
			}
		}
	}()

	return responseCh, errCh
}

func (t *Target) attemptSubscription(ctx context.Context, req *gnmi.SubscribeRequest,
	subscriptionName string, responseCh chan *SubscribeResponse, errCh chan *TargetError) bool {
	// create child context for this attempt
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()

	nctx = t.appendRequestMetadata(nctx)

	// create subscribe client
	subscribeClient, err := t.Client.Subscribe(nctx, t.callOpts()...)
	if err != nil {
		// check if cancellation was intentional
		if isCancellationError(err) {
			return false
		}
		sendError(errCh, ctx, subscriptionName,
			fmt.Errorf("failed to create subscribe client, target='%s', retry in %s: %w",
				t.Config.Name, t.Config.RetryTimer, err))
		return true
	}

	// store subscription state and register cleanup
	t.m.Lock()
	if oldCancel, ok := t.subscribeCancelFn[subscriptionName]; ok {
		oldCancel() // cancel previous attempt
	}
	t.SubscribeClients[subscriptionName] = subscribeClient
	t.subscribeCancelFn[subscriptionName] = cancel
	subConfig := t.Subscriptions[subscriptionName]
	t.m.Unlock()

	// cleanup on exit (registered after state is stored)
	defer t.StopSubscription(subscriptionName)

	// send initial subscribe request
	err = subscribeClient.Send(req)
	if err != nil {
		sendError(errCh, ctx, subscriptionName,
			fmt.Errorf("target '%s' send error, retry in %s: %w",
				t.Config.Name, t.Config.RetryTimer, err))
		return true
	}

	// handle subscription based on mode
	switch req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_STREAM:
		return t.handleSTREAMMode(nctx, ctx, subscribeClient, subscriptionName, subConfig, responseCh, errCh)

	case gnmi.SubscriptionList_ONCE:
		return t.handleONCEMode(nctx, ctx, subscribeClient, subscriptionName, subConfig, responseCh, errCh)

	case gnmi.SubscriptionList_POLL:
		return t.handlePOLLMode(nctx, ctx, subscribeClient, subscriptionName, subConfig, responseCh, errCh)
	}

	return false
}

func (t *Target) handleSTREAMMode(nctx, ctx context.Context, client gnmi.GNMI_SubscribeClient,
	subscriptionName string, subConfig *types.SubscriptionConfig,
	responseCh chan *SubscribeResponse, errCh chan *TargetError) bool {

	err := t.handleStreamSubscriptionRcv(nctx, client, subscriptionName, subConfig, responseCh)
	if err != nil {
		if isCancellationError(err) {
			return false
		}

		sendError(errCh, ctx, subscriptionName, err)
		sendError(errCh, ctx, subscriptionName,
			fmt.Errorf("retrying in %s", t.Config.RetryTimer))
		return true
	}
	return false
}

func (t *Target) handleONCEMode(nctx, ctx context.Context, client gnmi.GNMI_SubscribeClient,
	subscriptionName string, subConfig *types.SubscriptionConfig,
	responseCh chan *SubscribeResponse, errCh chan *TargetError) bool {

	err := t.handleONCESubscriptionRcv(nctx, client, subscriptionName, subConfig, responseCh)
	if err != nil {
		if isCancellationError(err) {
			return false
		}

		sendError(errCh, ctx, subscriptionName, err)

		// ONCE mode doesn't retry on EOF
		if errors.Is(err, io.EOF) {
			return false
		}

		sendError(errCh, ctx, subscriptionName,
			fmt.Errorf("retrying in %s", t.Config.RetryTimer))
		return true
	}
	return false
}

func (t *Target) handlePOLLMode(nctx, ctx context.Context, client gnmi.GNMI_SubscribeClient,
	subscriptionName string, subConfig *types.SubscriptionConfig,
	responseCh chan *SubscribeResponse, errCh chan *TargetError) bool {

	// Start poll listener once per target (not per subscription attempt)
	// This prevents goroutine leaks on retry
	t.m.Lock()
	if t.pollChan == nil {
		t.pollChan = make(chan string, 10)
		go t.listenPolls(ctx) // Use parent context, not nctx
	}
	t.m.Unlock()

	err := t.handlePollSubscriptionRcv(nctx, client, subscriptionName, subConfig, responseCh)
	if err != nil {
		if isCancellationError(err) {
			return false
		}

		sendError(errCh, ctx, subscriptionName, err)
		sendError(errCh, ctx, subscriptionName,
			fmt.Errorf("retrying in %s", t.Config.RetryTimer))
		return true
	}
	return false
}

// check if error is due to intentional cancellation
func isCancellationError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Canceled
}

// send error to channel with context awareness
func sendError(errCh chan *TargetError, ctx context.Context, subscriptionName string, err error) bool {
	select {
	case errCh <- &TargetError{
		SubscriptionName: subscriptionName,
		Err:              err,
	}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (t *Target) SubscribeStreamChan(ctx context.Context, req *gnmi.SubscribeRequest, subscriptionName string) (chan *gnmi.SubscribeResponse, chan error) {
	responseCh := make(chan *gnmi.SubscribeResponse)
	errCh := make(chan error)

	go func() {
		if req.GetSubscribe().GetMode() != gnmi.SubscriptionList_STREAM {
			errCh <- fmt.Errorf("subscribe request does not define a STREAM subscription: %v", req.GetSubscribe().GetMode())
			close(errCh)
			close(responseCh)
			return
		}
		var subscribeClient gnmi.GNMI_SubscribeClient
		var nctx context.Context
		var cancel context.CancelFunc
		var err error
		goto SUBSC_NODELAY
	SUBSC:
		{
			retry := time.NewTimer(t.Config.RetryTimer)
			select {
			case <-ctx.Done():
				retry.Stop()
				return
			case <-retry.C:
			}
		}
	SUBSC_NODELAY:
		select {
		case <-ctx.Done():
			return
		default:
			nctx, cancel = context.WithCancel(ctx)
			defer cancel()
			nctx = t.appendRequestMetadata(nctx)
			subscribeClient, err = t.Client.Subscribe(nctx, t.callOpts()...)
			if err != nil {
				errCh <- fmt.Errorf("failed to create a subscribe client, target='%s', retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err)
				cancel()
				goto SUBSC
			}
		}
		t.m.Lock()
		if cfn, ok := t.subscribeCancelFn[subscriptionName]; ok {
			cfn()
		}
		t.SubscribeClients[subscriptionName] = subscribeClient
		t.subscribeCancelFn[subscriptionName] = cancel
		t.m.Unlock()

		err = subscribeClient.Send(req)
		if err != nil {
			errCh <- fmt.Errorf("target '%s' send error, retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err)
			cancel()
			goto SUBSC
		}

		for {
			if ctx.Err() != nil {
				errCh <- err
				cancel()
				goto SUBSC
			}
			response, err := subscribeClient.Recv()
			if err != nil {
				errCh <- err
				cancel()
				goto SUBSC
			}
			responseCh <- response
		}
	}()
	return responseCh, errCh
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
	t.m.Lock()
	defer t.m.Unlock()
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
	cfn, ok := t.subscribeCancelFn[name]
	if ok {
		cfn()
	}
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

func (t *Target) handleStreamSubscriptionRcv(ctx context.Context, stream gnmi.GNMI_SubscribeClient, subscriptionName string, subConfig *types.SubscriptionConfig, ch chan *SubscribeResponse) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		select {
		case ch <- &SubscribeResponse{
			SubscriptionName:   subscriptionName,
			SubscriptionConfig: subConfig,
			Response:           response,
		}:
		case <-ctx.Done():
			return nil
		}
	}
}

func (t *Target) handleONCESubscriptionRcv(ctx context.Context, stream gnmi.GNMI_SubscribeClient, subscriptionName string, subConfig *types.SubscriptionConfig, ch chan *SubscribeResponse) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		ch <- &SubscribeResponse{
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

func (t *Target) handlePollSubscriptionRcv(ctx context.Context, stream gnmi.GNMI_SubscribeClient, subscriptionName string, subConfig *types.SubscriptionConfig, ch chan *SubscribeResponse) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			response, err := stream.Recv()
			if err != nil {
				return err
			}
			ch <- &SubscribeResponse{
				SubscriptionName:   subscriptionName,
				SubscriptionConfig: subConfig,
				Response:           response,
			}
		}
	}
}
