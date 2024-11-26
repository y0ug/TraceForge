package commons

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

func GetEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("%s must be set", key)
	}
	return value
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

func WriteJSONResponse(w http.ResponseWriter, httpStatus int, data *HttpResp) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(data)
}

func WriteSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	WriteJSONResponse(w,
		http.StatusOK,
		&HttpResp{Status: "success", Data: data, Message: message})
}

func WriteErrorResponse(w http.ResponseWriter, message string, httpStatus int) {
	WriteJSONResponse(w,
		httpStatus,
		&HttpResp{Status: "error", Data: nil, Message: message})
}

func WriteErrorResponseData(w http.ResponseWriter, message string, data interface{}, httpStatus int) {
	WriteJSONResponse(w,
		httpStatus,
		&HttpResp{Status: "error", Data: data, Message: message})
}
