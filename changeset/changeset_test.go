package changeset

import (
	"net/url"
	"testing"
)

type Person struct {
	First string `validate:"min=4"`
	Last  string `validate:"min=2"`
}

var (
	gv = NewGoPlaygroundChangesetConfig()
	cc = Config{
		Validator: gv,
		Decoder:   gv,
	}
)

func TestChangeset(t *testing.T) {
	type TestCase struct {
		Init              url.Values
		Update            url.Values
		Target            string
		ExpectedValid     bool
		ExpectedErrorKeys []string
	}
	// test changeset
	for i, tc := range []TestCase{
		{
			// no target so touching all and both are invalid
			Update: url.Values{
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
			Update: url.Values{
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
			Update: url.Values{
				"First":   []string{"firs"},
				"Last":    []string{""},
				"_target": []string{"First"},
			},
			ExpectedValid: true,
		},
		{
			// both valid inputs targeting all
			Update: url.Values{
				"First": []string{"firs"},
				"Last":  []string{"aa"},
			},
			ExpectedValid: true,
		},
		// go from valid init to invalid update
		{
			Init: url.Values{
				"First": []string{"firs"},
				"Last":  []string{"aa"},
			},
			Update: url.Values{
				"First":   []string{"f"},
				"Last":    []string{"aa"},
				"_target": []string{"First"},
			},
			ExpectedValid: false,
			ExpectedErrorKeys: []string{
				"First",
			},
		},
	} {

		cs := New[Person](&cc, tc.Init)
		if tc.Update != nil {
			cs.Update(tc.Update, "update")
		}

		if tc.ExpectedValid != cs.Valid() {
			t.Errorf("Expected Valid to be %v, got %v in test case %d", tc.ExpectedValid, cs.Valid(), i)
		}
		if len(tc.ExpectedErrorKeys) > 0 {
			for _, k := range tc.ExpectedErrorKeys {
				if e := cs.Error(k); e == nil {
					t.Errorf("Expected error key %s to be present in test case %d", k, i)
				}
			}
		}
		for k, v := range tc.Update {
			if k != "_target" {
				if cs.Value(k) != v[0] {
					t.Errorf("Expected value for key %s to be %s, got %s in test case %d", k, v[0], cs.Value(k), i)
				}
			}
		}
		s, err := cs.Struct()
		if err != nil {
			t.Errorf("Expected no error, got %v in test case %d", err, i)
		}
		p := s.(*Person)
		if p.First != tc.Update.Get("First") {
			t.Errorf("Expected First to be %s, got %s in test case %d", tc.Update.Get("First"), p.First, i)
		}
		if p.Last != tc.Update.Get("Last") {
			t.Errorf("Expected Last to be %s, got %s in test case %d", tc.Update.Get("Last"), p.Last, i)
		}
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
	cs := New[Person](&cc, nil)
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
	cs := New[Person](&cc, url.Values{
		"First": []string{"firs"},
		"Last":  []string{"last"},
	})
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

func TestDeterministicValidity(t *testing.T) {
	cs := New[Person](&cc, nil)
	if !cs.Valid() {
		t.Errorf("Expected Valid to be true")
	}
	// kind of hacky, but ran into issue ranging over map of Errors
	// being non-deterministic causing cs.Valid() to return true
	// when it should have been false - switched the implementation
	// of Valid() but kept this test anyway
	for i := 0; i < 10000; i++ {
		cs.Update(url.Values{
			"First":   []string{"fi"},
			"Last":    []string{""},
			"_target": []string{"First"},
		}, "update")
		if cs.Valid() {
			t.Errorf("Expected Valid to be false")
			break
		}
	}
}

func TestInitial(t *testing.T) {
	cs := New[Person](&cc, nil)
	if cs.Initial != nil {
		t.Errorf("Expected Init to be nil")
	}
	cs = New[Person](&cc, url.Values{"First": []string{"firs"}})
	if cs.Initial == nil {
		t.Errorf("Expected Init to be non-nil")
	}
	if cs.Initial.Get("First") != "firs" {
		t.Errorf("Expected Init to match, got %s", cs.Initial.Get("First"))
	}
}

func TestStruct(t *testing.T) {
	cs := New[Person](&cc, nil)
	if _, err := cs.Struct(); err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
}

type FormWithBool struct {
	Flag bool
}

func TestBoolUnset(t *testing.T) {
	cs := New[FormWithBool](&cc, url.Values{})
	form, err := cs.Struct()
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	if form.(*FormWithBool).Flag {
		t.Errorf("Expected Flag to be false, got %v", true)
	}

	cs.Update(url.Values{
		"Flag": []string{"on"},
	}, "update")

	form, err = cs.Struct()
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	if !form.(*FormWithBool).Flag {
		t.Errorf("Expected Flag to be true, got %v", false)
	}

	cs.Update(url.Values{
		"_target": []string{"Flag"},
	}, "update")

	form, err = cs.Struct()
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	if form.(*FormWithBool).Flag {
		t.Errorf("Expected Flag to be false, got %v", true)
	}
}

func TestBoolSet(t *testing.T) {
	cs := New[FormWithBool](&cc, url.Values{
		"Flag": []string{"on"},
	})

	form, err := cs.Struct()
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	if !form.(*FormWithBool).Flag {
		t.Errorf("Expected Flag to be true, got %v", false)
	}

	cs.Update(url.Values{
		"_target": []string{"Flag"},
	}, "update")

	form, err = cs.Struct()
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	if form.(*FormWithBool).Flag {
		t.Errorf("Expected Flag to be false, got %v", true)
	}

	cs.Update(url.Values{
		"Flag": []string{"on"},
	}, "update")

	form, err = cs.Struct()
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	if !form.(*FormWithBool).Flag {
		t.Errorf("Expected Flag to be true, got %v", false)
	}
}
