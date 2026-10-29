//go:build margince_overlay

// SPDX-License-Identifier: BUSL-1.1
// SPDX-FileCopyrightText: 2026 Gradion

package database

import (
	"slices"
	"testing"
	"time"

	"github.com/margince/margince/backend/internal/shared/kernel/ids"
)

func TestProfileArgsPrintEachArgumentAsTheStatementReceivedIt(t *testing.T) {
	var noID *ids.UUID
	name := "Acme"
	at := time.Date(2026, 10, 21, 9, 40, 28, 0, time.UTC)
	got := profileArgs([]any{
		noID, &name, nil, 42, []byte("text"), []byte{0xff, 0x00},
		"O'Brien", at, true, []string{"a", "b"}, []int{},
	})
	want := []string{
		"NULL", "'Acme'", "NULL", "42", "'text'", `'\xff00'`,
		"'O''Brien'", "'2026-10-21T09:40:28Z'", "true", "ARRAY['a', 'b']", "'{}'",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}
}
