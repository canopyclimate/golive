package htmltmpl

import (
	"encoding/json"
	"testing"
)

func TestExplore(t *testing.T) {
	x, err := New("x").Parse("abc {{ .X }} def {{ if .X }} xyz {{ end }}")
	if err != nil {
		t.Fatal(err)
	}
	for _, dot := range []any{
		map[string]int{"X": 5},
		map[string]int{"X": 0},
	} {
		lt, err := x.ExecuteTree(dot)
		if err != nil {
			t.Fatal(err)
		}
		out, err := json.MarshalIndent(lt, "", "\t")
		if err != nil {
			t.Fatal(err)
		}
		t.Log(dot, ":")
		t.Log(string(out))
		t.Log("----")
	}
}
