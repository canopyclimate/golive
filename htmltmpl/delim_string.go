// Code generated by "stringer -type delim"; DO NOT EDIT.

package htmltmpl

import "strconv"

const _delim_name = "delimNonedelimDoubleQuotedelimSingleQuotedelimSpaceOrTagEnd"

var _delim_index = [...]uint8{0, 9, 25, 41, 59}

func (i delim) String() string {
	if i >= delim(len(_delim_index)-1) {
		return "delim(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _delim_name[_delim_index[i]:_delim_index[i+1]]
}
