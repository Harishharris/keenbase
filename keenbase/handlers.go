package keenbase

import (
	"net/http"
	"time"
)

// ------------------------------------------------------------------ health

func (sb *SimpleBase) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  200,
		"message": "API is healthy.",
		"data": map[string]any{
			"canConnectDatabase": true,
			"time":               time.Now().UTC().Format(timeLayout),
		},
	})
}

// ------------------------------------------------------------------ auth stubs (implemented later)

func (sb *SimpleBase) handleListAuthMethods(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleAuthWithPassword(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleAuthRefresh(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleRequestVerification(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleConfirmVerification(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleRequestOTP(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}

func (sb *SimpleBase) handleAuthWithOTP(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}
