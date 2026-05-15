// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package clickhouse_output

import (
	"context"
	"errors"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	chd "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// fakeConn is a minimal chd.Conn for unit tests (PrepareBatch / Ping / Exec / Close).
type fakeConn struct {
	pingErr   error
	prepErr   error
	execErr   error
	appendErr error
	sendErr   error
	execCalls int
	closed    int
}

func (f *fakeConn) Contributors() []string { return nil }

func (f *fakeConn) ServerVersion() (*chd.ServerVersion, error) { return nil, nil }

func (f *fakeConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (f *fakeConn) Query(ctx context.Context, query string, args ...any) (chd.Rows, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeConn) QueryRow(ctx context.Context, query string, args ...any) chd.Row {
	return nil
}

func (f *fakeConn) PrepareBatch(ctx context.Context, query string, opts ...chd.PrepareBatchOption) (chd.Batch, error) {
	if f.prepErr != nil {
		return nil, f.prepErr
	}
	return &fakeBatch{appendErr: f.appendErr, sendErr: f.sendErr}, nil
}

func (f *fakeConn) Exec(ctx context.Context, query string, args ...any) error {
	f.execCalls++
	if f.execErr != nil {
		return f.execErr
	}
	return nil
}

func (f *fakeConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}

func (f *fakeConn) Ping(ctx context.Context) error { return f.pingErr }

func (f *fakeConn) Stats() chd.Stats { return chd.Stats{} }

func (f *fakeConn) Close() error {
	f.closed++
	return nil
}

type fakeBatch struct {
	appendErr error
	sendErr   error
	sent      bool
}

func (b *fakeBatch) Abort() error { return nil }

func (b *fakeBatch) Append(v ...any) error { return b.appendErr }

func (b *fakeBatch) AppendStruct(v any) error { return nil }

func (b *fakeBatch) Column(int) chd.BatchColumn { return &fakeBatchColumn{} }

func (b *fakeBatch) Flush() error { return nil }

func (b *fakeBatch) Send() error {
	if b.sendErr != nil {
		return b.sendErr
	}
	b.sent = true
	return nil
}

func (b *fakeBatch) IsSent() bool { return b.sent }

func (b *fakeBatch) Rows() int { return 0 }

func (b *fakeBatch) Columns() []column.Interface { return nil }

func (b *fakeBatch) Close() error { return nil }

type fakeBatchColumn struct{}

func (f *fakeBatchColumn) Append(any) error { return nil }

func (f *fakeBatchColumn) AppendRow(any) error { return nil }
