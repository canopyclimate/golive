package changeset

import (
	"net/url"
	"reflect"

	"golang.org/x/exp/slices"
)

// Validator validates a changeset and returns a map of field name to error message.
type Validator interface {
	Validate(c *Changeset) (map[string]error, bool, error)
}

// Decoder decodes a url.Values into a struct.
type Decoder interface {
	Decode(any, url.Values) error
}

// Config is a configuration for a Changeset providing a Validator and Decoder.
type Config struct {
	validator Validator
	decoder   Decoder
}

// NewConfig returns a new Config with the given Validator and Decoder.
func NewConfig(v Validator, d Decoder) *Config {
	return &Config{
		validator: v,
		decoder:   d,
	}
}

// Changeset provides a powerful API for decoding URL values into a struct and
// validating the struct. It provides a way to check if a given struct is valid
// and if not a way to access the errors for each field. A changeset is meant to
// work with HTML form data in concert with the phx-change and phx-submit events.
type Changeset struct {
	Action     string           // only run validations if action is not empty
	Valid      bool             // true if no validation errors or no action (used in form errors)
	Errors     map[string]error // map of field name to error message
	Changes    map[string]any   // map of field name that differs from the original value
	Touched    map[string]bool  // map of field names that were touched
	Values     url.Values       // map of merged changes and original values
	Struct     any              // type of object to decode into
	StructType string           // type name of the struct pointer

	config *Config
}

// AsStruct returns the changeset as a struct or an error if the data could not be decoded into the struct.
func (c *Changeset) AsStruct() (any, error) {
	s := c.Struct
	err := c.config.decoder.Decode(s, c.Values)
	return s, err
}

// NewChangeset returns a new Changeset based on the old data, new data, and action
// Typically this is called to initialize a changeset. If action is empty, the changeset
// will always return true for Valid. Passing a non-empty action will cause the
// changeset to make the validation errors available if the struct is not valid.
func (cc *Config) NewChangeset(old, new url.Values, action string, obj any) (*Changeset, error) {

	c := &Changeset{
		Action:     action,
		Valid:      action == "", // default to true if no action, otherwise false
		Errors:     make(map[string]error),
		Changes:    make(map[string]any),
		Touched:    make(map[string]bool),
		Values:     url.Values{},
		Struct:     obj,                               // TODO check is pointer to struct
		StructType: reflect.TypeOf(obj).Elem().Name(), // TODO better way?
		config:     cc,
	}
	// merge old and new data
	for k, v := range old {
		c.Values[k] = v
	}
	for k, v := range new {
		c.Values[k] = v
	}

	// validate changes
	if action != "" {
		// if we get a _target field, use it to indicate which fields were touched
		// if not, assume all fields were touched
		target := new.Get("_target")
		if target != "" {
			c.Touched[target] = true
		} else {
			for k := range new {
				c.Touched[k] = true
			}
		}
		errors, valid, err := c.config.validator.Validate(c)
		c.Valid = valid
		c.Errors = errors
		return c, err
	}
	// shallow diff to find changes
	for k, v := range new {
		if !slices.Equal(old[k], v) {
			c.Changes[k] = v
		}
	}
	return c, nil
}

// Update updates the changeset with new data and action. If action is empty, the changeset
// will always return true for Valid. Passing a non-empty action will cause the
// changeset to make the validation errors available if the struct is not valid.
func (c *Changeset) Update(newData url.Values, action string) error {
	c.Action = action
	c.Valid = action == ""
	c.Errors = make(map[string]error)

	// merge old and new data and calculate changes
	for k, v := range newData {
		if !slices.Equal(c.Values[k], v) {
			c.Changes[k] = v
		}
		c.Values[k] = v
	}

	// validate if action is not empty
	if action != "" {
		target := newData.Get("_target")
		if target != "" {
			c.Touched[target] = true
		} else {
			for k := range newData {
				c.Touched[k] = true
			}
		}
		errors, valid, err := c.config.validator.Validate(c)
		c.Valid = valid
		c.Errors = errors
		return err
	}
	return nil
}

// Value returns the value for the given key.
func (c *Changeset) Value(key string) string {
	return c.Values.Get(key)
}

// Error returns the error for the given key.
func (c *Changeset) Error(key string) error {
	if c == nil || c.Valid || c.Errors[key] == nil || !c.Touched[key] {
		return nil
	}
	return c.Errors[key]
}

// HasError returns true if the given key has an error.
func (c *Changeset) HasError(key string) bool {
	return c.Error(key) != nil
}
