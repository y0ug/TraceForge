package commons

import log "github.com/sirupsen/logrus"

type Server struct {
	Logger *log.Logger
}

// HttpResp represents the standard HTTP response structure.
// swagger:model
type HttpResp struct {
	Status  string      `json:"status" example:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message" example:"Operation completed successfully"`
}
