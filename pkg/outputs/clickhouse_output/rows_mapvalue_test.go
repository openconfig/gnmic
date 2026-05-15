// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"encoding/json"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stretchr/testify/require"
)

func TestMapValue_exhaustivePrimitiveTypes(t *testing.T) {
	cases := []struct {
		raw      any
		wantType string
	}{
		{int(1), "int"},
		{int8(2), "int"},
		{int16(3), "int"},
		{int32(4), "int"},
		{int64(5), "int"},
		{uint(6), "uint"},
		{uint8(7), "uint"},
		{uint16(8), "uint"},
		{uint32(9), "uint"},
		{uint64(10), "uint"},
		{float32(1.5), "float"},
		{float64(2.5), "float"},
		{json.Number("42"), "int"},
		{json.Number("18446744073709551615"), "uint"},
		{json.Number("3.125"), "float"},
		{&gnmi.Decimal64{Digits: 1, Precision: 1}, "float"},
	}
	for _, tc := range cases {
		var r telemetryRow
		mapValue(tc.raw, &r)
		require.Equal(t, tc.wantType, r.valueType, "type for %#v", tc.raw)
	}
}
