package commons

import log "github.com/sirupsen/logrus"

type Server struct {
	Logger *log.Logger
}

type HttpResp struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}
