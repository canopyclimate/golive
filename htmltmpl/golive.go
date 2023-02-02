package htmltmpl

import "github.com/canopyclimate/golive/internal/tmpl"

// ExecuteTree applies a parsed template to the specified data object,
// and returns a *tmpl.Tree.
//
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) ExecuteTree(data any) (*tmpl.Tree, error) {
	if err := t.escape(); err != nil {
		return nil, err
	}
	return t.text.ExecuteTree(data)
}
