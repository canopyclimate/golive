package tmpl

import (
	"errors"
	"testing"

	"github.com/canopyclimate/golive/htmltmpl"
	"github.com/canopyclimate/golive/internal/json"
)

func Fuzz(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		x, err := htmltmpl.New("x").Parse(data)
		if err != nil {
			// Junk input.
			return
		}

		// This is just a generic dot with a variety of types.
		dot := map[string]any{
			"I": 1,
			"S": []string{"A", "B"},
			"M": map[string]int{"A": 2, "B": 4},
			"N": []any{"A", []any{map[string]string{"B": "N"}}},
		}

		lt, err := x.ExecuteTree(dot)
		if err != nil {
			// Invalid templates are uninteresting.
			return
		}

		// Check invariants.
		if len(lt.Dynamics)+1 != len(lt.Statics) {
			t.Errorf("dyn=%d static=%d template:\n%s\n", len(lt.Dynamics), len(lt.Statics), data)
		}

		// Confirm that we can marshal it.
		if _, err := lt.JSON(); err != nil && !errors.Is(err, json.ErrInvalidUTF8) {
			t.Errorf("failed to JSON: %v, template:\n%s\n", err, data)
		}
	})
}
