//go:build gofuzz

package fuzz

import (
	"log"
)

func Fuzz(in []byte) int {
	data := string(in)
	return fuzz(log.Fatalf, data)
}
