package phx

import (
	"encoding/json"
	"fmt"
)

type Msg struct {
	JoinRef string // or nil
	MsgRef  string
	Topic   string
	Event   string
	Payload map[string]any
}

func Parse(msg []byte) (*Msg, error) {
	var raw []any
	err := json.Unmarshal(msg, &raw)
	if err != nil {
		return nil, err
	}

	// messages are always arrays of 5 elements
	if len(raw) != 5 {
		return nil, fmt.Errorf("phx message must contain 5 elements, got %d: %v", len(raw), raw)
	}

	var strings [4]string
	for i, x := range raw[:4] {
		if x == nil {
			continue
		}
		str, ok := x.(string)
		if !ok {
			return nil, fmt.Errorf("invalid format for element %d, got: %T", i, x)
		}
		strings[i] = str
	}

	// cast payload to map
	payload, ok := raw[4].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid payload format, should be map[string]any, got %T: %v", raw[4], raw[4])
	}

	pm := &Msg{
		// Note: Docs say JoinRef can be nil, but type doesn't allow for it.
		// JoinRef can be the empty string here, however, if it's nil in raw.
		JoinRef: strings[0],
		MsgRef:  strings[1],
		Topic:   strings[2],
		Event:   strings[3],
		Payload: payload,
	}
	return pm, nil
}
