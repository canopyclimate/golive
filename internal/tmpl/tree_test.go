package tmpl_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/canopyclimate/golive/htmltmpl"
	"github.com/canopyclimate/golive/internal/tmpl"
	"github.com/canopyclimate/golive/live"
	"github.com/josharian/tstruct"
)

func TestBasicSerialization(t *testing.T) {
	root := tmpl.NewTree()
	tree := root
	tree.AppendDynamic("abc")
	tree.AppendStatic("def")
	tree = tree.AppendSub()
	tree.AppendStatic("xyz")
	buf := new(bytes.Buffer)
	n, err := root.WriteTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("wrote %d but tracked %d", buf.Len(), n)
	}
	const want = `{"0":"abc","1":"xyz","s":["","def",""]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got %q want %q", got, want)
	}
}

type dot = map[string]any

func testExec(t *testing.T, funcs htmltmpl.FuncMap, tmpl, wantJSON, wantPlain string, dot dot) {
	t.Helper()
	x, err := htmltmpl.New("test_tmpl").Funcs(funcs).Parse(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	lt, err := x.ExecuteTree(dot)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(strings.Builder)
	n, err := lt.WriteTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Errorf("wrote %d but tracked %d", buf.Len(), n)
	}
	gotJSON := buf.String()
	if wantJSON != gotJSON {
		t.Errorf("json got\n\t%s\nwant\n\t%s\n", gotJSON, wantJSON)
	}

	buf.Reset()
	err = x.Execute(buf, dot)
	if err != nil {
		t.Fatal(err)
	}
	gotPlain := buf.String()
	if wantPlain != gotPlain {
		t.Errorf("exec got\n\t%s\nwant\n\t%s\n", gotPlain, wantPlain)
	}

	buf.Reset()
	err = lt.RenderTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	gotRendered := buf.String()
	if wantPlain != gotRendered {
		t.Errorf("render got\n\t%s\nwant\n\t%s\n", wantPlain, gotRendered)
	}
}

func TestOneDynamicWithEmptyStaticResultsInString(t *testing.T) {
	const tmpl = "{{ if .X }}{{.X}}{{ end }}"
	const want = `{"0":{"0":"foo","s":["",""]},"s":["",""]}`
	const plain = `foo`
	testExec(t, nil, tmpl, want, plain, dot{"X": "foo"})
}

func TestEmptyDynamic(t *testing.T) {
	const tmpl = "{{ if .X }}{{.X}}{{ end }}"
	const want = `{"0":"","s":["",""]}`
	const plain = ``
	testExec(t, nil, tmpl, want, plain, dot{"X": ""})
}

func mapEqFunc(a, b any) bool {
	// if types differ, they are not equal
	if a == nil || b == nil {
		return false
	}
	switch a.(type) {
	case string:
		return a.(string) == b.(string)
	case map[string]any:
		for k, v := range a.(map[string]any) {
			switch v.(type) {
			case string:
				if v.(string) != b.(map[string]any)[k].(string) {
					return false
				}
			default:
				return false
			}
		}
	case []any:
		for i, v := range a.([]any) {
			switch v.(type) {
			case string:
				if v.(string) != b.([]any)[i].(string) {
					return false
				}
			case map[string]any:
				return mapEqFunc(v, b.([]any)[i])
			}
		}

	default:
		return false
	}
	return true
}

func TestBasicDiff(t *testing.T) {
	t.Run("change_first_if", func(t *testing.T) {
		const tm = "{{ if .X }}{{.X}}{{ end }}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"X": "A"})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"X": "B"})
		if err != nil {
			t.Fatal(err)
		}

		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("change_second_if", func(t *testing.T) {
		const tm = "{{ if .X }}{{.X}}{{ end }}{{ if .Y }}{{.Y}}{{ end }}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"X": "A", "Y": "B"})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"X": "A", "Y": "C"})
		if err != nil {
			t.Fatal(err)
		}
		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("change_deep_if", func(t *testing.T) {
		const tm = "{{ if .X }}{{ if .Y }}{{.Y}}{{ end }}{{ end }}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"X": true, "Y": "Y"})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"X": true, "Y": ""})
		if err != nil {
			t.Fatal(err)
		}
		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("change_shallow_if", func(t *testing.T) {
		const tm = "{{ if .X }}{{ if .Y }}{{.Y}}{{ end }}{{ end }}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"X": true, "Y": "Y"})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"X": false, "Y": "Y"})
		if err != nil {
			t.Fatal(err)
		}
		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("one_to_zero", func(t *testing.T) {
		const tm = "{{if .Z}}{{.Z}}{{end}}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"Z": "Z"})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"Z": ""})
		if err != nil {
			t.Fatal(err)
		}
		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("range up", func(t *testing.T) {
		const tm = "{{range .R}}{{.}}{{end}}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"R": []string{"A", "B"}})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"R": []string{"A", "B", "C"}})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("oldTree: %s\n", oldTree.JSON())
		fmt.Printf("newTree: %s\n", newTree.JSON())
		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("range down", func(t *testing.T) {
		const tm = "{{range .R}}{{.}}{{end}}"
		x, err := htmltmpl.New("test_tmpl").Parse(tm)
		if err != nil {
			t.Fatal(err)
		}
		oldTree, err := x.ExecuteTree(map[string]any{"R": []string{"A", "B", "C"}})
		if err != nil {
			t.Fatal(err)
		}

		newTree, err := x.ExecuteTree(map[string]any{"R": []string{"A", "B"}})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("oldTree: %s\n", oldTree.JSON())
		fmt.Printf("newTree: %s\n", newTree.JSON())
		want := tmpl.DiffMapJSON(oldTree, newTree)
		got := tmpl.Diff(oldTree, newTree)
		if string(want) != string(got) {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

func TestVariableDynamic(t *testing.T) {
	const tmpl = `{{ $foo := "foo" }}
	{{ $foo }}
	`
	const want = `{"0":"foo","s":["\n\t","\n\t"]}`
	const plain = "\n\tfoo\n\t"
	testExec(t, nil, tmpl, want, plain, nil)
}

func TestTStructInTemplate(t *testing.T) {
	type TestTStruct struct {
		Attr string
	}
	funcs := htmltmpl.FuncMap{}
	err := tstruct.AddFuncMap[TestTStruct](funcs)
	if err != nil {
		t.Fatal(err)
	}
	const tmpl = `
	{{ define "test_template" }}
		Attr is: {{ .Attr }}
	{{ end }}{{/*comment*/}}
	{{ template "test_template" TestTStruct (Attr "foo") }}
	`
	const want = `{"0":{"0":"foo","s":["\n\t\tAttr is: ","\n\t"]},"s":["\n\t\n\t","\n\t"]}`
	const plain = "\n\t\n\t\n\t\tAttr is: foo\n\t\n\t"
	testExec(t, funcs, tmpl, want, plain, nil)
}

func TestBasicPostprocess(t *testing.T) {
	funcs := htmltmpl.FuncMap{
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
			if x[0] != "!" {
				t.Fatalf("$ppctx not set correctly, got %v (%T)", x[0], x[0])
			}
			return x[1].(string) + "]"
		},
	}

	tests := []struct {
		tmpl  string
		want  string
		plain string
	}{
		{
			tmpl:  `{{$ppctx := "!"}}a{{ pp }}123{{ xpp }}b{{qq}}456{{ xpp }}c`,
			want:  `{"0":"[123]","1":"(456]","s":["a","b","c"]}`,
			plain: `a[123]b(456]c`,
		},
		{
			tmpl:  `{{$ppctx := "!"}}a{{ .X }}b{{ pp }}123{{ xpp }}c`,
			want:  `{"0":"X","1":"[123]","s":["a","b","c"]}`,
			plain: `aXb[123]c`,
		},
	}

	for _, test := range tests {
		testExec(t, funcs, test.tmpl, test.want, test.plain, dot{"X": "X"})
	}
}

func TestEmptyRangeSerialization(t *testing.T) {
	root := tmpl.NewTree()
	tree := root
	tree.AppendDynamic("abc")
	tree.AppendStatic("def")
	tree.AppendRangeSub()
	buf := new(bytes.Buffer)
	n, err := root.WriteTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("wrote %d but tracked %d", buf.Len(), n)
	}
	const want = `{"0":"abc","1":"","s":["","def",""]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNonEmptyRangeSerialization(t *testing.T) {
	root := tmpl.NewTree()
	tree := root
	tree.AppendDynamic("abc")
	tree.AppendStatic("def")
	tree = tree.AppendRangeSub()
	tree.AppendStatic("x is ")
	tree.AppendDynamic("1")
	tree.AppendStatic(".")
	tree.IncRangeStep()
	tree.AppendStatic("x is ")
	tree.AppendDynamic("2")
	tree.AppendStatic(".")
	tree.IncRangeStep()
	tree.AppendStatic("x is ")
	tree.AppendDynamic("3")
	tree.AppendStatic(".")
	tree.IncRangeStep()
	buf := new(bytes.Buffer)
	n, err := root.WriteTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("wrote %d but tracked %d", buf.Len(), n)
	}
	const want = `{"0":"abc","1":{"d":[["1"],["2"],["3"]],"s":["x is ","."]},"s":["","def",""]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got \n%q want \n%q", got, want)
	}
}

func TestRangeTemplateSerialization(t *testing.T) {
	const tmpl = `{{ range $i, $v := .X }}{{ $i }}:{{ $v }}s{{/*comment*/}}t{{ end}}`
	const want = `{"0":{"d":[["0","foo"],["1","bar"]],"s":["",":","st"]},"s":["",""]}`
	const plain = `0:foost1:barst`
	testExec(t, nil, tmpl, want, plain, dot{"X": []string{"foo", "bar"}})
}

func TestEvents(t *testing.T) {
	root := tmpl.NewTree()
	tree := root
	tree.AppendDynamic("abc")
	tree.AppendStatic("def")
	tree = tree.AppendSub()
	tree.AppendStatic("xyz")
	vals := url.Values{}
	vals.Add("foo", "bar")
	vals.Add("baz", "qux")
	vals.Add("baz", "quv")
	evt := live.Event{
		Type: "some_event",
		Data: vals,
	}
	j, err := evt.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := (&live.Event{
		Type: "another_event",
		Data: url.Values{
			"biz": []string{"buz"},
		},
	}).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	root.Events = [][]byte{j, j2}
	buf := new(bytes.Buffer)
	n, err := root.WriteTo(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("wrote %d but tracked %d", buf.Len(), n)
	}
	const want = `{"0":"abc","1":"xyz","s":["","def",""],"e":[["some_event",{"baz":["qux","quv"],"foo":"bar"}],["another_event",{"biz":"buz"}]]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got %q want %q", got, want)
	}
}

