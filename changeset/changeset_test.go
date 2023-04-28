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

var gv = NewGoPlaygroundChangesetConfig()
var cc = Config{
	Validator: gv,
	Decoder:   gv,
}

func TestNonPointerChangeset(t *testing.T) {
	_, err := cc.NewChangeset(nil, Person{})
	if err == nil {
		t.Error("Expected error when passing a non-pointer to NewChangeset")
	}
}

func TestChangeset(t *testing.T) {
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

		cs, err := cc.NewChangeset(nil, &Person{})
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
	cs, err := cc.NewChangeset(nil, &Person{})
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

	// reset
	cs.Reset(nil)

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

	// update again
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
}

func expectValuesAndChanges(cs *Changeset, expectMatch bool, t *testing.T) {
	diffLength := len(cs.Changes) != len(cs.Values)
	if expectMatch && diffLength {
		t.Errorf("Expected Changes and Values match=%v, got %d and %d", expectMatch, len(cs.Changes), len(cs.Values))
	}
	if !expectMatch && !diffLength {
		t.Errorf("Expected Changes and Values match=%v, got %d and %d", expectMatch, len(cs.Changes), len(cs.Values))
	}
	for k, v := range cs.Changes {
		if expectMatch && len(v) != len(cs.Values[k]) {
			t.Errorf("Expected Changes to have same number of entries, got %v", cs.Changes)
		}
		for i, s := range v {
			if s != cs.Values[k][i] {
				t.Errorf("Expected matching entries for key %s, got %s and %s", k, s, cs.Values[k][i])
			}
		}
	}
}

func TestChangesOnEmptyInit(t *testing.T) {
	cs, err := cc.NewChangeset(nil, &Person{})
	if err != nil {
		t.Errorf("Unexpected error from NewChangeset: %s", err)
	}
	expectValuesAndChanges(cs, true, t)

	cs.Update(url.Values{
		"First": []string{"fi"},
	}, "update")
	expectValuesAndChanges(cs, true, t)

	cs.Update(url.Values{
		"First": []string{"firs"},
	}, "update")
	expectValuesAndChanges(cs, true, t)
}

func TestChangesOnNonEmptyInit(t *testing.T) {
	cs, err := cc.NewChangeset(url.Values{
		"First": []string{"firs"},
		"Last":  []string{"last"},
	}, &Person{})
	if err != nil {
		t.Errorf("Unexpected error from NewChangeset: %s", err)
	}
	expectValuesAndChanges(cs, false, t)

	cs.Update(url.Values{
		"First": []string{"f"},
	}, "update")
	expectValuesAndChanges(cs, false, t)

	cs.Update(url.Values{
		"First": []string{"fi"},
	}, "update")
	expectValuesAndChanges(cs, false, t)

	cs.Update(url.Values{
		"First": []string{"fir"},
		"Last":  []string{"l"},
	}, "update")
	expectValuesAndChanges(cs, true, t)
}

func TestIsStructPtr(t *testing.T) {
	_, err := cc.NewChangeset(nil, Person{})
	if err == nil {
		t.Errorf("Expected error from NewChangeset")
	}

	s := "not a struct ptr"
	_, err = cc.NewChangeset(nil, &s)
	if err == nil {
		t.Errorf("Expected error from NewChangeset")
	}
}

func TestInitial(t *testing.T) {
	cs, err := cc.NewChangeset(nil, &Person{})
	if err != nil {
		t.Errorf("Unexpected error from NewChangeset: %s", err)
	}
	if cs.Initial != nil {
		t.Errorf("Expected Init to be nil")
	}
	cs, err = cc.NewChangeset(url.Values{"First": []string{"firs"}}, &Person{})
	if err != nil {
		t.Errorf("Unexpected error from NewChangeset: %s", err)
	}
	if cs.Initial == nil {
		t.Errorf("Expected Init to be non-nil")
	}
	if cs.Initial.Get("First") != "firs" {
		t.Errorf("Expected Init to match, got %s", cs.Initial.Get("First"))
	}
}
