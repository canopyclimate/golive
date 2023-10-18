package tmpl

import (
	"bytes"
	"errors"
	"strings"
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
		if err := lt.Valid(); err != nil {
			t.Error(err)
		}

		// Confirm that we can marshal it.
		out, err := lt.JSON()
		if err != nil && !errors.Is(err, json.ErrInvalidUTF8) {
			t.Errorf("failed to JSON: %v, template:\n%s\n", err, data)
		}
		// Confirm that a second marshalling generates the same output.
		out2, err := lt.JSON()
		if err != nil && !errors.Is(err, json.ErrInvalidUTF8) {
			t.Errorf("failed to JSON second time: %v, template:\n%s\n", err, data)
		}
		if !bytes.Equal(out, out2) {
			t.Errorf("non-idempotent JSON: %q != %q", out, out2)
		}

		// Confirm that a regular exec matches a rendered tree.
		buf := new(strings.Builder)
		err = x.Execute(buf, dot)
		if err != nil {
			// We successfully executed earlier, so this ought to as well.
			t.Errorf("failed to exec: %v, template:\n%s\n", err, data)
		}
		exec := buf.String()
		buf.Reset()
		err = lt.RenderTo(buf)
		if err != nil {
			t.Errorf("failed to render: %v, template:\n%s\n", err, data)
		}
		render := buf.String()
		if exec != render {
			t.Errorf("exec != render: %q != %q", exec, render)
		}
	})
}
