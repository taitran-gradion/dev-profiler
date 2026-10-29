//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package httpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/margince/margince/backend/internal/platform/httperr"
	"github.com/margince/margince/backend/internal/shared/apperrors"

	"github.com/margince/margince/backend/internal/shared/kernel/principal"
)

// ProfileTokenHeader carries a profiled request's token, the key the
// profiler's screen opens it by.
const ProfileTokenHeader = "X-Margince-Profile"

// Correlate opens a profile inside the correlation scope, so the profile's
// token is the correlation id its log lines and audit rows carry.
//
//margince:decorate
func Correlate(next http.Handler) http.Handler {
	return Correlate__core(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, err := profileRequest(r)
		if err != nil {
			httperr.Write(w, r, err)
			return
		}
		ctx, token, finish, ok := principal.StartProfile(r.Context(), r.Method, r.URL.RequestURI(), request)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set(ProfileTokenHeader, token)
		rec := &profileRecorder{ResponseWriter: w, status: http.StatusOK}
		returned := false
		defer func() {
			// A panic is answered 500 by RecoverPanics, outside this wrapper.
			if !returned && !rec.wrote {
				rec.status = http.StatusInternalServerError
			}
			streamed := strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream")
			finish(rec.status, principal.NewProfileMessage(w.Header(), rec.body.Bytes(), rec.size, streamed))
		}()
		next.ServeHTTP(rec, r.WithContext(ctx))
		returned = true
	}))
}

// profileRequest keeps the request's headers and the first ProfileBodyCap of
// its body, and gives the handler the whole body back unread.
func profileRequest(r *http.Request) (principal.ProfileMessage, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return principal.NewProfileMessage(r.Header, nil, 0, false), nil
	}
	head, err := io.ReadAll(io.LimitReader(r.Body, principal.ProfileBodyCap))
	if err != nil {
		return principal.ProfileMessage{}, fmt.Errorf("%w: the request body could not be read", apperrors.ErrInvalidArgument)
	}
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), r.Body), r.Body}
	size := r.ContentLength
	if size < int64(len(head)) {
		size = int64(len(head))
	}
	return principal.NewProfileMessage(r.Header, head, size, false), nil
}

// profileRecorder keeps the first status sent and the first ProfileBodyCap of
// the body, and counts all of it.
type profileRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
	body   bytes.Buffer
	size   int64
}

func (r *profileRecorder) WriteHeader(status int) {
	if !r.wrote {
		r.status, r.wrote = status, true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *profileRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	r.size += int64(len(b))
	if room := principal.ProfileBodyCap - r.body.Len(); room > 0 {
		r.body.Write(b[:min(room, len(b))])
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.NewResponseController reach the connection for flushes and
// deadlines.
func (r *profileRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// WithCorrelation also keeps each line logged in a profiled request's context
// in that request's profile.
//
//margince:decorate
func WithCorrelation(h slog.Handler) slog.Handler {
	return WithCorrelation__core(&profileLogHandler{inner: h})
}

type profileLogHandler struct {
	inner slog.Handler
	attrs []slog.Attr
}

func (h *profileLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *profileLogHandler) Handle(ctx context.Context, rec slog.Record) error {
	attrs := make(map[string]string, len(h.attrs)+rec.NumAttrs())
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.String()
	}
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = fmt.Sprint(a.Value.Any())
		return true
	})
	principal.ProfileLogLine(ctx, rec.Level.String(), rec.Message, attrs)
	return h.inner.Handle(ctx, rec)
}

func (h *profileLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &profileLogHandler{inner: h.inner.WithAttrs(attrs), attrs: append(append([]slog.Attr{}, h.attrs...), attrs...)}
}

func (h *profileLogHandler) WithGroup(name string) slog.Handler {
	return &profileLogHandler{inner: h.inner.WithGroup(name), attrs: h.attrs}
}
