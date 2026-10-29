//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package httpserver

import (
	"bytes"
	"encoding/json"
	"expvar"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

type heldProfile struct {
	Spans []struct {
		Kind, Name string
		Detail     map[string]string
	}
	Token             string
	Path              string
	Route             string
	Status            int
	Writes            []struct{ Entity, Action string }
	Logs              []struct{ Message string }
	Request, Response struct {
		Headers   map[string][]string
		Body      string
		BodyBytes int64 `json:"body_bytes"`
		Cut       bool
	}
}

func ringHolds(t *testing.T, token string) (heldProfile, bool) {
	t.Helper()
	var ring []heldProfile
	if err := json.Unmarshal([]byte(expvar.Get(principal.ProfilesVar).String()), &ring); err != nil {
		t.Fatal(err)
	}
	for _, p := range ring {
		if p.Token == token {
			return p, true
		}
	}
	return heldProfile{}, false
}

func TestAProfiledRequestKeepsItsRouteWritesLogsAndStatus(t *testing.T) {
	log := slog.New(WithCorrelation(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	h := Correlate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal.ProfileRoute(r.Context(), "/deals", "createDeal")
		principal.ProfileWriteDone(r.Context(), "deal", "create", "d1")
		log.InfoContext(r.Context(), "deal created")
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/deals?limit=5&since=2026-10-21T09%3A40%3A28Z", nil))

	token := rec.Header().Get(ProfileTokenHeader)
	p, ok := ringHolds(t, token)
	if token == "" || !ok {
		t.Fatalf("no profile held for token %q", token)
	}
	if p.Status != http.StatusCreated || p.Route != "/deals" || len(p.Writes) != 1 || len(p.Logs) != 1 || p.Logs[0].Message != "deal created" {
		t.Fatalf("profile = %+v, want the 201, its route, one write and one log line", p)
	}
	if p.Path != "/v1/deals?limit=5&since=2026-10-21T09%3A40%3A28Z" {
		t.Fatalf("path = %q, want the path with its query as sent", p.Path)
	}
}

func TestTheProfilersOwnRoutesAreNotProfiled(t *testing.T) {
	h := Correlate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/ext/profiler/requests", nil))
	if got := rec.Header().Get(ProfileTokenHeader); got != "" {
		t.Fatalf("the profiler's own read was profiled as %s", got)
	}
}

func TestAStageEndsWhereItHandsOnAndAControllerIsNamedByItsOperation(t *testing.T) {
	passes := func(next http.Handler) http.Handler { return next }
	refuses := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	}
	controller := principal.ProfileController(func(*http.Request) string { return "listDeals" },
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	served := Correlate(principal.ProfileStage("auth", passes)(controller))
	refused := Correlate(principal.ProfileStage("gate", refuses)(controller))

	for name, tc := range map[string]struct {
		h         http.Handler
		wantSpans []string
		answered  bool
	}{
		"handed on": {served, []string{"stage:auth", "controller:listDeals"}, false},
		"answered":  {refused, []string{"stage:gate"}, true},
	} {
		rec := httptest.NewRecorder()
		tc.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/deals", nil))
		p, ok := ringHolds(t, rec.Header().Get(ProfileTokenHeader))
		if !ok {
			t.Fatalf("%s: no profile held", name)
		}
		var got []string
		for _, s := range p.Spans {
			got = append(got, s.Kind+":"+s.Name)
		}
		if strings.Join(got, ",") != strings.Join(tc.wantSpans, ",") {
			t.Fatalf("%s: spans = %v, want %v", name, got, tc.wantSpans)
		}
		if (p.Spans[0].Detail["answered"] == "here") != tc.answered {
			t.Fatalf("%s: stage detail = %v, answered here should be %v", name, p.Spans[0].Detail, tc.answered)
		}
	}
}

func TestAProfileKeepsTheRequestAndResponseAndHandsTheBodyOnWhole(t *testing.T) {
	var handlerRead string
	h := Correlate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		handlerRead = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/deals", strings.NewReader(`{"name":"Acme"}`))
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	p, ok := ringHolds(t, rec.Header().Get(ProfileTokenHeader))
	if !ok {
		t.Fatal("no profile held")
	}
	if handlerRead != `{"name":"Acme"}` {
		t.Fatalf("handler read %q, want the whole body", handlerRead)
	}
	if p.Request.Body != `{"name":"Acme"}` || p.Request.Headers["Authorization"][0] != "Bearer dev" {
		t.Fatalf("request = %+v, want its body and headers", p.Request)
	}
	if p.Response.Body != `{"ok":true}` || p.Response.Headers["Content-Type"][0] != "application/json" {
		t.Fatalf("response = %+v, want its body and headers", p.Response)
	}
}

func TestABodyPastTheCapIsCutAndCounted(t *testing.T) {
	big := strings.Repeat("a", principal.ProfileBodyCap+10)
	h := Correlate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(big)))

	if rec.Body.Len() != len(big) {
		t.Fatalf("the handler echoed %d bytes, want all %d", rec.Body.Len(), len(big))
	}
	p, _ := ringHolds(t, rec.Header().Get(ProfileTokenHeader))
	for side, m := range map[string]struct {
		Body      string
		BodyBytes int64
		Cut       bool
	}{"request": {p.Request.Body, p.Request.BodyBytes, p.Request.Cut}, "response": {p.Response.Body, p.Response.BodyBytes, p.Response.Cut}} {
		if len(m.Body) != principal.ProfileBodyCap || m.BodyBytes != int64(len(big)) || !m.Cut {
			t.Fatalf("%s kept %d of %d bytes, cut=%v; want the cap, the full size and cut", side, len(m.Body), m.BodyBytes, m.Cut)
		}
	}
}
