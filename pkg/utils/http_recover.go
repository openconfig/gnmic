// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gorilla/mux"
)

// RecoverMiddleware returns an HTTP middleware that recovers from panics in
// the wrapped handler. The panic is logged with the request method, path and
// matched route template, and a `500 Internal Server Error` JSON response is
// sent when the handler had not started writing a response yet.
//
// Without it, net/http recovers the panic per connection: the client gets a
// reset connection with no status, and a handler that panics while holding a
// lock leaves it held.
//
// The http.ErrAbortHandler sentinel is re-panicked, as net/http expects.
// logger is resolved per request so it may be nil until the server starts.
func RecoverMiddleware(logger func() *slog.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &recoverResponseWriter{ResponseWriter: w}
			defer func() {
				p := recover()
				if p == nil {
					return
				}
				if p == http.ErrAbortHandler {
					panic(p)
				}
				l := slog.Default()
				if logger != nil {
					if ll := logger(); ll != nil {
						l = ll
					}
				}
				route := ""
				if cr := mux.CurrentRoute(r); cr != nil {
					route, _ = cr.GetPathTemplate()
				}
				l.Error("panic in HTTP handler",
					"method", r.Method,
					"path", r.URL.Path,
					"route", route,
					"panic", p,
					"stack", string(debug.Stack()),
				)
				if rw.wroteHeader {
					// the response is already on its way, nothing more can be said to the client.
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":["internal server error"]}`))
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// recoverResponseWriter records whether the wrapped handler started a response.
// It keeps http.Flusher available for streaming handlers and exposes Unwrap for
// http.ResponseController.
type recoverResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *recoverResponseWriter) WriteHeader(code int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *recoverResponseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *recoverResponseWriter) Flush() {
	w.wroteHeader = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *recoverResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
