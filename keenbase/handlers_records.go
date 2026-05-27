package keenbase

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (sb *SimpleBase) handleListRecords(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}

	if err := sb.checkRule(w, r, col, RuleList, nil); err != nil {
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

	if fields := q.Get("fields"); fields != "" {
		result.Items = projectRecords(result.Items, splitFields(fields))
	}

	writeJSON(w, http.StatusOK, result)
}

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

	if err := sb.checkRule(w, r, col, RuleView, rec); err != nil {
		return
	}

	if fields := r.URL.Query().Get("fields"); fields != "" {
		rec = projectRecord(rec, splitFields(fields))
	}

	writeJSON(w, http.StatusOK, rec)
}

func (sb *SimpleBase) handleCreateRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	if col.IsView() {
		writeError(w, http.StatusMethodNotAllowed, "View collections are read-only.")
		return
	}

	if err := sb.checkRule(w, r, col, RuleCreate, nil); err != nil {
		return
	}

	data, err := sb.parseRecordBody(r, col)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if provisional, ok := data["__id"].(string); ok {
		delete(data, "__id")
		data["id"] = provisional
	}

	rec, err := sb.records.Create(col, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

func (sb *SimpleBase) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	if col.IsView() {
		writeError(w, http.StatusMethodNotAllowed, "View collections are read-only.")
		return
	}

	id := r.PathValue("id")

	existing, err := sb.records.GetByID(col, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch record.")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		return
	}

	if err := sb.checkRule(w, r, col, RuleUpdate, existing); err != nil {
		return
	}

	data, err := sb.parseRecordBody(r, col)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	delete(data, "__id")

	oldFiles := collectOldFileNames(col, existing, data)

	rec, err := sb.records.Update(col, id, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if rec == nil {
		writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		return
	}

	if sb.files != nil {
		for _, fname := range oldFiles {
			sb.files.DeleteFile(col.ID, id, fname)
		}
	}

	writeJSON(w, http.StatusOK, rec)
}

func (sb *SimpleBase) handleDeleteRecord(w http.ResponseWriter, r *http.Request) {
	col, ok := sb.resolveCollection(w, r)
	if !ok {
		return
	}
	if col.IsView() {
		writeError(w, http.StatusMethodNotAllowed, "View collections are read-only.")
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

	if err := sb.checkRule(w, r, col, RuleDelete, rec); err != nil {
		return
	}

	if err := sb.records.Delete(col, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete record.")
		return
	}

	if sb.files != nil {
		sb.files.DeleteRecordFiles(col.ID, id)
	}

	w.WriteHeader(http.StatusNoContent)
}

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

func projectRecords(records []*Record, fields map[string]bool) []*Record {
	out := make([]*Record, len(records))
	for i, r := range records {
		out[i] = projectRecord(r, fields)
	}
	return out
}

func collectOldFileNames(col *Collection, existing *Record, patch map[string]any) []string {
	var old []string
	for _, f := range col.Fields {
		if f.Type != FieldTypeFile {
			continue
		}
		if _, replacing := patch[f.Name]; !replacing {
			continue // field not in patch — nothing to replace
		}
		current := existing.GetString(f.Name)
		if current == "" {
			continue
		}
		// Single filename or JSON array of filenames.
		if current[0] == '[' {
			var names []string
			if err := json.Unmarshal([]byte(current), &names); err == nil {
				old = append(old, names...)
				continue
			}
		}
		old = append(old, current)
	}
	return old
}
