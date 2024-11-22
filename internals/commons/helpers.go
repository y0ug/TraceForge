package commons

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	Logger *logrus.Logger
}

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

func getClientIP(r *http.Request) string {
	// Look for X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can have multiple IPs; the first one is usually the original client IP
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Fallback to RemoteAddr if X-Forwarded-For is not set
	clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	return clientIP
}

func WriteJSONResponse(w http.ResponseWriter, httpStatus int, data HttpResp) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(data)
}

func WriteSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	WriteJSONResponse(w,
		http.StatusOK,
		HttpResp{Status: "success", Data: data, Message: message})
}

func WriteErrorResponse(w http.ResponseWriter, message string, httpStatus int) {
	WriteJSONResponse(w,
		httpStatus,
		HttpResp{Status: "error", Data: nil, Message: message})
}
