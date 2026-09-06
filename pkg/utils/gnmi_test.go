package utils

import (
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmic/pkg/api/types"
)

func TestValidateSubscriptionConfig(t *testing.T) {
	tests := map[string]struct {
		sc      *types.SubscriptionConfig
		wantErr bool
	}{
		"paths": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}},
		},
		"prefix only": {
			sc: &types.SubscriptionConfig{Name: "sub", Prefix: "/interfaces"},
		},
		"empty paths": {
			sc:      &types.SubscriptionConfig{Name: "sub", Paths: []string{}},
			wantErr: true,
		},
		"no paths": {
			sc:      &types.SubscriptionConfig{Name: "sub", StreamMode: "sample"},
			wantErr: true,
		},
		"paths and stream-subscriptions": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				Paths:               []string{"/"},
				StreamSubscriptions: []*types.SubscriptionConfig{{Paths: []string{"/"}}},
			},
			wantErr: true,
		},
		"known mode": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Mode: "once"},
		},
		"unknown mode": {
			sc:      &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Mode: "not-a-mode"},
			wantErr: true,
		},
		"once with stream-subscriptions": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				Mode:                "once",
				StreamSubscriptions: []*types.SubscriptionConfig{{Paths: []string{"/"}}},
			},
			wantErr: true,
		},
		"known stream-mode": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, StreamMode: "on-change"},
		},
		"unknown stream-mode": {
			sc:      &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, StreamMode: "not-a-mode"},
			wantErr: true,
		},
		"known encoding": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Encoding: pointer.ToString("json_ietf")},
		},
		"numeric encoding": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Encoding: pointer.ToString("2")},
		},
		"unknown encoding": {
			sc:      &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Encoding: pointer.ToString("not-an-encoding")},
			wantErr: true,
		},
		"stream-subscriptions": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				StreamSubscriptions: []*types.SubscriptionConfig{{Paths: []string{"/"}, StreamMode: "sample"}},
			},
		},
		"stream-subscription without paths": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				StreamSubscriptions: []*types.SubscriptionConfig{{StreamMode: "sample"}},
			},
			wantErr: true,
		},
		"stream-subscription with unknown stream-mode": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				StreamSubscriptions: []*types.SubscriptionConfig{{Paths: []string{"/"}, StreamMode: "not-a-mode"}},
			},
			wantErr: true,
		},
		"stream-subscription with mode": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				StreamSubscriptions: []*types.SubscriptionConfig{{Paths: []string{"/"}, Mode: "once"}},
			},
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			want := tc.sc.String()
			err := ValidateSubscriptionConfig(tc.sc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSubscriptionConfig() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got := tc.sc.String(); got != want {
				t.Errorf("ValidateSubscriptionConfig() modified the config:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestSetSubscriptionDefaults(t *testing.T) {
	tests := map[string]struct {
		sc   *types.SubscriptionConfig
		want *types.SubscriptionConfig
	}{
		"mode and stream-mode": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}},
			want: &types.SubscriptionConfig{
				Name:       "sub",
				Paths:      []string{"/"},
				Mode:       SubscriptionMode_STREAM,
				StreamMode: SubscriptionStreamMode_TARGET_DEFINED,
			},
		},
		"empty encoding": {
			sc: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Encoding: pointer.ToString("")},
			want: &types.SubscriptionConfig{
				Name:       "sub",
				Paths:      []string{"/"},
				Mode:       SubscriptionMode_STREAM,
				StreamMode: SubscriptionStreamMode_TARGET_DEFINED,
				Encoding:   pointer.ToString(subscriptionDefaultEncoding),
			},
		},
		"no stream-mode for non stream modes": {
			sc:   &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Mode: "once"},
			want: &types.SubscriptionConfig{Name: "sub", Paths: []string{"/"}, Mode: "once"},
		},
		"stream-subscriptions": {
			sc: &types.SubscriptionConfig{
				Name:                "sub",
				StreamSubscriptions: []*types.SubscriptionConfig{{Paths: []string{"/"}}},
			},
			want: &types.SubscriptionConfig{
				Name: "sub",
				Mode: SubscriptionMode_STREAM,
				StreamSubscriptions: []*types.SubscriptionConfig{
					{Paths: []string{"/"}, StreamMode: SubscriptionStreamMode_TARGET_DEFINED},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			setSubscriptionDefaults(tc.sc)
			if diff := cmp.Diff(tc.want, tc.sc); diff != "" {
				t.Errorf("setSubscriptionDefaults() diff (-want +got):\n%s", diff)
			}
		})
	}
}
