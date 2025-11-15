// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package store

import "testing"

func Test_memStore_Set(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		kind    string
		key     string
		value   any
		want    bool
		wantErr bool
	}{
		{
			name:  "set",
			kind:  "kind",
			key:   "k1",
			value: "v1",
			want:  true,
		},
		{
			name:  "set",
			kind:  "kind",
			key:   "k2",
			value: "v2",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMemStore(StoreOptions[any]{})
			created, gotErr := ms.Set(tt.kind, tt.key, tt.value)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Set() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Set() succeeded unexpectedly")
			}
			if tt.want != created {
				t.Errorf("Set() = %v, want %v", created, tt.want)
			}
			got, ok, err := ms.Get(tt.kind, tt.key)
			if !ok {
				t.Errorf("Get() failed: %v", ok)
			}
			t.Logf("got: %T", got)
			if got != tt.value {
				t.Errorf("Get() = %v, want %v", got, tt.value)
			}
			if err != nil {
				t.Errorf("Get() failed: %v", err)
			}
		})
	}
}