func FuzzTreeSerialization(f *testing.F) {
	f.Fuzz(func(t *testing.T, seed int64, s string, n byte) {
		if !utf8.ValidString(s) {
			return
		}
		root := tmpl.NewTree()
		trees := []*tmpl.Tree{root}
		prng := rand.New(rand.NewSource(seed))
		sub := func() string {
			x := s
			if len(x) == 0 {
				return x
			}
			x = x[prng.Intn(len(x)):]
			if len(x) == 0 {
				return x
			}
			x = x[:prng.Intn(len(x))]
			return x
		}
		for i := 0; i < int(n); i++ {
			tree := trees[prng.Intn(len(trees))]
			switch prng.Intn(3) {
			case 0:
				tree.AppendStatic(sub())
			case 1:
				tree.AppendDynamic(sub())
			case 2:
				trees = append(trees, tree.AppendSub())
			}
		}
		root.WriteTo(io.Discard)
	})
}

func BenchmarkWideStatic(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		root := tmpl.NewTree()
		for j := 0; j < 20; j++ {
			root.AppendStatic("a")
		}
		_, err := root.WriteTo(io.Discard)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWideDynamic(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		root := tmpl.NewTree()
		for j := 0; j < 20; j++ {
			root.AppendDynamic("a")
		}
		_, err := root.WriteTo(io.Discard)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeep(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		root := tmpl.NewTree()
		tree := root
		for j := 0; j < 20; j++ {
			tree = tree.AppendSub()
		}
		_, err := root.WriteTo(io.Discard)
		if err != nil {
			b.Fatal(err)
		}
	}
}
