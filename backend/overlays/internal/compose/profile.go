//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package compose

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/margince/margince/backend/internal/modules/agents"
	"github.com/margince/margince/backend/internal/platform/auth"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
	"github.com/margince/margince/backend/internal/shared/ports/datasource"
	"github.com/margince/margince/backend/pkg/extension"
)

// chiRoutePattern is asked once per contract call, after the match, so it is
// where the profiler learns the route and its operation.
//
//margince:decorate
func chiRoutePattern(r *http.Request) string {
	route := chiRoutePattern__core(r)
	if route != "" {
		principal.ProfileRoute(r.Context(), route, agentPolicies[r.Method+" "+route].Op)
	}
	return route
}

//margince:decorate
func runOperationHooks(r *http.Request, e extension.Extension, op *extension.Operation) error {
	return principal.ProfileHookRun(r.Context(), string(e.Name), "operation", op.ID, func() error {
		return runOperationHooks__core(r, e, op)
	})
}

//margince:decorate
func runWriteHook(ctx context.Context, tx pgx.Tx, e extension.Extension, hook extension.WriteHook, w extension.Write) error {
	return principal.ProfileHookRun(ctx, string(e.Name), "write", w.Entity+"."+w.Action, func() error {
		return runWriteHook__core(ctx, tx, e, hook, w)
	})
}

//margince:decorate
func operationHooks(next http.Handler) http.Handler {
	controller := principal.ProfileController(func(r *http.Request) string {
		route := chiRoutePattern__core(r)
		if op := agentPolicies[r.Method+" "+route].Op; op != "" {
			return op
		}
		return r.Method + " " + route
	}, next)
	return principal.ProfileStage("operation hooks", operationHooks__core)(controller)
}

//margince:decorate
func refuseUnknownQueryEnum(next http.Handler) http.Handler {
	return principal.ProfileStage("query check", refuseUnknownQueryEnum__core)(next)
}

//margince:decorate
func agentGate(reg *agents.Registry, staging agents.Approvals, stages agents.StageResolver, records datasource.SystemOfRecordProvider, ownership agents.FieldOwnership, imports agents.Imports, tags agents.Tags, gate *auth.Gate) func(http.Handler) http.Handler {
	return principal.ProfileStage("agent gate", agentGate__core(reg, staging, stages, records, ownership, imports, tags, gate))
}

//margince:decorate
func idempotency(pool *pgxpool.Pool, probes map[string]replayProbe, restore replayRestore) func(http.Handler) http.Handler {
	return principal.ProfileStage("idempotency", idempotency__core(pool, probes, restore))
}
