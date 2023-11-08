package tmpl

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"

	js "encoding/json"

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

func DiffMapJSON(old, new *Tree) []byte {
	// use map to diff
	oldMap := old.Map()
	newMap := new.Map()
	diffMap := DiffMap(oldMap, newMap)
	diffJSON, err := js.Marshal(diffMap)
	if err != nil {
		panic(err)
	}
	return diffJSON
}

// DiffMap returns a new map that represents the difference between the old and new maps.
func DiffMap(old, new map[string]any) map[string]any {
	diff := make(map[string]any)
	for k, nv := range new {
		ov := old[k]
		// new key
		if ov == nil {
			diff[k] = nv
			continue
		}
		// skip title and event keys
		if k == "t" || k == "e" {
			continue
		}
		// statics key
		if k == "s" {
			oldStatics := ov.([]any)
			newStatics := nv.([]any)
			// check length
			if len(oldStatics) != len(newStatics) {
				diff[k] = newStatics
			} else {
				// diff statics
				for i, sd := range newStatics {
					if sd != oldStatics[i] {
						diff[k] = newStatics
						break
					}
				}
			}
		}
		// range key
		if k == "d" {
			oldDynamics := ov.([]any)
			newDynamics := nv.([]any)
			// check length
			if len(oldDynamics) != len(newDynamics) {
				diff[k] = newDynamics
				break
			}
			// diff dynamics
			for i, nsd := range newDynamics {
				osd := oldDynamics[i]
				// should be an array of maps
				for j, nsdd := range nsd.([]any) {
					osdd := osd.([]any)[j]
					switch nsdd.(type) {
					case map[string]any:
						md := DiffMap(osdd.(map[string]any), nsdd.(map[string]any))
						if len(md) > 0 {
							diff[k] = newDynamics
							break
						}
					case string:
						if nsdd != osdd {
							diff[k] = newDynamics
							break
						}
					}
				}
			}
		}
		// numeric key
		// if different types at key take new
		if reflect.TypeOf(ov) != reflect.TypeOf(nv) {
			diff[k] = nv
			break
		}
		// compare same types
		switch nv.(type) {
		case map[string]any:
			md := DiffMap(ov.(map[string]any), nv.(map[string]any))
			if len(md) > 0 {
				diff[k] = md
			}
		case string:
			if nv != ov {
				diff[k] = nv
			}
		}
	}
	return diff
}

// Diff returns a new tree that represents the difference between the old and new trees.
// return value may alias some parts of the input trees
func Diff(old, new *Tree) []byte {
	// use map to diff
	oldMap := old.Map()
	newMap := new.Map()
	diffMap := DiffMap(oldMap, newMap)
	diffJSON, err := js.Marshal(diffMap)
	if err != nil {
		panic(err)
	}

	d := diff(old, new).JSON()
	if d == nil {
		return []byte("{}")
	}
	// read back in to map
	var dMap map[string]any
	err = js.Unmarshal(d, &dMap)
	if err != nil {
		panic(fmt.Sprintf("error unmarshalling d: %s, err: %s", d, err))
	}
	ddMap := DiffMap(dMap, diffMap)
	if len(ddMap) > 0 {
		fmt.Printf("diffMap: %s\n\n\n\n\n", ddMap)
	}
	return diffJSON
}

type skipRender struct{}

func excludeStatics(old, new []string) bool {
	if len(old) != len(new) {
		return false
	}
	for i := range new {
		if old[i] != new[i] {
			return false
		}
	}
	return true
}

