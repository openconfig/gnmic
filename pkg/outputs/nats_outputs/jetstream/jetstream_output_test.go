package jetstream_output

import (
	"strings"
	"testing"

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
			name: "mutual exclusivity - both use-existing-stream and create-stream",
			cfg: &config{
				Stream:            "test-stream",
				UseExistingStream: true,
				CreateStream: &createStreamConfig{
					Subjects: []string{"test.>"},
				},
			},
			wantErr: true,
			errMsg:  "use-existing-stream and create-stream are mutually exclusive",
		},
		{
			name: "valid use-existing-stream",
			cfg: &config{
				Stream:            "test-stream",
				UseExistingStream: true,
			},
			wantErr: false,
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
			n := &jetstreamOutput{
				Cfg: tt.cfg,
			}
			err := n.setDefaults()
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
				if n.Cfg.CreateStream != nil {
					if n.Cfg.CreateStream.Retention == "" {
						t.Errorf("setDefaults() did not set default retention policy")
					}
					if n.Cfg.CreateStream.Retention != "" && n.Cfg.CreateStream.Retention != "limits" && n.Cfg.CreateStream.Retention != "workqueue" && n.Cfg.CreateStream.Retention != "LIMITS" && n.Cfg.CreateStream.Retention != "WORKQUEUE" {
						t.Errorf("setDefaults() set invalid retention policy: %s", n.Cfg.CreateStream.Retention)
					}
				}
			}
		})
	}
}
