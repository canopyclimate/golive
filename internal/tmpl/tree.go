package tmpl

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"github.com/canopyclimate/golive/internal/json"
)

type Tree struct {
	Statics        []string
	Dynamics       []any // string | *Tree | []any
	ExcludeStatics bool  // controls if MarshalText Statics with serializing
	Title          string
	Events         [][]byte
	isRange        bool
	rangeStep      int
}

func NewTree() *Tree {
	return &Tree{Statics: []string{""}}
}

func (t *Tree) AppendStatic(text string) {
	if t.rangeStep > 0 {
		// We're inside a range loop, and we've already seen all the statics.
		return
	}
	// Prevent two consecutive statics by concatenation.
	// TODO: this is potentially quadratic, consider storing statics as slices of strings instead
	nDyn := t.nDyn()
	if len(t.Statics) > nDyn {
		t.Statics[len(t.Statics)-1] += text
		return
	}
	t.Statics = append(t.Statics, text)
}

func (t *Tree) AppendDynamic(d string) {
	t.appendDynamic(d)
}

func (t *Tree) AppendSub() *Tree {
	sub := NewTree()
	t.appendDynamic(sub)
	return sub
}

// AppendRangeSub adds a range subnode to tree.
// Templates call it on entering a range statement.
func (t *Tree) AppendRangeSub() *Tree {
	sub := NewTree()
	sub.isRange = true
	t.appendDynamic(sub)
	return sub
}

// IncRangeStep records that a single range iteration has completed it.
func (t *Tree) IncRangeStep() {
	if t == nil {
		return
	}
	t.rangeStep++
}

func (t *Tree) appendDynamic(d any) {
	// For non-ranges, we pair dynamics with statics directly.
	if !t.isRange {
		t.appendDynamicWithStatic(d)
		return
	}

	// For ranges, the Dynamics are held in the interior slices, each of which is a []any.
	// Append d to that inner slice, ensuring space first as needed.
	if t.rangeStep >= len(t.Dynamics) {
		t.Dynamics = append(t.Dynamics, []any(nil))
	}
	dyn := t.Dynamics[t.rangeStep].([]any)
	t.Dynamics[t.rangeStep] = append(dyn, d)
	// During the first range iteration, append placeholder statics in parallel with dynamics.
	if t.rangeStep == 0 {
		t.Statics = append(t.Statics, "")
	}
}

// appendDynamicWithStatic adds a to t's dynamics,
// preserving the alternating static/dynamic layout.
// Note that the empty static that gets appended here
// will get replaced if AppendStatic gets called next.
func (t *Tree) appendDynamicWithStatic(a any) {
	t.Dynamics = append(t.Dynamics, a)
	t.Statics = append(t.Statics, "")
}

// nDyn returns the number of dynamic elements of t.
func (t *Tree) nDyn() int {
	nDyn := len(t.Dynamics)
	if !t.isRange || nDyn == 0 {
		return nDyn
	}
	// In a range loop, the Dynamics are stored in the interior slices;
	// the outer slice is for each iteration of the range loop.
	// This must be identical for each element, so it suffices
	// to look at index 0, if present.
	return len(t.Dynamics[0].([]any))
}

// Valid performs an internal consistency check and returns an error if it fails.
func (t *Tree) Valid() error {
	nDyn := t.nDyn()
	if nDyn+1 != len(t.Statics) {
		return fmt.Errorf("nDyn = %d, nStatic = %d, want nDyn + 1 = nStatic", nDyn, len(t.Statics))
	}
	return nil
}

func Diff(a, b *Tree) {
	panic("TODO")
}

var (
	quoteColon   = []byte(`":`)
	startStatics = []byte(`,"s":[`)
	startTitle   = []byte(`,"t":`)
	startEvents  = []byte(`,"e":[`)
	startRange   = []byte(`"d":[`)
)

// JSON returns a JSON representation of the tree.
func (t *Tree) JSON() ([]byte, error) {
	b := new(bytes.Buffer)
	_, err := t.WriteTo(b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteTo writes a JSON representation of the tree to w.
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
	if len(t.Dynamics) == 0 {
		if len(t.Statics) != 1 {
			return written, fmt.Errorf("internal error: malformed tree with 0 dynamics and %d statics", len(t.Statics))
		}
		err = writeJSONString(t.Statics[0])
		return written, err
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
		if err := writeBytes(startRange); err != nil {
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
						panic(fmt.Sprintf("unexpected type of Dynamic inside []any: %T, want string or *Tree, value is: %v", dd, dd))
					}
				}
			case *Tree:
				n, err := d.WriteTo(w)
				written += n
				if err != nil {
					return written, err
				}
			default:
				panic(fmt.Sprintf("unexpected type of Dynamic: %T, want string or *Tree, value is: %v", d, d))
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
	// write events tree part if not empty
	if len(t.Events) > 0 {
		if err := writeBytes(startEvents); err != nil {
			return written, err
		}
		for i, e := range t.Events {
			if i > 0 {
				if err := writeByte(','); err != nil {
					return written, err
				}
			}
			if err := writeBytes(e); err != nil {
				return written, err
			}
		}
		if err := writeByte(']'); err != nil {
			return written, err
		}
	}

	if err := writeByte('}'); err != nil {
		return written, err
	}
	return written, nil
}

// RenderTo renders the content represented by t to w.
func (t *Tree) RenderTo(w io.Writer) error {
	if t.Events != nil || t.Title != "" {
		return fmt.Errorf("RenderTo does not support events or title")
	}
	dynamics := t.Dynamics
	if !t.isRange {
		dynamics = []any{t.Dynamics}
	}
	for _, dyns := range dynamics {
		dyns := dyns.([]any)
		for i := 0; i < len(t.Statics); i++ {
			if _, err := io.WriteString(w, t.Statics[i]); err != nil {
				return err
			}
			if i >= len(dyns) {
				continue
			}
			switch dyn := dyns[i].(type) {
			case string:
				if _, err := io.WriteString(w, dyn); err != nil {
					return err
				}
			case *Tree:
				if err := dyn.RenderTo(w); err != nil {
					return err
				}
			default:
				panic(fmt.Sprintf("unexpected type of Dynamic: %T, want string or *Tree, value is: %v", dyn, dyn))
			}
		}
	}
	return nil
}
