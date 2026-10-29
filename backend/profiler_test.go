// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package profiler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/margince/margince/backend/pkg/extension"
)

const ringJSON = `[
  {"token":"t2","method":"POST","path":"/v1/deals","route":"/deals","status":201,"started_at":"2026-10-28T10:00:01Z","duration_ms":12.5,
   "user_id":"alice","queries":[{"sql":"SELECT 1","duration_ms":1.5},{"sql":"INSERT","duration_ms":2}],"writes":[{"entity":"deal"}],"hooks":[],"logs":[],
   "spans":[{"kind":"ai"},{"kind":"redis"},{"kind":"redis"},{"kind":"http"},{"kind":"tool"},{"kind":"event"}]},
  {"token":"t1","method":"GET","path":"/v1/deals","status":200,"started_at":"2026-10-28T10:00:00Z","duration_ms":3,
   "user_id":"bob","queries":[],"writes":[],"hooks":[],"logs":[]}
]`

// callerRuntime answers Caller only; a handler here needs nothing else.
type callerRuntime struct {
	extension.Runtime
	caller extension.Caller
}

func (r callerRuntime) Caller() extension.Caller { return r.caller }

func as(user string) extension.Runtime {
	return callerRuntime{caller: extension.Caller{Type: extension.CallerHuman, UserID: user}}
}

func holding(raw string) requests {
	return requests{read: func() (json.RawMessage, bool) { return json.RawMessage(raw), true }}
}

func TestListShowsOnlyTheCallersRequestsWithTheirTotals(t *testing.T) {
	out, err := holding(ringJSON).list(context.Background(), as("alice"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Requests []summary }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != 1 {
		t.Fatalf("alice sees %d requests, want her one", len(got.Requests))
	}
	s := got.Requests[0]
	if s.Token != "t2" || s.Queries != 2 || s.SQLMs != 3.5 || s.Writes != 1 {
		t.Fatalf("summary = %+v, want t2 with 2 queries over 3.5ms and 1 write", s)
	}
	if s.AICalls != 1 || s.ToolCalls != 1 || s.Redis != 2 || s.HTTP != 1 {
		t.Fatalf("summary = %+v, want 1 AI call, 1 tool call, 2 Redis commands and 1 HTTP request", s)
	}
}

func TestGetRefusesAnotherMembersRequestAsNotFound(t *testing.T) {
	_, err := holding(ringJSON).get(context.Background(), as("alice"), json.RawMessage(`{"token":"t1"}`))
	if !errors.Is(err, extension.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for bob's request", err)
	}
}

func TestGetReturnsTheWholeProfile(t *testing.T) {
	out, err := holding(ringJSON).get(context.Background(), as("alice"), json.RawMessage(`{"token":"t2"}`))
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Queries []struct{ SQL string }
	}
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Queries) != 2 || p.Queries[0].SQL != "SELECT 1" {
		t.Fatalf("queries = %+v, want both statements", p.Queries)
	}
}

func TestACallWithNobodyBehindItOwnsNoRequests(t *testing.T) {
	out, err := holding(ringJSON).list(context.Background(), callerRuntime{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"requests":[]}` {
		t.Fatalf("got %s, want no requests", out)
	}
}

func TestWithoutTheOverlaysThereIsNothingToList(t *testing.T) {
	r := requests{read: func() (json.RawMessage, bool) { return nil, false }}
	out, err := r.list(context.Background(), as("alice"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"requests":[]}` {
		t.Fatalf("got %s, want no requests", out)
	}
}

func TestClearForgetsOnlyTheCallersRequests(t *testing.T) {
	var clearedFor string
	r := holding(ringJSON)
	r.forget = func(userID string) (int, bool) {
		clearedFor = userID
		return 1, true
	}
	out, err := r.clear(context.Background(), as("alice"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if clearedFor != "alice" || string(out) != `{"cleared":1}` {
		t.Fatalf("cleared for %q with %s, want alice's one request", clearedFor, out)
	}
}

func TestClearWithNobodyBehindItIsRefused(t *testing.T) {
	_, err := holding(ringJSON).clear(context.Background(), callerRuntime{}, json.RawMessage(`{}`))
	if !errors.Is(err, extension.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}
