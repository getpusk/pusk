// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"io"
	"log/slog"
)

// newLogHandler builds the process-wide slog handler: JSON when format is
// "json", text otherwise.
//
// Request data (paths, IPs, names from API bodies) is logged as attribute
// values. Both stdlib handlers escape them — JSON escapes control characters,
// text quotes any value containing spaces, quotes or control characters — so
// a value cannot start a new log record. Log messages stay string literals
// (enforced by TestSlogMessagesAreLiterals). Replacing these handlers with a
// custom one must keep that property (TestLogHandlerNeutralizesInjection).
func newLogHandler(format string, w io.Writer) slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if format == "json" {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}
