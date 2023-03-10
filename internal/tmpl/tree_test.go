package tmpl_test

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	"unicode/utf8"

	"github.com/canopyclimate/golive/internal/tmpl"
)

func TestBasicSerialization(t *testing.T) {
	root := new(tmpl.Tree)
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
	const want = `{"0":"abc","1":"xyz","s":["def"]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEmptyRangeSerialization(t *testing.T) {
	root := new(tmpl.Tree)
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
	const want = `{"0":"abc","1":"","s":["def"]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNonEmptyRangeSerialization(t *testing.T) {
	root := new(tmpl.Tree)
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
	const want = `{"0":"abc","1":{"d":[["1"],["2"],["3"]],"s":["x is ","."]},"s":["def"]}`
	got := buf.String()
	if want != got {
		t.Fatalf("got \n%q want \n%q", got, want)
	}
}

func FuzzTreeSerialization(f *testing.F) {
	f.Fuzz(func(t *testing.T, seed int64, s string, n byte) {
		if !utf8.ValidString(s) {
			return
		}
		root := new(tmpl.Tree)
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
		root := new(tmpl.Tree)
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
		root := new(tmpl.Tree)
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
		root := new(tmpl.Tree)
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
