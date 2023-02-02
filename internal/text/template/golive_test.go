package template

import (
	"bufio"
	"bytes"
	"fmt"
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
		fmt.Println(dot, ":")
		fmt.Println("w/Statics", string(out))
		fmt.Println("w/o Statics", string(wout))
		fmt.Println("----")
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
	fmt.Println(dot, ":")
	fmt.Println("w/Statics", string(out))
	rangeTree := lt.Dynamics[0].(*tmpl.Tree)
	targetStatics := []string{" X is ", " "}
	targetDynamics := [][]any{{"1"}, {"2"}, {"3"}}
	passes := true
	if !reflect.DeepEqual(rangeTree.Statics, targetStatics) {
		passes = false
		fmt.Println("expected rangeTree.Statics to be:", targetStatics, "got:", rangeTree.Statics)
	}
	// for each dynamic test against target
	for i, dyn := range rangeTree.Dynamics {
		if !reflect.DeepEqual(dyn, targetDynamics[i]) {
			passes = false
			fmt.Println("expected rangeTree.Dynamics to be:", targetDynamics, "got:", rangeTree.Dynamics)
		}
	}
	if !passes {
		t.Fatal("range tree did not match expected")
	}

	// test t.WriteTo
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	try.E1(rangeTree.WriteTo(w))
	try.E(w.Flush())
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
	fmt.Println(dot, ":")
	fmt.Println("w/Statics", string(out))

	// test t.WriteTo
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	try.E1(lt.WriteTo(w))
	try.E(w.Flush())
	res := b.String()

	want := `{"0":{"d":[[{"d":[["1"],["2"],["3"]],"s":["Y is ","."]}],[{"d":[["1"],["2"],["3"]],"s":["Y is ","."]}],[{"d":[["1"],["2"],["3"]],"s":["Y is ","."]}]],"s":[" X is ",""]},"s":["<div> ","</div>"]}`
	if res != want {
		t.Fatalf("range tree did not match expected \n%s want \n%s", res, want)
	}

}
