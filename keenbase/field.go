package keenbase

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

type GeoPoint struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

type FieldOptions struct {
	Min                 *int   `json:"min,omitempty"`                 // min length
	Max                 *int   `json:"max,omitempty"`                 // max length
	Pattern             string `json:"pattern,omitempty"`             // regex pattern
	AutogeneratePattern string `json:"autogeneratePattern,omitempty"` // auto-fill pattern

	NoDecimal bool `json:"noDecimal,omitempty"` // disallow decimals

	MaxSelect *int `json:"maxSelect,omitempty"` // nil or 1 = single, >=2 = multi

	Values []string `json:"values,omitempty"` // allowed option values

	MimeTypes []string `json:"mimeTypes,omitempty"` // allowed MIME types, empty = any
	Thumbs    []string `json:"thumbs,omitempty"`    // thumbnail sizes, e.g. "100x100"
	Protected bool     `json:"protected,omitempty"` // require a file token to access

	CollectionRef string `json:"collectionRef,omitempty"` // target collection ID or name
	CascadeDelete bool   `json:"cascadeDelete,omitempty"` // delete record when related is deleted

	OnCreate bool `json:"onCreate,omitempty"` // set value on record create
	OnUpdate bool `json:"onUpdate,omitempty"` // set value on record update
}

type Field struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     FieldType    `json:"type"`
	Required bool         `json:"required"`
	Options  FieldOptions `json:"options"`
}
