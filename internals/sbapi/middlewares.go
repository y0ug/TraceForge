package sbapi

import (
	"TraceForge/internals/commons"
	"net/http"
	"strings"
)

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.Logger.Warn("No Authorization header")
			commons.WriteErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			s.Logger.Warn("Invalid Authorization header format")
			commons.WriteErrorResponse(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		// Validate token
		if headerParts[1] != s.Config.AuthToken {
			s.Logger.Warn("Invalid token")
			commons.WriteErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Token is valid, proceed with the request
		next.ServeHTTP(w, r)
	})
}
