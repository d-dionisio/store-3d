package handlers

import (
	"context"
	"encoding/json"
	"net/http"
)

// ReadinessChecker reports whether an external dependency is available.
type ReadinessChecker interface {
	Check(context.Context) error
}

type readyResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func Ready(checker ReadinessChecker) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := checker.Check(request.Context()); err != nil {
			response.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(response).Encode(readyResponse{
				Status:   "unavailable",
				Database: "disconnected",
			})
			return
		}

		response.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(response).Encode(readyResponse{
			Status:   "ready",
			Database: "connected",
		})
	}
}
