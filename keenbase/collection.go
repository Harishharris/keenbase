package keenbase

import "time"

type CollectionType string

const (
	CollectionTypeBase CollectionType = "base"

	CollectionTypeAuth CollectionType = "auth"

	CollectionTypeView CollectionType = "view"
)

type APIRules struct {
	ListRule   *string `json:"listRule"`
	ViewRule   *string `json:"viewRule"`
	CreateRule *string `json:"createRule"`
	UpdateRule *string `json:"updateRule"`
	DeleteRule *string `json:"deleteRule"`

	ManageRule *string `json:"manageRule,omitempty"`
}

type PasswordAuthOptions struct {
	Enabled bool `json:"enabled"`

	IdentityFields []string `json:"identityFields"`
}

type OTPOptions struct {
	Enabled  bool `json:"enabled"`
	Duration int  `json:"duration"` // how long the OTP is valid, in seconds
	Length   int  `json:"length"`   // number of digits in the OTP
}

type MFAOptions struct {
	Enabled  bool `json:"enabled"`
	Duration int  `json:"duration"` // how long the pending MFA session lives, in seconds
}

type AuthOptions struct {
	PasswordAuth PasswordAuthOptions `json:"passwordAuth"`
	OTP          OTPOptions          `json:"otp"`
	MFA          MFAOptions          `json:"mfa"`
}

type Collection struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Type CollectionType `json:"type"`

	Fields []Field `json:"fields"`

	Rules APIRules `json:"rules"`

	ViewQuery string `json:"viewQuery,omitempty"`

	AuthOptions *AuthOptions `json:"authOptions,omitempty"`

	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

func (c *Collection) IsBase() bool { return c.Type == CollectionTypeBase }

func (c *Collection) IsAuth() bool { return c.Type == CollectionTypeAuth }

func (c *Collection) IsView() bool { return c.Type == CollectionTypeView }

func (c *Collection) FieldByName(name string) *Field {
	for i := range c.Fields {
		if c.Fields[i].Name == name {
			return &c.Fields[i]
		}
	}
	return nil
}

func (c *Collection) FieldByID(id string) *Field {
	for i := range c.Fields {
		if c.Fields[i].ID == id {
			return &c.Fields[i]
		}
	}
	return nil
}
