package keenbase

import (
	"encoding/json"
	"net/http"
)

// apiError is the standard error envelope returned by all endpoints,
// matching PocketBase's error response format.
type apiError struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// fieldError describes a validation failure on a single field.
// Used inside the Data map of an apiError.
type fieldError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises data as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a standard error envelope response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{
		Status:  status,
		Message: message,
		Data:    map[string]any{},
	})
}

// writeValidationError writes a 400 response with per-field validation details.
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
