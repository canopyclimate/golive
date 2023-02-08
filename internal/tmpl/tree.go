package tmpl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/canopyclimate/golive/internal/json"
)

type NodeType int

const (
	NodeTypeSubtree NodeType = iota
	NodeTypeStatic
	NodeTypeDynamic
)

type Tree struct {
	Statics        []string
	Dynamics       []any // string | *Tree | [](string | *Tree)
	ExcludeStatics bool  // controls if MarshalText Statics with serializing
	Title          string
	isRange        bool
	rangeStep      int
}

func (t *Tree) AppendStatic(text string) {
	// only add first set of statics inside a range
	if t.rangeStep > 0 {
		return
	}
	t.Statics = append(t.Statics, text)
}

func (t *Tree) AppendDynamic(d string) {
	if !t.isRange {
		t.Dynamics = append(t.Dynamics, d)
		return
	}

	// for ranges, the Dynanics are an array of arrays of (string or *Tree)
	if len(t.Dynamics) != t.rangeStep+1 {
		// create the array if it doesn't exist already this range step
		t.Dynamics = append(t.Dynamics, []any{d})
		return
	}
	// get the array for this range step and append the next dynamic
	dyn := t.Dynamics[t.rangeStep].([]any)
	dyn = append(dyn, d)
	t.Dynamics[t.rangeStep] = dyn
}

func (t *Tree) AppendSub() *Tree {
	sub := new(Tree)
	t.Dynamics = append(t.Dynamics, sub)
	return sub
}

func (t *Tree) IncRangeStep() {
	if t == nil {
		return
	}
	t.rangeStep++
}

func (t *Tree) AppendRangeSub() *Tree {
	sub := new(Tree)
	sub.isRange = true
	// if this isn't a range tree simply append the sub
	if !t.isRange {
		t.Dynamics = append(t.Dynamics, sub)
		return sub
	}

	// if this is a range tree, append the sub tree
	// to the dynamics array for the current range step
	if len(t.Dynamics) != t.rangeStep+1 {
		// create the array if it doesn't exist already this range step
		t.Dynamics = append(t.Dynamics, []any{sub})
		return sub
	}
	// get the array for this range step and append the sub as the next dynamic
	dyn := t.Dynamics[t.rangeStep].([]any)
	dyn = append(dyn, sub)
	t.Dynamics[t.rangeStep] = dyn
	return sub
}

func Diff(a, b *Tree) {
	panic("TODO")
}

var (
	quoteColon   = []byte(`":`)
	startStatics = []byte(`,"s":[`)
	emptyString  = []byte(`""`)
	startTitle   = []byte(`,"t":`)
)

// JSON returns a JSON representation of the tree.
func (t *Tree) JSON() ([]byte, error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	_, err := t.WriteTo(w)
	if err != nil {
		return nil, err
	}
	err = w.Flush()
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Accept that Josh knew that it wasn't worth fighting with Go's MarshalText :P
func (t *Tree) WriteTo(w io.Writer) (written int64, err error) {
	var buf []byte // re-usable buffer
	writeByte := func(b byte) error {
		buf = buf[:0]
		buf = append(buf, b)
		n, err := w.Write(buf)
		written += int64(n)
		return err
	}
	writeBytes := func(b []byte) error {
		n, err := w.Write(b)
		written += int64(n)
		return err
	}
	writeInt := func(x int) error {
		buf = buf[:0]
		buf = strconv.AppendInt(buf, int64(x), 10)
		n, err := w.Write(buf)
		written += int64(n)
		return err
	}
	writeJSONString := func(s string) error {
		buf = buf[:0]
		buf, err = json.AppendString(buf, s)
		if err != nil {
			return err
		}
		n, err := w.Write(buf)
		written += int64(n)
		return err
	}

	// handle no dynamics case - basically collapse tree into a single string
	if t.Dynamics == nil {
		switch len(t.Statics) {
		case 0:
			// no statics (end of template perhaps?)
			err = writeBytes(emptyString)
			return written, err
		case 1:
			err = writeJSONString(t.Statics[0])
		}
		return written, err
	} else if len(t.Dynamics) > len(t.Statics)+1 {
		// len(Dynaics) should be len(Statics) + 1
		// if they are equal (but not zero) then we add a single
		// empty string to the end of the statics to make up the difference
		t.Statics = append(t.Statics, "")
	}

	if err := writeByte('{'); err != nil {
		return written, err
	}

	if !t.isRange {
		for i, d := range t.Dynamics {
			if i > 0 {
				if err := writeByte(','); err != nil {
					return written, err
				}
			}
			if err := writeByte('"'); err != nil {
				return written, err
			}
			if err := writeInt(i); err != nil {
				return written, err
			}
			if err := writeBytes(quoteColon); err != nil {
				return written, err
			}
			switch d := d.(type) {
			case string:
				if err := writeJSONString(d); err != nil {
					return written, err
				}
			case *Tree:
				// TODO - we want to include statics and diff them out elsewhere
				n, err := d.WriteTo(w)
				written += n
				if err != nil {
					return written, err
				}
			}
		}
	} else {
		// handle range case
		if err := writeByte('"'); err != nil {
			return written, err
		}
		if err := writeByte('d'); err != nil {
			return written, err
		}
		if err := writeBytes(quoteColon); err != nil {
			return written, err
		}
		if err := writeByte('['); err != nil {
			return written, err
		}

		for i, d := range t.Dynamics {
			if i > 0 {
				if err := writeByte(','); err != nil {
					return written, err
				}
			}
			if err := writeByte('['); err != nil {
				return written, err
			}

			switch d := d.(type) {
			case []any:
				for j, dd := range d {
					if j > 0 {
						if err := writeByte(','); err != nil {
							return written, err
						}
					}
					switch dd := dd.(type) {
					case string:
						if err := writeJSONString(dd); err != nil {
							return written, err
						}

					case *Tree:
						n, err := dd.WriteTo(w)
						written += n
						if err != nil {
							return written, err
						}
					default:
						panic(fmt.Sprintf("unexpected type of Dynamic inside []any: %s, type:%q. Should be string or *Tree.", dd, reflect.TypeOf(dd)))
					}
				}
			default:
				panic(fmt.Sprintf("unexpected type of Dynamic: %s, type:%q.  Should be []any.", d, reflect.TypeOf(d)))
			}
			if err := writeByte(']'); err != nil {
				return written, err
			}
		}

		if err := writeByte(']'); err != nil {
			return written, err
		}
	}

	// if there are dynamics, we also should have statics
	// but only write them if ExcludeStatics is false
	if !t.ExcludeStatics {
		if err := writeBytes(startStatics); err != nil {
			return written, err
		}
		for i, s := range t.Statics {
			if i > 0 {
				if err := writeByte(','); err != nil {
					return written, err
				}
			}
			// TODO: json encode s when we first receive it, instead of every time
			if err := writeJSONString(s); err != nil {
				return written, err
			}
		}
		if err := writeByte(']'); err != nil {
			return written, err
		}
	}
	// write title tree part if not empty
	if t.Title != "" {
		if err := writeBytes(startTitle); err != nil {
			return written, err
		}
		if err := writeJSONString(t.Title); err != nil {
			return written, err
		}
	}

	if err := writeByte('}'); err != nil {
		return written, err
	}
	return written, nil
}
