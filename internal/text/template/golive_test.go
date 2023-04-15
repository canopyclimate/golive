package template

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/canopyclimate/golive/internal/tmpl"
	"github.com/dsnet/try"
	"github.com/go-json-experiment/json"
)

func TestExplore(t *testing.T) {
	x, err := New("x").Parse("<div> {{ .X }} def {{ if .X }} xyz {{ end }}</div>")
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
		out, err := json.Marshal(lt)
		if err != nil {
			t.Fatal(err)
		}
		lt.ExcludeStatics = true
		wout, err := json.Marshal(lt)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(dot, ":")
		t.Log("w/Statics", string(out))
		t.Log("w/o Statics", string(wout))
		t.Log("----")
	}
}

func TestComments(t *testing.T) {
	x, err := New("x").Parse("<div> {{ .X }} def {{/* comment */}} xyz</div>")
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
		out, err := json.Marshal(lt)
		if err != nil {
			t.Fatal(err)
		}
		if len(lt.Dynamics)+1 != len(lt.Statics) {
			t.Fatalf("expected len(lt.Dynamics)+1 to be len(lt.Statics), but %d + 1 != %d", len(lt.Dynamics), len(lt.Statics))
		}
		lt.ExcludeStatics = true
		wout, err := json.Marshal(lt)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(dot, ":")
		t.Log("w/Statics", string(out))
		t.Log("w/o Statics", string(wout))
		t.Log("----")
	}
}

func TestWithRange(t *testing.T) {
	x, err := New("x").Parse("<div> {{ range $x := .X }} X is {{$x}} {{ end }}</div>")
	if err != nil {
		t.Fatal(err)
	}
	dot := map[string][]int{"X": {1, 2, 3}}
	lt, err := x.ExecuteTree(dot)
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(lt)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(dot, ":")
	t.Log("w/Statics", string(out))
	rangeTree := lt.Dynamics[0].(*tmpl.Tree)
	targetStatics := []string{" X is ", " "}
	targetDynamics := [][]any{{"1"}, {"2"}, {"3"}}
	passes := true
	if !reflect.DeepEqual(rangeTree.Statics, targetStatics) {
		passes = false
		t.Log("expected rangeTree.Statics to be:", targetStatics, "got:", rangeTree.Statics)
	}
	// for each dynamic test against target
	for i, dyn := range rangeTree.Dynamics {
		if !reflect.DeepEqual(dyn, targetDynamics[i]) {
			passes = false
			t.Log("expected rangeTree.Dynamics to be:", targetDynamics, "got:", rangeTree.Dynamics)
		}
	}
	if !passes {
		t.Fatal("range tree did not match expected")
	}

	// test t.WriteTo
	b := new(bytes.Buffer)
	try.E1(rangeTree.WriteTo(b))
	res := b.String()
	if res != `{"d":[["1"],["2"],["3"]],"s":[" X is "," "]}` {
		t.Fatal("range tree did not match expected", res)
	}
}

func TestWithRangeWithSub(t *testing.T) {
	x, err := New("x").Parse("<div> {{ range $x := .X }} X is {{range $y := $.X}}Y is {{$y}}.{{ end }}{{ end }}</div>")
	if err != nil {
		t.Fatal(err)
	}
	dot := map[string][]int{"X": {1, 2, 3}}
	lt, err := x.ExecuteTree(dot)
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(lt)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(dot, ":")
	t.Log("w/Statics", string(out))

	// test t.WriteTo
	b := new(bytes.Buffer)
	try.E1(lt.WriteTo(b))
	res := b.String()

	want := `{"0":{"d":[[{"d":[["1"],["2"],["3"]],"s":["Y is ","."]}],[{"d":[["1"],["2"],["3"]],"s":["Y is ","."]}],[{"d":[["1"],["2"],["3"]],"s":["Y is ","."]}]],"s":[" X is ",""]},"s":["<div> ","</div>"]}`
	if res != want {
		t.Fatalf("range tree did not match expected \n%s want \n%s", res, want)
	}
}
