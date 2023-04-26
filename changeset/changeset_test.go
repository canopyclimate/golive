package changeset

import (
	"net/url"
	"testing"
)

type Person struct {
	First string `validate:"min=4"`
	Last  string `validate:"min=2"`
}

type TestCase struct {
	Values            url.Values
	Target            string
	ExpectedValid     bool
	ExpectedErrorKeys []string
}

func TestNonPointerChangeset(t *testing.T) {
	gv := NewGoPlaygroundChangesetConfig()
	cc := Config{
		Validator: gv,
		Decoder:   gv,
	}

	_, err := cc.NewChangeset(Person{})
	if err == nil {
		t.Error("Expected error when passing a non-pointer to NewChangeset")
	}
}

func TestChangeset(t *testing.T) {

	gv := NewGoPlaygroundChangesetConfig()
	cc := Config{
		Validator: gv,
		Decoder:   gv,
	}

	// test changeset
	for _, tc := range []TestCase{
		{
			// no target so touching all and both are invalid
			Values: url.Values{
				"First": []string{"fi"},
				"Last":  []string{""},
			},
			ExpectedValid: false,
			ExpectedErrorKeys: []string{
				"First",
				"Last",
			},
		},
		{
			// targeting last which is invalid
			Values: url.Values{
				"First":   []string{"firs"},
				"Last":    []string{"a"},
				"_target": []string{"Last"},
			},
			ExpectedValid: false,
			ExpectedErrorKeys: []string{
				"Last",
			},
		},
		{
			// targeting first which is valid
			Values: url.Values{
				"First":   []string{"firs"},
				"Last":    []string{""},
				"_target": []string{"First"},
			},
			ExpectedValid: true,
		},
		{
			// both valid inputs targeting all
			Values: url.Values{
				"First": []string{"firs"},
				"Last":  []string{"aa"},
			},
			ExpectedValid: true,
		},
	} {

		cs, err := cc.NewChangeset(&Person{})
		if err != nil {
			t.Errorf("Unexpected error from NewChangeset: %s", err)
		}
		cs.Update(tc.Values, "update")

		if tc.ExpectedValid != cs.Valid() {
			t.Errorf("Expected Valid to be %v, got %v", tc.ExpectedValid, cs.Valid())
		}
		if len(tc.ExpectedErrorKeys) > 0 {
			for _, k := range tc.ExpectedErrorKeys {
				if e := cs.Error(k); e == nil {
					t.Errorf("Expected error key %s to be present", k)
				}
			}
		}
		for k, v := range tc.Values {
			if k != "_target" {
				if cs.Value(k) != v[0] {
					t.Errorf("Expected value for key %s to be %s, got %s", k, v[0], cs.Value(k))
				}
			}
		}
		s := cs.Struct.(*Person)
		if s.First != tc.Values.Get("First") {
			t.Errorf("Expected First to be %s, got %s", tc.Values.Get("First"), s.First)
		}
		if s.Last != tc.Values.Get("Last") {
			t.Errorf("Expected Last to be %s, got %s", tc.Values.Get("Last"), s.Last)
		}
	}
}

func TestReset(t *testing.T) {
	gv := NewGoPlaygroundChangesetConfig()
	cc := Config{
		Validator: gv,
		Decoder:   gv,
	}

	cs, err := cc.NewChangeset(&Person{})
	if err != nil {
		t.Errorf("Unexpected error from NewChangeset: %s", err)
	}
	cs.Update(url.Values{
		"First": []string{"fi"},
		"Last":  []string{"a"},
	}, "update")

	if cs.Valid() {
		t.Errorf("Expected Valid to be false, got %v", cs.Valid())
	}
	if cs.Struct.(*Person).First != "fi" {
		t.Errorf("Expected First to be set, got %s", cs.Struct.(*Person).First)
	}

	cs.Reset()

	if !cs.Valid() {
		t.Errorf("Expected Valid to be true, got %v", cs.Valid())
	}
	s, err := cs.AsStruct()
	if err != nil {
		t.Errorf("Unexpected error from AsStruct: %s", err)
	}
	if s.(*Person).First != "" {
		t.Errorf("Expected First to be empty, got %s", s.(*Person).First)
	}
}
