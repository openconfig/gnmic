// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
)

// telemetryRow is one ClickHouse row (one path → value or delete).
type telemetryRow struct {
	timestamp    time.Time
	target       string
	source       string
	subscription string
	name         string
	path         string
	tags         map[string]string
	valueType    string
	valueInt     *int64
	valueUint    *uint64
	valueFloat   *float64
	valueBool    *bool
	valueString  string
	isDelete     bool
}

func eventToRows(ev *formatters.EventMsg, meta outputs.Meta, overrideTS bool) []telemetryRow {
	if ev == nil {
		return nil
	}
	ts := eventTimestamp(ev, overrideTS)
	out := make([]telemetryRow, 0, len(ev.Values)+len(ev.Deletes))

	for _, delPath := range ev.Deletes {
		out = append(out, telemetryRow{
			timestamp:    ts,
			target:       resolveTarget(ev, meta),
			source:       metaString(meta, ev, "source"),
			subscription: metaString(meta, ev, "subscription-name"),
			name:         ev.Name,
			path:         delPath,
			tags:         copyTags(ev.Tags),
			isDelete:     true,
		})
	}

	for path, raw := range ev.Values {
		r := telemetryRow{
			timestamp:    ts,
			target:       resolveTarget(ev, meta),
			source:       metaString(meta, ev, "source"),
			subscription: metaString(meta, ev, "subscription-name"),
			name:         ev.Name,
			path:         path,
			tags:         copyTags(ev.Tags),
		}
		mapValue(raw, &r)
		out = append(out, r)
	}

	return out
}

func eventTimestamp(ev *formatters.EventMsg, overrideTS bool) time.Time {
	if overrideTS {
		return time.Now().UTC()
	}
	if ev.Timestamp == 0 {
		return time.Now().UTC()
	}
	return time.Unix(0, ev.Timestamp).UTC()
}

func metaString(meta outputs.Meta, ev *formatters.EventMsg, key string) string {
	if meta != nil {
		if v := meta[key]; v != "" {
			return v
		}
	}
	if ev != nil && ev.Tags != nil {
		return ev.Tags[key]
	}
	return ""
}

func resolveTarget(ev *formatters.EventMsg, meta outputs.Meta) string {
	if ev != nil && ev.Tags != nil {
		if t := ev.Tags["target"]; t != "" {
			return t
		}
	}
	if meta != nil {
		if t := meta["subscription-target"]; t != "" {
			return t
		}
		if s := meta["source"]; s != "" {
			return utils.GetHost(s)
		}
	}
	if ev != nil && ev.Tags != nil {
		if t := ev.Tags["subscription-target"]; t != "" {
			return t
		}
		if s := ev.Tags["source"]; s != "" {
			return utils.GetHost(s)
		}
	}
	return ""
}

func copyTags(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func mapValue(raw any, r *telemetryRow) {
	switch v := raw.(type) {
	case int:
		i := int64(v)
		r.valueType = "int"
		r.valueInt = &i
	case int8:
		i := int64(v)
		r.valueType = "int"
		r.valueInt = &i
	case int16:
		i := int64(v)
		r.valueType = "int"
		r.valueInt = &i
	case int32:
		i := int64(v)
		r.valueType = "int"
		r.valueInt = &i
	case int64:
		r.valueType = "int"
		r.valueInt = &v
	case uint:
		u := uint64(v)
		r.valueType = "uint"
		r.valueUint = &u
	case uint8:
		u := uint64(v)
		r.valueType = "uint"
		r.valueUint = &u
	case uint16:
		u := uint64(v)
		r.valueType = "uint"
		r.valueUint = &u
	case uint32:
		u := uint64(v)
		r.valueType = "uint"
		r.valueUint = &u
	case uint64:
		r.valueType = "uint"
		r.valueUint = &v
	case float32:
		f := float64(v)
		r.valueType = "float"
		r.valueFloat = &f
	case float64:
		r.valueType = "float"
		r.valueFloat = &v
	case bool:
		r.valueType = "bool"
		r.valueBool = &v
	case string:
		r.valueType = "string"
		r.valueString = v
	case []byte:
		r.valueType = "bytes"
		r.valueString = base64.StdEncoding.EncodeToString(v)
	case json.Number:
		s := string(v)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			r.valueType = "int"
			r.valueInt = &i
			return
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			r.valueType = "uint"
			r.valueUint = &u
			return
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			r.valueType = "float"
			r.valueFloat = &f
			return
		}
		r.valueType = "string"
		r.valueString = s
	//lint:ignore SA1019 gnmi.Decimal64 still appears in decoded values
	case *gnmi.Decimal64:
		f := float64(v.Digits) / math.Pow10(int(v.Precision))
		r.valueType = "float"
		r.valueFloat = &f
	default:
		r.valueType = "json"
		b, err := json.Marshal(v)
		if err != nil {
			r.valueString = fmt.Sprintf("%v", v)
			return
		}
		r.valueString = string(b)
	}
}
