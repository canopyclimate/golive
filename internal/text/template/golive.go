package template

import (
	"bytes"
	"fmt"
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
	ppStartFn, ppEndFn, err := t.goLivePostprocessFuncNames()
	if err != nil {
		return nil, err
	}
	defer errRecover(&err)
	value, ok := data.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(data)
	}
	tree = tmpl.NewTree()
	state := &state{
		tmpl: t,
		wr:   new(bytes.Buffer),
		vars: []variable{{"$", value}},
		tree: tree,

		ppStartFn: ppStartFn,
		ppEndFn:   ppEndFn,
	}
	if t.Tree == nil || t.Root == nil {
		state.errorf("%q is an incomplete or empty template", t.Name())
	}
	state.walk(value, t.Root)
	if state.pp {
		state.errorf("missing %s call", state.ppEndFn)
	}
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// execGoLivePostprocessFunc executes the funcmap function name, if any, and returns its result.
// name must be of the form func() string.
// If name is present in the funcmap but is not of the correct form, execGoLivePostprocessFunc returns an error.
func (t *Template) execGoLivePostprocessFunc(name string) (string, error) {
	fn, _, ok := findFunction(name, t)
	if !ok {
		return "", nil
	}
	// Must take zero args and return a string.
	typ := fn.Type()
	if typ.NumIn() != 0 {
		return "", fmt.Errorf("%s must have zero arguments", name)
	}
	if typ.NumOut() != 1 {
		return "", fmt.Errorf("%s must return one value", name)
	}
	out := fn.Call(nil)[0]
	if out.Type().Kind() != reflect.String {
		return "", fmt.Errorf("%s must return a string", name)
	}
	funcName := out.String()
	if funcName == "" {
		return "", fmt.Errorf("%s must return a non-empty string", name)
	}
	return funcName, nil
}

func (t *Template) goLivePostprocessFuncNames() (start, end string, err error) {
	// Extract and validate the start function.
	startName, err := t.execGoLivePostprocessFunc("golive_postprocess_start")
	if err != nil {
		return "", "", err
	}
	endName, err := t.execGoLivePostprocessFunc("golive_postprocess_end")
	if err != nil {
		return "", "", err
	}
	if startName == "" && endName == "" {
		return "", "", nil
	}
	if startName == "" {
		return "", "", fmt.Errorf("golive_postprocess_start must be defined if golive_postprocess_end is defined")
	}
	if endName == "" {
		return "", "", fmt.Errorf("golive_postprocess_end must be defined if golive_postprocess_start is defined")
	}
	if startName == endName {
		return "", "", fmt.Errorf("golive_postprocess_start and golive_postprocess_end must return different function names")
	}

	// Validate start function: zero args
	ppStartFn, _, ok := findFunction(startName, t)
	if !ok {
		return "", "", fmt.Errorf("golive_postprocess_start must return the name of a function in the funcmap")
	}
	startTyp := ppStartFn.Type()
	if startTyp.NumIn() != 0 {
		return "", "", fmt.Errorf("golive_postprocess_start must return the name of a function with zero arguments")
	}

	// Validate end function: one arg, []string
	ppEndFn, _, ok := findFunction(endName, t)
	if !ok {
		return "", "", fmt.Errorf("golive_postprocess_end must return the name of a function in the funcmap")
	}
	endTyp := ppEndFn.Type()
	if endTyp.NumIn() != 1 {
		return "", "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument")
	}
	if endTyp.In(0).Kind() != reflect.String {
		return "", "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument of type string")
	}

	return startName, endName, nil
}
