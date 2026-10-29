//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package identity

import (
	"net/http"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// Middleware resolves the session behind every /v1 request.
//
//margince:decorate
func (h Handlers) Middleware(next http.Handler) http.Handler {
	return principal.ProfileStage("auth", h.Middleware__core)(next)
}
