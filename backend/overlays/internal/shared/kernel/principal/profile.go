//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package principal

// The profiler's collector lives in principal because every package that sees
// part of a request (the HTTP chassis, the pool, storekit, compose) already
// imports it, so no overlay adds an import edge core does not have.

import (
	"context"
	"encoding/json"
	"expvar"
	"net/http"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ProfilesVar is the expvar the profiler unit reads. The unit cannot import
// this package, and expvar is the registry both sides already share.
const ProfilesVar = "margince.profiler"

// The caps bound one request's profile and the whole ring, so a request that
// loops over the database cannot grow the process without limit.
const (
	profileRing       = 200
	profileMaxQueries = 500
	profileMaxEntries = 200
)

// profilerPath is the profiler's own routes, which are never recorded: a
// screen that polls would otherwise fill the ring with itself.
const profilerPath = "/v1/ext/profiler/"

// Profile is what one request did. JSON names are the unit's wire shape.
type Profile struct {
	Token  string `json:"token"`
	Method string `json:"method"`
	// Path is the request's path and query string, as the browser sent it.
	Path       string         `json:"path"`
	Route      string         `json:"route,omitempty"`
	Operation  string         `json:"operation,omitempty"`
	Status     int            `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	DurationMs float64        `json:"duration_ms"`
	ActorType  string         `json:"actor_type,omitempty"`
	ActorID    string         `json:"actor_id,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	Queries    []ProfileQuery `json:"queries"`
	Writes     []ProfileWrite `json:"writes"`
	Hooks      []ProfileHook  `json:"hooks"`
	Logs       []ProfileLog   `json:"logs"`
	Spans      []ProfileSpan  `json:"spans"`
	Request    ProfileMessage `json:"request"`
	Response   ProfileMessage `json:"response"`
	Dropped    int            `json:"dropped"`
}

// ProfileMessage is one side of the exchange: its headers and as much of its
// body as the cap keeps.
type ProfileMessage struct {
	Headers map[string][]string `json:"headers"`
	// Body is the body as text; a body that is not UTF-8 is left out and
	// Binary says so.
	Body      string `json:"body,omitempty"`
	BodyBytes int64  `json:"body_bytes"`
	Cut       bool   `json:"cut,omitempty"`
	Binary    bool   `json:"binary,omitempty"`
	Streamed  bool   `json:"streamed,omitempty"`
}

// ProfileBodyCap is how much of one body a profile keeps. The ring holds 200
// requests, so an upload or an export kept whole would hold gigabytes.
const ProfileBodyCap = 1 << 20

// NewProfileMessage keeps headers and body as they were sent. size is the
// whole body's length, which may be more than body holds.
func NewProfileMessage(headers http.Header, body []byte, size int64, streamed bool) ProfileMessage {
	m := ProfileMessage{Headers: headers.Clone(), BodyBytes: size, Streamed: streamed, Cut: size > int64(len(body))}
	if m.Headers == nil {
		m.Headers = map[string][]string{}
	}
	if utf8.Valid(body) {
		m.Body = string(body)
	} else {
		m.Binary = true
	}
	return m
}

// ProfileSpan is one call out of the request that is not SQL: an AI model
// call, an agent tool, a Redis command, an outbox event or an outgoing HTTP
// request. Kind names which; Detail holds what that kind knows.
type ProfileSpan struct {
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	AtMs       float64           `json:"at_ms"`
	DurationMs float64           `json:"duration_ms"`
	Detail     map[string]string `json:"detail,omitempty"`
	Caller     string            `json:"caller,omitempty"`
	Err        string            `json:"error,omitempty"`
}

// ProfileQuery is one statement and its arguments, as printed values.
type ProfileQuery struct {
	SQL        string   `json:"sql"`
	Args       []string `json:"args,omitempty"`
	Caller     string   `json:"caller,omitempty"`
	AtMs       float64  `json:"at_ms"`
	DurationMs float64  `json:"duration_ms"`
	Rows       int64    `json:"rows"`
	Err        string   `json:"error,omitempty"`
}

// ProfileWrite is one audited write.
type ProfileWrite struct {
	Entity string  `json:"entity"`
	Action string  `json:"action"`
	ID     string  `json:"id"`
	AtMs   float64 `json:"at_ms"`
}

// ProfileHook is one extension hook run.
type ProfileHook struct {
	Unit       string  `json:"unit"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	AtMs       float64 `json:"at_ms"`
	DurationMs float64 `json:"duration_ms"`
	Err        string  `json:"error,omitempty"`
}

// ProfileLog is one log line written while the request ran.
type ProfileLog struct {
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Attrs   map[string]string `json:"attrs,omitempty"`
	AtMs    float64           `json:"at_ms"`
}

type profileKey struct{}

// profileCollector is shared by every goroutine a request starts, so each
// append takes the lock.
type profileCollector struct {
	mu    sync.Mutex
	start time.Time
	p     Profile
}

type profileStore struct {
	mu   sync.Mutex
	ring []Profile
	next int
}

var profiles = func() *profileStore {
	s := &profileStore{}
	expvar.Publish(ProfilesVar, s)
	return s
}()

// String implements expvar.Var.
func (s *profileStore) String() string { return string(s.snapshot()) }

// ClearUser drops one member's profiles and keeps everyone else's. The
// profiler unit reaches it through expvar by this method's shape alone.
func (s *profileStore) ClearUser(userID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Profile, 0, len(s.ring))
	for i := range s.ring {
		p := s.ring[(s.next+i)%len(s.ring)]
		if p.UserID != userID {
			kept = append(kept, p)
		}
	}
	cleared := len(s.ring) - len(kept)
	s.ring, s.next = kept, 0
	return cleared
}

func (s *profileStore) add(p Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ring) < profileRing {
		s.ring = append(s.ring, p)
		return
	}
	s.ring[s.next] = p
	s.next = (s.next + 1) % profileRing
}

// snapshot is the ring newest first, as JSON.
func (s *profileStore) snapshot() json.RawMessage {
	s.mu.Lock()
	out := make([]Profile, 0, len(s.ring))
	for i := range s.ring {
		out = append(out, s.ring[(s.next-1-i+2*len(s.ring))%len(s.ring)])
	}
	s.mu.Unlock()
	raw, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return raw
}

func profiling(ctx context.Context) *profileCollector {
	c, _ := ctx.Value(profileKey{}).(*profileCollector)
	return c
}

func (c *profileCollector) sinceMs() float64 { return ms(time.Since(c.start)) }

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// room counts what is past the cap instead of keeping it. Callers hold mu.
func (c *profileCollector) room(n, limit int) bool {
	if n < limit {
		return true
	}
	c.p.Dropped++
	return false
}

func (c *profileCollector) entries() int {
	return len(c.p.Writes) + len(c.p.Hooks) + len(c.p.Logs) + len(c.p.Spans)
}

// StartProfile opens a profile for a request whose correlation id is bound,
// and returns its token and the finish func that files it. ok is false for the
// profiler's own paths and for a request with no correlation id.
func StartProfile(ctx context.Context, method, path string, request ProfileMessage) (_ context.Context, token string, finish func(status int, response ProfileMessage), ok bool) {
	id, bound := CorrelationID(ctx)
	if !bound || strings.HasPrefix(path, profilerPath) {
		return ctx, "", nil, false
	}
	c := &profileCollector{start: time.Now(), p: Profile{
		Token: id.String(), Method: method, Path: path, StartedAt: time.Now().UTC(), Request: request,
		Queries: []ProfileQuery{}, Writes: []ProfileWrite{}, Hooks: []ProfileHook{}, Logs: []ProfileLog{}, Spans: []ProfileSpan{},
	}}
	return context.WithValue(ctx, profileKey{}, c), c.p.Token, func(status int, response ProfileMessage) {
		c.mu.Lock()
		c.p.Status, c.p.DurationMs, c.p.Response = status, c.sinceMs(), response
		p := c.p
		c.mu.Unlock()
		profiles.add(p)
	}, true
}

// ProfileRoute notes the contract route and operation the request reached.
func ProfileRoute(ctx context.Context, route, operation string) {
	if c := profiling(ctx); c != nil {
		c.mu.Lock()
		c.p.Route, c.p.Operation = route, operation
		c.mu.Unlock()
	}
}

func profileActor(ctx context.Context, p Principal) {
	c := profiling(ctx)
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.p.ActorType, c.p.ActorID = string(p.Type), p.ID
	if !p.UserID.IsZero() {
		c.p.UserID = p.UserID.String()
	}
}

// ProfileQueryStart returns when a statement started, for ProfileQueryEnd.
func ProfileQueryStart(ctx context.Context) (float64, bool) {
	c := profiling(ctx)
	if c == nil {
		return 0, false
	}
	return c.sinceMs(), true
}

// ProfileQueryEnd notes one finished statement.
func ProfileQueryEnd(ctx context.Context, sql string, args []string, atMs float64, rows int64, err error) {
	c := profiling(ctx)
	if c == nil {
		return
	}
	q := ProfileQuery{SQL: sql, Args: args, Caller: profileCaller(), AtMs: atMs, DurationMs: c.sinceMs() - atMs, Rows: rows}
	if err != nil {
		q.Err = err.Error()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room(len(c.p.Queries), profileMaxQueries) {
		c.p.Queries = append(c.p.Queries, q)
	}
}

// ProfileWriteDone notes one audited write.
func ProfileWriteDone(ctx context.Context, entity, action, id string) {
	c := profiling(ctx)
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room(c.entries(), profileMaxEntries) {
		c.p.Writes = append(c.p.Writes, ProfileWrite{Entity: entity, Action: action, ID: id, AtMs: c.sinceMs()})
	}
}

// ProfileHookRun runs one extension hook and notes its time and refusal.
func ProfileHookRun(ctx context.Context, unit, kind, name string, run func() error) error {
	c := profiling(ctx)
	if c == nil {
		return run()
	}
	at := c.sinceMs()
	err := run()
	h := ProfileHook{Unit: unit, Kind: kind, Name: name, AtMs: at, DurationMs: c.sinceMs() - at}
	if err != nil {
		h.Err = err.Error()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room(c.entries(), profileMaxEntries) {
		c.p.Hooks = append(c.p.Hooks, h)
	}
	return err
}

// ProfileLogLine notes one log line.
func ProfileLogLine(ctx context.Context, level, message string, attrs map[string]string) {
	c := profiling(ctx)
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room(c.entries(), profileMaxEntries) {
		c.p.Logs = append(c.p.Logs, ProfileLog{Level: level, Message: message, Attrs: attrs, AtMs: c.sinceMs()})
	}
}

// ProfileSpanStart returns when a span started, for ProfileSpanEnd. ok is
// false outside a profiled request, so a caller skips building its detail.
func ProfileSpanStart(ctx context.Context) (float64, bool) {
	return ProfileQueryStart(ctx)
}

// ProfileSpanEnd notes one finished span. A span with no duration, an event,
// passes its start as now.
func ProfileSpanEnd(ctx context.Context, kind, name string, atMs float64, detail map[string]string, err error) {
	c := profiling(ctx)
	if c == nil {
		return
	}
	s := ProfileSpan{Kind: kind, Name: name, AtMs: atMs, DurationMs: c.sinceMs() - atMs, Detail: detail}
	if kind != "stage" && kind != "controller" {
		s.Caller = profileCaller()
	}
	if err != nil {
		s.Err = err.Error()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room(c.entries(), profileMaxEntries) {
		c.p.Spans = append(c.p.Spans, s)
	}
}

// ProfileTransport records each request an HTTP client sends from a profiled
// request. A nil inner transport is net/http's default, as on a Client.
func ProfileTransport(inner http.RoundTripper) http.RoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	return profileTransport{inner: inner}
}

type profileTransport struct{ inner http.RoundTripper }

func (t profileTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	at, ok := ProfileSpanStart(req.Context())
	resp, err := t.inner.RoundTrip(req)
	if !ok {
		return resp, err
	}
	detail := map[string]string{"host": req.URL.Host}
	if resp != nil {
		detail["status"] = strconv.Itoa(resp.StatusCode)
	}
	ProfileSpanEnd(req.Context(), "http", req.Method+" "+req.URL.Host+req.URL.Path, at, detail, err)
	return resp, err
}

const backendModule = "github.com/margince/margince/backend/"

// callerSkips are the frames between the product code that asked for
// something and the profiler noting it: drivers, the database and storekit
// plumbing, net/http, and the overlays themselves.
var callerSkips = []string{
	"runtime.", "net/http.", "github.com/jackc/", "github.com/redis/",
	backendModule + "internal/platform/database",
	backendModule + "internal/shared/kernel/principal.",
}

// profileCaller names the first product function on the stack, as
// "package.Function file:line" relative to the backend.
func profileCaller() string {
	pcs := make([]uintptr, 48)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs)])
	for {
		f, more := frames.Next()
		if strings.HasPrefix(f.Function, backendModule) && !strings.HasPrefix(filepath.Base(f.File), "zz_overlay_") &&
			!slices.ContainsFunc(callerSkips, func(p string) bool { return strings.HasPrefix(f.Function, p) }) {
			file := f.File
			if i := strings.Index(file, "/backend/"); i >= 0 {
				file = file[i+len("/backend/"):]
			}
			return strings.TrimPrefix(f.Function, backendModule+"internal/") + " " + file + ":" + strconv.Itoa(f.Line)
		}
		if !more {
			return ""
		}
	}
}

type profileStageKey string

type profileStageMark struct {
	atMs float64
	done bool
}

// ProfileStage times one middleware's own share of a request: from entering
// it to its handing on to the next handler, or to its return when it answers
// the request itself.
func ProfileStage(name string, wrap func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	key := profileStageKey(name)
	return func(next http.Handler) http.Handler {
		handOn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if mark, ok := r.Context().Value(key).(*profileStageMark); ok && !mark.done {
				mark.done = true
				ProfileSpanEnd(r.Context(), "stage", name, mark.atMs, nil, nil)
			}
			next.ServeHTTP(w, r)
		})
		wrapped := wrap(handOn)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			at, ok := ProfileSpanStart(r.Context())
			if !ok {
				wrapped.ServeHTTP(w, r)
				return
			}
			mark := &profileStageMark{atMs: at}
			wrapped.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key, mark)))
			if !mark.done {
				ProfileSpanEnd(r.Context(), "stage", name, mark.atMs, map[string]string{"answered": "here"}, nil)
			}
		})
	}
}

// ProfileController times the handler that serves the operation.
func ProfileController(operation func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		at, ok := ProfileSpanStart(r.Context())
		next.ServeHTTP(w, r)
		if ok {
			ProfileSpanEnd(r.Context(), "controller", operation(r), at, nil, nil)
		}
	})
}

//margince:decorate
func WithActor(ctx context.Context, p Principal) context.Context {
	profileActor(ctx, p)
	return WithActor__core(ctx, p)
}
