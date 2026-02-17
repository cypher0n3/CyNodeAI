package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
)

// Recovery middleware recovers from panics.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
					)
					handlers.WriteInternalError(w, "Internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
