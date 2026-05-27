package keenbase

import (
	"net/http"
)

func (sb *SimpleBase) handleServeFile(w http.ResponseWriter, r *http.Request) {
	collectionID := r.PathValue("collectionID")
	recordID := r.PathValue("recordID")
	filename := r.PathValue("filename")
	thumb := r.URL.Query().Get("thumb")

	col, err := sb.collections.GetByID(collectionID)
	if err != nil || col == nil {
		http.NotFound(w, r)
		return
	}

	if f := matchFileField(col, filename); f != nil && f.Options.Protected {
		auth := authFromContext(r.Context())
		if auth.IsGuest() {
			writeError(w, http.StatusUnauthorized, "The request requires valid authorization token.")
			return
		}
	}

	sb.files.ServeFile(w, r, collectionID, recordID, filename, thumb)
}

func matchFileField(col *Collection, filename string) *Field {
	for i := range col.Fields {
		f := &col.Fields[i]
		if f.Type != FieldTypeFile {
			continue
		}
		prefix := f.Name + "_"
		if len(filename) > len(prefix) && filename[:len(prefix)] == prefix {
			return f
		}
	}
	return nil
}
