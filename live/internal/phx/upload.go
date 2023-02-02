package phx

import (
	"fmt"

	"golang.org/x/exp/slices"
)

type UploadMsg struct {
	JoinRef string
	MsgRef  string
	Topic   string
	Event   string
	Payload []byte
}

func (u *UploadMsg) Equal(v *UploadMsg) bool {
	return v.JoinRef == u.JoinRef && v.MsgRef == u.MsgRef && v.Topic == u.Topic && v.Event == u.Event && string(v.Payload) == string(u.Payload)
}

func (u *UploadMsg) UnmarshalBinary(data []byte) error {
	// read size header from buffer
	if len(data) < 5 {
		return fmt.Errorf("buffer too short")
	}
	sh := data[:5]

	// get size data from size header
	ss := int(sh[0])
	if ss != 0 {
		return fmt.Errorf("expected ss to be 0, got %d", ss)
	}
	j := int(sh[1])
	m := int(sh[2])
	t := int(sh[3])
	e := int(sh[4])

	// calc header length
	hl := ss + j + m + t + e
	if 5+hl > len(data) {
		return fmt.Errorf("invalid header length")
	}

	// get header data
	h := string(data[5 : 5+hl])

	// read header data
	u.JoinRef = h[:j]
	h = h[j:]
	u.MsgRef = h[:m]
	h = h[m:]
	u.Topic = h[:t]
	h = h[t:]
	u.Event = h[:e]

	// get payload data
	u.Payload = slices.Clone(data[5+hl:]) // defensive copy
	return nil
}

func (u UploadMsg) MarshalBinary() ([]byte, error) {
	// get lengths
	jl := len(u.JoinRef)
	ml := len(u.MsgRef)
	tl := len(u.Topic)
	el := len(u.Event)
	pl := len(u.Payload)

	// calc header length
	hl := jl + ml + tl + el

	// make buffer
	b := make([]byte, 5+hl+pl)

	// create size header
	b[0] = 0
	b[1] = byte(jl)
	b[2] = byte(ml)
	b[3] = byte(tl)
	b[4] = byte(el)

	// write header and payload
	h := b[5:5]
	h = append(h, u.JoinRef...)
	h = append(h, u.MsgRef...)
	h = append(h, u.Topic...)
	h = append(h, u.Event...)
	_ = append(h, u.Payload...)
	return b, nil
}
