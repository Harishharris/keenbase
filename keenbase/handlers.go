package keenbase

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

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

func (sb *SimpleBase) handleListAuthMethods(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	if !col.IsAuth() {
		writeError(w, http.StatusBadRequest, "The collection is not an auth collection.")
		return
	}

	var passwordEnabled bool
	var identityFields []string

	if col.AuthOptions != nil {
		passwordEnabled = col.AuthOptions.PasswordAuth.Enabled
		identityFields = col.AuthOptions.PasswordAuth.IdentityFields
	}
	if len(identityFields) == 0 {
		identityFields = []string{"email"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"password": map[string]any{
			"enabled":        passwordEnabled,
			"identityFields": identityFields,
		},
		"oauth2": map[string]any{
			"enabled":   false,
			"providers": []any{},
		},
		"otp": map[string]any{
			"enabled":  false,
			"duration": 0,
		},
		"mfa": map[string]any{
			"enabled":  false,
			"duration": 0,
		},
	})
}

func (sb *SimpleBase) handleAuthWithPassword(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	if !col.IsAuth() {
		writeError(w, http.StatusBadRequest, "The collection is not an auth collection.")
		return
	}

	var body struct {
		Identity string `json:"identity"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if body.Identity == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "identity and password are required.")
		return
	}

	resp, err := sb.auth.AuthWithPassword(col, body.Identity, body.Password)
	if err != nil {
		if errors.Is(err, ErrPasswordAuthDisabled) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Use 400 for bad credentials — don't leak whether the user exists.
		writeError(w, http.StatusBadRequest, "Failed to authenticate. "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (sb *SimpleBase) handleAuthRefresh(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	if !col.IsAuth() {
		writeError(w, http.StatusBadRequest, "The collection is not an auth collection.")
		return
	}

	auth := authFromContext(r.Context())
	if auth.IsGuest() {
		writeError(w, http.StatusUnauthorized, "The request requires valid authorization token.")
		return
	}

	if auth.CollectionID != col.ID {
		writeError(w, http.StatusForbidden, "Token does not belong to this collection.")
		return
	}

	resp, err := sb.auth.RefreshToken(col, auth.RecordID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to refresh token.")
		return
	}

	writeJSON(w, http.StatusOK, resp)
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
