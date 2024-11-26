package mq

import (
	"TraceForge/internals/commons"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// PushMessage handles adding messages to a specific agent's queue
func (s *ServerSQS) PushMessageHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Body string `json:"body"`
	}
	vars := mux.Vars(r)
	queueID := vars["queue_id"]

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.Logger.WithError(err).Error("Invalid request body")
		commons.WriteErrorResponse(w, "Invalid Parameter", http.StatusBadRequest)
		return
	}

	err := CreateMessage(r.Context(), s.DB, queueID, body.Body)
	if err != nil {
		s.Logger.WithError(err).Error("failed to create message")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"queue_id": queueID,
		"body":     body.Body,
	}).Info("message pushed successfully")
	commons.WriteJSONResponse(w, http.StatusCreated,
		&commons.HttpResp{Status: "success", Data: nil, Message: "Message pushed successfully"})
}

// PullMessage handles fetching the next message for an agent
func (s *ServerSQS) PullMessageHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	queueID := vars["queue_id"]

	// TODO used a SQL transaction
	msg, err := GetMessage(ctx, s.DB, queueID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to retrieve message")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if msg == nil {
		s.Logger.WithField("queue_id", queueID).Info("no messages available")
		commons.WriteErrorResponse(w, "no messages available", http.StatusNotFound)
		return
	}

	// Set visibility timeout (e.g., 30 seconds)
	err = SetMessageVisibility(ctx, s.DB, msg.ID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update visibility timeout")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"queue_id": msg.QueueID,
		"message":  msg.Body,
	}).Info("Message pulled successfully")
	json.NewEncoder(w).Encode(msg)
}

// DeleteMessage handles deleting a processed message
func (s *ServerSQS) DeleteMessageHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	msgID := vars["message_id"]

	err := DeleteMessage(ctx, s.DB, msgID)
	if err != nil {
		s.Logger.WithError(err).WithField("message_id", msgID).Error("Failed to delete message")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithField("message_id", msgID).Info("Message deleted successfully")
	commons.WriteSuccessResponse(w, "message deleted", nil)
}
