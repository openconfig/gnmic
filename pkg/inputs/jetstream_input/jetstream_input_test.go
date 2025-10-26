package jetstream_input

import (
	"io"
	"log"
	"testing"
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
			name: "consumer mode defaults to single",
			cfg: &config{
				Stream: "test-stream",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.ConsumerMode != consumerModeSingle {
					t.Errorf("setDefaults() ConsumerMode = %v, want %v", cfg.ConsumerMode, consumerModeSingle)
				}
			},
		},
		{
			name: "consumer mode single is valid",
			cfg: &config{
				Stream:       "test-stream",
				ConsumerMode: consumerModeSingle,
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.ConsumerMode != consumerModeSingle {
					t.Errorf("setDefaults() ConsumerMode = %v, want %v", cfg.ConsumerMode, consumerModeSingle)
				}
			},
		},
		{
			name: "consumer mode multi is valid with filter-subjects",
			cfg: &config{
				Stream:         "test-stream",
				ConsumerMode:   consumerModeMulti,
				FilterSubjects: []string{"test.>"},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.ConsumerMode != consumerModeMulti {
					t.Errorf("setDefaults() ConsumerMode = %v, want %v", cfg.ConsumerMode, consumerModeMulti)
				}
			},
		},
		{
			name: "consumer mode multi requires filter-subjects",
			cfg: &config{
				Stream:       "test-stream",
				ConsumerMode: consumerModeMulti,
			},
			wantErr: true,
			errMsg:  "consumer-mode 'multi' requires filter-subjects to be specified",
		},
		{
			name: "invalid consumer mode",
			cfg: &config{
				Stream:       "test-stream",
				ConsumerMode: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid consumer-mode: invalid (must be 'single' or 'multi')",
		},
		{
			name: "consumer mode empty defaults to single",
			cfg: &config{
				Stream:       "test-stream",
				ConsumerMode: "",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.ConsumerMode != consumerModeSingle {
					t.Errorf("setDefaults() ConsumerMode = %v, want %v", cfg.ConsumerMode, consumerModeSingle)
				}
			},
		},
		{
			name: "consumer mode multi with empty filter-subjects fails",
			cfg: &config{
				Stream:         "test-stream",
				ConsumerMode:   consumerModeMulti,
				FilterSubjects: []string{},
			},
			wantErr: true,
			errMsg:  "consumer-mode 'multi' requires filter-subjects to be specified",
		},
		{
			name: "consumer mode multi with nil filter-subjects fails",
			cfg: &config{
				Stream:         "test-stream",
				ConsumerMode:   consumerModeMulti,
				FilterSubjects: nil,
			},
			wantErr: true,
			errMsg:  "consumer-mode 'multi' requires filter-subjects to be specified",
		},
		{
			name: "consumer mode single does not require filter-subjects",
			cfg: &config{
				Stream:       "test-stream",
				ConsumerMode: consumerModeSingle,
				Subjects:     []string{"test.subject"},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.ConsumerMode != consumerModeSingle {
					t.Errorf("setDefaults() ConsumerMode = %v, want %v", cfg.ConsumerMode, consumerModeSingle)
				}
			},
		},
		{
			name: "consumer mode multi with multiple filter-subjects",
			cfg: &config{
				Stream:         "test-stream",
				ConsumerMode:   consumerModeMulti,
				FilterSubjects: []string{"test.1", "test.2", "test.3"},
			},
			wantErr: false,
			check: func(t *testing.T, cfg *config) {
				if cfg.ConsumerMode != consumerModeMulti {
					t.Errorf("setDefaults() ConsumerMode = %v, want %v", cfg.ConsumerMode, consumerModeMulti)
				}
				if len(cfg.FilterSubjects) != 3 {
					t.Errorf("setDefaults() FilterSubjects length = %v, want 3", len(cfg.FilterSubjects))
				}
			},
		},
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
				Cfg:    tt.cfg,
				logger: log.New(io.Discard, loggingPrefix, 0),
			}
			err := n.setDefaults()
			if tt.wantErr {
				if err == nil {
					t.Errorf("setDefaults() expected error but got nil")
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("setDefaults() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("setDefaults() unexpected error = %v", err)
					return
				}
				if tt.check != nil {
					tt.check(t, n.Cfg)
				}
			}
		})
	}
}
