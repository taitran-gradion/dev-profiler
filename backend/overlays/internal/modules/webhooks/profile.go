//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package webhooks

import (
	"net/http"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

//margince:decorate
func NewGuardedClient() *http.Client {
	c := NewGuardedClient__core()
	c.Transport = principal.ProfileTransport(c.Transport)
	return c
}
