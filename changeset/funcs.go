package changeset

import (
	"strings"

	"github.com/canopyclimate/golive/htmltmpl"
)

// Funcs returns a map of functions that can be used in templates:
//   - inputTag: renders an input tag with the given key and value for the provided changeset
//   - errorTag: renders an error tag if there is an error for the given key in the provided changeset
func Funcs[T any]() htmltmpl.FuncMap {
	return htmltmpl.FuncMap{
		"inputTag": InputTag[T],
		"errorTag": ErrorTag[T],
	}
}

var (
	inputTagTmpl = htmltmpl.Must(htmltmpl.New("inputTag").Parse(
		`<input type="text" name="{{ .Key }}" value="{{ .Val }}"/>`,
	))
	errorTagTmpl = htmltmpl.Must(htmltmpl.New("errorTag").Parse(
		`<span class="error">{{ . }}</span>`,
	))
)

// InputTag renders an input tag with the given key and value for the provided changeset.
func InputTag[T any](cs *Changeset[T], key string) htmltmpl.HTML {
	val := cs.Value(key)
	buf := new(strings.Builder)
	dot := struct{ Key, Val string }{Key: key, Val: val}
	err := inputTagTmpl.Execute(buf, dot)
	if err != nil {
		panic(err)
	}
	return htmltmpl.HTML(buf.String())
}

// ErrorTag renders an error tag if there is an error for the given key in the provided changeset.
func ErrorTag[T any](cs *Changeset[T], key string) htmltmpl.HTML {
	val := cs.Error(key)
	buf := new(strings.Builder)
	err := errorTagTmpl.Execute(buf, val)
	if err != nil {
		panic(err)
	}
	return htmltmpl.HTML(buf.String())
}
