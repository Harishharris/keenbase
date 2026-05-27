package keenbase

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

func (sb *SimpleBase) loadAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractBearerToken(r)

		if tokenStr == "" {
			// No token — continue as guest.
			next.ServeHTTP(w, r.WithContext(contextWithAuth(r.Context(), &RequestAuth{})))
			return
		}

		auth, err := sb.resolveToken(tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "The request requires valid authorization token.")
			return
		}

		next.ServeHTTP(w, r.WithContext(contextWithAuth(r.Context(), auth)))
	})
}

func (sb *SimpleBase) resolveToken(tokenStr string) (*RequestAuth, error) {

	unverified, err := peekClaims(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("malformed token: %w", err)
	}

	collectionID := unverified.CollectionID
	recordID := unverified.Subject

	col, err := sb.collections.GetByID(collectionID)
	if err != nil || col == nil {
		return nil, fmt.Errorf("unknown collection in token")
	}

	tokenKey, err := sb.loadTokenKey(col, recordID)
	if err != nil || tokenKey == "" {
		return nil, fmt.Errorf("could not load token secret")
	}

	if _, err := parseAuthToken(tokenStr, tokenKey); err != nil {
		return nil, err
	}

	isSuperuser := collectionID == SuperusersCollectionID

	return &RequestAuth{
		RecordID:     recordID,
		CollectionID: collectionID,
		IsSuperuser:  isSuperuser,
	}, nil
}

func (sb *SimpleBase) loadTokenKey(col *Collection, recordID string) (string, error) {
	query := fmt.Sprintf(`SELECT "tokenKey" FROM %q WHERE "id" = ?`, col.Name)
	var tokenKey string
	err := sb.db.QueryRow(query, recordID).Scan(&tokenKey)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return tokenKey, err
}

func peekClaims(tokenStr string) (*authClaims, error) {
	parser := jwt.NewParser()
	claims := &authClaims{}
	_, _, err := parser.ParseUnverified(tokenStr, claims)
	if err != nil {
		return nil, err
	}
	if claims.CollectionID == "" || claims.Subject == "" {
		return nil, fmt.Errorf("missing required claims")
	}
	return claims, nil
}
