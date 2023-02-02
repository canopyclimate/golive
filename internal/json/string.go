package json

import (
	"errors"
	"math/bits"
	"strconv"
	"unicode/utf8"
)

var ErrInvalidUTF8 = errors.New("invalid UTF-8")

// EscapeString escapes JSON string contents.
// It does not add quotes around it.
// It returns ErrInvalidUTF8 if s contains invalid UTF-8.
// That is the only possible error.
func EscapeString(s string) (string, error) {
	b, err := AppendString(nil, s)
	return string(b), err
}

// The following code is extracted from https://github.com/go-json-experiment/json
// at commit 3fecd76f5acdb9037c6c96eb6b869c69293f9c21 (Nov 7, 2022).

// appendString appends src to dst as a JSON string per RFC 7159, section 7.
// This rejects input that contains invalid UTF-8.
func AppendString(dst []byte, src string) ([]byte, error) {
	var i, n int
	dst = append(dst, '"')
	for uint(len(src)) > uint(n) {
		// Handle single-byte ASCII.
		if c := src[n]; c < utf8.RuneSelf {
			n++
			if c < ' ' || c == '"' || c == '\\' {
				dst = append(dst, src[i:n-1]...)
				dst = appendEscapedASCII(dst, c)
				i = n
			}
			continue
		}

		// Handle multi-byte Unicode.
		_, rn := utf8.DecodeRuneInString(src[n:])
		n += rn
		if rn == 1 { // must be utf8.RuneError since we already checked for single-byte ASCII
			dst = append(dst, src[i:n-rn]...)
			return dst, ErrInvalidUTF8
		}
	}
	dst = append(dst, src[i:n]...)
	dst = append(dst, '"')
	return dst, nil
}

func appendEscapedASCII(dst []byte, c byte) []byte {
	switch c {
	case '"', '\\':
		dst = append(dst, '\\', c)
	case '\b':
		dst = append(dst, "\\b"...)
	case '\f':
		dst = append(dst, "\\f"...)
	case '\n':
		dst = append(dst, "\\n"...)
	case '\r':
		dst = append(dst, "\\r"...)
	case '\t':
		dst = append(dst, "\\t"...)
	default:
		dst = append(dst, "\\u"...)
		dst = appendHexUint16(dst, uint16(c))
	}
	return dst
}

// appendHexUint16 appends src to dst as a 4-byte hexadecimal number.
func appendHexUint16(dst []byte, src uint16) []byte {
	dst = append(dst, "0000"[1+(bits.Len16(src)-1)/4:]...)
	dst = strconv.AppendUint(dst, uint64(src), 16)
	return dst
}
