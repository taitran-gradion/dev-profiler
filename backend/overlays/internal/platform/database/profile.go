//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package database

import (
	"context"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// PoolConfig gives every pool the profiler's tracer. Outside a profiled
// request the tracer does nothing but a context lookup.
//
//margince:decorate
func PoolConfig(dsn string) (*pgxpool.Config, error) {
	cfg, err := PoolConfig__core(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.Tracer = profileTracer{}
	return cfg, nil
}

type profileTracer struct{}

type profileQueryKey struct{}

type profileQueryStart struct {
	sql  string
	args []string
	atMs float64
}

// profileArgs prints each argument as a Postgres literal, so a statement with
// its arguments put in place runs as it is in psql.
func profileArgs(args []any) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, profileArg(a))
	}
	return out
}

// profileArg dereferences a pointer first, so a nil one prints NULL instead
// of calling a method on nil, as pgx itself would send it.
func profileArg(a any) (printed string) {
	// A value whose own String panics prints as that panic rather than
	// failing the request it was only being shown for.
	defer func() {
		if r := recover(); r != nil {
			printed = fmt.Sprintf("/* unprintable: %v */ NULL", r)
		}
	}()
	v := reflect.ValueOf(a)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "NULL"
		}
		a, v = v.Elem().Interface(), v.Elem()
	}
	switch a := a.(type) {
	case nil:
		return "NULL"
	case []byte:
		if utf8.Valid(a) {
			return sqlString(string(a))
		}
		return `'\x` + hex.EncodeToString(a) + "'"
	case string:
		return sqlString(a)
	case time.Time:
		return sqlString(a.Format(time.RFC3339Nano))
	case bool:
		return strconv.FormatBool(a)
	case fmt.Stringer:
		return sqlString(a.String())
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		if v.Len() == 0 {
			return "'{}'"
		}
		items := make([]string, v.Len())
		for i := range items {
			items[i] = profileArg(v.Index(i).Interface())
		}
		return "ARRAY[" + strings.Join(items, ", ") + "]"
	}
	return fmt.Sprint(a)
}

// sqlString quotes s as a Postgres string literal.
func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (profileTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	at, ok := principal.ProfileQueryStart(ctx)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, profileQueryKey{}, profileQueryStart{sql: data.SQL, args: profileArgs(data.Args), atMs: at})
}

func (profileTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(profileQueryKey{}).(profileQueryStart)
	if !ok {
		return
	}
	principal.ProfileQueryEnd(ctx, start.sql, start.args, start.atMs, data.CommandTag.RowsAffected(), data.Err)
}
