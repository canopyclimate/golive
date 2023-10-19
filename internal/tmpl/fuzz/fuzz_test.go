//go:build !gofuzz

package fuzz

import (
	"testing"
)

func Fuzz(f *testing.F) {
	f.Fuzz(func(t *testing.T, data string) {
		fuzz(t.Fatalf, data)
	})
}
