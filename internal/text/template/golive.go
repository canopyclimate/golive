package template

import (
	"bytes"
	"reflect"

	"github.com/canopyclimate/golive/internal/tmpl"
)

// ExecuteTree applies a parsed template to the specified data object,
// and returns a *tmpl.Tree.
//
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
//
// If data is a reflect.Value, the template applies to the concrete
// value that the reflect.Value holds, as in fmt.Print.
func (t *Template) ExecuteTree(data any) (*tmpl.Tree, error) {
	return t.executeTree(data)
}

func (t *Template) executeTree(data any) (tree *tmpl.Tree, err error) {
	defer errRecover(&err)
	value, ok := data.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(data)
	}
	tree = new(tmpl.Tree)
	state := &state{
		tmpl: t,
		wr:   new(bytes.Buffer),
		vars: []variable{{"$", value}},
		tree: tree,
	}
	if t.Tree == nil || t.Root == nil {
		state.errorf("%q is an incomplete or empty template", t.Name())
	}
	state.walk(value, t.Root)
	if err != nil {
		return nil, err
	}
	return tree, nil
}
