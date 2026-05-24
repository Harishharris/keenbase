package keenbase

// FieldType represents the data type of a collection field.
type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeNumber   FieldType = "number"
	FieldTypeBool     FieldType = "bool"
	FieldTypeEmail    FieldType = "email"
	FieldTypeURL      FieldType = "url"
	FieldTypeEditor   FieldType = "editor" // rich HTML content
	FieldTypeDate     FieldType = "date"
	FieldTypeAutodate FieldType = "autodate" // auto set on create/update
	FieldTypeSelect   FieldType = "select"   // predefined options list
	FieldTypeFile     FieldType = "file"
	FieldTypeRelation FieldType = "relation" // references another collection
	FieldTypeJSON     FieldType = "json"
	FieldTypeGeoPoint FieldType = "geoPoint"
)

// GeoPoint holds geographic coordinates.
type GeoPoint struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

// FieldOptions holds the configuration for a field.
// Not every option applies to every field type — only the relevant ones are used.
type FieldOptions struct {
	// text, editor
	Min                 *int   `json:"min,omitempty"`                 // min length
	Max                 *int   `json:"max,omitempty"`                 // max length
	Pattern             string `json:"pattern,omitempty"`             // regex pattern
	AutogeneratePattern string `json:"autogeneratePattern,omitempty"` // auto-fill pattern

	// number
	NoDecimal bool `json:"noDecimal,omitempty"` // disallow decimals

	// select, file, relation — controls single vs multi value
	MaxSelect *int `json:"maxSelect,omitempty"` // nil or 1 = single, >=2 = multi

	// select
	Values []string `json:"values,omitempty"` // allowed option values

	// file
	MimeTypes []string `json:"mimeTypes,omitempty"` // allowed MIME types, empty = any
	Thumbs    []string `json:"thumbs,omitempty"`    // thumbnail sizes, e.g. "100x100"
	Protected bool     `json:"protected,omitempty"` // require a file token to access

	// relation
	CollectionRef string `json:"collectionRef,omitempty"` // target collection ID or name
	CascadeDelete bool   `json:"cascadeDelete,omitempty"` // delete record when related is deleted

	// autodate
	OnCreate bool `json:"onCreate,omitempty"` // set value on record create
	OnUpdate bool `json:"onUpdate,omitempty"` // set value on record update
}

// Field defines a single column in a collection.
type Field struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     FieldType    `json:"type"`
	Required bool         `json:"required"`
	Options  FieldOptions `json:"options"`
}
