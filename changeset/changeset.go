package changeset

import (
	"errors"
	"net/url"
	"reflect"

	"golang.org/x/exp/slices"
)

// Validator validates a changeset and returns a map of field name to error message.
type Validator interface {
	// Validate validates the changeset and returns a map of field name to errors
	// or an error if there was a problem validating the changeset.
	Validate(c *Changeset) (map[string]error, error)
}

// Decoder decodes a url.Values into a struct.
type Decoder interface {
	// Decode decodes the url.Values into the struct returning an error if there was a problem.
	Decode(any, url.Values) error
}

// Config is a configuration for a Changeset providing implementations of Validator and Decoder.
type Config struct {
	Validator
	Decoder
}

// Changeset provides a powerful API for decoding URL values into a struct and
// validating the struct. It provides a way to check if a given struct is valid
// and if not a way to access the errors for each field. A changeset is meant to
// work with HTML form data in concert with the phx-change and phx-submit events.
type Changeset struct {
	Errors  map[string]error // map of field name to error message
	Initial url.Values       // map of initial values
	Changes url.Values       // map of field name that differs from the original value
	Values  url.Values       // map of merged changes and original values
	Struct  any              // pointer to struct to decode into

	action  string          // last update action; only run validations if action is not empty
	touched map[string]bool // map of field names that were touched
	config  *Config
}

// Type returns the name of the struct.
func (c *Changeset) Type() string {
	return reflect.TypeOf(c.Struct).Elem().Name()
}

// Valid returns true if the changeset is valid or false if it is not.
// Valid depends on the last action that was performed on the changeset along
// with the Errors map and whether or not the field was touched.
// If the action is empty, Valid will always return true.
// If the action is not empty, Valid will return true if there are no errors
// or if there are errors but the field was not touched.
func (c *Changeset) Valid() bool {
	if c.action == "" {
		return true
	}
	for k, v := range c.Errors {
		if v != nil && c.touched != nil {
			return !c.touched[k]
		}
	}
	return true
}

// AsStruct returns the changeset decoded into a struct or an error if there was a problem decoding.
func (c *Changeset) AsStruct() (any, error) {
	s := c.Struct
	err := c.config.Decoder.Decode(s, c.Values)
	return s, err
}

// NewChangeset returns a new Changeset based on the provided pointer to a struct.
func (cc *Config) NewChangeset(initial url.Values, obj any) (*Changeset, error) {
	// we need a pointer to a struct to decode into
	if reflect.TypeOf(obj).Kind() != reflect.Ptr || reflect.TypeOf(obj).Elem().Kind() != reflect.Struct {
		return nil, errors.New("changeset: obj must be pointer to struct")
	}

	c := &Changeset{
		Initial: initial,
		Values:  initial,
		Struct:  obj,
		config:  cc,
	}
	return c, nil
}

// Update updates the changeset with new data and action. If action is empty, the changeset
// will always return true for Valid(). Passing a non-empty action will cause the
// changeset to run validations which may change the result of Valid() depending on
// whether or not there are errors and whether or not the field was touched.
func (c *Changeset) Update(newData url.Values, action string) error {
	c.action = action
	// TODO should we call Reset() if newData is nil and action is empty?

	// merge old and new data and calculate changes
	if c.Values == nil {
		c.Values = url.Values{}
	}
	for k, v := range newData {
		if !slices.Equal(c.Values[k], v) {
			if c.Changes == nil {
				c.Changes = url.Values{}
			}
			c.Changes[k] = v
		}
		c.Values[k] = v
	}

	// validate if action is not empty
	if action != "" {
		// if we get a _target field, use it to indicate which fields were touched
		// if not, assume all fields were touched
		if c.touched == nil {
			c.touched = make(map[string]bool)
		}
		target := newData.Get("_target")
		if target != "" {
			c.touched[target] = true
		} else {
			for k := range newData {
				c.touched[k] = true
			}
		}
		errors, err := c.config.Validator.Validate(c)
		if err != nil {
			return err
		}

		c.Errors = errors
	}
	return nil
}

// Reset resets the changeset to its initial state.
func (c *Changeset) Reset(initial url.Values) {
	c.Errors = nil
	c.Changes = nil
	c.Initial = initial
	c.Values = initial
	c.Struct = reflect.New(reflect.TypeOf(c.Struct).Elem()).Interface()

	c.action = ""
	c.touched = nil
}

// Value returns the value for the given key.
func (c *Changeset) Value(key string) string {
	return c.Values.Get(key)
}

// Error returns the error for the given key.
func (c *Changeset) Error(key string) error {
	if c == nil || c.Valid() || c.Errors == nil || c.Errors[key] == nil || c.touched == nil || !c.touched[key] {
		return nil
	}
	return c.Errors[key]
}

// HasError returns true if Error(key) returns a non-nil error
func (c *Changeset) HasError(key string) bool {
	return c.Error(key) != nil
}
