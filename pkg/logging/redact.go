// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"encoding/json"
	"log/slog"
	"strings"
)

// RedactedPlaceholder is the value substituted for any non-empty secret
// when a redacted view of a config is produced.
const RedactedPlaceholder = "***"

// Redact returns "***" when s is non-empty, and "" otherwise. Centralising
// this lets us change the placeholder in one place and avoids accidental
// "***" leakage when a secret was never set.
func Redact(s string) string {
	if s == "" {
		return ""
	}
	return RedactedPlaceholder
}

// DefaultSecretFields lists JSON keys that are treated as secrets by
// RedactedJSON when no explicit list is provided. Matching is
// case-insensitive against the lowercase form of the key.
var DefaultSecretFields = []string{
	"password",
	"token",
	"auth-token",
	"client-secret",
	"clientsecret",
	"bearer",
	"secret",
	"api-key",
	"apikey",
	"credentials",
}

// RedactedValue returns v's JSON representation with configured secret fields
// redacted. Types that can contain credentials can use this from LogValue to
// make slog.Any("config", v) safe by default.
func RedactedValue(v any, fields ...string) slog.Value {
	return slog.StringValue(RedactedJSON(v, fields...))
}

// RedactedJSON marshals v to JSON and replaces the values of any string
// fields whose key (case-insensitive) appears in fields with "***". If
// fields is empty, DefaultSecretFields is used. Non-string values keyed
// by a secret name are left untouched (this avoids surprising shape
// changes for typed numeric or bool fields). On error, an empty string
// is returned so callers can use it directly in log lines.
func RedactedJSON(v any, fields ...string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	if len(fields) == 0 {
		fields = DefaultSecretFields
	}
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		set[strings.ToLower(f)] = struct{}{}
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return string(b)
	}
	redactWalk(decoded, set)
	out, err := json.Marshal(decoded)
	if err != nil {
		return string(b)
	}
	return string(out)
}

func redactWalk(v any, secrets map[string]struct{}) {
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			if _, ok := secrets[strings.ToLower(k)]; ok {
				if s, isStr := vv.(string); isStr && s != "" {
					t[k] = RedactedPlaceholder
					continue
				}
			}
			redactWalk(vv, secrets)
		}
	case []any:
		for _, vv := range t {
			redactWalk(vv, secrets)
		}
	}
}
