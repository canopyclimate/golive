package changeset

import (
	"fmt"
	"net/url"
	"testing"
)

type Person struct {
	First string `validate:"min=4"`
	Last  string `validate:"min=2"`
}

func TestChangeset(t *testing.T) {

	// test changeset
	for _, data := range []url.Values{
		// test case 1
		{
			"First": []string{"fi"},
			"Last":  []string{""},
		},
		// test case 2
		{
			"First": []string{"firs"},
			"Last":  []string{"a"},
		},
		// test case 3
		{
			"First": []string{"firs"},
			"Last":  []string{"aa"},
		},
	} {
		gv := NewGoPlaygroundChangesetConfig()
		cc := NewConfig(gv, gv)
		cs := cc.NewChangeset(
			url.Values{}, // empty "old" data
			data,         // "new" data
			"action",     // must be non-empty to run validations
			&Person{},
		)

		fmt.Println("Valid?", cs.Valid)
		fmt.Println("Errors:", cs.Errors)
		// fmt.Println("w/Statics", try.E1(strconv.Unquote(string(out))))
		// fmt.Println("w/o Statics", try.E1(strconv.Unquote(string(wout))))
		// fmt.Println("----")
	}
}
