package keenbase

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// handleListRecords handles GET /api/collections/{name}/records
//
// Query params:
//
//	page      int     page number (default 1)
//	perPage   int     records per page (default 30, max 500)
//	sort      string  e.g. "-created,title"
//	filter    string  e.g. "title ~ 'hello' && active = true"
//	fields    string  comma-separated field names to include in the response
//	skipTotal bool    skip COUNT query for faster responses
func (sb *SimpleBase) handleListRecords(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	params := ListParams{
		Page:      queryInt(q.Get("page"), 1),
		PerPage:   queryInt(q.Get("perPage"), 30),
		Sort:      q.Get("sort"),
		Filter:    q.Get("filter"),
		SkipTotal: q.Get("skipTotal") == "1" || q.Get("skipTotal") == "true",
	}

	result, err := sb.records.List(col, params)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Optional field projection.
	if fields := q.Get("fields"); fields != "" {
		result.Items = projectRecords(result.Items, splitFields(fields))
	}

	writeJSON(w, http.StatusOK, result)
}

// handleViewRecord handles GET /api/collections/{name}/records/{id}
func (sb *SimpleBase) handleViewRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")
	rec, err := sb.records.GetByID(col, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch record.")
		return
	}
	if rec == nil {
		writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		return
	}

	if fields := r.URL.Query().Get("fields"); fields != "" {
		rec = projectRecord(rec, splitFields(fields))
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleCreateRecord handles POST /api/collections/{name}/records
func (sb *SimpleBase) handleCreateRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	rec, err := sb.records.Create(col, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleUpdateRecord handles PATCH /api/collections/{name}/records/{id}
func (sb *SimpleBase) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	rec, err := sb.records.Update(col, id, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if rec == nil {
		writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleDeleteRecord handles DELETE /api/collections/{name}/records/{id}
func (sb *SimpleBase) handleDeleteRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	id := r.PathValue("id")

	// Verify the record exists before deleting.
	rec, err := sb.records.GetByID(col, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch record.")
		return
	}
	if rec == nil {
		writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		return
	}

	if err := sb.records.Delete(col, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete record.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- helpers

// queryInt parses an integer query param, returning fallback on missing/invalid.
func queryInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

// splitFields splits a comma-separated fields string into a set for O(1) lookup.
func splitFields(fields string) map[string]bool {
	set := map[string]bool{}
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			set[f] = true
		}
	}
	return set
}

// projectRecord returns a copy of the record with only the requested fields.
// System fields (id, collectionId, collectionName, created, updated) are
// always included regardless of the fields parameter.
func projectRecord(rec *Record, fields map[string]bool) *Record {
	projected := &Record{
		id:             rec.id,
		collectionID:   rec.collectionID,
		collectionName: rec.collectionName,
		created:        rec.created,
		updated:        rec.updated,
		data:           make(map[string]any),
	}
	for k, v := range rec.data {
		if fields[k] {
			projected.data[k] = v
		}
	}
	return projected
}

// projectRecords applies projectRecord to every record in a slice.
func projectRecords(records []*Record, fields map[string]bool) []*Record {
	out := make([]*Record, len(records))
	for i, r := range records {
		out[i] = projectRecord(r, fields)
	}
	return out
}
