package json

import (
	"reflect"
	"strings"
	"testing"
)

func TestAppendString(t *testing.T) {
	tests := []struct {
		in          string
		want        string
		wantErrUTF8 error
	}{
		{"", `""`, nil},
		{"hello", `"hello"`, nil},
		{"\x00", `"\u0000"`, nil},
		{"\x1f", `"\u001f"`, nil},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", `"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"`, nil},
		{" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f", "\" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f\"", nil},
		{"x\x80\ufffd", "\"x\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xff\ufffd", "\"x\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xc0", "\"x\ufffd\"", ErrInvalidUTF8},
		{"x\xc0\x80", "\"x\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xe0", "\"x\ufffd\"", ErrInvalidUTF8},
		{"x\xe0\x80", "\"x\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xe0\x80\x80", "\"x\ufffd\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xf0", "\"x\ufffd\"", ErrInvalidUTF8},
		{"x\xf0\x80", "\"x\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xf0\x80\x80", "\"x\ufffd\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xf0\x80\x80\x80", "\"x\ufffd\ufffd\ufffd\ufffd\"", ErrInvalidUTF8},
		{"x\xed\xba\xad", "\"x\ufffd\ufffd\ufffd\"", ErrInvalidUTF8},
		{"\"\\/\b\f\n\r\t", `"\"\\/\b\f\n\r\t"`, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", `"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃)."`, nil},
		{"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", "\"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602\"", nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\u0080\u2028\u2029\ufffd\U0001f602\"", nil},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, gotErr := AppendString(nil, tt.in)
			switch {
			case tt.wantErrUTF8 == nil && (string(got) != tt.want || gotErr != nil):
				t.Errorf("appendString(nil, %q) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, nil)
			case tt.wantErrUTF8 != nil && (!strings.HasPrefix(tt.want, string(got)) || !reflect.DeepEqual(gotErr, tt.wantErrUTF8)):
				t.Errorf("appendString(nil, %q, true, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErrUTF8)
			}
		})
	}
}
