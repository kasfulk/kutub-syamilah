package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover returns a middleware that recovers from panics and returns a 500 error.
// Panics are logged with stack traces but never propagate to crash the server.
// Per golang-pro: no panics for normal error handling — this is a safety net.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				slog.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
					slog.Any("panic", rec),
					slog.String("stack", string(stack)),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				http.Error(w,
					`{"error":"INTERNAL_SERVER_ERROR","message":"internal server error"}`,
					http.StatusInternalServerError,
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
