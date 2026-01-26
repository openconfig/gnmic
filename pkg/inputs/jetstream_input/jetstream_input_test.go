package jetstream_input

import (
	"io"
	"log"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
)

func Test_setDefaults(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config
		wantErr bool
		errMsg  string
		check   func(*testing.T, *config)
	}{
		{
			name: "format defaults to event",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.Format != defaultFormat {
					t.Errorf("setDefaults() Format = %v, want %v", cfg.Format, defaultFormat)
				}
			},
		},
		{
			name: "deliver policy defaults to all",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.DeliverPolicy != deliverPolicyAll {
					t.Errorf("setDefaults() DeliverPolicy = %v, want %v", cfg.DeliverPolicy, deliverPolicyAll)
				}
			},
		},
		{
			name: "subject format defaults to static",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.SubjectFormat != subjectFormat_Static {
					t.Errorf("setDefaults() SubjectFormat = %v, want %v", cfg.SubjectFormat, subjectFormat_Static)
				}
			},
		},
		{
			name: "address defaults to localhost:4222",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.Address != defaultAddress {
					t.Errorf("setDefaults() Address = %v, want %v", cfg.Address, defaultAddress)
				}
			},
		},
		{
			name: "num workers defaults to 1",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.NumWorkers != defaultNumWorkers {
					t.Errorf("setDefaults() NumWorkers = %v, want %v", cfg.NumWorkers, defaultNumWorkers)
				}
			},
		},
		{
			name: "buffer size defaults to 500",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.BufferSize != defaultBufferSize {
					t.Errorf("setDefaults() BufferSize = %v, want %v", cfg.BufferSize, defaultBufferSize)
				}
			},
		},
		{
			name: "fetch batch size defaults to 500",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.FetchBatchSize != defaultFetchBatchSize {
					t.Errorf("setDefaults() FetchBatchSize = %v, want %v", cfg.FetchBatchSize, defaultFetchBatchSize)
				}
			},
		},
		{
			name: "max ack pending defaults to 1000",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.MaxAckPending == nil || *cfg.MaxAckPending != defaultMaxAckPending {
					t.Errorf("setDefaults() MaxAckPending = %v, want %v", cfg.MaxAckPending, defaultMaxAckPending)
				}
			},
		},
		{
			name: "invalid format event",
			cfg: &config{
				Stream: "test-stream",
				Format: "invalid",
			},
			wantErr: true,
			errMsg:  "unsupported input format",
		},
		{
			name: "valid format event",
			cfg: &config{
				Stream: "test-stream",
				Format: "event",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.Format != "event" {
					t.Errorf("setDefaults() Format = %v, want event", cfg.Format)
				}
			},
		},
		{
			name: "valid format proto",
			cfg: &config{
				Stream: "test-stream",
				Format: "proto",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.Format != "proto" {
					t.Errorf("setDefaults() Format = %v, want proto", cfg.Format)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &jetstreamInput{
				logger: log.New(io.Discard, loggingPrefix, 0),
			}
			err := n.setDefaultsFor(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("setDefaultsFor() expected error but got nil")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("setDefaultsFor() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("setDefaultsFor() unexpected error = %v", err)
					return
				}
				if tt.check != nil {
					tt.check(t, tt.cfg)
				}
			}
		})
	}
}

func Test_toJSDeliverPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy deliverPolicy
		want   jetstream.DeliverPolicy
	}{
		{
			name:   "deliver policy all",
			policy: deliverPolicyAll,
			want:   jetstream.DeliverAllPolicy,
		},
		{
			name:   "deliver policy last",
			policy: deliverPolicyLast,
			want:   jetstream.DeliverLastPolicy,
		},
		{
			name:   "deliver policy new",
			policy: deliverPolicyNew,
			want:   jetstream.DeliverNewPolicy,
		},
		{
			name:   "deliver policy last-per-subject",
			policy: deliverPolicyLastPerSubject,
			want:   jetstream.DeliverLastPerSubjectPolicy,
		},
		{
			name:   "invalid deliver policy returns zero",
			policy: "invalid",
			want:   0,
		},
		{
			name:   "empty deliver policy returns zero",
			policy: "",
			want:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSDeliverPolicy(tt.policy)
			if got != tt.want {
				t.Errorf("toJSDeliverPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_workqueueDeliverPolicy documents the expected behavior for workqueue streams
// When stream retention is WorkQueuePolicy:
// - AckPolicy is always set to AckExplicitPolicy
// - DeliverPolicy can be DeliverAllPolicy (process all queued jobs) or DeliverNewPolicy (process only new jobs)
// - Other deliver policies are converted to DeliverAllPolicy for compatibility
func Test_workqueueDeliverPolicy(t *testing.T) {
	// This is a documentation test - actual behavior is tested in integration tests
	// The workerStart function should:
	// 1. Detect stream retention policy
	// 2. Force AckExplicitPolicy for workqueue streams
	// 3. Allow DeliverAllPolicy or DeliverNewPolicy
	// 4. Convert other policies to DeliverAllPolicy
	t.Log("Workqueue streams support DeliverAllPolicy and DeliverNewPolicy")
}
