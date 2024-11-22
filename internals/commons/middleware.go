package commons

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func (s *Server) LoggingMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start timer
			start := time.Now()

			// Call the next handler
			next.ServeHTTP(w, r)

			// Log the response after handling
			duration := time.Since(start)

			// Log the incoming request
			s.Logger.WithFields(logrus.Fields{
				"path":     r.URL.Path,
				"method":   r.Method,
				"ip":       getClientIP(r),
				"duration": duration,
			}).Info("request")
		})
	}
}
