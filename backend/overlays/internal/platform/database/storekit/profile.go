//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package storekit

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/margince/margince/backend/internal/shared/kernel/ids"
	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// runWriteHook is reached once per audited write, after its audit row, so it
// is where the profiler counts writes.
//
//margince:decorate
func runWriteHook(ctx context.Context, tx pgx.Tx, w AuditedWrite) error {
	principal.ProfileWriteDone(ctx, w.EntityType, w.Action, w.EntityID.String())
	return runWriteHook__core(ctx, tx, w)
}

// Emit is the one writer of the outbox, so every event a request raises
// passes it.
//
//margince:decorate
func Emit(ctx context.Context, tx pgx.Tx, auditID ids.UUID, eventType, entityType string, entityID ids.UUID, payload any) error {
	at, ok := principal.ProfileSpanStart(ctx)
	err := Emit__core(ctx, tx, auditID, eventType, entityType, entityID, payload)
	if ok {
		principal.ProfileSpanEnd(ctx, "event", eventType, at, map[string]string{"entity": entityType, "id": entityID.String()}, err)
	}
	return err
}