// diff returns a new tree that represents the difference between the old and new trees which
// will be nil if there is no difference.
func diff(old, new *Tree) *Tree {
	diffTree := NewTree()

	// handle range
	if new.isRange {
		// check if both are ranges
		if !old.isRange {
			diffTree = new
			diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
			return diffTree
		}
		// check lengths
		if len(old.Dynamics) != len(new.Dynamics) {
			diffTree = new
			diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
			return diffTree
		}
		// if same length, check each element
		for i := range new.Dynamics {
			nd := new.Dynamics[i]
			od := old.Dynamics[i]
			// if nd type and od type are different take all new
			if reflect.TypeOf(od) != reflect.TypeOf(nd) {
				diffTree = new
				diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
				return diffTree
			}
			// each element is a slice of dynamics
			for j := range nd.([]any) {
				nsd := nd.([]any)[j]
				osd := od.([]any)[j]
				if reflect.TypeOf(osd) != reflect.TypeOf(nsd) {
					diffTree = new
					diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
					return diffTree
				}
				// replace old with new if different
				switch nsd.(type) {
				case string:
					if nsd != osd {
						diffTree = new
						diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
						return diffTree
					}
				case *Tree:
					subTree := diff(osd.(*Tree), nsd.(*Tree))
					if subTree != nil {
						diffTree = new
						diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
						return diffTree
					}
				default:
					panic(fmt.Sprintf("unexpected type of Dynamic in range: %T, want string or *Tree, value is: %v", nsd, nsd))
				}
			}
		}
		// if we get here, the ranges are the same if statics are the same
		// then we return nil
		if excludeStatics(old.Statics, new.Statics) {
			return nil
		}
		// otherwise we return the new tree *with* statics
		diffTree = new
		diffTree.ExcludeStatics = false
		return diffTree
	}

	// not a range
	allSkips := true
	for i := 0; i < len(new.Dynamics); i++ {
		nd := new.Dynamics[i]
		// Grab old dynamic if it exists
		// if old has more dynamics than new, we are just skipping them
		var od any
		if i < len(old.Dynamics) {
			od = old.Dynamics[i]
		}

		// Have more new dynamics than old dynamics. Add them.
		if od == nil {
			allSkips = false
			diffTree.Dynamics = append(diffTree.Dynamics, nd)
			continue
		}

		// if nd type and od type are different take new
		if reflect.TypeOf(od) != reflect.TypeOf(nd) {
			allSkips = false
			diffTree.Dynamics = append(diffTree.Dynamics, nd)
			continue
		}

		// if same type, check if different values
		switch nd := nd.(type) {
		case string:
			if od != nd {
				allSkips = false
				diffTree.Dynamics = append(diffTree.Dynamics, nd)
				continue
			}
		case *Tree:
			// fmt.Printf("both trees: %v -> %v\n", od.(*Tree), nd)
			subTree := diff(od.(*Tree), nd)
			// fmt.Printf("diffTree of this dynamic: %v\n", subTree)
			if subTree != nil {
				allSkips = false
				diffTree.Dynamics = append(diffTree.Dynamics, subTree)
				continue
			}
		default:
			panic(fmt.Sprintf("unexpected type of Dynamic: %T, want string or *Tree, value is: %v", nd, nd))
		}
		// if we've gotten this far then no diffs
		diffTree.Dynamics = append(diffTree.Dynamics, skipRender{})
	}

	// if all the dynamics are skipped and all the statics are skipped then no diff
	if excludeStatics(old.Statics, new.Statics) && allSkips {
		return nil
	}

	// otherwise return new tree and possibly new statics
	diffTree.ExcludeStatics = excludeStatics(old.Statics, new.Statics)
	return diffTree
}

// Map returns a map representation of the tree.
func (t *Tree) Map() map[string]any {
	m := map[string]any{}
	err := js.Unmarshal(t.JSON(), &m)
	if err != nil {
		panic(err)
	}
	return m
}

// JSON returns a JSON representation of the tree.
func (t *Tree) JSON() []byte {
	b := new(bytes.Buffer)
	_, err := t.WriteTo(b)
	if err != nil {
		panic(err)
	}
	return b.Bytes()
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
	if t == nil {
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
		hasPrevious := false
		for i, d := range t.Dynamics {
			switch d.(type) {
			case skipRender:
				continue // skip this dynamic
			}
			if hasPrevious {
				cw.writeString(`,`)
			}
			cw.writeString(`"`)
			cw.writeInt(i)
			cw.writeString(`":`)
			cw.writeDynamic(d)
			hasPrevious = true
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
