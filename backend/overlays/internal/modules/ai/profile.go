//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package ai

import (
	"context"
	"errors"
	"strconv"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// flush is reached once per logical model call, with every attempt it took,
// so it is where the profiler records AI calls.
//
//margince:decorate
func (r *Router) flush(ctx context.Context, b *binding, lc *logicalCall) {
	if now, ok := principal.ProfileSpanStart(ctx); ok && lc != nil && len(lc.attempts) > 0 {
		profileAICall(ctx, now, lc.attempts)
	}
	r.flush__core(ctx, b, lc)
}

func profileAICall(ctx context.Context, now float64, attempts []Call) {
	term := attempts[len(attempts)-1]
	var latency int64
	for _, a := range attempts {
		latency += a.LatencyMS
	}
	detail := map[string]string{
		"tier":       string(term.Tier),
		"provider":   term.Provider,
		"model":      term.ServedModel,
		"tokens_in":  strconv.Itoa(term.TokensIn),
		"tokens_out": strconv.Itoa(term.TokensOut),
		"cached":     strconv.Itoa(term.CachedTokens),
		"attempts":   strconv.Itoa(len(attempts)),
		"cache_hit":  strconv.FormatBool(term.CacheHit),
	}
	if term.Degraded {
		detail["degraded"] = "true"
	}
	var err error
	if term.ErrorSentinel != "" {
		err = errors.New(term.ErrorSentinel)
	}
	principal.ProfileSpanEnd(ctx, "ai", string(term.Task), now-float64(latency), detail, err)
}
