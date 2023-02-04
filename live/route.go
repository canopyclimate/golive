package live

import (
	"fmt"
	"net/http"
	"reflect"
)

// joinHandler wraps an http.ResponseWriter and swallows all HTTP activity—
// except for WriteHeader, which it steals the status code from but does not pass through.
// This is to prevent the live.Config.Mux routing from poisoning non-LiveView requests
// by writing, for examples, 404s to routes that very much exist elsewhere in the routing tree.
type joinHandler struct {
	w    http.ResponseWriter
	code int
}

func (x *joinHandler) Header() http.Header {
	return make(http.Header)
}

func (x *joinHandler) Write(b []byte) (int, error) {
	return 0, fmt.Errorf("joinHandler.Write does not accept writes")
}

func (x *joinHandler) WriteHeader(statusCode int) {
	if x.code == 0 {
		x.code = statusCode
	}
}

// SetView marks r as corresponding to v.
// Its handler will result in a rendered LiveView.
func SetView(r *http.Request, v View) {
	container, ok := r.Context().Value(liveViewRequestContextKey{}).(*liveViewContainer)
	if !ok {
		return
	}
	container.lv = v
}

// GetView returns the View of type T corresponding to r.
// If no such view has been set, returns the zero value for T.
func GetView[T View](r *http.Request) T {
	var zero T
	container, ok := r.Context().Value(liveViewRequestContextKey{}).(*liveViewContainer)
	if !ok {
		return zero
	}
	t, ok := container.lv.(T)
	if !ok {
		return zero
	}
	return t
}

// MakeView will either get the existing View of type T associated with r
// or create a View of type T.
// If r is not a LiveView-enabled request, returns the zero value of T.
func MakeView[T View](r *http.Request) T {
	var zero T
	container, ok := r.Context().Value(liveViewRequestContextKey{}).(*liveViewContainer)
	if !ok {
		return zero
	}
	t, ok := container.lv.(T)
	if !ok {
		typ := reflect.TypeOf(zero)
		if typ.Kind() != reflect.Pointer {
			return reflect.New(typ).Elem().Interface().(T)
		}
		return reflect.New(typ.Elem()).Interface().(T)
	}
	return t
}
