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

// JSON returns a JSON representation of the tree.
func (t *Tree) JSON() ([]byte, error) {
	b := new(bytes.Buffer)
	_, err := t.WriteTo(b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type countWriter struct {
	n   int64
	w   io.Writer
	buf []byte // re-usable buffer
	err error
}

func (cw *countWriter) writeBytes(p []byte) {
	if cw.err != nil {
		return
	}
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	cw.err = err
}

func (cw *countWriter) writeByte(b byte) {
	if cw.err != nil {
		return
	}
	cw.buf = cw.buf[:0]
	cw.buf = append(cw.buf, b)
	cw.writeBytes(cw.buf)
}

func (cw *countWriter) writeString(s string) {
	if cw.err != nil {
		return
	}
	if len(s) == 1 {
		cw.writeByte(s[0])
		return
	}
	if sw, ok := cw.w.(io.StringWriter); ok {
		n, err := sw.WriteString(s)
		cw.n += int64(n)
		cw.err = err
		return
	}
	cw.buf = cw.buf[:0]
	cw.buf = append(cw.buf, s...)
	cw.writeBytes(cw.buf)
}

func (cw *countWriter) writeInt(x int) {
	if cw.err != nil {
		return
	}
	cw.buf = cw.buf[:0]
	cw.buf = strconv.AppendInt(cw.buf, int64(x), 10)
	cw.writeBytes(cw.buf)
}

func (cw *countWriter) writeJSONString(s string) {
	if cw.err != nil {
		return
	}
	cw.buf = cw.buf[:0]
	var err error
	cw.buf, err = json.AppendString(cw.buf, s)
	if err != nil {
		cw.err = err
		return
	}
	cw.writeBytes(cw.buf)
}

func (cw *countWriter) writeLeadingComma(i int) {
	if i == 0 {
		return
	}
	cw.writeByte(',')
}

func (cw *countWriter) writeDynamic(d any) {
	if cw.err != nil {
		return
	}
	switch d := d.(type) {
	case string:
		cw.writeJSONString(d)
	case *Tree:
		// TODO - we want to include statics and diff them out elsewhere
		d.writeTo(cw)
	default:
		panic(fmt.Sprintf("unexpected type of Dynamic: %T, want string or *Tree, value is: %v", d, d))
	}
}

// WriteTo writes a JSON representation of the tree to w.
func (t *Tree) WriteTo(w io.Writer) (written int64, err error) {
	cw := &countWriter{w: w}
	t.writeTo(cw)
	return cw.n, cw.err
}

func (t *Tree) writeTo(cw *countWriter) {
	if cw.err != nil {
		return
	}

	if len(t.Dynamics) == 0 {
		if len(t.Statics) != 1 {
			panic(fmt.Sprintf("internal error: malformed tree with 0 dynamics and %d statics", len(t.Statics)))
		}
		cw.writeJSONString(t.Statics[0])
		return
	}

	cw.writeString(`{`)

	if !t.isRange {
		for i, d := range t.Dynamics {
			cw.writeLeadingComma(i)
			cw.writeString(`"`)
			cw.writeInt(i)
			cw.writeString(`":`)
			cw.writeDynamic(d)
		}
	} else {
		cw.writeString(`"d":[`)
		for i, d := range t.Dynamics {
			cw.writeLeadingComma(i)
			cw.writeString(`[`)
			for j, dd := range d.([]any) {
				cw.writeLeadingComma(j)
				cw.writeDynamic(dd)
			}
			cw.writeString(`]`)
		}
		cw.writeString(`]`)
	}

	if !t.ExcludeStatics {
		cw.writeString(`,"s":[`)
		for i, s := range t.Statics {
			cw.writeLeadingComma(i)
			// TODO: json encode s when we first receive it, instead of every time
			cw.writeJSONString(s)
		}
		cw.writeString(`]`)
	}

	if t.Title != "" {
		cw.writeString(`,"t":`)
		cw.writeJSONString(t.Title)
	}

	if len(t.Events) > 0 {
		cw.writeString(`,"e":[`)
		for i, e := range t.Events {
			cw.writeLeadingComma(i)
			cw.writeBytes(e)
		}
		cw.writeString(`]`)
	}

	cw.writeString(`}`)
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
