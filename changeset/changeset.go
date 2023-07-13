package changeset

import (
	"net/url"

	"golang.org/x/exp/slices"
)

// Validator validates url.Values for a give struct and returns a map of field name to error message.
type Validator interface {
	// Validate validates that the provided URL value are valid for the given struct.
	// It returns a map of field name to error message for each field that is invalid
	// or an error if there was a general error validating.
	Validate(any, url.Values) (map[string]error, error)
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

// Any represents the interface that all changeset structs must implement without the typing constraint.
type Any interface {
	Value(string) string
	Error(string) error
	AddError(string, error)
	RemoveError(string) error
	HasError(string) bool
}

// Changeset provides a powerful API for decoding URL values into a struct and
// validating the struct. It provides a way to check if a given struct is valid
// and if not a way to access the errors for each field. A changeset is meant to
// work with HTML form data in concert with the phx-change and phx-submit events.
type Changeset[T any] struct {
	Initial url.Values // map of initial values
	Changes url.Values // map of field name that differs from the original value
	Values  url.Values // map of merged changes and original values

	errors  map[string]error // map of field name to error message
	action  string           // last update action; only run validations if action is not empty
	touched map[string]bool  // map of field names that were touched
	config  *Config
}

// Valid returns true if the changeset is valid or false if it is not.
// Valid depends on the last action that was performed on the changeset along
// with the Errors map and whether or not the field was touched.
// If the action is empty, Valid will always return true.
// If the action is not empty, Valid will return true if there are no errors
// or if there are errors but the field was not touched.
func (c *Changeset[T]) Valid() bool {
	// blank action or nil Errors means valid
	if c.action == "" || c.errors == nil {
		return true
	}

	// if nothing was touched the changeset is valid
	// regardless of whether or not there are errors
	// otherwise, only check for errors on touched fields
	// and return false if there are any errors
	for k, touched := range c.touched {
		if touched && c.errors[k] != nil {
			return false
		}
	}
	return true
}

// Struct returns the changeset Values decoded into a struct of T or an error if there was a problem decoding.
func (c *Changeset[T]) Struct() (*T, error) {
	t := new(T)
	err := c.config.Decoder.Decode(t, c.Values)
	return t, err
}

// New returns a new Changeset of type T using the Config and initilizes the Changeset with the given initial values.
func New[T any](cc *Config, initial url.Values) *Changeset[T] {
	c := &Changeset[T]{
		Initial: initial,
		Values:  initial,
		config:  cc,
	}
	return c
}

// Update updates the changeset with new data and action. If action is empty, the changeset
// will always return true for Valid(). Passing a non-empty action will cause the
// changeset to run validations which may change the result of Valid() depending on
// whether or not there are errors and whether or not the field was touched.
func (c *Changeset[T]) Update(newData url.Values, action string) error {
	c.action = action

	// initialize Values if nil
	if c.Values == nil {
		c.Values = url.Values{}
	}

	// merge old and new data and calculate changes
	for k, v := range newData {
		if !slices.Equal(c.Values[k], v) {
			if c.Changes == nil {
				c.Changes = url.Values{}
			}
			c.Changes[k] = v
		}
		c.Values[k] = v
	}
	// handle case where _target is set but the newData does not contain the _target field
	// this happens in the case of a checkbox that is unchecked
	target := newData.Get("_target")
	if target != "" && newData.Get(target) == "" {
		c.Values.Del(target)
		if c.Changes == nil {
			c.Changes = url.Values{}
		}
		c.Changes[target] = []string{""}
	}

	// validate if action is not empty
	if action != "" {
		t := new(T)
		var err error
		c.errors, err = c.config.Validator.Validate(t, c.Values)
		if err != nil {
			return err
		}

		if c.touched == nil {
			c.touched = make(map[string]bool)
		}
		// if we get a _target field in the form, use it to indicate which fields were touched
		// if not, assume all fields in input were touched and all fields
		// with errors were touched
		if target != "" {
			c.touched[target] = true
		} else {
			for d := range newData {
				c.touched[d] = true
			}
			for k := range c.errors {
				c.touched[k] = true
			}
		}
	}
	return nil
}

// Value returns the value for the given key.
func (c *Changeset[T]) Value(key string) string {
	if c == nil || c.Values == nil {
		return ""
	}
	return c.Values.Get(key)
}

// Error returns the error for the given key.
func (c *Changeset[T]) Error(key string) error {
	if c == nil || c.errors == nil || c.errors[key] == nil || c.touched == nil || !c.touched[key] || c.Valid() {
		return nil
	}
	return c.errors[key]
}

// HasError returns true if Error(key) returns a non-nil error
func (c *Changeset[T]) HasError(key string) bool {
	return c.Error(key) != nil
}

// AddError adds an error for the given key and marks the field as touched.
func (c *Changeset[T]) AddError(key string, err error) {
	if c.errors == nil {
		c.errors = make(map[string]error)
	}
	c.errors[key] = err
	if c.touched == nil {
		c.touched = make(map[string]bool)
	}
	c.touched[key] = true
}

// RemoveError removes the error for the given key returning the error at the given key
// or nil if there was no error.
func (c *Changeset[T]) RemoveError(key string) error {
	if c.errors == nil {
		return nil
	}
	e := c.errors[key]
	delete(c.errors, key)
	return e
}

// Errors returns the raw map of errors.
func (c *Changeset[T]) Errors() map[string]error {
	return c.errors
}
