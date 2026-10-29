//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// sharedRedisClient is the api's one Redis client, so a hook on it sees every
// command a request sends: rate limits, caches and the event stream alike.
//
//margince:decorate
func sharedRedisClient(cfg apiConfig, logger *slog.Logger) (*redis.Client, func()) {
	rdb, closeRedis := sharedRedisClient__core(cfg, logger)
	rdb.AddHook(profileRedisHook{})
	return rdb, closeRedis
}

type profileRedisHook struct{}

func (profileRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (profileRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		at, ok := principal.ProfileSpanStart(ctx)
		err := next(ctx, cmd)
		if ok {
			principal.ProfileSpanEnd(ctx, "redis", strings.ToUpper(cmd.Name()), at, redisDetail(cmd), redisErr(err))
		}
		return err
	}
}

func (profileRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		at, ok := principal.ProfileSpanStart(ctx)
		err := next(ctx, cmds)
		if ok {
			names := make([]string, 0, len(cmds))
			for _, c := range cmds {
				names = append(names, strings.ToUpper(c.Name()))
			}
			principal.ProfileSpanEnd(ctx, "redis", "PIPELINE", at, map[string]string{"commands": strings.Join(names, " ")}, redisErr(err))
		}
		return err
	}
}

// redisDetail keeps the key a command names, never its values.
func redisDetail(cmd redis.Cmder) map[string]string {
	args := cmd.Args()
	if len(args) < 2 {
		return nil
	}
	key, ok := args[1].(string)
	if !ok {
		return nil
	}
	return map[string]string{"key": key}
}

// redisErr drops redis.Nil, which is a miss and not a failure.
func redisErr(err error) error {
	if errors.Is(err, redis.Nil) {
		return nil
	}
	return err
}
