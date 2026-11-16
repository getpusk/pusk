// Copyright (c) 2026 Volkov Pavel | DevITWay
// Licensed under the Business Source License 1.1. See LICENSE file for details.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Values a client controls (path, IP header, names in API bodies) that try to
// forge a second log record or rewrite the terminal.
var injectionPayloads = []string{
	"x\nlevel=ERROR msg=forged",
	"x\r\nlevel=ERROR msg=forged",
	`x" level=ERROR msg="forged`,
	"x level=ERROR msg=forged",
	"x\x1b[2Kforged",
}

// injectionSafe logs one record per payload through h and checks that the
// payload stays inside a single record.
func injectionSafe(newHandler func(io.Writer) slog.Handler, isJSON bool) error {
	for _, p := range injectionPayloads {
		var buf bytes.Buffer
		slog.New(newHandler(&buf)).Info("constant message", "v", p)
		out := buf.String()
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			return fmt.Errorf("payload %q: record spans several lines: %q", p, out)
		}
		for _, raw := range []string{"\r", " ", "\x1b"} {
			if strings.Contains(out, raw) {
				return fmt.Errorf("payload %q: raw %q in output: %q", p, raw, out)
			}
		}
		if isJSON {
			var rec map[string]any
			if err := json.Unmarshal([]byte(out), &rec); err != nil {
				return fmt.Errorf("payload %q: not one JSON object: %v", p, err)
			}
			if rec["v"] != p || rec["msg"] != "constant message" {
				return fmt.Errorf("payload %q: fields changed: %v", p, rec)
			}
			continue
		}
		// Text: the value must come back intact from its quoted form.
		i := strings.Index(out, " v=")
		if i < 0 {
			return fmt.Errorf("payload %q: no v= attribute: %q", p, out)
		}
		got, err := strconv.Unquote(strings.TrimSuffix(out[i+3:], "\n"))
		if err != nil || got != p {
			return fmt.Errorf("payload %q: value not quoted as one token: %q", p, out)
		}
	}
	return nil
}

func TestLogHandlerNeutralizesInjection(t *testing.T) {
	for _, format := range []string{"json", "", "text"} {
		h := func(w io.Writer) slog.Handler { return newLogHandler(format, w) }
		if err := injectionSafe(h, format == "json"); err != nil {
			t.Errorf("format %q: %v", format, err)
		}
	}
}

// rawHandler writes values verbatim — what a hand-rolled handler tends to do.
type rawHandler struct{ w io.Writer }

func (h rawHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h rawHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h rawHandler) WithGroup(string) slog.Handler            { return h }
func (h rawHandler) Handle(_ context.Context, r slog.Record) error {
	line := r.Message
	r.Attrs(func(a slog.Attr) bool { line += " " + a.Key + "=" + a.Value.String(); return true })
	_, err := fmt.Fprintln(h.w, line)
	return err
}

// Control: the check must reject a handler that does not escape values.
func TestInjectionSafeRejectsRawHandler(t *testing.T) {
	if err := injectionSafe(func(w io.Writer) slog.Handler { return rawHandler{w} }, false); err == nil {
		t.Fatal("injectionSafe accepted a handler that writes values verbatim")
	}
}

// Position of the message argument in package-level slog calls.
var slogMsgArg = map[string]int{
	"Debug": 0, "Info": 0, "Warn": 0, "Error": 0,
	"DebugContext": 1, "InfoContext": 1, "WarnContext": 1, "ErrorContext": 1,
	"Log": 2, "LogAttrs": 2,
}

// slogViolations reports slog calls whose message is not a string literal
// (request data would then bypass attribute escaping) and imports of the
// stdlib "log" package (it formats values into the line without escaping).
func slogViolations(fset *token.FileSet, f *ast.File) []string {
	var out []string
	for _, imp := range f.Imports {
		if imp.Path.Value == `"log"` {
			out = append(out, fmt.Sprintf("%s: stdlib log imported, use log/slog", fset.Position(imp.Pos())))
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "slog" {
			return true
		}
		idx, ok := slogMsgArg[sel.Sel.Name]
		if !ok || idx >= len(call.Args) {
			return true
		}
		if lit, ok := call.Args[idx].(*ast.BasicLit); !ok || lit.Kind != token.STRING {
			out = append(out, fmt.Sprintf("%s: slog.%s message is not a string literal", fset.Position(call.Pos()), sel.Sel.Name))
		}
		return true
	})
	return out
}

func TestSlogMessagesAreLiterals(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	scanned := 0
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			scanned++
			for _, v := range slogViolations(fset, f) {
				t.Error(v)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scanned < 20 {
		t.Fatalf("scanned only %d files — wrong root?", scanned)
	}
}

// Control: the check must flag each kind of violation and pass literals.
func TestSlogViolationsControl(t *testing.T) {
	src := `package x
import (
	"context"
	"fmt"
	"log"
	"log/slog"
)
func f(ctx context.Context, s string) {
	slog.Info("ok", "v", s)
	slog.Info(
		"ok multiline",
		"v", s,
	)
	slog.ErrorContext(ctx, "ok ctx", "v", s)
	slog.Info(fmt.Sprintf("bad %s", s))
	slog.WarnContext(ctx, s)
	slog.Log(ctx, slog.LevelInfo, "bad"+s)
	log.Printf("%s", s)
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := slogViolations(fset, f)
	if len(got) != 4 {
		t.Fatalf("want 4 violations (log import, Sprintf, variable, concat), got %d: %v", len(got), got)
	}
}
