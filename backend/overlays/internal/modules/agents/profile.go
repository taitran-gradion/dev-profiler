//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package agents

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// InvokeServing is where every tool call lands, from MCP and from a REST
// route alike.
//
//margince:decorate
func (r *Registry) InvokeServing(ctx context.Context, name string, in json.RawMessage) (json.RawMessage, int, error) {
	at, ok := principal.ProfileSpanStart(ctx)
	out, records, err := r.InvokeServing__core(ctx, name, in)
	if ok {
		principal.ProfileSpanEnd(ctx, "tool", name, at, map[string]string{
			"args_bytes": strconv.Itoa(len(in)), "result_bytes": strconv.Itoa(len(out)), "records": strconv.Itoa(records),
		}, err)
	}
	return out, records, err
}
