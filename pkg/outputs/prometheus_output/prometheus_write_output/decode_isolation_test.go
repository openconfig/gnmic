// © 2026 Supranett
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/outputs"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	"github.com/prometheus/prometheus/prompb"
)

func TestWorkerPreservesDecodedMetrics(t *testing.T) {
	p := &promWriteOutput{}
	p.init()
	p.cfg.Store(&config{Name: t.Name(), BufferSize: 4})
	p.dynCfg.Store(&dynConfig{mb: &promcom.MetricBuilder{}})
	queue := make(chan *prompb.TimeSeries, 4)
	p.timeSeriesCh.Store(&queue)
	p.workerHandleProto(t.Context(), outputs.NewProtoMsg(&gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{Update: []*gnmi.Update{
			{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "valid"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 42}}},
			{Path: &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "invalid"}}}, Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte("{")}}},
		}}},
	}, nil))
	if len(queue) != 1 || (<-queue).Samples[0].Value != 42 {
		t.Fatal("worker discarded the healthy metric alongside a decode error")
	}
}
