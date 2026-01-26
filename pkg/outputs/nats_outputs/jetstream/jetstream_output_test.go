package jetstream_output

import (
	"strings"
	"testing"

	"sync/atomic"

	"github.com/nats-io/nats.go"
)

func Test_isValidRetentionPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy string
		want   bool
	}{
		{
			name:   "valid limits policy",
			policy: "limits",
			want:   true,
		},
		{
			name:   "valid workqueue policy",
			policy: "workqueue",
			want:   true,
		},
		{
			name:   "valid limits policy uppercase",
			policy: "LIMITS",
			want:   true,
		},
		{
			name:   "valid workqueue policy uppercase",
			policy: "WORKQUEUE",
			want:   true,
		},
		{
			name:   "valid limits policy mixed case",
			policy: "Limits",
			want:   true,
		},
		{
			name:   "invalid empty policy",
			policy: "",
			want:   false,
		},
		{
			name:   "invalid interest policy",
			policy: "interest",
			want:   false,
		},
		{
			name:   "invalid random string",
			policy: "invalid",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidRetentionPolicy(tt.policy); got != tt.want {
				t.Errorf("isValidRetentionPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_retentionPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy string
		want   nats.RetentionPolicy
	}{
		{
			name:   "workqueue policy lowercase",
			policy: "workqueue",
			want:   nats.WorkQueuePolicy,
		},
		{
			name:   "workqueue policy uppercase",
			policy: "WORKQUEUE",
			want:   nats.WorkQueuePolicy,
		},
		{
			name:   "workqueue policy mixed case",
			policy: "WorkQueue",
			want:   nats.WorkQueuePolicy,
		},
		{
			name:   "limits policy lowercase",
			policy: "limits",
			want:   nats.LimitsPolicy,
		},
		{
			name:   "limits policy uppercase",
			policy: "LIMITS",
			want:   nats.LimitsPolicy,
		},
		{
			name:   "limits policy mixed case",
			policy: "Limits",
			want:   nats.LimitsPolicy,
		},
		{
			name:   "empty string defaults to limits",
			policy: "",
			want:   nats.LimitsPolicy,
		},
		{
			name:   "invalid policy defaults to limits",
			policy: "invalid",
			want:   nats.LimitsPolicy,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retentionPolicy(tt.policy); got != tt.want {
				t.Errorf("retentionPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setDefaults(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing stream name",
			cfg: &config{
				Stream: "",
			},
			wantErr: true,
			errMsg:  "missing stream name",
		},
		{
			name: "valid create-stream with limits retention",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects:  []string{"test.>"},
					Retention: "limits",
				},
			},
			wantErr: false,
		},
		{
			name: "valid create-stream with workqueue retention",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects:  []string{"test.>"},
					Retention: "workqueue",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid retention policy",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects:  []string{"test.>"},
					Retention: "interest",
				},
			},
			wantErr: true,
			errMsg:  "invalid retention-policy: interest",
		},
		{
			name: "create-stream with empty retention defaults to limits",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects: []string{"test.>"},
				},
			},
			wantErr: false,
		},
		{
			name: "create-stream with uppercase WORKQUEUE retention",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects:  []string{"test.>"},
					Retention: "WORKQUEUE",
				},
			},
			wantErr: false,
		},
		{
			name: "create-stream with uppercase LIMITS retention",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects:  []string{"test.>"},
					Retention: "LIMITS",
				},
			},
			wantErr: false,
		},
		{
			name: "create-stream with invalid retention",
			cfg: &config{
				Stream: "test-stream",
				CreateStream: &createStreamConfig{
					Subjects:  []string{"test.>"},
					Retention: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid retention-policy: invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := new(atomic.Pointer[config])
			cfg.Store(tt.cfg)
			n := &jetstreamOutput{
				cfg: cfg,
			}
			err := n.setDefaultsFor(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("setDefaults() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("setDefaults() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("setDefaults() unexpected error = %v", err)
					return
				}
				// Verify defaults were set correctly
				rcfg := cfg.Load()
				if rcfg.CreateStream != nil {
					if rcfg.CreateStream.Retention == "" {
						t.Errorf("setDefaults() did not set default retention policy")
					}
					if rcfg.CreateStream.Retention != "" && rcfg.CreateStream.Retention != "limits" && rcfg.CreateStream.Retention != "workqueue" && rcfg.CreateStream.Retention != "LIMITS" && rcfg.CreateStream.Retention != "WORKQUEUE" {
						t.Errorf("setDefaults() set invalid retention policy: %s", rcfg.CreateStream.Retention)
					}
				}
			}
		})
	}
}
