package keenbase

import "time"

// CollectionType distinguishes the three kinds of collections.
type CollectionType string

const (
	// CollectionTypeBase is the default — stores arbitrary app data.
	CollectionTypeBase CollectionType = "base"
	// CollectionTypeAuth adds user identity fields and auth endpoints.
	CollectionTypeAuth CollectionType = "auth"
	// CollectionTypeView is read-only, backed by a raw SQL SELECT query.
	CollectionTypeView CollectionType = "view"
)

// APIRules controls who can perform each CRUD operation on a collection.
//
// The *string pointer has three meaningful states:
//   - nil          → "locked": only superusers can access
//   - ""  (empty)  → public: anyone can access
//   - "expr"       → conditional: access granted when the filter expression is true
type APIRules struct {
	ListRule   *string `json:"listRule"`
	ViewRule   *string `json:"viewRule"`
	CreateRule *string `json:"createRule"`
	UpdateRule *string `json:"updateRule"`
	DeleteRule *string `json:"deleteRule"`
	// ManageRule is only relevant for auth collections.
	// When satisfied, the caller can update another user's sensitive fields
	// (email, password) without needing the old password.
	ManageRule *string `json:"manageRule,omitempty"`
}

// PasswordAuthOptions configures password-based authentication.
type PasswordAuthOptions struct {
	Enabled bool `json:"enabled"`
	// IdentityFields lists which unique fields can be used as the login identity.
	// Defaults to ["email"]. Add "username" to also allow username login.
	IdentityFields []string `json:"identityFields"`
}

// OTPOptions configures one-time password (email code) authentication.
type OTPOptions struct {
	Enabled  bool `json:"enabled"`
	Duration int  `json:"duration"` // how long the OTP is valid, in seconds
	Length   int  `json:"length"`   // number of digits in the OTP
}

// MFAOptions configures multi-factor authentication.
// When enabled, the user must authenticate with any two different auth methods.
type MFAOptions struct {
	Enabled  bool `json:"enabled"`
	Duration int  `json:"duration"` // how long the pending MFA session lives, in seconds
}

// AuthOptions bundles all authentication settings for an Auth collection.
type AuthOptions struct {
	PasswordAuth PasswordAuthOptions `json:"passwordAuth"`
	OTP          OTPOptions          `json:"otp"`
	MFA          MFAOptions          `json:"mfa"`
}

// Collection represents a group of records, backed by a SQLite table.
type Collection struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Type CollectionType `json:"type"`

	// Fields defines the schema — the columns of the underlying table.
	// System fields (id, created, updated) are implicit and not listed here.
	Fields []Field `json:"fields"`

	// Rules controls API-level access for each operation.
	Rules APIRules `json:"rules"`

	// ViewQuery is the SQL SELECT statement used to populate a View collection.
	// Only relevant when Type == CollectionTypeView.
	ViewQuery string `json:"viewQuery,omitempty"`

	// AuthOptions holds authentication settings.
	// Only relevant when Type == CollectionTypeAuth.
	AuthOptions *AuthOptions `json:"authOptions,omitempty"`

	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// IsBase returns true for Base collections.
func (c *Collection) IsBase() bool { return c.Type == CollectionTypeBase }

// IsAuth returns true for Auth collections.
func (c *Collection) IsAuth() bool { return c.Type == CollectionTypeAuth }

// IsView returns true for View collections.
func (c *Collection) IsView() bool { return c.Type == CollectionTypeView }

// FieldByName returns the first field with the given name, or nil if not found.
func (c *Collection) FieldByName(name string) *Field {
	for i := range c.Fields {
		if c.Fields[i].Name == name {
			return &c.Fields[i]
		}
	}
	return nil
}

// FieldByID returns the first field with the given ID, or nil if not found.
func (c *Collection) FieldByID(id string) *Field {
	for i := range c.Fields {
		if c.Fields[i].ID == id {
			return &c.Fields[i]
		}
	}
	return nil
}
