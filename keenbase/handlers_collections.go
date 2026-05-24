package keenbase

import (
	"encoding/json"
	"net/http"
)

// handleListCollections returns all collections.
//
//	GET /api/collections
func (sb *SimpleBase) handleListCollections(w http.ResponseWriter, r *http.Request) {
	cols, err := sb.collections.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch collections.")
		return
	}
	writeJSON(w, http.StatusOK, cols)
}

// handleViewCollection returns a single collection by name or ID.
//
//	GET /api/collections/{name}
func (sb *SimpleBase) handleViewCollection(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, col)
}

// handleCreateCollection creates a new collection and its underlying table.
//
//	POST /api/collections
func (sb *SimpleBase) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string         `json:"name"`
		Type        CollectionType `json:"type"`
		Fields      []Field        `json:"fields"`
		ListRule    *string        `json:"listRule"`
		ViewRule    *string        `json:"viewRule"`
		CreateRule  *string        `json:"createRule"`
		UpdateRule  *string        `json:"updateRule"`
		DeleteRule  *string        `json:"deleteRule"`
		ManageRule  *string        `json:"manageRule"`
		ViewQuery   string         `json:"viewQuery"`
		AuthOptions *AuthOptions   `json:"authOptions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	if body.Type == "" {
		body.Type = CollectionTypeBase
	}
	if body.Fields == nil {
		body.Fields = []Field{}
	}

	col := &Collection{
		Name:        body.Name,
		Type:        body.Type,
		Fields:      body.Fields,
		ViewQuery:   body.ViewQuery,
		AuthOptions: body.AuthOptions,
		Rules: APIRules{
			ListRule:   body.ListRule,
			ViewRule:   body.ViewRule,
			CreateRule: body.CreateRule,
			UpdateRule: body.UpdateRule,
			DeleteRule: body.DeleteRule,
			ManageRule: body.ManageRule,
		},
	}

	if err := sb.collections.Create(col); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, col)
}

// handleUpdateCollection applies a partial update to an existing collection.
//
//	PATCH /api/collections/{name}
//
// Only the keys present in the request body are applied. This is handled by
// decoding into a raw map first, then selectively patching the collection.
func (sb *SimpleBase) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	// Decode into a raw map so we can distinguish "key not sent" from
	// "key explicitly set to null" — important for *string rule fields.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	// Helper: unmarshal a raw value into dst if the key is present.
	patch := func(key string, dst any) {
		if v, ok := raw[key]; ok {
			_ = json.Unmarshal(v, dst)
		}
	}

	patch("name", &col.Name)
	patch("fields", &col.Fields)
	patch("viewQuery", &col.ViewQuery)
	patch("authOptions", &col.AuthOptions)
	patch("listRule", &col.Rules.ListRule)
	patch("viewRule", &col.Rules.ViewRule)
	patch("createRule", &col.Rules.CreateRule)
	patch("updateRule", &col.Rules.UpdateRule)
	patch("deleteRule", &col.Rules.DeleteRule)
	patch("manageRule", &col.Rules.ManageRule)

	if col.Fields == nil {
		col.Fields = []Field{}
	}

	if err := sb.collections.Update(col); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, col)
}

// handleDeleteCollection deletes a collection and drops its underlying table.
//
//	DELETE /api/collections/{name}
func (sb *SimpleBase) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	if err := sb.collections.Delete(col); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// resolveCollection looks up a collection by the {name} path value,
// writes a 404 on miss, and returns (collection, true) on hit.
func (sb *SimpleBase) resolveCollection(w http.ResponseWriter, r *http.Request) (*Collection, bool) {
	name := r.PathValue("name")
	col, err := sb.collections.GetByName(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch collection.")
		return nil, false
	}
	if col == nil {
		writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		return nil, false
	}
	return col, true
}
