package phx

import (
	"bytes"
	"testing"
)

func FuzzUploadMsgUnmarshalBinary(f *testing.F) {
	f.Fuzz(func(t *testing.T, in []byte) {
		u := new(UploadMsg)
		err := u.UnmarshalBinary(in)
		if err != nil {
			return
		}
		out, err := u.MarshalBinary()
		if err != nil {
			panic("failed to marshal unmarshalled data")
		}
		if !bytes.Equal(out, in) {
			t.Logf("in: %q", in)
			t.Logf("out: %q", out)
			panic("marshal/unmarshal round trip failure")
		}
	})
}

func FuzzPhxParse(f *testing.F) {
	f.Fuzz(func(t *testing.T, in []byte) {
		msg, err := Parse(in)
		if msg == nil && err == nil {
			panic("no msg, no err")
		}
	})
}
