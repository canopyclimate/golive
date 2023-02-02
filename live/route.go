package live

import (
	"fmt"
	"net/http"
	"reflect"
)

type JoinHandler struct {
	lv   View
	code int
}

func (x *JoinHandler) SetView(lv View) {
	x.lv = lv
}

func (x *JoinHandler) View() View {
	return x.lv
}

// Header returns an empty http.Header.
// Modifications to the returned header are ignored.
func (*JoinHandler) Header() http.Header {
	// TODO: record these for some reason?
	return make(http.Header)
}

// Write always returns an error.
func (*JoinHandler) Write(b []byte) (int, error) {
	// TODO: buffer instead?
	return 0, fmt.Errorf("live.JoinHandler rejects all writes: %s", b)
}

// WriteHeader notes a status code.
func (x *JoinHandler) WriteHeader(statusCode int) {
	x.code = statusCode
}

func SetView(rw http.ResponseWriter, v View) {
	j, ok := rw.(*JoinHandler)
	if !ok {
		return
	}
	j.SetView(v)
}

func PatchView[T View](rw http.ResponseWriter) T {
	var zero T
	j, ok := rw.(*JoinHandler)
	if !ok {
		return zero
	}
	t, ok := j.lv.(T)
	if !ok {
		return zero
	}
	return t
}

func MakeView[T View](rw http.ResponseWriter) T {
	var zero T
	j, ok := rw.(*JoinHandler)
	if !ok {
		return zero
	}
	t, ok := j.lv.(T)
	if !ok {
		typ := reflect.TypeOf(zero)
		if typ.Kind() != reflect.Pointer {
			return reflect.New(typ).Elem().Interface().(T)
		}
		return reflect.New(typ.Elem()).Interface().(T)
	}
	return t
}
