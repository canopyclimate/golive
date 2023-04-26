package changeset

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/go-playground/form"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

// GoPlaygroundChangesetConfig provides a GoPlayground Validate and Decoder
// based implementation of the Validator and Decoder interfaces.
type GoPlaygroundChangesetConfig struct {
	validator  *validator.Validate
	translator ut.Translator
	decoder    *form.Decoder
}

// NewGoPlaygroundChangesetConfig initializes the decoder and configures the
// validator with translations for the len, lte, and min tags. This is a minimal
// implementation to show how one can use different decoder and validator
// libraries with the changeset package.
func NewGoPlaygroundChangesetConfig() GoPlaygroundChangesetConfig {

	decoder := form.NewDecoder()
	v := validator.New()
	en := en.New()
	uni := ut.New(en, en)
	var t ut.Translator
	var ok bool
	if t, ok = uni.GetTranslator("en"); !ok {
		log.Fatal("could not get translator")
	}

	// register translations
	// translate len tag
	v.RegisterTranslation("len", t,
		func(ut ut.Translator) error {
			return nil
		},
		func(ut ut.Translator, fe validator.FieldError) string {
			len := fe.Param()
			if len == "1" {
				return "must be 1 character"
			}
			return fmt.Sprintf("must be %s characters", len)
		},
	)
	// translate lte tag
	v.RegisterTranslation("lte", t,
		func(ut ut.Translator) error { return nil },
		func(ut ut.Translator, fe validator.FieldError) string {
			lte := fe.Param()
			// Expected use is on int fields.  For Strings use "max" tag
			return fmt.Sprintf("must be at most %v", lte)
		},
	)
	// translate min tag
	v.RegisterTranslation("min", t,
		func(ut ut.Translator) error { return nil },
		func(ut ut.Translator, fe validator.FieldError) string {
			min := fe.Param()
			// Expected use is on String fields.  For numbers use "gte" tag
			if min == "1" {
				return "must be at least 1 character"
			}
			return fmt.Sprintf("must be at least %s characters", min)
		},
	)

	return GoPlaygroundChangesetConfig{
		validator:  v,
		translator: t,
		decoder:    decoder,
	}
}

// Validate runs decodes the URL values to the struct before running
// the validations on the changeset returning a map of errors if validation
// runs successfully or an error if decoding or validation fails.
func (a GoPlaygroundChangesetConfig) Validate(c *Changeset) (map[string]error, error) {

	// decode first
	if err := a.decoder.Decode(c.Struct, c.Values); err != nil {
		return nil, err
	}

	// run validations
	err := a.validator.Struct(c.Struct)
	// if any errors, set Valid to false (which it should already be but doesn't hurt to be defensive)
	if err == nil {
		return nil, nil
	}

	errMap := make(map[string]error)
	// attempt to cast to validator.ValidationErrors and Translate
	if _, ok := err.(validator.ValidationErrors); ok {
		translatedErrors := err.(validator.ValidationErrors).Translate(a.translator)
		// remove prefix from field name
		prefix := c.Type() + "."
		for k, v := range translatedErrors {
			trimmed := strings.TrimPrefix(k, prefix)
			errMap[trimmed] = errors.New(v)
		}
	}
	return errMap, nil
}

// Decode decodes the URL values to the struct.
func (a GoPlaygroundChangesetConfig) Decode(s any, v url.Values) error {
	return a.decoder.Decode(s, v)
}
