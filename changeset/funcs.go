package changeset

import (
	"strings"

	"github.com/canopyclimate/golive/htmltmpl"
)

// Funcs returns a map of functions that can be used in templates:
//   - inputTag: renders an input tag with the given key and value for the provided changeset
//   - errorTag: renders an error tag if there is an error for the given key in the provided changeset
func Funcs() htmltmpl.FuncMap {
	return htmltmpl.FuncMap{
		"inputTag": InputTag,
		"errorTag": ErrorTag,
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

type Valuer interface {
	Value(string) string
}

type Errorer interface {
	Error(string) error
}

// InputTag renders an input tag with the given key and value for the provided changeset.
func InputTag(v Valuer, key string) htmltmpl.HTML {
	val := v.Value(key)
	buf := new(strings.Builder)
	dot := struct{ Key, Val string }{Key: key, Val: val}
	err := inputTagTmpl.Execute(buf, dot)
	if err != nil {
		panic(err)
	}
	return htmltmpl.HTML(buf.String())
}

// ErrorTag renders an error tag if there is an error for the given key in the provided changeset.
func ErrorTag(e Errorer, key string) htmltmpl.HTML {
	val := e.Error(key)
	buf := new(strings.Builder)
	err := errorTagTmpl.Execute(buf, val)
	if err != nil {
		panic(err)
	}
	return htmltmpl.HTML(buf.String())
}
