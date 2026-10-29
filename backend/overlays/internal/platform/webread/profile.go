//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package webread

import (
	"net/http"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

//margince:decorate
func newFetcher(transport http.RoundTripper) *Fetcher {
	return newFetcher__core(principal.ProfileTransport(transport))
}
