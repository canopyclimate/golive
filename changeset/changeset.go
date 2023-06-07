package changeset

import (
	"net/url"
	"reflect"

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

// Changeset provides a powerful API for decoding URL values into a struct and
// validating the struct. It provides a way to check if a given struct is valid
// and if not a way to access the errors for each field. A changeset is meant to
// work with HTML form data in concert with the phx-change and phx-submit events.
type Changeset struct {
	Errors  map[string]error // map of field name to error message
	Initial url.Values       // map of initial values
	Changes url.Values       // map of field name that differs from the original value
	Values  url.Values       // map of merged changes and original values

	ptr     any             // pointer to struct type for changeset
	action  string          // last update action; only run validations if action is not empty
	touched map[string]bool // map of field names that were touched
	config  *Config
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
	// if nothing was touched the changeset is valid
	// regardless of whether or not there are errors
	// otherwise, only check for errors on touched fields
	// and return false if there are any errors
	for k, touched := range c.touched {
		if touched && c.Errors[k] != nil {
			return false
		}
	}
	return true
}

// Struct returns the changeset Values decoded into a struct of T or an error if there was a problem decoding.
func (c *Changeset) Struct() (any, error) {
	t := reflect.New(reflect.TypeOf(c.ptr).Elem()).Interface()
	err := c.config.Decoder.Decode(t, c.Values)
	return t, err
}

// New returns a new Changeset of type T using the Config and initilizes the Changeset with the given initial values.
func New[T any](cc *Config, initial url.Values) *Changeset {
	s := reflect.New(reflect.TypeOf(new(T)).Elem()).Interface()
	c := &Changeset{
		Initial: initial,
		Values:  initial,
		ptr:     s,
		config:  cc,
	}
	return c
}

// Update updates the changeset with new data and action. If action is empty, the changeset
// will always return true for Valid(). Passing a non-empty action will cause the
// changeset to run validations which may change the result of Valid() depending on
// whether or not there are errors and whether or not the field was touched.
func (c *Changeset) Update(newData url.Values, action string) error {
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
		// if we get a _target field, use it to indicate which fields were touched
		// if not, assume all fields were touched
		if c.touched == nil {
			c.touched = make(map[string]bool)
		}
		if target != "" {
			c.touched[target] = true
		} else {
			for k := range newData {
				c.touched[k] = true
			}
		}
		t := reflect.New(reflect.TypeOf(c.ptr).Elem()).Interface()
		errors, err := c.config.Validator.Validate(t, c.Values)
		if err != nil {
			return err
		}

		c.Errors = errors
	}
	return nil
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
