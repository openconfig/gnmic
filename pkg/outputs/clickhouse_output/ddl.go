// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"fmt"
	"regexp"
	"strings"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validateIdent(name string) error {
	if name == "" {
		return fmt.Errorf("empty identifier")
	}
	if !identRe.MatchString(name) {
		return fmt.Errorf("invalid identifier %q (allowed: letters, digits, underscore)", name)
	}
	return nil
}

func validateSQLExpr(label, expr string) error {
	if strings.TrimSpace(expr) == "" {
		return nil
	}
	if strings.ContainsAny(expr, "`;\n\r") {
		return fmt.Errorf("%s contains forbidden characters", label)
	}
	return nil
}

func buildCreateTableSQL(cfg *config) (string, error) {
	if err := validateIdent(cfg.Database); err != nil {
		return "", fmt.Errorf("database: %w", err)
	}
	if err := validateIdent(cfg.Table); err != nil {
		return "", fmt.Errorf("table: %w", err)
	}
	if err := validateIdent(cfg.TableEngine); err != nil {
		return "", fmt.Errorf("table-engine: %w", err)
	}
	for _, col := range cfg.OrderBy {
		if err := validateIdent(col); err != nil {
			return "", fmt.Errorf("order-by column %q: %w", col, err)
		}
	}

	if err := validateSQLExpr("partition-by", cfg.PartitionBy); err != nil {
		return "", err
	}
	ttlExpr := expandTelemetryTTL(cfg.TTL)
	if err := validateSQLExpr("ttl", ttlExpr); err != nil {
		return "", err
	}

	fullName := quoteIdent(cfg.Database) + "." + quoteIdent(cfg.Table)
	orderExpr := strings.Join(quoteIdents(cfg.OrderBy), ", ")

	var sb strings.Builder
	sb.WriteString("CREATE TABLE IF NOT EXISTS ")
	sb.WriteString(fullName)
	sb.WriteString(` (
    timestamp     DateTime64(9, 'UTC') CODEC(Delta, ZSTD),
    ingest_ts     DateTime64(9, 'UTC') DEFAULT now64(9) CODEC(Delta, ZSTD),

    target        LowCardinality(String),
    source        LowCardinality(String),
    subscription  LowCardinality(String),
    name          LowCardinality(String),

    path          String CODEC(ZSTD(3)),
    tags          Map(LowCardinality(String), String),

    value_type    LowCardinality(String),
    value_int     Nullable(Int64)   CODEC(DoubleDelta, ZSTD),
    value_uint    Nullable(UInt64)  CODEC(DoubleDelta, ZSTD),
    value_float   Nullable(Float64) CODEC(Gorilla, ZSTD),
    value_bool    Nullable(Bool),
    value_string  String CODEC(ZSTD(3)),

    is_delete     Bool DEFAULT false,

    INDEX idx_path     path          TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_tag_keys mapKeys(tags) TYPE bloom_filter(0.01) GRANULARITY 4
)
ENGINE = `)
	sb.WriteString(cfg.TableEngine)
	sb.WriteString("\nPARTITION BY ")
	sb.WriteString(cfg.PartitionBy)
	sb.WriteString("\nORDER BY (")
	sb.WriteString(orderExpr)
	sb.WriteString(")")
	if strings.TrimSpace(ttlExpr) != "" {
		sb.WriteString("\nTTL ")
		sb.WriteString(strings.TrimSpace(ttlExpr))
	}
	return sb.String(), nil
}

// expandTelemetryTTL turns shorthand like "30 DAY" into a valid MergeTree TTL.
// The table column `timestamp` is DateTime64; TTL must be DateTime or Date, so we use toDateTime(timestamp).
// Full expressions (e.g. containing '+', '(', or 'timestamp') are passed through unchanged.
func expandTelemetryTTL(ttl string) string {
	ttl = strings.TrimSpace(ttl)
	if ttl == "" {
		return ""
	}
	lower := strings.ToLower(ttl)
	if strings.Contains(lower, "timestamp") ||
		strings.Contains(ttl, "(") ||
		strings.Contains(ttl, "+") ||
		strings.Contains(lower, "tointerval") {
		return ttl
	}
	re := regexp.MustCompile(`(?i)^(\d+)\s+(day|days|week|weeks|month|months|year|years|hour|hours|minute|minutes|second|seconds)$`)
	m := re.FindStringSubmatch(ttl)
	if m == nil {
		return ttl
	}
	n := m[1]
	u := strings.ToUpper(m[2])
	var unit string
	switch {
	case strings.HasPrefix(u, "DAY"):
		unit = "DAY"
	case strings.HasPrefix(u, "WEEK"):
		unit = "WEEK"
	case strings.HasPrefix(u, "MONTH"):
		unit = "MONTH"
	case strings.HasPrefix(u, "YEAR"):
		unit = "YEAR"
	case strings.HasPrefix(u, "HOUR"):
		unit = "HOUR"
	case strings.HasPrefix(u, "MINUTE"):
		unit = "MINUTE"
	case strings.HasPrefix(u, "SECOND"):
		unit = "SECOND"
	default:
		return ttl
	}
	// MergeTree TTL must evaluate to Date or DateTime; timestamp is DateTime64.
	return "toDateTime(timestamp) + INTERVAL " + n + " " + unit
}

func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func quoteIdents(cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = quoteIdent(c)
	}
	return out
}
