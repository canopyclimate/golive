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
	ppStartFns, ppEndFns, ppCtxVar, err := t.goLivePostprocessNames()
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

		ppStartFns: ppStartFns,
		ppEndFns:   ppEndFns,
		ppCtxVar:   ppCtxVar,
	}
	if t.Tree == nil || t.Root == nil {
		state.errorf("%q is an incomplete or empty template", t.Name())
	}
	state.walk(value, t.Root)
	if state.pp {
		state.errorf("missing postprocess end call (%v)", state.ppEndFns)
	}
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// execGoLivePostprocessFunc executes the funcmap function name, if any, and returns its result.
// If name is present in the funcmap but is not of the correct form, execGoLivePostprocessFunc returns an error.
func (t *Template) execGoLivePostprocessFunc(name string) (any, error) {
	fn, _, ok := findFunction(name, t)
	if !ok {
		return nil, nil
	}
	// Must take zero args and return one.
	typ := fn.Type()
	if typ.NumIn() != 0 {
		return nil, fmt.Errorf("%s must have zero arguments", name)
	}
	if typ.NumOut() != 1 {
		return nil, fmt.Errorf("%s must return one value", name)
	}
	return fn.Call(nil)[0].Interface(), nil
}

func (t *Template) goLivePostprocessNames() (start, end []string, ctx string, err error) {
	// Extract and validate the start function.
	rawStartNames, err := t.execGoLivePostprocessFunc("golive_postprocess_start")
	if err != nil {
		return nil, nil, "", err
	}
	rawEndNames, err := t.execGoLivePostprocessFunc("golive_postprocess_end")
	if err != nil {
		return nil, nil, "", err
	}
	rawCtxName, err := t.execGoLivePostprocessFunc("golive_postprocess_context_var")
	if err != nil {
		return nil, nil, "", err
	}
	if rawStartNames == nil && rawEndNames == nil && rawCtxName == nil {
		return nil, nil, "", nil
	}

	startNames, ok := rawStartNames.([]string)
	if !ok {
		return nil, nil, "", fmt.Errorf("golive_postprocess_start must return a []string containing names of functions in the funcmap")
	}
	endNames, ok := rawEndNames.([]string)
	if !ok {
		return nil, nil, "", fmt.Errorf("golive_postprocess_end must return a []string containing the names of functions in the funcmap")
	}
	ctxName, ok := rawCtxName.(string)
	if !ok {
		return nil, nil, "", fmt.Errorf("golive_postprocess_context_var must return a string containing the name of a variable in the funcmap")
	}

	if len(startNames) == 0 {
		return nil, nil, "", fmt.Errorf("golive_postprocess_start must be defined if golive_postprocess_end is defined")
	}
	if len(endNames) == 0 {
		return nil, nil, "", fmt.Errorf("golive_postprocess_end must be defined if golive_postprocess_start is defined")
	}
	if ctxName == "" {
		return nil, nil, "", fmt.Errorf("golive_postprocess_context_var must be defined and non-empty if golive_postprocess_start is defined")
	}

	// Validate start functions: zero args
	for _, startName := range startNames {
		ppStartFn, _, ok := findFunction(startName, t)
		if !ok {
			return nil, nil, "", fmt.Errorf("golive_postprocess_start must return names of functions in the funcmap")
		}
		startTyp := ppStartFn.Type()
		if startTyp.NumIn() != 0 {
			return nil, nil, "", fmt.Errorf("golive_postprocess_start must return names of functions with zero arguments")
		}
	}

	// Validate end functions: one arg, ...any
	for _, endName := range endNames {
		ppEndFn, _, ok := findFunction(endName, t)
		if !ok {
			return nil, nil, "", fmt.Errorf("golive_postprocess_end must return the name of a function in the funcmap")
		}
		endTyp := ppEndFn.Type()
		if endTyp.NumIn() != 1 {
			return nil, nil, "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument")
		}
		if endTyp.In(0).Kind() != reflect.Slice {
			return nil, nil, "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument of type ...any")
		}
		if endTyp.In(0).Elem().Kind() != reflect.Interface {
			return nil, nil, "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument of type ...any")
		}
		if endTyp.In(0).Elem().NumMethod() != 0 {
			return nil, nil, "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument of type ...any")
		}
		if !endTyp.IsVariadic() {
			return nil, nil, "", fmt.Errorf("golive_postprocess_end must return the name of a function with one argument of type ...any")
		}
	}

	return startNames, endNames, ctxName, nil
}
