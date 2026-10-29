// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

// Package profiler is a Margince extension unit that shows what each request
// did: its SQL, audited writes, hook runs and log lines. The collection is
// compiled into core by the overlays in overlays/, which keep the last
// requests in memory; this package only serves them, each to the member who
// made them.
package profiler

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"

	"github.com/margince/margince/backend/pkg/extension"
)

// ringVar is the expvar the principal overlay publishes the ring under. This
// package cannot import core's internal packages, so the two meet in expvar.
const ringVar = "margince.profiler"

// held is the ring this process's overlays fill.
var held = requests{read: readRing, forget: clearRing}

// New returns the unit's declaration.
func New() extension.Extension {
	return extension.Extension{
		Name:        "profiler",
		Version:     "0.1.0",
		Description: "Shows what each of your recent requests did: its SQL, writes, hooks and log lines.",
		Tools: []extension.Tool{
			{Name: "profiler_list_requests", Handle: listRequests},
			{Name: "profiler_get_request", Handle: getRequest},
			{Name: "profiler_clear_requests", Handle: clearRequests},
		},
	}
}

func listRequests(ctx context.Context, rt extension.Runtime, args json.RawMessage) (json.RawMessage, error) {
	return held.list(ctx, rt, args)
}

func getRequest(ctx context.Context, rt extension.Runtime, args json.RawMessage) (json.RawMessage, error) {
	return held.get(ctx, rt, args)
}

func clearRequests(ctx context.Context, rt extension.Runtime, args json.RawMessage) (json.RawMessage, error) {
	return held.clear(ctx, rt, args)
}

// readRing returns the ring as JSON, newest first. ok is false when the
// overlays were not compiled in.
func readRing() (json.RawMessage, bool) {
	v := expvar.Get(ringVar)
	if v == nil {
		return nil, false
	}
	return json.RawMessage(v.String()), true
}

// userClearer is the ring's method for forgetting one member's requests,
// matched by shape because its type is core's.
type userClearer interface{ ClearUser(userID string) int }

func clearRing(userID string) (int, bool) {
	c, ok := expvar.Get(ringVar).(userClearer)
	if !ok {
		return 0, false
	}
	return c.ClearUser(userID), true
}

type requests struct {
	read   func() (json.RawMessage, bool)
	forget func(userID string) (int, bool)
}

// profile is the part of the overlay's profile the summary needs; get passes
// the whole of it through.
type profile struct {
	Token      string            `json:"token"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Route      string            `json:"route,omitempty"`
	Operation  string            `json:"operation,omitempty"`
	Status     int               `json:"status"`
	StartedAt  string            `json:"started_at"`
	DurationMs float64           `json:"duration_ms"`
	UserID     string            `json:"user_id,omitempty"`
	Queries    []query           `json:"queries"`
	Writes     []json.RawMessage `json:"writes"`
	Spans      []span            `json:"spans"`
}

type span struct {
	Kind string `json:"kind"`
}

type query struct {
	DurationMs float64 `json:"duration_ms"`
}

type summary struct {
	Token      string  `json:"token"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Route      string  `json:"route,omitempty"`
	Operation  string  `json:"operation,omitempty"`
	Status     int     `json:"status"`
	StartedAt  string  `json:"started_at"`
	DurationMs float64 `json:"duration_ms"`
	Queries    int     `json:"queries"`
	SQLMs      float64 `json:"sql_ms"`
	Writes     int     `json:"writes"`
	AICalls    int     `json:"ai_calls"`
	ToolCalls  int     `json:"tool_calls"`
	Redis      int     `json:"redis"`
	HTTP       int     `json:"http"`
}

// own returns the caller's profiles with each one's raw JSON. A call with no
// member behind it owns none.
func (r requests) own(rt extension.Runtime) ([]profile, []json.RawMessage, error) {
	caller := rt.Caller()
	raw, ok := r.read()
	if !ok || caller.UserID == "" {
		return nil, nil, nil
	}
	var all []json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, nil, fmt.Errorf("profiler: reading the profile ring: %w", err)
	}
	var mine []profile
	var mineRaw []json.RawMessage
	for _, one := range all {
		var p profile
		if err := json.Unmarshal(one, &p); err != nil {
			return nil, nil, fmt.Errorf("profiler: reading a profile: %w", err)
		}
		if p.UserID == caller.UserID {
			mine, mineRaw = append(mine, p), append(mineRaw, one)
		}
	}
	return mine, mineRaw, nil
}

func (r requests) list(_ context.Context, rt extension.Runtime, _ json.RawMessage) (json.RawMessage, error) {
	mine, _, err := r.own(rt)
	if err != nil {
		return nil, err
	}
	out := make([]summary, 0, len(mine))
	for _, p := range mine {
		s := summary{
			Token: p.Token, Method: p.Method, Path: p.Path, Route: p.Route, Operation: p.Operation,
			Status: p.Status, StartedAt: p.StartedAt, DurationMs: p.DurationMs,
			Queries: len(p.Queries), Writes: len(p.Writes),
		}
		for _, q := range p.Queries {
			s.SQLMs += q.DurationMs
		}
		for _, sp := range p.Spans {
			switch sp.Kind {
			case "ai":
				s.AICalls++
			case "tool":
				s.ToolCalls++
			case "redis":
				s.Redis++
			case "http":
				s.HTTP++
			}
		}
		out = append(out, s)
	}
	return json.Marshal(map[string][]summary{"requests": out})
}

func (r requests) get(_ context.Context, rt extension.Runtime, args json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(args, &in); err != nil || in.Token == "" {
		return nil, fmt.Errorf("%w: name the request by its token", extension.ErrInvalid)
	}
	mine, mineRaw, err := r.own(rt)
	if err != nil {
		return nil, err
	}
	for i, p := range mine {
		if p.Token == in.Token {
			return mineRaw[i], nil
		}
	}
	return nil, fmt.Errorf("%w: no request of yours with that token is still held; the profiler keeps the most recent 200", extension.ErrNotFound)
}

func (r requests) clear(_ context.Context, rt extension.Runtime, _ json.RawMessage) (json.RawMessage, error) {
	caller := rt.Caller()
	if caller.UserID == "" {
		return nil, fmt.Errorf("%w: clearing is something a member does, and this call has nobody behind it", extension.ErrForbidden)
	}
	// Without the overlays there is no ring, and so nothing of theirs to clear.
	cleared, _ := r.forget(caller.UserID)
	return json.Marshal(map[string]int{"cleared": cleared})
}
