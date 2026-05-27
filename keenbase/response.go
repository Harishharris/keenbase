package keenbase

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

type fieldError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{
		Status:  status,
		Message: message,
		Data:    map[string]any{},
	})
}

func writeValidationError(w http.ResponseWriter, fields map[string]fieldError) {
	data := make(map[string]any, len(fields))
	for name, fe := range fields {
		data[name] = fe
	}
	writeJSON(w, http.StatusBadRequest, apiError{
		Status:  http.StatusBadRequest,
		Message: "Failed to create record.",
		Data:    data,
	})
}
