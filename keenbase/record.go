package keenbase

import (
	"encoding/json"
	"time"
)

// Record represents a single entry (row) in a collection.
type Record struct {
	id             string
	collectionID   string
	collectionName string
	data           map[string]any
	created        time.Time
	updated        time.Time
}

// NewRecord creates a new empty Record bound to a collection.
func NewRecord(collection *Collection) *Record {
	return &Record{
		collectionID:   collection.ID,
		collectionName: collection.Name,
		data:           make(map[string]any),
	}
}

// --- Identity ---

func (r *Record) ID() string             { return r.id }
func (r *Record) CollectionID() string   { return r.collectionID }
func (r *Record) CollectionName() string { return r.collectionName }
func (r *Record) Created() time.Time     { return r.created }
func (r *Record) Updated() time.Time     { return r.updated }

func (r *Record) SetID(id string)        { r.id = id }
func (r *Record) SetCreated(t time.Time) { r.created = t }
func (r *Record) SetUpdated(t time.Time) { r.updated = t }

// --- Generic field access ---

// Get returns the raw value of a field.
func (r *Record) Get(field string) any {
	return r.data[field]
}

// Set stores a value for a field.
func (r *Record) Set(field string, value any) {
	if r.data == nil {
		r.data = make(map[string]any)
	}
	r.data[field] = value
}

// Unset removes a field value from the record.
func (r *Record) Unset(field string) {
	delete(r.data, field)
}

// --- Typed getters ---

// GetString returns the field value as a string, or "" if missing/wrong type.
func (r *Record) GetString(field string) string {
	v := r.Get(field)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetBool returns the field value as bool, or false if missing/wrong type.
func (r *Record) GetBool(field string) bool {
	v := r.Get(field)
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// GetFloat64 returns the field value as float64, handling common numeric types.
func (r *Record) GetFloat64(field string) float64 {
	v := r.Get(field)
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	}
	return 0
}

// GetInt returns the field value truncated to int.
func (r *Record) GetInt(field string) int {
	return int(r.GetFloat64(field))
}

// GetStringSlice returns the field value as []string.
// Handles both []string and []any (as returned by JSON unmarshalling).
func (r *Record) GetStringSlice(field string) []string {
	v := r.Get(field)
	if v == nil {
		return nil
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	if ai, ok := v.([]any); ok {
		out := make([]string, 0, len(ai))
		for _, item := range ai {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// GetGeoPoint returns the field value as a GeoPoint.
func (r *Record) GetGeoPoint(field string) GeoPoint {
	v := r.Get(field)
	if v == nil {
		return GeoPoint{}
	}
	if gp, ok := v.(GeoPoint); ok {
		return gp
	}
	// handle map[string]any as returned by JSON unmarshalling
	if m, ok := v.(map[string]any); ok {
		var gp GeoPoint
		if lon, ok := m["lon"].(float64); ok {
			gp.Lon = lon
		}
		if lat, ok := m["lat"].(float64); ok {
			gp.Lat = lat
		}
		return gp
	}
	return GeoPoint{}
}

// --- Serialisation ---

const timeLayout = "2006-01-02 15:04:05.000Z"

// MarshalJSON serialises the record as a flat JSON object, merging the
// system fields (id, collectionId, collectionName, created, updated) with
// the user-defined data fields at the top level — matching PocketBase's output.
func (r *Record) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(r.data)+5)

	// copy data fields first so system fields always win on key collision
	for k, v := range r.data {
		out[k] = v
	}

	out["id"] = r.id
	out["collectionId"] = r.collectionID
	out["collectionName"] = r.collectionName
	out["created"] = r.created.UTC().Format(timeLayout)
	out["updated"] = r.updated.UTC().Format(timeLayout)

	return json.Marshal(out)
}

// UnmarshalJSON restores a Record from the flat JSON format produced by MarshalJSON.
func (r *Record) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if v, ok := raw["id"].(string); ok {
		r.id = v
		delete(raw, "id")
	}
	if v, ok := raw["collectionId"].(string); ok {
		r.collectionID = v
		delete(raw, "collectionId")
	}
	if v, ok := raw["collectionName"].(string); ok {
		r.collectionName = v
		delete(raw, "collectionName")
	}
	if v, ok := raw["created"].(string); ok {
		if t, err := time.Parse(timeLayout, v); err == nil {
			r.created = t
		}
		delete(raw, "created")
	}
	if v, ok := raw["updated"].(string); ok {
		if t, err := time.Parse(timeLayout, v); err == nil {
			r.updated = t
		}
		delete(raw, "updated")
	}

	r.data = raw
	return nil
}
