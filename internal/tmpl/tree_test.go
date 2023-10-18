package tmpl_test

import (
	"bytes"
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

func testExec(t *testing.T, funcs htmltmpl.FuncMap, tmpl, want string, dot dot) {
	t.Helper()
	x, err := htmltmpl.New("x").Funcs(funcs).Parse(tmpl)
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
	got := buf.String()
	if want != got {
		t.Fatalf("got\n\t%s\nwant\n\t%s\n", got, want)
	}
}

func TestOneDynamicWithEmptyStaticResultsInString(t *testing.T) {
	const tmpl = "{{ if .X }}{{.X}}{{ end }}"
	const want = `{"0":{"0":"foo","s":["",""]},"s":["",""]}`
	testExec(t, nil, tmpl, want, dot{"X": "foo"})
}

func TestVariableDynamic(t *testing.T) {
	const tmpl = `{{ $foo := "foo" }}
	{{ $foo }}
	`
	const want = `{"0":"foo","s":["\n\t","\n\t"]}`
	testExec(t, nil, tmpl, want, nil)
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
	testExec(t, funcs, tmpl, want, nil)
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
	testExec(t, nil, tmpl, want, dot{"X": []string{"foo", "bar"}})
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
