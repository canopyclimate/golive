package fuzz

import (
	"bytes"
	"errors"
	"strings"

	"github.com/canopyclimate/golive/htmltmpl"
	"github.com/canopyclimate/golive/internal/json"
)

var funcs = htmltmpl.FuncMap{
	"golive_postprocess_context_var": func() string { return "$ppctx" },
	"golive_postprocess_start":       func() []string { return []string{"pp", "qq"} },
	"golive_postprocess_end":         func() []string { return []string{"xpp"} },
	"pp": func() string {
		return "["
	},
	"qq": func() string {
		return "("
	},
	"xpp": func(x ...any) string {
		if len(x) == 0 {
			return "]"
		}
		if len(x) != 2 {
			panic("expected 2 args")
		}
		return x[1].(string) + "]"
	},
}

func fuzz(fatalf func(string, ...any), data string) int {
	x, err := htmltmpl.New("x").Funcs(funcs).Parse(data)
	if err != nil {
		// Junk input.
		return -1
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
		return -1
	}

	// Check invariants.
	if err := lt.Valid(); err != nil {
		panic(err)
	}

	// Confirm that we can marshal it.
	out, err := lt.JSON()
	if err != nil && !errors.Is(err, json.ErrInvalidUTF8) {
		fatalf("failed to JSON: %v, template:\n%s\n", err, data)
	}
	// Confirm that a second marshalling generates the same output.
	out2, err := lt.JSON()
	if err != nil && !errors.Is(err, json.ErrInvalidUTF8) {
		fatalf("failed to JSON second time: %v, template:\n%s\n", err, data)
	}
	if !bytes.Equal(out, out2) {
		fatalf("non-idempotent JSON: %q != %q", out, out2)
	}

	// Confirm that a regular exec matches a rendered tree.
	buf := new(strings.Builder)
	err = x.Execute(buf, dot)
	if err != nil {
		// We successfully executed earlier, so this ought to as well.
		fatalf("failed to exec: %v, template:\n%s\n", err, data)
	}
	exec := buf.String()
	buf.Reset()
	err = lt.RenderTo(buf)
	if err != nil {
		fatalf("failed to render: %v, template:\n%s\n", err, data)
	}
	render := buf.String()
	if exec != render {
		fatalf("exec != render: %q != %q", exec, render)
	}
	return 0
}
