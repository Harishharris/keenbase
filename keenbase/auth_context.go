package keenbase

import (
	"context"
	"net/http"
	"strings"
)

type authContextKey struct{}

type RequestAuth struct {
	RecordID     string
	CollectionID string
	IsSuperuser  bool
	// record holds the full auth record loaded from the DB (lazy, may be nil).
	record *Record
}

func (a *RequestAuth) IsGuest() bool {
	return a == nil || a.RecordID == ""
}

func contextWithAuth(ctx context.Context, auth *RequestAuth) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}

func authFromContext(ctx context.Context) *RequestAuth {
	if a, ok := ctx.Value(authContextKey{}).(*RequestAuth); ok && a != nil {
		return a
	}
	return &RequestAuth{}
}

// "Authorization: TOKEN" header (Bearer prefix is optional).
func extractBearerToken(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	if raw == "" {
		return ""
	}
	// Support both "Bearer <token>" and bare "<token>".
	if after, ok := strings.CutPrefix(raw, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return raw
}
